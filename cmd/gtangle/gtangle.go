//line cmd/gtangle/gtangle.w:19
package main

import (
	"flag"
	"fmt"
	"go/format"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/sjnam/gweb/common"
)

//line cmd/gtangle/gtangle.w:46
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

//line cmd/gtangle/gtangle.w:68
func usage() {
	fmt.Fprintln(os.Stderr, "usage: gtangle [-o dir] file[.w] [change[.ch]]")
	flag.PrintDefaults()
}

//line cmd/gtangle/gtangle.w:77
func reportProgress(w *common.Web) {
	for _, s := range w.Sections {
		if s.Starred {
			fmt.Fprintf(os.Stderr, "*%d", s.Number)
		}
	}
	fmt.Fprintln(os.Stderr)
}

//line cmd/gtangle/gtangle.w:91
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

//line cmd/gtangle/gtangle.w:122
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

//line cmd/gtangle/gtangle.w:115
	return nil
}

//line cmd/gtangle/gtangle.w:157
type Output struct {
	File    string
	Content []byte
	Warning string
}

//line cmd/gtangle/gtangle.w:175
type Tangler struct {
	w     *common.Web
	defs  map[string][]codePiece // canonical named-section $\rightarrow$ code pieces
	files map[string][]codePiece // \.{@(file@>=name} $\rightarrow$ code pieces
	main  []codePiece            // unnamed \.{@c} sections, in order
}

type codePiece struct {
	code string
	line int
}

//line cmd/gtangle/gtangle.w:191
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

//line cmd/gtangle/gtangle.w:218
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
		return nil, fmt.Errorf("no code to tangle (no @c or @(file@>= sections)")
	}
	return outs, nil
}

//line cmd/gtangle/gtangle.w:251
func nonEmpty(pieces []codePiece) bool {
	for _, p := range pieces {
		if strings.TrimSpace(p.code) != "" {
			return true
		}
	}
	return false
}

//line cmd/gtangle/gtangle.w:274
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

//line cmd/gtangle/gtangle.w:314
func thinLineMarks(src []byte) []byte {
	marks, ok := lineMarkLines(src)
	if !ok || len(marks) == 0 {
		return src
	}

//line cmd/gtangle/gtangle.w:360
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

//line cmd/gtangle/gtangle.w:320
}

//line cmd/gtangle/gtangle.w:325
func lineMarkLines(src []byte) (map[int]bool, bool) {
	fset := token.NewFileSet()
	f := fset.AddFile("", fset.Base(), len(src))
	var s scanner.Scanner
	bad := false
	s.Init(f, src, func(token.Position, string) { bad = true }, scanner.ScanComments)
	marks := map[int]bool{}
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok != token.COMMENT || !strings.HasPrefix(lit, "//line ") {
			continue
		}
		if p := f.PositionFor(pos, false); p.Column == 1 {
			marks[p.Line] = true
		}
	}
	return marks, !bad
}

//line cmd/gtangle/gtangle.w:388
func blankLine(s string) bool { return strings.TrimSpace(s) == "" }

func betweenBlanks(prevBlank bool, lines []string, i int) bool {
	return prevBlank && i+1 < len(lines) && blankLine(lines[i+1])
}

//line cmd/gtangle/gtangle.w:397
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

//line cmd/gtangle/gtangle.w:413
func (t *Tangler) expandPieces(pieces []codePiece, o *buffer, stack []string) error {
	for _, p := range pieces {
		if err := t.expand(p.code, p.line, o, stack); err != nil {
			return err
		}
	}
	return nil
}

//line cmd/gtangle/gtangle.w:427
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

//line cmd/gtangle/gtangle.w:449
			name := t.w.Resolve(a.Text)
			def, ok := t.defs[name]
			if !ok {
				return fmt.Errorf("undefined section <%s>", a.Text)
			}
			if slices.Contains(stack, name) {
				return fmt.Errorf("circular reference through <%s>", name)
			}
			o.newline()
			if err := t.expandPieces(def, o, append(stack, name)); err != nil {
				return err
			}
			o.newline()

//line cmd/gtangle/gtangle.w:438
		case common.ATeX, common.AIndex, common.ALayout, common.AIndexDef:
			// woven-output only; ignored by tangle
		}
	}
	return nil
}

//line cmd/gtangle/gtangle.w:470
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

//line cmd/gtangle/gtangle.w:487
type goLex int

const (
	lexCode goLex = iota
	lexLineComment
	lexBlockComment
	lexQuote
	lexRune
	lexRaw

//line cmd/gtangle/gtangle.w:496
)

func (l goLex) spansLines() bool { return l == lexRaw || l == lexBlockComment }

//line cmd/gtangle/gtangle.w:505
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

//line cmd/gtangle/gtangle.w:532
func (o *buffer) track(c byte) {
	switch o.lex {
	case lexCode:

//line cmd/gtangle/gtangle.w:557
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

//line cmd/gtangle/gtangle.w:536
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

//line cmd/gtangle/gtangle.w:581
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

//line cmd/gtangle/gtangle.w:547
	case lexRaw:
		if c == '`' {
			o.lex = lexCode
		}
	}
}

//line cmd/gtangle/gtangle.w:597
func (o *buffer) lineMark(line int) {
	file, ln := o.t.w.Origin(line)
	o.b = append(o.b, fmt.Sprintf("//line %s:%d\n", file, ln)...)
}

func (o *buffer) newline() {
	o.b = append(o.b, '\n')
	o.track('\n')
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
