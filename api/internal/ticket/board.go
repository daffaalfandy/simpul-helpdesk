package ticket

import (
	"slices"
	"time"
)

// Column is a column of the agent board (HANDOFF §4). Values match the BoardColumnId enum.
type Column string

const (
	ColumnNew             Column = "new"
	ColumnInProgress      Column = "in_progress"
	ColumnWaitingOnClient Column = "waiting_on_client"
	ColumnInUAT           Column = "in_uat"
	ColumnResolved        Column = "resolved" // resolved today only
)

// Columns lists the board's columns from left to right.
var Columns = []Column{ColumnNew, ColumnInProgress, ColumnWaitingOnClient, ColumnInUAT, ColumnResolved}

// columnStatuses: New holds the work nobody has started yet, owned or not. Closed tickets
// are off the board.
var columnStatuses = map[Column][]Status{
	ColumnNew:             {StatusNew, StatusAssigned},
	ColumnInProgress:      {StatusInProgress},
	ColumnWaitingOnClient: {StatusWaitingOnClient},
	ColumnInUAT:           {StatusInUAT},
	ColumnResolved:        {StatusResolved},
}

// BoardWeekStart is the first day of the week for the board's "Resolved this week".
const BoardWeekStart = time.Monday

// ColumnOf reports the column a status shows in, and false for statuses off the board.
func ColumnOf(s Status) (Column, bool) {
	for _, c := range Columns {
		if slices.Contains(columnStatuses[c], s) {
			return c, true
		}
	}
	return "", false
}

// DropStatus is the status a card dropped into the column gets. New splits on the owner:
// assigned when someone has the ticket, new when nobody does.
func DropStatus(c Column, hasAssignee bool) Status {
	if c == ColumnNew && hasAssignee {
		return StatusAssigned
	}
	return columnStatuses[c][0]
}

// BoardStatuses lists every status on the board in lifecycle order.
func BoardStatuses() []Status {
	out := []Status{}
	for _, s := range Statuses {
		if _, ok := ColumnOf(s); ok {
			out = append(out, s)
		}
	}
	return out
}
