//line cmd/gtangle/gtangle.w:21
package main

//line cmd/gtangle/gtangle.w:23
import (
//line cmd/gtangle/gtangle.w:24
	"flag"
//line cmd/gtangle/gtangle.w:25
	"fmt"
//line cmd/gtangle/gtangle.w:26
	"go/format"
//line cmd/gtangle/gtangle.w:27
	"os"
//line cmd/gtangle/gtangle.w:28
	"path/filepath"
//line cmd/gtangle/gtangle.w:29
	"slices"
//line cmd/gtangle/gtangle.w:30
	"sort"
//line cmd/gtangle/gtangle.w:31
	"strings"

//line cmd/gtangle/gtangle.w:33
	"github.com/sjnam/gweb/common"
//line cmd/gtangle/gtangle.w:34
)

//line cmd/gtangle/gtangle.w:45
func main() {
//line cmd/gtangle/gtangle.w:46
	outDir := flag.String("o", "", "output directory (default: input file's directory)")
//line cmd/gtangle/gtangle.w:47
	showVersion := flag.Bool("version", false, "print version and exit")
//line cmd/gtangle/gtangle.w:48
	flag.Usage = usage
//line cmd/gtangle/gtangle.w:49
	flag.Parse()
//line cmd/gtangle/gtangle.w:50
	if *showVersion {
//line cmd/gtangle/gtangle.w:51
		fmt.Printf("gtangle (GWEB) %s\n", common.Version)
//line cmd/gtangle/gtangle.w:52
		return
//line cmd/gtangle/gtangle.w:53
	}
//line cmd/gtangle/gtangle.w:54
	if flag.NArg() < 1 || flag.NArg() > 2 {
//line cmd/gtangle/gtangle.w:55
		usage()
//line cmd/gtangle/gtangle.w:56
		os.Exit(2)
//line cmd/gtangle/gtangle.w:57
	}
//line cmd/gtangle/gtangle.w:58
	fmt.Fprintf(os.Stderr, "This is GTANGLE, Version %s\n", common.Version)
//line cmd/gtangle/gtangle.w:59
	if err := run(flag.Arg(0), flag.Arg(1), *outDir); err != nil {
//line cmd/gtangle/gtangle.w:60
		fmt.Fprintln(os.Stderr, "gtangle:", err)
//line cmd/gtangle/gtangle.w:61
		os.Exit(1)
//line cmd/gtangle/gtangle.w:62
	}
//line cmd/gtangle/gtangle.w:63
}

//line cmd/gtangle/gtangle.w:67
func usage() {
//line cmd/gtangle/gtangle.w:68
	fmt.Fprintln(os.Stderr, "usage: gtangle [-o dir] file[.w] [change[.ch]]")
//line cmd/gtangle/gtangle.w:69
	flag.PrintDefaults()
//line cmd/gtangle/gtangle.w:70
}

//line cmd/gtangle/gtangle.w:76
func reportProgress(w *common.Web) {
//line cmd/gtangle/gtangle.w:77
	for _, s := range w.Sections {
//line cmd/gtangle/gtangle.w:78
		if s.Starred {
//line cmd/gtangle/gtangle.w:79
			fmt.Fprintf(os.Stderr, "*%d", s.Number)
//line cmd/gtangle/gtangle.w:80
		}
//line cmd/gtangle/gtangle.w:81
	}
//line cmd/gtangle/gtangle.w:82
	fmt.Fprintln(os.Stderr)
//line cmd/gtangle/gtangle.w:83
}

//line cmd/gtangle/gtangle.w:90
func run(input, changeFile, outDir string) error {
//line cmd/gtangle/gtangle.w:91
	input = common.DefaultExt(input, ".w")
//line cmd/gtangle/gtangle.w:92
	changeFile = common.DefaultExt(changeFile, ".ch")
//line cmd/gtangle/gtangle.w:93
	w, err := common.ParseWithChange(input, changeFile)
//line cmd/gtangle/gtangle.w:94
	if err != nil {
//line cmd/gtangle/gtangle.w:95
		return err
//line cmd/gtangle/gtangle.w:96
	}
//line cmd/gtangle/gtangle.w:97
	for _, warn := range w.Warnings {
//line cmd/gtangle/gtangle.w:98
		fmt.Fprintln(os.Stderr, "gtangle: warning:", warn)
//line cmd/gtangle/gtangle.w:99
	}
//line cmd/gtangle/gtangle.w:100
	reportProgress(w)
//line cmd/gtangle/gtangle.w:101
	if outDir == "" {
//line cmd/gtangle/gtangle.w:102
		outDir = filepath.Dir(input)
//line cmd/gtangle/gtangle.w:103
	}

//line cmd/gtangle/gtangle.w:105
	base := filepath.Base(input)
//line cmd/gtangle/gtangle.w:106
	base = strings.TrimSuffix(base, filepath.Ext(base))
//line cmd/gtangle/gtangle.w:107
	defaultFile := base + ".go"

//line cmd/gtangle/gtangle.w:109
	outs, err := New(w).Tangle(defaultFile)
//line cmd/gtangle/gtangle.w:110
	if err != nil {
//line cmd/gtangle/gtangle.w:111
		return err
//line cmd/gtangle/gtangle.w:112
	}
//line cmd/gtangle/gtangle.w:113

//line cmd/gtangle/gtangle.w:121
	for _, out := range outs {
//line cmd/gtangle/gtangle.w:122
		path := filepath.Join(outDir, out.File)
//line cmd/gtangle/gtangle.w:123
		if dir := filepath.Dir(path); dir != "." {
//line cmd/gtangle/gtangle.w:124
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
//line cmd/gtangle/gtangle.w:125
				return mkErr
//line cmd/gtangle/gtangle.w:126
			}
//line cmd/gtangle/gtangle.w:127
		}
//line cmd/gtangle/gtangle.w:128
		if writeErr := os.WriteFile(path, out.Content, 0o644); writeErr != nil {
//line cmd/gtangle/gtangle.w:129
			return writeErr
//line cmd/gtangle/gtangle.w:130
		}
//line cmd/gtangle/gtangle.w:131
		if out.Warning != "" {
//line cmd/gtangle/gtangle.w:132
			fmt.Fprintf(os.Stderr, "gtangle: warning: %s: %s\n", path, out.Warning)
//line cmd/gtangle/gtangle.w:133
		}
//line cmd/gtangle/gtangle.w:134
		fmt.Printf("gtangle: wrote %s (%d bytes)\n", path, len(out.Content))
//line cmd/gtangle/gtangle.w:135
	}

//line cmd/gtangle/gtangle.w:114
	return nil
//line cmd/gtangle/gtangle.w:115
}

//line cmd/gtangle/gtangle.w:156
type Output struct {
//line cmd/gtangle/gtangle.w:157
	File string
//line cmd/gtangle/gtangle.w:158
	Content []byte
//line cmd/gtangle/gtangle.w:159
	Warning string
//line cmd/gtangle/gtangle.w:160
}

//line cmd/gtangle/gtangle.w:171
type Tangler struct {
//line cmd/gtangle/gtangle.w:172
	w *common.Web
//line cmd/gtangle/gtangle.w:173
	defs map[string][]codePiece // canonical named-section $\rightarrow$ code pieces
//line cmd/gtangle/gtangle.w:174
	files map[string][]codePiece // \.{@(file@>=name} $\rightarrow$ code pieces
//line cmd/gtangle/gtangle.w:175
	main []codePiece // unnamed \.{@c} sections, in order
//line cmd/gtangle/gtangle.w:176
}

//line cmd/gtangle/gtangle.w:178
type codePiece struct {
//line cmd/gtangle/gtangle.w:179
	code string
//line cmd/gtangle/gtangle.w:180
	line int
//line cmd/gtangle/gtangle.w:181
}

//line cmd/gtangle/gtangle.w:187
func New(w *common.Web) *Tangler {
//line cmd/gtangle/gtangle.w:188
	t := &Tangler{
//line cmd/gtangle/gtangle.w:189
		w: w,
//line cmd/gtangle/gtangle.w:190
		defs: map[string][]codePiece{},
//line cmd/gtangle/gtangle.w:191
		files: map[string][]codePiece{},
//line cmd/gtangle/gtangle.w:192
	}
//line cmd/gtangle/gtangle.w:193
	for _, s := range w.Sections {
//line cmd/gtangle/gtangle.w:194
		if !s.HasCode {
//line cmd/gtangle/gtangle.w:195
			continue
//line cmd/gtangle/gtangle.w:196
		}
//line cmd/gtangle/gtangle.w:197
		p := codePiece{s.Code, s.CodeLine}
//line cmd/gtangle/gtangle.w:198
		switch {
//line cmd/gtangle/gtangle.w:199
		case s.Name == "":
//line cmd/gtangle/gtangle.w:200
			t.main = append(t.main, p)
//line cmd/gtangle/gtangle.w:201
		case s.IsFile:
//line cmd/gtangle/gtangle.w:202
			t.files[s.Name] = append(t.files[s.Name], p)
//line cmd/gtangle/gtangle.w:203
		default:
//line cmd/gtangle/gtangle.w:204
			name := w.Resolve(s.Name)
//line cmd/gtangle/gtangle.w:205
			t.defs[name] = append(t.defs[name], p)
//line cmd/gtangle/gtangle.w:206
		}
//line cmd/gtangle/gtangle.w:207
	}
//line cmd/gtangle/gtangle.w:208
	return t
//line cmd/gtangle/gtangle.w:209
}

//line cmd/gtangle/gtangle.w:214
func (t *Tangler) Tangle(defaultFile string) ([]Output, error) {
//line cmd/gtangle/gtangle.w:215
	var outs []Output

//line cmd/gtangle/gtangle.w:217
	if nonEmpty(t.main) {
//line cmd/gtangle/gtangle.w:218
		out, err := t.renderOutput(defaultFile, t.main)
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
	names := make([]string, 0, len(t.files))
//line cmd/gtangle/gtangle.w:226
	for name := range t.files {
//line cmd/gtangle/gtangle.w:227
		names = append(names, name)
//line cmd/gtangle/gtangle.w:228
	}
//line cmd/gtangle/gtangle.w:229
	sort.Strings(names)
//line cmd/gtangle/gtangle.w:230
	for _, name := range names {
//line cmd/gtangle/gtangle.w:231
		out, err := t.renderOutput(name, t.files[name])
//line cmd/gtangle/gtangle.w:232
		if err != nil {
//line cmd/gtangle/gtangle.w:233
			return nil, err
//line cmd/gtangle/gtangle.w:234
		}
//line cmd/gtangle/gtangle.w:235
		outs = append(outs, out)
//line cmd/gtangle/gtangle.w:236
	}

//line cmd/gtangle/gtangle.w:238
	if len(outs) == 0 {
//line cmd/gtangle/gtangle.w:239
		return nil, fmt.Errorf("no code to tangle (no @c or @(file@>= sections)")
//line cmd/gtangle/gtangle.w:240
	}
//line cmd/gtangle/gtangle.w:241
	return outs, nil
//line cmd/gtangle/gtangle.w:242
}

//line cmd/gtangle/gtangle.w:247
func nonEmpty(pieces []codePiece) bool {
//line cmd/gtangle/gtangle.w:248
	for _, p := range pieces {
//line cmd/gtangle/gtangle.w:249
		if strings.TrimSpace(p.code) != "" {
//line cmd/gtangle/gtangle.w:250
			return true
//line cmd/gtangle/gtangle.w:251
		}
//line cmd/gtangle/gtangle.w:252
	}
//line cmd/gtangle/gtangle.w:253
	return false
//line cmd/gtangle/gtangle.w:254
}

//line cmd/gtangle/gtangle.w:261
func (t *Tangler) renderOutput(file string, pieces []codePiece) (Output, error) {
//line cmd/gtangle/gtangle.w:262
	o := &buffer{t: t, atLineStart: true}
//line cmd/gtangle/gtangle.w:263
	if err := t.expandPieces(pieces, o, nil); err != nil {
//line cmd/gtangle/gtangle.w:264
		return Output{}, err
//line cmd/gtangle/gtangle.w:265
	}
//line cmd/gtangle/gtangle.w:266
	raw := o.bytes()
//line cmd/gtangle/gtangle.w:267
	if formatted, err := format.Source(raw); err == nil {
//line cmd/gtangle/gtangle.w:268
		return Output{File: file, Content: formatted}, nil
//line cmd/gtangle/gtangle.w:269
	} else {
//line cmd/gtangle/gtangle.w:270
		return Output{File: file, Content: raw,
//line cmd/gtangle/gtangle.w:271
			Warning: "gofmt could not format the output: " + err.Error()}, nil
//line cmd/gtangle/gtangle.w:272
	}
//line cmd/gtangle/gtangle.w:273
}

//line cmd/gtangle/gtangle.w:277
func (t *Tangler) expandPieces(pieces []codePiece, o *buffer, stack []string) error {
//line cmd/gtangle/gtangle.w:278
	for _, p := range pieces {
//line cmd/gtangle/gtangle.w:279
		if err := t.expand(p.code, p.line, o, stack); err != nil {
//line cmd/gtangle/gtangle.w:280
			return err
//line cmd/gtangle/gtangle.w:281
		}
//line cmd/gtangle/gtangle.w:282
	}
//line cmd/gtangle/gtangle.w:283
	return nil
//line cmd/gtangle/gtangle.w:284
}

//line cmd/gtangle/gtangle.w:291
func (t *Tangler) expand(code string, line int, o *buffer, stack []string) error {
//line cmd/gtangle/gtangle.w:292
	for _, a := range common.ScanCode(code) {
//line cmd/gtangle/gtangle.w:293
		switch a.Kind {
//line cmd/gtangle/gtangle.w:294
		case common.AText, common.AVerbatim:
//line cmd/gtangle/gtangle.w:295
			line = o.writeText(a.Text, line)
//line cmd/gtangle/gtangle.w:296
		case common.APaste:
//line cmd/gtangle/gtangle.w:297
			o.trimRight()
//line cmd/gtangle/gtangle.w:298
			o.pasteNext = true
//line cmd/gtangle/gtangle.w:299
			o.atLineStart = false
//line cmd/gtangle/gtangle.w:300
		case common.ARef:
//line cmd/gtangle/gtangle.w:301

//line cmd/gtangle/gtangle.w:313
			name := t.w.Resolve(a.Text)
//line cmd/gtangle/gtangle.w:314
			def, ok := t.defs[name]
//line cmd/gtangle/gtangle.w:315
			if !ok {
//line cmd/gtangle/gtangle.w:316
				return fmt.Errorf("undefined section <%s>", a.Text)
//line cmd/gtangle/gtangle.w:317
			}
//line cmd/gtangle/gtangle.w:318
			if slices.Contains(stack, name) {
//line cmd/gtangle/gtangle.w:319
				return fmt.Errorf("circular reference through <%s>", name)
//line cmd/gtangle/gtangle.w:320
			}
//line cmd/gtangle/gtangle.w:321
			o.newline()
//line cmd/gtangle/gtangle.w:322
			if err := t.expandPieces(def, o, append(stack, name)); err != nil {
//line cmd/gtangle/gtangle.w:323
				return err
//line cmd/gtangle/gtangle.w:324
			}
//line cmd/gtangle/gtangle.w:325
			o.newline()

//line cmd/gtangle/gtangle.w:302
		case common.ATeX, common.AIndex, common.ALayout, common.AIndexDef:
//line cmd/gtangle/gtangle.w:303
			// woven-output only; ignored by tangle
//line cmd/gtangle/gtangle.w:304
		}
//line cmd/gtangle/gtangle.w:305
	}
//line cmd/gtangle/gtangle.w:306
	return nil
//line cmd/gtangle/gtangle.w:307
}

//line cmd/gtangle/gtangle.w:331
type buffer struct {
//line cmd/gtangle/gtangle.w:332
	t *Tangler
//line cmd/gtangle/gtangle.w:333
	b []byte
//line cmd/gtangle/gtangle.w:334
	pasteNext bool
//line cmd/gtangle/gtangle.w:335
	atLineStart bool
//line cmd/gtangle/gtangle.w:336
}

//line cmd/gtangle/gtangle.w:342
func (o *buffer) writeText(s string, line int) int {
//line cmd/gtangle/gtangle.w:343
	if o.pasteNext {
//line cmd/gtangle/gtangle.w:344
		s = strings.TrimLeft(s, " \t\n\r")
//line cmd/gtangle/gtangle.w:345
		o.pasteNext = false
//line cmd/gtangle/gtangle.w:346
	}
//line cmd/gtangle/gtangle.w:347
	for i := 0; i < len(s); i++ {
//line cmd/gtangle/gtangle.w:348
		c := s[i]
//line cmd/gtangle/gtangle.w:349
		if o.atLineStart && c != '\n' {
//line cmd/gtangle/gtangle.w:350
			o.lineMark(line)
//line cmd/gtangle/gtangle.w:351
			o.atLineStart = false
//line cmd/gtangle/gtangle.w:352
		}
//line cmd/gtangle/gtangle.w:353
		o.b = append(o.b, c)
//line cmd/gtangle/gtangle.w:354
		if c == '\n' {
//line cmd/gtangle/gtangle.w:355
			line++
//line cmd/gtangle/gtangle.w:356
			o.atLineStart = true
//line cmd/gtangle/gtangle.w:357
		}
//line cmd/gtangle/gtangle.w:358
	}
//line cmd/gtangle/gtangle.w:359
	return line
//line cmd/gtangle/gtangle.w:360
}

//line cmd/gtangle/gtangle.w:367
func (o *buffer) lineMark(line int) {
//line cmd/gtangle/gtangle.w:368
	file, ln := o.t.w.Origin(line)
//line cmd/gtangle/gtangle.w:369
	o.b = append(o.b, fmt.Sprintf("//line %s:%d\n", file, ln)...)
//line cmd/gtangle/gtangle.w:370
}

//line cmd/gtangle/gtangle.w:372
func (o *buffer) newline() {
//line cmd/gtangle/gtangle.w:373
	o.b = append(o.b, '\n')
//line cmd/gtangle/gtangle.w:374
	o.atLineStart = true
//line cmd/gtangle/gtangle.w:375
}

//line cmd/gtangle/gtangle.w:377
func (o *buffer) trimRight() {
//line cmd/gtangle/gtangle.w:378
	for len(o.b) > 0 {
//line cmd/gtangle/gtangle.w:379
		switch o.b[len(o.b)-1] {
//line cmd/gtangle/gtangle.w:380
		case ' ', '\t', '\n', '\r':
//line cmd/gtangle/gtangle.w:381
			o.b = o.b[:len(o.b)-1]
//line cmd/gtangle/gtangle.w:382
		default:
//line cmd/gtangle/gtangle.w:383
			return
//line cmd/gtangle/gtangle.w:384
		}
//line cmd/gtangle/gtangle.w:385
	}
//line cmd/gtangle/gtangle.w:386
}

//line cmd/gtangle/gtangle.w:388
func (o *buffer) bytes() []byte { return o.b }
