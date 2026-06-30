//line common/common.w:13
package common

//line common/common.w:15
import (
//line common/common.w:16
	"fmt"
//line common/common.w:17
	"os"
//line common/common.w:18
	"path/filepath"
//line common/common.w:19
	"strings"
//line common/common.w:20
)

//line common/common.w:22
const Version = "0.3.0"

//line common/common.w:24
type Format struct {
//line common/common.w:25
	Original string
//line common/common.w:26
	Like string
//line common/common.w:27
	NoIndex bool
//line common/common.w:28
	Macro bool // @d: typeset Original in typewriter (a CWEB-style macro)
//line common/common.w:29
}

//line common/common.w:35
type Section struct {
//line common/common.w:36
	Number int // 1-based section number
//line common/common.w:37
	Line int // 1-based source line where the section begins
//line common/common.w:38
	Starred bool // true for @* sections
//line common/common.w:39
	Depth int // group depth for starred sections (-1 == @**, 0 == @*, n == @*n)
//line common/common.w:40
	Title string // starred-section title (text up to the first period)
//line common/common.w:41
	Tex string // commentary, raw TeX with in-text @-codes still embedded
//line common/common.w:42
	Formats []Format
//line common/common.w:43
	HasCode bool // true if the section contributes code
//line common/common.w:44
	Name string // named-section name, or "" for an unnamed @c section
//line common/common.w:45
	IsFile bool // true if the name is an output file (@(file@>=)
//line common/common.w:46
	Code string // raw code text with in-code @-codes still embedded
//line common/common.w:47
	CodeLine int // 1-based combined-source line where Code begins (0 if none)
//line common/common.w:48
}

//line common/common.w:55
type Web struct {
//line common/common.w:56
	Limbo string
//line common/common.w:57
	Formats []Format // @f / @s directives found in limbo (apply globally)
//line common/common.w:58
	Sections []*Section
//line common/common.w:59
	Warnings []string // non-fatal diagnostics gathered while parsing/checking
//line common/common.w:60
	file string // source filename, for diagnostics ("" if unknown)
//line common/common.w:61
	locs []srcLoc // origin (file, line) of each combined-source line
//line common/common.w:62
	full []string // canonical (non-abbreviated) section names
//line common/common.w:63
}

//line common/common.w:70
func Parse(filename string) (*Web, error) {
//line common/common.w:71
	return ParseWithChange(filename, "")
//line common/common.w:72
}

//line common/common.w:74
func ParseWithChange(filename, changeFile string) (*Web, error) {
//line common/common.w:75
	lines, locs, err := expandIncludes(filename, 0)
//line common/common.w:76
	if err != nil {
//line common/common.w:77
		return nil, err
//line common/common.w:78
	}
//line common/common.w:79
	if changeFile != "" {
//line common/common.w:80
		chData, err := os.ReadFile(changeFile)
//line common/common.w:81
		if err != nil {
//line common/common.w:82
			return nil, err
//line common/common.w:83
		}
//line common/common.w:84
		changes, err := parseChangeFile(string(chData))
//line common/common.w:85
		if err != nil {
//line common/common.w:86
			return nil, err
//line common/common.w:87
		}
//line common/common.w:88
		lines, locs, err = applyChangesMapped(lines, locs, changes, changeFile)
//line common/common.w:89
		if err != nil {
//line common/common.w:90
			return nil, err
//line common/common.w:91
		}
//line common/common.w:92
	}
//line common/common.w:93
	src := strings.Join(lines, "\n")
//line common/common.w:94
	w := parse(src)
//line common/common.w:95
	w.file = filename
//line common/common.w:96
	w.locs = locs
//line common/common.w:97
	w.finish(src)
//line common/common.w:98
	return w, nil
//line common/common.w:99
}

//line common/common.w:101
func ParseString(src string) *Web {
//line common/common.w:102
	w := parse(src)
//line common/common.w:103
	w.finish(src)
//line common/common.w:104
	return w
//line common/common.w:105
}

//line common/common.w:107
func (w *Web) finish(src string) {
//line common/common.w:108
	w.collectNames()
//line common/common.w:109
	w.Warnings = append(w.Warnings, w.scanDiagnostics(src)...)
//line common/common.w:110
	w.Warnings = append(w.Warnings, w.checkNames()...)
//line common/common.w:111
}

//line common/common.w:113
func (w *Web) at(line int) string {
//line common/common.w:114
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line common/common.w:115
		return w.locs[i].String()
//line common/common.w:116
	}
//line common/common.w:117
	if w.file != "" {
//line common/common.w:118
		return fmt.Sprintf("%s:%d", w.file, line)
//line common/common.w:119
	}
//line common/common.w:120
	return fmt.Sprintf("line %d", line)
//line common/common.w:121
}

//line common/common.w:128
func (w *Web) Origin(line int) (file string, ln int) {
//line common/common.w:129
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line common/common.w:130
		return w.locs[i].file, w.locs[i].line
//line common/common.w:131
	}
//line common/common.w:132
	return w.file, line
//line common/common.w:133
}

//line common/common.w:139
func DefaultExt(name, ext string) string {
//line common/common.w:140
	if name == "" || filepath.Ext(name) != "" {
//line common/common.w:141
		return name
//line common/common.w:142
	}
//line common/common.w:143
	return name + ext
//line common/common.w:144
}

//line common/common.w:150
func expandIncludes(file string, depth int) ([]string, []srcLoc, error) {
//line common/common.w:151
	if depth > 25 {
//line common/common.w:152
		return nil, nil, fmt.Errorf("gweb: @i include nesting too deep at %q", file)
//line common/common.w:153
	}
//line common/common.w:154
	data, err := os.ReadFile(file)
//line common/common.w:155
	if err != nil {
//line common/common.w:156
		return nil, nil, err
//line common/common.w:157
	}
//line common/common.w:158
	raw := splitLines(string(data))
//line common/common.w:159
	if n := len(raw); n > 0 && raw[n-1] == "" {
//line common/common.w:160
		raw = raw[:n-1]
//line common/common.w:161
	}

//line common/common.w:163
	var lines []string
//line common/common.w:164
	var locs []srcLoc
//line common/common.w:165
	dir := filepath.Dir(file)
//line common/common.w:166
	for i, line := range raw {
//line common/common.w:167
		if name, ok := includeDirective(line); ok {
//line common/common.w:168
			path := name
//line common/common.w:169
			if !filepath.IsAbs(path) {
//line common/common.w:170
				path = filepath.Join(dir, name)
//line common/common.w:171
			}
//line common/common.w:172
			sub, subLocs, err := expandIncludes(path, depth+1)
//line common/common.w:173
			if err != nil {
//line common/common.w:174
				return nil, nil, fmt.Errorf("%s:%d: %w", file, i+1, err)
//line common/common.w:175
			}
//line common/common.w:176
			lines = append(lines, sub...)
//line common/common.w:177
			locs = append(locs, subLocs...)
//line common/common.w:178
			continue
//line common/common.w:179
		}
//line common/common.w:180
		lines = append(lines, line)
//line common/common.w:181
		locs = append(locs, srcLoc{file, i + 1})
//line common/common.w:182
	}
//line common/common.w:183
	return lines, locs, nil
//line common/common.w:184
}

//line common/common.w:186
func includeDirective(line string) (name string, ok bool) {
//line common/common.w:187
	t := strings.TrimLeft(line, " \t")
//line common/common.w:188
	if !strings.HasPrefix(t, "@i") {
//line common/common.w:189
		return "", false
//line common/common.w:190
	}
//line common/common.w:191
	rest := t[2:]
//line common/common.w:192
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
//line common/common.w:193
		return "", false
//line common/common.w:194
	}
//line common/common.w:195
	name = strings.Trim(strings.TrimSpace(rest), "\"")
//line common/common.w:196
	return name, name != ""
//line common/common.w:197
}

//line common/common.w:204
func (w *Web) collectNames() {
//line common/common.w:205
	seen := map[string]bool{}
//line common/common.w:206
	add := func(name string) {
//line common/common.w:207
		if name != "" && !strings.HasSuffix(name, "...") && !seen[name] {
//line common/common.w:208
			seen[name] = true
//line common/common.w:209
			w.full = append(w.full, name)
//line common/common.w:210
		}
//line common/common.w:211
	}
//line common/common.w:212
	for _, s := range w.Sections {
//line common/common.w:213
		if !s.IsFile {
//line common/common.w:214
			add(s.Name) // a definition's name
//line common/common.w:215
		}
//line common/common.w:216
		for _, raw := range []string{s.Code, s.Tex} {
//line common/common.w:217
			for _, a := range ScanCode(raw) {
//line common/common.w:218
				if a.Kind == ARef {
//line common/common.w:219
					add(a.Text) // a reference's name
//line common/common.w:220
				}
//line common/common.w:221
			}
//line common/common.w:222
		}
//line common/common.w:223
	}
//line common/common.w:224
}

//line common/common.w:226
func (w *Web) prefixMatches(prefix string) int {
//line common/common.w:227
	n := 0
//line common/common.w:228
	for _, full := range w.full {
//line common/common.w:229
		if strings.HasPrefix(full, prefix) {
//line common/common.w:230
			n++
//line common/common.w:231
		}
//line common/common.w:232
	}
//line common/common.w:233
	return n
//line common/common.w:234
}

//line common/common.w:241
func (w *Web) checkNames() []string {
//line common/common.w:242
	// "defined" is the set of sections that actually have a definition (not just
//line common/common.w:243
	// the full names known for abbreviation resolution, which include references).
//line common/common.w:244
	defined := map[string]bool{}
//line common/common.w:245
	for _, s := range w.Sections {
//line common/common.w:246
		if s.Name != "" && !s.IsFile {
//line common/common.w:247
			defined[w.Resolve(s.Name)] = true
//line common/common.w:248
		}
//line common/common.w:249
	}
//line common/common.w:250
	used := map[string]bool{}
//line common/common.w:251
	var warns []string

//line common/common.w:253
	for _, s := range w.Sections {
//line common/common.w:254
		scan := func(raw string) {
//line common/common.w:255
			for _, a := range ScanCode(raw) {
//line common/common.w:256
				if a.Kind != ARef {
//line common/common.w:257
					continue
//line common/common.w:258
				}
//line common/common.w:259
				canon := w.Resolve(a.Text)
//line common/common.w:260
				if strings.HasSuffix(a.Text, "...") && canon == a.Text {
//line common/common.w:261
					prefix := strings.TrimSpace(strings.TrimSuffix(a.Text, "..."))
//line common/common.w:262
					if m := w.prefixMatches(prefix); m == 0 {
//line common/common.w:263
						warns = append(warns, fmt.Sprintf("%s: no section name matches <%s>", w.at(s.Line), a.Text))
//line common/common.w:264
					} else {
//line common/common.w:265
						warns = append(warns, fmt.Sprintf("%s: ambiguous prefix <%s> matches %d section names", w.at(s.Line), a.Text, m))
//line common/common.w:266
					}
//line common/common.w:267
					continue
//line common/common.w:268
				}
//line common/common.w:269
				if !defined[canon] {
//line common/common.w:270
					warns = append(warns, fmt.Sprintf("%s: reference to undefined section <%s>", w.at(s.Line), a.Text))
//line common/common.w:271
				}
//line common/common.w:272
				used[canon] = true
//line common/common.w:273
			}
//line common/common.w:274
		}
//line common/common.w:275
		scan(s.Code)
//line common/common.w:276
		scan(s.Tex)
//line common/common.w:277
	}

//line common/common.w:279
	warned := map[string]bool{}
//line common/common.w:280
	for _, s := range w.Sections {
//line common/common.w:281
		if s.Name == "" || s.IsFile {
//line common/common.w:282
			continue
//line common/common.w:283
		}
//line common/common.w:284
		canon := w.Resolve(s.Name)
//line common/common.w:285
		if !used[canon] && !warned[canon] {
//line common/common.w:286
			warned[canon] = true
//line common/common.w:287
			warns = append(warns, fmt.Sprintf("%s: section <%s> is defined but never used", w.at(s.Line), s.Name))
//line common/common.w:288
		}
//line common/common.w:289
	}
//line common/common.w:290
	return warns
//line common/common.w:291
}

//line common/common.w:298
func lineAt(src string, off int) int {
//line common/common.w:299
	if off > len(src) {
//line common/common.w:300
		off = len(src)
//line common/common.w:301
	}
//line common/common.w:302
	return 1 + strings.Count(src[:off], "\n")
//line common/common.w:303
}

//line common/common.w:305
func canonName(name string) string {
//line common/common.w:306
	return strings.Join(strings.Fields(name), " ")
//line common/common.w:307
}

//line common/common.w:309
func (w *Web) Resolve(name string) string {
//line common/common.w:310
	name = canonName(name)
//line common/common.w:311
	if !strings.HasSuffix(name, "...") {
//line common/common.w:312
		return name
//line common/common.w:313
	}
//line common/common.w:314
	prefix := strings.TrimSpace(strings.TrimSuffix(name, "..."))
//line common/common.w:315
	var match string
//line common/common.w:316
	count := 0
//line common/common.w:317
	for _, full := range w.full {
//line common/common.w:318
		if strings.HasPrefix(full, prefix) {
//line common/common.w:319
			match = full
//line common/common.w:320
			count++
//line common/common.w:321
		}
//line common/common.w:322
	}
//line common/common.w:323
	if count == 1 {
//line common/common.w:324
		return match
//line common/common.w:325
	}
//line common/common.w:326
	return name // unresolved or ambiguous; leave as-is for caller to report
//line common/common.w:327
}

//line common/common.w:329
func indexFrom(s, sub string, from int) int {
//line common/common.w:330
	if from >= len(s) {
//line common/common.w:331
		return -1
//line common/common.w:332
	}
//line common/common.w:333
	idx := strings.Index(s[from:], sub)
//line common/common.w:334
	if idx < 0 {
//line common/common.w:335
		return -1
//line common/common.w:336
	}
//line common/common.w:337
	return from + idx
//line common/common.w:338
}

//line common/common.w:345
type ctrlKind int

//line common/common.w:347
const (
//line common/common.w:348
	cEOF ctrlKind = iota
//line common/common.w:349
	cSection
//line common/common.w:350
	cCode // @c (or its synonym @p)
//line common/common.w:351
	cNamed // @<name@>= or @(file@>=
//line common/common.w:352
	cDefn // @d
//line common/common.w:353
	cFormat

//line common/common.w:354
)

//line common/common.w:356
type ctrl struct {
//line common/common.w:357
	kind ctrlKind
//line common/common.w:358
	pos int // index of the leading '@'
//line common/common.w:359
	end int // index just past the control token
//line common/common.w:360
	depth int // for cSection: -1 unstarred (or @** top level), else starred depth
//line common/common.w:361
	starred bool // for cSection (distinguishes @** from an unstarred section)
//line common/common.w:362
	name string // for cNamed
//line common/common.w:363
	isFile bool // for cNamed (@( vs @<)
//line common/common.w:364
	noIndex bool // for cFormat (@s)
//line common/common.w:365
}

//line common/common.w:372
func scanStruct(src string, i int) ctrl {
//line common/common.w:373
	n := len(src)
//line common/common.w:374
	for i < n {
//line common/common.w:375
		if src[i] != '@' {
//line common/common.w:376
			i++
//line common/common.w:377
			continue
//line common/common.w:378
		}
//line common/common.w:379
		if i+1 >= n {
//line common/common.w:380
			break
//line common/common.w:381
		}
//line common/common.w:382
		switch c := src[i+1]; {
//line common/common.w:383
		case c == '@':
//line common/common.w:384
			i += 2
//line common/common.w:385
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line common/common.w:386
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line common/common.w:387
		case c == '*':
//line common/common.w:388
			j := i + 2
//line common/common.w:389
			depth := 0
//line common/common.w:390
			if j < n && src[j] == '*' {
//line common/common.w:391
				j++
//line common/common.w:392
				depth = -1 // "@**" is the top level: bold in the contents, as cweb
//line common/common.w:393
			} else {
//line common/common.w:394
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:395
					depth = depth*10 + int(src[j]-'0')
//line common/common.w:396
					j++
//line common/common.w:397
				}
//line common/common.w:398
			}
//line common/common.w:399
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}
//line common/common.w:400
		case c == 'c' || c == 'p':
//line common/common.w:401
			return ctrl{kind: cCode, pos: i, end: i + 2}
//line common/common.w:402
		case c == 'd':
//line common/common.w:403
			return ctrl{kind: cDefn, pos: i, end: i + 2}
//line common/common.w:404
		case c == 'f':
//line common/common.w:405
			return ctrl{kind: cFormat, pos: i, end: i + 2}
//line common/common.w:406
		case c == 's':
//line common/common.w:407
			return ctrl{kind: cFormat, pos: i, end: i + 2, noIndex: true}
//line common/common.w:408
		case c == '<' || c == '(':
//line common/common.w:409
			end := indexFrom(src, "@>", i+2)
//line common/common.w:410
			if end < 0 {
//line common/common.w:411
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:412
			}
//line common/common.w:413
			after := end + 2
//line common/common.w:414
			k := after
//line common/common.w:415
			for k < n && (src[k] == ' ' || src[k] == '\t') {
//line common/common.w:416
				k++
//line common/common.w:417
			}
//line common/common.w:418
			if k < n && src[k] == '=' {
//line common/common.w:419
				return ctrl{kind: cNamed, pos: i, end: k + 1,
//line common/common.w:420
					name: canonName(src[i+2 : end]), isFile: c == '('}
//line common/common.w:421
			}
//line common/common.w:422
			i = after // a reference, not a definition
//line common/common.w:423
		case c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line common/common.w:424
			end := indexFrom(src, "@>", i+2)
//line common/common.w:425
			if end < 0 {
//line common/common.w:426
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:427
			}
//line common/common.w:428
			i = end + 2
//line common/common.w:429
		case c == '%':
//line common/common.w:430
			j := i + 2
//line common/common.w:431
			for j < n && src[j] != '\n' {
//line common/common.w:432
				j++
//line common/common.w:433
			}
//line common/common.w:434
			i = j
//line common/common.w:435
		default:
//line common/common.w:436
			i += 2
//line common/common.w:437
		}
//line common/common.w:438
	}
//line common/common.w:439
	return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:440
}

//line common/common.w:446
func findNextSection(src string, i int) ctrl {
//line common/common.w:447
	n := len(src)
//line common/common.w:448
	for i < n {
//line common/common.w:449
		if src[i] != '@' {
//line common/common.w:450
			i++
//line common/common.w:451
			continue
//line common/common.w:452
		}
//line common/common.w:453
		if i+1 >= n {
//line common/common.w:454
			break
//line common/common.w:455
		}
//line common/common.w:456
		switch c := src[i+1]; {
//line common/common.w:457
		case c == '@':
//line common/common.w:458
			i += 2
//line common/common.w:459
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line common/common.w:460
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line common/common.w:461
		case c == '*':
//line common/common.w:462
			j := i + 2
//line common/common.w:463
			depth := 0
//line common/common.w:464
			if j < n && src[j] == '*' {
//line common/common.w:465
				j++
//line common/common.w:466
				depth = -1 // "@**" is the top level: bold in the contents, as cweb
//line common/common.w:467
			} else {
//line common/common.w:468
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:469
					depth = depth*10 + int(src[j]-'0')
//line common/common.w:470
					j++
//line common/common.w:471
				}
//line common/common.w:472
			}
//line common/common.w:473
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}
//line common/common.w:474
		case c == '<' || c == '(' || c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line common/common.w:475
			end := indexFrom(src, "@>", i+2)
//line common/common.w:476
			if end < 0 {
//line common/common.w:477
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:478
			}
//line common/common.w:479
			i = end + 2
//line common/common.w:480
		case c == '%':
//line common/common.w:481
			j := i + 2
//line common/common.w:482
			for j < n && src[j] != '\n' {
//line common/common.w:483
				j++
//line common/common.w:484
			}
//line common/common.w:485
			i = j
//line common/common.w:486
		default:
//line common/common.w:487
			i += 2
//line common/common.w:488
		}
//line common/common.w:489
	}
//line common/common.w:490
	return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:491
}

//line common/common.w:496
func parse(src string) *Web {
//line common/common.w:497
	w := &Web{}
//line common/common.w:498
	n := len(src)

//line common/common.w:500
	// Limbo runs until the first section break. Format directives placed there
//line common/common.w:501
	// (@f / @s, a common CWEB idiom) are extracted and removed from the copied
//line common/common.w:502
	// TeX so they apply globally rather than printing literally.
//line common/common.w:503
	first := findNextSection(src, 0)
//line common/common.w:504
	w.Limbo, w.Formats = extractLimboFormats(src[:first.pos])
//line common/common.w:505
	i := first.pos

//line common/common.w:507
	num := 0
//line common/common.w:508
	for i < n {
//line common/common.w:509
		// We are positioned at a section break.
//line common/common.w:510
		hdr := src[i+1]
//line common/common.w:511
		num++
//line common/common.w:512
		sec := &Section{Number: num, Line: lineAt(src, i)}
//line common/common.w:513
		if hdr == '*' {
//line common/common.w:514
			h := findSectionHeaderEnd(src, i)
//line common/common.w:515
			sec.Starred = true
//line common/common.w:516
			sec.Depth = h.depth
//line common/common.w:517
			i = h.end
//line common/common.w:518
		} else {
//line common/common.w:519
			i += 2
//line common/common.w:520
		}

//line common/common.w:522
		// TeX part: from here to the next structural control.
//line common/common.w:523
		ct := scanStruct(src, i)
//line common/common.w:524
		sec.Tex = src[i:ct.pos]
//line common/common.w:525
		if sec.Starred {
//line common/common.w:526
			sec.Title = extractTitle(sec.Tex)
//line common/common.w:527
		}

//line common/common.w:529
		// Definition part: a run of @d / @f / @s.
//line common/common.w:530
		for ct.kind == cDefn || ct.kind == cFormat {
//line common/common.w:531
			nx := scanStruct(src, ct.end)
//line common/common.w:532
			seg := src[ct.end:nx.pos]
//line common/common.w:533
			// @d has no Go analogue (Go has no preprocessor), so it never tangles
//line common/common.w:534
			// to code; gweave uses it only to set the named identifier in
//line common/common.w:535
			// typewriter, as cweave sets a macro. @f/@s format like another word.
//line common/common.w:536
			if ct.kind == cDefn {
//line common/common.w:537
				if f, ok := parseMacro(seg); ok {
//line common/common.w:538
					sec.Formats = append(sec.Formats, f)
//line common/common.w:539
				}
//line common/common.w:540
			} else if f, ok := parseFormat(seg, ct.noIndex); ok {
//line common/common.w:541
				sec.Formats = append(sec.Formats, f)
//line common/common.w:542
			}
//line common/common.w:543
			ct = nx
//line common/common.w:544
		}

//line common/common.w:546
		switch ct.kind {
//line common/common.w:547
		case cCode:
//line common/common.w:548
			sec.HasCode = true
//line common/common.w:549
			sec.CodeLine = lineAt(src, ct.end)
//line common/common.w:550
			nx := findNextSection(src, ct.end)
//line common/common.w:551
			sec.Code = src[ct.end:nx.pos]
//line common/common.w:552
			i = nx.pos
//line common/common.w:553
		case cNamed:
//line common/common.w:554
			sec.HasCode = true
//line common/common.w:555
			sec.Name = ct.name
//line common/common.w:556
			sec.IsFile = ct.isFile
//line common/common.w:557
			sec.CodeLine = lineAt(src, ct.end)
//line common/common.w:558
			nx := findNextSection(src, ct.end)
//line common/common.w:559
			sec.Code = src[ct.end:nx.pos]
//line common/common.w:560
			i = nx.pos
//line common/common.w:561
		default: // cSection or cEOF: a documentation-only section
//line common/common.w:562
			i = ct.pos
//line common/common.w:563
		}

//line common/common.w:565
		w.Sections = append(w.Sections, sec)
//line common/common.w:566
		if ct.kind == cEOF && sec.Code == "" {
//line common/common.w:567
			break
//line common/common.w:568
		}
//line common/common.w:569
		if i >= n {
//line common/common.w:570
			break
//line common/common.w:571
		}
//line common/common.w:572
	}
//line common/common.w:573
	return w
//line common/common.w:574
}

//line common/common.w:578
func findSectionHeaderEnd(src string, i int) ctrl {
//line common/common.w:579
	n := len(src)
//line common/common.w:580
	j := i + 2
//line common/common.w:581
	depth := 0
//line common/common.w:582
	if j < n && src[j] == '*' {
//line common/common.w:583
		j++
//line common/common.w:584
		depth = -1 // "@**" is the top level: bold in the contents, as cweb
//line common/common.w:585
	} else {
//line common/common.w:586
		for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:587
			depth = depth*10 + int(src[j]-'0')
//line common/common.w:588
			j++
//line common/common.w:589
		}
//line common/common.w:590
	}
//line common/common.w:591
	return ctrl{end: j, depth: depth}
//line common/common.w:592
}

//line common/common.w:597
func extractTitle(tex string) string {
//line common/common.w:598
	t := strings.TrimLeft(tex, " \t\n")
//line common/common.w:599
	if i := titleEnd(t); i >= 0 {
//line common/common.w:600
		t = t[:i]
//line common/common.w:601
	}
//line common/common.w:602
	return strings.Join(strings.Fields(t), " ")
//line common/common.w:603
}

//line common/common.w:605
func titleEnd(s string) int {
//line common/common.w:606
	for i := 0; i < len(s); i++ {
//line common/common.w:607
		if s[i] == '.' && (i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t' ||
//line common/common.w:608
			s[i+1] == '\n' || s[i+1] == '\r') {
//line common/common.w:609
			return i
//line common/common.w:610
		}
//line common/common.w:611
	}
//line common/common.w:612
	return -1
//line common/common.w:613
}

//line common/common.w:618
func (w *Web) scanDiagnostics(src string) []string {
//line common/common.w:619
	var warns []string
//line common/common.w:620
	n := len(src)
//line common/common.w:621
	i := 0
//line common/common.w:622
	for i < n {
//line common/common.w:623
		if src[i] != '@' || i+1 >= n {
//line common/common.w:624
			i++
//line common/common.w:625
			continue
//line common/common.w:626
		}
//line common/common.w:627
		switch c := src[i+1]; c {
//line common/common.w:628
		case '@':
//line common/common.w:629
			i += 2
//line common/common.w:630
		case '<', '(', '=', 't', '^', '.', ':', 'q':
//line common/common.w:631
			if end := indexFrom(src, "@>", i+2); end < 0 {
//line common/common.w:632
				warns = append(warns, fmt.Sprintf("%s: unterminated `@%c ... @>'", w.at(lineAt(src, i)), c))
//line common/common.w:633
				i = n
//line common/common.w:634
			} else {
//line common/common.w:635
				i = end + 2
//line common/common.w:636
			}
//line common/common.w:637
		default:
//line common/common.w:638
			i += 2
//line common/common.w:639
		}
//line common/common.w:640
	}
//line common/common.w:641
	return warns
//line common/common.w:642
}

//line common/common.w:646
func parseFormat(seg string, noIndex bool) (Format, bool) {
//line common/common.w:647
	fields := strings.Fields(seg)
//line common/common.w:648
	if len(fields) < 2 {
//line common/common.w:649
		return Format{}, false
//line common/common.w:650
	}
//line common/common.w:651
	return Format{Original: fields[0], Like: fields[1], NoIndex: noIndex}, true
//line common/common.w:652
}

//line common/common.w:660
func parseMacro(seg string) (Format, bool) {
//line common/common.w:661
	fields := strings.Fields(seg)
//line common/common.w:662
	if len(fields) == 0 {
//line common/common.w:663
		return Format{}, false
//line common/common.w:664
	}
//line common/common.w:665
	name := fields[0]
//line common/common.w:666
	if k := strings.LastIndex(name, "."); k >= 0 {
//line common/common.w:667
		name = name[k+1:]
//line common/common.w:668
	}
//line common/common.w:669
	if name == "" {
//line common/common.w:670
		return Format{}, false
//line common/common.w:671
	}
//line common/common.w:672
	return Format{Original: name, Macro: true}, true
//line common/common.w:673
}

//line common/common.w:679
func extractLimboFormats(src string) (string, []Format) {
//line common/common.w:680
	var b strings.Builder
//line common/common.w:681
	var formats []Format
//line common/common.w:682
	n := len(src)
//line common/common.w:683
	i := 0
//line common/common.w:684
	for i < n {
//line common/common.w:685
		if src[i] != '@' || i+1 >= n {
//line common/common.w:686
			b.WriteByte(src[i])
//line common/common.w:687
			i++
//line common/common.w:688
			continue
//line common/common.w:689
		}
//line common/common.w:690
		switch c := src[i+1]; c {
//line common/common.w:691
		case '@':
//line common/common.w:692
			b.WriteString("@@")
//line common/common.w:693
			i += 2
//line common/common.w:694
		case 'd', 'f', 's':
//line common/common.w:695
			j := i + 2
//line common/common.w:696
			for j < n && src[j] != '\n' {
//line common/common.w:697
				j++
//line common/common.w:698
			}
//line common/common.w:699
			var f Format
//line common/common.w:700
			var ok bool
//line common/common.w:701
			if c == 'd' {
//line common/common.w:702
				f, ok = parseMacro(src[i+2 : j])
//line common/common.w:703
			} else {
//line common/common.w:704
				f, ok = parseFormat(src[i+2:j], c == 's')
//line common/common.w:705
			}
//line common/common.w:706
			if ok {
//line common/common.w:707
				formats = append(formats, f)
//line common/common.w:708
			}
//line common/common.w:709
			if j < n {
//line common/common.w:710
				j++ // also drop the newline that ended the directive
//line common/common.w:711
			}
//line common/common.w:712
			i = j
//line common/common.w:713
		case '<', '(', '=', 't', '^', '.', ':', 'q':
//line common/common.w:714
			end := indexFrom(src, "@>", i+2)
//line common/common.w:715
			if end < 0 {
//line common/common.w:716
				b.WriteString(src[i:])
//line common/common.w:717
				i = n
//line common/common.w:718
			} else {
//line common/common.w:719
				b.WriteString(src[i : end+2])
//line common/common.w:720
				i = end + 2
//line common/common.w:721
			}
//line common/common.w:722
		default:
//line common/common.w:723
			b.WriteString(src[i : i+2])
//line common/common.w:724
			i += 2
//line common/common.w:725
		}
//line common/common.w:726
	}
//line common/common.w:727
	return b.String(), formats
//line common/common.w:728
}

//line common/common.w:735
type AtomKind int

//line common/common.w:737
const (
//line common/common.w:738
	AText AtomKind = iota // ordinary Go source text
//line common/common.w:739
	ARef // @<name@> reference to a named section
//line common/common.w:740
	AVerbatim // @=text@> passed verbatim to tangled output
//line common/common.w:741
	ATeX // @t text@> TeX text for the woven output
//line common/common.w:742
	AIndex // @^/@./@: index entry
//line common/common.w:743
	APaste // @& join (delete surrounding whitespace)
//line common/common.w:744
	ALayout // @, @/ @| @# woven-output layout hints
//line common/common.w:745
	AIndexDef // @! force the next identifier to index as a definition
//line common/common.w:746
)

//line common/common.w:748
type Atom struct {
//line common/common.w:749
	Kind AtomKind
//line common/common.w:750
	Text string // payload for AText/AVerbatim/ATeX/AIndex; name for ARef
//line common/common.w:751
	Index byte // '^','.',':' for AIndex; ',' '/' '|' '#' for ALayout
//line common/common.w:752
}

//line common/common.w:758
func ScanCode(code string) []Atom {
//line common/common.w:759
	var atoms []Atom
//line common/common.w:760
	var buf strings.Builder
//line common/common.w:761
	flush := func() {
//line common/common.w:762
		if buf.Len() > 0 {
//line common/common.w:763
			atoms = append(atoms, Atom{Kind: AText, Text: buf.String()})
//line common/common.w:764
			buf.Reset()
//line common/common.w:765
		}
//line common/common.w:766
	}
//line common/common.w:767
	n := len(code)
//line common/common.w:768
	i := 0
//line common/common.w:769
	for i < n {
//line common/common.w:770
		c := code[i]
//line common/common.w:771
		if c != '@' || i+1 >= n {
//line common/common.w:772
			buf.WriteByte(c)
//line common/common.w:773
			i++
//line common/common.w:774
			continue
//line common/common.w:775
		}
//line common/common.w:776
		switch d := code[i+1]; d {
//line common/common.w:777
		case '@':
//line common/common.w:778
			buf.WriteByte('@')
//line common/common.w:779
			i += 2
//line common/common.w:780
		case '&':
//line common/common.w:781
			flush()
//line common/common.w:782
			atoms = append(atoms, Atom{Kind: APaste})
//line common/common.w:783
			i += 2
//line common/common.w:784
		case '<':
//line common/common.w:785
			end := indexFrom(code, "@>", i+2)
//line common/common.w:786
			if end < 0 {
//line common/common.w:787
				buf.WriteString(code[i:])
//line common/common.w:788
				i = n
//line common/common.w:789
				continue
//line common/common.w:790
			}
//line common/common.w:791
			flush()
//line common/common.w:792
			atoms = append(atoms, Atom{Kind: ARef, Text: canonName(code[i+2 : end])})
//line common/common.w:793
			i = end + 2
//line common/common.w:794
		case '=':
//line common/common.w:795
			end := indexFrom(code, "@>", i+2)
//line common/common.w:796
			if end < 0 {
//line common/common.w:797
				i = n
//line common/common.w:798
				continue
//line common/common.w:799
			}
//line common/common.w:800
			flush()
//line common/common.w:801
			atoms = append(atoms, Atom{Kind: AVerbatim, Text: code[i+2 : end]})
//line common/common.w:802
			i = end + 2
//line common/common.w:803
		case 't':
//line common/common.w:804
			end := indexFrom(code, "@>", i+2)
//line common/common.w:805
			if end < 0 {
//line common/common.w:806
				i = n
//line common/common.w:807
				continue
//line common/common.w:808
			}
//line common/common.w:809
			flush()
//line common/common.w:810
			atoms = append(atoms, Atom{Kind: ATeX, Text: code[i+2 : end]})
//line common/common.w:811
			i = end + 2
//line common/common.w:812
		case '^', '.', ':':
//line common/common.w:813
			end := indexFrom(code, "@>", i+2)
//line common/common.w:814
			if end < 0 {
//line common/common.w:815
				i = n
//line common/common.w:816
				continue
//line common/common.w:817
			}
//line common/common.w:818
			flush()
//line common/common.w:819
			atoms = append(atoms, Atom{Kind: AIndex, Text: code[i+2 : end], Index: d})
//line common/common.w:820
			i = end + 2
//line common/common.w:821
		case 'q':
//line common/common.w:822
			end := indexFrom(code, "@>", i+2)
//line common/common.w:823
			if end < 0 {
//line common/common.w:824
				i = n
//line common/common.w:825
				continue
//line common/common.w:826
			}
//line common/common.w:827
			i = end + 2 // ignored material
//line common/common.w:828
		case '%':
//line common/common.w:829
			j := i + 2
//line common/common.w:830
			for j < n && code[j] != '\n' {
//line common/common.w:831
				j++
//line common/common.w:832
			}
//line common/common.w:833
			i = j
//line common/common.w:834
		case '>':
//line common/common.w:835
			i += 2 // stray terminator
//line common/common.w:836
		case ',', '/', '|', '#':
//line common/common.w:837
			// Woven-output layout hints: thin space, line break, optional line
//line common/common.w:838
			// break, and break-plus-blank-line. Ignored by gtangle.
//line common/common.w:839
			flush()
//line common/common.w:840
			atoms = append(atoms, Atom{Kind: ALayout, Index: d})
//line common/common.w:841
			i += 2
//line common/common.w:842
		case '!':
//line common/common.w:843
			// Force the next identifier's index entry to be a definition,
//line common/common.w:844
			// overriding the heuristic. Produces no output by itself.
//line common/common.w:845
			flush()
//line common/common.w:846
			atoms = append(atoms, Atom{Kind: AIndexDef})
//line common/common.w:847
			i += 2
//line common/common.w:848
		case '+', '[', ']', ';':
//line common/common.w:849
			// CWEB prettyprinter hints (cancel break, expression brackets,
//line common/common.w:850
			// invisible semicolon). GWEB mirrors the source instead of reflowing
//line common/common.w:851
			// it, so these have no effect; accept and drop them for portability.
//line common/common.w:852
			i += 2
//line common/common.w:853
		default:
//line common/common.w:854
			i += 2 // unknown @x: drop it rather than corrupt the output
//line common/common.w:855
		}
//line common/common.w:856
	}
//line common/common.w:857
	flush()
//line common/common.w:858
	return atoms
//line common/common.w:859
}

//line common/common.w:866
// A change file (CWEB's ".ch" mechanism) patches the master source without
//line common/common.w:867
// editing it. It is a sequence of changes, each of the form
//line common/common.w:868
//
//line common/common.w:869
//	@x
//line common/common.w:870
//	<lines to find in the master source>
//line common/common.w:871
//	@y
//line common/common.w:872
//	<lines to substitute>
//line common/common.w:873
//	@z
//line common/common.w:874
//
//line common/common.w:875
// Text outside an @x...@z group is ignored (it serves as commentary). Changes
//line common/common.w:876
// are matched against the master source — after @i includes are expanded — in
//line common/common.w:877
// the order they appear: GWEB scans the master line by line, and at the first
//line common/common.w:878
// line equal to a change's first match line it requires the whole match block
//line common/common.w:879
// to match, then substitutes the replacement lines.

//line common/common.w:885
type change struct {
//line common/common.w:886
	match []string // lines to find in the master source
//line common/common.w:887
	repl []string // lines to substitute for them
//line common/common.w:888
	line int // 1-based line of the @x in the change file (for diagnostics)
//line common/common.w:889
	replLine int // 1-based change-file line of the first replacement line
//line common/common.w:890
}

//line common/common.w:892
type srcLoc struct {
//line common/common.w:893
	file string
//line common/common.w:894
	line int
//line common/common.w:895
}

//line common/common.w:897
func (l srcLoc) String() string {
//line common/common.w:898
	if l.file == "" {
//line common/common.w:899
		return fmt.Sprintf("line %d", l.line)
//line common/common.w:900
	}
//line common/common.w:901
	return fmt.Sprintf("%s:%d", l.file, l.line)
//line common/common.w:902
}

//line common/common.w:907
func isChangeCtrl(line string, c byte) bool {
//line common/common.w:908
	return len(line) >= 2 && line[0] == '@' && line[1] == c
//line common/common.w:909
}

//line common/common.w:911
func splitLines(s string) []string {
//line common/common.w:912
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
//line common/common.w:913
}

//line common/common.w:915
func sameLine(a, b string) bool {
//line common/common.w:916
	return strings.TrimRight(a, " \t") == strings.TrimRight(b, " \t")
//line common/common.w:917
}

//line common/common.w:922
func parseChangeFile(src string) ([]change, error) {
//line common/common.w:923
	lines := splitLines(src)
//line common/common.w:924
	var changes []change
//line common/common.w:925
	n := len(lines)
//line common/common.w:926
	for i := 0; i < n; {
//line common/common.w:927
		if !isChangeCtrl(lines[i], 'x') {
//line common/common.w:928
			i++ // commentary between changes
//line common/common.w:929
			continue
//line common/common.w:930
		}
//line common/common.w:931
		c := change{line: i + 1}
//line common/common.w:932
		i++
//line common/common.w:933
		for i < n && !isChangeCtrl(lines[i], 'y') {
//line common/common.w:934
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'z') {
//line common/common.w:935
				return nil, fmt.Errorf("change file line %d: expected @y to close the @x match part", c.line)
//line common/common.w:936
			}
//line common/common.w:937
			c.match = append(c.match, lines[i])
//line common/common.w:938
			i++
//line common/common.w:939
		}
//line common/common.w:940
		if i >= n {
//line common/common.w:941
			return nil, fmt.Errorf("change file line %d: @x without a matching @y", c.line)
//line common/common.w:942
		}
//line common/common.w:943
		i++ // skip @y
//line common/common.w:944
		c.replLine = i + 1
//line common/common.w:945
		for i < n && !isChangeCtrl(lines[i], 'z') {
//line common/common.w:946
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'y') {
//line common/common.w:947
				return nil, fmt.Errorf("change file line %d: expected @z to close the change", c.line)
//line common/common.w:948
			}
//line common/common.w:949
			c.repl = append(c.repl, lines[i])
//line common/common.w:950
			i++
//line common/common.w:951
		}
//line common/common.w:952
		if i >= n {
//line common/common.w:953
			return nil, fmt.Errorf("change file line %d: change has no @z", c.line)
//line common/common.w:954
		}
//line common/common.w:955
		i++ // skip @z
//line common/common.w:956
		if len(c.match) == 0 {
//line common/common.w:957
			return nil, fmt.Errorf("change file line %d: the @x match part is empty", c.line)
//line common/common.w:958
		}
//line common/common.w:959
		changes = append(changes, c)
//line common/common.w:960
	}
//line common/common.w:961
	return changes, nil
//line common/common.w:962
}

//line common/common.w:966
func applyChanges(src string, changes []change, chFile string) (string, error) {
//line common/common.w:967
	out, _, err := applyChangesMapped(splitLines(src), nil, changes, chFile)
//line common/common.w:968
	if err != nil {
//line common/common.w:969
		return "", err
//line common/common.w:970
	}
//line common/common.w:971
	return strings.Join(out, "\n"), nil
//line common/common.w:972
}

//line common/common.w:979
func applyChangesMapped(master []string, locs []srcLoc, changes []change, chFile string) ([]string, []srcLoc, error) {
//line common/common.w:980
	loc := func(i int) srcLoc {
//line common/common.w:981
		if locs != nil && i < len(locs) {
//line common/common.w:982
			return locs[i]
//line common/common.w:983
		}
//line common/common.w:984
		return srcLoc{line: i + 1}
//line common/common.w:985
	}
//line common/common.w:986
	out := make([]string, 0, len(master))
//line common/common.w:987
	var outLocs []srcLoc
//line common/common.w:988
	ci := 0
//line common/common.w:989
	for i := 0; i < len(master); {
//line common/common.w:990
		if ci < len(changes) && sameLine(master[i], changes[ci].match[0]) {
//line common/common.w:991
			if !blockMatches(master, i, changes[ci].match) {
//line common/common.w:992
				return nil, nil, fmt.Errorf("%s:%d: change did not match the master source at %s",
//line common/common.w:993
					chFile, changes[ci].line, loc(i))
//line common/common.w:994
			}
//line common/common.w:995
			for r, rl := range changes[ci].repl {
//line common/common.w:996
				out = append(out, rl)
//line common/common.w:997
				outLocs = append(outLocs, srcLoc{chFile, changes[ci].replLine + r})
//line common/common.w:998
			}
//line common/common.w:999
			i += len(changes[ci].match)
//line common/common.w:1000
			ci++
//line common/common.w:1001
			continue
//line common/common.w:1002
		}
//line common/common.w:1003
		out = append(out, master[i])
//line common/common.w:1004
		outLocs = append(outLocs, loc(i))
//line common/common.w:1005
		i++
//line common/common.w:1006
	}
//line common/common.w:1007
	if ci < len(changes) {
//line common/common.w:1008
		return nil, nil, fmt.Errorf("%s:%d: change was never matched (looking for %q)",
//line common/common.w:1009
			chFile, changes[ci].line, changes[ci].match[0])
//line common/common.w:1010
	}
//line common/common.w:1011
	return out, outLocs, nil
//line common/common.w:1012
}

//line common/common.w:1017
func blockMatches(master []string, at int, match []string) bool {
//line common/common.w:1018
	if at+len(match) > len(master) {
//line common/common.w:1019
		return false
//line common/common.w:1020
	}
//line common/common.w:1021
	for k, m := range match {
//line common/common.w:1022
		if !sameLine(master[at+k], m) {
//line common/common.w:1023
			return false
//line common/common.w:1024
		}
//line common/common.w:1025
	}
//line common/common.w:1026
	return true
//line common/common.w:1027
}
