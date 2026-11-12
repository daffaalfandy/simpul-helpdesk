package auth

import (
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/apperr"
)

// Capability is something a role may do. Add capabilities as features land;
// the table mirrors HANDOFF §3.
type Capability string

// Capabilities.
const (
	CapAgentApp       Capability = "app.agent"        // board, tickets, internal notes
	CapPortal         Capability = "app.portal"       // client portal
	CapProjectsManage Capability = "projects.manage"  // projects & DoD settings
	CapReportsView    Capability = "reports.view"     // SLA reports
	CapUsersManage    Capability = "users.manage"     // members & roles
	CapSettingsManage Capability = "settings.manage"  // organisation, SLA, clients, integrations
	CapClientsViewAll Capability = "clients.view_all" // every client; agents see only assigned ones
)

var roleCapabilities = map[Role]map[Capability]bool{
	RoleAdmin: {
		CapAgentApp: true, CapProjectsManage: true, CapReportsView: true,
		CapUsersManage: true, CapSettingsManage: true, CapClientsViewAll: true,
	},
	RoleSupervisor: {CapAgentApp: true, CapProjectsManage: true, CapReportsView: true, CapClientsViewAll: true},
	RoleAgent:      {CapAgentApp: true},
	RoleRequester:  {CapPortal: true},
}

// Can reports whether role has capability c.
func Can(role Role, c Capability) bool {
	return roleCapabilities[role][c]
}

// Require returns the principal if it has capability c, otherwise a 401 or 403 error.
func Require(ctx context.Context, c Capability) (*Principal, error) {
	p, ok := PrincipalFrom(ctx)
	if !ok {
		return nil, apperr.Unauthenticated()
	}
	if !Can(p.Role, c) {
		return nil, apperr.Forbidden()
	}
	return p, nil
}
