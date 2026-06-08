// Command example demonstrates the mdx-go library by compiling a sample MDX
// document and printing the resulting ESM JavaScript module.
//
// Run it with:
//
//	go run ./cmd/example/
package main

import (
	"fmt"
	"os"

	mdxgo "github.com/omurilo/mdx-go"
)

var exampleMDX = `import Button from './components/Button.js'
import { useState } from 'react'

export const meta = {
  title: 'My MDX Page',
  author: 'Alice',
}

# Welcome to MDX

This is a paragraph with **bold text**, _italic text_, and ` + "`inline code`" + `.

Here's an expression: {1 + 2 + 3}.

<Button variant="primary" disabled onClick={() => console.log('clicked')}>
  Click Me
</Button>

## Features

- Markdown **and** JSX in the same file
- Full component support
- Expression interpolation: {meta.title}

` + "```" + `javascript
// Fenced code blocks work too
import { something } from 'somewhere'
const value = 42
` + "```" + `

> Blockquotes are also supported.

---

<section className="footer" style={{ padding: '1rem' }}>
  <p>Footer content with JSX attributes.</p>
</section>
`

func main() {
	fmt.Println("=== mdx-go Example ===")
	fmt.Println()
	fmt.Println("--- Input MDX ---")
	fmt.Println(exampleMDX)
	fmt.Println("--- Compiled JS Output ---")

	output, err := mdxgo.Compile([]byte(exampleMDX))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(output)
	fmt.Printf("\n=== Done (mdx-go v%s) ===\n", mdxgo.Version())
}
