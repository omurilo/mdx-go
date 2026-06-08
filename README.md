# mdx-go

A Go library that parses **MDX** (Markdown + JSX) source and transpiles it to a pure **ESM JavaScript module**, ready to be executed by any modern JS engine or bundler.

## Architecture

```
MDX source ([]byte)
        │
        ▼
┌───────────────────────────────────┐
│  extractTopLevelStatements()      │  Pre-scans bare import/export lines
│  → statements []string            │  before Goldmark sees the source
│  → body       []byte              │
└───────────────┬───────────────────┘
                │
                ▼
┌───────────────────────────────────┐
│  Goldmark Markdown Parser         │
│                                   │
│  Extensions:                      │
│  ┌─────────────────────────────┐  │
│  │ jsxBlockParser (priority 90)│  │  Captures <Tag ...>...</Tag>
│  │ jsxInlineParser (pri. 200)  │  │  Captures inline <Tag /> in text
│  │ jsxExpressionParser(pri.199)│  │  Captures { expression }
│  └─────────────────────────────┘  │
│                                   │
│  Custom AST Nodes:                │
│  • JSXBlock    (block-level JSX)  │
│  • JSXInline   (inline JSX tags)  │
│  • JSXExpression ({ ... })        │
└───────────────┬───────────────────┘
                │ AST walk
                ▼
┌───────────────────────────────────┐
│  jsxRenderer (NodeRenderer)       │
│                                   │
│  Emits JSX-flavoured source:      │
│  • import React from 'react'      │
│  • [hoisted import/export stmts]  │
│  • export default function        │
│      MDXContent({ components }) { │
│    const _c = { h1,p,... };      │
│    return (<> ... </>);           │
│  }                                │
└───────────────┬───────────────────┘
                │ JSX string
                ▼
┌───────────────────────────────────┐
│  esbuild.Transform()              │
│                                   │
│  Options:                         │
│  • Loader: JSX                    │
│  • JSXFactory: React.createElement│
│  • Format: ESModule               │
│  • Target: ES2020                 │
└───────────────┬───────────────────┘
                │
                ▼
        Plain JS ESM string
```

## File Structure

| File | Responsibility |
|------|----------------|
| `ast.go` | Custom AST node types: `JSXBlock`, `JSXInline`, `JSXExpression` |
| `attrlexer.go` | State-machine attribute lexer (handles nested `{{}}`, lambdas) |
| `parser.go` | Goldmark `BlockParser` and `InlineParser` implementations |
| `renderer.go` | Goldmark `NodeRenderer` that emits JSX source |
| `extension.go` | Goldmark `Extender` that registers all parsers + renderer |
| `mdxgo.go` | Public `Compile([]byte) (string, error)` API |

## Why a Custom Attribute Lexer?

Regex cannot correctly parse JSX attributes with nested structures:

```jsx
// These all break naive regex approaches:
style={{ color: 'red', padding: 8 }}          // nested braces
onClose={() => { setOpen(false); work(); }}   // multi-level nesting  
label="Value with {braces} inside"            // brace inside string
```

The `attrLexer` in `attrlexer.go` solves this with a character-by-character
state machine that tracks:
- Brace depth (`depth` counter)
- Whether we're inside a string literal (single/double/backtick)
- Escape sequences (`\"`, `\'`)

## Usage

```go
import mdxgo "github.com/your-org/mdx-go"

source := []byte(`
import Button from './Button.js'

# Hello MDX

<Button type="primary">Click me</Button>

The answer is {21 * 2}.
`)

js, err := mdxgo.Compile(source)
if err != nil {
    log.Fatal(err)
}
fmt.Println(js)
// Output: ESM module with React.createElement calls
```

## Output Format

The compiled output is an ESM module that:

1. Re-exports any `import`/`export` statements from the MDX source
2. Exports a default `MDXContent` React component
3. Accepts a `components` prop for overriding default HTML elements
4. Uses `React.createElement` (no JSX, ready for any bundler)

Example output for `# Hello`:

```javascript
import React from "react";
export default function MDXContent({ components, ...props }) {
  const _c = { h1: "h1", p: "p", /* ... */ ...components };
  return React.createElement(
    React.Fragment,
    null,
    React.createElement(_c.h1, null, "Hello")
  );
}
```

## Component Overriding

```jsx
// In your app:
import MDXContent from './my-page.mdx.js'
import { Heading } from './design-system'

<MDXContent components={{ h1: Heading }} />
```

## Supported MDX Features

| Feature | Status |
|---------|--------|
| CommonMark paragraphs, headings | ✅ |
| **Bold**, _italic_, `code` | ✅ |
| Fenced code blocks with language | ✅ |
| Blockquotes | ✅ |
| Ordered & unordered lists | ✅ |
| Links and images | ✅ |
| Thematic breaks (`---`) | ✅ |
| Block-level JSX `<Tag>...</Tag>` | ✅ |
| Self-closing JSX `<Tag />` | ✅ |
| Inline JSX `<Tag>` in paragraphs | ✅ |
| Expression interpolation `{expr}` | ✅ |
| Top-level `import` statements | ✅ |
| Top-level `export` statements | ✅ |
| Nested JSX attributes `prop={{}}` | ✅ |
| Lambda attributes `onClick={() => {}}` | ✅ |
| GFM tables, strikethrough | ✅ (via goldmark GFM extension) |
| MDX v2 ESM exports | ✅ |

## Dependencies

- [`github.com/yuin/goldmark`](https://github.com/yuin/goldmark) – Extensible Markdown parser
- [`github.com/evanw/esbuild`](https://github.com/evanw/esbuild) – Fast JSX→JS transformer

## License

MIT
