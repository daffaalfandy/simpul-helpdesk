package notification

import (
	"slices"
	"testing"
)

func TestSettingsListOrderAndDefaults(t *testing.T) {
	team := Settings(AudienceTeam, nil)
	want := []Setting{
		{EventComment, webAndEmail, false},
		{EventInternalNote, webOnly, false},
		{EventStatus, webAndEmail, false},
		{EventAssignment, webAndEmail, true},
		{EventPriority, webOnly, false},
		{EventSLA, webAndEmail, true},
	}
	if !slices.Equal(team, want) {
		t.Errorf("team settings\n got %v\nwant %v", team, want)
	}
	portal := Settings(AudiencePortal, nil)
	var events []Event
	for _, s := range portal {
		events = append(events, s.Event)
	}
	// ticket_logged is always on and not listed.
	if !slices.Equal(events, []Event{EventReply, EventStatus, EventQuestion, EventResolved, EventColleague}) {
		t.Errorf("portal events = %v", events)
	}
}

func TestChannels(t *testing.T) {
	off := map[Event]Pref{EventComment: {}, EventAssignment: {}, EventReply: {Web: true}}
	tests := []struct {
		audience Audience
		event    Event
		want     Pref
	}{
		{AudienceTeam, EventComment, Pref{}},             // switched off
		{AudienceTeam, EventAssignment, webAndEmail},     // locked wins over a stored choice
		{AudienceTeam, EventStatus, webAndEmail},         // default
		{AudienceTeam, EventReply, Pref{}},               // not a team event
		{AudiencePortal, EventReply, webOnly},            // stored choice
		{AudiencePortal, EventTicketLogged, webAndEmail}, // hidden and locked
		{AudiencePortal, EventInternalNote, Pref{}},      // never for client staff
		{AudiencePortal, EventColleague, webOnly},        // default
	}
	for _, tt := range tests {
		if got := Channels(tt.audience, tt.event, off); got != tt.want {
			t.Errorf("Channels(%s, %s) = %v, want %v", tt.audience, tt.event, got, tt.want)
		}
	}
}

func TestSwitchable(t *testing.T) {
	tests := []struct {
		audience       Audience
		event          Event
		pref           Pref
		known, allowed bool
	}{
		{AudienceTeam, EventComment, Pref{}, true, true},
		{AudienceTeam, EventSLA, Pref{Web: true}, true, false},
		{AudienceTeam, EventSLA, webAndEmail, true, true},
		{AudienceTeam, EventReply, webOnly, false, false},
		{AudiencePortal, EventTicketLogged, webAndEmail, false, false},
	}
	for _, tt := range tests {
		known, allowed := Switchable(tt.audience, tt.event, tt.pref)
		if known != tt.known || allowed != tt.allowed {
			t.Errorf("Switchable(%s, %s, %v) = %v, %v; want %v, %v",
				tt.audience, tt.event, tt.pref, known, allowed, tt.known, tt.allowed)
		}
	}
}

// Every event an audience lists is a known event, and every filter names known events.
func TestEventTablesAreConsistent(t *testing.T) {
	for aud, specs := range audienceEvents {
		for _, s := range specs {
			if !s.event.Valid() {
				t.Errorf("%s lists unknown event %q", aud, s.event)
			}
		}
	}
	for f, events := range filterEvents {
		if !f.Valid() {
			t.Errorf("unknown filter %q", f)
		}
		for _, e := range events {
			if !e.Valid() {
				t.Errorf("filter %s names unknown event %q", f, e)
			}
		}
	}
	for _, e := range Events {
		if !slices.Contains(precedence, e) {
			t.Errorf("event %s has no precedence", e)
		}
	}
}
