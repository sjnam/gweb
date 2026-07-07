@* Command \.{gweave}.
This is the command-line front end of \.{gweave}; the weave engine it drives is
defined in the second half of this web. The input may be named with or without
its \.{.w} extension (|gweave wc| reads \.{wc.w}, as in cweb). The woven document
is written to the input's base name with a \.{.tex} extension; process it with a
\TEX/ engine that can find \.{gwebmac.tex} to produce a PDF.
@c
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sjnam/gweb/common"
)

@<The command's entry point@>
@<Print a usage message@>
@<Report a progress line@>
@<Weave the web and write the \TEX/@>

@ The entry point parses the flags and arguments and dispatches to |run|. With
\.{-version} it just prints the version; otherwise it prints a one-line banner
to the standard error, in the style of \.{CWEB}, before processing.
@<The command's entry point@>=
func main() {
	outDir := flag.String("o", "", "output directory (default: input file's directory)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = usage
	flag.Parse()
	if *showVersion {
		fmt.Printf("gweave (GWEB) %s\n", common.Version)
		return
	}
	if flag.NArg() < 1 || flag.NArg() > 2 {
		usage()
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "This is GWEAVE, Version %s\n", common.Version)
	if err := run(flag.Arg(0), flag.Arg(1), *outDir); err != nil {
		fmt.Fprintln(os.Stderr, "gweave:", err)
		os.Exit(1)
	}
}

@ Usage.
@<Print a usage message@>=
func usage() {
	fmt.Fprintln(os.Stderr, "usage: gweave [-o dir] file[.w] [change[.ch]]")
	flag.PrintDefaults()
}

@ A brief progress report in the style of \.{CWEB}: one \.{*N} on the standard error
for each starred (chapter) section, giving a sense of the web's structure as it
is processed.
@<Report a progress line@>=
func reportProgress(w *common.Web) {
	for _, s := range w.Sections {
		if s.Starred {
			fmt.Fprintf(os.Stderr, "*%d", s.Number)
		}
	}
	fmt.Fprintln(os.Stderr)
}

@ |run| supplies the default \.{.w} (and \.{.ch}) extension, parses the web
(applying a change file if given), prints any warnings and a short progress
report, and writes the woven \TEX/.
@<Weave the web and write the \TEX/@>=
func run(input, changeFile, outDir string) error {
	input = common.DefaultExt(input, ".w")
	changeFile = common.DefaultExt(changeFile, ".ch")
	w, err := common.ParseWithChange(input, changeFile)
	if err != nil {
		return err
	}
	for _, warn := range w.Warnings {
		fmt.Fprintln(os.Stderr, "gweave: warning:", warn)
	}
	reportProgress(w)
	if outDir == "" {
		outDir = filepath.Dir(input)
	}
	base := filepath.Base(input)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	outPath := filepath.Join(outDir, base+".tex")

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := New(w).Weave(f); err != nil {
		return err
	}
	fmt.Printf("gweave: wrote %s\n", outPath)
	return nil
}

@* The weave engine.
The rest of this web is the engine that \.{gweave}'s front end drives: it turns a
parsed web into a \TEX/ document with pretty-printed \GO/ code (bold reserved words,
italic identifiers), linked section references, and -- assembled in the
cross-reference part below -- an index and a list of section names, the \GO/
analogue of \.{CWEB}'s \.{cweave}. It is part of the command's \.{main} package,
tangled together with the front end into the single file \.{gweave.go}.
@c
@<The weaver's state@>
@<Create a weaver@>
@<Detect type declarations@>
@<Scan a declaration group@>
@<Detect iota constant declarations@>
@<The effective token class@>
@<Weave the document in two passes@>
@<Drop a stray gwebmac input@>
@<Write one section@>
@<Render a code part@>
@<Space code tokens by grammar@>
@<Render one token@>
@<Render a numeric literal@>
@<Process commentary \TEX/@>
@<Render an inline code fragment@>
@<Render a code comment@>
@<Render a section name@>
@<Index predicates and declaration keywords@>
@<Decide whether an identifier is a definition@>
@<Track structural indentation@>

@ A |Weaver| carries the per-document state: the map from a named section to its
first defining section, the \.{@@f}/\.{@@s} format overrides, and the
cross-reference tables (built lazily).
@<The weaver's state@>=
type Weaver struct {
	w      *common.Web
	defNum map[string]int // canonical named-section $\rightarrow$ first defining section

	format  map[string]tokKind // \.{@@f}/\.{@@s}: identifier $\rightarrow$ the token class to use
	noIndex map[string]bool    // \.{@@s}: identifiers omitted from the index
	isFile  map[string]bool    // \.{@@(file@@>=} outputs: names are literal file paths

	xref *xref // identifier and section cross-references (built lazily)
}

@ |New| records the first defining section of each refinement and installs the
global and per-section format directives (later ones win).
@<Create a weaver@>=
func New(w *common.Web) *Weaver {
	wv := &Weaver{
		w:       w,
		defNum:  map[string]int{},
		format:  map[string]tokKind{},
		noIndex: map[string]bool{},
		isFile:  map[string]bool{},
	}
	@<Number the refinements and file outputs@>
	@<Install the format directives@>
	return wv
}

@ Both refinements and \.{@@(file@@>=} outputs get a defining section number, so
their headlines and links resolve (only a \.{@@(file@@>=} name is never the target
of a \.{@@<...@@>} reference). A file name is a literal path, not \TEX/, so we
remember which names are files and later typeset them verbatim.
@<Number the refinements and file outputs@>=
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

@ Format directives apply globally, then per section, with later definitions
winning: the display class of identifier |a| in \.{@@f a b} is the class |b|
would be typeset in, while \.{@@d} asks for typewriter. One right-hand side is
magic: \.{@@f a TeX} does not borrow a class but asks that |a| be set as a custom
control sequence |\a| of your own devising, exactly as \.{cweave} does --- so
after \.{\\def\\x\#1\{x\_\{\#1\}\}} the directive \.{@@f x1 TeX} makes the code
identifier |x1| come out as $x_1$. Finally, as in cweave, a name declared with
\.{\|type\|} is set bold like the predeclared types, and a constant declared in
an |iota| enumeration is set in typewriter like a \.{@@d} macro; an explicit
\.{@@f}/\.{@@s} above still wins for either.
@<Install the format directives@>=
apply := func(fs []common.Format) {
	for _, f := range fs {
		switch {
		case f.Macro:
			wv.format[f.Original] = tkMacro // \.{@d}: typewriter, like a \.{CWEB} macro
		case f.Like == "TeX":
			wv.format[f.Original] = tkTeXCS // \.{@f name TeX}: a custom control sequence
		default:
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
wv.detectDecls("type", tkBuiltin)
wv.detectIotaConsts()

@ \.{cweave} sets names declared with \.{\|type\|} in bold, like the predeclared
types, and \.{GWEB} does the same. |detectDecls| scans the code for declarations
introduced by |keyword| --- both \.{keyword NAME ...} and the block form
\.{keyword (...)} --- and records each declared name with |kind|. This is a
heuristic scan, not a full \GO/ parse, but it covers the forms that occur in
practice; a type name you want left in italic can be reset with \.{@@f NAME int},
and any name can be set in typewriter with \.{@@d}.
@<Detect type declarations@>=
func (wv *Weaver) detectDecls(keyword string, kind tokKind) {
	wv.scanAllCode(func(toks []token) {
		scanDecls(toks, keyword, func(name string) { wv.noteFormat(name, kind) })
	})
}

@ |noteFormat| files a detected name under its display |kind|, but never overrides
an explicit \.{@@f}/\.{@@s}/\.{@@d} directive --- those are installed first --- and
the blank identifier |_| names nothing.
@<Detect type declarations@>=
func (wv *Weaver) noteFormat(name string, kind tokKind) {
	if name == "" || name == "_" {
		return
	}
	if _, ok := wv.format[name]; !ok {
		wv.format[name] = kind
	}
}

@ |scanAllCode| is the traversal the detectors share: it re-lexes every code part
of every section --- a scan independent of the rendering pass --- and hands each
token list to |visit|.
@<Detect type declarations@>=
func (wv *Weaver) scanAllCode(visit func([]token)) {
	for _, s := range wv.w.Sections {
		if !s.HasCode {
			continue
		}
		var st lexState
		for _, a := range common.ScanCode(s.Code) {
			if a.Kind == common.AText {
				visit(lexGo(a.Text, &st))
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
@<Scan a declaration group@>=
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

@ |nextSignificant| skips whitespace and newlines to the next real token.
@<Scan a declaration group@>=
func nextSignificant(toks []token, i int) int {
	for ; i < len(toks); i++ {
		if toks[i].kind != tkSpace && toks[i].kind != tkNewline {
			return i
		}
	}
	return -1
}

@ |scanDeclGroup| collects the names in a parenthesized declaration group ---
each entry that starts a line at the group's own nesting level --- tracking brace
and bracket depth so that struct fields and the like are not mistaken for names.
@<Scan a declaration group@>=
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

@ A \GO/ program's manifest integer constants are written as an |iota|
enumeration: a parenthesized |const| group whose first entry seeds the counter
with |iota|, the following ones inheriting the expression as it advances.
$$\vbox{\halign{\.{#}\hfil\cr
const (\cr
\qquad tkIdent tokKind = iota\cr
\qquad tkKeyword\cr
\qquad \dots\cr
)\cr}}$$
These read like \CEE/'s |enum| members, or \.{CWEB}'s \.{@@d} macros, so \.{GWEB}
sets them in typewriter --- the same class as |nil|, |true|, and |false|.
|detectIotaConsts| registers each such name as a typewriter macro, everywhere it
is used, just as |detectDecls| registers |type| names as bold.
@<Detect iota constant declarations@>=
func (wv *Weaver) detectIotaConsts() {
	wv.scanAllCode(func(toks []token) {
		scanIotaConsts(toks, func(name string) { wv.noteFormat(name, tkMacro) })
	})
}

@ |scanIotaConsts| finds each |const (...)| group and, when it is an |iota|
enumeration, collects its declared names with the shared |scanDeclGroup|. A plain
|const| block with no |iota|, and a one-line |const|, match neither arm and are
left exactly as before; only the enumerations change.
@<Detect iota constant declarations@>=
func scanIotaConsts(toks []token, add func(string)) {
	for i := 0; i < len(toks); i++ {
		if toks[i].kind != tkKeyword || toks[i].text != "const" {
			continue
		}
		j := nextSignificant(toks, i+1)
		if j < 0 || toks[j].kind != tkOp || toks[j].text != "(" {
			continue
		}
		names := add
		if !constGroupUsesIota(toks, j+1) {
			names = func(string) {}
		}
		i = scanDeclGroup(toks, j+1, names)
	}
}

@ |constGroupUsesIota| judges a group by its first spec line alone: if |iota|
appears there, before the line ends at the group's own nesting level, the group
is an enumeration. Blank and comment lines ahead of the first spec are skipped,
and brace and bracket nesting is tracked so a composite value cannot end the line
early.
@<Detect iota constant declarations@>=
func constGroupUsesIota(toks []token, i int) bool {
	depth := 0
	seen := false // a significant token seen on the current spec line
	for ; i < len(toks); i++ {
		switch t := toks[i]; t.kind {
		case tkNewline:
			if depth == 0 && seen {
				return false
			}
		case tkSpace, tkComment:
			// not part of the spec proper
		case tkOp:
			switch t.text {
			case "(", "{", "[":
				depth++
			case ")":
				if depth == 0 {
					return false
				}
				depth--
			case "}", "]":
				if depth > 0 {
					depth--
				}
			}
			seen = true
		default:
			if t.text == "iota" {
				return true
			}
			seen = true
		}
	}
	return false
}

@ |effKind| returns the token class to typeset a token in, honoring \.{@@f}/\.{@@s}
overrides for identifiers, keywords, and builtins. A directive may be {\it
qualified\/}: \.{@@s foo.Bar int} sets only the |Bar| written as |foo.Bar|, while
the unqualified \.{@@s Bar int} sets every |Bar|. |lookupFormat| therefore tries
the qualified key |qual.name| first (|qual| is the identifier just before the
|.|, supplied by the caller) and falls back to the bare name; |noIndexed|
resolves the index-suppression flag the same way.
@<The effective token class@>=
func (wv *Weaver) effKind(t token, qual string) tokKind {
	switch t.kind {
	case tkIdent, tkKeyword, tkBuiltin:
		if k, ok := wv.lookupFormat(t.text, qual); ok {
			return k
		}
	}
	return t.kind
}

func (wv *Weaver) lookupFormat(name, qual string) (tokKind, bool) {
	if qual != "" {
		if k, ok := wv.format[qual+"."+name]; ok {
			return k, true
		}
	}
	k, ok := wv.format[name]
	return k, ok
}

func (wv *Weaver) noIndexed(name, qual string) bool {
	return (qual != "" && wv.noIndex[qual+"."+name]) || wv.noIndex[name]
}

@ |qualifierOf| gives the qualifier of the token now being typeset: the text of
the significant token two back, when the one immediately before it is a |.| ---
so in |foo.Bar| the |Bar| sees |foo|. Anything else (|x[i].Bar|, a bare name)
yields no qualifier.
@<The effective token class@>=
func qualifierOf(prevKind tokKind, prevText, prevPrevText string) string {
	if prevKind == tkOp && prevText == "." {
		return prevPrevText
	}
	return ""
}

@ |Weave| writes the whole document. It runs two passes: the first is discarded
and only fills the cross-reference tables (so a ``used in section'' note can be
printed under a definition even when the use occurs later); the second produces
the real output. \.{gweave} emits the \.{gwebmac} macro package itself, so a
\.{.w} file need not (and should not) \.{\\input} it; any stray copy is dropped
from the limbo.
@<Weave the document in two passes@>=
func (wv *Weaver) Weave(out io.Writer) error {
	wv.xref = newXref()
	scan := bufio.NewWriter(io.Discard)
	for _, sec := range wv.w.Sections {
		wv.writeSection(scan, sec)
	}

	bw := bufio.NewWriter(out)
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
@<Drop a stray gwebmac input@>=
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
commentary, and -- if present -- its code part bracketed by \.{\\GB...\\GE}, with
the definition headline and cross-reference notes for a named section.
@<Write one section@>=
func (wv *Weaver) writeSection(bw *bufio.Writer, sec *common.Section) {
	@<Write the section headline and commentary@>
	if sec.HasCode {
		@<Write the section's code part@>
	}
}

@ A starred section's title is free \TEX/ (it may contain \. typewriter and other
control sequences), so it goes through |processTex| rather than being escaped
like a refinement name; its commentary is whatever follows the title's
terminating period (matching the |common| package's title rule, so a period
inside \. does not split early). A plain section just emits its number and its
commentary.
@<Write the section headline and commentary@>=
if sec.Starred {
	fmt.Fprintf(bw, "\n\\GN{%d}{%d}{%s}", sec.Depth, sec.Number, wv.processTex(sec.Number, sec.Title))
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

@ The code part is bracketed by \.{\\GB}$\,\ldots\,$\.{\\GE}. A code-only section
(no commentary) runs in on the section-number line, as cweave does: an unnamed
section's first code line uses \.{\\GBr} (no break). A named section emits its
definition headline first, and its cross-reference notes after the code.
@<Write the section's code part@>=
runin := !sec.Starred && strings.TrimSpace(sec.Tex) == ""
@<Write a named section's definition headline@>
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

@ A named section's header is \.{\\GD}, or \.{\\GDp} for a continuation of an
earlier definition, with an \.{r} suffix when it runs in beside the number.
Emitting it also records this section as a definition of the name.
@<Write a named section's definition headline@>=
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

@ |renderCode| formats a code part into a sequence of \.{\\GL} code lines. A run
of tokens set with no space between them becomes one tight math ``chunk''
($\ldots$); where a space belongs, a breakable \.{\\GS} ends the chunk, so a long
line may fold there. Neither the horizontal spacing nor the indentation is copied
from the source: |spaceBefore| decides each gap from the grammar (the \.{Spacing
code by grammar} section) and the |indenter| decides each line's indent from the
block structure (the \.{Structural indentation} section), so even cramped, ragged
source is laid out the way |gofmt| would. Among the state variables, |prevSigKind|
and |prevSigText| track the most recent significant token --- so an identifier
following |func|/|var|/|const|/|type| can be flagged as a definition, and a \.*
after an operand told from a product --- and |prevPrevSigText| keeps the one before
that, so a qualifier like |foo| in |foo.Bar| can be recovered.
@<Render a code part@>=
func (wv *Weaver) renderCode(secNum int, code string, runin bool) string {
	var out strings.Builder
	var line strings.Builder // the current source line: chunks joined by \.{\\GS}
	var run strings.Builder  // the current tight chunk (one \TEX/ math group)
	var st lexState
	var in indenter
	indent := 0
	atLineStart := true
	pendingSpace := false
	forceDef := false     // set by \.{@@!} to force the next identifier to index as a def
	haveContent := false  // at least one code line has been emitted
	blankPending := false // a blank source line is waiting to become a \.{\\GBK} gap

	prevSigKind := tkNewline
	prevSigText := ""
	prevPrevSigText := ""
	prevUnary := false // the previous token was a unary prefix operator
	manualGap := false // a layout code has set the next gap by hand

	@<Accumulate a chunk into the current line@>
	@<Emit the current line@>
	@<End or force-break a line@>

	for _, a := range common.ScanCode(code) {
		@<Render one code atom@>
	}
	flushLine()
	return out.String()
}

@ The rendered code is built up chunk by chunk. |flushRun| closes the current
tight math chunk into the line, and |emit| adds a token's \TEX/ to the chunk,
first turning a pending grammar space (|pendingSpace|) into a breakable \.{\\GS}.
@<Accumulate a chunk into the current line@>=
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

@ |emitLine| writes the accumulated line as a \.{\\GL}, leaving the indent
intact. A blank source line between two code lines becomes a small \.{\\GBK}
gap, giving a little air between, say, the import block and the function body.
The first line of an unnamed code-only section runs in beside the section number
(\.{\\GLr}, no break); the rest are ordinary \.{\\GL} lines.
@<Emit the current line@>=
emitLine := func() {
	flushRun()
	if strings.TrimSpace(line.String()) != "" {
		if blankPending {
			out.WriteString("\\GBK\n")
			blankPending = false
		}
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

@ |flushLine| ends a source line, resetting the indent; |forceBreak| starts a
fresh woven line at the same indent (\.{@@/}), optionally preceded by a blank
line (\.{@@\#}).
@<End or force-break a line@>=
flushLine := func() {
	emitLine()
	indent = 0
	atLineStart = true
	pendingSpace = false
}
forceBreak := func(blank bool) {
	emitLine()
	if blank {
		out.WriteString("\\GBL\n")
	}
	atLineStart = false
	pendingSpace = false
}

@ Each atom of the scanned code is rendered in turn: \GO/ text is tokenized and
pretty-printed, a section reference becomes a \.{\\GX} link, verbatim and \TEX/
material passes through, and the layout hints (\.{@@,} \.{@@/} \.{@@\#} \.{@@\|})
shape the woven line.
@<Render one code atom@>=
switch a.Kind {
case common.AText:
	toks := lexGo(a.Text, &st)
	@<Render the tokens of a text atom@>
case common.ARef:
	if atLineStart {
		indent = in.beginGeneric()
	}
	name := wv.w.Resolve(a.Text)
	wv.xref.addSectionUse(name, secNum)
	emit(fmt.Sprintf("\\GX{%d}{%s}", wv.defNum[name], wv.renderName(name)))
	in.advanceGeneric()
case common.AVerbatim:
	if atLineStart {
		indent = in.beginGeneric()
	}
	emit(fmt.Sprintf("\\GST{%s}", escTT(a.Text)))
	in.advanceGeneric()
case common.ATeX:
	emit(a.Text)
case common.AIndex:
	wv.xref.addManualIndex(a.Index, a.Text, secNum)
case common.APaste:
	pendingSpace = false // join: no space before the next token
	manualGap = true     // ...and let no grammar space creep back in
case common.ALayout:
	manualGap = true // a hand-placed break or space overrides the grammar's
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
case common.AIndexDef:
	forceDef = true // @@!: the next identifier is a definition
}

@ Within a text atom, a newline flushes the woven line and closes the
|indenter|'s view of it; whitespace is ignored entirely, since both the
indentation and the inter-token spacing are now derived from the grammar rather
than copied from the source; anything else is a significant token.
@<Render the tokens of a text atom@>=
for k, t := range toks {
	switch t.kind {
	case tkNewline:
		flushLine()
		in.endLine()
	case tkSpace:
		// ignored: spacing is structural, not from the source
	default:
		@<Typeset a significant token@>
	}
}

@ A significant token first gets the space the grammar calls for --- a wide
\.{\\GS} (a chunk boundary, where the line may wrap), a \.{\\Gthin} kept within the
chunk, or nothing --- and records its index entry (a preceding declaration
keyword, or a following |:=|, marks a definition). Then it is emitted in its
effective class: a comment through |renderComment|, everything else through
|renderToken|. The line's first token takes its indentation from the |indenter|
and no leading space.
@<Typeset a significant token@>=
if atLineStart {
	indent = in.beginLine(t, toks, k)
} else if manualGap {
	manualGap = false // a hand-placed layout code already set the spacing here
} else {
	blockBrace := t.kind == tkOp && t.text == "{" &&
		in.pendingBlock && in.parenDepth == in.blockParenDepth
	switch spaceBefore(prevSigKind, prevSigText, prevUnary, t, blockBrace, in.inSquareBracket(), toks, k) {
	case gWide:
		pendingSpace = true
	case gThin:
		emit("\\Gthin ")
	}
}
qual := qualifierOf(prevSigKind, prevSigText, prevPrevSigText)
if t.kind == tkIdent || t.kind == tkBuiltin {
	def := forceDef || isDefinition(prevSigKind, prevSigText, toks, k)
	forceDef = false
	if indexable(t.text) && !wv.noIndexed(t.text, qual) {
		if def {
			wv.xref.addIdentDef(t.text, secNum)
		} else {
			wv.xref.addIdentUse(t.text, secNum)
		}
	}
}
if t.kind == tkComment {
	emit(wv.renderComment(secNum, t.text))
} else {
	emit(renderToken(token{kind: wv.effKind(t, qual), text: t.text}))
}
in.advance(t)
prevUnary = isUnaryPrefix(prevSigKind, prevSigText, t) ||
	pointerStar(prevSigKind, prevSigText, t, toks, k)
prevPrevSigText = prevSigText
prevSigKind, prevSigText = t.kind, t.text

@* Spacing code by grammar.
Like \.{cweave}, and unlike the source-driven scheme it replaces, \.{gweave}
decides the space between two code tokens from what they are, not from whether the
author happened to leave a blank between them. The rules are the math-like ones
\.{cweave} uses: a binary operator or a relation takes a space on each side, a
unary prefix operator binds tight to its operand, the brackets and the selector dot
are tight, a comma or semicolon takes a space only after it, and a keyword is
followed by a space --- with |map| the exception that runs straight into its
bracket, and |func| taking before its parenthesis the same hair space a call's
name does. Only the local context is needed: the previous significant token,
and whether it was itself unary. No parser and no precedence table is required ---
the one thing this gives up against |gofmt|, which tightens spacing around
higher-precedence operators, is that \.{gweave} spaces them all alike, as
\.{cweave} does.

@ Three widths of gap: |gTight| (no space), |gThin| (a \.{\\Gthin} kept within the
math chunk --- cweave's hair space before a call's parenthesis) and |gWide| (a
breakable \.{\\GS} that ends the chunk, where a long line may fold).
@<Space code tokens by grammar@>=
const (
	gTight = iota
	gThin
	gWide
)

@ |spaceBefore| gives the gap that precedes token |cur|, given the previous
significant token, whether that token was a unary prefix (so |cur| clings to it),
and whether a following |cur|~=~\.{\char123} opens a statement block (spaced, as in
\.{if x \char123}) rather than a composite literal (tight).
@<Space code tokens by grammar@>=
func spaceBefore(pk tokKind, pt string, pUnary bool, cur token, blockBrace, inSlice bool,
	toks []token, k int) int {
	if pUnary {
		return gTight
	}
	if cur.kind == tkOp {
		@<Return the gap before an operator@>
	}
	if pk == tkOp {
		switch pt {
		case ".", "(", "[", "{", "[]":
			return gTight
		case ":":
			if inSlice {
				return gTight // the second colon in a slice a[i:j]
			}
		}
	}
	return gWide
}

@ Most operators are handled by their text. An opening parenthesis or bracket
clings to an operand it follows (a call or an index, the parenthesis getting a hair
space as in \.{cweave}); after \.{func} its parameter list gets that same hair
space (\.{func (x int)}), unless it is a method's receiver, which |isMethodReceiver|
picks out for a full space (\.{func (r T) m()}). A lone \.{[]} takes a space after
a name (\.{x []int}) but not after another bracket (\.{[][]int}). Anything left is
a binary operator or relation, and gets a space.
@<Return the gap before an operator@>=
switch cur.text {
case ",", ";", ".", ")", "]", ":", "++", "--", "}", "{}":
	return gTight
case "{":
	if blockBrace {
		return gWide
	}
	return gTight
case "[]":
	if pk == tkOp {
		switch pt {
		case "]", "[]", "}", "[", "(", ".":
			return gTight
		}
	}
	return gWide
case "(", "[":
	if pk == tkKeyword && pt == "func" && cur.text == "(" {
		if isMethodReceiver(toks, k) {
			return gWide
		}
		return gThin // a literal's or type's ( gets the same hair space a call's does
	}
	if isOperandEnd(pk, pt) {
		if cur.text == "(" {
			return gThin
		}
		return gTight
	}
	if pk == tkKeyword && pt == "map" {
		return gTight
	}
	return gapAfter(pk, pt)
}
if isSignOp(cur.text) && !isOperandEnd(pk, pt) {
	return gapAfter(pk, pt) // a unary prefix operator
}
return gWide

@ |gapAfter| is the space a token leaves after it, consulted when |cur| itself does
not force the decision --- the leading space of a unary operator or an open bracket
simply follows whatever came before.
@<Space code tokens by grammar@>=
func gapAfter(pk tokKind, pt string) int {
	switch pk {
	case tkKeyword:
		if pt == "map" || pt == "func" {
			return gTight
		}
		return gWide
	case tkOp:
		switch pt {
		case ",", ";":
			return gWide
		case ".", "(", "[", "{", ")", "]", "}", "[]", "{}", "++", "--":
			return gTight
		}
		return gWide // a binary operator or relation
	}
	return gTight // an operand leaves no inherent space
}

@ An {\it operand end\/} is a token a value can finish with, so that a following
|*|, |&|, |-|, |+|, or |<-| is the binary form, not the unary. |isUnaryPrefix|
makes the converse judgement about the token just emitted, so the next token's gap
knows whether to cling to it.
@<Space code tokens by grammar@>=
func isOperandEnd(k tokKind, text string) bool {
	switch k {
	case tkIdent, tkNumber, tkString, tkBuiltin, tkMacro:
		return true
	case tkOp:
		switch text {
		case ")", "]", "}", "{}", "++", "--":
			return true
		}
	}
	return false
}

func isSignOp(s string) bool {
	switch s {
	case "*", "&", "-", "+", "!", "<-", "^":
		return true
	}
	return false
}

func isUnaryPrefix(pk tokKind, pt string, cur token) bool {
	if cur.kind != tkOp {
		return false
	}
	if cur.text == "!" {
		return true
	}
	return isSignOp(cur.text) && !isOperandEnd(pk, pt)
}

@ A \.* after an operand is the one genuine ambiguity: a product or a pointer type.
The programmer's own spacing settles it, and reliably --- for the two read quite
differently. A pointer type keeps a space {\it before\/} the star and runs straight
into its type after it: \.{p~*int}, \.{w~*W}. A product has either no space at all
(\.{a*b}, the form |gofmt| uses to group a higher-precedence factor) or a space on
each side (\.{a~*~b}); either way it is set spaced, the \.{cweave} way. So
|pointerStar| clings the star to its right just when the source put a blank before
it but none after. That leaves \.{*a**b} a product of two dereferences --- the
middle star is tight on its left --- so it comes out \.{*a~*~*b}, as in \.{cweave}.
This lone appeal to the source is the escape hatch \.{cweave} spells
\.{@@[}\thinspace\dots\thinspace\.{@@]}: where intent must be marked, the author's
spacing marks it.
@<Space code tokens by grammar@>=
func pointerStar(pk tokKind, pt string, cur token, toks []token, k int) bool {
	if cur.kind != tkOp || cur.text != "*" || !isOperandEnd(pk, pt) {
		return false
	}
	spaceBefore := k > 0 && toks[k-1].kind == tkSpace
	tightAfter := k+1 < len(toks) && toks[k+1].kind != tkSpace
	return spaceBefore && tightAfter
}

@ |isMethodReceiver| decides whether the parenthesis just after \.{func} opens a
method receiver rather than a function literal's parameters. A receiver is followed
by the method name and then another parenthesis --- \.{func (r T) Name(\dots)} ---
whereas a literal's parameter list is followed by a result type or a body.
@<Space code tokens by grammar@>=
func isMethodReceiver(toks []token, k int) bool {
	depth := 0
	for i := k; i < len(toks); i++ {
		if toks[i].kind != tkOp {
			continue
		}
		switch toks[i].text {
		case "(":
			depth++
		case ")":
			if depth--; depth == 0 {
				j := nextSignificant(toks, i+1)
				if j < 0 || toks[j].kind != tkIdent {
					return false
				}
				j = nextSignificant(toks, j+1)
				return j >= 0 && toks[j].kind == tkOp && toks[j].text == "("
			}
		}
	}
	return false
}

@ |renderToken| renders a single \GO/ token as a \TEX/ fragment, used inside
math. Keywords and builtins are set bold (\.{\\GKW}), identifiers italic
(\.{\\GID}). A typewriter macro --- an \.{@@d} name or a predeclared constant ---
uses \.{\\GMAC}, which wraps \.{\\tentex} in an \.{\\hbox} so it works in the
surrounding math mode; the sole exception is |nil|, \GO/'s null value, shown with
a symbol (\.{\\Gnil}, a capital lambda) as cweave shows \CEE/'s \.{NULL}. An
identifier reformatted by \.{@@f name TeX} is set as the control sequence
|\name| (see |texControlSeq|), letting you dress a plain name up as any bit of
mathematics you please. A comment is set in roman with \.{\\GCM} (escaped for
roman text mode, not the typewriter \.{\\charNN} codes, but letting $...$ math
through), its leading \.{//} tightened by a small kern (\.{\\Gcommentkern}),
whose two slashes are otherwise set rather far apart.
@<Render one token@>=
func renderToken(t token) string {
	switch t.kind {
	case tkKeyword, tkBuiltin:
		return "\\GKW{" + escIdent(t.text) + "}"
	case tkIdent:
		return "\\GID{" + escIdent(t.text) + "}"
	case tkMacro:
		if t.text == "nil" {
			return "\\Gnil "
		}
		return "\\GMAC{" + escTT(t.text) + "}"
	case tkTeXCS:
		return "\\" + texControlSeq(t.text) + " "
	case tkNumber:
		return renderNumber(t.text)
	case tkString:
		return "\\GST{" + escTT(t.text) + "}"
	case tkComment:
		if rest, ok := strings.CutPrefix(t.text, "//"); ok {
			return "\\GCM{/\\kern\\Gcommentkern/" + escComment(rest) + "}"
		}
		return "\\GCM{" + escComment(t.text) + "}"
	case tkOp:
		return renderOp(t.text)
	}
	return ""
}

@ |texControlSeq| turns an identifier into the name of the \TEX/ control sequence
that \.{@@f name TeX} conjures for it. \TEX/ control words are made of letters
only, so the two identifier characters that are not letters are transliterated,
following \.{cweave}: an underscore becomes |x| and a dollar sign becomes |X|
(thus |foo_bar| formats through \.{\\fooxbar}). A digit is left alone, which is
why \.{\\def\\x\#1\{...\}} catches the |1| of |x1| as its argument.
@<Render one token@>=
func texControlSeq(name string) string {
	var b strings.Builder
	for _, c := range []byte(name) {
		switch c {
		case '_':
			b.WriteByte('x')
		case '$':
			b.WriteByte('X')
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

@ |renderNumber| classifies a numeric literal the way cweave does. A hexadecimal
literal (|0x|\dots) is set in typewriter with a superscript \.{\#}; an octal literal
(a classic |0|\dots, or |0o|\dots) gets a small raised circle and oldstyle italic
digits; a binary literal (|0b|\dots) a superscript |b|; a decimal or floating
literal stays roman. A |_| digit separator becomes a thin space.
@<Render a numeric literal@>=
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

@ |isOctalDigits| recognizes a classic octal literal (all digits |0|--|7|, with an
optional |_| separator), and |numDigits| turns each |_| separator into a thin space.
@<Render a numeric literal@>=
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

func numDigits(s string) string {
	return strings.ReplaceAll(s, "_", "\\,")
}

@ |processTex| transforms commentary: \GO/ code inline, \.{@@<refs@@>}, \.{@@@@} to
a literal at-sign, and index entries (\.{@@\^ @@. @@:}) are recorded and removed.
Everything else -- the user's \TEX/ -- passes through unchanged.
@<Process commentary \TEX/@>=
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
		@<Set an inline code span in prose@>
		@<Handle a control code in prose@>
		b.WriteByte(c)
		i++
	}
	return b.String()
}

@ A |...| span in prose is set as the inline \GO/ code it represents, via
|renderInline|; a literal bar is written \.{\|} and handled above.
@<Set an inline code span in prose@>=
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

@ In prose, \.{@@@@} is a literal at-sign, \.{@@<...@@>} is a section reference
(set as a \.{\\GX} link and recorded as a use), and an index entry \.{@@\^},
\.{@@.}, or \.{@@:} is recorded and removed. Everything else --- the user's
\TEX/ --- falls through unchanged.
@<Handle a control code in prose@>=
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

@ |renderInline| formats a |...| inline \GO/ fragment from prose, recording its
identifiers in the index.
@<Render an inline code fragment@>=
func (wv *Weaver) renderInline(secNum int, code string) string {
	return wv.inlineCode(code, secNum, true)
}

@ |inlineCode| does the work for both |renderInline| and the section-name and
comment renderers: it sets the fragment as one math group, mirroring the source
whitespace (such fragments are not wrapped), and, when |record| is set, adds
each identifier to the index.
@<Render an inline code fragment@>=
func (wv *Weaver) inlineCode(code string, secNum int, record bool) string {
	var st lexState
	var b strings.Builder
	b.WriteString("$")
	pendingSpace := false
	started := false
	prevSigKind := tkNewline
	prevSigText := ""
	prevPrevSigText := ""
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
			qual := qualifierOf(prevSigKind, prevSigText, prevPrevSigText)
			if record && (t.kind == tkIdent || t.kind == tkBuiltin) && indexable(t.text) && !wv.noIndexed(t.text, qual) {
				wv.xref.addIdentUse(t.text, secNum)
			}
			b.WriteString(renderToken(token{kind: wv.effKind(t, qual), text: t.text}))
			started = true
			prevPrevSigText = prevSigText
			prevSigKind, prevSigText = t.kind, t.text
		}
	}
	b.WriteString("$")
	return b.String()
}

@ |renderComment| typesets a code comment. As in \.{CWEB}, the comment is \TEX/:
a |...| span inside it is set as the \GO/ code it represents (via |inlineCode|),
and everything else passes through verbatim, so ordinary \TEX/ control sequences
work -- at the cost (again as in \.{CWEB}) that the author must escape any \TEX/
specials. A literal bar is written \.{\|}. The whole thing is wrapped in \.{\\GCM},
with the leading \.{//} tightened by a small kern.
@<Render a code comment@>=
func (wv *Weaver) renderComment(secNum int, text string) string {
	prefix := ""
	body := text
	if rest, ok := strings.CutPrefix(text, "//"); ok {
		prefix = "/\\kern\\Gcommentkern/"
		body = rest
	}
	return "\\GCM{" + prefix + wv.commentBody(secNum, body) + "}"
}

@ |commentBody| walks the comment text, accumulating raw \TEX/ in |lit| and
flushing it around each recognized span. A backslash-bar \.{\|} is a literal
bar; the two multi-character spans (an opaque typewriter group and an inline
code span) are handled below; anything else is copied verbatim.
@<Render a code comment@>=
func (wv *Weaver) commentBody(secNum int, s string) string {
	var b, lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			b.WriteString(lit.String()) // raw \TEX/, as cweb treats a comment
			lit.Reset()
		}
	}
	n := len(s)
	for i := 0; i < n; {
		if s[i] == '\\' && i+1 < n && s[i+1] == '|' {
			lit.WriteByte('|') // \| is a literal bar
			i += 2
			continue
		}
		@<Pass an opaque typewriter span through@>
		@<Set an inline code span in the comment@>
		lit.WriteByte(s[i])
		i++
	}
	flush()
	return b.String()
}

@ A \.{\\.\{...\}} span is opaque: it passes through verbatim and its interior is
not scanned for |...| code spans, so a typewriter bar stays literal. An unclosed
span falls through and its backslash is copied like any other character.
@<Pass an opaque typewriter span through@>=
if s[i] == '\\' && i+2 < n && s[i+1] == '.' && s[i+2] == '{' {
	j := i + 3
	for j < n && s[j] != '}' {
		if s[j] == '\\' && j+1 < n {
			j++
		}
		j++
	}
	if j < n {
		lit.WriteString(s[i : j+1])
		i = j + 1
		continue
	}
}

@ A |...| span is set as the \GO/ code it represents, via |inlineCode|; an
unmatched bar is copied as a literal bar.
@<Set an inline code span in the comment@>=
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

@ |renderName| typesets a section name for text mode: a |...| span is set as
inline code, as in \.{CWEB} section names, and the rest is passed through verbatim as
\TEX/, so control sequences (a typewriter group, say) and math typeset, exactly as
in a starred-section title. A \.{@@(file@@>=} name is different: it is a literal
file path, not \TEX/, so it is set in typewriter with its specials escaped (an
underscore in a name like \.{squint\_test.go} would otherwise derail \TEX/).
@<Render a section name@>=
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
		@<Set an inline code span in a name@>
		start := i
		for i < n && name[i] != '|' && !(name[i] == '\\' && i+1 < n && name[i+1] == '|') {
			i++
		}
		b.WriteString(name[start:i])
	}
	return b.String()
}

@ A |...| span in a name is set as inline code (as in \.{CWEB} section names),
with cross-reference recording turned off since a name is not itself a use site.
@<Set an inline code span in a name@>=
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

@ |indexable| excludes the blank identifier from the index, and |declKeywords|
lists the keywords that introduce a declaration.
@<Index predicates and declaration keywords@>=
func indexable(name string) bool { return name != "_" }

var declKeywords = map[string]bool{
	"func": true, "var": true, "const": true, "type": true,
}

@ |isDefinition| heuristically decides whether an identifier is being declared:
it follows a |func| / |var| / |const| / |type| keyword, or it is immediately followed
by |:=|. This is best-effort -- there is no full \GO/ parse -- but it covers the
cases \.{CWEB} underlines in its index.
@<Decide whether an identifier is a definition@>=
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

@* Structural indentation.
Unlike \.{cweave}, which parses \CEE/ and lays out each construct by its grammar,
\.{gweave} once copied the woven indentation straight from the source whitespace.
That is exact for |gofmt|'d code but reproduces sloppy code just as sloppily.
Instead we derive each line's indentation from the block structure, the way
|gofmt| does: a running stack of open brackets, with the special cases a plain
brace-counter would miss --- |switch|/|select| case bodies, dedented labels and
closers, and a statement continued across a line by a trailing operator.

@ An |indentFrame| is one open bracket. |openerIndent| is the level its content is
measured from: the body sits one past it and the closing bracket returns to it. For
a composite literal or parentheses that is the physical line the bracket opened on,
but for a statement block it is the {\it statement's\/} own indentation, so the body
of a |func| whose signature wrapped across lines still lines up under the |func|,
not under the wrapped parameters. A |switch| or |select| body is the exception
|gofmt| makes: its |case|/|default| labels sit at |openerIndent|, not one deeper,
and only the statements beneath a label indent a further level.
@<Track structural indentation@>=
type indentFrame struct {
	openerIndent int  // the indentation this bracket's content is measured from
	opener       byte // which bracket opened it: a brace, paren, or square bracket
	isBlock      bool // a statement block, not a composite literal or parentheses
	isSwitch     bool // a switch/select body: its cases sit at openerIndent
	sawCase      bool // a case/default label has been seen in this body
}

@ The |indenter| carries the bracket stack and the little look-behind the special
cases need: |parenDepth| (open parentheses and brackets), a note that a
block-opening keyword (|func|, |if|, |for|, |switch|, \dots) is waiting for its
brace so the brace can be told from a composite literal's, whether a multi-line raw
string or block comment is still open (its continuation lines are verbatim, never
re-indented), and enough about the previous line to tell whether the current one
continues its statement.
@<Track structural indentation@>=
type indenter struct {
	stack           []indentFrame
	parenDepth      int
	pendingBlock    bool // a block-opening keyword awaits its brace
	pendingSwitch   bool // ...and that block is a switch/select
	blockParenDepth int  // the parenDepth at which that brace is expected
	rawOpen, cmtOpen bool
	prevContinues   bool  // the previous line ended without closing its statement
	stmtDepth       int   // bracket depth where the current statement began
	curLineIndent   int   // indentation chosen for the line now being built
	lineHadToken    bool
	lastToken       token // last significant token of the current line
}

@ |top| is the innermost open frame, or a synthetic base frame (|openerIndent|
$-1$, so its content sits at column zero) when nothing is open.
@<Track structural indentation@>=
func (in *indenter) top() indentFrame {
	if len(in.stack) == 0 {
		return indentFrame{openerIndent: -1}
	}
	return in.stack[len(in.stack)-1]
}

func (in *indenter) inSquareBracket() bool {
	n := len(in.stack)
	return n > 0 && in.stack[n-1].opener == '['
}

@ |beginLine| chooses the indentation for a line whose first significant token is
|t|. A continuation line of an open literal is emitted verbatim at column zero. A
fresh statement --- one the previous line did not continue --- resets |stmtDepth| to
the current bracket depth, so |contExtra| only fires while the statement stays at
that depth.
@<Track structural indentation@>=
func (in *indenter) beginLine(t token, toks []token, k int) int {
	if in.rawOpen || in.cmtOpen {
		in.curLineIndent = 0
		return 0
	}
	if !in.prevContinues {
		in.stmtDepth = len(in.stack)
	}
	in.curLineIndent = in.lineIndent(t, toks, k)
	return in.curLineIndent
}

@ |lineIndent| applies the layout rules in priority order: a leading closer aligns
with its opener; a bare label is pulled one level in; a |case|/|default| sits at the
switch body's level; and any other line takes its frame's content level, plus one
more if it continues the previous line's statement. |contentOf| is that content
level --- one past |openerIndent|, except that a |switch| body's own level (where a
stray comment before the first case sits) is |openerIndent| itself.
@<Track structural indentation@>=
func (in *indenter) lineIndent(t token, toks []token, k int) int {
	top := in.top()
	switch {
	case isCloser(t), isLabel(toks, k):
		return clampIndent(top.openerIndent)
	case top.isSwitch && isCaseLabel(t):
		return clampIndent(top.openerIndent)
	}
	return contentOf(top) + in.contExtra()
}

func contentOf(f indentFrame) int {
	if f.isSwitch && !f.sawCase {
		return f.openerIndent
	}
	return f.openerIndent + 1
}

@ |contExtra| grants the extra level a continued statement gets, but only while the
line is still at the bracket depth where the statement began; once a bracket has
opened, that bracket's own indentation takes over.
@<Track structural indentation@>=
func (in *indenter) contExtra() int {
	if in.prevContinues && len(in.stack) == in.stmtDepth {
		return 1
	}
	return 0
}

@ |advance| updates the stack and the look-behind after a significant token |t| is
emitted. A block-opening keyword arms |pendingBlock| so the next brace at its
parenthesis depth is recognized as a block rather than a composite literal; a
|case|/|default| marks its switch body; brackets push and pop frames; and an open
raw string or block comment is tracked so its continuation lines are left verbatim.
@<Track structural indentation@>=
func (in *indenter) advance(t token) {
	in.lineHadToken = true
	switch t.kind {
	case tkString:
		in.trackRawString(t.text)
		in.lastToken = t
		return
	case tkComment:
		in.trackBlockComment(t.text)
		return
	}
	if in.rawOpen || in.cmtOpen {
		return
	}
	@<Update the bracket stack for a token@>
	in.lastToken = t
}

@ A brace opens a block when a keyword armed |pendingBlock| at this parenthesis
depth, and a composite literal (or struct/interface type) otherwise. Only a block's
indentation follows the statement; a composite's follows its own line, so the two
push |openerIndent| from different sources. Parentheses and brackets are never
blocks. (A composite literal inside a |for \dots range| header could be mistaken for
the block, but only |for| allows one unparenthesized, and only when it spans lines
would the misread show --- a corner rare enough to leave be.)
@<Update the bracket stack for a token@>=
switch t.kind {
case tkKeyword:
	switch t.text {
	case "func", "if", "for", "else", "switch", "select", "struct", "interface":
		in.pendingBlock = true
		in.pendingSwitch = t.text == "switch" || t.text == "select"
		in.blockParenDepth = in.parenDepth
	case "case", "default":
		if n := len(in.stack); n > 0 && in.stack[n-1].isSwitch {
			in.stack[n-1].sawCase = true
		}
	}
case tkOp:
	switch t.text {
	case "{":
		if in.pendingBlock && in.parenDepth == in.blockParenDepth {
			in.stack = append(in.stack, indentFrame{opener: '{',
				openerIndent: in.blockOpenerIndent(), isBlock: true, isSwitch: in.pendingSwitch})
			in.pendingBlock = false
		} else {
			in.stack = append(in.stack, indentFrame{opener: '{', openerIndent: in.curLineIndent})
		}
	case "(", "[":
		in.parenDepth++
		in.stack = append(in.stack, indentFrame{opener: t.text[0], openerIndent: in.curLineIndent})
	case "}":
		in.popFrame()
	case ")", "]":
		in.popFrame()
		if in.parenDepth > 0 {
			in.parenDepth--
		}
	}
}

@ A block's body is indented from the enclosing block's content level when the brace
sits at statement level, but from the current line when it opens inside an open
expression --- a function literal passed as an argument, say, whose body should not
also pay for the enclosing parentheses.
@<Track structural indentation@>=
func (in *indenter) blockOpenerIndent() int {
	if n := len(in.stack); n > 0 && !in.stack[n-1].isBlock {
		return in.curLineIndent
	}
	return contentOf(in.top())
}

@ Multi-line literals are lexed one physical line at a time, so the renderer sees a
raw string or block comment as a token per line. |trackRawString| and
|trackBlockComment| follow the open/close of each so |beginLine| can leave the
continuation lines untouched.
@<Track structural indentation@>=
func (in *indenter) trackRawString(text string) {
	if in.rawOpen {
		if strings.Contains(text, "`") {
			in.rawOpen = false
		}
	} else if strings.HasPrefix(text, "`") && !strings.Contains(text[1:], "`") {
		in.rawOpen = true
	}
}

func (in *indenter) trackBlockComment(text string) {
	if in.cmtOpen {
		if strings.Contains(text, "*/") {
			in.cmtOpen = false
		}
	} else if strings.HasPrefix(text, "/*") && !strings.Contains(text, "*/") {
		in.cmtOpen = true
	}
}

func (in *indenter) popFrame() {
	if n := len(in.stack); n > 0 {
		in.stack = in.stack[:n-1]
	}
}

@ A line that carried at least one token continues its statement when that token was
an operator that cannot end one --- \GO/'s own automatic-semicolon rule. |endLine|
records the verdict for the next line. |beginGeneric| and |advanceGeneric| are the
section-reference and verbatim counterparts of |beginLine| and |advance|, treating
that material as an ordinary statement token.
@<Track structural indentation@>=
func (in *indenter) endLine() {
	if in.lineHadToken {
		in.prevContinues = continuesStmt(in.lastToken)
		if !in.prevContinues && in.parenDepth <= in.blockParenDepth {
			in.pendingBlock = false // a func type, say, that never opened a block
		}
	}
	in.lineHadToken = false
}

func (in *indenter) beginGeneric() int {
	if in.rawOpen || in.cmtOpen {
		in.curLineIndent = 0
		return 0
	}
	if !in.prevContinues {
		in.stmtDepth = len(in.stack)
	}
	in.curLineIndent = contentOf(in.top()) + in.contExtra()
	return in.curLineIndent
}

func (in *indenter) advanceGeneric() {
	in.lineHadToken = true
	in.lastToken = token{kind: tkIdent}
}

@ The remaining predicates are small enough to read at a glance. |isLabel| spots a
statement label |Name:| alone on its line --- an identifier, a colon, then the
line's end --- so it can be pulled in a level; requiring the colon to end the line
keeps a composite-literal key (|Name: value|, value and all) from being mistaken
for one. |continuesStmt| lists the operators that, at a line's end, leave its
statement open.
@<Track structural indentation@>=
func isCloser(t token) bool {
	return t.kind == tkOp && (t.text == "}" || t.text == ")" || t.text == "]")
}

func isCaseLabel(t token) bool {
	return t.kind == tkKeyword && (t.text == "case" || t.text == "default")
}

func clampIndent(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func isLabel(toks []token, k int) bool {
	if toks[k].kind != tkIdent {
		return false
	}
	j := skipSpace(toks, k+1)
	if j < 0 || toks[j].kind != tkOp || toks[j].text != ":" {
		return false
	}
	j = skipSpace(toks, j+1)
	return j < 0 || toks[j].kind == tkNewline
}

func skipSpace(toks []token, i int) int {
	for ; i < len(toks); i++ {
		if toks[i].kind != tkSpace {
			return i
		}
	}
	return -1
}

func continuesStmt(t token) bool {
	if t.kind != tkOp {
		return false
	}
	switch t.text {
	case "(", "[", "{", ")", "]", "}", "[]", "{}", ",", ":", "++", "--":
		return false
	}
	return true
}

@* A Go lexer for the woven output.
Unlike \.{go/scanner} this lexer tolerates the partial fragments found in web
sections and reports whitespace, newlines, and comments as tokens so the
pretty-printer can preserve layout. State (an open block comment or raw string)
is carried across calls because a code part may be interrupted by \.{@@<...@@>}
references.
@c
@<Token kinds@>
@<A token and the lexer state@>
@<Reserved words and predeclared names@>
@<Classify a word; character predicates@>
@<Tokenize a code fragment@>
@<Match a multi-character operator@>
@<Scan a quoted literal@>
@<Number characters and search helpers@>

@ @<Token kinds@>=
type tokKind int

const (
	tkIdent   tokKind = iota // ordinary identifier
	tkKeyword                // \GO/ reserved word
	tkBuiltin                // predeclared type or constant (also set bold)
	tkNumber                 // numeric literal
	tkString                 // \.{"..."} or \.{`...`} or \.{'...'}
	tkComment                // \.{//} or \.{/* */} text (no trailing newline)
	tkOp                     // operator or punctuation run
	tkSpace                  // a run of spaces/tabs
	tkNewline                // a single '\.{\\.\{\\\\n\}}'
	tkMacro                  // typewriter: an \.{@@d} name or a predeclared constant
	tkTeXCS                  // \.{@@f name TeX}: set as a custom control sequence
)

@ A |token| pairs a kind with its text; |lexState| carries the cross-fragment
state.
@<A token and the lexer state@>=
type token struct {
	kind tokKind
	text string
}

type lexState struct {
	inBlockComment bool
	inRawString    bool
}

@ The reserved words and the predeclared types and constants (both set bold).
@<Reserved words and predeclared names@>=
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

var goConstants = map[string]bool{"nil": true, "true": true, "false": true, "iota": true}

@ |classifyWord| maps a word to its class; the character-class predicates follow
the \GO/ spec closely enough for typesetting. The predeclared constants |nil|,
|true|, and |false| are set in typewriter rather than bold --- they are constant
values, not types, so they read like the other constants.
@<Classify a word; character predicates@>=
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
@<Tokenize a code fragment@>=
func lexGo(src string, st *lexState) []token {
	var toks []token
	n := len(src)
	i := 0
	for i < n {
		if st.inBlockComment {
			@<Resume an open block comment@>
			continue
		}
		if st.inRawString {
			@<Resume an open raw string@>
			continue
		}
		@<Scan the next token@>
	}
	return toks
}

@ An open block comment (carried across an interrupting \.{@@<...@@>}) is closed
at its \.{*/}; otherwise this physical line is emitted and the comment stays
open, so each line becomes its own woven line.
@<Resume an open block comment@>=
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

@ An open raw string is closed only if its backtick comes before any newline;
otherwise this line is emitted and the string stays open, so a multi-line raw
string becomes one woven line per physical line (a single \.{\\GST} spanning
blank lines would end the \.{\\GL}'s paragraph).
@<Resume an open raw string@>=
end := indexByte(src, '`', i)
nl := indexByte(src, '\n', i)
if end >= 0 && (nl < 0 || end < nl) {
	toks = append(toks, token{tkString, src[i : end+1]})
	st.inRawString = false
	i = end + 1
} else if nl >= 0 {
	if nl > i {
		toks = append(toks, token{tkString, src[i:nl]})
	}
	toks = append(toks, token{tkNewline, "\n"})
	i = nl + 1
} else {
	toks = append(toks, token{tkString, src[i:]})
	i = n
}

@ With no comment or string open, the next character decides the token: a
newline, a run of blanks, a line or block comment, an interpreted or raw string,
or --- the default --- a word, number, or operator.
@<Scan the next token@>=
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
	@<Scan the start of a block comment@>
case c == '"':
	i = lexQuoted(src, i, '"', &toks)
case c == '\'':
	i = lexQuoted(src, i, '\'', &toks)
case c == '`':
	@<Scan the start of a raw string@>
default:
	@<Scan a word, number, or operator@>
}

@ A \.{/*} block comment ends at its \.{*/}; if it runs off this line, the first
line is emitted and |inBlockComment| carries the rest to the next call.
@<Scan the start of a block comment@>=
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

@ A raw string may span lines; it is closed on this line only if the backtick
precedes the next newline, else this line is emitted and |inRawString| carries
the open state on, so each physical line becomes its own woven line.
@<Scan the start of a raw string@>=
end := indexByte(src, '`', i+1)
nl := indexByte(src, '\n', i+1)
if end >= 0 && (nl < 0 || end < nl) {
	toks = append(toks, token{tkString, src[i : end+1]})
	i = end + 1
} else if nl >= 0 {
	toks = append(toks, token{tkString, src[i:nl]})
	toks = append(toks, token{tkNewline, "\n"})
	st.inRawString = true
	i = nl + 1
} else {
	toks = append(toks, token{tkString, src[i:]})
	st.inRawString = true
	i = n
}

@ A word runs while identifier characters continue and is then classified; a
number runs over the number characters; anything else is an operator, matched
greedily by |matchOp| or taken as a single byte.
@<Scan a word, number, or operator@>=
switch {
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

@ The multi-character operators (longest first) and the greedy matcher that
combines them into single tokens. The empty pairs |[]| and |{}| are kept whole
so the typesetter can give them a thin space.
@<Match a multi-character operator@>=
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
@<Scan a quoted literal@>=
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

@ Number characters and two small string-search helpers. The exponent signs
|+|/|-| are intentionally excluded from a number, so that |1+2| is not swallowed
as a single token; |1e+10| then splits harmlessly instead of staying whole,
which is fine for typesetting.
@<Number characters and search helpers@>=
func isNumberPart(c byte) bool {
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

@* \TEX/ escaping.
Three contexts need different treatment: identifiers and keywords (only |_| is
troublesome); typewriter text for strings and comments (every \TEX/ special is
emitted as a |\charNN| code so it prints literally); and prose names and math
operators (text- or math-mode-safe sequences).
@c
@<Escape an identifier@>
@<Escape typewriter text@>
@<Escape a math-mode operator@>
@<Render an operator as a math atom@>
@<Set an operator's characters tight@>
@<Escape roman prose@>
@<Escape a comment, passing math through@>

@ |escIdent| escapes an identifier or keyword for text mode.
@<Escape an identifier@>=
func escIdent(s string) string {
	return strings.ReplaceAll(s, "_", "\\_")
}

@ |escTT| escapes arbitrary text for a typewriter box. Every \TEX/ special
becomes a \.{\\charNN} code so it prints literally, and a blank becomes a visible
space (\.{\\GSP}), the space glyph in slot 32 of the typewriter font, the way
\.{CWEB} prints the blanks inside a string.
@<Escape typewriter text@>=
func escTT(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\', '{', '}', '$', '&', '#', '%', '^', '_', '~':
			fmt.Fprintf(&b, "\\char%d ", c)
		case ' ':
			b.WriteString("\\GSP ")
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

@ |escMathOp| encodes an operator run so it is safe inside math mode.
@<Escape a math-mode operator@>=
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

@ |renderOp| typesets a \GO/ operator as a single tight math atom, using real math
symbols where they exist. Because inter-token spacing comes from the source, the
unary/binary distinction for |*|, |&|, and friends needs no grammar analysis.
@<Render an operator as a math atom@>=
func renderOp(s string) string {
	switch s {
	case "<=":
		return "\\mathord{\\leq}"
	case ">=":
		return "\\mathord{\\geq}"
	case "!=":
		return "\\mathord{\\neq}"
	case "==":
		return "\\mathord{\\equiv}" // equality test, as \.{CWEB} (an equivalence sign)
	case "!":
		return "\\mathord{\\lnot}" // logical not, as \.{CWEB} (a negation sign)
	case "&&":
		return "\\mathord{\\land}" // logical and, as \.{CWEB} (a wedge)
	case "||":
		return "\\mathord{\\lor}" // logical or, as \.{CWEB} (a vee)
	case "<-":
		return "\\mathord{\\leftarrow}"
	case "^":
		return "\\mathord{\\oplus}" // bitwise xor, as \.{CWEB} (a circled plus)
	case "^=":
		return "\\mathord{\\oplus}\\mathord{=}" // xor-assign: \.{\^} is a circled plus too
	case "&^":
		return "\\mathord{\\&}\\mathord{\\oplus}" // bit clear (and-not): \.{\^} as circled plus
	case "&^=":
		return "\\mathord{\\&}\\mathord{\\oplus}\\mathord{=}" // and-not-assign
	case "<<":
		return "\\mathord{\\ll}" // left shift, as \.{CWEB} (a tight double angle)
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
@<Set an operator's characters tight@>=
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
@<Escape roman prose@>=
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
so \TEX/ math works inside a comment (as in \.{CWEB}); everything outside the math is
still escaped for roman text mode.
@<Escape a comment, passing math through@>=
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
@c
@<The cross-reference tables@>
@<Build and fill the tables@>
@<Render a section list@>
@<Write the back matter@>
@<Write the PDF bookmarks@>
@<Reduce a title for the outline@>
@<Write the index@>
@<Write the list of section names@>
@<The ``used in'' note@>
@<The cross-reference notes@>

@ The tables themselves and a manual index entry.
@<The cross-reference tables@>=
type xref struct {
	identUse    map[string]map[int]bool
	identDef    map[string]map[int]bool
	sectionDefs map[string]map[int]bool
	sectionUses map[string]map[int]bool
	manualIndex []manualEntry
}

type manualEntry struct {
	kind byte // '\.{\^}', '.', ':'
	text string
	sec  int
}

@ The constructor and the small accumulator helpers.
@<Build and fill the tables@>=
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
@<Render a section list@>=
func sortedKeys(m map[int]bool) []int {
	ks := make([]int, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Ints(ks)
	return ks
}

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
sections, and the table of contents that close a woven document. The \.{\\Gdest}
destination at the top of the section-names page (targeted by the ``Names of the
sections'' bookmark) is numbered one past the last section, so it never collides
with a section's own destination.
@<Write the back matter@>=
func (wv *Weaver) writeBackMatter(bw *bufio.Writer) {
	wv.writeBookmarks(bw)
	bw.WriteString("\n\\Ginx\n")
	wv.writeIndex(bw)
	bw.WriteString("\\Gfin\n")
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
@<Write the PDF bookmarks@>=
func (wv *Weaver) writeBookmarks(bw *bufio.Writer) {
	var starred []*common.Section
	for _, s := range wv.w.Sections {
		if s.Starred {
			starred = append(starred, s)
		}
	}
	bw.WriteString("\n\\par")
	topDepth := 0
	@<Emit one bookmark per starred section@>
	@<Emit the ``Names of the sections'' bookmarks@>
}

@ Each starred section becomes a bookmark carrying its depth (for the dvipdfmx
route, which nests by level) and its number of direct children (for pdftex's
count model). We track the shallowest depth seen, for the top-level entry below.
@<Emit one bookmark per starred section@>=
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

@ A final top-level ``Names of the sections'' entry (linking to the destination
one past the last section) lists every defined section name beneath it, each
linking to its defining section, as cweave does. The negative child count starts
the group collapsed; \.{\\Goutsecname} holds the title, which the Korean backend
localizes.
@<Emit the ``Names of the sections'' bookmarks@>=
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

@ |bookmarkTitle| reduces a starred-section title to plain text safe for a PDF
outline: a |...| span keeps its inner text, \.{@@@@} becomes an at-sign, the
\TEX/-special characters are dropped, and a known text-logo control sequence is
replaced by its plain form --- as \.{CWEB}'s outline sanitizer does --- so that a
title like \.{\\TEX/ escaping} shows ``TeX escaping'' in the bookmark list, not a
stray ``/ escaping''.
@<Reduce a title for the outline@>=
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
			@<Substitute or drop a control sequence@>
		case c == '{' || c == '}' || c == '$' || c == '&' ||
			c == '#' || c == '%' || c == '^' || c == '_' || c == '~':
			// \TEX/-special: drop
		default:
			b.WriteByte(c)
		}
	}
	return strings.TrimSpace(b.String())
}

@ A control word --- a backslash and a run of letters --- that names one of the
known text logos is replaced by its plain form, swallowing the \.{/} that
terminates the (slash-delimited) macro; \.{CWEB} sanitizes its outline the same
way. Any other control word (\.{\\web} reduces to ``web'', dropping the \.{\\web}
control sequence) or a control symbol like \.{\\\&} is simply dropped.
@<Substitute or drop a control sequence@>=
if i+1 < n {
	if d := raw[i+1]; (d >= 'a' && d <= 'z') || (d >= 'A' && d <= 'Z') {
		j := i + 2
		for j < n && ((raw[j] >= 'a' && raw[j] <= 'z') || (raw[j] >= 'A' && raw[j] <= 'Z')) {
			j++
		}
		word := raw[i+1 : j]
		i = j - 1 // the outer loop's i++ steps past the last letter
		if text, ok := bookmarkLogos[word]; ok {
			b.WriteString(text)
			if i+1 < n && raw[i+1] == '/' {
				i++ // swallow the logo's slash delimiter
			}
		}
	} else {
		i++ // a control symbol: drop the backslash and the symbol
	}
}

@ The text logos \.{gweave} may meet in a title, each with the plain-text form
\.{CWEB} gives it in an outline. They are the slash-delimited macros of
\.{gwebmac.tex} (\.{\\CEE/}, \.{\\GO/}, \.{\\UNIX/}, \.{\\TEX/}), so the
terminating \.{/} is swallowed after the substitution.
@<Reduce a title for the outline@>=
var bookmarkLogos = map[string]string{
	"CEE": "C", "GO": "Go", "UNIX": "UNIX", "TEX": "TeX",
}

@ The index. Each |indexItem| collects the sections where an entry appears (the
subset in |defs| are where it is defined).
@<Write the index@>=
type indexItem struct {
	sortKey string
	render  string // typeset form of the entry head (\.{\\GID}{...}, \.{\\GIR}{...}, ...)
	secs    map[int]bool
	defs    map[int]bool
}

@ |writeIndex| accumulates every entry into |items|, keyed by its typeset head so
uses and definitions of one name merge, then orders and prints them. |get| finds
or creates the entry for a head.
@<Write the index@>=
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
	@<Collect the identifier index entries@>
	@<Collect the manual index entries@>
	@<Sort and emit the index@>
}

@ An identifier's index head follows its display class: a typewriter name (a
\.{@@d} macro or a predeclared constant) is set in typewriter, an \.{@@f name TeX}
name as its own control sequence, everything else italic. A use adds the section;
a definition also flags it in |defs|.
@<Collect the identifier index entries@>=
head := func(name string) string {
	switch wv.format[name] {
	case tkMacro:
		return "\\GMAC{" + escTT(name) + "}"
	case tkTeXCS:
		return "$\\" + texControlSeq(name) + "$" // its macro assumes math mode
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

@ A manual entry --- \.{@@.} typewriter, \.{@@:} raw \TEX/, \.{@@\^} roman --- is
rendered by its kind and recorded at the section where it appeared.
@<Collect the manual index entries@>=
for _, e := range wv.xref.manualIndex {
	var render string
	switch e.kind {
	case '.':
		render = "\\GIT{" + escTT(e.text) + "}"
	case ':':
		render = "\\GIC{" + e.text + "}"
	default: // '\.{\^}'
		render = "\\GIR{" + escProse(e.text) + "}"
	}
	it := get(render, strings.ToLower(e.text))
	it.secs[e.sec] = true
}

@ The entries are sorted by their case-folded key (ties broken by the rendered
form) and emitted as \.{\\GII} lines, each pairing the head with its section list.
@<Sort and emit the index@>=
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

@ |writeSectionNames| emits the list of named sections with their defining and
using section numbers. |sortedSectionNames| gives the shared ordering used both
here and for the PDF outline children beneath ``Names of the sections''.
@<Write the list of section names@>=
func (wv *Weaver) writeSectionNames(bw *bufio.Writer) {
	for _, n := range wv.sortedSectionNames() {
		fmt.Fprintf(bw, "\\GNS{%s}{%d}{%s}\n",
			wv.renderName(n), wv.defNum[n], usedNote(wv.xref.sectionUses[n]))
	}
}

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
@<The ``used in'' note@>=
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
@<The cross-reference notes@>=
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

@* Tests.
The weave engine's tests, one section per case. %'
@(gweave_test.go@>=
package main

import (
	"regexp"
	"strings"
	"testing"

	"github.com/sjnam/gweb/common"
)

@ @(gweave_test.go@>=
func weaveString(t *testing.T, src string) string {
	t.Helper()
	var b strings.Builder
	if err := New(common.ParseString(src)).Weave(&b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

@ @(gweave_test.go@>=
func TestWeaveHighlighting(t *testing.T) {
	out := weaveString(t, `\input gwebmac
@@* Demo.
The |main| entry.
@@c
package main

func main() {
	@@<body@@>
}

@@ Body.
@@<body@@>=
println(x)
`)
	checks := []string{
		`\GN{0}{1}{Demo}`, // starred section with title
		`$\GID{main}$`,    // inline code
		`\GKW{package}`,   // keyword bold
		`\GKW{func}`,      // keyword bold
		`\GID{main}`,      // identifier italic
		`\GX{2}{body}`,    // reference resolved to defining section 2
		`\GD{2}{body}`,    // definition headline
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("woven output missing %q\n---\n%s", c, out)
		}
	}
}

@ The back matter ends with a top-level "Names of the sections" PDF outline
entry (\.{\\Goutsecname}) linking to a destination on the section-names page
(numbered one past the last section), under which every section name is a
collapsible child linking to its defining section, as cweave does. Here the
one name "x" is defined in section 2, so the group has a single child and a
negative count (collapsed).
@(gweave_test.go@>=
func TestNamesBookmark(t *testing.T) {
	out := weaveString(t, "@@* A.\n@@c\npackage main\n@@ B.\n@@<x@@>=\n_ = 0\n")
	if !strings.Contains(out, `\Gbookmark{0}{3}{-1}{\Goutsecname}`) {
		t.Errorf("missing/!collapsed Names-of-the-sections bookmark:\n%s", out)
	}
	if !strings.Contains(out, `\Gbookmark{1}{2}{0}{x}`) {
		t.Errorf("missing section-name child bookmark:\n%s", out)
	}
	if !strings.Contains(out, `\Gdest{3}`) {
		t.Errorf("missing section-names destination:\n%s", out)
	}
}

@ @(gweave_test.go@>=
func TestWeaveEscaping(t *testing.T) {
	out := weaveString(t, `@@ x
@@c
m := map[string]int{}
s := "a\tb"
`)
	if !strings.Contains(out, `\GID{m}`) || !strings.Contains(out, `\GKW{map}`) {
		t.Errorf("expected identifier/keyword highlighting:\n%s", out)
	}
	// Braces in code must be escaped for math mode.
	if !strings.Contains(out, `\{`) || !strings.Contains(out, `\}`) {
		t.Errorf("braces not escaped:\n%s", out)
	}
}

@ A blank inside a string literal prints as a visible space (\.{\\GSP}), as cweb
does; each blank becomes its own marker.
@(gweave_test.go@>=
func TestWeaveStringVisibleSpace(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\ns := \"a b  c\"\n")
	if !strings.Contains(out, `\GST{"a\GSP b\GSP \GSP c"}`) {
		t.Errorf("string blanks should become \\GSP markers:\n%s", out)
	}
}

@ \.{nil} prints as a symbol (\.{\\Gnil}), the way cweb shows \CEE/'s \.{NULL}, not in
typewriter; the other predeclared constants stay typewriter.
@(gweave_test.go@>=
func TestWeaveNilSymbol(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nvar p *int = nil\n_ = true\n")
	if !strings.Contains(out, `\Gnil `) {
		t.Errorf("nil should render as \\Gnil:\n%s", out)
	}
	if strings.Contains(out, `\GMAC{nil}`) {
		t.Errorf("nil should not be typewriter:\n%s", out)
	}
	if !strings.Contains(out, `\GMAC{true}`) {
		t.Errorf("true should stay typewriter:\n%s", out)
	}
}

@ The leading ``\.{//}" of a comment is tightened with \.{\\Gcommentkern}.
@(gweave_test.go@>=
func TestWeaveCommentSlashKern(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nx := 1 // hi\n")
	if !strings.Contains(out, `\GCM{/\kern\Gcommentkern/ hi}`) {
		t.Errorf("comment // not kerned:\n%s", out)
	}
}

@ @(gweave_test.go@>=
func TestWeaveUnderscoreIdent(t *testing.T) {
	out := weaveString(t, `@@ x
@@c
var my_var int
`)
	if !strings.Contains(out, `\GID{my\_var}`) {
		t.Errorf("underscore not escaped in identifier:\n%s", out)
	}
}

@ @(gweave_test.go@>=
func TestWeaveIndexAndXref(t *testing.T) {
	out := weaveString(t, `@@ Program.
@@c
package main

func main() {
	x := compute()
	@@<use x@@>
}

@@ A refinement.
@@<use x@@>=
println(x)

@@ Another definition site.
@@<use x@@>=
println(x + 1)
`)
	checks := []string{
		`\GII{\GID{main}}{\GsD{1}}`, // main defined (underlined) in section 1
		`\GII{\GID{x}}{`,            // x indexed
		`\GsD{1}`,                   // x defined via := in section 1
		`\GNS{use x}`,               // named section in the list
		`\GU{`,                      // "used in" note
		`\GA{`,                      // "also defined in" note (two def sites)
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("woven output missing %q\n---\n%s", c, out)
		}
	}
}

@ @(gweave_test.go@>=
func TestWeaveOperators(t *testing.T) {
	out := weaveString(t, `@@ x
@@c
func f(ch chan int) {
	for i := 0; i != 3; i++ {
		ch <- i
	}
	if !done && i >= 1 {
	}
	switch x {
	default:
	}
}
`)
	checks := map[string]string{
		`\neq`:                     "!= should render as \\neq",
		`\geq`:                     ">= should render as \\geq",
		`\mathord{\leftarrow}`:     "<- should render as a left arrow",
		`\mathord{+}\mathord{+}`:   "++ should render tight",
		`$\GKW{if}$\GS `:           "a source space after if becomes a breakable \\GS",
		`\GKW{default}\mathord{:}`: "default: should be tight (no space before colon)",
	}
	for sub, msg := range checks {
		if !strings.Contains(out, sub) {
			t.Errorf("%s\nwant substring %q in:\n%s", msg, sub, out)
		}
	}
}

@ A name declared with `type' is a user type and renders bold (\.{\\GKW})
everywhere, like a predeclared type -- as cweave does.
@(gweave_test.go@>=
func TestWeaveTypeNamesAreBold(t *testing.T) {
	out := weaveString(t, `@@ x
@@c
type entry struct {
	frac float64
}

type (
	Graph = int
	Vertex = int
)

func use() {
	var e entry
	var g Graph
	_ = e
	_ = g
}
`)
	for _, want := range []string{`\GKW{entry}`, `\GKW{Graph}`, `\GKW{Vertex}`} {
		if !strings.Contains(out, want) {
			t.Errorf("want declared type bold %q in:\n%s", want, out)
		}
	}
	// frac is a struct field, not a type, so it stays an italic identifier.
	if strings.Contains(out, `\GKW{frac}`) {
		t.Errorf("a struct field must not be bolded as a type:\n%s", out)
	}
}

@ As in cweave, a call's ``\.{(}'' directly after a function name gets a thin
space, so it does not jam against it; a func literal's or type's \.{(} gets that
same thin space (\.{func (n int)}), while a method receiver's takes a full space
(\.{func (r T) m()}).
@(gweave_test.go@>=
func TestWeaveThinSpaceBeforeParen(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nvar _ = f(a)\nvar cdq func(l, r int)\nfunc (r *T) m() {}\n")
	checks := map[string]string{
		`\GID{f}\Gthin \mathord{(}`:    "a call f( gets a thin space",
		`\GKW{func}\Gthin \mathord{(}`: "a func literal/type func( gets the same thin space",
		`\GKW{func}$\GS $\mathord{(}`:  "a method receiver func ( gets a full space",
	}
	for sub, msg := range checks {
		if !strings.Contains(out, sub) {
			t.Errorf("%s\nwant %q in:\n%s", msg, sub, out)
		}
	}
}

@ \.{<<} and \.{>>} render as the tight double-angle symbols \.{\\ll} and
\.{\\gg} (as \.{CWEB}), not two separate less-than/greater-than signs.
@(gweave_test.go@>=
func TestWeaveShiftOperators(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nvar a = b<<2 | c>>3\n")
	for _, want := range []string{`\mathord{\ll}`, `\mathord{\gg}`} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, `\mathord{<}\mathord{<}`) {
		t.Errorf("<< should not render as two less-than signs:\n%s", out)
	}
}

@ Every operator containing \.{\^} shows it as a circled plus (\.{\\oplus}), as
\.{CWEB} does: \.{\^}, \.{\^=}, \.{\&\^} (bit clear), and \.{\&\^=}. A bare
caret must never appear. 
@(gweave_test.go@>=
func TestWeaveXorOperators(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\na = b ^ c\na ^= b\na &^= b\nd := e &^ f\n")
	for _, want := range []string{
		`\mathord{\oplus}\mathord{=}`,             // \.{\^=}
		`\mathord{\&}\mathord{\oplus}\mathord{=}`, // \.{\&\^=}
		`\mathord{\&}\mathord{\oplus}`,            // \.{\&\^}
	} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, `\char94`) {
		t.Errorf("a caret (\\char94) should never appear; ^ must be \\oplus:\n%s", out)
	}
}

@ @(gweave_test.go@>=
func TestWeaveFormatDirective(t *testing.T) {
	out := weaveString(t, `\input gwebmac
@@f Counts int
@@s hidden int
@@ x
@@c
type Counts struct{}

var c Counts
var hidden int
`)
	if !strings.Contains(out, `\GKW{Counts}`) {
		t.Errorf("@@f should typeset Counts bold like a type:\n%s", out)
	}
	if !strings.Contains(out, `\GKW{hidden}`) {
		t.Errorf("@@s should also change the typeset class:\n%s", out)
	}
	if !strings.Contains(out, `\GII{\GID{Counts}}`) {
		t.Errorf("@@f keeps the identifier in the index:\n%s", out)
	}
	if strings.Contains(out, `\GII{\GID{hidden}}`) {
		t.Errorf("@@s should omit the identifier from the index:\n%s", out)
	}
}

@ The special right-hand side |TeX| makes \.{@@f name TeX} typeset the identifier
as its own control sequence |\name|, with an underscore transliterated to |x| (so
|two_words| goes through \.{\\twoxwords}).
@(gweave_test.go@>=
func TestWeaveTeXFormat(t *testing.T) {
	out := weaveString(t, "\\input gwebmac\n@@f x1 TeX\n@@f two_words TeX\n@@ x\n@@c\nvar a = x1 + two_words\n")
	if !strings.Contains(out, `\x1 `) {
		t.Errorf("@@f x1 TeX should set x1 as the control sequence \\x1:\n%s", out)
	}
	if !strings.Contains(out, `\twoxwords `) {
		t.Errorf("@@f two_words TeX should transliterate _ to x:\n%s", out)
	}
	if strings.Contains(out, `\GID{x1}`) {
		t.Errorf("x1 should not fall back to an italic identifier:\n%s", out)
	}
}

@ A {\it qualified\/} format directive (\.{@@s foo.Bar int}) sets only the |Bar|
written as |foo.Bar|, leaving the |Bar| of |abc.Bar| (or a bare |Bar|) alone; the
unqualified \.{@@s Bar int} sets every |Bar|.
@(gweave_test.go@>=
func TestWeaveQualifiedFormat(t *testing.T) {
	q := weaveString(t, "\\input gwebmac\n@@s foo.Bar int\n@@ x\n@@c\nvar a = foo.Bar\nvar b = abc.Bar\n")
	if !strings.Contains(q, `\GID{foo}\mathord{.}\GKW{Bar}`) {
		t.Errorf("foo.Bar should typeset Bar bold:\n%s", q)
	}
	if !strings.Contains(q, `\GID{abc}\mathord{.}\GID{Bar}`) {
		t.Errorf("abc.Bar should leave Bar italic under a qualified directive:\n%s", q)
	}
	all := weaveString(t, "\\input gwebmac\n@@s Bar int\n@@ x\n@@c\nvar b = abc.Bar\n")
	if !strings.Contains(all, `\GID{abc}\mathord{.}\GKW{Bar}`) {
		t.Errorf("unqualified @@s Bar should bold every Bar:\n%s", all)
	}
}

@ Constants declared in an |iota| enumeration are set in typewriter (\.{\\GMAC}),
everywhere they are used, while a plain \.{const} block and a one-line \.{const}
stay italic (\.{\\GID}), and the |iota| line's type stays whatever it was.
@(gweave_test.go@>=
func TestWeaveIotaConst(t *testing.T) {
	out := weaveString(t, "\\input gwebmac\n@@ x\n@@c\n"+
		"const (\n\tRed Color = iota\n\tGreen\n)\n"+
		"const (\n\tPi = 3.14\n)\n"+
		"const Limit = 1\n"+
		"var _ = Red + Green\n")
	for _, name := range []string{"Red", "Green"} {
		if !strings.Contains(out, `\GMAC{`+name+`}`) {
			t.Errorf("iota constant %s should be typewriter:\n%s", name, out)
		}
	}
	for _, name := range []string{"Pi", "Limit", "Color"} {
		if !strings.Contains(out, `\GID{`+name+`}`) {
			t.Errorf("%s should stay italic:\n%s", name, out)
		}
	}
}

@ Indentation is derived from the block structure, not the source whitespace, so
this deliberately flush-left fragment --- a range loop with a nested |if| --- is
laid out the way |gofmt| would, each \.{\\GL} carrying its structural level.
@(gweave_test.go@>=
func TestWeaveStructuralIndent(t *testing.T) {
	out := weaveString(t, "@@ x\n@@<b@@>=\n"+
		"for v := range s {\nif i >= n || !yield(v) {\nreturn\n}\ni++\n}\n")
	var got []string
	for _, m := range regexp.MustCompile(`\\GL\{(\d+)\}`).FindAllStringSubmatch(out, -1) {
		got = append(got, m[1])
	}
	want := []string{"0", "1", "2", "1", "1", "0"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("structural indent levels = %v, want %v\n%s", got, want, out)
	}
}

@ A \.{switch} is the case a plain brace-counter gets wrong: |gofmt| keeps the
|case| labels at the \.{switch}'s own level and indents only the bodies, and so
does the woven output.
@(gweave_test.go@>=
func TestWeaveSwitchIndent(t *testing.T) {
	out := weaveString(t, "@@ x\n@@<b@@>=\n"+
		"switch x {\ncase 1:\nf()\ndefault:\ng()\n}\n")
	var got []string
	for _, m := range regexp.MustCompile(`\\GL\{(\d+)\}`).FindAllStringSubmatch(out, -1) {
		got = append(got, m[1])
	}
	want := []string{"0", "0", "1", "0", "1", "0"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("switch indent levels = %v, want %v\n%s", got, want, out)
	}
}

@ Spacing is derived from the grammar, math-like, not copied from the source: a
pointer type \.{*int} is tight, an index \.{xs[i]} is tight, but a product ---
even the tight \.{a*b} |gofmt| writes to group a factor --- is set spaced, as in
\.{cweave}.
@(gweave_test.go@>=
func TestWeaveGrammarSpacing(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nfunc f(p *int) {\nr := a*b + c\ns := xs[i]\n}\n")
	checks := map[string]string{
		`\mathord{*}\GKW{int}`:                  "a pointer type *int is tight",
		`\GID{xs}\mathord{[}\GID{i}\mathord{]}`: "an index xs[i] is tight",
		`\GID{a}$\GS $\mathord{*}$\GS $\GID{b}`: "a product a*b is set spaced",
	}
	for sub, msg := range checks {
		if !strings.Contains(out, sub) {
			t.Errorf("%s\nwant substring %q in:\n%s", msg, sub, out)
		}
	}
}

@ The one ambiguous operator, \.*, is disambiguated from the source's own spacing:
a pointer type (a space before, none after) clings to its type, a product
(crammed, or spaced on both sides) is set spaced. So \.{*a**b}, a product of two
dereferences, comes out \.{*a * *b}, the middle star spaced.
@(gweave_test.go@>=
func TestWeaveStarSpacing(t *testing.T) {
	out := weaveString(t, "@@ x\n@@<b@@>=\n"+
		"c := *a**b\nvar h []*T\nfunc g(p *int) {}\n")
	checks := map[string]string{
		`\mathord{*}\GID{a}$\GS $\mathord{*}$\GS $\mathord{*}\GID{b}`: "*a**b -> *a * *b",
		`\mathord{*}\GKW{int}`: "a pointer *int clings to its type",
		`\mathord{*}\GID{T}`:   "[]*T keeps *T tight",
	}
	for sub, msg := range checks {
		if !strings.Contains(out, sub) {
			t.Errorf("%s\nwant substring %q in:\n%s", msg, sub, out)
		}
	}
}

@ @(gweave_test.go@>=
func TestWeaveCodeInSectionName(t *testing.T) {
	out := weaveString(t, `@@ use
@@c
package main

var _ = @@<Compute |area| now@@>

@@ def
@@<Compute |area| now@@>=
w * h
`)
	want := `Compute $\GID{area}$ now`
	if !strings.Contains(out, `\GX{2}{`+want+`}`) {
		t.Errorf("reference name should typeset |area| as code; missing %q in:\n%s", want, out)
	}
	if !strings.Contains(out, `\GD{2}{`+want+`}`) {
		t.Errorf("definition headline should typeset |area| as code; missing %q in:\n%s", want, out)
	}
}

@ @(gweave_test.go@>=
func TestWeaveLayoutCodes(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nvar y = a@@,b\nvar z = c@@/d\nvar w = e@@|f\nvar v = g@@#h\n")
	checks := map[string]string{
		`\GID{a}\,\GID{b}`:  "@@, should insert a thin space within the chunk",
		`\GL{0}{$\GID{d}$}`: "@@/ should force a new line",
		`\GSO `:             "@@| should emit an optional break",
		`\GBL`:              "@@# should emit a blank line",
	}
	for sub, msg := range checks {
		if !strings.Contains(out, sub) {
			t.Errorf("%s\nwant substring %q in:\n%s", msg, sub, out)
		}
	}
}

@ Here |foo| is only {\it used\/} (inside a call), but \.{@@!} forces it to be
indexed as a definition, so its section number is underlined in the index.
@(gweave_test.go@>=
func TestWeaveForceDefinition(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nfunc f() { use(@@!foo) }\n")
	if !strings.Contains(out, `\GII{\GID{foo}}{\GsD{1}}`) {
		t.Errorf("@@! should index foo as a definition (underlined):\n%s", out)
	}
}

@ @(gweave_test.go@>=
func TestWeaveIndexExcludesBlankAndPluralizes(t *testing.T) {
	out := weaveString(t, `@@ def
@@<chunk@@>=
println(1)

@@ first user
@@c
package main

func f() {
	for _, x := range xs {
		@@<chunk@@>
	}
}

@@ second user
@@c
func g() { @@<chunk@@> }
`)
	if strings.Contains(out, `\GII{\GID{_}}`) {
		t.Errorf("the blank identifier _ should not be indexed:\n%s", out)
	}
	// chunk is used in two different sections (2 and 3), so the plural notes apply.
	if !strings.Contains(out, `\GUs{\Gs{2}, \Gs{3}}`) {
		t.Errorf("uses in two sections should emit \\GUs:\n%s", out)
	}
	if !strings.Contains(out, `\GNS{chunk}{1}{\GNuseds{\Gs{2}, \Gs{3}}}`) {
		t.Errorf("section-names entry malformed:\n%s", out)
	}
}

@ A section name wrapped across lines (a newline inside \.{@@<...@@>}) must match
the same name written on one line, as in \.{CWEB}. Otherwise the reference
resolves to section 0, which also crashes luatex's PDF backend.
@(gweave_test.go@>=
func TestWrappedSectionName(t *testing.T) {
	out := weaveString(t, "@@* Start.\n@@c\nfunc main() { @@<do the\nthing@@> }\n@@ @@<do the thing@@>=\nx := 1\n")
	if strings.Contains(out, `\GX{0}`) {
		t.Errorf("wrapped section name failed to resolve (got \\GX{0}):\n%s", out)
	}
	if !strings.Contains(out, `\GX{2}`) {
		t.Errorf("wrapped reference should resolve to defining section 2:\n%s", out)
	}
}

@ Code comments follow the \.{CWEB} rules: a |...| span is set as the \GO/ code it
names (not printed literally), a \.{\\.\{...\}} typewriter span passes through
verbatim rather than being escaped character by character, and an unmatched bar
stays a literal bar without turning the rest of the comment into code.
@(gweave_test.go@>=
func TestCommentInlineCode(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nx := 1 // set |x| now\n")
	if !strings.Contains(out, `\GCM{`) {
		t.Fatalf("no comment emitted:\n%s", out)
	}
	if !strings.Contains(out, `\GID{x}`) {
		t.Errorf("|x| in a comment should render as code \\GID{x}:\n%s", out)
	}
	if strings.Contains(out, "|x|") {
		t.Errorf("the bars should be consumed, not printed literally:\n%s", out)
	}
	out3 := weaveString(t, "@@ x\n@@c\nx := 1 // see \\.{foo.go}\n")
	if !strings.Contains(out3, `\.{foo.go}`) {
		t.Errorf("\\.{...} in a comment should pass through verbatim:\n%s", out3)
	}

	out2 := weaveString(t, "@@ x\n@@c\nx := 1 // a | b\n")
	if strings.Contains(out2, `\GID{b}`) {
		t.Errorf("an unmatched bar must not turn the rest into code:\n%s", out2)
	}
}

@ @(gweave_test.go@>=
func TestWeaveEmptyBrackets(t *testing.T) {
	// The empty brackets of a slice type get a thin space so they don't jam.
	out := weaveString(t, "@@ x\n@@c\nvar s []byte\n")
	if !strings.Contains(out, `\mathord{[}\,\mathord{]}`) {
		t.Errorf("slice brackets [] should get a thin space:\n%s", out)
	}
	// Indexing a[i] must stay tight (the brackets are not empty).
	out2 := weaveString(t, "@@ x\n@@c\nvar v = a[i]\n")
	if strings.Contains(out2, `\mathord{[}\,`) {
		t.Errorf("index brackets a[i] should not get a thin space:\n%s", out2)
	}
	// Empty braces (struct{}, ...) get a thin space; non-empty braces do not.
	out3 := weaveString(t, "@@ x\n@@c\ntype E struct{}\n")
	if !strings.Contains(out3, `\mathord{\{}\,\mathord{\}}`) {
		t.Errorf("empty braces {} should get a thin space:\n%s", out3)
	}
	out4 := weaveString(t, "@@ x\n@@c\nv := T{x}\n")
	if strings.Contains(out4, `\mathord{\{}\,`) {
		t.Errorf("non-empty braces should not get a thin space:\n%s", out4)
	}
}

@ Chapter one (depth 0, section 1) has two direct children: the \.{@@*1}
subsections (depth 1). \.{\\Gbookmark} is \.{\{depth\}\{secNum\}\{children\}\{title\}}.
@(gweave_test.go@>=
func TestWeaveBookmarks(t *testing.T) {
	out := weaveString(t, `@@* Chapter one. intro.
@@c
package main
@@*1 Sub A. first.
@@<a@@>=
1
@@*1 Sub B. second.
@@<b@@>=
2
@@* Chapter two. more.
@@c
var _ = 0
`)
	for _, want := range []string{
		`\Gbookmark{0}{1}{2}{Chapter one}`,
		`\Gbookmark{1}{2}{0}{Sub A}`,
		`\Gbookmark{1}{3}{0}{Sub B}`,
		`\Gbookmark{0}{4}{0}{Chapter two}`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing bookmark %q:\n%s", want, out)
		}
	}
}

@ A known text logo keeps its plain form in the outline (with the slash-delimited
macro's trailing \.{/} swallowed), so \.{\\TEX/ escaping} does not degrade to a
stray ``/ escaping''; an unknown control sequence is still dropped.
@(gweave_test.go@>=
func TestBookmarkTitle(t *testing.T) {
	cases := map[string]string{
		"The scanner":         "The scanner",
		"Update for |b| now":  "Update for b now",
		"Foo \\& Bar":         "Foo  Bar",
		"a @@@@ b":              "a @@ b",
		"\\TEX/ escaping":      "TeX escaping",
		"The \\GO/ way":        "The Go way",
		"\\CEE/ and \\UNIX/":   "C and UNIX",
		"drop \\unknown macro": "drop  macro",
	}
	for in, want := range cases {
		if got := bookmarkTitle(in); got != want {
			t.Errorf("bookmarkTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

@ @(gweave_test.go@>=
func TestWeaveInjectsGwebmac(t *testing.T) {
	// gweave supplies \.{\\input} gwebmac; the .w file need not.
	out := weaveString(t, "@@ x\n@@c\npackage main\n")
	if !strings.HasPrefix(out, "\\input gwebmac\n") {
		t.Errorf("woven output should start with \\input gwebmac, got:\n%.30q", out)
	}
	// A stray copy in the limbo is stripped, never duplicated.
	out2 := weaveString(t, "\\input gwebmac\n@@ x\n@@c\npackage main\n")
	if n := strings.Count(out2, "\\input gwebmac"); n != 1 {
		t.Errorf("want exactly one \\input gwebmac, got %d", n)
	}
}

@ A raw string spanning lines weaves as one code line per physical line, not
as a single multi-line \.{\\GST} (which would end the enclosing \.{\\GL} paragraph).
@(gweave_test.go@>=
func TestWeaveMultilineRawString(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nvar s = `a\n\nb`\n")
	if strings.Count(out, `\GL`) < 2 {
		t.Errorf("multi-line raw string should span multiple \\GL lines:\n%s", out)
	}
}
