@* Command \.{gtangle}.
This is the command-line front end of \.{gtangle}; the tangle engine it drives is
defined in the second half of this web. The input may be named with or without
its \.{.w} extension (|gtangle wc| reads \.{wc.w}, as in cweb). The unnamed \.{@@c} sections are written to the input's base name with a
\.{.go} extension (in the |-o| directory, default the input's directory);
\.{@@(file@@>=} sections are written to their named files.
@(cmd/gtangle/gtangle.go@>=
package main

import (
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/sjnam/gweb/common"
)

@ The entry point parses the flags and arguments and dispatches to |run|. With
\.{-version} it just prints the version; otherwise it prints a one-line banner
to the standard error, in the style of \.{CWEB}, before processing.
@(cmd/gtangle/gtangle.go@>=
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
@(cmd/gtangle/gtangle.go@>=
func usage() {
	fmt.Fprintln(os.Stderr, "usage: gtangle [-o dir] file[.w] [change[.ch]]")
	flag.PrintDefaults()
}

@ A brief progress report in the style of \.{CWEB}: one |*N| on the standard error
for each starred (chapter) section, giving a sense of the web's structure as it
is processed.
@(cmd/gtangle/gtangle.go@>=
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
@(cmd/gtangle/gtangle.go@>=
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
	return nil
}

@* The tangle engine.
The rest of this web is the engine that \.{gtangle}'s front end drives: it
extracts compilable \GO/ source from a parsed web, expanding named-section
references in program order, the \GO/ analogue of \.{CWEB}'s \.{ctangle}. It is part of
the command's \.{main} package, tangled together with the front end into the
single file \.{gtangle.go}.

@ An |Output| is one tangled file: its target name and \GO/ contents. |Warning|
is set (non-fatally) when |gofmt| could not format the assembled program.
@(cmd/gtangle/gtangle.go@>=
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
the generated \.{.go}.
@(cmd/gtangle/gtangle.go@>=
type Tangler struct {
	w     *common.Web
	defs  map[string][]codePiece // canonical named-section -> code pieces
	files map[string][]codePiece // @@(file@@>= name -> code pieces
	main  []codePiece            // unnamed @@c sections, in order
}

type codePiece struct {
	code string
	line int
}

@ |New| classifies every code section into the unnamed program, an output file,
or a named refinement, appending each section's code -- with the source line it
began on -- to the pieces for that destination.
@(cmd/gtangle/gtangle.go@>=
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
@(cmd/gtangle/gtangle.go@>=
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
@(cmd/gtangle/gtangle.go@>=
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
|gofmt| failure is not -- the unformatted \GO/ is kept and reported via
|Output.Warning|.
@(cmd/gtangle/gtangle.go@>=
func (t *Tangler) renderOutput(file string, pieces []codePiece) (Output, error) {
	o := &buffer{t: t, atLineStart: true}
	if err := t.expandPieces(pieces, o, nil); err != nil {
		return Output{}, err
	}
	raw := o.bytes()
	if formatted, err := format.Source(raw); err == nil {
		return Output{File: file, Content: formatted}, nil
	} else {
		return Output{File: file, Content: raw,
			Warning: "gofmt could not format the output: " + err.Error()}, nil
	}
}

@ |expandPieces| expands a list of code pieces in order. |expand| expands one
piece, threading the combined-source line through the text so \.{//line}
directives stay accurate, and following \.{@@<...@@>} references recursively
(guarding against cycles).
@(cmd/gtangle/gtangle.go@>=
func (t *Tangler) expandPieces(pieces []codePiece, o *buffer, stack []string) error {
	for _, p := range pieces {
		if err := t.expand(p.code, p.line, o, stack); err != nil {
			return err
		}
	}
	return nil
}

func (t *Tangler) expand(code string, line int, o *buffer, stack []string) error {
	for _, a := range common.ScanCode(code) {
		switch a.Kind {
		case common.AText, common.AVerbatim:
			line = o.writeText(a.Text, line)
		case common.APaste:
			o.trimRight()
			o.pasteNext = true
			o.atLineStart = false
		case common.ARef:
			name := t.w.Resolve(a.Text)
			def, ok := t.defs[name]
			if !ok {
				return fmt.Errorf("undefined section <%s>", a.Text)
			}
			if slices.Contains(stack, name) {
				return fmt.Errorf("circular reference through <%s>", name)
			}
			// Surround an expanded reference with newlines so adjacent
			// statements stay on separate lines; gofmt collapses the rest.
			o.newline()
			if err := t.expandPieces(def, o, append(stack, name)); err != nil {
				return err
			}
			o.newline()
		case common.ATeX, common.AIndex, common.ALayout, common.AIndexDef:
			// woven-output only; ignored by tangle
		}
	}
	return nil
}

@ The output |buffer| accumulates bytes. It tracks whether it is at the start of
a line so it can prefix each line with a \.{//line} directive, and it supports
the \.{@@\&} paste operation, which deletes the whitespace surrounding it.
@(cmd/gtangle/gtangle.go@>=
type buffer struct {
	t           *Tangler
	b           []byte
	pasteNext   bool
	atLineStart bool
}

func (o *buffer) writeText(s string, line int) int {
	if o.pasteNext {
		s = strings.TrimLeft(s, " \t\n\r")
		o.pasteNext = false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if o.atLineStart && c != '\n' {
			o.lineMark(line)
			o.atLineStart = false
		}
		o.b = append(o.b, c)
		if c == '\n' {
			line++
			o.atLineStart = true
		}
	}
	return line
}

func (o *buffer) lineMark(line int) {
	file, ln := o.t.w.Origin(line)
	o.b = append(o.b, fmt.Sprintf("//line %s:%d\n", file, ln)...)
}

func (o *buffer) newline() {
	o.b = append(o.b, '\n')
	o.atLineStart = true
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
@(cmd/gtangle/gtangle_test.go@>=
package main

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sjnam/gweb/common"
)

@ @(cmd/gtangle/gtangle_test.go@>=
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

@ @(cmd/gtangle/gtangle_test.go@>=
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

@ @(cmd/gtangle/gtangle_test.go@>=
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

@ @(cmd/gtangle/gtangle_test.go@>=
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

@ @(cmd/gtangle/gtangle_test.go@>=
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
	// The //line directives gtangle always emits put the expanded refinement on
	// its own line, so check that the name resolved (the host line and the value
	// both appear) rather than that they sit on one line.
	got := string(outs[0].Content)
	if !strings.Contains(got, "var area =") || !strings.Contains(got, "42") {
		t.Errorf("name containing |x| should still match for tangling:\n%s", got)
	}
}

@ @(cmd/gtangle/gtangle_test.go@>=
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

@ @(cmd/gtangle/gtangle_test.go@>=
func TestTangleDropsUnknownCode(t *testing.T) {
	// An unknown @@x must drop exactly its two characters, not corrupt the rest
	// (guards against a former double-skip bug).
	const src = "@@ x\n@@c\npackage main\n\nvar a@@?bc = 1\n"
	outs, err := New(common.ParseString(src)).Tangle("p.go")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(outs[0].Content); !strings.Contains(got, "var abc = 1") {
		t.Errorf("unknown @@x should drop exactly two chars:\n%s", got)
	}
}

@ @(cmd/gtangle/gtangle_test.go@>=
func TestTangleAbbrevAtDefinition(t *testing.T) {
	// The reference carries the full name; the definition is abbreviated.
	const src = "@@ x\n@@c\npackage main\n\nvar v = @@<the value@@>\n\n@@ d\n@@<the val...@@>=\n42\n"
	outs, err := New(common.ParseString(src)).Tangle("p.go")
	if err != nil {
		t.Fatal(err)
	}
	// As above, the always-on //line directives break the expansion onto its own
	// line, so check both halves are present rather than that they are adjacent.
	if got := string(outs[0].Content); !strings.Contains(got, "var v =") || !strings.Contains(got, "42") {
		t.Errorf("abbreviated definition should resolve:\n%s", got)
	}
}

@* Integration tests.
The tangle engine's integration tests: every example tangles to compilable \GO/.

@ @(cmd/gtangle/gtangle_test.go@>=
func importsThirdParty(content []byte) bool {
	f, err := parser.ParseFile(token.NewFileSet(), "", content, parser.ImportsOnly)
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

@ @(cmd/gtangle/gtangle_test.go@>=
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

@ @(cmd/gtangle/gtangle_test.go@>=
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
		// go 1.23 so examples may use range-over-func iterators (e.g. seq.w).
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

@ @(cmd/gtangle/gtangle_test.go@>=
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
