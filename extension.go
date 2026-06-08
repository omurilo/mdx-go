package mdxgo

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// MDXExtension is the [goldmark.Extender] that adds MDX support. It registers the
// JSX block, inline and expression parsers together with the JSX renderer. The
// renderer is exposed so that [Compile] can retrieve the hoisted top-level
// statements collected during the render walk.
type MDXExtension struct {
	// Renderer is the JSX renderer shared between parsing and compilation.
	Renderer *jsxRenderer
}

// NewMDXExtension creates an MDXExtension along with its shared renderer
// instance.
func NewMDXExtension() *MDXExtension {
	return &MDXExtension{
		Renderer: &jsxRenderer{},
	}
}

// Extend registers the MDX block parser, inline parsers and renderer with m,
// satisfying the [goldmark.Extender] interface.
//
// The block parser is given priority 90 so it runs ahead of goldmark's default
// HTML block parser (priority 70). The inline parsers sit above goldmark's
// defaults. The renderer is registered at priority 100: goldmark applies node
// renderers in reverse priority order, so a number below the default HTML
// renderer's 1000 ensures every kind registered here overrides it.
func (e *MDXExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithBlockParsers(
			util.Prioritized(defaultJSXBlockParser, 90),
		),
	)

	m.Parser().AddOptions(
		parser.WithInlineParsers(
			util.Prioritized(defaultJSXInlineParser, 200),
			util.Prioritized(defaultJSXExpressionParser, 199),
		),
	)

	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(e.Renderer, 100),
		),
	)
}
