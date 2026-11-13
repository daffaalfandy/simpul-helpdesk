package ticket

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
)

func TestEffectiveSLA(t *testing.T) {
	snap := SLASnapshot{Version: SnapshotVersion, ProjectSLA: sla.DefaultProjectSLA()}
	snap.PauseOnUAT = false
	tests := []struct {
		name       string
		p          sla.Priority
		typ        Type
		first      sla.Duration
		resolution sla.Duration
		calendar   sla.CalendarKind
	}{
		{"incident uses resolve", sla.P1, TypeIncident, sla.Duration{Value: 15, Unit: sla.UnitMinutes}, sla.Duration{Value: 4, Unit: sla.UnitHours}, sla.CalendarAlways},
		{"request uses deliver", sla.P2, TypeRequest, sla.Duration{Value: 30, Unit: sla.UnitMinutes}, sla.Duration{Value: 1, Unit: sla.UnitWorkingDays}, sla.CalendarBusinessHours},
		{"incident on business hours", sla.P2, TypeIncident, sla.Duration{Value: 30, Unit: sla.UnitMinutes}, sla.Duration{Value: 8, Unit: sla.UnitHours}, sla.CalendarBusinessHours},
	}
	for _, tt := range tests {
		got, err := snap.Effective(tt.p, tt.typ)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		want := EffectiveSLA{
			Priority: tt.p, FirstResponse: tt.first, Resolution: tt.resolution, Calendar: tt.calendar,
			PauseOnWaitingClient: true, PauseOnUAT: false,
		}
		if got != want {
			t.Errorf("%s: got %+v, want %+v", tt.name, got, want)
		}
	}
	if _, err := snap.Effective("P9", TypeIncident); !errors.Is(err, ErrNoTarget) {
		t.Errorf("unknown priority: err = %v", err)
	}
}

// TestSnapshotJSON guards the stored form: tickets.sla_snapshot holds this JSON (and migration
// 00005 writes it), so renaming a tag on SLASnapshot or the sla types breaks stored tickets.
func TestSnapshotJSON(t *testing.T) {
	snap := SLASnapshot{
		Version: SnapshotVersion, ProjectID: uuid.MustParse("019a0000-0000-7000-8000-000000000001"),
		ProjectName: "e-Office",
		ProjectSLA: sla.ProjectSLA{
			Targets: []sla.ProjectTarget{{
				Priority:      sla.P3,
				FirstResponse: sla.Duration{Value: 2, Unit: sla.UnitHours},
				Resolve:       sla.Duration{Value: 2, Unit: sla.UnitWorkingDays},
				Deliver:       sla.Duration{Value: 3, Unit: sla.UnitDays},
				Calendar:      sla.CalendarBusinessHours,
			}},
			ProjectSettings: sla.ProjectSettings{
				PauseOnWaitingClient: true, PauseOnUAT: false, AtRiskPercent: sla.AtRiskThreeQuarters,
				NotifySupervisorOnBreach: true, NotifyPicOnP1Breach: false,
			},
		},
	}
	const golden = `{"version":2,"projectId":"019a0000-0000-7000-8000-000000000001","projectName":"e-Office",` +
		`"targets":[{"priority":"P3","firstResponse":{"value":2,"unit":"h"},"resolve":{"value":2,"unit":"wd"},` +
		`"deliver":{"value":3,"unit":"d"},"calendar":"business_hours"}],"pauseOnWaitingClient":true,"pauseOnUat":false,` +
		`"atRiskPercent":75,"notifySupervisorOnBreach":true,"notifyPicOnP1Breach":false}`
	b, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != golden {
		t.Fatalf("JSON changed:\n got %s\nwant %s", b, golden)
	}
	var back SLASnapshot
	if err := json.Unmarshal([]byte(golden), &back); err != nil {
		t.Fatal(err)
	}
	if back.ProjectName != snap.ProjectName || back.Targets[0] != snap.Targets[0] || back.ProjectSettings != snap.ProjectSettings {
		t.Fatalf("round trip = %+v", back)
	}
}
