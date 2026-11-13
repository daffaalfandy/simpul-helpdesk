package ticket

import (
	"regexp"
	"strconv"
	"strings"
)

// KeySeparator joins the organisation prefix and the ticket number, as in HD-24871.
const KeySeparator = "-"

// keyPattern matches a stored key; it mirrors the tickets_key_format CHECK constraint.
var keyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]{0,9}-[1-9][0-9]*$`)

// FormatKey builds the key for ticket number n.
func FormatKey(prefix string, n int64) string {
	return prefix + KeySeparator + strconv.FormatInt(n, 10)
}

// NormalizeKey turns user input such as " hd-24871" into the stored form.
func NormalizeKey(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// ValidKey reports whether s is a well-formed key. Any prefix is accepted: keys keep the
// prefix the organisation had when the ticket was created.
func ValidKey(s string) bool { return keyPattern.MatchString(s) }
