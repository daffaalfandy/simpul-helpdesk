// Package audit writes entries to the append-only audit log.
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/reqinfo"
)

const (
	ActorUser   = "user"
	ActorSystem = "system"
	ActorGit    = "git"
)

// Entry is one audit record. Write it in the same transaction as the change it describes.
type Entry struct {
	ActorKind   string
	ActorUserID *uuid.UUID
	ActorName   string
	Action      string
	TargetType  string
	TargetID    *uuid.UUID
	Data        map[string]any
	Reason      string
}

// Write inserts the entry, filling IP and request ID from the context.
func Write(ctx context.Context, q *db.Queries, e Entry) error {
	_, err := Insert(ctx, q, e)
	return err
}

// Insert is Write that also returns the entry's id, for jobs that read the entry back.
func Insert(ctx context.Context, q *db.Queries, e Entry) (uuid.UUID, error) {
	if e.ActorKind == "" {
		e.ActorKind = ActorSystem
		if e.ActorUserID != nil {
			e.ActorKind = ActorUser
		}
	}
	data := []byte("{}")
	if len(e.Data) > 0 {
		b, err := json.Marshal(e.Data)
		if err != nil {
			return uuid.Nil, fmt.Errorf("audit data: %w", err)
		}
		data = b
	}
	info := reqinfo.From(ctx)
	params := db.InsertAuditParams{
		ActorKind:   e.ActorKind,
		ActorUserID: e.ActorUserID,
		ActorName:   nonEmpty(e.ActorName),
		Action:      e.Action,
		TargetType:  nonEmpty(e.TargetType),
		TargetID:    e.TargetID,
		Data:        data,
		Reason:      nonEmpty(e.Reason),
		RequestID:   nonEmpty(info.RequestID),
	}
	if info.IP.IsValid() {
		ip := info.IP
		params.Ip = &ip
	}
	id, err := q.InsertAudit(ctx, params)
	if err != nil {
		return uuid.Nil, fmt.Errorf("write audit %s: %w", e.Action, err)
	}
	return id, nil
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
