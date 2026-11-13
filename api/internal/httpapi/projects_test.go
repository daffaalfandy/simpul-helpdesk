package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/priorities"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/projects"
)

func newClient(t *testing.T, e *env, name string) uuid.UUID {
	t.Helper()
	id, err := e.st.Q().UpsertClient(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func projectInput(clientID uuid.UUID, name string) gen.ProjectCreate {
	return gen.ProjectCreate{ClientId: clientID, Name: name, Active: true}
}

func projectPath(p gen.Project) string { return "/projects/" + p.Id.String() }

func (s session) projects(t *testing.T, query string) []gen.Project {
	t.Helper()
	return decode[[]gen.Project](t, s.do(http.MethodGet, "/projects"+query, nil), http.StatusOK)
}

func TestProjects(t *testing.T) {
	e := newEnv(t)
	sup := e.as(auth.RoleSupervisor)
	client := newClient(t, e, "BPKAD Kota Batam")

	first := decode[gen.Project](t, sup.do(http.MethodPost, "/projects", projectInput(client, "  Network & infra support ")), http.StatusCreated)
	if first.Name != "Network & infra support" || !first.IsDefault || !first.Active || first.Client.Id != client {
		t.Fatalf("first project = %+v", first)
	}
	if len(first.Sla.Targets) != 4 || first.Sla.AtRiskPercent != gen.AtRiskPercentN75 || !first.Sla.PauseOnUat {
		t.Fatalf("first project SLA = %+v", first.Sla)
	}
	second := decode[gen.Project](t, sup.do(http.MethodPost, "/projects", projectInput(client, "SIPD connector")), http.StatusCreated)
	if second.IsDefault {
		t.Fatal("a second project became the default")
	}
	expectProblem(t, sup.do(http.MethodPost, "/projects", projectInput(client, "sipd CONNECTOR")), http.StatusConflict, apperr.CodeProjectNameTaken)
	expectProblem(t, sup.do(http.MethodPost, "/projects", projectInput(uuid.New(), "X")), http.StatusBadRequest, apperr.CodeClientNotFound)
	expectProblem(t, sup.do(http.MethodPost, "/projects", projectInput(client, "  ")), http.StatusBadRequest, apperr.CodeInvalidName)

	// The default moves; it can't be unset, hidden or deleted while the client has other projects.
	upd := gen.ProjectUpdate{Name: second.Name, Description: str("Budgeting and finance systems"), IsDefault: true, Active: true}
	got := decode[gen.Project](t, sup.do(http.MethodPut, projectPath(second), upd), http.StatusOK)
	if !got.IsDefault || got.Description == nil || *got.Description != "Budgeting and finance systems" {
		t.Fatalf("updated = %+v", got)
	}
	if list := sup.projects(t, "?clientId="+client.String()); len(list) != 2 || list[0].IsDefault {
		t.Fatalf("after switching the default: %+v", list)
	}
	expectProblem(t, sup.do(http.MethodPut, projectPath(second), gen.ProjectUpdate{Name: second.Name, Active: true}), http.StatusConflict, apperr.CodeProjectIsDefault)
	expectProblem(t, sup.do(http.MethodPut, projectPath(second), gen.ProjectUpdate{Name: second.Name, IsDefault: true}), http.StatusBadRequest, apperr.CodeProjectIsDefault)
	expectProblem(t, sup.do(http.MethodDelete, projectPath(second), nil), http.StatusConflict, apperr.CodeProjectIsDefault)

	// A non-default project can be hidden and deleted.
	hidden := decode[gen.Project](t, sup.do(http.MethodPut, projectPath(first), gen.ProjectUpdate{Name: first.Name}), http.StatusOK)
	if hidden.Active {
		t.Fatal("project still active")
	}
	if rec := sup.do(http.MethodDelete, projectPath(first), nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	// The last project, default or not, can go.
	if rec := sup.do(http.MethodDelete, projectPath(second), nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete last: %d %s", rec.Code, rec.Body)
	}
	if list := sup.projects(t, "?clientId="+client.String()); len(list) != 0 {
		t.Fatalf("projects left: %+v", list)
	}

	want := []string{
		projects.ActionCreated, projects.ActionCreated, projects.ActionUpdated, projects.ActionDefaultChanged,
		projects.ActionUpdated, projects.ActionDeleted, projects.ActionDeleted,
	}
	if got := e.auditActionsLike(t, "project.%"); !slices.Equal(got, want) {
		t.Fatalf("audit = %v, want %v", got, want)
	}
}

func TestProjectSLA(t *testing.T) {
	e := newEnv(t)
	sup := e.as(auth.RoleSupervisor)
	p := decode[gen.Project](t, sup.do(http.MethodPost, "/projects", projectInput(newClient(t, e, "Bappeda"), "e-Office")), http.StatusCreated)
	path := projectPath(p) + "/sla"

	next := p.Sla
	next.Targets[0].Resolve = gen.Duration{Value: 2, Unit: gen.DurationUnitH}
	next.AtRiskPercent = gen.AtRiskPercentN50
	next.NotifyPicOnP1Breach = true
	got := decode[gen.Project](t, sup.do(http.MethodPut, path, next), http.StatusOK)
	if got.Sla.Targets[0].Resolve != (gen.Duration{Value: 2, Unit: gen.DurationUnitH}) || got.Sla.AtRiskPercent != gen.AtRiskPercentN50 || !got.Sla.NotifyPicOnP1Breach {
		t.Fatalf("SLA = %+v", got.Sla)
	}

	bad := got.Sla
	bad.Targets[1].FirstResponse = gen.Duration{Value: 1, Unit: gen.DurationUnitWd} // P2 deliver is 1 working day
	expectProblem(t, sup.do(http.MethodPut, path, bad), http.StatusBadRequest, apperr.CodeInvalidSlaTargets)
	unit := got.Sla
	unit.Targets[2].Deliver.Unit = "weeks"
	if rec := sup.do(http.MethodPut, path, unit); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown unit: %d", rec.Code)
	}

	var data map[string]any
	if err := e.st.Pool().QueryRow(context.Background(),
		`SELECT data FROM audit_log WHERE action = $1 AND target_id = $2`, projects.ActionSLAUpdated, p.Id).Scan(&data); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"P1.resolve", "at_risk_percent", "notify_pic_on_p1_breach"} {
		if _, ok := data[key]; !ok {
			t.Errorf("audit data lacks %s: %v", key, data)
		}
	}
	if _, ok := data["P2.resolve"]; ok {
		t.Errorf("audit records an unchanged target: %v", data)
	}
}

// projectOps are the operations on one project, with what each role gets for a client it isn't
// linked to (agent) or a project it can see but not change.
var projectOps = map[string]struct{ requester, otherAgent, linkedAgent int }{
	"getProject":       {http.StatusForbidden, http.StatusNotFound, http.StatusOK},
	"updateProject":    {http.StatusForbidden, http.StatusForbidden, http.StatusForbidden},
	"deleteProject":    {http.StatusForbidden, http.StatusForbidden, http.StatusForbidden},
	"updateProjectSla": {http.StatusForbidden, http.StatusForbidden, http.StatusForbidden},
}

func TestProjectScope(t *testing.T) {
	e := newEnv(t)
	admin, sup := e.as(auth.RoleAdmin), e.as(auth.RoleSupervisor)
	linked, other := newClient(t, e, "Linked client"), newClient(t, e, "Other client")
	mine := decode[gen.Project](t, sup.do(http.MethodPost, "/projects", projectInput(linked, "Mine")), http.StatusCreated)
	theirs := decode[gen.Project](t, sup.do(http.MethodPost, "/projects", projectInput(other, "Theirs")), http.StatusCreated)
	decode[gen.Client](t, admin.do(http.MethodPut, "/clients/"+linked.String()+"/agents",
		gen.ClientAgentsInput{UserIds: []uuid.UUID{userID(t, e, auth.RoleAgent)}}), http.StatusOK)

	calls := map[string]func(s session, p gen.Project) *httptest.ResponseRecorder{
		"getProject": func(s session, p gen.Project) *httptest.ResponseRecorder {
			return s.do(http.MethodGet, projectPath(p), nil)
		},
		"updateProject": func(s session, p gen.Project) *httptest.ResponseRecorder {
			return s.do(http.MethodPut, projectPath(p), gen.ProjectUpdate{Name: p.Name, IsDefault: true, Active: true})
		},
		"deleteProject": func(s session, p gen.Project) *httptest.ResponseRecorder {
			return s.do(http.MethodDelete, projectPath(p), nil)
		},
		"updateProjectSla": func(s session, p gen.Project) *httptest.ResponseRecorder {
			return s.do(http.MethodPut, projectPath(p)+"/sla", p.Sla)
		},
	}

	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	for p, item := range spec.Paths.Map() {
		if !strings.Contains(p, "{projectId}") {
			continue
		}
		for _, op := range item.Operations() {
			id := strings.ToLower(op.OperationID[:1]) + op.OperationID[1:]
			if _, ok := projectOps[id]; !ok {
				t.Errorf("%s is not in the project scope matrix", id)
			}
		}
	}

	requester, agent := e.as(auth.RoleRequester), e.as(auth.RoleAgent)
	for op, want := range projectOps {
		if rec := calls[op](requester, mine); rec.Code != want.requester {
			t.Errorf("%s as requester: %d, want %d", op, rec.Code, want.requester)
		}
		if rec := calls[op](agent, theirs); rec.Code != want.otherAgent {
			t.Errorf("%s as agent of another client: %d, want %d", op, rec.Code, want.otherAgent)
		}
		if rec := calls[op](agent, mine); rec.Code != want.linkedAgent {
			t.Errorf("%s as linked agent: %d, want %d", op, rec.Code, want.linkedAgent)
		}
	}

	if list := agent.projects(t, ""); len(list) != 1 || list[0].Id != mine.Id {
		t.Fatalf("agent sees %+v", list)
	}
	expectProblem(t, agent.do(http.MethodGet, "/projects?clientId="+other.String(), nil), http.StatusNotFound, apperr.CodeNotFound)
	expectProblem(t, requester.do(http.MethodGet, "/projects", nil), http.StatusForbidden, apperr.CodeForbidden)
	if list := sup.projects(t, ""); len(list) != 2 {
		t.Fatalf("supervisor sees %d projects, want 2", len(list))
	}
}

func TestPriorityLevels(t *testing.T) {
	e := newEnv(t)
	levels := decode[[]gen.PriorityLevel](t, e.as(auth.RoleRequester).do(http.MethodGet, "/priorities", nil), http.StatusOK)
	if len(levels) != 4 || levels[0].Code != gen.PriorityP1 || levels[0].Name != "Critical" ||
		levels[3].Description != "Minor issue or question, no rush" {
		t.Fatalf("levels = %+v", levels)
	}

	admin := e.as(auth.RoleAdmin)
	levels[0].Name = "  Urgent "
	got := decode[[]gen.PriorityLevel](t, admin.do(http.MethodPut, "/priorities", levels), http.StatusOK)
	if got[0].Name != "Urgent" || got[1].Name != "High" {
		t.Fatalf("after rename = %+v", got)
	}
	if actions := e.auditActionsLike(t, "priority_level.%"); !slices.Equal(actions, []string{priorities.ActionUpdated}) {
		t.Fatalf("audit = %v", actions)
	}

	blank := slices.Clone(got)
	blank[2].Description = " "
	expectProblem(t, admin.do(http.MethodPut, "/priorities", blank), http.StatusBadRequest, apperr.CodeInvalidPriorityLevels)
	dup := slices.Clone(got)
	dup[3].Code = gen.PriorityP1
	expectProblem(t, admin.do(http.MethodPut, "/priorities", dup), http.StatusBadRequest, apperr.CodeInvalidPriorityLevels)
	for _, role := range []auth.Role{auth.RoleSupervisor, auth.RoleAgent, auth.RoleRequester} {
		expectProblem(t, e.as(role).do(http.MethodPut, "/priorities", got), http.StatusForbidden, apperr.CodeForbidden)
	}
}

// auditActionsLike lists audit actions matching a LIKE pattern, oldest first.
func (e *env) auditActionsLike(t *testing.T, pattern string) []string {
	t.Helper()
	rows, err := e.st.Pool().Query(context.Background(), `SELECT action FROM audit_log WHERE action LIKE $1 ORDER BY at, id`, pattern)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			t.Fatal(err)
		}
		out = append(out, a)
	}
	return out
}
