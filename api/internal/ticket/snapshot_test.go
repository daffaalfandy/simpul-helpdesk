package ticket

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
)

var standardTargets = []sla.Target{
	{Priority: sla.P1, FirstResponseMinutes: 15, ResolveMinutes: 240, DeliverMinutes: 240, Calendar: sla.CalendarAlways},
	{Priority: sla.P2, FirstResponseMinutes: 30, ResolveMinutes: 480, DeliverMinutes: 540, Calendar: sla.CalendarBusinessHours},
	{Priority: sla.P3, FirstResponseMinutes: 120, ResolveMinutes: 1080, DeliverMinutes: 1080, DeliverFromCatalog: true, Calendar: sla.CalendarBusinessHours},
	{Priority: sla.P4, FirstResponseMinutes: 240, ResolveMinutes: 2700, DeliverMinutes: 2700, Calendar: sla.CalendarBusinessHours},
}

func TestEffectiveSLA(t *testing.T) {
	catalog := 2160
	withItem := SLASnapshot{Version: SnapshotVersion, Targets: standardTargets, CatalogDeliverMinutes: &catalog}
	noItem := SLASnapshot{Version: SnapshotVersion, Targets: standardTargets}
	tests := []struct {
		name        string
		snap        SLASnapshot
		p           sla.Priority
		typ         Type
		resolution  int
		fromCatalog bool
		calendar    sla.CalendarKind
	}{
		{"incident uses resolve", noItem, sla.P1, TypeIncident, 240, false, sla.CalendarAlways},
		{"request uses deliver", noItem, sla.P2, TypeRequest, 540, false, sla.CalendarBusinessHours},
		{"request uses catalogue when flagged", withItem, sla.P3, TypeRequest, 2160, true, sla.CalendarBusinessHours},
		{"flag without item falls back", noItem, sla.P3, TypeRequest, 1080, false, sla.CalendarBusinessHours},
		{"catalogue ignored without flag", withItem, sla.P4, TypeRequest, 2700, false, sla.CalendarBusinessHours},
		{"incident ignores catalogue", withItem, sla.P3, TypeIncident, 1080, false, sla.CalendarBusinessHours},
	}
	for _, tt := range tests {
		got, err := tt.snap.Effective(tt.p, tt.typ)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if got.ResolutionMinutes != tt.resolution || got.FromCatalog != tt.fromCatalog ||
			got.Calendar != tt.calendar || got.Priority != tt.p {
			t.Errorf("%s: got %+v", tt.name, got)
		}
	}
	if _, err := noItem.Effective("P9", TypeIncident); !errors.Is(err, ErrNoTarget) {
		t.Errorf("unknown priority: err = %v", err)
	}
}

// TestSnapshotJSON guards the stored form: tickets.sla_snapshot holds this JSON, so renaming
// a tag on SLASnapshot or sla.Target would break every stored ticket.
func TestSnapshotJSON(t *testing.T) {
	minutes := 540
	snap := SLASnapshot{
		Version: SnapshotVersion, PlanID: uuid.MustParse("019a0000-0000-7000-8000-000000000001"),
		PlanName: "Gold", CatalogDeliverMinutes: &minutes,
		Targets: []sla.Target{{
			Priority: sla.P3, FirstResponseMinutes: 120, ResolveMinutes: 1080, DeliverMinutes: 1080,
			DeliverFromCatalog: true, Calendar: sla.CalendarBusinessHours,
		}},
	}
	const golden = `{"version":1,"planId":"019a0000-0000-7000-8000-000000000001","planName":"Gold",` +
		`"targets":[{"priority":"P3","firstResponseMinutes":120,"resolveMinutes":1080,"deliverMinutes":1080,` +
		`"deliverFromCatalog":true,"calendar":"business_hours"}],"catalogDeliverMinutes":540}`
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
	if back.PlanName != snap.PlanName || *back.CatalogDeliverMinutes != minutes || back.Targets[0] != snap.Targets[0] {
		t.Fatalf("round trip = %+v", back)
	}
}
