// Command gtangle extracts compilable Go source from a GWEB (.w) file.
//
// Usage:
//
//	gtangle [-o dir] file[.w] [change[.ch]]
//
// The .w (and .ch) extension may be omitted. The unnamed @c sections are
// written to <basename>.go (in -o dir, default the input's directory);
// @(file@>= sections are written to their named files. As in cweb's ctangle,
// the Go output always carries //line directives so the compiler reports errors
// at .w positions.
//
//line cmd/gtangle/gtangle.w:8
//line cmd/gtangle/gtangle.w:9
//line cmd/gtangle/gtangle.w:10
//line cmd/gtangle/gtangle.w:11
//line cmd/gtangle/gtangle.w:12
//line cmd/gtangle/gtangle.w:13
//line cmd/gtangle/gtangle.w:14
//line cmd/gtangle/gtangle.w:15
//line cmd/gtangle/gtangle.w:16
//line cmd/gtangle/gtangle.w:17
//line cmd/gtangle/gtangle.w:18
//line cmd/gtangle/gtangle.w:19
package main

//line cmd/gtangle/gtangle.w:21
import (
//line cmd/gtangle/gtangle.w:22
	"flag"
//line cmd/gtangle/gtangle.w:23
	"fmt"
//line cmd/gtangle/gtangle.w:24
	"go/format"
//line cmd/gtangle/gtangle.w:25
	"os"
//line cmd/gtangle/gtangle.w:26
	"path/filepath"
//line cmd/gtangle/gtangle.w:27
	"slices"
//line cmd/gtangle/gtangle.w:28
	"sort"
//line cmd/gtangle/gtangle.w:29
	"strings"

//line cmd/gtangle/gtangle.w:31
	"github.com/sjnam/gweb/common"
//line cmd/gtangle/gtangle.w:32
)

//line cmd/gtangle/gtangle.w:38
func main() {
//line cmd/gtangle/gtangle.w:39
	outDir := flag.String("o", "", "output directory (default: input file's directory)")
//line cmd/gtangle/gtangle.w:40
	showVersion := flag.Bool("version", false, "print version and exit")
//line cmd/gtangle/gtangle.w:41
	flag.Usage = usage
//line cmd/gtangle/gtangle.w:42
	flag.Parse()
//line cmd/gtangle/gtangle.w:43
	if *showVersion {
//line cmd/gtangle/gtangle.w:44
		fmt.Printf("gtangle (GWEB) %s\n", common.Version)
//line cmd/gtangle/gtangle.w:45
		return
//line cmd/gtangle/gtangle.w:46
	}
//line cmd/gtangle/gtangle.w:47
	if flag.NArg() < 1 || flag.NArg() > 2 {
//line cmd/gtangle/gtangle.w:48
		usage()
//line cmd/gtangle/gtangle.w:49
		os.Exit(2)
//line cmd/gtangle/gtangle.w:50
	}
//line cmd/gtangle/gtangle.w:51
	fmt.Fprintf(os.Stderr, "This is GTANGLE, Version %s\n", common.Version)
//line cmd/gtangle/gtangle.w:52
	if err := run(flag.Arg(0), flag.Arg(1), *outDir); err != nil {
//line cmd/gtangle/gtangle.w:53
		fmt.Fprintln(os.Stderr, "gtangle:", err)
//line cmd/gtangle/gtangle.w:54
		os.Exit(1)
//line cmd/gtangle/gtangle.w:55
	}
//line cmd/gtangle/gtangle.w:56
}

//line cmd/gtangle/gtangle.w:60
func usage() {
//line cmd/gtangle/gtangle.w:61
	fmt.Fprintln(os.Stderr, "usage: gtangle [-o dir] file[.w] [change[.ch]]")
//line cmd/gtangle/gtangle.w:62
	flag.PrintDefaults()
//line cmd/gtangle/gtangle.w:63
}

//line cmd/gtangle/gtangle.w:69
func reportProgress(w *common.Web) {
//line cmd/gtangle/gtangle.w:70
	for _, s := range w.Sections {
//line cmd/gtangle/gtangle.w:71
		if s.Starred {
//line cmd/gtangle/gtangle.w:72
			fmt.Fprintf(os.Stderr, "*%d", s.Number)
//line cmd/gtangle/gtangle.w:73
		}
//line cmd/gtangle/gtangle.w:74
	}
//line cmd/gtangle/gtangle.w:75
	fmt.Fprintln(os.Stderr)
//line cmd/gtangle/gtangle.w:76
}

//line cmd/gtangle/gtangle.w:83
func run(input, changeFile, outDir string) error {
//line cmd/gtangle/gtangle.w:84
	input = common.DefaultExt(input, ".w")
//line cmd/gtangle/gtangle.w:85
	changeFile = common.DefaultExt(changeFile, ".ch")
//line cmd/gtangle/gtangle.w:86
	w, err := common.ParseWithChange(input, changeFile)
//line cmd/gtangle/gtangle.w:87
	if err != nil {
//line cmd/gtangle/gtangle.w:88
		return err
//line cmd/gtangle/gtangle.w:89
	}
//line cmd/gtangle/gtangle.w:90
	for _, warn := range w.Warnings {
//line cmd/gtangle/gtangle.w:91
		fmt.Fprintln(os.Stderr, "gtangle: warning:", warn)
//line cmd/gtangle/gtangle.w:92
	}
//line cmd/gtangle/gtangle.w:93
	reportProgress(w)
//line cmd/gtangle/gtangle.w:94
	if outDir == "" {
//line cmd/gtangle/gtangle.w:95
		outDir = filepath.Dir(input)
//line cmd/gtangle/gtangle.w:96
	}

//line cmd/gtangle/gtangle.w:98
	base := filepath.Base(input)
//line cmd/gtangle/gtangle.w:99
	base = strings.TrimSuffix(base, filepath.Ext(base))
//line cmd/gtangle/gtangle.w:100
	defaultFile := base + ".go"

//line cmd/gtangle/gtangle.w:102
	outs, err := New(w).Tangle(defaultFile)
//line cmd/gtangle/gtangle.w:103
	if err != nil {
//line cmd/gtangle/gtangle.w:104
		return err
//line cmd/gtangle/gtangle.w:105
	}

//line cmd/gtangle/gtangle.w:107
	for _, out := range outs {
//line cmd/gtangle/gtangle.w:108
		path := filepath.Join(outDir, out.File)
//line cmd/gtangle/gtangle.w:109
		if dir := filepath.Dir(path); dir != "." {
//line cmd/gtangle/gtangle.w:110
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
//line cmd/gtangle/gtangle.w:111
				return mkErr
//line cmd/gtangle/gtangle.w:112
			}
//line cmd/gtangle/gtangle.w:113
		}
//line cmd/gtangle/gtangle.w:114
		if writeErr := os.WriteFile(path, out.Content, 0o644); writeErr != nil {
//line cmd/gtangle/gtangle.w:115
			return writeErr
//line cmd/gtangle/gtangle.w:116
		}
//line cmd/gtangle/gtangle.w:117
		if out.Warning != "" {
//line cmd/gtangle/gtangle.w:118
			fmt.Fprintf(os.Stderr, "gtangle: warning: %s: %s\n", path, out.Warning)
//line cmd/gtangle/gtangle.w:119
		}
//line cmd/gtangle/gtangle.w:120
		fmt.Printf("gtangle: wrote %s (%d bytes)\n", path, len(out.Content))
//line cmd/gtangle/gtangle.w:121
	}
//line cmd/gtangle/gtangle.w:122
	return nil
//line cmd/gtangle/gtangle.w:123
}

// Output is one tangled file: its target name and Go contents. Warning is set
// (non-fatal) when gofmt could not format the assembled program.
//
//line cmd/gtangle/gtangle.w:135
//line cmd/gtangle/gtangle.w:136
//line cmd/gtangle/gtangle.w:137
type Output struct {
//line cmd/gtangle/gtangle.w:138
	File string
//line cmd/gtangle/gtangle.w:139
	Content []byte
//line cmd/gtangle/gtangle.w:140
	Warning string
//line cmd/gtangle/gtangle.w:141
}

// Tangler holds the resolved code of a web, classified by destination.
//
//line cmd/gtangle/gtangle.w:152
//line cmd/gtangle/gtangle.w:153
type Tangler struct {
//line cmd/gtangle/gtangle.w:154
	w *common.Web
//line cmd/gtangle/gtangle.w:155
	defs map[string][]codePiece // canonical named-section -> code pieces
//line cmd/gtangle/gtangle.w:156
	files map[string][]codePiece // @(file@>= name -> code pieces
//line cmd/gtangle/gtangle.w:157
	main []codePiece // unnamed @c sections, in order
//line cmd/gtangle/gtangle.w:158
}

// codePiece is one section's raw code together with the 1-based combined-source
// line it begins on, so tangled output can be mapped back to the .w file.
//
//line cmd/gtangle/gtangle.w:160
//line cmd/gtangle/gtangle.w:161
//line cmd/gtangle/gtangle.w:162
type codePiece struct {
//line cmd/gtangle/gtangle.w:163
	code string
//line cmd/gtangle/gtangle.w:164
	line int
//line cmd/gtangle/gtangle.w:165
}

// New builds a Tangler from a parsed web.
//
//line cmd/gtangle/gtangle.w:171
//line cmd/gtangle/gtangle.w:172
func New(w *common.Web) *Tangler {
//line cmd/gtangle/gtangle.w:173
	t := &Tangler{
//line cmd/gtangle/gtangle.w:174
		w: w,
//line cmd/gtangle/gtangle.w:175
		defs: map[string][]codePiece{},
//line cmd/gtangle/gtangle.w:176
		files: map[string][]codePiece{},
//line cmd/gtangle/gtangle.w:177
	}
//line cmd/gtangle/gtangle.w:178
	for _, s := range w.Sections {
//line cmd/gtangle/gtangle.w:179
		if !s.HasCode {
//line cmd/gtangle/gtangle.w:180
			continue
//line cmd/gtangle/gtangle.w:181
		}
//line cmd/gtangle/gtangle.w:182
		p := codePiece{s.Code, s.CodeLine}
//line cmd/gtangle/gtangle.w:183
		switch {
//line cmd/gtangle/gtangle.w:184
		case s.Name == "":
//line cmd/gtangle/gtangle.w:185
			t.main = append(t.main, p)
//line cmd/gtangle/gtangle.w:186
		case s.IsFile:
//line cmd/gtangle/gtangle.w:187
			t.files[s.Name] = append(t.files[s.Name], p)
//line cmd/gtangle/gtangle.w:188
		default:
//line cmd/gtangle/gtangle.w:189
			name := w.Resolve(s.Name)
//line cmd/gtangle/gtangle.w:190
			t.defs[name] = append(t.defs[name], p)
//line cmd/gtangle/gtangle.w:191
		}
//line cmd/gtangle/gtangle.w:192
	}
//line cmd/gtangle/gtangle.w:193
	return t
//line cmd/gtangle/gtangle.w:194
}

// Tangle produces all output files. defaultFile names the file that receives
// the unnamed program text (typically "<basename>.go").
//
//line cmd/gtangle/gtangle.w:199
//line cmd/gtangle/gtangle.w:200
//line cmd/gtangle/gtangle.w:201
func (t *Tangler) Tangle(defaultFile string) ([]Output, error) {
//line cmd/gtangle/gtangle.w:202
	var outs []Output

//line cmd/gtangle/gtangle.w:204
	if nonEmpty(t.main) {
//line cmd/gtangle/gtangle.w:205
		out, err := t.renderOutput(defaultFile, t.main)
//line cmd/gtangle/gtangle.w:206
		if err != nil {
//line cmd/gtangle/gtangle.w:207
			return nil, err
//line cmd/gtangle/gtangle.w:208
		}
//line cmd/gtangle/gtangle.w:209
		outs = append(outs, out)
//line cmd/gtangle/gtangle.w:210
	}

//line cmd/gtangle/gtangle.w:212
	names := make([]string, 0, len(t.files))
//line cmd/gtangle/gtangle.w:213
	for name := range t.files {
//line cmd/gtangle/gtangle.w:214
		names = append(names, name)
//line cmd/gtangle/gtangle.w:215
	}
//line cmd/gtangle/gtangle.w:216
	sort.Strings(names)
//line cmd/gtangle/gtangle.w:217
	for _, name := range names {
//line cmd/gtangle/gtangle.w:218
		out, err := t.renderOutput(name, t.files[name])
//line cmd/gtangle/gtangle.w:219
		if err != nil {
//line cmd/gtangle/gtangle.w:220
			return nil, err
//line cmd/gtangle/gtangle.w:221
		}
//line cmd/gtangle/gtangle.w:222
		outs = append(outs, out)
//line cmd/gtangle/gtangle.w:223
	}

//line cmd/gtangle/gtangle.w:225
	if len(outs) == 0 {
//line cmd/gtangle/gtangle.w:226
		return nil, fmt.Errorf("no code to tangle (no @c or @(file@>= sections)")
//line cmd/gtangle/gtangle.w:227
	}
//line cmd/gtangle/gtangle.w:228
	return outs, nil
//line cmd/gtangle/gtangle.w:229
}

// nonEmpty reports whether any piece carries non-blank code.
//
//line cmd/gtangle/gtangle.w:234
//line cmd/gtangle/gtangle.w:235
func nonEmpty(pieces []codePiece) bool {
//line cmd/gtangle/gtangle.w:236
	for _, p := range pieces {
//line cmd/gtangle/gtangle.w:237
		if strings.TrimSpace(p.code) != "" {
//line cmd/gtangle/gtangle.w:238
			return true
//line cmd/gtangle/gtangle.w:239
		}
//line cmd/gtangle/gtangle.w:240
	}
//line cmd/gtangle/gtangle.w:241
	return false
//line cmd/gtangle/gtangle.w:242
}

// renderOutput expands a destination's code pieces and runs gofmt. A genuine web
// error (undefined or circular reference) is fatal; a gofmt failure is not: the
// unformatted Go is kept and reported via Output.Warning.
//
//line cmd/gtangle/gtangle.w:249
//line cmd/gtangle/gtangle.w:250
//line cmd/gtangle/gtangle.w:251
//line cmd/gtangle/gtangle.w:252
func (t *Tangler) renderOutput(file string, pieces []codePiece) (Output, error) {
//line cmd/gtangle/gtangle.w:253
	o := &buffer{t: t, atLineStart: true}
//line cmd/gtangle/gtangle.w:254
	if err := t.expandPieces(pieces, o, nil); err != nil {
//line cmd/gtangle/gtangle.w:255
		return Output{}, err
//line cmd/gtangle/gtangle.w:256
	}
//line cmd/gtangle/gtangle.w:257
	raw := o.bytes()
//line cmd/gtangle/gtangle.w:258
	if formatted, err := format.Source(raw); err == nil {
//line cmd/gtangle/gtangle.w:259
		return Output{File: file, Content: formatted}, nil
//line cmd/gtangle/gtangle.w:260
	} else {
//line cmd/gtangle/gtangle.w:261
		return Output{File: file, Content: raw,
//line cmd/gtangle/gtangle.w:262
			Warning: "gofmt could not format the output: " + err.Error()}, nil
//line cmd/gtangle/gtangle.w:263
	}
//line cmd/gtangle/gtangle.w:264
}

// expandPieces expands a list of code pieces in order.
//
//line cmd/gtangle/gtangle.w:271
//line cmd/gtangle/gtangle.w:272
func (t *Tangler) expandPieces(pieces []codePiece, o *buffer, stack []string) error {
//line cmd/gtangle/gtangle.w:273
	for _, p := range pieces {
//line cmd/gtangle/gtangle.w:274
		if err := t.expand(p.code, p.line, o, stack); err != nil {
//line cmd/gtangle/gtangle.w:275
			return err
//line cmd/gtangle/gtangle.w:276
		}
//line cmd/gtangle/gtangle.w:277
	}
//line cmd/gtangle/gtangle.w:278
	return nil
//line cmd/gtangle/gtangle.w:279
}

// expand writes the expansion of one code piece into o, starting at the given
// combined-source line and following @<...@> references.
//
//line cmd/gtangle/gtangle.w:281
//line cmd/gtangle/gtangle.w:282
//line cmd/gtangle/gtangle.w:283
func (t *Tangler) expand(code string, line int, o *buffer, stack []string) error {
//line cmd/gtangle/gtangle.w:284
	for _, a := range common.ScanCode(code) {
//line cmd/gtangle/gtangle.w:285
		switch a.Kind {
//line cmd/gtangle/gtangle.w:286
		case common.AText, common.AVerbatim:
//line cmd/gtangle/gtangle.w:287
			line = o.writeText(a.Text, line)
//line cmd/gtangle/gtangle.w:288
		case common.APaste:
//line cmd/gtangle/gtangle.w:289
			o.trimRight()
//line cmd/gtangle/gtangle.w:290
			o.pasteNext = true
//line cmd/gtangle/gtangle.w:291
			o.atLineStart = false
//line cmd/gtangle/gtangle.w:292
		case common.ARef:
//line cmd/gtangle/gtangle.w:293
			name := t.w.Resolve(a.Text)
//line cmd/gtangle/gtangle.w:294
			def, ok := t.defs[name]
//line cmd/gtangle/gtangle.w:295
			if !ok {
//line cmd/gtangle/gtangle.w:296
				return fmt.Errorf("undefined section <%s>", a.Text)
//line cmd/gtangle/gtangle.w:297
			}
//line cmd/gtangle/gtangle.w:298
			if slices.Contains(stack, name) {
//line cmd/gtangle/gtangle.w:299
				return fmt.Errorf("circular reference through <%s>", name)
//line cmd/gtangle/gtangle.w:300
			}
//line cmd/gtangle/gtangle.w:301
			// Surround an expanded reference with newlines so adjacent
//line cmd/gtangle/gtangle.w:302
			// statements stay on separate lines; gofmt collapses the rest.
//line cmd/gtangle/gtangle.w:303
			o.newline()
//line cmd/gtangle/gtangle.w:304
			if err := t.expandPieces(def, o, append(stack, name)); err != nil {
//line cmd/gtangle/gtangle.w:305
				return err
//line cmd/gtangle/gtangle.w:306
			}
//line cmd/gtangle/gtangle.w:307
			o.newline()
//line cmd/gtangle/gtangle.w:308
		case common.ATeX, common.AIndex, common.ALayout, common.AIndexDef:
//line cmd/gtangle/gtangle.w:309
			// woven-output only; ignored by tangle
//line cmd/gtangle/gtangle.w:310
		}
//line cmd/gtangle/gtangle.w:311
	}
//line cmd/gtangle/gtangle.w:312
	return nil
//line cmd/gtangle/gtangle.w:313
}

// buffer accumulates output, tracks line starts for //line directives, and
// supports the @& paste operation.
//
//line cmd/gtangle/gtangle.w:319
//line cmd/gtangle/gtangle.w:320
//line cmd/gtangle/gtangle.w:321
type buffer struct {
//line cmd/gtangle/gtangle.w:322
	t *Tangler
//line cmd/gtangle/gtangle.w:323
	b []byte
//line cmd/gtangle/gtangle.w:324
	pasteNext bool
//line cmd/gtangle/gtangle.w:325
	atLineStart bool
//line cmd/gtangle/gtangle.w:326
}

// writeText appends s, advancing the source line across newlines. It prefixes
// each output line with a //line comment mapping it back to its .w origin, and
// returns the updated source line.
//
//line cmd/gtangle/gtangle.w:328
//line cmd/gtangle/gtangle.w:329
//line cmd/gtangle/gtangle.w:330
//line cmd/gtangle/gtangle.w:331
func (o *buffer) writeText(s string, line int) int {
//line cmd/gtangle/gtangle.w:332
	if o.pasteNext {
//line cmd/gtangle/gtangle.w:333
		s = strings.TrimLeft(s, " \t\n\r")
//line cmd/gtangle/gtangle.w:334
		o.pasteNext = false
//line cmd/gtangle/gtangle.w:335
	}
//line cmd/gtangle/gtangle.w:336
	for i := 0; i < len(s); i++ {
//line cmd/gtangle/gtangle.w:337
		c := s[i]
//line cmd/gtangle/gtangle.w:338
		if o.atLineStart && c != '\n' {
//line cmd/gtangle/gtangle.w:339
			o.lineMark(line)
//line cmd/gtangle/gtangle.w:340
			o.atLineStart = false
//line cmd/gtangle/gtangle.w:341
		}
//line cmd/gtangle/gtangle.w:342
		o.b = append(o.b, c)
//line cmd/gtangle/gtangle.w:343
		if c == '\n' {
//line cmd/gtangle/gtangle.w:344
			line++
//line cmd/gtangle/gtangle.w:345
			o.atLineStart = true
//line cmd/gtangle/gtangle.w:346
		}
//line cmd/gtangle/gtangle.w:347
	}
//line cmd/gtangle/gtangle.w:348
	return line
//line cmd/gtangle/gtangle.w:349
}

// lineMark emits a //line directive for the given combined-source line.
//
//line cmd/gtangle/gtangle.w:351
//line cmd/gtangle/gtangle.w:352
func (o *buffer) lineMark(line int) {
//line cmd/gtangle/gtangle.w:353
	file, ln := o.t.w.Origin(line)
//line cmd/gtangle/gtangle.w:354
	o.b = append(o.b, fmt.Sprintf("//line %s:%d\n", file, ln)...)
//line cmd/gtangle/gtangle.w:355
}

// newline starts a fresh output line (used around an expanded reference).
//
//line cmd/gtangle/gtangle.w:357
//line cmd/gtangle/gtangle.w:358
func (o *buffer) newline() {
//line cmd/gtangle/gtangle.w:359
	o.b = append(o.b, '\n')
//line cmd/gtangle/gtangle.w:360
	o.atLineStart = true
//line cmd/gtangle/gtangle.w:361
}

//line cmd/gtangle/gtangle.w:363
func (o *buffer) trimRight() {
//line cmd/gtangle/gtangle.w:364
	for len(o.b) > 0 {
//line cmd/gtangle/gtangle.w:365
		switch o.b[len(o.b)-1] {
//line cmd/gtangle/gtangle.w:366
		case ' ', '\t', '\n', '\r':
//line cmd/gtangle/gtangle.w:367
			o.b = o.b[:len(o.b)-1]
//line cmd/gtangle/gtangle.w:368
		default:
//line cmd/gtangle/gtangle.w:369
			return
//line cmd/gtangle/gtangle.w:370
		}
//line cmd/gtangle/gtangle.w:371
	}
//line cmd/gtangle/gtangle.w:372
}

//line cmd/gtangle/gtangle.w:374
func (o *buffer) bytes() []byte { return o.b }
