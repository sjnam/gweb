package weave

import (
	"bufio"
	"fmt"
	"sort"
	"strings"

	"github.com/sjnam/gweb/internal/web"
)

// xref accumulates cross-reference information while a web is woven:
//   - where each identifier is used and (heuristically) defined;
//   - where each named section is defined and used;
//   - manual index entries from @^ @. @: control codes.
//
// It is populated during a first (discarded) weaving pass and then consulted
// during the real pass and when emitting the back matter.
type xref struct {
	identUse    map[string]map[int]bool
	identDef    map[string]map[int]bool
	sectionDefs map[string]map[int]bool
	sectionUses map[string]map[int]bool
	manualIndex []manualEntry
}

type manualEntry struct {
	kind byte // '^', '.', ':'
	text string
	sec  int
}

func newXref() *xref {
	return &xref{
		identUse:    map[string]map[int]bool{},
		identDef:    map[string]map[int]bool{},
		sectionDefs: map[string]map[int]bool{},
		sectionUses: map[string]map[int]bool{},
	}
}

func addTo(m map[string]map[int]bool, key string, sec int) {
	if m[key] == nil {
		m[key] = map[int]bool{}
	}
	m[key][sec] = true
}

func (x *xref) addIdentUse(name string, sec int)   { addTo(x.identUse, name, sec) }
func (x *xref) addIdentDef(name string, sec int)   { addTo(x.identDef, name, sec) }
func (x *xref) addSectionDef(name string, sec int) { addTo(x.sectionDefs, name, sec) }
func (x *xref) addSectionUse(name string, sec int) { addTo(x.sectionUses, name, sec) }
func (x *xref) addManualIndex(kind byte, text string, sec int) {
	x.manualIndex = append(x.manualIndex, manualEntry{kind, text, sec})
}

// sortedKeys returns the keys of a section set in ascending order.
func sortedKeys(m map[int]bool) []int {
	ks := make([]int, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Ints(ks)
	return ks
}

// secList renders a set of section numbers as hyperlinks, with the defining
// sections (those in def) additionally underlined.
func secList(secs, def map[int]bool) string {
	nums := sortedKeys(secs)
	parts := make([]string, len(nums))
	for i, n := range nums {
		if def != nil && def[n] {
			parts[i] = fmt.Sprintf("\\GsD{%d}", n)
		} else {
			parts[i] = fmt.Sprintf("\\Gs{%d}", n)
		}
	}
	return strings.Join(parts, ", ")
}

// writeBackMatter emits the index, the list of named sections, and the table of
// contents that close a woven document.
func (wv *Weaver) writeBackMatter(bw *bufio.Writer) {
	wv.writeBookmarks(bw)
	bw.WriteString("\n\\Ginx\n")
	wv.writeIndex(bw)
	bw.WriteString("\\Gfin\n")
	wv.writeSectionNames(bw)
	bw.WriteString("\\Gcon\n\\end\n")
}

// writeBookmarks emits one \Gbookmark per starred section, in document (pre)
// order, so a PDF outline can be built whose nesting follows the @*, @*1,
// @*2 ... depths. Each entry carries its depth (the dvipdfmx route nests by
// level) and its number of direct children (pdftex's count model).
func (wv *Weaver) writeBookmarks(bw *bufio.Writer) {
	var starred []*web.Section
	for _, s := range wv.w.Sections {
		if s.Starred {
			starred = append(starred, s)
		}
	}
	if len(starred) == 0 {
		return
	}
	bw.WriteString("\n\\par")
	for i, s := range starred {
		children := 0
		for j := i + 1; j < len(starred) && starred[j].Depth > s.Depth; j++ {
			if starred[j].Depth == s.Depth+1 {
				children++
			}
		}
		fmt.Fprintf(bw, "\\Gbookmark{%d}{%d}{%d}{%s}%%\n", s.Depth, s.Number, children, bookmarkTitle(s.Title))
	}
}

// bookmarkTitle reduces a starred-section title to plain text safe for a PDF
// outline: |code| spans keep their inner text, @@ becomes @, and TeX-special
// characters (which are rare in titles) are dropped.
func bookmarkTitle(raw string) string {
	var b strings.Builder
	n := len(raw)
	for i := 0; i < n; i++ {
		c := raw[i]
		switch {
		case c == '\\' && i+1 < n && raw[i+1] == '|':
			b.WriteByte('|')
			i++
		case c == '@' && i+1 < n && raw[i+1] == '@':
			b.WriteByte('@')
			i++
		case c == '|':
			// drop the bar; keep the inline code's text
		case c == '\\':
			// drop a TeX control sequence (backslash plus a run of letters, or
			// backslash plus one symbol), so e.g. \.{web} reduces to "web".
			if i+1 < n {
				if d := raw[i+1]; (d >= 'a' && d <= 'z') || (d >= 'A' && d <= 'Z') {
					i++
					for i+1 < n {
						if e := raw[i+1]; (e >= 'a' && e <= 'z') || (e >= 'A' && e <= 'Z') {
							i++
						} else {
							break
						}
					}
				} else {
					i++
				}
			}
		case c == '{' || c == '}' || c == '$' || c == '&' ||
			c == '#' || c == '%' || c == '^' || c == '_' || c == '~':
			// TeX-special: drop
		default:
			b.WriteByte(c)
		}
	}
	return strings.TrimSpace(b.String())
}

// indexItem is one alphabetized entry of the identifier/manual index.
type indexItem struct {
	sortKey string
	render  string // typeset form of the entry head (\GID{...}, \GIR{...}, ...)
	secs    map[int]bool
	defs    map[int]bool
}

func (wv *Weaver) writeIndex(bw *bufio.Writer) {
	items := map[string]*indexItem{}
	get := func(render, sortKey string) *indexItem {
		it := items[render]
		if it == nil {
			it = &indexItem{sortKey: sortKey, render: render,
				secs: map[int]bool{}, defs: map[int]bool{}}
			items[render] = it
		}
		return it
	}

	// An identifier's index head follows its display class: a const is set in
	// typewriter (like its uses in the text), everything else in italic.
	head := func(name string) string {
		if wv.format[name] == tkMacro {
			return "\\GMAC{" + escTT(name) + "}"
		}
		return "\\GID{" + escIdent(name) + "}"
	}
	for name, secs := range wv.xref.identUse {
		it := get(head(name), strings.ToLower(name))
		for s := range secs {
			it.secs[s] = true
		}
	}
	for name, secs := range wv.xref.identDef {
		it := get(head(name), strings.ToLower(name))
		for s := range secs {
			it.secs[s] = true
			it.defs[s] = true
		}
	}
	for _, e := range wv.xref.manualIndex {
		var render string
		switch e.kind {
		case '.':
			render = "\\GIT{" + escTT(e.text) + "}"
		case ':':
			render = "\\GIC{" + e.text + "}"
		default: // '^'
			render = "\\GIR{" + escProse(e.text) + "}"
		}
		it := get(render, strings.ToLower(e.text))
		it.secs[e.sec] = true
	}

	list := make([]*indexItem, 0, len(items))
	for _, it := range items {
		list = append(list, it)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].sortKey != list[j].sortKey {
			return list[i].sortKey < list[j].sortKey
		}
		return list[i].render < list[j].render
	})
	for _, it := range list {
		fmt.Fprintf(bw, "\\GII{%s}{%s}\n", it.render, secList(it.secs, it.defs))
	}
}

// writeSectionNames emits the list of named sections with their defining and
// using section numbers.
func (wv *Weaver) writeSectionNames(bw *bufio.Writer) {
	names := map[string]bool{}
	for n := range wv.xref.sectionDefs {
		names[n] = true
	}
	for n := range wv.xref.sectionUses {
		names[n] = true
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return strings.ToLower(sorted[i]) < strings.ToLower(sorted[j])
	})
	for _, n := range sorted {
		fmt.Fprintf(bw, "\\GNS{%s}{%d}{%s}\n",
			wv.renderName(n), wv.defNum[n], usedNote(wv.xref.sectionUses[n]))
	}
}

// usedNote renders the "Used in section(s) ..." note for the section-names list,
// or "" when the section is never used.
func usedNote(uses map[int]bool) string {
	if len(uses) == 0 {
		return ""
	}
	noun := "section"
	if len(uses) > 1 {
		noun = "sections"
	}
	return "Used in " + noun + " " + secList(uses, nil)
}

// crossRefNotes returns the "also defined in"/"used in" notes printed under the
// first definition of a named section, or "" if none apply.
func (wv *Weaver) crossRefNotes(name string, secNum int) string {
	if wv.defNum[name] != secNum {
		return "" // notes appear only under the first definition
	}
	var b strings.Builder
	defs := wv.xref.sectionDefs[name]
	if len(defs) > 1 {
		others := map[int]bool{}
		for s := range defs {
			if s != secNum {
				others[s] = true
			}
		}
		macro := "\\GA"
		if len(others) > 1 {
			macro = "\\GAs"
		}
		fmt.Fprintf(&b, "%s{%s}%%\n", macro, secList(others, nil))
	}
	if uses := wv.xref.sectionUses[name]; len(uses) > 0 {
		macro := "\\GU"
		if len(uses) > 1 {
			macro = "\\GUs"
		}
		fmt.Fprintf(&b, "%s{%s}%%\n", macro, secList(uses, nil))
	}
	return b.String()
}
