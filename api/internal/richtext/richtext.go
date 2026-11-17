// Package richtext reads ticket descriptions and messages. They are Markdown (GitHub flavour,
// where a single newline is a line break, as plain text always was), so every plain-text body
// written before the editor is still valid. HTML renders them for email and Text turns them
// into plain text for excerpts.
//
// Both follow the web renderer's rules: raw HTML shows as the literal text that was typed,
// images show their alt text, and only http, https and mailto links stay links.
package richtext

import (
	"bytes"
	"html/template"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Link schemes that stay links; anything else keeps its text and loses the link.
var allowedSchemes = []string{"http://", "https://", "mailto:"}

// wwwPrefix is a bare web address, which GFM's autolinks turn into an http link.
const wwwPrefix = "www."

// Priorities: the sanitizer runs before any other transformer; the email renderer registers
// after goldmark's own (1000) and GFM's (500), since lower numbers register last and win.
const (
	sanitizerPriority     = 0
	emailRendererPriority = 100
)

// Plain-text layout marks.
const (
	bulletMarker  = "- "
	orderedMarker = ". "
	cellSeparator = " | "
	lineBreak     = "\n"
	blockBreak    = "\n\n"
)

var (
	sanitize = goldmark.WithParserOptions(parser.WithASTTransformers(util.Prioritized(sanitizer{}, sanitizerPriority)))
	emailMD  = goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		sanitize,
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			renderer.WithNodeRenderers(util.Prioritized(emailRenderer{}, emailRendererPriority)),
		),
	)
	parseMD    = goldmark.New(goldmark.WithExtensions(extension.GFM), sanitize)
	blankLines = regexp.MustCompile(`\n{3,}`)
	whitespace = regexp.MustCompile(`\s+`)
)

// HTML renders md for an HTML email, with inline styles only (mail clients drop stylesheets).
// It never passes raw HTML through: see emailRenderer.
func HTML(md string) template.HTML {
	var b bytes.Buffer
	if err := emailMD.Convert([]byte(md), &b); err != nil {
		return template.HTML(template.HTMLEscapeString(md)) //nolint:gosec // escaped
	}
	return template.HTML(strings.TrimSpace(b.String())) //nolint:gosec // safe renderer, see emailRenderer
}

// Text is md as plain text: formatting marks gone, a blank line between blocks, list items
// and code lines kept.
func Text(md string) string {
	src := []byte(md)
	doc := parseMD.Parser().Parse(text.NewReader(src))
	var b strings.Builder
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			writeExit(&b, n)
			return ast.WalkContinue, nil
		}
		return writeEntering(&b, n, src), nil
	})
	return strings.TrimSpace(blankLines.ReplaceAllString(b.String(), blockBreak))
}

// Line is Text on one line, for a short quote such as a notification excerpt.
func Line(md string) string {
	return strings.TrimSpace(whitespace.ReplaceAllString(Text(md), " "))
}

func writeEntering(b *strings.Builder, n ast.Node, src []byte) ast.WalkStatus {
	switch n := n.(type) {
	case *ast.Text:
		b.Write(n.Segment.Value(src))
		if n.SoftLineBreak() || n.HardLineBreak() {
			b.WriteString(lineBreak)
		}
	case *ast.String:
		b.Write(n.Value)
	case *ast.AutoLink:
		b.Write(n.Label(src))
		return ast.WalkSkipChildren
	case *ast.RawHTML:
		for i := range n.Segments.Len() {
			seg := n.Segments.At(i)
			b.Write(seg.Value(src))
		}
		return ast.WalkSkipChildren
	case *ast.CodeBlock, *ast.FencedCodeBlock, *ast.HTMLBlock:
		b.WriteString(strings.TrimRight(blockLines(n, src), lineBreak))
		return ast.WalkSkipChildren
	case *ast.ListItem:
		b.WriteString(listMarker(n))
	}
	return ast.WalkContinue
}

func writeExit(b *strings.Builder, n ast.Node) {
	switch n.(type) {
	case *ast.Paragraph, *ast.Heading, *ast.TextBlock, *ast.ThematicBreak,
		*ast.CodeBlock, *ast.FencedCodeBlock, *ast.HTMLBlock:
		if inTightList(n) {
			b.WriteString(lineBreak)
		} else {
			b.WriteString(blockBreak)
		}
	case *east.TableHeader, *east.TableRow:
		b.WriteString(lineBreak)
	case *east.TableCell:
		if n.NextSibling() != nil {
			b.WriteString(cellSeparator)
		}
	}
}

// blockLines is the source of a code or HTML block, line by line.
func blockLines(n ast.Node, src []byte) string {
	var b strings.Builder
	lines := n.Lines()
	for i := range lines.Len() {
		seg := lines.At(i)
		b.Write(seg.Value(src))
	}
	if hb, ok := n.(*ast.HTMLBlock); ok && hb.HasClosure() {
		b.Write(hb.ClosureLine.Value(src))
	}
	return b.String()
}

func listMarker(item *ast.ListItem) string {
	list, ok := item.Parent().(*ast.List)
	if !ok || !list.IsOrdered() {
		return bulletMarker
	}
	n := list.Start
	for s := item.PreviousSibling(); s != nil; s = s.PreviousSibling() {
		n++
	}
	return strconv.Itoa(n) + orderedMarker
}

func inTightList(n ast.Node) bool {
	item, ok := n.Parent().(*ast.ListItem)
	if !ok {
		return false
	}
	list, ok := item.Parent().(*ast.List)
	return ok && list.IsTight
}

// allowedURL reports whether a link may stay a link.
func allowedURL(dest []byte) bool {
	d := strings.ToLower(strings.TrimSpace(string(dest)))
	for _, s := range allowedSchemes {
		if strings.HasPrefix(d, s) {
			return true
		}
	}
	return false
}

// sanitizer rewrites the tree before anything reads it: images, and links with a scheme
// outside allowedSchemes, become their text.
type sanitizer struct{}

func (sanitizer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()
	var unwrap []ast.Node
	var unlink []*ast.AutoLink
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Link:
			if !allowedURL(n.Destination) {
				unwrap = append(unwrap, n)
			}
		case *ast.Image:
			unwrap = append(unwrap, n)
		case *ast.AutoLink:
			url := n.URL(src)
			if n.AutoLinkType == ast.AutoLinkURL && !allowedURL(url) &&
				!strings.HasPrefix(strings.ToLower(string(url)), wwwPrefix) {
				unlink = append(unlink, n)
			}
		}
		return ast.WalkContinue, nil
	})
	for _, n := range unwrap {
		parent := n.Parent()
		for c := n.FirstChild(); c != nil; {
			next := c.NextSibling()
			parent.InsertBefore(parent, n, c)
			c = next
		}
		parent.RemoveChild(parent, n)
	}
	for _, n := range unlink {
		n.Parent().ReplaceChild(n.Parent(), n, ast.NewString(n.Label(src)))
	}
}
