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

@ A |Tangler| holds the resolved named-section definitions for a web: the
refinements (|defs|), the \.{@@(file@@>=} outputs, and the concatenated unnamed
program text.
@(internal/tangle/tangle.go@>=
// Tangler holds the resolved named-section definitions for a web.
type Tangler struct {
	w     *web.Web
	defs  map[string]string // canonical named-section -> concatenated code
	files map[string]string // @@(file@@>= name -> concatenated code
	main  string            // concatenation of all unnamed @@c sections
}

@ |New| classifies every code section into the unnamed program, an output file,
or a named refinement, concatenating sections that share a target.
@(internal/tangle/tangle.go@>=
// New builds a Tangler from a parsed web.
func New(w *web.Web) *Tangler {
	t := &Tangler{
		w:     w,
		defs:  map[string]string{},
		files: map[string]string{},
	}
	var mainParts []string
	for _, s := range w.Sections {
		if !s.HasCode {
			continue
		}
		switch {
		case s.Name == "":
			mainParts = append(mainParts, s.Code)
		case s.IsFile:
			t.files[s.Name] += s.Code
		default:
			name := w.Resolve(s.Name)
			t.defs[name] += s.Code
		}
	}
	t.main = strings.Join(mainParts, "")
	return t
}

@ |Tangle| produces all output files: first the unnamed program (written to
|defaultFile|), then each \.{@@(file@@>=} target in sorted order.
@(internal/tangle/tangle.go@>=
// Tangle produces all output files. defaultFile names the file that receives
// the unnamed program text (typically "<basename>.go").
func (t *Tangler) Tangle(defaultFile string) ([]Output, error) {
	var outs []Output

	if strings.TrimSpace(t.main) != "" {
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

@ |renderOutput| expands one root code part and runs |gofmt| on the result. A
genuine web error (an undefined or circular reference) is fatal; a |gofmt|
failure is not -- the unformatted Go is kept and reported via |Output.Warning|.
@(internal/tangle/tangle.go@>=
// renderOutput expands a root code part and runs gofmt on the result. A
// genuine web error (undefined or circular reference) is fatal; a gofmt failure
// is non-fatal: the unformatted Go is kept and reported via Output.Warning.
func (t *Tangler) renderOutput(file, code string) (Output, error) {
	var o buffer
	if err := t.expand(code, &o, nil); err != nil {
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

@ |expand| writes the expansion of a code part into the buffer, following
\.{@@<...@@>} references recursively and guarding against cycles.
@(internal/tangle/tangle.go@>=
// expand writes the expansion of code into o, following @@<...@@> references.
func (t *Tangler) expand(code string, o *buffer, stack []string) error {
	for _, a := range web.ScanCode(code) {
		switch a.Kind {
		case web.AText:
			o.writeMaybeTrimLeft(a.Text)
		case web.AVerbatim:
			o.writeMaybeTrimLeft(a.Text)
		case web.APaste:
			o.trimRight()
			o.pasteNext = true
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
			o.WriteString("\n")
			if err := t.expand(def, o, append(stack, name)); err != nil {
				return err
			}
			o.WriteString("\n")
		case web.ATeX, web.AIndex, web.ALayout, web.AIndexDef:
			// woven-output only; ignored by tangle
		}
	}
	return nil
}

@ The output |buffer| accumulates bytes and supports the \.{@@\&} paste operation,
which deletes the whitespace surrounding it.
@(internal/tangle/tangle.go@>=
// buffer accumulates output and supports the @@& paste operation.
type buffer struct {
	b         []byte
	pasteNext bool
}

func (o *buffer) WriteString(s string) { o.b = append(o.b, s...) }

func (o *buffer) writeMaybeTrimLeft(s string) {
	if o.pasteNext {
		s = strings.TrimLeft(s, " \t\n\r")
		o.pasteNext = false
	}
	o.b = append(o.b, s...)
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
