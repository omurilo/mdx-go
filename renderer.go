package mdxgo

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// jsxRenderer is the [renderer.NodeRenderer] that emits JSX-flavoured JavaScript
// rather than HTML. Its output is not valid HTML; it is valid JSX that esbuild
// transforms to plain JavaScript in the next pipeline stage. Top-level
// (module-scope) statements are collected separately from the JSX body so they
// can be emitted before the component function.
type jsxRenderer struct {
	topLevelStatements []string
}

// RegisterFuncs registers a rendering function for each AST node kind. goldmark
// calls it once during setup.
func (r *jsxRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindDocument, r.renderDocument)
	reg.Register(ast.KindHeading, r.renderHeading)
	reg.Register(ast.KindParagraph, r.renderParagraph)
	reg.Register(ast.KindText, r.renderText)
	reg.Register(ast.KindTextBlock, r.renderTextBlock)
	reg.Register(ast.KindEmphasis, r.renderEmphasis)
	reg.Register(ast.KindCodeSpan, r.renderCodeSpan)
	reg.Register(ast.KindFencedCodeBlock, r.renderFencedCode)
	reg.Register(ast.KindCodeBlock, r.renderCodeBlock)
	reg.Register(ast.KindBlockquote, r.renderBlockquote)
	reg.Register(ast.KindList, r.renderList)
	reg.Register(ast.KindListItem, r.renderListItem)
	reg.Register(ast.KindThematicBreak, r.renderThematicBreak)
	reg.Register(ast.KindHTMLBlock, r.renderHTMLBlock)
	reg.Register(ast.KindRawHTML, r.renderRawHTML)
	reg.Register(ast.KindImage, r.renderImage)
	reg.Register(ast.KindLink, r.renderLink)
	reg.Register(ast.KindAutoLink, r.renderAutoLink)
	reg.Register(ast.KindString, r.renderString)

	reg.Register(KindJSXBlock, r.renderJSXBlock)
	reg.Register(KindJSXInline, r.renderJSXInline)
	reg.Register(KindJSXExpression, r.renderJSXExpression)
}

// renderDocument wraps all children in an ESM module that exports a React
// component following the MDX v2 output convention:
//
//	import React from 'react';
//	// ... hoisted statements ...
//	export default function MDXContent({ components, ...props }) {
//	  const _c = { h1:'h1', p:'p', ..., ...components };
//	  return (<> ... </>);
//	}
//
// A placeholder comment is written where hoisted top-level statements belong;
// Compile replaces it after the walk with the statements collected in
// topLevelStatements.
func (r *jsxRenderer) renderDocument(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = fmt.Fprintln(w, `import React from 'react';`)
		_, _ = fmt.Fprintln(w, `// __MDX_TOP_LEVEL_PLACEHOLDER__`)
		_, _ = fmt.Fprintln(w, `export default function MDXContent({ components, ...props }) {`)
		_, _ = fmt.Fprintln(w, `  const _c = { h1:'h1', h2:'h2', h3:'h3', h4:'h4', h5:'h5', h6:'h6',`)
		_, _ = fmt.Fprintln(w, `    p:'p', a:'a', ul:'ul', ol:'ol', li:'li', blockquote:'blockquote',`)
		_, _ = fmt.Fprintln(w, `    code:'code', pre:'pre', strong:'strong', em:'em', hr:'hr',`)
		_, _ = fmt.Fprintln(w, `    img:'img', table:'table', thead:'thead', tbody:'tbody', tr:'tr',`)
		_, _ = fmt.Fprintln(w, `    th:'th', td:'td', ...components };`)
		_, _ = fmt.Fprintln(w, `  return (`)
		_, _ = fmt.Fprintln(w, `    <>`)
	} else {
		_, _ = fmt.Fprintln(w, `    </>`)
		_, _ = fmt.Fprintln(w, `  );`)
		_, _ = fmt.Fprintln(w, `}`)
	}
	return ast.WalkContinue, nil
}

// renderHeading emits a heading as a _c.hN element matching its level.
func (r *jsxRenderer) renderHeading(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Heading)
	tag := fmt.Sprintf("_c.h%d", n.Level)
	if entering {
		_, _ = fmt.Fprintf(w, "      <%s>", tag)
	} else {
		_, _ = fmt.Fprintf(w, "</%s>\n", tag)
	}
	return ast.WalkContinue, nil
}

// renderParagraph emits a paragraph as a _c.p element.
func (r *jsxRenderer) renderParagraph(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = fmt.Fprint(w, "      <_c.p>")
	} else {
		_, _ = fmt.Fprint(w, "</_c.p>\n")
	}
	return ast.WalkContinue, nil
}

// renderText emits a text node, escaping characters that are special in JSX and
// translating hard and soft line breaks.
func (r *jsxRenderer) renderText(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Text)
	segment := n.Segment
	value := source[segment.Start:segment.Stop]
	escaped := escapeJSXText(string(value))
	_, _ = w.WriteString(escaped)
	if n.HardLineBreak() {
		_, _ = w.WriteString("<br />")
	} else if n.SoftLineBreak() {
		_, _ = w.WriteString("\n      ")
	}
	return ast.WalkContinue, nil
}

// renderTextBlock emits a trailing newline after a text block's children.
func (r *jsxRenderer) renderTextBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		_, _ = fmt.Fprint(w, "\n")
	}
	return ast.WalkContinue, nil
}

// renderString emits a raw string node verbatim.
func (r *jsxRenderer) renderString(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.String)
	_, _ = w.Write(n.Value)
	return ast.WalkContinue, nil
}

// renderEmphasis emits emphasis as _c.em and strong emphasis as _c.strong.
func (r *jsxRenderer) renderEmphasis(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Emphasis)
	var tag string
	if n.Level == 1 {
		tag = "_c.em"
	} else {
		tag = "_c.strong"
	}
	if entering {
		_, _ = fmt.Fprintf(w, "<%s>", tag)
	} else {
		_, _ = fmt.Fprintf(w, "</%s>", tag)
	}
	return ast.WalkContinue, nil
}

// renderCodeSpan emits an inline code span as a _c.code element whose content is
// a quoted JS string inside a JSX expression. Emitting a string literal rather
// than JSX text avoids every text-escaping pitfall around <, {, & and quotes.
func (r *jsxRenderer) renderCodeSpan(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		var sb strings.Builder
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			switch c := child.(type) {
			case *ast.Text:
				sb.Write(source[c.Segment.Start:c.Segment.Stop])
			case *ast.String:
				sb.Write(c.Value)
			}
		}
		_, _ = fmt.Fprintf(w, "<_c.code>{%s}</_c.code>", strconv.Quote(sb.String()))
		return ast.WalkSkipChildren, nil
	}
	return ast.WalkContinue, nil
}

// renderFencedCode emits a fenced code block as a _c.pre/_c.code pair, attaching
// a language-<lang> class name when an info string supplies a language.
func (r *jsxRenderer) renderFencedCode(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.FencedCodeBlock)
	lang := ""
	if n.Info != nil {
		lang = string(n.Info.Segment.Value(source))
		lang = strings.SplitN(lang, " ", 2)[0]
	}
	lines := n.Lines()
	var sb strings.Builder
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		sb.Write(source[line.Start:line.Stop])
	}
	code := escapeTemplateLiteral(strings.TrimRight(sb.String(), "\n"))
	if lang != "" {
		_, _ = fmt.Fprintf(w, "      <_c.pre><_c.code className=%q>{`%s`}</_c.code></_c.pre>\n",
			"language-"+lang, code)
	} else {
		_, _ = fmt.Fprintf(w, "      <_c.pre><_c.code>{`%s`}</_c.code></_c.pre>\n", code)
	}
	return ast.WalkSkipChildren, nil
}

// renderCodeBlock emits an indented code block as a _c.pre/_c.code pair.
func (r *jsxRenderer) renderCodeBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	lines := node.Lines()
	var sb strings.Builder
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		sb.Write(source[line.Start:line.Stop])
	}
	code := escapeTemplateLiteral(strings.TrimRight(sb.String(), "\n"))
	_, _ = fmt.Fprintf(w, "      <_c.pre><_c.code>{`%s`}</_c.code></_c.pre>\n", code)
	return ast.WalkSkipChildren, nil
}

// renderBlockquote emits a blockquote as a _c.blockquote element.
func (r *jsxRenderer) renderBlockquote(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = fmt.Fprint(w, "      <_c.blockquote>\n")
	} else {
		_, _ = fmt.Fprint(w, "      </_c.blockquote>\n")
	}
	return ast.WalkContinue, nil
}

// renderList emits a list as _c.ol when ordered or _c.ul otherwise.
func (r *jsxRenderer) renderList(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.List)
	var tag string
	if n.IsOrdered() {
		tag = "_c.ol"
	} else {
		tag = "_c.ul"
	}
	if entering {
		_, _ = fmt.Fprintf(w, "      <%s>\n", tag)
	} else {
		_, _ = fmt.Fprintf(w, "      </%s>\n", tag)
	}
	return ast.WalkContinue, nil
}

// renderListItem emits a list item as a _c.li element.
func (r *jsxRenderer) renderListItem(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = fmt.Fprint(w, "        <_c.li>")
	} else {
		_, _ = fmt.Fprint(w, "</_c.li>\n")
	}
	return ast.WalkContinue, nil
}

// renderThematicBreak emits a thematic break as a self-closing _c.hr element.
func (r *jsxRenderer) renderThematicBreak(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = fmt.Fprint(w, "      <_c.hr />\n")
	}
	return ast.WalkContinue, nil
}

// renderLink emits a link as a _c.a element carrying its destination as href.
func (r *jsxRenderer) renderLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	if entering {
		_, _ = fmt.Fprintf(w, `<_c.a href=%q>`, string(n.Destination))
	} else {
		_, _ = fmt.Fprint(w, `</_c.a>`)
	}
	return ast.WalkContinue, nil
}

// renderAutoLink emits an autolink as a _c.a element whose text and href are both
// the detected URL.
func (r *jsxRenderer) renderAutoLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.AutoLink)
	url := string(n.URL(source))
	_, _ = fmt.Fprintf(w, `<_c.a href=%q>%s</_c.a>`, url, escapeJSXText(url))
	return ast.WalkSkipChildren, nil
}

// renderImage emits an image as a self-closing _c.img element, deriving the alt
// text from the title or, failing that, from the child text nodes.
func (r *jsxRenderer) renderImage(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*ast.Image)
	alt := string(n.Title)
	if alt == "" {
		var sb strings.Builder
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			if t, ok := child.(*ast.Text); ok {
				sb.Write(source[t.Segment.Start:t.Segment.Stop])
			}
		}
		alt = sb.String()
	}
	_, _ = fmt.Fprintf(w, `<_c.img src=%q alt=%q />`, string(n.Destination), alt)
	return ast.WalkSkipChildren, nil
}

// renderHTMLBlock emits a raw HTML block verbatim.
func (r *jsxRenderer) renderHTMLBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	lines := node.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		_, _ = w.Write(source[line.Start:line.Stop])
	}
	return ast.WalkSkipChildren, nil
}

// renderRawHTML emits raw inline HTML verbatim.
func (r *jsxRenderer) renderRawHTML(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.RawHTML)
	segs := n.Segments
	for i := 0; i < segs.Len(); i++ {
		seg := segs.At(i)
		_, _ = w.Write(source[seg.Start:seg.Stop])
	}
	return ast.WalkSkipChildren, nil
}

// renderJSXBlock emits a JSX component or element. Attributes are written in
// sorted key order for deterministic output, distinguishing boolean shorthand,
// brace expressions and string values. Any captured RawInner content is emitted
// verbatim so the downstream JSX compiler can process it.
func (r *jsxRenderer) renderJSXBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*JSXBlock)
	if entering {
		_, _ = fmt.Fprintf(w, "      <%s", n.TagName)
		keys := make([]string, 0, len(n.Attrs))
		for k := range n.Attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := n.Attrs[k]
			if v == "true" {
				_, _ = fmt.Fprintf(w, " %s", k)
			} else if strings.HasPrefix(v, "{") {
				_, _ = fmt.Fprintf(w, " %s=%s", k, v)
			} else {
				_, _ = fmt.Fprintf(w, " %s=%q", k, v)
			}
		}
		if n.IsSelfClosing {
			_, _ = fmt.Fprintf(w, " />\n")
			return ast.WalkSkipChildren, nil
		}
		_, _ = fmt.Fprint(w, ">\n")

		if len(n.RawInner) > 0 {
			_, _ = w.Write(n.RawInner)
		}
	} else {
		// goldmark still invokes the exit callback after a WalkSkipChildren on
		// enter, so guard against writing a closing tag for an element that
		// already emitted />.
		if n.IsSelfClosing {
			return ast.WalkContinue, nil
		}
		_, _ = fmt.Fprintf(w, "      </%s>\n", n.TagName)
	}
	return ast.WalkContinue, nil
}

// renderJSXInline emits an inline JSX tag verbatim, since it was captured raw
// during parsing.
func (r *jsxRenderer) renderJSXInline(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*JSXInline)
	_, _ = w.Write(n.RawContent)
	return ast.WalkSkipChildren, nil
}

// renderJSXExpression emits a JavaScript expression interpolation. Top-level
// statements are stored for hoisting and produce no in-place output; ordinary
// expressions are wrapped in braces for JSX interpolation.
func (r *jsxRenderer) renderJSXExpression(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*JSXExpression)
	if n.IsTopLevel {
		r.topLevelStatements = append(r.topLevelStatements, string(n.Expression))
		return ast.WalkSkipChildren, nil
	}
	_, _ = fmt.Fprintf(w, "{%s}", string(n.Expression))
	return ast.WalkSkipChildren, nil
}

// escapeJSXText escapes the characters that have special meaning in JSX text
// content, replacing <, >, { and } with their HTML entities so that literal
// occurrences in Markdown prose are not interpreted as tags or expressions. JSX
// decodes these entities back to the literal characters at render time. Raw
// HTML and JSX passthrough never flow through here, so escaping < and > is safe.
func escapeJSXText(s string) string {
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "{", "&#123;")
	s = strings.ReplaceAll(s, "}", "&#125;")
	return s
}

// escapeTemplateLiteral escapes the characters that are special inside a
// JavaScript template literal so that arbitrary code-block text can be embedded
// verbatim within an enclosing {`...`} expression without prematurely closing
// the literal or triggering interpolation. The order matters: backslashes are
// escaped first so the backslashes introduced for backticks and ${ are not
// themselves doubled. JSX/JS decodes these escapes back to the original bytes
// at render time, so the rendered text round-trips to the source byte-for-byte.
func escapeTemplateLiteral(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "`", "\\`")
	s = strings.ReplaceAll(s, "${", "\\${")
	return s
}

// GetTopLevelStatements returns the module-scope statements collected during the
// render walk. Compile calls it after rendering to hoist the statements into the
// final module.
func (r *jsxRenderer) GetTopLevelStatements() []string {
	return r.topLevelStatements
}
