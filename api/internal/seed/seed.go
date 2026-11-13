// Package seed loads demo data for local development (HANDOFF §16).
package seed

import (
	"context"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/config"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/holidaypreset"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

const (
	tableMinWidth = 0
	tableTabWidth = 0
	tablePadding  = 2
	tablePadChar  = ' '
)

const (
	// DevPassword is the password of every seeded user.
	DevPassword = "simdesk-dev"
	OrgName     = "Demo Vendor"
	// orgFooterNote is the demo data-residency line shown in the footer.
	orgFooterNote = "Data stored in Indonesia · Personal data handled under UU PDP"
	// RequesterClient is the client the demo requester belongs to.
	RequesterClient = "Dinas Kominfo Jambi"
	// GoldPlan is the demo premium SLA plan. Its numbers are placeholders (HANDOFF §13 Q3).
	GoldPlan = "Gold"

	emailRina   = "rina@demo-vendor.test"
	emailAdi    = "adi@demo-vendor.test"
	emailYoga   = "yoga@demo-vendor.test"
	emailHendra = "hendra@kominfo-jambi.test"
	emailDian   = "dian@kominfo-jambi.test"
	emailSri    = "sri.lestari@bpkad-batam.test"

	clientBatam  = "BPKAD Kota Batam"
	clientRiau   = "Bappeda Prov. Riau"
	clientBintan = "Diskominfo Kab. Bintan"

	// workingDay is one 08:00–17:00 working day in minutes (the default business hours).
	workingDay = 540
	// sortStep spaces list entries so admins can slot new ones in between.
	sortStep = 10
)

// goldTargets are the Standard targets with P2 also running 24/7.
var goldTargets = []sla.Target{
	{Priority: sla.P1, FirstResponseMinutes: 15, ResolveMinutes: 240, DeliverMinutes: 240, Calendar: sla.CalendarAlways},
	{Priority: sla.P2, FirstResponseMinutes: 30, ResolveMinutes: 480, DeliverMinutes: 540, Calendar: sla.CalendarAlways},
	{Priority: sla.P3, FirstResponseMinutes: 120, ResolveMinutes: 1080, DeliverMinutes: 1080, DeliverFromCatalog: true, Calendar: sla.CalendarBusinessHours},
	{Priority: sla.P4, FirstResponseMinutes: 240, ResolveMinutes: 2700, DeliverMinutes: 2700, Calendar: sla.CalendarBusinessHours},
}

var ErrNotDev = errors.New("seed only runs when APP_ENV=" + config.EnvDev)

// client is a demo client from the mock (Settings › Clients). Empty plan means the default.
type client struct {
	name, contractNo, start, end, plan, picName, picEmail string
	agents                                                []string // emails
}

var clients = []client{
	{RequesterClient, "KTR/KOMINFO-JMB/006/2026", "2026-01-01", "2026-12-31", GoldPlan, "Hendra Gunawan", "hendra@kominfo-jambi.test", []string{emailRina, emailAdi}},
	{clientBatam, "KTR/BPKAD-BTM/014/2026", "2026-03-01", "2027-02-28", "", "Sri Lestari", emailSri, []string{emailYoga}},
	{clientRiau, "SPK/BAPPEDA-RIAU/031/2026", "2026-01-01", "2026-12-31", "", "Fajar Nugroho", "fajar@bappeda-riau.test", []string{emailYoga}},
	{clientBintan, "KTR/KOMINFO-BTN/009/2026", "2026-06-01", "2027-05-31", "", "Maya Sari", "maya@kominfo-bintan.test", []string{emailYoga}},
	{"Kementerian PANRB", "KTR/PANRB/112/2026", "2026-01-01", "2026-12-31", GoldPlan, "Arif Rahman", "arif@panrb.test", []string{emailRina, emailAdi}},
	{name: "Setda Kab. Muaro Jambi"},
	{name: "Sekretariat DPRD Kota Batam"},
	{name: "Disdukcapil Kab. Muaro Jambi"},
}

type person struct {
	email, name string
	role        auth.Role
	client      string
	jobTitle    string
	invited     bool // added by an agent; no password until an invite (M9)
}

var people = []person{
	{"admin@demo-vendor.test", "Demo Admin", auth.RoleAdmin, "", "", false},
	{"dewi@demo-vendor.test", "Dewi Wulandari", auth.RoleSupervisor, "", "Team lead", false},
	{emailRina, "Rina Pratiwi", auth.RoleAgent, "", "L2", false},
	{emailAdi, "Adi Saputra", auth.RoleAgent, "", "L2", false},
	{emailYoga, "Yoga Hidayat", auth.RoleAgent, "", "L1", false},
	{emailHendra, "Hendra Gunawan", auth.RoleRequester, RequesterClient, "IT Officer", false},
	{emailSri, "Sri Lestari", auth.RoleRequester, clientBatam, "Head of Finance IT", false},
	{emailDian, "Dian Puspita", auth.RoleRequester, RequesterClient, "Public Relations", true},
}

// entry is a "Which system?" or catalogue choice from the mock (NewTicket).
type entry struct {
	name, description string
	deliveryMinutes   int // catalogue items only
}

var systems = []entry{
	{"e-Office", "Letters, dispositions, archives", 0},
	{"Email", "Mailboxes and accounts", 0},
	{"Network / VPN", "Internet, Wi-Fi, remote access", 0},
	{"Website", "Public website", 0},
	{"SIPD / finance apps", "Budgeting and finance systems", 0},
	{"Computer or printer", "Hardware at your desk", 0},
	{"Other", "", 0},
}

var catalogItems = []entry{
	{"New email account", "", 1 * workingDay},
	{"VPN / remote access", "", 2 * workingDay},
	{"Install licensed software", "", 2 * workingDay},
	{"New or replacement device", "", 5 * workingDay},
	{"Something else", "", 5 * workingDay},
}

// Run upserts the demo organisation, SLA plans, clients, users, agent assignments and
// holidays. It is safe to run repeatedly.
func Run(ctx context.Context, env string, st *store.Store, hasher *auth.Hasher, out io.Writer) error {
	if env != config.EnvDev {
		return ErrNotDev
	}
	hash, err := hasher.Hash(ctx, DevPassword)
	if err != nil {
		return err
	}
	err = st.Tx(ctx, func(tx store.Tx) error {
		footer := orgFooterNote
		if err := tx.Q.UpdateOrganizationProfile(ctx, db.UpdateOrganizationProfileParams{
			Name: OrgName, FooterNote: &footer,
		}); err != nil {
			return err
		}
		plans, err := seedPlans(ctx, tx.Q)
		if err != nil {
			return err
		}
		clientIDs := make(map[string]uuid.UUID, len(clients))
		for _, c := range clients {
			id, err := upsertClient(ctx, tx.Q, c, plans)
			if err != nil {
				return fmt.Errorf("client %s: %w", c.name, err)
			}
			clientIDs[c.name] = id
		}
		userIDs := make(map[string]uuid.UUID, len(people))
		for _, p := range people {
			params := db.UpsertUserByEmailParams{
				Email: p.email, Name: p.name, PasswordHash: &hash, Role: string(p.role),
				Status: string(auth.UserStatusActive),
			}
			if p.invited {
				params.PasswordHash, params.Status = nil, string(auth.UserStatusInvited)
			}
			if p.client != "" {
				id := clientIDs[p.client]
				params.ClientID = &id
			}
			if p.jobTitle != "" {
				title := p.jobTitle
				params.JobTitle = &title
			}
			id, err := tx.Q.UpsertUserByEmail(ctx, params)
			if err != nil {
				return fmt.Errorf("user %s: %w", p.email, err)
			}
			userIDs[p.email] = id
		}
		for _, c := range clients {
			ids := make([]uuid.UUID, 0, len(c.agents))
			for _, email := range c.agents {
				ids = append(ids, userIDs[email])
			}
			if err := tx.Q.InsertClientAgents(ctx, db.InsertClientAgentsParams{ClientID: clientIDs[c.name], UserIds: ids}); err != nil {
				return fmt.Errorf("agents of %s: %w", c.name, err)
			}
		}
		if err := seedHolidays(ctx, tx.Q); err != nil {
			return err
		}
		if _, err := seedProjects(ctx, tx.Q, clientIDs); err != nil {
			return err
		}
		lists, err := seedCatalog(ctx, tx.Q)
		if err != nil {
			return err
		}
		return seedTickets(ctx, tx.Q, time.Now(), refs{clients: clientIDs, users: userIDs, lists: lists})
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Seeded organisation %q with %d clients, %d projects, %d users, %d tickets and holidays for %v.\n\n",
		OrgName, len(clients), len(demoProjects), len(people), len(demoTickets), holidaypreset.Years())
	tw := tabwriter.NewWriter(out, tableMinWidth, tableTabWidth, tablePadding, tablePadChar, 0)
	fmt.Fprintln(tw, "EMAIL\tNAME\tROLE")
	for _, p := range people {
		if !p.invited {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", p.email, p.name, p.role)
		}
	}
	_ = tw.Flush()
	fmt.Fprintf(out, "\nPassword for every user: %s\n", DevPassword)
	return nil
}

// seedPlans adds the Gold plan and returns plan IDs by name; "" maps to the default plan.
func seedPlans(ctx context.Context, q *db.Queries) (map[string]uuid.UUID, error) {
	goldID, err := q.UpsertSlaPlanByName(ctx, GoldPlan)
	if err != nil {
		return nil, err
	}
	for _, t := range goldTargets {
		if err := q.UpsertSlaTarget(ctx, db.UpsertSlaTargetParams{
			PlanID: goldID, Priority: string(t.Priority), FirstResponseMinutes: int32(t.FirstResponseMinutes),
			ResolveMinutes: int32(t.ResolveMinutes), DeliverMinutes: int32(t.DeliverMinutes),
			DeliverFromCatalog: t.DeliverFromCatalog, Calendar: string(t.Calendar),
		}); err != nil {
			return nil, err
		}
	}
	all, err := q.ListSlaPlans(ctx)
	if err != nil {
		return nil, err
	}
	ids := map[string]uuid.UUID{GoldPlan: goldID}
	for _, p := range all {
		if p.IsDefault {
			ids[""] = p.ID
		}
	}
	return ids, nil
}

func upsertClient(ctx context.Context, q *db.Queries, c client, plans map[string]uuid.UUID) (uuid.UUID, error) {
	start, err := optionalDate(c.start)
	if err != nil {
		return uuid.Nil, err
	}
	end, err := optionalDate(c.end)
	if err != nil {
		return uuid.Nil, err
	}
	return q.UpsertClientDetails(ctx, db.UpsertClientDetailsParams{
		Name: c.name, ContractNo: optional(c.contractNo), ContractStart: start, ContractEnd: end,
		PicName: optional(c.picName), PicEmail: optional(c.picEmail), SlaPlanID: plans[c.plan],
	})
}

// seedHolidays loads every bundled holiday list, keeping dates that already exist.
func seedHolidays(ctx context.Context, q *db.Queries) error {
	for _, year := range holidaypreset.Years() {
		p, _ := holidaypreset.ForYear(year)
		for _, h := range p.Holidays {
			if _, err := q.InsertHolidayIfMissing(ctx, db.InsertHolidayIfMissingParams{Date: h.Date, Name: h.Name}); err != nil {
				return fmt.Errorf("holiday %s: %w", h.Date, err)
			}
		}
	}
	return nil
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func optionalDate(s string) (*calendar.Date, error) {
	if s == "" {
		return nil, nil
	}
	d, err := calendar.ParseDate(s)
	if err != nil {
		return nil, err
	}
	return &d, nil
}
