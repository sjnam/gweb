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

// Format records an "@f a b" (or "@s a b") directive: typeset identifier
// Original the way identifier/keyword Like is typeset. NoIndex is true for @s.
type Format struct {
	Original string
	Like     string
	NoIndex  bool
}

// Section is one numbered section of the web.
type Section struct {
	Number  int    // 1-based section number
	Line    int    // 1-based source line where the section begins
	Starred bool   // true for @* sections
	Depth   int    // group depth for starred sections (0 == top level)
	Title   string // starred-section title (text up to the first period)
	Tex     string // commentary, raw TeX with in-text @-codes still embedded
	Formats []Format
	HasCode bool   // true if the section contributes code
	Name    string // named-section name, or "" for an unnamed @c section
	IsFile  bool   // true if the name is an output file (@(file@>=)
	Code    string // raw code text with in-code @-codes still embedded
}

// Web is a fully parsed GWEB document.
type Web struct {
	Limbo    string
	Formats  []Format // @f / @s directives found in limbo (apply globally)
	Sections []*Section
	Warnings []string // non-fatal diagnostics gathered while parsing/checking
	file     string   // source filename, for diagnostics ("" if unknown)
	full     []string // canonical (non-abbreviated) section names
}

// Parse reads filename, expands @i includes, and parses the result.
func Parse(filename string) (*Web, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	src, err := expandIncludes(string(data), filepath.Dir(filename), 0)
	if err != nil {
		return nil, err
	}
	w := parse(src)
	w.file = filename
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
	w.Warnings = append(w.Warnings, scanDiagnostics(src, w.file)...)
	w.Warnings = append(w.Warnings, w.checkNames()...)
}

// at formats a source location for a diagnostic message.
func (w *Web) at(line int) string {
	if w.file != "" {
		return fmt.Sprintf("%s:%d", w.file, line)
	}
	return fmt.Sprintf("line %d", line)
}

// expandIncludes splices in @i files, respecting argument-terminated control
// codes so an @i inside @<...@> or @=...@> is not mistaken for an include.
func expandIncludes(src, dir string, depth int) (string, error) {
	if depth > 25 {
		return "", fmt.Errorf("gweb: @i include nesting too deep")
	}
	var b strings.Builder
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
		case 'i':
			j := i + 2
			for j < n && (src[j] == ' ' || src[j] == '\t') {
				j++
			}
			start := j
			for j < n && src[j] != '\n' {
				j++
			}
			name := strings.TrimSpace(src[start:j])
			name = strings.Trim(name, "\"")
			path := name
			if !filepath.IsAbs(path) {
				path = filepath.Join(dir, name)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return "", fmt.Errorf("gweb: cannot include %q: %w", name, err)
			}
			inc, err := expandIncludes(string(data), filepath.Dir(path), depth+1)
			if err != nil {
				return "", err
			}
			b.WriteString(inc)
			i = j // leave the trailing newline in place
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
	return b.String(), nil
}

// collectNames records the set of canonical (non-abbreviated) named sections so
// abbreviations ending in "..." can be resolved. Output files (@(...@>=) are
// roots, not referable refinements, so they are excluded.
func (w *Web) collectNames() {
	seen := map[string]bool{}
	for _, s := range w.Sections {
		if s.Name != "" && !s.IsFile && !strings.HasSuffix(s.Name, "...") {
			if !seen[s.Name] {
				seen[s.Name] = true
				w.full = append(w.full, s.Name)
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
	defined := map[string]bool{}
	for _, n := range w.full {
		defined[n] = true
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

// Resolve maps a (possibly abbreviated) name to its canonical form. An
// abbreviation "Prefix..." matches the unique full name starting with Prefix.
func (w *Web) Resolve(name string) string {
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
