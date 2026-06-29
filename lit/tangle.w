@* The \.{tangle} package.
This package implements \.{gtangle}: it extracts compilable Go source from a
parsed web, expanding named-section references in program order. It is the Go
analogue of CWEB's \.{ctangle}.
@(internal/tangle/tangle.go@>=
// Package tangle implements gtangle: it extracts compilable Go source from a
// GWEB web, expanding named-section references in program order. It is the Go
// analogue of CWEB's ctangle.
package tangle

import (
	"fmt"
	"go/format"
	"slices"
	"sort"
	"strings"

	"github.com/sjnam/gweb/internal/web"
)

@ An |Output| is one tangled file: its target name and Go contents. |Warning|
is set (non-fatally) when |gofmt| could not format the assembled program.
@(internal/tangle/tangle.go@>=
// Output is one tangled file: its target name and Go contents. Warning is set
// (non-fatal) when gofmt could not format the assembled program.
type Output struct {
	File    string
	Content []byte
	Warning string
}

@ A |Tangler| holds the resolved code of a web, classified by destination: the
refinements (|defs|), the \.{@@(file@@>=} outputs, and the unnamed program text
(|main|). Each destination keeps a list of |codePiece|s rather than one joined
string, so every piece remembers the \.{.w} line it began on -- the anchor for
the \.{//line} directives. As in cweb's \.{ctangle}, those directives are always
emitted (there is no switch to suppress them), so the Go compiler, \.{go vet},
and panic traces report positions in the literate \.{.w} source rather than in
the generated \.{.go}.
@(internal/tangle/tangle.go@>=
// Tangler holds the resolved code of a web, classified by destination.
type Tangler struct {
	w     *web.Web
	defs  map[string][]codePiece // canonical named-section -> code pieces
	files map[string][]codePiece // @@(file@@>= name -> code pieces
	main  []codePiece            // unnamed @@c sections, in order
}

// codePiece is one section's raw code together with the 1-based combined-source
// line it begins on, so tangled output can be mapped back to the .w file.
type codePiece struct {
	code string
	line int
}

@ |New| classifies every code section into the unnamed program, an output file,
or a named refinement, appending each section's code -- with the source line it
began on -- to the pieces for that destination.
@(internal/tangle/tangle.go@>=
// New builds a Tangler from a parsed web.
func New(w *web.Web) *Tangler {
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
@(internal/tangle/tangle.go@>=
// Tangle produces all output files. defaultFile names the file that receives
// the unnamed program text (typically "<basename>.go").
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
@(internal/tangle/tangle.go@>=
// nonEmpty reports whether any piece carries non-blank code.
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
|gofmt| failure is not -- the unformatted Go is kept and reported via
|Output.Warning|.
@(internal/tangle/tangle.go@>=
// renderOutput expands a destination's code pieces and runs gofmt. A genuine web
// error (undefined or circular reference) is fatal; a gofmt failure is not: the
// unformatted Go is kept and reported via Output.Warning.
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
@(internal/tangle/tangle.go@>=
// expandPieces expands a list of code pieces in order.
func (t *Tangler) expandPieces(pieces []codePiece, o *buffer, stack []string) error {
	for _, p := range pieces {
		if err := t.expand(p.code, p.line, o, stack); err != nil {
			return err
		}
	}
	return nil
}

// expand writes the expansion of one code piece into o, starting at the given
// combined-source line and following @@<...@@> references.
func (t *Tangler) expand(code string, line int, o *buffer, stack []string) error {
	for _, a := range web.ScanCode(code) {
		switch a.Kind {
		case web.AText, web.AVerbatim:
			line = o.writeText(a.Text, line)
		case web.APaste:
			o.trimRight()
			o.pasteNext = true
			o.atLineStart = false
		case web.ARef:
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
		case web.ATeX, web.AIndex, web.ALayout, web.AIndexDef:
			// woven-output only; ignored by tangle
		}
	}
	return nil
}

@ The output |buffer| accumulates bytes. It tracks whether it is at the start of
a line so it can prefix each line with a \.{//line} directive, and it supports
the \.{@@\&} paste operation, which deletes the whitespace surrounding it.
@(internal/tangle/tangle.go@>=
// buffer accumulates output, tracks line starts for //line directives, and
// supports the @@& paste operation.
type buffer struct {
	t           *Tangler
	b           []byte
	pasteNext   bool
	atLineStart bool
}

// writeText appends s, advancing the source line across newlines. It prefixes
// each output line with a //line comment mapping it back to its .w origin, and
// returns the updated source line.
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

// lineMark emits a //line directive for the given combined-source line.
func (o *buffer) lineMark(line int) {
	file, ln := o.t.w.Origin(line)
	o.b = append(o.b, fmt.Sprintf("//line %s:%d\n", file, ln)...)
}

// newline starts a fresh output line (used around an expanded reference).
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
