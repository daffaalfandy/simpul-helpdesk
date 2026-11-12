// Package audit writes entries to the append-only audit log.
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"simdesk/internal/db"
	"simdesk/internal/reqinfo"
)

// Actor kinds.
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
			return fmt.Errorf("audit data: %w", err)
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
	if err := q.InsertAudit(ctx, params); err != nil {
		return fmt.Errorf("write audit %s: %w", e.Action, err)
	}
	return nil
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
