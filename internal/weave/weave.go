// Package weave implements gweave: it turns a GWEB web into a TeX document with
// pretty-printed Go code (bold reserved words, italic identifiers), linked
// section references, and (see xref.go) cross-references and an index. It is the
// Go analogue of CWEB's cweave.
//
//line lit/weave.w:7
//line lit/weave.w:8
//line lit/weave.w:9
//line lit/weave.w:10
//line lit/weave.w:11
package weave

//line lit/weave.w:13
import (
//line lit/weave.w:14
	"bufio"
//line lit/weave.w:15
	"fmt"
//line lit/weave.w:16
	"io"
//line lit/weave.w:17
	"strings"

//line lit/weave.w:19
	"github.com/sjnam/gweb/internal/web"
//line lit/weave.w:20
)

// Weaver turns a parsed web into woven TeX.
//
//line lit/weave.w:26
//line lit/weave.w:27
type Weaver struct {
//line lit/weave.w:28
	w *web.Web
//line lit/weave.w:29
	defNum map[string]int // canonical named-section -> first defining section

//line lit/weave.w:31
	format map[string]tokKind // @f/@s: identifier -> the token class to use
//line lit/weave.w:32
	noIndex map[string]bool // @s: identifiers omitted from the index
//line lit/weave.w:33
	isFile map[string]bool // @(file@>= outputs: names are literal file paths

//line lit/weave.w:35
	xref *xref // identifier and section cross-references (built lazily)
//line lit/weave.w:36
}

// New builds a Weaver for the given web.
//
//line lit/weave.w:41
//line lit/weave.w:42
func New(w *web.Web) *Weaver {
//line lit/weave.w:43
	wv := &Weaver{
//line lit/weave.w:44
		w: w,
//line lit/weave.w:45
		defNum: map[string]int{},
//line lit/weave.w:46
		format: map[string]tokKind{},
//line lit/weave.w:47
		noIndex: map[string]bool{},
//line lit/weave.w:48
		isFile: map[string]bool{},
//line lit/weave.w:49
	}
//line lit/weave.w:50
	// Both refinements and @(file@>= outputs get a defining section number, so
//line lit/weave.w:51
	// their headlines and links resolve; only @(file@>= names are never the
//line lit/weave.w:52
	// target of a @<...@> reference. A file name is a literal path, not TeX, so
//line lit/weave.w:53
	// we remember which names are files and typeset them verbatim.
//line lit/weave.w:54
	for _, s := range w.Sections {
//line lit/weave.w:55
		if s.HasCode && s.Name != "" {
//line lit/weave.w:56
			name := w.Resolve(s.Name)
//line lit/weave.w:57
			if _, ok := wv.defNum[name]; !ok {
//line lit/weave.w:58
				wv.defNum[name] = s.Number
//line lit/weave.w:59
			}
//line lit/weave.w:60
			if s.IsFile {
//line lit/weave.w:61
				wv.isFile[name] = true
//line lit/weave.w:62
			}
//line lit/weave.w:63
		}
//line lit/weave.w:64
	}
//line lit/weave.w:65
	// Format directives apply globally; later definitions win. The display
//line lit/weave.w:66
	// class of identifier a (@f a b) is the class b would be typeset in.
//line lit/weave.w:67
	apply := func(fs []web.Format) {
//line lit/weave.w:68
		for _, f := range fs {
//line lit/weave.w:69
			if f.Macro {
//line lit/weave.w:70
				wv.format[f.Original] = tkMacro // : typewriter, like a CWEB macro
//line lit/weave.w:71
			} else {
//line lit/weave.w:72
				wv.format[f.Original] = classifyWord(f.Like)
//line lit/weave.w:73
			}
//line lit/weave.w:74
			if f.NoIndex {
//line lit/weave.w:75
				wv.noIndex[f.Original] = true
//line lit/weave.w:76
			}
//line lit/weave.w:77
		}
//line lit/weave.w:78
	}
//line lit/weave.w:79
	apply(w.Formats)
//line lit/weave.w:80
	for _, s := range w.Sections {
//line lit/weave.w:81
		apply(s.Formats)
//line lit/weave.w:82
	}
//line lit/weave.w:83
	// As in cweave, a name declared with |type| is set bold, like the predeclared
//line lit/weave.w:84
	// types. (Typewriter treatment is applied only on request, with |@d|.) An
//line lit/weave.w:85
	// explicit |@f|/|@s| above still wins.
//line lit/weave.w:86
	wv.detectDecls("type", tkBuiltin)
//line lit/weave.w:87
	return wv
//line lit/weave.w:88
}

//line lit/weave.w:98
func (wv *Weaver) detectDecls(keyword string, kind tokKind) {
//line lit/weave.w:99
	add := func(name string) {
//line lit/weave.w:100
		if name == "" || name == "_" {
//line lit/weave.w:101
			return
//line lit/weave.w:102
		}
//line lit/weave.w:103
		if _, ok := wv.format[name]; !ok {
//line lit/weave.w:104
			wv.format[name] = kind
//line lit/weave.w:105
		}
//line lit/weave.w:106
	}
//line lit/weave.w:107
	for _, s := range wv.w.Sections {
//line lit/weave.w:108
		if !s.HasCode {
//line lit/weave.w:109
			continue
//line lit/weave.w:110
		}
//line lit/weave.w:111
		var st lexState
//line lit/weave.w:112
		for _, a := range web.ScanCode(s.Code) {
//line lit/weave.w:113
			if a.Kind == web.AText {
//line lit/weave.w:114
				scanDecls(lexGo(a.Text, &st), keyword, add)
//line lit/weave.w:115
			}
//line lit/weave.w:116
		}
//line lit/weave.w:117
	}
//line lit/weave.w:118
}

//line lit/weave.w:127
func scanDecls(toks []token, keyword string, add func(string)) {
//line lit/weave.w:128
	for i := 0; i < len(toks); i++ {
//line lit/weave.w:129
		if toks[i].kind != tkKeyword || toks[i].text != keyword {
//line lit/weave.w:130
			continue
//line lit/weave.w:131
		}
//line lit/weave.w:132
		j := nextSignificant(toks, i+1)
//line lit/weave.w:133
		if j < 0 {
//line lit/weave.w:134
			return
//line lit/weave.w:135
		}
//line lit/weave.w:136
		if toks[j].kind == tkOp && toks[j].text == "(" {
//line lit/weave.w:137
			i = scanDeclGroup(toks, j+1, add)
//line lit/weave.w:138
		} else if toks[j].kind == tkIdent {
//line lit/weave.w:139
			add(toks[j].text)
//line lit/weave.w:140
		}
//line lit/weave.w:141
	}
//line lit/weave.w:142
}

// nextSignificant returns the index of the first token at or after i that is not
// whitespace or a newline, or -1 if there is none.
//
//line lit/weave.w:144
//line lit/weave.w:145
//line lit/weave.w:146
func nextSignificant(toks []token, i int) int {
//line lit/weave.w:147
	for ; i < len(toks); i++ {
//line lit/weave.w:148
		if toks[i].kind != tkSpace && toks[i].kind != tkNewline {
//line lit/weave.w:149
			return i
//line lit/weave.w:150
		}
//line lit/weave.w:151
	}
//line lit/weave.w:152
	return -1
//line lit/weave.w:153
}

// scanDeclGroup collects the declared names in a parenthesized type or const
// group beginning at index i, returning the index of the closing ")". The first
// identifier on each line at nesting depth 0 is a declared name.
//
//line lit/weave.w:155
//line lit/weave.w:156
//line lit/weave.w:157
//line lit/weave.w:158
func scanDeclGroup(toks []token, i int, add func(string)) int {
//line lit/weave.w:159
	depth := 0
//line lit/weave.w:160
	atStart := true
//line lit/weave.w:161
	for ; i < len(toks); i++ {
//line lit/weave.w:162
		switch t := toks[i]; t.kind {
//line lit/weave.w:163
		case tkNewline:
//line lit/weave.w:164
			if depth == 0 {
//line lit/weave.w:165
				atStart = true
//line lit/weave.w:166
			}
//line lit/weave.w:167
		case tkSpace:
//line lit/weave.w:168
			// keep atStart
//line lit/weave.w:169
		case tkOp:
//line lit/weave.w:170
			switch t.text {
//line lit/weave.w:171
			case "(", "{", "[":
//line lit/weave.w:172
				depth++
//line lit/weave.w:173
			case ")":
//line lit/weave.w:174
				if depth == 0 {
//line lit/weave.w:175
					return i
//line lit/weave.w:176
				}
//line lit/weave.w:177
				depth--
//line lit/weave.w:178
			case "}", "]":
//line lit/weave.w:179
				if depth > 0 {
//line lit/weave.w:180
					depth--
//line lit/weave.w:181
				}
//line lit/weave.w:182
			}
//line lit/weave.w:183
			atStart = false
//line lit/weave.w:184
		default:
//line lit/weave.w:185
			if atStart && depth == 0 && t.kind == tkIdent {
//line lit/weave.w:186
				add(t.text)
//line lit/weave.w:187
			}
//line lit/weave.w:188
			atStart = false
//line lit/weave.w:189
		}
//line lit/weave.w:190
	}
//line lit/weave.w:191
	return i
//line lit/weave.w:192
}

// effKind returns the token class to typeset t in, honoring @f/@s overrides.
//
//line lit/weave.w:197
//line lit/weave.w:198
func (wv *Weaver) effKind(t token) tokKind {
//line lit/weave.w:199
	switch t.kind {
//line lit/weave.w:200
	case tkIdent, tkKeyword, tkBuiltin:
//line lit/weave.w:201
		if k, ok := wv.format[t.text]; ok {
//line lit/weave.w:202
			return k
//line lit/weave.w:203
		}
//line lit/weave.w:204
	}
//line lit/weave.w:205
	return t.kind
//line lit/weave.w:206
}

// Weave writes the complete TeX document to out. It runs two passes: the first
// is discarded and only populates the cross-reference tables (so that, e.g.,
// "used in section N" notes can be printed under a definition even when the use
// occurs later); the second produces the real output.
//
//line lit/weave.w:213
//line lit/weave.w:214
//line lit/weave.w:215
//line lit/weave.w:216
//line lit/weave.w:217
func (wv *Weaver) Weave(out io.Writer) error {
//line lit/weave.w:218
	wv.xref = newXref()
//line lit/weave.w:219
	scan := bufio.NewWriter(io.Discard)
//line lit/weave.w:220
	for _, sec := range wv.w.Sections {
//line lit/weave.w:221
		wv.writeSection(scan, sec)
//line lit/weave.w:222
	}

//line lit/weave.w:224
	bw := bufio.NewWriter(out)
//line lit/weave.w:225
	// gweave supplies the macro package itself, so a .w file need not (and
//line lit/weave.w:226
	// should not) \input it; drop any stray copy from the limbo.
//line lit/weave.w:227
	bw.WriteString("\\input gwebmac\n")
//line lit/weave.w:228
	bw.WriteString(stripGwebmacInput(wv.w.Limbo))
//line lit/weave.w:229
	for _, sec := range wv.w.Sections {
//line lit/weave.w:230
		wv.writeSection(bw, sec)
//line lit/weave.w:231
	}
//line lit/weave.w:232
	wv.writeBackMatter(bw)
//line lit/weave.w:233
	return bw.Flush()
//line lit/weave.w:234
}

// stripGwebmacInput removes any "\input gwebmac" line from the limbo, since
// gweave now emits it automatically.
//
//line lit/weave.w:239
//line lit/weave.w:240
//line lit/weave.w:241
func stripGwebmacInput(limbo string) string {
//line lit/weave.w:242
	lines := strings.Split(limbo, "\n")
//line lit/weave.w:243
	kept := make([]string, 0, len(lines))
//line lit/weave.w:244
	for _, ln := range lines {
//line lit/weave.w:245
		if strings.TrimSpace(ln) == "\\input gwebmac" {
//line lit/weave.w:246
			continue
//line lit/weave.w:247
		}
//line lit/weave.w:248
		kept = append(kept, ln)
//line lit/weave.w:249
	}
//line lit/weave.w:250
	return strings.Join(kept, "\n")
//line lit/weave.w:251
}

//line lit/weave.w:257
func (wv *Weaver) writeSection(bw *bufio.Writer, sec *web.Section) {
//line lit/weave.w:258
	if sec.Starred {
//line lit/weave.w:259
		// A starred-section title is free TeX (it may contain \. typewriter and
//line lit/weave.w:260
		// other control sequences), so it is passed through processTex rather than
//line lit/weave.w:261
		// escaped like a refinement name.
//line lit/weave.w:262
		fmt.Fprintf(bw, "\n\\GN{%d}{%d}{%s}", sec.Depth, sec.Number, wv.processTex(sec.Number, sec.Title))
//line lit/weave.w:263
		// The commentary is whatever follows the title's terminating period (the
//line lit/weave.w:264
		// first period at end of text or followed by whitespace, matching the web
//line lit/weave.w:265
		// package's title rule so a period inside \. does not split early).
//line lit/weave.w:266
		rest := sec.Tex
//line lit/weave.w:267
		for i := 0; i < len(rest); i++ {
//line lit/weave.w:268
			if rest[i] == '.' && (i+1 == len(rest) || rest[i+1] == ' ' ||
//line lit/weave.w:269
				rest[i+1] == '\t' || rest[i+1] == '\n' || rest[i+1] == '\r') {
//line lit/weave.w:270
				rest = rest[i+1:]
//line lit/weave.w:271
				break
//line lit/weave.w:272
			}
//line lit/weave.w:273
		}
//line lit/weave.w:274
		bw.WriteString(wv.processTex(sec.Number, rest))
//line lit/weave.w:275
	} else {
//line lit/weave.w:276
		fmt.Fprintf(bw, "\n\\GM{%d}", sec.Number)
//line lit/weave.w:277
		bw.WriteString(wv.processTex(sec.Number, sec.Tex))
//line lit/weave.w:278
	}

//line lit/weave.w:280
	if sec.HasCode {
//line lit/weave.w:281
		// A code-only section (no commentary) runs in on the section-number line,
//line lit/weave.w:282
		// as cweave does: a named section's header (\GDr/\GDpr) and an unnamed
//line lit/weave.w:283
		// section's first code line (\GBr + \GLr) omit the usual break.
//line lit/weave.w:284
		runin := !sec.Starred && strings.TrimSpace(sec.Tex) == ""
//line lit/weave.w:285
		if sec.Name != "" {
//line lit/weave.w:286
			name := wv.w.Resolve(sec.Name)
//line lit/weave.w:287
			cont := wv.defNum[name] != sec.Number
//line lit/weave.w:288
			wv.xref.addSectionDef(name, sec.Number)
//line lit/weave.w:289
			macro := "\\GD"
//line lit/weave.w:290
			if cont {
//line lit/weave.w:291
				macro = "\\GDp" // continuation of an earlier definition
//line lit/weave.w:292
			}
//line lit/weave.w:293
			if runin {
//line lit/weave.w:294
				macro += "r"
//line lit/weave.w:295
			}
//line lit/weave.w:296
			fmt.Fprintf(bw, "\n%s{%d}{%s}", macro, wv.defNum[name], wv.renderName(name))
//line lit/weave.w:297
		}
//line lit/weave.w:298
		// With a named header the code always starts below it, so only an unnamed
//line lit/weave.w:299
		// code-only section runs its first code line in beside the number.
//line lit/weave.w:300
		runinCode := runin && sec.Name == ""
//line lit/weave.w:301
		if runinCode {
//line lit/weave.w:302
			bw.WriteString("\n\\GBr%\n")
//line lit/weave.w:303
		} else {
//line lit/weave.w:304
			bw.WriteString("\n\\GB%\n")
//line lit/weave.w:305
		}
//line lit/weave.w:306
		bw.WriteString(wv.renderCode(sec.Number, sec.Code, runinCode))
//line lit/weave.w:307
		bw.WriteString("\\GE\n")
//line lit/weave.w:308
		if sec.Name != "" {
//line lit/weave.w:309
			bw.WriteString(wv.crossRefNotes(wv.w.Resolve(sec.Name), sec.Number))
//line lit/weave.w:310
		}
//line lit/weave.w:311
	}
//line lit/weave.w:312
}

// renderCode formats a code part into a sequence of \GL code lines. Spacing
// mirrors the source: a run of tokens with no source whitespace between them
// becomes one tight math "chunk" (one TeX math group), and a gap becomes a
// breakable \GS space between chunks. Because gofmt-formatted Go already encodes
// the grammar in its spacing, this reproduces it exactly (pointer *T vs a * b,
// slice []T vs index a[i], and so on) and lets long lines wrap at \GS.
//
//line lit/weave.w:321
//line lit/weave.w:322
//line lit/weave.w:323
//line lit/weave.w:324
//line lit/weave.w:325
//line lit/weave.w:326
//line lit/weave.w:327
func (wv *Weaver) renderCode(secNum int, code string, runin bool) string {
//line lit/weave.w:328
	var out strings.Builder
//line lit/weave.w:329
	var line strings.Builder // the current source line: chunks joined by \GS
//line lit/weave.w:330
	var run strings.Builder // the current tight chunk (one TeX math group)
//line lit/weave.w:331
	var st lexState
//line lit/weave.w:332
	indent := 0
//line lit/weave.w:333
	atLineStart := true
//line lit/weave.w:334
	pendingSpace := false
//line lit/weave.w:335
	forceDef := false // set by @! to force the next identifier to index as a def
//line lit/weave.w:336
	haveContent := false // at least one code line has been emitted
//line lit/weave.w:337
	blankPending := false // a blank source line is waiting to become a \GBK gap

//line lit/weave.w:339
	// prevSig* tracks the most recent significant token so that an identifier
//line lit/weave.w:340
	// following func/var/const/type can be flagged as a definition.
//line lit/weave.w:341
	prevSigKind := tkNewline
//line lit/weave.w:342
	prevSigText := ""

//line lit/weave.w:344
	flushRun := func() {
//line lit/weave.w:345
		if run.Len() > 0 {
//line lit/weave.w:346
			line.WriteString("$")
//line lit/weave.w:347
			line.WriteString(run.String())
//line lit/weave.w:348
			line.WriteString("$")
//line lit/weave.w:349
			run.Reset()
//line lit/weave.w:350
		}
//line lit/weave.w:351
	}
//line lit/weave.w:352
	emit := func(s string) {
//line lit/weave.w:353
		if pendingSpace {
//line lit/weave.w:354
			flushRun()
//line lit/weave.w:355
			line.WriteString("\\GS ")
//line lit/weave.w:356
			pendingSpace = false
//line lit/weave.w:357
		}
//line lit/weave.w:358
		run.WriteString(s)
//line lit/weave.w:359
		atLineStart = false
//line lit/weave.w:360
	}
//line lit/weave.w:361
	// emitLine writes the accumulated line as a \GL but leaves indent intact. A
//line lit/weave.w:362
	// blank source line between two code lines becomes a small \GBK gap, which
//line lit/weave.w:363
	// gives a little air between, e.g., the import block and the function body.
//line lit/weave.w:364
	emitLine := func() {
//line lit/weave.w:365
		flushRun()
//line lit/weave.w:366
		if strings.TrimSpace(line.String()) != "" {
//line lit/weave.w:367
			if blankPending {
//line lit/weave.w:368
				out.WriteString("\\GBK\n")
//line lit/weave.w:369
				blankPending = false
//line lit/weave.w:370
			}
//line lit/weave.w:371
			// The first line of an unnamed code-only section runs in beside the
//line lit/weave.w:372
			// section number (\GLr, no break); the rest are ordinary \GL lines.
//line lit/weave.w:373
			macro := "GL"
//line lit/weave.w:374
			if runin && !haveContent {
//line lit/weave.w:375
				macro = "GLr"
//line lit/weave.w:376
			}
//line lit/weave.w:377
			fmt.Fprintf(&out, "\\%s{%d}{%s}%%\n", macro, indent, line.String())
//line lit/weave.w:378
			haveContent = true
//line lit/weave.w:379
		} else if haveContent {
//line lit/weave.w:380
			blankPending = true
//line lit/weave.w:381
		}
//line lit/weave.w:382
		line.Reset()
//line lit/weave.w:383
	}
//line lit/weave.w:384
	// flushLine ends a source line.
//line lit/weave.w:385
	flushLine := func() {
//line lit/weave.w:386
		emitLine()
//line lit/weave.w:387
		indent = 0
//line lit/weave.w:388
		atLineStart = true
//line lit/weave.w:389
		pendingSpace = false
//line lit/weave.w:390
	}
//line lit/weave.w:391
	// forceBreak starts a fresh woven line at the same indent (@/), optionally
//line lit/weave.w:392
	// preceded by a blank line (@#).
//line lit/weave.w:393
	forceBreak := func(blank bool) {
//line lit/weave.w:394
		emitLine()
//line lit/weave.w:395
		if blank {
//line lit/weave.w:396
			out.WriteString("\\GBL\n")
//line lit/weave.w:397
		}
//line lit/weave.w:398
		atLineStart = false
//line lit/weave.w:399
		pendingSpace = false
//line lit/weave.w:400
	}

//line lit/weave.w:402
	for _, a := range web.ScanCode(code) {
//line lit/weave.w:403
		switch a.Kind {
//line lit/weave.w:404
		case web.AText:
//line lit/weave.w:405
			toks := lexGo(a.Text, &st)
//line lit/weave.w:406
			for k, t := range toks {
//line lit/weave.w:407
				switch t.kind {
//line lit/weave.w:408
				case tkNewline:
//line lit/weave.w:409
					flushLine()
//line lit/weave.w:410
				case tkSpace:
//line lit/weave.w:411
					if atLineStart {
//line lit/weave.w:412
						indent += indentLevel(t.text)
//line lit/weave.w:413
					} else {
//line lit/weave.w:414
						pendingSpace = true
//line lit/weave.w:415
					}
//line lit/weave.w:416
				default:
//line lit/weave.w:417
					if t.kind == tkIdent || t.kind == tkBuiltin {
//line lit/weave.w:418
						def := forceDef || isDefinition(prevSigKind, prevSigText, toks, k)
//line lit/weave.w:419
						forceDef = false
//line lit/weave.w:420
						if indexable(t.text) && !wv.noIndex[t.text] {
//line lit/weave.w:421
							if def {
//line lit/weave.w:422
								wv.xref.addIdentDef(t.text, secNum)
//line lit/weave.w:423
							} else {
//line lit/weave.w:424
								wv.xref.addIdentUse(t.text, secNum)
//line lit/weave.w:425
							}
//line lit/weave.w:426
						}
//line lit/weave.w:427
					}
//line lit/weave.w:428
					// A thin space (\Gthin, a tunable muskip) before a "(" that
//line lit/weave.w:429
					// directly follows a word (a function name, or a keyword like
//line lit/weave.w:430
					// func), as cweave does, so the paren does not jam against it:
//line lit/weave.w:431
					// f (x), func (...).
//line lit/weave.w:432
					if t.kind == tkOp && t.text == "(" && !pendingSpace && !atLineStart &&
//line lit/weave.w:433
						(prevSigKind == tkIdent || prevSigKind == tkKeyword || prevSigKind == tkBuiltin) {
//line lit/weave.w:434
						emit("\\Gthin ")
//line lit/weave.w:435
					}
//line lit/weave.w:436
					if t.kind == tkComment {
//line lit/weave.w:437
						emit(wv.renderComment(secNum, t.text))
//line lit/weave.w:438
					} else {
//line lit/weave.w:439
						emit(renderToken(token{kind: wv.effKind(t), text: t.text}))
//line lit/weave.w:440
					}
//line lit/weave.w:441
					prevSigKind, prevSigText = t.kind, t.text
//line lit/weave.w:442
				}
//line lit/weave.w:443
			}
//line lit/weave.w:444
		case web.ARef:
//line lit/weave.w:445
			name := wv.w.Resolve(a.Text)
//line lit/weave.w:446
			wv.xref.addSectionUse(name, secNum)
//line lit/weave.w:447
			emit(fmt.Sprintf("\\GX{%d}{%s}", wv.defNum[name], wv.renderName(name)))
//line lit/weave.w:448
		case web.AVerbatim:
//line lit/weave.w:449
			emit(fmt.Sprintf("\\GST{%s}", escTT(a.Text)))
//line lit/weave.w:450
		case web.ATeX:
//line lit/weave.w:451
			emit(a.Text)
//line lit/weave.w:452
		case web.AIndex:
//line lit/weave.w:453
			wv.xref.addManualIndex(a.Index, a.Text, secNum)
//line lit/weave.w:454
		case web.APaste:
//line lit/weave.w:455
			pendingSpace = false // join: no space before the next token
//line lit/weave.w:456
		case web.ALayout:
//line lit/weave.w:457
			switch a.Index {
//line lit/weave.w:458
			case ',': // thin space, stays within the current chunk
//line lit/weave.w:459
				emit("\\,")
//line lit/weave.w:460
			case '/': // force a line break at the same indent
//line lit/weave.w:461
				forceBreak(false)
//line lit/weave.w:462
			case '#': // force a line break preceded by a blank line
//line lit/weave.w:463
				forceBreak(true)
//line lit/weave.w:464
			case '|': // optional (zero-width) line break between chunks
//line lit/weave.w:465
				flushRun()
//line lit/weave.w:466
				line.WriteString("\\GSO ")
//line lit/weave.w:467
				pendingSpace = false
//line lit/weave.w:468
				atLineStart = false
//line lit/weave.w:469
			}
//line lit/weave.w:470
		case web.AIndexDef:
//line lit/weave.w:471
			forceDef = true // @!: the next identifier is a definition
//line lit/weave.w:472
		}
//line lit/weave.w:473
	}
//line lit/weave.w:474
	flushLine()
//line lit/weave.w:475
	return out.String()
//line lit/weave.w:476
}

// renderToken renders a single Go token as a TeX fragment (used inside math).
//
//line lit/weave.w:480
//line lit/weave.w:481
func renderToken(t token) string {
//line lit/weave.w:482
	switch t.kind {
//line lit/weave.w:483
	case tkKeyword, tkBuiltin:
//line lit/weave.w:484
		return "\\GKW{" + escIdent(t.text) + "}"
//line lit/weave.w:485
	case tkIdent:
//line lit/weave.w:486
		return "\\GID{" + escIdent(t.text) + "}"
//line lit/weave.w:487
	case tkMacro:
//line lit/weave.w:488
		if t.text == "nil" {
//line lit/weave.w:489
			// nil is Go's null value; show it with a symbol (\Gnil, a capital
//line lit/weave.w:490
			// lambda) as cweave shows C's NULL, rather than in typewriter.
//line lit/weave.w:491
			return "\\Gnil "
//line lit/weave.w:492
		}
//line lit/weave.w:493
		// Typewriter, like a CWEB  macro (an  name or a predeclared constant).
//line lit/weave.w:494
		// \GMAC wraps \tentex in an \hbox so it works in the math mode that code is
//line lit/weave.w:495
		// typeset in.
//line lit/weave.w:496
		return "\\GMAC{" + escTT(t.text) + "}"
//line lit/weave.w:497
	case tkNumber:
//line lit/weave.w:498
		return renderNumber(t.text)
//line lit/weave.w:499
	case tkString:
//line lit/weave.w:500
		return "\\GST{" + escTT(t.text) + "}"
//line lit/weave.w:501
	case tkComment:
//line lit/weave.w:502
		// Comments are set in roman (\GCM); escape them for roman text mode (not
//line lit/weave.w:503
		// the typewriter \charNN codes escTT emits), but let $...$ math through.
//line lit/weave.w:504
		// Tighten the leading "//" marker with a small kern (\Gcommentkern), whose
//line lit/weave.w:505
		// two slashes are otherwise set rather far apart in roman.
//line lit/weave.w:506
		if rest, ok := strings.CutPrefix(t.text, "//"); ok {
//line lit/weave.w:507
			return "\\GCM{/\\kern\\Gcommentkern/" + escComment(rest) + "}"
//line lit/weave.w:508
		}
//line lit/weave.w:509
		return "\\GCM{" + escComment(t.text) + "}"
//line lit/weave.w:510
	case tkOp:
//line lit/weave.w:511
		return renderOp(t.text)
//line lit/weave.w:512
	}
//line lit/weave.w:513
	return ""
//line lit/weave.w:514
}

//line lit/weave.w:522
func renderNumber(s string) string {
//line lit/weave.w:523
	if len(s) >= 2 && s[0] == '0' {
//line lit/weave.w:524
		switch s[1] {
//line lit/weave.w:525
		case 'x', 'X':
//line lit/weave.w:526
			return "\\Ghex{" + numDigits(s[2:]) + "}"
//line lit/weave.w:527
		case 'o', 'O':
//line lit/weave.w:528
			return "\\Goct{" + numDigits(s[2:]) + "}"
//line lit/weave.w:529
		case 'b', 'B':
//line lit/weave.w:530
			return "\\Gbin{" + numDigits(s[2:]) + "}"
//line lit/weave.w:531
		}
//line lit/weave.w:532
		if isOctalDigits(s[1:]) {
//line lit/weave.w:533
			return "\\Goct{" + numDigits(s[1:]) + "}"
//line lit/weave.w:534
		}
//line lit/weave.w:535
	}
//line lit/weave.w:536
	return "\\GNU{" + numDigits(s) + "}"
//line lit/weave.w:537
}

// isOctalDigits reports whether s is a nonempty run of octal digits (with
// optional _ separators) -- the tail of a classic 0NNN octal literal.
//
//line lit/weave.w:539
//line lit/weave.w:540
//line lit/weave.w:541
func isOctalDigits(s string) bool {
//line lit/weave.w:542
	if s == "" {
//line lit/weave.w:543
		return false
//line lit/weave.w:544
	}
//line lit/weave.w:545
	for i := 0; i < len(s); i++ {
//line lit/weave.w:546
		if c := s[i]; (c < '0' || c > '7') && c != '_' {
//line lit/weave.w:547
			return false
//line lit/weave.w:548
		}
//line lit/weave.w:549
	}
//line lit/weave.w:550
	return true
//line lit/weave.w:551
}

// numDigits renders the digits of a literal: a _ separator becomes a thin space;
// digits and hex letters are safe as is (no TeX specials occur in a number).
//
//line lit/weave.w:553
//line lit/weave.w:554
//line lit/weave.w:555
func numDigits(s string) string {
//line lit/weave.w:556
	return strings.ReplaceAll(s, "_", "\\,")
//line lit/weave.w:557
}

// processTex transforms commentary: |Go code| inline, @<refs@>, @@->@, and
// index entries (@^ @. @:) are recorded and removed. Everything else (the
// user's TeX) passes through unchanged.
//
//line lit/weave.w:563
//line lit/weave.w:564
//line lit/weave.w:565
//line lit/weave.w:566
func (wv *Weaver) processTex(secNum int, s string) string {
//line lit/weave.w:567
	var b strings.Builder
//line lit/weave.w:568
	n := len(s)
//line lit/weave.w:569
	i := 0
//line lit/weave.w:570
	for i < n {
//line lit/weave.w:571
		c := s[i]
//line lit/weave.w:572
		if c == '\\' && i+1 < n && s[i+1] == '|' {
//line lit/weave.w:573
			b.WriteString("|") // \| is a literal bar in prose
//line lit/weave.w:574
			i += 2
//line lit/weave.w:575
			continue
//line lit/weave.w:576
		}
//line lit/weave.w:577
		if c == '|' {
//line lit/weave.w:578
			j := i + 1
//line lit/weave.w:579
			var code strings.Builder
//line lit/weave.w:580
			for j < n {
//line lit/weave.w:581
				if s[j] == '\\' && j+1 < n && s[j+1] == '|' {
//line lit/weave.w:582
					code.WriteByte('|')
//line lit/weave.w:583
					j += 2
//line lit/weave.w:584
					continue
//line lit/weave.w:585
				}
//line lit/weave.w:586
				if s[j] == '|' {
//line lit/weave.w:587
					break
//line lit/weave.w:588
				}
//line lit/weave.w:589
				code.WriteByte(s[j])
//line lit/weave.w:590
				j++
//line lit/weave.w:591
			}
//line lit/weave.w:592
			b.WriteString(wv.renderInline(secNum, code.String()))
//line lit/weave.w:593
			i = j + 1
//line lit/weave.w:594
			continue
//line lit/weave.w:595
		}
//line lit/weave.w:596
		if c == '@' && i+1 < n {
//line lit/weave.w:597
			switch d := s[i+1]; d {
//line lit/weave.w:598
			case '@':
//line lit/weave.w:599
				b.WriteByte('@')
//line lit/weave.w:600
				i += 2
//line lit/weave.w:601
				continue
//line lit/weave.w:602
			case '<':
//line lit/weave.w:603
				if end := strings.Index(s[i+2:], "@>"); end >= 0 {
//line lit/weave.w:604
					end += i + 2
//line lit/weave.w:605
					name := wv.w.Resolve(strings.TrimSpace(s[i+2 : end]))
//line lit/weave.w:606
					wv.xref.addSectionUse(name, secNum)
//line lit/weave.w:607
					fmt.Fprintf(&b, "\\GX{%d}{%s}", wv.defNum[name], wv.renderName(name))
//line lit/weave.w:608
					i = end + 2
//line lit/weave.w:609
					continue
//line lit/weave.w:610
				}
//line lit/weave.w:611
			case '^', '.', ':':
//line lit/weave.w:612
				if end := strings.Index(s[i+2:], "@>"); end >= 0 {
//line lit/weave.w:613
					end += i + 2
//line lit/weave.w:614
					wv.xref.addManualIndex(d, s[i+2:end], secNum)
//line lit/weave.w:615
					i = end + 2
//line lit/weave.w:616
					continue
//line lit/weave.w:617
				}
//line lit/weave.w:618
			}
//line lit/weave.w:619
		}
//line lit/weave.w:620
		b.WriteByte(c)
//line lit/weave.w:621
		i++
//line lit/weave.w:622
	}
//line lit/weave.w:623
	return b.String()
//line lit/weave.w:624
}

// renderInline formats a |...| inline Go fragment (from prose) as one math
// group, recording identifier uses in section secNum.
//
//line lit/weave.w:630
//line lit/weave.w:631
//line lit/weave.w:632
func (wv *Weaver) renderInline(secNum int, code string) string {
//line lit/weave.w:633
	return wv.inlineCode(code, secNum, true)
//line lit/weave.w:634
}

// inlineCode formats a short Go fragment as one math group, mirroring the source
// whitespace (it is not wrapped). When record is true, identifier uses are added
// to the cross-reference under secNum.
//
//line lit/weave.w:636
//line lit/weave.w:637
//line lit/weave.w:638
//line lit/weave.w:639
func (wv *Weaver) inlineCode(code string, secNum int, record bool) string {
//line lit/weave.w:640
	var st lexState
//line lit/weave.w:641
	var b strings.Builder
//line lit/weave.w:642
	b.WriteString("$")
//line lit/weave.w:643
	pendingSpace := false
//line lit/weave.w:644
	started := false
//line lit/weave.w:645
	for _, t := range lexGo(code, &st) {
//line lit/weave.w:646
		switch t.kind {
//line lit/weave.w:647
		case tkSpace, tkNewline:
//line lit/weave.w:648
			if started {
//line lit/weave.w:649
				pendingSpace = true
//line lit/weave.w:650
			}
//line lit/weave.w:651
		default:
//line lit/weave.w:652
			if pendingSpace {
//line lit/weave.w:653
				b.WriteString("\\ ")
//line lit/weave.w:654
				pendingSpace = false
//line lit/weave.w:655
			}
//line lit/weave.w:656
			if record && (t.kind == tkIdent || t.kind == tkBuiltin) && indexable(t.text) && !wv.noIndex[t.text] {
//line lit/weave.w:657
				wv.xref.addIdentUse(t.text, secNum)
//line lit/weave.w:658
			}
//line lit/weave.w:659
			b.WriteString(renderToken(token{kind: wv.effKind(t), text: t.text}))
//line lit/weave.w:660
			started = true
//line lit/weave.w:661
		}
//line lit/weave.w:662
	}
//line lit/weave.w:663
	b.WriteString("$")
//line lit/weave.w:664
	return b.String()
//line lit/weave.w:665
}

//line lit/weave.w:673
func (wv *Weaver) renderComment(secNum int, text string) string {
//line lit/weave.w:674
	prefix := ""
//line lit/weave.w:675
	body := text
//line lit/weave.w:676
	if rest, ok := strings.CutPrefix(text, "//"); ok {
//line lit/weave.w:677
		prefix = "/\\kern\\Gcommentkern/"
//line lit/weave.w:678
		body = rest
//line lit/weave.w:679
	}
//line lit/weave.w:680
	return "\\GCM{" + prefix + wv.commentBody(secNum, body) + "}"
//line lit/weave.w:681
}

// commentBody escapes a comment for roman text mode but, as cweb does, renders a
// |...| span as inline Go code and lets a \.{...} typewriter span through
// verbatim (\. escapes its own argument). \| is a literal bar.
//
//line lit/weave.w:683
//line lit/weave.w:684
//line lit/weave.w:685
//line lit/weave.w:686
func (wv *Weaver) commentBody(secNum int, s string) string {
//line lit/weave.w:687
	var b, lit strings.Builder
//line lit/weave.w:688
	flush := func() {
//line lit/weave.w:689
		if lit.Len() > 0 {
//line lit/weave.w:690
			b.WriteString(escComment(lit.String()))
//line lit/weave.w:691
			lit.Reset()
//line lit/weave.w:692
		}
//line lit/weave.w:693
	}
//line lit/weave.w:694
	n := len(s)
//line lit/weave.w:695
	for i := 0; i < n; {
//line lit/weave.w:696
		if s[i] == '\\' && i+1 < n && s[i+1] == '|' {
//line lit/weave.w:697
			lit.WriteByte('|')
//line lit/weave.w:698
			i += 2
//line lit/weave.w:699
			continue
//line lit/weave.w:700
		}
//line lit/weave.w:701
		// \.{...}: a typewriter span, passed through verbatim. Find the matching
//line lit/weave.w:702
		// close brace, skipping \-escaped characters (\\ \{ \} inside the span).
//line lit/weave.w:703
		if s[i] == '\\' && i+2 < n && s[i+1] == '.' && s[i+2] == '{' {
//line lit/weave.w:704
			j := i + 3
//line lit/weave.w:705
			for j < n && s[j] != '}' {
//line lit/weave.w:706
				if s[j] == '\\' && j+1 < n {
//line lit/weave.w:707
					j++
//line lit/weave.w:708
				}
//line lit/weave.w:709
				j++
//line lit/weave.w:710
			}
//line lit/weave.w:711
			if j < n { // matched close brace
//line lit/weave.w:712
				flush()
//line lit/weave.w:713
				b.WriteString(s[i : j+1])
//line lit/weave.w:714
				i = j + 1
//line lit/weave.w:715
				continue
//line lit/weave.w:716
			}
//line lit/weave.w:717
		}
//line lit/weave.w:718
		if s[i] == '|' {
//line lit/weave.w:719
			j := i + 1
//line lit/weave.w:720
			var code strings.Builder
//line lit/weave.w:721
			closed := false
//line lit/weave.w:722
			for j < n {
//line lit/weave.w:723
				if s[j] == '\\' && j+1 < n && s[j+1] == '|' {
//line lit/weave.w:724
					code.WriteByte('|')
//line lit/weave.w:725
					j += 2
//line lit/weave.w:726
					continue
//line lit/weave.w:727
				}
//line lit/weave.w:728
				if s[j] == '|' {
//line lit/weave.w:729
					closed = true
//line lit/weave.w:730
					break
//line lit/weave.w:731
				}
//line lit/weave.w:732
				code.WriteByte(s[j])
//line lit/weave.w:733
				j++
//line lit/weave.w:734
			}
//line lit/weave.w:735
			if !closed {
//line lit/weave.w:736
				lit.WriteByte('|') // an unmatched bar is a literal bar
//line lit/weave.w:737
				i++
//line lit/weave.w:738
				continue
//line lit/weave.w:739
			}
//line lit/weave.w:740
			flush()
//line lit/weave.w:741
			b.WriteString(wv.inlineCode(code.String(), secNum, true))
//line lit/weave.w:742
			i = j + 1
//line lit/weave.w:743
			continue
//line lit/weave.w:744
		}
//line lit/weave.w:745
		lit.WriteByte(s[i])
//line lit/weave.w:746
		i++
//line lit/weave.w:747
	}
//line lit/weave.w:748
	flush()
//line lit/weave.w:749
	return b.String()
//line lit/weave.w:750
}

// renderName typesets a section name for TeX text mode. A |...| span is set as
// inline code (as in CWEB section names); the rest passes through as TeX, so
// control sequences and math work. A literal bar is written backslash-bar.
// An file= name is a literal path: typeset it in typewriter, escaped.
//
//line lit/weave.w:759
//line lit/weave.w:760
//line lit/weave.w:761
//line lit/weave.w:762
//line lit/weave.w:763
func (wv *Weaver) renderName(name string) string {
//line lit/weave.w:764
	if wv.isFile[name] {
//line lit/weave.w:765
		return "\\.{" + escTT(name) + "}"
//line lit/weave.w:766
	}
//line lit/weave.w:767
	var b strings.Builder
//line lit/weave.w:768
	n := len(name)
//line lit/weave.w:769
	i := 0
//line lit/weave.w:770
	for i < n {
//line lit/weave.w:771
		if name[i] == '\\' && i+1 < n && name[i+1] == '|' {
//line lit/weave.w:772
			b.WriteString("|")
//line lit/weave.w:773
			i += 2
//line lit/weave.w:774
			continue
//line lit/weave.w:775
		}
//line lit/weave.w:776
		if name[i] == '|' {
//line lit/weave.w:777
			j := i + 1
//line lit/weave.w:778
			var code strings.Builder
//line lit/weave.w:779
			for j < n {
//line lit/weave.w:780
				if name[j] == '\\' && j+1 < n && name[j+1] == '|' {
//line lit/weave.w:781
					code.WriteByte('|')
//line lit/weave.w:782
					j += 2
//line lit/weave.w:783
					continue
//line lit/weave.w:784
				}
//line lit/weave.w:785
				if name[j] == '|' {
//line lit/weave.w:786
					break
//line lit/weave.w:787
				}
//line lit/weave.w:788
				code.WriteByte(name[j])
//line lit/weave.w:789
				j++
//line lit/weave.w:790
			}
//line lit/weave.w:791
			b.WriteString(wv.inlineCode(code.String(), 0, false))
//line lit/weave.w:792
			i = j + 1
//line lit/weave.w:793
			continue
//line lit/weave.w:794
		}
//line lit/weave.w:795
		start := i
//line lit/weave.w:796
		for i < n && name[i] != '|' && !(name[i] == '\\' && i+1 < n && name[i+1] == '|') {
//line lit/weave.w:797
			i++
//line lit/weave.w:798
		}
//line lit/weave.w:799
		// The non-code text of a name is TeX, passed through as in CWEB so that
//line lit/weave.w:800
		// control sequences and math typeset; the user escapes any specials.
//line lit/weave.w:801
		b.WriteString(name[start:i])
//line lit/weave.w:802
	}
//line lit/weave.w:803
	return b.String()
//line lit/weave.w:804
}

// indexable reports whether an identifier should appear in the index. The blank
// identifier "_" is excluded.
//
//line lit/weave.w:809
//line lit/weave.w:810
//line lit/weave.w:811
func indexable(name string) bool { return name != "_" }

//line lit/weave.w:813
var declKeywords = map[string]bool{
//line lit/weave.w:814
	"func": true, "var": true, "const": true, "type": true,
//line lit/weave.w:815
}

// isDefinition heuristically decides whether the identifier at toks[k] is being
// declared: it follows a func/var/const/type keyword, or it is immediately
// followed by ":=". This is best-effort (no full Go parse) but covers the
// common cases CWEB underlines in its index.
//
//line lit/weave.w:822
//line lit/weave.w:823
//line lit/weave.w:824
//line lit/weave.w:825
//line lit/weave.w:826
func isDefinition(prevKind tokKind, prevText string, toks []token, k int) bool {
//line lit/weave.w:827
	if prevKind == tkKeyword && declKeywords[prevText] {
//line lit/weave.w:828
		return true
//line lit/weave.w:829
	}
//line lit/weave.w:830
	for j := k + 1; j < len(toks); j++ {
//line lit/weave.w:831
		switch toks[j].kind {
//line lit/weave.w:832
		case tkSpace:
//line lit/weave.w:833
			continue
//line lit/weave.w:834
		case tkOp:
//line lit/weave.w:835
			return toks[j].text == ":="
//line lit/weave.w:836
		default:
//line lit/weave.w:837
			return false
//line lit/weave.w:838
		}
//line lit/weave.w:839
	}
//line lit/weave.w:840
	return false
//line lit/weave.w:841
}

// indentLevel returns the indentation level of a leading-whitespace run: one
// level per tab, plus one per four spaces.
//
//line lit/weave.w:846
//line lit/weave.w:847
//line lit/weave.w:848
func indentLevel(s string) int {
//line lit/weave.w:849
	level, spaces := 0, 0
//line lit/weave.w:850
	for i := 0; i < len(s); i++ {
//line lit/weave.w:851
		switch s[i] {
//line lit/weave.w:852
		case '\t':
//line lit/weave.w:853
			level++
//line lit/weave.w:854
			spaces = 0
//line lit/weave.w:855
		case ' ':
//line lit/weave.w:856
			spaces++
//line lit/weave.w:857
			if spaces == 4 {
//line lit/weave.w:858
				level++
//line lit/weave.w:859
				spaces = 0
//line lit/weave.w:860
			}
//line lit/weave.w:861
		}
//line lit/weave.w:862
	}
//line lit/weave.w:863
	return level
//line lit/weave.w:864
}
