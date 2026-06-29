package web

import (
	"fmt"
	"strings"
)

// ctrlKind classifies a structural control code found while scanning.
type ctrlKind int

const (
	cEOF ctrlKind = iota
	cSection
	cCode  // @c (or its synonym @p)
	cNamed // @<name@>= or @(file@>=
	cDefn  // @d
	cFormat
)

type ctrl struct {
	kind    ctrlKind
	pos     int    // index of the leading '@'
	end     int    // index just past the control token
	depth   int    // for cSection: -1 unstarred (or @** top level), else starred depth
	starred bool   // for cSection (distinguishes @** from an unstarred section)
	name    string // for cNamed
	isFile  bool   // for cNamed (@( vs @<)
	noIndex bool   // for cFormat (@s)
}

// scanStruct finds the next structural control at or after i. It skips literal
// "@@" and argument-terminated codes (@<...@>, @=...@>, etc.) so their contents
// never trigger a false section break. A "@<...@>" not followed by "=" is a
// reference, not a definition, and is skipped.
func scanStruct(src string, i int) ctrl {
	n := len(src)
	for i < n {
		if src[i] != '@' {
			i++
			continue
		}
		if i+1 >= n {
			break
		}
		switch c := src[i+1]; {
		case c == '@':
			i += 2
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
		case c == '*':
			j := i + 2
			depth := 0
			if j < n && src[j] == '*' {
				j++
				depth = -1 // "@**" is the top level: bold in the contents, as cweb
			} else {
				for j < n && src[j] >= '0' && src[j] <= '9' {
					depth = depth*10 + int(src[j]-'0')
					j++
				}
			}
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}
		case c == 'c' || c == 'p':
			return ctrl{kind: cCode, pos: i, end: i + 2}
		case c == 'd':
			return ctrl{kind: cDefn, pos: i, end: i + 2}
		case c == 'f':
			return ctrl{kind: cFormat, pos: i, end: i + 2}
		case c == 's':
			return ctrl{kind: cFormat, pos: i, end: i + 2, noIndex: true}
		case c == '<' || c == '(':
			end := indexFrom(src, "@>", i+2)
			if end < 0 {
				return ctrl{kind: cEOF, pos: n, end: n}
			}
			after := end + 2
			k := after
			for k < n && (src[k] == ' ' || src[k] == '\t') {
				k++
			}
			if k < n && src[k] == '=' {
				return ctrl{kind: cNamed, pos: i, end: k + 1,
					name: canonName(src[i+2 : end]), isFile: c == '('}
			}
			i = after // a reference, not a definition
		case c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
			end := indexFrom(src, "@>", i+2)
			if end < 0 {
				return ctrl{kind: cEOF, pos: n, end: n}
			}
			i = end + 2
		case c == '%':
			j := i + 2
			for j < n && src[j] != '\n' {
				j++
			}
			i = j
		default:
			i += 2
		}
	}
	return ctrl{kind: cEOF, pos: n, end: n}
}

// findNextSection scans forward to the next section break (@ or @*), skipping
// everything else including argument-terminated codes. Used inside code parts,
// where @c/@d/@f never legitimately appear.
func findNextSection(src string, i int) ctrl {
	n := len(src)
	for i < n {
		if src[i] != '@' {
			i++
			continue
		}
		if i+1 >= n {
			break
		}
		switch c := src[i+1]; {
		case c == '@':
			i += 2
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
		case c == '*':
			j := i + 2
			depth := 0
			if j < n && src[j] == '*' {
				j++
				depth = -1 // "@**" is the top level: bold in the contents, as cweb
			} else {
				for j < n && src[j] >= '0' && src[j] <= '9' {
					depth = depth*10 + int(src[j]-'0')
					j++
				}
			}
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}
		case c == '<' || c == '(' || c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
			end := indexFrom(src, "@>", i+2)
			if end < 0 {
				return ctrl{kind: cEOF, pos: n, end: n}
			}
			i = end + 2
		case c == '%':
			j := i + 2
			for j < n && src[j] != '\n' {
				j++
			}
			i = j
		default:
			i += 2
		}
	}
	return ctrl{kind: cEOF, pos: n, end: n}
}

// parse splits source into limbo and sections.
func parse(src string) *Web {
	w := &Web{}
	n := len(src)

	// Limbo runs until the first section break. Format directives placed there
	// (@f / @s, a common CWEB idiom) are extracted and removed from the copied
	// TeX so they apply globally rather than printing literally.
	first := findNextSection(src, 0)
	w.Limbo, w.Formats = extractLimboFormats(src[:first.pos])
	i := first.pos

	num := 0
	for i < n {
		// We are positioned at a section break.
		hdr := src[i+1]
		num++
		sec := &Section{Number: num, Line: lineAt(src, i)}
		if hdr == '*' {
			h := findSectionHeaderEnd(src, i)
			sec.Starred = true
			sec.Depth = h.depth
			i = h.end
		} else {
			i += 2
		}

		// TeX part: from here to the next structural control.
		ct := scanStruct(src, i)
		sec.Tex = src[i:ct.pos]
		if sec.Starred {
			sec.Title = extractTitle(sec.Tex)
		}

		// Definition part: a run of @d / @f / @s.
		for ct.kind == cDefn || ct.kind == cFormat {
			nx := scanStruct(src, ct.end)
			seg := src[ct.end:nx.pos]
			// @d has no Go analogue (Go has no preprocessor), so it never tangles
			// to code; gweave uses it only to set the named identifier in
			// typewriter, as cweave sets a macro. @f/@s format like another word.
			if ct.kind == cDefn {
				if f, ok := parseMacro(seg); ok {
					sec.Formats = append(sec.Formats, f)
				}
			} else if f, ok := parseFormat(seg, ct.noIndex); ok {
				sec.Formats = append(sec.Formats, f)
			}
			ct = nx
		}

		switch ct.kind {
		case cCode:
			sec.HasCode = true
			sec.CodeLine = lineAt(src, ct.end)
			nx := findNextSection(src, ct.end)
			sec.Code = src[ct.end:nx.pos]
			i = nx.pos
		case cNamed:
			sec.HasCode = true
			sec.Name = ct.name
			sec.IsFile = ct.isFile
			sec.CodeLine = lineAt(src, ct.end)
			nx := findNextSection(src, ct.end)
			sec.Code = src[ct.end:nx.pos]
			i = nx.pos
		default: // cSection or cEOF: a documentation-only section
			i = ct.pos
		}

		w.Sections = append(w.Sections, sec)
		if ct.kind == cEOF && sec.Code == "" {
			break
		}
		if i >= n {
			break
		}
	}
	return w
}

func findSectionHeaderEnd(src string, i int) ctrl {
	n := len(src)
	j := i + 2
	depth := 0
	if j < n && src[j] == '*' {
		j++
		depth = -1 // "@**" is the top level: bold in the contents, as cweb
	} else {
		for j < n && src[j] >= '0' && src[j] <= '9' {
			depth = depth*10 + int(src[j]-'0')
			j++
		}
	}
	return ctrl{end: j, depth: depth}
}

// extractTitle returns the text of a starred section up to its terminating
// period, with whitespace collapsed, for use in the table of contents. The
// terminator is the first period at end of text or followed by whitespace, so a
// period inside a control sequence such as \.{web} does not end the title early.
func extractTitle(tex string) string {
	t := strings.TrimLeft(tex, " \t\n")
	if i := titleEnd(t); i >= 0 {
		t = t[:i]
	}
	return strings.Join(strings.Fields(t), " ")
}

// titleEnd returns the index of the period that ends a starred-section title --
// the first '.' at end of s or followed by whitespace -- or -1 if there is none.
func titleEnd(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' && (i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t' ||
			s[i+1] == '\n' || s[i+1] == '\r') {
			return i
		}
	}
	return -1
}

// scanDiagnostics walks the source looking for malformed control codes —
// currently argument-terminated codes (@<, @(, @=, @t, @^, @., @:, @q) that are
// missing their closing @> — and returns one warning per problem.
func (w *Web) scanDiagnostics(src string) []string {
	var warns []string
	n := len(src)
	i := 0
	for i < n {
		if src[i] != '@' || i+1 >= n {
			i++
			continue
		}
		switch c := src[i+1]; c {
		case '@':
			i += 2
		case '<', '(', '=', 't', '^', '.', ':', 'q':
			if end := indexFrom(src, "@>", i+2); end < 0 {
				warns = append(warns, fmt.Sprintf("%s: unterminated `@%c ... @>'", w.at(lineAt(src, i)), c))
				i = n
			} else {
				i = end + 2
			}
		default:
			i += 2
		}
	}
	return warns
}

// parseFormat parses the body of an @f/@s directive: two identifiers.
func parseFormat(seg string, noIndex bool) (Format, bool) {
	fields := strings.Fields(seg)
	if len(fields) < 2 {
		return Format{}, false
	}
	return Format{Original: fields[0], Like: fields[1], NoIndex: noIndex}, true
}

// parseMacro parses an @d directive: its first word names a constant to set in
// typewriter; any value after it is ignored (Go has no preprocessor). A
// qualified name keeps its final component, so "@d http.StatusOK" and
// "@d StatusOK" both register StatusOK.
func parseMacro(seg string) (Format, bool) {
	fields := strings.Fields(seg)
	if len(fields) == 0 {
		return Format{}, false
	}
	name := fields[0]
	if k := strings.LastIndex(name, "."); k >= 0 {
		name = name[k+1:]
	}
	if name == "" {
		return Format{}, false
	}
	return Format{Original: name, Macro: true}, true
}

// extractLimboFormats pulls @d/@f/@s directives out of the limbo text
// (consuming each to end of line) and returns the cleaned text together with the
// formats. Other control codes and argument-terminated groups are copied through.
func extractLimboFormats(src string) (string, []Format) {
	var b strings.Builder
	var formats []Format
	n := len(src)
	i := 0
	for i < n {
		if src[i] != '@' || i+1 >= n {
			b.WriteByte(src[i])
			i++
			continue
		}
		switch c := src[i+1]; c {
		case '@':
			b.WriteString("@@")
			i += 2
		case 'd', 'f', 's':
			j := i + 2
			for j < n && src[j] != '\n' {
				j++
			}
			var f Format
			var ok bool
			if c == 'd' {
				f, ok = parseMacro(src[i+2 : j])
			} else {
				f, ok = parseFormat(src[i+2:j], c == 's')
			}
			if ok {
				formats = append(formats, f)
			}
			if j < n {
				j++ // also drop the newline that ended the directive
			}
			i = j
		case '<', '(', '=', 't', '^', '.', ':', 'q':
			end := indexFrom(src, "@>", i+2)
			if end < 0 {
				b.WriteString(src[i:])
				i = n
			} else {
				b.WriteString(src[i : end+2])
				i = end + 2
			}
		default:
			b.WriteString(src[i : i+2])
			i += 2
		}
	}
	return b.String(), formats
}
