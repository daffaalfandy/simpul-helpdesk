package richtext

import (
	"fmt"
	"strconv"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/palette"
)

// Email styles. Mail clients drop stylesheets, so each block carries its own. Colours come
// from the palette that mirrors the web tokens; the last block of a quote gets no margin.
const (
	blockGap      = "margin:0 0 10px"
	lastBlockGap  = "margin:0"
	listIndent    = "padding-left:22px"
	monoFont      = "font-family:ui-monospace,Menlo,Consolas,monospace;font-size:13px"
	codeBlockBox  = "padding:10px 12px;border-radius:8px;white-space:pre-wrap"
	codeSpanBox   = "padding:1px 4px;border-radius:4px"
	quoteBorder   = "padding:0 0 0 12px;border-left:3px solid "
	headingWeight = "font-weight:800"
	preLine       = "white-space:pre-line"
	styleSep      = ";"
)

// HTML elements the email renderer writes.
const (
	tagParagraph  = "p"
	tagBullets    = "ul"
	tagNumbers    = "ol"
	tagBlockquote = "blockquote"
	tagPre        = "pre"
	tagCode       = "code"
	cssBackground = "background:"
	cssColor      = "color:"
)

// Palette colour names the email blocks use.
const (
	colorCodeBg      = "tint"
	colorQuoteBorder = "border-strong"
	colorQuoteText   = "muted"
)

var (
	codeBg      = mustHex(colorCodeBg)
	quoteRule   = mustHex(colorQuoteBorder)
	quoteText   = mustHex(colorQuoteText)
	codeSpanCSS = codeSpanBox + styleSep + monoFont + styleSep + cssBackground + codeBg
)

func mustHex(name string) string {
	h, err := palette.Hex(name)
	if err != nil {
		panic(err)
	}
	return h
}

// emailRenderer overrides goldmark's HTML for the blocks an email has to style, and for raw
// HTML, which it writes as escaped text so a typed "<b>" reads "<b>".
type emailRenderer struct{}

func (emailRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindParagraph, renderParagraph)
	reg.Register(ast.KindHeading, renderHeading)
	reg.Register(ast.KindList, renderList)
	reg.Register(ast.KindBlockquote, renderBlockquote)
	reg.Register(ast.KindCodeBlock, renderCodeBlock)
	reg.Register(ast.KindFencedCodeBlock, renderCodeBlock)
	reg.Register(ast.KindHTMLBlock, renderHTMLBlock)
	reg.Register(ast.KindCodeSpan, renderCodeSpan)
	reg.Register(ast.KindRawHTML, renderRawHTML)
}

// gap is a block's bottom margin: none for the last block of its container.
func gap(n ast.Node) string {
	if n.NextSibling() == nil {
		return lastBlockGap
	}
	return blockGap
}

func open(w util.BufWriter, tag, style string) {
	_, _ = fmt.Fprintf(w, `<%s style="%s">`, tag, style)
}

func closeTag(w util.BufWriter, tag string) {
	_, _ = fmt.Fprintf(w, `</%s>`, tag)
}

func renderParagraph(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		open(w, tagParagraph, gap(n))
	} else {
		closeTag(w, tagParagraph)
	}
	return ast.WalkContinue, nil
}

// renderHeading writes a heading as a bold paragraph: a quote in an email isn't a document.
func renderHeading(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		open(w, tagParagraph, gap(n)+styleSep+headingWeight)
	} else {
		closeTag(w, tagParagraph)
	}
	return ast.WalkContinue, nil
}

func renderList(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	list := n.(*ast.List)
	tag := tagBullets
	if list.IsOrdered() {
		tag = tagNumbers
	}
	if !entering {
		closeTag(w, tag)
		return ast.WalkContinue, nil
	}
	style := gap(n) + styleSep + listIndent
	if list.IsOrdered() && list.Start != 1 {
		_, _ = fmt.Fprintf(w, `<%s start="%s" style="%s">`, tagNumbers, strconv.Itoa(list.Start), style)
		return ast.WalkContinue, nil
	}
	open(w, tag, style)
	return ast.WalkContinue, nil
}

func renderBlockquote(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		open(w, tagBlockquote, gap(n)+styleSep+quoteBorder+quoteRule+styleSep+cssColor+quoteText)
	} else {
		closeTag(w, tagBlockquote)
	}
	return ast.WalkContinue, nil
}

func renderCodeBlock(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	open(w, tagPre, gap(n)+styleSep+codeBlockBox+styleSep+monoFont+styleSep+cssBackground+codeBg)
	_, _ = fmt.Fprintf(w, `<%s>`, tagCode)
	_, _ = w.Write(util.EscapeHTML([]byte(blockLines(n, src))))
	closeTag(w, tagCode)
	closeTag(w, tagPre)
	return ast.WalkSkipChildren, nil
}

func renderHTMLBlock(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	open(w, tagParagraph, gap(n)+styleSep+preLine)
	_, _ = w.Write(util.EscapeHTML([]byte(blockLines(n, src))))
	closeTag(w, tagParagraph)
	return ast.WalkSkipChildren, nil
}

func renderCodeSpan(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	open(w, tagCode, codeSpanCSS)
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			_, _ = w.Write(util.EscapeHTML(t.Segment.Value(src)))
		}
	}
	closeTag(w, tagCode)
	return ast.WalkSkipChildren, nil
}

func renderRawHTML(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	raw := n.(*ast.RawHTML)
	for i := range raw.Segments.Len() {
		seg := raw.Segments.At(i)
		_, _ = w.Write(util.EscapeHTML(seg.Value(src)))
	}
	return ast.WalkSkipChildren, nil
}
