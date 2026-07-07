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
const Version = "0.4.5"

//line common/common.w:37
type Format struct {
//line common/common.w:38
	Original string
//line common/common.w:39
	Like string
//line common/common.w:40
	NoIndex bool
//line common/common.w:41
	Macro bool // \.{@d}: typeset Original in typewriter (a \.{CWEB}-style macro)
//line common/common.w:42
}

//line common/common.w:48
type Section struct {
//line common/common.w:49
	Number int // 1-based section number
//line common/common.w:50
	Line int // 1-based source line where the section begins
//line common/common.w:51
	Starred bool // true for \.{@*} sections
//line common/common.w:52
	Depth int // group depth for starred sections (-1 |==| \.{@**}, 0 |==| \.{@*}, n |==| \.{@*n})
//line common/common.w:53
	Title string // starred-section title (text up to the first period)
//line common/common.w:54
	Tex string // commentary, raw \TEX/ with in-text\.{@}-codes still embedded
//line common/common.w:55
	Formats []Format
//line common/common.w:56
	HasCode bool // true if the section contributes code
//line common/common.w:57
	Name string // named-section name, or \.{""} for an unnamed @c section
//line common/common.w:58
	IsFile bool // true if the name is an output file (\.{@(file@>=})
//line common/common.w:59
	Code string // raw code text with in-code \.{@}-codes still embedded
//line common/common.w:60
	CodeLine int // 1-based combined-source line where Code begins (0 if none)
//line common/common.w:61
}

//line common/common.w:68
type Web struct {
//line common/common.w:69
	Limbo string
//line common/common.w:70
	Formats []Format // \.{@f}/\.{@s} directives found in limbo (apply globally)
//line common/common.w:71
	Sections []*Section
//line common/common.w:72
	Warnings []string // non-fatal diagnostics gathered while parsing/checking
//line common/common.w:73
	file string // source filename, for diagnostics (\.{""} if unknown)
//line common/common.w:74
	locs []srcLoc // origin (file, line) of each combined-source line
//line common/common.w:75
	full []string // canonical (non-abbreviated) section names
//line common/common.w:76
}

//line common/common.w:83
func Parse(filename string) (*Web, error) {
//line common/common.w:84
	return ParseWithChange(filename, "")
//line common/common.w:85
}

//line common/common.w:87
func ParseWithChange(filename, changeFile string) (*Web, error) {
//line common/common.w:88
	lines, locs, err := expandIncludes(filename, 0)
//line common/common.w:89
	if err != nil {
//line common/common.w:90
		return nil, err
//line common/common.w:91
	}
//line common/common.w:92

//line common/common.w:105
	if changeFile != "" {
//line common/common.w:106
		chData, err := os.ReadFile(changeFile)
//line common/common.w:107
		if err != nil {
//line common/common.w:108
			return nil, err
//line common/common.w:109
		}
//line common/common.w:110
		changes, err := parseChangeFile(string(chData))
//line common/common.w:111
		if err != nil {
//line common/common.w:112
			return nil, err
//line common/common.w:113
		}
//line common/common.w:114
		lines, locs, err = applyChangesMapped(lines, locs, changes, changeFile)
//line common/common.w:115
		if err != nil {
//line common/common.w:116
			return nil, err
//line common/common.w:117
		}
//line common/common.w:118
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

//line common/common.w:124
func ParseString(src string) *Web {
//line common/common.w:125
	w := parse(src)
//line common/common.w:126
	w.finish(src)
//line common/common.w:127
	return w
//line common/common.w:128
}

//line common/common.w:130
func (w *Web) finish(src string) {
//line common/common.w:131
	w.collectNames()
//line common/common.w:132
	w.Warnings = append(w.Warnings, w.scanDiagnostics(src)...)
//line common/common.w:133
	w.Warnings = append(w.Warnings, w.checkNames()...)
//line common/common.w:134
}

//line common/common.w:142
func (w *Web) Origin(line int) (file string, ln int) {
//line common/common.w:143
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line common/common.w:144
		return w.locs[i].file, w.locs[i].line
//line common/common.w:145
	}
//line common/common.w:146
	return w.file, line
//line common/common.w:147
}

//line common/common.w:149
func (w *Web) at(line int) string {
//line common/common.w:150
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line common/common.w:151
		return w.locs[i].String()
//line common/common.w:152
	}
//line common/common.w:153
	if w.file != "" {
//line common/common.w:154
		return fmt.Sprintf("%s:%d", w.file, line)
//line common/common.w:155
	}
//line common/common.w:156
	return fmt.Sprintf("line %d", line)
//line common/common.w:157
}

//line common/common.w:163
func DefaultExt(name, ext string) string {
//line common/common.w:164
	if name == "" || filepath.Ext(name) != "" {
//line common/common.w:165
		return name
//line common/common.w:166
	}
//line common/common.w:167
	return name + ext
//line common/common.w:168
}

//line common/common.w:174
func expandIncludes(file string, depth int) ([]string, []srcLoc, error) {
//line common/common.w:175
	if depth > 25 {
//line common/common.w:176
		return nil, nil, fmt.Errorf("gweb: @i include nesting too deep at %q", file)
//line common/common.w:177
	}
//line common/common.w:178
	data, err := os.ReadFile(file)
//line common/common.w:179
	if err != nil {
//line common/common.w:180
		return nil, nil, err
//line common/common.w:181
	}
//line common/common.w:182
	raw := splitLines(string(data))
//line common/common.w:183
	if n := len(raw); n > 0 && raw[n-1] == "" {
//line common/common.w:184
		raw = raw[:n-1]
//line common/common.w:185
	}

//line common/common.w:187
	var lines []string
//line common/common.w:188
	var locs []srcLoc
//line common/common.w:189
	dir := filepath.Dir(file)
//line common/common.w:190
	for i, line := range raw {
//line common/common.w:191
		if name, ok := includeDirective(line); ok {
//line common/common.w:192
			path := name
//line common/common.w:193
			if !filepath.IsAbs(path) {
//line common/common.w:194
				path = filepath.Join(dir, name)
//line common/common.w:195
			}
//line common/common.w:196
			sub, subLocs, err := expandIncludes(path, depth+1)
//line common/common.w:197
			if err != nil {
//line common/common.w:198
				return nil, nil, fmt.Errorf("%s:%d: %w", file, i+1, err)
//line common/common.w:199
			}
//line common/common.w:200
			lines = append(lines, sub...)
//line common/common.w:201
			locs = append(locs, subLocs...)
//line common/common.w:202
			continue
//line common/common.w:203
		}
//line common/common.w:204
		lines = append(lines, line)
//line common/common.w:205
		locs = append(locs, srcLoc{file, i + 1})
//line common/common.w:206
	}
//line common/common.w:207
	return lines, locs, nil
//line common/common.w:208
}

//line common/common.w:211
func includeDirective(line string) (name string, ok bool) {
//line common/common.w:212
	t := strings.TrimLeft(line, " \t")
//line common/common.w:213
	if !strings.HasPrefix(t, "@i") {
//line common/common.w:214
		return "", false
//line common/common.w:215
	}
//line common/common.w:216
	rest := t[2:]
//line common/common.w:217
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
//line common/common.w:218
		return "", false
//line common/common.w:219
	}
//line common/common.w:220
	name = strings.Trim(strings.TrimSpace(rest), "\"")
//line common/common.w:221
	return name, name != ""
//line common/common.w:222
}

//line common/common.w:229
func (w *Web) collectNames() {
//line common/common.w:230
	seen := map[string]bool{}
//line common/common.w:231
	add := func(name string) {
//line common/common.w:232
		if name != "" && !strings.HasSuffix(name, "...") && !seen[name] {
//line common/common.w:233
			seen[name] = true
//line common/common.w:234
			w.full = append(w.full, name)
//line common/common.w:235
		}
//line common/common.w:236
	}
//line common/common.w:237
	for _, s := range w.Sections {
//line common/common.w:238
		if !s.IsFile {
//line common/common.w:239
			add(s.Name) // a definition's name
//line common/common.w:240
		}
//line common/common.w:241
		for _, raw := range []string{s.Code, s.Tex} {
//line common/common.w:242
			for _, a := range ScanCode(raw) {
//line common/common.w:243
				if a.Kind == ARef {
//line common/common.w:244
					add(a.Text) // a reference's name
//line common/common.w:245
				}
//line common/common.w:246
			}
//line common/common.w:247
		}
//line common/common.w:248
	}
//line common/common.w:249
}

//line common/common.w:254
func (w *Web) prefixMatches(prefix string) int {
//line common/common.w:255
	n := 0
//line common/common.w:256
	for _, full := range w.full {
//line common/common.w:257
		if strings.HasPrefix(full, prefix) {
//line common/common.w:258
			n++
//line common/common.w:259
		}
//line common/common.w:260
	}
//line common/common.w:261
	return n
//line common/common.w:262
}

//line common/common.w:273
func (w *Web) checkNames() []string {
//line common/common.w:274
	defined := map[string]bool{}
//line common/common.w:275
	for _, s := range w.Sections {
//line common/common.w:276
		if s.Name != "" && !s.IsFile {
//line common/common.w:277
			defined[w.Resolve(s.Name)] = true
//line common/common.w:278
		}
//line common/common.w:279
	}
//line common/common.w:280
	used := map[string]bool{}
//line common/common.w:281
	var warns []string

//line common/common.w:283
	for _, s := range w.Sections {
//line common/common.w:284

//line common/common.w:304
		scan := func(raw string) {
//line common/common.w:305
			for _, a := range ScanCode(raw) {
//line common/common.w:306
				if a.Kind != ARef {
//line common/common.w:307
					continue
//line common/common.w:308
				}
//line common/common.w:309
				canon := w.Resolve(a.Text)
//line common/common.w:310
				if strings.HasSuffix(a.Text, "...") && canon == a.Text {
//line common/common.w:311
					prefix := strings.TrimSpace(strings.TrimSuffix(a.Text, "..."))
//line common/common.w:312
					if m := w.prefixMatches(prefix); m == 0 {
//line common/common.w:313
						warns = append(warns, fmt.Sprintf("%s: no section name matches <%s>", w.at(s.Line), a.Text))
//line common/common.w:314
					} else {
//line common/common.w:315
						warns = append(warns, fmt.Sprintf("%s: ambiguous prefix <%s> matches %d section names", w.at(s.Line), a.Text, m))
//line common/common.w:316
					}
//line common/common.w:317
					continue
//line common/common.w:318
				}
//line common/common.w:319
				if !defined[canon] {
//line common/common.w:320
					warns = append(warns, fmt.Sprintf("%s: reference to undefined section <%s>", w.at(s.Line), a.Text))
//line common/common.w:321
				}
//line common/common.w:322
				used[canon] = true
//line common/common.w:323
			}
//line common/common.w:324
		}

//line common/common.w:285
		scan(s.Code)
//line common/common.w:286
		scan(s.Tex)
//line common/common.w:287
	}

//line common/common.w:289
	warned := map[string]bool{}
//line common/common.w:290
	for _, s := range w.Sections {
//line common/common.w:291
		if s.Name == "" || s.IsFile {
//line common/common.w:292
			continue
//line common/common.w:293
		}
//line common/common.w:294
		canon := w.Resolve(s.Name)
//line common/common.w:295
		if !used[canon] && !warned[canon] {
//line common/common.w:296
			warned[canon] = true
//line common/common.w:297
			warns = append(warns, fmt.Sprintf("%s: section <%s> is defined but never used", w.at(s.Line), s.Name))
//line common/common.w:298
		}
//line common/common.w:299
	}
//line common/common.w:300
	return warns
//line common/common.w:301
}

//line common/common.w:358
func (w *Web) Resolve(name string) string {
//line common/common.w:359
	name = canonName(name)
//line common/common.w:360
	if !strings.HasSuffix(name, "...") {
//line common/common.w:361
		return name
//line common/common.w:362
	}
//line common/common.w:363
	prefix := strings.TrimSpace(strings.TrimSuffix(name, "..."))
//line common/common.w:364
	var match string
//line common/common.w:365
	count := 0
//line common/common.w:366
	for _, full := range w.full {
//line common/common.w:367
		if strings.HasPrefix(full, prefix) {
//line common/common.w:368
			match = full
//line common/common.w:369
			count++
//line common/common.w:370
		}
//line common/common.w:371
	}
//line common/common.w:372
	if count == 1 {
//line common/common.w:373
		return match
//line common/common.w:374
	}
//line common/common.w:375
	return name // unresolved or ambiguous; leave as-is for caller to report
//line common/common.w:376
}

//line common/common.w:331
func lineAt(src string, off int) int {
//line common/common.w:332
	if off > len(src) {
//line common/common.w:333
		off = len(src)
//line common/common.w:334
	}
//line common/common.w:335
	return 1 + strings.Count(src[:off], "\n")
//line common/common.w:336
}

//line common/common.w:338
func canonName(name string) string {
//line common/common.w:339
	return strings.Join(strings.Fields(name), " ")
//line common/common.w:340
}

//line common/common.w:342
func indexFrom(s, sub string, from int) int {
//line common/common.w:343
	if from >= len(s) {
//line common/common.w:344
		return -1
//line common/common.w:345
	}
//line common/common.w:346
	idx := strings.Index(s[from:], sub)
//line common/common.w:347
	if idx < 0 {
//line common/common.w:348
		return -1
//line common/common.w:349
	}
//line common/common.w:350
	return from + idx
//line common/common.w:351
}

//line common/common.w:393
type ctrlKind int

//line common/common.w:395
const (
//line common/common.w:396
	cEOF ctrlKind = iota
//line common/common.w:397
	cSection
//line common/common.w:398
	cCode // \.{@c} (or its synonym \.{@p})
//line common/common.w:399
	cNamed // \.{@<name@>=} or \.{@(file@>=}
//line common/common.w:400
	cDefn // \.{@d}
//line common/common.w:401
	cFormat

//line common/common.w:402
)

//line common/common.w:404
type ctrl struct {
//line common/common.w:405
	kind ctrlKind
//line common/common.w:406
	pos int // index of the leading `\.{@}'
//line common/common.w:407
	end int // index just past the control token
//line common/common.w:408
	depth int // for cSection: -1 unstarred (or \.{@**} top level), else starred depth
//line common/common.w:409
	starred bool // for cSection (distinguishes \.{@**} from an unstarred section)
//line common/common.w:410
	name string // for cNamed
//line common/common.w:411
	isFile bool // for cNamed (\.{@(} vs \.{@<})
//line common/common.w:412
	noIndex bool // for cFormat (\.{@s})
//line common/common.w:413
}

//line common/common.w:420
func scanStruct(src string, i int) ctrl {
//line common/common.w:421
	n := len(src)
//line common/common.w:422
	for i < n {
//line common/common.w:423
		if src[i] != '@' {
//line common/common.w:424
			i++
//line common/common.w:425
			continue
//line common/common.w:426
		}
//line common/common.w:427
		if i+1 >= n {
//line common/common.w:428
			break
//line common/common.w:429
		}
//line common/common.w:430
		switch c := src[i+1]; {
//line common/common.w:431
		case c == '@':
//line common/common.w:432
			i += 2
//line common/common.w:433
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line common/common.w:434
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line common/common.w:435
		case c == '*':
//line common/common.w:436

//line common/common.w:511
			j := i + 2
//line common/common.w:512
			depth := 0
//line common/common.w:513
			if j < n && src[j] == '*' {
//line common/common.w:514
				j++
//line common/common.w:515
				depth = -1
//line common/common.w:516
			} else {
//line common/common.w:517
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:518
					depth = depth*10 + int(src[j]-'0')
//line common/common.w:519
					j++
//line common/common.w:520
				}
//line common/common.w:521
			}
//line common/common.w:522
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}

//line common/common.w:437
		case c == 'c' || c == 'p':
//line common/common.w:438
			return ctrl{kind: cCode, pos: i, end: i + 2}
//line common/common.w:439
		case c == 'd':
//line common/common.w:440
			return ctrl{kind: cDefn, pos: i, end: i + 2}
//line common/common.w:441
		case c == 'f':
//line common/common.w:442
			return ctrl{kind: cFormat, pos: i, end: i + 2}
//line common/common.w:443
		case c == 's':
//line common/common.w:444
			return ctrl{kind: cFormat, pos: i, end: i + 2, noIndex: true}
//line common/common.w:445
		case c == '<' || c == '(':
//line common/common.w:446

//line common/common.w:528
			end := indexFrom(src, "@>", i+2)
//line common/common.w:529
			if end < 0 {
//line common/common.w:530
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:531
			}
//line common/common.w:532
			after := end + 2
//line common/common.w:533
			k := after
//line common/common.w:534
			for k < n && (src[k] == ' ' || src[k] == '\t') {
//line common/common.w:535
				k++
//line common/common.w:536
			}
//line common/common.w:537
			if k < n && src[k] == '=' {
//line common/common.w:538
				return ctrl{kind: cNamed, pos: i, end: k + 1,
//line common/common.w:539
					name: canonName(src[i+2 : end]), isFile: c == '('}
//line common/common.w:540
			}
//line common/common.w:541
			i = after // a reference, not a definition

//line common/common.w:447
		case c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line common/common.w:448
			end := indexFrom(src, "@>", i+2)
//line common/common.w:449
			if end < 0 {
//line common/common.w:450
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:451
			}
//line common/common.w:452
			i = end + 2
//line common/common.w:453
		case c == '%':
//line common/common.w:454
			j := i + 2
//line common/common.w:455
			for j < n && src[j] != '\n' {
//line common/common.w:456
				j++
//line common/common.w:457
			}
//line common/common.w:458
			i = j
//line common/common.w:459
		default:
//line common/common.w:460
			i += 2
//line common/common.w:461
		}
//line common/common.w:462
	}
//line common/common.w:463
	return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:464
}

//line common/common.w:470
func findNextSection(src string, i int) ctrl {
//line common/common.w:471
	n := len(src)
//line common/common.w:472
	for i < n {
//line common/common.w:473
		if src[i] != '@' {
//line common/common.w:474
			i++
//line common/common.w:475
			continue
//line common/common.w:476
		}
//line common/common.w:477
		if i+1 >= n {
//line common/common.w:478
			break
//line common/common.w:479
		}
//line common/common.w:480
		switch c := src[i+1]; {
//line common/common.w:481
		case c == '@':
//line common/common.w:482
			i += 2
//line common/common.w:483
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line common/common.w:484
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line common/common.w:485
		case c == '*':
//line common/common.w:486

//line common/common.w:511
			j := i + 2
//line common/common.w:512
			depth := 0
//line common/common.w:513
			if j < n && src[j] == '*' {
//line common/common.w:514
				j++
//line common/common.w:515
				depth = -1
//line common/common.w:516
			} else {
//line common/common.w:517
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:518
					depth = depth*10 + int(src[j]-'0')
//line common/common.w:519
					j++
//line common/common.w:520
				}
//line common/common.w:521
			}
//line common/common.w:522
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}

//line common/common.w:487
		case c == '<' || c == '(' || c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line common/common.w:488
			end := indexFrom(src, "@>", i+2)
//line common/common.w:489
			if end < 0 {
//line common/common.w:490
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:491
			}
//line common/common.w:492
			i = end + 2
//line common/common.w:493
		case c == '%':
//line common/common.w:494
			j := i + 2
//line common/common.w:495
			for j < n && src[j] != '\n' {
//line common/common.w:496
				j++
//line common/common.w:497
			}
//line common/common.w:498
			i = j
//line common/common.w:499
		default:
//line common/common.w:500
			i += 2
//line common/common.w:501
		}
//line common/common.w:502
	}
//line common/common.w:503
	return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:504
}

//line common/common.w:549
func parse(src string) *Web {
//line common/common.w:550
	w := &Web{}
//line common/common.w:551
	n := len(src)

//line common/common.w:553
	first := findNextSection(src, 0)
//line common/common.w:554
	w.Limbo, w.Formats = extractLimboFormats(src[:first.pos])
//line common/common.w:555
	i := first.pos

//line common/common.w:557
	num := 0
//line common/common.w:558
	for i < n {
//line common/common.w:559
		// We are positioned at a section break.
//line common/common.w:560
		hdr := src[i+1]
//line common/common.w:561
		num++
//line common/common.w:562
		sec := &Section{Number: num, Line: lineAt(src, i)}
//line common/common.w:563
		if hdr == '*' {
//line common/common.w:564
			h := findSectionHeaderEnd(src, i)
//line common/common.w:565
			sec.Starred = true
//line common/common.w:566
			sec.Depth = h.depth
//line common/common.w:567
			i = h.end
//line common/common.w:568
		} else {
//line common/common.w:569
			i += 2
//line common/common.w:570
		}

//line common/common.w:572
		// \TEX/ part: from here to the next structural control.
//line common/common.w:573
		ct := scanStruct(src, i)
//line common/common.w:574
		sec.Tex = src[i:ct.pos]
//line common/common.w:575
		if sec.Starred {
//line common/common.w:576
			sec.Title = extractTitle(sec.Tex)
//line common/common.w:577
		}

//line common/common.w:579

//line common/common.w:597
		for ct.kind == cDefn || ct.kind == cFormat {
//line common/common.w:598
			nx := scanStruct(src, ct.end)
//line common/common.w:599
			seg := src[ct.end:nx.pos]
//line common/common.w:600
			if ct.kind == cDefn {
//line common/common.w:601
				if f, ok := parseMacro(seg); ok {
//line common/common.w:602
					sec.Formats = append(sec.Formats, f)
//line common/common.w:603
				}
//line common/common.w:604
			} else if f, ok := parseFormat(seg, ct.noIndex); ok {
//line common/common.w:605
				sec.Formats = append(sec.Formats, f)
//line common/common.w:606
			}
//line common/common.w:607
			ct = nx
//line common/common.w:608
		}

//line common/common.w:580

//line common/common.w:614
		switch ct.kind {
//line common/common.w:615
		case cCode:
//line common/common.w:616
			sec.HasCode = true
//line common/common.w:617
			sec.CodeLine = lineAt(src, ct.end)
//line common/common.w:618
			nx := findNextSection(src, ct.end)
//line common/common.w:619
			sec.Code = src[ct.end:nx.pos]
//line common/common.w:620
			i = nx.pos
//line common/common.w:621
		case cNamed:
//line common/common.w:622
			sec.HasCode = true
//line common/common.w:623
			sec.Name = ct.name
//line common/common.w:624
			sec.IsFile = ct.isFile
//line common/common.w:625
			sec.CodeLine = lineAt(src, ct.end)
//line common/common.w:626
			nx := findNextSection(src, ct.end)
//line common/common.w:627
			sec.Code = src[ct.end:nx.pos]
//line common/common.w:628
			i = nx.pos
//line common/common.w:629
		default: // cSection or cEOF: a documentation-only section
//line common/common.w:630
			i = ct.pos
//line common/common.w:631
		}

//line common/common.w:582
		w.Sections = append(w.Sections, sec)
//line common/common.w:583
		if ct.kind == cEOF && sec.Code == "" {
//line common/common.w:584
			break
//line common/common.w:585
		}
//line common/common.w:586
		if i >= n {
//line common/common.w:587
			break
//line common/common.w:588
		}
//line common/common.w:589
	}
//line common/common.w:590
	return w
//line common/common.w:591
}

//line common/common.w:635
func findSectionHeaderEnd(src string, i int) ctrl {
//line common/common.w:636
	n := len(src)
//line common/common.w:637
	j := i + 2
//line common/common.w:638
	depth := 0
//line common/common.w:639
	if j < n && src[j] == '*' {
//line common/common.w:640
		j++
//line common/common.w:641
		depth = -1 // ``\.{@**}'' is the top level: bold in the contents, as \.{CWEB}
//line common/common.w:642
	} else {
//line common/common.w:643
		for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:644
			depth = depth*10 + int(src[j]-'0')
//line common/common.w:645
			j++
//line common/common.w:646
		}
//line common/common.w:647
	}
//line common/common.w:648
	return ctrl{end: j, depth: depth}
//line common/common.w:649
}

//line common/common.w:654
func extractTitle(tex string) string {
//line common/common.w:655
	t := strings.TrimLeft(tex, " \t\n")
//line common/common.w:656
	if i := titleEnd(t); i >= 0 {
//line common/common.w:657
		t = t[:i]
//line common/common.w:658
	}
//line common/common.w:659
	return strings.Join(strings.Fields(t), " ")
//line common/common.w:660
}

//line common/common.w:662
func titleEnd(s string) int {
//line common/common.w:663
	for i := 0; i < len(s); i++ {
//line common/common.w:664
		if s[i] == '.' && (i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t' ||
//line common/common.w:665
			s[i+1] == '\n' || s[i+1] == '\r') {
//line common/common.w:666
			return i
//line common/common.w:667
		}
//line common/common.w:668
	}
//line common/common.w:669
	return -1
//line common/common.w:670
}

//line common/common.w:675
func (w *Web) scanDiagnostics(src string) []string {
//line common/common.w:676
	var warns []string
//line common/common.w:677
	n := len(src)
//line common/common.w:678
	i := 0
//line common/common.w:679
	for i < n {
//line common/common.w:680
		if src[i] != '@' || i+1 >= n {
//line common/common.w:681
			i++
//line common/common.w:682
			continue
//line common/common.w:683
		}
//line common/common.w:684
		switch c := src[i+1]; c {
//line common/common.w:685
		case '@':
//line common/common.w:686
			i += 2
//line common/common.w:687
		case '<', '(', '=', 't', '^', '.', ':', 'q':
//line common/common.w:688
			if end := indexFrom(src, "@>", i+2); end < 0 {
//line common/common.w:689
				warns = append(warns, fmt.Sprintf("%s: unterminated `@%c ... @>'", w.at(lineAt(src, i)), c))
//line common/common.w:690
				i = n
//line common/common.w:691
			} else {
//line common/common.w:692
				i = end + 2
//line common/common.w:693
			}
//line common/common.w:694
		default:
//line common/common.w:695
			i += 2
//line common/common.w:696
		}
//line common/common.w:697
	}
//line common/common.w:698
	return warns
//line common/common.w:699
}

//line common/common.w:703
func parseFormat(seg string, noIndex bool) (Format, bool) {
//line common/common.w:704
	fields := strings.Fields(seg)
//line common/common.w:705
	if len(fields) < 2 {
//line common/common.w:706
		return Format{}, false
//line common/common.w:707
	}
//line common/common.w:708
	return Format{Original: fields[0], Like: fields[1], NoIndex: noIndex}, true
//line common/common.w:709
}

//line common/common.w:717
func parseMacro(seg string) (Format, bool) {
//line common/common.w:718
	fields := strings.Fields(seg)
//line common/common.w:719
	if len(fields) == 0 {
//line common/common.w:720
		return Format{}, false
//line common/common.w:721
	}
//line common/common.w:722
	name := fields[0]
//line common/common.w:723
	if k := strings.LastIndex(name, "."); k >= 0 {
//line common/common.w:724
		name = name[k+1:]
//line common/common.w:725
	}
//line common/common.w:726
	if name == "" {
//line common/common.w:727
		return Format{}, false
//line common/common.w:728
	}
//line common/common.w:729
	return Format{Original: name, Macro: true}, true
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
			var f Format
//line common/common.w:787
			var ok bool
//line common/common.w:788
			var j int
//line common/common.w:789
			if c == 'd' {
//line common/common.w:790
				j = i + 2
//line common/common.w:791
				for j < n && src[j] != '\n' {
//line common/common.w:792
					j++
//line common/common.w:793
				}
//line common/common.w:794
				f, ok = parseMacro(src[i+2 : j])
//line common/common.w:795
			} else {
//line common/common.w:796
				j = endOfFormatArgs(src, i+2, n)
//line common/common.w:797
				f, ok = parseFormat(src[i+2:j], c == 's')
//line common/common.w:798
			}
//line common/common.w:799
			if ok {
//line common/common.w:800
				formats = append(formats, f)
//line common/common.w:801
			}
//line common/common.w:802
			if k := skipBlanks(src, j, n); k < n && src[k] == '\n' {
//line common/common.w:803
				j = k + 1 // the directive ended its line; drop the blanks and the newline
//line common/common.w:804
			}
//line common/common.w:805
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

//line common/common.w:811
func endOfFormatArgs(src string, p, n int) int {
//line common/common.w:812
	for word := 0; word < 2; word++ {
//line common/common.w:813
		p = skipBlanks(src, p, n)
//line common/common.w:814
		for p < n && src[p] != ' ' && src[p] != '\t' && src[p] != '\n' {
//line common/common.w:815
			p++
//line common/common.w:816
		}
//line common/common.w:817
	}
//line common/common.w:818
	return p
//line common/common.w:819
}

//line common/common.w:821
func skipBlanks(src string, p, n int) int {
//line common/common.w:822
	for p < n && (src[p] == ' ' || src[p] == '\t') {
//line common/common.w:823
		p++
//line common/common.w:824
	}
//line common/common.w:825
	return p
//line common/common.w:826
}

//line common/common.w:837
type AtomKind int

//line common/common.w:839
const (
//line common/common.w:840
	AText AtomKind = iota // ordinary \GO/ source text
//line common/common.w:841
	ARef // \.{@<name@>} reference to a named section
//line common/common.w:842
	AVerbatim // \.{@=text@>} passed verbatim to tangled output
//line common/common.w:843
	ATeX // \.{@t text@>} \TEX/ text for the woven output
//line common/common.w:844
	AIndex // \.{@\^/@./@}: index entry
//line common/common.w:845
	APaste // \.{@\&} join (delete surrounding whitespace)
//line common/common.w:846
	ALayout // \.{@}, \.{@/} \.{@|} \.{@\#} woven-output layout hints
//line common/common.w:847
	AIndexDef // \.{@!} force the next identifier to index as a definition
//line common/common.w:848
)

//line common/common.w:850
type Atom struct {
//line common/common.w:851
	Kind AtomKind
//line common/common.w:852
	Text string // payload for |AText|/|AVerbatim|/|ATeX|/|AIndex|; name for |ARef|
//line common/common.w:853
	Index byte // '\.{\^}','\.{.}','\.{:}' for AIndex; '\.{,}' '\.{/}' '\.{|}' '\.{\#}' for |ALayout|
//line common/common.w:854
}

//line common/common.w:860
func ScanCode(code string) []Atom {
//line common/common.w:861
	var atoms []Atom
//line common/common.w:862
	var buf strings.Builder
//line common/common.w:863
	flush := func() {
//line common/common.w:864
		if buf.Len() > 0 {
//line common/common.w:865
			atoms = append(atoms, Atom{Kind: AText, Text: buf.String()})
//line common/common.w:866
			buf.Reset()
//line common/common.w:867
		}
//line common/common.w:868
	}
//line common/common.w:869
	n := len(code)
//line common/common.w:870
	i := 0
//line common/common.w:871
	for i < n {
//line common/common.w:872
		c := code[i]
//line common/common.w:873
		if c != '@' || i+1 >= n {
//line common/common.w:874
			buf.WriteByte(c)
//line common/common.w:875
			i++
//line common/common.w:876
			continue
//line common/common.w:877
		}
//line common/common.w:878

//line common/common.w:898
		switch d := code[i+1]; d {
//line common/common.w:899
		case '@':
//line common/common.w:900
			buf.WriteByte('@')
//line common/common.w:901
			i += 2
//line common/common.w:902
		case '&':
//line common/common.w:903
			flush()
//line common/common.w:904
			atoms = append(atoms, Atom{Kind: APaste})
//line common/common.w:905
			i += 2
//line common/common.w:906
		case '<':
//line common/common.w:907
			end := indexFrom(code, "@>", i+2)
//line common/common.w:908
			if end < 0 {
//line common/common.w:909
				buf.WriteString(code[i:])
//line common/common.w:910
				i = n
//line common/common.w:911
				continue
//line common/common.w:912
			}
//line common/common.w:913
			flush()
//line common/common.w:914
			atoms = append(atoms, Atom{Kind: ARef, Text: canonName(code[i+2 : end])})
//line common/common.w:915
			i = end + 2
//line common/common.w:916
		case '=':
//line common/common.w:917
			end := indexFrom(code, "@>", i+2)
//line common/common.w:918
			if end < 0 {
//line common/common.w:919
				i = n
//line common/common.w:920
				continue
//line common/common.w:921
			}
//line common/common.w:922
			flush()
//line common/common.w:923
			atoms = append(atoms, Atom{Kind: AVerbatim, Text: code[i+2 : end]})
//line common/common.w:924
			i = end + 2
//line common/common.w:925
		case 't':
//line common/common.w:926
			end := indexFrom(code, "@>", i+2)
//line common/common.w:927
			if end < 0 {
//line common/common.w:928
				i = n
//line common/common.w:929
				continue
//line common/common.w:930
			}
//line common/common.w:931
			flush()
//line common/common.w:932
			atoms = append(atoms, Atom{Kind: ATeX, Text: code[i+2 : end]})
//line common/common.w:933
			i = end + 2
//line common/common.w:934
		case '^', '.', ':':
//line common/common.w:935
			end := indexFrom(code, "@>", i+2)
//line common/common.w:936
			if end < 0 {
//line common/common.w:937
				i = n
//line common/common.w:938
				continue
//line common/common.w:939
			}
//line common/common.w:940
			flush()
//line common/common.w:941
			atoms = append(atoms, Atom{Kind: AIndex, Text: code[i+2 : end], Index: d})
//line common/common.w:942
			i = end + 2
//line common/common.w:943
		case 'q':
//line common/common.w:944
			end := indexFrom(code, "@>", i+2)
//line common/common.w:945
			if end < 0 {
//line common/common.w:946
				i = n
//line common/common.w:947
				continue
//line common/common.w:948
			}
//line common/common.w:949
			i = end + 2 // ignored material
//line common/common.w:950
		case '%':
//line common/common.w:951
			j := i + 2
//line common/common.w:952
			for j < n && code[j] != '\n' {
//line common/common.w:953
				j++
//line common/common.w:954
			}
//line common/common.w:955
			i = j
//line common/common.w:956
		case '>':
//line common/common.w:957
			i += 2 // stray terminator
//line common/common.w:958
		case ',', '/', '|', '#':
//line common/common.w:959
			flush()
//line common/common.w:960
			atoms = append(atoms, Atom{Kind: ALayout, Index: d})
//line common/common.w:961
			i += 2
//line common/common.w:962
		case '!':
//line common/common.w:963
			flush()
//line common/common.w:964
			atoms = append(atoms, Atom{Kind: AIndexDef})
//line common/common.w:965
			i += 2
//line common/common.w:966
		case '+', '[', ']', ';':
//line common/common.w:967
			i += 2 // \.{CWEB} prettyprinter hints, dropped
//line common/common.w:968
		default:
//line common/common.w:969
			i += 2 // unknown \.{@x}: drop it rather than corrupt the output
//line common/common.w:970
		}

//line common/common.w:879
	}
//line common/common.w:880
	flush()
//line common/common.w:881
	return atoms
//line common/common.w:882
}

//line common/common.w:992
type change struct {
//line common/common.w:993
	match []string // lines to find in the master source
//line common/common.w:994
	repl []string // lines to substitute for them
//line common/common.w:995
	line int // 1-based line of the \.{@x} in the change file (for diagnostics)
//line common/common.w:996
	replLine int // 1-based change-file line of the first replacement line
//line common/common.w:997
}

//line common/common.w:999
type srcLoc struct {
//line common/common.w:1000
	file string
//line common/common.w:1001
	line int
//line common/common.w:1002
}

//line common/common.w:1004
func (l srcLoc) String() string {
//line common/common.w:1005
	if l.file == "" {
//line common/common.w:1006
		return fmt.Sprintf("line %d", l.line)
//line common/common.w:1007
	}
//line common/common.w:1008
	return fmt.Sprintf("%s:%d", l.file, l.line)
//line common/common.w:1009
}

//line common/common.w:1014
func isChangeCtrl(line string, c byte) bool {
//line common/common.w:1015
	return len(line) >= 2 && line[0] == '@' && line[1] == c
//line common/common.w:1016
}

//line common/common.w:1018
func splitLines(s string) []string {
//line common/common.w:1019
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
//line common/common.w:1020
}

//line common/common.w:1022
func sameLine(a, b string) bool {
//line common/common.w:1023
	return strings.TrimRight(a, " \t") == strings.TrimRight(b, " \t")
//line common/common.w:1024
}

//line common/common.w:1029
func parseChangeFile(src string) ([]change, error) {
//line common/common.w:1030
	lines := splitLines(src)
//line common/common.w:1031
	var changes []change
//line common/common.w:1032
	n := len(lines)
//line common/common.w:1033
	for i := 0; i < n; {
//line common/common.w:1034
		if !isChangeCtrl(lines[i], 'x') {
//line common/common.w:1035
			i++ // commentary between changes
//line common/common.w:1036
			continue
//line common/common.w:1037
		}
//line common/common.w:1038
		c := change{line: i + 1}
//line common/common.w:1039
		i++
//line common/common.w:1040

//line common/common.w:1054
		for i < n && !isChangeCtrl(lines[i], 'y') {
//line common/common.w:1055
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'z') {
//line common/common.w:1056
				return nil, fmt.Errorf("change file line %d: expected @y to close the @x match part", c.line)
//line common/common.w:1057
			}
//line common/common.w:1058
			c.match = append(c.match, lines[i])
//line common/common.w:1059
			i++
//line common/common.w:1060
		}
//line common/common.w:1061
		if i >= n {
//line common/common.w:1062
			return nil, fmt.Errorf("change file line %d: @x without a matching @y", c.line)
//line common/common.w:1063
		}
//line common/common.w:1064
		i++ // skip \.{@y}
//line common/common.w:1065
		c.replLine = i + 1

//line common/common.w:1041

//line common/common.w:1070
		for i < n && !isChangeCtrl(lines[i], 'z') {
//line common/common.w:1071
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'y') {
//line common/common.w:1072
				return nil, fmt.Errorf("change file line %d: expected @z to close the change", c.line)
//line common/common.w:1073
			}
//line common/common.w:1074
			c.repl = append(c.repl, lines[i])
//line common/common.w:1075
			i++
//line common/common.w:1076
		}
//line common/common.w:1077
		if i >= n {
//line common/common.w:1078
			return nil, fmt.Errorf("change file line %d: change has no @z", c.line)
//line common/common.w:1079
		}
//line common/common.w:1080
		i++ // skip \.{@z}

//line common/common.w:1042
		if len(c.match) == 0 {
//line common/common.w:1043
			return nil, fmt.Errorf("change file line %d: the @x match part is empty", c.line)
//line common/common.w:1044
		}
//line common/common.w:1045
		changes = append(changes, c)
//line common/common.w:1046
	}
//line common/common.w:1047
	return changes, nil
//line common/common.w:1048
}

//line common/common.w:1084
func applyChanges(src string, changes []change, chFile string) (string, error) {
//line common/common.w:1085
	out, _, err := applyChangesMapped(splitLines(src), nil, changes, chFile)
//line common/common.w:1086
	if err != nil {
//line common/common.w:1087
		return "", err
//line common/common.w:1088
	}
//line common/common.w:1089
	return strings.Join(out, "\n"), nil
//line common/common.w:1090
}

//line common/common.w:1097
func applyChangesMapped(master []string, locs []srcLoc, changes []change, chFile string) ([]string, []srcLoc, error) {
//line common/common.w:1098
	loc := func(i int) srcLoc {
//line common/common.w:1099
		if locs != nil && i < len(locs) {
//line common/common.w:1100
			return locs[i]
//line common/common.w:1101
		}
//line common/common.w:1102
		return srcLoc{line: i + 1}
//line common/common.w:1103
	}
//line common/common.w:1104
	out := make([]string, 0, len(master))
//line common/common.w:1105
	var outLocs []srcLoc
//line common/common.w:1106
	ci := 0
//line common/common.w:1107
	for i := 0; i < len(master); {
//line common/common.w:1108
		if ci < len(changes) && sameLine(master[i], changes[ci].match[0]) {
//line common/common.w:1109
			if !blockMatches(master, i, changes[ci].match) {
//line common/common.w:1110
				return nil, nil, fmt.Errorf("%s:%d: change did not match the master source at %s",
//line common/common.w:1111
					chFile, changes[ci].line, loc(i))
//line common/common.w:1112
			}
//line common/common.w:1113
			for r, rl := range changes[ci].repl {
//line common/common.w:1114
				out = append(out, rl)
//line common/common.w:1115
				outLocs = append(outLocs, srcLoc{chFile, changes[ci].replLine + r})
//line common/common.w:1116
			}
//line common/common.w:1117
			i += len(changes[ci].match)
//line common/common.w:1118
			ci++
//line common/common.w:1119
			continue
//line common/common.w:1120
		}
//line common/common.w:1121
		out = append(out, master[i])
//line common/common.w:1122
		outLocs = append(outLocs, loc(i))
//line common/common.w:1123
		i++
//line common/common.w:1124
	}
//line common/common.w:1125
	if ci < len(changes) {
//line common/common.w:1126
		return nil, nil, fmt.Errorf("%s:%d: change was never matched (looking for %q)",
//line common/common.w:1127
			chFile, changes[ci].line, changes[ci].match[0])
//line common/common.w:1128
	}
//line common/common.w:1129
	return out, outLocs, nil
//line common/common.w:1130
}

//line common/common.w:1135
func blockMatches(master []string, at int, match []string) bool {
//line common/common.w:1136
	if at+len(match) > len(master) {
//line common/common.w:1137
		return false
//line common/common.w:1138
	}
//line common/common.w:1139
	for k, m := range match {
//line common/common.w:1140
		if !sameLine(master[at+k], m) {
//line common/common.w:1141
			return false
//line common/common.w:1142
		}
//line common/common.w:1143
	}
//line common/common.w:1144
	return true
//line common/common.w:1145
}
