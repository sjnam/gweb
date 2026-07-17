//line common/common.w:27
package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const Version = "0.8.0"

//line common/common.w:51
type Format struct {
	Original string
	Like     string
	NoIndex  bool
	Macro    bool // \.{@d}: typeset Original in \.{typewriter} (a \.{CWEB}-style macro)
}

//line common/common.w:62
type Section struct {
	Number   int    // 1-based section number
	Line     int    // 1-based source line where the section begins
	Starred  bool   // |true| for \.{@*} sections
	Depth    int    // group depth for starred sections ($-1\equiv{}$\.{@**}, $0\equiv{}$\.{@*}, $n\equiv{}$\.{@*n})
	Title    string // starred-section title (text up to the first period)
	Tex      string // commentary, raw \TEX/ with in-text \.{@}-codes still embedded
	Formats  []Format
	HasCode  bool   // |true| if the section contributes code
	Name     string // named-section name, or \.{""} for an unnamed @c section
	IsFile   bool   // |true| if the name is an output file (\.{@(file@>=})
	Code     string // raw code text with in-code \.{@}-codes still embedded
	CodeLine int    // 1-based combined-source line where |Code| begins (0 if none)
}

//line common/common.w:82
type Web struct {
	Limbo    string
	Formats  []Format // \.{@f}/\.{@s} directives found in limbo (apply globally)
	Sections []*Section
	Warnings []string // non-fatal diagnostics gathered while parsing/checking
	file     string   // source filename, for diagnostics (\.{""} if unknown)
	locs     []srcLoc // origin (file, line) of each combined-source line
	full     []string // canonical (non-abbreviated) section names
}

//line common/common.w:97
func Parse(filename string) (*Web, error) {
	return ParseWithChange(filename, "")
}

func ParseWithChange(filename, changeFile string) (*Web, error) {
	lines, locs, err := expandIncludes(filename, 0)
	if err != nil {
		return nil, err
	}

//line common/common.w:119
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

//line common/common.w:107
	src := strings.Join(lines, "\n")
	w := parse(src)
	w.file = filename
	w.locs = locs
	w.finish(src)
	return w, nil
}

//line common/common.w:138
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

//line common/common.w:156
func (w *Web) Origin(line int) (file string, ln int) {
	if i := line - 1; i >= 0 && i < len(w.locs) {
		return w.locs[i].file, w.locs[i].line
	}
	return w.file, line
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

//line common/common.w:177
func DefaultExt(name, ext string) string {
	if name == "" || filepath.Ext(name) != "" {
		return name
	}
	return name + ext
}

//line common/common.w:188
func expandIncludes(file string, depth int) ([]string, []srcLoc, error) {
	if depth > 25 {
		return nil, nil, fmt.Errorf("gweb: @i include nesting too deep at %q", file)
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

//line common/common.w:225
func includeDirective(line string) (name string, ok bool) {
	t := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(t, "@i") {
		return "", false
	}
	rest := t[2:]
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return "", false
	}
	name = strings.Trim(strings.TrimSpace(rest), "\"")
	return name, name != ""
}

//line common/common.w:243
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

//line common/common.w:268
func (w *Web) prefixMatches(prefix string) int {
	n := 0
	for _, full := range w.full {
		if strings.HasPrefix(full, prefix) {
			n++
		}
	}
	return n
}

//line common/common.w:287
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

//line common/common.w:318
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

//line common/common.w:299
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

//line common/common.w:372
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

//line common/common.w:345
func lineAt(src string, off int) int {
	if off > len(src) {
		off = len(src)
	}
	return 1 + strings.Count(src[:off], "\n")
}

func canonName(name string) string {
	return strings.Join(strings.Fields(name), " ")
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

//line common/common.w:407
type ctrlKind int

const (
	cEOF ctrlKind = iota
	cSection
	cCode  // \.{@c} (or its synonym \.{@p})
	cNamed // \.{@<name@>=} or \.{@(file@>=}
	cDefn  // \.{@d}
	cFormat

//line common/common.w:416
)

type ctrl struct {
	kind    ctrlKind
	pos     int    // index of the leading `\.{@}'
	end     int    // index just past the control token
	depth   int    // for |cSection|: -1 unstarred (or \.{@**} top level), else starred depth
	starred bool   // for |cSection| (distinguishes \.{@**} from an unstarred section)
	name    string // for |cNamed|
	isFile  bool   // for |cNamed| (\.{@(} vs \.{@<})
	noIndex bool   // for |cFormat| (\.{@s})
}

//line common/common.w:434
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

//line common/common.w:525
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

//line common/common.w:451
		case c == 'c' || c == 'p':
			return ctrl{kind: cCode, pos: i, end: i + 2}
		case c == 'd':
			return ctrl{kind: cDefn, pos: i, end: i + 2}
		case c == 'f':
			return ctrl{kind: cFormat, pos: i, end: i + 2}
		case c == 's':
			return ctrl{kind: cFormat, pos: i, end: i + 2, noIndex: true}
		case c == '<' || c == '(':

//line common/common.w:542
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

//line common/common.w:461
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

//line common/common.w:484
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

//line common/common.w:525
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

//line common/common.w:501
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

//line common/common.w:563
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

//line common/common.w:593

//line common/common.w:611
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

//line common/common.w:594

//line common/common.w:626
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

//line common/common.w:596
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

//line common/common.w:647
func findSectionHeaderEnd(src string, i int) ctrl {
	n := len(src)
	j := i + 2
	depth := 0
	if j < n && src[j] == '*' {
		j++
		depth = -1 // ``\.{@**}'' is the top level: bold in the contents, as \.{CWEB}
	} else {
		for j < n && src[j] >= '0' && src[j] <= '9' {
			depth = depth*10 + int(src[j]-'0')
			j++
		}
	}
	return ctrl{end: j, depth: depth}
}

//line common/common.w:666
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

//line common/common.w:687
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

//line common/common.w:715
func parseFormat(seg string, noIndex bool) (Format, bool) {
	fields := strings.Fields(seg)
	if len(fields) < 2 {
		return Format{}, false
	}
	return Format{Original: fields[0], Like: fields[1], NoIndex: noIndex}, true
}

//line common/common.w:731
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

//line common/common.w:750
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

//line common/common.w:799
			var fs []Format
			var j int
			if c == 'd' {
				j = i + 2
				for j < n && src[j] != '@' {
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

//line common/common.w:767
		case 'q':
			if end := indexFrom(src, "@>", i+2); end < 0 {
				i = n // unterminated: drop the rest of limbo
			} else {
				i = end + 2 // drop the source-only comment
			}
		case '<', '(', '=', 't', '^', '.', ':':
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

//line common/common.w:823
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

//line common/common.w:849
type AtomKind int

const (
	AText     AtomKind = iota // ordinary \GO/ source text
	ARef                      // \.{@<name@>} reference to a named section
	AVerbatim                 // \.{@=text@>} passed verbatim to tangled output
	ATeX                      // \.{@t text@>} \TEX/ text for the woven output
	AIndex                    // \.{@\^/@./@}: index entry
	APaste                    // \.{@\&} join (delete surrounding whitespace)
	ALayout                   // \.{@}, \.{@/} \.{@|} \.{@\#} woven-output layout hints
	AIndexDef                 // \.{@!} force the next identifier to index as a definition
)

type Atom struct {
	Kind  AtomKind
	Text  string // payload for |AText|/|AVerbatim|/|ATeX|/|AIndex|; name for |ARef|
	Index byte   // '\.{\^}','\.{.}','\.{:}' for AIndex; '\.{,}' '\.{/}' '\.{|}' '\.{\#}' for |ALayout|
}

//line common/common.w:872
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

//line common/common.w:902
		switch d := code[i+1]; d {

//line common/common.w:912
		case '@':
			buf.WriteByte('@')
			i += 2
		case '&':
			flush()
			atoms = append(atoms, Atom{Kind: APaste})
			i += 2

//line common/common.w:925
		case '<':
			end := indexFrom(code, "@>", i+2)
			if end < 0 {
				buf.WriteString(code[i:])
				i = n
				continue
			}
			flush()
			atoms = append(atoms, Atom{Kind: ARef, Text: canonName(code[i+2 : end])})
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

//line common/common.w:981
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
			i += 2 // unknown \.{@x}: drop it rather than corrupt the output

//line common/common.w:906
		}

//line common/common.w:891
	}
	flush()
	return atoms
}

//line common/common.w:1022
type change struct {
	match    []string // lines to find in the master source
	repl     []string // lines to substitute for them
	line     int      // 1-based line of the \.{@x} in the change file (for diagnostics)
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

//line common/common.w:1044
func isChangeCtrl(line string, c byte) bool {
	return len(line) >= 2 && line[0] == '@' && line[1] == c
}

func splitLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}

func sameLine(a, b string) bool {
	return strings.TrimRight(a, " \t") == strings.TrimRight(b, " \t")
}

//line common/common.w:1059
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

//line common/common.w:1084
		for i < n && !isChangeCtrl(lines[i], 'y') {
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'z') {
				return nil, fmt.Errorf("change file line %d: expected @y to close the @x match part", c.line)
			}
			c.match = append(c.match, lines[i])
			i++
		}
		if i >= n {
			return nil, fmt.Errorf("change file line %d: @x without a matching @y", c.line)
		}
		i++ // skip \.{@y}
		c.replLine = i + 1

//line common/common.w:1071

//line common/common.w:1100
		for i < n && !isChangeCtrl(lines[i], 'z') {
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'y') {
				return nil, fmt.Errorf("change file line %d: expected @z to close the change", c.line)
			}
			c.repl = append(c.repl, lines[i])
			i++
		}
		if i >= n {
			return nil, fmt.Errorf("change file line %d: change has no @z", c.line)
		}
		i++ // skip \.{@z}

//line common/common.w:1072
		if len(c.match) == 0 {
			return nil, fmt.Errorf("change file line %d: the @x match part is empty", c.line)
		}
		changes = append(changes, c)
	}
	return changes, nil
}

//line common/common.w:1114
func applyChanges(src string, changes []change, chFile string) (string, error) {
	out, _, err := applyChangesMapped(splitLines(src), nil, changes, chFile)
	if err != nil {
		return "", err
	}
	return strings.Join(out, "\n"), nil
}

//line common/common.w:1127
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

//line common/common.w:1167
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
