package seed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/organization"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// The second demo vendor (owner, 17 Nov: one install holds several vendor organizations). It
// serves its own client from Makassar, on its own calendar. Rina works in both: an agent at Demo
// Vendor and the supervisor here, so the organization switcher shows two roles.
const (
	SecondOrgName     = "PT Mitra Data"
	secondOrgPrefix   = "MD"
	secondOrgTimezone = "Asia/Makassar"

	clientMakassar = "Dinas Kesehatan Kota Makassar"
	projectSIMPUS  = "SIMPUS"

	emailPutu = "putu@mitra-data.test"
	emailNur  = "nur@dinkes-makassar.test"
)

var secondClient = client{
	clientMakassar, "SPK/DINKES-MKS/021/2026", "2026-02-01", "2027-01-31", "Nur Aisyah", emailNur, []string{emailPutu},
}

// secondPeople work only at the second vendor.
var secondPeople = []person{
	{emailPutu, "Putu Ardana", auth.RoleAgent, "", "L1", false},
	{emailNur, "Nur Aisyah", auth.RoleRequester, clientMakassar, "Head of Health Information", false},
}

// secondMembers are the second vendor's team and their role there.
var secondMembers = []struct {
	email string
	role  auth.Role
}{
	{emailRina, auth.RoleSupervisor},
	{emailPutu, auth.RoleAgent},
}

var secondProjects = []project{
	{clientMakassar, projectSIMPUS, "Health centre information system", sla.DefaultProjectSLA(), confirmDoD()},
}

// secondTickets number on from the first vendor's demo tickets: numbers are install-wide.
var secondTickets = []demoTicket{
	{
		number: 24893, typ: ticket.TypeIncident, priority: sla.P2, channel: ticket.ChannelPortal,
		client: clientMakassar, requester: emailNur, creator: emailNur, what: projectSIMPUS,
		subject:     "Patient queue screen frozen at Puskesmas Batua",
		description: "The waiting-room screen has shown the same queue number since 09:00. Registration still works.",
		age:         3 * minutesPerHour,
		steps: []demoStep{
			{10, emailRina, emailPutu, "", ""}, {25, emailPutu, "", ticket.StatusInProgress, ""},
		},
		messages: []demoMessage{
			{20, emailPutu, false, "Hi Bu Nur, the queue display lost its connection to the server. We're restarting the service."},
		},
	},
	{
		number: 24895, typ: ticket.TypeIncident, priority: sla.P3, channel: ticket.ChannelEmail,
		client: clientMakassar, requester: emailNur, creator: emailPutu, what: projectSIMPUS,
		subject:     "Monthly visit report misses two health centres",
		description: "Bu Nur emailed: October's visit report has no rows for Puskesmas Kassi-Kassi and Puskesmas Tamalate.",
		age:         minutesPerDay,
		steps: []demoStep{
			{30, emailRina, emailPutu, "", ""}, {60, emailPutu, "", ticket.StatusInProgress, ""},
			{90, emailPutu, "", ticket.StatusWaitingOnClient, ""},
		},
		messages: []demoMessage{
			{85, emailPutu, false, "Both centres last synced on 28 October. Could their staff open SIMPUS once while online so the data uploads?"},
		},
	},
	{
		number: 24897, typ: ticket.TypeIncident, priority: sla.P4, channel: ticket.ChannelPortal,
		client: clientMakassar, requester: emailNur, creator: emailNur, what: projectSIMPUS,
		subject:     "Lab results print without the clinic's logo",
		description: "Since the last update, printed lab results have an empty box where the clinic's logo was.",
		age:         40,
	},
}

// seedSecondVendor adds the second vendor, its client, team, calendar, project and tickets.
// users gains the second vendor's people.
func seedSecondVendor(ctx context.Context, tx store.Tx, hash string, now time.Time, users map[string]uuid.UUID) error {
	org, err := secondOrganization(ctx, tx)
	if err != nil {
		return err
	}
	clientID, err := upsertClient(ctx, tx.Q, org, secondClient)
	if err != nil {
		return fmt.Errorf("client %s: %w", secondClient.name, err)
	}
	clientIDs := map[string]uuid.UUID{secondClient.name: clientID}
	for _, p := range secondPeople {
		id, err := upsertPerson(ctx, tx.Q, hash, p, clientIDs)
		if err != nil {
			return err
		}
		users[p.email] = id
	}
	for _, m := range secondMembers {
		if err := tx.Q.UpsertMembership(ctx, db.UpsertMembershipParams{UserID: users[m.email], OrganizationID: org, Role: string(m.role)}); err != nil {
			return fmt.Errorf("member %s: %w", m.email, err)
		}
	}
	agents := make([]uuid.UUID, 0, len(secondClient.agents))
	for _, email := range secondClient.agents {
		agents = append(agents, users[email])
	}
	if err := tx.Q.InsertClientAgents(ctx, db.InsertClientAgentsParams{OrganizationID: org, ClientID: clientID, UserIds: agents}); err != nil {
		return fmt.Errorf("agents of %s: %w", secondClient.name, err)
	}
	if err := seedHolidays(ctx, tx.Q, org); err != nil {
		return err
	}
	projectIDs, err := seedProjects(ctx, tx.Q, org, clientIDs, secondProjects)
	if err != nil {
		return err
	}
	r := refs{org: org, clients: clientIDs, users: users, projects: projectIDs}
	return seedTickets(ctx, tx.Q, now, r, nil, secondTickets)
}

// secondOrganization finds the second vendor by its ticket prefix, or creates it.
func secondOrganization(ctx context.Context, tx store.Tx) (uuid.UUID, error) {
	id, err := tx.Q.GetOrganizationByPrefix(ctx, secondOrgPrefix)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	return organization.CreateTx(ctx, tx, organization.Input{Name: SecondOrgName, TicketPrefix: secondOrgPrefix, Timezone: secondOrgTimezone})
}
