package audit

import "github.com/google/uuid"

// ActorNameCLI names the actor for changes made from the command line.
const ActorNameCLI = "cli"

// Actor is who made a change: a signed-in user, or the system (CLI, jobs).
type Actor struct {
	UserID *uuid.UUID
	Name   string
}

func UserActor(id uuid.UUID, name string) Actor { return Actor{UserID: &id, Name: name} }

// SystemActor is a non-user actor such as the CLI.
func SystemActor(name string) Actor { return Actor{Name: name} }

func (a Actor) Entry(action, targetType string, targetID *uuid.UUID, data map[string]any) Entry {
	kind := ActorSystem
	if a.UserID != nil {
		kind = ActorUser
	}
	return Entry{
		ActorKind: kind, ActorUserID: a.UserID, ActorName: a.Name,
		Action: action, TargetType: targetType, TargetID: targetID, Data: data,
	}
}

// Change is one field's old and new value.
type Change struct {
	From any `json:"from"`
	To   any `json:"to"`
}

// Changes collects the fields an update changed, for an entry's Data.
type Changes map[string]any

// Track records field if the value changed.
func Track[T comparable](c Changes, field string, from, to T) {
	if from != to {
		c[field] = Change{From: from, To: to}
	}
}

// TrackPtr records field if the pointed-to value changed; nil means "not set".
func TrackPtr[T comparable](c Changes, field string, from, to *T) {
	switch {
	case from == nil && to == nil:
	case from == nil || to == nil || *from != *to:
		c[field] = Change{From: from, To: to}
	}
}
