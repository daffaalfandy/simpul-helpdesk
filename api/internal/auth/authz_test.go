package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"simdesk/internal/apperr"
)

func TestCapabilitiesMatchHandoffRoleTable(t *testing.T) {
	// HANDOFF §3: who may do what.
	want := map[Capability][]Role{
		CapAgentApp:       {RoleAdmin, RoleSupervisor, RoleAgent},
		CapPortal:         {RoleRequester},
		CapProjectsManage: {RoleAdmin, RoleSupervisor},
		CapReportsView:    {RoleAdmin, RoleSupervisor},
		CapUsersManage:    {RoleAdmin},
		CapSettingsManage: {RoleAdmin},
	}
	all := []Role{RoleAdmin, RoleSupervisor, RoleAgent, RoleRequester}
	for c, allowed := range want {
		for _, r := range all {
			expected := false
			for _, a := range allowed {
				expected = expected || a == r
			}
			if got := Can(r, c); got != expected {
				t.Errorf("Can(%s, %s) = %v, want %v", r, c, got, expected)
			}
		}
	}
}

func TestRequire(t *testing.T) {
	status := func(err error) int {
		var ae *apperr.Error
		if errors.As(err, &ae) {
			return ae.Status
		}
		return 0
	}
	if _, err := Require(context.Background(), CapUsersManage); status(err) != http.StatusUnauthorized {
		t.Fatalf("anonymous: err = %v, want 401", err)
	}
	agent := WithPrincipal(context.Background(), &Principal{Role: RoleAgent})
	if _, err := Require(agent, CapUsersManage); status(err) != http.StatusForbidden {
		t.Fatalf("agent: err = %v, want 403", err)
	}
	admin := WithPrincipal(context.Background(), &Principal{Role: RoleAdmin})
	if p, err := Require(admin, CapUsersManage); err != nil || p.Role != RoleAdmin {
		t.Fatalf("admin: p=%v err=%v", p, err)
	}
}

func TestRoleValid(t *testing.T) {
	if !RoleRequester.Valid() || Role("owner").Valid() {
		t.Fatal("Role.Valid is wrong")
	}
}
