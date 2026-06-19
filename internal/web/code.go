package web

import "strings"

// AtomKind classifies a piece of a code part.
type AtomKind int

const (
	AText     AtomKind = iota // ordinary Go source text
	ARef                      // @<name@> reference to a named section
	AVerbatim                 // @=text@> passed verbatim to tangled output
	ATeX                      // @t text@> TeX text for the woven output
	AIndex                    // @^/@./@: index entry
	APaste                    // @& join (delete surrounding whitespace)
	ALayout                   // @, @/ @| @# woven-output layout hints
)

// Atom is one element of a scanned code part.
type Atom struct {
	Kind  AtomKind
	Text  string // payload for AText/AVerbatim/ATeX/AIndex; name for ARef
	Index byte   // '^','.',':' for AIndex; ',' '/' '|' '#' for ALayout
}

// ScanCode splits a raw code part into atoms, interpreting in-code control
// codes. "@@" becomes a literal '@' folded into the surrounding text.
func ScanCode(code string) []Atom {
	var atoms []Atom
	var buf strings.Builder
	flush := func() {
		if buf.Len() > 0 {
			atoms = append(atoms, Atom{Kind: AText, Text: buf.String()})
			buf.Reset()
		}
	}
	n := len(code)
	i := 0
	for i < n {
		c := code[i]
		if c != '@' || i+1 >= n {
			buf.WriteByte(c)
			i++
			continue
		}
		switch d := code[i+1]; d {
		case '@':
			buf.WriteByte('@')
			i += 2
		case '&':
			flush()
			atoms = append(atoms, Atom{Kind: APaste})
			i += 2
		case '<':
			end := indexFrom(code, "@>", i+2)
			if end < 0 {
				buf.WriteString(code[i:])
				i = n
				continue
			}
			flush()
			atoms = append(atoms, Atom{Kind: ARef, Text: strings.TrimSpace(code[i+2 : end])})
			i = end + 2
		case '=':
			end := indexFrom(code, "@>", i+2)
			if end < 0 {
				i = n
				continue
			}
			flush()
			atoms = append(atoms, Atom{Kind: AVerbatim, Text: code[i+2 : end]})
			i = end + 2
		case 't':
			end := indexFrom(code, "@>", i+2)
			if end < 0 {
				i = n
				continue
			}
			flush()
			atoms = append(atoms, Atom{Kind: ATeX, Text: code[i+2 : end]})
			i = end + 2
		case '^', '.', ':':
			end := indexFrom(code, "@>", i+2)
			if end < 0 {
				i = n
				continue
			}
			flush()
			atoms = append(atoms, Atom{Kind: AIndex, Text: code[i+2 : end], Index: d})
			i = end + 2
		case 'q':
			end := indexFrom(code, "@>", i+2)
			if end < 0 {
				i = n
				continue
			}
			i = end + 2 // ignored material
		case '%':
			j := i + 2
			for j < n && code[j] != '\n' {
				j++
			}
			i = j
		case '>':
			i += 2 // stray terminator
		case ',', '/', '|', '#':
			// Woven-output layout hints: thin space, line break, optional line
			// break, and break-plus-blank-line. Ignored by gtangle.
			flush()
			atoms = append(atoms, Atom{Kind: ALayout, Index: d})
			i += 2
		case '+', '[', ']', ';':
			// CWEB prettyprinter hints (cancel break, expression brackets,
			// invisible semicolon). GWEB mirrors the source instead of reflowing
			// it, so these have no effect; accept and drop them for portability.
			i += 2
		default:
			i += 2 // unknown @x: drop it rather than corrupt the output
			i += 2
		}
	}
	flush()
	return atoms
}
