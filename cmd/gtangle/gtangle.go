//line cmd/gtangle/gtangle.w:9
package main

//line cmd/gtangle/gtangle.w:11
import (
//line cmd/gtangle/gtangle.w:12
	"flag"
//line cmd/gtangle/gtangle.w:13
	"fmt"
//line cmd/gtangle/gtangle.w:14
	"go/format"
//line cmd/gtangle/gtangle.w:15
	"os"
//line cmd/gtangle/gtangle.w:16
	"path/filepath"
//line cmd/gtangle/gtangle.w:17
	"slices"
//line cmd/gtangle/gtangle.w:18
	"sort"
//line cmd/gtangle/gtangle.w:19
	"strings"

//line cmd/gtangle/gtangle.w:21
	"github.com/sjnam/gweb/common"
//line cmd/gtangle/gtangle.w:22
)

//line cmd/gtangle/gtangle.w:33
func main() {
//line cmd/gtangle/gtangle.w:34
	outDir := flag.String("o", "", "output directory (default: input file's directory)")
//line cmd/gtangle/gtangle.w:35
	showVersion := flag.Bool("version", false, "print version and exit")
//line cmd/gtangle/gtangle.w:36
	flag.Usage = usage
//line cmd/gtangle/gtangle.w:37
	flag.Parse()
//line cmd/gtangle/gtangle.w:38
	if *showVersion {
//line cmd/gtangle/gtangle.w:39
		fmt.Printf("gtangle (GWEB) %s\n", common.Version)
//line cmd/gtangle/gtangle.w:40
		return
//line cmd/gtangle/gtangle.w:41
	}
//line cmd/gtangle/gtangle.w:42
	if flag.NArg() < 1 || flag.NArg() > 2 {
//line cmd/gtangle/gtangle.w:43
		usage()
//line cmd/gtangle/gtangle.w:44
		os.Exit(2)
//line cmd/gtangle/gtangle.w:45
	}
//line cmd/gtangle/gtangle.w:46
	fmt.Fprintf(os.Stderr, "This is GTANGLE, Version %s\n", common.Version)
//line cmd/gtangle/gtangle.w:47
	if err := run(flag.Arg(0), flag.Arg(1), *outDir); err != nil {
//line cmd/gtangle/gtangle.w:48
		fmt.Fprintln(os.Stderr, "gtangle:", err)
//line cmd/gtangle/gtangle.w:49
		os.Exit(1)
//line cmd/gtangle/gtangle.w:50
	}
//line cmd/gtangle/gtangle.w:51
}

//line cmd/gtangle/gtangle.w:55
func usage() {
//line cmd/gtangle/gtangle.w:56
	fmt.Fprintln(os.Stderr, "usage: gtangle [-o dir] file[.w] [change[.ch]]")
//line cmd/gtangle/gtangle.w:57
	flag.PrintDefaults()
//line cmd/gtangle/gtangle.w:58
}

//line cmd/gtangle/gtangle.w:64
func reportProgress(w *common.Web) {
//line cmd/gtangle/gtangle.w:65
	for _, s := range w.Sections {
//line cmd/gtangle/gtangle.w:66
		if s.Starred {
//line cmd/gtangle/gtangle.w:67
			fmt.Fprintf(os.Stderr, "*%d", s.Number)
//line cmd/gtangle/gtangle.w:68
		}
//line cmd/gtangle/gtangle.w:69
	}
//line cmd/gtangle/gtangle.w:70
	fmt.Fprintln(os.Stderr)
//line cmd/gtangle/gtangle.w:71
}

//line cmd/gtangle/gtangle.w:78
func run(input, changeFile, outDir string) error {
//line cmd/gtangle/gtangle.w:79
	input = common.DefaultExt(input, ".w")
//line cmd/gtangle/gtangle.w:80
	changeFile = common.DefaultExt(changeFile, ".ch")
//line cmd/gtangle/gtangle.w:81
	w, err := common.ParseWithChange(input, changeFile)
//line cmd/gtangle/gtangle.w:82
	if err != nil {
//line cmd/gtangle/gtangle.w:83
		return err
//line cmd/gtangle/gtangle.w:84
	}
//line cmd/gtangle/gtangle.w:85
	for _, warn := range w.Warnings {
//line cmd/gtangle/gtangle.w:86
		fmt.Fprintln(os.Stderr, "gtangle: warning:", warn)
//line cmd/gtangle/gtangle.w:87
	}
//line cmd/gtangle/gtangle.w:88
	reportProgress(w)
//line cmd/gtangle/gtangle.w:89
	if outDir == "" {
//line cmd/gtangle/gtangle.w:90
		outDir = filepath.Dir(input)
//line cmd/gtangle/gtangle.w:91
	}

//line cmd/gtangle/gtangle.w:93
	base := filepath.Base(input)
//line cmd/gtangle/gtangle.w:94
	base = strings.TrimSuffix(base, filepath.Ext(base))
//line cmd/gtangle/gtangle.w:95
	defaultFile := base + ".go"

//line cmd/gtangle/gtangle.w:97
	outs, err := New(w).Tangle(defaultFile)
//line cmd/gtangle/gtangle.w:98
	if err != nil {
//line cmd/gtangle/gtangle.w:99
		return err
//line cmd/gtangle/gtangle.w:100
	}
//line cmd/gtangle/gtangle.w:101

//line cmd/gtangle/gtangle.w:109
	for _, out := range outs {
//line cmd/gtangle/gtangle.w:110
		path := filepath.Join(outDir, out.File)
//line cmd/gtangle/gtangle.w:111
		if dir := filepath.Dir(path); dir != "." {
//line cmd/gtangle/gtangle.w:112
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
//line cmd/gtangle/gtangle.w:113
				return mkErr
//line cmd/gtangle/gtangle.w:114
			}
//line cmd/gtangle/gtangle.w:115
		}
//line cmd/gtangle/gtangle.w:116
		if writeErr := os.WriteFile(path, out.Content, 0o644); writeErr != nil {
//line cmd/gtangle/gtangle.w:117
			return writeErr
//line cmd/gtangle/gtangle.w:118
		}
//line cmd/gtangle/gtangle.w:119
		if out.Warning != "" {
//line cmd/gtangle/gtangle.w:120
			fmt.Fprintf(os.Stderr, "gtangle: warning: %s: %s\n", path, out.Warning)
//line cmd/gtangle/gtangle.w:121
		}
//line cmd/gtangle/gtangle.w:122
		fmt.Printf("gtangle: wrote %s (%d bytes)\n", path, len(out.Content))
//line cmd/gtangle/gtangle.w:123
	}

//line cmd/gtangle/gtangle.w:102
	return nil
//line cmd/gtangle/gtangle.w:103
}

//line cmd/gtangle/gtangle.w:144
type Output struct {
//line cmd/gtangle/gtangle.w:145
	File string
//line cmd/gtangle/gtangle.w:146
	Content []byte
//line cmd/gtangle/gtangle.w:147
	Warning string
//line cmd/gtangle/gtangle.w:148
}

//line cmd/gtangle/gtangle.w:159
type Tangler struct {
//line cmd/gtangle/gtangle.w:160
	w *common.Web
//line cmd/gtangle/gtangle.w:161
	defs map[string][]codePiece // canonical named-section -> code pieces
//line cmd/gtangle/gtangle.w:162
	files map[string][]codePiece // @(file@>= name -> code pieces
//line cmd/gtangle/gtangle.w:163
	main []codePiece // unnamed @c sections, in order
//line cmd/gtangle/gtangle.w:164
}

//line cmd/gtangle/gtangle.w:166
type codePiece struct {
//line cmd/gtangle/gtangle.w:167
	code string
//line cmd/gtangle/gtangle.w:168
	line int
//line cmd/gtangle/gtangle.w:169
}

//line cmd/gtangle/gtangle.w:175
func New(w *common.Web) *Tangler {
//line cmd/gtangle/gtangle.w:176
	t := &Tangler{
//line cmd/gtangle/gtangle.w:177
		w: w,
//line cmd/gtangle/gtangle.w:178
		defs: map[string][]codePiece{},
//line cmd/gtangle/gtangle.w:179
		files: map[string][]codePiece{},
//line cmd/gtangle/gtangle.w:180
	}
//line cmd/gtangle/gtangle.w:181
	for _, s := range w.Sections {
//line cmd/gtangle/gtangle.w:182
		if !s.HasCode {
//line cmd/gtangle/gtangle.w:183
			continue
//line cmd/gtangle/gtangle.w:184
		}
//line cmd/gtangle/gtangle.w:185
		p := codePiece{s.Code, s.CodeLine}
//line cmd/gtangle/gtangle.w:186
		switch {
//line cmd/gtangle/gtangle.w:187
		case s.Name == "":
//line cmd/gtangle/gtangle.w:188
			t.main = append(t.main, p)
//line cmd/gtangle/gtangle.w:189
		case s.IsFile:
//line cmd/gtangle/gtangle.w:190
			t.files[s.Name] = append(t.files[s.Name], p)
//line cmd/gtangle/gtangle.w:191
		default:
//line cmd/gtangle/gtangle.w:192
			name := w.Resolve(s.Name)
//line cmd/gtangle/gtangle.w:193
			t.defs[name] = append(t.defs[name], p)
//line cmd/gtangle/gtangle.w:194
		}
//line cmd/gtangle/gtangle.w:195
	}
//line cmd/gtangle/gtangle.w:196
	return t
//line cmd/gtangle/gtangle.w:197
}

//line cmd/gtangle/gtangle.w:202
func (t *Tangler) Tangle(defaultFile string) ([]Output, error) {
//line cmd/gtangle/gtangle.w:203
	var outs []Output

//line cmd/gtangle/gtangle.w:205
	if nonEmpty(t.main) {
//line cmd/gtangle/gtangle.w:206
		out, err := t.renderOutput(defaultFile, t.main)
//line cmd/gtangle/gtangle.w:207
		if err != nil {
//line cmd/gtangle/gtangle.w:208
			return nil, err
//line cmd/gtangle/gtangle.w:209
		}
//line cmd/gtangle/gtangle.w:210
		outs = append(outs, out)
//line cmd/gtangle/gtangle.w:211
	}

//line cmd/gtangle/gtangle.w:213
	names := make([]string, 0, len(t.files))
//line cmd/gtangle/gtangle.w:214
	for name := range t.files {
//line cmd/gtangle/gtangle.w:215
		names = append(names, name)
//line cmd/gtangle/gtangle.w:216
	}
//line cmd/gtangle/gtangle.w:217
	sort.Strings(names)
//line cmd/gtangle/gtangle.w:218
	for _, name := range names {
//line cmd/gtangle/gtangle.w:219
		out, err := t.renderOutput(name, t.files[name])
//line cmd/gtangle/gtangle.w:220
		if err != nil {
//line cmd/gtangle/gtangle.w:221
			return nil, err
//line cmd/gtangle/gtangle.w:222
		}
//line cmd/gtangle/gtangle.w:223
		outs = append(outs, out)
//line cmd/gtangle/gtangle.w:224
	}

//line cmd/gtangle/gtangle.w:226
	if len(outs) == 0 {
//line cmd/gtangle/gtangle.w:227
		return nil, fmt.Errorf("no code to tangle (no @c or @(file@>= sections)")
//line cmd/gtangle/gtangle.w:228
	}
//line cmd/gtangle/gtangle.w:229
	return outs, nil
//line cmd/gtangle/gtangle.w:230
}

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

//line cmd/gtangle/gtangle.w:249
func (t *Tangler) renderOutput(file string, pieces []codePiece) (Output, error) {
//line cmd/gtangle/gtangle.w:250
	o := &buffer{t: t, atLineStart: true}
//line cmd/gtangle/gtangle.w:251
	if err := t.expandPieces(pieces, o, nil); err != nil {
//line cmd/gtangle/gtangle.w:252
		return Output{}, err
//line cmd/gtangle/gtangle.w:253
	}
//line cmd/gtangle/gtangle.w:254
	raw := o.bytes()
//line cmd/gtangle/gtangle.w:255
	if formatted, err := format.Source(raw); err == nil {
//line cmd/gtangle/gtangle.w:256
		return Output{File: file, Content: formatted}, nil
//line cmd/gtangle/gtangle.w:257
	} else {
//line cmd/gtangle/gtangle.w:258
		return Output{File: file, Content: raw,
//line cmd/gtangle/gtangle.w:259
			Warning: "gofmt could not format the output: " + err.Error()}, nil
//line cmd/gtangle/gtangle.w:260
	}
//line cmd/gtangle/gtangle.w:261
}

//line cmd/gtangle/gtangle.w:265
func (t *Tangler) expandPieces(pieces []codePiece, o *buffer, stack []string) error {
//line cmd/gtangle/gtangle.w:266
	for _, p := range pieces {
//line cmd/gtangle/gtangle.w:267
		if err := t.expand(p.code, p.line, o, stack); err != nil {
//line cmd/gtangle/gtangle.w:268
			return err
//line cmd/gtangle/gtangle.w:269
		}
//line cmd/gtangle/gtangle.w:270
	}
//line cmd/gtangle/gtangle.w:271
	return nil
//line cmd/gtangle/gtangle.w:272
}

//line cmd/gtangle/gtangle.w:279
func (t *Tangler) expand(code string, line int, o *buffer, stack []string) error {
//line cmd/gtangle/gtangle.w:280
	for _, a := range common.ScanCode(code) {
//line cmd/gtangle/gtangle.w:281
		switch a.Kind {
//line cmd/gtangle/gtangle.w:282
		case common.AText, common.AVerbatim:
//line cmd/gtangle/gtangle.w:283
			line = o.writeText(a.Text, line)
//line cmd/gtangle/gtangle.w:284
		case common.APaste:
//line cmd/gtangle/gtangle.w:285
			o.trimRight()
//line cmd/gtangle/gtangle.w:286
			o.pasteNext = true
//line cmd/gtangle/gtangle.w:287
			o.atLineStart = false
//line cmd/gtangle/gtangle.w:288
		case common.ARef:
//line cmd/gtangle/gtangle.w:289

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
			o.newline()
//line cmd/gtangle/gtangle.w:310
			if err := t.expandPieces(def, o, append(stack, name)); err != nil {
//line cmd/gtangle/gtangle.w:311
				return err
//line cmd/gtangle/gtangle.w:312
			}
//line cmd/gtangle/gtangle.w:313
			o.newline()

//line cmd/gtangle/gtangle.w:290
		case common.ATeX, common.AIndex, common.ALayout, common.AIndexDef:
//line cmd/gtangle/gtangle.w:291
			// woven-output only; ignored by tangle
//line cmd/gtangle/gtangle.w:292
		}
//line cmd/gtangle/gtangle.w:293
	}
//line cmd/gtangle/gtangle.w:294
	return nil
//line cmd/gtangle/gtangle.w:295
}

//line cmd/gtangle/gtangle.w:319
type buffer struct {
//line cmd/gtangle/gtangle.w:320
	t *Tangler
//line cmd/gtangle/gtangle.w:321
	b []byte
//line cmd/gtangle/gtangle.w:322
	pasteNext bool
//line cmd/gtangle/gtangle.w:323
	atLineStart bool
//line cmd/gtangle/gtangle.w:324
}

//line cmd/gtangle/gtangle.w:330
func (o *buffer) writeText(s string, line int) int {
//line cmd/gtangle/gtangle.w:331
	if o.pasteNext {
//line cmd/gtangle/gtangle.w:332
		s = strings.TrimLeft(s, " \t\n\r")
//line cmd/gtangle/gtangle.w:333
		o.pasteNext = false
//line cmd/gtangle/gtangle.w:334
	}
//line cmd/gtangle/gtangle.w:335
	for i := 0; i < len(s); i++ {
//line cmd/gtangle/gtangle.w:336
		c := s[i]
//line cmd/gtangle/gtangle.w:337
		if o.atLineStart && c != '\n' {
//line cmd/gtangle/gtangle.w:338
			o.lineMark(line)
//line cmd/gtangle/gtangle.w:339
			o.atLineStart = false
//line cmd/gtangle/gtangle.w:340
		}
//line cmd/gtangle/gtangle.w:341
		o.b = append(o.b, c)
//line cmd/gtangle/gtangle.w:342
		if c == '\n' {
//line cmd/gtangle/gtangle.w:343
			line++
//line cmd/gtangle/gtangle.w:344
			o.atLineStart = true
//line cmd/gtangle/gtangle.w:345
		}
//line cmd/gtangle/gtangle.w:346
	}
//line cmd/gtangle/gtangle.w:347
	return line
//line cmd/gtangle/gtangle.w:348
}

//line cmd/gtangle/gtangle.w:355
func (o *buffer) lineMark(line int) {
//line cmd/gtangle/gtangle.w:356
	file, ln := o.t.w.Origin(line)
//line cmd/gtangle/gtangle.w:357
	o.b = append(o.b, fmt.Sprintf("//line %s:%d\n", file, ln)...)
//line cmd/gtangle/gtangle.w:358
}

//line cmd/gtangle/gtangle.w:360
func (o *buffer) newline() {
//line cmd/gtangle/gtangle.w:361
	o.b = append(o.b, '\n')
//line cmd/gtangle/gtangle.w:362
	o.atLineStart = true
//line cmd/gtangle/gtangle.w:363
}

//line cmd/gtangle/gtangle.w:365
func (o *buffer) trimRight() {
//line cmd/gtangle/gtangle.w:366
	for len(o.b) > 0 {
//line cmd/gtangle/gtangle.w:367
		switch o.b[len(o.b)-1] {
//line cmd/gtangle/gtangle.w:368
		case ' ', '\t', '\n', '\r':
//line cmd/gtangle/gtangle.w:369
			o.b = o.b[:len(o.b)-1]
//line cmd/gtangle/gtangle.w:370
		default:
//line cmd/gtangle/gtangle.w:371
			return
//line cmd/gtangle/gtangle.w:372
		}
//line cmd/gtangle/gtangle.w:373
	}
//line cmd/gtangle/gtangle.w:374
}

//line cmd/gtangle/gtangle.w:376
func (o *buffer) bytes() []byte { return o.b }
