package tickets

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/projects"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// SnapshotFor copies the project's current SLA (targets and settings) for storing on a new
// ticket, so later SLA edits never change it.
func SnapshotFor(ctx context.Context, q *db.Queries, projectID uuid.UUID) (ticket.SLASnapshot, error) {
	p, err := projects.Load(ctx, q, projectID)
	if err != nil {
		return ticket.SLASnapshot{}, fmt.Errorf("project SLA: %w", err)
	}
	return ticket.SLASnapshot{
		Version: ticket.SnapshotVersion, ProjectID: p.ID, ProjectName: p.Name, ProjectSLA: p.SLA,
	}, nil
}

func parseSnapshot(raw []byte) (ticket.SLASnapshot, error) {
	var snap ticket.SLASnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return snap, fmt.Errorf("ticket SLA snapshot: %w", err)
	}
	if snap.Version != ticket.SnapshotVersion {
		return snap, fmt.Errorf("%w: %d", ticket.ErrSnapshotVersion, snap.Version)
	}
	return snap, nil
}
