package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/clients"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/testdb"
)

const requesterClient = "Dinas Kominfo Jambi"

type session struct {
	e      *env
	cookie *http.Cookie
}

func (e *env) as(role auth.Role) session { return session{e: e, cookie: e.sessionFor(role)} }

func (s session) do(method, path string, body any) *httptest.ResponseRecorder {
	s.e.t.Helper()
	return s.e.do(request{method: method, path: apiBase + path, body: body, cookie: s.cookie})
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder, status int) T {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d: %s", rec.Code, status, rec.Body)
	}
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func expectProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d: %s", rec.Code, status, rec.Body)
	}
	if got := problemCode(t, rec); got != code {
		t.Fatalf("code = %q, want %q", got, code)
	}
}

func date(t *testing.T, s string) *openapi_types.Date {
	t.Helper()
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		t.Fatal(err)
	}
	return &openapi_types.Date{Time: d}
}

func str(s string) *string { return &s }

func userID(t *testing.T, e *env, role auth.Role) uuid.UUID {
	t.Helper()
	u, err := e.st.Q().GetUserByEmail(context.Background(), e.users[role])
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func TestClientsCRUD(t *testing.T) {
	e := newEnv(t)
	admin := e.as(auth.RoleAdmin)

	in := gen.ClientInput{
		Name: "  BPKAD Kota Batam ", ContractNo: str("KTR/BPKAD-BTM/014/2026"),
		ContractStart: date(t, "2026-03-01"), ContractEnd: date(t, "2027-02-28"),
		PicName: str("Sri Lestari"), PicEmail: str("Sri@Batam.test"),
	}
	created := decode[gen.Client](t, admin.do(http.MethodPost, "/clients", in), http.StatusCreated)
	if created.Name != "BPKAD Kota Batam" || *created.PicEmail != "sri@batam.test" ||
		created.ContractEnd.String() != "2027-02-28" || len(created.Projects) != 0 || len(created.Agents) != 0 {
		t.Fatalf("created = %+v", created)
	}

	list := decode[[]gen.Client](t, admin.do(http.MethodGet, "/clients", nil), http.StatusOK)
	if len(list) != 2 || list[0].Name != "BPKAD Kota Batam" || list[1].Name != requesterClient || list[1].RequesterCount != 1 {
		t.Fatalf("list = %+v", list)
	}

	in.Name, in.ContractNo, in.PicEmail = "BPKAD Batam", nil, nil
	updated := decode[gen.Client](t, admin.do(http.MethodPut, "/clients/"+created.Id.String(), in), http.StatusOK)
	if updated.Name != "BPKAD Batam" || updated.ContractNo != nil || updated.PicEmail != nil {
		t.Fatalf("updated = %+v", updated)
	}

	// The client lists its projects; deleting a client without tickets takes them along.
	testdb.Project(t, e.st, created.Id, "Network & infra support", true)
	testdb.Project(t, e.st, created.Id, "Finance apps", false)
	withProjects := decode[gen.Client](t, admin.do(http.MethodGet, "/clients/"+created.Id.String(), nil), http.StatusOK)
	if len(withProjects.Projects) != 2 || withProjects.Projects[0].Name != "Finance apps" {
		t.Fatalf("projects = %+v", withProjects.Projects)
	}
	if rec := admin.do(http.MethodDelete, "/clients/"+created.Id.String(), nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	expectProblem(t, admin.do(http.MethodGet, "/clients/"+created.Id.String(), nil), http.StatusNotFound, apperr.CodeNotFound)
}

func TestClientValidation(t *testing.T) {
	e := newEnv(t)
	admin := e.as(auth.RoleAdmin)
	tests := map[string]struct {
		in     gen.ClientInput
		status int
		code   string
	}{
		"name taken":       {gen.ClientInput{Name: requesterClient}, http.StatusConflict, apperr.CodeClientNameTaken},
		"blank name":       {gen.ClientInput{Name: "   "}, http.StatusBadRequest, apperr.CodeInvalidName},
		"period backwards": {gen.ClientInput{Name: "C", ContractStart: date(t, "2026-12-31"), ContractEnd: date(t, "2026-01-01")}, http.StatusBadRequest, apperr.CodeInvalidContractPeriod},
		"bad PIC email":    {gen.ClientInput{Name: "C", PicEmail: str("@nowhere")}, http.StatusBadRequest, apperr.CodeInvalidEmail},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			expectProblem(t, admin.do(http.MethodPost, "/clients", tt.in), tt.status, tt.code)
		})
	}
	jambi := decode[[]gen.Client](t, admin.do(http.MethodGet, "/clients", nil), http.StatusOK)[0]
	expectProblem(t, admin.do(http.MethodDelete, "/clients/"+jambi.Id.String(), nil), http.StatusConflict, apperr.CodeClientInUse)
}

func TestClientWritesAreAdminOnly(t *testing.T) {
	e := newEnv(t)
	for _, role := range []auth.Role{auth.RoleSupervisor, auth.RoleAgent, auth.RoleRequester} {
		s := e.as(role)
		expectProblem(t, s.do(http.MethodPost, "/clients", gen.ClientInput{Name: "X"}), http.StatusForbidden, apperr.CodeForbidden)
		expectProblem(t, s.do(http.MethodPost, "/holidays", gen.HolidayInput{Date: *date(t, "2026-01-01"), Name: "X"}), http.StatusForbidden, apperr.CodeForbidden)
		expectProblem(t, s.do(http.MethodPost, "/holiday-presets/2026/load", nil), http.StatusForbidden, apperr.CodeForbidden)
	}
}

func TestClientVisibilityAndAgents(t *testing.T) {
	e := newEnv(t)
	admin := e.as(auth.RoleAdmin)
	a := decode[gen.Client](t, admin.do(http.MethodPost, "/clients", gen.ClientInput{Name: "A"}), http.StatusCreated)
	b := decode[gen.Client](t, admin.do(http.MethodPost, "/clients", gen.ClientInput{Name: "B"}), http.StatusCreated)
	agentID := userID(t, e, auth.RoleAgent)

	agentsPath := "/clients/" + a.Id.String() + "/agents"
	got := decode[gen.Client](t, admin.do(http.MethodPut, agentsPath, gen.ClientAgentsInput{UserIds: []uuid.UUID{agentID, agentID}}), http.StatusOK)
	if len(got.Agents) != 1 || got.Agents[0].Id != agentID {
		t.Fatalf("agents = %+v", got.Agents)
	}
	expectProblem(t, admin.do(http.MethodPut, agentsPath, gen.ClientAgentsInput{UserIds: []uuid.UUID{userID(t, e, auth.RoleSupervisor)}}), http.StatusBadRequest, apperr.CodeInvalidAgent)
	expectProblem(t, admin.do(http.MethodPut, agentsPath, gen.ClientAgentsInput{UserIds: []uuid.UUID{uuid.New()}}), http.StatusBadRequest, apperr.CodeInvalidAgent)

	agent := e.as(auth.RoleAgent)
	visible := decode[[]gen.Client](t, agent.do(http.MethodGet, "/clients", nil), http.StatusOK)
	if len(visible) != 1 || visible[0].Id != a.Id {
		t.Fatalf("agent sees %+v", visible)
	}
	decode[gen.Client](t, agent.do(http.MethodGet, "/clients/"+a.Id.String(), nil), http.StatusOK)
	expectProblem(t, agent.do(http.MethodGet, "/clients/"+b.Id.String(), nil), http.StatusNotFound, apperr.CodeNotFound)

	if all := decode[[]gen.Client](t, e.as(auth.RoleSupervisor).do(http.MethodGet, "/clients", nil), http.StatusOK); len(all) != 3 {
		t.Fatalf("supervisor sees %d clients, want 3", len(all))
	}
	expectProblem(t, e.as(auth.RoleRequester).do(http.MethodGet, "/clients", nil), http.StatusForbidden, apperr.CodeForbidden)

	cleared := decode[gen.Client](t, admin.do(http.MethodPut, agentsPath, gen.ClientAgentsInput{UserIds: []uuid.UUID{}}), http.StatusOK)
	if len(cleared.Agents) != 0 {
		t.Fatalf("agents after clearing = %+v", cleared.Agents)
	}
}

func TestBusinessHours(t *testing.T) {
	e := newEnv(t)
	h := decode[gen.BusinessHours](t, e.as(auth.RoleRequester).do(http.MethodGet, "/business-hours", nil), http.StatusOK)
	want := []gen.Weekday{gen.WeekdayMon, gen.WeekdayTue, gen.WeekdayWed, gen.WeekdayThu, gen.WeekdayFri}
	if h.Timezone != "Asia/Jakarta" || !slices.Equal(h.Weekdays, want) || h.Opens != "08:00" || h.Closes != "17:00" || h.WorkingDayMinutes != 540 {
		t.Fatalf("hours = %+v", h)
	}

	in := gen.BusinessHoursInput{Weekdays: []gen.Weekday{gen.WeekdayFri, gen.WeekdayMon}, Opens: "07:30", Closes: "16:00"}
	expectProblem(t, e.as(auth.RoleAgent).do(http.MethodPut, "/business-hours", in), http.StatusForbidden, apperr.CodeForbidden)
	admin := e.as(auth.RoleAdmin)
	got := decode[gen.BusinessHours](t, admin.do(http.MethodPut, "/business-hours", in), http.StatusOK)
	if !slices.Equal(got.Weekdays, []gen.Weekday{gen.WeekdayMon, gen.WeekdayFri}) || got.WorkingDayMinutes != 510 {
		t.Fatalf("updated = %+v", got)
	}
	expectProblem(t, admin.do(http.MethodPut, "/business-hours", gen.BusinessHoursInput{Weekdays: want, Opens: "17:00", Closes: "08:00"}), http.StatusBadRequest, apperr.CodeInvalidBusinessHours)
	expectProblem(t, admin.do(http.MethodPut, "/business-hours", gen.BusinessHoursInput{Weekdays: want, Opens: "8:00", Closes: "17:00"}), http.StatusBadRequest, apperr.CodeInvalidRequest)
	expectProblem(t, admin.do(http.MethodPut, "/business-hours", gen.BusinessHoursInput{Weekdays: []gen.Weekday{}, Opens: "08:00", Closes: "17:00"}), http.StatusBadRequest, apperr.CodeInvalidRequest)
}

func TestHolidays(t *testing.T) {
	e := newEnv(t)
	admin := e.as(auth.RoleAdmin)
	h := decode[gen.Holiday](t, admin.do(http.MethodPost, "/holidays", gen.HolidayInput{Date: *date(t, "2026-06-01"), Name: "Hari Lahir Pancasila"}), http.StatusCreated)
	expectProblem(t, admin.do(http.MethodPost, "/holidays", gen.HolidayInput{Date: *date(t, "2026-06-01"), Name: "Again"}), http.StatusConflict, apperr.CodeHolidayDateTaken)

	agent := e.as(auth.RoleAgent)
	if got := decode[[]gen.Holiday](t, agent.do(http.MethodGet, "/holidays?year=2026", nil), http.StatusOK); len(got) != 1 || got[0].Date.String() != "2026-06-01" {
		t.Fatalf("2026 = %+v", got)
	}
	if got := decode[[]gen.Holiday](t, agent.do(http.MethodGet, "/holidays?year=2027", nil), http.StatusOK); len(got) != 0 {
		t.Fatalf("2027 = %+v", got)
	}
	if rec := admin.do(http.MethodDelete, "/holidays/"+h.Id.String(), nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	expectProblem(t, admin.do(http.MethodDelete, "/holidays/"+h.Id.String(), nil), http.StatusNotFound, apperr.CodeNotFound)
}

func TestHolidayPresets(t *testing.T) {
	e := newEnv(t)
	admin := e.as(auth.RoleAdmin)
	presets := decode[[]gen.HolidayPreset](t, admin.do(http.MethodGet, "/holiday-presets", nil), http.StatusOK)
	if len(presets) < 2 || presets[0].Year != 2026 || presets[0].MissingCount != presets[0].HolidayCount {
		t.Fatalf("presets = %+v", presets)
	}
	first := decode[gen.HolidayPresetLoadResult](t, admin.do(http.MethodPost, "/holiday-presets/2026/load", nil), http.StatusOK)
	if first.AddedCount != presets[0].HolidayCount || first.SkippedCount != 0 {
		t.Fatalf("first load = %+v", first)
	}
	again := decode[gen.HolidayPresetLoadResult](t, admin.do(http.MethodPost, "/holiday-presets/2026/load", nil), http.StatusOK)
	if again.AddedCount != 0 || again.SkippedCount != presets[0].HolidayCount {
		t.Fatalf("second load = %+v", again)
	}
	expectProblem(t, admin.do(http.MethodPost, "/holiday-presets/1999/load", nil), http.StatusNotFound, apperr.CodeNotFound)
	expectProblem(t, e.as(auth.RoleAgent).do(http.MethodGet, "/holiday-presets", nil), http.StatusForbidden, apperr.CodeForbidden)
}

func TestSettingsWritesAreAudited(t *testing.T) {
	e := newEnv(t)
	admin := e.as(auth.RoleAdmin)
	c := decode[gen.Client](t, admin.do(http.MethodPost, "/clients", gen.ClientInput{Name: "C"}), http.StatusCreated)
	decode[gen.Client](t, admin.do(http.MethodPut, "/clients/"+c.Id.String(), gen.ClientInput{Name: "C2"}), http.StatusOK)
	decode[gen.Client](t, admin.do(http.MethodPut, "/clients/"+c.Id.String()+"/agents", gen.ClientAgentsInput{UserIds: []uuid.UUID{userID(t, e, auth.RoleAgent)}}), http.StatusOK)
	admin.do(http.MethodDelete, "/clients/"+c.Id.String(), nil)
	decode[gen.BusinessHours](t, admin.do(http.MethodPut, "/business-hours", gen.BusinessHoursInput{Weekdays: []gen.Weekday{gen.WeekdayMon}, Opens: "09:00", Closes: "17:00"}), http.StatusOK)
	h := decode[gen.Holiday](t, admin.do(http.MethodPost, "/holidays", gen.HolidayInput{Date: *date(t, "2026-01-02"), Name: "X"}), http.StatusCreated)
	admin.do(http.MethodDelete, "/holidays/"+h.Id.String(), nil)
	decode[gen.HolidayPresetLoadResult](t, admin.do(http.MethodPost, "/holiday-presets/2027/load", nil), http.StatusOK)

	rows, err := e.st.Pool().Query(context.Background(),
		`SELECT action, actor_kind FROM audit_log WHERE action NOT LIKE 'auth.%' AND action <> 'user.created' ORDER BY at, id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var action, kind string
		if err := rows.Scan(&action, &kind); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprintf("%s/%s", action, kind))
	}
	user := func(action string) string { return action + "/user" }
	want := []string{
		user(clients.ActionCreated), user(clients.ActionUpdated), user(clients.ActionAgentsUpdated), user(clients.ActionDeleted),
		user(sla.ActionHoursUpdated), user(sla.ActionHolidayCreated), user(sla.ActionHolidayDeleted), user(sla.ActionPresetLoaded),
	}
	if !slices.Equal(got, want) {
		t.Fatalf("audit actions =\n%v\nwant\n%v", got, want)
	}
}
