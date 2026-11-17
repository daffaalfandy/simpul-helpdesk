package httpapi

import (
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/catalog"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
)

// ListCatalogItems lists every catalogue item, hidden ones included.
func (s *server) ListCatalogItems(ctx context.Context, _ gen.ListCatalogItemsRequestObject) (gen.ListCatalogItemsResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapAgentApp)
	if err != nil {
		return nil, err
	}
	list, err := s.catalog.ListItems(ctx, p.OrganizationID, false)
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
	p, err := requireOrg(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	it, err := s.catalog.CreateItem(ctx, p.AuditActor(), p.OrganizationID, itemInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return gen.CreateCatalogItem201JSONResponse(apiCatalogItem(it)), nil
}

func (s *server) UpdateCatalogItem(ctx context.Context, req gen.UpdateCatalogItemRequestObject) (gen.UpdateCatalogItemResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	it, err := s.catalog.UpdateItem(ctx, p.AuditActor(), p.OrganizationID, req.CatalogItemId, itemInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return gen.UpdateCatalogItem200JSONResponse(apiCatalogItem(it)), nil
}

func (s *server) DeleteCatalogItem(ctx context.Context, req gen.DeleteCatalogItemRequestObject) (gen.DeleteCatalogItemResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	if err := s.catalog.DeleteItem(ctx, p.AuditActor(), p.OrganizationID, req.CatalogItemId); err != nil {
		return nil, err
	}
	return gen.DeleteCatalogItem204Response{}, nil
}

func itemInput(b gen.CatalogItemInput) catalog.ItemInput {
	return catalog.ItemInput{
		Name: b.Name, Description: b.Description, SortOrder: b.SortOrder, Active: b.Active,
	}
}

func apiCatalogItem(i catalog.Item) gen.CatalogItem {
	return gen.CatalogItem{
		Id: i.ID, Name: i.Name, Description: i.Description, SortOrder: i.SortOrder, Active: i.Active,
		TicketCount: i.TicketCount,
	}
}
