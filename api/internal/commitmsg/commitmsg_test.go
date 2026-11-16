package commitmsg

import (
	"slices"
	"testing"
)

func TestMessage(t *testing.T) {
	p := New([]string{"HD", "OPS"})
	tests := []struct {
		name string
		msg  string
		want []Ref
	}{
		{"no key", "update readme", nil},
		{"key only", "refs HD-24871 add logging", []Ref{{"HD-24871", ActionLink}}},
		{"fixes", "Fixes HD-24871: revert TTL", []Ref{{"HD-24871", ActionFix}}},
		{"each fix word", "fixed HD-1, fixing HD-2, close HD-3, closes HD-4, closed HD-5, closing HD-6",
			[]Ref{{"HD-1", ActionFix}, {"HD-2", ActionFix}, {"HD-3", ActionFix}, {"HD-4", ActionFix}, {"HD-5", ActionFix}, {"HD-6", ActionFix}}},
		{"resolve words", "resolve HD-1 resolves HD-2 resolved HD-3 resolving HD-4",
			[]Ref{{"HD-1", ActionFix}, {"HD-2", ActionFix}, {"HD-3", ActionFix}, {"HD-4", ActionFix}}},
		{"colon", "fix: HD-7", []Ref{{"HD-7", ActionFix}}},
		{"conventional scope", "fix(HD-7): handle empty token", []Ref{{"HD-7", ActionFix}}},
		{"keyword not next to the key", "fix: refs HD-7", []Ref{{"HD-7", ActionLink}}},
		{"keyword covers one key", "Fixes HD-1, HD-2", []Ref{{"HD-1", ActionFix}, {"HD-2", ActionLink}}},
		{"reopen", "reopen HD-24830 date bug back", []Ref{{"HD-24830", ActionReopen}}},
		{"reopens and reopened", "Reopens HD-1; reopened HD-2", []Ref{{"HD-1", ActionReopen}, {"HD-2", ActionReopen}}},
		{"case-insensitive", "CLOSES hd-12", []Ref{{"HD-12", ActionFix}}},
		{"strongest action wins", "refs HD-3 then fixes HD-3", []Ref{{"HD-3", ActionFix}}},
		{"reopen beats fix", "fixes HD-3, reopen HD-3", []Ref{{"HD-3", ActionReopen}}},
		{"fix never weakened", "fixes HD-3 and refs HD-3", []Ref{{"HD-3", ActionFix}}},
		{"other prefix", "refs OPS-9", []Ref{{"OPS-9", ActionLink}}},
		{"unknown prefix", "refs UTF-8 and ISO-8601", nil},
		{"leading zero", "refs HD-012", nil},
		{"part of a word", "xHD-1 HD-1x prefix HD-2", []Ref{{"HD-2", ActionLink}}},
		{"keyword inside a word", "unfixed HD-4", []Ref{{"HD-4", ActionLink}}},
		{"multi-line message", "Revert TTL\n\nCloses HD-9\nrefs HD-10", []Ref{{"HD-9", ActionFix}, {"HD-10", ActionLink}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.Message(tt.msg); !slices.Equal(got, tt.want) {
				t.Fatalf("Message(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

func TestKeys(t *testing.T) {
	p := New([]string{"HD"})
	tests := []struct {
		name  string
		texts []string
		want  []string
	}{
		{"branch", []string{"fix/HD-24871-sso"}, []string{"HD-24871"}},
		{"keywords ignored", []string{"Reopen HD-1"}, []string{"HD-1"}},
		{"several texts, first mention order", []string{"HD-2 title", "body names HD-1 and HD-2", "fix/hd-3"}, []string{"HD-2", "HD-1", "HD-3"}},
		{"nothing", []string{"main", ""}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.Keys(tt.texts...); !slices.Equal(got, tt.want) {
				t.Fatalf("Keys = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrefixes(t *testing.T) {
	if got := New(nil).Message("refs HD-1"); got != nil {
		t.Errorf("no prefixes matched %v", got)
	}
	if got := New([]string{"bad prefix", "1X", ""}).Message("refs HD-1"); got != nil {
		t.Errorf("malformed prefixes matched %v", got)
	}
	// A longer prefix sharing the start of a shorter one is read whole.
	p := New([]string{"HD", "HDX", " hd "})
	want := []Ref{{"HDX-5", ActionLink}, {"HD-6", ActionLink}}
	if got := p.Message("refs HDX-5 and HD-6"); !slices.Equal(got, want) {
		t.Errorf("Message = %v, want %v", got, want)
	}
}
