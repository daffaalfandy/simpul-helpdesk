package ticket

import (
	"testing"

	"github.com/google/uuid"
)

func TestSelfAssignment(t *testing.T) {
	me, colleague := uuid.New(), uuid.New()
	tests := []struct {
		name          string
		current, next *uuid.UUID
		want          bool
	}{
		{"take an unassigned ticket", nil, &me, true},
		{"release my ticket", &me, nil, true},
		{"give an unassigned ticket to a colleague", nil, &colleague, false},
		{"hand my ticket to a colleague", &me, &colleague, false},
		{"take a colleague's ticket", &colleague, &me, false},
		{"unassign a colleague", &colleague, nil, false},
		{"move a colleague's ticket to another colleague", &colleague, &uuid.Nil, false},
		{"nothing to nothing", nil, nil, false},
	}
	for _, tt := range tests {
		if got := SelfAssignment(tt.current, tt.next, me); got != tt.want {
			t.Errorf("%s: SelfAssignment = %v, want %v", tt.name, got, tt.want)
		}
	}
}
