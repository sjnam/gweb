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
	Starred bool    // true for @* sections
	Depth   int     // group depth for starred sections (0 == top level)
	Title   string  // starred-section title (text up to the first period)
	Tex     string  // commentary, raw TeX with in-text @-codes still embedded
	Formats []Format
	HasCode bool   // true if the section contributes code
	Name    string // named-section name, or "" for an unnamed @c section
	IsFile  bool   // true if the name is an output file (@(file@>=)
	Code    string // raw code text with in-code @-codes still embedded
}

// Web is a fully parsed GWEB document.
type Web struct {
	Limbo    string
	Sections []*Section
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
	w.collectNames()
	return w, nil
}

// ParseString parses already-loaded source (used by tests; no includes).
func ParseString(src string) *Web {
	w := parse(src)
	w.collectNames()
	return w
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
// abbreviations ending in "..." can be resolved.
func (w *Web) collectNames() {
	seen := map[string]bool{}
	for _, s := range w.Sections {
		if s.Name != "" && !strings.HasSuffix(s.Name, "...") {
			if !seen[s.Name] {
				seen[s.Name] = true
				w.full = append(w.full, s.Name)
			}
		}
	}
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
