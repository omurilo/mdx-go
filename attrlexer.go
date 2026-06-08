package mdxgo

import (
	"strings"
	"unicode"
)

// attrLexer holds the scanning state for a single JSX attribute-list string. The
// attribute list is the portion of an opening tag after the tag name; for
// `<Alert type="warning" onClose={() => setOpen(false)} dismissible>` the input
// is `type="warning" onClose={() => setOpen(false)} dismissible`.
//
// A rune-by-rune scanner with an explicit depth counter is used instead of a
// regular expression because attribute values may contain nested structures such
// as style={{ color: 'red' }} or onClick={(e) => { ... }} that a regex cannot
// reliably delimit.
type attrLexer struct {
	src  []rune
	pos  int
	size int
}

// newAttrLexer creates a lexer for the given attribute string.
func newAttrLexer(s string) *attrLexer {
	r := []rune(s)
	return &attrLexer{src: r, size: len(r)}
}

// peek returns the rune at the current position without advancing, or 0 when the
// cursor is past the end of the input.
func (l *attrLexer) peek() rune {
	if l.pos >= l.size {
		return 0
	}
	return l.src[l.pos]
}

// next consumes and returns the current rune, advancing the cursor by one, or
// returns 0 when the cursor is past the end of the input.
func (l *attrLexer) next() rune {
	if l.pos >= l.size {
		return 0
	}
	r := l.src[l.pos]
	l.pos++
	return r
}

// skipWS advances the cursor past any whitespace.
func (l *attrLexer) skipWS() {
	for l.pos < l.size && unicode.IsSpace(l.peek()) {
		l.pos++
	}
}

// scanIdent reads a JSX attribute name and returns it. Names may contain
// letters, digits, hyphens, underscores, dots and colons, covering forms such as
// className, aria-label, data-id and on:click.
func (l *attrLexer) scanIdent() string {
	var b strings.Builder
	for l.pos < l.size {
		r := l.peek()
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' || r == ':' {
			b.WriteRune(l.next())
		} else {
			break
		}
	}
	return b.String()
}

// scanQuotedString reads a string value delimited by quote, honouring backslash
// escapes, and returns its content without the surrounding quotes.
func (l *attrLexer) scanQuotedString(quote rune) string {
	var b strings.Builder
	for l.pos < l.size {
		r := l.next()
		if r == '\\' && l.pos < l.size {
			b.WriteRune(r)
			b.WriteRune(l.next())
			continue
		}
		if r == quote {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

// scanBraceExpression reads a JSX expression value, assuming the caller has
// already consumed the opening brace. It tracks brace depth to find the matching
// closing brace and skips over string literals so that braces inside them do not
// affect the count. The returned string is the expression content without the
// outer braces.
func (l *attrLexer) scanBraceExpression() string {
	var b strings.Builder
	depth := 1

	for l.pos < l.size && depth > 0 {
		r := l.next()
		switch r {
		case '{':
			depth++
			b.WriteRune(r)
		case '}':
			depth--
			if depth > 0 {
				b.WriteRune(r)
			}
		case '"', '\'', '`':
			b.WriteRune(r)
			b.WriteString(l.scanQuotedString(r))
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ParseAttributes lexes a complete JSX attribute-list string and returns a map
// of attribute name to value. It recognises boolean shorthand, single- and
// double-quoted strings, and brace expressions:
//
//	disabled                        → {"disabled": "true"}
//	type="warning"                  → {"type": "warning"}
//	type='warning'                  → {"type": "warning"}
//	count={42}                      → {"count": "{42}"}
//	style={{ color: 'red' }}        → {"style": "{{ color: 'red' }}"}
//	onClose={() => setOpen(false)}  → {"onClose": "{() => setOpen(false)}"}
//
// Brace-expression values retain their outer braces so the renderer can re-emit
// them as valid JSX.
func ParseAttributes(attrStr string) map[string]string {
	l := newAttrLexer(strings.TrimSpace(attrStr))
	attrs := make(map[string]string)

	for {
		l.skipWS()
		if l.pos >= l.size {
			break
		}

		name := l.scanIdent()
		if name == "" {
			l.next()
			continue
		}

		l.skipWS()

		if l.pos >= l.size || l.peek() != '=' {
			attrs[name] = "true"
			continue
		}
		l.next()
		l.skipWS()

		if l.pos >= l.size {
			attrs[name] = "true"
			break
		}

		switch l.peek() {
		case '"':
			l.next()
			attrs[name] = l.scanQuotedString('"')

		case '\'':
			l.next()
			attrs[name] = l.scanQuotedString('\'')

		case '{':
			l.next()
			inner := l.scanBraceExpression()
			attrs[name] = "{" + inner + "}"

		default:
			var b strings.Builder
			for l.pos < l.size {
				r := l.peek()
				if unicode.IsSpace(r) || r == '>' || r == '/' {
					break
				}
				b.WriteRune(l.next())
			}
			attrs[name] = b.String()
		}
	}

	return attrs
}
