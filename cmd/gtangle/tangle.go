//line cmd/gtangle/gtangle.w:128
package main

//line cmd/gtangle/gtangle.w:130
import (
//line cmd/gtangle/gtangle.w:131
	"fmt"
//line cmd/gtangle/gtangle.w:132
	"go/format"
//line cmd/gtangle/gtangle.w:133
	"slices"
//line cmd/gtangle/gtangle.w:134
	"sort"
//line cmd/gtangle/gtangle.w:135
	"strings"

//line cmd/gtangle/gtangle.w:137
	"github.com/sjnam/gweb/internal/web"
//line cmd/gtangle/gtangle.w:138
)

// Output is one tangled file: its target name and Go contents. Warning is set
// (non-fatal) when gofmt could not format the assembled program.
//
//line cmd/gtangle/gtangle.w:143
//line cmd/gtangle/gtangle.w:144
//line cmd/gtangle/gtangle.w:145
type Output struct {
//line cmd/gtangle/gtangle.w:146
	File string
//line cmd/gtangle/gtangle.w:147
	Content []byte
//line cmd/gtangle/gtangle.w:148
	Warning string
//line cmd/gtangle/gtangle.w:149
}

// Tangler holds the resolved code of a web, classified by destination.
//
//line cmd/gtangle/gtangle.w:160
//line cmd/gtangle/gtangle.w:161
type Tangler struct {
//line cmd/gtangle/gtangle.w:162
	w *web.Web
//line cmd/gtangle/gtangle.w:163
	defs map[string][]codePiece // canonical named-section -> code pieces
//line cmd/gtangle/gtangle.w:164
	files map[string][]codePiece // @(file@>= name -> code pieces
//line cmd/gtangle/gtangle.w:165
	main []codePiece // unnamed @c sections, in order
//line cmd/gtangle/gtangle.w:166
}

// codePiece is one section's raw code together with the 1-based combined-source
// line it begins on, so tangled output can be mapped back to the .w file.
//
//line cmd/gtangle/gtangle.w:168
//line cmd/gtangle/gtangle.w:169
//line cmd/gtangle/gtangle.w:170
type codePiece struct {
//line cmd/gtangle/gtangle.w:171
	code string
//line cmd/gtangle/gtangle.w:172
	line int
//line cmd/gtangle/gtangle.w:173
}

// New builds a Tangler from a parsed web.
//
//line cmd/gtangle/gtangle.w:179
//line cmd/gtangle/gtangle.w:180
func New(w *web.Web) *Tangler {
//line cmd/gtangle/gtangle.w:181
	t := &Tangler{
//line cmd/gtangle/gtangle.w:182
		w: w,
//line cmd/gtangle/gtangle.w:183
		defs: map[string][]codePiece{},
//line cmd/gtangle/gtangle.w:184
		files: map[string][]codePiece{},
//line cmd/gtangle/gtangle.w:185
	}
//line cmd/gtangle/gtangle.w:186
	for _, s := range w.Sections {
//line cmd/gtangle/gtangle.w:187
		if !s.HasCode {
//line cmd/gtangle/gtangle.w:188
			continue
//line cmd/gtangle/gtangle.w:189
		}
//line cmd/gtangle/gtangle.w:190
		p := codePiece{s.Code, s.CodeLine}
//line cmd/gtangle/gtangle.w:191
		switch {
//line cmd/gtangle/gtangle.w:192
		case s.Name == "":
//line cmd/gtangle/gtangle.w:193
			t.main = append(t.main, p)
//line cmd/gtangle/gtangle.w:194
		case s.IsFile:
//line cmd/gtangle/gtangle.w:195
			t.files[s.Name] = append(t.files[s.Name], p)
//line cmd/gtangle/gtangle.w:196
		default:
//line cmd/gtangle/gtangle.w:197
			name := w.Resolve(s.Name)
//line cmd/gtangle/gtangle.w:198
			t.defs[name] = append(t.defs[name], p)
//line cmd/gtangle/gtangle.w:199
		}
//line cmd/gtangle/gtangle.w:200
	}
//line cmd/gtangle/gtangle.w:201
	return t
//line cmd/gtangle/gtangle.w:202
}

// Tangle produces all output files. defaultFile names the file that receives
// the unnamed program text (typically "<basename>.go").
//
//line cmd/gtangle/gtangle.w:207
//line cmd/gtangle/gtangle.w:208
//line cmd/gtangle/gtangle.w:209
func (t *Tangler) Tangle(defaultFile string) ([]Output, error) {
//line cmd/gtangle/gtangle.w:210
	var outs []Output

//line cmd/gtangle/gtangle.w:212
	if nonEmpty(t.main) {
//line cmd/gtangle/gtangle.w:213
		out, err := t.renderOutput(defaultFile, t.main)
//line cmd/gtangle/gtangle.w:214
		if err != nil {
//line cmd/gtangle/gtangle.w:215
			return nil, err
//line cmd/gtangle/gtangle.w:216
		}
//line cmd/gtangle/gtangle.w:217
		outs = append(outs, out)
//line cmd/gtangle/gtangle.w:218
	}

//line cmd/gtangle/gtangle.w:220
	names := make([]string, 0, len(t.files))
//line cmd/gtangle/gtangle.w:221
	for name := range t.files {
//line cmd/gtangle/gtangle.w:222
		names = append(names, name)
//line cmd/gtangle/gtangle.w:223
	}
//line cmd/gtangle/gtangle.w:224
	sort.Strings(names)
//line cmd/gtangle/gtangle.w:225
	for _, name := range names {
//line cmd/gtangle/gtangle.w:226
		out, err := t.renderOutput(name, t.files[name])
//line cmd/gtangle/gtangle.w:227
		if err != nil {
//line cmd/gtangle/gtangle.w:228
			return nil, err
//line cmd/gtangle/gtangle.w:229
		}
//line cmd/gtangle/gtangle.w:230
		outs = append(outs, out)
//line cmd/gtangle/gtangle.w:231
	}

//line cmd/gtangle/gtangle.w:233
	if len(outs) == 0 {
//line cmd/gtangle/gtangle.w:234
		return nil, fmt.Errorf("no code to tangle (no @c or @(file@>= sections)")
//line cmd/gtangle/gtangle.w:235
	}
//line cmd/gtangle/gtangle.w:236
	return outs, nil
//line cmd/gtangle/gtangle.w:237
}

// nonEmpty reports whether any piece carries non-blank code.
//
//line cmd/gtangle/gtangle.w:242
//line cmd/gtangle/gtangle.w:243
func nonEmpty(pieces []codePiece) bool {
//line cmd/gtangle/gtangle.w:244
	for _, p := range pieces {
//line cmd/gtangle/gtangle.w:245
		if strings.TrimSpace(p.code) != "" {
//line cmd/gtangle/gtangle.w:246
			return true
//line cmd/gtangle/gtangle.w:247
		}
//line cmd/gtangle/gtangle.w:248
	}
//line cmd/gtangle/gtangle.w:249
	return false
//line cmd/gtangle/gtangle.w:250
}

// renderOutput expands a destination's code pieces and runs gofmt. A genuine web
// error (undefined or circular reference) is fatal; a gofmt failure is not: the
// unformatted Go is kept and reported via Output.Warning.
//
//line cmd/gtangle/gtangle.w:257
//line cmd/gtangle/gtangle.w:258
//line cmd/gtangle/gtangle.w:259
//line cmd/gtangle/gtangle.w:260
func (t *Tangler) renderOutput(file string, pieces []codePiece) (Output, error) {
//line cmd/gtangle/gtangle.w:261
	o := &buffer{t: t, atLineStart: true}
//line cmd/gtangle/gtangle.w:262
	if err := t.expandPieces(pieces, o, nil); err != nil {
//line cmd/gtangle/gtangle.w:263
		return Output{}, err
//line cmd/gtangle/gtangle.w:264
	}
//line cmd/gtangle/gtangle.w:265
	raw := o.bytes()
//line cmd/gtangle/gtangle.w:266
	if formatted, err := format.Source(raw); err == nil {
//line cmd/gtangle/gtangle.w:267
		return Output{File: file, Content: formatted}, nil
//line cmd/gtangle/gtangle.w:268
	} else {
//line cmd/gtangle/gtangle.w:269
		return Output{File: file, Content: raw,
//line cmd/gtangle/gtangle.w:270
			Warning: "gofmt could not format the output: " + err.Error()}, nil
//line cmd/gtangle/gtangle.w:271
	}
//line cmd/gtangle/gtangle.w:272
}

// expandPieces expands a list of code pieces in order.
//
//line cmd/gtangle/gtangle.w:279
//line cmd/gtangle/gtangle.w:280
func (t *Tangler) expandPieces(pieces []codePiece, o *buffer, stack []string) error {
//line cmd/gtangle/gtangle.w:281
	for _, p := range pieces {
//line cmd/gtangle/gtangle.w:282
		if err := t.expand(p.code, p.line, o, stack); err != nil {
//line cmd/gtangle/gtangle.w:283
			return err
//line cmd/gtangle/gtangle.w:284
		}
//line cmd/gtangle/gtangle.w:285
	}
//line cmd/gtangle/gtangle.w:286
	return nil
//line cmd/gtangle/gtangle.w:287
}

// expand writes the expansion of one code piece into o, starting at the given
// combined-source line and following @<...@> references.
//
//line cmd/gtangle/gtangle.w:289
//line cmd/gtangle/gtangle.w:290
//line cmd/gtangle/gtangle.w:291
func (t *Tangler) expand(code string, line int, o *buffer, stack []string) error {
//line cmd/gtangle/gtangle.w:292
	for _, a := range web.ScanCode(code) {
//line cmd/gtangle/gtangle.w:293
		switch a.Kind {
//line cmd/gtangle/gtangle.w:294
		case web.AText, web.AVerbatim:
//line cmd/gtangle/gtangle.w:295
			line = o.writeText(a.Text, line)
//line cmd/gtangle/gtangle.w:296
		case web.APaste:
//line cmd/gtangle/gtangle.w:297
			o.trimRight()
//line cmd/gtangle/gtangle.w:298
			o.pasteNext = true
//line cmd/gtangle/gtangle.w:299
			o.atLineStart = false
//line cmd/gtangle/gtangle.w:300
		case web.ARef:
//line cmd/gtangle/gtangle.w:301
			name := t.w.Resolve(a.Text)
//line cmd/gtangle/gtangle.w:302
			def, ok := t.defs[name]
//line cmd/gtangle/gtangle.w:303
			if !ok {
//line cmd/gtangle/gtangle.w:304
				return fmt.Errorf("undefined section <%s>", a.Text)
//line cmd/gtangle/gtangle.w:305
			}
//line cmd/gtangle/gtangle.w:306
			if slices.Contains(stack, name) {
//line cmd/gtangle/gtangle.w:307
				return fmt.Errorf("circular reference through <%s>", name)
//line cmd/gtangle/gtangle.w:308
			}
//line cmd/gtangle/gtangle.w:309
			// Surround an expanded reference with newlines so adjacent
//line cmd/gtangle/gtangle.w:310
			// statements stay on separate lines; gofmt collapses the rest.
//line cmd/gtangle/gtangle.w:311
			o.newline()
//line cmd/gtangle/gtangle.w:312
			if err := t.expandPieces(def, o, append(stack, name)); err != nil {
//line cmd/gtangle/gtangle.w:313
				return err
//line cmd/gtangle/gtangle.w:314
			}
//line cmd/gtangle/gtangle.w:315
			o.newline()
//line cmd/gtangle/gtangle.w:316
		case web.ATeX, web.AIndex, web.ALayout, web.AIndexDef:
//line cmd/gtangle/gtangle.w:317
			// woven-output only; ignored by tangle
//line cmd/gtangle/gtangle.w:318
		}
//line cmd/gtangle/gtangle.w:319
	}
//line cmd/gtangle/gtangle.w:320
	return nil
//line cmd/gtangle/gtangle.w:321
}

// buffer accumulates output, tracks line starts for //line directives, and
// supports the @& paste operation.
//
//line cmd/gtangle/gtangle.w:327
//line cmd/gtangle/gtangle.w:328
//line cmd/gtangle/gtangle.w:329
type buffer struct {
//line cmd/gtangle/gtangle.w:330
	t *Tangler
//line cmd/gtangle/gtangle.w:331
	b []byte
//line cmd/gtangle/gtangle.w:332
	pasteNext bool
//line cmd/gtangle/gtangle.w:333
	atLineStart bool
//line cmd/gtangle/gtangle.w:334
}

// writeText appends s, advancing the source line across newlines. It prefixes
// each output line with a //line comment mapping it back to its .w origin, and
// returns the updated source line.
//
//line cmd/gtangle/gtangle.w:336
//line cmd/gtangle/gtangle.w:337
//line cmd/gtangle/gtangle.w:338
//line cmd/gtangle/gtangle.w:339
func (o *buffer) writeText(s string, line int) int {
//line cmd/gtangle/gtangle.w:340
	if o.pasteNext {
//line cmd/gtangle/gtangle.w:341
		s = strings.TrimLeft(s, " \t\n\r")
//line cmd/gtangle/gtangle.w:342
		o.pasteNext = false
//line cmd/gtangle/gtangle.w:343
	}
//line cmd/gtangle/gtangle.w:344
	for i := 0; i < len(s); i++ {
//line cmd/gtangle/gtangle.w:345
		c := s[i]
//line cmd/gtangle/gtangle.w:346
		if o.atLineStart && c != '\n' {
//line cmd/gtangle/gtangle.w:347
			o.lineMark(line)
//line cmd/gtangle/gtangle.w:348
			o.atLineStart = false
//line cmd/gtangle/gtangle.w:349
		}
//line cmd/gtangle/gtangle.w:350
		o.b = append(o.b, c)
//line cmd/gtangle/gtangle.w:351
		if c == '\n' {
//line cmd/gtangle/gtangle.w:352
			line++
//line cmd/gtangle/gtangle.w:353
			o.atLineStart = true
//line cmd/gtangle/gtangle.w:354
		}
//line cmd/gtangle/gtangle.w:355
	}
//line cmd/gtangle/gtangle.w:356
	return line
//line cmd/gtangle/gtangle.w:357
}

// lineMark emits a //line directive for the given combined-source line.
//
//line cmd/gtangle/gtangle.w:359
//line cmd/gtangle/gtangle.w:360
func (o *buffer) lineMark(line int) {
//line cmd/gtangle/gtangle.w:361
	file, ln := o.t.w.Origin(line)
//line cmd/gtangle/gtangle.w:362
	o.b = append(o.b, fmt.Sprintf("//line %s:%d\n", file, ln)...)
//line cmd/gtangle/gtangle.w:363
}

// newline starts a fresh output line (used around an expanded reference).
//
//line cmd/gtangle/gtangle.w:365
//line cmd/gtangle/gtangle.w:366
func (o *buffer) newline() {
//line cmd/gtangle/gtangle.w:367
	o.b = append(o.b, '\n')
//line cmd/gtangle/gtangle.w:368
	o.atLineStart = true
//line cmd/gtangle/gtangle.w:369
}

//line cmd/gtangle/gtangle.w:371
func (o *buffer) trimRight() {
//line cmd/gtangle/gtangle.w:372
	for len(o.b) > 0 {
//line cmd/gtangle/gtangle.w:373
		switch o.b[len(o.b)-1] {
//line cmd/gtangle/gtangle.w:374
		case ' ', '\t', '\n', '\r':
//line cmd/gtangle/gtangle.w:375
			o.b = o.b[:len(o.b)-1]
//line cmd/gtangle/gtangle.w:376
		default:
//line cmd/gtangle/gtangle.w:377
			return
//line cmd/gtangle/gtangle.w:378
		}
//line cmd/gtangle/gtangle.w:379
	}
//line cmd/gtangle/gtangle.w:380
}

//line cmd/gtangle/gtangle.w:382
func (o *buffer) bytes() []byte { return o.b }
