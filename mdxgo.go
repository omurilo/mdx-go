// Package mdxgo compiles MDX (Markdown + JSX) source into a ready-to-run ESM
// JavaScript module.
//
// MDX combines Markdown prose with JSX components and JavaScript expressions.
// mdxgo parses that source with [goldmark] extended by custom JSX block, inline,
// and expression parsers, renders it to JSX, and then transforms the JSX to plain
// ECMAScript modules with [esbuild].
//
// The typical entry point is [Compile]:
//
//	js, err := mdxgo.Compile([]byte(mdxSource))
//	if err != nil {
//		// handle compilation error
//	}
//	// js is an ESM module string whose default export, MDXContent, is a React
//	// functional component accepting { components, ...props }.
//
// [goldmark]: https://github.com/yuin/goldmark
// [esbuild]: https://github.com/evanw/esbuild
package mdxgo

import (
	"bytes"
	"fmt"
	"strings"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
)

// Compile parses an MDX source string and transpiles it to a pure ESM
// JavaScript module string ready for execution in any modern JS engine.
//
// The compilation runs in two stages: goldmark parses the Markdown together with
// the custom JSX nodes into JSX-flavoured source, then esbuild transforms that
// JSX into plain ECMAScript modules. The returned string contains a default
// export named MDXContent, a React functional component accepting
// { components, ...props }.
//
// Compile returns an error if goldmark fails to render the document or if
// esbuild reports any transform errors; the latter error includes the
// intermediate JSX source to aid debugging.
func Compile(source []byte) (string, error) {
	topLevel, mdxBody := extractTopLevelStatements(source)

	ext := NewMDXExtension()
	md := newMarkdown(ext)

	var buf bytes.Buffer
	if err := md.Convert(mdxBody, &buf); err != nil {
		return "", fmt.Errorf("mdx-go: goldmark render error: %w", err)
	}

	jsxSource := buf.String()

	hoisted := ext.Renderer.GetTopLevelStatements()
	allTopLevel := append(topLevel, hoisted...)

	hoistedBlock := strings.Join(allTopLevel, "\n")
	if hoistedBlock != "" {
		hoistedBlock += "\n"
	}

	jsxSource = strings.Replace(
		jsxSource,
		"// __MDX_TOP_LEVEL_PLACEHOLDER__\n",
		hoistedBlock,
		1,
	)

	result := esbuild.Transform(jsxSource, esbuild.TransformOptions{
		Loader:            esbuild.LoaderJSX,
		JSXFactory:        "React.createElement",
		JSXFragment:       "React.Fragment",
		Format:            esbuild.FormatESModule,
		Target:            esbuild.ES2020,
		MinifyWhitespace:  false,
		MinifyIdentifiers: false,
		MinifySyntax:      false,
	})

	if len(result.Errors) > 0 {
		var errMsgs []string
		for _, e := range result.Errors {
			loc := ""
			if e.Location != nil {
				loc = fmt.Sprintf(" (line %d, col %d): %s",
					e.Location.Line, e.Location.Column, e.Location.LineText)
			}
			errMsgs = append(errMsgs, e.Text+loc)
		}
		return "", fmt.Errorf("mdx-go: esbuild transform errors:\n%s\n\n--- intermediate JSX ---\n%s",
			strings.Join(errMsgs, "\n"), jsxSource)
	}

	return string(result.Code), nil
}

// newMarkdown builds the goldmark instance used for both whole-document and
// fragment compilation, wiring in the MDX extension alongside GFM and footnote
// support so nested content parses identically to top-level content.
func newMarkdown(ext *MDXExtension) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			ext,
			extension.GFM,
			extension.Footnote,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(),
		),
	)
}

// compileFragment renders the inner content of a block-level JSX element to a
// JSX body string, without the surrounding ESM module wrapper. The body is
// spliced as children into the enclosing element by the renderer, so code spans
// and expressions inside the block are processed by the normal renderer rather
// than emitted verbatim. Any module-level import/export statements found in the
// fragment are returned so the caller can hoist them to the document scope.
func compileFragment(source []byte) (body string, topLevel []string, err error) {
	stmts, mdxBody := extractTopLevelStatements(source)

	ext := NewMDXExtension()
	ext.Renderer.fragment = true
	md := newMarkdown(ext)

	var buf bytes.Buffer
	if err := md.Convert(mdxBody, &buf); err != nil {
		return "", nil, fmt.Errorf("mdx-go: goldmark render error in JSX block: %w", err)
	}

	hoisted := ext.Renderer.GetTopLevelStatements()
	return buf.String(), append(stmts, hoisted...), nil
}

// extractTopLevelStatements scans the leading region of an MDX source for bare
// ES module statements (import/export lines that appear before any Markdown
// content) and returns them separately from the remaining body so they can be
// hoisted to module scope in the output.
//
// Per the MDX 2 specification these statements are written without surrounding
// braces, unlike inline expression statements:
//
//	import Foo from './foo.js'
//	export const year = 2024
//
//	# Heading
//
// A statement may span multiple physical lines; continuation lines are
// accumulated until the bracket nesting depth opened on the first line returns
// to zero.
func extractTopLevelStatements(source []byte) (statements []string, rest []byte) {
	lines := bytes.Split(source, []byte("\n"))
	var bodyLines [][]byte
	inTopLevel := true

	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(string(line))

		if inTopLevel {
			if trimmed == "" {
				i++
				continue
			}

			if isTopLevelStatement([]byte(trimmed)) {
				stmtLines := []string{string(line)}
				depth := bracketDelta(string(line))
				j := i + 1
				for depth > 0 && j < len(lines) {
					stmtLines = append(stmtLines, string(lines[j]))
					depth += bracketDelta(string(lines[j]))
					j++
				}
				statements = append(statements, strings.Join(stmtLines, "\n"))
				i = j
				continue
			}

			inTopLevel = false
		}

		bodyLines = append(bodyLines, line)
		i++
	}

	rest = bytes.Join(bodyLines, []byte("\n"))
	return statements, rest
}

// bracketDelta returns the net change in nesting depth contributed by a single
// line, counting (, [ and { as +1 and ), ] and } as -1. Characters inside string
// and template literals are ignored so that braces appearing in data do not
// unbalance the multi-line statement accumulator.
func bracketDelta(s string) int {
	depth := 0
	var quote rune
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if quote != 0 {
			if r == '\\' {
				i++
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '"', '\'', '`':
			quote = r
		case '{', '(', '[':
			depth++
		case '}', ')', ']':
			depth--
		}
	}
	return depth
}

// Version returns the library version string.
func Version() string { return "0.1.0" }
