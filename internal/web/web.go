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

// Format records an "@f a b" (or "@s a b") directive: typeset identifier
// Original the way identifier/keyword Like is typeset. NoIndex is true for @s.
type Format struct {
	Original string
	Like     string
	NoIndex  bool
	Macro    bool // : typeset Original in typewriter (a CWEB-style macro)
}

// Section is one numbered section of the web.
type Section struct {
	Number   int    // 1-based section number
	Line     int    // 1-based source line where the section begins
	Starred  bool   // true for @* sections
	Depth    int    // group depth for starred sections (0 == top level)
	Title    string // starred-section title (text up to the first period)
	Tex      string // commentary, raw TeX with in-text @-codes still embedded
	Formats  []Format
	HasCode  bool   // true if the section contributes code
	Name     string // named-section name, or "" for an unnamed @c section
	IsFile   bool   // true if the name is an output file (@(file@>=)
	Code     string // raw code text with in-code @-codes still embedded
	CodeLine int    // 1-based combined-source line where Code begins (0 if none)
}

// Web is a fully parsed GWEB document.
type Web struct {
	Limbo    string
	Formats  []Format // @f / @s directives found in limbo (apply globally)
	Sections []*Section
	Warnings []string // non-fatal diagnostics gathered while parsing/checking
	file     string   // source filename, for diagnostics ("" if unknown)
	locs     []srcLoc // origin (file, line) of each combined-source line
	full     []string // canonical (non-abbreviated) section names
}

// Parse reads filename, expands @i includes, and parses the result.
func Parse(filename string) (*Web, error) {
	return ParseWithChange(filename, "")
}

// ParseWithChange reads the master file, expands @i includes, applies the change
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

// Origin maps a 1-based combined-source line back to the original file and line.
// When no origin map is available it falls back to the web's own filename.
func (w *Web) Origin(line int) (file string, ln int) {
	if i := line - 1; i >= 0 && i < len(w.locs) {
		return w.locs[i].file, w.locs[i].line
	}
	return w.file, line
}

// DefaultExt returns name with ext appended when name has no extension of its
// own (and is non-empty), so "wc" becomes "wc.w". A name that already carries an
// extension is left alone.
func DefaultExt(name, ext string) string {
	if name == "" || filepath.Ext(name) != "" {
		return name
	}
	return name + ext
}

// expandIncludes reads file and splices in @i includes, returning the combined
// lines together with a parallel origin map. As in CWEB, @i is line-oriented: a
// line whose first non-blank text is "@i" (followed by whitespace) names a file
// whose expansion replaces that line. A final newline does not produce a
// trailing blank line.
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

// includeDirective returns the file named by an "@i" line, or ok=false. The @i
// must be the first non-blank text on the line and be followed by whitespace.
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

// collectNames records the set of canonical (non-abbreviated) section names so
// abbreviations ending in "..." can be resolved. A full name may appear at a
// definition (@<name@>=) or at any reference (@<name@>), in code or in
// commentary, so all of those are scanned. Output files (@(...@>=) are roots,
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

// checkNames validates @<...@> references: it reports ambiguous or unmatched
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
