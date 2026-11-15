package httpapi

import (
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/tickets"
)

func (s *server) ListTicketWatchers(ctx context.Context, req gen.ListTicketWatchersRequestObject) (gen.ListTicketWatchersResponseObject, error) {
	p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	ws, err := s.tickets.Watchers(ctx, p, req.TicketKey)
	if err != nil {
		return nil, err
	}
	return gen.ListTicketWatchers200JSONResponse(apiColleagues(ws)), nil
}

func (s *server) ListTicketWatcherCandidates(ctx context.Context, req gen.ListTicketWatcherCandidatesRequestObject) (gen.ListTicketWatcherCandidatesResponseObject, error) {
	p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	var q string
	if req.Params.Q != nil {
		q = *req.Params.Q
	}
	cs, err := s.tickets.WatcherCandidates(ctx, p, req.TicketKey, q)
	if err != nil {
		return nil, err
	}
	return gen.ListTicketWatcherCandidates200JSONResponse(apiColleagues(cs)), nil
}

func (s *server) AddTicketWatcher(ctx context.Context, req gen.AddTicketWatcherRequestObject) (gen.AddTicketWatcherResponseObject, error) {
	p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	ws, err := s.tickets.AddWatcher(ctx, p, req.TicketKey, req.Body.UserId)
	if err != nil {
		return nil, err
	}
	return gen.AddTicketWatcher200JSONResponse(apiColleagues(ws)), nil
}

func (s *server) RemoveTicketWatcher(ctx context.Context, req gen.RemoveTicketWatcherRequestObject) (gen.RemoveTicketWatcherResponseObject, error) {
	p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.tickets.RemoveWatcher(ctx, p, req.TicketKey, req.UserId); err != nil {
		return nil, err
	}
	return gen.RemoveTicketWatcher204Response{}, nil
}

func apiColleagues(cs []tickets.Colleague) []gen.Colleague {
	out := make([]gen.Colleague, len(cs))
	for i, c := range cs {
		out[i] = gen.Colleague{Id: c.ID, Name: c.Name, JobTitle: c.JobTitle}
	}
	return out
}
