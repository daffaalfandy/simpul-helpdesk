// Package commitmsg finds ticket keys in commit messages (HANDOFF §8). It is pure: the
// connector decides which tickets a key reaches. Keys only link; no word in front of a key moves
// the ticket (owner, 17 Nov).
package commitmsg

import (
	"regexp"
	"slices"
	"strings"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

// numberPattern mirrors the ticket number in ticket.ValidKey: no leading zero.
const numberPattern = `[1-9][0-9]*`

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
	pattern := `(?i)\b((?:` + strings.Join(quoted, "|") + `)` + ticket.KeySeparator + numberPattern + `)\b`
	return &Parser{re: regexp.MustCompile(pattern)}
}

// Keys returns the keys the texts name, upper-cased, in order of first mention.
func (p *Parser) Keys(texts ...string) []string {
	if p.re == nil {
		return nil
	}
	var out []string
	for _, text := range texts {
		for _, m := range p.re.FindAllStringSubmatch(text, -1) {
			if key := ticket.NormalizeKey(m[1]); !slices.Contains(out, key) {
				out = append(out, key)
			}
		}
	}
	return out
}
