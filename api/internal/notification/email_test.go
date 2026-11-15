package notification

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestUsesTicketEmail(t *testing.T) {
	tests := []struct {
		f      Frequency
		urgent bool
		want   bool
	}{
		{FrequencyInstant, false, true},
		{FrequencyHourly, false, false},
		{FrequencyDaily, false, false},
		{FrequencyHourly, true, true},
		{FrequencyDaily, true, true},
	}
	for _, tt := range tests {
		if got := UsesTicketEmail(tt.f, tt.urgent); got != tt.want {
			t.Errorf("UsesTicketEmail(%s, %v) = %v", tt.f, tt.urgent, got)
		}
	}
}

func TestTicketEmailDue(t *testing.T) {
	oldest := time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	if TicketEmailDue(oldest, oldest.Add(GroupWindow-time.Second)) {
		t.Error("due before the group window closed")
	}
	if !TicketEmailDue(oldest, oldest.Add(GroupWindow)) {
		t.Error("not due when the group window closed")
	}
}

func TestDigestCutoff(t *testing.T) {
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	at := func(day, hour, minute int) time.Time { return time.Date(2026, 11, day, hour, minute, 0, 0, jakarta) }
	tests := []struct {
		name       string
		f          Frequency
		now        time.Time
		wantCutoff time.Time
		wantPeriod string
		wantOK     bool
	}{
		{"hourly mid-hour", FrequencyHourly, at(16, 10, 37), at(16, 10, 0), "2026-11-16T10", true},
		{"hourly on the hour", FrequencyHourly, at(16, 11, 0), at(16, 11, 0), "2026-11-16T11", true},
		{"daily after 08:00", FrequencyDaily, at(16, 8, 1), at(16, 8, 0), "2026-11-16", true},
		{"daily at 08:00", FrequencyDaily, at(16, 8, 0), at(16, 8, 0), "2026-11-16", true},
		{"daily before 08:00 keeps yesterday's", FrequencyDaily, at(16, 7, 59), at(15, 8, 0), "2026-11-15", true},
		{"daily across a month", FrequencyDaily, time.Date(2026, 12, 1, 6, 0, 0, 0, jakarta), at(30, 8, 0), "2026-11-30", true},
		{"instant has no summary", FrequencyInstant, at(16, 9, 0), time.Time{}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The clock may be in any zone; the boundary is local to the organisation.
			cutoff, period, ok := DigestCutoff(tt.f, tt.now.UTC(), jakarta)
			if !cutoff.Equal(tt.wantCutoff) || period != tt.wantPeriod || ok != tt.wantOK {
				t.Errorf("DigestCutoff = %v %q %v, want %v %q %v", cutoff, period, ok, tt.wantCutoff, tt.wantPeriod, tt.wantOK)
			}
		})
	}
}

// A daily summary goes out at 08:00 wall-clock time even on a day the clocks change.
func TestDailyCutoffAcrossDST(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 29, 9, 0, 0, 0, berlin) // clocks went forward at 02:00
	cutoff, _, _ := DigestCutoff(FrequencyDaily, now, berlin)
	if h, m, _ := cutoff.In(berlin).Clock(); h != 8 || m != 0 {
		t.Errorf("cutoff %v, want 08:00 local", cutoff.In(berlin))
	}
}

func TestPlanEmails(t *testing.T) {
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 16, 10, 30, 0, 0, jakarta)
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	user, hourly, daily := uuid.New(), uuid.New(), uuid.New()
	ticketA, ticketB := uuid.New(), uuid.New()
	ids := make([]uuid.UUID, 8)
	for i := range ids {
		ids[i] = uuid.New()
	}
	rows := []Pending{
		{ids[0], user, ticketA, false, ago(5 * time.Minute), FrequencyInstant},   // due, oldest of A
		{ids[1], user, ticketA, false, ago(time.Minute), FrequencyInstant},       // joins A's email
		{ids[2], user, ticketB, false, ago(time.Minute), FrequencyInstant},       // B still gathering
		{ids[3], hourly, ticketA, false, ago(40 * time.Minute), FrequencyHourly}, // before 10:00: summary
		{ids[4], hourly, ticketB, false, ago(10 * time.Minute), FrequencyHourly}, // after 10:00: waits
		{ids[5], hourly, ticketB, true, ago(3 * time.Minute), FrequencyHourly},   // urgent: own email now
		{ids[6], daily, ticketA, false, ago(time.Hour), FrequencyDaily},          // 09:30, after 08:00: waits
		{ids[7], daily, ticketB, false, ago(3 * time.Hour), FrequencyDaily},      // 07:30: today's summary
	}
	ticketEmails, digests := PlanEmails(rows, now.UTC(), jakarta)
	wantTicket := []TicketEmailArgs{
		{UserID: user, TicketID: ticketA, FirstID: ids[0]},
		{UserID: hourly, TicketID: ticketB, FirstID: ids[5]},
	}
	if !slices.Equal(ticketEmails, wantTicket) {
		t.Errorf("ticket emails\n got %v\nwant %v", ticketEmails, wantTicket)
	}
	at := func(h int) time.Time { return time.Date(2026, 11, 16, h, 0, 0, 0, jakarta) }
	wantDigests := []DigestArgs{
		{UserID: hourly, Period: "2026-11-16T10", Before: at(10)},
		{UserID: daily, Period: "2026-11-16", Before: at(8)},
	}
	if len(digests) != len(wantDigests) {
		t.Fatalf("digests = %v", digests)
	}
	for i, d := range digests {
		w := wantDigests[i]
		if d.UserID != w.UserID || d.Period != w.Period || !d.Before.Equal(w.Before) {
			t.Errorf("digest %d = %v, want %v", i, d, w)
		}
	}
}
