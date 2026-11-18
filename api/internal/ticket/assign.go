package ticket

import "github.com/google/uuid"

// SelfAssignment reports whether moving a ticket from the current assignee to next (nil: nobody)
// only takes an unassigned ticket for self or releases self's own ticket. That is all an agent may
// do; supervisors and administrators assign anyone (owner, 18 Nov). Callers treat an unchanged
// assignee as a no-op before asking.
func SelfAssignment(current, next *uuid.UUID, self uuid.UUID) bool {
	switch {
	case current == nil && next != nil:
		return *next == self
	case current != nil && next == nil:
		return *current == self
	}
	return false
}
