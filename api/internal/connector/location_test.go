package connector

import (
	"testing"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/gitevent"
)

func TestParseLocation(t *testing.T) {
	gh, gl := gitevent.ProviderGitHub, gitevent.ProviderGitLab
	tests := []struct {
		p    gitevent.Provider
		raw  string
		want Location
		ok   bool
	}{
		{gh, "https://github.com/vendor-id/eoffice-jambi", Location{"github.com", "vendor-id/eoffice-jambi", "https://github.com/vendor-id/eoffice-jambi"}, true},
		{gh, " https://GitHub.com/vendor-id/eoffice-jambi.git/ ", Location{"github.com", "vendor-id/eoffice-jambi", "https://github.com/vendor-id/eoffice-jambi"}, true},
		{gh, "https://ghe.vendor.co.id:8443/team/web", Location{"ghe.vendor.co.id:8443", "team/web", "https://ghe.vendor.co.id:8443/team/web"}, true},
		{gh, "https://github.com/vendor-id", Location{}, false},
		{gh, "https://github.com/a/b/c", Location{}, false},
		{gl, "https://gitlab.vendor.co.id/gov/sipd-connector", Location{"gitlab.vendor.co.id", "gov/sipd-connector", "https://gitlab.vendor.co.id/gov/sipd-connector"}, true},
		{gl, "http://10.0.0.5/gov/apps/sipd/-/tree/main", Location{"10.0.0.5", "gov/apps/sipd", "http://10.0.0.5/gov/apps/sipd"}, true},
		{gl, "https://gitlab.com/solo", Location{}, false},
		{gh, "git@github.com:vendor-id/eoffice.git", Location{}, false},
		{gh, "ftp://github.com/a/b", Location{}, false},
		{gh, "https://user:token@github.com/a/b", Location{}, false},
		{gh, "https://github.com/a/b?tab=readme", Location{}, false},
		{gh, "https://github.com/a/b#x", Location{}, false},
		{gh, "https://github.com/a/b c", Location{}, false},
		{gh, "https://github.com/../b", Location{}, false},
		{gh, "github.com/a/b", Location{}, false},
		{"bitbucket", "https://bitbucket.org/a/b", Location{}, false},
	}
	for _, tt := range tests {
		got, err := ParseLocation(tt.p, tt.raw)
		if (err == nil) != tt.ok || got != tt.want {
			t.Errorf("ParseLocation(%s, %q) = %+v, %v", tt.p, tt.raw, got, err)
		}
	}
}
