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
	Macro bool // \.{@d}: typeset Original in typewriter (a \.{CWEB}-style macro)
//line common/common.w:29
}

//line common/common.w:35
type Section struct {
//line common/common.w:36
	Number int // 1-based section number
//line common/common.w:37
	Line int // 1-based source line where the section begins
//line common/common.w:38
	Starred bool // true for \.{@*} sections
//line common/common.w:39
	Depth int // group depth for starred sections (-1 |==| \.{@**}, 0 |==| \.{@*}, n |==| \.{@*n})
//line common/common.w:40
	Title string // starred-section title (text up to the first period)
//line common/common.w:41
	Tex string // commentary, raw TeX with in-text\.{@}-codes still embedded
//line common/common.w:42
	Formats []Format
//line common/common.w:43
	HasCode bool // true if the section contributes code
//line common/common.w:44
	Name string // named-section name, or \.{""} for an unnamed @c section
//line common/common.w:45
	IsFile bool // true if the name is an output file (\.{@(file@>=})
//line common/common.w:46
	Code string // raw code text with in-code \.{@}-codes still embedded
//line common/common.w:47
	CodeLine int // 1-based combined-source line where Code begins (0 if none)
//line common/common.w:48
}

//line common/common.w:55
type Web struct {
//line common/common.w:56
	Limbo string
//line common/common.w:57
	Formats []Format // \.{@f}/\.{@s} directives found in limbo (apply globally)
//line common/common.w:58
	Sections []*Section
//line common/common.w:59
	Warnings []string // non-fatal diagnostics gathered while parsing/checking
//line common/common.w:60
	file string // source filename, for diagnostics (\.{""} if unknown)
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

//line common/common.w:102
func ParseString(src string) *Web {
//line common/common.w:103
	w := parse(src)
//line common/common.w:104
	w.finish(src)
//line common/common.w:105
	return w
//line common/common.w:106
}

//line common/common.w:108
func (w *Web) finish(src string) {
//line common/common.w:109
	w.collectNames()
//line common/common.w:110
	w.Warnings = append(w.Warnings, w.scanDiagnostics(src)...)
//line common/common.w:111
	w.Warnings = append(w.Warnings, w.checkNames()...)
//line common/common.w:112
}

//line common/common.w:114
func (w *Web) at(line int) string {
//line common/common.w:115
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line common/common.w:116
		return w.locs[i].String()
//line common/common.w:117
	}
//line common/common.w:118
	if w.file != "" {
//line common/common.w:119
		return fmt.Sprintf("%s:%d", w.file, line)
//line common/common.w:120
	}
//line common/common.w:121
	return fmt.Sprintf("line %d", line)
//line common/common.w:122
}

//line common/common.w:129
func (w *Web) Origin(line int) (file string, ln int) {
//line common/common.w:130
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line common/common.w:131
		return w.locs[i].file, w.locs[i].line
//line common/common.w:132
	}
//line common/common.w:133
	return w.file, line
//line common/common.w:134
}

//line common/common.w:140
func DefaultExt(name, ext string) string {
//line common/common.w:141
	if name == "" || filepath.Ext(name) != "" {
//line common/common.w:142
		return name
//line common/common.w:143
	}
//line common/common.w:144
	return name + ext
//line common/common.w:145
}

//line common/common.w:151
func expandIncludes(file string, depth int) ([]string, []srcLoc, error) {
//line common/common.w:152
	if depth > 25 {
//line common/common.w:153
		return nil, nil, fmt.Errorf("gweb: @i include nesting too deep at %q", file)
//line common/common.w:154
	}
//line common/common.w:155
	data, err := os.ReadFile(file)
//line common/common.w:156
	if err != nil {
//line common/common.w:157
		return nil, nil, err
//line common/common.w:158
	}
//line common/common.w:159
	raw := splitLines(string(data))
//line common/common.w:160
	if n := len(raw); n > 0 && raw[n-1] == "" {
//line common/common.w:161
		raw = raw[:n-1]
//line common/common.w:162
	}

//line common/common.w:164
	var lines []string
//line common/common.w:165
	var locs []srcLoc
//line common/common.w:166
	dir := filepath.Dir(file)
//line common/common.w:167
	for i, line := range raw {
//line common/common.w:168
		if name, ok := includeDirective(line); ok {
//line common/common.w:169
			path := name
//line common/common.w:170
			if !filepath.IsAbs(path) {
//line common/common.w:171
				path = filepath.Join(dir, name)
//line common/common.w:172
			}
//line common/common.w:173
			sub, subLocs, err := expandIncludes(path, depth+1)
//line common/common.w:174
			if err != nil {
//line common/common.w:175
				return nil, nil, fmt.Errorf("%s:%d: %w", file, i+1, err)
//line common/common.w:176
			}
//line common/common.w:177
			lines = append(lines, sub...)
//line common/common.w:178
			locs = append(locs, subLocs...)
//line common/common.w:179
			continue
//line common/common.w:180
		}
//line common/common.w:181
		lines = append(lines, line)
//line common/common.w:182
		locs = append(locs, srcLoc{file, i + 1})
//line common/common.w:183
	}
//line common/common.w:184
	return lines, locs, nil
//line common/common.w:185
}

//line common/common.w:188
func includeDirective(line string) (name string, ok bool) {
//line common/common.w:189
	t := strings.TrimLeft(line, " \t")
//line common/common.w:190
	if !strings.HasPrefix(t, "@i") {
//line common/common.w:191
		return "", false
//line common/common.w:192
	}
//line common/common.w:193
	rest := t[2:]
//line common/common.w:194
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
//line common/common.w:195
		return "", false
//line common/common.w:196
	}
//line common/common.w:197
	name = strings.Trim(strings.TrimSpace(rest), "\"")
//line common/common.w:198
	return name, name != ""
//line common/common.w:199
}

//line common/common.w:206
func (w *Web) collectNames() {
//line common/common.w:207
	seen := map[string]bool{}
//line common/common.w:208
	add := func(name string) {
//line common/common.w:209
		if name != "" && !strings.HasSuffix(name, "...") && !seen[name] {
//line common/common.w:210
			seen[name] = true
//line common/common.w:211
			w.full = append(w.full, name)
//line common/common.w:212
		}
//line common/common.w:213
	}
//line common/common.w:214
	for _, s := range w.Sections {
//line common/common.w:215
		if !s.IsFile {
//line common/common.w:216
			add(s.Name) // a definition's name
//line common/common.w:217
		}
//line common/common.w:218
		for _, raw := range []string{s.Code, s.Tex} {
//line common/common.w:219
			for _, a := range ScanCode(raw) {
//line common/common.w:220
				if a.Kind == ARef {
//line common/common.w:221
					add(a.Text) // a reference's name
//line common/common.w:222
				}
//line common/common.w:223
			}
//line common/common.w:224
		}
//line common/common.w:225
	}
//line common/common.w:226
}

//line common/common.w:228
func (w *Web) prefixMatches(prefix string) int {
//line common/common.w:229
	n := 0
//line common/common.w:230
	for _, full := range w.full {
//line common/common.w:231
		if strings.HasPrefix(full, prefix) {
//line common/common.w:232
			n++
//line common/common.w:233
		}
//line common/common.w:234
	}
//line common/common.w:235
	return n
//line common/common.w:236
}

//line common/common.w:243
func (w *Web) checkNames() []string {
//line common/common.w:244
	// "defined" is the set of sections that actually have a definition (not just
//line common/common.w:245
	// the full names known for abbreviation resolution, which include references).
//line common/common.w:246
	defined := map[string]bool{}
//line common/common.w:247
	for _, s := range w.Sections {
//line common/common.w:248
		if s.Name != "" && !s.IsFile {
//line common/common.w:249
			defined[w.Resolve(s.Name)] = true
//line common/common.w:250
		}
//line common/common.w:251
	}
//line common/common.w:252
	used := map[string]bool{}
//line common/common.w:253
	var warns []string

//line common/common.w:255
	for _, s := range w.Sections {
//line common/common.w:256
		scan := func(raw string) {
//line common/common.w:257
			for _, a := range ScanCode(raw) {
//line common/common.w:258
				if a.Kind != ARef {
//line common/common.w:259
					continue
//line common/common.w:260
				}
//line common/common.w:261
				canon := w.Resolve(a.Text)
//line common/common.w:262
				if strings.HasSuffix(a.Text, "...") && canon == a.Text {
//line common/common.w:263
					prefix := strings.TrimSpace(strings.TrimSuffix(a.Text, "..."))
//line common/common.w:264
					if m := w.prefixMatches(prefix); m == 0 {
//line common/common.w:265
						warns = append(warns, fmt.Sprintf("%s: no section name matches <%s>", w.at(s.Line), a.Text))
//line common/common.w:266
					} else {
//line common/common.w:267
						warns = append(warns, fmt.Sprintf("%s: ambiguous prefix <%s> matches %d section names", w.at(s.Line), a.Text, m))
//line common/common.w:268
					}
//line common/common.w:269
					continue
//line common/common.w:270
				}
//line common/common.w:271
				if !defined[canon] {
//line common/common.w:272
					warns = append(warns, fmt.Sprintf("%s: reference to undefined section <%s>", w.at(s.Line), a.Text))
//line common/common.w:273
				}
//line common/common.w:274
				used[canon] = true
//line common/common.w:275
			}
//line common/common.w:276
		}
//line common/common.w:277
		scan(s.Code)
//line common/common.w:278
		scan(s.Tex)
//line common/common.w:279
	}

//line common/common.w:281
	warned := map[string]bool{}
//line common/common.w:282
	for _, s := range w.Sections {
//line common/common.w:283
		if s.Name == "" || s.IsFile {
//line common/common.w:284
			continue
//line common/common.w:285
		}
//line common/common.w:286
		canon := w.Resolve(s.Name)
//line common/common.w:287
		if !used[canon] && !warned[canon] {
//line common/common.w:288
			warned[canon] = true
//line common/common.w:289
			warns = append(warns, fmt.Sprintf("%s: section <%s> is defined but never used", w.at(s.Line), s.Name))
//line common/common.w:290
		}
//line common/common.w:291
	}
//line common/common.w:292
	return warns
//line common/common.w:293
}

//line common/common.w:300
func lineAt(src string, off int) int {
//line common/common.w:301
	if off > len(src) {
//line common/common.w:302
		off = len(src)
//line common/common.w:303
	}
//line common/common.w:304
	return 1 + strings.Count(src[:off], "\n")
//line common/common.w:305
}

//line common/common.w:307
func canonName(name string) string {
//line common/common.w:308
	return strings.Join(strings.Fields(name), " ")
//line common/common.w:309
}

//line common/common.w:311
func (w *Web) Resolve(name string) string {
//line common/common.w:312
	name = canonName(name)
//line common/common.w:313
	if !strings.HasSuffix(name, "...") {
//line common/common.w:314
		return name
//line common/common.w:315
	}
//line common/common.w:316
	prefix := strings.TrimSpace(strings.TrimSuffix(name, "..."))
//line common/common.w:317
	var match string
//line common/common.w:318
	count := 0
//line common/common.w:319
	for _, full := range w.full {
//line common/common.w:320
		if strings.HasPrefix(full, prefix) {
//line common/common.w:321
			match = full
//line common/common.w:322
			count++
//line common/common.w:323
		}
//line common/common.w:324
	}
//line common/common.w:325
	if count == 1 {
//line common/common.w:326
		return match
//line common/common.w:327
	}
//line common/common.w:328
	return name // unresolved or ambiguous; leave as-is for caller to report
//line common/common.w:329
}

//line common/common.w:331
func indexFrom(s, sub string, from int) int {
//line common/common.w:332
	if from >= len(s) {
//line common/common.w:333
		return -1
//line common/common.w:334
	}
//line common/common.w:335
	idx := strings.Index(s[from:], sub)
//line common/common.w:336
	if idx < 0 {
//line common/common.w:337
		return -1
//line common/common.w:338
	}
//line common/common.w:339
	return from + idx
//line common/common.w:340
}

//line common/common.w:347
type ctrlKind int

//line common/common.w:349
const (
//line common/common.w:350
	cEOF ctrlKind = iota
//line common/common.w:351
	cSection
//line common/common.w:352
	cCode // \.{@c} (or its synonym \.{@p})
//line common/common.w:353
	cNamed // \.{@<name@>=} or \.{@(file@>=}
//line common/common.w:354
	cDefn // \.{@d}
//line common/common.w:355
	cFormat

//line common/common.w:356
)

//line common/common.w:358
type ctrl struct {
//line common/common.w:359
	kind ctrlKind
//line common/common.w:360
	pos int // index of the leading '\.{@}'
//line common/common.w:361
	end int // index just past the control token
//line common/common.w:362
	depth int // for cSection: -1 unstarred (or \.{@**} top level), else starred depth
//line common/common.w:363
	starred bool // for cSection (distinguishes \.{@**} from an unstarred section)
//line common/common.w:364
	name string // for cNamed
//line common/common.w:365
	isFile bool // for cNamed (\.{@(} vs \.{@<})
//line common/common.w:366
	noIndex bool // for cFormat (\.{@s})
//line common/common.w:367
}

//line common/common.w:374
func scanStruct(src string, i int) ctrl {
//line common/common.w:375
	n := len(src)
//line common/common.w:376
	for i < n {
//line common/common.w:377
		if src[i] != '@' {
//line common/common.w:378
			i++
//line common/common.w:379
			continue
//line common/common.w:380
		}
//line common/common.w:381
		if i+1 >= n {
//line common/common.w:382
			break
//line common/common.w:383
		}
//line common/common.w:384
		switch c := src[i+1]; {
//line common/common.w:385
		case c == '@':
//line common/common.w:386
			i += 2
//line common/common.w:387
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line common/common.w:388
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line common/common.w:389
		case c == '*':
//line common/common.w:390
			j := i + 2
//line common/common.w:391
			depth := 0
//line common/common.w:392
			if j < n && src[j] == '*' {
//line common/common.w:393
				j++
//line common/common.w:394
				depth = -1 // "@**" is the top level: bold in the contents, as cweb
//line common/common.w:395
			} else {
//line common/common.w:396
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:397
					depth = depth*10 + int(src[j]-'0')
//line common/common.w:398
					j++
//line common/common.w:399
				}
//line common/common.w:400
			}
//line common/common.w:401
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}
//line common/common.w:402
		case c == 'c' || c == 'p':
//line common/common.w:403
			return ctrl{kind: cCode, pos: i, end: i + 2}
//line common/common.w:404
		case c == 'd':
//line common/common.w:405
			return ctrl{kind: cDefn, pos: i, end: i + 2}
//line common/common.w:406
		case c == 'f':
//line common/common.w:407
			return ctrl{kind: cFormat, pos: i, end: i + 2}
//line common/common.w:408
		case c == 's':
//line common/common.w:409
			return ctrl{kind: cFormat, pos: i, end: i + 2, noIndex: true}
//line common/common.w:410
		case c == '<' || c == '(':
//line common/common.w:411
			end := indexFrom(src, "@>", i+2)
//line common/common.w:412
			if end < 0 {
//line common/common.w:413
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:414
			}
//line common/common.w:415
			after := end + 2
//line common/common.w:416
			k := after
//line common/common.w:417
			for k < n && (src[k] == ' ' || src[k] == '\t') {
//line common/common.w:418
				k++
//line common/common.w:419
			}
//line common/common.w:420
			if k < n && src[k] == '=' {
//line common/common.w:421
				return ctrl{kind: cNamed, pos: i, end: k + 1,
//line common/common.w:422
					name: canonName(src[i+2 : end]), isFile: c == '('}
//line common/common.w:423
			}
//line common/common.w:424
			i = after // a reference, not a definition
//line common/common.w:425
		case c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line common/common.w:426
			end := indexFrom(src, "@>", i+2)
//line common/common.w:427
			if end < 0 {
//line common/common.w:428
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:429
			}
//line common/common.w:430
			i = end + 2
//line common/common.w:431
		case c == '%':
//line common/common.w:432
			j := i + 2
//line common/common.w:433
			for j < n && src[j] != '\n' {
//line common/common.w:434
				j++
//line common/common.w:435
			}
//line common/common.w:436
			i = j
//line common/common.w:437
		default:
//line common/common.w:438
			i += 2
//line common/common.w:439
		}
//line common/common.w:440
	}
//line common/common.w:441
	return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:442
}

//line common/common.w:448
func findNextSection(src string, i int) ctrl {
//line common/common.w:449
	n := len(src)
//line common/common.w:450
	for i < n {
//line common/common.w:451
		if src[i] != '@' {
//line common/common.w:452
			i++
//line common/common.w:453
			continue
//line common/common.w:454
		}
//line common/common.w:455
		if i+1 >= n {
//line common/common.w:456
			break
//line common/common.w:457
		}
//line common/common.w:458
		switch c := src[i+1]; {
//line common/common.w:459
		case c == '@':
//line common/common.w:460
			i += 2
//line common/common.w:461
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line common/common.w:462
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line common/common.w:463
		case c == '*':
//line common/common.w:464
			j := i + 2
//line common/common.w:465
			depth := 0
//line common/common.w:466
			if j < n && src[j] == '*' {
//line common/common.w:467
				j++
//line common/common.w:468
				depth = -1 // "\.{@**}" is the top level: bold in the contents, as \.{CWEB}
//line common/common.w:469
			} else {
//line common/common.w:470
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:471
					depth = depth*10 + int(src[j]-'0')
//line common/common.w:472
					j++
//line common/common.w:473
				}
//line common/common.w:474
			}
//line common/common.w:475
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}
//line common/common.w:476
		case c == '<' || c == '(' || c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line common/common.w:477
			end := indexFrom(src, "@>", i+2)
//line common/common.w:478
			if end < 0 {
//line common/common.w:479
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:480
			}
//line common/common.w:481
			i = end + 2
//line common/common.w:482
		case c == '%':
//line common/common.w:483
			j := i + 2
//line common/common.w:484
			for j < n && src[j] != '\n' {
//line common/common.w:485
				j++
//line common/common.w:486
			}
//line common/common.w:487
			i = j
//line common/common.w:488
		default:
//line common/common.w:489
			i += 2
//line common/common.w:490
		}
//line common/common.w:491
	}
//line common/common.w:492
	return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:493
}

//line common/common.w:501
func parse(src string) *Web {
//line common/common.w:502
	w := &Web{}
//line common/common.w:503
	n := len(src)

//line common/common.w:505
	first := findNextSection(src, 0)
//line common/common.w:506
	w.Limbo, w.Formats = extractLimboFormats(src[:first.pos])
//line common/common.w:507
	i := first.pos

//line common/common.w:509
	num := 0
//line common/common.w:510
	for i < n {
//line common/common.w:511
		// We are positioned at a section break.
//line common/common.w:512
		hdr := src[i+1]
//line common/common.w:513
		num++
//line common/common.w:514
		sec := &Section{Number: num, Line: lineAt(src, i)}
//line common/common.w:515
		if hdr == '*' {
//line common/common.w:516
			h := findSectionHeaderEnd(src, i)
//line common/common.w:517
			sec.Starred = true
//line common/common.w:518
			sec.Depth = h.depth
//line common/common.w:519
			i = h.end
//line common/common.w:520
		} else {
//line common/common.w:521
			i += 2
//line common/common.w:522
		}

//line common/common.w:524
		// TeX part: from here to the next structural control.
//line common/common.w:525
		ct := scanStruct(src, i)
//line common/common.w:526
		sec.Tex = src[i:ct.pos]
//line common/common.w:527
		if sec.Starred {
//line common/common.w:528
			sec.Title = extractTitle(sec.Tex)
//line common/common.w:529
		}

//line common/common.w:531
		// Definition part: a run of \.{@d} / \.{@f} / \.{@s}.
//line common/common.w:532
		for ct.kind == cDefn || ct.kind == cFormat {
//line common/common.w:533
			nx := scanStruct(src, ct.end)
//line common/common.w:534
			seg := src[ct.end:nx.pos]
//line common/common.w:535
			// \.{@d} has no Go analogue (Go has no preprocessor), so it never tangles
//line common/common.w:536
			// to code; gweave uses it only to set the named identifier in
//line common/common.w:537
			// typewriter, as cweave sets a macro. \.{@f}/\.{@s} format like another word.
//line common/common.w:538
			if ct.kind == cDefn {
//line common/common.w:539
				if f, ok := parseMacro(seg); ok {
//line common/common.w:540
					sec.Formats = append(sec.Formats, f)
//line common/common.w:541
				}
//line common/common.w:542
			} else if f, ok := parseFormat(seg, ct.noIndex); ok {
//line common/common.w:543
				sec.Formats = append(sec.Formats, f)
//line common/common.w:544
			}
//line common/common.w:545
			ct = nx
//line common/common.w:546
		}

//line common/common.w:548
		switch ct.kind {
//line common/common.w:549
		case cCode:
//line common/common.w:550
			sec.HasCode = true
//line common/common.w:551
			sec.CodeLine = lineAt(src, ct.end)
//line common/common.w:552
			nx := findNextSection(src, ct.end)
//line common/common.w:553
			sec.Code = src[ct.end:nx.pos]
//line common/common.w:554
			i = nx.pos
//line common/common.w:555
		case cNamed:
//line common/common.w:556
			sec.HasCode = true
//line common/common.w:557
			sec.Name = ct.name
//line common/common.w:558
			sec.IsFile = ct.isFile
//line common/common.w:559
			sec.CodeLine = lineAt(src, ct.end)
//line common/common.w:560
			nx := findNextSection(src, ct.end)
//line common/common.w:561
			sec.Code = src[ct.end:nx.pos]
//line common/common.w:562
			i = nx.pos
//line common/common.w:563
		default: // cSection or cEOF: a documentation-only section
//line common/common.w:564
			i = ct.pos
//line common/common.w:565
		}

//line common/common.w:567
		w.Sections = append(w.Sections, sec)
//line common/common.w:568
		if ct.kind == cEOF && sec.Code == "" {
//line common/common.w:569
			break
//line common/common.w:570
		}
//line common/common.w:571
		if i >= n {
//line common/common.w:572
			break
//line common/common.w:573
		}
//line common/common.w:574
	}
//line common/common.w:575
	return w
//line common/common.w:576
}

//line common/common.w:580
func findSectionHeaderEnd(src string, i int) ctrl {
//line common/common.w:581
	n := len(src)
//line common/common.w:582
	j := i + 2
//line common/common.w:583
	depth := 0
//line common/common.w:584
	if j < n && src[j] == '*' {
//line common/common.w:585
		j++
//line common/common.w:586
		depth = -1 // ``\.{@**}'' is the top level: bold in the contents, as \.{CWEB}
//line common/common.w:587
	} else {
//line common/common.w:588
		for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:589
			depth = depth*10 + int(src[j]-'0')
//line common/common.w:590
			j++
//line common/common.w:591
		}
//line common/common.w:592
	}
//line common/common.w:593
	return ctrl{end: j, depth: depth}
//line common/common.w:594
}

//line common/common.w:599
func extractTitle(tex string) string {
//line common/common.w:600
	t := strings.TrimLeft(tex, " \t\n")
//line common/common.w:601
	if i := titleEnd(t); i >= 0 {
//line common/common.w:602
		t = t[:i]
//line common/common.w:603
	}
//line common/common.w:604
	return strings.Join(strings.Fields(t), " ")
//line common/common.w:605
}

//line common/common.w:607
func titleEnd(s string) int {
//line common/common.w:608
	for i := 0; i < len(s); i++ {
//line common/common.w:609
		if s[i] == '.' && (i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t' ||
//line common/common.w:610
			s[i+1] == '\n' || s[i+1] == '\r') {
//line common/common.w:611
			return i
//line common/common.w:612
		}
//line common/common.w:613
	}
//line common/common.w:614
	return -1
//line common/common.w:615
}

//line common/common.w:620
func (w *Web) scanDiagnostics(src string) []string {
//line common/common.w:621
	var warns []string
//line common/common.w:622
	n := len(src)
//line common/common.w:623
	i := 0
//line common/common.w:624
	for i < n {
//line common/common.w:625
		if src[i] != '@' || i+1 >= n {
//line common/common.w:626
			i++
//line common/common.w:627
			continue
//line common/common.w:628
		}
//line common/common.w:629
		switch c := src[i+1]; c {
//line common/common.w:630
		case '@':
//line common/common.w:631
			i += 2
//line common/common.w:632
		case '<', '(', '=', 't', '^', '.', ':', 'q':
//line common/common.w:633
			if end := indexFrom(src, "@>", i+2); end < 0 {
//line common/common.w:634
				warns = append(warns, fmt.Sprintf("%s: unterminated `@%c ... @>'", w.at(lineAt(src, i)), c))
//line common/common.w:635
				i = n
//line common/common.w:636
			} else {
//line common/common.w:637
				i = end + 2
//line common/common.w:638
			}
//line common/common.w:639
		default:
//line common/common.w:640
			i += 2
//line common/common.w:641
		}
//line common/common.w:642
	}
//line common/common.w:643
	return warns
//line common/common.w:644
}

//line common/common.w:648
func parseFormat(seg string, noIndex bool) (Format, bool) {
//line common/common.w:649
	fields := strings.Fields(seg)
//line common/common.w:650
	if len(fields) < 2 {
//line common/common.w:651
		return Format{}, false
//line common/common.w:652
	}
//line common/common.w:653
	return Format{Original: fields[0], Like: fields[1], NoIndex: noIndex}, true
//line common/common.w:654
}

//line common/common.w:662
func parseMacro(seg string) (Format, bool) {
//line common/common.w:663
	fields := strings.Fields(seg)
//line common/common.w:664
	if len(fields) == 0 {
//line common/common.w:665
		return Format{}, false
//line common/common.w:666
	}
//line common/common.w:667
	name := fields[0]
//line common/common.w:668
	if k := strings.LastIndex(name, "."); k >= 0 {
//line common/common.w:669
		name = name[k+1:]
//line common/common.w:670
	}
//line common/common.w:671
	if name == "" {
//line common/common.w:672
		return Format{}, false
//line common/common.w:673
	}
//line common/common.w:674
	return Format{Original: name, Macro: true}, true
//line common/common.w:675
}

//line common/common.w:681
func extractLimboFormats(src string) (string, []Format) {
//line common/common.w:682
	var b strings.Builder
//line common/common.w:683
	var formats []Format
//line common/common.w:684
	n := len(src)
//line common/common.w:685
	i := 0
//line common/common.w:686
	for i < n {
//line common/common.w:687
		if src[i] != '@' || i+1 >= n {
//line common/common.w:688
			b.WriteByte(src[i])
//line common/common.w:689
			i++
//line common/common.w:690
			continue
//line common/common.w:691
		}
//line common/common.w:692
		switch c := src[i+1]; c {
//line common/common.w:693
		case '@':
//line common/common.w:694
			b.WriteString("@@")
//line common/common.w:695
			i += 2
//line common/common.w:696
		case 'd', 'f', 's':
//line common/common.w:697
			j := i + 2
//line common/common.w:698
			for j < n && src[j] != '\n' {
//line common/common.w:699
				j++
//line common/common.w:700
			}
//line common/common.w:701
			var f Format
//line common/common.w:702
			var ok bool
//line common/common.w:703
			if c == 'd' {
//line common/common.w:704
				f, ok = parseMacro(src[i+2 : j])
//line common/common.w:705
			} else {
//line common/common.w:706
				f, ok = parseFormat(src[i+2:j], c == 's')
//line common/common.w:707
			}
//line common/common.w:708
			if ok {
//line common/common.w:709
				formats = append(formats, f)
//line common/common.w:710
			}
//line common/common.w:711
			if j < n {
//line common/common.w:712
				j++ // also drop the newline that ended the directive
//line common/common.w:713
			}
//line common/common.w:714
			i = j
//line common/common.w:715
		case '<', '(', '=', 't', '^', '.', ':', 'q':
//line common/common.w:716
			end := indexFrom(src, "@>", i+2)
//line common/common.w:717
			if end < 0 {
//line common/common.w:718
				b.WriteString(src[i:])
//line common/common.w:719
				i = n
//line common/common.w:720
			} else {
//line common/common.w:721
				b.WriteString(src[i : end+2])
//line common/common.w:722
				i = end + 2
//line common/common.w:723
			}
//line common/common.w:724
		default:
//line common/common.w:725
			b.WriteString(src[i : i+2])
//line common/common.w:726
			i += 2
//line common/common.w:727
		}
//line common/common.w:728
	}
//line common/common.w:729
	return b.String(), formats
//line common/common.w:730
}

//line common/common.w:737
type AtomKind int

//line common/common.w:739
const (
//line common/common.w:740
	AText AtomKind = iota // ordinary Go source text
//line common/common.w:741
	ARef // \.{@<name@>} reference to a named section
//line common/common.w:742
	AVerbatim // \.{@=text@>} passed verbatim to tangled output
//line common/common.w:743
	ATeX // \.{@t text@>} TeX text for the woven output
//line common/common.w:744
	AIndex // \.{@\^/@./@}: index entry
//line common/common.w:745
	APaste // \.{@\&} join (delete surrounding whitespace)
//line common/common.w:746
	ALayout // \.{@}, \.{@/} \.{@|} \.{@\#} woven-output layout hints
//line common/common.w:747
	AIndexDef // \.{@!} force the next identifier to index as a definition
//line common/common.w:748
)

//line common/common.w:750
type Atom struct {
//line common/common.w:751
	Kind AtomKind
//line common/common.w:752
	Text string // payload for |AText|/|AVerbatim|/|ATeX|/|AIndex|; name for |ARef|
//line common/common.w:753
	Index byte // '\.{\^}','\.{.}','\.{:}' for AIndex; '\.{,}' '\.{/}' '\.{|}' '\.{\#}' for |ALayout|
//line common/common.w:754
}

//line common/common.w:760
func ScanCode(code string) []Atom {
//line common/common.w:761
	var atoms []Atom
//line common/common.w:762
	var buf strings.Builder
//line common/common.w:763
	flush := func() {
//line common/common.w:764
		if buf.Len() > 0 {
//line common/common.w:765
			atoms = append(atoms, Atom{Kind: AText, Text: buf.String()})
//line common/common.w:766
			buf.Reset()
//line common/common.w:767
		}
//line common/common.w:768
	}
//line common/common.w:769
	n := len(code)
//line common/common.w:770
	i := 0
//line common/common.w:771
	for i < n {
//line common/common.w:772
		c := code[i]
//line common/common.w:773
		if c != '@' || i+1 >= n {
//line common/common.w:774
			buf.WriteByte(c)
//line common/common.w:775
			i++
//line common/common.w:776
			continue
//line common/common.w:777
		}
//line common/common.w:778
		switch d := code[i+1]; d {
//line common/common.w:779
		case '@':
//line common/common.w:780
			buf.WriteByte('@')
//line common/common.w:781
			i += 2
//line common/common.w:782
		case '&':
//line common/common.w:783
			flush()
//line common/common.w:784
			atoms = append(atoms, Atom{Kind: APaste})
//line common/common.w:785
			i += 2
//line common/common.w:786
		case '<':
//line common/common.w:787
			end := indexFrom(code, "@>", i+2)
//line common/common.w:788
			if end < 0 {
//line common/common.w:789
				buf.WriteString(code[i:])
//line common/common.w:790
				i = n
//line common/common.w:791
				continue
//line common/common.w:792
			}
//line common/common.w:793
			flush()
//line common/common.w:794
			atoms = append(atoms, Atom{Kind: ARef, Text: canonName(code[i+2 : end])})
//line common/common.w:795
			i = end + 2
//line common/common.w:796
		case '=':
//line common/common.w:797
			end := indexFrom(code, "@>", i+2)
//line common/common.w:798
			if end < 0 {
//line common/common.w:799
				i = n
//line common/common.w:800
				continue
//line common/common.w:801
			}
//line common/common.w:802
			flush()
//line common/common.w:803
			atoms = append(atoms, Atom{Kind: AVerbatim, Text: code[i+2 : end]})
//line common/common.w:804
			i = end + 2
//line common/common.w:805
		case 't':
//line common/common.w:806
			end := indexFrom(code, "@>", i+2)
//line common/common.w:807
			if end < 0 {
//line common/common.w:808
				i = n
//line common/common.w:809
				continue
//line common/common.w:810
			}
//line common/common.w:811
			flush()
//line common/common.w:812
			atoms = append(atoms, Atom{Kind: ATeX, Text: code[i+2 : end]})
//line common/common.w:813
			i = end + 2
//line common/common.w:814
		case '^', '.', ':':
//line common/common.w:815
			end := indexFrom(code, "@>", i+2)
//line common/common.w:816
			if end < 0 {
//line common/common.w:817
				i = n
//line common/common.w:818
				continue
//line common/common.w:819
			}
//line common/common.w:820
			flush()
//line common/common.w:821
			atoms = append(atoms, Atom{Kind: AIndex, Text: code[i+2 : end], Index: d})
//line common/common.w:822
			i = end + 2
//line common/common.w:823
		case 'q':
//line common/common.w:824
			end := indexFrom(code, "@>", i+2)
//line common/common.w:825
			if end < 0 {
//line common/common.w:826
				i = n
//line common/common.w:827
				continue
//line common/common.w:828
			}
//line common/common.w:829
			i = end + 2 // ignored material
//line common/common.w:830
		case '%':
//line common/common.w:831
			j := i + 2
//line common/common.w:832
			for j < n && code[j] != '\n' {
//line common/common.w:833
				j++
//line common/common.w:834
			}
//line common/common.w:835
			i = j
//line common/common.w:836
		case '>':
//line common/common.w:837
			i += 2 // stray terminator
//line common/common.w:838
		case ',', '/', '|', '#':
//line common/common.w:839
			// Woven-output layout hints: thin space, line break, optional line
//line common/common.w:840
			// break, and break-plus-blank-line. Ignored by gtangle.
//line common/common.w:841
			flush()
//line common/common.w:842
			atoms = append(atoms, Atom{Kind: ALayout, Index: d})
//line common/common.w:843
			i += 2
//line common/common.w:844
		case '!':
//line common/common.w:845
			// Force the next identifier's index entry to be a definition,
//line common/common.w:846
			// overriding the heuristic. Produces no output by itself.
//line common/common.w:847
			flush()
//line common/common.w:848
			atoms = append(atoms, Atom{Kind: AIndexDef})
//line common/common.w:849
			i += 2
//line common/common.w:850
		case '+', '[', ']', ';':
//line common/common.w:851
			// \.{CWEB} prettyprinter hints (cancel break, expression brackets,
//line common/common.w:852
			// invisible semicolon). \.{GWEB} mirrors the source instead of reflowing
//line common/common.w:853
			// it, so these have no effect; accept and drop them for portability.
//line common/common.w:854
			i += 2
//line common/common.w:855
		default:
//line common/common.w:856
			i += 2 // unknown \.{@x}: drop it rather than corrupt the output
//line common/common.w:857
		}
//line common/common.w:858
	}
//line common/common.w:859
	flush()
//line common/common.w:860
	return atoms
//line common/common.w:861
}

//line common/common.w:878
type change struct {
//line common/common.w:879
	match []string // lines to find in the master source
//line common/common.w:880
	repl []string // lines to substitute for them
//line common/common.w:881
	line int // 1-based line of the \.{@x} in the change file (for diagnostics)
//line common/common.w:882
	replLine int // 1-based change-file line of the first replacement line
//line common/common.w:883
}

//line common/common.w:885
type srcLoc struct {
//line common/common.w:886
	file string
//line common/common.w:887
	line int
//line common/common.w:888
}

//line common/common.w:890
func (l srcLoc) String() string {
//line common/common.w:891
	if l.file == "" {
//line common/common.w:892
		return fmt.Sprintf("line %d", l.line)
//line common/common.w:893
	}
//line common/common.w:894
	return fmt.Sprintf("%s:%d", l.file, l.line)
//line common/common.w:895
}

//line common/common.w:900
func isChangeCtrl(line string, c byte) bool {
//line common/common.w:901
	return len(line) >= 2 && line[0] == '@' && line[1] == c
//line common/common.w:902
}

//line common/common.w:904
func splitLines(s string) []string {
//line common/common.w:905
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
//line common/common.w:906
}

//line common/common.w:908
func sameLine(a, b string) bool {
//line common/common.w:909
	return strings.TrimRight(a, " \t") == strings.TrimRight(b, " \t")
//line common/common.w:910
}

//line common/common.w:915
func parseChangeFile(src string) ([]change, error) {
//line common/common.w:916
	lines := splitLines(src)
//line common/common.w:917
	var changes []change
//line common/common.w:918
	n := len(lines)
//line common/common.w:919
	for i := 0; i < n; {
//line common/common.w:920
		if !isChangeCtrl(lines[i], 'x') {
//line common/common.w:921
			i++ // commentary between changes
//line common/common.w:922
			continue
//line common/common.w:923
		}
//line common/common.w:924
		c := change{line: i + 1}
//line common/common.w:925
		i++
//line common/common.w:926
		for i < n && !isChangeCtrl(lines[i], 'y') {
//line common/common.w:927
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'z') {
//line common/common.w:928
				return nil, fmt.Errorf("change file line %d: expected @y to close the @x match part", c.line)
//line common/common.w:929
			}
//line common/common.w:930
			c.match = append(c.match, lines[i])
//line common/common.w:931
			i++
//line common/common.w:932
		}
//line common/common.w:933
		if i >= n {
//line common/common.w:934
			return nil, fmt.Errorf("change file line %d: @x without a matching @y", c.line)
//line common/common.w:935
		}
//line common/common.w:936
		i++ // skip @y
//line common/common.w:937
		c.replLine = i + 1
//line common/common.w:938
		for i < n && !isChangeCtrl(lines[i], 'z') {
//line common/common.w:939
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'y') {
//line common/common.w:940
				return nil, fmt.Errorf("change file line %d: expected @z to close the change", c.line)
//line common/common.w:941
			}
//line common/common.w:942
			c.repl = append(c.repl, lines[i])
//line common/common.w:943
			i++
//line common/common.w:944
		}
//line common/common.w:945
		if i >= n {
//line common/common.w:946
			return nil, fmt.Errorf("change file line %d: change has no @z", c.line)
//line common/common.w:947
		}
//line common/common.w:948
		i++ // skip @z
//line common/common.w:949
		if len(c.match) == 0 {
//line common/common.w:950
			return nil, fmt.Errorf("change file line %d: the @x match part is empty", c.line)
//line common/common.w:951
		}
//line common/common.w:952
		changes = append(changes, c)
//line common/common.w:953
	}
//line common/common.w:954
	return changes, nil
//line common/common.w:955
}

//line common/common.w:959
func applyChanges(src string, changes []change, chFile string) (string, error) {
//line common/common.w:960
	out, _, err := applyChangesMapped(splitLines(src), nil, changes, chFile)
//line common/common.w:961
	if err != nil {
//line common/common.w:962
		return "", err
//line common/common.w:963
	}
//line common/common.w:964
	return strings.Join(out, "\n"), nil
//line common/common.w:965
}

//line common/common.w:972
func applyChangesMapped(master []string, locs []srcLoc, changes []change, chFile string) ([]string, []srcLoc, error) {
//line common/common.w:973
	loc := func(i int) srcLoc {
//line common/common.w:974
		if locs != nil && i < len(locs) {
//line common/common.w:975
			return locs[i]
//line common/common.w:976
		}
//line common/common.w:977
		return srcLoc{line: i + 1}
//line common/common.w:978
	}
//line common/common.w:979
	out := make([]string, 0, len(master))
//line common/common.w:980
	var outLocs []srcLoc
//line common/common.w:981
	ci := 0
//line common/common.w:982
	for i := 0; i < len(master); {
//line common/common.w:983
		if ci < len(changes) && sameLine(master[i], changes[ci].match[0]) {
//line common/common.w:984
			if !blockMatches(master, i, changes[ci].match) {
//line common/common.w:985
				return nil, nil, fmt.Errorf("%s:%d: change did not match the master source at %s",
//line common/common.w:986
					chFile, changes[ci].line, loc(i))
//line common/common.w:987
			}
//line common/common.w:988
			for r, rl := range changes[ci].repl {
//line common/common.w:989
				out = append(out, rl)
//line common/common.w:990
				outLocs = append(outLocs, srcLoc{chFile, changes[ci].replLine + r})
//line common/common.w:991
			}
//line common/common.w:992
			i += len(changes[ci].match)
//line common/common.w:993
			ci++
//line common/common.w:994
			continue
//line common/common.w:995
		}
//line common/common.w:996
		out = append(out, master[i])
//line common/common.w:997
		outLocs = append(outLocs, loc(i))
//line common/common.w:998
		i++
//line common/common.w:999
	}
//line common/common.w:1000
	if ci < len(changes) {
//line common/common.w:1001
		return nil, nil, fmt.Errorf("%s:%d: change was never matched (looking for %q)",
//line common/common.w:1002
			chFile, changes[ci].line, changes[ci].match[0])
//line common/common.w:1003
	}
//line common/common.w:1004
	return out, outLocs, nil
//line common/common.w:1005
}

//line common/common.w:1010
func blockMatches(master []string, at int, match []string) bool {
//line common/common.w:1011
	if at+len(match) > len(master) {
//line common/common.w:1012
		return false
//line common/common.w:1013
	}
//line common/common.w:1014
	for k, m := range match {
//line common/common.w:1015
		if !sameLine(master[at+k], m) {
//line common/common.w:1016
			return false
//line common/common.w:1017
		}
//line common/common.w:1018
	}
//line common/common.w:1019
	return true
//line common/common.w:1020
}
