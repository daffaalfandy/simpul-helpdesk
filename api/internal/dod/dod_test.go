package dod

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

var jakarta = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		panic(err)
	}
	return loc
}()

// at is a wall-clock time in Jakarta in November 2026 (13 = Friday, 16 = Monday).
func at(day, hour int) time.Time { return time.Date(2026, time.November, day, hour, 0, 0, 0, jakarta) }

func schedules(t *testing.T, holidays ...calendar.Date) sla.Schedules {
	t.Helper()
	weekdays := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}
	window := calendar.Window{Open: 8 * time.Hour, Close: 17 * time.Hour}
	b, err := calendar.NewBusinessHours(calendar.Config{
		Location: jakarta, Week: calendar.UniformWeek(weekdays, window), Holidays: holidays,
	})
	if err != nil {
		t.Fatal(err)
	}
	return sla.Schedules{Business: b, Always: calendar.Always{}, WorkingDay: window.Length()}
}

// with builds a configuration from None by applying the changes.
func with(changes ...func(*Config)) Config {
	c := None()
	for _, f := range changes {
		f(&c)
	}
	return c
}

func uat(days int) func(*Config) {
	return func(c *Config) { c.UAT = UAT{Enabled: true, AutoApproveWorkingDays: days} }
}
func confirms(c *Config) { c.RequesterConfirms = true }

func onBranch(name string) func(*Config) {
	return func(c *Config) { c.CodeOnBranch = CodeOnBranch{Enabled: true, Branch: name} }
}

// TestStorageFormat pins the JSON stored in projects.dod_config and ticket_dod_overrides.
func TestStorageFormat(t *testing.T) {
	full := with(onBranch("main"), uat(AutoApproveFiveDays), confirms)
	got, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"code_on_branch":{"enabled":true,"branch":"main"},"uat":{"enabled":true,"auto_approve_working_days":5},` +
		`"requester_confirms":true}`
	if string(got) != want {
		t.Fatalf("JSON = %s\nwant   %s", got, want)
	}
	if string(full.JSON()) != want {
		t.Fatal("JSON() differs from json.Marshal")
	}
	back, err := Parse(got)
	if err != nil || back != full {
		t.Fatalf("round trip = %+v, %v", back, err)
	}
	if partial, err := Parse([]byte(`{"requester_confirms":true}`)); err != nil || partial != with(confirms) {
		t.Fatalf("missing fields = %+v, %v", partial, err)
	}
	// JSON stored before 10b has no code_on_branch: the condition is off.
	stored := `{"uat":{"enabled":true,"auto_approve_working_days":3},"requester_confirms":false}`
	if c, err := Parse([]byte(stored)); err != nil || c != with(uat(AutoApproveThreeDays)) || c.CodeOnBranch.Enabled {
		t.Fatalf("10a JSON = %+v, %v", c, err)
	}
	// JSON stored before the code conditions went still reads; the old code keys are dropped and
	// don't turn the branch condition on.
	old := `{"merged":{"enabled":true,"branch":"release/*"},"ci_passed":true,"deployed_production":false,` +
		`"uat":{"enabled":true,"auto_approve_working_days":3},"requester_confirms":false}`
	if c, err := Parse([]byte(old)); err != nil || c != with(uat(AutoApproveThreeDays)) {
		t.Fatalf("old JSON = %+v, %v", c, err)
	}
}

func TestDefaults(t *testing.T) {
	if Default() != with(uat(DefaultAutoApproveDays)) {
		t.Fatalf("Default = %+v", Default())
	}
	if len(Required(None())) != 0 || HasClientCheck(None()) {
		t.Fatalf("None requires %v", Required(None()))
	}
}

func TestValidate(t *testing.T) {
	keptBranch := func(name string) func(*Config) {
		return func(c *Config) { c.CodeOnBranch = CodeOnBranch{Branch: name} }
	}
	tests := []struct {
		name    string
		c       Config
		hasRepo bool
		want    error
	}{
		{"nothing", None(), false, nil},
		{"both client checks", with(uat(AutoApproveNever), confirms), false, nil},
		{"four days", with(uat(4)), false, ErrAutoApproveDays},
		{"days checked while UAT is off", with(func(c *Config) { c.UAT.AutoApproveWorkingDays = 4 }), false, ErrAutoApproveDays},
		{"code on main", with(onBranch("main"), uat(3)), true, nil},
		{"code without a repository", with(onBranch("main")), false, ErrNeedsRepository},
		{"no branch", with(onBranch("")), true, ErrInvalidBranch},
		{"bad branch", with(onBranch("fix me")), true, ErrInvalidBranch},
		{"bad branch without a repository", with(onBranch("a..b")), false, ErrInvalidBranch},
		{"branch kept while off, no repository", with(keptBranch("main")), false, nil},
		{"bad branch kept while off", with(keptBranch("main~1")), true, ErrInvalidBranch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.c, tt.hasRepo); !errors.Is(err, tt.want) {
				t.Fatalf("Validate = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestNormalizeBranch(t *testing.T) {
	tests := []struct{ in, want string }{
		{"main", "main"},
		{"  release/2026.11 ", "release/2026.11"},
		{"refs/heads/main", "main"},
		{" refs/heads/feature/x ", "feature/x"},
		{"refs/tags/v1", "refs/tags/v1"},
		{"Main", "Main"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := NormalizeBranch(tt.in); got != tt.want {
			t.Errorf("NormalizeBranch(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	if got := Normalize(with(onBranch(" refs/heads/main "))); got != with(onBranch("main")) {
		t.Errorf("Normalize = %+v", got)
	}
}

func TestValidBranch(t *testing.T) {
	tests := []struct {
		name string
		ok   bool
	}{
		{"main", true},
		{"release/2026.11", true},
		{"feature/HD-24871-sso", true},
		{"user@host", true},
		{"rilis-ke-2", true},
		{strings.Repeat("b", MaxBranchLen), true},
		{"", false},
		{strings.Repeat("b", MaxBranchLen+1), false},
		{"fix me", false},
		{"tab\tname", false},
		{"bell\aname", false},
		{"a~1", false},
		{"a^", false},
		{"a:b", false},
		{"what?", false},
		{"star*", false},
		{"[x", false},
		{"back\\slash", false},
		{"a..b", false},
		{"a//b", false},
		{"a@{1}", false},
		{"@", false},
		{"-main", false},
		{"/main", false},
		{"main/", false},
		{"main.", false},
		{"main.lock", false},
		{".hidden", false},
		{"release/.x", false},
	}
	for _, tt := range tests {
		if got := ValidBranch(tt.name); got != tt.ok {
			t.Errorf("ValidBranch(%q) = %v, want %v", tt.name, got, tt.ok)
		}
	}
}

func TestForRepository(t *testing.T) {
	c := with(onBranch("main"), uat(3))
	if got := c.ForRepository(true); got != c {
		t.Errorf("with a repository = %+v", got)
	}
	got := c.ForRepository(false)
	if got.CodeOnBranch.Enabled || got.CodeOnBranch.Branch != "main" || !got.UAT.Enabled {
		t.Errorf("without a repository = %+v", got)
	}
	if !slices.Equal(Required(got), []Item{ItemUATApproved}) {
		t.Errorf("required without a repository = %v", Required(got))
	}
}

func TestBranchReached(t *testing.T) {
	tests := []struct {
		name   string
		c      Config
		branch string
		want   bool
	}{
		{"same branch", with(onBranch("main")), "main", true},
		{"other branch", with(onBranch("main")), "fix/HD-1", false},
		{"letter case differs", with(onBranch("main")), "Main", false},
		{"condition off", with(func(c *Config) { c.CodeOnBranch.Branch = "main" }), "main", false},
		{"no branch pushed", with(onBranch("main")), "", false},
		{"folders match exactly", with(onBranch("release/2026.11")), "release/2026.11", true},
	}
	for _, tt := range tests {
		if got := BranchReached(tt.c, tt.branch); got != tt.want {
			t.Errorf("%s: BranchReached = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestBranchChanged(t *testing.T) {
	off := with(func(c *Config) { c.CodeOnBranch.Branch = "main" })
	tests := []struct {
		name       string
		prev, next Config
		want       bool
	}{
		{"turned on", off, with(onBranch("main")), true},
		{"turned on from nothing", None(), with(onBranch("main")), true},
		{"other branch", with(onBranch("main")), with(onBranch("release")), true},
		{"unchanged", with(onBranch("main")), with(onBranch("main"), uat(3)), false},
		{"turned off", with(onBranch("main")), off, false},
		{"branch changed while off", off, with(func(c *Config) { c.CodeOnBranch.Branch = "release" }), false},
	}
	for _, tt := range tests {
		if got := BranchChanged(tt.prev, tt.next); got != tt.want {
			t.Errorf("%s: BranchChanged = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestResolveNeedsReason(t *testing.T) {
	tests := []struct {
		name      string
		mandatory bool
		pending   []Item
		want      bool
	}{
		{"mandatory, unmet", true, []Item{ItemUATApproved}, true},
		{"mandatory, met", true, nil, false},
		{"guidance only, unmet", false, []Item{ItemCodeOnBranch, ItemUATApproved}, false},
		{"guidance only, met", false, []Item{}, false},
	}
	for _, tt := range tests {
		if got := ResolveNeedsReason(tt.mandatory, tt.pending); got != tt.want {
			t.Errorf("%s: ResolveNeedsReason = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestRequiredOrder(t *testing.T) {
	tests := []struct {
		name     string
		c        Config
		required []Item
	}{
		{"nothing", None(), nil},
		{"UAT", with(uat(3)), []Item{ItemUATApproved}},
		{"both", with(confirms, uat(0)), ClientItems},
		{"requester confirms", with(confirms), []Item{ItemRequesterConfirmed}},
		{"code first", with(confirms, onBranch("main"), uat(0)), Items},
		{"code only", with(onBranch("main")), []Item{ItemCodeOnBranch}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Required(tt.c); !slices.Equal(got, tt.required) {
				t.Errorf("Required = %v, want %v", got, tt.required)
			}
		})
	}
}

func TestEvaluate(t *testing.T) {
	eOffice := with(uat(3))
	tests := []struct {
		name    string
		c       Config
		done    []Item
		met     bool
		pending []Item
	}{
		{"nothing required", None(), nil, true, nil},
		{"nothing done", eOffice, nil, false, []Item{ItemUATApproved}},
		{"all done", eOffice, []Item{ItemUATApproved}, true, nil},
		{"items outside the DoD don't count", with(confirms), []Item{ItemUATApproved}, false, []Item{ItemRequesterConfirmed}},
		{"code pending", with(onBranch("main"), uat(3)), []Item{ItemUATApproved}, false, []Item{ItemCodeOnBranch}},
		{"code and test done", with(onBranch("main"), uat(3)), []Item{ItemCodeOnBranch, ItemUATApproved}, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done := map[Item]bool{}
			for _, it := range tt.done {
				done[it] = true
			}
			met, pending := Evaluate(tt.c, done)
			if met != tt.met || !slices.Equal(pending, tt.pending) {
				t.Fatalf("Evaluate = %v %v, want %v %v", met, pending, tt.met, tt.pending)
			}
		})
	}
}

func TestAutoApproveDays(t *testing.T) {
	tests := []struct {
		name   string
		c      Config
		days   int
		ok     bool
		client bool
	}{
		{"no client check", None(), 0, false, false},
		{"UAT never", with(uat(AutoApproveNever)), 0, false, true},
		{"UAT three days", with(uat(AutoApproveThreeDays)), 3, true, true},
		{"UAT five days", with(uat(AutoApproveFiveDays)), 5, true, true},
		{"requester confirms", with(confirms), RequesterConfirmsAutoDays, true, true},
		{"UAT decides over the confirmation", with(uat(AutoApproveNever), confirms), 0, false, true},
		{"UAT five days with the confirmation", with(uat(AutoApproveFiveDays), confirms), 5, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			days, ok := AutoApproveDays(tt.c)
			if days != tt.days || ok != tt.ok {
				t.Errorf("AutoApproveDays = %d %v, want %d %v", days, ok, tt.days, tt.ok)
			}
			if got := HasClientCheck(tt.c); got != tt.client {
				t.Errorf("HasClientCheck = %v, want %v", got, tt.client)
			}
		})
	}
}

func TestEffective(t *testing.T) {
	project, hotfix := with(uat(3)), with(confirms)
	if c, src := Effective(project, nil); c != project || src != SourceProject {
		t.Errorf("no override = %+v %s", c, src)
	}
	if c, src := Effective(project, &hotfix); c != hotfix || src != SourceTicket {
		t.Errorf("override = %+v %s", c, src)
	}
}

func TestAutoApproveAt(t *testing.T) {
	holiday := calendar.NewDate(2026, time.November, 18)
	tests := []struct {
		name     string
		c        Config
		entered  time.Time
		holidays []calendar.Date
		want     time.Time
		ok       bool
	}{
		{"three days over a weekend", with(uat(3)), at(13, 16), nil, at(18, 16), true},
		{"three days over a holiday", with(uat(3)), at(16, 15), []calendar.Date{holiday}, at(20, 15), true},
		{"five days", with(uat(5)), at(16, 15), nil, at(23, 15), true},
		{"never", with(uat(AutoApproveNever)), at(16, 15), nil, time.Time{}, false},
		{"no client check", None(), at(16, 15), nil, time.Time{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := AutoApproveAt(tt.c, tt.entered, schedules(t, tt.holidays...))
			if ok != tt.ok || !got.Equal(tt.want) {
				t.Fatalf("AutoApproveAt = %v %v, want %v %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestUATDay(t *testing.T) {
	s := schedules(t)
	tests := []struct {
		name string
		now  time.Time
		days int
		want int
	}{
		{"first day", at(16, 16), 3, 1},
		{"second day", at(17, 11), 3, 2},
		{"capped at the limit", at(25, 9), 3, 3},
		{"no limit", at(25, 9), 0, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UATDay(at(16, 10), tt.now, s, tt.days); got != tt.want {
				t.Fatalf("UATDay = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestClientRequired(t *testing.T) {
	tests := []struct {
		c    Config
		want []Item
	}{
		{None(), nil},
		{with(uat(3)), []Item{ItemUATApproved}},
		{with(uat(0), confirms), []Item{ItemUATApproved, ItemRequesterConfirmed}},
		{with(onBranch("main"), uat(3)), []Item{ItemUATApproved}},
		{with(onBranch("main")), nil},
	}
	for _, tt := range tests {
		if got := ClientRequired(tt.c); !slices.Equal(got, tt.want) {
			t.Errorf("ClientRequired(%+v) = %v, want %v", tt.c, got, tt.want)
		}
	}
}

func TestParseEffective(t *testing.T) {
	project, own := with(uat(3)).JSON(), with(confirms).JSON()
	if c, src, err := ParseEffective(project, nil); err != nil || src != SourceProject || c != with(uat(3)) {
		t.Errorf("project = %+v %s %v", c, src, err)
	}
	if c, src, err := ParseEffective(project, own); err != nil || src != SourceTicket || c != with(confirms) {
		t.Errorf("own = %+v %s %v", c, src, err)
	}
	if _, _, err := ParseEffective([]byte(`{"uat": "yes"}`), nil); err == nil {
		t.Error("a broken config parsed")
	}
}

func TestCleared(t *testing.T) {
	tests := []struct {
		from, to ticket.Status
		want     []Item
	}{
		{ticket.StatusResolved, ticket.StatusInProgress, []Item{ItemUATApproved, ItemRequesterConfirmed, ItemCodeOnBranch}},
		{ticket.StatusResolved, ticket.StatusNew, []Item{ItemUATApproved, ItemRequesterConfirmed, ItemCodeOnBranch}},
		{ticket.StatusClosed, ticket.StatusInProgress, []Item{ItemUATApproved, ItemRequesterConfirmed, ItemCodeOnBranch}},
		{ticket.StatusResolved, ticket.StatusClosed, nil},
		{ticket.StatusInUAT, ticket.StatusInProgress, CodeItems},
		{ticket.StatusInUAT, ticket.StatusWaitingOnClient, CodeItems},
		{ticket.StatusInUAT, ticket.StatusResolved, nil},
		{ticket.StatusNew, ticket.StatusAssigned, nil},
		{ticket.StatusInProgress, ticket.StatusWaitingOnClient, nil},
		{ticket.StatusWaitingOnClient, ticket.StatusInUAT, nil},
	}
	for _, tt := range tests {
		if got := Cleared(tt.from, tt.to); !slices.Equal(got, tt.want) {
			t.Errorf("Cleared(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}
