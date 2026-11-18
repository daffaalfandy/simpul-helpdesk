// Package dod is the pure Definition of Done (HANDOFF §7): what a project counts as done, which
// conditions a ticket has met, and when the client's check is approved automatically. It has no
// database access; the tickets and projects services store and load the state.
package dod

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// Item is one condition on the way to done. Values match the ticket_dod_items.item CHECK.
type Item string

const (
	// ItemCodeOnBranch is ticked by Git when a commit naming the ticket reaches the branch the
	// Definition of Done names (10b, owner 18 Nov). It never moves the ticket.
	ItemCodeOnBranch       Item = "code_on_branch"
	ItemUATApproved        Item = "uat_approved"
	ItemRequesterConfirmed Item = "requester_confirmed"
)

// Items lists every item in checklist order.
var Items = []Item{ItemCodeOnBranch, ItemUATApproved, ItemRequesterConfirmed}

// ClientItems are the items the client ticks by answering the test (or the system, when the
// answer is overdue).
var ClientItems = []Item{ItemUATApproved, ItemRequesterConfirmed}

// CodeItems are the items read from the repository.
var CodeItems = []Item{ItemCodeOnBranch}

// workStatuses are where a ticket is worked on, before anyone checks the result.
var workStatuses = []ticket.Status{ticket.StatusNew, ticket.StatusAssigned, ticket.StatusInProgress, ticket.StatusWaitingOnClient}

// Cleared lists the items a status change forgets. A done ticket that opens again asks the
// client again. A ticket sent back to work from the client's test or from done needs a new fix,
// so its code item goes too: the commit that ticked it didn't fix the ticket.
func Cleared(from, to ticket.Status) []Item {
	var out []Item
	if ticket.MarksResolved(from) && !ticket.MarksResolved(to) {
		out = append(out, ClientItems...)
	}
	if (from == ticket.StatusInUAT || ticket.MarksResolved(from)) && slices.Contains(workStatuses, to) {
		out = append(out, CodeItems...)
	}
	return out
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

// CodeOnBranch asks for a commit naming the ticket to reach Branch (say main, once the fix
// merges). The branch is kept while the condition is off.
type CodeOnBranch struct {
	Enabled bool   `json:"enabled"`
	Branch  string `json:"branch"`
}

// UAT is the client's acceptance test. The day count is kept while the condition is off.
type UAT struct {
	Enabled                bool `json:"enabled"`
	AutoApproveWorkingDays int  `json:"auto_approve_working_days"`
}

// Config is a Definition of Done. Its JSON is the storage format of projects.dod_config and
// ticket_dod_overrides.config (HANDOFF §7), so the tags must not change. JSON stored before the
// code conditions went (merged, ci_passed, deployed_production) still parses: unknown keys are
// ignored. Config stays comparable (==): the services compare stored and saved ones.
type Config struct {
	CodeOnBranch      CodeOnBranch `json:"code_on_branch"`
	UAT               UAT          `json:"uat"`
	RequesterConfirms bool         `json:"requester_confirms"`
}

// None has no conditions: an agent resolves each ticket by hand.
func None() Config {
	return Config{UAT: UAT{AutoApproveWorkingDays: DefaultAutoApproveDays}}
}

// DefaultMandatory: a project's Definition of Done starts mandatory, so resolving with it unmet
// needs a reason (owner, 18 Nov).
const DefaultMandatory = true

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

var (
	ErrAutoApproveDays = errors.New("auto-approve must be never, 3 or 5 working days")
	ErrInvalidBranch   = errors.New("enter a valid branch name, such as main")
	ErrNeedsRepository = errors.New("code reaching a branch needs a repository linked to the project")
)

// MaxBranchLen is the longest branch name, as code_events stores it.
const MaxBranchLen = 255

// branchRefPrefix starts a full branch ref (refs/heads/main); people paste it from Git.
const branchRefPrefix = "refs/heads/"

// Parts of a name Git refuses in a branch (git check-ref-format), the subset worth checking here.
const (
	branchForbiddenRunes = "~^:?*[\\"
	branchDotDot         = ".."
	branchDoubleSlash    = "//"
	branchReflog         = "@{"
	branchAt             = "@"
	branchDash           = "-"
	branchSlash          = "/"
	branchDot            = "."
	branchLockSuffix     = ".lock"
)

// NormalizeBranch trims a branch name and drops a refs/heads/ prefix.
func NormalizeBranch(name string) string {
	name = strings.TrimSpace(name)
	return strings.TrimPrefix(name, branchRefPrefix)
}

// Normalize is c with its branch name normalized (NormalizeBranch).
func Normalize(c Config) Config {
	c.CodeOnBranch.Branch = NormalizeBranch(c.CodeOnBranch.Branch)
	return c
}

// ValidBranch reports whether name is a branch name Git accepts, by the rules of
// git check-ref-format that matter for names people type.
func ValidBranch(name string) bool {
	switch {
	case name == "" || utf8.RuneCountInString(name) > MaxBranchLen || name == branchAt,
		strings.ContainsAny(name, branchForbiddenRunes),
		strings.Contains(name, branchDotDot), strings.Contains(name, branchDoubleSlash), strings.Contains(name, branchReflog),
		strings.HasPrefix(name, branchDash), strings.HasPrefix(name, branchSlash),
		strings.HasSuffix(name, branchSlash), strings.HasSuffix(name, branchDot), strings.HasSuffix(name, branchLockSuffix):
		return false
	}
	for _, r := range name {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	for _, part := range strings.Split(name, branchSlash) {
		if strings.HasPrefix(part, branchDot) {
			return false
		}
	}
	return true
}

// Validate checks a configuration whose branch is normalized. Code reaching a branch is read
// from the project's repository, so turning it on needs one; a branch kept while the condition
// is off may be empty.
func Validate(c Config, hasRepository bool) error {
	switch {
	case !slices.Contains(AutoApproveDayOptions, c.UAT.AutoApproveWorkingDays):
		return ErrAutoApproveDays
	case (c.CodeOnBranch.Enabled || c.CodeOnBranch.Branch != "") && !ValidBranch(c.CodeOnBranch.Branch):
		return ErrInvalidBranch
	case c.CodeOnBranch.Enabled && !hasRepository:
		return ErrNeedsRepository
	}
	return nil
}

// ForRepository is c as a ticket is held to it: code reaching a branch stops applying while
// the project has no repository to read it from.
func (c Config) ForRepository(hasRepository bool) Config {
	if !hasRepository {
		c.CodeOnBranch.Enabled = false
	}
	return c
}

// BranchReached reports whether a push to branch meets c's code condition. Names match exactly,
// letter case included, as Git compares them.
func BranchReached(c Config, branch string) bool {
	return c.CodeOnBranch.Enabled && branch != "" && branch == c.CodeOnBranch.Branch
}

// BranchChanged reports whether going from prev to next asks for code on a branch it didn't ask
// for before: the condition was turned on, or names another branch. Commits already linked may
// meet it then.
func BranchChanged(prev, next Config) bool {
	return next.CodeOnBranch.Enabled && (!prev.CodeOnBranch.Enabled || prev.CodeOnBranch.Branch != next.CodeOnBranch.Branch)
}

// ResolveNeedsReason reports whether resolving a ticket by hand needs a reason: its project's
// Definition of Done is mandatory and items are still pending. One that isn't mandatory is
// guidance only, and anyone resolves freely.
func ResolveNeedsReason(mandatory bool, pending []Item) bool { return mandatory && len(pending) > 0 }

// Required lists the items that must be done before the ticket counts as done, in order: what
// the ticket's checklist shows.
func Required(c Config) []Item {
	var out []Item
	if c.CodeOnBranch.Enabled {
		out = append(out, ItemCodeOnBranch)
	}
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

// ClientRequired lists the client items c requires: what one answer to the test meets.
func ClientRequired(c Config) []Item {
	var out []Item
	for _, it := range Required(c) {
		if slices.Contains(ClientItems, it) {
			out = append(out, it)
		}
	}
	return out
}

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
