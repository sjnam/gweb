//line cmd/gtangle/gtangle.w:8
package main

//line cmd/gtangle/gtangle.w:10
import (
//line cmd/gtangle/gtangle.w:11
	"flag"
//line cmd/gtangle/gtangle.w:12
	"fmt"
//line cmd/gtangle/gtangle.w:13
	"go/format"
//line cmd/gtangle/gtangle.w:14
	"os"
//line cmd/gtangle/gtangle.w:15
	"path/filepath"
//line cmd/gtangle/gtangle.w:16
	"slices"
//line cmd/gtangle/gtangle.w:17
	"sort"
//line cmd/gtangle/gtangle.w:18
	"strings"

//line cmd/gtangle/gtangle.w:20
	"github.com/sjnam/gweb/common"
//line cmd/gtangle/gtangle.w:21
)

//line cmd/gtangle/gtangle.w:27
func main() {
//line cmd/gtangle/gtangle.w:28
	outDir := flag.String("o", "", "output directory (default: input file's directory)")
//line cmd/gtangle/gtangle.w:29
	showVersion := flag.Bool("version", false, "print version and exit")
//line cmd/gtangle/gtangle.w:30
	flag.Usage = usage
//line cmd/gtangle/gtangle.w:31
	flag.Parse()
//line cmd/gtangle/gtangle.w:32
	if *showVersion {
//line cmd/gtangle/gtangle.w:33
		fmt.Printf("gtangle (GWEB) %s\n", common.Version)
//line cmd/gtangle/gtangle.w:34
		return
//line cmd/gtangle/gtangle.w:35
	}
//line cmd/gtangle/gtangle.w:36
	if flag.NArg() < 1 || flag.NArg() > 2 {
//line cmd/gtangle/gtangle.w:37
		usage()
//line cmd/gtangle/gtangle.w:38
		os.Exit(2)
//line cmd/gtangle/gtangle.w:39
	}
//line cmd/gtangle/gtangle.w:40
	fmt.Fprintf(os.Stderr, "This is GTANGLE, Version %s\n", common.Version)
//line cmd/gtangle/gtangle.w:41
	if err := run(flag.Arg(0), flag.Arg(1), *outDir); err != nil {
//line cmd/gtangle/gtangle.w:42
		fmt.Fprintln(os.Stderr, "gtangle:", err)
//line cmd/gtangle/gtangle.w:43
		os.Exit(1)
//line cmd/gtangle/gtangle.w:44
	}
//line cmd/gtangle/gtangle.w:45
}

//line cmd/gtangle/gtangle.w:49
func usage() {
//line cmd/gtangle/gtangle.w:50
	fmt.Fprintln(os.Stderr, "usage: gtangle [-o dir] file[.w] [change[.ch]]")
//line cmd/gtangle/gtangle.w:51
	flag.PrintDefaults()
//line cmd/gtangle/gtangle.w:52
}

//line cmd/gtangle/gtangle.w:58
func reportProgress(w *common.Web) {
//line cmd/gtangle/gtangle.w:59
	for _, s := range w.Sections {
//line cmd/gtangle/gtangle.w:60
		if s.Starred {
//line cmd/gtangle/gtangle.w:61
			fmt.Fprintf(os.Stderr, "*%d", s.Number)
//line cmd/gtangle/gtangle.w:62
		}
//line cmd/gtangle/gtangle.w:63
	}
//line cmd/gtangle/gtangle.w:64
	fmt.Fprintln(os.Stderr)
//line cmd/gtangle/gtangle.w:65
}

//line cmd/gtangle/gtangle.w:72
func run(input, changeFile, outDir string) error {
//line cmd/gtangle/gtangle.w:73
	input = common.DefaultExt(input, ".w")
//line cmd/gtangle/gtangle.w:74
	changeFile = common.DefaultExt(changeFile, ".ch")
//line cmd/gtangle/gtangle.w:75
	w, err := common.ParseWithChange(input, changeFile)
//line cmd/gtangle/gtangle.w:76
	if err != nil {
//line cmd/gtangle/gtangle.w:77
		return err
//line cmd/gtangle/gtangle.w:78
	}
//line cmd/gtangle/gtangle.w:79
	for _, warn := range w.Warnings {
//line cmd/gtangle/gtangle.w:80
		fmt.Fprintln(os.Stderr, "gtangle: warning:", warn)
//line cmd/gtangle/gtangle.w:81
	}
//line cmd/gtangle/gtangle.w:82
	reportProgress(w)
//line cmd/gtangle/gtangle.w:83
	if outDir == "" {
//line cmd/gtangle/gtangle.w:84
		outDir = filepath.Dir(input)
//line cmd/gtangle/gtangle.w:85
	}

//line cmd/gtangle/gtangle.w:87
	base := filepath.Base(input)
//line cmd/gtangle/gtangle.w:88
	base = strings.TrimSuffix(base, filepath.Ext(base))
//line cmd/gtangle/gtangle.w:89
	defaultFile := base + ".go"

//line cmd/gtangle/gtangle.w:91
	outs, err := New(w).Tangle(defaultFile)
//line cmd/gtangle/gtangle.w:92
	if err != nil {
//line cmd/gtangle/gtangle.w:93
		return err
//line cmd/gtangle/gtangle.w:94
	}

//line cmd/gtangle/gtangle.w:96
	for _, out := range outs {
//line cmd/gtangle/gtangle.w:97
		path := filepath.Join(outDir, out.File)
//line cmd/gtangle/gtangle.w:98
		if dir := filepath.Dir(path); dir != "." {
//line cmd/gtangle/gtangle.w:99
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
//line cmd/gtangle/gtangle.w:100
				return mkErr
//line cmd/gtangle/gtangle.w:101
			}
//line cmd/gtangle/gtangle.w:102
		}
//line cmd/gtangle/gtangle.w:103
		if writeErr := os.WriteFile(path, out.Content, 0o644); writeErr != nil {
//line cmd/gtangle/gtangle.w:104
			return writeErr
//line cmd/gtangle/gtangle.w:105
		}
//line cmd/gtangle/gtangle.w:106
		if out.Warning != "" {
//line cmd/gtangle/gtangle.w:107
			fmt.Fprintf(os.Stderr, "gtangle: warning: %s: %s\n", path, out.Warning)
//line cmd/gtangle/gtangle.w:108
		}
//line cmd/gtangle/gtangle.w:109
		fmt.Printf("gtangle: wrote %s (%d bytes)\n", path, len(out.Content))
//line cmd/gtangle/gtangle.w:110
	}
//line cmd/gtangle/gtangle.w:111
	return nil
//line cmd/gtangle/gtangle.w:112
}

//line cmd/gtangle/gtangle.w:124
type Output struct {
//line cmd/gtangle/gtangle.w:125
	File string
//line cmd/gtangle/gtangle.w:126
	Content []byte
//line cmd/gtangle/gtangle.w:127
	Warning string
//line cmd/gtangle/gtangle.w:128
}

//line cmd/gtangle/gtangle.w:139
type Tangler struct {
//line cmd/gtangle/gtangle.w:140
	w *common.Web
//line cmd/gtangle/gtangle.w:141
	defs map[string][]codePiece // canonical named-section -> code pieces
//line cmd/gtangle/gtangle.w:142
	files map[string][]codePiece // @(file@>= name -> code pieces
//line cmd/gtangle/gtangle.w:143
	main []codePiece // unnamed @c sections, in order
//line cmd/gtangle/gtangle.w:144
}

//line cmd/gtangle/gtangle.w:146
type codePiece struct {
//line cmd/gtangle/gtangle.w:147
	code string
//line cmd/gtangle/gtangle.w:148
	line int
//line cmd/gtangle/gtangle.w:149
}

//line cmd/gtangle/gtangle.w:155
func New(w *common.Web) *Tangler {
//line cmd/gtangle/gtangle.w:156
	t := &Tangler{
//line cmd/gtangle/gtangle.w:157
		w: w,
//line cmd/gtangle/gtangle.w:158
		defs: map[string][]codePiece{},
//line cmd/gtangle/gtangle.w:159
		files: map[string][]codePiece{},
//line cmd/gtangle/gtangle.w:160
	}
//line cmd/gtangle/gtangle.w:161
	for _, s := range w.Sections {
//line cmd/gtangle/gtangle.w:162
		if !s.HasCode {
//line cmd/gtangle/gtangle.w:163
			continue
//line cmd/gtangle/gtangle.w:164
		}
//line cmd/gtangle/gtangle.w:165
		p := codePiece{s.Code, s.CodeLine}
//line cmd/gtangle/gtangle.w:166
		switch {
//line cmd/gtangle/gtangle.w:167
		case s.Name == "":
//line cmd/gtangle/gtangle.w:168
			t.main = append(t.main, p)
//line cmd/gtangle/gtangle.w:169
		case s.IsFile:
//line cmd/gtangle/gtangle.w:170
			t.files[s.Name] = append(t.files[s.Name], p)
//line cmd/gtangle/gtangle.w:171
		default:
//line cmd/gtangle/gtangle.w:172
			name := w.Resolve(s.Name)
//line cmd/gtangle/gtangle.w:173
			t.defs[name] = append(t.defs[name], p)
//line cmd/gtangle/gtangle.w:174
		}
//line cmd/gtangle/gtangle.w:175
	}
//line cmd/gtangle/gtangle.w:176
	return t
//line cmd/gtangle/gtangle.w:177
}

//line cmd/gtangle/gtangle.w:182
func (t *Tangler) Tangle(defaultFile string) ([]Output, error) {
//line cmd/gtangle/gtangle.w:183
	var outs []Output

//line cmd/gtangle/gtangle.w:185
	if nonEmpty(t.main) {
//line cmd/gtangle/gtangle.w:186
		out, err := t.renderOutput(defaultFile, t.main)
//line cmd/gtangle/gtangle.w:187
		if err != nil {
//line cmd/gtangle/gtangle.w:188
			return nil, err
//line cmd/gtangle/gtangle.w:189
		}
//line cmd/gtangle/gtangle.w:190
		outs = append(outs, out)
//line cmd/gtangle/gtangle.w:191
	}

//line cmd/gtangle/gtangle.w:193
	names := make([]string, 0, len(t.files))
//line cmd/gtangle/gtangle.w:194
	for name := range t.files {
//line cmd/gtangle/gtangle.w:195
		names = append(names, name)
//line cmd/gtangle/gtangle.w:196
	}
//line cmd/gtangle/gtangle.w:197
	sort.Strings(names)
//line cmd/gtangle/gtangle.w:198
	for _, name := range names {
//line cmd/gtangle/gtangle.w:199
		out, err := t.renderOutput(name, t.files[name])
//line cmd/gtangle/gtangle.w:200
		if err != nil {
//line cmd/gtangle/gtangle.w:201
			return nil, err
//line cmd/gtangle/gtangle.w:202
		}
//line cmd/gtangle/gtangle.w:203
		outs = append(outs, out)
//line cmd/gtangle/gtangle.w:204
	}

//line cmd/gtangle/gtangle.w:206
	if len(outs) == 0 {
//line cmd/gtangle/gtangle.w:207
		return nil, fmt.Errorf("no code to tangle (no @c or @(file@>= sections)")
//line cmd/gtangle/gtangle.w:208
	}
//line cmd/gtangle/gtangle.w:209
	return outs, nil
//line cmd/gtangle/gtangle.w:210
}

//line cmd/gtangle/gtangle.w:215
func nonEmpty(pieces []codePiece) bool {
//line cmd/gtangle/gtangle.w:216
	for _, p := range pieces {
//line cmd/gtangle/gtangle.w:217
		if strings.TrimSpace(p.code) != "" {
//line cmd/gtangle/gtangle.w:218
			return true
//line cmd/gtangle/gtangle.w:219
		}
//line cmd/gtangle/gtangle.w:220
	}
//line cmd/gtangle/gtangle.w:221
	return false
//line cmd/gtangle/gtangle.w:222
}

//line cmd/gtangle/gtangle.w:229
func (t *Tangler) renderOutput(file string, pieces []codePiece) (Output, error) {
//line cmd/gtangle/gtangle.w:230
	o := &buffer{t: t, atLineStart: true}
//line cmd/gtangle/gtangle.w:231
	if err := t.expandPieces(pieces, o, nil); err != nil {
//line cmd/gtangle/gtangle.w:232
		return Output{}, err
//line cmd/gtangle/gtangle.w:233
	}
//line cmd/gtangle/gtangle.w:234
	raw := o.bytes()
//line cmd/gtangle/gtangle.w:235
	if formatted, err := format.Source(raw); err == nil {
//line cmd/gtangle/gtangle.w:236
		return Output{File: file, Content: formatted}, nil
//line cmd/gtangle/gtangle.w:237
	} else {
//line cmd/gtangle/gtangle.w:238
		return Output{File: file, Content: raw,
//line cmd/gtangle/gtangle.w:239
			Warning: "gofmt could not format the output: " + err.Error()}, nil
//line cmd/gtangle/gtangle.w:240
	}
//line cmd/gtangle/gtangle.w:241
}

//line cmd/gtangle/gtangle.w:248
func (t *Tangler) expandPieces(pieces []codePiece, o *buffer, stack []string) error {
//line cmd/gtangle/gtangle.w:249
	for _, p := range pieces {
//line cmd/gtangle/gtangle.w:250
		if err := t.expand(p.code, p.line, o, stack); err != nil {
//line cmd/gtangle/gtangle.w:251
			return err
//line cmd/gtangle/gtangle.w:252
		}
//line cmd/gtangle/gtangle.w:253
	}
//line cmd/gtangle/gtangle.w:254
	return nil
//line cmd/gtangle/gtangle.w:255
}

//line cmd/gtangle/gtangle.w:257
func (t *Tangler) expand(code string, line int, o *buffer, stack []string) error {
//line cmd/gtangle/gtangle.w:258
	for _, a := range common.ScanCode(code) {
//line cmd/gtangle/gtangle.w:259
		switch a.Kind {
//line cmd/gtangle/gtangle.w:260
		case common.AText, common.AVerbatim:
//line cmd/gtangle/gtangle.w:261
			line = o.writeText(a.Text, line)
//line cmd/gtangle/gtangle.w:262
		case common.APaste:
//line cmd/gtangle/gtangle.w:263
			o.trimRight()
//line cmd/gtangle/gtangle.w:264
			o.pasteNext = true
//line cmd/gtangle/gtangle.w:265
			o.atLineStart = false
//line cmd/gtangle/gtangle.w:266
		case common.ARef:
//line cmd/gtangle/gtangle.w:267
			name := t.w.Resolve(a.Text)
//line cmd/gtangle/gtangle.w:268
			def, ok := t.defs[name]
//line cmd/gtangle/gtangle.w:269
			if !ok {
//line cmd/gtangle/gtangle.w:270
				return fmt.Errorf("undefined section <%s>", a.Text)
//line cmd/gtangle/gtangle.w:271
			}
//line cmd/gtangle/gtangle.w:272
			if slices.Contains(stack, name) {
//line cmd/gtangle/gtangle.w:273
				return fmt.Errorf("circular reference through <%s>", name)
//line cmd/gtangle/gtangle.w:274
			}
//line cmd/gtangle/gtangle.w:275
			// Surround an expanded reference with newlines so adjacent
//line cmd/gtangle/gtangle.w:276
			// statements stay on separate lines; gofmt collapses the rest.
//line cmd/gtangle/gtangle.w:277
			o.newline()
//line cmd/gtangle/gtangle.w:278
			if err := t.expandPieces(def, o, append(stack, name)); err != nil {
//line cmd/gtangle/gtangle.w:279
				return err
//line cmd/gtangle/gtangle.w:280
			}
//line cmd/gtangle/gtangle.w:281
			o.newline()
//line cmd/gtangle/gtangle.w:282
		case common.ATeX, common.AIndex, common.ALayout, common.AIndexDef:
//line cmd/gtangle/gtangle.w:283
			// woven-output only; ignored by tangle
//line cmd/gtangle/gtangle.w:284
		}
//line cmd/gtangle/gtangle.w:285
	}
//line cmd/gtangle/gtangle.w:286
	return nil
//line cmd/gtangle/gtangle.w:287
}

//line cmd/gtangle/gtangle.w:293
type buffer struct {
//line cmd/gtangle/gtangle.w:294
	t *Tangler
//line cmd/gtangle/gtangle.w:295
	b []byte
//line cmd/gtangle/gtangle.w:296
	pasteNext bool
//line cmd/gtangle/gtangle.w:297
	atLineStart bool
//line cmd/gtangle/gtangle.w:298
}

//line cmd/gtangle/gtangle.w:300
func (o *buffer) writeText(s string, line int) int {
//line cmd/gtangle/gtangle.w:301
	if o.pasteNext {
//line cmd/gtangle/gtangle.w:302
		s = strings.TrimLeft(s, " \t\n\r")
//line cmd/gtangle/gtangle.w:303
		o.pasteNext = false
//line cmd/gtangle/gtangle.w:304
	}
//line cmd/gtangle/gtangle.w:305
	for i := 0; i < len(s); i++ {
//line cmd/gtangle/gtangle.w:306
		c := s[i]
//line cmd/gtangle/gtangle.w:307
		if o.atLineStart && c != '\n' {
//line cmd/gtangle/gtangle.w:308
			o.lineMark(line)
//line cmd/gtangle/gtangle.w:309
			o.atLineStart = false
//line cmd/gtangle/gtangle.w:310
		}
//line cmd/gtangle/gtangle.w:311
		o.b = append(o.b, c)
//line cmd/gtangle/gtangle.w:312
		if c == '\n' {
//line cmd/gtangle/gtangle.w:313
			line++
//line cmd/gtangle/gtangle.w:314
			o.atLineStart = true
//line cmd/gtangle/gtangle.w:315
		}
//line cmd/gtangle/gtangle.w:316
	}
//line cmd/gtangle/gtangle.w:317
	return line
//line cmd/gtangle/gtangle.w:318
}

//line cmd/gtangle/gtangle.w:320
func (o *buffer) lineMark(line int) {
//line cmd/gtangle/gtangle.w:321
	file, ln := o.t.w.Origin(line)
//line cmd/gtangle/gtangle.w:322
	o.b = append(o.b, fmt.Sprintf("//line %s:%d\n", file, ln)...)
//line cmd/gtangle/gtangle.w:323
}

//line cmd/gtangle/gtangle.w:325
func (o *buffer) newline() {
//line cmd/gtangle/gtangle.w:326
	o.b = append(o.b, '\n')
//line cmd/gtangle/gtangle.w:327
	o.atLineStart = true
//line cmd/gtangle/gtangle.w:328
}

//line cmd/gtangle/gtangle.w:330
func (o *buffer) trimRight() {
//line cmd/gtangle/gtangle.w:331
	for len(o.b) > 0 {
//line cmd/gtangle/gtangle.w:332
		switch o.b[len(o.b)-1] {
//line cmd/gtangle/gtangle.w:333
		case ' ', '\t', '\n', '\r':
//line cmd/gtangle/gtangle.w:334
			o.b = o.b[:len(o.b)-1]
//line cmd/gtangle/gtangle.w:335
		default:
//line cmd/gtangle/gtangle.w:336
			return
//line cmd/gtangle/gtangle.w:337
		}
//line cmd/gtangle/gtangle.w:338
	}
//line cmd/gtangle/gtangle.w:339
}

//line cmd/gtangle/gtangle.w:341
func (o *buffer) bytes() []byte { return o.b }
