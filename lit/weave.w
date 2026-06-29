@* The \.{weave} package.
This package implements \.{gweave}: it turns a parsed web into a TeX document
with pretty-printed Go code (bold reserved words, italic identifiers), linked
section references, and -- assembled in the cross-reference part below -- an
index and a list of section names. It is the Go analogue of CWEB's \.{cweave}.
@(internal/weave/weave.go@>=
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

@ A |Weaver| carries the per-document state: the map from a named section to its
first defining section, the \.{@@f}/\.{@@s} format overrides, and the
cross-reference tables (built lazily).
@(internal/weave/weave.go@>=
// Weaver turns a parsed web into woven TeX.
type Weaver struct {
	w      *web.Web
	defNum map[string]int // canonical named-section -> first defining section

	format  map[string]tokKind // @@f/@@s: identifier -> the token class to use
	noIndex map[string]bool    // @@s: identifiers omitted from the index
	isFile  map[string]bool    // @@(file@@>= outputs: names are literal file paths

	xref *xref // identifier and section cross-references (built lazily)
}

@ |New| records the first defining section of each refinement and installs the
global and per-section format directives (later ones win).
@(internal/weave/weave.go@>=
// New builds a Weaver for the given web.
func New(w *web.Web) *Weaver {
	wv := &Weaver{
		w:       w,
		defNum:  map[string]int{},
		format:  map[string]tokKind{},
		noIndex: map[string]bool{},
		isFile:  map[string]bool{},
	}
	// Both refinements and @@(file@@>= outputs get a defining section number, so
	// their headlines and links resolve; only @@(file@@>= names are never the
	// target of a @@<...@@> reference. A file name is a literal path, not TeX, so
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
	// class of identifier a (@@f a b) is the class b would be typeset in.
	apply := func(fs []web.Format) {
		for _, f := range fs {
			if f.Macro {
				wv.format[f.Original] = tkMacro // @d: typewriter, like a CWEB macro
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
	// types. (Typewriter treatment is applied only on request, with |@@d|.) An
	// explicit |@@f|/|@@s| above still wins.
	wv.detectDecls("type", tkBuiltin)
	return wv
}

@ cweave sets names declared with |type| in bold, like the predeclared types, and
GWEB does the same. |detectDecls| scans the code for declarations introduced by
|keyword| --- both |keyword NAME ...| and the block form |keyword (...)| --- and
records each declared name with |kind| (unless an |@@f|/|@@s| directive already
classified it). This is a heuristic scan, not a full Go parse, but it covers the
forms that occur in practice; a type name you want left in italic can be reset
with |@@f NAME int|, and any name can be set in typewriter with |@@d|.
@(internal/weave/weave.go@>=
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

@ |scanDecls| walks a token list and, at each |keyword| (here |type|), records the
declared name. The keyword followed by |(| opens a parenthesized group of
declarations, each naming an entry on its own line; |scanDeclGroup| collects those
until the matching |)|, tracking brace and bracket nesting so that struct fields
are not mistaken for names. (A |type| inside a type switch, |x.(type)|, is
followed by |)| and so names nothing.)
@(internal/weave/weave.go@>=
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

@ |effKind| returns the token class to typeset a token in, honoring \.{@@f}/\.{@@s}
overrides for identifiers, keywords, and builtins.
@(internal/weave/weave.go@>=
// effKind returns the token class to typeset t in, honoring @@f/@@s overrides.
func (wv *Weaver) effKind(t token) tokKind {
	switch t.kind {
	case tkIdent, tkKeyword, tkBuiltin:
		if k, ok := wv.format[t.text]; ok {
			return k
		}
	}
	return t.kind
}

@ |Weave| writes the whole document. It runs two passes: the first is discarded
and only fills the cross-reference tables (so a ``used in section'' note can be
printed under a definition even when the use occurs later); the second produces
the real output. \.{gweave} supplies the macro package itself.
@(internal/weave/weave.go@>=
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

@ |Weave| emits the macro package itself, so any stray copy of it in the limbo
is dropped.
@(internal/weave/weave.go@>=
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

@ |writeSection| emits one section: its headline (starred or numbered), its
commentary, and -- if present -- its code part bracketed by |\GB|...|\GE|, with
the definition headline and cross-reference notes for a named section.
@(internal/weave/weave.go@>=
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

@ |renderCode| formats a code part into a sequence of |\GL| code lines. The
spacing mirrors the source: a run of tokens with no source whitespace becomes
one tight math ``chunk'' ($\ldots$), and a gap becomes a breakable |\GS| space
between chunks. Because |gofmt| already encodes the grammar in its spacing, this
reproduces it exactly (pointer |*T| vs.\ |a * b|, slice |[]T| vs.\ index |a[i]|)
and lets long lines wrap at |\GS|.
@(internal/weave/weave.go@>=
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
	forceDef := false     // set by @@! to force the next identifier to index as a def
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
	// forceBreak starts a fresh woven line at the same indent (@@/), optionally
	// preceded by a blank line (@@#).
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
			forceDef = true // @@!: the next identifier is a definition
		}
	}
	flushLine()
	return out.String()
}

@ |renderToken| renders a single Go token as a TeX fragment (used inside math).
@(internal/weave/weave.go@>=
// renderToken renders a single Go token as a TeX fragment (used inside math).
func renderToken(t token) string {
	switch t.kind {
	case tkKeyword, tkBuiltin:
		return "\\GKW{" + escIdent(t.text) + "}"
	case tkIdent:
		return "\\GID{" + escIdent(t.text) + "}"
	case tkMacro:
		// Typewriter, like a CWEB @d macro (an @d name or a predeclared constant).
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

@ |renderNumber| classifies a numeric literal the way cweave does. A hexadecimal
literal (|0x|\dots) is set in typewriter with a superscript |#|; an octal literal
(a classic |0|\dots, or |0o|\dots) gets a small raised circle and oldstyle italic
digits; a binary literal (|0b|\dots) a superscript |b|; a decimal or floating
literal stays roman. A |_| digit separator becomes a thin space.
@(internal/weave/weave.go@>=
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

@ |processTex| transforms commentary: |Go code| inline, \.{@@<refs@@>}, \.{@@@@} to
a literal at-sign, and index entries (\.{@@\^ @@. @@:}) are recorded and removed.
Everything else -- the user's TeX -- passes through unchanged.
@(internal/weave/weave.go@>=
// processTex transforms commentary: |Go code| inline, @@<refs@@>, @@@@->@@, and
// index entries (@@^ @@. @@:) are recorded and removed. Everything else (the
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
		if c == '@@' && i+1 < n {
			switch d := s[i+1]; d {
			case '@@':
				b.WriteByte('@@')
				i += 2
				continue
			case '<':
				if end := strings.Index(s[i+2:], "@@>"); end >= 0 {
					end += i + 2
					name := wv.w.Resolve(strings.TrimSpace(s[i+2 : end]))
					wv.xref.addSectionUse(name, secNum)
					fmt.Fprintf(&b, "\\GX{%d}{%s}", wv.defNum[name], wv.renderName(name))
					i = end + 2
					continue
				}
			case '^', '.', ':':
				if end := strings.Index(s[i+2:], "@@>"); end >= 0 {
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

@ |renderInline| and |inlineCode| format a |...| inline Go fragment from prose
as one math group, mirroring the source whitespace (such fragments are not
wrapped).
@(internal/weave/weave.go@>=
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

@ |renderComment| typesets a code comment. As in cweb, a |...| span inside a
comment is set as the Go code it represents (via |inlineCode|), and the text
around it is escaped for roman text mode (|$...$| math still passes through). A
literal bar is written |\\||. The whole thing is wrapped in |\GCM|, with the
leading \.{//} tightened by a small kern.
@(internal/weave/weave.go@>=
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

@ |renderName| typesets a section name for text mode: a |...| span is set as
inline code, as in CWEB section names, and the rest is passed through verbatim as
TeX, so control sequences (a typewriter group, say) and math typeset, exactly as
in a starred-section title. A |@@(file@@>=| name is different: it is a literal
file path, not TeX, so it is set in typewriter with its specials escaped (an
underscore in a name like \.{squint\_test.go} would otherwise derail \TeX).
@(internal/weave/weave.go@>=
// renderName typesets a section name for TeX text mode. A |...| span is set as
// inline code (as in CWEB section names); the rest passes through as TeX, so
// control sequences and math work. A literal bar is written backslash-bar.
// An @(file@>= name is a literal path: typeset it in typewriter, escaped.
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

@ |indexable| excludes the blank identifier from the index, and |declKeywords|
lists the keywords that introduce a declaration.
@(internal/weave/weave.go@>=
// indexable reports whether an identifier should appear in the index. The blank
// identifier "_" is excluded.
func indexable(name string) bool { return name != "_" }

var declKeywords = map[string]bool{
	"func": true, "var": true, "const": true, "type": true,
}

@ |isDefinition| heuristically decides whether an identifier is being declared:
it follows a |func|/|var|/|const|/|type| keyword, or it is immediately followed
by |:=|. This is best-effort -- there is no full Go parse -- but it covers the
cases CWEB underlines in its index.
@(internal/weave/weave.go@>=
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

@ |indentLevel| measures a leading-whitespace run: one level per tab, plus one
per four spaces.
@(internal/weave/weave.go@>=
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

@* A Go lexer for the woven output.
Unlike |go/scanner| this lexer tolerates the partial fragments found in web
sections and reports whitespace, newlines, and comments as tokens so the
pretty-printer can preserve layout. State (an open block comment or raw string)
is carried across calls because a code part may be interrupted by \.{@@<...@@>}
references.
@(internal/weave/gotok.go@>=
package weave

// A small, line-oriented Go lexer for the woven output. Unlike go/scanner it
// tolerates the partial fragments found in web sections and reports whitespace,
// newlines, and comments as tokens so the pretty-printer can preserve layout.
// State (open block comment / raw string) is carried across calls because a
// code part may be interrupted by @@<...@@> references.

type tokKind int

@ The token kinds.
@(internal/weave/gotok.go@>=
const (
	tkIdent   tokKind = iota // ordinary identifier
	tkKeyword                // Go reserved word
	tkBuiltin                // predeclared type or constant (also set bold)
	tkNumber                 // numeric literal
	tkString                 // "..." or `...` or '...'
	tkComment                // // or /* */ text (no trailing newline)
	tkOp                     // operator or punctuation run
	tkSpace                  // a run of spaces/tabs
	tkNewline                // a single '\n'
	tkMacro                  // typewriter: an @d name or a predeclared constant
)

@ A |token| pairs a kind with its text; |lexState| carries the cross-fragment
state.
@(internal/weave/gotok.go@>=
type token struct {
	kind tokKind
	text string
}

// lexState carries lexer state across fragments of one code part.
type lexState struct {
	inBlockComment bool
	inRawString    bool
}

@ The reserved words and the predeclared types and constants (both set bold).
@(internal/weave/gotok.go@>=
var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

var goBuiltins = map[string]bool{
	"bool": true, "byte": true, "complex64": true, "complex128": true, "error": true,
	"float32": true, "float64": true, "int": true, "int8": true, "int16": true,
	"int32": true, "int64": true, "rune": true, "string": true, "uint": true,
	"uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true,
	"any": true, "comparable": true,
}

// The predeclared constant values are set in typewriter (like a const), not bold
// like the predeclared types; they denote values, not types.
var goConstants = map[string]bool{"nil": true, "true": true, "false": true, "iota": true}

@ |classifyWord| maps a word to its class; the character-class predicates follow
the Go spec closely enough for typesetting. The predeclared constants |nil|,
|true|, and |false| are set in typewriter rather than bold --- they are constant
values, not types, so they read like the other constants.
@(internal/weave/gotok.go@>=
func classifyWord(w string) tokKind {
	switch {
	case goKeywords[w]:
		return tkKeyword
	case goConstants[w]:
		return tkMacro // a predeclared constant: typewriter, like a const
	case goBuiltins[w]:
		return tkBuiltin
	default:
		return tkIdent
	}
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}
func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}
func isDigit(c byte) bool { return c >= '0' && c <= '9' }

@ |lexGo| tokenizes a fragment, updating |*st|. Newlines and whitespace runs are
returned as their own tokens.
@(internal/weave/gotok.go@>=
// lexGo tokenizes src, updating *st. Newlines and whitespace runs are returned
// as their own tokens.
func lexGo(src string, st *lexState) []token {
	var toks []token
	n := len(src)
	i := 0
	for i < n {
		// Resume an open block comment.
		if st.inBlockComment {
			if end := indexStr(src, "*/", i); end >= 0 {
				toks = append(toks, token{tkComment, src[i : end+2]})
				st.inBlockComment = false
				i = end + 2
			} else if nl := indexByte(src, '\n', i); nl >= 0 {
				if nl > i {
					toks = append(toks, token{tkComment, src[i:nl]})
				}
				toks = append(toks, token{tkNewline, "\n"})
				i = nl + 1
			} else {
				toks = append(toks, token{tkComment, src[i:]})
				i = n
			}
			continue
		}
		// Resume an open raw string.
		if st.inRawString {
			if end := indexByte(src, '`', i); end >= 0 {
				toks = append(toks, token{tkString, src[i : end+1]})
				st.inRawString = false
				i = end + 1
			} else if nl := indexByte(src, '\n', i); nl >= 0 {
				if nl > i {
					toks = append(toks, token{tkString, src[i:nl]})
				}
				toks = append(toks, token{tkNewline, "\n"})
				i = nl + 1
			} else {
				toks = append(toks, token{tkString, src[i:]})
				i = n
			}
			continue
		}

		c := src[i]
		switch {
		case c == '\n':
			toks = append(toks, token{tkNewline, "\n"})
			i++
		case c == ' ' || c == '\t' || c == '\r':
			j := i
			for j < n && (src[j] == ' ' || src[j] == '\t' || src[j] == '\r') {
				j++
			}
			toks = append(toks, token{tkSpace, src[i:j]})
			i = j
		case c == '/' && i+1 < n && src[i+1] == '/':
			j := indexByte(src, '\n', i)
			if j < 0 {
				j = n
			}
			toks = append(toks, token{tkComment, src[i:j]})
			i = j
		case c == '/' && i+1 < n && src[i+1] == '*':
			if end := indexStr(src, "*/", i+2); end >= 0 {
				toks = append(toks, token{tkComment, src[i : end+2]})
				i = end + 2
			} else if nl := indexByte(src, '\n', i); nl >= 0 {
				toks = append(toks, token{tkComment, src[i:nl]})
				toks = append(toks, token{tkNewline, "\n"})
				st.inBlockComment = true
				i = nl + 1
			} else {
				toks = append(toks, token{tkComment, src[i:]})
				st.inBlockComment = true
				i = n
			}
		case c == '"':
			i = lexQuoted(src, i, '"', &toks)
		case c == '\'':
			i = lexQuoted(src, i, '\'', &toks)
		case c == '`':
			if end := indexByte(src, '`', i+1); end >= 0 {
				toks = append(toks, token{tkString, src[i : end+1]})
				i = end + 1
			} else if nl := indexByte(src, '\n', i+1); nl >= 0 {
				toks = append(toks, token{tkString, src[i:nl]})
				toks = append(toks, token{tkNewline, "\n"})
				st.inRawString = true
				i = nl + 1
			} else {
				toks = append(toks, token{tkString, src[i:]})
				st.inRawString = true
				i = n
			}
		case isIdentStart(c):
			j := i + 1
			for j < n && isIdentPart(src[j]) {
				j++
			}
			w := src[i:j]
			toks = append(toks, token{classifyWord(w), w})
			i = j
		case isDigit(c) || (c == '.' && i+1 < n && isDigit(src[i+1])):
			j := i + 1
			for j < n && isNumberPart(src[j]) {
				j++
			}
			toks = append(toks, token{tkNumber, src[i:j]})
			i = j
		default:
			if l := matchOp(src, i); l > 0 {
				toks = append(toks, token{tkOp, src[i : i+l]})
				i += l
			} else {
				toks = append(toks, token{tkOp, string(c)})
				i++
			}
		}
	}
	return toks
}

@ The multi-character operators (longest first) and the greedy matcher that
combines them into single tokens. The empty pairs |[]| and |{}| are kept whole
so the typesetter can give them a thin space.
@(internal/weave/gotok.go@>=
// multiOps lists Go's multi-character operators, longest first, so matchOp can
// greedily combine them into single tokens.
var multiOps = []string{
	"<<=", ">>=", "&^=", "...",
	"<-", "++", "--", "==", "!=", "<=", ">=", ":=", "&&", "||",
	"<<", ">>", "&^", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=",
	"[]", // the empty brackets of a slice/array type, kept as one token
	"{}", // empty braces (struct{}, interface{}, T{}), kept as one token
}

func matchOp(src string, i int) int {
	for _, op := range multiOps {
		if i+len(op) <= len(src) && src[i:i+len(op)] == op {
			return len(op)
		}
	}
	return 0
}

@ |lexQuoted| scans an interpreted string or rune literal, honoring backslash
escapes and tolerating an unterminated literal.
@(internal/weave/gotok.go@>=
// lexQuoted scans an interpreted string ("...") or rune ('...') starting at i,
// honoring backslash escapes, and appends a tkString token. It stops at the
// closing quote or end of line (unterminated literals are tolerated).
func lexQuoted(src string, i int, quote byte, toks *[]token) int {
	n := len(src)
	j := i + 1
	for j < n {
		if src[j] == '\\' && j+1 < n {
			j += 2
			continue
		}
		if src[j] == quote || src[j] == '\n' {
			break
		}
		j++
	}
	if j < n && src[j] == quote {
		j++
	}
	*toks = append(*toks, token{tkString, src[i:j]})
	return j
}

@ Number characters and two small string-search helpers.
@(internal/weave/gotok.go@>=
func isNumberPart(c byte) bool {
	// Note: '+'/'-' (exponent signs) are intentionally excluded so that "1+2"
	// is not swallowed as a single number; "1e+10" splits harmlessly instead.
	return isDigit(c) || c == '.' || c == '_' ||
		(c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') ||
		c == 'x' || c == 'X' || c == 'o' || c == 'O' || c == 'b' || c == 'B' ||
		c == 'p' || c == 'P'
}

func indexByte(s string, b byte, from int) int {
	for i := from; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func indexStr(s, sub string, from int) int {
	for i := from; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

@* TeX escaping.
Three contexts need different treatment: identifiers and keywords (only |_| is
troublesome); typewriter text for strings and comments (every TeX special is
emitted as a |\charNN| code so it prints literally); and prose names and math
operators (text- or math-mode-safe sequences).
@(internal/weave/tex.go@>=
package weave

import (
	"fmt"
	"strings"
)

// TeX escaping. Three contexts need different treatment:
//
//   - identifiers/keywords: only '_' is troublesome (\_ works in text mode);
//   - typewriter text (strings, comments): every TeX special is emitted as a
//     \charNN code so it prints literally regardless of the current font;
//   - prose names and math operators: text-mode / math-mode safe sequences.

@ |escIdent| escapes an identifier or keyword for text mode.
@(internal/weave/tex.go@>=
// escIdent escapes an identifier or keyword for text mode.
func escIdent(s string) string {
	return strings.ReplaceAll(s, "_", "\\_")
}

@ |escTT| escapes arbitrary text for a typewriter box.
@(internal/weave/tex.go@>=
// escTT escapes arbitrary text for a typewriter (\tt) box. Specials become
// \charNN so braces, backslashes, etc. survive.
func escTT(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\', '{', '}', '$', '&', '#', '%', '^', '_', '~':
			fmt.Fprintf(&b, "\\char%d ", c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

@ |escMathOp| encodes an operator run so it is safe inside math mode.
@(internal/weave/tex.go@>=
// escMathOp encodes an operator/punctuation run so it is safe inside math mode.
func escMathOp(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '{':
			b.WriteString("\\{")
		case '}':
			b.WriteString("\\}")
		case '&':
			b.WriteString("\\&")
		case '#':
			b.WriteString("\\#")
		case '%':
			b.WriteString("\\%")
		case '$':
			b.WriteString("\\$")
		case '_':
			b.WriteString("\\_")
		case '^':
			b.WriteString("\\char94 ")
		case '~':
			b.WriteString("\\char126 ")
		case '|':
			b.WriteString("\\char124 ")
		case '\\':
			b.WriteString("\\backslash ")
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

@ |renderOp| typesets a Go operator as a single tight math atom, using real math
symbols where they exist. Because inter-token spacing comes from the source, the
unary/binary distinction for |*|, |&|, and friends needs no grammar analysis.
@(internal/weave/tex.go@>=
// renderOp typesets a Go operator token as a single tight math atom (no math
// spacing of its own), using real math symbols where they exist. Inter-token
// spacing is supplied by the surrounding source whitespace, so the result
// reproduces gofmt's spacing exactly and the unary/binary distinction for *, &,
// etc. needs no grammar analysis.
func renderOp(s string) string {
	switch s {
	case "<=":
		return "\\mathord{\\leq}"
	case ">=":
		return "\\mathord{\\geq}"
	case "!=":
		return "\\mathord{\\neq}"
	case "==":
		return "\\mathord{\\equiv}" // equality test, as cweb (an equivalence sign)
	case "!":
		return "\\mathord{\\lnot}" // logical not, as cweb (a negation sign)
	case "&&":
		return "\\mathord{\\land}" // logical and, as cweb (a wedge)
	case "||":
		return "\\mathord{\\lor}" // logical or, as cweb (a vee)
	case "<-":
		return "\\mathord{\\leftarrow}"
	case "^":
		return "\\mathord{\\oplus}" // bitwise xor, as cweb (a circled plus)
	case "^=":
		return "\\mathord{\\oplus}\\mathord{=}" // xor-assign: ^ is a circled plus too
	case "&^":
		return "\\mathord{\\&}\\mathord{\\oplus}" // bit clear (and-not): ^ as circled plus
	case "&^=":
		return "\\mathord{\\&}\\mathord{\\oplus}\\mathord{=}" // and-not-assign
	case "<<":
		return "\\mathord{\\ll}" // left shift, as cweb (a tight double angle)
	case ">>":
		return "\\mathord{\\gg}" // right shift
	case "<<=":
		return "\\mathord{\\ll}\\mathord{=}"
	case ">>=":
		return "\\mathord{\\gg}\\mathord{=}"
	case "...":
		return "\\mathord{\\ldots}"
	case "[]":
		// empty slice/array brackets: a thin space keeps them from jamming
		return "\\mathord{[}\\,\\mathord{]}"
	case "{}":
		// empty braces (struct{}, interface{}, T{}): likewise a thin space
		return "\\mathord{\\{}\\,\\mathord{\\}}"
	}
	if len(s) == 1 {
		return "\\mathord{" + escMathOp(s) + "}"
	}
	return tightMathOp(s)
}

@ |tightMathOp| sets each character of an operator as an ordinary atom, so |==|
or |&&| prints with its characters adjacent.
@(internal/weave/tex.go@>=
// tightMathOp encodes each character of an operator as an ordinary atom, so that
// e.g. "==" or "<<" prints with the characters adjacent rather than spaced.
func tightMathOp(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		b.WriteString("\\mathord{")
		b.WriteString(escMathOp(s[i : i+1]))
		b.WriteString("}")
	}
	return b.String()
}

@ |escProse| escapes text for ordinary roman text mode (used for section names).
@(internal/weave/tex.go@>=
// escProse escapes text for ordinary roman text mode (used for section names).
func escProse(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '_':
			b.WriteString("\\_")
		case '&':
			b.WriteString("\\&")
		case '#':
			b.WriteString("\\#")
		case '%':
			b.WriteString("\\%")
		case '$':
			b.WriteString("\\$")
		case '{':
			b.WriteString("$\\{$")
		case '}':
			b.WriteString("$\\}$")
		case '\\':
			b.WriteString("$\\backslash$")
		case '^':
			b.WriteString("\\^{}")
		case '~':
			b.WriteString("\\~{}")
		case '<':
			b.WriteString("$<$") // cmr (OT1) has no < glyph; use math
		case '>':
			b.WriteString("$>$") // likewise for >
		case '|':
			b.WriteString("$\\vert$")
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

@ |escComment| is like |escProse| but lets a |$...$| span pass through verbatim,
so TeX math works inside a comment (as in cweb); everything outside the math is
still escaped for roman text mode.
@(internal/weave/tex.go@>=
// escComment escapes a comment for roman text mode, but passes a $...$ span
// through unescaped so TeX math works inside comments (as in cweb).
func escComment(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '$' {
			if k := strings.IndexByte(s[i+1:], '$'); k >= 0 {
				j := i + 1 + k
				b.WriteString(s[i : j+1]) // the $...$ math span, verbatim
				i = j + 1
				continue
			}
		}
		b.WriteString(escProse(s[i : i+1]))
		i++
	}
	return b.String()
}

@* Cross-references and the index.
The |xref| tables accumulate, during the first weaving pass, where each
identifier is used and (heuristically) defined, where each named section is
defined and used, and the manual index entries from \.{@@\^ @@. @@:}. They are then
consulted during the real pass and when emitting the back matter.
@(internal/weave/xref.go@>=
package weave

import (
	"bufio"
	"fmt"
	"sort"
	"strings"

	"github.com/sjnam/gweb/internal/web"
)

@ The tables themselves and a manual index entry.
@(internal/weave/xref.go@>=
// xref accumulates cross-reference information while a web is woven:
//   - where each identifier is used and (heuristically) defined;
//   - where each named section is defined and used;
//   - manual index entries from @@^ @@. @@: control codes.
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

@ The constructor and the small accumulator helpers.
@(internal/weave/xref.go@>=
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

@ |sortedKeys| orders a section set, and |secList| renders it as hyperlinks with
the defining sections underlined.
@(internal/weave/xref.go@>=
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

@ |writeBackMatter| emits the PDF bookmarks, the index, the list of named
sections, and the table of contents that close a woven document.
@(internal/weave/xref.go@>=
// writeBackMatter emits the index, the list of named sections, and the table of
// contents that close a woven document.
func (wv *Weaver) writeBackMatter(bw *bufio.Writer) {
	wv.writeBookmarks(bw)
	bw.WriteString("\n\\Ginx\n")
	wv.writeIndex(bw)
	bw.WriteString("\\Gfin\n")
	// A destination at the top of the section-names page, targeted by the "Names
	// of the sections" bookmark. Its number is one past the last section, so it
	// never collides with a section's own destination.
	fmt.Fprintf(bw, "\\Gdest{%d}%%\n", len(wv.w.Sections)+1)
	wv.writeSectionNames(bw)
	bw.WriteString("\\Gcon\n\\end\n")
}

@ |writeBookmarks| emits one |\Gbookmark| per starred section, in document
order, so a PDF outline can be built whose nesting follows the \.{@@*}, \.{@@*1},
\.{@@*2} depths. Each entry carries its depth (for the dvipdfmx route, which
nests by level) and its number of direct children (for pdftex's count model). A
final top-level entry, \.{Names of the sections}, lists every section name as a
collapsible child linking to its defining section, as cweave does.
@(internal/weave/xref.go@>=
// writeBookmarks emits one \Gbookmark per starred section, in document (pre)
// order, so a PDF outline can be built whose nesting follows the @@*, @@*1,
// @@*2 ... depths. Each entry carries its depth (the dvipdfmx route nests by
// level) and its number of direct children (pdftex's count model). A final
// top-level "Names of the sections" entry lists every section name as a
// collapsible child linking to its defining section.
func (wv *Weaver) writeBookmarks(bw *bufio.Writer) {
	var starred []*web.Section
	for _, s := range wv.w.Sections {
		if s.Starred {
			starred = append(starred, s)
		}
	}
	bw.WriteString("\n\\par")
	topDepth := 0
	for i, s := range starred {
		children := 0
		for j := i + 1; j < len(starred) && starred[j].Depth > s.Depth; j++ {
			if starred[j].Depth == s.Depth+1 {
				children++
			}
		}
		if s.Depth < topDepth {
			topDepth = s.Depth
		}
		fmt.Fprintf(bw, "\\Gbookmark{%d}{%d}{%d}{%s}%%\n", s.Depth, s.Number, children, bookmarkTitle(s.Title))
	}
	// A top-level "Names of the sections" entry (linking to the destination on
	// that page, one past the last section) with every section name listed
	// beneath it, each linking to its defining section, as cweave does. The
	// negative child count starts the group collapsed; the reader can expand it.
	// \Goutsecname holds the title, which the Korean backend localizes.
	var names []string
	for _, n := range wv.sortedSectionNames() {
		if wv.defNum[n] > 0 {
			names = append(names, n)
		}
	}
	fmt.Fprintf(bw, "\\Gbookmark{%d}{%d}{%d}{\\Goutsecname}%%\n", topDepth, len(wv.w.Sections)+1, -len(names))
	for _, n := range names {
		fmt.Fprintf(bw, "\\Gbookmark{%d}{%d}{0}{%s}%%\n", topDepth+1, wv.defNum[n], bookmarkTitle(n))
	}
}

@ |bookmarkTitle| reduces a starred-section title to plain text safe for a PDF
outline: a |...| span keeps its inner text, \.{@@@@} becomes an at-sign, and the
(rare) TeX-special characters are dropped.
@(internal/weave/xref.go@>=
// bookmarkTitle reduces a starred-section title to plain text safe for a PDF
// outline: |code| spans keep their inner text, @@@@ becomes @@, and TeX-special
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
		case c == '@@' && i+1 < n && raw[i+1] == '@@':
			b.WriteByte('@@')
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

@ The index. Each |indexItem| collects the sections where an entry appears;
|writeIndex| merges identifier uses and definitions with the manual entries,
sorts them case-insensitively, and emits one |\GII| line apiece.
@(internal/weave/xref.go@>=
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

	// An identifier's index head follows its display class: a typewriter name
	// (@d or a predeclared constant) is set in typewriter, everything else italic.
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

@ |writeSectionNames| emits the list of named sections with their defining and
using section numbers. |sortedSectionNames| gives the shared ordering used both
here and for the PDF outline children beneath ``Names of the sections''.
@(internal/weave/xref.go@>=
// writeSectionNames emits the list of named sections with their defining and
// using section numbers.
func (wv *Weaver) writeSectionNames(bw *bufio.Writer) {
	for _, n := range wv.sortedSectionNames() {
		fmt.Fprintf(bw, "\\GNS{%s}{%d}{%s}\n",
			wv.renderName(n), wv.defNum[n], usedNote(wv.xref.sectionUses[n]))
	}
}

// sortedSectionNames returns every section name (defined or used), ordered
// case-insensitively, as it appears on the section-names page and in the PDF
// outline beneath "Names of the sections".
func (wv *Weaver) sortedSectionNames() []string {
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
	return sorted
}

@ |usedNote| renders the ``Used in section(s) \dots'' note for the section-names
list, or |""| when the section is never used. The wording is deferred to the
|\GNused|/|\GNuseds| macros (singular/plural) so a localization file can
translate it, exactly as |\GU|/|\GUs| do for the under-definition notes.
@(internal/weave/xref.go@>=
// usedNote renders the "Used in section(s) ..." note for the section-names list,
// or "" when the section is never used. It emits a \GNused/\GNuseds macro so the
// wording can be localized, like the \GU/\GUs notes under a definition.
func usedNote(uses map[int]bool) string {
	if len(uses) == 0 {
		return ""
	}
	macro := "\\GNused"
	if len(uses) > 1 {
		macro = "\\GNuseds"
	}
	return macro + "{" + secList(uses, nil) + "}"
}

@ |crossRefNotes| returns the ``also defined in'' and ``used in'' notes printed
under the first definition of a named section.
@(internal/weave/xref.go@>=
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
