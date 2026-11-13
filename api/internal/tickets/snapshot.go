package tickets

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// SnapshotFor copies the client's current SLA plan, plus the catalogue item's delivery time
// when the ticket has one, for storing on a new ticket.
func SnapshotFor(ctx context.Context, q *db.Queries, clientID uuid.UUID, catalogDeliverMinutes *int) (ticket.SLASnapshot, error) {
	c, err := q.GetClient(ctx, db.GetClientParams{ID: clientID})
	if err != nil {
		return ticket.SLASnapshot{}, fmt.Errorf("client plan: %w", err)
	}
	targets, err := sla.Targets(ctx, q, c.SlaPlanID)
	if err != nil {
		return ticket.SLASnapshot{}, err
	}
	return ticket.SLASnapshot{
		Version: ticket.SnapshotVersion, PlanID: c.SlaPlanID, PlanName: c.SlaPlanName,
		Targets: targets, CatalogDeliverMinutes: catalogDeliverMinutes,
	}, nil
}

func parseSnapshot(raw []byte) (ticket.SLASnapshot, error) {
	var snap ticket.SLASnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return snap, fmt.Errorf("ticket SLA snapshot: %w", err)
	}
	return snap, nil
}
