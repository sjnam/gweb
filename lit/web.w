@* The \.{web} package.
This package is the shared front end of GWEB. It reads a |.w| source file,
expands \.{@@i} includes, optionally applies a change file, and splits the
result into a sequence of {\it sections\/}. Both \.{gtangle} and \.{gweave}
are built on top of it, so it plays the role of CWEB's \.{common.w}.

We start with the package declaration, the imports, and the |Format| record,
which captures one \.{@@f} or \.{@@s} directive: typeset identifier |Original| the
way identifier or keyword |Like| is typeset; |NoIndex| is true for \.{@@s}.
@(internal/web/web.go@>=
// Package web parses GWEB source files (.w) into a sequence of sections that
// gtangle and gweave consume. It is the shared front end of the GWEB system,
// playing the role of CWEB's common.w.
package web

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Version is the GWEB release, shared by gtangle and gweave for their startup
// banner and their --version output.
const Version = "0.2.0"

// Format records an "@@f a b" (or "@@s a b") directive: typeset identifier
// Original the way identifier/keyword Like is typeset. NoIndex is true for @@s.
type Format struct {
	Original string
	Like     string
	NoIndex  bool
	Macro    bool // @@d: typeset Original in typewriter (a CWEB-style macro)
}

@ A |Section| is one numbered section of the web. Its three optional parts --
commentary, definitions, and code -- are stored as raw text with the in-text
and in-code \.{@@}-codes still embedded; the consumers interpret them later.
@(internal/web/web.go@>=
// Section is one numbered section of the web.
type Section struct {
	Number  int    // 1-based section number
	Line    int    // 1-based source line where the section begins
	Starred bool   // true for @@* sections
	Depth   int    // group depth for starred sections (-1 == @@**, 0 == @@*, n == @@*n)
	Title   string // starred-section title (text up to the first period)
	Tex     string // commentary, raw TeX with in-text @@-codes still embedded
	Formats []Format
	HasCode bool   // true if the section contributes code
	Name    string // named-section name, or "" for an unnamed @@c section
	IsFile  bool   // true if the name is an output file (@@(file@@>=)
	Code    string // raw code text with in-code @@-codes still embedded
	CodeLine int   // 1-based combined-source line where Code begins (0 if none)
}

@ A |Web| is a fully parsed GWEB document: the limbo text, the global format
directives, the sections, and the diagnostics gathered while parsing. The
unexported fields support diagnostics (mapping a combined-source line back to
its original file) and name-abbreviation resolution.
@(internal/web/web.go@>=
// Web is a fully parsed GWEB document.
type Web struct {
	Limbo    string
	Formats  []Format // @@f / @@s directives found in limbo (apply globally)
	Sections []*Section
	Warnings []string // non-fatal diagnostics gathered while parsing/checking
	file     string   // source filename, for diagnostics ("" if unknown)
	locs     []srcLoc // origin (file, line) of each combined-source line
	full     []string // canonical (non-abbreviated) section names
}

@ The parsing entry points. |Parse| handles the common case; |ParseWithChange|
adds CWEB's change-file mechanism; |ParseString| is for tests. All three end by
calling |finish|, which runs the post-parse bookkeeping. |at| formats a
combined-source line for a diagnostic, mapping it back to the user's file.
@(internal/web/web.go@>=
// Parse reads filename, expands @@i includes, and parses the result.
func Parse(filename string) (*Web, error) {
	return ParseWithChange(filename, "")
}

// ParseWithChange reads the master file, expands @@i includes, applies the change
// file (CWEB's ".ch" mechanism) if changeFile is non-empty, and parses the
// result. Diagnostics point back to the original file and line via an origin map
// kept in step through include expansion and change application.
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

// ParseString parses already-loaded source (used by tests; no includes).
func ParseString(src string) *Web {
	w := parse(src)
	w.finish(src)
	return w
}

// finish runs post-parse bookkeeping: name collection and diagnostics.
func (w *Web) finish(src string) {
	w.collectNames()
	w.Warnings = append(w.Warnings, w.scanDiagnostics(src)...)
	w.Warnings = append(w.Warnings, w.checkNames()...)
}

// at formats a combined-source line for a diagnostic, mapping it back to the
// original file and line when an origin map is available.
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
string). |gtangle| uses it to emit \.{//line} directives so the Go compiler
reports errors at \.{.w} positions.
@(internal/web/web.go@>=
// Origin maps a 1-based combined-source line back to the original file and line.
// When no origin map is available it falls back to the web's own filename.
func (w *Web) Origin(line int) (file string, ln int) {
	if i := line - 1; i >= 0 && i < len(w.locs) {
		return w.locs[i].file, w.locs[i].line
	}
	return w.file, line
}

@ |DefaultExt| supplies a default extension when the user omits one, so the
commands accept a bare web name as CWEB does (|gtangle wc| reads \.{wc.w}). A
name that already has an extension, or is empty, is returned unchanged.
@(internal/web/web.go@>=
// DefaultExt returns name with ext appended when name has no extension of its
// own (and is non-empty), so "wc" becomes "wc.w". A name that already carries an
// extension is left alone.
func DefaultExt(name, ext string) string {
	if name == "" || filepath.Ext(name) != "" {
		return name
	}
	return name + ext
}

@ Include expansion. As in CWEB, \.{@@i} is line-oriented: a line whose first
non-blank text is \.{@@i} names a file whose expansion replaces that line. We
keep a parallel origin map so diagnostics can cite the file the user wrote.
@(internal/web/web.go@>=
// expandIncludes reads file and splices in @@i includes, returning the combined
// lines together with a parallel origin map. As in CWEB, @@i is line-oriented: a
// line whose first non-blank text is "@@i" (followed by whitespace) names a file
// whose expansion replaces that line. A final newline does not produce a
// trailing blank line.
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

// includeDirective returns the file named by an "@@i" line, or ok=false. The @@i
// must be the first non-blank text on the line and be followed by whitespace.
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

@ Collecting section names. To resolve an abbreviation ending in |...| we need
the set of canonical (non-abbreviated) names. A full name may appear at a
definition or at any reference, in code or in commentary, so all of those are
scanned.
@(internal/web/web.go@>=
// collectNames records the set of canonical (non-abbreviated) section names so
// abbreviations ending in "..." can be resolved. A full name may appear at a
// definition (@@<name@@>=) or at any reference (@@<name@@>), in code or in
// commentary, so all of those are scanned. Output files (@@(...@@>=) are roots,
// not referable refinements, so their names are excluded.
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

// prefixMatches counts canonical names beginning with prefix.
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
@(internal/web/web.go@>=
// checkNames validates @@<...@@> references: it reports ambiguous or unmatched
// abbreviations, references to undefined sections, and named sections that are
// defined but never used. All are warnings (gtangle still fails hard if it
// actually meets an undefined reference while expanding).
func (w *Web) checkNames() []string {
	// "defined" is the set of sections that actually have a definition (not just
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

@ A few small helpers: |lineAt| converts a byte offset to a line number,
|Resolve| maps a possibly abbreviated name to its canonical form, and
|indexFrom| is a bounded |strings.Index|.
@(internal/web/web.go@>=
// lineAt returns the 1-based line number of byte offset off in src.
func lineAt(src string, off int) int {
	if off > len(src) {
		off = len(src)
	}
	return 1 + strings.Count(src[:off], "\n")
}

// canonName canonicalizes a section name's whitespace: every run of spaces,
// tabs, and newlines becomes a single space, and leading/trailing space is
// dropped. As in CWEB, this lets a long name that is wrapped across lines in one
// place still match the same name written on a single line elsewhere.
func canonName(name string) string {
	return strings.Join(strings.Fields(name), " ")
}

// Resolve maps a (possibly abbreviated) name to its canonical form. An
// abbreviation "Prefix..." matches the unique full name starting with Prefix.
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
@(internal/web/parse.go@>=
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
	cCode  // @@c (or its synonym @@p)
	cNamed // @@<name@@>= or @@(file@@>=
	cDefn  // @@d
	cFormat
)

type ctrl struct {
	kind    ctrlKind
	pos     int    // index of the leading '@@'
	end     int    // index just past the control token
	depth   int    // for cSection: -1 unstarred (or @@** top level), else starred depth
	starred bool   // for cSection (distinguishes @@** from an unstarred section)
	name    string // for cNamed
	isFile  bool   // for cNamed (@@( vs @@<)
	noIndex bool   // for cFormat (@@s)
}

@ |scanStruct| finds the next structural control at or after |i|. It skips
literal \.{@@@@} and argument-terminated codes (\.{@@<...@@>}, \.{@@=...@@>}, and so
on) so their contents never trigger a false section break. A \.{@@<...@@>} not
followed by |=| is a reference, not a definition, and is skipped.
@(internal/web/parse.go@>=
// scanStruct finds the next structural control at or after i. It skips literal
// "@@@@" and argument-terminated codes (@@<...@@>, @@=...@@>, etc.) so their contents
// never trigger a false section break. A "@@<...@@>" not followed by "=" is a
// reference, not a definition, and is skipped.
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
				depth = -1 // "@@**" is the top level: bold in the contents, as cweb
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
@(internal/web/parse.go@>=
// findNextSection scans forward to the next section break (@@ or @@*), skipping
// everything else including argument-terminated codes. Used inside code parts,
// where @@c/@@d/@@f never legitimately appear.
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
				depth = -1 // "@@**" is the top level: bold in the contents, as cweb
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
each section into its TeX, definition, and code parts.
@(internal/web/parse.go@>=
// parse splits source into limbo and sections.
func parse(src string) *Web {
	w := &Web{}
	n := len(src)

	// Limbo runs until the first section break. Format directives placed there
	// (@@f / @@s, a common CWEB idiom) are extracted and removed from the copied
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

		// Definition part: a run of @@d / @@f / @@s.
		for ct.kind == cDefn || ct.kind == cFormat {
			nx := scanStruct(src, ct.end)
			seg := src[ct.end:nx.pos]
			// @@d has no Go analogue (Go has no preprocessor), so it never tangles
			// to code; gweave uses it only to set the named identifier in
			// typewriter, as cweave sets a macro. @@f/@@s format like another word.
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
@(internal/web/parse.go@>=
func findSectionHeaderEnd(src string, i int) ctrl {
	n := len(src)
	j := i + 2
	depth := 0
	if j < n && src[j] == '*' {
		j++
		depth = -1 // "@@**" is the top level: bold in the contents, as cweb
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
@(internal/web/parse.go@>=
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

@ |scanDiagnostics| walks the source looking for malformed control codes --
currently argument-terminated codes missing their closing \.{@@>}.
@(internal/web/parse.go@>=
// scanDiagnostics walks the source looking for malformed control codes —
// currently argument-terminated codes (@@<, @@(, @@=, @@t, @@^, @@., @@:, @@q) that are
// missing their closing @@> — and returns one warning per problem.
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
@(internal/web/parse.go@>=
// parseFormat parses the body of an @@f/@@s directive: two identifiers.
func parseFormat(seg string, noIndex bool) (Format, bool) {
	fields := strings.Fields(seg)
	if len(fields) < 2 {
		return Format{}, false
	}
	return Format{Original: fields[0], Like: fields[1], NoIndex: noIndex}, true
}

@ |parseMacro| parses the body of an \.{@@d} directive. Its first word names a
constant to set in typewriter (like a CWEB macro); any value after it is ignored,
since Go has no preprocessor and \.{@@d} never tangles to code. A qualified name
keeps its final component, so \.{@@d http.StatusOK} and \.{@@d StatusOK} both
register the identifier |StatusOK|.
@(internal/web/parse.go@>=
// parseMacro parses an @@d directive: its first word names a constant to set in
// typewriter; any value after it is ignored (Go has no preprocessor). A
// qualified name keeps its final component, so "@@d http.StatusOK" and
// "@@d StatusOK" both register StatusOK.
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
@(internal/web/parse.go@>=
// extractLimboFormats pulls @@d/@@f/@@s directives out of the limbo text
// (consuming each to end of line) and returns the cleaned text together with the
// formats. Other control codes and argument-terminated groups are copied through.
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
A code part is a mix of ordinary Go text and in-code control codes. |ScanCode|
turns it into a slice of |Atom|s; the kind of each atom tells \.{gtangle} and
\.{gweave} how to treat it.
@(internal/web/code.go@>=
package web

import "strings"

// AtomKind classifies a piece of a code part.
type AtomKind int

const (
	AText     AtomKind = iota // ordinary Go source text
	ARef                      // @@<name@@> reference to a named section
	AVerbatim                 // @@=text@@> passed verbatim to tangled output
	ATeX                      // @@t text@@> TeX text for the woven output
	AIndex                    // @@^/@@./@@: index entry
	APaste                    // @@& join (delete surrounding whitespace)
	ALayout                   // @@, @@/ @@| @@# woven-output layout hints
	AIndexDef                 // @@! force the next identifier to index as a definition
)

// Atom is one element of a scanned code part.
type Atom struct {
	Kind  AtomKind
	Text  string // payload for AText/AVerbatim/ATeX/AIndex; name for ARef
	Index byte   // '^','.',':' for AIndex; ',' '/' '|' '#' for ALayout
}

@ The scanner itself. \.{@@@@} becomes a literal \.{@@} folded into the surrounding
text; every other control code flushes the pending text and appends its own
atom.
@(internal/web/code.go@>=
// ScanCode splits a raw code part into atoms, interpreting in-code control
// codes. "@@@@" becomes a literal '@@' folded into the surrounding text.
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
			// CWEB prettyprinter hints (cancel break, expression brackets,
			// invisible semicolon). GWEB mirrors the source instead of reflowing
			// it, so these have no effect; accept and drop them for portability.
			i += 2
		default:
			i += 2 // unknown @@x: drop it rather than corrupt the output
		}
	}
	flush()
	return atoms
}

@* Change files.
A change file (CWEB's |.ch| mechanism) patches the master source without
editing it. It is a sequence of changes, each finding a block of lines in the
master and substituting a replacement block.
@(internal/web/change.go@>=
package web

import (
	"fmt"
	"strings"
)

// A change file (CWEB's ".ch" mechanism) patches the master source without
// editing it. It is a sequence of changes, each of the form
//
//	@@x
//	<lines to find in the master source>
//	@@y
//	<lines to substitute>
//	@@z
//
// Text outside an @@x...@@z group is ignored (it serves as commentary). Changes
// are matched against the master source — after @@i includes are expanded — in
// the order they appear: GWEB scans the master line by line, and at the first
// line equal to a change's first match line it requires the whole match block
// to match, then substitutes the replacement lines.

@ A |change| records the lines to find and the lines to substitute, plus
change-file line numbers for diagnostics. A |srcLoc| identifies the origin of a
line of the expanded, change-applied source.
@(internal/web/change.go@>=
type change struct {
	match    []string // lines to find in the master source
	repl     []string // lines to substitute for them
	line     int      // 1-based line of the @@x in the change file (for diagnostics)
	replLine int      // 1-based change-file line of the first replacement line
}

// srcLoc identifies the origin (file and 1-based line) of a line of the
// includes-expanded, change-applied source, so diagnostics can point back to
// the file the user actually wrote.
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
trailing whitespace, as WEB does), with a line splitter that normalizes CRLF.
@(internal/web/change.go@>=
// isChangeCtrl reports whether line begins with the change control "@@<c>"
// (c is 'x', 'y', or 'z'), which must start in the first column.
func isChangeCtrl(line string, c byte) bool {
	return len(line) >= 2 && line[0] == '@@' && line[1] == c
}

// splitLines splits text into lines, normalizing CRLF, so that joining the
// result with "\n" reproduces the (LF-normalized) input.
func splitLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}

// sameLine compares two source lines for change matching, ignoring trailing
// whitespace (as WEB does).
func sameLine(a, b string) bool {
	return strings.TrimRight(a, " \t") == strings.TrimRight(b, " \t")
}

@ |parseChangeFile| parses change-file text into an ordered list of changes,
reporting the malformed-change errors precisely.
@(internal/web/change.go@>=
// parseChangeFile parses change-file text into an ordered list of changes.
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
		i++ // skip @@y
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
		i++ // skip @@z
		if len(c.match) == 0 {
			return nil, fmt.Errorf("change file line %d: the @@x match part is empty", c.line)
		}
		changes = append(changes, c)
	}
	return changes, nil
}

@ |applyChanges| is the string convenience form used by tests.
@(internal/web/change.go@>=
// applyChanges returns src with the changes applied (string convenience form,
// used by tests). See applyChangesMapped for the origin-tracking version.
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
@(internal/web/change.go@>=
// applyChangesMapped applies changes to master, keeping a parallel origin map in
// step: passed-through lines keep their origin, and replacement lines are
// attributed to the change file. locs may be nil if origins are not tracked.
// chFile names the change file for diagnostics. It is an error if a change's
// first line is never found, or is found but the rest of the block does not
// match.
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
@(internal/web/change.go@>=
// blockMatches reports whether match lines up with master starting at index at.
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
