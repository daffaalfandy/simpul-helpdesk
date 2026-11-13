package httpapi

import (
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/catalog"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
)

// ListSystems lists every system, hidden ones included.
func (s *server) ListSystems(ctx context.Context, _ gen.ListSystemsRequestObject) (gen.ListSystemsResponseObject, error) {
	if _, err := auth.Require(ctx, auth.CapAgentApp); err != nil {
		return nil, err
	}
	list, err := s.catalog.ListSystems(ctx, false)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListSystems200JSONResponse, len(list))
	for i, sys := range list {
		out[i] = apiSystem(sys)
	}
	return out, nil
}

func (s *server) CreateSystem(ctx context.Context, req gen.CreateSystemRequestObject) (gen.CreateSystemResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	sys, err := s.catalog.CreateSystem(ctx, p.AuditActor(), systemInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return gen.CreateSystem201JSONResponse(apiSystem(sys)), nil
}

func (s *server) UpdateSystem(ctx context.Context, req gen.UpdateSystemRequestObject) (gen.UpdateSystemResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	sys, err := s.catalog.UpdateSystem(ctx, p.AuditActor(), req.SystemId, systemInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return gen.UpdateSystem200JSONResponse(apiSystem(sys)), nil
}

func (s *server) DeleteSystem(ctx context.Context, req gen.DeleteSystemRequestObject) (gen.DeleteSystemResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	if err := s.catalog.DeleteSystem(ctx, p.AuditActor(), req.SystemId); err != nil {
		return nil, err
	}
	return gen.DeleteSystem204Response{}, nil
}

// ListCatalogItems lists every catalogue item, hidden ones included.
func (s *server) ListCatalogItems(ctx context.Context, _ gen.ListCatalogItemsRequestObject) (gen.ListCatalogItemsResponseObject, error) {
	if _, err := auth.Require(ctx, auth.CapAgentApp); err != nil {
		return nil, err
	}
	list, err := s.catalog.ListItems(ctx, false)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListCatalogItems200JSONResponse, len(list))
	for i, it := range list {
		out[i] = apiCatalogItem(it)
	}
	return out, nil
}

func (s *server) CreateCatalogItem(ctx context.Context, req gen.CreateCatalogItemRequestObject) (gen.CreateCatalogItemResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	it, err := s.catalog.CreateItem(ctx, p.AuditActor(), itemInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return gen.CreateCatalogItem201JSONResponse(apiCatalogItem(it)), nil
}

func (s *server) UpdateCatalogItem(ctx context.Context, req gen.UpdateCatalogItemRequestObject) (gen.UpdateCatalogItemResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	it, err := s.catalog.UpdateItem(ctx, p.AuditActor(), req.CatalogItemId, itemInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return gen.UpdateCatalogItem200JSONResponse(apiCatalogItem(it)), nil
}

func (s *server) DeleteCatalogItem(ctx context.Context, req gen.DeleteCatalogItemRequestObject) (gen.DeleteCatalogItemResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	if err := s.catalog.DeleteItem(ctx, p.AuditActor(), req.CatalogItemId); err != nil {
		return nil, err
	}
	return gen.DeleteCatalogItem204Response{}, nil
}

func systemInput(b gen.SystemInput) catalog.SystemInput {
	return catalog.SystemInput{Name: b.Name, Description: b.Description, SortOrder: b.SortOrder, Active: b.Active}
}

func itemInput(b gen.CatalogItemInput) catalog.ItemInput {
	return catalog.ItemInput{
		Name: b.Name, Description: b.Description, DeliveryMinutes: b.DeliveryMinutes, SortOrder: b.SortOrder, Active: b.Active,
	}
}

func apiSystem(s catalog.System) gen.System {
	return gen.System{
		Id: s.ID, Name: s.Name, Description: s.Description, SortOrder: s.SortOrder, Active: s.Active, TicketCount: s.TicketCount,
	}
}

func apiCatalogItem(i catalog.Item) gen.CatalogItem {
	return gen.CatalogItem{
		Id: i.ID, Name: i.Name, Description: i.Description, DeliveryMinutes: i.DeliveryMinutes,
		SortOrder: i.SortOrder, Active: i.Active, TicketCount: i.TicketCount,
	}
}
