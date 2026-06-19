package web

import "strings"

// ctrlKind classifies a structural control code found while scanning.
type ctrlKind int

const (
	cEOF ctrlKind = iota
	cSection
	cCode  // @c
	cNamed // @<name@>= or @(file@>=
	cDefn  // @d
	cFormat
)

type ctrl struct {
	kind    ctrlKind
	pos     int    // index of the leading '@'
	end     int    // index just past the control token
	depth   int    // for cSection: -1 unstarred, >=0 starred depth
	starred bool   // for cSection
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
				j++ // "@**" is depth 0
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
					name: strings.TrimSpace(src[i+2 : end]), isFile: c == '('}
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

	// Limbo runs until the first section break.
	first := findNextSection(src, 0)
	w.Limbo = src[:first.pos]
	i := first.pos

	num := 0
	for i < n {
		// We are positioned at a section break.
		hdr := src[i+1]
		num++
		sec := &Section{Number: num}
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
			if ct.kind == cFormat {
				if f, ok := parseFormat(seg, ct.noIndex); ok {
					sec.Formats = append(sec.Formats, f)
				}
			} else {
				// @d has no Go analogue; keep its text as code-like commentary.
				sec.Tex += "\n@d" + seg
			}
			ct = nx
		}

		switch ct.kind {
		case cCode:
			sec.HasCode = true
			nx := findNextSection(src, ct.end)
			sec.Code = src[ct.end:nx.pos]
			i = nx.pos
		case cNamed:
			sec.HasCode = true
			sec.Name = ct.name
			sec.IsFile = ct.isFile
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
	} else {
		for j < n && src[j] >= '0' && src[j] <= '9' {
			depth = depth*10 + int(src[j]-'0')
			j++
		}
	}
	return ctrl{end: j, depth: depth}
}

// extractTitle returns the text of a starred section up to its first period,
// with whitespace collapsed, for use in the table of contents.
func extractTitle(tex string) string {
	t := strings.TrimLeft(tex, " \t\n")
	if dot := strings.Index(t, "."); dot >= 0 {
		t = t[:dot]
	}
	return strings.Join(strings.Fields(t), " ")
}

// parseFormat parses the body of an @f/@s directive: two identifiers.
func parseFormat(seg string, noIndex bool) (Format, bool) {
	fields := strings.Fields(seg)
	if len(fields) < 2 {
		return Format{}, false
	}
	return Format{Original: fields[0], Like: fields[1], NoIndex: noIndex}, true
}
