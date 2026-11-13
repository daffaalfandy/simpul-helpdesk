package ticket

import (
	"testing"
	"time"
)

func TestProgress(t *testing.T) {
	t0 := time.Date(2026, 11, 12, 1, 14, 0, 0, time.UTC)
	at := func(min int) time.Time { return t0.Add(time.Duration(min) * time.Minute) }
	// st describes one expected step: its state and, if set, the minute it was entered.
	type st struct {
		state StepState
		min   int // -1: no time
	}
	none := -1
	tests := []struct {
		name    string
		current Status
		history []StatusChange
		want    []st
	}{
		{
			name: "just received", current: nw,
			history: []StatusChange{{nw, at(0)}},
			want:    []st{{StateCurrent, 0}, {StateUpcoming, none}, {StateUpcoming, none}, {StateUpcoming, none}, {StateUpcoming, none}},
		},
		{
			name: "assigned at creation", current: as,
			history: []StatusChange{{as, at(0)}},
			want:    []st{{StateDone, 0}, {StateCurrent, 0}, {StateUpcoming, none}, {StateUpcoming, none}, {StateUpcoming, none}},
		},
		{
			name: "in progress", current: ip,
			history: []StatusChange{{nw, at(0)}, {as, at(2)}, {ip, at(38)}},
			want:    []st{{StateDone, 0}, {StateDone, 2}, {StateCurrent, 38}, {StateUpcoming, none}, {StateUpcoming, none}},
		},
		{
			name: "started without an assignee", current: ip,
			history: []StatusChange{{nw, at(0)}, {ip, at(5)}},
			want:    []st{{StateDone, 0}, {StateSkipped, none}, {StateCurrent, 5}, {StateUpcoming, none}, {StateUpcoming, none}},
		},
		{
			name: "waiting on client shows as in progress", current: wc,
			history: []StatusChange{{nw, at(0)}, {as, at(2)}, {ip, at(10)}, {wc, at(20)}},
			want:    []st{{StateDone, 0}, {StateDone, 2}, {StateCurrent, 20}, {StateUpcoming, none}, {StateUpcoming, none}},
		},
		{
			name: "ready for test", current: ua,
			history: []StatusChange{{nw, at(0)}, {as, at(2)}, {ip, at(10)}, {ua, at(60)}},
			want:    []st{{StateDone, 0}, {StateDone, 2}, {StateDone, 10}, {StateCurrent, 60}, {StateUpcoming, none}},
		},
		{
			name: "resolved is done", current: rs,
			history: []StatusChange{{nw, at(0)}, {as, at(2)}, {ip, at(10)}, {rs, at(90)}},
			want:    []st{{StateDone, 0}, {StateDone, 2}, {StateDone, 10}, {StateSkipped, none}, {StateDone, 90}},
		},
		{
			name: "closed shows as resolved", current: cl,
			history: []StatusChange{{nw, at(0)}, {as, at(2)}, {ip, at(10)}, {ua, at(60)}, {rs, at(90)}, {cl, at(500)}},
			want:    []st{{StateDone, 0}, {StateDone, 2}, {StateDone, 10}, {StateDone, 60}, {StateDone, 500}},
		},
		{
			name: "reopened after resolve", current: ip,
			history: []StatusChange{{nw, at(0)}, {as, at(2)}, {ip, at(10)}, {ua, at(60)}, {rs, at(90)}, {ip, at(120)}},
			want:    []st{{StateDone, 0}, {StateDone, 2}, {StateCurrent, 120}, {StateUpcoming, none}, {StateUpcoming, none}},
		},
	}
	for _, tt := range tests {
		got := Progress(tt.current, t0, tt.history)
		if len(got) != len(Steps) {
			t.Fatalf("%s: %d steps", tt.name, len(got))
		}
		for i, w := range tt.want {
			g := got[i]
			if g.Step != Steps[i] || g.State != w.state {
				t.Errorf("%s: step %s = %s, want %s", tt.name, Steps[i], g.State, w.state)
			}
			switch {
			case w.min == none && g.At != nil:
				t.Errorf("%s: step %s at %v, want none", tt.name, Steps[i], *g.At)
			case w.min != none && (g.At == nil || !g.At.Equal(at(w.min))):
				t.Errorf("%s: step %s at %v, want minute %d", tt.name, Steps[i], g.At, w.min)
			}
		}
	}
}
