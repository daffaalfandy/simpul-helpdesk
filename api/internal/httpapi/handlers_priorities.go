package httpapi

import (
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/priorities"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
)

// ListPriorityLevels is open to every signed-in user: requesters see the names when they report.
func (s *server) ListPriorityLevels(ctx context.Context, _ gen.ListPriorityLevelsRequestObject) (gen.ListPriorityLevelsResponseObject, error) {
	if _, err := signedIn(ctx); err != nil {
		return nil, err
	}
	levels, err := s.priorities.List(ctx)
	if err != nil {
		return nil, err
	}
	return gen.ListPriorityLevels200JSONResponse(apiPriorityLevels(levels)), nil
}

func (s *server) UpdatePriorityLevels(ctx context.Context, req gen.UpdatePriorityLevelsRequestObject) (gen.UpdatePriorityLevelsResponseObject, error) {
	p, err := auth.Require(ctx, auth.CapSettingsManage)
	if err != nil {
		return nil, err
	}
	in := make([]priorities.Level, len(*req.Body))
	for i, l := range *req.Body {
		in[i] = priorities.Level{Code: sla.Priority(l.Code), Name: l.Name, Description: l.Description}
	}
	levels, err := s.priorities.Update(ctx, p.AuditActor(), in)
	if err != nil {
		return nil, err
	}
	return gen.UpdatePriorityLevels200JSONResponse(apiPriorityLevels(levels)), nil
}

func apiPriorityLevels(levels []priorities.Level) []gen.PriorityLevel {
	out := make([]gen.PriorityLevel, len(levels))
	for i, l := range levels {
		out[i] = gen.PriorityLevel{Code: gen.Priority(l.Code), Name: l.Name, Description: l.Description}
	}
	return out
}
