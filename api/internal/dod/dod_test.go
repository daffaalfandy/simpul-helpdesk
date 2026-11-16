package dod

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/calendar"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
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

func merged(b Branch) func(*Config) {
	return func(c *Config) { c.Merged = Merged{Enabled: true, Branch: b} }
}
func ci(c *Config)       { c.CIPassed = true }
func deployed(c *Config) { c.DeployedProduction = true }
func uat(days int) func(*Config) {
	return func(c *Config) { c.UAT = UAT{Enabled: true, AutoApproveWorkingDays: days} }
}
func confirms(c *Config) { c.RequesterConfirms = true }

// TestStorageFormat pins the JSON stored in projects.dod_config and ticket_dod_overrides.
func TestStorageFormat(t *testing.T) {
	got, err := json.Marshal(with(merged(BranchRelease), ci, uat(AutoApproveFiveDays)))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"merged":{"enabled":true,"branch":"release/*"},"ci_passed":true,"deployed_production":false,` +
		`"uat":{"enabled":true,"auto_approve_working_days":5},"requester_confirms":false}`
	if string(got) != want {
		t.Fatalf("JSON = %s\nwant   %s", got, want)
	}
	if string(with(merged(BranchRelease), ci, uat(AutoApproveFiveDays)).JSON()) != want {
		t.Fatal("JSON() differs from json.Marshal")
	}
	back, err := Parse(got)
	if err != nil || back != with(merged(BranchRelease), ci, uat(AutoApproveFiveDays)) {
		t.Fatalf("round trip = %+v, %v", back, err)
	}
	if partial, err := Parse([]byte(`{"ci_passed":true}`)); err != nil || partial != with(ci) {
		t.Fatalf("missing fields = %+v, %v", partial, err)
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
	tests := []struct {
		name    string
		c       Config
		hasRepo bool
		want    error
	}{
		{"nothing", None(), false, nil},
		{"client check without a repository", with(uat(AutoApproveNever), confirms), false, nil},
		{"merge without a repository", with(merged(BranchMain)), false, ErrNeedsRepository},
		{"CI without a repository", with(ci), false, ErrNeedsRepository},
		{"deploy without a repository", with(deployed), false, ErrNeedsRepository},
		{"code with a repository", with(merged(BranchStaging), ci, deployed), true, nil},
		{"unknown branch", with(func(c *Config) { c.Merged.Branch = "trunk" }), true, ErrUnknownBranch},
		{"four days", with(uat(4)), false, ErrAutoApproveDays},
		{"days checked while UAT is off", with(func(c *Config) { c.UAT.AutoApproveWorkingDays = 4 }), false, ErrAutoApproveDays},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.c, tt.hasRepo); !errors.Is(err, tt.want) {
				t.Fatalf("Validate = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestChecklistOrder(t *testing.T) {
	tests := []struct {
		name      string
		c         Config
		checklist []Item
		required  []Item
	}{
		{"nothing", None(), nil, nil},
		{"e-Office", with(merged(BranchMain), uat(3)),
			[]Item{ItemFixCommitted, ItemMerged, ItemUATApproved}, []Item{ItemMerged, ItemUATApproved}},
		{"everything", with(confirms, uat(0), deployed, ci, merged(BranchMain)),
			Items, []Item{ItemMerged, ItemCIPassed, ItemDeployed, ItemUATApproved, ItemRequesterConfirmed}},
		{"deploy only", with(deployed), []Item{ItemFixCommitted, ItemDeployed}, []Item{ItemDeployed}},
		{"requester confirms", with(confirms), []Item{ItemRequesterConfirmed}, []Item{ItemRequesterConfirmed}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Checklist(tt.c); !slices.Equal(got, tt.checklist) {
				t.Errorf("Checklist = %v, want %v", got, tt.checklist)
			}
			if got := Required(tt.c); !slices.Equal(got, tt.required) {
				t.Errorf("Required = %v, want %v", got, tt.required)
			}
		})
	}
}

func TestEvaluate(t *testing.T) {
	eOffice := with(merged(BranchMain), uat(3))
	tests := []struct {
		name    string
		c       Config
		done    []Item
		met     bool
		pending []Item
	}{
		{"nothing required", None(), nil, true, nil},
		{"nothing done", eOffice, nil, false, []Item{ItemMerged, ItemUATApproved}},
		{"fix committed is not required", eOffice, []Item{ItemFixCommitted, ItemMerged}, false, []Item{ItemUATApproved}},
		{"all done", eOffice, []Item{ItemMerged, ItemUATApproved}, true, nil},
		{"items outside the DoD don't count", with(confirms), []Item{ItemUATApproved}, false, []Item{ItemRequesterConfirmed}},
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
		{"no client check", with(merged(BranchMain)), 0, false, false},
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
