package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/mail"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/notification"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

// Email job timing.
const (
	DispatchInterval = time.Minute
	PurgeInterval    = 24 * time.Hour
	emailMaxAttempts = 8
	dispatchBatch    = 5000
	purgeBatch       = 1000
	// purgeMaxBatches bounds one run; the next run deletes the rest.
	purgeMaxBatches = 100

	ActionPurged        = "notification.purged"
	TargetNotification  = "notification"
	ActorNamePurge      = "notification_purge"
	dataCount           = "count"
	timeLayoutToday     = "15:04"
	timeLayoutOtherDay  = "2 Jan, 15:04"
	settingsFragment    = "settings"
	pathTickets         = "tickets"
	pathPortal          = "portal"
	pathNotificationsUI = "notifications"
)

// allClientRoles see every client's tickets (the tickets package's CapClientsViewAll).
var allClientRoles = func() []string {
	var out []string
	for _, r := range auth.RolesWith(auth.CapClientsViewAll) {
		out = append(out, string(r))
	}
	return out
}()

// emailInsertOpts queue each email once per group or period.
var emailInsertOpts = &river.InsertOpts{MaxAttempts: emailMaxAttempts, UniqueOpts: river.UniqueOpts{ByArgs: true}}

// DispatchJob runs the dispatcher every minute. Register it only on clients that run jobs.
func DispatchJob() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(DispatchInterval),
		func() (river.JobArgs, *river.InsertOpts) { return notification.DispatchArgs{}, nil },
		&river.PeriodicJobOpts{ID: notification.JobKindDispatch, RunOnStart: true})
}

// PurgeJob deletes old notifications daily. Register it only on clients that run jobs.
func PurgeJob() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(PurgeInterval),
		func() (river.JobArgs, *river.InsertOpts) { return notification.PurgeArgs{}, nil },
		&river.PeriodicJobOpts{ID: notification.JobKindPurge, RunOnStart: true})
}

// DispatchWorker queues the emails that are due.
type DispatchWorker struct {
	river.WorkerDefaults[notification.DispatchArgs]
	store *store.Store
	jobs  tickets.JobInserter
	now   func() time.Time
}

func NewDispatchWorker(st *store.Store, jobs tickets.JobInserter) *DispatchWorker {
	return &DispatchWorker{store: st, jobs: jobs, now: time.Now}
}

func (w *DispatchWorker) Work(ctx context.Context, _ *river.Job[notification.DispatchArgs]) error {
	now := w.now()
	return w.store.Tx(ctx, func(tx store.Tx) error {
		rows, err := tx.Q.ListPendingEmails(ctx, db.ListPendingEmailsParams{
			StaleBefore: now.Add(-notification.ClaimTimeout), RowLimit: dispatchBatch,
		})
		if err != nil || len(rows) == 0 {
			return err
		}
		loc, err := orgLocation(ctx, tx.Q)
		if err != nil {
			return err
		}
		pending := make([]notification.Pending, len(rows))
		for i, r := range rows {
			pending[i] = notification.Pending{
				ID: r.ID, UserID: r.UserID, TicketID: r.TicketID, Urgent: r.Urgent, CreatedAt: r.CreatedAt,
				Frequency: notification.Frequency(r.EmailFrequency),
			}
		}
		ticketEmails, digests := notification.PlanEmails(pending, now, loc)
		for _, a := range ticketEmails {
			if _, err := w.jobs.InsertTx(ctx, tx.PG, a, emailInsertOpts); err != nil {
				return err
			}
		}
		for _, a := range digests {
			if _, err := w.jobs.InsertTx(ctx, tx.PG, a, emailInsertOpts); err != nil {
				return err
			}
		}
		return nil
	})
}

// Mailer renders and sends notification emails.
type Mailer struct {
	store  *store.Store
	sender mail.Sender
	appURL *url.URL
	log    *slog.Logger
	now    func() time.Time
}

func NewMailer(st *store.Store, sender mail.Sender, appURL *url.URL, log *slog.Logger) *Mailer {
	return &Mailer{store: st, sender: sender, appURL: appURL, log: log, now: time.Now}
}

// TicketEmailWorker sends one user the pending changes to one ticket.
type TicketEmailWorker struct {
	river.WorkerDefaults[notification.TicketEmailArgs]
	m *Mailer
}

func NewTicketEmailWorker(m *Mailer) *TicketEmailWorker { return &TicketEmailWorker{m: m} }

func (w *TicketEmailWorker) Work(ctx context.Context, job *river.Job[notification.TicketEmailArgs]) error {
	user, err := w.m.recipient(ctx, job.Args.UserID)
	if err != nil || user == nil {
		return err
	}
	rows, err := w.m.store.Q().ClaimTicketEmail(ctx, db.ClaimTicketEmailParams{
		UserID: user.ID, TicketID: job.Args.TicketID, StaleBefore: w.m.now().Add(-notification.ClaimTimeout),
		IncludeRoutine: notification.Frequency(user.EmailFrequency) == notification.FrequencyInstant,
	})
	if err != nil {
		return err
	}
	claimed := make([]claimedRow, len(rows))
	for i, r := range rows {
		claimed[i] = claimedRow(r)
	}
	return w.m.deliver(ctx, user, claimed, false)
}

// DigestWorker sends one user's hourly or daily summary.
type DigestWorker struct {
	river.WorkerDefaults[notification.DigestArgs]
	m *Mailer
}

func NewDigestWorker(m *Mailer) *DigestWorker { return &DigestWorker{m: m} }

func (w *DigestWorker) Work(ctx context.Context, job *river.Job[notification.DigestArgs]) error {
	user, err := w.m.recipient(ctx, job.Args.UserID)
	if err != nil || user == nil {
		return err
	}
	rows, err := w.m.store.Q().ClaimDigest(ctx, db.ClaimDigestParams{
		UserID: user.ID, CreatedBefore: job.Args.Before, StaleBefore: w.m.now().Add(-notification.ClaimTimeout),
	})
	if err != nil {
		return err
	}
	claimed := make([]claimedRow, len(rows))
	for i, r := range rows {
		claimed[i] = claimedRow(r)
	}
	return w.m.deliver(ctx, user, claimed, true)
}

// claimedRow is a notification a send job holds.
type claimedRow struct {
	ID          uuid.UUID
	TicketID    uuid.UUID
	EventType   string
	ActorUserID *uuid.UUID
	MessageID   *uuid.UUID
	Data        []byte
	Urgent      bool
	CreatedAt   time.Time
}

type recipient = db.GetEmailRecipientRow

func (m *Mailer) recipient(ctx context.Context, id uuid.UUID) (*recipient, error) {
	u, err := m.store.Q().GetEmailRecipient(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

// deliver sends the claimed rows, as one ticket email or one summary, and marks them. Rows the
// user may no longer get (suspended account, ticket out of reach) are marked skipped. A failed
// send gives the rows back so River's retry claims them again.
func (m *Mailer) deliver(ctx context.Context, user *recipient, rows []claimedRow, digest bool) error {
	if len(rows) == 0 {
		return nil
	}
	q := m.store.Q()
	if auth.UserStatus(user.Status) == auth.UserStatusSuspended {
		return m.mark(ctx, q, ids(rows), true)
	}
	ticketIDs := []uuid.UUID{}
	for _, r := range rows {
		if !slices.Contains(ticketIDs, r.TicketID) {
			ticketIDs = append(ticketIDs, r.TicketID)
		}
	}
	ts, err := q.ListEmailTickets(ctx, db.ListEmailTicketsParams{
		AllClientRoles: allClientRoles, AgentRole: string(auth.RoleAgent), RequesterRole: string(auth.RoleRequester),
		UserID: user.ID, TicketIds: ticketIDs,
	})
	if err != nil {
		return err
	}
	byTicket := map[uuid.UUID]db.ListEmailTicketsRow{}
	for _, t := range ts {
		byTicket[t.ID] = t
	}
	var send, skip []claimedRow
	for _, r := range rows {
		if t, ok := byTicket[r.TicketID]; ok && t.Visible {
			send = append(send, r)
		} else {
			skip = append(skip, r)
		}
	}
	if err := m.mark(ctx, q, ids(skip), true); err != nil || len(send) == 0 {
		return err
	}
	msg, err := m.render(ctx, q, user, send, byTicket, digest)
	if err == nil {
		err = m.sender.Send(ctx, msg)
	}
	if err != nil {
		if rerr := q.ReleaseNotificationClaims(ctx, ids(send)); rerr != nil {
			m.log.ErrorContext(ctx, "release notification claims", "error", rerr)
		}
		return err
	}
	return m.mark(ctx, q, ids(send), false)
}

func (m *Mailer) mark(ctx context.Context, q *db.Queries, rowIDs []uuid.UUID, skipped bool) error {
	if len(rowIDs) == 0 {
		return nil
	}
	return q.MarkNotificationsEmailed(ctx, db.MarkNotificationsEmailedParams{Skipped: skipped, Ids: rowIDs})
}

// render builds the email for the rows (sorted oldest first).
func (m *Mailer) render(ctx context.Context, q *db.Queries, user *recipient, rows []claimedRow,
	byTicket map[uuid.UUID]db.ListEmailTicketsRow, digest bool) (mail.Message, error) {
	org, err := q.GetOrganization(ctx)
	if err != nil {
		return mail.Message{}, err
	}
	loc, err := time.LoadLocation(org.Timezone)
	if err != nil {
		return mail.Message{}, err
	}
	names, err := m.actorNames(ctx, q, rows)
	if err != nil {
		return mail.Message{}, err
	}
	portal := auth.Role(user.Role) == auth.RoleRequester
	f := formatter{loc: loc, now: m.now(), portal: portal, names: names}
	settingsURL := m.settingsURL(portal)
	headers := mail.AutomaticHeaders(settingsURL)
	slices.SortStableFunc(rows, func(a, b claimedRow) int { return a.CreatedAt.Compare(b.CreatedAt) })

	if digest {
		data := mail.DigestData{OrgName: org.Name, Portal: portal, Count: len(rows), SettingsURL: settingsURL,
			Daily: notification.Frequency(user.EmailFrequency) == notification.FrequencyDaily}
		index := map[uuid.UUID]int{}
		for _, r := range rows {
			i, ok := index[r.TicketID]
			if !ok {
				t := byTicket[r.TicketID]
				i = len(data.Tickets)
				index[r.TicketID] = i
				data.Tickets = append(data.Tickets, mail.DigestTicket{
					TicketKey: t.Key, Title: t.Subject, Priority: t.Priority, PriorityName: t.PriorityName, ProjectName: t.ProjectName,
					TicketURL: m.ticketURL(portal, t.Key),
				})
			}
			data.Tickets[i].Notices = append(data.Tickets[i].Notices, f.notice(r))
		}
		msg, err := mail.Render(mail.TemplateDigest, user.Email, user.Name, data)
		msg.Headers = headers
		return msg, err
	}

	t := byTicket[rows[0].TicketID]
	main := mainRow(rows)
	data := mail.TicketUpdateData{
		OrgName: org.Name, Portal: portal, TicketKey: t.Key, Title: t.Subject, Priority: t.Priority, PriorityName: t.PriorityName,
		ProjectName: t.ProjectName, Main: f.notice(main), TicketURL: m.ticketURL(portal, t.Key), SettingsURL: settingsURL,
		Relation: relation(user, t, main),
	}
	for _, a := range f.data(main).Also {
		data.Changes = append(data.Changes, mail.ChangeData{
			Change: string(a.Change), Portal: portal, StatusFrom: string(a.StatusFrom), StatusTo: string(a.StatusTo),
			PriorityFrom: string(a.PriorityFrom), PriorityTo: string(a.PriorityTo), Assignee: a.Assignee,
		})
	}
	for _, r := range rows {
		if r.ID != main.ID {
			data.Others = append(data.Others, f.notice(r))
		}
	}
	if data.Quote, err = m.quote(ctx, q, portal, main, rows, f); err != nil {
		return mail.Message{}, err
	}
	msg, err := mail.Render(mail.TemplateTicketUpdate, user.Email, user.Name, data)
	msg.Headers = headers
	return msg, err
}

// mainRow is the email's headline: the most important change, the latest among equals.
func mainRow(rows []claimedRow) claimedRow {
	main := rows[0]
	for _, r := range rows[1:] {
		if notification.Rank(notification.Event(r.EventType)) <= notification.Rank(notification.Event(main.EventType)) {
			main = r
		}
	}
	return main
}

// quote is the message the email quotes: the headline's, else the latest the rows name.
// Client staff only ever see public messages.
func (m *Mailer) quote(ctx context.Context, q *db.Queries, portal bool, main claimedRow, rows []claimedRow, f formatter) (*mail.QuoteData, error) {
	id := main.MessageID
	for i := len(rows) - 1; id == nil && i >= 0; i-- {
		id = rows[i].MessageID
	}
	if id == nil {
		return nil, nil
	}
	msg, err := q.GetEmailQuote(ctx, *id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if portal && msg.Visibility != publicVisibility {
		return nil, nil
	}
	return &mail.QuoteData{
		Author: msg.AuthorName, FromSupport: auth.Role(msg.AuthorRole) != auth.RoleRequester,
		Time: f.time(msg.CreatedAt), Body: msg.Body,
	}, nil
}

func (m *Mailer) actorNames(ctx context.Context, q *db.Queries, rows []claimedRow) (map[uuid.UUID]string, error) {
	var actorIDs []uuid.UUID
	for _, r := range rows {
		if r.ActorUserID != nil && !slices.Contains(actorIDs, *r.ActorUserID) {
			actorIDs = append(actorIDs, *r.ActorUserID)
		}
	}
	names := map[uuid.UUID]string{}
	if len(actorIDs) == 0 {
		return names, nil
	}
	users, err := q.ListUserNames(ctx, actorIDs)
	for _, u := range users {
		names[u.ID] = u.Name
	}
	return names, err
}

func (m *Mailer) ticketURL(portal bool, key string) string {
	if portal {
		return m.appURL.JoinPath(pathPortal, pathTickets, key).String()
	}
	return m.appURL.JoinPath(pathTickets, key).String()
}

func (m *Mailer) settingsURL(portal bool) string {
	u := m.appURL.JoinPath(pathNotificationsUI)
	if portal {
		u = m.appURL.JoinPath(pathPortal, pathNotificationsUI)
	}
	u.Fragment = settingsFragment
	return u.String()
}

// relation is why the user gets the email, for its footer.
func relation(user *recipient, t db.ListEmailTicketsRow, main claimedRow) mail.Relation {
	switch {
	case notification.Event(main.EventType) == notification.EventTicketLogged:
		return mail.RelationLogged
	case user.ID == t.RequesterID:
		return mail.RelationReported
	case t.Watching:
		return mail.RelationWatching
	case t.AssigneeID != nil && *t.AssigneeID == user.ID:
		return mail.RelationAssigned
	case notification.Event(main.EventType) == notification.EventSLA && auth.Role(user.Role) == auth.RoleSupervisor:
		return mail.RelationSupervisor
	}
	return mail.RelationTeam
}

// formatter turns stored rows into email lines in the organisation's time zone.
type formatter struct {
	loc    *time.Location
	now    time.Time
	portal bool
	names  map[uuid.UUID]string
}

func (f formatter) data(r claimedRow) notification.Data {
	var d notification.Data
	_ = json.Unmarshal(r.Data, &d)
	return d
}

func (f formatter) notice(r claimedRow) mail.NoticeData {
	d := f.data(r)
	n := mail.NoticeData{
		Event: r.EventType, Kind: string(d.Kind), Portal: f.portal, StatusFrom: string(d.StatusFrom),
		StatusTo: string(d.StatusTo), PriorityFrom: string(d.PriorityFrom), PriorityTo: string(d.PriorityTo),
		Assignee: d.Assignee, Reason: d.Reason, SLAClock: d.SLAClock, SLAMinutesLeft: d.SLALeftMinutes,
		Time: f.time(r.CreatedAt),
	}
	if r.ActorUserID != nil {
		n.ActorName = f.names[*r.ActorUserID]
	}
	if d.SLADueAt != nil {
		n.SLADue = f.time(*d.SLADueAt)
	}
	return n
}

// time is "14:02" today, "12 Nov, 14:02" otherwise.
func (f formatter) time(t time.Time) string {
	local, today := t.In(f.loc), f.now.In(f.loc)
	if local.Year() == today.Year() && local.YearDay() == today.YearDay() {
		return local.Format(timeLayoutToday)
	}
	return local.Format(timeLayoutOtherDay)
}

func ids(rows []claimedRow) []uuid.UUID {
	out := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	return out
}

func orgLocation(ctx context.Context, q *db.Queries) (*time.Location, error) {
	tz, err := q.GetOrganizationTimezone(ctx)
	if err != nil {
		return nil, err
	}
	return time.LoadLocation(tz)
}

// PurgeWorker deletes notifications older than notification.Retention.
type PurgeWorker struct {
	river.WorkerDefaults[notification.PurgeArgs]
	store *store.Store
	now   func() time.Time
}

func NewPurgeWorker(st *store.Store) *PurgeWorker { return &PurgeWorker{store: st, now: time.Now} }

func (w *PurgeWorker) Work(ctx context.Context, _ *river.Job[notification.PurgeArgs]) error {
	cutoff := w.now().Add(-notification.Retention)
	var total int64
	for range purgeMaxBatches {
		n, err := w.store.Q().DeleteOldNotifications(ctx, db.DeleteOldNotificationsParams{
			CreatedBefore: cutoff, Batch: purgeBatch,
		})
		if err != nil {
			return err
		}
		total += n
		if n < purgeBatch {
			break
		}
	}
	if total == 0 {
		return nil
	}
	return audit.Write(ctx, w.store.Q(), audit.SystemActor(ActorNamePurge).Entry(
		ActionPurged, TargetNotification, nil, map[string]any{dataCount: total}))
}

// LateInserter is a job inserter whose River client is set after the workers are registered:
// River needs its workers before the client exists, and the dispatcher needs the client.
type LateInserter struct {
	tickets.JobInserter
}

// PicEmailWorker tells a client's contact person that a P1 ticket missed its resolution target
// (the project's "notify the client PIC" switch). The contact may have no account, so there is
// no inbox item and the link goes to the portal.
type PicEmailWorker struct {
	river.WorkerDefaults[notification.PicEmailArgs]
	m *Mailer
}

func NewPicEmailWorker(m *Mailer) *PicEmailWorker { return &PicEmailWorker{m: m} }

func (w *PicEmailWorker) Work(ctx context.Context, job *river.Job[notification.PicEmailArgs]) error {
	q := w.m.store.Q()
	c, err := q.GetPicEmailContext(ctx, job.Args.TicketID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil || c.PicEmail == nil || *c.PicEmail == "" || c.ResolutionBreachedAt == nil {
		return err
	}
	var snap ticket.SLASnapshot
	if err := json.Unmarshal(c.SlaSnapshot, &snap); err != nil {
		return err
	}
	eff, err := snap.Effective(sla.Priority(c.Priority), ticket.Type(c.Type))
	if err != nil {
		return err
	}
	org, err := q.GetOrganization(ctx)
	if err != nil {
		return err
	}
	loc, err := time.LoadLocation(org.Timezone)
	if err != nil {
		return err
	}
	f := formatter{loc: loc, now: w.m.now()}
	name := c.ClientName
	if c.PicName != nil && *c.PicName != "" {
		name = *c.PicName
	}
	msg, err := mail.Render(mail.TemplatePicBreach, *c.PicEmail, name, mail.PicBreachData{
		OrgName: org.Name, ClientName: c.ClientName, TicketKey: c.Key, Title: c.Subject, Priority: c.Priority,
		PriorityName: c.PriorityName, ProjectName: c.ProjectName, TargetValue: eff.Resolution.Value,
		TargetUnit: string(eff.Resolution.Unit), PassedAt: f.time(*c.ResolutionBreachedAt), TicketURL: w.m.ticketURL(true, c.Key),
	})
	if err != nil {
		return err
	}
	msg.Headers = mail.AutoReplyHeaders()
	return w.m.sender.Send(ctx, msg)
}
