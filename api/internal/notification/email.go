package notification

import (
	"slices"
	"time"

	"github.com/google/uuid"
)

// Frequency is how often a user's notification emails go out. Values match the
// users.email_frequency CHECK constraint.
type Frequency string

const (
	FrequencyInstant Frequency = "instant" // one email per change, grouped over GroupWindow
	FrequencyHourly  Frequency = "hourly"
	FrequencyDaily   Frequency = "daily"
)

var Frequencies = []Frequency{FrequencyInstant, FrequencyHourly, FrequencyDaily}

func (f Frequency) Valid() bool { return slices.Contains(Frequencies, f) }

// Delivery timing and retention.
const (
	// GroupWindow is how long changes to one ticket gather before their email goes out, so
	// a burst of edits sends one email (HANDOFF §9: "about 2 minutes").
	GroupWindow = 2 * time.Minute
	// DailyDigestAt is when the daily summary goes out, in the organisation's time zone.
	DailyDigestAt = 8 * time.Hour
	// ClaimTimeout is how long a send job may hold rows before another job may take them
	// (a crashed job leaves its claim behind).
	ClaimTimeout = 15 * time.Minute
	// Retention is how long notifications are kept (owner, 15 Nov).
	Retention = 90 * 24 * time.Hour
	// QuoteMaxRunes caps the message excerpt an inbox item quotes.
	QuoteMaxRunes = 280
)

// Digest period formats: one hourly summary per local hour, one daily summary per local day.
const (
	hourlyPeriodLayout = "2006-01-02T15"
	dailyPeriodLayout  = "2006-01-02"
)

// UsesTicketEmail reports whether a pending row goes out in its own ticket email rather than a
// summary: everything for instant users, urgent items for everyone.
func UsesTicketEmail(f Frequency, urgent bool) bool {
	return urgent || f == FrequencyInstant
}

// TicketEmailDue reports whether the ticket email for rows gathering since oldest is due.
func TicketEmailDue(oldest, now time.Time) bool {
	return !now.Before(oldest.Add(GroupWindow))
}

// DigestCutoff is the latest summary boundary at or before now: the start of the current local
// hour for hourly users, the latest 08:00 for daily users. Pending summary rows created before
// it are due, and period names the summary so it is sent once. Instant users have none.
func DigestCutoff(f Frequency, now time.Time, loc *time.Location) (cutoff time.Time, period string, ok bool) {
	local := now.In(loc)
	switch f {
	case FrequencyHourly:
		cutoff = time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), 0, 0, 0, loc)
		return cutoff, cutoff.Format(hourlyPeriodLayout), true
	case FrequencyDaily:
		// Wall-clock 08:00 (time.Date normalises the minutes), so a DST day still sends at 08:00.
		atMinute := int(DailyDigestAt / time.Minute)
		cutoff = time.Date(local.Year(), local.Month(), local.Day(), 0, atMinute, 0, 0, loc)
		if local.Before(cutoff) {
			cutoff = time.Date(local.Year(), local.Month(), local.Day()-1, 0, atMinute, 0, 0, loc)
		}
		return cutoff, cutoff.Format(dailyPeriodLayout), true
	}
	return time.Time{}, "", false
}

// Pending is one notification still waiting for its email.
type Pending struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TicketID  uuid.UUID
	Urgent    bool
	CreatedAt time.Time
	Frequency Frequency // the user's, now
}

// PlanEmails decides which emails are due now: a ticket email per user and ticket whose oldest
// row for it has waited GroupWindow, and a summary per hourly or daily user with routine rows
// from before their latest boundary. Rows not yet due wait for a later run.
func PlanEmails(rows []Pending, now time.Time, loc *time.Location) ([]TicketEmailArgs, []DigestArgs) {
	type pair struct{ user, ticket uuid.UUID }
	oldest := map[pair]Pending{}
	var pairs []pair
	digests := map[uuid.UUID]DigestArgs{}
	oldestDigest := map[uuid.UUID]time.Time{}
	var digestUsers []uuid.UUID
	for _, r := range rows {
		if UsesTicketEmail(r.Frequency, r.Urgent) {
			k := pair{r.UserID, r.TicketID}
			if o, ok := oldest[k]; !ok || r.CreatedAt.Before(o.CreatedAt) {
				if !ok {
					pairs = append(pairs, k)
				}
				oldest[k] = r
			}
			continue
		}
		cutoff, period, ok := DigestCutoff(r.Frequency, now, loc)
		if !ok || !r.CreatedAt.Before(cutoff) {
			continue
		}
		d, seen := digests[r.UserID]
		if !seen {
			digestUsers = append(digestUsers, r.UserID)
			d = DigestArgs{UserID: r.UserID, Period: period, Before: cutoff}
		}
		if !seen || r.CreatedAt.Before(oldestDigest[r.UserID]) {
			d.FirstID, oldestDigest[r.UserID] = r.ID, r.CreatedAt
		}
		digests[r.UserID] = d
	}
	var ticketEmails []TicketEmailArgs
	for _, k := range pairs {
		if o := oldest[k]; TicketEmailDue(o.CreatedAt, now) {
			ticketEmails = append(ticketEmails, TicketEmailArgs{UserID: k.user, TicketID: k.ticket, FirstID: o.ID})
		}
	}
	out := make([]DigestArgs, 0, len(digestUsers))
	for _, u := range digestUsers {
		out = append(out, digests[u])
	}
	return ticketEmails, out
}
