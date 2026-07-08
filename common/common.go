//line common/common.w:14
package common

//line common/common.w:16
import (
//line common/common.w:17
	"fmt"
//line common/common.w:18
	"os"
//line common/common.w:19
	"path/filepath"
//line common/common.w:20
	"strings"
//line common/common.w:21
)

//line common/common.w:23
const Version = "0.4.9"

//line common/common.w:38
type Format struct {
//line common/common.w:39
	Original string
//line common/common.w:40
	Like string
//line common/common.w:41
	NoIndex bool
//line common/common.w:42
	Macro bool // \.{@d}: typeset Original in typewriter (a \.{CWEB}-style macro)
//line common/common.w:43
}

//line common/common.w:49
type Section struct {
//line common/common.w:50
	Number int // 1-based section number
//line common/common.w:51
	Line int // 1-based source line where the section begins
//line common/common.w:52
	Starred bool // true for \.{@*} sections
//line common/common.w:53
	Depth int // group depth for starred sections (-1 |==| \.{@**}, 0 |==| \.{@*}, n |==| \.{@*n})
//line common/common.w:54
	Title string // starred-section title (text up to the first period)
//line common/common.w:55
	Tex string // commentary, raw \TEX/ with in-text\.{@}-codes still embedded
//line common/common.w:56
	Formats []Format
//line common/common.w:57
	HasCode bool // true if the section contributes code
//line common/common.w:58
	Name string // named-section name, or \.{""} for an unnamed @c section
//line common/common.w:59
	IsFile bool // true if the name is an output file (\.{@(file@>=})
//line common/common.w:60
	Code string // raw code text with in-code \.{@}-codes still embedded
//line common/common.w:61
	CodeLine int // 1-based combined-source line where Code begins (0 if none)
//line common/common.w:62
}

//line common/common.w:69
type Web struct {
//line common/common.w:70
	Limbo string
//line common/common.w:71
	Formats []Format // \.{@f}/\.{@s} directives found in limbo (apply globally)
//line common/common.w:72
	Sections []*Section
//line common/common.w:73
	Warnings []string // non-fatal diagnostics gathered while parsing/checking
//line common/common.w:74
	file string // source filename, for diagnostics (\.{""} if unknown)
//line common/common.w:75
	locs []srcLoc // origin (file, line) of each combined-source line
//line common/common.w:76
	full []string // canonical (non-abbreviated) section names
//line common/common.w:77
}

//line common/common.w:84
func Parse(filename string) (*Web, error) {
//line common/common.w:85
	return ParseWithChange(filename, "")
//line common/common.w:86
}

//line common/common.w:88
func ParseWithChange(filename, changeFile string) (*Web, error) {
//line common/common.w:89
	lines, locs, err := expandIncludes(filename, 0)
//line common/common.w:90
	if err != nil {
//line common/common.w:91
		return nil, err
//line common/common.w:92
	}
//line common/common.w:93

//line common/common.w:106
	if changeFile != "" {
//line common/common.w:107
		chData, err := os.ReadFile(changeFile)
//line common/common.w:108
		if err != nil {
//line common/common.w:109
			return nil, err
//line common/common.w:110
		}
//line common/common.w:111
		changes, err := parseChangeFile(string(chData))
//line common/common.w:112
		if err != nil {
//line common/common.w:113
			return nil, err
//line common/common.w:114
		}
//line common/common.w:115
		lines, locs, err = applyChangesMapped(lines, locs, changes, changeFile)
//line common/common.w:116
		if err != nil {
//line common/common.w:117
			return nil, err
//line common/common.w:118
		}
//line common/common.w:119
	}

//line common/common.w:94
	src := strings.Join(lines, "\n")
//line common/common.w:95
	w := parse(src)
//line common/common.w:96
	w.file = filename
//line common/common.w:97
	w.locs = locs
//line common/common.w:98
	w.finish(src)
//line common/common.w:99
	return w, nil
//line common/common.w:100
}

//line common/common.w:125
func ParseString(src string) *Web {
//line common/common.w:126
	w := parse(src)
//line common/common.w:127
	w.finish(src)
//line common/common.w:128
	return w
//line common/common.w:129
}

//line common/common.w:131
func (w *Web) finish(src string) {
//line common/common.w:132
	w.collectNames()
//line common/common.w:133
	w.Warnings = append(w.Warnings, w.scanDiagnostics(src)...)
//line common/common.w:134
	w.Warnings = append(w.Warnings, w.checkNames()...)
//line common/common.w:135
}

//line common/common.w:143
func (w *Web) Origin(line int) (file string, ln int) {
//line common/common.w:144
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line common/common.w:145
		return w.locs[i].file, w.locs[i].line
//line common/common.w:146
	}
//line common/common.w:147
	return w.file, line
//line common/common.w:148
}

//line common/common.w:150
func (w *Web) at(line int) string {
//line common/common.w:151
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line common/common.w:152
		return w.locs[i].String()
//line common/common.w:153
	}
//line common/common.w:154
	if w.file != "" {
//line common/common.w:155
		return fmt.Sprintf("%s:%d", w.file, line)
//line common/common.w:156
	}
//line common/common.w:157
	return fmt.Sprintf("line %d", line)
//line common/common.w:158
}

//line common/common.w:164
func DefaultExt(name, ext string) string {
//line common/common.w:165
	if name == "" || filepath.Ext(name) != "" {
//line common/common.w:166
		return name
//line common/common.w:167
	}
//line common/common.w:168
	return name + ext
//line common/common.w:169
}

//line common/common.w:175
func expandIncludes(file string, depth int) ([]string, []srcLoc, error) {
//line common/common.w:176
	if depth > 25 {
//line common/common.w:177
		return nil, nil, fmt.Errorf("gweb: @i include nesting too deep at %q", file)
//line common/common.w:178
	}
//line common/common.w:179
	data, err := os.ReadFile(file)
//line common/common.w:180
	if err != nil {
//line common/common.w:181
		return nil, nil, err
//line common/common.w:182
	}
//line common/common.w:183
	raw := splitLines(string(data))
//line common/common.w:184
	if n := len(raw); n > 0 && raw[n-1] == "" {
//line common/common.w:185
		raw = raw[:n-1]
//line common/common.w:186
	}

//line common/common.w:188
	var lines []string
//line common/common.w:189
	var locs []srcLoc
//line common/common.w:190
	dir := filepath.Dir(file)
//line common/common.w:191
	for i, line := range raw {
//line common/common.w:192
		if name, ok := includeDirective(line); ok {
//line common/common.w:193
			path := name
//line common/common.w:194
			if !filepath.IsAbs(path) {
//line common/common.w:195
				path = filepath.Join(dir, name)
//line common/common.w:196
			}
//line common/common.w:197
			sub, subLocs, err := expandIncludes(path, depth+1)
//line common/common.w:198
			if err != nil {
//line common/common.w:199
				return nil, nil, fmt.Errorf("%s:%d: %w", file, i+1, err)
//line common/common.w:200
			}
//line common/common.w:201
			lines = append(lines, sub...)
//line common/common.w:202
			locs = append(locs, subLocs...)
//line common/common.w:203
			continue
//line common/common.w:204
		}
//line common/common.w:205
		lines = append(lines, line)
//line common/common.w:206
		locs = append(locs, srcLoc{file, i + 1})
//line common/common.w:207
	}
//line common/common.w:208
	return lines, locs, nil
//line common/common.w:209
}

//line common/common.w:212
func includeDirective(line string) (name string, ok bool) {
//line common/common.w:213
	t := strings.TrimLeft(line, " \t")
//line common/common.w:214
	if !strings.HasPrefix(t, "@i") {
//line common/common.w:215
		return "", false
//line common/common.w:216
	}
//line common/common.w:217
	rest := t[2:]
//line common/common.w:218
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
//line common/common.w:219
		return "", false
//line common/common.w:220
	}
//line common/common.w:221
	name = strings.Trim(strings.TrimSpace(rest), "\"")
//line common/common.w:222
	return name, name != ""
//line common/common.w:223
}

//line common/common.w:230
func (w *Web) collectNames() {
//line common/common.w:231
	seen := map[string]bool{}
//line common/common.w:232
	add := func(name string) {
//line common/common.w:233
		if name != "" && !strings.HasSuffix(name, "...") && !seen[name] {
//line common/common.w:234
			seen[name] = true
//line common/common.w:235
			w.full = append(w.full, name)
//line common/common.w:236
		}
//line common/common.w:237
	}
//line common/common.w:238
	for _, s := range w.Sections {
//line common/common.w:239
		if !s.IsFile {
//line common/common.w:240
			add(s.Name) // a definition's name
//line common/common.w:241
		}
//line common/common.w:242
		for _, raw := range []string{s.Code, s.Tex} {
//line common/common.w:243
			for _, a := range ScanCode(raw) {
//line common/common.w:244
				if a.Kind == ARef {
//line common/common.w:245
					add(a.Text) // a reference's name
//line common/common.w:246
				}
//line common/common.w:247
			}
//line common/common.w:248
		}
//line common/common.w:249
	}
//line common/common.w:250
}

//line common/common.w:255
func (w *Web) prefixMatches(prefix string) int {
//line common/common.w:256
	n := 0
//line common/common.w:257
	for _, full := range w.full {
//line common/common.w:258
		if strings.HasPrefix(full, prefix) {
//line common/common.w:259
			n++
//line common/common.w:260
		}
//line common/common.w:261
	}
//line common/common.w:262
	return n
//line common/common.w:263
}

//line common/common.w:274
func (w *Web) checkNames() []string {
//line common/common.w:275
	defined := map[string]bool{}
//line common/common.w:276
	for _, s := range w.Sections {
//line common/common.w:277
		if s.Name != "" && !s.IsFile {
//line common/common.w:278
			defined[w.Resolve(s.Name)] = true
//line common/common.w:279
		}
//line common/common.w:280
	}
//line common/common.w:281
	used := map[string]bool{}
//line common/common.w:282
	var warns []string

//line common/common.w:284
	for _, s := range w.Sections {
//line common/common.w:285

//line common/common.w:305
		scan := func(raw string) {
//line common/common.w:306
			for _, a := range ScanCode(raw) {
//line common/common.w:307
				if a.Kind != ARef {
//line common/common.w:308
					continue
//line common/common.w:309
				}
//line common/common.w:310
				canon := w.Resolve(a.Text)
//line common/common.w:311
				if strings.HasSuffix(a.Text, "...") && canon == a.Text {
//line common/common.w:312
					prefix := strings.TrimSpace(strings.TrimSuffix(a.Text, "..."))
//line common/common.w:313
					if m := w.prefixMatches(prefix); m == 0 {
//line common/common.w:314
						warns = append(warns, fmt.Sprintf("%s: no section name matches <%s>", w.at(s.Line), a.Text))
//line common/common.w:315
					} else {
//line common/common.w:316
						warns = append(warns, fmt.Sprintf("%s: ambiguous prefix <%s> matches %d section names", w.at(s.Line), a.Text, m))
//line common/common.w:317
					}
//line common/common.w:318
					continue
//line common/common.w:319
				}
//line common/common.w:320
				if !defined[canon] {
//line common/common.w:321
					warns = append(warns, fmt.Sprintf("%s: reference to undefined section <%s>", w.at(s.Line), a.Text))
//line common/common.w:322
				}
//line common/common.w:323
				used[canon] = true
//line common/common.w:324
			}
//line common/common.w:325
		}

//line common/common.w:286
		scan(s.Code)
//line common/common.w:287
		scan(s.Tex)
//line common/common.w:288
	}

//line common/common.w:290
	warned := map[string]bool{}
//line common/common.w:291
	for _, s := range w.Sections {
//line common/common.w:292
		if s.Name == "" || s.IsFile {
//line common/common.w:293
			continue
//line common/common.w:294
		}
//line common/common.w:295
		canon := w.Resolve(s.Name)
//line common/common.w:296
		if !used[canon] && !warned[canon] {
//line common/common.w:297
			warned[canon] = true
//line common/common.w:298
			warns = append(warns, fmt.Sprintf("%s: section <%s> is defined but never used", w.at(s.Line), s.Name))
//line common/common.w:299
		}
//line common/common.w:300
	}
//line common/common.w:301
	return warns
//line common/common.w:302
}

//line common/common.w:359
func (w *Web) Resolve(name string) string {
//line common/common.w:360
	name = canonName(name)
//line common/common.w:361
	if !strings.HasSuffix(name, "...") {
//line common/common.w:362
		return name
//line common/common.w:363
	}
//line common/common.w:364
	prefix := strings.TrimSpace(strings.TrimSuffix(name, "..."))
//line common/common.w:365
	var match string
//line common/common.w:366
	count := 0
//line common/common.w:367
	for _, full := range w.full {
//line common/common.w:368
		if strings.HasPrefix(full, prefix) {
//line common/common.w:369
			match = full
//line common/common.w:370
			count++
//line common/common.w:371
		}
//line common/common.w:372
	}
//line common/common.w:373
	if count == 1 {
//line common/common.w:374
		return match
//line common/common.w:375
	}
//line common/common.w:376
	return name // unresolved or ambiguous; leave as-is for caller to report
//line common/common.w:377
}

//line common/common.w:332
func lineAt(src string, off int) int {
//line common/common.w:333
	if off > len(src) {
//line common/common.w:334
		off = len(src)
//line common/common.w:335
	}
//line common/common.w:336
	return 1 + strings.Count(src[:off], "\n")
//line common/common.w:337
}

//line common/common.w:339
func canonName(name string) string {
//line common/common.w:340
	return strings.Join(strings.Fields(name), " ")
//line common/common.w:341
}

//line common/common.w:343
func indexFrom(s, sub string, from int) int {
//line common/common.w:344
	if from >= len(s) {
//line common/common.w:345
		return -1
//line common/common.w:346
	}
//line common/common.w:347
	idx := strings.Index(s[from:], sub)
//line common/common.w:348
	if idx < 0 {
//line common/common.w:349
		return -1
//line common/common.w:350
	}
//line common/common.w:351
	return from + idx
//line common/common.w:352
}

//line common/common.w:394
type ctrlKind int

//line common/common.w:396
const (
//line common/common.w:397
	cEOF ctrlKind = iota
//line common/common.w:398
	cSection
//line common/common.w:399
	cCode // \.{@c} (or its synonym \.{@p})
//line common/common.w:400
	cNamed // \.{@<name@>=} or \.{@(file@>=}
//line common/common.w:401
	cDefn // \.{@d}
//line common/common.w:402
	cFormat

//line common/common.w:403
)

//line common/common.w:405
type ctrl struct {
//line common/common.w:406
	kind ctrlKind
//line common/common.w:407
	pos int // index of the leading `\.{@}'
//line common/common.w:408
	end int // index just past the control token
//line common/common.w:409
	depth int // for cSection: -1 unstarred (or \.{@**} top level), else starred depth
//line common/common.w:410
	starred bool // for cSection (distinguishes \.{@**} from an unstarred section)
//line common/common.w:411
	name string // for cNamed
//line common/common.w:412
	isFile bool // for cNamed (\.{@(} vs \.{@<})
//line common/common.w:413
	noIndex bool // for cFormat (\.{@s})
//line common/common.w:414
}

//line common/common.w:421
func scanStruct(src string, i int) ctrl {
//line common/common.w:422
	n := len(src)
//line common/common.w:423
	for i < n {
//line common/common.w:424
		if src[i] != '@' {
//line common/common.w:425
			i++
//line common/common.w:426
			continue
//line common/common.w:427
		}
//line common/common.w:428
		if i+1 >= n {
//line common/common.w:429
			break
//line common/common.w:430
		}
//line common/common.w:431
		switch c := src[i+1]; {
//line common/common.w:432
		case c == '@':
//line common/common.w:433
			i += 2
//line common/common.w:434
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line common/common.w:435
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line common/common.w:436
		case c == '*':
//line common/common.w:437

//line common/common.w:512
			j := i + 2
//line common/common.w:513
			depth := 0
//line common/common.w:514
			if j < n && src[j] == '*' {
//line common/common.w:515
				j++
//line common/common.w:516
				depth = -1
//line common/common.w:517
			} else {
//line common/common.w:518
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:519
					depth = depth*10 + int(src[j]-'0')
//line common/common.w:520
					j++
//line common/common.w:521
				}
//line common/common.w:522
			}
//line common/common.w:523
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}

//line common/common.w:438
		case c == 'c' || c == 'p':
//line common/common.w:439
			return ctrl{kind: cCode, pos: i, end: i + 2}
//line common/common.w:440
		case c == 'd':
//line common/common.w:441
			return ctrl{kind: cDefn, pos: i, end: i + 2}
//line common/common.w:442
		case c == 'f':
//line common/common.w:443
			return ctrl{kind: cFormat, pos: i, end: i + 2}
//line common/common.w:444
		case c == 's':
//line common/common.w:445
			return ctrl{kind: cFormat, pos: i, end: i + 2, noIndex: true}
//line common/common.w:446
		case c == '<' || c == '(':
//line common/common.w:447

//line common/common.w:529
			end := indexFrom(src, "@>", i+2)
//line common/common.w:530
			if end < 0 {
//line common/common.w:531
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:532
			}
//line common/common.w:533
			after := end + 2
//line common/common.w:534
			k := after
//line common/common.w:535
			for k < n && (src[k] == ' ' || src[k] == '\t') {
//line common/common.w:536
				k++
//line common/common.w:537
			}
//line common/common.w:538
			if k < n && src[k] == '=' {
//line common/common.w:539
				return ctrl{kind: cNamed, pos: i, end: k + 1,
//line common/common.w:540
					name: canonName(src[i+2 : end]), isFile: c == '('}
//line common/common.w:541
			}
//line common/common.w:542
			i = after // a reference, not a definition

//line common/common.w:448
		case c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line common/common.w:449
			end := indexFrom(src, "@>", i+2)
//line common/common.w:450
			if end < 0 {
//line common/common.w:451
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:452
			}
//line common/common.w:453
			i = end + 2
//line common/common.w:454
		case c == '%':
//line common/common.w:455
			j := i + 2
//line common/common.w:456
			for j < n && src[j] != '\n' {
//line common/common.w:457
				j++
//line common/common.w:458
			}
//line common/common.w:459
			i = j
//line common/common.w:460
		default:
//line common/common.w:461
			i += 2
//line common/common.w:462
		}
//line common/common.w:463
	}
//line common/common.w:464
	return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:465
}

//line common/common.w:471
func findNextSection(src string, i int) ctrl {
//line common/common.w:472
	n := len(src)
//line common/common.w:473
	for i < n {
//line common/common.w:474
		if src[i] != '@' {
//line common/common.w:475
			i++
//line common/common.w:476
			continue
//line common/common.w:477
		}
//line common/common.w:478
		if i+1 >= n {
//line common/common.w:479
			break
//line common/common.w:480
		}
//line common/common.w:481
		switch c := src[i+1]; {
//line common/common.w:482
		case c == '@':
//line common/common.w:483
			i += 2
//line common/common.w:484
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line common/common.w:485
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line common/common.w:486
		case c == '*':
//line common/common.w:487

//line common/common.w:512
			j := i + 2
//line common/common.w:513
			depth := 0
//line common/common.w:514
			if j < n && src[j] == '*' {
//line common/common.w:515
				j++
//line common/common.w:516
				depth = -1
//line common/common.w:517
			} else {
//line common/common.w:518
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:519
					depth = depth*10 + int(src[j]-'0')
//line common/common.w:520
					j++
//line common/common.w:521
				}
//line common/common.w:522
			}
//line common/common.w:523
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}

//line common/common.w:488
		case c == '<' || c == '(' || c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line common/common.w:489
			end := indexFrom(src, "@>", i+2)
//line common/common.w:490
			if end < 0 {
//line common/common.w:491
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:492
			}
//line common/common.w:493
			i = end + 2
//line common/common.w:494
		case c == '%':
//line common/common.w:495
			j := i + 2
//line common/common.w:496
			for j < n && src[j] != '\n' {
//line common/common.w:497
				j++
//line common/common.w:498
			}
//line common/common.w:499
			i = j
//line common/common.w:500
		default:
//line common/common.w:501
			i += 2
//line common/common.w:502
		}
//line common/common.w:503
	}
//line common/common.w:504
	return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:505
}

//line common/common.w:550
func parse(src string) *Web {
//line common/common.w:551
	w := &Web{}
//line common/common.w:552
	n := len(src)

//line common/common.w:554
	first := findNextSection(src, 0)
//line common/common.w:555
	w.Limbo, w.Formats = extractLimboFormats(src[:first.pos])
//line common/common.w:556
	i := first.pos

//line common/common.w:558
	num := 0
//line common/common.w:559
	for i < n {
//line common/common.w:560
		// We are positioned at a section break.
//line common/common.w:561
		hdr := src[i+1]
//line common/common.w:562
		num++
//line common/common.w:563
		sec := &Section{Number: num, Line: lineAt(src, i)}
//line common/common.w:564
		if hdr == '*' {
//line common/common.w:565
			h := findSectionHeaderEnd(src, i)
//line common/common.w:566
			sec.Starred = true
//line common/common.w:567
			sec.Depth = h.depth
//line common/common.w:568
			i = h.end
//line common/common.w:569
		} else {
//line common/common.w:570
			i += 2
//line common/common.w:571
		}

//line common/common.w:573
		// \TEX/ part: from here to the next structural control.
//line common/common.w:574
		ct := scanStruct(src, i)
//line common/common.w:575
		sec.Tex = src[i:ct.pos]
//line common/common.w:576
		if sec.Starred {
//line common/common.w:577
			sec.Title = extractTitle(sec.Tex)
//line common/common.w:578
		}

//line common/common.w:580

//line common/common.w:598
		for ct.kind == cDefn || ct.kind == cFormat {
//line common/common.w:599
			nx := scanStruct(src, ct.end)
//line common/common.w:600
			seg := src[ct.end:nx.pos]
//line common/common.w:601
			if ct.kind == cDefn {
//line common/common.w:602
				sec.Formats = append(sec.Formats, parseMacro(seg)...)
//line common/common.w:603
			} else if f, ok := parseFormat(seg, ct.noIndex); ok {
//line common/common.w:604
				sec.Formats = append(sec.Formats, f)
//line common/common.w:605
			}
//line common/common.w:606
			ct = nx
//line common/common.w:607
		}

//line common/common.w:581

//line common/common.w:613
		switch ct.kind {
//line common/common.w:614
		case cCode:
//line common/common.w:615
			sec.HasCode = true
//line common/common.w:616
			sec.CodeLine = lineAt(src, ct.end)
//line common/common.w:617
			nx := findNextSection(src, ct.end)
//line common/common.w:618
			sec.Code = src[ct.end:nx.pos]
//line common/common.w:619
			i = nx.pos
//line common/common.w:620
		case cNamed:
//line common/common.w:621
			sec.HasCode = true
//line common/common.w:622
			sec.Name = ct.name
//line common/common.w:623
			sec.IsFile = ct.isFile
//line common/common.w:624
			sec.CodeLine = lineAt(src, ct.end)
//line common/common.w:625
			nx := findNextSection(src, ct.end)
//line common/common.w:626
			sec.Code = src[ct.end:nx.pos]
//line common/common.w:627
			i = nx.pos
//line common/common.w:628
		default: // cSection or cEOF: a documentation-only section
//line common/common.w:629
			i = ct.pos
//line common/common.w:630
		}

//line common/common.w:583
		w.Sections = append(w.Sections, sec)
//line common/common.w:584
		if ct.kind == cEOF && sec.Code == "" {
//line common/common.w:585
			break
//line common/common.w:586
		}
//line common/common.w:587
		if i >= n {
//line common/common.w:588
			break
//line common/common.w:589
		}
//line common/common.w:590
	}
//line common/common.w:591
	return w
//line common/common.w:592
}

//line common/common.w:634
func findSectionHeaderEnd(src string, i int) ctrl {
//line common/common.w:635
	n := len(src)
//line common/common.w:636
	j := i + 2
//line common/common.w:637
	depth := 0
//line common/common.w:638
	if j < n && src[j] == '*' {
//line common/common.w:639
		j++
//line common/common.w:640
		depth = -1 // ``\.{@**}'' is the top level: bold in the contents, as \.{CWEB}
//line common/common.w:641
	} else {
//line common/common.w:642
		for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:643
			depth = depth*10 + int(src[j]-'0')
//line common/common.w:644
			j++
//line common/common.w:645
		}
//line common/common.w:646
	}
//line common/common.w:647
	return ctrl{end: j, depth: depth}
//line common/common.w:648
}

//line common/common.w:653
func extractTitle(tex string) string {
//line common/common.w:654
	t := strings.TrimLeft(tex, " \t\n")
//line common/common.w:655
	if i := titleEnd(t); i >= 0 {
//line common/common.w:656
		t = t[:i]
//line common/common.w:657
	}
//line common/common.w:658
	return strings.Join(strings.Fields(t), " ")
//line common/common.w:659
}

//line common/common.w:661
func titleEnd(s string) int {
//line common/common.w:662
	for i := 0; i < len(s); i++ {
//line common/common.w:663
		if s[i] == '.' && (i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t' ||
//line common/common.w:664
			s[i+1] == '\n' || s[i+1] == '\r') {
//line common/common.w:665
			return i
//line common/common.w:666
		}
//line common/common.w:667
	}
//line common/common.w:668
	return -1
//line common/common.w:669
}

//line common/common.w:674
func (w *Web) scanDiagnostics(src string) []string {
//line common/common.w:675
	var warns []string
//line common/common.w:676
	n := len(src)
//line common/common.w:677
	i := 0
//line common/common.w:678
	for i < n {
//line common/common.w:679
		if src[i] != '@' || i+1 >= n {
//line common/common.w:680
			i++
//line common/common.w:681
			continue
//line common/common.w:682
		}
//line common/common.w:683
		switch c := src[i+1]; c {
//line common/common.w:684
		case '@':
//line common/common.w:685
			i += 2
//line common/common.w:686
		case '<', '(', '=', 't', '^', '.', ':', 'q':
//line common/common.w:687
			if end := indexFrom(src, "@>", i+2); end < 0 {
//line common/common.w:688
				warns = append(warns, fmt.Sprintf("%s: unterminated `@%c ... @>'", w.at(lineAt(src, i)), c))
//line common/common.w:689
				i = n
//line common/common.w:690
			} else {
//line common/common.w:691
				i = end + 2
//line common/common.w:692
			}
//line common/common.w:693
		default:
//line common/common.w:694
			i += 2
//line common/common.w:695
		}
//line common/common.w:696
	}
//line common/common.w:697
	return warns
//line common/common.w:698
}

//line common/common.w:702
func parseFormat(seg string, noIndex bool) (Format, bool) {
//line common/common.w:703
	fields := strings.Fields(seg)
//line common/common.w:704
	if len(fields) < 2 {
//line common/common.w:705
		return Format{}, false
//line common/common.w:706
	}
//line common/common.w:707
	return Format{Original: fields[0], Like: fields[1], NoIndex: noIndex}, true
//line common/common.w:708
}

//line common/common.w:718
func parseMacro(seg string) []Format {
//line common/common.w:719
	var fs []Format
//line common/common.w:720
	for _, field := range strings.Fields(seg) {
//line common/common.w:721
		name := field
//line common/common.w:722
		if k := strings.LastIndex(name, "."); k >= 0 {
//line common/common.w:723
			name = name[k+1:]
//line common/common.w:724
		}
//line common/common.w:725
		if name != "" {
//line common/common.w:726
			fs = append(fs, Format{Original: name, Macro: true})
//line common/common.w:727
		}
//line common/common.w:728
	}
//line common/common.w:729
	return fs
//line common/common.w:730
}

//line common/common.w:737
func extractLimboFormats(src string) (string, []Format) {
//line common/common.w:738
	var b strings.Builder
//line common/common.w:739
	var formats []Format
//line common/common.w:740
	n := len(src)
//line common/common.w:741
	i := 0
//line common/common.w:742
	for i < n {
//line common/common.w:743
		if src[i] != '@' || i+1 >= n {
//line common/common.w:744
			b.WriteByte(src[i])
//line common/common.w:745
			i++
//line common/common.w:746
			continue
//line common/common.w:747
		}
//line common/common.w:748
		switch c := src[i+1]; c {
//line common/common.w:749
		case '@':
//line common/common.w:750
			b.WriteString("@@")
//line common/common.w:751
			i += 2
//line common/common.w:752
		case 'd', 'f', 's':
//line common/common.w:753

//line common/common.w:786
			var fs []Format
//line common/common.w:787
			var j int
//line common/common.w:788
			if c == 'd' {
//line common/common.w:789
				j = i + 2
//line common/common.w:790
				for j < n && src[j] != '@' {
//line common/common.w:791
					j++ // the body runs to the next control code
//line common/common.w:792
				}
//line common/common.w:793
				fs = parseMacro(src[i+2 : j])
//line common/common.w:794
			} else {
//line common/common.w:795
				j = endOfFormatArgs(src, i+2, n)
//line common/common.w:796
				if f, ok := parseFormat(src[i+2:j], c == 's'); ok {
//line common/common.w:797
					fs = []Format{f}
//line common/common.w:798
				}
//line common/common.w:799
			}
//line common/common.w:800
			formats = append(formats, fs...)
//line common/common.w:801
			if k := skipBlanks(src, j, n); k < n && src[k] == '\n' {
//line common/common.w:802
				j = k + 1 // the directive ended its line; drop the blanks and the newline
//line common/common.w:803
			}
//line common/common.w:804
			i = j

//line common/common.w:754
		case 'q':
//line common/common.w:755
			if end := indexFrom(src, "@>", i+2); end < 0 {
//line common/common.w:756
				i = n // unterminated: drop the rest of limbo
//line common/common.w:757
			} else {
//line common/common.w:758
				i = end + 2 // drop the source-only comment
//line common/common.w:759
			}
//line common/common.w:760
		case '<', '(', '=', 't', '^', '.', ':':
//line common/common.w:761
			end := indexFrom(src, "@>", i+2)
//line common/common.w:762
			if end < 0 {
//line common/common.w:763
				b.WriteString(src[i:])
//line common/common.w:764
				i = n
//line common/common.w:765
			} else {
//line common/common.w:766
				b.WriteString(src[i : end+2])
//line common/common.w:767
				i = end + 2
//line common/common.w:768
			}
//line common/common.w:769
		default:
//line common/common.w:770
			b.WriteString(src[i : i+2])
//line common/common.w:771
			i += 2
//line common/common.w:772
		}
//line common/common.w:773
	}
//line common/common.w:774
	return b.String(), formats
//line common/common.w:775
}

//line common/common.w:810
func endOfFormatArgs(src string, p, n int) int {
//line common/common.w:811
	for word := 0; word < 2; word++ {
//line common/common.w:812
		p = skipBlanks(src, p, n)
//line common/common.w:813
		for p < n && src[p] != ' ' && src[p] != '\t' && src[p] != '\n' {
//line common/common.w:814
			p++
//line common/common.w:815
		}
//line common/common.w:816
	}
//line common/common.w:817
	return p
//line common/common.w:818
}

//line common/common.w:820
func skipBlanks(src string, p, n int) int {
//line common/common.w:821
	for p < n && (src[p] == ' ' || src[p] == '\t') {
//line common/common.w:822
		p++
//line common/common.w:823
	}
//line common/common.w:824
	return p
//line common/common.w:825
}

//line common/common.w:836
type AtomKind int

//line common/common.w:838
const (
//line common/common.w:839
	AText AtomKind = iota // ordinary \GO/ source text
//line common/common.w:840
	ARef // \.{@<name@>} reference to a named section
//line common/common.w:841
	AVerbatim // \.{@=text@>} passed verbatim to tangled output
//line common/common.w:842
	ATeX // \.{@t text@>} \TEX/ text for the woven output
//line common/common.w:843
	AIndex // \.{@\^/@./@}: index entry
//line common/common.w:844
	APaste // \.{@\&} join (delete surrounding whitespace)
//line common/common.w:845
	ALayout // \.{@}, \.{@/} \.{@|} \.{@\#} woven-output layout hints
//line common/common.w:846
	AIndexDef // \.{@!} force the next identifier to index as a definition
//line common/common.w:847
)

//line common/common.w:849
type Atom struct {
//line common/common.w:850
	Kind AtomKind
//line common/common.w:851
	Text string // payload for |AText|/|AVerbatim|/|ATeX|/|AIndex|; name for |ARef|
//line common/common.w:852
	Index byte // '\.{\^}','\.{.}','\.{:}' for AIndex; '\.{,}' '\.{/}' '\.{|}' '\.{\#}' for |ALayout|
//line common/common.w:853
}

//line common/common.w:859
func ScanCode(code string) []Atom {
//line common/common.w:860
	var atoms []Atom
//line common/common.w:861
	var buf strings.Builder
//line common/common.w:862
	flush := func() {
//line common/common.w:863
		if buf.Len() > 0 {
//line common/common.w:864
			atoms = append(atoms, Atom{Kind: AText, Text: buf.String()})
//line common/common.w:865
			buf.Reset()
//line common/common.w:866
		}
//line common/common.w:867
	}
//line common/common.w:868
	n := len(code)
//line common/common.w:869
	i := 0
//line common/common.w:870
	for i < n {
//line common/common.w:871
		c := code[i]
//line common/common.w:872
		if c != '@' || i+1 >= n {
//line common/common.w:873
			buf.WriteByte(c)
//line common/common.w:874
			i++
//line common/common.w:875
			continue
//line common/common.w:876
		}
//line common/common.w:877

//line common/common.w:897
		switch d := code[i+1]; d {
//line common/common.w:898
		case '@':
//line common/common.w:899
			buf.WriteByte('@')
//line common/common.w:900
			i += 2
//line common/common.w:901
		case '&':
//line common/common.w:902
			flush()
//line common/common.w:903
			atoms = append(atoms, Atom{Kind: APaste})
//line common/common.w:904
			i += 2
//line common/common.w:905
		case '<':
//line common/common.w:906
			end := indexFrom(code, "@>", i+2)
//line common/common.w:907
			if end < 0 {
//line common/common.w:908
				buf.WriteString(code[i:])
//line common/common.w:909
				i = n
//line common/common.w:910
				continue
//line common/common.w:911
			}
//line common/common.w:912
			flush()
//line common/common.w:913
			atoms = append(atoms, Atom{Kind: ARef, Text: canonName(code[i+2 : end])})
//line common/common.w:914
			i = end + 2
//line common/common.w:915
		case '=':
//line common/common.w:916
			end := indexFrom(code, "@>", i+2)
//line common/common.w:917
			if end < 0 {
//line common/common.w:918
				i = n
//line common/common.w:919
				continue
//line common/common.w:920
			}
//line common/common.w:921
			flush()
//line common/common.w:922
			atoms = append(atoms, Atom{Kind: AVerbatim, Text: code[i+2 : end]})
//line common/common.w:923
			i = end + 2
//line common/common.w:924
		case 't':
//line common/common.w:925
			end := indexFrom(code, "@>", i+2)
//line common/common.w:926
			if end < 0 {
//line common/common.w:927
				i = n
//line common/common.w:928
				continue
//line common/common.w:929
			}
//line common/common.w:930
			flush()
//line common/common.w:931
			atoms = append(atoms, Atom{Kind: ATeX, Text: code[i+2 : end]})
//line common/common.w:932
			i = end + 2
//line common/common.w:933
		case '^', '.', ':':
//line common/common.w:934
			end := indexFrom(code, "@>", i+2)
//line common/common.w:935
			if end < 0 {
//line common/common.w:936
				i = n
//line common/common.w:937
				continue
//line common/common.w:938
			}
//line common/common.w:939
			flush()
//line common/common.w:940
			atoms = append(atoms, Atom{Kind: AIndex, Text: code[i+2 : end], Index: d})
//line common/common.w:941
			i = end + 2
//line common/common.w:942
		case 'q':
//line common/common.w:943
			end := indexFrom(code, "@>", i+2)
//line common/common.w:944
			if end < 0 {
//line common/common.w:945
				i = n
//line common/common.w:946
				continue
//line common/common.w:947
			}
//line common/common.w:948
			i = end + 2 // ignored material
//line common/common.w:949
		case '%':
//line common/common.w:950
			j := i + 2
//line common/common.w:951
			for j < n && code[j] != '\n' {
//line common/common.w:952
				j++
//line common/common.w:953
			}
//line common/common.w:954
			i = j
//line common/common.w:955
		case '>':
//line common/common.w:956
			i += 2 // stray terminator
//line common/common.w:957
		case ',', '/', '|', '#':
//line common/common.w:958
			flush()
//line common/common.w:959
			atoms = append(atoms, Atom{Kind: ALayout, Index: d})
//line common/common.w:960
			i += 2
//line common/common.w:961
		case '!':
//line common/common.w:962
			flush()
//line common/common.w:963
			atoms = append(atoms, Atom{Kind: AIndexDef})
//line common/common.w:964
			i += 2
//line common/common.w:965
		case '+', '[', ']', ';':
//line common/common.w:966
			i += 2 // \.{CWEB} prettyprinter hints, dropped
//line common/common.w:967
		default:
//line common/common.w:968
			i += 2 // unknown \.{@x}: drop it rather than corrupt the output
//line common/common.w:969
		}

//line common/common.w:878
	}
//line common/common.w:879
	flush()
//line common/common.w:880
	return atoms
//line common/common.w:881
}

//line common/common.w:991
type change struct {
//line common/common.w:992
	match []string // lines to find in the master source
//line common/common.w:993
	repl []string // lines to substitute for them
//line common/common.w:994
	line int // 1-based line of the \.{@x} in the change file (for diagnostics)
//line common/common.w:995
	replLine int // 1-based change-file line of the first replacement line
//line common/common.w:996
}

//line common/common.w:998
type srcLoc struct {
//line common/common.w:999
	file string
//line common/common.w:1000
	line int
//line common/common.w:1001
}

//line common/common.w:1003
func (l srcLoc) String() string {
//line common/common.w:1004
	if l.file == "" {
//line common/common.w:1005
		return fmt.Sprintf("line %d", l.line)
//line common/common.w:1006
	}
//line common/common.w:1007
	return fmt.Sprintf("%s:%d", l.file, l.line)
//line common/common.w:1008
}

//line common/common.w:1013
func isChangeCtrl(line string, c byte) bool {
//line common/common.w:1014
	return len(line) >= 2 && line[0] == '@' && line[1] == c
//line common/common.w:1015
}

//line common/common.w:1017
func splitLines(s string) []string {
//line common/common.w:1018
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
//line common/common.w:1019
}

//line common/common.w:1021
func sameLine(a, b string) bool {
//line common/common.w:1022
	return strings.TrimRight(a, " \t") == strings.TrimRight(b, " \t")
//line common/common.w:1023
}

//line common/common.w:1028
func parseChangeFile(src string) ([]change, error) {
//line common/common.w:1029
	lines := splitLines(src)
//line common/common.w:1030
	var changes []change
//line common/common.w:1031
	n := len(lines)
//line common/common.w:1032
	for i := 0; i < n; {
//line common/common.w:1033
		if !isChangeCtrl(lines[i], 'x') {
//line common/common.w:1034
			i++ // commentary between changes
//line common/common.w:1035
			continue
//line common/common.w:1036
		}
//line common/common.w:1037
		c := change{line: i + 1}
//line common/common.w:1038
		i++
//line common/common.w:1039

//line common/common.w:1053
		for i < n && !isChangeCtrl(lines[i], 'y') {
//line common/common.w:1054
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'z') {
//line common/common.w:1055
				return nil, fmt.Errorf("change file line %d: expected @y to close the @x match part", c.line)
//line common/common.w:1056
			}
//line common/common.w:1057
			c.match = append(c.match, lines[i])
//line common/common.w:1058
			i++
//line common/common.w:1059
		}
//line common/common.w:1060
		if i >= n {
//line common/common.w:1061
			return nil, fmt.Errorf("change file line %d: @x without a matching @y", c.line)
//line common/common.w:1062
		}
//line common/common.w:1063
		i++ // skip \.{@y}
//line common/common.w:1064
		c.replLine = i + 1

//line common/common.w:1040

//line common/common.w:1069
		for i < n && !isChangeCtrl(lines[i], 'z') {
//line common/common.w:1070
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'y') {
//line common/common.w:1071
				return nil, fmt.Errorf("change file line %d: expected @z to close the change", c.line)
//line common/common.w:1072
			}
//line common/common.w:1073
			c.repl = append(c.repl, lines[i])
//line common/common.w:1074
			i++
//line common/common.w:1075
		}
//line common/common.w:1076
		if i >= n {
//line common/common.w:1077
			return nil, fmt.Errorf("change file line %d: change has no @z", c.line)
//line common/common.w:1078
		}
//line common/common.w:1079
		i++ // skip \.{@z}

//line common/common.w:1041
		if len(c.match) == 0 {
//line common/common.w:1042
			return nil, fmt.Errorf("change file line %d: the @x match part is empty", c.line)
//line common/common.w:1043
		}
//line common/common.w:1044
		changes = append(changes, c)
//line common/common.w:1045
	}
//line common/common.w:1046
	return changes, nil
//line common/common.w:1047
}

//line common/common.w:1083
func applyChanges(src string, changes []change, chFile string) (string, error) {
//line common/common.w:1084
	out, _, err := applyChangesMapped(splitLines(src), nil, changes, chFile)
//line common/common.w:1085
	if err != nil {
//line common/common.w:1086
		return "", err
//line common/common.w:1087
	}
//line common/common.w:1088
	return strings.Join(out, "\n"), nil
//line common/common.w:1089
}

//line common/common.w:1096
func applyChangesMapped(master []string, locs []srcLoc, changes []change, chFile string) ([]string, []srcLoc, error) {
//line common/common.w:1097
	loc := func(i int) srcLoc {
//line common/common.w:1098
		if locs != nil && i < len(locs) {
//line common/common.w:1099
			return locs[i]
//line common/common.w:1100
		}
//line common/common.w:1101
		return srcLoc{line: i + 1}
//line common/common.w:1102
	}
//line common/common.w:1103
	out := make([]string, 0, len(master))
//line common/common.w:1104
	var outLocs []srcLoc
//line common/common.w:1105
	ci := 0
//line common/common.w:1106
	for i := 0; i < len(master); {
//line common/common.w:1107
		if ci < len(changes) && sameLine(master[i], changes[ci].match[0]) {
//line common/common.w:1108
			if !blockMatches(master, i, changes[ci].match) {
//line common/common.w:1109
				return nil, nil, fmt.Errorf("%s:%d: change did not match the master source at %s",
//line common/common.w:1110
					chFile, changes[ci].line, loc(i))
//line common/common.w:1111
			}
//line common/common.w:1112
			for r, rl := range changes[ci].repl {
//line common/common.w:1113
				out = append(out, rl)
//line common/common.w:1114
				outLocs = append(outLocs, srcLoc{chFile, changes[ci].replLine + r})
//line common/common.w:1115
			}
//line common/common.w:1116
			i += len(changes[ci].match)
//line common/common.w:1117
			ci++
//line common/common.w:1118
			continue
//line common/common.w:1119
		}
//line common/common.w:1120
		out = append(out, master[i])
//line common/common.w:1121
		outLocs = append(outLocs, loc(i))
//line common/common.w:1122
		i++
//line common/common.w:1123
	}
//line common/common.w:1124
	if ci < len(changes) {
//line common/common.w:1125
		return nil, nil, fmt.Errorf("%s:%d: change was never matched (looking for %q)",
//line common/common.w:1126
			chFile, changes[ci].line, changes[ci].match[0])
//line common/common.w:1127
	}
//line common/common.w:1128
	return out, outLocs, nil
//line common/common.w:1129
}

//line common/common.w:1134
func blockMatches(master []string, at int, match []string) bool {
//line common/common.w:1135
	if at+len(match) > len(master) {
//line common/common.w:1136
		return false
//line common/common.w:1137
	}
//line common/common.w:1138
	for k, m := range match {
//line common/common.w:1139
		if !sameLine(master[at+k], m) {
//line common/common.w:1140
			return false
//line common/common.w:1141
		}
//line common/common.w:1142
	}
//line common/common.w:1143
	return true
//line common/common.w:1144
}
