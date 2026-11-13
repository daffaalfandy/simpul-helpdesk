package tickets

import (
	"github.com/google/uuid"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// access is what one user may see. It fills the scope parameters of every ticket query, so
// requesters get their client's tickets and public messages, and agents their clients' tickets.
type access struct {
	scopeClientID *uuid.UUID // requesters: their client
	scopeAgentID  *uuid.UUID // agents: clients they are assigned to
	visibility    *string    // requesters: public messages only
	staff         bool       // vendor staff: internal notes, agent actions
}

var publicOnly = string(ticket.VisibilityPublic)

// accessFor decides p's ticket scope. Anything it doesn't recognise is refused.
func accessFor(p *auth.Principal) (access, error) {
	switch {
	case p == nil:
		return access{}, apperr.Unauthenticated()
	case auth.Can(p.Role, auth.CapPortal):
		if p.ClientID == nil {
			return access{}, apperr.Forbidden()
		}
		client := *p.ClientID
		return access{scopeClientID: &client, visibility: &publicOnly}, nil
	case auth.Can(p.Role, auth.CapClientsViewAll):
		return access{staff: true}, nil
	case auth.Can(p.Role, auth.CapAgentApp):
		agent := p.UserID
		return access{scopeAgentID: &agent, staff: true}, nil
	}
	return access{}, apperr.Forbidden()
}

// staffAccess is accessFor for agent-only actions: requesters get 403.
func staffAccess(p *auth.Principal) (access, error) {
	a, err := accessFor(p)
	if err != nil {
		return access{}, err
	}
	if !a.staff {
		return access{}, apperr.Forbidden()
	}
	return a, nil
}
