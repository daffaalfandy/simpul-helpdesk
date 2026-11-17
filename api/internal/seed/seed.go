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
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/config"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/holidaypreset"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/organization"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/secretbox"
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

	emailAdmin  = "admin@demo-vendor.test"
	emailDewi   = "dewi@demo-vendor.test"
	emailRina   = "rina@demo-vendor.test"
	emailAdi    = "adi@demo-vendor.test"
	emailYoga   = "yoga@demo-vendor.test"
	emailHendra = "hendra@kominfo-jambi.test"
	emailDian   = "dian@kominfo-jambi.test"
	emailSri    = "sri.lestari@bpkad-batam.test"
	emailFajar  = "fajar@bappeda-riau.test"
	emailMaya   = "maya@kominfo-bintan.test"
	emailArif   = "arif@panrb.test"

	clientBatam  = "BPKAD Kota Batam"
	clientRiau   = "Bappeda Prov. Riau"
	clientBintan = "Diskominfo Kab. Bintan"
	clientPANRB  = "Kementerian PANRB"

	// sortStep spaces list entries so admins can slot new ones in between.
	sortStep = 10
)

var ErrNotDev = errors.New("seed only runs when APP_ENV=" + config.EnvDev)

// client is a demo client from the mock (Settings › Clients).
type client struct {
	name, contractNo, start, end, picName, picEmail string
	agents                                          []string // emails
}

var clients = []client{
	{RequesterClient, "KTR/KOMINFO-JMB/006/2026", "2026-01-01", "2026-12-31", "Hendra Gunawan", "hendra@kominfo-jambi.test", []string{emailRina, emailAdi}},
	{clientBatam, "KTR/BPKAD-BTM/014/2026", "2026-03-01", "2027-02-28", "Sri Lestari", emailSri, []string{emailYoga}},
	{clientRiau, "SPK/BAPPEDA-RIAU/031/2026", "2026-01-01", "2026-12-31", "Fajar Nugroho", emailFajar, []string{emailYoga}},
	{clientBintan, "KTR/KOMINFO-BTN/009/2026", "2026-06-01", "2027-05-31", "Maya Sari", emailMaya, []string{emailYoga}},
	{clientPANRB, "KTR/PANRB/112/2026", "2026-01-01", "2026-12-31", "Arif Rahman", emailArif, []string{emailRina, emailAdi}},
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
	{emailAdmin, "Demo Admin", auth.RoleAdmin, "", "", false},
	{emailDewi, "Dewi Wulandari", auth.RoleSupervisor, "", "Team lead", false},
	{emailRina, "Rina Pratiwi", auth.RoleAgent, "", "L2", false},
	{emailAdi, "Adi Saputra", auth.RoleAgent, "", "L2", false},
	{emailYoga, "Yoga Hidayat", auth.RoleAgent, "", "L1", false},
	{emailHendra, "Hendra Gunawan", auth.RoleRequester, RequesterClient, "IT Officer", false},
	{emailSri, "Sri Lestari", auth.RoleRequester, clientBatam, "Head of Finance IT", false},
	{emailDian, "Dian Puspita", auth.RoleRequester, RequesterClient, "Public Relations", true},
	// The PICs of the other demo clients, so the board has tickets from them (agent-logged).
	{emailFajar, "Fajar Nugroho", auth.RoleRequester, clientRiau, "Head of Planning IT", true},
	{emailMaya, "Maya Sari", auth.RoleRequester, clientBintan, "Web Administrator", true},
	{emailArif, "Arif Rahman", auth.RoleRequester, clientPANRB, "IT Coordinator", true},
}

// catalogItem is a "What do you need?" choice from the mock (NewTicket). Delivery times come
// from each project's SLA.
type catalogItem struct {
	name, description string
}

var catalogItems = []catalogItem{
	{"New email account", ""},
	{"VPN / remote access", ""},
	{"Install licensed software", ""},
	{"New or replacement device", ""},
	{"Something else", ""},
}

// Run upserts the demo organisation, clients, projects, users, agent assignments, holidays,
// catalogue, repositories, tickets with their code activity, and notifications. It is safe to
// run repeatedly. box seals the demo repositories' webhook secret.
func Run(ctx context.Context, env string, st *store.Store, hasher *auth.Hasher, box *secretbox.Box, out io.Writer) error {
	if env != config.EnvDev {
		return ErrNotDev
	}
	hash, err := hasher.Hash(ctx, DevPassword)
	if err != nil {
		return err
	}
	err = st.Tx(ctx, func(tx store.Tx) error {
		org, err := demoOrganization(ctx, tx)
		if err != nil {
			return err
		}
		clientIDs := make(map[string]uuid.UUID, len(clients))
		for _, c := range clients {
			id, err := upsertClient(ctx, tx.Q, org, c)
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
		// The demo has its administrator, so first-run setup is closed like on a real install.
		if err := tx.Q.CompleteSetup(ctx); err != nil {
			return err
		}
		for _, c := range clients {
			ids := make([]uuid.UUID, 0, len(c.agents))
			for _, email := range c.agents {
				ids = append(ids, userIDs[email])
			}
			if err := tx.Q.InsertClientAgents(ctx, db.InsertClientAgentsParams{OrganizationID: org, ClientID: clientIDs[c.name], UserIds: ids}); err != nil {
				return fmt.Errorf("agents of %s: %w", c.name, err)
			}
		}
		if err := seedHolidays(ctx, tx.Q, org); err != nil {
			return err
		}
		projectIDs, err := seedProjects(ctx, tx.Q, org, clientIDs)
		if err != nil {
			return err
		}
		items, err := seedCatalog(ctx, tx.Q, org)
		if err != nil {
			return err
		}
		now := time.Now()
		r := refs{org: org, clients: clientIDs, users: userIDs, projects: projectIDs, items: items}
		repos, err := seedRepositories(ctx, tx.Q, box, now, r)
		if err != nil {
			return err
		}
		if err := seedHistory(ctx, tx.Q, now, r); err != nil {
			return err
		}
		if err := seedTickets(ctx, tx.Q, now, r, repos); err != nil {
			return err
		}
		if err := seedUnknownKey(ctx, tx.Q, repos, r, now); err != nil {
			return err
		}
		return seedNotifications(ctx, tx.Q, now, r)
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
	fmt.Fprintf(out, "Webhook secret for every demo repository: %s\n", DevWebhookSecret)
	return nil
}

func upsertClient(ctx context.Context, q *db.Queries, org uuid.UUID, c client) (uuid.UUID, error) {
	start, err := optionalDate(c.start)
	if err != nil {
		return uuid.Nil, err
	}
	end, err := optionalDate(c.end)
	if err != nil {
		return uuid.Nil, err
	}
	return q.UpsertClientDetails(ctx, db.UpsertClientDetailsParams{
		OrganizationID: org, Name: c.name, ContractNo: optional(c.contractNo), ContractStart: start, ContractEnd: end,
		PicName: optional(c.picName), PicEmail: optional(c.picEmail),
	})
}

// seedHolidays loads every bundled holiday list, keeping dates that already exist.
func seedHolidays(ctx context.Context, q *db.Queries, org uuid.UUID) error {
	for _, year := range holidaypreset.Years() {
		p, _ := holidaypreset.ForYear(year)
		for _, h := range p.Holidays {
			if _, err := q.InsertHolidayIfMissing(ctx, db.InsertHolidayIfMissingParams{OrganizationID: org, Date: h.Date, Name: h.Name}); err != nil {
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

// demoOrganization is the demo vendor: the install's first organization, named for the demo, or
// a new one on a fresh install.
func demoOrganization(ctx context.Context, tx store.Tx) (uuid.UUID, error) {
	footer := orgFooterNote
	first, err := tx.Q.GetFirstOrganization(ctx)
	switch {
	case err == nil:
		return first.ID, tx.Q.UpdateOrganizationProfile(ctx, db.UpdateOrganizationProfileParams{ID: first.ID, Name: OrgName, FooterNote: &footer})
	case !errors.Is(err, pgx.ErrNoRows):
		return uuid.Nil, err
	}
	id, err := organization.CreateTx(ctx, tx, organization.Input{
		Name: OrgName, TicketPrefix: organization.DefaultTicketPrefix, Timezone: organization.DefaultTimezone,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return id, tx.Q.UpdateOrganizationProfile(ctx, db.UpdateOrganizationProfileParams{ID: id, Name: OrgName, FooterNote: &footer})
}
