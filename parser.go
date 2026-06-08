package mdxgo

import (
	"bytes"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// jsxBlockParser is the [parser.BlockParser] for block-level JSX elements. It
// triggers on lines beginning with '<' followed by a tag name, covering both
// components and HTML elements used as block-level JSX.
type jsxBlockParser struct{}

var defaultJSXBlockParser = &jsxBlockParser{}

// Trigger reports the first bytes that invoke this parser, namely '<'.
func (p *jsxBlockParser) Trigger() []byte { return []byte{'<'} }

// Priority returns 90 so the parser runs ahead of goldmark's built-in HTML block
// parser, which has priority 70.
func (p *jsxBlockParser) Priority() int { return 90 }

// Open begins a potential JSX block. It returns the new node together with
// parser.Continue|parser.HasChildren when the line opens a JSX element, or
// (nil, parser.NoChildren) to pass control to the next parser. Self-closing tags
// are completed immediately; otherwise the expected closing tag and nesting
// state are stored in the parser context for Continue to consume.
func (p *jsxBlockParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, segment := reader.PeekLine()

	indent := 0
	for indent < len(line) && indent < 4 && line[indent] == ' ' {
		indent++
	}
	if indent >= len(line) || line[indent] != '<' {
		return nil, parser.NoChildren
	}

	if bytes.HasPrefix(line[indent:], []byte("<!--")) {
		return nil, parser.NoChildren
	}

	tagName, attrs, selfClosing, tagEndOffset := parseOpeningTag(line[indent+1:])
	if tagName == "" {
		return nil, parser.NoChildren
	}

	node := newJSXBlock(tagName, selfClosing, attrs)

	if selfClosing {
		reader.AdvanceLine()
		return node, parser.NoChildren
	}

	closingTag := []byte("</" + tagName + ">")
	if inner, _, found := bytes.Cut(line[indent+1+tagEndOffset:], closingTag); found {
		node.RawInner = append([]byte{}, inner...)
		node.complete = true
		reader.AdvanceLine()
		return node, parser.NoChildren
	}

	reader.Advance(segment.Len())

	pc.Set(contextKeyClosingTag, closingTag)
	pc.Set(contextKeyNestDepth, 1)
	pc.Set(contextKeyTagName, tagName)
	pc.Set(contextKeyRawInner, []byte{})

	return node, parser.NoChildren
}

// Continue processes a subsequent line of an open JSX block. It tracks nesting of
// identically named tags, accumulates inner content verbatim, and returns
// parser.Close once the matching closing tag is reached at depth zero.
func (p *jsxBlockParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	jsxNode, ok := node.(*JSXBlock)
	if !ok {
		return parser.Close
	}

	if jsxNode.IsSelfClosing || jsxNode.complete {
		return parser.Close
	}

	closingTag, _ := pc.Get(contextKeyClosingTag).([]byte)
	tagName, _ := pc.Get(contextKeyTagName).(string)
	depthVal, _ := pc.Get(contextKeyNestDepth).(int)
	rawInner, _ := pc.Get(contextKeyRawInner).([]byte)

	line, _ := reader.PeekLine()

	openingPrefix := []byte("<" + tagName)
	if bytes.Contains(line, openingPrefix) {
		depthVal++
		pc.Set(contextKeyNestDepth, depthVal)
	}

	if bytes.Contains(line, closingTag) {
		depthVal--
		pc.Set(contextKeyNestDepth, depthVal)
		if depthVal <= 0 {
			jsxNode.RawInner = rawInner
			reader.Advance(len(line))
			return parser.Close
		}
	}

	rawInner = append(rawInner, line...)
	pc.Set(contextKeyRawInner, rawInner)
	reader.Advance(len(line))

	return parser.Continue | parser.NoChildren
}

// Close finalises the block when Continue returns parser.Close or at EOF,
// ensuring any accumulated inner content is stored on the node.
func (p *jsxBlockParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	if jsxNode, ok := node.(*JSXBlock); ok {
		if ri, ok2 := pc.Get(contextKeyRawInner).([]byte); ok2 && jsxNode.RawInner == nil {
			jsxNode.RawInner = ri
		}
	}
}

// CanInterruptParagraph reports that a JSX block may begin within a paragraph's
// text flow.
func (p *jsxBlockParser) CanInterruptParagraph() bool { return true }

// CanAcceptIndentedLine reports that JSX blocks must start at column zero and
// therefore cannot accept an indented line.
func (p *jsxBlockParser) CanAcceptIndentedLine() bool { return false }

// Context keys used to carry JSX block state across Open, Continue and Close.
// goldmark requires each key to be a unique parser.ContextKey allocated via
// parser.NewContextKey.
var (
	contextKeyClosingTag = parser.NewContextKey()
	contextKeyNestDepth  = parser.NewContextKey()
	contextKeyTagName    = parser.NewContextKey()
	contextKeyRawInner   = parser.NewContextKey()
)

// parseOpeningTag parses the bytes following the initial '<' of an opening tag.
// It returns the element or component name (empty on failure), the parsed
// attribute map, whether the tag is self-closing, and the number of bytes
// consumed relative to data. Quoted strings and nested braces are skipped while
// searching for the closing '>' or '/>' so that attribute values containing
// those characters do not terminate the tag prematurely. If no closing bracket
// is found on the line, the tag is treated as an opening tag to be completed
// later.
func parseOpeningTag(data []byte) (tagName string, attrs map[string]string, selfClosing bool, endOffset int) {
	s := string(data)
	runes := []rune(s)
	pos := 0
	size := len(runes)

	if pos >= size || (!unicode.IsLetter(runes[pos]) && runes[pos] != '_') {
		return
	}
	nameStart := pos
	for pos < size && (unicode.IsLetter(runes[pos]) || unicode.IsDigit(runes[pos]) || runes[pos] == '-' || runes[pos] == '_' || runes[pos] == '.') {
		pos++
	}
	tagName = string(runes[nameStart:pos])

	attrStart := pos
	depth := 0
	for pos < size {
		r := runes[pos]
		switch {
		case r == '{':
			depth++
			pos++
		case r == '}':
			depth--
			pos++
		case (r == '"' || r == '\'') && depth == 0:
			q := r
			pos++
			for pos < size && runes[pos] != q {
				if runes[pos] == '\\' {
					pos++
				}
				pos++
			}
			pos++
		case r == '/' && pos+1 < size && runes[pos+1] == '>' && depth == 0:
			attrStr := string(runes[attrStart:pos])
			attrs = ParseAttributes(strings.TrimSpace(attrStr))
			selfClosing = true
			endOffset = len(string(runes[:pos+2]))
			return
		case r == '>' && depth == 0:
			attrStr := string(runes[attrStart:pos])
			attrs = ParseAttributes(strings.TrimSpace(attrStr))
			selfClosing = false
			endOffset = len(string(runes[:pos+1]))
			return
		default:
			pos++
		}
	}

	attrStr := string(runes[attrStart:pos])
	attrs = ParseAttributes(strings.TrimSpace(attrStr))
	endOffset = len(string(runes[:pos]))
	return
}

// jsxExpressionParser is the [parser.InlineParser] that captures a JavaScript
// expression interpolation, { expression }, anywhere within Markdown text.
type jsxExpressionParser struct{}

var defaultJSXExpressionParser = &jsxExpressionParser{}

// Trigger reports the first bytes that invoke this parser, namely '{'.
func (p *jsxExpressionParser) Trigger() []byte { return []byte{'{'} }

// Parse reads a brace expression starting at the current position. It tracks
// brace depth and skips string literals so that braces inside strings do not end
// the expression early, and returns nil when no matching closing brace is found
// on the line. Expressions recognised as top-level ES module statements are
// flagged for hoisting to module scope.
func (p *jsxExpressionParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()

	if len(line) == 0 || line[0] != '{' {
		return nil
	}

	depth := 0
	inString := rune(0)
	runes := []rune(string(line))
	end := -1

	for i, r := range runes {
		switch {
		case inString != 0:
			if r == inString && (i == 0 || runes[i-1] != '\\') {
				inString = 0
			}
		case r == '"' || r == '\'' || r == '`':
			inString = r
		case r == '{':
			depth++
		case r == '}':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}

	if end < 0 {
		return nil
	}

	expr := []byte(string(runes[1:end]))
	exprTrimmed := bytes.TrimSpace(expr)

	topLevel := isTopLevelStatement(exprTrimmed)

	node := newJSXExpression(exprTrimmed, topLevel)

	consumed := len([]byte(string(runes[:end+1])))
	block.Advance(consumed)

	return node
}

// isTopLevelStatement reports whether b is an ES module-level statement (an
// import or export) that must live at module scope rather than inside the
// component function body.
func isTopLevelStatement(b []byte) bool {
	s := strings.TrimSpace(string(b))
	return strings.HasPrefix(s, "import ") ||
		strings.HasPrefix(s, "export ") ||
		strings.HasPrefix(s, "import{") ||
		strings.HasPrefix(s, "export{")
}

// jsxInlineParser is the [parser.InlineParser] that captures inline JSX tags,
// such as <Button>click</Button> or <Icon />, appearing mid-sentence within a
// Markdown paragraph.
type jsxInlineParser struct{}

var defaultJSXInlineParser = &jsxInlineParser{}

// Trigger reports the first bytes that invoke this parser, namely '<'.
func (p *jsxInlineParser) Trigger() []byte { return []byte{'<'} }

// Parse reads an inline JSX tag starting at the current position. The tag is only
// recognised when '<' is followed by a letter or underscore, which avoids
// conflicting with '<' used as a comparison operator or with raw HTML handled by
// goldmark. The whole tag, including any children up to its closing tag, is
// captured verbatim. Multi-line inline JSX is unsupported and causes Parse to
// return nil.
//
// Indices are kept in bytes throughout because parseOpeningTag returns a byte
// offset and text.Reader.Advance expects a byte count; mixing rune indices with
// byte offsets would corrupt slice boundaries on multi-byte input.
func (p *jsxInlineParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()

	if len(line) < 2 || line[0] != '<' {
		return nil
	}

	if !unicode.IsLetter(rune(line[1])) && line[1] != '_' {
		return nil
	}

	tagName, _, selfClosing, tagEnd := parseOpeningTag(line[1:])
	if tagName == "" {
		return nil
	}

	var rawEnd int
	if selfClosing {
		rawEnd = 1 + tagEnd
	} else {
		closingTag := []byte("</" + tagName + ">")
		rest := line[1+tagEnd:]
		idx := bytes.Index(rest, closingTag)
		if idx < 0 {
			return nil
		}
		rawEnd = 1 + tagEnd + idx + len(closingTag)
	}

	rawBytes := make([]byte, rawEnd)
	copy(rawBytes, line[:rawEnd])
	node := newJSXInline(rawBytes)

	block.Advance(rawEnd)

	return node
}

var _ = util.PrioritizedValue{}
