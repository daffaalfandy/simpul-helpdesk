// Package commitmsg finds ticket keys in commit messages, pull/merge request text and branch
// names, and the keyword in front of each key (HANDOFF §8). It is pure: the connector decides
// what each reference does to a ticket.
package commitmsg

import (
	"regexp"
	"slices"
	"strings"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// Action is what a reference asks for. Values match the CommitRefAction enum in the API.
type Action string

const (
	ActionLink   Action = "link"   // the key alone: link the commit to the ticket
	ActionFix    Action = "fix"    // fix/close/resolve + key: the fix is committed
	ActionReopen Action = "reopen" // reopen + key: the problem is back
)

// strength orders the actions when one text names a key twice: the stronger one wins.
var strength = map[Action]int{ActionLink: 0, ActionFix: 1, ActionReopen: 2}

// Keywords follow GitHub's and GitLab's closing words, so developers don't learn new ones.
var (
	fixWords    = []string{"fix", "fixes", "fixed", "fixing", "close", "closes", "closed", "closing", "resolve", "resolves", "resolved", "resolving"}
	reopenWords = []string{"reopen", "reopens", "reopened"}
)

// keywordSeparators may sit between a keyword and its key: "Fixes HD-1", "fix: HD-1", "fix(HD-1)".
const keywordSeparators = `[\s:(]+`

// numberPattern mirrors the ticket number in ticket.ValidKey: no leading zero.
const numberPattern = `[1-9][0-9]*`

// Ref is one ticket key found in a text and what the text asks for.
type Ref struct {
	Key    string
	Action Action
}

// Parser finds the keys of a set of ticket prefixes. Build one per batch of texts: the prefixes
// are the organisation's current one plus every prefix already stored on a ticket.
type Parser struct {
	re *regexp.Regexp // nil when there is no prefix: nothing matches
}

// New builds a parser for the given prefixes. Malformed prefixes are skipped.
func New(prefixes []string) *Parser {
	var valid []string
	for _, p := range prefixes {
		p = ticket.NormalizeKey(p)
		if ticket.ValidKey(ticket.FormatKey(p, 1)) && !slices.Contains(valid, p) {
			valid = append(valid, p)
		}
	}
	if len(valid) == 0 {
		return &Parser{}
	}
	// Longer prefixes first, so HDX-1 is never read as HD followed by junk.
	slices.SortFunc(valid, func(a, b string) int { return len(b) - len(a) })
	quoted := make([]string, len(valid))
	for i, p := range valid {
		quoted[i] = regexp.QuoteMeta(p)
	}
	keywords := strings.Join(append(slices.Clone(fixWords), reopenWords...), "|")
	pattern := `(?i)(?:\b(` + keywords + `)` + keywordSeparators + `)?\b((?:` + strings.Join(quoted, "|") + `)` +
		ticket.KeySeparator + numberPattern + `)\b`
	return &Parser{re: regexp.MustCompile(pattern)}
}

// Message returns the references in a commit message, one per key in order of first mention.
// A keyword counts only right before its key, as on GitHub: "Fixes HD-1, HD-2" fixes HD-1 and
// links HD-2. A key named twice keeps its strongest action (reopen, then fix, then link).
func (p *Parser) Message(text string) []Ref {
	if p.re == nil {
		return nil
	}
	var out []Ref
	for _, m := range p.re.FindAllStringSubmatch(text, -1) {
		ref := Ref{Key: ticket.NormalizeKey(m[2]), Action: actionOf(m[1])}
		i := slices.IndexFunc(out, func(r Ref) bool { return r.Key == ref.Key })
		switch {
		case i < 0:
			out = append(out, ref)
		case strength[ref.Action] > strength[out[i].Action]:
			out[i].Action = ref.Action
		}
	}
	return out
}

// Keys returns the keys a text names, in order of first mention, ignoring keywords. Branch
// names and pull/merge request text only link.
func (p *Parser) Keys(texts ...string) []string {
	var out []string
	for _, text := range texts {
		for _, r := range p.Message(text) {
			if !slices.Contains(out, r.Key) {
				out = append(out, r.Key)
			}
		}
	}
	return out
}

func actionOf(keyword string) Action {
	switch k := strings.ToLower(keyword); {
	case slices.Contains(reopenWords, k):
		return ActionReopen
	case slices.Contains(fixWords, k):
		return ActionFix
	}
	return ActionLink
}
