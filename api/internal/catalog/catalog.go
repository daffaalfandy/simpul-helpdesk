// Package catalog manages the service catalogue the report form offers for requests. Delivery
// times come from each project's SLA (follow-up #1). Items tickets use are hidden, not deleted.
package catalog

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/audit"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/db"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/store"
)

// Audit actions and target type written by this package.
const (
	ActionCatalogItemCreated = "catalog_item.created"
	ActionCatalogItemUpdated = "catalog_item.updated"
	ActionCatalogItemDeleted = "catalog_item.deleted"

	TargetCatalogItem = "catalog_item"
)

const (
	dataName        = "name"
	dataDescription = "description"
	dataSortOrder   = "sort_order"
	dataActive      = "active"
)

// Field limits, matching the database CHECK constraints.
const (
	MaxNameLen        = 100
	MaxDescriptionLen = 300
	MaxSortOrder      = 10000
)

const (
	constraintItemName     = "catalog_items_name_key"
	constraintTicketItemFK = "tickets_catalog_item_id_fkey"
)

// Item is an entry of the service catalogue ("What do you need?").
type Item struct {
	ID          uuid.UUID
	Name        string
	Description *string
	SortOrder   int
	Active      bool // shown on the report form
	TicketCount int
}

type ItemInput struct {
	Name        string
	Description *string
	SortOrder   int
	Active      bool
}

type Service struct {
	store *store.Store
}

func NewService(st *store.Store) *Service { return &Service{store: st} }

// ListItems returns catalogue items in display order; activeOnly leaves out hidden ones.
func (s *Service) ListItems(ctx context.Context, activeOnly bool) ([]Item, error) {
	rows, err := s.store.Q().ListCatalogItems(ctx, activeFilter(activeOnly))
	if err != nil {
		return nil, err
	}
	out := make([]Item, 0, len(rows))
	for _, r := range rows {
		out = append(out, itemFromRow(db.GetCatalogItemRow(r)))
	}
	return out, nil
}

func (s *Service) CreateItem(ctx context.Context, actor audit.Actor, in ItemInput) (Item, error) {
	in, err := normalizeItem(in)
	if err != nil {
		return Item{}, err
	}
	var id uuid.UUID
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		if id, err = tx.Q.CreateCatalogItem(ctx, db.CreateCatalogItemParams{
			Name: in.Name, Description: in.Description, SortOrder: int32(in.SortOrder), Active: in.Active,
		}); err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionCatalogItemCreated, TargetCatalogItem, &id, map[string]any{
			dataName: in.Name, dataActive: in.Active,
		}))
	})
	if err != nil {
		return Item{}, catalogError(err)
	}
	return s.getItem(ctx, id)
}

// UpdateItem replaces a catalogue item.
func (s *Service) UpdateItem(ctx context.Context, actor audit.Actor, id uuid.UUID, in ItemInput) (Item, error) {
	in, err := normalizeItem(in)
	if err != nil {
		return Item{}, err
	}
	err = s.store.Tx(ctx, func(tx store.Tx) error {
		old, err := tx.Q.GetCatalogItemForUpdate(ctx, id)
		if err != nil {
			return notFound(err)
		}
		if err := tx.Q.UpdateCatalogItem(ctx, db.UpdateCatalogItemParams{
			ID: id, Name: in.Name, Description: in.Description, SortOrder: int32(in.SortOrder), Active: in.Active,
		}); err != nil {
			return err
		}
		changes := audit.Changes{}
		audit.Track(changes, dataName, old.Name, in.Name)
		audit.TrackPtr(changes, dataDescription, old.Description, in.Description)
		audit.Track(changes, dataSortOrder, int(old.SortOrder), in.SortOrder)
		audit.Track(changes, dataActive, old.Active, in.Active)
		if len(changes) == 0 {
			return nil
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionCatalogItemUpdated, TargetCatalogItem, &id, changes))
	})
	if err != nil {
		return Item{}, catalogError(err)
	}
	return s.getItem(ctx, id)
}

// DeleteItem removes a catalogue item no ticket uses.
func (s *Service) DeleteItem(ctx context.Context, actor audit.Actor, id uuid.UUID) error {
	err := s.store.Tx(ctx, func(tx store.Tx) error {
		old, err := tx.Q.GetCatalogItemForUpdate(ctx, id)
		if err != nil {
			return notFound(err)
		}
		if n, err := tx.Q.CountCatalogItemTickets(ctx, &id); err != nil {
			return err
		} else if n > 0 {
			return errItemInUse()
		}
		if err := tx.Q.DeleteCatalogItem(ctx, id); err != nil {
			return err
		}
		return audit.Write(ctx, tx.Q, actor.Entry(ActionCatalogItemDeleted, TargetCatalogItem, &id, map[string]any{dataName: old.Name}))
	})
	return catalogError(err)
}

func (s *Service) getItem(ctx context.Context, id uuid.UUID) (Item, error) {
	r, err := s.store.Q().GetCatalogItem(ctx, id)
	if err != nil {
		return Item{}, notFound(err)
	}
	return itemFromRow(r), nil
}

func itemFromRow(r db.GetCatalogItemRow) Item {
	return Item{
		ID: r.ID, Name: r.Name, Description: r.Description, SortOrder: int(r.SortOrder), Active: r.Active,
		TicketCount: int(r.TicketCount),
	}
}

func activeFilter(activeOnly bool) *bool {
	if !activeOnly {
		return nil
	}
	active := true
	return &active
}

func normalizeItem(in ItemInput) (ItemInput, error) {
	var err error
	in.Name, in.Description, err = normalizeEntry(in.Name, in.Description, in.SortOrder)
	return in, err
}

func normalizeEntry(name string, description *string, sortOrder int) (string, *string, error) {
	name = strings.TrimSpace(name)
	if description != nil {
		d := strings.TrimSpace(*description)
		description = &d
		if d == "" {
			description = nil
		}
	}
	switch {
	case name == "" || len([]rune(name)) > MaxNameLen:
		return name, description, apperr.Invalid(apperr.CodeInvalidName, "Enter a name", "")
	case description != nil && len([]rune(*description)) > MaxDescriptionLen:
		return name, description, apperr.Invalid(apperr.CodeInvalidRequest, "The description is too long", "")
	case sortOrder < 0 || sortOrder > MaxSortOrder:
		return name, description, apperr.Invalid(apperr.CodeInvalidRequest, "The sort order is out of range", "")
	}
	return name, description, nil
}

func catalogError(err error) error {
	switch {
	case err == nil:
		return nil
	case store.IsViolation(err, store.SQLStateUniqueViolation, constraintItemName):
		return apperr.Conflict(apperr.CodeCatalogItemNameTaken, "A catalogue item with this name already exists", "")
	case store.IsViolation(err, store.SQLStateRestrictViolation, constraintTicketItemFK):
		return errItemInUse()
	}
	return err
}

func errItemInUse() error {
	return apperr.Conflict(apperr.CodeCatalogItemInUse, "Tickets use this catalogue item", "Hide it from the report form instead.")
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound()
	}
	return err
}
