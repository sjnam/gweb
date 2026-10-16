@i ../../common/types.w
\def\title{GTANGLE (Version 0.10.3)}
\def\topofcontents{\null\vfill
  \centerline{\titlefont The {\ttitlefont GTANGLE} processor}
  \vskip 15pt
  \centerline{(Version 0.10.3)}
  \vfill}
\def\botofcontents{\vfill\centerline{\smallfont
  Copyright \copyright\ 2026 Soojin Nam. MIT License.}}

@** Processor gtangle.
This is the command-line front end of \.{gtangle}; the tangle engine it drives is
defined in the second half of this web. The input may be named with or without
its \.{.w} extension (\.{gtangle wc} reads \.{wc.w}, as in \.{CWEB}). The unnamed \.{@@c}
sections are written to the input's base name with a \.{.go} extension (in the
\.{-o} directory, default the input's directory); \.{@@(file@@>=} sections are
written to their named files.
@c
package main

import (
	"flag"
	"fmt"
	"go/format"
	"go/scanner"
	gotoken "go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/sjnam/gweb/common"
)

@<The command's entry point@>
@<Print a usage message@>
@<Report a progress line@>
@<Tangle the web and write its files@>

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
		fmt.Printf("gtangle (GWEB) %s\n", common.Version)
		return
	}
	if flag.NArg() < 1 || flag.NArg() > 2 {
		usage()
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "This is GTANGLE, Version %s\n", common.Version)
	if err := run(flag.Arg(0), flag.Arg(1), *outDir); err != nil {
		fmt.Fprintln(os.Stderr, "gtangle:", err)
		os.Exit(1)
	}
}

@ Usage.
@<Print a usage message@>=
func usage() {
	fmt.Fprintln(os.Stderr, "usage: gtangle [-o dir] file[.w] [change[.ch]]")
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
report, tangles (always with \.{//line} directives), and writes each output
file, creating its directory if necessary.
@<Tangle the web and write its files@>=
func run(input, changeFile, outDir string) error {
	input = common.DefaultExt(input, ".w")
	changeFile = common.DefaultExt(changeFile, ".ch")
	w, err := common.ParseWithChange(input, changeFile)
	if err != nil {
		return err
	}
	for _, warn := range w.Warnings {
		fmt.Fprintln(os.Stderr, "gtangle: warning:", warn)
	}
	reportProgress(w)
	if outDir == "" {
		outDir = filepath.Dir(input)
	}

	base := filepath.Base(input)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	defaultFile := base + ".go"

	outs, err := New(w).Tangle(defaultFile)
	if err != nil {
		return err
	}
	@<Write the tangled output files@>
	return nil
}

@ Each tangled output is written under |outDir| (its subdirectories created as
needed); a \.{gofmt} warning is reported if one was attached, and a one-line
confirmation is printed for each file.
@<Write the tangled output files@>=
for _, out := range outs {
	path := filepath.Join(outDir, out.File)
	if dir := filepath.Dir(path); dir != "." {
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			return mkErr
		}
	}
	if writeErr := os.WriteFile(path, out.Content, 0o644); writeErr != nil {
		return writeErr
	}
	if out.Warning != "" {
		fmt.Fprintf(os.Stderr, "gtangle: warning: %s: %s\n", path, out.Warning)
	}
	fmt.Printf("gtangle: wrote %s (%d bytes)\n", path, len(out.Content))
}

@* The tangle engine.
The rest of this web is the engine that \.{gtangle}'s front end drives: it
extracts compilable \GO/ source from a parsed web, expanding named-section
references in program order, the \GO/ analogue of \.{CWEB}'s \.{ctangle}. It is part of
the command's \.{main} package, tangled together with the front end into the
single file \.{gtangle.go}.
@c
@<An output file@>
@<The tangler and its code pieces@>
@<Classify a web's code sections@>
@<Produce all output files@>
@<Skip an all-blank destination@>
@<Render and gofmt one output@>
@<Expand code pieces and references@>
@<The output buffer@>

@ An |Output| is one tangled file: its target name and \GO/ contents. |Warning|
is set (non-fatally) when \.{gofmt} could not format the assembled program.
@<An output file@>=
type Output struct {
	File    string
	Content []byte
	Warning string
}

@ A |Tangler| holds the resolved code of a web, classified by destination: the
refinements (|defs|), the \.{@@(file@@>=} outputs, and the unnamed program text
(|main|). Each destination keeps a list of |codePiece|s rather than one joined
string, so every piece remembers the \.{.w} line it began on -- the anchor for
the \.{//line} directives. As in \.{CWEB}'s \.{ctangle}, those directives are always
emitted (there is no switch to suppress them), so the \GO/ compiler, \.{go vet},
and panic traces report positions in the literate \.{.w} source rather than in
the generated \.{.go}. The buffer marks every line as it goes and
|thinLineMarks| then drops the ones \GO/'s own line counting already implies, so
what is written keeps a mark only where the origin jumps---\.{ctangle}'s density,
with the same positions.
@<The tangler and its code pieces@>=
type Tangler struct {
	w     *common.Web
	defs  map[string][]codePiece // canonical named-section $\rightarrow$ code pieces
	files map[string][]codePiece // \.{@@(file@@>=name} $\rightarrow$ code pieces
	main  []codePiece            // unnamed \.{@@c} sections, in order
}

type codePiece struct {
	code string
	line int
}

@ |New| classifies every code section into the unnamed program, an output file,
or a named refinement, appending each section's code -- with the source line it
began on -- to the pieces for that destination.
@<Classify a web's code sections@>=
func New(w *common.Web) *Tangler {
	t := &Tangler{
		w:     w,
		defs:  map[string][]codePiece{},
		files: map[string][]codePiece{},
	}
	for _, s := range w.Sections {
		if !s.HasCode {
			continue
		}
		p := codePiece{s.Code, s.CodeLine}
		switch {
		case s.Name == "":
			t.main = append(t.main, p)
		case s.IsFile:
			t.files[s.Name] = append(t.files[s.Name], p)
		default:
			name := w.Resolve(s.Name)
			t.defs[name] = append(t.defs[name], p)
		}
	}
	return t
}

@ |Tangle| produces all output files: first the unnamed program (written to
|defaultFile|), then each \.{@@(file@@>=} target in sorted order.
@<Produce all output files@>=
func (t *Tangler) Tangle(defaultFile string) ([]Output, error) {
	var outs []Output

	if nonEmpty(t.main) {
		out, err := t.renderOutput(defaultFile, t.main)
		if err != nil {
			return nil, err
		}
		outs = append(outs, out)
	}

	names := make([]string, 0, len(t.files))
	for name := range t.files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out, err := t.renderOutput(name, t.files[name])
		if err != nil {
			return nil, err
		}
		outs = append(outs, out)
	}

	if len(outs) == 0 {
		return nil, fmt.Errorf("no code to tangle (no @@c or @@(file@@>= sections)")
	}
	return outs, nil
}

@ |nonEmpty| reports whether any piece carries non-blank code, so a destination
made only of whitespace does not produce an empty output file.
@<Skip an all-blank destination@>=
func nonEmpty(pieces []codePiece) bool {
	for _, p := range pieces {
		if strings.TrimSpace(p.code) != "" {
			return true
		}
	}
	return false
}

@ |renderOutput| expands one destination's pieces and runs |gofmt| on the
result. A genuine web error (an undefined or circular reference) is fatal; a
\.{gofmt} failure is not -- the unformatted \GO/ is kept and reported via
|Output.Warning|.

\.{gofmt} runs {\it twice}, on either side of |thinLineMarks|. The first pass
does the formatting proper, on text where every line carries a mark, so it may
reflow as it likes without loosening the map. But a mark is a comment, and a
comment standing between two struct fields breaks the column block \.{gofmt}
aligns fields in; with a mark on every line, no field ever gets aligned. Thinning
removes those comments and so changes what \.{gofmt} itself would write --- the
thinned text is no longer \.{gofmt}'s own output, and \.{gofmt -l} would report
it. The second pass settles that: what we write is what \.{gofmt} writes.
@<Render and gofmt one output@>=
func (t *Tangler) renderOutput(file string, pieces []codePiece) (Output, error) {
	o := &buffer{t: t, atLineStart: true}
	if err := t.expandPieces(pieces, o, nil); err != nil {
		return Output{}, err
	}
	raw := o.bytes()
	formatted, err := format.Source(raw)
	if err != nil {
		return Output{File: file, Content: thinLineMarks(raw),
			Warning: "gofmt could not format the output: " + err.Error()}, nil
	}
	thinned := thinLineMarks(formatted)
	if again, err := format.Source(thinned); err == nil {
		thinned = again
	}
	return Output{File: file, Content: thinned}, nil
}

@* Thinning the line marks.
The buffer marks every line, which keeps the map exact no matter how \.{gofmt}
reflows the text---but it leaves an output where nearly every other line is a
directive. \GO/ counts lines the way \CEE/ does: \.{//line f:N} fixes the {\it
next\/} line at |f:N| and each line after it advances by one, so a mark that only
restates what the count already gives is redundant. |thinLineMarks| drops exactly
those, leaving a mark just where the origin jumps---a new section, an expanded
refinement---which is the density \.{ctangle} writes by hand.

This runs {\it after\/} \.{gofmt}, on the text that will be written, so whatever
\.{gofmt} did to the line structure is already accounted for; the marks that
survive are the ones the final text really needs. Every line keeps the origin it
had before: only redundant directives go, and a directive line carries no origin
of its own.

@ Which lines are marks is settled by \.{go/scanner}, not by matching text: a line
inside a raw string literal can look exactly like a directive, and dropping it
would corrupt the string. |PositionFor| with |adjusted| false reports the physical
line---the scanner honours the very directives we are reading, so the adjusted
position would be the \.{.w} line instead. If the text cannot be scanned cleanly
(a \.{gofmt} failure leaves broken \GO/), nothing is touched.
@<Render and gofmt one output@>=
func thinLineMarks(src []byte) []byte {
	marks, inline, ok := lineMarkLines(src)
	if !ok || len(marks) == 0 {
		return src
	}
	@<Drop the redundant line marks@>
}

@ |lineMarkLines| returns the physical line numbers that begin a \.{//line}
directive, those that hold an inline \.{/*line*/} directive (an expansion inside a
line, which resets the count partway along it), and whether the scan was clean
enough to trust.
@d gotoken.Position gotoken.EOF gotoken.COMMENT
@<Render and gofmt one output@>=
func lineMarkLines(src []byte) (map[int]bool, map[int]bool, bool) {
	fset := gotoken.NewFileSet()
	f := fset.AddFile("", fset.Base(), len(src))
	var s scanner.Scanner
	bad := false
	s.Init(f, src, func(gotoken.Position, string) { bad = true }, scanner.ScanComments)
	marks, inline := map[int]bool{}, map[int]bool{}
	for {
		pos, tok, lit := s.Scan()
		if tok == gotoken.EOF {
			break
		}
		if tok != gotoken.COMMENT {
			continue
		}
		p := f.PositionFor(pos, false)
		switch {
		case strings.HasPrefix(lit, "//line ") && p.Column == 1:
			marks[p.Line] = true
		case strings.HasPrefix(lit, "/*line "):
			inline[p.Line] = true
		}
	}
	return marks, inline, !bad
}

@ The walk itself. |expLine| is the origin the next line would take on its own; a
mark that agrees with it is dropped, any other mark is kept and resets the count.
An ordinary line consumes one origin, exactly as \GO/ will count it. A line that
holds an inline \.{/*line*/} directive has moved the count somewhere the walk does
not follow, so after it the count is not trusted, and the next mark stays.

The one mark we keep although it is redundant is a mark with a blank line on each
side---as where a \.{@@<...@@>} on its own line follows a blank one, the reference
leaving nothing behind but its indentation. Dropping it would leave the two blanks
adjacent, and \.{gofmt}, which allows at most one blank line in a row, would close
the gap on the second pass. That would cost a line, and every origin counted from
here to the next mark would be short by one. Keeping the mark keeps the blanks
apart, so the second pass has no line to remove: it may align columns, but it
cannot move a line, and the marks that survive stay true.
@<Drop the redundant line marks@>=
lines := strings.SplitAfter(string(src), "\n")
out := make([]byte, 0, len(src))
expFile, expLine, have := "", 0, false
prevBlank := false
for i, ln := range lines {
	if !marks[i+1] {
		out = append(out, ln...)
		if have {
			expLine++
		}
		if inline[i+1] {
			have = false
		}
		prevBlank = blankLine(ln)
		continue
	}
	file, n, ok := parseLineMark(ln)
	if ok && have && file == expFile && n == expLine && !betweenBlanks(prevBlank, lines, i) {
		continue // the count already says this; the mark is redundant
	}
	if ok {
		expFile, expLine, have = file, n, true
	}
	out = append(out, ln...)
	prevBlank = false
}
return out

@ |betweenBlanks| reports whether dropping the mark on line |i| would run the
blank line before it into a blank line after it.
@<Render and gofmt one output@>=
func blankLine(s string) bool { return strings.TrimSpace(s) == "" }

func betweenBlanks(prevBlank bool, lines []string, i int) bool {
	return prevBlank && i+1 < len(lines) && blankLine(lines[i+1])
}

@ |parseLineMark| reads back a \.{//line file:N} directive. The file name may hold
a colon, so the number is taken from the last one, as \GO/ itself does.
@<Render and gofmt one output@>=
func parseLineMark(s string) (string, int, bool) {
	rest := strings.TrimSuffix(strings.TrimSuffix(s, "\n"), "\r")
	rest = strings.TrimPrefix(rest, "//line ")
	i := strings.LastIndex(rest, ":")
	if i < 0 {
		return "", 0, false
	}
	n, err := strconv.Atoi(rest[i+1:])
	if err != nil || n <= 0 {
		return "", 0, false
	}
	return rest[:i], n, true
}

@ |expandPieces| expands a list of code pieces in order.
@<Expand code pieces and references@>=
func (t *Tangler) expandPieces(pieces []codePiece, o *buffer, stack []string) error {
	for _, p := range pieces {
		if err := t.expand(p.code, p.line, o, stack); err != nil {
			return err
		}
	}
	return nil
}

@ |expand| expands one code piece, threading the combined-source line through
the text so `\.{//line}' directives stay accurate. Verbatim text tangles like
ordinary text; a paste deletes the preceding whitespace; a reference is followed
recursively; and the woven-output-only atoms are ignored. The atoms are walked by
index, because how a reference is expanded depends on what follows it on its line.
@<Expand code pieces and references@>=
func (t *Tangler) expand(code string, line int, o *buffer, stack []string) error {
	atoms := common.ScanCode(code)
	for i, a := range atoms {
		switch a.Kind {
		case common.AText, common.AVerbatim:
			line = o.writeText(a.Text, line)
		case common.APaste:
			o.trimRight()
			o.pasteNext = true
			o.atLineStart = false
		case common.ARef:
			@<Expand a section reference@>
		case common.ATeX, common.AIndex, common.ALayout, common.AIndexDef:
			// woven-output only; ignored by tangle
		}
	}
	return nil
}

@ A reference is resolved to its canonical name, checked against the recursion
stack for a cycle, and expanded in place.

How it is expanded depends on where it stands. A reference alone on its line is a
statement, or a run of declarations, and is set off by newlines: they keep it apart
from its neighbours, and \.{gofmt} collapses the rest. But a reference may also
stand {\it inside\/} a line, as part of an expression, as \.{CWEB} programs and
Knuth's own \.{WEB}s often have it:
$$\vbox{\halign{#\hfil\cr
  \.{if @@<the character is unprintable@@> \{}\cr
  \.{x := 1 + @@<the offset@@>*3}\cr
  \.{newSpec(@@<the glue@@>)}\cr}}$$
Newlines there would be fatal in \GO/, though harmless in \CEE/. \GO/ ends a
statement at a newline that follows an identifier, a literal, or a closing
bracket, so the condition above, put on a line of its own, becomes the statement
\.{if k < 10;} and the brace after it is an error. Such a reference is therefore
expanded {\it inline\/} (|o.lineHasCode| and |codeFollows| tell which case we
are in). A comment that merely follows a reference does not make it inline.
@<Expand a section reference@>=
name := t.w.Resolve(a.Text)
def, ok := t.defs[name]
if !ok {
	return fmt.Errorf("undefined section <%s>", a.Text)
}
if slices.Contains(stack, name) {
	return fmt.Errorf("circular reference through <%s>", name)
}
if o.lineHasCode() || codeFollows(atoms[i+1:]) {
	@<Expand the reference inside its line@>
} else {
	o.newline()
	if err := t.expandPieces(def, o, append(stack, name)); err != nil {
		return err
	}
	o.newline()
}

@ An inline expansion goes through a buffer of its own; its leading and trailing
blanks are cut off, so no newline is left on either side. A \.{//line} directive
must stand at the start of a line, so the two ends of the expansion are marked by
\GO/'s other form of the directive, \.{/*line file:N*/}, which may stand anywhere
and applies to the character that follows it. The first marks where the
refinement's text begins in the web; the second, after it, puts the rest of the
host line back on the line it came from. Lines {\it inside\/} a refinement that
spans several lines keep their ordinary \.{//line} marks: in the middle of an
expression a newline does no harm as long as the line before it ends in an
operator, which is where a multi-line expression breaks anyway.
@<Expand the reference inside its line@>=
sub := &buffer{t: t}
if err := t.expandPieces(def, sub, append(stack, name)); err != nil {
	return err
}
file, ln := t.w.Origin(def[0].line)
text, file, ln := inlineText(sub.bytes(), file, ln)
o.writeRaw(fmt.Sprintf("/*line %s:%d*/", file, ln))
o.writeRaw(text)
file, ln = t.w.Origin(line)
o.writeRaw(fmt.Sprintf("/*line %s:%d*/", file, ln))

@ |codeFollows| reports whether the atoms after a reference put more code on its
line: text that is not blank and is not just a trailing comment, or another
reference. The woven-output-only atoms are transparent.
@<Expand code pieces and references@>=
func codeFollows(rest []common.Atom) bool {
	for _, a := range rest {
		switch a.Kind {
		case common.AText, common.AVerbatim:
			s, _, nl := strings.Cut(a.Text, "\n")
			s = strings.TrimLeft(s, " \t\r")
			if s != "" {
				return !strings.HasPrefix(s, "//")
			}
			if nl {
				return false
			}
		case common.ARef, common.APaste:
			return true
		}
	}
	return false
}

@ |inlineText| trims an inline expansion. The blanks and the \.{//line} marks in
front of its first character are dropped, and the position of that character is
worked out from them (|file| and |ln| start as the origin of the refinement's
first line); the blanks behind its last character are dropped too. If the text
then ends in a line comment, the directive written after it would be swallowed by
that comment, and the rest of the host line with it; such a comment becomes a
block comment, or is dropped if it holds a \.{*/} of its own.
@<Expand code pieces and references@>=
func inlineText(b []byte, file string, ln int) (string, string, int) {
	s := string(b)
	for {
		t := strings.TrimLeft(s, " \t\r")
		if strings.HasPrefix(t, "\n") {
			s, ln = t[1:], ln+1
			continue
		}
		if mark, rest, ok := strings.Cut(t, "\n"); ok && strings.HasPrefix(mark, "//line ") {
			if f, n, ok := parseLineMark(mark); ok {
				s, file, ln = rest, f, n
				continue
			}
		}
		s = t
		break
	}
	s = strings.TrimRight(s, " \t\r\n")
	if i := trailingLineComment(s); i >= 0 {
		body := s[i+2:]
		if strings.Contains(body, "*/") {
			s = strings.TrimRight(s[:i], " \t")
		} else {
			s = s[:i] + "/*" + body + " */"
		}
	}
	return s, file, ln
}

@ |trailingLineComment| gives the offset of the \.{//} that opens a line comment
running to the end of |s|, or $-1$ if |s| does not end inside one. It reuses the
buffer's lexical tracking, so a \.{//} in a string is not mistaken for a comment.
@<Expand code pieces and references@>=
func trailingLineComment(s string) int {
	var l buffer
	start := -1
	for i := 0; i < len(s); i++ {
		before := l.lex
		l.track(s[i])
		if before != lexLineComment && l.lex == lexLineComment {
			start = i - 1
		}
	}
	if l.lex == lexLineComment {
		return start
	}
	return -1
}

@ The output |buffer| accumulates bytes. It tracks whether it is at the start of
a line so it can prefix each line with a \.{//line} directive, enough of \GO/'s
lexical state to know when such a line start falls {\it inside\/} a token that
spans lines (|lex|, with |slash|, |star| and |esc| for the two-character and
escaped forms), and it supports the \.{@@\&} paste operation, which deletes the
whitespace surrounding it.
@<The output buffer@>=
type buffer struct {
	t           *Tangler
	b           []byte
	pasteNext   bool
	atLineStart bool
	lex         goLex
	slash       bool // a \./ is pending: \.{//} or \./\.* may be starting
	star        bool // a \.* is pending inside a block comment: \.*\./ may close it
	esc         bool // a backslash is pending inside a quoted string or rune
}

@ Only two \GO/ tokens may span a newline: a raw string and a block comment. A
directive written at the start of a line inside either one is not a directive at
all---inside a raw string it silently becomes part of the string's value, and
inside a comment it is just text---so |writeText| marks a line only when the line
really begins in code.
@<The output buffer@>=
type goLex int

const (
	lexCode goLex = iota
	lexLineComment
	lexBlockComment
	lexQuote
	lexRune
	lexRaw
)

func (l goLex) spansLines() bool { return l == lexRaw || l == lexBlockComment }

@ |writeText| copies a run of text, emitting a \.{//line} directive at the start
of each non-blank line that begins in code, and honoring a pending paste (which
trims the leading whitespace of the next text). It returns the updated
combined-source line.
@<The output buffer@>=
func (o *buffer) writeText(s string, line int) int {
	if o.pasteNext {
		s = strings.TrimLeft(s, " \t\n\r")
		o.pasteNext = false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if o.atLineStart && c != '\n' {
			if !o.lex.spansLines() {
				o.lineMark(line)
			}
			o.atLineStart = false
		}
		o.b = append(o.b, c)
		o.track(c)
		if c == '\n' {
			line++
			o.atLineStart = true
		}
	}
	return line
}

@ |track| advances the lexical state by one byte of copied text. A quoted string
or rune cannot legally reach a newline, so an unterminated one is closed there,
as \GO/'s own scanner does.
@<The output buffer@>=
func (o *buffer) track(c byte) {
	switch o.lex {
	case lexCode:
		@<Advance the lexical state in code@>
	case lexLineComment:
		if c == '\n' {
			o.lex = lexCode
		}
	case lexBlockComment:
		if o.star && c == '/' {
			o.lex = lexCode
		}
		o.star = c == '*'
	case lexQuote, lexRune:
		@<Advance the lexical state in a quoted string or rune@>
	case lexRaw:
		if c == '`' {
			o.lex = lexCode
		}
	}
}

@ In code, a \./ waits to see whether a comment is starting; a quote of any of the
three kinds opens its token.
@<Advance the lexical state in code@>=
if o.slash {
	o.slash = false
	switch c {
	case '/':
		o.lex = lexLineComment
		return
	case '*':
		o.lex = lexBlockComment
		o.star = false
		return
	}
}
switch c {
case '/':
	o.slash = true
case '"':
	o.lex = lexQuote
case '\'':
	o.lex = lexRune
case '`':
	o.lex = lexRaw
}

@ @<Advance the lexical state in a quoted string or rune@>=
switch {
case o.esc:
	o.esc = false
case c == '\\':
	o.esc = true
case c == '\n':
	o.lex = lexCode // unterminated: \GO/ ends it at the newline too
case o.lex == lexQuote && c == '"', o.lex == lexRune && c == '\'':
	o.lex = lexCode
}

@ The small mutators: |lineMark| writes a \.{//line} directive for the given
combined-source line (mapped back to its origin), |newline| starts a fresh
line, |writeRaw| copies text that is already marked (an inline expansion) without
adding marks of its own, |trimRight| removes trailing whitespace (for a paste),
|lineHasCode| reports whether the current line already holds code, and |bytes|
returns the accumulated output.
@<The output buffer@>=
func (o *buffer) lineMark(line int) {
	file, ln := o.t.w.Origin(line)
	o.b = append(o.b, fmt.Sprintf("//line %s:%d\n", file, ln)...)
}

func (o *buffer) newline() {
	o.b = append(o.b, '\n')
	o.track('\n')
	o.atLineStart = true
}

func (o *buffer) writeRaw(s string) {
	if o.pasteNext {
		s = strings.TrimLeft(s, " \t\n\r")
		o.pasteNext = false
	}
	for i := 0; i < len(s); i++ {
		o.b = append(o.b, s[i])
		o.track(s[i])
	}
	o.atLineStart = false
}

func (o *buffer) lineHasCode() bool {
	i := strings.LastIndexByte(string(o.b), '\n')
	return strings.TrimSpace(string(o.b[i+1:])) != ""
}

func (o *buffer) trimRight() {
	for len(o.b) > 0 {
		switch o.b[len(o.b)-1] {
		case ' ', '\t', '\n', '\r':
			o.b = o.b[:len(o.b)-1]
		default:
			return
		}
	}
}

func (o *buffer) bytes() []byte { return o.b }

@* Tests.
The tangle engine's tests, one section per case.
@(gtangle_test.go@>=
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	gotoken "go/token"
	"os"
	"go/scanner"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/sjnam/gweb/common"
)

@ @(gtangle_test.go@>=
func TestTangleExpandsAndConcatenates(t *testing.T) {
	const src = `@@ main
@@c
package main

func main() {
	@@<body@@>
}

@@ helper
@@<body@@>=
greet()

@@ more program text
@@c
func greet() { println("hi") }
`
	outs, err := New(common.ParseString(src)).Tangle("prog.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(outs) != 1 {
		t.Fatalf("got %d outputs, want 1", len(outs))
	}
	got := string(outs[0].Content)
	if outs[0].Warning != "" {
		t.Errorf("unexpected warning: %s", outs[0].Warning)
	}
	for _, want := range []string{"func main()", "greet()", `func greet()`} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

@ @(gtangle_test.go@>=
func TestTangleFileSections(t *testing.T) {
	const src = `@@ first
@@(extra.go@@>=
package main

@@ second
@@c
package main
`
	outs, err := New(common.ParseString(src)).Tangle("main.go")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]bool{}
	for _, o := range outs {
		files[o.File] = true
	}
	if !files["main.go"] || !files["extra.go"] {
		t.Errorf("expected main.go and extra.go, got %v", files)
	}
}

@ @(gtangle_test.go@>=
func TestTangleUndefinedReference(t *testing.T) {
	const src = `@@ x
@@c
package main
var _ = @@<missing@@>
`
	_, err := New(common.ParseString(src)).Tangle("p.go")
	if err == nil || !strings.Contains(err.Error(), "undefined") {
		t.Errorf("want undefined-section error, got %v", err)
	}
}

@ @(gtangle_test.go@>=
func TestTangleCircularReference(t *testing.T) {
	const src = `@@ a
@@<a@@>=
@@<b@@>
@@ b
@@<b@@>=
@@<a@@>
@@ root
@@c
package main
var _ = @@<a@@>
`
	_, err := New(common.ParseString(src)).Tangle("p.go")
	if err == nil || !strings.Contains(err.Error(), "circular") {
		t.Errorf("want circular-reference error, got %v", err)
	}
}

@ A name containing a |...| code span still resolves for tangling. The reference
stands inside its line, so the value is expanded there, between two inline
\.{/*line*/} marks.
@(gtangle_test.go@>=
func TestTangleCodeInName(t *testing.T) {
	const src = `@@ root
@@c
package main

var area = @@<the |x| value@@>

@@ helper
@@<the |x| value@@>=
42
`
	outs, err := New(common.ParseString(src)).Tangle("p.go")
	if err != nil {
		t.Fatal(err)
	}
	got := string(outs[0].Content)
	if !strings.Contains(got, "var area = /*line :9*/ 42") {
		t.Errorf("name containing |x| should still match for tangling:\n%s", got)
	}
}

@ |originOf| follows the \.{//line} marks of a tangled file the way \GO/ does---a
mark fixes the next line, every line after it advances by one---and reports the
web line that the first line containing |want| will be blamed for.
@(gtangle_test.go@>=
func originOf(src, want string) (int, bool) {
	line := 0
	for _, ln := range strings.Split(src, "\n") {
		if rest, isMark := strings.CutPrefix(ln, "//line "); isMark {
			if i := strings.LastIndex(rest, ":"); i >= 0 {
				if n, err := strconv.Atoi(rest[i+1:]); err == nil {
					line = n
				}
			}
			continue
		}
		if strings.Contains(ln, want) {
			return line, true
		}
		line++
	}
	return 0, false
}

@ A straight run of lines needs one mark, not one per line---\GO/'s own counting
supplies the rest---and the positions it reports are unchanged.
@(gtangle_test.go@>=
func TestTangleThinsLineMarks(t *testing.T) {
	const src = "@@ x\n@@c\npackage main\n\nfunc main() {\n\ta := 1\n\tb := 2\n" +
		"\tc := 3\n\t_, _, _ = a, b, c\n}\n"
	outs, err := New(common.ParseString(src)).Tangle("p.go")
	if err != nil {
		t.Fatal(err)
	}
	got := string(outs[0].Content)
	if n := strings.Count(got, "//line "); n > 2 {
		t.Errorf("a straight run wants one mark, got %d:\n%s", n, got)
	}
	for _, c := range []struct {
		text string
		line int
	}{{"a := 1", 6}, {"b := 2", 7}, {"c := 3", 8}} {
		if got, ok := originOf(got, c.text); !ok || got != c.line {
			t.Errorf("%q should report web line %d, got %d (found=%v)", c.text, c.line, got, ok)
		}
	}
}

@ What we write must be what \.{gofmt} writes: the tangled \GO/ is committed, and
a tree that \.{gofmt} would still change does not pass its own check. Thinning
runs after the formatting and takes comments out, which is exactly the kind of
edit that can leave text \.{gofmt} wants to touch again---the fields of a struct
align only once the marks between them are gone---so the fixpoint is worth
asserting. The struct here is the case in point: its fields align, and the
alignment must already be there.
@(gtangle_test.go@>=
func TestTangleOutputIsGofmtStable(t *testing.T) {
	const src = "@@ x\n@@c\npackage main\n\ntype T struct {\n\tOriginal string\n" +
		"\tLike string\n\tNoIndex bool\n}\n\n@@ y\n@@<More@@>=\nfunc f() {\n\t_ = T{}\n}\n" +
		"\n@@ z\n@@c\n\n@@<More@@>\n"
	outs, err := New(common.ParseString(src)).Tangle("p.go")
	if err != nil {
		t.Fatal(err)
	}
	got := outs[0].Content
	want, err := format.Source(got)
	if err != nil {
		t.Fatalf("the output does not even parse: %v\n%s", err, got)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("gofmt would still change the output:\n%s", diffLines(string(got), string(want)))
	}
	if !strings.Contains(string(got), "Like     string") {
		t.Errorf("the struct fields should be aligned:\n%s", got)
	}
}

@ The blank-straddling mark, whose case is easy to miss. A reference that expands
to nothing leaves its line blank, so two such references after a blank line give
three blank lines in a row, each with a mark of its own between them. Thin every
one of those marks away and the blanks fall together, whereupon \.{gofmt} closes
the run---costing the lines the origins after it were counted from, which is why
the marks between blanks stay. The web below puts |x := 1| on line 9; before the
guard, \.{gofmt} moved it to the seventh.
@(gtangle_test.go@>=
func TestTangleOriginsAcrossEmptyRefs(t *testing.T) {
	const src = "@@ x\n@@c\npackage main\n\nfunc main() {\n\n\t@@<A@@>\n\t@@<B@@>\n" +
		"\tx := 1\n\t_ = x\n}\n\n@@ a\n@@<A@@>=\n\n@@ b\n@@<B@@>=\n"
	outs, err := New(common.ParseString(src)).Tangle("p.go")
	if err != nil {
		t.Fatal(err)
	}
	got := outs[0].Content
	want, err := format.Source(got)
	if err != nil {
		t.Fatalf("the output does not parse: %v\n%s", err, got)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("gofmt would still change the output:\n%s", diffLines(string(got), string(want)))
	}
	if line, ok := originOf(string(got), "x := 1"); !ok || line != 9 {
		t.Errorf("x := 1 should report web line 9, got %d (found=%v)", line, ok)
	}
}

@ |diffLines| shows the first line where two texts part, which is all the
fixpoint tests need to say what \.{gofmt} would have done differently.
@(gtangle_test.go@>=
func diffLines(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < len(g) && i < len(w); i++ {
		if g[i] != w[i] {
			return fmt.Sprintf("line %d:\n  got  %q\n  want %q", i+1, g[i], w[i])
		}
	}
	return fmt.Sprintf("got %d lines, want %d", len(g), len(w))
}

@ A raw string and a block comment are the two \GO/ tokens that may span a
newline. A mark written at the start of a line inside one would not be a mark at
all---it would become part of the string's value, or text in the comment---so the
buffer writes none there and the token comes through exactly as the web wrote it.
@(gtangle_test.go@>=
func TestTangleLeavesMultilineTokensAlone(t *testing.T) {
	const src = "@@ x\n@@c\npackage main\n\nvar s = `\nplain\n\tindented\n`\n" +
		"\nvar t = \"one line\"\n"
	outs, err := New(common.ParseString(src)).Tangle("p.go")
	if err != nil {
		t.Fatal(err)
	}
	got := string(outs[0].Content)
	if want := "`\nplain\n\tindented\n`"; !strings.Contains(got, want) {
		t.Errorf("a raw string must come through verbatim, want %q in:\n%s", want, got)
	}
	if strings.Contains(got, "plain\n//line") || strings.Contains(got, "`\n//line") {
		t.Errorf("no mark may be written inside a raw string:\n%s", got)
	}
}

@ A line inside a raw string can look exactly like a mark; because |thinLineMarks|
asks \.{go/scanner} rather than matching text, such a line is never mistaken for
one and never dropped.
@(gtangle_test.go@>=
func TestTangleKeepsLineMarkLookalikeInString(t *testing.T) {
	const src = "@@ x\n@@c\npackage main\n\nvar s = `\n//line fake.w:99\n`\n"
	outs, err := New(common.ParseString(src)).Tangle("p.go")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(outs[0].Content); !strings.Contains(got, "//line fake.w:99") {
		t.Errorf("a mark-lookalike inside a raw string must survive:\n%s", got)
	}
}

@ @(gtangle_test.go@>=
func TestTangleIgnoresLayoutCodes(t *testing.T) {
	const src = "@@ x\n@@c\npackage main\n\nvar n = 1@@,@@/@@|@@#@@+@@[@@]@@;2\n"
	outs, err := New(common.ParseString(src)).Tangle("p.go")
	if err != nil {
		t.Fatal(err)
	}
	got := string(outs[0].Content)
	if !strings.Contains(got, "var n = 12") {
		t.Errorf("layout/hint codes must not leak into tangled output:\n%s", got)
	}
}

@ An unknown \.{@@x} must drop exactly its two characters, not corrupt the rest
---a guard against a former double-skip bug.
@(gtangle_test.go@>=
func TestTangleDropsUnknownCode(t *testing.T) {
	const src = "@@ x\n@@c\npackage main\n\nvar a@@?bc = 1\n"
	outs, err := New(common.ParseString(src)).Tangle("p.go")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(outs[0].Content); !strings.Contains(got, "var abc = 1") {
		t.Errorf("unknown @@x should drop exactly two chars:\n%s", got)
	}
}

@ An abbreviated definition (\.{@@<the val...@@>=}) resolves against a reference
that carries the full name.
@(gtangle_test.go@>=
func TestTangleAbbrevAtDefinition(t *testing.T) {
	const src = "@@ x\n@@c\npackage main\n\nvar v = @@<the value@@>\n\n@@ d\n@@<the val...@@>=\n42\n"
	outs, err := New(common.ParseString(src)).Tangle("p.go")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(outs[0].Content); !strings.Contains(got, "var v = /*line :9*/ 42") {
		t.Errorf("abbreviated definition should resolve:\n%s", got)
	}
}

@ A reference inside a line is part of an expression and must stay there: as the
condition of an |if|, in the middle of an arithmetic expression, as an argument,
and spanning lines. Put on lines of their own, each of these would be cut short by
\GO/'s automatic semicolons, and the program would not even parse. A refinement
whose text ends in a line comment is the trap the directive after it must not fall
into, so it is here too; and a reference followed only by a comment still expands
on its own line.
@(gtangle_test.go@>=
func TestTangleRefInsideLine(t *testing.T) {
	const src = "@@ x\n@@c\npackage main\n\nfunc f(n int) int { return n }\n\n" +
		"func main() {\n\tk := 3\n" +
		"\tif @@<k is small@@> {\n\t\tk++\n\t}\n" +
		"\tx := 1 + @@<two@@>*3\n" +
		"\ty := f(@@<two@@>)\n" +
		"\tif @@<a long test@@> {\n\t\tk--\n\t}\n" +
		"\tz := @@<the answer@@>\n" +
		"\t@@<two statements@@> // a comment\n" +
		"\t_, _, _ = x, y, z\n}\n\n" +
		"@@ a\n@@<k is small@@>=\nk < 10\n\n" +
		"@@ b\n@@<two@@>=\n2\n\n" +
		"@@ c\n@@<a long test@@>=\nk > 0 &&\n\tk < 100\n\n" +
		"@@ d\n@@<the answer@@>=\n40 + 2 // a line comment\n\n" +
		"@@ e\n@@<two statements@@>=\nk = 1\nk = 2\n"
	outs, err := New(common.ParseString(src)).Tangle("p.go")
	if err != nil {
		t.Fatal(err)
	}
	got := string(outs[0].Content)
	if outs[0].Warning != "" {
		t.Fatalf("the output should format cleanly: %s\n%s", outs[0].Warning, got)
	}
	if _, err := parser.ParseFile(gotoken.NewFileSet(), "p.go", got, 0); err != nil {
		t.Fatalf("the output does not parse: %v\n%s", err, got)
	}
	for _, want := range []string{"if /*line ", "x := 1 + /*line ", "f( /*line ", "/* a line comment */"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "\n\tk = 1\n") {
		t.Errorf("a reference followed only by a comment should stay on its own line:\n%s", got)
	}
}

@ The inline marks must keep positions true on both sides of an expansion: a token
of the refinement is reported at the refinement's line, and the code after the
reference---on the same line or below it---at its own. The scanner's adjusted
positions are what the compiler, \.{go vet} and panic traces will report.
@(gtangle_test.go@>=
func TestTangleInlineRefPositions(t *testing.T) {
	const src = "@@ x\n@@c\npackage main\n\nfunc main() {\n\tsmall := 3\n" +
		"\tif @@<the test@@> {\n\t\tprintln(\"yes\")\n\t}\n}\n\n" +
		"@@ a\n@@<the test@@>=\nsmall > 0 &&\n\tlimit(small)\n"
	outs, err := New(common.ParseString(src)).Tangle("p.go")
	if err != nil {
		t.Fatal(err)
	}
	got := outs[0].Content
	fset := gotoken.NewFileSet()
	f := fset.AddFile("p.go", fset.Base(), len(got))
	var s scanner.Scanner
	s.Init(f, got, nil, 0)
	lines := map[string][]int{}
	for {
		pos, tok, lit := s.Scan()
		if tok == gotoken.EOF {
			break
		}
		if tok == gotoken.IDENT || tok == gotoken.STRING || tok == gotoken.LBRACE {
			if lit == "" {
				lit = tok.String()
			}
			lines[lit] = append(lines[lit], f.Position(pos).Line)
		}
	}
	for _, c := range []struct {
		tok  string
		want []int
	}{
		{"small", []int{6, 14, 15}}, // the declaration, then the refinement's two lines
		{"limit", []int{15}},
		{`"yes"`, []int{8}},
		{"{", []int{5, 7}}, // the body of main, and the brace after the expansion
	} {
		if g := lines[c.tok]; !slices.Equal(g, c.want) {
			t.Errorf("%s reported at lines %v, want %v:\n%s", c.tok, g, c.want, got)
		}
	}
}

@* Integration tests.
The tangle engine's integration tests: every example tangles to compilable \GO/.

@ @(gtangle_test.go@>=
func importsThirdParty(content []byte) bool {
	f, err := parser.ParseFile(gotoken.NewFileSet(), "", content, parser.ImportsOnly)
	if err != nil {
		return false
	}
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		if i := strings.IndexByte(p, '/'); i >= 0 {
			p = p[:i]
		}
		if strings.Contains(p, ".") {
			return true
		}
	}
	return false
}

@ @(gtangle_test.go@>=
func TestExamplesBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping go build of examples in -short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not found in PATH")
	}

	examples, err := filepath.Glob(filepath.Join("..", "..", "examples", "*.w"))
	if err != nil {
		t.Fatal(err)
	}
	if len(examples) == 0 {
		t.Fatal("no example .w files found")
	}

	for _, ex := range examples {
		t.Run(filepath.Base(ex), func(t *testing.T) {
			t.Parallel()
			buildExample(t, ex)
		})
	}
}

@ @(gtangle_test.go@>=
func buildExample(t *testing.T, path string) {
	t.Helper()

	w, err := common.Parse(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	outs, err := New(w).Tangle(base + ".go")
	if err != nil {
		t.Fatalf("tangle: %v", err)
	}
	for _, o := range outs {
		if strings.HasSuffix(o.File, ".go") && importsThirdParty(o.Content) {
			t.Skipf("%s imports a third-party module; tangled OK, skipping go build", filepath.Base(path))
		}
	}

	dir := t.TempDir()
	haveMod := false
	for _, o := range outs {
		if o.Warning != "" {
			t.Fatalf("%s: %s", o.File, o.Warning)
		}
		if o.File == "go.mod" {
			haveMod = true
		}
		if err := os.WriteFile(filepath.Join(dir, o.File), o.Content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !haveMod {
		// go 1.23 so examples may use range-over-func iterators (e.g. \.{seq.w}).
		const mod = "module gwebexample\n\ngo 1.23\n"
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command("go", "build", "-o", os.DevNull, ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
}

@ @(gtangle_test.go@>=
func TestChangeFileBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping go build in -short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not found in PATH")
	}
	w, err := common.ParseWithChange(
		filepath.Join("..", "..", "examples", "wc.w"),
		filepath.Join("..", "..", "examples", "wc.ch"),
	)
	if err != nil {
		t.Fatal(err)
	}
	outs, err := New(w).Tangle("wc.go")
	if err != nil {
		t.Fatal(err)
	}
	got := string(outs[0].Content)
	if !strings.Contains(got, `%d,%d,%d`) || strings.Contains(got, `%8d`) {
		t.Fatalf("change file not applied to tangled output:\n%s", got)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wc.go"), outs[0].Content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module gwebexample\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", os.DevNull, ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
}

@** Index.
This index covers \.{GTANGLE} alone, as it stands when \.{gtangle.w} is woven
by itself: every identifier it uses (a section number is underlined where the
identifier is defined), together with the manual index entries. In the combined
\.{GWEB} document \.{gweb.ch} removes this section, and the index at the end of
\.{gweb.w} covers all three webs. The list of section names follows.
