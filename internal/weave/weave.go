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

	format  map[string]tokKind // @f/@s: identifier -> the token class to use
	noIndex map[string]bool    // @s: identifiers omitted from the index
	isFile  map[string]bool    // @(file@>= outputs: names are literal file paths

	xref *xref // identifier and section cross-references (built lazily)
}

// New builds a Weaver for the given web.
func New(w *web.Web) *Weaver {
	wv := &Weaver{
		w:       w,
		defNum:  map[string]int{},
		format:  map[string]tokKind{},
		noIndex: map[string]bool{},
		isFile:  map[string]bool{},
	}
	// Both refinements and @(file@>= outputs get a defining section number, so
	// their headlines and links resolve; only @(file@>= names are never the
	// target of a @<...@> reference. A file name is a literal path, not TeX, so
	// we remember which names are files and typeset them verbatim.
	for _, s := range w.Sections {
		if s.HasCode && s.Name != "" {
			name := w.Resolve(s.Name)
			if _, ok := wv.defNum[name]; !ok {
				wv.defNum[name] = s.Number
			}
			if s.IsFile {
				wv.isFile[name] = true
			}
		}
	}
	// Format directives apply globally; later definitions win. The display
	// class of identifier a (@f a b) is the class b would be typeset in.
	apply := func(fs []web.Format) {
		for _, f := range fs {
			if f.Macro {
				wv.format[f.Original] = tkMacro // : typewriter, like a CWEB macro
			} else {
				wv.format[f.Original] = classifyWord(f.Like)
			}
			if f.NoIndex {
				wv.noIndex[f.Original] = true
			}
		}
	}
	apply(w.Formats)
	for _, s := range w.Sections {
		apply(s.Formats)
	}
	// As in cweave, a name declared with |type| is set bold, like the predeclared
	// types. (Typewriter treatment is applied only on request, with |@d|.) An
	// explicit |@f|/|@s| above still wins.
	wv.detectDecls("type", tkBuiltin)
	return wv
}

func (wv *Weaver) detectDecls(keyword string, kind tokKind) {
	add := func(name string) {
		if name == "" || name == "_" {
			return
		}
		if _, ok := wv.format[name]; !ok {
			wv.format[name] = kind
		}
	}
	for _, s := range wv.w.Sections {
		if !s.HasCode {
			continue
		}
		var st lexState
		for _, a := range web.ScanCode(s.Code) {
			if a.Kind == web.AText {
				scanDecls(lexGo(a.Text, &st), keyword, add)
			}
		}
	}
}

func scanDecls(toks []token, keyword string, add func(string)) {
	for i := 0; i < len(toks); i++ {
		if toks[i].kind != tkKeyword || toks[i].text != keyword {
			continue
		}
		j := nextSignificant(toks, i+1)
		if j < 0 {
			return
		}
		if toks[j].kind == tkOp && toks[j].text == "(" {
			i = scanDeclGroup(toks, j+1, add)
		} else if toks[j].kind == tkIdent {
			add(toks[j].text)
		}
	}
}

// nextSignificant returns the index of the first token at or after i that is not
// whitespace or a newline, or -1 if there is none.
func nextSignificant(toks []token, i int) int {
	for ; i < len(toks); i++ {
		if toks[i].kind != tkSpace && toks[i].kind != tkNewline {
			return i
		}
	}
	return -1
}

// scanDeclGroup collects the declared names in a parenthesized type or const
// group beginning at index i, returning the index of the closing ")". The first
// identifier on each line at nesting depth 0 is a declared name.
func scanDeclGroup(toks []token, i int, add func(string)) int {
	depth := 0
	atStart := true
	for ; i < len(toks); i++ {
		switch t := toks[i]; t.kind {
		case tkNewline:
			if depth == 0 {
				atStart = true
			}
		case tkSpace:
			// keep atStart
		case tkOp:
			switch t.text {
			case "(", "{", "[":
				depth++
			case ")":
				if depth == 0 {
					return i
				}
				depth--
			case "}", "]":
				if depth > 0 {
					depth--
				}
			}
			atStart = false
		default:
			if atStart && depth == 0 && t.kind == tkIdent {
				add(t.text)
			}
			atStart = false
		}
	}
	return i
}

// effKind returns the token class to typeset t in, honoring @f/@s overrides.
func (wv *Weaver) effKind(t token) tokKind {
	switch t.kind {
	case tkIdent, tkKeyword, tkBuiltin:
		if k, ok := wv.format[t.text]; ok {
			return k
		}
	}
	return t.kind
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
	// gweave supplies the macro package itself, so a .w file need not (and
	// should not) \input it; drop any stray copy from the limbo.
	bw.WriteString("\\input gwebmac\n")
	bw.WriteString(stripGwebmacInput(wv.w.Limbo))
	for _, sec := range wv.w.Sections {
		wv.writeSection(bw, sec)
	}
	wv.writeBackMatter(bw)
	return bw.Flush()
}

// stripGwebmacInput removes any "\input gwebmac" line from the limbo, since
// gweave now emits it automatically.
func stripGwebmacInput(limbo string) string {
	lines := strings.Split(limbo, "\n")
	kept := make([]string, 0, len(lines))
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "\\input gwebmac" {
			continue
		}
		kept = append(kept, ln)
	}
	return strings.Join(kept, "\n")
}

func (wv *Weaver) writeSection(bw *bufio.Writer, sec *web.Section) {
	if sec.Starred {
		// A starred-section title is free TeX (it may contain \. typewriter and
		// other control sequences), so it is passed through processTex rather than
		// escaped like a refinement name.
		fmt.Fprintf(bw, "\n\\GN{%d}{%d}{%s}", sec.Depth, sec.Number, wv.processTex(sec.Number, sec.Title))
		// The commentary is whatever follows the title's terminating period (the
		// first period at end of text or followed by whitespace, matching the web
		// package's title rule so a period inside \. does not split early).
		rest := sec.Tex
		for i := 0; i < len(rest); i++ {
			if rest[i] == '.' && (i+1 == len(rest) || rest[i+1] == ' ' ||
				rest[i+1] == '\t' || rest[i+1] == '\n' || rest[i+1] == '\r') {
				rest = rest[i+1:]
				break
			}
		}
		bw.WriteString(wv.processTex(sec.Number, rest))
	} else {
		fmt.Fprintf(bw, "\n\\GM{%d}", sec.Number)
		bw.WriteString(wv.processTex(sec.Number, sec.Tex))
	}

	if sec.HasCode {
		// A code-only section (no commentary) runs in on the section-number line,
		// as cweave does: a named section's header (\GDr/\GDpr) and an unnamed
		// section's first code line (\GBr + \GLr) omit the usual break.
		runin := !sec.Starred && strings.TrimSpace(sec.Tex) == ""
		if sec.Name != "" {
			name := wv.w.Resolve(sec.Name)
			cont := wv.defNum[name] != sec.Number
			wv.xref.addSectionDef(name, sec.Number)
			macro := "\\GD"
			if cont {
				macro = "\\GDp" // continuation of an earlier definition
			}
			if runin {
				macro += "r"
			}
			fmt.Fprintf(bw, "\n%s{%d}{%s}", macro, wv.defNum[name], wv.renderName(name))
		}
		// With a named header the code always starts below it, so only an unnamed
		// code-only section runs its first code line in beside the number.
		runinCode := runin && sec.Name == ""
		if runinCode {
			bw.WriteString("\n\\GBr%\n")
		} else {
			bw.WriteString("\n\\GB%\n")
		}
		bw.WriteString(wv.renderCode(sec.Number, sec.Code, runinCode))
		bw.WriteString("\\GE\n")
		if sec.Name != "" {
			bw.WriteString(wv.crossRefNotes(wv.w.Resolve(sec.Name), sec.Number))
		}
	}
}

// renderCode formats a code part into a sequence of \GL code lines. Spacing
// mirrors the source: a run of tokens with no source whitespace between them
// becomes one tight math "chunk" (one TeX math group), and a gap becomes a
// breakable \GS space between chunks. Because gofmt-formatted Go already encodes
// the grammar in its spacing, this reproduces it exactly (pointer *T vs a * b,
// slice []T vs index a[i], and so on) and lets long lines wrap at \GS.
func (wv *Weaver) renderCode(secNum int, code string, runin bool) string {
	var out strings.Builder
	var line strings.Builder // the current source line: chunks joined by \GS
	var run strings.Builder  // the current tight chunk (one TeX math group)
	var st lexState
	indent := 0
	atLineStart := true
	pendingSpace := false
	forceDef := false     // set by @! to force the next identifier to index as a def
	haveContent := false  // at least one code line has been emitted
	blankPending := false // a blank source line is waiting to become a \GBK gap

	// prevSig* tracks the most recent significant token so that an identifier
	// following func/var/const/type can be flagged as a definition.
	prevSigKind := tkNewline
	prevSigText := ""

	flushRun := func() {
		if run.Len() > 0 {
			line.WriteString("$")
			line.WriteString(run.String())
			line.WriteString("$")
			run.Reset()
		}
	}
	emit := func(s string) {
		if pendingSpace {
			flushRun()
			line.WriteString("\\GS ")
			pendingSpace = false
		}
		run.WriteString(s)
		atLineStart = false
	}
	// emitLine writes the accumulated line as a \GL but leaves indent intact. A
	// blank source line between two code lines becomes a small \GBK gap, which
	// gives a little air between, e.g., the import block and the function body.
	emitLine := func() {
		flushRun()
		if strings.TrimSpace(line.String()) != "" {
			if blankPending {
				out.WriteString("\\GBK\n")
				blankPending = false
			}
			// The first line of an unnamed code-only section runs in beside the
			// section number (\GLr, no break); the rest are ordinary \GL lines.
			macro := "GL"
			if runin && !haveContent {
				macro = "GLr"
			}
			fmt.Fprintf(&out, "\\%s{%d}{%s}%%\n", macro, indent, line.String())
			haveContent = true
		} else if haveContent {
			blankPending = true
		}
		line.Reset()
	}
	// flushLine ends a source line.
	flushLine := func() {
		emitLine()
		indent = 0
		atLineStart = true
		pendingSpace = false
	}
	// forceBreak starts a fresh woven line at the same indent (@/), optionally
	// preceded by a blank line (@#).
	forceBreak := func(blank bool) {
		emitLine()
		if blank {
			out.WriteString("\\GBL\n")
		}
		atLineStart = false
		pendingSpace = false
	}

	for _, a := range web.ScanCode(code) {
		switch a.Kind {
		case web.AText:
			toks := lexGo(a.Text, &st)
			for k, t := range toks {
				switch t.kind {
				case tkNewline:
					flushLine()
				case tkSpace:
					if atLineStart {
						indent += indentLevel(t.text)
					} else {
						pendingSpace = true
					}
				default:
					if t.kind == tkIdent || t.kind == tkBuiltin {
						def := forceDef || isDefinition(prevSigKind, prevSigText, toks, k)
						forceDef = false
						if indexable(t.text) && !wv.noIndex[t.text] {
							if def {
								wv.xref.addIdentDef(t.text, secNum)
							} else {
								wv.xref.addIdentUse(t.text, secNum)
							}
						}
					}
					// A thin space (\Gthin, a tunable muskip) before a "(" that
					// directly follows a word (a function name, or a keyword like
					// func), as cweave does, so the paren does not jam against it:
					// f (x), func (...).
					if t.kind == tkOp && t.text == "(" && !pendingSpace && !atLineStart &&
						(prevSigKind == tkIdent || prevSigKind == tkKeyword || prevSigKind == tkBuiltin) {
						emit("\\Gthin ")
					}
					if t.kind == tkComment {
						emit(wv.renderComment(secNum, t.text))
					} else {
						emit(renderToken(token{kind: wv.effKind(t), text: t.text}))
					}
					prevSigKind, prevSigText = t.kind, t.text
				}
			}
		case web.ARef:
			name := wv.w.Resolve(a.Text)
			wv.xref.addSectionUse(name, secNum)
			emit(fmt.Sprintf("\\GX{%d}{%s}", wv.defNum[name], wv.renderName(name)))
		case web.AVerbatim:
			emit(fmt.Sprintf("\\GST{%s}", escTT(a.Text)))
		case web.ATeX:
			emit(a.Text)
		case web.AIndex:
			wv.xref.addManualIndex(a.Index, a.Text, secNum)
		case web.APaste:
			pendingSpace = false // join: no space before the next token
		case web.ALayout:
			switch a.Index {
			case ',': // thin space, stays within the current chunk
				emit("\\,")
			case '/': // force a line break at the same indent
				forceBreak(false)
			case '#': // force a line break preceded by a blank line
				forceBreak(true)
			case '|': // optional (zero-width) line break between chunks
				flushRun()
				line.WriteString("\\GSO ")
				pendingSpace = false
				atLineStart = false
			}
		case web.AIndexDef:
			forceDef = true // @!: the next identifier is a definition
		}
	}
	flushLine()
	return out.String()
}

// renderToken renders a single Go token as a TeX fragment (used inside math).
func renderToken(t token) string {
	switch t.kind {
	case tkKeyword, tkBuiltin:
		return "\\GKW{" + escIdent(t.text) + "}"
	case tkIdent:
		return "\\GID{" + escIdent(t.text) + "}"
	case tkMacro:
		if t.text == "nil" {
			// nil is Go's null value; show it with a symbol (\Gnil, a capital
			// lambda) as cweave shows C's NULL, rather than in typewriter.
			return "\\Gnil "
		}
		// Typewriter, like a CWEB  macro (an  name or a predeclared constant).
		// \GMAC wraps \tentex in an \hbox so it works in the math mode that code is
		// typeset in.
		return "\\GMAC{" + escTT(t.text) + "}"
	case tkNumber:
		return renderNumber(t.text)
	case tkString:
		return "\\GST{" + escTT(t.text) + "}"
	case tkComment:
		// Comments are set in roman (\GCM); escape them for roman text mode (not
		// the typewriter \charNN codes escTT emits), but let $...$ math through.
		// Tighten the leading "//" marker with a small kern (\Gcommentkern), whose
		// two slashes are otherwise set rather far apart in roman.
		if rest, ok := strings.CutPrefix(t.text, "//"); ok {
			return "\\GCM{/\\kern\\Gcommentkern/" + escComment(rest) + "}"
		}
		return "\\GCM{" + escComment(t.text) + "}"
	case tkOp:
		return renderOp(t.text)
	}
	return ""
}

func renderNumber(s string) string {
	if len(s) >= 2 && s[0] == '0' {
		switch s[1] {
		case 'x', 'X':
			return "\\Ghex{" + numDigits(s[2:]) + "}"
		case 'o', 'O':
			return "\\Goct{" + numDigits(s[2:]) + "}"
		case 'b', 'B':
			return "\\Gbin{" + numDigits(s[2:]) + "}"
		}
		if isOctalDigits(s[1:]) {
			return "\\Goct{" + numDigits(s[1:]) + "}"
		}
	}
	return "\\GNU{" + numDigits(s) + "}"
}

// isOctalDigits reports whether s is a nonempty run of octal digits (with
// optional _ separators) -- the tail of a classic 0NNN octal literal.
func isOctalDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; (c < '0' || c > '7') && c != '_' {
			return false
		}
	}
	return true
}

// numDigits renders the digits of a literal: a _ separator becomes a thin space;
// digits and hex letters are safe as is (no TeX specials occur in a number).
func numDigits(s string) string {
	return strings.ReplaceAll(s, "_", "\\,")
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
					fmt.Fprintf(&b, "\\GX{%d}{%s}", wv.defNum[name], wv.renderName(name))
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

// renderInline formats a |...| inline Go fragment (from prose) as one math
// group, recording identifier uses in section secNum.
func (wv *Weaver) renderInline(secNum int, code string) string {
	return wv.inlineCode(code, secNum, true)
}

// inlineCode formats a short Go fragment as one math group, mirroring the source
// whitespace (it is not wrapped). When record is true, identifier uses are added
// to the cross-reference under secNum.
func (wv *Weaver) inlineCode(code string, secNum int, record bool) string {
	var st lexState
	var b strings.Builder
	b.WriteString("$")
	pendingSpace := false
	started := false
	for _, t := range lexGo(code, &st) {
		switch t.kind {
		case tkSpace, tkNewline:
			if started {
				pendingSpace = true
			}
		default:
			if pendingSpace {
				b.WriteString("\\ ")
				pendingSpace = false
			}
			if record && (t.kind == tkIdent || t.kind == tkBuiltin) && indexable(t.text) && !wv.noIndex[t.text] {
				wv.xref.addIdentUse(t.text, secNum)
			}
			b.WriteString(renderToken(token{kind: wv.effKind(t), text: t.text}))
			started = true
		}
	}
	b.WriteString("$")
	return b.String()
}

func (wv *Weaver) renderComment(secNum int, text string) string {
	prefix := ""
	body := text
	if rest, ok := strings.CutPrefix(text, "//"); ok {
		prefix = "/\\kern\\Gcommentkern/"
		body = rest
	}
	return "\\GCM{" + prefix + wv.commentBody(secNum, body) + "}"
}

// commentBody escapes a comment for roman text mode but, as cweb does, renders a
// |...| span as inline Go code and lets a \.{...} typewriter span through
// verbatim (\. escapes its own argument). \| is a literal bar.
func (wv *Weaver) commentBody(secNum int, s string) string {
	var b, lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			b.WriteString(escComment(lit.String()))
			lit.Reset()
		}
	}
	n := len(s)
	for i := 0; i < n; {
		if s[i] == '\\' && i+1 < n && s[i+1] == '|' {
			lit.WriteByte('|')
			i += 2
			continue
		}
		// \.{...}: a typewriter span, passed through verbatim. Find the matching
		// close brace, skipping \-escaped characters (\\ \{ \} inside the span).
		if s[i] == '\\' && i+2 < n && s[i+1] == '.' && s[i+2] == '{' {
			j := i + 3
			for j < n && s[j] != '}' {
				if s[j] == '\\' && j+1 < n {
					j++
				}
				j++
			}
			if j < n { // matched close brace
				flush()
				b.WriteString(s[i : j+1])
				i = j + 1
				continue
			}
		}
		if s[i] == '|' {
			j := i + 1
			var code strings.Builder
			closed := false
			for j < n {
				if s[j] == '\\' && j+1 < n && s[j+1] == '|' {
					code.WriteByte('|')
					j += 2
					continue
				}
				if s[j] == '|' {
					closed = true
					break
				}
				code.WriteByte(s[j])
				j++
			}
			if !closed {
				lit.WriteByte('|') // an unmatched bar is a literal bar
				i++
				continue
			}
			flush()
			b.WriteString(wv.inlineCode(code.String(), secNum, true))
			i = j + 1
			continue
		}
		lit.WriteByte(s[i])
		i++
	}
	flush()
	return b.String()
}

// renderName typesets a section name for TeX text mode. A |...| span is set as
// inline code (as in CWEB section names); the rest passes through as TeX, so
// control sequences and math work. A literal bar is written backslash-bar.
// An file= name is a literal path: typeset it in typewriter, escaped.
func (wv *Weaver) renderName(name string) string {
	if wv.isFile[name] {
		return "\\.{" + escTT(name) + "}"
	}
	var b strings.Builder
	n := len(name)
	i := 0
	for i < n {
		if name[i] == '\\' && i+1 < n && name[i+1] == '|' {
			b.WriteString("|")
			i += 2
			continue
		}
		if name[i] == '|' {
			j := i + 1
			var code strings.Builder
			for j < n {
				if name[j] == '\\' && j+1 < n && name[j+1] == '|' {
					code.WriteByte('|')
					j += 2
					continue
				}
				if name[j] == '|' {
					break
				}
				code.WriteByte(name[j])
				j++
			}
			b.WriteString(wv.inlineCode(code.String(), 0, false))
			i = j + 1
			continue
		}
		start := i
		for i < n && name[i] != '|' && !(name[i] == '\\' && i+1 < n && name[i+1] == '|') {
			i++
		}
		// The non-code text of a name is TeX, passed through as in CWEB so that
		// control sequences and math typeset; the user escapes any specials.
		b.WriteString(name[start:i])
	}
	return b.String()
}

// indexable reports whether an identifier should appear in the index. The blank
// identifier "_" is excluded.
func indexable(name string) bool { return name != "_" }

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
