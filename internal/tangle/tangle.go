// Package tangle implements gtangle: it extracts compilable Go source from a
// GWEB web, expanding named-section references in program order. It is the Go
// analogue of CWEB's ctangle.
//
//line lit/tangle.w:6
//line lit/tangle.w:7
//line lit/tangle.w:8
//line lit/tangle.w:9
package tangle

//line lit/tangle.w:11
import (
//line lit/tangle.w:12
	"fmt"
//line lit/tangle.w:13
	"go/format"
//line lit/tangle.w:14
	"slices"
//line lit/tangle.w:15
	"sort"
//line lit/tangle.w:16
	"strings"

//line lit/tangle.w:18
	"github.com/sjnam/gweb/internal/web"
//line lit/tangle.w:19
)

// Output is one tangled file: its target name and Go contents. Warning is set
// (non-fatal) when gofmt could not format the assembled program.
//
//line lit/tangle.w:24
//line lit/tangle.w:25
//line lit/tangle.w:26
type Output struct {
//line lit/tangle.w:27
	File string
//line lit/tangle.w:28
	Content []byte
//line lit/tangle.w:29
	Warning string
//line lit/tangle.w:30
}

// Tangler holds the resolved code of a web, classified by destination.
//
//line lit/tangle.w:41
//line lit/tangle.w:42
type Tangler struct {
//line lit/tangle.w:43
	w *web.Web
//line lit/tangle.w:44
	defs map[string][]codePiece // canonical named-section -> code pieces
//line lit/tangle.w:45
	files map[string][]codePiece // @(file@>= name -> code pieces
//line lit/tangle.w:46
	main []codePiece // unnamed @c sections, in order
//line lit/tangle.w:47
}

// codePiece is one section's raw code together with the 1-based combined-source
// line it begins on, so tangled output can be mapped back to the .w file.
//
//line lit/tangle.w:49
//line lit/tangle.w:50
//line lit/tangle.w:51
type codePiece struct {
//line lit/tangle.w:52
	code string
//line lit/tangle.w:53
	line int
//line lit/tangle.w:54
}

// New builds a Tangler from a parsed web.
//
//line lit/tangle.w:60
//line lit/tangle.w:61
func New(w *web.Web) *Tangler {
//line lit/tangle.w:62
	t := &Tangler{
//line lit/tangle.w:63
		w: w,
//line lit/tangle.w:64
		defs: map[string][]codePiece{},
//line lit/tangle.w:65
		files: map[string][]codePiece{},
//line lit/tangle.w:66
	}
//line lit/tangle.w:67
	for _, s := range w.Sections {
//line lit/tangle.w:68
		if !s.HasCode {
//line lit/tangle.w:69
			continue
//line lit/tangle.w:70
		}
//line lit/tangle.w:71
		p := codePiece{s.Code, s.CodeLine}
//line lit/tangle.w:72
		switch {
//line lit/tangle.w:73
		case s.Name == "":
//line lit/tangle.w:74
			t.main = append(t.main, p)
//line lit/tangle.w:75
		case s.IsFile:
//line lit/tangle.w:76
			t.files[s.Name] = append(t.files[s.Name], p)
//line lit/tangle.w:77
		default:
//line lit/tangle.w:78
			name := w.Resolve(s.Name)
//line lit/tangle.w:79
			t.defs[name] = append(t.defs[name], p)
//line lit/tangle.w:80
		}
//line lit/tangle.w:81
	}
//line lit/tangle.w:82
	return t
//line lit/tangle.w:83
}

// Tangle produces all output files. defaultFile names the file that receives
// the unnamed program text (typically "<basename>.go").
//
//line lit/tangle.w:88
//line lit/tangle.w:89
//line lit/tangle.w:90
func (t *Tangler) Tangle(defaultFile string) ([]Output, error) {
//line lit/tangle.w:91
	var outs []Output

//line lit/tangle.w:93
	if nonEmpty(t.main) {
//line lit/tangle.w:94
		out, err := t.renderOutput(defaultFile, t.main)
//line lit/tangle.w:95
		if err != nil {
//line lit/tangle.w:96
			return nil, err
//line lit/tangle.w:97
		}
//line lit/tangle.w:98
		outs = append(outs, out)
//line lit/tangle.w:99
	}

//line lit/tangle.w:101
	names := make([]string, 0, len(t.files))
//line lit/tangle.w:102
	for name := range t.files {
//line lit/tangle.w:103
		names = append(names, name)
//line lit/tangle.w:104
	}
//line lit/tangle.w:105
	sort.Strings(names)
//line lit/tangle.w:106
	for _, name := range names {
//line lit/tangle.w:107
		out, err := t.renderOutput(name, t.files[name])
//line lit/tangle.w:108
		if err != nil {
//line lit/tangle.w:109
			return nil, err
//line lit/tangle.w:110
		}
//line lit/tangle.w:111
		outs = append(outs, out)
//line lit/tangle.w:112
	}

//line lit/tangle.w:114
	if len(outs) == 0 {
//line lit/tangle.w:115
		return nil, fmt.Errorf("no code to tangle (no @c or @(file@>= sections)")
//line lit/tangle.w:116
	}
//line lit/tangle.w:117
	return outs, nil
//line lit/tangle.w:118
}

// nonEmpty reports whether any piece carries non-blank code.
//
//line lit/tangle.w:123
//line lit/tangle.w:124
func nonEmpty(pieces []codePiece) bool {
//line lit/tangle.w:125
	for _, p := range pieces {
//line lit/tangle.w:126
		if strings.TrimSpace(p.code) != "" {
//line lit/tangle.w:127
			return true
//line lit/tangle.w:128
		}
//line lit/tangle.w:129
	}
//line lit/tangle.w:130
	return false
//line lit/tangle.w:131
}

// renderOutput expands a destination's code pieces and runs gofmt. A genuine web
// error (undefined or circular reference) is fatal; a gofmt failure is not: the
// unformatted Go is kept and reported via Output.Warning.
//
//line lit/tangle.w:138
//line lit/tangle.w:139
//line lit/tangle.w:140
//line lit/tangle.w:141
func (t *Tangler) renderOutput(file string, pieces []codePiece) (Output, error) {
//line lit/tangle.w:142
	o := &buffer{t: t, atLineStart: true}
//line lit/tangle.w:143
	if err := t.expandPieces(pieces, o, nil); err != nil {
//line lit/tangle.w:144
		return Output{}, err
//line lit/tangle.w:145
	}
//line lit/tangle.w:146
	raw := o.bytes()
//line lit/tangle.w:147
	if formatted, err := format.Source(raw); err == nil {
//line lit/tangle.w:148
		return Output{File: file, Content: formatted}, nil
//line lit/tangle.w:149
	} else {
//line lit/tangle.w:150
		return Output{File: file, Content: raw,
//line lit/tangle.w:151
			Warning: "gofmt could not format the output: " + err.Error()}, nil
//line lit/tangle.w:152
	}
//line lit/tangle.w:153
}

// expandPieces expands a list of code pieces in order.
//
//line lit/tangle.w:160
//line lit/tangle.w:161
func (t *Tangler) expandPieces(pieces []codePiece, o *buffer, stack []string) error {
//line lit/tangle.w:162
	for _, p := range pieces {
//line lit/tangle.w:163
		if err := t.expand(p.code, p.line, o, stack); err != nil {
//line lit/tangle.w:164
			return err
//line lit/tangle.w:165
		}
//line lit/tangle.w:166
	}
//line lit/tangle.w:167
	return nil
//line lit/tangle.w:168
}

// expand writes the expansion of one code piece into o, starting at the given
// combined-source line and following @<...@> references.
//
//line lit/tangle.w:170
//line lit/tangle.w:171
//line lit/tangle.w:172
func (t *Tangler) expand(code string, line int, o *buffer, stack []string) error {
//line lit/tangle.w:173
	for _, a := range web.ScanCode(code) {
//line lit/tangle.w:174
		switch a.Kind {
//line lit/tangle.w:175
		case web.AText, web.AVerbatim:
//line lit/tangle.w:176
			line = o.writeText(a.Text, line)
//line lit/tangle.w:177
		case web.APaste:
//line lit/tangle.w:178
			o.trimRight()
//line lit/tangle.w:179
			o.pasteNext = true
//line lit/tangle.w:180
			o.atLineStart = false
//line lit/tangle.w:181
		case web.ARef:
//line lit/tangle.w:182
			name := t.w.Resolve(a.Text)
//line lit/tangle.w:183
			def, ok := t.defs[name]
//line lit/tangle.w:184
			if !ok {
//line lit/tangle.w:185
				return fmt.Errorf("undefined section <%s>", a.Text)
//line lit/tangle.w:186
			}
//line lit/tangle.w:187
			if slices.Contains(stack, name) {
//line lit/tangle.w:188
				return fmt.Errorf("circular reference through <%s>", name)
//line lit/tangle.w:189
			}
//line lit/tangle.w:190
			// Surround an expanded reference with newlines so adjacent
//line lit/tangle.w:191
			// statements stay on separate lines; gofmt collapses the rest.
//line lit/tangle.w:192
			o.newline()
//line lit/tangle.w:193
			if err := t.expandPieces(def, o, append(stack, name)); err != nil {
//line lit/tangle.w:194
				return err
//line lit/tangle.w:195
			}
//line lit/tangle.w:196
			o.newline()
//line lit/tangle.w:197
		case web.ATeX, web.AIndex, web.ALayout, web.AIndexDef:
//line lit/tangle.w:198
			// woven-output only; ignored by tangle
//line lit/tangle.w:199
		}
//line lit/tangle.w:200
	}
//line lit/tangle.w:201
	return nil
//line lit/tangle.w:202
}

// buffer accumulates output, tracks line starts for //line directives, and
// supports the @& paste operation.
//
//line lit/tangle.w:208
//line lit/tangle.w:209
//line lit/tangle.w:210
type buffer struct {
//line lit/tangle.w:211
	t *Tangler
//line lit/tangle.w:212
	b []byte
//line lit/tangle.w:213
	pasteNext bool
//line lit/tangle.w:214
	atLineStart bool
//line lit/tangle.w:215
}

// writeText appends s, advancing the source line across newlines. It prefixes
// each output line with a //line comment mapping it back to its .w origin, and
// returns the updated source line.
//
//line lit/tangle.w:217
//line lit/tangle.w:218
//line lit/tangle.w:219
//line lit/tangle.w:220
func (o *buffer) writeText(s string, line int) int {
//line lit/tangle.w:221
	if o.pasteNext {
//line lit/tangle.w:222
		s = strings.TrimLeft(s, " \t\n\r")
//line lit/tangle.w:223
		o.pasteNext = false
//line lit/tangle.w:224
	}
//line lit/tangle.w:225
	for i := 0; i < len(s); i++ {
//line lit/tangle.w:226
		c := s[i]
//line lit/tangle.w:227
		if o.atLineStart && c != '\n' {
//line lit/tangle.w:228
			o.lineMark(line)
//line lit/tangle.w:229
			o.atLineStart = false
//line lit/tangle.w:230
		}
//line lit/tangle.w:231
		o.b = append(o.b, c)
//line lit/tangle.w:232
		if c == '\n' {
//line lit/tangle.w:233
			line++
//line lit/tangle.w:234
			o.atLineStart = true
//line lit/tangle.w:235
		}
//line lit/tangle.w:236
	}
//line lit/tangle.w:237
	return line
//line lit/tangle.w:238
}

// lineMark emits a //line directive for the given combined-source line.
//
//line lit/tangle.w:240
//line lit/tangle.w:241
func (o *buffer) lineMark(line int) {
//line lit/tangle.w:242
	file, ln := o.t.w.Origin(line)
//line lit/tangle.w:243
	o.b = append(o.b, fmt.Sprintf("//line %s:%d\n", file, ln)...)
//line lit/tangle.w:244
}

// newline starts a fresh output line (used around an expanded reference).
//
//line lit/tangle.w:246
//line lit/tangle.w:247
func (o *buffer) newline() {
//line lit/tangle.w:248
	o.b = append(o.b, '\n')
//line lit/tangle.w:249
	o.atLineStart = true
//line lit/tangle.w:250
}

//line lit/tangle.w:252
func (o *buffer) trimRight() {
//line lit/tangle.w:253
	for len(o.b) > 0 {
//line lit/tangle.w:254
		switch o.b[len(o.b)-1] {
//line lit/tangle.w:255
		case ' ', '\t', '\n', '\r':
//line lit/tangle.w:256
			o.b = o.b[:len(o.b)-1]
//line lit/tangle.w:257
		default:
//line lit/tangle.w:258
			return
//line lit/tangle.w:259
		}
//line lit/tangle.w:260
	}
//line lit/tangle.w:261
}

//line lit/tangle.w:263
func (o *buffer) bytes() []byte { return o.b }
