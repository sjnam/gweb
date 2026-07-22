@i types.w
\def\title{Common code for GTANGLE and GWEAVE (Version 0.9.5)}
\def\topofcontents{\null\vfill
  \centerline{\titlefont Common code for {\ttitlefont GTANGLE} and
    {\ttitlefont GWEAVE}}
  \vskip 15pt
  \centerline{(Version 0.9.5)}
  \vfill}
\def\botofcontents{\vfill\centerline{\smallfont
  Copyright \copyright\ 2026 Soojin Nam. MIT License.}}

@** Package common.
This package is the shared front end of \.{GWEB}. It reads a \.{.w} source file,
expands \.{@@i} includes, optionally applies a change file, and splits the
result into a sequence of {\it sections\/}. Both \.{gtangle} and \.{gweave}
are built on top of it, so it plays the role of \.{CWEB}'s \.{common.w}.

The skeleton is the package declaration, the imports, and the |Version| constant
(the \.{GWEB} release, shared by \.{gtangle} and \.{gweave} for their startup
banner and their \.{-version} output). The named sections that follow supply the
records and the parsing machinery, in reading order.
@d Version
@c
package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const Version = "0.9.5"

@<Records shared across the web@>
@<Parse a web from a file@>
@<Map a combined-source line back to its origin@>
@<Supply a default file extension@>
@<Expand include files@>
@<Collect and resolve section names@>
@<Small parser helpers@>

@ The |Format| record captures one directive's request for a single identifier:
an \.{@@f} or \.{@@s} asks that identifier |Original| be typeset the way identifier
or keyword |Like| is. |NoIndex| is true for \.{@@s}, and |Macro| is true for a
name listed in \.{@@d} (one |Format| per name, since \.{@@d} may name several).
@<Records shared across the web@>=
type Format struct {
	Original string
	Like     string
	NoIndex  bool
	Macro    bool // \.{@@d}: typeset Original in \.{typewriter} (a \.{CWEB}-style macro)
}

@ A |Section| is one numbered section of the web. Its three optional parts --
commentary, definitions, and code -- are stored as raw text with the in-text
and in-code \.{@@}-codes still embedded; the consumers interpret them later.
@<Records shared across the web@>=
type Section struct {
	Number  int    // 1-based section number
	Line    int    // 1-based source line where the section begins
	Starred bool   // |true| for \.{@@*} sections
	Depth   int    // group depth for starred sections ($-1\equiv{}$\.{@@**}, $0\equiv{}$\.{@@*}, $n\equiv{}$\.{@@*n})
	Title   string // starred-section title (text up to the first period)
	Tex     string // commentary, raw \TEX/ with in-text \.{@@}-codes still embedded
	Formats []Format
	HasCode bool   // |true| if the section contributes code
	Name    string // named-section name, or \.{""} for an unnamed @@c section
	IsFile  bool   // |true| if the name is an output file (\.{@@(file@@>=})
	Code    string // raw code text with in-code \.{@@}-codes still embedded
	CodeLine int   // 1-based combined-source line where |Code| begins (0 if none)
}

@ A |Web| is a fully parsed \.{GWEB} document: the limbo text, the global format
directives, the sections, and the diagnostics gathered while parsing. The
unexported fields support diagnostics (mapping a combined-source line back to
its original file) and name-abbreviation resolution.
@<Records shared across the web@>=
type Web struct {
	Limbo    string
	Formats  []Format // \.{@@f}/\.{@@s} directives found in limbo (apply globally)
	Sections []*Section
	Warnings []string // non-fatal diagnostics gathered while parsing/checking
	file     string   // source filename, for diagnostics (\.{""} if unknown)
	locs     []srcLoc // origin (file, line) of each combined-source line
	full     []string // canonical (non-abbreviated) section names
}

@ The file-based entry points. |Parse| handles the common case; |ParseWithChange|
adds \.{CWEB}'s change-file mechanism. Each expands \.{@@i} includes, joins the
result into one combined source string, parses it, and records where it came
from before running the shared bookkeeping.
@<Parse a web from a file@>=
func Parse(filename string) (*Web, error) {
	return ParseWithChange(filename, "")
}
@#
func ParseWithChange(filename, changeFile string) (*Web, error) {
	lines, locs, err := expandIncludes(filename, 0)
	if err != nil {
		return nil, err
	}
	@<Apply the change file, if any@>
	src := strings.Join(lines, "\n")
	w := parse(src)
	w.file = filename
	w.locs = locs
	w.finish(src)
	return w, nil
}

@ The change file (\.{CWEB}'s \.{.ch} mechanism) is optional. When one is named
we read it, parse it into an ordered list of changes, and splice them into the
included source in step with the origin map. Any failure aborts the parse.
@<Apply the change file, if any@>=
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

@ |ParseString| is the string entry point used by the tests. It and the
file-based entries all end in |finish|, the post-parse bookkeeping that resolves
section names and gathers the parse-time diagnostics.
@<Parse a web from a file@>=
func ParseString(src string) *Web {
	w := parse(src)
	w.finish(src)
	return w
}
@#
func (w *Web) finish(src string) {
	w.collectNames()
	w.Warnings = append(w.Warnings, w.scanDiagnostics(src)...)
	w.Warnings = append(w.Warnings, w.checkNames()...)
}

@ Two views of the origin map. |Origin| maps a combined-source line back to the
file and line the user wrote, returning the two parts separately; \.{gtangle}
uses it to emit \.{//line} directives so the \GO/ compiler reports errors at
\.{.w} positions. |at| formats the same information as a string for a
diagnostic, falling back to the source filename or a bare line number.
@<Map a combined-source line back to its origin@>=
func (w *Web) Origin(line int) (file string, ln int) {
	if i := line - 1; i >= 0 && i < len(w.locs) {
		return w.locs[i].file, w.locs[i].line
	}
	return w.file, line
}
@#
func (w *Web) at(line int) string {
	if i := line - 1; i >= 0 && i < len(w.locs) {
		return w.locs[i].String()
	}
	if w.file != "" {
		return fmt.Sprintf("%s:%d", w.file, line)
	}
	return fmt.Sprintf("line %d", line)
}

@ |DefaultExt| supplies a default extension when the user omits one, so the
commands accept a bare web name as \.{CWEB} does (\.{gtangle wc} reads \.{wc.w}). A
name that already has an extension, or is empty, is returned unchanged.
@<Supply a default file extension@>=
func DefaultExt(name, ext string) string {
	if name == "" || filepath.Ext(name) != "" {
		return name
	}
	return name + ext
}

@ Include expansion. As in \.{CWEB}, \.{@@i} is line-oriented: a line whose first
non-blank text is \.{@@i} names a file whose expansion replaces that line. We
keep a parallel origin map so diagnostics can cite the file the user wrote.
@<Expand include files@>=
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

	var lines, hoisted []string
	var locs, hoistedLocs []srcLoc
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
			@<Take the include's limbo directives out of line@>
			continue
		}
		lines = append(lines, line)
		locs = append(locs, srcLoc{file, i + 1})
	}
	return append(hoisted, lines...), append(hoistedLocs, locs...), nil
}

@ An included file that is also a web in its own right opens with its own limbo:
a run of \.{@@d}/\.{@@f}/\.{@@s} that applies to the whole of it. Spliced in where
the \.{@@i} sits, that run would land in the middle of the host document---after
some section's code part, if the \.{@@i} follows one---where it is no longer a
definition part at all, and would be set as though it were program text.

So we lift the run out and put it back at the front, ahead of everything this
file contributes. Each level does the same, so the directives rise until they
reach the master's own limbo, where |extractLimboFormats| registers them
globally---which is what they meant in the included web to begin with. The
origin map rides along, so diagnostics still cite the file the directives were
written in.
@<Take the include's limbo directives out of line@>=
h, rest, hLocs, restLocs := hoistLimboDirectives(sub, subLocs)
hoisted = append(hoisted, h...)
hoistedLocs = append(hoistedLocs, hLocs...)
lines = append(lines, rest...)
locs = append(locs, restLocs...)

@ The run is the leading directive lines, blank lines between them allowed; it
ends at the first line that is neither. Anything else at the top of the file---a
\TeX\ comment, a section break---stops the scan, so only a genuine limbo run is
lifted.
@<Expand include files@>=
func hoistLimboDirectives(lines []string, locs []srcLoc) (h, rest []string, hLocs, restLocs []srcLoc) {
	end := 0
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			continue // a blank line inside the run
		}
		if !isLimboDirective(t) {
			break
		}
		end = i + 1
	}
	return lines[:end], lines[end:], locs[:end], locs[end:]
}

@ @<Expand include files@>=
func isLimboDirective(t string) bool {
	if len(t) < 2 || t[0] != '@@' {
		return false
	}
	switch t[1] {
	case 'd', 'f', 's':
		return len(t) == 2 || t[2] == ' ' || t[2] == '\t'
	}
	return false
}

@ @<Expand include files@>=
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
@<Collect and resolve section names@>=
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

@ |prefixMatches| counts how many canonical names begin with a given prefix, so
|checkNames| can tell an unmatched abbreviation from an ambiguous one.
@<Collect and resolve section names@>=
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

The |defined| set holds only the sections that actually have a definition---%
not the full names known for abbreviation resolution, which also include names
that appear only at references.
@<Collect and resolve section names@>=
func (w *Web) checkNames() []string {
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

@ A few small helpers used throughout the parser: |lineAt| converts a byte
offset to a line number, |canonName| normalizes a section name by collapsing
each run of whitespace to a single space, and |indexFrom| is a bounded
|strings.Index| that searches only from a given offset.
@<Small parser helpers@>=
func lineAt(src string, off int) int {
	if off > len(src) {
		off = len(src)
	}
	return 1 + strings.Count(src[:off], "\n")
}
@#
func canonName(name string) string {
	return strings.Join(strings.Fields(name), " ")
}
@#
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

@ |Resolve| maps a possibly abbreviated name to its canonical form. A name that
does not end in \.{...} is already canonical (bar whitespace); otherwise its
prefix must match exactly one full name, and a zero or ambiguous match is left
unresolved for the caller to report.
@<Collect and resolve section names@>=
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

@* Scanning structural controls.
The parser works directly on the source text. The pieces below describe the
structural \.{@@}-codes, scan the source for them, run the main loop that splits
it into sections, and parse the definition-part directives.
@c
@<Structural-control descriptors@>
@<Scan forward to a control code@>
@<The main parse loop@>
@<Scan for malformed control codes@>
@<Parse the definition-part directives@>

@ A |ctrl| value describes the next structural control code -- a section break, a
code part, a named definition, or a definition-part directive -- together with
the byte range it occupies.
@<Structural-control descriptors@>=
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
	depth   int    // for |cSection|: -1 unstarred (or \.{@@**} top level), else starred depth
	starred bool   // for |cSection| (distinguishes \.{@@**} from an unstarred section)
	name    string // for |cNamed|
	isFile  bool   // for |cNamed| (\.{@@(} vs \.{@@<})
	noIndex bool   // for |cFormat| (\.{@@s})
}

@ |scanStruct| finds the next structural control at or after |i|. It skips
literal \.{@@@@} and argument-terminated codes (\.{@@<...@@>}, \.{@@=...@@>}, and so
on) so their contents never trigger a false section break. A \.{@@<...@@>} not
followed by |=| is a reference, not a definition, and is skipped.
@<Scan forward to a control code@>=
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
			@<Read a starred-section break@>
		case c == 'c' || c == 'p':
			return ctrl{kind: cCode, pos: i, end: i + 2}
		case c == 'd':
			return ctrl{kind: cDefn, pos: i, end: i + 2}
		case c == 'f':
			return ctrl{kind: cFormat, pos: i, end: i + 2}
		case c == 's':
			return ctrl{kind: cFormat, pos: i, end: i + 2, noIndex: true}
		case c == '<' || c == '(':
			@<Classify a named-section definition@>
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
\.{@@f} never legitimately appear---a section's directives belong to its
definition part, and |hoistLimboDirectives| keeps an included web's own from
drifting past it.
@<Scan forward to a control code@>=
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
			@<Read a starred-section break@>
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

@ A starred section may be \.{@@**} (the top level, depth $-1$, set bold in the
contents as \.{CWEB} does), \.{@@*} (depth 0), or \.{@@*n} (depth |n|). Both
scanners read past the stars and any digits and return the break; the shared
code keeps the two in step.
@<Read a starred-section break@>=
j := i + 2
depth := 0
if j < n && src[j] == '*' {
	j++
	depth = -1
} else {
	for j < n && src[j] >= '0' && src[j] <= '9' {
		depth = depth*10 + int(src[j]-'0')
		j++
	}
}
return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}

@ A \.{@@<...@@>} or \.{@@(...@@>} begins a definition only when it is followed
(after optional blanks) by |=|; otherwise it is a reference, whose contents are
skipped so they cannot trigger a false section break.
@<Classify a named-section definition@>=
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

@ The main loop. |parse| splits the source into limbo and sections, and for
each section into its \TEX/, definition, and code parts.
Limbo runs until the first section break. Format directiveas placed there
(\.{@@f}/\.{@@s}, a common \.{CWEB} idiom) are extracted and removed from the copied
\TEX/ so they apply globally rather than printing literally.
@<The main parse loop@>=
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

		@<Collect the section's definition part@>
		@<Collect the section's code part@>

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

@ The definition part is a run of \.{@@d} / \.{@@f} / \.{@@s} directives. Each only
registers a format hint on the section; none of them tangles to code (see the
definition-part directive sections above).
@<Collect the section's definition part@>=
for ct.kind == cDefn || ct.kind == cFormat {
	nx := scanStruct(src, ct.end)
	seg := src[ct.end:nx.pos]
	if ct.kind == cDefn {
		sec.Formats = append(sec.Formats, parseMacro(seg)...)
	} else if f, ok := parseFormat(seg, ct.noIndex); ok {
		sec.Formats = append(sec.Formats, f)
	}
	ct = nx
}

@ The code part, if any, runs to the next section break: an unnamed \.{@@c}
section feeds the program text, while \.{@@<name@@>=} or \.{@@(file@@>=} names its
destination. Anything else is a documentation-only section.
@<Collect the section's code part@>=
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
default: // |cSection| or |cEOF|: a documentation-only section
	i = ct.pos
}

@ |findSectionHeaderEnd| locates the end of a \.{@@*} header and its depth.
@<The main parse loop@>=
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
@<The main parse loop@>=
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
@<Scan for malformed control codes@>=
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
@<Parse the definition-part directives@>=
func parseFormat(seg string, noIndex bool) (Format, bool) {
	fields := strings.Fields(seg)
	if len(fields) < 2 {
		return Format{}, false
	}
	return Format{Original: fields[0], Like: fields[1], NoIndex: noIndex}, true
}

@ |parseMacro| parses the body of an \.{@@d} directive. Where \.{CWEB}'s \.{@@d}
names one macro and gives its replacement text, \GO/ has no preprocessor, so
\.{GWEB} borrows the code for a lighter purpose: every whitespace-separated word
of the body---the body runs to the next \.{@@}, so it may span several lines---%
is an identifier to set in typewriter, like a \.{CWEB} macro. A qualified name
keeps its final component, so `\.{@@d http.StatusOK}' registers \.{StatusOK}, and
`\.{@@d Push Pop Peek}' sets all three at once.
@<Parse the definition-part directives@>=
func parseMacro(seg string) []Format {
	var fs []Format
	for _, field := range strings.Fields(seg) {
		name := field
		if k := strings.LastIndex(name, "."); k >= 0 {
			name = name[k+1:]
		}
		if name != "" {
			fs = append(fs, Format{Original: name, Macro: true})
		}
	}
	return fs
}

@ |extractLimboFormats| pulls \.{@@d}/\.{@@f}/\.{@@s} directives out of the limbo text and
returns the cleaned text together with the formats. A \.{@@q...@@>} source comment
is dropped, as it is everywhere; other control codes and argument-terminated
groups are copied through unchanged.
@<Parse the definition-part directives@>=
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
			@<Extract one limbo directive@>
		case 'q':
			if end := indexFrom(src, "@@>", i+2); end < 0 {
				i = n // unterminated: drop the rest of limbo
			} else {
				i = end + 2 // drop the source-only comment
			}
		case '<', '(', '=', 't', '^', '.', ':':
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

@ An \.{@@f}/\.{@@s} directive is exactly its control code and two identifiers, so
several may share a line---as \.{cweave}'s own manual does with \.{@@f x1 TeX
@@f x2 TeX}. We therefore scan just those two words and let the loop pick up any
directive that follows, rather than swallowing the rest of the line. A \.{@@d}
lists identifiers to set in typewriter, so it runs to the next \.{@@} (it may span
several lines) and each word becomes a format. Either way, once the directive is
parsed, a newline with nothing but blanks before it is dropped, so a line given
over to directives leaves no blank line in the copied \TEX/.
@<Extract one limbo directive@>=
var fs []Format
var j int
if c == 'd' {
	j = i + 2
	for j < n && src[j] != '@@' {
		j++ // the body runs to the next control code
	}
	fs = parseMacro(src[i+2 : j])
} else {
	j = endOfFormatArgs(src, i+2, n)
	if f, ok := parseFormat(src[i+2:j], c == 's'); ok {
		fs = []Format{f}
	}
}
formats = append(formats, fs...)
if k := skipBlanks(src, j, n); k < n && src[k] == '\n' {
	j = k + 1 // the directive ended its line; drop the blanks and the newline
}
i = j

@ |endOfFormatArgs| returns the index just past the second whitespace-delimited
word starting at |p|---the |l| and |r| of an \.{@@f}/\.{@@s} directive. |skipBlanks|
runs past spaces and tabs; a directive that ends its line meets a newline next.
@<Parse the definition-part directives@>=
func endOfFormatArgs(src string, p, n int) int {
	for word := 0; word < 2; word++ {
		p = skipBlanks(src, p, n)
		for p < n && src[p] != ' ' && src[p] != '\t' && src[p] != '\n' {
			p++
		}
	}
	return p
}

func skipBlanks(src string, p, n int) int {
	for p < n && (src[p] == ' ' || src[p] == '\t') {
		p++
	}
	return p
}

@* Scanning a code part into atoms.
A code part is a mix of ordinary \GO/ text and in-code control codes. |ScanCode|
turns it into a slice of |Atom|s; the kind of each atom tells \.{gtangle} and
\.{gweave} how to treat it.
@c
@<Code-part atom descriptors@>
@<Scan a code part into atoms@>

@ @<Code-part atom descriptors@>=
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
@<Scan a code part into atoms@>=
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
		@<Scan one in-code control code@>
	}
	flush()
	return atoms
}

@ Each in-code \.{@@}-code flushes the pending text and appends its own atom (or,
for the layout and prettyprinter hints, is simply recorded or dropped). The switch
falls into three parts: the codes that fold or join text, the argument-terminated
forms that read up to a closing \.{@@>}, and the one- and two-character layout and
prettyprinter hints.
@<Scan one in-code control code@>=
switch d := code[i+1]; d {
@<Fold a literal atsign, or join a paste@>
@<Scan an argument-terminated code@>
@<Record a layout hint, or drop a prettyprinter hint@>
}

@ A doubled \.{@@@@} folds to a single literal \.{@@} in the surrounding text;
\.{@@\&} flushes the pending text and emits a paste atom, which later deletes the
whitespace around it.
@<Fold a literal atsign, or join a paste@>=
case '@@':
	buf.WriteByte('@@')
	i += 2
case '&':
	flush()
	atoms = append(atoms, Atom{Kind: APaste})
	i += 2

@ The argument-terminated forms (\.{@@<...@@>}, \.{@@=...@@>}, and so on) read up to
their closing \.{@@>}; an unterminated one ends the scan. A reference keeps the raw
text between the brackets; a verbatim, \TEX/, or index atom keeps its own payload;
and a \.{@@q} comment is read and thrown away.
@<Scan an argument-terminated code@>=
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

@ The layout hints \.{@@,} \.{@@/} \.{@@\|} \.{@@\#} (a thin space, a line break, an
optional line break, and a break preceded by a blank line) become |ALayout|
atoms for \.{gweave} and are ignored by \.{gtangle}. \.{@@!} forces the next
identifier to index as a definition, overriding the heuristic, and produces no
output by itself. The \.{CWEB} prettyprinter hints \.{@@+} \.{@@[} \.{@@]} \.{@@;}
(cancel break, expression brackets, invisible semicolon) have no effect here---%
\.{GWEB} mirrors the source rather than reflowing it---so they are accepted and
dropped for portability. A \.{@@\%} comment runs to the end of its line, a stray
\.{@@>} is skipped, and any unknown \.{@@x} is dropped rather than left to corrupt
the output.
@<Record a layout hint, or drop a prettyprinter hint@>=
case '%':
	j := i + 2
	for j < n && code[j] != '\n' {
		j++
	}
	i = j
case '>':
	i += 2 // stray terminator
case ',', '/', '|', '#':
	flush()
	atoms = append(atoms, Atom{Kind: ALayout, Index: d})
	i += 2
case '!':
	flush()
	atoms = append(atoms, Atom{Kind: AIndexDef})
	i += 2
case '+', '[', ']', ';':
	i += 2 // \.{CWEB} prettyprinter hints, dropped
default:
	i += 2 // unknown \.{@@x}: drop it rather than corrupt the output

@* Change files.
A change file (\.{CWEB}'s .{.ch} mechanism) patches the master source without
editing it. It is a sequence of changes, each finding a block of lines in the
master and substituting a replacement block.

Text outside an \.{@@x...@@z} group is ignored (it serves as commentary). Changes
are matched against the master source---after \.{@@i} includes are expanded---in
the order they appear: \.{GWEB} scans the master line by line, and at the first
line equal to a change's first match line it requires the whole match block
to match, then substitutes the replacement lines.
@c
@<Change-file records@>
@<Recognize and compare change lines@>
@<Parse a change file@>
@<Apply changes to the master source@>

@ A |change| records the lines to find and the lines to substitute, plus
change-file line numbers for diagnostics. A |srcLoc| identifies the origin of a
line of the expanded, change-applied source.
@<Change-file records@>=
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
@<Recognize and compare change lines@>=
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
@<Parse a change file@>=
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
		@<Collect the match part up to \.{@@y}@>
		@<Collect the replacement part up to \.{@@z}@>
		if len(c.match) == 0 {
			return nil, fmt.Errorf("change file line %d: the @@x match part is empty", c.line)
		}
		changes = append(changes, c)
	}
	return changes, nil
}

@ The match part runs from after the \.{@@x} to the \.{@@y}; a stray \.{@@x} or
\.{@@z} inside it, or the end of file, is an error. After the \.{@@y} we record
where the replacement lines begin, for diagnostics.
@<Collect the match part up to \.{@@y}@>=
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

@ The replacement part runs from after the \.{@@y} to the \.{@@z}, with the same
guard against a stray \.{@@x} or \.{@@y} and against running off the end.
@<Collect the replacement part up to \.{@@z}@>=
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

@ |applyChanges| is the string convenience form used by tests.
@<Apply changes to the master source@>=
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
@<Apply changes to the master source@>=
func applyChangesMapped(master []string, locs []srcLoc, changes []change, chFile string) (
	[]string, []srcLoc, error,
) {
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
@<Apply changes to the master source@>=
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
@(common_test.go@>=
package common

import (
	"os"
	"path/filepath"
	"testing"
)

@ @(common_test.go@>=
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

@ @(common_test.go@>=
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
@(common_test.go@>=
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

@ @(common_test.go@>=
func TestResolveAbbrev(t *testing.T) {
	w := ParseString(sample)
	if got := w.Resolve("Print the..."); got != "Print the greeting" {
		t.Errorf("Resolve abbrev = %q, want %q", got, "Print the greeting")
	}
}

@ @(common_test.go@>=
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

@ @(common_test.go@>=
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

@ @(common_test.go@>=
func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexFrom(s, sub, 0) >= 0
}

@ @(common_test.go@>=
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

@ Several \.{@@f}/\.{@@s} directives may share one line, even after ordinary limbo
\TEX/, exactly as \.{cweave}'s manual writes \.{\\def\\x\#1\{x\_\{\#1\}\} @@f x1
TeX @@f x2 TeX}. Each is picked up, and the leading \.{\\def} survives in the
copied limbo.
@(common_test.go@>=
func TestLimboFormatsOneLine(t *testing.T) {
	w := ParseString("\\def\\x#1{x_{#1}} @@f x1 TeX @@f x2 TeX\n@@ x\n@@c\npackage main\n")
	if len(w.Formats) != 2 {
		t.Fatalf("limbo formats = %d, want 2: %+v", len(w.Formats), w.Formats)
	}
	if w.Formats[0].Original != "x1" || w.Formats[0].Like != "TeX" {
		t.Errorf("format[0] = %+v, want x1 TeX", w.Formats[0])
	}
	if w.Formats[1].Original != "x2" || w.Formats[1].Like != "TeX" {
		t.Errorf("format[1] = %+v, want x2 TeX", w.Formats[1])
	}
	if contains(w.Limbo, "@@f") {
		t.Errorf("directives not stripped from limbo: %q", w.Limbo)
	}
	if !contains(w.Limbo, "\\def\\x") {
		t.Errorf("limbo lost the \\def before the directives: %q", w.Limbo)
	}
}

@ A \.{@@q...@@>} comment speaks to the reader of the \.{.w} file alone, so it is
dropped everywhere---from the limbo and from code alike---and the text on
either side simply joins. Its own words (here \.{SECRET}) never reach the output.
@(common_test.go@>=
func TestQComment(t *testing.T) {
	w := ParseString("A@@q SECRET @@>B\n@@ @@c\nvar x = 1 @@q SECRET @@>+ 2\n")
	if contains(w.Limbo, "SECRET") || contains(w.Limbo, "@@q") {
		t.Errorf("@@q not dropped from limbo: %q", w.Limbo)
	}
	if !contains(w.Limbo, "A") || !contains(w.Limbo, "B") {
		t.Errorf("limbo text around @@q was lost: %q", w.Limbo)
	}
	for _, a := range ScanCode(w.Sections[0].Code) {
		if a.Kind == AText && contains(a.Text, "SECRET") {
			t.Errorf("@@q not dropped from code: %q", a.Text)
		}
	}
}

@ A \.{@@d} names several identifiers at once---its body runs to the next
\.{@@} and may span lines---and each word becomes its own typewriter |Format|.
A qualified name keeps its final component.
@(common_test.go@>=
func TestMacroMultipleNames(t *testing.T) {
	w := ParseString("@@ @@d Push Pop\n   Peek http.Get\n@@c\npackage main\n")
	got := w.Sections[0].Formats
	want := []string{"Push", "Pop", "Peek", "Get"}
	if len(got) != len(want) {
		t.Fatalf("@@d formats = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, name := range want {
		if got[i].Original != name || !got[i].Macro {
			t.Errorf("format[%d] = %+v, want %s (macro)", i, got[i], name)
		}
	}
}

@ @(common_test.go@>=
func hasWarning(ws []string, sub string) bool {
	for _, w := range ws {
		if indexFrom(w, sub, 0) >= 0 {
			return true
		}
	}
	return false
}

@ @(common_test.go@>=
func TestSectionLines(t *testing.T) {
	w := ParseString("limbo\n\n@@ first\n@@c\nx\n\n@@ second\n@@c\ny\n")
	if w.Sections[0].Line != 3 {
		t.Errorf("section 1 line = %d, want 3", w.Sections[0].Line)
	}
	if w.Sections[1].Line != 7 {
		t.Errorf("section 2 line = %d, want 7", w.Sections[1].Line)
	}
}

@ @(common_test.go@>=
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

@ @(common_test.go@>=
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

@ @(common_test.go@>=
func TestChangeFileNoMatch(t *testing.T) {
	master := "@@ x\n@@c\npackage main\n"
	changes, _ := parseChangeFile("@@x\nnonexistent line\n@@y\nwhatever\n@@z\n")
	if _, err := applyChanges(master, changes, "c.ch"); err == nil ||
		!contains(err.Error(), "never matched") {
		t.Errorf("want never-matched error, got %v", err)
	}
}

@ @(common_test.go@>=
func TestChangeFilePartialMismatch(t *testing.T) {
	master := "alpha\nbeta\ngamma\n"
	changes, _ := parseChangeFile("@@x\nbeta\nWRONG\n@@y\nx\n@@z\n")
	if _, err := applyChanges(master, changes, "c.ch"); err == nil ||
		!contains(err.Error(), "did not match") {
		t.Errorf("want did-not-match error, got %v", err)
	}
}

@ @(common_test.go@>=
func TestChangeFileMalformed(t *testing.T) {
	if _, err := parseChangeFile("@@x\nfind\n@@z\n"); err == nil {
		t.Error("want error for @@x without @@y")
	}
}

@ Include-line mapping. An undefined reference living in an included file must be
reported against that file (here \.{part.w}), not against a line number in the
includes-expanded master, and a section's origin must map back the same way.
@(common_test.go@>=
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
@(common_test.go@>=
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
