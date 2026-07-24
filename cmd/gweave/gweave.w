@i ../../common/types.w
\def\title{GWEAVE (Version 0.9.5)}
\def\topofcontents{\null\vfill
  \centerline{\titlefont The {\ttitlefont GWEAVE} processor}
  \vskip 15pt
  \centerline{(Version 0.9.5)}
  \vfill}
\def\botofcontents{\vfill\centerline{\smallfont
  Copyright \copyright\ 2026 Soojin Nam. MIT License.}}

@** Processor gweave.
This is the command-line front end of \.{gweave}; the weave engine it drives is
defined in the second half of this web. The input may be named with or without
its \.{.w} extension (\.{gweave wc} reads \.{wc.w}, as in \.{CWEB}). The woven document
is written to the input's base name with a \.{.tex} extension; process it with a
\TEX/ engine that can find \.{gwebmac.tex} to produce a {\sc PDF}.
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
@<Detect |iota| constant declarations@>
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

	xr *xref // identifier and section cross-references (built lazily)
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
control sequence |\a| of your own devising, exactly as \.{cweave} does---so
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
			wv.format[f.Original] = tkMacro // \.{@@d}: \.{typewriter}, like a \.{CWEB} macro
		case f.Like == "TeX":
			wv.format[f.Original] = tkTeXCS // \.{@@f name TeX}: a custom control sequence
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
introduced by |keyword|---both \.{keyword NAME ...} and the block form
\.{keyword (...)}---and records each declared name with |kind|. This is a
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
an explicit \.{@@f}/\.{@@s}/\.{@@d} directive---those are installed first---and
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
of every section---a scan independent of the rendering pass---and hands each
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

@ |nextSignificant| skips whitespace and newlines to the next real token;
|prevSignificant| is its mirror, scanning back for the previous one.
@<Scan a declaration group@>=
func nextSignificant(toks []token, i int) int {
	for ; i < len(toks); i++ {
		if toks[i].kind != tkSpace && toks[i].kind != tkNewline {
			return i
		}
	}
	return -1
}
@#
func prevSignificant(toks []token, i int) int {
	for i--; i >= 0; i-- {
		if toks[i].kind != tkSpace && toks[i].kind != tkNewline {
			return i
		}
	}
	return -1
}

@ |scanDeclGroup| collects the names in a parenthesized declaration group---%
each entry that starts a line at the group's own nesting level---tracking brace
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
			// keep |atStart|
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
$$\vbox{\halign{#\hfil\cr
{\bf const}\ (\cr
\qquad \.{tkIdent} \.{tokKind} $=$ \.{iota}\cr
\qquad \.{tkKeyword}\cr
\qquad \dots\cr
)\cr}}$$
These read like \CEE/'s |enum| members, or \.{CWEB}'s \.{@@d} macros, so \.{GWEB}
sets them in typewriter---the same class as |nil|, |true|, and |false|.
|detectIotaConsts| registers each such name as a typewriter macro, everywhere it
is used, just as |detectDecls| registers |type| names as bold.
@<Detect |iota| constant declarations@>=
func (wv *Weaver) detectIotaConsts() {
	wv.scanAllCode(func(toks []token) {
		scanIotaConsts(toks, func(name string) { wv.noteFormat(name, tkMacro) })
	})
}

@ |scanIotaConsts| finds each |const (...)| group and, when it is an |iota|
enumeration, collects its declared names with the shared |scanDeclGroup|. A plain
|const| block with no |iota|, and a one-line |const|, match neither arm and are
left exactly as before; only the enumerations change.
@<Detect |iota| constant declarations@>=
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
@<Detect |iota| constant declarations@>=
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

@ A struct field's tag is a raw string, and the backquotes that delimit it are
\GO/'s syntax rather than part of what the tag says---so the woven page drops them
and sets the tag itself in the typewriter of any other string. The tangled \.{.go}
file of course keeps them; this is a matter of display alone.

No look at the enclosing |struct| is needed to know a tag when we see one. In \GO/
two operands never abut without an operator between them {\it except\/} in a
declaration, |name Type|, and a string cannot be a type---so a raw string standing
immediately after an operand end can only be the tag of the field it follows.
Everywhere else a raw string is preceded by an operator, a bracket, a comma, or a
keyword, and keeps its backquotes.
@<The effective token class@>=
func structTagged(t token, prevKind tokKind, prevText string) token {
	if t.kind == tkString && len(t.text) > 1 && isOperandEnd(prevKind, prevText) &&
		t.text[0] == '`' && t.text[len(t.text)-1] == '`' {
		t.kind = tkStructTag // both backquotes present: never a raw string cut across lines
	}
	return t
}

@ @<The effective token class@>=
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
the significant token two back, when the one immediately before it is a |.|---%
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
	wv.xr = newXref()
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
commentary, and -- if present -- its code part bracketed by \.{\\B...\\E}, with
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
	fmt.Fprintf(bw, "\n\\N{%d}{%d}{%s}", sec.Depth, sec.Number, wv.processTex(sec.Number, sec.Title))
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
	fmt.Fprintf(bw, "\n\\M{%d}", sec.Number)
	bw.WriteString(wv.processTex(sec.Number, sec.Tex))
}

@ The code part is bracketed by \.{\\B}$\,\ldots\,$\.{\\E}. A code-only section
(no commentary) runs in on the section-number line, as cweave does: an unnamed
section's first code line uses \.{\\Br} (no break). A named section emits its
definition headline first, and its cross-reference notes after the code.
@<Write the section's code part@>=
runin := !sec.Starred && strings.TrimSpace(sec.Tex) == ""
@<Write a named section's definition headline@>
runinCode := runin && sec.Name == ""
if runinCode {
	bw.WriteString("\n\\Br%\n")
} else {
	bw.WriteString("\n\\B%\n")
}
bw.WriteString(wv.renderCode(sec.Number, sec.Code, runinCode))
bw.WriteString("\\E\n")
if sec.Name != "" {
	bw.WriteString(wv.crossRefNotes(wv.w.Resolve(sec.Name), sec.Number))
}

@ A named section's header is \.{\\D}, or \.{\\Dp} for a continuation of an
earlier definition, with an \.{r} suffix when it runs in beside the number.
Emitting it also records this section as a definition of the name.
@<Write a named section's definition headline@>=
if sec.Name != "" {
	name := wv.w.Resolve(sec.Name)
	cont := wv.defNum[name] != sec.Number
	wv.xr.addSectionDef(name, sec.Number)
	macro := "\\D"
	if cont {
		macro = "\\Dp" // continuation of an earlier definition
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
from the source: each token is sorted into a spacing category and |gapBetween|
reads the gap off the two neighbouring categories (the \.{Spacing code by grammar}
section), while the |indenter| decides each line's indent from the block structure
(the {\bf Structural indentation} section), so even cramped, ragged source is laid
out the way |gofmt| would. Among the state variables, |prevCat| carries the
previous token's category forward for the next gap; |prevSigKind| and |prevSigText|
track the most recent significant token---so an identifier following
|func|/|var|/|const|/|type| can be flagged as a definition, and a \.* after an
operand told from a product---and |prevPrevSigText| keeps the one before that, so a
qualifier like |foo| in |foo.Bar| can be recovered.
@<Render a code part@>=
func (wv *Weaver) renderCode(secNum int, code string, runin bool) string {
	var out strings.Builder
	var line strings.Builder // the current source line: chunks joined by \.{\\GS}
	var run strings.Builder  // the current tight chunk (one \TEX/ math group)
	var st lexState
	var in indenter
	indent := 0
	atLineStart := true
	pendingGap := gTight  // width of the space owed before the next token (gTight = none)
	forceDef := false     // set by \.{@@!} to force the next identifier to index as a def
	haveContent := false  // at least one code line has been emitted
	blankPending := false // a blank source line is waiting to become a \.{\\BK} gap

	prevSigKind := tkNewline
	prevSigText := ""
	prevPrevSigText := ""
	prevCat := catKeyword // the previous token's spacing category (any value; unused at a line start)
	manualGap := false    // a layout code has set the next gap by hand

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
first turning a pending grammar space (|pendingGap|) into the breakable code
space its width calls for: an ordinary \.{\\GS}, or the wider \.{\\BS} that sets
a statement block's braces apart.
@<Accumulate a chunk into the current line@>=
flushRun := func() {
	if run.Len() > 0 {
		line.WriteString("$")
		line.WriteString(run.String())
		line.WriteString("$")
		run.Reset()
	}
}
@#
emit := func(s string) {
	if pendingGap != gTight {
		flushRun()
		switch pendingGap {
		case gBlock:
			line.WriteString("\\BS ")
		case gWord:
			line.WriteString("\\W ")
		case gRel:
			line.WriteString("\\rel ")
		case gPunct:
			line.WriteString("\\punct ")
		default:
			line.WriteString("\\GS ")
		}
		pendingGap = gTight
	}
	run.WriteString(s)
	atLineStart = false
}

@ |emitLine| writes the accumulated line as a \.{\\GL}, leaving the indent
intact. A blank source line between two code lines becomes a small \.{\\BK}
gap, giving a little air between, say, the import block and the function body.
The first line of an unnamed code-only section runs in beside the section number
(\.{\\Lr}, no break); the rest are ordinary \.{\\GL} lines.
@<Emit the current line@>=
emitLine := func() {
	flushRun()
	if strings.TrimSpace(line.String()) != "" {
		if blankPending {
			out.WriteString("\\BK\n")
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

@ |flushLine| ends a source line, resetting the indent; |forceBreak| (\.{@@/}, or
\.{@@\#} with a blank line first) ends the current woven line in the middle of a
source line. It closes the |indenter|'s view of the line and leaves us at a line
start, so the continuation's indentation is recomputed for the current nesting---%
a break inside an open bracket steps its continuation in, rather than hanging at
the statement's own margin.
@<End or force-break a line@>=
flushLine := func() {
	emitLine()
	indent = 0
	atLineStart = true
	pendingGap = gTight
}
@#
forceBreak := func(blank bool) {
	emitLine()
	if blank {
		out.WriteString("\\BL\n")
	}
	in.endLine()
	atLineStart = true
	pendingGap = gTight
	manualGap = false
}

@ Each atom of the scanned code is rendered in turn: \GO/ text is tokenized and
pretty-printed, a section reference becomes a \.{\\X} link, verbatim and \TEX/
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
	wv.xr.addSectionUse(name, secNum)
	emit(fmt.Sprintf("\\X{%d}{%s}", wv.defNum[name], wv.renderName(name)))
	in.advanceGeneric()
case common.AVerbatim:
	if atLineStart {
		indent = in.beginGeneric()
	}
	emit(fmt.Sprintf("\\ST{%s}", escTT(a.Text)))
	in.advanceGeneric()
case common.ATeX:
	emit("\\hbox{" + a.Text + "}") // \.{@@t}: a \TEX/ box set amid the code, as in cweave
case common.AIndex:
	wv.xr.addManualIndex(a.Index, a.Text, secNum)
case common.APaste:
	pendingGap = gTight // join: no space before the next token
	manualGap = true    // ...and let no grammar space creep back in
case common.ALayout:
	switch a.Index {
	case ',': // an explicit thin space, added on top of the grammar's own
		emit("\\,")
	case '/': // force a line break, re-indenting the continuation
		forceBreak(false)
	case '#': // force a line break, preceded by a blank line
		forceBreak(true)
	case '|': // optional (zero-width) line break between chunks
		manualGap = true // this hand-placed break overrides the grammar's space
		flushRun()
		line.WriteString("\\SO ")
		pendingGap = gTight
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

@ Typesetting a significant token runs four small phases in order: it takes the
space the grammar asks for, records its index entry, is emitted in its effective
class, and then rolls itself forward as the look-behind the next token will
consult.
@<Typeset a significant token@>=
@<Space the significant token@>
@<Record the token's index entry@>
@<Emit the token@>
@<Advance the look-behind@>

@ The gap is one of the breakable code spaces (a chunk boundary, where the woven
line may wrap) or nothing at all. The first token of a line is the exception twice
over: it takes its indentation from the |indenter| and gets no leading space; and a
hand-placed layout code may already have fixed the gap, in which case we leave it
alone.
@<Space the significant token@>=
blockBrace := t.kind == tkOp && t.text == "{" && in.opensBlock()
curCat := classify(t, prevSigKind, prevSigText, toks, k,
	blockBrace, in.top().isBlock, in.inSquareBracket())
if atLineStart {
	indent = in.beginLine(t, toks, k)
} else if manualGap {
	manualGap = false // a hand-placed layout code already set the spacing here
} else {
	switch g := gapBetween(prevCat, curCat); g {
	case gPunct, gWide, gRel, gWord, gBlock:
		pendingGap = g
	}
}

@ An identifier or builtin earns an index entry under the current section---a
definition when a preceding declaration keyword or a following |:=| marks it as
one, a use otherwise. The |qualifierOf| result is computed here because it settles
both what to index and, in a moment, the token's effective display class.
@<Record the token's index entry@>=
qual := qualifierOf(prevSigKind, prevSigText, prevPrevSigText)
if t.kind == tkIdent || t.kind == tkBuiltin {
	def := forceDef || isDefinition(prevSigKind, prevSigText, toks, k)
	forceDef = false
	if indexable(t.text) && !wv.noIndexed(t.text, qual) {
		if def {
			wv.xr.addIdentDef(t.text, secNum)
		} else {
			wv.xr.addIdentUse(t.text, secNum)
		}
	}
}

@ A comment goes through |renderComment|, everything else through |renderToken| in
its effective class---which |structTagged| may refine once more, since only the
look-behind kept here can tell a field's tag from any other raw string. A trailing
comment is the one exception to the grammar's spacing: whatever the code before it
was---even a \.] or a selector dot, which cling to what follows them---it is set
off by the generous \.{\\CS} gap \.{cweave} leaves before a comment, in place of
the ordinary \.{\\GS}. Only a comment that opens its own line takes no \.{\\CS};
there the indent already stands in for it.
@<Emit the token@>=
if t.kind == tkComment {
	if !atLineStart { // a trailing comment is always set off from the code, as cweave does
		flushRun()
		line.WriteString("\\CS ")
	}
	pendingGap = gTight // never let a grammar gap glue in front of the comment
	emit(wv.renderComment(secNum, t.text))
} else {
	emit(renderToken(structTagged(token{kind: wv.effKind(t, qual), text: t.text}, prevSigKind, prevSigText)))
}

@ With the token set, it becomes the past: |advance| updates the |indenter|, and
the previous-token fields---including this token's spacing category, which the next
token's gap looks back on---roll forward.
@<Advance the look-behind@>=
in.advance(t)
prevCat = curCat
prevPrevSigText = prevSigText
prevSigKind, prevSigText = t.kind, t.text

@* Spacing code by grammar.
Like \.{cweave}, and unlike the source-driven scheme it replaces, \.{gweave}
decides the space between two code tokens from what they are, not from whether the
author happened to leave a blank between them. It does this in two steps, and the
whole rule set lives in those two steps rather than being scattered across a pile of
special cases. First, |classify| maps each token to one of two dozen {\it spacing
categories\/}---the grammatical role that governs the gaps around it---resolving
there the handful of ambiguities a token alone cannot settle: a statement block's
brace from a composite literal's, a call's parenthesis from a grouping one, a
pointer star from a product. Then |gapBetween| reads the gap straight off the two
neighbouring categories.

@ This is \.{cweave}'s own model, pared down. \.{cweave} carries each scrap through
a bottom-up grammar of some forty categories and a hundred productions, each
production emitting the spacing for the structure it recognizes; a category here is
one of those scrap categories, named in the comments beside |spaceCat|. The
difference is that |gofmt| has already normalized the layout before \.{gweave} sees
it, so where \.{cweave} must {\it parse\/} C to know a scrap's category, \.{gweave}
merely {\it classifies\/} an already-tidy token---no grammar, no precedence table.
Like \.{cweave}, \.{gweave} sets a comma, an arithmetic operator, and a relation
with three different widths---\.{cweave}'s thin, medium, and thick math muskips. The
one thing it still gives up against |gofmt|, which tightens spacing around
higher-precedence operators, is that within arithmetic it spaces them all alike:
\.{a*b} sets like \.{a + b}, not tighter.

@ The categories and the table read, in brief:
$$\vbox{\halign{\.{#}\hfil\quad&#\hfil\cr
&{\rm gap before this category}\cr
\noalign{\smallskip}
arithmetic op&a medium space on each side (|a + b|)\cr
relation, assignment&a wider thick space on each side (|a == b|, |x = y|)\cr
two adjacent words&a wider text space (|var foo Type|, |n int|), as \.{cweave}\cr
a name and its type&the same word space, whatever the type opens with (|c *Node|, |p []byte|)\cr
unary prefix&clings to its operand (|*p|, |-1|, |!done|)\cr
call \.( or empty \.{()}&tight (|f(x)|), as \.{cweave} sets a call\cr
receiver \.(&a full space (|func (r T)|)\cr
index \.[, selector \..&tight (|a[i]|, |x.f|)\cr
comma&tight before, a thin space after (|a, b|)\cr
header semicolon&tight before, a full clause space after, as \.{cweave} breaks a \.{for} header\cr
\.{if}, \.{for}, \.{switch}, \.{select}&a structural space before the clause, as before the brace\cr
block brace \.{\char123}\thinspace\.{\char125}&a wider structural space, both sides\cr
literal brace \.{\char123}\thinspace\.{\char125}&tight against its type (|T{a}|); as an element, spaced like one\cr
}}$$
Every other pair falls to the default: a full space between words, and the ``space
a token leaves after it'' for whatever follows an open bracket or a unary sign.

@ Six widths of gap, in increasing order. |gTight| (no space, as before a call's
parenthesis, \.{cweave}'s tight |f(x)|); then \.{cweave}'s three math muskips, each
a breakable chunk boundary: |gPunct| (a \.{\\punct}, the thinmuskip after a comma),
|gWide| (a
\.{\\GS}, the medmuskip around an arithmetic operator), |gRel| (a \.{\\rel}, the
thickmuskip around a relation or assignment); |gWord| (a wider \.{\\W} between two
words---cweave's text interword space, as in \.{int foo}); and |gBlock| (a wider
\.{\\BS} that sets a statement block's braces off from the block's head and body,
and a \.{for} or \.{if} header's semicolon off from the next clause---where
\.{cweave} breaks the math and leaves a half-em, its punctuation thin space plus a
text interword space).
The three operator widths are why \.{a,\ b}, \.{a+b}, and \.{a==b} set with visibly
different spaces, exactly as \.{cweave} does.
@<Space code tokens by grammar@>=
const (
	gTight = iota
	gPunct
	gWide
	gRel
	gWord
	gBlock
)

@ Every token is first mapped to a |spaceCat|, its {\it spacing category\/}: the
grammatical role that decides the gaps around it. The mapping is where the few
context-dependent ambiguities are resolved, once---a statement block's brace from a
composite literal's, a call's parenthesis from a grouping one, a pointer star from
a product---so that the gap table that follows is a pure function of two
categories. Each category corresponds to a \.{cweave} scrap category, noted in the
comment; the difference is that |gofmt| has already fixed the layout, so |gweave|
classifies a token where \.{cweave} must parse to reach the same scrap.
@<Space code tokens by grammar@>=
type spaceCat int

const (
	catExpr       spaceCat = iota // an operand: identifier, number, string, builtin, macro (cweave |exp|)
	catClose                      // \.) a literal's brace, \.{\char123\char125} \.{++} \.{--}: an operand end (|exp|)
	catCloseBracket               // \.] : an operand end that a following \.{[]} clings to, as \.{[3][]int} (|rpar|)
	catBlockClose                 // a statement block's closing brace (|rbrace|)
	catEmptyParen                 // \.{()} (|exp|)
	catLoneBrackets               // \.{[]} (|lpar| |rpar|)
	catComma                      // \., (|comma|)
	catSemi                       // \.; a \.{for} or \.{if} header's clause separator (|semi|)
	catDot                        // \.. a selector (part of |exp|)
	catSliceColon                 // the \.: of a slice \.{a[i:j]} (|colon|)
	catColon                      // the \.: of a label, case, or map key (|colon|)
	catBlockOpen                  // a statement block's opening brace (|lbrace|)
	catLitOpen                    // a composite literal's opening brace (|lbrace|)
	catCallParen                  // the \.( of a call, or a func type's or literal's parameters (|lpar|)
	catRecvParen                  // the \.( of a method receiver (|lpar|)
	catIndex                      // the \.[ of an index (|lpar|)
	catArrayType                  // the \.[ of an array type after a spaced name (|lpar|)
	catMapBracket                 // the \.[ following |map| (|lpar|)
	catOpen                       // a \.( or \.[ opening a type or a grouping, otherwise (|lpar|)
	catUnary                      // a unary prefix \.\& \.- \.+ \.! \.{<-} \.\^ or a unary \.* (|unop|)
	catPtrStar                    // a pointer \.* crammed against an array type (|ubinop|)
	catSpacedPtr                  // a pointer \.* with a blank before it, \.{p *int} (|ubinop|)
	catBinop                      // an arithmetic, bitwise, or logical binary operator, or a product \.* (|binop|)
	catRel                        // an operator cweave sets thick: a relation, an assignment, \.{\char124}, \.{<<}, \.{>>}
	catOrdOp                      // \./ : the one binary operator cweave sets tight, as an ordinary atom
	catFunc                       // the keyword |func|
	catMap                        // the keyword |map|
	catStmtKw                     // a block-heading statement keyword: |if|, |for|, |switch|, |select|
	catTypeKw                     // |struct| or |interface|: a composite-type body opener (|struct_like|)
	catKeyword                    // any other reserved word (|int_like|, |else_like|, \dots)
)

@ |classify| resolves a token to its category, consulting the previous significant
token and the |indenter|'s state exactly where a role is ambiguous: whether a brace
opens a block (|blockBrace|) or closes one (|inBlock|), and whether a colon sits in
a slice (|inSlice|). An ordinary word settles first; an operator hands off to the
sections below.
@<Space code tokens by grammar@>=
func classify(cur token, pk tokKind, pt string, toks []token, k int,
	blockBrace, inBlock, inSlice bool) spaceCat {
	if cur.kind != tkOp {
		switch {
		case cur.kind != tkKeyword:
			return catExpr
		case cur.text == "func":
			return catFunc
		case cur.text == "map":
			return catMap
		case cur.text == "if" || cur.text == "for" || cur.text == "switch" || cur.text == "select":
			return catStmtKw // a block head, set off from its clause like the block's brace
		case cur.text == "struct" || cur.text == "interface":
			return catTypeKw // a composite-type body; its brace takes a word space, not a block space
		}
		return catKeyword
	}
	@<Classify an operator token@>
}

@ The brackets and punctuation settle from the token alone (a brace also needs to
know whether it opens or closes a block, a colon whether it is a slice's); an open
paren or bracket needs the role calls in the next two sections; anything left is a
sign or a binary operator.
@<Classify an operator token@>=
switch cur.text {
case ",":
	return catComma
case ";":
	return catSemi
case ".":
	return catDot
case ")", "{}", "++", "--":
	return catClose
case "]":
	return catCloseBracket
case "}":
	if inBlock {
		return catBlockClose
	}
	return catClose
case "()":
	return catEmptyParen
case "[]":
	return catLoneBrackets
case ":":
	if inSlice {
		return catSliceColon
	}
	return catColon
case "{":
	if blockBrace {
		return catBlockOpen
	}
	return catLitOpen
case "...":
	if isOperandEnd(pk, pt) && spreadEllipsis(toks, k) {
		return catClose // a spread, \.{f(args...)}: clings to its operand like a postfix
	}
case "(":
	@<Classify an open parenthesis@>
case "[":
	@<Classify an open bracket@>
}
@<Classify a sign or a binary operator@>

@ An open parenthesis is a call's when it follows an operand, a receiver's when it
follows |func| and |isMethodReceiver| says so, a func type's or literal's (the same
hair space as a call's) after any other |func|, and a plain grouping otherwise.
@<Classify an open parenthesis@>=
if pk == tkKeyword && pt == "func" {
	if isMethodReceiver(toks, k) {
		return catRecvParen
	}
	return catCallParen
}
if isOperandEnd(pk, pt) {
	return catCallParen
}
return catOpen

@ An open bracket is |map|'s after the keyword, an index when crammed against an
operand, an array type when the source keeps a blank between the name and it
(\.{b [256]int}), and a plain type/grouping opener otherwise.
@<Classify an open bracket@>=
if pk == tkKeyword && pt == "map" {
	return catMapBracket
}
if isOperandEnd(pk, pt) {
	if k > 0 && toks[k-1].kind == tkSpace {
		return catArrayType
	}
	return catIndex
}
return catOpen

@ A sign operator is unary when no operand precedes it; a \.* after an operand is a
pointer---crammed against an array type, or spaced by the source---rather than a
product. |starAfterArrayType| and |pointerStar| are the same judgements the old
scheme made, now feeding one classification instead of the gap directly.
@<Classify a sign or a binary operator@>=
if isSignOp(cur.text) && !isOperandEnd(pk, pt) {
	return catUnary
}
if cur.text == "*" && isOperandEnd(pk, pt) {
	if starAfterArrayType(toks, k) {
		return catPtrStar
	}
	if pointerStar(pk, pt, cur, toks, k) {
		return catSpacedPtr
	}
}
if isRelOp(cur.text) {
	return catRel // cweave's thick relation space: comparisons, assignments, \.{\char124}, shifts
}
if cur.text == "/" {
	return catOrdOp // cweave sets division tight, an ordinary atom
}
return catBinop

@ |gapBetween| is the whole spacing rule in one table: the gap that goes between a
token of category |left| and the following token of category |right|. A handful of
rules are keyed on |left|---an operand clings to a preceding unary or pointer
operator or to \./, and a header's semicolon breaks its clause whatever follows;
otherwise the gap is read off |right|, with the few cases that still look back at
|left| spelled out in the section below.
@<Space code tokens by grammar@>=
func gapBetween(left, right spaceCat) int {
	switch left {
	case catUnary, catPtrStar, catSpacedPtr:
		return gTight // the operand clings to a unary or pointer operator
	case catOrdOp:
		return gTight // \./ is an ordinary atom: its operand clings on the right too
	case catSemi:
		return gBlock // a \.{for} or \.{if} header's semicolon breaks its clause, whatever follows
	}
	@<Read the gap off the right-hand category@>
}

@ Most categories fix the gap outright. A lone \.{[]} clings to a preceding
bracket, brace, or selector dot but takes a space after a name; a block brace
breathes, except a composite type's body brace, which \.{cweave} sets off its
|struct| or |interface| with a plain word space, not the structural \.{\\5}; an
open bracket or a unary sign, whose leading gap follows whatever came before,
defers to |gapAfterCat|; an ordinary word defers to |afterNonOp|.
@<Read the gap off the right-hand category@>=
switch right {
case catComma, catSemi, catDot, catColon, catSliceColon, catClose, catCloseBracket, catIndex,
	catMapBracket, catPtrStar, catOrdOp, catCallParen, catEmptyParen:
	return gTight // a call's \.( clings to its name (|f(x)|), tight as \.{cweave} sets it
case catLitOpen:
	return gapBeforeLit(left)
case catBlockOpen:
	if left == catTypeKw {
		return gWord // \.{struct \char123}, \.{interface \char123}: a word space, as in \.{cweave}
	}
	return gBlock // a statement block's opening brace breathes, as in \.{cweave}
case catBlockClose:
	return gBlock // a statement block's closing brace breathes, as in \.{cweave}
case catRecvParen, catBinop:
	return gWide
case catRel:
	return gRel // a relation or assignment gets cweave's wider thick space
case catArrayType, catSpacedPtr:
	if left == catExpr || left == catClose || left == catEmptyParen {
		return gWord // a name or a func result and its array/pointer type: x [3]int, func() [3]int
	}
	return gWide
case catLoneBrackets:
	return gapBeforeLone(left)
case catUnary, catOpen:
	return gapAfterCat(left)
}
return afterNonOp(left) // a word: identifier, number, string, or keyword

@ |gapBeforeLone| gives the gap before a \.{[]}: tight after any bracket, brace, or
selector dot (\.{[][]int}, \.{a[i][]}); the word space after a declared name, or a
function's parameter list, and its slice type (\.{p []byte}, \.{func() []V}), to
match the other name-and-type gaps; a plain space after anything else.
@<Space code tokens by grammar@>=
func gapBeforeLone(left spaceCat) int {
	switch left {
	case catCloseBracket, catBlockClose, catLoneBrackets, catCallParen, catRecvParen,
		catIndex, catArrayType, catMapBracket, catOpen, catDot:
		return gTight
	case catExpr, catClose, catEmptyParen:
		return gWord // a name or a func result and its slice type: p []byte, func() []V
	}
	return gWide
}

@ |gapBeforeLit| gives the gap before a composite literal's \.\{. Usually none: the
brace belongs to the type it follows, and \.{P\char123 4, 5\char125} or
\.{[\,]int\char123 \char125} must read as one thing. But a literal's braces can also
open an {\it element\/} of an enclosing literal, where the type is elided --- and
there the brace is just another element, so it takes whatever gap its neighbor
leaves, exactly as a name in the same slot would: \.{\char123 1, 2\char125, \char123
3, 4\char125} spaces like \.{a, b}, and a keyed \.{23: \char123 2, 3\char125} like
\.{45: bar}.
@<Space code tokens by grammar@>=
func gapBeforeLit(left spaceCat) int {
	switch left {
	case catComma:
		return gPunct // an element after a comma, as any other element
	case catColon:
		return gWide // a keyed element's value, as any other value
	}
	return gTight // the brace clings to its type
}

@ |gapAfterCat| is the space a token leaves after it---\.{cweave}'s notion, used
when the following token is an open bracket or a unary sign. The brackets, the
selector dot, and the increment operators leave none; |map| and |func| run straight
into what follows and an operand leaves no inherent space; a block-heading keyword
sets off its clause with the same structural space its brace gets; a keyword, a
comma, a colon, or a binary operator leaves a plain space.
@<Space code tokens by grammar@>=
func gapAfterCat(left spaceCat) int {
	switch left {
	case catDot, catCallParen, catRecvParen, catOpen, catIndex, catArrayType,
		catMapBracket, catLitOpen, catBlockOpen, catClose, catCloseBracket, catBlockClose,
		catLoneBrackets, catEmptyParen, catFunc, catMap, catExpr:
		return gTight
	case catStmtKw:
		return gBlock
	case catComma:
		return gPunct // a comma leaves cweave's narrow thin space
	case catRel:
		return gRel // a relation or assignment leaves cweave's wide thick space
	}
	return gWide
}

@ |afterNonOp| gives the gap before an ordinary word: tight after a selector dot,
an open bracket or paren, a lone \.{[]}, a close bracket, a composite literal's
brace, or a slice colon; a full block space after a statement block's brace or a
block-heading keyword (so \.{if x \char123} reads evenly on both sides of the
clause); the wider word space between two words, \.{cweave}'s text interword space
(\.{var foo Type}, \.{func bar}, \.{n int}); the thin space after a comma, the thick
space after a relation, and the plain medmuskip after any other operator or a colon.
A close bracket clings to a following element type so that an array or map type sets
like its slice: \.{[256]int} and \.{map[K]V} match \.{[]int}.
@<Space code tokens by grammar@>=
func afterNonOp(left spaceCat) int {
	switch left {
	case catDot, catCallParen, catRecvParen, catOpen, catIndex, catArrayType,
		catMapBracket, catLoneBrackets, catCloseBracket, catLitOpen, catSliceColon:
		return gTight
	case catBlockOpen, catStmtKw:
		return gBlock
	case catExpr, catKeyword, catFunc, catMap, catTypeKw, catClose, catEmptyParen:
		return gWord // two adjacent words, or a func result and its type: \.{n int}, \.{func() T}
	case catComma:
		return gPunct // a comma leaves cweave's narrow thin space
	case catRel:
		return gRel // a relation or assignment leaves cweave's wide thick space
	}
	return gWide
}

@ An {\it operand end\/} is a token a value can finish with, so that a following
|*|, |&|, |-|, |+|, or |<-| is the binary form, not the unary---|classify| makes
that call to tell a |catUnary| from a |catBinop|. A closed |()| ends an operand
just as |)| would, so a chained call or index after an empty call --- |f()(x)|,
|f()[0]| --- stays tight.
@<Space code tokens by grammar@>=
func isOperandEnd(k tokKind, text string) bool {
	switch k {
	case tkIdent, tkNumber, tkString, tkBuiltin, tkMacro:
		return true
	case tkOp:
		switch text {
		case ")", "]", "}", "{}", "()", "++", "--":
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

@ A \.{...} is a {\it spread\/} when it ends an operand inside a call---the next
significant token is a closer or a comma, as in \.{f(args...)} or \.{f(a, b...)}. It
then clings to its operand like the postfix \.{++}, rather than taking the space a
binary operator would. A variadic {\it parameter\/} \.{...int}, whose \.{...} is
followed by a type, is not a spread and keeps its ordinary spacing.
@<Space code tokens by grammar@>=
func spreadEllipsis(toks []token, k int) bool {
	for i := k + 1; i < len(toks); i++ {
		if toks[i].kind == tkSpace {
			continue
		}
		switch toks[i].text {
		case ")", ",", "]", "}":
			return true
		}
		return false
	}
	return false
}

@ These are the operators \.{cweave} sets with the wide thickmuskip: it makes each a
\TEX/ |\mathrel|, where \.+ or \.* is a |\mathbin| with the narrower medmuskip. Most
are what one expects---the comparisons and the assignments---but \.{cweave} also
draws bitwise-or as \.{\char124} (|\mid|) and the shifts as $\ll$ and $\gg$, all
three relations, so they take the thick space too. |gweave| mirrors the whole set,
so \.{a == b}, \.{a\char124b}, and \.{a<<b} breathe wider than \.{a + b}, exactly as
in \.{cweave}.
@<Space code tokens by grammar@>=
func isRelOp(s string) bool {
	switch s {
	case "==", "!=", "<", ">", "<=", ">=", "=", ":=",
		"+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<=", ">>=", "&^=",
		"|", "<<", ">>":
		return true
	}
	return false
}

@ A \.* after an operand is the one genuine ambiguity: a product or a pointer type.
The programmer's own spacing usually settles it, and reliably---for the two read
quite differently. A pointer type keeps a space {\it before\/} the star and runs
straight into its type after it: \.{p~*int}, \.{w~*W}. A product has either no space
at all (\.{a*b}, the form |gofmt| uses to group a higher-precedence factor) or a
space on each side (\.{a~*~b}); either way it is set spaced, the \.{cweave} way. So
|pointerStar| clings the star to its right when the source put a blank before it but
none after. That leaves \.{*a**b} a product of two dereferences---the middle star
is tight on its left---so it comes out \.{*a~*~*b}, as in \.{cweave}. This appeal
to the source is the escape hatch \.{cweave} spells
\.{@@[}\thinspace\dots\thinspace\.{@@]}: where intent must be marked, the author's
spacing marks it.

@ Spacing alone cannot settle every star, though. The element type of an array runs
tight against the brackets---|gofmt| writes \.{[256]*Node}, the star crammed on
both sides, exactly as it writes the product \.{a[i]*b}. Here the deciding fact is
grammatical, not typographic: the \.] before the star closes an {\it array type\/},
not an {\it index}, so the star is a pointer. |starAfterArrayType| makes that call,
and the star clings right whenever either signal---a leading blank, or a preceding
array-type bracket---says pointer.
@<Space code tokens by grammar@>=
func pointerStar(pk tokKind, pt string, cur token, toks []token, k int) bool {
	if cur.kind != tkOp || cur.text != "*" || !isOperandEnd(pk, pt) {
		return false
	}
	tightAfter := k+1 < len(toks) && toks[k+1].kind != tkSpace
	if !tightAfter {
		return false
	}
	spaceBefore := k > 0 && toks[k-1].kind == tkSpace
	return spaceBefore || starAfterArrayType(toks, k)
}

@ The gap before a \.[ is settled directly by the source blank |gofmt| writes
between a declared name and its type (\.{b [256]int}) but never before an index
(\.{a[i]}). The star after the matching \.] needs the grammatical judgement
|arrayType| makes for the bracket opening at |open|. A \.[ that follows no operand
at all---at the start of a type, or after \.*, \.{map}, \.{chan}, a comma, an open
paren---always begins a type. A \.[ that does follow an operand is an index when
crammed against it and an array type when the source keeps them apart. Stacked
brackets defer to the innermost, so the whole run agrees: \.{[3][4]int} is a type
throughout, \.{m[3][4]} an index chain throughout.
@<Space code tokens by grammar@>=
func arrayType(toks []token, open int) bool {
	p := prevSignificant(toks, open)
	if p < 0 {
		return true
	}
	if toks[p].kind == tkOp && toks[p].text == "]" {
		return arrayType(toks, matchingBracket(toks, p))
	}
	if !isOperandEnd(toks[p].kind, toks[p].text) {
		return true
	}
	return open > 0 && toks[open-1].kind == tkSpace
}

@ |matchingBracket| walks back from a \.] to its \.[, counting depth so nested
brackets are skipped; a lone \.{[]} is a single token and never confuses the count.
|starAfterArrayType| uses it to ask whether the token just before a star is such an
array type's closing bracket.
@<Space code tokens by grammar@>=
func matchingBracket(toks []token, close int) int {
	depth := 0
	for i := close; i >= 0; i-- {
		if toks[i].kind != tkOp {
			continue
		}
		switch toks[i].text {
		case "]":
			depth++
		case "[":
			if depth--; depth == 0 {
				return i
			}
		}
	}
	return -1
}
@#
func starAfterArrayType(toks []token, k int) bool {
	j := prevSignificant(toks, k)
	if j < 0 || toks[j].kind != tkOp || toks[j].text != "]" {
		return false
	}
	open := matchingBracket(toks, j)
	return open >= 0 && arrayType(toks, open)
}

@ |isMethodReceiver| decides whether the parenthesis just after \.{func} opens a
method receiver rather than a function literal's parameters. A receiver is followed
by the method name and then another parenthesis---\.{func (r T) Name(\dots)}---%
whereas a literal's parameter list is followed by a result type or a body. That
trailing parenthesis may itself be empty (\.{func (r T) Name()}), in which case it
is the merged |()| token rather than a lone |(|.
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
				return j >= 0 && toks[j].kind == tkOp && (toks[j].text == "(" || toks[j].text == "()")
			}
		}
	}
	return false
}

@ |renderToken| renders a single \GO/ token as a \TEX/ fragment, used inside
math. Keywords and builtins are set bold (\.{\\KW}), identifiers italic
(\.{\\ID}). A typewriter macro---an \.{@@d} name or a predeclared constant---%
uses \.{\\MAC}, which wraps \.{\\tentex} in an \.{\\hbox} so it works in the
surrounding math mode; the sole exception is |nil|, \GO/'s null value, shown with
a symbol (\.{\\nil}, a capital lambda) as cweave shows \CEE/'s \.{NULL}. An
identifier reformatted by \.{@@f name TeX} is set as the control sequence
|\name| (see |texControlSeq|), letting you dress a plain name up as any bit of
mathematics you please. A comment is set in roman with \.{\\CM} (escaped for
roman text mode, not the typewriter \.{\\charNN} codes, but letting $...$ math
through), its leading \.{//} tightened by a small kern (\.{\\commentkern}),
whose two slashes are otherwise set rather far apart.
@<Render one token@>=
func renderToken(t token) string {
	switch t.kind {
	case tkKeyword, tkBuiltin:
		return "\\KW{" + escIdent(t.text) + "}"
	case tkIdent:
		return "\\ID{" + escIdent(t.text) + "}"
	case tkMacro:
		if t.text == "nil" {
			return "\\nil "
		}
		return "\\MAC{" + escTT(t.text) + "}"
	case tkTeXCS:
		return "\\" + texControlSeq(t.text) + " "
	case tkNumber:
		return renderNumber(t.text)
	case tkString:
		return "\\ST{" + escTT(t.text) + "}"
	case tkStructTag:
		return "\\ST{" + escTT(t.text[1:len(t.text)-1]) + "}"
	case tkComment:
		if rest, ok := strings.CutPrefix(t.text, "//"); ok {
			return "\\CM{/\\kern\\commentkern/" + escComment(rest) + "}"
		}
		open, close, body := blockCommentDelims(t.text)
		return "\\CM{" + open + escComment(body) + close + "}"
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
			return "\\hex{" + numDigits(s[2:]) + "}"
		case 'o', 'O':
			return "\\oct{" + numDigits(s[2:]) + "}"
		case 'b', 'B':
			return "\\bin{" + numDigits(s[2:]) + "}"
		}
		if isOctalDigits(s[1:]) {
			return "\\oct{" + numDigits(s[1:]) + "}"
		}
	}
	return "\\NU{" + numDigits(s) + "}"
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
(set as a \.{\\X} link and recorded as a use), an index entry \.{@@\^},
\.{@@.}, or \.{@@:} is recorded and removed, and a \.{@@q...@@>} source comment is
dropped. Everything else---the user's \TEX/---falls through unchanged.
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
			wv.xr.addSectionUse(name, secNum)
			fmt.Fprintf(&b, "\\X{%d}{%s}", wv.defNum[name], wv.renderName(name))
			i = end + 2
			continue
		}
	case '^', '.', ':':
		if end := strings.Index(s[i+2:], "@@>"); end >= 0 {
			end += i + 2
			wv.xr.addManualIndex(d, s[i+2:end], secNum)
			i = closeUpBlankedLine(s, i, end+2)
			continue
		}
	case 'q':
		if end := strings.Index(s[i+2:], "@@>"); end >= 0 {
			end += i + 2
			i = closeUpBlankedLine(s, i, end+2) // drop the source-only comment
			continue
		}
	}
}

@ An index entry sets no text of its own, and by long habit it is written on a
line to itself, just under the word it files---as \.{CWEB}'s own sources do.
Removing it would leave that line blank, and to \TEX/ a blank line is a
\.{\\par}: the paragraph would break where the author only meant to file an
entry. So when the entry had the line to itself we take the newline with it, and
the prose closes up as |cweave| leaves it.

An entry with prose beside it keeps its newline, which \TEX/ reads as the space
it always was---swallowing that one would run the neighbouring words together.
The same care serves a \.{@@q...@@>} source comment, which likewise vanishes.
@<Process commentary \TEX/@>=
func closeUpBlankedLine(s string, start, after int) int {
	for k := start - 1; k >= 0; k-- {
		if s[k] == '\n' {
			break // nothing but blanks before it: the line was the entry's own
		}
		if s[k] != ' ' && s[k] != '\t' && s[k] != '\r' {
			return after // prose shares the line; its newline is a real space
		}
	}
	j := after
	for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\r') {
		j++
	}
	if j < len(s) && s[j] == '\n' {
		return j + 1
	}
	return after
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
each identifier to the index. Like a code part, the fragment is first split by
|common.ScanCode|, so the control texts \.{CWEB} allows in inner-\CEE/ context are
honored rather than lexed as \GO/: an index entry (\.{@@\^ @@. @@:}) is recorded, a
\TEX/ box (\.{@@t}) and verbatim output (\.{@@=}) are set as they are in a code
part, a section name is linked, and an \.{@@q} comment has already been dropped.
The lone |emit| both flushes a pending source blank (as \.{\\\ }) and marks the
run started, so the next blank counts. The whole fragment is wrapped in \.{\\PB},
which supplies the enclosing \.{\$...\$} only when \TEX/ is not already in math
mode. So a \.{\|...\|} the author placed inside a \.{\$...\$} (as \.{CWEB} allows),
or inside a text-mode \.{\\halign} cell of a \.{\$\$...\$\$} display, comes out
right either way---\TEX/ itself, not \.{gweave}, decides the mode.
@<Render an inline code fragment@>=
func (wv *Weaver) inlineCode(code string, secNum int, record bool) string {
	var st lexState
	var b strings.Builder
	b.WriteString("\\PB{")
	pendingSpace := false
	started := false
	prevSigKind := tkNewline
	prevSigText := ""
	prevPrevSigText := ""
	emit := func(s string) {
		if pendingSpace {
			b.WriteString("\\ ")
			pendingSpace = false
		}
		b.WriteString(s)
		started = true
	}
	for _, a := range common.ScanCode(code) {
		@<Render one inline atom@>
	}
	b.WriteString("}")
	return b.String()
}

@ The atoms are dispatched by kind, as a code part's are, but written into the
single math group rather than into broken lines. A paste cancels the pending
space so its neighbours abut; \GO/ text is handed to the token loop.
@<Render one inline atom@>=
switch a.Kind {
case common.AText:
	for _, t := range lexGo(a.Text, &st) {
		@<Set one token of an inline atom@>
	}
case common.AIndex:
	if record {
		wv.xr.addManualIndex(a.Index, a.Text, secNum)
	}
case common.ATeX:
	emit("\\hbox{" + a.Text + "}")
case common.AVerbatim:
	emit("\\ST{" + escTT(a.Text) + "}")
case common.ARef:
	name := wv.w.Resolve(a.Text)
	wv.xr.addSectionUse(name, secNum)
	emit(fmt.Sprintf("\\X{%d}{%s}", wv.defNum[name], wv.renderName(name)))
case common.APaste:
	pendingSpace = false
}

@ A blank or newline only arms the pending space; a significant token flushes it,
is recorded in the index when the run records and the name is indexable, and is
set in its effective class---a field tag inside a |...| losing its backquotes just
as one in a code part does.
@<Set one token of an inline atom@>=
if t.kind == tkSpace || t.kind == tkNewline {
	if started {
		pendingSpace = true
	}
	continue
}
qual := qualifierOf(prevSigKind, prevSigText, prevPrevSigText)
if record && (t.kind == tkIdent || t.kind == tkBuiltin) && indexable(t.text) && !wv.noIndexed(t.text, qual) {
	wv.xr.addIdentUse(t.text, secNum)
}
emit(renderToken(structTagged(token{kind: wv.effKind(t, qual), text: t.text}, prevSigKind, prevSigText)))
prevPrevSigText = prevSigText
prevSigKind, prevSigText = t.kind, t.text

@ |renderComment| typesets a code comment. As in \.{CWEB}, the comment is \TEX/:
a |...| span inside it is set as the \GO/ code it represents (via |inlineCode|),
and everything else passes through verbatim, so ordinary \TEX/ control sequences
work -- at the cost (again as in \.{CWEB}) that the author must escape any \TEX/
specials. A literal bar is written \.{\|}. The whole thing is wrapped in \.{\\CM},
with the leading \.{//} of a line comment tightened by a small kern and the
\.{/*}\thinspace\.{*/} of a block comment set the \.{CWEB} way (see below).
@<Render a code comment@>=
func (wv *Weaver) renderComment(secNum int, text string) string {
	if rest, ok := strings.CutPrefix(text, "//"); ok {
		return "\\CM{/\\kern\\commentkern/" + wv.commentBody(secNum, rest) + "}"
	}
	open, close, body := blockCommentDelims(text)
	return "\\CM{" + open + wv.commentBody(secNum, body) + close + "}"
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
			b.WriteString(lit.String()) // raw \TEX/, as \.{CWEB} treats a comment
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

@ \.{CWEB} sets the \.* of a block comment's \.{/*} and \.{*/} as a math \.{\\ast},
which rides on the axis rather than high up where the roman star sits; we follow
suit. Either delimiter may be missing---a continuation line of a multi-line
comment can carry only one, or neither---so each is peeled off on its own, and
the blank beside it is trimmed so the \.{\\,} sets the gap.
@<Render a code comment@>=
func blockCommentDelims(text string) (open, close, body string) {
	body = text
	if rest, ok := strings.CutPrefix(body, "/*"); ok {
		open = "$/\\ast\\,$"
		body = strings.TrimLeft(rest, " \t")
	}
	if rest, ok := strings.CutSuffix(body, "*/"); ok {
		close = "$\\,\\ast/$"
		body = strings.TrimRight(rest, " \t")
	}
	return open, close, body
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
brace-counter would miss---|switch|/|select| case bodies, dedented labels and
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
	pending         []pendingBlock // block-opening keywords awaiting their braces
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

@ A block-opening keyword (|func|, |if|, |for|, |switch|, \dots) arms a pending
block: a brace that later turns up at its parenthesis depth opens a statement block
rather than a composite literal. A single slot cannot see the whole picture, though
---the signature |func f(g func() int) {| carries a nested |func| {\it type\/} whose
own arming would bury the outer function's, so the body's brace, arriving at depth
zero, would be misread. So the pending blocks form a stack. |opensBlock| asks whether
the brace now at hand matches the innermost pending; |dropPendingFrom| discards those
armed at a parenthesis depth |d| or deeper---a |func| type that a closing
parenthesis leaves without ever a brace, or any keyword whose statement simply ended.
@<Track structural indentation@>=
type pendingBlock struct {
	parenDepth int  // the parenthesis depth at which the brace is expected
	isSwitch   bool // whether the block is a switch or select
}

func (in *indenter) opensBlock() bool {
	n := len(in.pending)
	return n > 0 && in.pending[n-1].parenDepth == in.parenDepth
}

func (in *indenter) dropPendingFrom(d int) {
	kept := in.pending[:0]
	for _, p := range in.pending {
		if p.parenDepth < d {
			kept = append(kept, p)
		}
	}
	in.pending = kept
}

@ |beginLine| chooses the indentation for a line whose first significant token is
|t|. A continuation line of an open literal is emitted verbatim at column zero. A
fresh statement---one the previous line did not continue---resets |stmtDepth| to
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
level---one past |openerIndent|, except that a |switch| body's own level (where a
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

@ A brace opens a block when the innermost pending keyword sits at this parenthesis
depth (|opensBlock|), and a composite literal (or struct/interface type) otherwise;
matching it pops that pending. Only a block's indentation follows the statement; a
composite's follows its own line, so the two push |openerIndent| from different
sources. Parentheses and brackets are never blocks; when one closes it discards any
pending keyword the parentheses swallowed unbraced, so a nested |func| type in a
signature no longer masks the outer function's own pending block. (A composite literal
inside a ${\bf for}\ldots{\bf range}$ header could be mistaken for the block, but only |for|
allows one unparenthesized, and only when it spans lines would the misread show---a
corner rare enough to leave be.)
@<Update the bracket stack for a token@>=
switch t.kind {
case tkKeyword:
	switch t.text {
	case "func", "if", "for", "else", "switch", "select", "struct", "interface":
		in.pending = append(in.pending, pendingBlock{parenDepth: in.parenDepth,
			isSwitch: t.text == "switch" || t.text == "select"})
	case "case", "default":
		if n := len(in.stack); n > 0 && in.stack[n-1].isSwitch {
			in.stack[n-1].sawCase = true
		}
	}
case tkOp:
	switch t.text {
	case "{":
		if in.opensBlock() {
			p := in.pending[len(in.pending)-1]
			in.pending = in.pending[:len(in.pending)-1]
			in.stack = append(in.stack, indentFrame{opener: '{',
				openerIndent: in.blockOpenerIndent(), isBlock: true, isSwitch: p.isSwitch})
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
		in.dropPendingFrom(in.parenDepth + 1) // a func type these parens left unbraced
	}
}

@ A block's body is indented from the enclosing block's content level when the brace
sits at statement level, but from the current line when it opens inside an open
expression---a function literal passed as an argument, say, whose body should not
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
an operator that cannot end one---\GO/'s own automatic-semicolon rule. |endLine|
records the verdict for the next line. |beginGeneric| and |advanceGeneric| are the
section-reference and verbatim counterparts of |beginLine| and |advance|, treating
that material as an ordinary statement token.
@<Track structural indentation@>=
func (in *indenter) endLine() {
	if in.lineHadToken {
		in.prevContinues = continuesStmt(in.lastToken)
		if !in.prevContinues {
			in.dropPendingFrom(in.parenDepth) // a keyword whose statement ended unbraced
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
statement label |Name:| alone on its line---an identifier, a colon, then the
line's end---so it can be pulled in a level; requiring the colon to end the line
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
	case "(", "[", "{", ")", "]", "}", "[]", "{}", "()", ",", ":", "++", "--":
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
	tkStructTag              // a field's \.{`...`} tag, set without its backquotes
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
|true|, and |false| are set in typewriter rather than bold---they are constant
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
string becomes one woven line per physical line (a single \.{\\ST} spanning
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
or---the default---a word, number, or operator.
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
combines them into single tokens. The empty pairs |[]|, |{}|, and |()| are kept
whole so the typesetter can give them a thin space.
@<Match a multi-character operator@>=
var multiOps = []string{
	"<<=", ">>=", "&^=", "...",
	"<-", "++", "--", "==", "!=", "<=", ">=", ":=", "&&", "||",
	"<<", ">>", "&^", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=",
	"[]", // the empty brackets of a slice/array type, kept as one token
	"{}", // empty braces (struct{}, interface{}, T{}), kept as one token
	"()", // an empty call or parameter list, kept as one token
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
@#
func indexByte(s string, b byte, from int) int {
	for i := from; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
@#
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
space (\.{\\SP}), the space glyph in slot 32 of the typewriter font, the way
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
			b.WriteString("\\SP ")
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
unary/binary distinction for |*|, |&|, and friends needs no grammar analysis. The
cases fall into three families, spelled out in the sections just below; an operator
named in none of them is set from its own characters---a lone byte escaped, a
longer run set tight.
@<Render an operator as a math atom@>=
func renderOp(s string) string {
	switch s {
	@<Typeset a relation or a logical connective@>
	@<Typeset a bitwise or shift operator@>
	@<Typeset an increment or decrement@>
	@<Typeset an ellipsis or an empty bracket pair@>
	}
	if len(s) == 1 {
		return "\\mathord{" + escMathOp(s) + "}"
	}
	return tightMathOp(s)
}

@ The relations get their real math symbols, and the logical connectives borrow
\.{CWEB}'s---a wedge, a vee, a negation sign---so the code reads like the
mathematics it mirrors. The short declaration \.{:=} is the one operator here that
prints as itself, but it goes out as a single macro, \.{\\K}, rather than as two
atoms: \.{WEB} set Pascal's \.{:=} as a left arrow through \.{\\K}, and \.{\\K} is
the same hook for \GO/. Its default in \.{gwebmac.tex} is the two characters run
together, so nothing changes unless a document asks for something else in its
limbo---say |\let\K=\Leftarrow|.
@<Typeset a relation or a logical connective@>=
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
	return "\\mathord{\\gets}"
case ":=":
	return "\\mathord{\\K}" // short declaration: one macro, so a document can \.{\\let} it

@ Bitwise-or is \.{CWEB}'s \.{\\mid}, a relation bar; exclusive-or borrows
\.{CWEB}'s circled plus for \.{\^}; the shifts are tight double angles. Each
assignment form simply appends an \.{=}. Bit-clear, \.{\&\^}, is \GO/'s alone---C
has no such operator, so its glyph is a choice and not a tradition---and it goes
out through a macro of its own, \.{\\AN}, a circled slash by default, on the same
reasoning as \.{\\K} above.
@<Typeset a bitwise or shift operator@>=
case "|":
	return "\\mathord{\\mid}" // bitwise or, as \.{CWEB} (a mid relation bar)
case "|=":
	return "\\mathord{\\mid}\\mathord{=}" // or-assign
case "^":
	return "\\mathord{\\oplus}" // bitwise xor, as \.{CWEB} (a circled plus)
case "^=":
	return "\\mathord{\\oplus}\\mathord{=}" // xor-assign: \.{\^} is a circled plus too
case "&^":
	return "\\mathord{\\AN}" // bit clear (and-not): its own symbol, and its own macro
case "&^=":
	return "\\mathord{\\AN}\\mathord{=}" // and-not-assign
case "<<":
	return "\\mathord{\\ll}" // left shift, as \.{CWEB} (a tight double angle)
case ">>":
	return "\\mathord{\\gg}" // right shift
case "<<=":
	return "\\mathord{\\ll}\\mathord{=}"
case ">>=":
	return "\\mathord{\\gg}\\mathord{=}"

@ The postfix \.{++} and \.{--} borrow \.{CWEB}'s \.{\\PP} and \.{\\MM}: a pair of
small, raised, tightly kerned signs that read as one operator rather than two full
plus or minus glyphs jammed side by side.
@<Typeset an increment or decrement@>=
case "++":
	return "\\mathord{\\PP}"
case "--":
	return "\\mathord{\\MM}"

@ The ellipsis is a single math \.{\\ldots}; the empty bracket, brace, and paren
pairs get a thin space so their two halves do not jam together --- the same gap
regardless of which bracket it is.
@<Typeset an ellipsis or an empty bracket pair@>=
case "...":
	return "\\mathord{\\ldots}"
case "[]":
	// empty slice/array brackets: a thin space keeps them from jamming
	return "\\mathord{[}\\,\\mathord{]}"
case "{}":
	// empty braces (struct{}, interface{}, T{}): likewise a thin space
	return "\\mathord{\\{}\\,\\mathord{\\}}"
case "()":
	// an empty call or parameter list: the same thin space as [] and {}
	return "\\mathord{(}\\,\\mathord{)}"

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
			b.WriteString("$<$") // \.{cmr} \.{(OT1)} has no \.{<} glyph; use math
		case '>':
			b.WriteString("$>$") // likewise for \.{>}
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
			parts[i] = fmt.Sprintf("\\sD{%d}", n)
		} else {
			parts[i] = fmt.Sprintf("\\s{%d}", n)
		}
	}
	return strings.Join(parts, ", ")
}

@ |writeBackMatter| emits the {\sc PDF} bookmarks, the index, the list of named
sections, and the table of contents that close a woven document. The \.{\\dest}
destination at the top of the section-names page (targeted by the ``Names of the
sections'' bookmark) is numbered one past the last section, so it never collides
with a section's own destination.
@<Write the back matter@>=
func (wv *Weaver) writeBackMatter(bw *bufio.Writer) {
	wv.writeBookmarks(bw)
	bw.WriteString("\n\\inx\n")
	wv.writeIndex(bw)
	bw.WriteString("\\fin\n")
	fmt.Fprintf(bw, "\\secdest{%d}%%\n", len(wv.w.Sections)+1)
	wv.writeSectionNames(bw)
	bw.WriteString("\\con\n\\end\n")
}

@ |writeBookmarks| emits one \.{\\bookmark} per starred section, in document
order, so a {\sc PDF} outline can be built whose nesting follows the \.{@@*}, \.{@@*1},
\.{@@*2} depths. Each entry carries its depth (for the \.{dvipdfmx} route, which
nests by level) and its number of direct children (for \.{pdftex}'s count model). A
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

@ Each starred section becomes a bookmark carrying its depth (for the \.{dvipdfmx}
route, which nests by level) and its number of direct children (for \.{pdftex}'s
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
	fmt.Fprintf(bw, "\\bookmark{%d}{%d}{%d}{%s}%%\n", s.Depth, s.Number, children, bookmarkTitle(s.Title))
}

@ A final top-level ``Names of the sections'' entry (linking to the destination
one past the last section) lists every defined section name beneath it, each
linking to its defining section, as cweave does. The negative child count starts
the group collapsed; \.{\\outsecname} holds the title, which the Korean backend
localizes. The whole block is bracketed by \.{\\ifsecs} so that \.{\\nosecs} (and
\.{\\noinx}), which drop the section-name list, drop its outline too rather than
leaving it pointing at a destination that is no longer emitted.
@<Emit the ``Names of the sections'' bookmarks@>=
var names []string
for _, n := range wv.sortedSectionNames() {
	if wv.defNum[n] > 0 {
		names = append(names, n)
	}
}
bw.WriteString("\\ifsecs\n")
fmt.Fprintf(bw, "\\bookmark{%d}{%d}{%d}{\\outsecname}%%\n", topDepth, len(wv.w.Sections)+1, -len(names))
for _, n := range names {
	fmt.Fprintf(bw, "\\bookmark{%d}{%d}{0}{%s}%%\n", topDepth+1, wv.defNum[n], bookmarkTitle(n))
}
bw.WriteString("\\fi\n")

@ |bookmarkTitle| reduces a starred-section title to plain text safe for a {\sc PDF}
outline: a |...| span keeps its inner text, \.{@@@@} becomes an at-sign, the
\TEX/-special characters are dropped, and a known text-logo control sequence is
replaced by its plain form---as \.{CWEB}'s outline sanitizer does---so that a
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

@ A control word---a backslash and a run of letters---that names one of the
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
	render  string // typeset form of the entry head (\.{\\ID}{...}, \.{\\IR}{...}, ...)
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
		return "\\MAC{" + escTT(name) + "}"
	case tkTeXCS:
		return "$\\" + texControlSeq(name) + "$" // its macro assumes math mode
	}
	return "\\ID{" + escIdent(name) + "}"
}
for name, secs := range wv.xr.identUse {
	it := get(head(name), strings.ToLower(name))
	for s := range secs {
		it.secs[s] = true
	}
}
for name, secs := range wv.xr.identDef {
	it := get(head(name), strings.ToLower(name))
	for s := range secs {
		it.secs[s] = true
		it.defs[s] = true
	}
}

@ A manual entry---\.{@@.} typewriter, \.{@@:} raw \TEX/, \.{@@\^} roman---is
rendered by its kind and recorded at the section where it appeared.
@<Collect the manual index entries@>=
for _, e := range wv.xr.manualIndex {
	var render string
	switch e.kind {
	case '.':
		render = "\\IT{" + escTT(e.text) + "}"
	case ':':
		render = "\\IC{" + e.text + "}"
	default: // '\.{\^}'
		render = "\\IR{" + escProse(e.text) + "}"
	}
	it := get(render, strings.ToLower(e.text))
	it.secs[e.sec] = true
}

@ The entries are sorted by their case-folded key (ties broken by the rendered
form) and emitted as \.{\\II} lines, each pairing the head with its section list.
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
	fmt.Fprintf(bw, "\\II{%s}{%s}\n", it.render, secList(it.secs, it.defs))
}

@ |writeSectionNames| emits the list of named sections with their defining and
using section numbers. |sortedSectionNames| gives the shared ordering used both
here and for the {\sc PDF} outline children beneath ``Names of the sections''.
@<Write the list of section names@>=
func (wv *Weaver) writeSectionNames(bw *bufio.Writer) {
	for _, n := range wv.sortedSectionNames() {
		fmt.Fprintf(bw, "\\NS{%s}{%d}{%s}\n",
			wv.renderName(n), wv.defNum[n], usedNote(wv.xr.sectionUses[n]))
	}
}

func (wv *Weaver) sortedSectionNames() []string {
	names := map[string]bool{}
	for n := range wv.xr.sectionDefs {
		names[n] = true
	}
	for n := range wv.xr.sectionUses {
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
|\Nused|/|\Nuseds| macros (singular/plural) so a localization file can
translate it, exactly as |\U|/|\Us| do for the under-definition notes.
@<The ``used in'' note@>=
func usedNote(uses map[int]bool) string {
	if len(uses) == 0 {
		return ""
	}
	macro := "\\Nused"
	if len(uses) > 1 {
		macro = "\\Nuseds"
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
	defs := wv.xr.sectionDefs[name]
	if len(defs) > 1 {
		others := map[int]bool{}
		for s := range defs {
			if s != secNum {
				others[s] = true
			}
		}
		macro := "\\A"
		if len(others) > 1 {
			macro = "\\As"
		}
		fmt.Fprintf(&b, "%s{%s}%%\n", macro, secList(others, nil))
	}
	if uses := wv.xr.sectionUses[name]; len(uses) > 0 {
		macro := "\\U"
		if len(uses) > 1 {
			macro = "\\Us"
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
	"os"
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
		`\N{0}{1}{Demo}`, // starred section with title
		`$\ID{main}$`,    // inline code
		`\KW{package}`,   // keyword bold
		`\KW{func}`,      // keyword bold
		`\ID{main}`,      // identifier italic
		`\X{2}{body}`,    // reference resolved to defining section 2
		`\D{2}{body}`,    // definition headline
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("woven output missing %q\n---\n%s", c, out)
		}
	}
}

@ The back matter ends with a top-level "Names of the sections" \sc {PDF} outline
entry (\.{\\outsecname}) linking to a destination on the section-names page
(numbered one past the last section), under which every section name is a
collapsible child linking to its defining section, as cweave does. Here the
one name "x" is defined in section 2, so the group has a single child and a
negative count (collapsed).
@(gweave_test.go@>=
func TestNamesBookmark(t *testing.T) {
	out := weaveString(t, "@@* A.\n@@c\npackage main\n@@ B.\n@@<x@@>=\n_ = 0\n")
	if !strings.Contains(out, `\bookmark{0}{3}{-1}{\outsecname}`) {
		t.Errorf("missing/!collapsed Names-of-the-sections bookmark:\n%s", out)
	}
	if !strings.Contains(out, `\bookmark{1}{2}{0}{x}`) {
		t.Errorf("missing section-name child bookmark:\n%s", out)
	}
	if !strings.Contains(out, `\secdest{3}`) {
		t.Errorf("missing section-names destination:\n%s", out)
	}
	if !strings.Contains(out, "\\ifsecs\n\\bookmark{0}{3}{-1}{\\outsecname}") {
		t.Errorf("Names-of-the-sections outline should be guarded by \\ifsecs:\n%s", out)
	}
}

@ @(gweave_test.go@>=
func TestWeaveEscaping(t *testing.T) {
	out := weaveString(t, `@@ x
@@c
m := map[string]int{}
s := "a\tb"
`)
	if !strings.Contains(out, `\ID{m}`) || !strings.Contains(out, `\KW{map}`) {
		t.Errorf("expected identifier/keyword highlighting:\n%s", out)
	}
	// Braces in code must be escaped for math mode.
	if !strings.Contains(out, `\{`) || !strings.Contains(out, `\}`) {
		t.Errorf("braces not escaped:\n%s", out)
	}
}

@ A blank inside a string literal prints as a visible space (\.{\\SP}), as cweb
does; each blank becomes its own marker.
@(gweave_test.go@>=
func TestWeaveStringVisibleSpace(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\ns := \"a b  c\"\n")
	if !strings.Contains(out, `\ST{"a\SP b\SP \SP c"}`) {
		t.Errorf("string blanks should become \\SP markers:\n%s", out)
	}
}

@ A \.{@@q...@@>} comment in prose addresses the \.{.w} reader alone, so it never
reaches the woven \TEX/, while the prose on either side of it survives.
@(gweave_test.go@>=
func TestWeaveQCommentInProse(t *testing.T) {
	out := weaveString(t, "@@ Visible @@q HIDDEN @@> tail.\n@@c\npackage main\n")
	if strings.Contains(out, "HIDDEN") {
		t.Errorf("@@q text leaked into woven prose:\n%s", out)
	}
	if !strings.Contains(out, "Visible") || !strings.Contains(out, "tail") {
		t.Errorf("prose around @@q was lost:\n%s", out)
	}
}

@ A \.{@@t...@@>} control text is a \TEX/ box set amid the code; \.{gweave} wraps
it in an \.{\\hbox} as \.{cweave} does, so math (cweb's own \.{2@@t\$\^\{15\}\$@@>}
trick) survives the surrounding math mode instead of derailing it.
@(gweave_test.go@>=
func TestWeaveTeXBoxInCode(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nvar a = 1 @@t$^{15}$@@> + 2\n")
	if !strings.Contains(out, `\hbox{$^{15}$}`) {
		t.Errorf("@@t should be wrapped in \\hbox:\n%s", out)
	}
}

@ Inner-\CEE/ code (a \.{\|...\|} span in prose) is scanned like a code part, so the
control texts \.{CWEB} allows there are honored, not lexed as \GO/: an index
entry records (and prints nothing inline), a \.{@@t} box and \.{@@=} verbatim are set,
and a \.{@@q} comment vanishes---none of it leaking as stray tokens.
@(gweave_test.go@>=
func TestWeaveInnerCControlCodes(t *testing.T) {
	out := weaveString(t, "@@ A |x @@^ROM@@> @@.TT@@> @@t\\bf B@@> @@=V@@> @@q Z @@> y| end.\n@@c\npackage main\n")
	for _, want := range []string{`\IR{ROM}`, `\IT{TT}`, `\hbox{\bf B}`, `\ST{V}`} {
		if !strings.Contains(out, want) {
			t.Errorf("inner-C control text lost %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, `\mathord{@}`) || strings.Contains(out, `\ID{ROM}`) || strings.Contains(out, `\ID{Z}`) {
		t.Errorf("inner-C control code leaked as stray tokens:\n%s", out)
	}
}

@ \.{nil} prints as a symbol (\.{\\nil}), the way cweb shows \CEE/'s \.{NULL}, not in
typewriter; the other predeclared constants stay typewriter.
@(gweave_test.go@>=
func TestWeaveNilSymbol(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nvar p *int = nil\n_ = true\n")
	if !strings.Contains(out, `\nil `) {
		t.Errorf("nil should render as \\nil:\n%s", out)
	}
	if strings.Contains(out, `\MAC{nil}`) {
		t.Errorf("nil should not be typewriter:\n%s", out)
	}
	if !strings.Contains(out, `\MAC{true}`) {
		t.Errorf("true should stay typewriter:\n%s", out)
	}
}

@ The leading ``\.{//}" of a comment is tightened with \.{\\commentkern}.
@(gweave_test.go@>=
func TestWeaveCommentSlashKern(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nx := 1 // hi\n")
	if !strings.Contains(out, `\CM{/\kern\commentkern/ hi}`) {
		t.Errorf("comment // not kerned:\n%s", out)
	}
}

@ A block comment's \.{/*} and \.{*/} are set with \.{CWEB}'s math \.{\\ast}, so
the star sits on the axis instead of riding high as the roman \.* does.
@(gweave_test.go@>=
func TestWeaveBlockCommentAsterisk(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nx := 1 /* hi */\n")
	if !strings.Contains(out, `\CM{$/\ast\,$hi$\,\ast/$}`) {
		t.Errorf("block comment /* */ should use CWEB's math asterisk:\n%s", out)
	}
}

@ A trailing comment is set off from the code by the generous \.{\\CS} gap, not the
ordinary \.{\\GS} of an inter-token space.
@(gweave_test.go@>=
func TestWeaveCommentGap(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nx := 1 // hi\n")
	if !strings.Contains(out, `\CS $\CM{`) {
		t.Errorf("trailing comment should get the generous \\CS gap:\n%s", out)
	}
}

@ @(gweave_test.go@>=
func TestWeaveUnderscoreIdent(t *testing.T) {
	out := weaveString(t, `@@ x
@@c
var my_var int
`)
	if !strings.Contains(out, `\ID{my\_var}`) {
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
		`\II{\ID{main}}{\sD{1}}`, // main defined (underlined) in section 1
		`\II{\ID{x}}{`,            // x indexed
		`\sD{1}`,                   // x defined via := in section 1
		`\NS{use x}`,               // named section in the list
		`\U{`,                      // "used in" note
		`\A{`,                      // "also defined in" note (two def sites)
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
		i--
	}
	switch x {
	default:
	}
}
`)
	checks := map[string]string{
		`\neq`:                     "!= should render as \\neq",
		`\geq`:                     ">= should render as \\geq",
		`\mathord{\gets}`:          "<- should render as a left arrow",
		`\mathord{\PP}`:           "++ should render as cweave's tight \\PP symbol",
		`\mathord{\MM}`:           "-- should render as cweave's tight \\MM symbol",
		`$\KW{if}$\BS `:          "if is set off from its clause with the wider \\BS",
		`\KW{default}\mathord{:}`: "default: should be tight (no space before colon)",
	}
	for sub, msg := range checks {
		if !strings.Contains(out, sub) {
			t.Errorf("%s\nwant substring %q in:\n%s", msg, sub, out)
		}
	}
}

@ A name declared with `type' is a user type and renders bold (\.{\\KW})
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
	for _, want := range []string{`\KW{entry}`, `\KW{Graph}`, `\KW{Vertex}`} {
		if !strings.Contains(out, want) {
			t.Errorf("want declared type bold %q in:\n%s", want, out)
		}
	}
	// frac is a struct field, not a type, so it stays an italic identifier.
	if strings.Contains(out, `\KW{frac}`) {
		t.Errorf("a struct field must not be bolded as a type:\n%s", out)
	}
}

@ As in \.{cweave}, a call's ``\.{(}'' directly after a function name clings tight,
the way \.{cweave} sets |f(x)|; a func literal's or type's \.{(} clings the same way
(\.{func(n int)}), while a method receiver's takes a full space (\.{func (r T)
m()})---a reserved word's space before its parenthesis, as \.{cweave} gives \.{if}
or \.{sizeof}.
@(gweave_test.go@>=
func TestWeaveCallParenTight(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nvar _ = f(a)\nvar cdq func(l, r int)\nfunc (r *T) m() {}\n")
	checks := map[string]string{
		`\ID{f}\mathord{(}`:          "a call f( clings tight, as cweave sets a call",
		`\KW{func}\mathord{(}`:       "a func literal/type func( clings the same way",
		`\KW{func}$\GS $\mathord{(}`: "a method receiver func ( gets a full space",
	}
	for sub, msg := range checks {
		if !strings.Contains(out, sub) {
			t.Errorf("%s\nwant %q in:\n%s", msg, sub, out)
		}
	}
}

@ Two adjacent words---a keyword and a name, a name and its type---get the wider
\.{\\W}, \.{cweave}'s text interword space (\.{var foo Type}, \.{n int}), while an
operator keeps the narrower medmuskip \.{\\GS} (\.{a + b}).
@(gweave_test.go@>=
func TestWeaveWordSpacing(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nvar foo Type\nfunc bar(n int) T\nz := a + b\n")
	checks := map[string]string{
		`\KW{var}$\W $\ID{foo}$\W $\ID{Type}`: "var, name, and type get the wider word space",
		`\KW{func}$\W $\ID{bar}`:                "func and its name get the word space",
		`\ID{n}$\W $\KW{int}`:                   "a parameter and its type get the word space",
		`\ID{a}$\GS $\mathord{+}$\GS $\ID{b}`:    "an operator keeps the narrower medmuskip",
	}
	for sub, msg := range checks {
		if !strings.Contains(out, sub) {
			t.Errorf("%s\nwant %q in:\n%s", msg, sub, out)
		}
	}
}

@ A statement block's braces are set off with the wider \.{\\BS} on every open
side---before the opening \.{\char123}, after it, and before the closing
\.{\char125}---while a composite literal's braces cling to their contents, as
\.{cweave} sets them.
@(gweave_test.go@>=
func TestWeaveBlockBraceSpacing(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nfunc f() int { return g([]int{1, 2}) }\n")
	checks := map[string]string{
		`\KW{int}$\BS $\mathord{\{}`:    "a block's opening brace is set off from its head",
		`\mathord{\{}$\BS $\KW{return}`: "a block's opening brace breathes before its body",
		`\mathord{)}$\BS $\mathord{\}}`:  "a block's closing brace breathes after its body",
		`\KW{int}\mathord{\{}\NU{1}`:    "a composite literal's opening brace clings to its first element",
		`\NU{2}\mathord{\}}`:             "a composite literal's closing brace clings to its last element",
	}
	for sub, msg := range checks {
		if !strings.Contains(out, sub) {
			t.Errorf("%s\nwant %q in:\n%s", msg, sub, out)
		}
	}
}

@ Inside an enclosing literal the element type may be elided, and then a brace
opens a value rather than following a type. Such a brace is spaced as the element
it is: after a comma it takes the comma's thin space, and after a key's colon the
same gap a plain value would get, so \.{23: \char123 2, 3\char125} lines up with
\.{45: bar} beneath it.
@(gweave_test.go@>=
func TestWeaveElidedLiteralBraceSpacing(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nvar t = map[int]P{23: {2, 3}, 45: bar}\nvar u = [][]int{{1}, {2}}\n")
	checks := map[string]string{
		`\mathord{:}$\GS $\mathord{\{}`:    "a keyed element's brace is spaced like any other value",
		`\mathord{,}$\punct $\mathord{\{}`: "an element's brace takes the comma's thin space",
		`\KW{int}\mathord{\{}\mathord{\{}`: "but a brace still clings to its type, and to an enclosing brace",
	}
	for sub, msg := range checks {
		if !strings.Contains(out, sub) {
			t.Errorf("%s\nwant %q in:\n%s", msg, sub, out)
		}
	}
}

@ The spacing categories are exercised token by token: a slice result type sits
apart from the parameter list (\.{func() []T}, as |gofmt| spaces it) while a stacked
slice type clings (\.{[3][]int}); a named array type keeps the source's blank
(\.{b [256]int}) but an index clings (\.{a[i]}); and a spaced pointer keeps its
blank before the star yet clings after (\.{p *int}).
@(gweave_test.go@>=
func TestWeaveTypeSpacing(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nfunc f(x int) []int { return nil }\n"+
		"func g() [3][]int { return z }\nvar b [256]int\nvar p *int\nvar s = a[i]\n")
	checks := map[string]string{
		`\mathord{)}$\W $\mathord{[}\,\mathord{]}\KW{int}`: "a slice result type takes the word space, as p []byte does",
		`\mathord{]}\mathord{[}\,\mathord{]}\KW{int}`:       "a stacked slice type clings: [3][]int",
		`\ID{b}$\W $\mathord{[}\NU{256}`:                  "a named array type gets the name-and-type word space: b [256]int",
		`\ID{p}$\W $\mathord{*}\KW{int}`:                  "a named pointer type gets the word space, clinging after the star: p *int",
		`\ID{a}\mathord{[}\ID{i}\mathord{]}`:               "an index clings to its operand: a[i]",
	}
	for sub, msg := range checks {
		if !strings.Contains(out, sub) {
			t.Errorf("%s\nwant %q in:\n%s", msg, sub, out)
		}
	}
}

@ These are the Go-only spacing decisions with no \CEE/ analogue, frozen so they do
not regress. A spread \.{...} in a call clings to its operand, while a variadic
parameter's \.{...int} keeps the ordinary medium space---the two are told apart by
what follows the \.{...}. Channel send is left at the medium space, matched to
nothing.
@(gweave_test.go@>=
func TestGoOnlySpacingDecisions(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"spread clings", "@@ x\n@@c\npackage p\nvar _ = f(a...)\n",
			`\ID{a}\mathord{\ldots}\mathord{)}`},
		{"variadic keeps the medium space", "@@ x\n@@c\nfunc v(a ...int) {}\n",
			`\ID{a}$\GS $\mathord{\ldots}$\GS $\KW{int}`},
		{"channel send stays medium", "@@ x\n@@c\npackage p\nvar _ = c <- d\n",
			`\ID{c}$\GS $\mathord{\gets}$\GS $\ID{d}`},
	} {
		out := weaveString(t, c.src)
		if !strings.Contains(out, c.want) {
			t.Errorf("%s: %s\n\twant: %s\n\tgot:  %s", c.name, c.src, c.want, out)
		}
	}
}

@ A block-heading keyword---|if|, |for|, |switch|, |select|---is set off from its
clause with the same wider \.{\\BS} its brace gets, so \.{if x \char123} reads
evenly on both sides of the clause. An ordinary keyword like |case| or |func| keeps
the plain \.{\\GS}. The header's own semicolons take that same block space after
them, clinging to the clause before---as \.{cweave} breaks a \.{for} header.
@(gweave_test.go@>=
func TestWeaveStmtHeadSpacing(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nfunc f() {\nif x { g() }\n"+
		"for i := 0; i < n; i++ { h() }\nswitch v {\ncase 1:\n}\n}\n")
	checks := map[string]string{
		`\KW{if}$\BS $\ID{x}`:     "if is set off from its clause with a block space",
		`\KW{for}$\BS $\ID{i}`:    "for is set off from its clause",
		`\NU{0}\mathord{;}$\BS $\ID{i}`: "a for-header ; clings before and breaks after",
		`\KW{switch}$\BS $\ID{v}`: "switch is set off from its clause",
		`\KW{case}$\W $\NU{1}`:    "case gets the word space, not a block head's",
		`\KW{func}$\W $\ID{f}`:    "func gets the word space, not a block head's",
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

@ Bitwise xor (\.{\^}, \.{\^=}) shows as a circled plus (\.{\\oplus}), as \.{CWEB}
sets a caret. Bit clear (\.{\&\^}, \.{\&\^=}), which has no \CEE/ analogue, gets its
own symbol through \.{\\AN}, rather than an \.{\&} run together with a circled
plus.
@(gweave_test.go@>=
func TestWeaveCaretOperators(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\na = b ^ c\na ^= b\na &^= b\nd := e &^ f\n")
	for _, want := range []string{
		`\mathord{\oplus}`,            // \.{\^} xor: a circled plus
		`\mathord{\oplus}\mathord{=}`, // \.{\^=}
		`\mathord{\AN}`,              // \.{\&\^} bit clear: its own macro
		`\mathord{\AN}\mathord{=}`,   // \.{\&\^=}
	} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, `\mathord{\&}\mathord{\oplus}`) {
		t.Errorf("bit clear must be \\AN, not an ampersand-circledplus:\n%s", out)
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
	if !strings.Contains(out, `\KW{Counts}`) {
		t.Errorf("@@f should typeset Counts bold like a type:\n%s", out)
	}
	if !strings.Contains(out, `\KW{hidden}`) {
		t.Errorf("@@s should also change the typeset class:\n%s", out)
	}
	if !strings.Contains(out, `\II{\ID{Counts}}`) {
		t.Errorf("@@f keeps the identifier in the index:\n%s", out)
	}
	if strings.Contains(out, `\II{\ID{hidden}}`) {
		t.Errorf("@@s should omit the identifier from the index:\n%s", out)
	}
}

@ A \.{@@d} sets every identifier it lists in typewriter (\.{\\MAC}), even across
line breaks, while a name it does not mention stays italic (\.{\\ID}).
@(gweave_test.go@>=
func TestWeaveMacroMultipleNames(t *testing.T) {
	out := weaveString(t, "@@ @@d Push Pop\n   Peek\n@@c\nvar _ = Push + Pop + Peek + Other\n")
	for _, name := range []string{"Push", "Pop", "Peek"} {
		if !strings.Contains(out, `\MAC{`+name+`}`) {
			t.Errorf("@@d should set %s in typewriter:\n%s", name, out)
		}
	}
	if !strings.Contains(out, `\ID{Other}`) {
		t.Errorf("a name not in @@d should stay italic:\n%s", out)
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
	if strings.Contains(out, `\ID{x1}`) {
		t.Errorf("x1 should not fall back to an italic identifier:\n%s", out)
	}
}

@ A {\it qualified\/} format directive (\.{@@s foo.Bar int}) sets only the |Bar|
written as |foo.Bar|, leaving the |Bar| of |abc.Bar| (or a bare |Bar|) alone; the
unqualified \.{@@s Bar int} sets every |Bar|.
@(gweave_test.go@>=
func TestWeaveQualifiedFormat(t *testing.T) {
	q := weaveString(t, "\\input gwebmac\n@@s foo.Bar int\n@@ x\n@@c\nvar a = foo.Bar\nvar b = abc.Bar\n")
	if !strings.Contains(q, `\ID{foo}\mathord{.}\KW{Bar}`) {
		t.Errorf("foo.Bar should typeset Bar bold:\n%s", q)
	}
	if !strings.Contains(q, `\ID{abc}\mathord{.}\ID{Bar}`) {
		t.Errorf("abc.Bar should leave Bar italic under a qualified directive:\n%s", q)
	}
	all := weaveString(t, "\\input gwebmac\n@@s Bar int\n@@ x\n@@c\nvar b = abc.Bar\n")
	if !strings.Contains(all, `\ID{abc}\mathord{.}\KW{Bar}`) {
		t.Errorf("unqualified @@s Bar should bold every Bar:\n%s", all)
	}
}

@ Constants declared in an |iota| enumeration are set in typewriter (\.{\\MAC}),
everywhere they are used, while a plain \.{const} block and a one-line \.{const}
stay italic (\.{\\ID}), and the |iota| line's type stays whatever it was.
@(gweave_test.go@>=
func TestWeaveIotaConst(t *testing.T) {
	out := weaveString(t, "\\input gwebmac\n@@ x\n@@c\n"+
		"const (\n\tRed Color = iota\n\tGreen\n)\n"+
		"const (\n\tPi = 3.14\n)\n"+
		"const Limit = 1\n"+
		"var _ = Red + Green\n")
	for _, name := range []string{"Red", "Green"} {
		if !strings.Contains(out, `\MAC{`+name+`}`) {
			t.Errorf("iota constant %s should be typewriter:\n%s", name, out)
		}
	}
	for _, name := range []string{"Pi", "Limit", "Color"} {
		if !strings.Contains(out, `\ID{`+name+`}`) {
			t.Errorf("%s should stay italic:\n%s", name, out)
		}
	}
}

@ Indentation is derived from the block structure, not the source whitespace, so
this deliberately flush-left fragment---a range loop with a nested |if|---is
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

@ A signature that spans lines and carries a |func| {\it type\/} once fooled the
brace tracker: the nested |func| buried the outer function's pending block, so the
body's brace was misread as a composite literal and the body sat a level too deep.
The pending-block stack keeps them apart, and the body lands at level one.
@(gweave_test.go@>=
func TestWeaveMultilineSignatureIndent(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\n"+
		"func f(g func() int,\nh int) {\nx := 1\n}\n")
	var got []string
	for _, m := range regexp.MustCompile(`\\GL\{(\d+)\}`).FindAllStringSubmatch(out, -1) {
		got = append(got, m[1])
	}
	want := []string{"0", "1", "1", "0"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("multiline-signature indent levels = %v, want %v\n%s", got, want, out)
	}
}

@ Spacing is derived from the grammar, math-like, not copied from the source: a
pointer type \.{*int} is tight, an index \.{xs[i]} is tight, but a product---%
even the tight \.{a*b} |gofmt| writes to group a factor---is set spaced, as in
\.{cweave}.
@(gweave_test.go@>=
func TestWeaveGrammarSpacing(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nfunc f(p *int) {\nr := a*b + c\ns := xs[i]\n}\n")
	checks := map[string]string{
		`\mathord{*}\KW{int}`:                  "a pointer type *int is tight",
		`\ID{xs}\mathord{[}\ID{i}\mathord{]}`: "an index xs[i] is tight",
		`\ID{a}$\GS $\mathord{*}$\GS $\ID{b}`: "a product a*b is set spaced",
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
		`\mathord{*}\ID{a}$\GS $\mathord{*}$\GS $\mathord{*}\ID{b}`: "*a**b -> *a * *b",
		`\mathord{*}\KW{int}`: "a pointer *int clings to its type",
		`\mathord{*}\ID{T}`:   "[]*T keeps *T tight",
	}
	for sub, msg := range checks {
		if !strings.Contains(out, sub) {
			t.Errorf("%s\nwant substring %q in:\n%s", msg, sub, out)
		}
	}
}

@ A pointer element type is crammed against its array brackets, so spacing alone
cannot tell \.{[256]*Node} (a pointer) from \.{a[i]*b} (a product); the closing
\.] settles it by grammar. And a declared name keeps the space before its array
type (\.{b [256]int}) that |gofmt| never puts before an index.
@(gweave_test.go@>=
func TestWeaveArrayPointer(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\n"+
		"func f(src *[256]*Node, b [256]*Node) {\nvar m [3][4]*int\nr := a[i]*b\n}\n")
	checks := map[string]string{
		`\mathord{]}\mathord{*}\ID{Node}`:                 "[256]*Node keeps *Node tight",
		`\ID{b}$\W $\mathord{[}`:                         "b [256] gets the name-and-type word space",
		`\mathord{[}\NU{3}\mathord{]}\mathord{[}\NU{4}\mathord{]}\mathord{*}\KW{int}`: "[3][4]*int stays tight throughout",
		`\ID{i}\mathord{]}$\GS $\mathord{*}$\GS $\ID{b}`: "a[i]*b stays a spaced product",
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
	want := `Compute \PB{\ID{area}} now`
	if !strings.Contains(out, `\X{2}{`+want+`}`) {
		t.Errorf("reference name should typeset |area| as code; missing %q in:\n%s", want, out)
	}
	if !strings.Contains(out, `\D{2}{`+want+`}`) {
		t.Errorf("definition headline should typeset |area| as code; missing %q in:\n%s", want, out)
	}
}

@ @(gweave_test.go@>=
func TestWeaveLayoutCodes(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nvar y = a@@,b\nvar z = c@@/d\nvar w = e@@|f\nvar v = g@@#h\n")
	checks := map[string]string{
		`\ID{a}\,$\W $\ID{b}`: "@@, should add a thin space on top of the grammar's word space",
		`\GL{0}{$\ID{d}$}`:      "@@/ should force a new line",
		`\SO `:                  "@@| should emit an optional break",
		`\BL`:                   "@@# should emit a blank line",
	}
	for sub, msg := range checks {
		if !strings.Contains(out, sub) {
			t.Errorf("%s\nwant substring %q in:\n%s", msg, sub, out)
		}
	}
}

@ A forced break (\.{@@/} or \.{@@\#}) inside an open bracket re-indents its
continuation for the nesting rather than hanging it at the statement's own margin:
here the break after the comma steps \.{2)} in one level (\.{\\GL\{2\}} beneath the
statement's \.{\\GL\{1\}}).
@(gweave_test.go@>=
func TestWeaveForceBreakIndent(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nfunc f() {\n\tg(1,@@/2)\n}\n")
	if !strings.Contains(out, `\GL{1}{$\ID{g}\mathord{(}\NU{1}\mathord{,}$}`) {
		t.Errorf("statement line should be at indent 1:\n%s", out)
	}
	if !strings.Contains(out, `\GL{2}{$\NU{2}\mathord{)}$}`) {
		t.Errorf("@@/ continuation inside ( should step in to indent 2:\n%s", out)
	}
}

@ Here |foo| is only {\it used\/} (inside a call), but \.{@@!} forces it to be
indexed as a definition, so its section number is underlined in the index.
@(gweave_test.go@>=
func TestWeaveForceDefinition(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nfunc f() { use(@@!foo) }\n")
	if !strings.Contains(out, `\II{\ID{foo}}{\sD{1}}`) {
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
	if strings.Contains(out, `\II{\ID{_}}`) {
		t.Errorf("the blank identifier _ should not be indexed:\n%s", out)
	}
	// chunk is used in two different sections (2 and 3), so the plural notes apply.
	if !strings.Contains(out, `\Us{\s{2}, \s{3}}`) {
		t.Errorf("uses in two sections should emit \\Us:\n%s", out)
	}
	if !strings.Contains(out, `\NS{chunk}{1}{\Nuseds{\s{2}, \s{3}}}`) {
		t.Errorf("section-names entry malformed:\n%s", out)
	}
}

@ A section name wrapped across lines (a newline inside \.{@@<...@@>}) must match
the same name written on one line, as in \.{CWEB}. Otherwise the reference
resolves to section 0, which also crashes luatex's {\sc PDF} backend.
@(gweave_test.go@>=
func TestWrappedSectionName(t *testing.T) {
	out := weaveString(t, "@@* Start.\n@@c\nfunc main() { @@<do the\nthing@@> }\n@@ @@<do the thing@@>=\nx := 1\n")
	if strings.Contains(out, `\X{0}`) {
		t.Errorf("wrapped section name failed to resolve (got \\X{0}):\n%s", out)
	}
	if !strings.Contains(out, `\X{2}`) {
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
	if !strings.Contains(out, `\CM{`) {
		t.Fatalf("no comment emitted:\n%s", out)
	}
	if !strings.Contains(out, `\ID{x}`) {
		t.Errorf("|x| in a comment should render as code \\ID{x}:\n%s", out)
	}
	if strings.Contains(out, "|x|") {
		t.Errorf("the bars should be consumed, not printed literally:\n%s", out)
	}
	out3 := weaveString(t, "@@ x\n@@c\nx := 1 // see \\.{foo.go}\n")
	if !strings.Contains(out3, `\.{foo.go}`) {
		t.Errorf("\\.{...} in a comment should pass through verbatim:\n%s", out3)
	}

	out2 := weaveString(t, "@@ x\n@@c\nx := 1 // a | b\n")
	if strings.Contains(out2, `\ID{b}`) {
		t.Errorf("an unmatched bar must not turn the rest into code:\n%s", out2)
	}
}

@ Every \.{\|...\|} from a \TEX/ part is wrapped in \.{\\PB}, never in bare
dollars of \.{gweave}'s own choosing: \.{\\PB} adds the \.{\$...\$} at typeset
time only when \TEX/ is not already in math, so a span inside a \.{\$...\$} the
author opened---or inside a text-mode \.{\\halign} cell of a \.{\$\$...\$\$}
display---comes out right without \.{gweave} guessing the mode. (The
\.{\\ifmmode} logic itself is exercised by weaving and typesetting \.{torture.w}.)
@(gweave_test.go@>=
func TestInlineCodeUsesGPB(t *testing.T) {
	out := weaveString(t, "@@ In prose |maxDim| and in math $k < |maxDim|$ alike.\n@@c\npackage main\n")
	if strings.Count(out, `\PB{\ID{maxDim}}`) != 2 {
		t.Errorf("both |...| spans should render as \\PB{\\ID{maxDim}}:\n%s", out)
	}
	if strings.Contains(out, `$\ID{maxDim}$`) {
		t.Errorf("gweave must not add its own $...$ around a span; \\PB decides:\n%s", out)
	}
	// A |...| in a comment is wrapped in \PB too.
	cm := weaveString(t, "@@ x\n@@c\nvar _ = 0 // note |y| here\n")
	if !strings.Contains(cm, `\PB{\ID{y}}`) {
		t.Errorf("a comment span should render as \\PB{\\ID{y}}:\n%s", cm)
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

@ An empty call or parameter list, \.{()}, gets the same thin space as \.{[]} and
\.{\{\}} --- the gap this section fixes was cramped before. A non-empty \.{(x)}
must stay tight, exactly as a non-empty \.{\{x\}} does above.
@(gweave_test.go@>=
func TestWeaveEmptyParens(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nvar _ = f()\n")
	if !strings.Contains(out, `\mathord{(}\,\mathord{)}`) {
		t.Errorf("empty parens () should get a thin space, like [] and {}:\n%s", out)
	}
	out2 := weaveString(t, "@@ x\n@@c\nvar _ = f(x)\n")
	if strings.Contains(out2, `\mathord{(}\,`) {
		t.Errorf("a non-empty call f(x) should not get a thin space:\n%s", out2)
	}
}

@ A method receiver's own parentheses still get a full space even when the
method's parameter list --- past the name --- is empty; |isMethodReceiver| must
recognize that trailing \.{()} as a parameter list, not mistake the receiver for
a function literal. A chained call or index after an empty call, \.{f()(x)} or
\.{f()[0]}, stays as tight as it would after any other operand.
@(gweave_test.go@>=
func TestWeaveEmptyParensMethodReceiver(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nfunc (r *T) m() {}\n")
	if !strings.Contains(out, `\KW{func}$\GS $\mathord{(}`) {
		t.Errorf("a receiver with empty params should still get a full space:\n%s", out)
	}
	out2 := weaveString(t, "@@ x\n@@c\nvar _ = f()[0]\n")
	if strings.Contains(out2, `)}$\GS $\mathord{[}`) {
		t.Errorf("indexing the result of an empty call should stay tight:\n%s", out2)
	}
}

@ A statement ending in an empty call or parameter list, such as a bare function
type, still closes its statement --- the next line must not be misread as its
continuation and over-indented.
@(gweave_test.go@>=
func TestWeaveEmptyParensStatementEnd(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nfunc f() {\n\tvar g func()\n\tx := 1\n\t_ = g\n\t_ = x\n}\n")
	if !strings.Contains(out, `\GL{1}{$\ID{x}$\rel $\mathord{\K}$\rel $\NU{1}$}`) {
		t.Errorf("a statement after a bare func() type must not over-indent:\n%s", out)
	}
}

@ The next few sections are the operator half of the \.{gweave}-versus-\.{cweave}
spacing audit, frozen as a regression test. Each operator's woven gap was measured,
by rendering both systems, to equal the width \.{cweave} sets it at; freezing the
gap macro here makes any later change to the gap table that drifts from \.{cweave}
fail in \.{CI}, where a full \TEX/ render is not available. The macros' measured
widths: \.{\\punct} is \.{cweave}'s thinmuskip (3mu, 1.667pt), \.{\\GS} the
medmuskip (4mu, 2.22pt), \.{\\rel} the thickmuskip (5mu, 2.778pt); \./ joins tight,
an ordinary atom.

@ Arithmetic, bitwise-and, xor, and the logical operators are all \.{cweave}
|\mathbin|s, so they keep the medium \.{\\GS}; \./ alone is an ordinary atom and
sets tight.
@(gweave_test.go@>=
var spacingLockBinop = []struct{ src, want string }{
	{"a + b", `\ID{a}$\GS $\mathord{+}$\GS $\ID{b}`},
	{"a - b", `\ID{a}$\GS $\mathord{-}$\GS $\ID{b}`},
	{"a * b", `\ID{a}$\GS $\mathord{*}$\GS $\ID{b}`},
	{"a % b", `\ID{a}$\GS $\mathord{\%}$\GS $\ID{b}`},
	{"a & b", `\ID{a}$\GS $\mathord{\&}$\GS $\ID{b}`},
	{"a ^ b", `\ID{a}$\GS $\mathord{\oplus}$\GS $\ID{b}`},
	{"a && b", `\ID{a}$\GS $\mathord{\land}$\GS $\ID{b}`},
	{"a || b", `\ID{a}$\GS $\mathord{\lor}$\GS $\ID{b}`},
	{"a / b", `\ID{a}\mathord{/}\ID{b}`},
}

@ The relations and assignments take the thick \.{\\rel}; so do bitwise-or and the
shifts, which \.{cweave} draws as the relations |\mid|, $\ll$, and $\gg$.
@(gweave_test.go@>=
var spacingLockRel = []struct{ src, want string }{
	{"a == b", `\ID{a}$\rel $\mathord{\equiv}$\rel $\ID{b}`},
	{"a != b", `\ID{a}$\rel $\mathord{\neq}$\rel $\ID{b}`},
	{"a < b", `\ID{a}$\rel $\mathord{<}$\rel $\ID{b}`},
	{"a <= b", `\ID{a}$\rel $\mathord{\leq}$\rel $\ID{b}`},
	{"a | b", `\ID{a}$\rel $\mathord{\mid}$\rel $\ID{b}`},
	{"a << b", `\ID{a}$\rel $\mathord{\ll}$\rel $\ID{b}`},
	{"a >> b", `\ID{a}$\rel $\mathord{\gg}$\rel $\ID{b}`},
}

@ A comma takes the thin \.{\\punct} after it (tight before). Channel send has no
\CEE/ analogue, so it is left at the medium space, not matched to anything.
@(gweave_test.go@>=
var spacingLockMisc = []struct{ src, want string }{
	{"[]int{a, b}", `\ID{a}\mathord{,}$\punct $\ID{b}`},
	{"c <- d", `\ID{c}$\GS $\mathord{\gets}$\GS $\ID{d}`},
}

@ The three tables run through one check: weave \.{var\ \_\ =\ }{\it src\/} and look
for the frozen fragment.
@(gweave_test.go@>=
func TestOperatorSpacingLockedToCweave(t *testing.T) {
	all := append(append(spacingLockBinop, spacingLockRel...), spacingLockMisc...)
	for _, c := range all {
		out := weaveString(t, "@@ x\n@@c\npackage p\nvar _ = "+c.src+"\n")
		if !strings.Contains(out, c.want) {
			t.Errorf("%s: spacing drifted from cweave\n\twant: %s\n\tgot:  %s",
				c.src, c.want, out)
		}
	}
}

@ The gap table freezes the {\it choice\/} of macro; this freezes the macros'
{\it widths}, so a change to \.{gwebmac.tex} that pulls a gap off \.{cweave}'s
muskip is caught too. The file sits two levels up from the test's directory.
@(gweave_test.go@>=
func TestGapMacroWidthsMatchCweave(t *testing.T) {
	src, err := os.ReadFile("../../gwebmac.tex")
	if err != nil {
		t.Skip("gwebmac.tex not reachable from the test directory")
	}
	text := string(src)
	for _, w := range []struct{ macro, em string }{
		{`\def\punct{`, ".1667em"}, // cweave thinmuskip, 3mu
		{`\def\GS{`, ".222em"},      // cweave medmuskip, 4mu
		{`\def\rel{`, ".2778em"},   // cweave thickmuskip, 5mu
	} {
		i := strings.Index(text, w.macro)
		if i < 0 {
			t.Errorf("%s not defined in gwebmac.tex", w.macro)
			continue
		}
		line := text[i:]
		if j := strings.IndexByte(line, '\n'); j >= 0 {
			line = line[:j]
		}
		if !strings.Contains(line, w.em) {
			t.Errorf("%s must keep cweave's width %s; got:\n%s", w.macro, w.em, line)
		}
	}
}

@ Chapter one (depth 0, section 1) has two direct children: the \.{@@*1}
subsections (depth 1). \.{\\bookmark} is \.{\{depth\}\{secNum\}\{children\}\{title\}}.
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
		`\bookmark{0}{1}{2}{Chapter one}`,
		`\bookmark{1}{2}{0}{Sub A}`,
		`\bookmark{1}{3}{0}{Sub B}`,
		`\bookmark{0}{4}{0}{Chapter two}`,
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
as a single multi-line \.{\\ST} (which would end the enclosing \.{\\GL} paragraph).
@(gweave_test.go@>=
func TestWeaveMultilineRawString(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\nvar s = `a\n\nb`\n")
	if strings.Count(out, `\GL`) < 2 {
		t.Errorf("multi-line raw string should span multiple \\GL lines:\n%s", out)
	}
}

@ A field tag sheds its backquotes; a raw string anywhere else keeps them. The
witness puts one of each in every place a raw string can stand---after a type
(the tag), and after a paren, a brace, a comma, a key's colon, and a keyword.
@(gweave_test.go@>=
func TestWeaveStructTagUnquoted(t *testing.T) {
	out := weaveString(t, "@@ x\n@@c\ntype T struct {\n\tA int      `json:\"a\"`\n"+
		"\tB []string `json:\"b\"`\n}\n\nvar v = f(`p`, `q`)\nvar w = []string{`r`}\n"+
		"var y = map[string]string{\"k\": `s`}\n\nfunc g() string { return `t` }\n")
	for _, want := range []string{`\ST{json:"a"}`, `\ST{json:"b"}`} {
		if !strings.Contains(out, want) {
			t.Errorf("a field tag should lose its backquotes; want %q in:\n%s", want, out)
		}
	}
	for _, want := range []string{"\\ST{`p`}", "\\ST{`q`}", "\\ST{`r`}", "\\ST{`s`}", "\\ST{`t`}"} {
		if !strings.Contains(out, want) {
			t.Errorf("a raw string that is not a tag should keep them; want %q in:\n%s", want, out)
		}
	}
}
