package audit

import "github.com/google/uuid"

// ActorNameCLI names the actor for changes made from the command line.
const ActorNameCLI = "cli"

// Actor is who made a change: a signed-in user, the system (CLI, jobs), or code activity from
// GitHub/GitLab.
type Actor struct {
	UserID *uuid.UUID
	Name   string
	git    bool
	// org is the organization the actor works in; its entries carry it.
	org *uuid.UUID
}

// In returns the actor working in the organization, so its entries name it.
func (a Actor) In(orgID uuid.UUID) Actor {
	if orgID != uuid.Nil {
		a.org = &orgID
	}
	return a
}

func UserActor(id uuid.UUID, name string) Actor { return Actor{UserID: &id, Name: name} }

// SystemActor is a non-user actor such as the CLI.
func SystemActor(name string) Actor { return Actor{Name: name} }

// GitActor is code activity: a commit, pull/merge request, CI run or deployment. userID is the
// staff member who wrote the code when their email is known, so they aren't notified of it.
func GitActor(userID *uuid.UUID, name string) Actor {
	return Actor{UserID: userID, Name: name, git: true}
}

func (a Actor) Entry(action, targetType string, targetID *uuid.UUID, data map[string]any) Entry {
	kind := ActorSystem
	switch {
	case a.git:
		kind = ActorGit
	case a.UserID != nil:
		kind = ActorUser
	}
	return Entry{
		OrganizationID: a.org, ActorKind: kind, ActorUserID: a.UserID, ActorName: a.Name,
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
