package ticket

import (
	"slices"
	"testing"
)

func TestColumnOf(t *testing.T) {
	cases := []struct {
		status Status
		column Column
		on     bool
	}{
		{StatusNew, ColumnNew, true},
		{StatusAssigned, ColumnNew, true},
		{StatusInProgress, ColumnInProgress, true},
		{StatusWaitingOnClient, ColumnWaitingOnClient, true},
		{StatusInUAT, ColumnInUAT, true},
		{StatusResolved, ColumnResolved, true},
		{StatusClosed, "", false},
		{Status("archived"), "", false},
	}
	for _, c := range cases {
		col, on := ColumnOf(c.status)
		if col != c.column || on != c.on {
			t.Errorf("ColumnOf(%s) = %q, %v; want %q, %v", c.status, col, on, c.column, c.on)
		}
	}
}

func TestDropStatus(t *testing.T) {
	cases := []struct {
		column      Column
		hasAssignee bool
		want        Status
	}{
		{ColumnNew, false, StatusNew},
		{ColumnNew, true, StatusAssigned},
		{ColumnInProgress, false, StatusInProgress},
		{ColumnInProgress, true, StatusInProgress},
		{ColumnWaitingOnClient, true, StatusWaitingOnClient},
		{ColumnInUAT, true, StatusInUAT},
		{ColumnResolved, false, StatusResolved},
	}
	for _, c := range cases {
		if got := DropStatus(c.column, c.hasAssignee); got != c.want {
			t.Errorf("DropStatus(%s, %v) = %s, want %s", c.column, c.hasAssignee, got, c.want)
		}
	}
}

// Every column's drop status shows in that same column, so a card never lands somewhere else.
func TestDropStaysInColumn(t *testing.T) {
	for _, c := range Columns {
		for _, owned := range []bool{false, true} {
			if got, _ := ColumnOf(DropStatus(c, owned)); got != c {
				t.Errorf("drop into %s (owned %v) shows in %s", c, owned, got)
			}
		}
	}
}

func TestBoardStatuses(t *testing.T) {
	want := []Status{StatusNew, StatusAssigned, StatusInProgress, StatusWaitingOnClient, StatusInUAT, StatusResolved}
	if got := BoardStatuses(); !slices.Equal(got, want) {
		t.Fatalf("BoardStatuses() = %v, want %v", got, want)
	}
}
