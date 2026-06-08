package mdxgo

import (
	"strings"
	"testing"

	esbuild "github.com/evanw/esbuild/pkg/api"
)

func TestParseAttributes_Empty(t *testing.T) {
	attrs := ParseAttributes("")
	if len(attrs) != 0 {
		t.Errorf("expected empty map, got %v", attrs)
	}
}

func TestParseAttributes_Boolean(t *testing.T) {
	attrs := ParseAttributes("disabled")
	if attrs["disabled"] != "true" {
		t.Errorf("expected 'true', got %q", attrs["disabled"])
	}
}

func TestParseAttributes_StringDouble(t *testing.T) {
	attrs := ParseAttributes(`type="warning"`)
	if attrs["type"] != "warning" {
		t.Errorf("expected 'warning', got %q", attrs["type"])
	}
}

func TestParseAttributes_StringSingle(t *testing.T) {
	attrs := ParseAttributes(`type='error'`)
	if attrs["type"] != "error" {
		t.Errorf("expected 'error', got %q", attrs["type"])
	}
}

func TestParseAttributes_JSXExpression(t *testing.T) {
	attrs := ParseAttributes(`count={42}`)
	if attrs["count"] != "{42}" {
		t.Errorf("expected '{42}', got %q", attrs["count"])
	}
}

func TestParseAttributes_NestedBraces(t *testing.T) {
	attrs := ParseAttributes(`style={{ color: 'red', padding: 8 }}`)
	val := attrs["style"]
	if !strings.Contains(val, "color") || !strings.Contains(val, "red") {
		t.Errorf("expected nested object in style, got %q", val)
	}
}

func TestParseAttributes_ComplexLambda(t *testing.T) {
	attrs := ParseAttributes(`onClose={() => { setOpen(false); doSomething(); }}`)
	val := attrs["onClose"]
	if !strings.Contains(val, "setOpen") {
		t.Errorf("expected lambda body, got %q", val)
	}
}

func TestParseAttributes_Multiple(t *testing.T) {
	attrs := ParseAttributes(`type="warning" dismissible count={5}`)
	if attrs["type"] != "warning" {
		t.Errorf("type: expected 'warning', got %q", attrs["type"])
	}
	if attrs["dismissible"] != "true" {
		t.Errorf("dismissible: expected 'true', got %q", attrs["dismissible"])
	}
	if attrs["count"] != "{5}" {
		t.Errorf("count: expected '{5}', got %q", attrs["count"])
	}
}

func TestParseAttributes_StringWithNestedBrace(t *testing.T) {
	attrs := ParseAttributes(`label="Hello {world}"`)
	if attrs["label"] != "Hello {world}" {
		t.Errorf("expected 'Hello {world}', got %q", attrs["label"])
	}
}

func TestCompile_SimpleParagraph(t *testing.T) {
	src := `Hello, world!`
	out, err := Compile([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// esbuild's ESM formatter normalizes the default export into
	// `export { MDXContent as default }`, so assert on that marker rather than
	// the literal `export default` keyword.
	if !strings.Contains(out, "as default") {
		t.Errorf("expected ESM default export, got:\n%s", out)
	}
	if !strings.Contains(out, "MDXContent") {
		t.Errorf("expected MDXContent function, got:\n%s", out)
	}
}

func TestCompile_Heading(t *testing.T) {
	src := `# Hello MDX`
	out, err := Compile([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "h1") {
		t.Errorf("expected h1 in output, got:\n%s", out)
	}
}

func TestCompile_JSXBlock_SelfClosing(t *testing.T) {
	src := `<Alert type="warning" />`
	out, err := Compile([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v\n\nsource: %s", err, src)
	}
	if !strings.Contains(out, "Alert") {
		t.Errorf("expected 'Alert' in output, got:\n%s", out)
	}
}

func TestCompile_JSXBlock_WithChildren(t *testing.T) {
	src := `<Alert type="info">
Some content here.
</Alert>`
	out, err := Compile([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Alert") {
		t.Errorf("expected 'Alert' in output, got:\n%s", out)
	}
}

func TestCompile_TopLevelImport(t *testing.T) {
	src := `import Foo from './foo.js'

# Hello`
	out, err := Compile([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "foo.js") {
		t.Errorf("expected import statement in output, got:\n%s", out)
	}
}

func TestCompile_ExpressionInterpolation(t *testing.T) {
	src := "The year is {new Date().getFullYear()}."
	out, err := Compile([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "getFullYear") {
		t.Errorf("expected expression in output, got:\n%s", out)
	}
}

func TestCompile_ComplexDocument(t *testing.T) {
	src := `import Button from './Button.js'
export const author = "MDX Team"

# Getting Started

Welcome to **MDX**! Here is some _italic_ text.

<Button variant="primary" onClick={() => alert('hi')}>
  Click me
</Button>

## Code Example

` + "```" + `javascript
const x = 1 + 2;
console.log(x);
` + "```" + `

> This is a blockquote.

- Item one
- Item two
- Item three
`
	out, err := Compile([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v\n\nsource:\n%s", err, src)
	}

	checks := []string{
		"Button",
		"author",
		"MDXContent",
		// esbuild emits `export { author, MDXContent as default }` when another
		// named export is present, so assert on the default-export marker rather
		// than the literal `export default` keyword form.
		"as default",
		"h1",
		"h2",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Errorf("expected %q in output; got:\n%s", check, out)
		}
	}
}

func TestCompile_EmptySource(t *testing.T) {
	out, err := Compile([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error on empty source: %v", err)
	}
	if !strings.Contains(out, "MDXContent") {
		t.Errorf("expected MDXContent even for empty source, got:\n%s", out)
	}
}

func TestExtractTopLevelStatements(t *testing.T) {
	src := `import Foo from './foo.js'
export const x = 1

# Heading

Paragraph here.`

	stmts, rest := extractTopLevelStatements([]byte(src))

	if len(stmts) != 2 {
		t.Errorf("expected 2 top-level statements, got %d: %v", len(stmts), stmts)
	}
	if !strings.Contains(string(rest), "Heading") {
		t.Errorf("expected markdown body in rest, got: %s", rest)
	}
	for _, s := range stmts {
		if strings.Contains(s, "Heading") {
			t.Errorf("heading leaked into top-level statements: %v", stmts)
		}
	}
}

// fence wraps body in a fenced code block with the given info string.
func fence(lang, body string) string {
	return "```" + lang + "\n" + body + "\n```"
}

// decodeTemplateLiteral reverses escapeTemplateLiteral the way a JavaScript
// engine evaluates a template literal: a backslash escapes the character that
// follows it, which the literal emits verbatim. Because escapeTemplateLiteral
// only ever introduces backslashes in escape pairs (`\\`, "\\`", "\\${"), this
// faithfully recovers the original source bytes.
func decodeTemplateLiteral(s string) string {
	var sb strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) {
			sb.WriteRune(runes[i+1])
			i++
			continue
		}
		sb.WriteRune(runes[i])
	}
	return sb.String()
}

func TestEscapeTemplateLiteral_RoundTrip(t *testing.T) {
	cases := []string{
		"plain code",
		"return `Hello, ${name}!`;",
		`const re = /\d+\.\d+/;`,
		`a backslash: \ and a backtick: ` + "`",
		`literal ${ without closing`,
		"multi\nline\n\twith tabs and ${x} `quotes`",
		`mixed \${escaped} and ${live}`,
	}
	for _, original := range cases {
		escaped := escapeTemplateLiteral(original)
		got := decodeTemplateLiteral(escaped)
		if got != original {
			t.Errorf("round-trip mismatch:\n original: %q\n escaped:  %q\n decoded:  %q", original, escaped, got)
		}
	}
}

func TestEscapeTemplateLiteral_Order(t *testing.T) {
	// A lone backslash must be doubled first, otherwise the backslashes added
	// for backticks/${ would themselves be doubled. "\`" must become "\\`".
	if got := escapeTemplateLiteral("`"); got != "\\`" {
		t.Errorf("backtick: expected %q, got %q", "\\`", got)
	}
	if got := escapeTemplateLiteral(`\`); got != `\\` {
		t.Errorf("backslash: expected %q, got %q", `\\`, got)
	}
	if got := escapeTemplateLiteral("${"); got != "\\${" {
		t.Errorf("interpolation: expected %q, got %q", "\\${", got)
	}
	// A backslash already followed by a backtick must not collapse into a
	// single escaped backtick: `\`+"`" -> `\\\`+"`" -> "\\\\`".
	if got := escapeTemplateLiteral("\\`"); got != "\\\\\\`" {
		t.Errorf("backslash+backtick: expected %q, got %q", "\\\\\\`", got)
	}
}

func TestCompile_FencedCode_BacktickAndInterpolation(t *testing.T) {
	body := "export function greet(name: string): string {\n  return `Hello, ${name}!`;\n}"
	src := fence("ts", body)
	out, err := Compile([]byte(src))
	if err != nil {
		t.Fatalf("Compile failed on code block with backticks/${}: %v", err)
	}
	if !strings.Contains(out, "language-ts") {
		t.Errorf("expected language-ts class in output, got:\n%s", out)
	}
}

func TestCompile_FencedCode_Backslashes(t *testing.T) {
	body := `const re = /\d+\.\d+/;`
	src := fence("js", body)
	if _, err := Compile([]byte(src)); err != nil {
		t.Fatalf("Compile failed on code block with backslashes: %v", err)
	}
}

func TestCompile_FencedCode_UnclosedInterpolation(t *testing.T) {
	body := "the literal ${ should not interpolate"
	src := fence("", body)
	if _, err := Compile([]byte(src)); err != nil {
		t.Fatalf("Compile failed on code block with unclosed ${: %v", err)
	}
}

func TestCompile_IndentedCodeBlock_Specials(t *testing.T) {
	// Four-space indent produces an ast.CodeBlock rather than a fenced one.
	src := "    return `x ${y}` + '\\\\';"
	if _, err := Compile([]byte(src)); err != nil {
		t.Fatalf("Compile failed on indented code block with specials: %v", err)
	}
}

// TestCompile_FencedCode_EsbuildTransformSmoke exercises the exact path that
// failed before the fix: Compile already runs esbuild.Transform internally, so
// a clean Compile proves the JSX template literal is well-formed. We then run
// esbuild.Transform once more over a hand-built intermediate to assert the
// escaped form yields zero transform errors under Loader=JSX.
func TestCompile_FencedCode_EsbuildTransformSmoke(t *testing.T) {
	body := "export function greet(name: string): string {\n  return `Hello, ${name}!`;\n}"
	src := fence("ts", body)
	if _, err := Compile([]byte(src)); err != nil {
		t.Fatalf("Compile (which runs esbuild.Transform) failed: %v", err)
	}

	escaped := escapeTemplateLiteral(body)
	jsx := "const x = (<pre><code className=\"language-ts\">{`" + escaped + "`}</code></pre>);"
	res := esbuild.Transform(jsx, esbuild.TransformOptions{
		Loader:      esbuild.LoaderJSX,
		JSXFactory:  "React.createElement",
		JSXFragment: "React.Fragment",
	})
	if len(res.Errors) != 0 {
		t.Fatalf("esbuild.Transform reported %d errors on escaped code block: %v",
			len(res.Errors), res.Errors)
	}
}

func BenchmarkCompile(b *testing.B) {
	src := []byte(`# Hello MDX

This is a paragraph with **bold** and _italic_ text.

<Alert type="warning">
  Watch out!
</Alert>

` + "```" + `go
func main() { fmt.Println("hello") }
` + "```")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Compile(src)
		if err != nil {
			b.Fatal(err)
		}
	}
}
