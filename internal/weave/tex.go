package weave

import (
	"fmt"
	"strings"
)

// TeX escaping. Three contexts need different treatment:
//
//   - identifiers/keywords: only '_' is troublesome (\_ works in text mode);
//   - typewriter text (strings, comments): every TeX special is emitted as a
//     \charNN code so it prints literally regardless of the current font;
//   - prose names and math operators: text-mode / math-mode safe sequences.

// escIdent escapes an identifier or keyword for text mode.
func escIdent(s string) string {
	return strings.ReplaceAll(s, "_", "\\_")
}

// escTT escapes arbitrary text for a typewriter (\tt) box. Specials become
// \charNN so braces, backslashes, etc. survive.
func escTT(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\', '{', '}', '$', '&', '#', '%', '^', '_', '~':
			fmt.Fprintf(&b, "\\char%d ", c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// escMathOp encodes an operator/punctuation run so it is safe inside math mode.
func escMathOp(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '{':
			b.WriteString("\\{")
		case '}':
			b.WriteString("\\}")
		case '&':
			b.WriteString("\\&")
		case '#':
			b.WriteString("\\#")
		case '%':
			b.WriteString("\\%")
		case '$':
			b.WriteString("\\$")
		case '_':
			b.WriteString("\\_")
		case '^':
			b.WriteString("\\char94 ")
		case '~':
			b.WriteString("\\char126 ")
		case '|':
			b.WriteString("\\char124 ")
		case '\\':
			b.WriteString("\\backslash ")
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// escProse escapes text for ordinary roman text mode (used for section names).
func escProse(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '_':
			b.WriteString("\\_")
		case '&':
			b.WriteString("\\&")
		case '#':
			b.WriteString("\\#")
		case '%':
			b.WriteString("\\%")
		case '$':
			b.WriteString("\\$")
		case '{':
			b.WriteString("$\\{$")
		case '}':
			b.WriteString("$\\}$")
		case '\\':
			b.WriteString("$\\backslash$")
		case '^':
			b.WriteString("\\^{}")
		case '~':
			b.WriteString("\\~{}")
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
