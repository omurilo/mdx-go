package mdxgo

import (
	"github.com/yuin/goldmark/ast"
)

// KindJSXBlock is the [ast.NodeKind] identifying a block-level JSX element such
// as <MyComponent prop="val">...</MyComponent>.
var KindJSXBlock = ast.NewNodeKind("JSXBlock")

// KindJSXInline is the [ast.NodeKind] identifying an inline JSX element embedded
// within paragraph text.
var KindJSXInline = ast.NewNodeKind("JSXInline")

// KindJSXExpression is the [ast.NodeKind] identifying a JavaScript expression
// interpolation of the form { someValue }.
var KindJSXExpression = ast.NewNodeKind("JSXExpression")

// JSXBlock represents a block-level JSX element parsed from MDX source, for
// example:
//
//	<Alert type="warning" dismissible>
//	  Some **markdown** content here.
//	</Alert>
type JSXBlock struct {
	ast.BaseBlock

	// TagName is the element or component name, such as "Alert", "div" or
	// "MyComp".
	TagName string

	// IsSelfClosing reports whether the tag ends with /> and therefore has no
	// children and no closing tag.
	IsSelfClosing bool

	// Attrs holds every prop parsed from the opening tag. Values are raw
	// strings: string literals are stripped of their outer quotes, while JSX
	// expressions are kept verbatim so the renderer can re-emit them.
	//
	// The field is deliberately named Attrs rather than Attributes: the
	// embedded ast.BaseBlock already promotes an Attributes method from the
	// ast.Node interface, and a field named Attributes would shadow it and
	// prevent *JSXBlock from satisfying ast.Node.
	Attrs map[string]string

	// RawInner holds the block's inner content captured verbatim in source
	// order. The renderer recompiles it as a Markdown fragment, so it may hold
	// Markdown, nested JSX or a mix of both.
	RawInner []byte

	// complete reports that the whole element, including its closing tag, was
	// consumed during Open because it fit on a single line (for example
	// <summary>Title</summary>). Continue uses it to close the block straight
	// away instead of treating the following lines as inner content.
	complete bool
}

// Dump writes a human-readable representation of the node for debugging,
// satisfying the [ast.Node] contract.
func (n *JSXBlock) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"TagName":       n.TagName,
		"IsSelfClosing": boolStr(n.IsSelfClosing),
	}, nil)
}

// Kind returns [KindJSXBlock], satisfying the [ast.Node] contract.
func (n *JSXBlock) Kind() ast.NodeKind { return KindJSXBlock }

// newJSXBlock allocates a JSXBlock with the given tag name, self-closing flag
// and attribute map.
func newJSXBlock(tag string, selfClosing bool, attrs map[string]string) *JSXBlock {
	n := &JSXBlock{
		TagName:       tag,
		IsSelfClosing: selfClosing,
		Attrs:         attrs,
	}
	n.SetBlankPreviousLines(true)
	return n
}

// JSXInline represents a JSX tag that appears inline within paragraph text, for
// example the <Button> in "Click <Button onClick={handler}>here</Button> to
// continue." The entire raw tag, including children, is stored verbatim so the
// renderer can emit it unchanged.
type JSXInline struct {
	ast.BaseInline

	// RawContent is the full raw source of the inline JSX, such as
	// `<Kbd>Ctrl</Kbd>`.
	RawContent []byte
}

// Dump writes a human-readable representation of the node for debugging,
// satisfying the [ast.Node] contract.
func (n *JSXInline) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"RawContent": string(n.RawContent),
	}, nil)
}

// Kind returns [KindJSXInline], satisfying the [ast.Node] contract.
func (n *JSXInline) Kind() ast.NodeKind { return KindJSXInline }

// newJSXInline allocates a JSXInline node holding the given raw source.
func newJSXInline(raw []byte) *JSXInline {
	n := &JSXInline{RawContent: raw}
	return n
}

// JSXExpression represents a JavaScript expression wrapped in curly braces, such
// as the {2 + 2} in "The result is {2 + 2} items." It also represents top-level
// import and export statements, in which case IsTopLevel is true.
type JSXExpression struct {
	ast.BaseInline

	// Expression is the raw JS expression without the surrounding braces.
	Expression []byte

	// IsTopLevel reports whether the expression must be hoisted to module scope,
	// as with import declarations and export statements.
	IsTopLevel bool
}

// Dump writes a human-readable representation of the node for debugging,
// satisfying the [ast.Node] contract.
func (n *JSXExpression) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"Expression": string(n.Expression),
		"IsTopLevel": boolStr(n.IsTopLevel),
	}, nil)
}

// Kind returns [KindJSXExpression], satisfying the [ast.Node] contract.
func (n *JSXExpression) Kind() ast.NodeKind { return KindJSXExpression }

// newJSXExpression allocates a JSXExpression node holding the given expression
// and top-level flag.
func newJSXExpression(expr []byte, topLevel bool) *JSXExpression {
	return &JSXExpression{Expression: expr, IsTopLevel: topLevel}
}

// boolStr returns the canonical string form of b, "true" or "false".
func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
