package richtext

import (
	"strings"
	"testing"
)

func TestTooComplex(t *testing.T) {
	wideHeader := strings.Repeat("|a", 500) + "|\n" + strings.Repeat("|-", 500) + "|\n"
	tests := []struct {
		name string
		md   string
		want bool
	}{
		{"plain text", "Login fails since this morning.", false},
		{"small table", "| a | b |\n|---|---|\n| 1 | 2 |", false},
		{"table rows up to the limit", "|a|b|\n|-|-|\n" + strings.Repeat("|1|2|\n", MaxTableCells/2-1), false},
		{"table rows past the limit", "|a|b|\n|-|-|\n" + strings.Repeat("|1|2|\n", MaxTableCells/2), true},
		{"short rows still fill to the header", wideHeader + strings.Repeat("|\n", MaxTableCells/500), true},
		{"a blank line ends the table", wideHeader + "\n" + strings.Repeat("text\n", MaxTableCells), false},
		{"tables add up", strings.Repeat("|a|b|\n|-|-|\n"+strings.Repeat("|1|2|\n", 999)+"\n", 3), true},
		{"aligned delimiter", "| a |\n| :-: |\n| 1 |", false},
		{"thematic break is not a table", strings.Repeat("---\n", MaxTableCells+1), false},
		{"quotes up to the limit", strings.Repeat(">", MaxNestingDepth) + " deep", false},
		{"quotes past the limit", strings.Repeat(">", MaxNestingDepth+1) + " deep", true},
		{"spaced quotes", strings.Repeat("> ", MaxNestingDepth+1) + "deep", true},
		{"bullets past the limit", strings.Repeat("- ", MaxNestingDepth+1) + "deep", true},
		{"numbers past the limit", strings.Repeat("1. ", MaxNestingDepth+1) + "deep", true},
		{"mixed markers", strings.Repeat("> * 2) ", MaxNestingDepth/3+1) + "deep", true},
		{"dashes without spaces", strings.Repeat("-", 1000), false},
		{"a long number is not a marker", "1234567890. text", false},
		{"emphasis is not nesting", strings.Repeat("*", 1000), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TooComplex(tt.md); got != tt.want {
				t.Errorf("TooComplex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTooComplexRendersAsText(t *testing.T) {
	md := strings.Repeat(">", MaxNestingDepth+1) + " <b>deep</b>\nnext"
	if got, want := string(HTML(md)), strings.Repeat("&gt;", MaxNestingDepth+1)+" &lt;b&gt;deep&lt;/b&gt;<br>next"; got != want {
		t.Errorf("HTML() = %q, want %q", got, want)
	}
	if got := Text(md); got != md {
		t.Errorf("Text() = %q, want the source", got)
	}
}
