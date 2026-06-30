//line lit/gtangle.w:128
package main

//line lit/gtangle.w:130
import (
//line lit/gtangle.w:131
	"fmt"
//line lit/gtangle.w:132
	"go/format"
//line lit/gtangle.w:133
	"slices"
//line lit/gtangle.w:134
	"sort"
//line lit/gtangle.w:135
	"strings"

//line lit/gtangle.w:137
	"github.com/sjnam/gweb/internal/web"
//line lit/gtangle.w:138
)

// Output is one tangled file: its target name and Go contents. Warning is set
// (non-fatal) when gofmt could not format the assembled program.
//
//line lit/gtangle.w:143
//line lit/gtangle.w:144
//line lit/gtangle.w:145
type Output struct {
//line lit/gtangle.w:146
	File string
//line lit/gtangle.w:147
	Content []byte
//line lit/gtangle.w:148
	Warning string
//line lit/gtangle.w:149
}

// Tangler holds the resolved code of a web, classified by destination.
//
//line lit/gtangle.w:160
//line lit/gtangle.w:161
type Tangler struct {
//line lit/gtangle.w:162
	w *web.Web
//line lit/gtangle.w:163
	defs map[string][]codePiece // canonical named-section -> code pieces
//line lit/gtangle.w:164
	files map[string][]codePiece // @(file@>= name -> code pieces
//line lit/gtangle.w:165
	main []codePiece // unnamed @c sections, in order
//line lit/gtangle.w:166
}

// codePiece is one section's raw code together with the 1-based combined-source
// line it begins on, so tangled output can be mapped back to the .w file.
//
//line lit/gtangle.w:168
//line lit/gtangle.w:169
//line lit/gtangle.w:170
type codePiece struct {
//line lit/gtangle.w:171
	code string
//line lit/gtangle.w:172
	line int
//line lit/gtangle.w:173
}

// New builds a Tangler from a parsed web.
//
//line lit/gtangle.w:179
//line lit/gtangle.w:180
func New(w *web.Web) *Tangler {
//line lit/gtangle.w:181
	t := &Tangler{
//line lit/gtangle.w:182
		w: w,
//line lit/gtangle.w:183
		defs: map[string][]codePiece{},
//line lit/gtangle.w:184
		files: map[string][]codePiece{},
//line lit/gtangle.w:185
	}
//line lit/gtangle.w:186
	for _, s := range w.Sections {
//line lit/gtangle.w:187
		if !s.HasCode {
//line lit/gtangle.w:188
			continue
//line lit/gtangle.w:189
		}
//line lit/gtangle.w:190
		p := codePiece{s.Code, s.CodeLine}
//line lit/gtangle.w:191
		switch {
//line lit/gtangle.w:192
		case s.Name == "":
//line lit/gtangle.w:193
			t.main = append(t.main, p)
//line lit/gtangle.w:194
		case s.IsFile:
//line lit/gtangle.w:195
			t.files[s.Name] = append(t.files[s.Name], p)
//line lit/gtangle.w:196
		default:
//line lit/gtangle.w:197
			name := w.Resolve(s.Name)
//line lit/gtangle.w:198
			t.defs[name] = append(t.defs[name], p)
//line lit/gtangle.w:199
		}
//line lit/gtangle.w:200
	}
//line lit/gtangle.w:201
	return t
//line lit/gtangle.w:202
}

// Tangle produces all output files. defaultFile names the file that receives
// the unnamed program text (typically "<basename>.go").
//
//line lit/gtangle.w:207
//line lit/gtangle.w:208
//line lit/gtangle.w:209
func (t *Tangler) Tangle(defaultFile string) ([]Output, error) {
//line lit/gtangle.w:210
	var outs []Output

//line lit/gtangle.w:212
	if nonEmpty(t.main) {
//line lit/gtangle.w:213
		out, err := t.renderOutput(defaultFile, t.main)
//line lit/gtangle.w:214
		if err != nil {
//line lit/gtangle.w:215
			return nil, err
//line lit/gtangle.w:216
		}
//line lit/gtangle.w:217
		outs = append(outs, out)
//line lit/gtangle.w:218
	}

//line lit/gtangle.w:220
	names := make([]string, 0, len(t.files))
//line lit/gtangle.w:221
	for name := range t.files {
//line lit/gtangle.w:222
		names = append(names, name)
//line lit/gtangle.w:223
	}
//line lit/gtangle.w:224
	sort.Strings(names)
//line lit/gtangle.w:225
	for _, name := range names {
//line lit/gtangle.w:226
		out, err := t.renderOutput(name, t.files[name])
//line lit/gtangle.w:227
		if err != nil {
//line lit/gtangle.w:228
			return nil, err
//line lit/gtangle.w:229
		}
//line lit/gtangle.w:230
		outs = append(outs, out)
//line lit/gtangle.w:231
	}

//line lit/gtangle.w:233
	if len(outs) == 0 {
//line lit/gtangle.w:234
		return nil, fmt.Errorf("no code to tangle (no @c or @(file@>= sections)")
//line lit/gtangle.w:235
	}
//line lit/gtangle.w:236
	return outs, nil
//line lit/gtangle.w:237
}

// nonEmpty reports whether any piece carries non-blank code.
//
//line lit/gtangle.w:242
//line lit/gtangle.w:243
func nonEmpty(pieces []codePiece) bool {
//line lit/gtangle.w:244
	for _, p := range pieces {
//line lit/gtangle.w:245
		if strings.TrimSpace(p.code) != "" {
//line lit/gtangle.w:246
			return true
//line lit/gtangle.w:247
		}
//line lit/gtangle.w:248
	}
//line lit/gtangle.w:249
	return false
//line lit/gtangle.w:250
}

// renderOutput expands a destination's code pieces and runs gofmt. A genuine web
// error (undefined or circular reference) is fatal; a gofmt failure is not: the
// unformatted Go is kept and reported via Output.Warning.
//
//line lit/gtangle.w:257
//line lit/gtangle.w:258
//line lit/gtangle.w:259
//line lit/gtangle.w:260
func (t *Tangler) renderOutput(file string, pieces []codePiece) (Output, error) {
//line lit/gtangle.w:261
	o := &buffer{t: t, atLineStart: true}
//line lit/gtangle.w:262
	if err := t.expandPieces(pieces, o, nil); err != nil {
//line lit/gtangle.w:263
		return Output{}, err
//line lit/gtangle.w:264
	}
//line lit/gtangle.w:265
	raw := o.bytes()
//line lit/gtangle.w:266
	if formatted, err := format.Source(raw); err == nil {
//line lit/gtangle.w:267
		return Output{File: file, Content: formatted}, nil
//line lit/gtangle.w:268
	} else {
//line lit/gtangle.w:269
		return Output{File: file, Content: raw,
//line lit/gtangle.w:270
			Warning: "gofmt could not format the output: " + err.Error()}, nil
//line lit/gtangle.w:271
	}
//line lit/gtangle.w:272
}

// expandPieces expands a list of code pieces in order.
//
//line lit/gtangle.w:279
//line lit/gtangle.w:280
func (t *Tangler) expandPieces(pieces []codePiece, o *buffer, stack []string) error {
//line lit/gtangle.w:281
	for _, p := range pieces {
//line lit/gtangle.w:282
		if err := t.expand(p.code, p.line, o, stack); err != nil {
//line lit/gtangle.w:283
			return err
//line lit/gtangle.w:284
		}
//line lit/gtangle.w:285
	}
//line lit/gtangle.w:286
	return nil
//line lit/gtangle.w:287
}

// expand writes the expansion of one code piece into o, starting at the given
// combined-source line and following @<...@> references.
//
//line lit/gtangle.w:289
//line lit/gtangle.w:290
//line lit/gtangle.w:291
func (t *Tangler) expand(code string, line int, o *buffer, stack []string) error {
//line lit/gtangle.w:292
	for _, a := range web.ScanCode(code) {
//line lit/gtangle.w:293
		switch a.Kind {
//line lit/gtangle.w:294
		case web.AText, web.AVerbatim:
//line lit/gtangle.w:295
			line = o.writeText(a.Text, line)
//line lit/gtangle.w:296
		case web.APaste:
//line lit/gtangle.w:297
			o.trimRight()
//line lit/gtangle.w:298
			o.pasteNext = true
//line lit/gtangle.w:299
			o.atLineStart = false
//line lit/gtangle.w:300
		case web.ARef:
//line lit/gtangle.w:301
			name := t.w.Resolve(a.Text)
//line lit/gtangle.w:302
			def, ok := t.defs[name]
//line lit/gtangle.w:303
			if !ok {
//line lit/gtangle.w:304
				return fmt.Errorf("undefined section <%s>", a.Text)
//line lit/gtangle.w:305
			}
//line lit/gtangle.w:306
			if slices.Contains(stack, name) {
//line lit/gtangle.w:307
				return fmt.Errorf("circular reference through <%s>", name)
//line lit/gtangle.w:308
			}
//line lit/gtangle.w:309
			// Surround an expanded reference with newlines so adjacent
//line lit/gtangle.w:310
			// statements stay on separate lines; gofmt collapses the rest.
//line lit/gtangle.w:311
			o.newline()
//line lit/gtangle.w:312
			if err := t.expandPieces(def, o, append(stack, name)); err != nil {
//line lit/gtangle.w:313
				return err
//line lit/gtangle.w:314
			}
//line lit/gtangle.w:315
			o.newline()
//line lit/gtangle.w:316
		case web.ATeX, web.AIndex, web.ALayout, web.AIndexDef:
//line lit/gtangle.w:317
			// woven-output only; ignored by tangle
//line lit/gtangle.w:318
		}
//line lit/gtangle.w:319
	}
//line lit/gtangle.w:320
	return nil
//line lit/gtangle.w:321
}

// buffer accumulates output, tracks line starts for //line directives, and
// supports the @& paste operation.
//
//line lit/gtangle.w:327
//line lit/gtangle.w:328
//line lit/gtangle.w:329
type buffer struct {
//line lit/gtangle.w:330
	t *Tangler
//line lit/gtangle.w:331
	b []byte
//line lit/gtangle.w:332
	pasteNext bool
//line lit/gtangle.w:333
	atLineStart bool
//line lit/gtangle.w:334
}

// writeText appends s, advancing the source line across newlines. It prefixes
// each output line with a //line comment mapping it back to its .w origin, and
// returns the updated source line.
//
//line lit/gtangle.w:336
//line lit/gtangle.w:337
//line lit/gtangle.w:338
//line lit/gtangle.w:339
func (o *buffer) writeText(s string, line int) int {
//line lit/gtangle.w:340
	if o.pasteNext {
//line lit/gtangle.w:341
		s = strings.TrimLeft(s, " \t\n\r")
//line lit/gtangle.w:342
		o.pasteNext = false
//line lit/gtangle.w:343
	}
//line lit/gtangle.w:344
	for i := 0; i < len(s); i++ {
//line lit/gtangle.w:345
		c := s[i]
//line lit/gtangle.w:346
		if o.atLineStart && c != '\n' {
//line lit/gtangle.w:347
			o.lineMark(line)
//line lit/gtangle.w:348
			o.atLineStart = false
//line lit/gtangle.w:349
		}
//line lit/gtangle.w:350
		o.b = append(o.b, c)
//line lit/gtangle.w:351
		if c == '\n' {
//line lit/gtangle.w:352
			line++
//line lit/gtangle.w:353
			o.atLineStart = true
//line lit/gtangle.w:354
		}
//line lit/gtangle.w:355
	}
//line lit/gtangle.w:356
	return line
//line lit/gtangle.w:357
}

// lineMark emits a //line directive for the given combined-source line.
//
//line lit/gtangle.w:359
//line lit/gtangle.w:360
func (o *buffer) lineMark(line int) {
//line lit/gtangle.w:361
	file, ln := o.t.w.Origin(line)
//line lit/gtangle.w:362
	o.b = append(o.b, fmt.Sprintf("//line %s:%d\n", file, ln)...)
//line lit/gtangle.w:363
}

// newline starts a fresh output line (used around an expanded reference).
//
//line lit/gtangle.w:365
//line lit/gtangle.w:366
func (o *buffer) newline() {
//line lit/gtangle.w:367
	o.b = append(o.b, '\n')
//line lit/gtangle.w:368
	o.atLineStart = true
//line lit/gtangle.w:369
}

//line lit/gtangle.w:371
func (o *buffer) trimRight() {
//line lit/gtangle.w:372
	for len(o.b) > 0 {
//line lit/gtangle.w:373
		switch o.b[len(o.b)-1] {
//line lit/gtangle.w:374
		case ' ', '\t', '\n', '\r':
//line lit/gtangle.w:375
			o.b = o.b[:len(o.b)-1]
//line lit/gtangle.w:376
		default:
//line lit/gtangle.w:377
			return
//line lit/gtangle.w:378
		}
//line lit/gtangle.w:379
	}
//line lit/gtangle.w:380
}

//line lit/gtangle.w:382
func (o *buffer) bytes() []byte { return o.b }
