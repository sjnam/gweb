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

// renderOp typesets a Go operator token as a single tight math atom (no math
// spacing of its own), using real math symbols where they exist. Inter-token
// spacing is supplied by the surrounding source whitespace, so the result
// reproduces gofmt's spacing exactly and the unary/binary distinction for *, &,
// etc. needs no grammar analysis.
func renderOp(s string) string {
	switch s {
	case "<=":
		return "\\mathord{\\leq}"
	case ">=":
		return "\\mathord{\\geq}"
	case "!=":
		return "\\mathord{\\neq}"
	case "==":
		return "\\mathord{\\equiv}" // equality test, as cweb (an equivalence sign)
	case "!":
		return "\\mathord{\\lnot}" // logical not, as cweb (a negation sign)
	case "&&":
		return "\\mathord{\\land}" // logical and, as cweb (a wedge)
	case "||":
		return "\\mathord{\\lor}" // logical or, as cweb (a vee)
	case "<-":
		return "\\mathord{\\leftarrow}"
	case "^":
		return "\\mathord{\\oplus}" // bitwise xor, as cweb (a circled plus)
	case "<<":
		return "\\mathord{\\ll}" // left shift, as cweb (a tight double angle)
	case ">>":
		return "\\mathord{\\gg}" // right shift
	case "<<=":
		return "\\mathord{\\ll}\\mathord{=}"
	case ">>=":
		return "\\mathord{\\gg}\\mathord{=}"
	case "...":
		return "\\mathord{\\ldots}"
	case "[]":
		// empty slice/array brackets: a thin space keeps them from jamming
		return "\\mathord{[}\\,\\mathord{]}"
	case "{}":
		// empty braces (struct{}, interface{}, T{}): likewise a thin space
		return "\\mathord{\\{}\\,\\mathord{\\}}"
	}
	if len(s) == 1 {
		return "\\mathord{" + escMathOp(s) + "}"
	}
	return tightMathOp(s)
}

// tightMathOp encodes each character of an operator as an ordinary atom, so that
// e.g. "==" or "<<" prints with the characters adjacent rather than spaced.
func tightMathOp(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		b.WriteString("\\mathord{")
		b.WriteString(escMathOp(s[i : i+1]))
		b.WriteString("}")
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
		case '<':
			b.WriteString("$<$") // cmr (OT1) has no < glyph; use math
		case '>':
			b.WriteString("$>$") // likewise for >
		case '|':
			b.WriteString("$\\vert$")
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// escComment escapes a comment for roman text mode, but passes a $...$ span
// through unescaped so TeX math works inside comments (as in cweb).
func escComment(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '$' {
			if k := strings.IndexByte(s[i+1:], '$'); k >= 0 {
				j := i + 1 + k
				b.WriteString(s[i : j+1]) // the $...$ math span, verbatim
				i = j + 1
				continue
			}
		}
		b.WriteString(escProse(s[i : i+1]))
		i++
	}
	return b.String()
}
