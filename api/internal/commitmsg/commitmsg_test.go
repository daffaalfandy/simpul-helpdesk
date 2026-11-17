package commitmsg

import (
	"slices"
	"testing"
)

func TestKeys(t *testing.T) {
	p := New([]string{"HD", "OPS"})
	tests := []struct {
		name  string
		texts []string
		want  []string
	}{
		{"no key", []string{"update readme"}, nil},
		{"key only", []string{"refs HD-24871 add logging"}, []string{"HD-24871"}},
		// Keywords no longer mean anything: the key just links.
		{"fix and reopen words only link", []string{"Fixes HD-1; reopen HD-2"}, []string{"HD-1", "HD-2"}},
		{"conventional scope", []string{"fix(HD-7): handle empty token"}, []string{"HD-7"}},
		{"case-insensitive", []string{"CLOSES hd-12"}, []string{"HD-12"}},
		{"named twice", []string{"refs HD-3 then HD-3 again"}, []string{"HD-3"}},
		{"other prefix", []string{"refs OPS-9"}, []string{"OPS-9"}},
		{"unknown prefix", []string{"refs UTF-8 and ISO-8601"}, nil},
		{"leading zero", []string{"refs HD-012"}, nil},
		{"part of a word", []string{"xHD-1 HD-1x prefix HD-2"}, []string{"HD-2"}},
		{"multi-line message", []string{"Revert TTL\n\nCloses HD-9\nrefs HD-10"}, []string{"HD-9", "HD-10"}},
		{"several texts, first mention order", []string{"HD-2 title", "body names HD-1 and HD-2", "fix/hd-3"}, []string{"HD-2", "HD-1", "HD-3"}},
		{"nothing", []string{"main", ""}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.Keys(tt.texts...); !slices.Equal(got, tt.want) {
				t.Fatalf("Keys(%q) = %v, want %v", tt.texts, got, tt.want)
			}
		})
	}
}

func TestPrefixes(t *testing.T) {
	if got := New(nil).Keys("refs HD-1"); got != nil {
		t.Errorf("no prefixes matched %v", got)
	}
	if got := New([]string{"bad prefix", "1X", ""}).Keys("refs HD-1"); got != nil {
		t.Errorf("malformed prefixes matched %v", got)
	}
	// A longer prefix sharing the start of a shorter one is read whole.
	p := New([]string{"HD", "HDX", " hd "})
	if got, want := p.Keys("refs HDX-5 and HD-6"), []string{"HDX-5", "HD-6"}; !slices.Equal(got, want) {
		t.Errorf("Keys = %v, want %v", got, want)
	}
}
