package audit

import (
	"testing"

	"github.com/google/uuid"
)

func TestActorEntryKind(t *testing.T) {
	id := uuid.New()
	if e := UserActor(id, "Rina").Entry("x.y", "x", nil, nil); e.ActorKind != ActorUser || *e.ActorUserID != id || e.ActorName != "Rina" {
		t.Fatalf("user entry = %+v", e)
	}
	if e := SystemActor(ActorNameCLI).Entry("x.y", "x", nil, nil); e.ActorKind != ActorSystem || e.ActorUserID != nil {
		t.Fatalf("system entry = %+v", e)
	}
}

func TestTrackRecordsOnlyChanges(t *testing.T) {
	a, b, a2 := "a", "b", "a"
	c := Changes{}
	Track(c, "same", 1, 1)
	Track(c, "changed", 1, 2)
	TrackPtr(c, "both nil", (*string)(nil), nil)
	TrackPtr(c, "equal values", &a, &a2)
	TrackPtr(c, "set", nil, &b)
	TrackPtr(c, "cleared", &a, nil)
	TrackPtr(c, "different", &a, &b)
	for _, k := range []string{"changed", "set", "cleared", "different"} {
		if _, ok := c[k]; !ok {
			t.Errorf("%s not tracked", k)
		}
	}
	if len(c) != 4 {
		t.Fatalf("changes = %v", c)
	}
}
