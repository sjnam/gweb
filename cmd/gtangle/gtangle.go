//line cmd/gtangle/gtangle.w:27
package main

//line cmd/gtangle/gtangle.w:29
import (
//line cmd/gtangle/gtangle.w:30
	"flag"
//line cmd/gtangle/gtangle.w:31
	"fmt"
//line cmd/gtangle/gtangle.w:32
	"go/format"
//line cmd/gtangle/gtangle.w:33
	"os"
//line cmd/gtangle/gtangle.w:34
	"path/filepath"
//line cmd/gtangle/gtangle.w:35
	"slices"
//line cmd/gtangle/gtangle.w:36
	"sort"
//line cmd/gtangle/gtangle.w:37
	"strings"

//line cmd/gtangle/gtangle.w:39
	"github.com/sjnam/gweb/common"
//line cmd/gtangle/gtangle.w:40
)

//line cmd/gtangle/gtangle.w:51
func main() {
//line cmd/gtangle/gtangle.w:52
	outDir := flag.String("o", "", "output directory (default: input file's directory)")
//line cmd/gtangle/gtangle.w:53
	showVersion := flag.Bool("version", false, "print version and exit")
//line cmd/gtangle/gtangle.w:54
	flag.Usage = usage
//line cmd/gtangle/gtangle.w:55
	flag.Parse()
//line cmd/gtangle/gtangle.w:56
	if *showVersion {
//line cmd/gtangle/gtangle.w:57
		fmt.Printf("gtangle (GWEB) %s\n", common.Version)
//line cmd/gtangle/gtangle.w:58
		return
//line cmd/gtangle/gtangle.w:59
	}
//line cmd/gtangle/gtangle.w:60
	if flag.NArg() < 1 || flag.NArg() > 2 {
//line cmd/gtangle/gtangle.w:61
		usage()
//line cmd/gtangle/gtangle.w:62
		os.Exit(2)
//line cmd/gtangle/gtangle.w:63
	}
//line cmd/gtangle/gtangle.w:64
	fmt.Fprintf(os.Stderr, "This is GTANGLE, Version %s\n", common.Version)
//line cmd/gtangle/gtangle.w:65
	if err := run(flag.Arg(0), flag.Arg(1), *outDir); err != nil {
//line cmd/gtangle/gtangle.w:66
		fmt.Fprintln(os.Stderr, "gtangle:", err)
//line cmd/gtangle/gtangle.w:67
		os.Exit(1)
//line cmd/gtangle/gtangle.w:68
	}
//line cmd/gtangle/gtangle.w:69
}

//line cmd/gtangle/gtangle.w:73
func usage() {
//line cmd/gtangle/gtangle.w:74
	fmt.Fprintln(os.Stderr, "usage: gtangle [-o dir] file[.w] [change[.ch]]")
//line cmd/gtangle/gtangle.w:75
	flag.PrintDefaults()
//line cmd/gtangle/gtangle.w:76
}

//line cmd/gtangle/gtangle.w:82
func reportProgress(w *common.Web) {
//line cmd/gtangle/gtangle.w:83
	for _, s := range w.Sections {
//line cmd/gtangle/gtangle.w:84
		if s.Starred {
//line cmd/gtangle/gtangle.w:85
			fmt.Fprintf(os.Stderr, "*%d", s.Number)
//line cmd/gtangle/gtangle.w:86
		}
//line cmd/gtangle/gtangle.w:87
	}
//line cmd/gtangle/gtangle.w:88
	fmt.Fprintln(os.Stderr)
//line cmd/gtangle/gtangle.w:89
}

//line cmd/gtangle/gtangle.w:96
func run(input, changeFile, outDir string) error {
//line cmd/gtangle/gtangle.w:97
	input = common.DefaultExt(input, ".w")
//line cmd/gtangle/gtangle.w:98
	changeFile = common.DefaultExt(changeFile, ".ch")
//line cmd/gtangle/gtangle.w:99
	w, err := common.ParseWithChange(input, changeFile)
//line cmd/gtangle/gtangle.w:100
	if err != nil {
//line cmd/gtangle/gtangle.w:101
		return err
//line cmd/gtangle/gtangle.w:102
	}
//line cmd/gtangle/gtangle.w:103
	for _, warn := range w.Warnings {
//line cmd/gtangle/gtangle.w:104
		fmt.Fprintln(os.Stderr, "gtangle: warning:", warn)
//line cmd/gtangle/gtangle.w:105
	}
//line cmd/gtangle/gtangle.w:106
	reportProgress(w)
//line cmd/gtangle/gtangle.w:107
	if outDir == "" {
//line cmd/gtangle/gtangle.w:108
		outDir = filepath.Dir(input)
//line cmd/gtangle/gtangle.w:109
	}

//line cmd/gtangle/gtangle.w:111
	base := filepath.Base(input)
//line cmd/gtangle/gtangle.w:112
	base = strings.TrimSuffix(base, filepath.Ext(base))
//line cmd/gtangle/gtangle.w:113
	defaultFile := base + ".go"

//line cmd/gtangle/gtangle.w:115
	outs, err := New(w).Tangle(defaultFile)
//line cmd/gtangle/gtangle.w:116
	if err != nil {
//line cmd/gtangle/gtangle.w:117
		return err
//line cmd/gtangle/gtangle.w:118
	}
//line cmd/gtangle/gtangle.w:119

//line cmd/gtangle/gtangle.w:127
	for _, out := range outs {
//line cmd/gtangle/gtangle.w:128
		path := filepath.Join(outDir, out.File)
//line cmd/gtangle/gtangle.w:129
		if dir := filepath.Dir(path); dir != "." {
//line cmd/gtangle/gtangle.w:130
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
//line cmd/gtangle/gtangle.w:131
				return mkErr
//line cmd/gtangle/gtangle.w:132
			}
//line cmd/gtangle/gtangle.w:133
		}
//line cmd/gtangle/gtangle.w:134
		if writeErr := os.WriteFile(path, out.Content, 0o644); writeErr != nil {
//line cmd/gtangle/gtangle.w:135
			return writeErr
//line cmd/gtangle/gtangle.w:136
		}
//line cmd/gtangle/gtangle.w:137
		if out.Warning != "" {
//line cmd/gtangle/gtangle.w:138
			fmt.Fprintf(os.Stderr, "gtangle: warning: %s: %s\n", path, out.Warning)
//line cmd/gtangle/gtangle.w:139
		}
//line cmd/gtangle/gtangle.w:140
		fmt.Printf("gtangle: wrote %s (%d bytes)\n", path, len(out.Content))
//line cmd/gtangle/gtangle.w:141
	}

//line cmd/gtangle/gtangle.w:120
	return nil
//line cmd/gtangle/gtangle.w:121
}

//line cmd/gtangle/gtangle.w:162
type Output struct {
//line cmd/gtangle/gtangle.w:163
	File string
//line cmd/gtangle/gtangle.w:164
	Content []byte
//line cmd/gtangle/gtangle.w:165
	Warning string
//line cmd/gtangle/gtangle.w:166
}

//line cmd/gtangle/gtangle.w:177
type Tangler struct {
//line cmd/gtangle/gtangle.w:178
	w *common.Web
//line cmd/gtangle/gtangle.w:179
	defs map[string][]codePiece // canonical named-section $\rightarrow$ code pieces
//line cmd/gtangle/gtangle.w:180
	files map[string][]codePiece // \.{@(file@>=name} $\rightarrow$ code pieces
//line cmd/gtangle/gtangle.w:181
	main []codePiece // unnamed \.{@c} sections, in order
//line cmd/gtangle/gtangle.w:182
}

//line cmd/gtangle/gtangle.w:184
type codePiece struct {
//line cmd/gtangle/gtangle.w:185
	code string
//line cmd/gtangle/gtangle.w:186
	line int
//line cmd/gtangle/gtangle.w:187
}

//line cmd/gtangle/gtangle.w:193
func New(w *common.Web) *Tangler {
//line cmd/gtangle/gtangle.w:194
	t := &Tangler{
//line cmd/gtangle/gtangle.w:195
		w: w,
//line cmd/gtangle/gtangle.w:196
		defs: map[string][]codePiece{},
//line cmd/gtangle/gtangle.w:197
		files: map[string][]codePiece{},
//line cmd/gtangle/gtangle.w:198
	}
//line cmd/gtangle/gtangle.w:199
	for _, s := range w.Sections {
//line cmd/gtangle/gtangle.w:200
		if !s.HasCode {
//line cmd/gtangle/gtangle.w:201
			continue
//line cmd/gtangle/gtangle.w:202
		}
//line cmd/gtangle/gtangle.w:203
		p := codePiece{s.Code, s.CodeLine}
//line cmd/gtangle/gtangle.w:204
		switch {
//line cmd/gtangle/gtangle.w:205
		case s.Name == "":
//line cmd/gtangle/gtangle.w:206
			t.main = append(t.main, p)
//line cmd/gtangle/gtangle.w:207
		case s.IsFile:
//line cmd/gtangle/gtangle.w:208
			t.files[s.Name] = append(t.files[s.Name], p)
//line cmd/gtangle/gtangle.w:209
		default:
//line cmd/gtangle/gtangle.w:210
			name := w.Resolve(s.Name)
//line cmd/gtangle/gtangle.w:211
			t.defs[name] = append(t.defs[name], p)
//line cmd/gtangle/gtangle.w:212
		}
//line cmd/gtangle/gtangle.w:213
	}
//line cmd/gtangle/gtangle.w:214
	return t
//line cmd/gtangle/gtangle.w:215
}

//line cmd/gtangle/gtangle.w:220
func (t *Tangler) Tangle(defaultFile string) ([]Output, error) {
//line cmd/gtangle/gtangle.w:221
	var outs []Output

//line cmd/gtangle/gtangle.w:223
	if nonEmpty(t.main) {
//line cmd/gtangle/gtangle.w:224
		out, err := t.renderOutput(defaultFile, t.main)
//line cmd/gtangle/gtangle.w:225
		if err != nil {
//line cmd/gtangle/gtangle.w:226
			return nil, err
//line cmd/gtangle/gtangle.w:227
		}
//line cmd/gtangle/gtangle.w:228
		outs = append(outs, out)
//line cmd/gtangle/gtangle.w:229
	}

//line cmd/gtangle/gtangle.w:231
	names := make([]string, 0, len(t.files))
//line cmd/gtangle/gtangle.w:232
	for name := range t.files {
//line cmd/gtangle/gtangle.w:233
		names = append(names, name)
//line cmd/gtangle/gtangle.w:234
	}
//line cmd/gtangle/gtangle.w:235
	sort.Strings(names)
//line cmd/gtangle/gtangle.w:236
	for _, name := range names {
//line cmd/gtangle/gtangle.w:237
		out, err := t.renderOutput(name, t.files[name])
//line cmd/gtangle/gtangle.w:238
		if err != nil {
//line cmd/gtangle/gtangle.w:239
			return nil, err
//line cmd/gtangle/gtangle.w:240
		}
//line cmd/gtangle/gtangle.w:241
		outs = append(outs, out)
//line cmd/gtangle/gtangle.w:242
	}

//line cmd/gtangle/gtangle.w:244
	if len(outs) == 0 {
//line cmd/gtangle/gtangle.w:245
		return nil, fmt.Errorf("no code to tangle (no @c or @(file@>= sections)")
//line cmd/gtangle/gtangle.w:246
	}
//line cmd/gtangle/gtangle.w:247
	return outs, nil
//line cmd/gtangle/gtangle.w:248
}

//line cmd/gtangle/gtangle.w:253
func nonEmpty(pieces []codePiece) bool {
//line cmd/gtangle/gtangle.w:254
	for _, p := range pieces {
//line cmd/gtangle/gtangle.w:255
		if strings.TrimSpace(p.code) != "" {
//line cmd/gtangle/gtangle.w:256
			return true
//line cmd/gtangle/gtangle.w:257
		}
//line cmd/gtangle/gtangle.w:258
	}
//line cmd/gtangle/gtangle.w:259
	return false
//line cmd/gtangle/gtangle.w:260
}

//line cmd/gtangle/gtangle.w:267
func (t *Tangler) renderOutput(file string, pieces []codePiece) (Output, error) {
//line cmd/gtangle/gtangle.w:268
	o := &buffer{t: t, atLineStart: true}
//line cmd/gtangle/gtangle.w:269
	if err := t.expandPieces(pieces, o, nil); err != nil {
//line cmd/gtangle/gtangle.w:270
		return Output{}, err
//line cmd/gtangle/gtangle.w:271
	}
//line cmd/gtangle/gtangle.w:272
	raw := o.bytes()
//line cmd/gtangle/gtangle.w:273
	if formatted, err := format.Source(raw); err == nil {
//line cmd/gtangle/gtangle.w:274
		return Output{File: file, Content: formatted}, nil
//line cmd/gtangle/gtangle.w:275
	} else {
//line cmd/gtangle/gtangle.w:276
		return Output{File: file, Content: raw,
//line cmd/gtangle/gtangle.w:277
			Warning: "gofmt could not format the output: " + err.Error()}, nil
//line cmd/gtangle/gtangle.w:278
	}
//line cmd/gtangle/gtangle.w:279
}

//line cmd/gtangle/gtangle.w:283
func (t *Tangler) expandPieces(pieces []codePiece, o *buffer, stack []string) error {
//line cmd/gtangle/gtangle.w:284
	for _, p := range pieces {
//line cmd/gtangle/gtangle.w:285
		if err := t.expand(p.code, p.line, o, stack); err != nil {
//line cmd/gtangle/gtangle.w:286
			return err
//line cmd/gtangle/gtangle.w:287
		}
//line cmd/gtangle/gtangle.w:288
	}
//line cmd/gtangle/gtangle.w:289
	return nil
//line cmd/gtangle/gtangle.w:290
}

//line cmd/gtangle/gtangle.w:297
func (t *Tangler) expand(code string, line int, o *buffer, stack []string) error {
//line cmd/gtangle/gtangle.w:298
	for _, a := range common.ScanCode(code) {
//line cmd/gtangle/gtangle.w:299
		switch a.Kind {
//line cmd/gtangle/gtangle.w:300
		case common.AText, common.AVerbatim:
//line cmd/gtangle/gtangle.w:301
			line = o.writeText(a.Text, line)
//line cmd/gtangle/gtangle.w:302
		case common.APaste:
//line cmd/gtangle/gtangle.w:303
			o.trimRight()
//line cmd/gtangle/gtangle.w:304
			o.pasteNext = true
//line cmd/gtangle/gtangle.w:305
			o.atLineStart = false
//line cmd/gtangle/gtangle.w:306
		case common.ARef:
//line cmd/gtangle/gtangle.w:307

//line cmd/gtangle/gtangle.w:319
			name := t.w.Resolve(a.Text)
//line cmd/gtangle/gtangle.w:320
			def, ok := t.defs[name]
//line cmd/gtangle/gtangle.w:321
			if !ok {
//line cmd/gtangle/gtangle.w:322
				return fmt.Errorf("undefined section <%s>", a.Text)
//line cmd/gtangle/gtangle.w:323
			}
//line cmd/gtangle/gtangle.w:324
			if slices.Contains(stack, name) {
//line cmd/gtangle/gtangle.w:325
				return fmt.Errorf("circular reference through <%s>", name)
//line cmd/gtangle/gtangle.w:326
			}
//line cmd/gtangle/gtangle.w:327
			o.newline()
//line cmd/gtangle/gtangle.w:328
			if err := t.expandPieces(def, o, append(stack, name)); err != nil {
//line cmd/gtangle/gtangle.w:329
				return err
//line cmd/gtangle/gtangle.w:330
			}
//line cmd/gtangle/gtangle.w:331
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

//line cmd/gtangle/gtangle.w:337
type buffer struct {
//line cmd/gtangle/gtangle.w:338
	t *Tangler
//line cmd/gtangle/gtangle.w:339
	b []byte
//line cmd/gtangle/gtangle.w:340
	pasteNext bool
//line cmd/gtangle/gtangle.w:341
	atLineStart bool
//line cmd/gtangle/gtangle.w:342
}

//line cmd/gtangle/gtangle.w:348
func (o *buffer) writeText(s string, line int) int {
//line cmd/gtangle/gtangle.w:349
	if o.pasteNext {
//line cmd/gtangle/gtangle.w:350
		s = strings.TrimLeft(s, " \t\n\r")
//line cmd/gtangle/gtangle.w:351
		o.pasteNext = false
//line cmd/gtangle/gtangle.w:352
	}
//line cmd/gtangle/gtangle.w:353
	for i := 0; i < len(s); i++ {
//line cmd/gtangle/gtangle.w:354
		c := s[i]
//line cmd/gtangle/gtangle.w:355
		if o.atLineStart && c != '\n' {
//line cmd/gtangle/gtangle.w:356
			o.lineMark(line)
//line cmd/gtangle/gtangle.w:357
			o.atLineStart = false
//line cmd/gtangle/gtangle.w:358
		}
//line cmd/gtangle/gtangle.w:359
		o.b = append(o.b, c)
//line cmd/gtangle/gtangle.w:360
		if c == '\n' {
//line cmd/gtangle/gtangle.w:361
			line++
//line cmd/gtangle/gtangle.w:362
			o.atLineStart = true
//line cmd/gtangle/gtangle.w:363
		}
//line cmd/gtangle/gtangle.w:364
	}
//line cmd/gtangle/gtangle.w:365
	return line
//line cmd/gtangle/gtangle.w:366
}

//line cmd/gtangle/gtangle.w:373
func (o *buffer) lineMark(line int) {
//line cmd/gtangle/gtangle.w:374
	file, ln := o.t.w.Origin(line)
//line cmd/gtangle/gtangle.w:375
	o.b = append(o.b, fmt.Sprintf("//line %s:%d\n", file, ln)...)
//line cmd/gtangle/gtangle.w:376
}

//line cmd/gtangle/gtangle.w:378
func (o *buffer) newline() {
//line cmd/gtangle/gtangle.w:379
	o.b = append(o.b, '\n')
//line cmd/gtangle/gtangle.w:380
	o.atLineStart = true
//line cmd/gtangle/gtangle.w:381
}

//line cmd/gtangle/gtangle.w:383
func (o *buffer) trimRight() {
//line cmd/gtangle/gtangle.w:384
	for len(o.b) > 0 {
//line cmd/gtangle/gtangle.w:385
		switch o.b[len(o.b)-1] {
//line cmd/gtangle/gtangle.w:386
		case ' ', '\t', '\n', '\r':
//line cmd/gtangle/gtangle.w:387
			o.b = o.b[:len(o.b)-1]
//line cmd/gtangle/gtangle.w:388
		default:
//line cmd/gtangle/gtangle.w:389
			return
//line cmd/gtangle/gtangle.w:390
		}
//line cmd/gtangle/gtangle.w:391
	}
//line cmd/gtangle/gtangle.w:392
}

//line cmd/gtangle/gtangle.w:394
func (o *buffer) bytes() []byte { return o.b }
