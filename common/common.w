@d Version

@* The \.{common} package.
This package is the shared front end of \.{GWEB}. It reads a \.{.w} source file,
expands \.{@@i} includes, optionally applies a change file, and splits the
result into a sequence of {\it sections\/}. Both \.{gtangle} and \.{gweave}
are built on top of it, so it plays the role of \.{CWEB}'s \.{common.w}.

We start with the package declaration, the imports, the |Version| constant (the
\.{GWEB} release, shared by \.{gtangle} and \.{gweave} for their startup banner and
their \.{-version} output), and the |Format| record, which captures one \.{@@f}
or \.{@@s} directive: typeset identifier |Original| the way identifier or keyword
|Like| is typeset; |NoIndex| is true for \.{@@s}.
@(common/common.go@>=
package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const Version = "0.3.0"

type Format struct {
	Original string
	Like     string
	NoIndex  bool
	Macro    bool // \.{@@d}: typeset Original in typewriter (a \.{CWEB}-style macro)
}

@ A |Section| is one numbered section of the web. Its three optional parts --
commentary, definitions, and code -- are stored as raw text with the in-text
and in-code \.{@@}-codes still embedded; the consumers interpret them later.
@(common/common.go@>=
type Section struct {
	Number  int    // 1-based section number
	Line    int    // 1-based source line where the section begins
	Starred bool   // true for \.{@@*} sections
	Depth   int    // group depth for starred sections (-1 |==| \.{@@**}, 0 |==| \.{@@*}, n |==| \.{@@*n})
	Title   string // starred-section title (text up to the first period)
	Tex     string // commentary, raw \TEX/ with in-text\.{@@}-codes still embedded
	Formats []Format
	HasCode bool   // true if the section contributes code
	Name    string // named-section name, or \.{""} for an unnamed @@c section
	IsFile  bool   // true if the name is an output file (\.{@@(file@@>=})
	Code    string // raw code text with in-code \.{@@}-codes still embedded
	CodeLine int   // 1-based combined-source line where Code begins (0 if none)
}

@ A |Web| is a fully parsed \.{GWEB} document: the limbo text, the global format
directives, the sections, and the diagnostics gathered while parsing. The
unexported fields support diagnostics (mapping a combined-source line back to
its original file) and name-abbreviation resolution.
@(common/common.go@>=
type Web struct {
	Limbo    string
	Formats  []Format // \.{@@f}/\.{@@s} directives found in limbo (apply globally)
	Sections []*Section
	Warnings []string // non-fatal diagnostics gathered while parsing/checking
	file     string   // source filename, for diagnostics (\.{""} if unknown)
	locs     []srcLoc // origin (file, line) of each combined-source line
	full     []string // canonical (non-abbreviated) section names
}

@ The parsing entry points. |Parse| handles the common case; |ParseWithChange|
adds \.{CWEB}'s change-file mechanism; |ParseString| is for tests. All three end by
calling |finish|, which runs the post-parse bookkeeping. |at| formats a
combined-source line for a diagnostic, mapping it back to the user's file.
@(common/common.go@>=
func Parse(filename string) (*Web, error) {
	return ParseWithChange(filename, "")
}

func ParseWithChange(filename, changeFile string) (*Web, error) {
	lines, locs, err := expandIncludes(filename, 0)
	if err != nil {
		return nil, err
	}
	if changeFile != "" {
		chData, err := os.ReadFile(changeFile)
		if err != nil {
			return nil, err
		}
		changes, err := parseChangeFile(string(chData))
		if err != nil {
			return nil, err
		}
		lines, locs, err = applyChangesMapped(lines, locs, changes, changeFile)
		if err != nil {
			return nil, err
		}
	}
	src := strings.Join(lines, "\n")
	w := parse(src)
	w.file = filename
	w.locs = locs
	w.finish(src)
	return w, nil
}

@ @(common/common.go@>=
func ParseString(src string) *Web {
	w := parse(src)
	w.finish(src)
	return w
}

func (w *Web) finish(src string) {
	w.collectNames()
	w.Warnings = append(w.Warnings, w.scanDiagnostics(src)...)
	w.Warnings = append(w.Warnings, w.checkNames()...)
}

func (w *Web) at(line int) string {
	if i := line - 1; i >= 0 && i < len(w.locs) {
		return w.locs[i].String()
	}
	if w.file != "" {
		return fmt.Sprintf("%s:%d", w.file, line)
	}
	return fmt.Sprintf("line %d", line)
}

@ |Origin| maps a combined-source line back to the file and line the user wrote,
returning the two parts separately (|at| formats the same information as a
string). \.{gtangle} uses it to emit \.{//line} directives so the \GO/ compiler
reports errors at \.{.w} positions.
@(common/common.go@>=
func (w *Web) Origin(line int) (file string, ln int) {
	if i := line - 1; i >= 0 && i < len(w.locs) {
		return w.locs[i].file, w.locs[i].line
	}
	return w.file, line
}

@ |DefaultExt| supplies a default extension when the user omits one, so the
commands accept a bare web name as \.{CWEB} does (\.{gtangle wc} reads \.{wc.w}). A
name that already has an extension, or is empty, is returned unchanged.
@(common/common.go@>=
func DefaultExt(name, ext string) string {
	if name == "" || filepath.Ext(name) != "" {
		return name
	}
	return name + ext
}

@ Include expansion. As in \.{CWEB}, \.{@@i} is line-oriented: a line whose first
non-blank text is \.{@@i} names a file whose expansion replaces that line. We
keep a parallel origin map so diagnostics can cite the file the user wrote.
@(common/common.go@>=
func expandIncludes(file string, depth int) ([]string, []srcLoc, error) {
	if depth > 25 {
		return nil, nil, fmt.Errorf("gweb: @@i include nesting too deep at %q", file)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, nil, err
	}
	raw := splitLines(string(data))
	if n := len(raw); n > 0 && raw[n-1] == "" {
		raw = raw[:n-1]
	}

	var lines []string
	var locs []srcLoc
	dir := filepath.Dir(file)
	for i, line := range raw {
		if name, ok := includeDirective(line); ok {
			path := name
			if !filepath.IsAbs(path) {
				path = filepath.Join(dir, name)
			}
			sub, subLocs, err := expandIncludes(path, depth+1)
			if err != nil {
				return nil, nil, fmt.Errorf("%s:%d: %w", file, i+1, err)
			}
			lines = append(lines, sub...)
			locs = append(locs, subLocs...)
			continue
		}
		lines = append(lines, line)
		locs = append(locs, srcLoc{file, i + 1})
	}
	return lines, locs, nil
}

@ @(common/common.go@>=
func includeDirective(line string) (name string, ok bool) {
	t := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(t, "@@i") {
		return "", false
	}
	rest := t[2:]
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return "", false
	}
	name = strings.Trim(strings.TrimSpace(rest), "\"")
	return name, name != ""
}

@ Collecting section names. To resolve an abbreviation ending in \.{...} we need
the set of canonical (non-abbreviated) names. A full name may appear at a
definition or at any reference, in code or in commentary, so all of those are
scanned.
@(common/common.go@>=
func (w *Web) collectNames() {
	seen := map[string]bool{}
	add := func(name string) {
		if name != "" && !strings.HasSuffix(name, "...") && !seen[name] {
			seen[name] = true
			w.full = append(w.full, name)
		}
	}
	for _, s := range w.Sections {
		if !s.IsFile {
			add(s.Name) // a definition's name
		}
		for _, raw := range []string{s.Code, s.Tex} {
			for _, a := range ScanCode(raw) {
				if a.Kind == ARef {
					add(a.Text) // a reference's name
				}
			}
		}
	}
}

func (w *Web) prefixMatches(prefix string) int {
	n := 0
	for _, full := range w.full {
		if strings.HasPrefix(full, prefix) {
			n++
		}
	}
	return n
}

@ Checking references. We report ambiguous or unmatched abbreviations,
references to undefined sections, and named sections that are defined but never
used. All are warnings; \.{gtangle} still fails hard if it actually meets an
undefined reference while expanding.
@(common/common.go@>=
func (w *Web) checkNames() []string {
	// |defined| is the set of sections that actually have a definition (not just
	// the full names known for abbreviation resolution, which include references).
	defined := map[string]bool{}
	for _, s := range w.Sections {
		if s.Name != "" && !s.IsFile {
			defined[w.Resolve(s.Name)] = true
		}
	}
	used := map[string]bool{}
	var warns []string

	for _, s := range w.Sections {
		@<Scan raw string@>
		scan(s.Code)
		scan(s.Tex)
	}

	warned := map[string]bool{}
	for _, s := range w.Sections {
		if s.Name == "" || s.IsFile {
			continue
		}
		canon := w.Resolve(s.Name)
		if !used[canon] && !warned[canon] {
			warned[canon] = true
			warns = append(warns, fmt.Sprintf("%s: section <%s> is defined but never used", w.at(s.Line), s.Name))
		}
	}
	return warns
}

@ @<Scan raw string@>=
scan := func(raw string) {
	for _, a := range ScanCode(raw) {
		if a.Kind != ARef {
			continue
		}
		canon := w.Resolve(a.Text)
		if strings.HasSuffix(a.Text, "...") && canon == a.Text {
			prefix := strings.TrimSpace(strings.TrimSuffix(a.Text, "..."))
			if m := w.prefixMatches(prefix); m == 0 {
				warns = append(warns, fmt.Sprintf("%s: no section name matches <%s>", w.at(s.Line), a.Text))
			} else {
				warns = append(warns, fmt.Sprintf("%s: ambiguous prefix <%s> matches %d section names", w.at(s.Line), a.Text, m))
			}
			continue
		}
		if !defined[canon] {
			warns = append(warns, fmt.Sprintf("%s: reference to undefined section <%s>", w.at(s.Line), a.Text))
		}
		used[canon] = true
	}
}

@ A few small helpers: |lineAt| converts a byte offset to a line number,
|canonName| normalizes a name by collapsing each run of whitespace to a single
space, |Resolve| maps a possibly abbreviated name to its canonical form, and
|indexFrom| is a bounded |strings.Index|.
@(common/common.go@>=
func lineAt(src string, off int) int {
	if off > len(src) {
		off = len(src)
	}
	return 1 + strings.Count(src[:off], "\n")
}

func canonName(name string) string {
	return strings.Join(strings.Fields(name), " ")
}

@ @(common/common.go@>=
func (w *Web) Resolve(name string) string {
	name = canonName(name)
	if !strings.HasSuffix(name, "...") {
		return name
	}
	prefix := strings.TrimSpace(strings.TrimSuffix(name, "..."))
	var match string
	count := 0
	for _, full := range w.full {
		if strings.HasPrefix(full, prefix) {
			match = full
			count++
		}
	}
	if count == 1 {
		return match
	}
	return name // unresolved or ambiguous; leave as-is for caller to report
}

func indexFrom(s, sub string, from int) int {
	if from >= len(s) {
		return -1
	}
	idx := strings.Index(s[from:], sub)
	if idx < 0 {
		return -1
	}
	return from + idx
}

@* Scanning structural controls.
The parser works directly on the source text. A |ctrl| value describes the next
structural control code -- a section break, a code part, a named definition, or
a definition-part directive -- together with the byte range it occupies.
@(common/common.go@>=
type ctrlKind int

const (
	cEOF ctrlKind = iota
	cSection
	cCode  // \.{@@c} (or its synonym \.{@@p})
	cNamed // \.{@@<name@@>=} or \.{@@(file@@>=}
	cDefn  // \.{@@d}
	cFormat
)

type ctrl struct {
	kind    ctrlKind
	pos     int    // index of the leading `\.{@@}'
	end     int    // index just past the control token
	depth   int    // for cSection: -1 unstarred (or \.{@@**} top level), else starred depth
	starred bool   // for cSection (distinguishes \.{@@**} from an unstarred section)
	name    string // for cNamed
	isFile  bool   // for cNamed (\.{@@(} vs \.{@@<})
	noIndex bool   // for cFormat (\.{@@s})
}

@ |scanStruct| finds the next structural control at or after |i|. It skips
literal \.{@@@@} and argument-terminated codes (\.{@@<...@@>}, \.{@@=...@@>}, and so
on) so their contents never trigger a false section break. A \.{@@<...@@>} not
followed by |=| is a reference, not a definition, and is skipped.
@(common/common.go@>=
func scanStruct(src string, i int) ctrl {
	n := len(src)
	for i < n {
		if src[i] != '@@' {
			i++
			continue
		}
		if i+1 >= n {
			break
		}
		switch c := src[i+1]; {
		case c == '@@':
			i += 2
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
		case c == '*':
			j := i + 2
			depth := 0
			if j < n && src[j] == '*' {
				j++
				depth = -1 // ``\.{@@**}" is the top level: bold in the contents, as \.{CWEB}
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
			end := indexFrom(src, "@@>", i+2)
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
			end := indexFrom(src, "@@>", i+2)
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

@ |findNextSection| scans forward to the next section break (\.{@@ } or \.{@@*}),
skipping everything else. It is used inside code parts, where \.{@@c}, \.{@@d}, and
\.{@@f} never legitimately appear.
@(common/common.go@>=
func findNextSection(src string, i int) ctrl {
	n := len(src)
	for i < n {
		if src[i] != '@@' {
			i++
			continue
		}
		if i+1 >= n {
			break
		}
		switch c := src[i+1]; {
		case c == '@@':
			i += 2
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
		case c == '*':
			j := i + 2
			depth := 0
			if j < n && src[j] == '*' {
				j++
				depth = -1 // "\.{@@**}" is the top level: bold in the contents, as \.{CWEB}
			} else {
				for j < n && src[j] >= '0' && src[j] <= '9' {
					depth = depth*10 + int(src[j]-'0')
					j++
				}
			}
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}
		case c == '<' || c == '(' || c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
			end := indexFrom(src, "@@>", i+2)
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

@ The main loop. |parse| splits the source into limbo and sections, and for
each section into its \TEX/, definition, and code parts.
Limbo runs until the first section break. Format directiveas placed there
(\.{@@f}/\.{@@s}, a common \.{CWEB} idiom) are extracted and removed from the copied
\TEX/ so they apply globally rather than printing literally.
@(common/common.go@>=
func parse(src string) *Web {
	w := &Web{}
	n := len(src)

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

		// \TEX/ part: from here to the next structural control.
		ct := scanStruct(src, i)
		sec.Tex = src[i:ct.pos]
		if sec.Starred {
			sec.Title = extractTitle(sec.Tex)
		}

		// Definition part: a run of \.{@@d} / \.{@@f} / \.{@@s}.
		for ct.kind == cDefn || ct.kind == cFormat {
			nx := scanStruct(src, ct.end)
			seg := src[ct.end:nx.pos]
			// \.{@@d} has no \GO/ analogue (\GO/ has no preprocessor), so it never tangles
			// to code; gweave uses it only to set the named identifier in
			// typewriter, as cweave sets a macro. \.{@@f}/\.{@@s} format like another word.
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

@ |findSectionHeaderEnd| locates the end of a \.{@@*} header and its depth.
@(common/common.go@>=
func findSectionHeaderEnd(src string, i int) ctrl {
	n := len(src)
	j := i + 2
	depth := 0
	if j < n && src[j] == '*' {
		j++
		depth = -1 // ``\.{@@**}'' is the top level: bold in the contents, as \.{CWEB}
	} else {
		for j < n && src[j] >= '0' && src[j] <= '9' {
			depth = depth*10 + int(src[j]-'0')
			j++
		}
	}
	return ctrl{end: j, depth: depth}
}

@ |extractTitle| returns the text of a starred section up to its first period,
with whitespace collapsed, for the table of contents.
@(common/common.go@>=
func extractTitle(tex string) string {
	t := strings.TrimLeft(tex, " \t\n")
	if i := titleEnd(t); i >= 0 {
		t = t[:i]
	}
	return strings.Join(strings.Fields(t), " ")
}

func titleEnd(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' && (i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t' ||
			s[i+1] == '\n' || s[i+1] == '\r') {
			return i
		}
	}
	return -1
}

@ |scanDiagnostics| walks the source looking for malformed control codes --
currently argument-terminated codes missing their closing \.{@@>}.
@(common/common.go@>=
func (w *Web) scanDiagnostics(src string) []string {
	var warns []string
	n := len(src)
	i := 0
	for i < n {
		if src[i] != '@@' || i+1 >= n {
			i++
			continue
		}
		switch c := src[i+1]; c {
		case '@@':
			i += 2
		case '<', '(', '=', 't', '^', '.', ':', 'q':
			if end := indexFrom(src, "@@>", i+2); end < 0 {
				warns = append(warns, fmt.Sprintf("%s: unterminated `@@%c ... @@>'", w.at(lineAt(src, i)), c))
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

@ |parseFormat| parses the body of an \.{@@f} or \.{@@s} directive: two identifiers.
@(common/common.go@>=
func parseFormat(seg string, noIndex bool) (Format, bool) {
	fields := strings.Fields(seg)
	if len(fields) < 2 {
		return Format{}, false
	}
	return Format{Original: fields[0], Like: fields[1], NoIndex: noIndex}, true
}

@ |parseMacro| parses the body of an \.{@@d} directive. Its first word names a
constant to set in typewriter (like a \.{CWEB} macro); any value after it is ignored,
since \GO/ has no preprocessor and \.{@@d} never tangles to code. A qualified name
keeps its final component, so \.{@@d http.StatusOK} and \.{@@d StatusOK} both
register the identifier \.{StatusOK}.
@(common/common.go@>=
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

@ |extractLimboFormats| pulls \.{@@d}/\.{@@f}/\.{@@s} directives out of the limbo text and
returns the cleaned text together with the formats. Other control codes and
argument-terminated groups are copied through unchanged.
@(common/common.go@>=
func extractLimboFormats(src string) (string, []Format) {
	var b strings.Builder
	var formats []Format
	n := len(src)
	i := 0
	for i < n {
		if src[i] != '@@' || i+1 >= n {
			b.WriteByte(src[i])
			i++
			continue
		}
		switch c := src[i+1]; c {
		case '@@':
			b.WriteString("@@@@")
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
			end := indexFrom(src, "@@>", i+2)
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

@* Scanning a code part into atoms.
A code part is a mix of ordinary \GO/ text and in-code control codes. |ScanCode|
turns it into a slice of |Atom|s; the kind of each atom tells \.{gtangle} and
\.{gweave} how to treat it.
@(common/common.go@>=
type AtomKind int

const (
	AText     AtomKind = iota // ordinary \GO/ source text
	ARef                      // \.{@@<name@@>} reference to a named section
	AVerbatim                 // \.{@@=text@@>} passed verbatim to tangled output
	ATeX                      // \.{@@t text@@>} \TEX/ text for the woven output
	AIndex                    // \.{@@\^/@@./@@}: index entry
	APaste                    // \.{@@\&} join (delete surrounding whitespace)
	ALayout                   // \.{@@}, \.{@@/} \.{@@|} \.{@@\#} woven-output layout hints
	AIndexDef                 // \.{@@!} force the next identifier to index as a definition
)

type Atom struct {
	Kind  AtomKind
	Text  string // payload for |AText|/|AVerbatim|/|ATeX|/|AIndex|; name for |ARef|
	Index byte   // '\.{\^}','\.{.}','\.{:}' for AIndex; '\.{,}' '\.{/}' '\.{|}' '\.{\#}' for |ALayout|
}

@ The scanner itself. \.{@@@@} becomes a literal \.{@@} folded into the surrounding
text; every other control code flushes the pending text and appends its own
atom.
@(common/common.go@>=
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
		if c != '@@' || i+1 >= n {
			buf.WriteByte(c)
			i++
			continue
		}
		switch d := code[i+1]; d {
		case '@@':
			buf.WriteByte('@@')
			i += 2
		case '&':
			flush()
			atoms = append(atoms, Atom{Kind: APaste})
			i += 2
		case '<':
			end := indexFrom(code, "@@>", i+2)
			if end < 0 {
				buf.WriteString(code[i:])
				i = n
				continue
			}
			flush()
			atoms = append(atoms, Atom{Kind: ARef, Text: canonName(code[i+2 : end])})
			i = end + 2
		case '=':
			end := indexFrom(code, "@@>", i+2)
			if end < 0 {
				i = n
				continue
			}
			flush()
			atoms = append(atoms, Atom{Kind: AVerbatim, Text: code[i+2 : end]})
			i = end + 2
		case 't':
			end := indexFrom(code, "@@>", i+2)
			if end < 0 {
				i = n
				continue
			}
			flush()
			atoms = append(atoms, Atom{Kind: ATeX, Text: code[i+2 : end]})
			i = end + 2
		case '^', '.', ':':
			end := indexFrom(code, "@@>", i+2)
			if end < 0 {
				i = n
				continue
			}
			flush()
			atoms = append(atoms, Atom{Kind: AIndex, Text: code[i+2 : end], Index: d})
			i = end + 2
		case 'q':
			end := indexFrom(code, "@@>", i+2)
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
		case '!':
			// Force the next identifier's index entry to be a definition,
			// overriding the heuristic. Produces no output by itself.
			flush()
			atoms = append(atoms, Atom{Kind: AIndexDef})
			i += 2
		case '+', '[', ']', ';':
			// \.{CWEB} prettyprinter hints (cancel break, expression brackets,
			// invisible semicolon). \.{GWEB} mirrors the source instead of reflowing
			// it, so these have no effect; accept and drop them for portability.
			i += 2
		default:
			i += 2 // unknown \.{@@x}: drop it rather than corrupt the output
		}
	}
	flush()
	return atoms
}

@* Change files.
A change file (\.{CWEB}'s .{.ch} mechanism) patches the master source without
editing it. It is a sequence of changes, each finding a block of lines in the
master and substituting a replacement block.

Text outside an \.{@@x...@@z} group is ignored (it serves as commentary). Changes
are matched against the master source — after \.{@@i} includes are expanded — in
the order they appear: \.{GWEB} scans the master line by line, and at the first
line equal to a change's first match line it requires the whole match block
to match, then substitutes the replacement lines.

@ A |change| records the lines to find and the lines to substitute, plus
change-file line numbers for diagnostics. A |srcLoc| identifies the origin of a
line of the expanded, change-applied source.
@(common/common.go@>=
type change struct {
	match    []string // lines to find in the master source
	repl     []string // lines to substitute for them
	line     int      // 1-based line of the \.{@@x} in the change file (for diagnostics)
	replLine int      // 1-based change-file line of the first replacement line
}

type srcLoc struct {
	file string
	line int
}

func (l srcLoc) String() string {
	if l.file == "" {
		return fmt.Sprintf("line %d", l.line)
	}
	return fmt.Sprintf("%s:%d", l.file, l.line)
}

@ Recognizing a change control line and comparing source lines (ignoring
trailing whitespace, as \.{CWEB} does), with a line splitter that normalizes \.{CRLF}.
@(common/common.go@>=
func isChangeCtrl(line string, c byte) bool {
	return len(line) >= 2 && line[0] == '@@' && line[1] == c
}

func splitLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}

func sameLine(a, b string) bool {
	return strings.TrimRight(a, " \t") == strings.TrimRight(b, " \t")
}

@ |parseChangeFile| parses change-file text into an ordered list of changes,
reporting the malformed-change errors precisely.
@(common/common.go@>=
func parseChangeFile(src string) ([]change, error) {
	lines := splitLines(src)
	var changes []change
	n := len(lines)
	for i := 0; i < n; {
		if !isChangeCtrl(lines[i], 'x') {
			i++ // commentary between changes
			continue
		}
		c := change{line: i + 1}
		i++
		for i < n && !isChangeCtrl(lines[i], 'y') {
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'z') {
				return nil, fmt.Errorf("change file line %d: expected @@y to close the @@x match part", c.line)
			}
			c.match = append(c.match, lines[i])
			i++
		}
		if i >= n {
			return nil, fmt.Errorf("change file line %d: @@x without a matching @@y", c.line)
		}
		i++ // skip \.{@@y}
		c.replLine = i + 1
		for i < n && !isChangeCtrl(lines[i], 'z') {
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'y') {
				return nil, fmt.Errorf("change file line %d: expected @@z to close the change", c.line)
			}
			c.repl = append(c.repl, lines[i])
			i++
		}
		if i >= n {
			return nil, fmt.Errorf("change file line %d: change has no @@z", c.line)
		}
		i++ // skip \.{@@z}
		if len(c.match) == 0 {
			return nil, fmt.Errorf("change file line %d: the @@x match part is empty", c.line)
		}
		changes = append(changes, c)
	}
	return changes, nil
}

@ |applyChanges| is the string convenience form used by tests.
@(common/common.go@>=
func applyChanges(src string, changes []change, chFile string) (string, error) {
	out, _, err := applyChangesMapped(splitLines(src), nil, changes, chFile)
	if err != nil {
		return "", err
	}
	return strings.Join(out, "\n"), nil
}

@ |applyChangesMapped| applies the changes while keeping the origin map in
step: passed-through lines keep their origin, replacement lines are attributed
to the change file. It is an error if a change is never matched, or matches its
first line but not the rest.
@(common/common.go@>=
func applyChangesMapped(master []string, locs []srcLoc, changes []change, chFile string) ([]string, []srcLoc, error) {
	loc := func(i int) srcLoc {
		if locs != nil && i < len(locs) {
			return locs[i]
		}
		return srcLoc{line: i + 1}
	}
	out := make([]string, 0, len(master))
	var outLocs []srcLoc
	ci := 0
	for i := 0; i < len(master); {
		if ci < len(changes) && sameLine(master[i], changes[ci].match[0]) {
			if !blockMatches(master, i, changes[ci].match) {
				return nil, nil, fmt.Errorf("%s:%d: change did not match the master source at %s",
					chFile, changes[ci].line, loc(i))
			}
			for r, rl := range changes[ci].repl {
				out = append(out, rl)
				outLocs = append(outLocs, srcLoc{chFile, changes[ci].replLine + r})
			}
			i += len(changes[ci].match)
			ci++
			continue
		}
		out = append(out, master[i])
		outLocs = append(outLocs, loc(i))
		i++
	}
	if ci < len(changes) {
		return nil, nil, fmt.Errorf("%s:%d: change was never matched (looking for %q)",
			chFile, changes[ci].line, changes[ci].match[0])
	}
	return out, outLocs, nil
}

@ |blockMatches| reports whether a match block lines up with the master source
at a given index.
@(common/common.go@>=
func blockMatches(master []string, at int, match []string) bool {
	if at+len(match) > len(master) {
		return false
	}
	for k, m := range match {
		if !sameLine(master[at+k], m) {
			return false
		}
	}
	return true
}

@* Tests. The \.{common} package's tests, one section per case.
@(common/common_test.go@>=
package common

import (
	"os"
	"path/filepath"
	"testing"
)

@ @(common/common_test.go@>=
const sample = `\input gwebmac
This is limbo text.

@@* Introduction.
This program prints a greeting.
@@f println foo

@@ Here is the main function.
@@c
package main

func main() {
	@@<Print the greeting@@>
}

@@ The greeting itself.
@@<Print the greeting@@>=
println("hello, world")

@@ A section with no code, just prose.

@@*1 A deeper group.
@@<Print the greeting@@>=
println("again")
`

@ @(common/common_test.go@>=
func TestParseStructure(t *testing.T) {
	w := ParseString(sample)

	if got := len(w.Sections); got != 5 {
		t.Fatalf("section count = %d, want 5", got)
	}

	s1 := w.Sections[0]
	if !s1.Starred || s1.Depth != 0 {
		t.Errorf("section 1: starred=%v depth=%d, want starred depth 0", s1.Starred, s1.Depth)
	}
	if s1.Title != "Introduction" {
		t.Errorf("section 1 title = %q, want %q", s1.Title, "Introduction")
	}
	if len(s1.Formats) != 1 || s1.Formats[0].Original != "println" || s1.Formats[0].Like != "foo" {
		t.Errorf("section 1 formats = %+v", s1.Formats)
	}

	s2 := w.Sections[1]
	if s2.Name != "" || !s2.HasCode {
		t.Errorf("section 2 should be unnamed code, got name=%q hasCode=%v", s2.Name, s2.HasCode)
	}
	if !contains(s2.Code, "package main") || !contains(s2.Code, "@@<Print the greeting@@>") {
		t.Errorf("section 2 code missing pieces:\n%s", s2.Code)
	}

	s3 := w.Sections[2]
	if s3.Name != "Print the greeting" {
		t.Errorf("section 3 name = %q", s3.Name)
	}

	s4 := w.Sections[3]
	if s4.HasCode {
		t.Errorf("section 4 should be prose-only, got code %q", s4.Code)
	}

	s5 := w.Sections[4]
	if !s5.Starred || s5.Depth != 1 {
		t.Errorf("section 5: starred=%v depth=%d, want starred depth 1", s5.Starred, s5.Depth)
	}
	if s5.Name != "Print the greeting" {
		t.Errorf("section 5 name = %q", s5.Name)
	}
}

@ \.{@@**} is the top-level group (depth -1), printed bold in the contents, as
 \.{CWEB} does; \.{@@*} stays depth 0 and @@*n stays depth n.
@(common/common_test.go@>=
func TestDoubleStarDepth(t *testing.T) {
	w := ParseString("@@** Top.\n@@c\npackage main\n@@* Ordinary.\n@@ x\n@@*2 Deep.\n@@ y\n")
	want := []int{-1, 0, 2}
	var got []int
	for _, s := range w.Sections {
		if s.Starred {
			got = append(got, s.Depth)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("starred sections = %v, want depths %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("starred section %d: depth=%d, want %d", i, got[i], want[i])
		}
	}
}

@ @(common/common_test.go@>=
func TestResolveAbbrev(t *testing.T) {
	w := ParseString(sample)
	if got := w.Resolve("Print the..."); got != "Print the greeting" {
		t.Errorf("Resolve abbrev = %q, want %q", got, "Print the greeting")
	}
}

@ @(common/common_test.go@>=
func TestCodePragmaP(t *testing.T) {
	// \.{@@p} is a synonym for @@c (\.{CWEB} compatibility).
	w := ParseString("@@ x\n@@p\npackage main\n")
	if len(w.Sections) != 1 || !w.Sections[0].HasCode {
		t.Fatalf("@@p should begin a code section, got %+v", w.Sections)
	}
	if w.Sections[0].Name != "" {
		t.Errorf("@@p section should be unnamed, got name %q", w.Sections[0].Name)
	}
	if !contains(w.Sections[0].Code, "package main") {
		t.Errorf("@@p code missing: %q", w.Sections[0].Code)
	}
}

@ @(common/common_test.go@>=
func TestDefaultExt(t *testing.T) {
	cases := []struct{ name, ext, want string }{
		{"wc", ".w", "wc.w"},         // bare name gets the extension
		{"wc.w", ".w", "wc.w"},       // already has one: unchanged
		{"foo.bar", ".w", "foo.bar"}, // a different extension is respected
		{"dir/wc", ".w", "dir/wc.w"}, // path components are fine
		{"", ".ch", ""},              // empty (e.g. no change file) stays empty
	}
	for _, c := range cases {
		if got := DefaultExt(c.name, c.ext); got != c.want {
			t.Errorf("DefaultExt(%q, %q) = %q, want %q", c.name, c.ext, got, c.want)
		}
	}
}

@ @(common/common_test.go@>=
func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexFrom(s, sub, 0) >= 0
}

@ @(common/common_test.go@>=
func TestLimboFormats(t *testing.T) {
	w := ParseString(`\input gwebmac
@@f Counts int
@@s hidden int
@@ x
@@c
package main
`)
	if len(w.Formats) != 2 {
		t.Fatalf("limbo formats = %d, want 2: %+v", len(w.Formats), w.Formats)
	}
	if w.Formats[0].Original != "Counts" || w.Formats[0].Like != "int" || w.Formats[0].NoIndex {
		t.Errorf("format[0] = %+v", w.Formats[0])
	}
	if w.Formats[1].Original != "hidden" || !w.Formats[1].NoIndex {
		t.Errorf("format[1] = %+v", w.Formats[1])
	}
	if contains(w.Limbo, "@@f") || contains(w.Limbo, "@@s") {
		t.Errorf("directives not stripped from limbo: %q", w.Limbo)
	}
	if !contains(w.Limbo, "\\input gwebmac") {
		t.Errorf("limbo lost its TeX: %q", w.Limbo)
	}
}

@ @(common/common_test.go@>=
func hasWarning(ws []string, sub string) bool {
	for _, w := range ws {
		if indexFrom(w, sub, 0) >= 0 {
			return true
		}
	}
	return false
}

@ @(common/common_test.go@>=
func TestSectionLines(t *testing.T) {
	w := ParseString("limbo\n\n@@ first\n@@c\nx\n\n@@ second\n@@c\ny\n")
	if w.Sections[0].Line != 3 {
		t.Errorf("section 1 line = %d, want 3", w.Sections[0].Line)
	}
	if w.Sections[1].Line != 7 {
		t.Errorf("section 2 line = %d, want 7", w.Sections[1].Line)
	}
}

@ @(common/common_test.go@>=
func TestDiagnostics(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"unterminated", "@@ x\n@@c\ny := @@<oops\n", "unterminated"},
		{"undefined ref", "@@ x\n@@c\n@@<nope@@>\n", "undefined section <nope>"},
		{"never used", "@@ x\n@@<helper@@>=\ndoit()\n@@ y\n@@c\npackage main\n", "defined but never used"},
		{
			"ambiguous",
			"@@ a\n@@<Set X@@>=\n1\n@@ b\n@@<Set Y@@>=\n2\n@@ c\n@@c\n@@<Set...@@>\n",
			"ambiguous prefix <Set...>",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := ParseString(c.src)
			if !hasWarning(w.Warnings, c.want) {
				t.Errorf("want a warning containing %q, got %v", c.want, w.Warnings)
			}
		})
	}
}

@ @(common/common_test.go@>=
func TestChangeFileApply(t *testing.T) {
	master := "@@ greet\n@@c\npackage main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	chSrc := "Ignored commentary.\n@@x\n\tprintln(\"hello\")\n@@y\n\tprintln(\"goodbye\")\n@@z\n"
	changes, err := parseChangeFile(chSrc)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || len(changes[0].match) != 1 || len(changes[0].repl) != 1 {
		t.Fatalf("bad parse: %+v", changes)
	}
	out, err := applyChanges(master, changes, "c.ch")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(out, `println("goodbye")`) || contains(out, `println("hello")`) {
		t.Errorf("change not applied:\n%s", out)
	}
}

@ @(common/common_test.go@>=
func TestChangeFileNoMatch(t *testing.T) {
	master := "@@ x\n@@c\npackage main\n"
	changes, _ := parseChangeFile("@@x\nnonexistent line\n@@y\nwhatever\n@@z\n")
	if _, err := applyChanges(master, changes, "c.ch"); err == nil ||
		!contains(err.Error(), "never matched") {
		t.Errorf("want never-matched error, got %v", err)
	}
}

@ @(common/common_test.go@>=
func TestChangeFilePartialMismatch(t *testing.T) {
	master := "alpha\nbeta\ngamma\n"
	changes, _ := parseChangeFile("@@x\nbeta\nWRONG\n@@y\nx\n@@z\n")
	if _, err := applyChanges(master, changes, "c.ch"); err == nil ||
		!contains(err.Error(), "did not match") {
		t.Errorf("want did-not-match error, got %v", err)
	}
}

@ @(common/common_test.go@>=
func TestChangeFileMalformed(t *testing.T) {
	if _, err := parseChangeFile("@@x\nfind\n@@z\n"); err == nil {
		t.Error("want error for @@x without @@y")
	}
}

@ @(common/common_test.go@>=
func TestIncludeLineMapping(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	mustWrite("part.w", "@@ A section in the included file.\n@@c\nx := @@<undef@@>\n")
	main := mustWrite("main.w", "@@* Main.\n@@c\npackage main\n\n@@i part.w\n")

	w, err := Parse(main)
	if err != nil {
		t.Fatal(err)
	}
	// The undefined reference lives in part.w; the diagnostic must cite it
	// (not a line number in the includes-expanded master).
	if !hasWarning(w.Warnings, "part.w:1") {
		t.Errorf("want a warning citing part.w:1, got %v", w.Warnings)
	}
	// Section 2 (the \.{@@c} in \.{part.w}) should map back to \.{part.w}.
	if got := w.at(w.Sections[1].Line); !contains(got, "part.w") {
		t.Errorf("section 2 origin = %q, want a part.w location", got)
	}
}

@ The full name may appear only at a reference, with the definition
abbreviated -- and vice versa. Neither should warn.
@(common/common_test.go@>=
func TestResolveAbbrevEitherSide(t *testing.T) {
	srcs := []string{
		"@@ x\n@@c\nvar _ = @@<The parallel-map function@@>\n@@ d\n@@<The parallel...@@>=\n1\n",
		"@@ x\n@@c\nvar _ = @@<The parallel...@@>\n@@ d\n@@<The parallel-map function@@>=\n1\n",
	}
	for _, src := range srcs {
		w := ParseString(src)
		for _, bad := range []string{"undefined", "ambiguous", "never"} {
			if hasWarning(w.Warnings, bad) {
				t.Errorf("unexpected %q warning for %q: %v", bad, src, w.Warnings)
			}
		}
	}
}
