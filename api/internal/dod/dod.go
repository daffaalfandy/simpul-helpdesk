// Package dod is the pure Definition of Done (HANDOFF §7): what a project counts as done, which
// conditions a ticket has met, and when the client's check is approved automatically. It has no
// database access; the tickets and projects services store and load the state.
package dod

import (
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// Item is one condition on the way to done. Values match the ticket_dod_items.item CHECK.
type Item string

// Every item is the client's (owner, 17 Nov: Git no longer ticks code conditions).
const (
	ItemUATApproved        Item = "uat_approved"
	ItemRequesterConfirmed Item = "requester_confirmed"
)

// Items lists every item in checklist order.
var Items = []Item{ItemUATApproved, ItemRequesterConfirmed}

// ClientItems are the items the client ticks by answering the test (or the system, when the
// answer is overdue). Today that is every item.
var ClientItems = Items

// Cleared lists the items a status change forgets: a done ticket that opens again asks the
// client again.
func Cleared(from, to ticket.Status) []Item {
	if ticket.MarksResolved(from) && !ticket.MarksResolved(to) {
		return ClientItems
	}
	return nil
}

func (i Item) Valid() bool { return slices.Contains(Items, i) }

// Auto-approve choices on the project's UAT condition, in working days (Projects mock).
const (
	AutoApproveNever     = 0 // wait for the client's answer however long it takes
	AutoApproveThreeDays = 3
	AutoApproveFiveDays  = 5
	// DefaultAutoApproveDays is the UAT setting a new project starts with (HANDOFF §7).
	DefaultAutoApproveDays = AutoApproveThreeDays
	// RequesterConfirmsAutoDays is when a requester's confirmation is assumed (Projects mock).
	RequesterConfirmsAutoDays = 3
)

// AutoApproveDayOptions lists the auto-approve choices in order.
var AutoApproveDayOptions = []int{AutoApproveNever, AutoApproveThreeDays, AutoApproveFiveDays}

// Source says where a ticket's Definition of Done comes from.
type Source string

const (
	SourceProject Source = "project" // the project's Definition of Done
	SourceTicket  Source = "ticket"  // an agent set a different one on this ticket
)

// UAT is the client's acceptance test. The day count is kept while the condition is off.
type UAT struct {
	Enabled                bool `json:"enabled"`
	AutoApproveWorkingDays int  `json:"auto_approve_working_days"`
}

// Config is a Definition of Done. Its JSON is the storage format of projects.dod_config and
// ticket_dod_overrides.config (HANDOFF §7), so the tags must not change. JSON stored before the
// code conditions went (merged, ci_passed, deployed_production) still parses: unknown keys are
// ignored.
type Config struct {
	UAT               UAT  `json:"uat"`
	RequesterConfirms bool `json:"requester_confirms"`
}

// None has no conditions: an agent resolves each ticket by hand.
func None() Config {
	return Config{UAT: UAT{AutoApproveWorkingDays: DefaultAutoApproveDays}}
}

// Default is a new project's Definition of Done: the client approves a test, and the fix is
// approved automatically after three working days without an answer.
func Default() Config {
	c := None()
	c.UAT.Enabled = true
	return c
}

// Parse reads a stored configuration. Fields missing from older JSON keep None's values.
func Parse(b []byte) (Config, error) {
	c := None()
	err := json.Unmarshal(b, &c)
	return c, err
}

// JSON is the stored form of c.
func (c Config) JSON() []byte {
	b, err := json.Marshal(c)
	if err != nil {
		panic(err) // a struct of bools, ints and strings always marshals
	}
	return b
}

var ErrAutoApproveDays = errors.New("auto-approve must be never, 3 or 5 working days")

// Validate checks a configuration.
func Validate(c Config) error {
	if !slices.Contains(AutoApproveDayOptions, c.UAT.AutoApproveWorkingDays) {
		return ErrAutoApproveDays
	}
	return nil
}

// Required lists the items that must be done before the ticket counts as done, in order: what
// the ticket's checklist shows.
func Required(c Config) []Item {
	var out []Item
	if c.UAT.Enabled {
		out = append(out, ItemUATApproved)
	}
	if c.RequesterConfirms {
		out = append(out, ItemRequesterConfirmed)
	}
	return out
}

// Evaluate reports whether every required item is done, and lists the ones still pending.
func Evaluate(c Config, done map[Item]bool) (met bool, pending []Item) {
	for _, it := range Required(c) {
		if !done[it] {
			pending = append(pending, it)
		}
	}
	return len(pending) == 0, pending
}

// ClientRequired lists the client items c requires: what one answer to the test meets. Every
// item is the client's, so it is Required.
func ClientRequired(c Config) []Item { return Required(c) }

// HasClientCheck reports whether the client answers before the ticket is done. UAT and the
// requester's confirmation are one step: the ticket waits In UAT and one answer ticks both.
func HasClientCheck(c Config) bool { return c.UAT.Enabled || c.RequesterConfirms }

// AutoApproveDays is how many working days a ticket waits In UAT before the system approves
// it. ok is false when it waits for the client's answer however long it takes.
func AutoApproveDays(c Config) (days int, ok bool) {
	switch {
	case c.UAT.Enabled:
		return c.UAT.AutoApproveWorkingDays, c.UAT.AutoApproveWorkingDays != AutoApproveNever
	case c.RequesterConfirms:
		return RequesterConfirmsAutoDays, true
	}
	return 0, false
}

// Effective is the Definition of Done a ticket follows: its own when an agent set one, else
// the project's current one (project changes apply to open tickets).
func Effective(project Config, override *Config) (Config, Source) {
	if override != nil {
		return *override, SourceTicket
	}
	return project, SourceProject
}

// ParseEffective reads a ticket's stored Definition of Done (the project's JSON and the
// ticket's own, nil when it has none) and returns the one it follows.
func ParseEffective(projectJSON, overrideJSON []byte) (Config, Source, error) {
	project, err := Parse(projectJSON)
	if err != nil {
		return Config{}, "", err
	}
	if overrideJSON == nil {
		return project, SourceProject, nil
	}
	own, err := Parse(overrideJSON)
	if err != nil {
		return Config{}, "", err
	}
	return own, SourceTicket, nil
}

// AutoApproveAt is when a ticket that entered In UAT at enteredAt is approved automatically.
func AutoApproveAt(c Config, enteredAt time.Time, s sla.Schedules) (time.Time, bool) {
	days, ok := AutoApproveDays(c)
	if !ok {
		return time.Time{}, false
	}
	return s.AddWorkingDays(enteredAt, days), true
}

// UATDay is which working day of the client's test now falls on, counting from 1. It stops at
// days when the test has a limit (days > 0), so an overdue test reads "day 3 of 3".
func UATDay(enteredAt, now time.Time, s sla.Schedules, days int) int {
	day := s.WorkingDaysBetween(enteredAt, now) + 1
	if days > 0 && day > days {
		return days
	}
	return day
}
