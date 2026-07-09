//line cmd/gtangle/gtangle.w:18
package main

//line cmd/gtangle/gtangle.w:20
import (
//line cmd/gtangle/gtangle.w:21
	"flag"
//line cmd/gtangle/gtangle.w:22
	"fmt"
//line cmd/gtangle/gtangle.w:23
	"go/format"
//line cmd/gtangle/gtangle.w:24
	"os"
//line cmd/gtangle/gtangle.w:25
	"path/filepath"
//line cmd/gtangle/gtangle.w:26
	"slices"
//line cmd/gtangle/gtangle.w:27
	"sort"
//line cmd/gtangle/gtangle.w:28
	"strings"

//line cmd/gtangle/gtangle.w:30
	"github.com/sjnam/gweb/common"
//line cmd/gtangle/gtangle.w:31
)

//line cmd/gtangle/gtangle.w:42
func main() {
//line cmd/gtangle/gtangle.w:43
	outDir := flag.String("o", "", "output directory (default: input file's directory)")
//line cmd/gtangle/gtangle.w:44
	showVersion := flag.Bool("version", false, "print version and exit")
//line cmd/gtangle/gtangle.w:45
	flag.Usage = usage
//line cmd/gtangle/gtangle.w:46
	flag.Parse()
//line cmd/gtangle/gtangle.w:47
	if *showVersion {
//line cmd/gtangle/gtangle.w:48
		fmt.Printf("gtangle (GWEB) %s\n", common.Version)
//line cmd/gtangle/gtangle.w:49
		return
//line cmd/gtangle/gtangle.w:50
	}
//line cmd/gtangle/gtangle.w:51
	if flag.NArg() < 1 || flag.NArg() > 2 {
//line cmd/gtangle/gtangle.w:52
		usage()
//line cmd/gtangle/gtangle.w:53
		os.Exit(2)
//line cmd/gtangle/gtangle.w:54
	}
//line cmd/gtangle/gtangle.w:55
	fmt.Fprintf(os.Stderr, "This is GTANGLE, Version %s\n", common.Version)
//line cmd/gtangle/gtangle.w:56
	if err := run(flag.Arg(0), flag.Arg(1), *outDir); err != nil {
//line cmd/gtangle/gtangle.w:57
		fmt.Fprintln(os.Stderr, "gtangle:", err)
//line cmd/gtangle/gtangle.w:58
		os.Exit(1)
//line cmd/gtangle/gtangle.w:59
	}
//line cmd/gtangle/gtangle.w:60
}

//line cmd/gtangle/gtangle.w:64
func usage() {
//line cmd/gtangle/gtangle.w:65
	fmt.Fprintln(os.Stderr, "usage: gtangle [-o dir] file[.w] [change[.ch]]")
//line cmd/gtangle/gtangle.w:66
	flag.PrintDefaults()
//line cmd/gtangle/gtangle.w:67
}

//line cmd/gtangle/gtangle.w:73
func reportProgress(w *common.Web) {
//line cmd/gtangle/gtangle.w:74
	for _, s := range w.Sections {
//line cmd/gtangle/gtangle.w:75
		if s.Starred {
//line cmd/gtangle/gtangle.w:76
			fmt.Fprintf(os.Stderr, "*%d", s.Number)
//line cmd/gtangle/gtangle.w:77
		}
//line cmd/gtangle/gtangle.w:78
	}
//line cmd/gtangle/gtangle.w:79
	fmt.Fprintln(os.Stderr)
//line cmd/gtangle/gtangle.w:80
}

//line cmd/gtangle/gtangle.w:87
func run(input, changeFile, outDir string) error {
//line cmd/gtangle/gtangle.w:88
	input = common.DefaultExt(input, ".w")
//line cmd/gtangle/gtangle.w:89
	changeFile = common.DefaultExt(changeFile, ".ch")
//line cmd/gtangle/gtangle.w:90
	w, err := common.ParseWithChange(input, changeFile)
//line cmd/gtangle/gtangle.w:91
	if err != nil {
//line cmd/gtangle/gtangle.w:92
		return err
//line cmd/gtangle/gtangle.w:93
	}
//line cmd/gtangle/gtangle.w:94
	for _, warn := range w.Warnings {
//line cmd/gtangle/gtangle.w:95
		fmt.Fprintln(os.Stderr, "gtangle: warning:", warn)
//line cmd/gtangle/gtangle.w:96
	}
//line cmd/gtangle/gtangle.w:97
	reportProgress(w)
//line cmd/gtangle/gtangle.w:98
	if outDir == "" {
//line cmd/gtangle/gtangle.w:99
		outDir = filepath.Dir(input)
//line cmd/gtangle/gtangle.w:100
	}

//line cmd/gtangle/gtangle.w:102
	base := filepath.Base(input)
//line cmd/gtangle/gtangle.w:103
	base = strings.TrimSuffix(base, filepath.Ext(base))
//line cmd/gtangle/gtangle.w:104
	defaultFile := base + ".go"

//line cmd/gtangle/gtangle.w:106
	outs, err := New(w).Tangle(defaultFile)
//line cmd/gtangle/gtangle.w:107
	if err != nil {
//line cmd/gtangle/gtangle.w:108
		return err
//line cmd/gtangle/gtangle.w:109
	}
//line cmd/gtangle/gtangle.w:110

//line cmd/gtangle/gtangle.w:118
	for _, out := range outs {
//line cmd/gtangle/gtangle.w:119
		path := filepath.Join(outDir, out.File)
//line cmd/gtangle/gtangle.w:120
		if dir := filepath.Dir(path); dir != "." {
//line cmd/gtangle/gtangle.w:121
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
//line cmd/gtangle/gtangle.w:122
				return mkErr
//line cmd/gtangle/gtangle.w:123
			}
//line cmd/gtangle/gtangle.w:124
		}
//line cmd/gtangle/gtangle.w:125
		if writeErr := os.WriteFile(path, out.Content, 0o644); writeErr != nil {
//line cmd/gtangle/gtangle.w:126
			return writeErr
//line cmd/gtangle/gtangle.w:127
		}
//line cmd/gtangle/gtangle.w:128
		if out.Warning != "" {
//line cmd/gtangle/gtangle.w:129
			fmt.Fprintf(os.Stderr, "gtangle: warning: %s: %s\n", path, out.Warning)
//line cmd/gtangle/gtangle.w:130
		}
//line cmd/gtangle/gtangle.w:131
		fmt.Printf("gtangle: wrote %s (%d bytes)\n", path, len(out.Content))
//line cmd/gtangle/gtangle.w:132
	}

//line cmd/gtangle/gtangle.w:111
	return nil
//line cmd/gtangle/gtangle.w:112
}

//line cmd/gtangle/gtangle.w:153
type Output struct {
//line cmd/gtangle/gtangle.w:154
	File string
//line cmd/gtangle/gtangle.w:155
	Content []byte
//line cmd/gtangle/gtangle.w:156
	Warning string
//line cmd/gtangle/gtangle.w:157
}

//line cmd/gtangle/gtangle.w:168
type Tangler struct {
//line cmd/gtangle/gtangle.w:169
	w *common.Web
//line cmd/gtangle/gtangle.w:170
	defs map[string][]codePiece // canonical named-section -> code pieces
//line cmd/gtangle/gtangle.w:171
	files map[string][]codePiece // @(file@>= name -> code pieces
//line cmd/gtangle/gtangle.w:172
	main []codePiece // unnamed @c sections, in order
//line cmd/gtangle/gtangle.w:173
}

//line cmd/gtangle/gtangle.w:175
type codePiece struct {
//line cmd/gtangle/gtangle.w:176
	code string
//line cmd/gtangle/gtangle.w:177
	line int
//line cmd/gtangle/gtangle.w:178
}

//line cmd/gtangle/gtangle.w:184
func New(w *common.Web) *Tangler {
//line cmd/gtangle/gtangle.w:185
	t := &Tangler{
//line cmd/gtangle/gtangle.w:186
		w: w,
//line cmd/gtangle/gtangle.w:187
		defs: map[string][]codePiece{},
//line cmd/gtangle/gtangle.w:188
		files: map[string][]codePiece{},
//line cmd/gtangle/gtangle.w:189
	}
//line cmd/gtangle/gtangle.w:190
	for _, s := range w.Sections {
//line cmd/gtangle/gtangle.w:191
		if !s.HasCode {
//line cmd/gtangle/gtangle.w:192
			continue
//line cmd/gtangle/gtangle.w:193
		}
//line cmd/gtangle/gtangle.w:194
		p := codePiece{s.Code, s.CodeLine}
//line cmd/gtangle/gtangle.w:195
		switch {
//line cmd/gtangle/gtangle.w:196
		case s.Name == "":
//line cmd/gtangle/gtangle.w:197
			t.main = append(t.main, p)
//line cmd/gtangle/gtangle.w:198
		case s.IsFile:
//line cmd/gtangle/gtangle.w:199
			t.files[s.Name] = append(t.files[s.Name], p)
//line cmd/gtangle/gtangle.w:200
		default:
//line cmd/gtangle/gtangle.w:201
			name := w.Resolve(s.Name)
//line cmd/gtangle/gtangle.w:202
			t.defs[name] = append(t.defs[name], p)
//line cmd/gtangle/gtangle.w:203
		}
//line cmd/gtangle/gtangle.w:204
	}
//line cmd/gtangle/gtangle.w:205
	return t
//line cmd/gtangle/gtangle.w:206
}

//line cmd/gtangle/gtangle.w:211
func (t *Tangler) Tangle(defaultFile string) ([]Output, error) {
//line cmd/gtangle/gtangle.w:212
	var outs []Output

//line cmd/gtangle/gtangle.w:214
	if nonEmpty(t.main) {
//line cmd/gtangle/gtangle.w:215
		out, err := t.renderOutput(defaultFile, t.main)
//line cmd/gtangle/gtangle.w:216
		if err != nil {
//line cmd/gtangle/gtangle.w:217
			return nil, err
//line cmd/gtangle/gtangle.w:218
		}
//line cmd/gtangle/gtangle.w:219
		outs = append(outs, out)
//line cmd/gtangle/gtangle.w:220
	}

//line cmd/gtangle/gtangle.w:222
	names := make([]string, 0, len(t.files))
//line cmd/gtangle/gtangle.w:223
	for name := range t.files {
//line cmd/gtangle/gtangle.w:224
		names = append(names, name)
//line cmd/gtangle/gtangle.w:225
	}
//line cmd/gtangle/gtangle.w:226
	sort.Strings(names)
//line cmd/gtangle/gtangle.w:227
	for _, name := range names {
//line cmd/gtangle/gtangle.w:228
		out, err := t.renderOutput(name, t.files[name])
//line cmd/gtangle/gtangle.w:229
		if err != nil {
//line cmd/gtangle/gtangle.w:230
			return nil, err
//line cmd/gtangle/gtangle.w:231
		}
//line cmd/gtangle/gtangle.w:232
		outs = append(outs, out)
//line cmd/gtangle/gtangle.w:233
	}

//line cmd/gtangle/gtangle.w:235
	if len(outs) == 0 {
//line cmd/gtangle/gtangle.w:236
		return nil, fmt.Errorf("no code to tangle (no @c or @(file@>= sections)")
//line cmd/gtangle/gtangle.w:237
	}
//line cmd/gtangle/gtangle.w:238
	return outs, nil
//line cmd/gtangle/gtangle.w:239
}

//line cmd/gtangle/gtangle.w:244
func nonEmpty(pieces []codePiece) bool {
//line cmd/gtangle/gtangle.w:245
	for _, p := range pieces {
//line cmd/gtangle/gtangle.w:246
		if strings.TrimSpace(p.code) != "" {
//line cmd/gtangle/gtangle.w:247
			return true
//line cmd/gtangle/gtangle.w:248
		}
//line cmd/gtangle/gtangle.w:249
	}
//line cmd/gtangle/gtangle.w:250
	return false
//line cmd/gtangle/gtangle.w:251
}

//line cmd/gtangle/gtangle.w:258
func (t *Tangler) renderOutput(file string, pieces []codePiece) (Output, error) {
//line cmd/gtangle/gtangle.w:259
	o := &buffer{t: t, atLineStart: true}
//line cmd/gtangle/gtangle.w:260
	if err := t.expandPieces(pieces, o, nil); err != nil {
//line cmd/gtangle/gtangle.w:261
		return Output{}, err
//line cmd/gtangle/gtangle.w:262
	}
//line cmd/gtangle/gtangle.w:263
	raw := o.bytes()
//line cmd/gtangle/gtangle.w:264
	if formatted, err := format.Source(raw); err == nil {
//line cmd/gtangle/gtangle.w:265
		return Output{File: file, Content: formatted}, nil
//line cmd/gtangle/gtangle.w:266
	} else {
//line cmd/gtangle/gtangle.w:267
		return Output{File: file, Content: raw,
//line cmd/gtangle/gtangle.w:268
			Warning: "gofmt could not format the output: " + err.Error()}, nil
//line cmd/gtangle/gtangle.w:269
	}
//line cmd/gtangle/gtangle.w:270
}

//line cmd/gtangle/gtangle.w:274
func (t *Tangler) expandPieces(pieces []codePiece, o *buffer, stack []string) error {
//line cmd/gtangle/gtangle.w:275
	for _, p := range pieces {
//line cmd/gtangle/gtangle.w:276
		if err := t.expand(p.code, p.line, o, stack); err != nil {
//line cmd/gtangle/gtangle.w:277
			return err
//line cmd/gtangle/gtangle.w:278
		}
//line cmd/gtangle/gtangle.w:279
	}
//line cmd/gtangle/gtangle.w:280
	return nil
//line cmd/gtangle/gtangle.w:281
}

//line cmd/gtangle/gtangle.w:288
func (t *Tangler) expand(code string, line int, o *buffer, stack []string) error {
//line cmd/gtangle/gtangle.w:289
	for _, a := range common.ScanCode(code) {
//line cmd/gtangle/gtangle.w:290
		switch a.Kind {
//line cmd/gtangle/gtangle.w:291
		case common.AText, common.AVerbatim:
//line cmd/gtangle/gtangle.w:292
			line = o.writeText(a.Text, line)
//line cmd/gtangle/gtangle.w:293
		case common.APaste:
//line cmd/gtangle/gtangle.w:294
			o.trimRight()
//line cmd/gtangle/gtangle.w:295
			o.pasteNext = true
//line cmd/gtangle/gtangle.w:296
			o.atLineStart = false
//line cmd/gtangle/gtangle.w:297
		case common.ARef:
//line cmd/gtangle/gtangle.w:298

//line cmd/gtangle/gtangle.w:310
			name := t.w.Resolve(a.Text)
//line cmd/gtangle/gtangle.w:311
			def, ok := t.defs[name]
//line cmd/gtangle/gtangle.w:312
			if !ok {
//line cmd/gtangle/gtangle.w:313
				return fmt.Errorf("undefined section <%s>", a.Text)
//line cmd/gtangle/gtangle.w:314
			}
//line cmd/gtangle/gtangle.w:315
			if slices.Contains(stack, name) {
//line cmd/gtangle/gtangle.w:316
				return fmt.Errorf("circular reference through <%s>", name)
//line cmd/gtangle/gtangle.w:317
			}
//line cmd/gtangle/gtangle.w:318
			o.newline()
//line cmd/gtangle/gtangle.w:319
			if err := t.expandPieces(def, o, append(stack, name)); err != nil {
//line cmd/gtangle/gtangle.w:320
				return err
//line cmd/gtangle/gtangle.w:321
			}
//line cmd/gtangle/gtangle.w:322
			o.newline()

//line cmd/gtangle/gtangle.w:299
		case common.ATeX, common.AIndex, common.ALayout, common.AIndexDef:
//line cmd/gtangle/gtangle.w:300
			// woven-output only; ignored by tangle
//line cmd/gtangle/gtangle.w:301
		}
//line cmd/gtangle/gtangle.w:302
	}
//line cmd/gtangle/gtangle.w:303
	return nil
//line cmd/gtangle/gtangle.w:304
}

//line cmd/gtangle/gtangle.w:328
type buffer struct {
//line cmd/gtangle/gtangle.w:329
	t *Tangler
//line cmd/gtangle/gtangle.w:330
	b []byte
//line cmd/gtangle/gtangle.w:331
	pasteNext bool
//line cmd/gtangle/gtangle.w:332
	atLineStart bool
//line cmd/gtangle/gtangle.w:333
}

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

//line cmd/gtangle/gtangle.w:364
func (o *buffer) lineMark(line int) {
//line cmd/gtangle/gtangle.w:365
	file, ln := o.t.w.Origin(line)
//line cmd/gtangle/gtangle.w:366
	o.b = append(o.b, fmt.Sprintf("//line %s:%d\n", file, ln)...)
//line cmd/gtangle/gtangle.w:367
}

//line cmd/gtangle/gtangle.w:369
func (o *buffer) newline() {
//line cmd/gtangle/gtangle.w:370
	o.b = append(o.b, '\n')
//line cmd/gtangle/gtangle.w:371
	o.atLineStart = true
//line cmd/gtangle/gtangle.w:372
}

//line cmd/gtangle/gtangle.w:374
func (o *buffer) trimRight() {
//line cmd/gtangle/gtangle.w:375
	for len(o.b) > 0 {
//line cmd/gtangle/gtangle.w:376
		switch o.b[len(o.b)-1] {
//line cmd/gtangle/gtangle.w:377
		case ' ', '\t', '\n', '\r':
//line cmd/gtangle/gtangle.w:378
			o.b = o.b[:len(o.b)-1]
//line cmd/gtangle/gtangle.w:379
		default:
//line cmd/gtangle/gtangle.w:380
			return
//line cmd/gtangle/gtangle.w:381
		}
//line cmd/gtangle/gtangle.w:382
	}
//line cmd/gtangle/gtangle.w:383
}

//line cmd/gtangle/gtangle.w:385
func (o *buffer) bytes() []byte { return o.b }
