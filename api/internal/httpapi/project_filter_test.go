package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/projects"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
)

func (s session) projectFilter(t *testing.T) []uuid.UUID {
	t.Helper()
	return decode[gen.ProjectFilter](t, s.do(http.MethodGet, "/me/project-filter", nil), http.StatusOK).ProjectIds
}

func (s session) setProjectFilter(ids ...uuid.UUID) *httptest.ResponseRecorder {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return s.do(http.MethodPut, "/me/project-filter", gen.ProjectFilter{ProjectIds: ids})
}

// boardKeys lists every card on the board, sorted.
func boardKeys(b gen.Board) []string {
	keys := []string{}
	for _, c := range b.Columns {
		for _, card := range c.Cards {
			keys = append(keys, card.Key)
		}
	}
	slices.Sort(keys)
	return keys
}

func (s session) listKeys(t *testing.T) []string {
	t.Helper()
	keys := []string{}
	for _, r := range decode[[]gen.TicketSummary](t, s.do(http.MethodGet, "/tickets", nil), http.StatusOK) {
		keys = append(keys, r.Key)
	}
	slices.Sort(keys)
	return keys
}

func sortedKeys(keys ...string) []string {
	slices.Sort(keys)
	return keys
}

// Each user saves which projects they see on the board, its counts and the ticket list, per
// organization (owner, 17 Nov).
func TestProjectFilter(t *testing.T) {
	te := newTicketEnv(t)
	office := te.requester.report(t, incident(te.projectA, "e-Office down"))
	mail := te.requester.report(t, incident(te.projectA2, "Mailbox full"))
	router := te.otherRequester.report(t, incident(te.projectB, "Batam router"))
	all := sortedKeys(office.Key, mail.Key, router.Key)

	if got := te.supervisor.projectFilter(t); len(got) != 0 {
		t.Fatalf("filter before saving = %v", got)
	}
	if got := boardKeys(te.supervisor.board(t, "")); !slices.Equal(got, all) {
		t.Fatalf("board without a filter = %v", got)
	}

	// Repeats collapse.
	saved := decode[gen.ProjectFilter](t, te.supervisor.setProjectFilter(te.projectA, te.projectB, te.projectA), http.StatusOK)
	if len(saved.ProjectIds) != 2 || !slices.Equal(te.supervisor.projectFilter(t), saved.ProjectIds) {
		t.Fatalf("saved = %v, read back %v", saved.ProjectIds, te.supervisor.projectFilter(t))
	}
	filtered := sortedKeys(office.Key, router.Key)
	b := te.supervisor.board(t, "")
	if got := boardKeys(b); !slices.Equal(got, filtered) {
		t.Errorf("filtered board = %v, want %v", got, filtered)
	}
	if b.Stats.OpenTickets != len(filtered) {
		t.Errorf("open tickets counted = %d, want %d", b.Stats.OpenTickets, len(filtered))
	}
	if got := te.supervisor.listKeys(t); !slices.Equal(got, filtered) {
		t.Errorf("filtered list = %v, want %v", got, filtered)
	}
	// One user's filter is theirs alone; a requester sees their client's tickets regardless.
	if got := boardKeys(te.admin.board(t, "")); !slices.Equal(got, all) {
		t.Errorf("admin board = %v", got)
	}
	if got := te.requester.listKeys(t); !slices.Equal(got, sortedKeys(office.Key, mail.Key)) {
		t.Errorf("requester list = %v", got)
	}
	// Saving the same projects again changes nothing.
	decode[gen.ProjectFilter](t, te.supervisor.setProjectFilter(te.projectB, te.projectA), http.StatusOK)
	if n := te.count(t, `SELECT count(*) FROM audit_log WHERE action = $1`, projects.ActionFilterChanged); n != 1 {
		t.Errorf("filter audit entries = %d, want 1", n)
	}

	// Agents pick among their clients' projects only; another organization's project is refused.
	if code := problemCode(t, te.agent.setProjectFilter(te.projectB)); code != apperr.CodeInvalidProject {
		t.Errorf("agent picking an unlinked client's project: %s", code)
	}
	ctx := context.Background()
	org2 := te.secondOrganization(t, "PT Mitra Data", "MD")
	foreignClient, err := te.st.Q().UpsertClient(ctx, db.UpsertClientParams{OrganizationID: org2, Name: "Dinas Kesehatan"})
	if err != nil {
		t.Fatal(err)
	}
	foreign := testdb.Project(t, te.st, foreignClient, "SIMRS", true)
	if code := problemCode(t, te.supervisor.setProjectFilter(foreign)); code != apperr.CodeInvalidProject {
		t.Errorf("another organization's project: %s", code)
	}
	// The filter belongs to the organization: elsewhere the user starts with every project.
	te.member(t, auth.RoleSupervisor, org2, auth.RoleSupervisor)
	if got := te.supervisor.in(org2).projectFilter(t); len(got) != 0 {
		t.Errorf("filter in the second organization = %v", got)
	}
	decode[gen.ProjectFilter](t, te.supervisor.in(org2).setProjectFilter(foreign), http.StatusOK)
	if got := te.supervisor.projectFilter(t); len(got) != 2 {
		t.Errorf("first organization's filter after saving the second = %v", got)
	}

	// Clearing shows everything again.
	if got := decode[gen.ProjectFilter](t, te.supervisor.setProjectFilter(), http.StatusOK); len(got.ProjectIds) != 0 {
		t.Fatalf("cleared = %v", got.ProjectIds)
	}
	if got := boardKeys(te.supervisor.board(t, "")); !slices.Equal(got, all) {
		t.Errorf("board after clearing = %v", got)
	}
	if rec := te.requester.setProjectFilter(); rec.Code != http.StatusForbidden {
		t.Errorf("requester saving a filter: %d", rec.Code)
	}
}

// An agent unlinked from a client stops filtering on its projects: they drop out of the filter.
func TestProjectFilterForgetsProjectsOutOfSight(t *testing.T) {
	te := newTicketEnv(t)
	decode[gen.ProjectFilter](t, te.agent.setProjectFilter(te.projectA2), http.StatusOK)
	rec := te.admin.do(http.MethodPut, "/clients/"+te.clientA.String()+"/agents", gen.ClientAgentsInput{UserIds: []uuid.UUID{}})
	decode[gen.Client](t, rec, http.StatusOK)
	if got := te.agent.projectFilter(t); len(got) != 0 {
		t.Errorf("filter after unlinking = %v", got)
	}
}
