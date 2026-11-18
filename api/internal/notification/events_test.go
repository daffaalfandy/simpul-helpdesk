package notification

import (
	"slices"
	"testing"
)

func TestSettingsListOrderAndDefaults(t *testing.T) {
	team := Settings(AudienceTeam, false, nil)
	want := []Setting{
		{EventComment, webAndEmail, false},
		{EventInternalNote, webOnly, false},
		{EventStatus, webAndEmail, false},
		{EventAssignment, webAndEmail, true},
		{EventPriority, webOnly, false},
		{EventSLA, webAndEmail, true},
		{EventUATResult, webAndEmail, false},
		{EventCode, webOnly, false},
	}
	if !slices.Equal(team, want) {
		t.Errorf("agent settings\n got %v\nwant %v", team, want)
	}
	// Supervisors also switch new tickets, right after assignments.
	supervisor := Settings(AudienceTeam, true, map[Event]Pref{EventNewTicket: webOnly})
	wantSupervisor := slices.Insert(slices.Clone(want), 4, Setting{EventNewTicket, webOnly, false})
	if !slices.Equal(supervisor, wantSupervisor) {
		t.Errorf("supervisor settings\n got %v\nwant %v", supervisor, wantSupervisor)
	}
	portal := Settings(AudiencePortal, false, nil)
	var events []Event
	for _, s := range portal {
		events = append(events, s.Event)
		// Tests and questions for the client are always on: the ticket waits for their answer.
		if locked := s.Event == EventUATRequested || s.Event == EventQuestion; s.Locked != locked {
			t.Errorf("portal %s locked = %v", s.Event, s.Locked)
		}
	}
	// ticket_logged is always on and not listed.
	if !slices.Equal(events, []Event{EventReply, EventStatus, EventUATRequested, EventQuestion, EventResolved, EventColleague}) {
		t.Errorf("portal events = %v", events)
	}
}

func TestChannels(t *testing.T) {
	off := map[Event]Pref{EventComment: {}, EventAssignment: {}, EventReply: {Web: true}, EventQuestion: {}, EventNewTicket: webOnly}
	tests := []struct {
		audience Audience
		event    Event
		want     Pref
	}{
		{AudienceTeam, EventComment, Pref{}},             // switched off
		{AudienceTeam, EventAssignment, webAndEmail},     // locked wins over a stored choice
		{AudienceTeam, EventStatus, webAndEmail},         // default
		{AudienceTeam, EventReply, Pref{}},               // not a team event
		{AudienceTeam, EventNewTicket, webOnly},          // stored choice
		{AudiencePortal, EventQuestion, webAndEmail},     // locked wins over a stored choice
		{AudiencePortal, EventNewTicket, Pref{}},         // never for client staff
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
		supervisor     bool
		event          Event
		pref           Pref
		known, allowed bool
	}{
		{AudienceTeam, false, EventComment, Pref{}, true, true},
		{AudienceTeam, false, EventSLA, Pref{Web: true}, true, false},
		{AudienceTeam, false, EventSLA, webAndEmail, true, true},
		{AudienceTeam, false, EventReply, webOnly, false, false},
		{AudienceTeam, false, EventNewTicket, Pref{}, false, false}, // supervisors only
		{AudienceTeam, true, EventNewTicket, Pref{}, true, true},
		{AudienceTeam, true, EventDoD, Pref{}, false, false}, // hidden, even for supervisors
		{AudiencePortal, false, EventTicketLogged, webAndEmail, false, false},
		{AudiencePortal, false, EventQuestion, webOnly, true, false},
		{AudiencePortal, false, EventQuestion, webAndEmail, true, true},
	}
	for _, tt := range tests {
		known, allowed := Switchable(tt.audience, tt.supervisor, tt.event, tt.pref)
		if known != tt.known || allowed != tt.allowed {
			t.Errorf("Switchable(%s, supervisor %v, %s, %v) = %v, %v; want %v, %v",
				tt.audience, tt.supervisor, tt.event, tt.pref, known, allowed, tt.known, tt.allowed)
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
	// A missing event would rank -1 and outrank SLA alerts.
	for _, e := range Events {
		if Rank(e) < 0 {
			t.Errorf("event %s has no precedence", e)
		}
	}
	if len(precedence) != len(Events) {
		t.Errorf("precedence lists %d events, Events %d", len(precedence), len(Events))
	}
}
