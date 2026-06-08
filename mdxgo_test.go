package mdxgo

import (
	"strings"
	"testing"
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
