// Package weave implements gweave: it turns a GWEB web into a TeX document with
// pretty-printed Go code (bold reserved words, italic identifiers), linked
// section references, and (see xref.go) cross-references and an index. It is the
// Go analogue of CWEB's cweave.
package weave

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/sjnam/gweb/internal/web"
)

// Weaver turns a parsed web into woven TeX.
type Weaver struct {
	w      *web.Web
	defNum map[string]int // canonical named-section -> first defining section

	xref *xref // identifier and section cross-references (built lazily)
}

// New builds a Weaver for the given web.
func New(w *web.Web) *Weaver {
	wv := &Weaver{w: w, defNum: map[string]int{}}
	for _, s := range w.Sections {
		if s.HasCode && s.Name != "" && !s.IsFile {
			name := w.Resolve(s.Name)
			if _, ok := wv.defNum[name]; !ok {
				wv.defNum[name] = s.Number
			}
		}
	}
	return wv
}

// Weave writes the complete TeX document to out. It runs two passes: the first
// is discarded and only populates the cross-reference tables (so that, e.g.,
// "used in section N" notes can be printed under a definition even when the use
// occurs later); the second produces the real output.
func (wv *Weaver) Weave(out io.Writer) error {
	wv.xref = newXref()
	scan := bufio.NewWriter(io.Discard)
	for _, sec := range wv.w.Sections {
		wv.writeSection(scan, sec)
	}

	bw := bufio.NewWriter(out)
	bw.WriteString(wv.w.Limbo)
	for _, sec := range wv.w.Sections {
		wv.writeSection(bw, sec)
	}
	wv.writeBackMatter(bw)
	return bw.Flush()
}

func (wv *Weaver) writeSection(bw *bufio.Writer, sec *web.Section) {
	if sec.Starred {
		fmt.Fprintf(bw, "\n\\GN{%d}{%d}{%s}", sec.Depth, sec.Number, escProse(sec.Title))
		rest := sec.Tex
		if dot := strings.Index(rest, "."); dot >= 0 {
			rest = rest[dot+1:]
		}
		bw.WriteString(wv.processTex(sec.Number, rest))
	} else {
		fmt.Fprintf(bw, "\n\\GM{%d}", sec.Number)
		bw.WriteString(wv.processTex(sec.Number, sec.Tex))
	}

	if sec.HasCode {
		if sec.Name != "" {
			name := wv.w.Resolve(sec.Name)
			cont := wv.defNum[name] != sec.Number
			wv.xref.addSectionDef(name, sec.Number)
			macro := "\\GD"
			if cont {
				macro = "\\GDp" // continuation of an earlier definition
			}
			fmt.Fprintf(bw, "\n%s{%d}{%s}", macro, wv.defNum[name], escProse(name))
		}
		bw.WriteString("\n\\GB%\n")
		bw.WriteString(wv.renderCode(sec.Number, sec.Code))
		bw.WriteString("\\GE\n")
		if sec.Name != "" {
			bw.WriteString(wv.crossRefNotes(wv.w.Resolve(sec.Name), sec.Number))
		}
	}
}

// renderCode formats a code part into a sequence of \GL code lines.
func (wv *Weaver) renderCode(secNum int, code string) string {
	var out strings.Builder
	var line strings.Builder
	var st lexState
	indent := 0
	atLineStart := true
	pendingSpace := false

	// prevWord tracks the most recent significant token's kind/text so that an
	// identifier following func/var/const/type can be flagged as a definition.
	prevSigKind := tkNewline
	prevSigText := ""

	flush := func() {
		if strings.TrimSpace(line.String()) != "" {
			fmt.Fprintf(&out, "\\GL{%d}{%s}%%\n", indent, line.String())
		}
		line.Reset()
		indent = 0
		atLineStart = true
		pendingSpace = false
	}
	emitSpaceBefore := func() {
		if pendingSpace {
			line.WriteString("\\ ")
			pendingSpace = false
		}
	}

	for _, a := range web.ScanCode(code) {
		switch a.Kind {
		case web.AText:
			toks := lexGo(a.Text, &st)
			for k, t := range toks {
				switch t.kind {
				case tkNewline:
					flush()
				case tkSpace:
					if atLineStart {
						indent += indentLevel(t.text)
					} else {
						pendingSpace = true
					}
				default:
					atLineStart = false
					emitSpaceBefore()
					if t.kind == tkIdent || t.kind == tkBuiltin {
						if isDefinition(prevSigKind, prevSigText, toks, k) {
							wv.xref.addIdentDef(t.text, secNum)
						} else {
							wv.xref.addIdentUse(t.text, secNum)
						}
					}
					line.WriteString(renderToken(t))
					prevSigKind, prevSigText = t.kind, t.text
				}
			}
		case web.ARef:
			atLineStart = false
			emitSpaceBefore()
			name := wv.w.Resolve(a.Text)
			wv.xref.addSectionUse(name, secNum)
			fmt.Fprintf(&line, "\\GX{%d}{%s}", wv.defNum[name], escProse(name))
		case web.AVerbatim:
			atLineStart = false
			emitSpaceBefore()
			fmt.Fprintf(&line, "\\GST{%s}", escTT(a.Text))
		case web.ATeX:
			atLineStart = false
			emitSpaceBefore()
			line.WriteString(a.Text)
		case web.AIndex:
			wv.xref.addManualIndex(a.Index, a.Text, secNum)
		case web.APaste:
			pendingSpace = false // join: suppress surrounding space
		}
	}
	flush()
	return out.String()
}

// renderToken renders a single Go token as a TeX fragment (used inside math).
func renderToken(t token) string {
	switch t.kind {
	case tkKeyword, tkBuiltin:
		return "\\GKW{" + escIdent(t.text) + "}"
	case tkIdent:
		return "\\GID{" + escIdent(t.text) + "}"
	case tkNumber:
		return "\\GNU{" + escTT(t.text) + "}"
	case tkString:
		return "\\GST{" + escTT(t.text) + "}"
	case tkComment:
		return "\\GCM{" + escTT(t.text) + "}"
	case tkOp:
		return escMathOp(t.text)
	}
	return ""
}

// processTex transforms commentary: |Go code| inline, @<refs@>, @@->@, and
// index entries (@^ @. @:) are recorded and removed. Everything else (the
// user's TeX) passes through unchanged.
func (wv *Weaver) processTex(secNum int, s string) string {
	var b strings.Builder
	n := len(s)
	i := 0
	for i < n {
		c := s[i]
		if c == '\\' && i+1 < n && s[i+1] == '|' {
			b.WriteString("|") // \| is a literal bar in prose
			i += 2
			continue
		}
		if c == '|' {
			j := i + 1
			var code strings.Builder
			for j < n {
				if s[j] == '\\' && j+1 < n && s[j+1] == '|' {
					code.WriteByte('|')
					j += 2
					continue
				}
				if s[j] == '|' {
					break
				}
				code.WriteByte(s[j])
				j++
			}
			b.WriteString(wv.renderInline(secNum, code.String()))
			i = j + 1
			continue
		}
		if c == '@' && i+1 < n {
			switch d := s[i+1]; d {
			case '@':
				b.WriteByte('@')
				i += 2
				continue
			case '<':
				if end := strings.Index(s[i+2:], "@>"); end >= 0 {
					end += i + 2
					name := wv.w.Resolve(strings.TrimSpace(s[i+2 : end]))
					wv.xref.addSectionUse(name, secNum)
					fmt.Fprintf(&b, "\\GX{%d}{%s}", wv.defNum[name], escProse(name))
					i = end + 2
					continue
				}
			case '^', '.', ':':
				if end := strings.Index(s[i+2:], "@>"); end >= 0 {
					end += i + 2
					wv.xref.addManualIndex(d, s[i+2:end], secNum)
					i = end + 2
					continue
				}
			}
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

// renderInline formats a |...| inline Go fragment as math.
func (wv *Weaver) renderInline(secNum int, code string) string {
	var st lexState
	var b strings.Builder
	b.WriteString("$")
	pending := false
	started := false
	for _, t := range lexGo(code, &st) {
		switch t.kind {
		case tkSpace, tkNewline:
			if started {
				pending = true
			}
		default:
			if pending {
				b.WriteString("\\ ")
				pending = false
			}
			if t.kind == tkIdent || t.kind == tkBuiltin {
				wv.xref.addIdentUse(t.text, secNum)
			}
			b.WriteString(renderToken(t))
			started = true
		}
	}
	b.WriteString("$")
	return b.String()
}

var declKeywords = map[string]bool{
	"func": true, "var": true, "const": true, "type": true,
}

// isDefinition heuristically decides whether the identifier at toks[k] is being
// declared: it follows a func/var/const/type keyword, or it is immediately
// followed by ":=". This is best-effort (no full Go parse) but covers the
// common cases CWEB underlines in its index.
func isDefinition(prevKind tokKind, prevText string, toks []token, k int) bool {
	if prevKind == tkKeyword && declKeywords[prevText] {
		return true
	}
	for j := k + 1; j < len(toks); j++ {
		switch toks[j].kind {
		case tkSpace:
			continue
		case tkOp:
			return toks[j].text == ":="
		default:
			return false
		}
	}
	return false
}

// indentLevel returns the indentation level of a leading-whitespace run: one
// level per tab, plus one per four spaces.
func indentLevel(s string) int {
	level, spaces := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\t':
			level++
			spaces = 0
		case ' ':
			spaces++
			if spaces == 4 {
				level++
				spaces = 0
			}
		}
	}
	return level
}
