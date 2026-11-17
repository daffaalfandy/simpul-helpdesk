package richtext

import (
	"strings"
	"testing"
)

func TestText(t *testing.T) {
	tests := []struct {
		name, md, want string
	}{
		{"plain text stays", "Login fails since this morning.", "Login fails since this morning."},
		{"single newline is a line break", "Line one\nLine two", "Line one\nLine two"},
		{"paragraphs", "First.\n\nSecond.", "First.\n\nSecond."},
		{"emphasis marks go", "It is **really** _down_ ~~again~~", "It is really down again"},
		{"inline code keeps text", "Run `make seed` first", "Run make seed first"},
		{"bullets", "Steps:\n\n- open\n- click", "Steps:\n\n- open\n- click"},
		{"numbered from start", "3. third\n4. fourth", "3. third\n4. fourth"},
		{"link keeps label", "See [the log](https://example.com/log)", "See the log"},
		{"image keeps alt", "Look ![a screenshot](https://example.com/a.png) here", "Look a screenshot here"},
		{"code block lines", "```\nerror 500\nat line 2\n```", "error 500\nat line 2"},
		{"raw html literal", "Error: <html> not found", "Error: <html> not found"},
		{"heading text", "## Steps", "Steps"},
		{"quote", "> client said hi", "client said hi"},
		{"table", "| a | b |\n|---|---|\n| 1 | 2 |", "a | b\n1 | 2"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Text(tt.md); got != tt.want {
				t.Errorf("Text(%q) = %q, want %q", tt.md, got, tt.want)
			}
		})
	}
}

func TestLine(t *testing.T) {
	if got, want := Line("**Fixed** on staging.\n\n- check login\n- check upload"), "Fixed on staging. - check login - check upload"; got != want {
		t.Errorf("Line = %q, want %q", got, want)
	}
}

func TestHTML(t *testing.T) {
	tests := []struct {
		name     string
		md       string
		contains []string
		absent   []string
	}{
		{
			name: "plain text with a line break", md: "Hello\nworld",
			contains: []string{"<p", "Hello<br", "world</p>"},
		},
		{
			name: "bold and italic", md: "**bold** and *it*",
			contains: []string{"<strong>bold</strong>", "<em>it</em>"},
		},
		{
			name: "script tag shows as text", md: "<script>alert(1)</script>",
			contains: []string{"&lt;script&gt;alert(1)&lt;/script&gt;"}, absent: []string{"<script"},
		},
		{
			name: "inline html shows as text", md: "Say <b onclick=\"x()\">hi</b>",
			contains: []string{"&lt;b onclick=&quot;x()&quot;&gt;hi&lt;/b&gt;"}, absent: []string{"<b "},
		},
		{
			name: "javascript link loses the link", md: "[click](javascript:alert(1))",
			contains: []string{"click"}, absent: []string{"href", "javascript"},
		},
		{
			name: "data link loses the link", md: "[x](data:text/html;base64,AAAA)",
			absent: []string{"href", "data:"},
		},
		{
			name: "relative link loses the link", md: "[board](/board)",
			contains: []string{"board"}, absent: []string{"href"},
		},
		{
			name: "https link stays", md: "[docs](https://example.com/docs)",
			contains: []string{`<a href="https://example.com/docs">docs</a>`},
		},
		{
			name: "mailto stays", md: "[mail](mailto:it@example.com)",
			contains: []string{`href="mailto:it@example.com"`},
		},
		{
			name: "bare www link stays", md: "see www.example.com",
			contains: []string{`href="http://www.example.com"`},
		},
		{
			name: "image becomes alt text", md: "![tracking pixel](https://evil.example/p.gif)",
			contains: []string{"tracking pixel"}, absent: []string{"<img", "evil.example"},
		},
		{
			name: "code block escaped and styled", md: "```\n<b>x</b>\n```",
			contains: []string{"<pre style=", "&lt;b&gt;x&lt;/b&gt;"}, absent: []string{"<b>"},
		},
		{
			name: "inline code styled", md: "run `ls`",
			contains: []string{`<code style="`, ">ls</code>"},
		},
		{
			name: "list styled", md: "- a\n- b",
			contains: []string{`<ul style="margin:0;padding-left:22px">`, "<li>a</li>"},
		},
		{
			name: "ordered list keeps start", md: "3. c\n4. d",
			contains: []string{`<ol start="3"`},
		},
		{
			name: "only the last block loses its margin", md: "one\n\ntwo",
			contains: []string{`<p style="margin:0 0 10px">one</p>`, `<p style="margin:0">two</p>`},
		},
		{
			name: "heading is a bold paragraph", md: "# Title",
			contains: []string{"font-weight:800", "Title</p>"}, absent: []string{"<h1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(HTML(tt.md))
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("HTML(%q) = %q, missing %q", tt.md, got, want)
				}
			}
			for _, bad := range tt.absent {
				if strings.Contains(got, bad) {
					t.Errorf("HTML(%q) = %q, must not contain %q", tt.md, got, bad)
				}
			}
		})
	}
}
