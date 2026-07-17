//line common/common.w:27
package common

//line common/common.w:29
import (
//line common/common.w:30
	"fmt"
//line common/common.w:31
	"os"
//line common/common.w:32
	"path/filepath"
//line common/common.w:33
	"strings"
//line common/common.w:34
)

//line common/common.w:36
const Version = "0.7.0"

//line common/common.w:51
type Format struct {
//line common/common.w:52
	Original string
//line common/common.w:53
	Like string
//line common/common.w:54
	NoIndex bool
//line common/common.w:55
	Macro bool // \.{@d}: typeset Original in \.{typewriter} (a \.{CWEB}-style macro)
//line common/common.w:56
}

//line common/common.w:62
type Section struct {
//line common/common.w:63
	Number int // 1-based section number
//line common/common.w:64
	Line int // 1-based source line where the section begins
//line common/common.w:65
	Starred bool // |true| for \.{@*} sections
//line common/common.w:66
	Depth int // group depth for starred sections ($-1\equiv{}$\.{@**}, $0\equiv{}$\.{@*}, $n\equiv{}$\.{@*n})
//line common/common.w:67
	Title string // starred-section title (text up to the first period)
//line common/common.w:68
	Tex string // commentary, raw \TEX/ with in-text \.{@}-codes still embedded
//line common/common.w:69
	Formats []Format
//line common/common.w:70
	HasCode bool // |true| if the section contributes code
//line common/common.w:71
	Name string // named-section name, or \.{""} for an unnamed @c section
//line common/common.w:72
	IsFile bool // |true| if the name is an output file (\.{@(file@>=})
//line common/common.w:73
	Code string // raw code text with in-code \.{@}-codes still embedded
//line common/common.w:74
	CodeLine int // 1-based combined-source line where |Code| begins (0 if none)
//line common/common.w:75
}

//line common/common.w:82
type Web struct {
//line common/common.w:83
	Limbo string
//line common/common.w:84
	Formats []Format // \.{@f}/\.{@s} directives found in limbo (apply globally)
//line common/common.w:85
	Sections []*Section
//line common/common.w:86
	Warnings []string // non-fatal diagnostics gathered while parsing/checking
//line common/common.w:87
	file string // source filename, for diagnostics (\.{""} if unknown)
//line common/common.w:88
	locs []srcLoc // origin (file, line) of each combined-source line
//line common/common.w:89
	full []string // canonical (non-abbreviated) section names
//line common/common.w:90
}

//line common/common.w:97
func Parse(filename string) (*Web, error) {
//line common/common.w:98
	return ParseWithChange(filename, "")
//line common/common.w:99
}

//line common/common.w:101
func ParseWithChange(filename, changeFile string) (*Web, error) {
//line common/common.w:102
	lines, locs, err := expandIncludes(filename, 0)
//line common/common.w:103
	if err != nil {
//line common/common.w:104
		return nil, err
//line common/common.w:105
	}
//line common/common.w:106

//line common/common.w:119
	if changeFile != "" {
//line common/common.w:120
		chData, err := os.ReadFile(changeFile)
//line common/common.w:121
		if err != nil {
//line common/common.w:122
			return nil, err
//line common/common.w:123
		}
//line common/common.w:124
		changes, err := parseChangeFile(string(chData))
//line common/common.w:125
		if err != nil {
//line common/common.w:126
			return nil, err
//line common/common.w:127
		}
//line common/common.w:128
		lines, locs, err = applyChangesMapped(lines, locs, changes, changeFile)
//line common/common.w:129
		if err != nil {
//line common/common.w:130
			return nil, err
//line common/common.w:131
		}
//line common/common.w:132
	}

//line common/common.w:107
	src := strings.Join(lines, "\n")
//line common/common.w:108
	w := parse(src)
//line common/common.w:109
	w.file = filename
//line common/common.w:110
	w.locs = locs
//line common/common.w:111
	w.finish(src)
//line common/common.w:112
	return w, nil
//line common/common.w:113
}

//line common/common.w:138
func ParseString(src string) *Web {
//line common/common.w:139
	w := parse(src)
//line common/common.w:140
	w.finish(src)
//line common/common.w:141
	return w
//line common/common.w:142
}

//line common/common.w:144
func (w *Web) finish(src string) {
//line common/common.w:145
	w.collectNames()
//line common/common.w:146
	w.Warnings = append(w.Warnings, w.scanDiagnostics(src)...)
//line common/common.w:147
	w.Warnings = append(w.Warnings, w.checkNames()...)
//line common/common.w:148
}

//line common/common.w:156
func (w *Web) Origin(line int) (file string, ln int) {
//line common/common.w:157
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line common/common.w:158
		return w.locs[i].file, w.locs[i].line
//line common/common.w:159
	}
//line common/common.w:160
	return w.file, line
//line common/common.w:161
}

//line common/common.w:163
func (w *Web) at(line int) string {
//line common/common.w:164
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line common/common.w:165
		return w.locs[i].String()
//line common/common.w:166
	}
//line common/common.w:167
	if w.file != "" {
//line common/common.w:168
		return fmt.Sprintf("%s:%d", w.file, line)
//line common/common.w:169
	}
//line common/common.w:170
	return fmt.Sprintf("line %d", line)
//line common/common.w:171
}

//line common/common.w:177
func DefaultExt(name, ext string) string {
//line common/common.w:178
	if name == "" || filepath.Ext(name) != "" {
//line common/common.w:179
		return name
//line common/common.w:180
	}
//line common/common.w:181
	return name + ext
//line common/common.w:182
}

//line common/common.w:188
func expandIncludes(file string, depth int) ([]string, []srcLoc, error) {
//line common/common.w:189
	if depth > 25 {
//line common/common.w:190
		return nil, nil, fmt.Errorf("gweb: @i include nesting too deep at %q", file)
//line common/common.w:191
	}
//line common/common.w:192
	data, err := os.ReadFile(file)
//line common/common.w:193
	if err != nil {
//line common/common.w:194
		return nil, nil, err
//line common/common.w:195
	}
//line common/common.w:196
	raw := splitLines(string(data))
//line common/common.w:197
	if n := len(raw); n > 0 && raw[n-1] == "" {
//line common/common.w:198
		raw = raw[:n-1]
//line common/common.w:199
	}

//line common/common.w:201
	var lines []string
//line common/common.w:202
	var locs []srcLoc
//line common/common.w:203
	dir := filepath.Dir(file)
//line common/common.w:204
	for i, line := range raw {
//line common/common.w:205
		if name, ok := includeDirective(line); ok {
//line common/common.w:206
			path := name
//line common/common.w:207
			if !filepath.IsAbs(path) {
//line common/common.w:208
				path = filepath.Join(dir, name)
//line common/common.w:209
			}
//line common/common.w:210
			sub, subLocs, err := expandIncludes(path, depth+1)
//line common/common.w:211
			if err != nil {
//line common/common.w:212
				return nil, nil, fmt.Errorf("%s:%d: %w", file, i+1, err)
//line common/common.w:213
			}
//line common/common.w:214
			lines = append(lines, sub...)
//line common/common.w:215
			locs = append(locs, subLocs...)
//line common/common.w:216
			continue
//line common/common.w:217
		}
//line common/common.w:218
		lines = append(lines, line)
//line common/common.w:219
		locs = append(locs, srcLoc{file, i + 1})
//line common/common.w:220
	}
//line common/common.w:221
	return lines, locs, nil
//line common/common.w:222
}

//line common/common.w:225
func includeDirective(line string) (name string, ok bool) {
//line common/common.w:226
	t := strings.TrimLeft(line, " \t")
//line common/common.w:227
	if !strings.HasPrefix(t, "@i") {
//line common/common.w:228
		return "", false
//line common/common.w:229
	}
//line common/common.w:230
	rest := t[2:]
//line common/common.w:231
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
//line common/common.w:232
		return "", false
//line common/common.w:233
	}
//line common/common.w:234
	name = strings.Trim(strings.TrimSpace(rest), "\"")
//line common/common.w:235
	return name, name != ""
//line common/common.w:236
}

//line common/common.w:243
func (w *Web) collectNames() {
//line common/common.w:244
	seen := map[string]bool{}
//line common/common.w:245
	add := func(name string) {
//line common/common.w:246
		if name != "" && !strings.HasSuffix(name, "...") && !seen[name] {
//line common/common.w:247
			seen[name] = true
//line common/common.w:248
			w.full = append(w.full, name)
//line common/common.w:249
		}
//line common/common.w:250
	}
//line common/common.w:251
	for _, s := range w.Sections {
//line common/common.w:252
		if !s.IsFile {
//line common/common.w:253
			add(s.Name) // a definition's name
//line common/common.w:254
		}
//line common/common.w:255
		for _, raw := range []string{s.Code, s.Tex} {
//line common/common.w:256
			for _, a := range ScanCode(raw) {
//line common/common.w:257
				if a.Kind == ARef {
//line common/common.w:258
					add(a.Text) // a reference's name
//line common/common.w:259
				}
//line common/common.w:260
			}
//line common/common.w:261
		}
//line common/common.w:262
	}
//line common/common.w:263
}

//line common/common.w:268
func (w *Web) prefixMatches(prefix string) int {
//line common/common.w:269
	n := 0
//line common/common.w:270
	for _, full := range w.full {
//line common/common.w:271
		if strings.HasPrefix(full, prefix) {
//line common/common.w:272
			n++
//line common/common.w:273
		}
//line common/common.w:274
	}
//line common/common.w:275
	return n
//line common/common.w:276
}

//line common/common.w:287
func (w *Web) checkNames() []string {
//line common/common.w:288
	defined := map[string]bool{}
//line common/common.w:289
	for _, s := range w.Sections {
//line common/common.w:290
		if s.Name != "" && !s.IsFile {
//line common/common.w:291
			defined[w.Resolve(s.Name)] = true
//line common/common.w:292
		}
//line common/common.w:293
	}
//line common/common.w:294
	used := map[string]bool{}
//line common/common.w:295
	var warns []string

//line common/common.w:297
	for _, s := range w.Sections {
//line common/common.w:298

//line common/common.w:318
		scan := func(raw string) {
//line common/common.w:319
			for _, a := range ScanCode(raw) {
//line common/common.w:320
				if a.Kind != ARef {
//line common/common.w:321
					continue
//line common/common.w:322
				}
//line common/common.w:323
				canon := w.Resolve(a.Text)
//line common/common.w:324
				if strings.HasSuffix(a.Text, "...") && canon == a.Text {
//line common/common.w:325
					prefix := strings.TrimSpace(strings.TrimSuffix(a.Text, "..."))
//line common/common.w:326
					if m := w.prefixMatches(prefix); m == 0 {
//line common/common.w:327
						warns = append(warns, fmt.Sprintf("%s: no section name matches <%s>", w.at(s.Line), a.Text))
//line common/common.w:328
					} else {
//line common/common.w:329
						warns = append(warns, fmt.Sprintf("%s: ambiguous prefix <%s> matches %d section names", w.at(s.Line), a.Text, m))
//line common/common.w:330
					}
//line common/common.w:331
					continue
//line common/common.w:332
				}
//line common/common.w:333
				if !defined[canon] {
//line common/common.w:334
					warns = append(warns, fmt.Sprintf("%s: reference to undefined section <%s>", w.at(s.Line), a.Text))
//line common/common.w:335
				}
//line common/common.w:336
				used[canon] = true
//line common/common.w:337
			}
//line common/common.w:338
		}

//line common/common.w:299
		scan(s.Code)
//line common/common.w:300
		scan(s.Tex)
//line common/common.w:301
	}

//line common/common.w:303
	warned := map[string]bool{}
//line common/common.w:304
	for _, s := range w.Sections {
//line common/common.w:305
		if s.Name == "" || s.IsFile {
//line common/common.w:306
			continue
//line common/common.w:307
		}
//line common/common.w:308
		canon := w.Resolve(s.Name)
//line common/common.w:309
		if !used[canon] && !warned[canon] {
//line common/common.w:310
			warned[canon] = true
//line common/common.w:311
			warns = append(warns, fmt.Sprintf("%s: section <%s> is defined but never used", w.at(s.Line), s.Name))
//line common/common.w:312
		}
//line common/common.w:313
	}
//line common/common.w:314
	return warns
//line common/common.w:315
}

//line common/common.w:372
func (w *Web) Resolve(name string) string {
//line common/common.w:373
	name = canonName(name)
//line common/common.w:374
	if !strings.HasSuffix(name, "...") {
//line common/common.w:375
		return name
//line common/common.w:376
	}
//line common/common.w:377
	prefix := strings.TrimSpace(strings.TrimSuffix(name, "..."))
//line common/common.w:378
	var match string
//line common/common.w:379
	count := 0
//line common/common.w:380
	for _, full := range w.full {
//line common/common.w:381
		if strings.HasPrefix(full, prefix) {
//line common/common.w:382
			match = full
//line common/common.w:383
			count++
//line common/common.w:384
		}
//line common/common.w:385
	}
//line common/common.w:386
	if count == 1 {
//line common/common.w:387
		return match
//line common/common.w:388
	}
//line common/common.w:389
	return name // unresolved or ambiguous; leave as-is for caller to report
//line common/common.w:390
}

//line common/common.w:345
func lineAt(src string, off int) int {
//line common/common.w:346
	if off > len(src) {
//line common/common.w:347
		off = len(src)
//line common/common.w:348
	}
//line common/common.w:349
	return 1 + strings.Count(src[:off], "\n")
//line common/common.w:350
}

//line common/common.w:352
func canonName(name string) string {
//line common/common.w:353
	return strings.Join(strings.Fields(name), " ")
//line common/common.w:354
}

//line common/common.w:356
func indexFrom(s, sub string, from int) int {
//line common/common.w:357
	if from >= len(s) {
//line common/common.w:358
		return -1
//line common/common.w:359
	}
//line common/common.w:360
	idx := strings.Index(s[from:], sub)
//line common/common.w:361
	if idx < 0 {
//line common/common.w:362
		return -1
//line common/common.w:363
	}
//line common/common.w:364
	return from + idx
//line common/common.w:365
}

//line common/common.w:407
type ctrlKind int

//line common/common.w:409
const (
//line common/common.w:410
	cEOF ctrlKind = iota
//line common/common.w:411
	cSection
//line common/common.w:412
	cCode // \.{@c} (or its synonym \.{@p})
//line common/common.w:413
	cNamed // \.{@<name@>=} or \.{@(file@>=}
//line common/common.w:414
	cDefn // \.{@d}
//line common/common.w:415
	cFormat

//line common/common.w:416
)

//line common/common.w:418
type ctrl struct {
//line common/common.w:419
	kind ctrlKind
//line common/common.w:420
	pos int // index of the leading `\.{@}'
//line common/common.w:421
	end int // index just past the control token
//line common/common.w:422
	depth int // for |cSection|: -1 unstarred (or \.{@**} top level), else starred depth
//line common/common.w:423
	starred bool // for |cSection| (distinguishes \.{@**} from an unstarred section)
//line common/common.w:424
	name string // for |cNamed|
//line common/common.w:425
	isFile bool // for |cNamed| (\.{@(} vs \.{@<})
//line common/common.w:426
	noIndex bool // for |cFormat| (\.{@s})
//line common/common.w:427
}

//line common/common.w:434
func scanStruct(src string, i int) ctrl {
//line common/common.w:435
	n := len(src)
//line common/common.w:436
	for i < n {
//line common/common.w:437
		if src[i] != '@' {
//line common/common.w:438
			i++
//line common/common.w:439
			continue
//line common/common.w:440
		}
//line common/common.w:441
		if i+1 >= n {
//line common/common.w:442
			break
//line common/common.w:443
		}
//line common/common.w:444
		switch c := src[i+1]; {
//line common/common.w:445
		case c == '@':
//line common/common.w:446
			i += 2
//line common/common.w:447
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line common/common.w:448
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line common/common.w:449
		case c == '*':
//line common/common.w:450

//line common/common.w:525
			j := i + 2
//line common/common.w:526
			depth := 0
//line common/common.w:527
			if j < n && src[j] == '*' {
//line common/common.w:528
				j++
//line common/common.w:529
				depth = -1
//line common/common.w:530
			} else {
//line common/common.w:531
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:532
					depth = depth*10 + int(src[j]-'0')
//line common/common.w:533
					j++
//line common/common.w:534
				}
//line common/common.w:535
			}
//line common/common.w:536
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}

//line common/common.w:451
		case c == 'c' || c == 'p':
//line common/common.w:452
			return ctrl{kind: cCode, pos: i, end: i + 2}
//line common/common.w:453
		case c == 'd':
//line common/common.w:454
			return ctrl{kind: cDefn, pos: i, end: i + 2}
//line common/common.w:455
		case c == 'f':
//line common/common.w:456
			return ctrl{kind: cFormat, pos: i, end: i + 2}
//line common/common.w:457
		case c == 's':
//line common/common.w:458
			return ctrl{kind: cFormat, pos: i, end: i + 2, noIndex: true}
//line common/common.w:459
		case c == '<' || c == '(':
//line common/common.w:460

//line common/common.w:542
			end := indexFrom(src, "@>", i+2)
//line common/common.w:543
			if end < 0 {
//line common/common.w:544
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:545
			}
//line common/common.w:546
			after := end + 2
//line common/common.w:547
			k := after
//line common/common.w:548
			for k < n && (src[k] == ' ' || src[k] == '\t') {
//line common/common.w:549
				k++
//line common/common.w:550
			}
//line common/common.w:551
			if k < n && src[k] == '=' {
//line common/common.w:552
				return ctrl{kind: cNamed, pos: i, end: k + 1,
//line common/common.w:553
					name: canonName(src[i+2 : end]), isFile: c == '('}
//line common/common.w:554
			}
//line common/common.w:555
			i = after // a reference, not a definition

//line common/common.w:461
		case c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line common/common.w:462
			end := indexFrom(src, "@>", i+2)
//line common/common.w:463
			if end < 0 {
//line common/common.w:464
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:465
			}
//line common/common.w:466
			i = end + 2
//line common/common.w:467
		case c == '%':
//line common/common.w:468
			j := i + 2
//line common/common.w:469
			for j < n && src[j] != '\n' {
//line common/common.w:470
				j++
//line common/common.w:471
			}
//line common/common.w:472
			i = j
//line common/common.w:473
		default:
//line common/common.w:474
			i += 2
//line common/common.w:475
		}
//line common/common.w:476
	}
//line common/common.w:477
	return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:478
}

//line common/common.w:484
func findNextSection(src string, i int) ctrl {
//line common/common.w:485
	n := len(src)
//line common/common.w:486
	for i < n {
//line common/common.w:487
		if src[i] != '@' {
//line common/common.w:488
			i++
//line common/common.w:489
			continue
//line common/common.w:490
		}
//line common/common.w:491
		if i+1 >= n {
//line common/common.w:492
			break
//line common/common.w:493
		}
//line common/common.w:494
		switch c := src[i+1]; {
//line common/common.w:495
		case c == '@':
//line common/common.w:496
			i += 2
//line common/common.w:497
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line common/common.w:498
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line common/common.w:499
		case c == '*':
//line common/common.w:500

//line common/common.w:525
			j := i + 2
//line common/common.w:526
			depth := 0
//line common/common.w:527
			if j < n && src[j] == '*' {
//line common/common.w:528
				j++
//line common/common.w:529
				depth = -1
//line common/common.w:530
			} else {
//line common/common.w:531
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:532
					depth = depth*10 + int(src[j]-'0')
//line common/common.w:533
					j++
//line common/common.w:534
				}
//line common/common.w:535
			}
//line common/common.w:536
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}

//line common/common.w:501
		case c == '<' || c == '(' || c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line common/common.w:502
			end := indexFrom(src, "@>", i+2)
//line common/common.w:503
			if end < 0 {
//line common/common.w:504
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:505
			}
//line common/common.w:506
			i = end + 2
//line common/common.w:507
		case c == '%':
//line common/common.w:508
			j := i + 2
//line common/common.w:509
			for j < n && src[j] != '\n' {
//line common/common.w:510
				j++
//line common/common.w:511
			}
//line common/common.w:512
			i = j
//line common/common.w:513
		default:
//line common/common.w:514
			i += 2
//line common/common.w:515
		}
//line common/common.w:516
	}
//line common/common.w:517
	return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:518
}

//line common/common.w:563
func parse(src string) *Web {
//line common/common.w:564
	w := &Web{}
//line common/common.w:565
	n := len(src)

//line common/common.w:567
	first := findNextSection(src, 0)
//line common/common.w:568
	w.Limbo, w.Formats = extractLimboFormats(src[:first.pos])
//line common/common.w:569
	i := first.pos

//line common/common.w:571
	num := 0
//line common/common.w:572
	for i < n {
//line common/common.w:573
		// We are positioned at a section break.
//line common/common.w:574
		hdr := src[i+1]
//line common/common.w:575
		num++
//line common/common.w:576
		sec := &Section{Number: num, Line: lineAt(src, i)}
//line common/common.w:577
		if hdr == '*' {
//line common/common.w:578
			h := findSectionHeaderEnd(src, i)
//line common/common.w:579
			sec.Starred = true
//line common/common.w:580
			sec.Depth = h.depth
//line common/common.w:581
			i = h.end
//line common/common.w:582
		} else {
//line common/common.w:583
			i += 2
//line common/common.w:584
		}

//line common/common.w:586
		// \TEX/ part: from here to the next structural control.
//line common/common.w:587
		ct := scanStruct(src, i)
//line common/common.w:588
		sec.Tex = src[i:ct.pos]
//line common/common.w:589
		if sec.Starred {
//line common/common.w:590
			sec.Title = extractTitle(sec.Tex)
//line common/common.w:591
		}

//line common/common.w:593

//line common/common.w:611
		for ct.kind == cDefn || ct.kind == cFormat {
//line common/common.w:612
			nx := scanStruct(src, ct.end)
//line common/common.w:613
			seg := src[ct.end:nx.pos]
//line common/common.w:614
			if ct.kind == cDefn {
//line common/common.w:615
				sec.Formats = append(sec.Formats, parseMacro(seg)...)
//line common/common.w:616
			} else if f, ok := parseFormat(seg, ct.noIndex); ok {
//line common/common.w:617
				sec.Formats = append(sec.Formats, f)
//line common/common.w:618
			}
//line common/common.w:619
			ct = nx
//line common/common.w:620
		}

//line common/common.w:594

//line common/common.w:626
		switch ct.kind {
//line common/common.w:627
		case cCode:
//line common/common.w:628
			sec.HasCode = true
//line common/common.w:629
			sec.CodeLine = lineAt(src, ct.end)
//line common/common.w:630
			nx := findNextSection(src, ct.end)
//line common/common.w:631
			sec.Code = src[ct.end:nx.pos]
//line common/common.w:632
			i = nx.pos
//line common/common.w:633
		case cNamed:
//line common/common.w:634
			sec.HasCode = true
//line common/common.w:635
			sec.Name = ct.name
//line common/common.w:636
			sec.IsFile = ct.isFile
//line common/common.w:637
			sec.CodeLine = lineAt(src, ct.end)
//line common/common.w:638
			nx := findNextSection(src, ct.end)
//line common/common.w:639
			sec.Code = src[ct.end:nx.pos]
//line common/common.w:640
			i = nx.pos
//line common/common.w:641
		default: // |cSection| or |cEOF|: a documentation-only section
//line common/common.w:642
			i = ct.pos
//line common/common.w:643
		}

//line common/common.w:596
		w.Sections = append(w.Sections, sec)
//line common/common.w:597
		if ct.kind == cEOF && sec.Code == "" {
//line common/common.w:598
			break
//line common/common.w:599
		}
//line common/common.w:600
		if i >= n {
//line common/common.w:601
			break
//line common/common.w:602
		}
//line common/common.w:603
	}
//line common/common.w:604
	return w
//line common/common.w:605
}

//line common/common.w:647
func findSectionHeaderEnd(src string, i int) ctrl {
//line common/common.w:648
	n := len(src)
//line common/common.w:649
	j := i + 2
//line common/common.w:650
	depth := 0
//line common/common.w:651
	if j < n && src[j] == '*' {
//line common/common.w:652
		j++
//line common/common.w:653
		depth = -1 // ``\.{@**}'' is the top level: bold in the contents, as \.{CWEB}
//line common/common.w:654
	} else {
//line common/common.w:655
		for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:656
			depth = depth*10 + int(src[j]-'0')
//line common/common.w:657
			j++
//line common/common.w:658
		}
//line common/common.w:659
	}
//line common/common.w:660
	return ctrl{end: j, depth: depth}
//line common/common.w:661
}

//line common/common.w:666
func extractTitle(tex string) string {
//line common/common.w:667
	t := strings.TrimLeft(tex, " \t\n")
//line common/common.w:668
	if i := titleEnd(t); i >= 0 {
//line common/common.w:669
		t = t[:i]
//line common/common.w:670
	}
//line common/common.w:671
	return strings.Join(strings.Fields(t), " ")
//line common/common.w:672
}

//line common/common.w:674
func titleEnd(s string) int {
//line common/common.w:675
	for i := 0; i < len(s); i++ {
//line common/common.w:676
		if s[i] == '.' && (i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t' ||
//line common/common.w:677
			s[i+1] == '\n' || s[i+1] == '\r') {
//line common/common.w:678
			return i
//line common/common.w:679
		}
//line common/common.w:680
	}
//line common/common.w:681
	return -1
//line common/common.w:682
}

//line common/common.w:687
func (w *Web) scanDiagnostics(src string) []string {
//line common/common.w:688
	var warns []string
//line common/common.w:689
	n := len(src)
//line common/common.w:690
	i := 0
//line common/common.w:691
	for i < n {
//line common/common.w:692
		if src[i] != '@' || i+1 >= n {
//line common/common.w:693
			i++
//line common/common.w:694
			continue
//line common/common.w:695
		}
//line common/common.w:696
		switch c := src[i+1]; c {
//line common/common.w:697
		case '@':
//line common/common.w:698
			i += 2
//line common/common.w:699
		case '<', '(', '=', 't', '^', '.', ':', 'q':
//line common/common.w:700
			if end := indexFrom(src, "@>", i+2); end < 0 {
//line common/common.w:701
				warns = append(warns, fmt.Sprintf("%s: unterminated `@%c ... @>'", w.at(lineAt(src, i)), c))
//line common/common.w:702
				i = n
//line common/common.w:703
			} else {
//line common/common.w:704
				i = end + 2
//line common/common.w:705
			}
//line common/common.w:706
		default:
//line common/common.w:707
			i += 2
//line common/common.w:708
		}
//line common/common.w:709
	}
//line common/common.w:710
	return warns
//line common/common.w:711
}

//line common/common.w:715
func parseFormat(seg string, noIndex bool) (Format, bool) {
//line common/common.w:716
	fields := strings.Fields(seg)
//line common/common.w:717
	if len(fields) < 2 {
//line common/common.w:718
		return Format{}, false
//line common/common.w:719
	}
//line common/common.w:720
	return Format{Original: fields[0], Like: fields[1], NoIndex: noIndex}, true
//line common/common.w:721
}

//line common/common.w:731
func parseMacro(seg string) []Format {
//line common/common.w:732
	var fs []Format
//line common/common.w:733
	for _, field := range strings.Fields(seg) {
//line common/common.w:734
		name := field
//line common/common.w:735
		if k := strings.LastIndex(name, "."); k >= 0 {
//line common/common.w:736
			name = name[k+1:]
//line common/common.w:737
		}
//line common/common.w:738
		if name != "" {
//line common/common.w:739
			fs = append(fs, Format{Original: name, Macro: true})
//line common/common.w:740
		}
//line common/common.w:741
	}
//line common/common.w:742
	return fs
//line common/common.w:743
}

//line common/common.w:750
func extractLimboFormats(src string) (string, []Format) {
//line common/common.w:751
	var b strings.Builder
//line common/common.w:752
	var formats []Format
//line common/common.w:753
	n := len(src)
//line common/common.w:754
	i := 0
//line common/common.w:755
	for i < n {
//line common/common.w:756
		if src[i] != '@' || i+1 >= n {
//line common/common.w:757
			b.WriteByte(src[i])
//line common/common.w:758
			i++
//line common/common.w:759
			continue
//line common/common.w:760
		}
//line common/common.w:761
		switch c := src[i+1]; c {
//line common/common.w:762
		case '@':
//line common/common.w:763
			b.WriteString("@@")
//line common/common.w:764
			i += 2
//line common/common.w:765
		case 'd', 'f', 's':
//line common/common.w:766

//line common/common.w:799
			var fs []Format
//line common/common.w:800
			var j int
//line common/common.w:801
			if c == 'd' {
//line common/common.w:802
				j = i + 2
//line common/common.w:803
				for j < n && src[j] != '@' {
//line common/common.w:804
					j++ // the body runs to the next control code
//line common/common.w:805
				}
//line common/common.w:806
				fs = parseMacro(src[i+2 : j])
//line common/common.w:807
			} else {
//line common/common.w:808
				j = endOfFormatArgs(src, i+2, n)
//line common/common.w:809
				if f, ok := parseFormat(src[i+2:j], c == 's'); ok {
//line common/common.w:810
					fs = []Format{f}
//line common/common.w:811
				}
//line common/common.w:812
			}
//line common/common.w:813
			formats = append(formats, fs...)
//line common/common.w:814
			if k := skipBlanks(src, j, n); k < n && src[k] == '\n' {
//line common/common.w:815
				j = k + 1 // the directive ended its line; drop the blanks and the newline
//line common/common.w:816
			}
//line common/common.w:817
			i = j

//line common/common.w:767
		case 'q':
//line common/common.w:768
			if end := indexFrom(src, "@>", i+2); end < 0 {
//line common/common.w:769
				i = n // unterminated: drop the rest of limbo
//line common/common.w:770
			} else {
//line common/common.w:771
				i = end + 2 // drop the source-only comment
//line common/common.w:772
			}
//line common/common.w:773
		case '<', '(', '=', 't', '^', '.', ':':
//line common/common.w:774
			end := indexFrom(src, "@>", i+2)
//line common/common.w:775
			if end < 0 {
//line common/common.w:776
				b.WriteString(src[i:])
//line common/common.w:777
				i = n
//line common/common.w:778
			} else {
//line common/common.w:779
				b.WriteString(src[i : end+2])
//line common/common.w:780
				i = end + 2
//line common/common.w:781
			}
//line common/common.w:782
		default:
//line common/common.w:783
			b.WriteString(src[i : i+2])
//line common/common.w:784
			i += 2
//line common/common.w:785
		}
//line common/common.w:786
	}
//line common/common.w:787
	return b.String(), formats
//line common/common.w:788
}

//line common/common.w:823
func endOfFormatArgs(src string, p, n int) int {
//line common/common.w:824
	for word := 0; word < 2; word++ {
//line common/common.w:825
		p = skipBlanks(src, p, n)
//line common/common.w:826
		for p < n && src[p] != ' ' && src[p] != '\t' && src[p] != '\n' {
//line common/common.w:827
			p++
//line common/common.w:828
		}
//line common/common.w:829
	}
//line common/common.w:830
	return p
//line common/common.w:831
}

//line common/common.w:833
func skipBlanks(src string, p, n int) int {
//line common/common.w:834
	for p < n && (src[p] == ' ' || src[p] == '\t') {
//line common/common.w:835
		p++
//line common/common.w:836
	}
//line common/common.w:837
	return p
//line common/common.w:838
}

//line common/common.w:849
type AtomKind int

//line common/common.w:851
const (
//line common/common.w:852
	AText AtomKind = iota // ordinary \GO/ source text
//line common/common.w:853
	ARef // \.{@<name@>} reference to a named section
//line common/common.w:854
	AVerbatim // \.{@=text@>} passed verbatim to tangled output
//line common/common.w:855
	ATeX // \.{@t text@>} \TEX/ text for the woven output
//line common/common.w:856
	AIndex // \.{@\^/@./@}: index entry
//line common/common.w:857
	APaste // \.{@\&} join (delete surrounding whitespace)
//line common/common.w:858
	ALayout // \.{@}, \.{@/} \.{@|} \.{@\#} woven-output layout hints
//line common/common.w:859
	AIndexDef // \.{@!} force the next identifier to index as a definition
//line common/common.w:860
)

//line common/common.w:862
type Atom struct {
//line common/common.w:863
	Kind AtomKind
//line common/common.w:864
	Text string // payload for |AText|/|AVerbatim|/|ATeX|/|AIndex|; name for |ARef|
//line common/common.w:865
	Index byte // '\.{\^}','\.{.}','\.{:}' for AIndex; '\.{,}' '\.{/}' '\.{|}' '\.{\#}' for |ALayout|
//line common/common.w:866
}

//line common/common.w:872
func ScanCode(code string) []Atom {
//line common/common.w:873
	var atoms []Atom
//line common/common.w:874
	var buf strings.Builder
//line common/common.w:875
	flush := func() {
//line common/common.w:876
		if buf.Len() > 0 {
//line common/common.w:877
			atoms = append(atoms, Atom{Kind: AText, Text: buf.String()})
//line common/common.w:878
			buf.Reset()
//line common/common.w:879
		}
//line common/common.w:880
	}
//line common/common.w:881
	n := len(code)
//line common/common.w:882
	i := 0
//line common/common.w:883
	for i < n {
//line common/common.w:884
		c := code[i]
//line common/common.w:885
		if c != '@' || i+1 >= n {
//line common/common.w:886
			buf.WriteByte(c)
//line common/common.w:887
			i++
//line common/common.w:888
			continue
//line common/common.w:889
		}
//line common/common.w:890

//line common/common.w:902
		switch d := code[i+1]; d {

//line common/common.w:912
		case '@':
//line common/common.w:913
			buf.WriteByte('@')
//line common/common.w:914
			i += 2
//line common/common.w:915
		case '&':
//line common/common.w:916
			flush()
//line common/common.w:917
			atoms = append(atoms, Atom{Kind: APaste})
//line common/common.w:918
			i += 2

//line common/common.w:925
		case '<':
//line common/common.w:926
			end := indexFrom(code, "@>", i+2)
//line common/common.w:927
			if end < 0 {
//line common/common.w:928
				buf.WriteString(code[i:])
//line common/common.w:929
				i = n
//line common/common.w:930
				continue
//line common/common.w:931
			}
//line common/common.w:932
			flush()
//line common/common.w:933
			atoms = append(atoms, Atom{Kind: ARef, Text: canonName(code[i+2 : end])})
//line common/common.w:934
			i = end + 2
//line common/common.w:935
		case '=':
//line common/common.w:936
			end := indexFrom(code, "@>", i+2)
//line common/common.w:937
			if end < 0 {
//line common/common.w:938
				i = n
//line common/common.w:939
				continue
//line common/common.w:940
			}
//line common/common.w:941
			flush()
//line common/common.w:942
			atoms = append(atoms, Atom{Kind: AVerbatim, Text: code[i+2 : end]})
//line common/common.w:943
			i = end + 2
//line common/common.w:944
		case 't':
//line common/common.w:945
			end := indexFrom(code, "@>", i+2)
//line common/common.w:946
			if end < 0 {
//line common/common.w:947
				i = n
//line common/common.w:948
				continue
//line common/common.w:949
			}
//line common/common.w:950
			flush()
//line common/common.w:951
			atoms = append(atoms, Atom{Kind: ATeX, Text: code[i+2 : end]})
//line common/common.w:952
			i = end + 2
//line common/common.w:953
		case '^', '.', ':':
//line common/common.w:954
			end := indexFrom(code, "@>", i+2)
//line common/common.w:955
			if end < 0 {
//line common/common.w:956
				i = n
//line common/common.w:957
				continue
//line common/common.w:958
			}
//line common/common.w:959
			flush()
//line common/common.w:960
			atoms = append(atoms, Atom{Kind: AIndex, Text: code[i+2 : end], Index: d})
//line common/common.w:961
			i = end + 2
//line common/common.w:962
		case 'q':
//line common/common.w:963
			end := indexFrom(code, "@>", i+2)
//line common/common.w:964
			if end < 0 {
//line common/common.w:965
				i = n
//line common/common.w:966
				continue
//line common/common.w:967
			}
//line common/common.w:968
			i = end + 2 // ignored material

//line common/common.w:981
		case '%':
//line common/common.w:982
			j := i + 2
//line common/common.w:983
			for j < n && code[j] != '\n' {
//line common/common.w:984
				j++
//line common/common.w:985
			}
//line common/common.w:986
			i = j
//line common/common.w:987
		case '>':
//line common/common.w:988
			i += 2 // stray terminator
//line common/common.w:989
		case ',', '/', '|', '#':
//line common/common.w:990
			flush()
//line common/common.w:991
			atoms = append(atoms, Atom{Kind: ALayout, Index: d})
//line common/common.w:992
			i += 2
//line common/common.w:993
		case '!':
//line common/common.w:994
			flush()
//line common/common.w:995
			atoms = append(atoms, Atom{Kind: AIndexDef})
//line common/common.w:996
			i += 2
//line common/common.w:997
		case '+', '[', ']', ';':
//line common/common.w:998
			i += 2 // \.{CWEB} prettyprinter hints, dropped
//line common/common.w:999
		default:
//line common/common.w:1000
			i += 2 // unknown \.{@x}: drop it rather than corrupt the output

//line common/common.w:906
		}

//line common/common.w:891
	}
//line common/common.w:892
	flush()
//line common/common.w:893
	return atoms
//line common/common.w:894
}

//line common/common.w:1022
type change struct {
//line common/common.w:1023
	match []string // lines to find in the master source
//line common/common.w:1024
	repl []string // lines to substitute for them
//line common/common.w:1025
	line int // 1-based line of the \.{@x} in the change file (for diagnostics)
//line common/common.w:1026
	replLine int // 1-based change-file line of the first replacement line
//line common/common.w:1027
}

//line common/common.w:1029
type srcLoc struct {
//line common/common.w:1030
	file string
//line common/common.w:1031
	line int
//line common/common.w:1032
}

//line common/common.w:1034
func (l srcLoc) String() string {
//line common/common.w:1035
	if l.file == "" {
//line common/common.w:1036
		return fmt.Sprintf("line %d", l.line)
//line common/common.w:1037
	}
//line common/common.w:1038
	return fmt.Sprintf("%s:%d", l.file, l.line)
//line common/common.w:1039
}

//line common/common.w:1044
func isChangeCtrl(line string, c byte) bool {
//line common/common.w:1045
	return len(line) >= 2 && line[0] == '@' && line[1] == c
//line common/common.w:1046
}

//line common/common.w:1048
func splitLines(s string) []string {
//line common/common.w:1049
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
//line common/common.w:1050
}

//line common/common.w:1052
func sameLine(a, b string) bool {
//line common/common.w:1053
	return strings.TrimRight(a, " \t") == strings.TrimRight(b, " \t")
//line common/common.w:1054
}

//line common/common.w:1059
func parseChangeFile(src string) ([]change, error) {
//line common/common.w:1060
	lines := splitLines(src)
//line common/common.w:1061
	var changes []change
//line common/common.w:1062
	n := len(lines)
//line common/common.w:1063
	for i := 0; i < n; {
//line common/common.w:1064
		if !isChangeCtrl(lines[i], 'x') {
//line common/common.w:1065
			i++ // commentary between changes
//line common/common.w:1066
			continue
//line common/common.w:1067
		}
//line common/common.w:1068
		c := change{line: i + 1}
//line common/common.w:1069
		i++
//line common/common.w:1070

//line common/common.w:1084
		for i < n && !isChangeCtrl(lines[i], 'y') {
//line common/common.w:1085
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'z') {
//line common/common.w:1086
				return nil, fmt.Errorf("change file line %d: expected @y to close the @x match part", c.line)
//line common/common.w:1087
			}
//line common/common.w:1088
			c.match = append(c.match, lines[i])
//line common/common.w:1089
			i++
//line common/common.w:1090
		}
//line common/common.w:1091
		if i >= n {
//line common/common.w:1092
			return nil, fmt.Errorf("change file line %d: @x without a matching @y", c.line)
//line common/common.w:1093
		}
//line common/common.w:1094
		i++ // skip \.{@y}
//line common/common.w:1095
		c.replLine = i + 1

//line common/common.w:1071

//line common/common.w:1100
		for i < n && !isChangeCtrl(lines[i], 'z') {
//line common/common.w:1101
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'y') {
//line common/common.w:1102
				return nil, fmt.Errorf("change file line %d: expected @z to close the change", c.line)
//line common/common.w:1103
			}
//line common/common.w:1104
			c.repl = append(c.repl, lines[i])
//line common/common.w:1105
			i++
//line common/common.w:1106
		}
//line common/common.w:1107
		if i >= n {
//line common/common.w:1108
			return nil, fmt.Errorf("change file line %d: change has no @z", c.line)
//line common/common.w:1109
		}
//line common/common.w:1110
		i++ // skip \.{@z}

//line common/common.w:1072
		if len(c.match) == 0 {
//line common/common.w:1073
			return nil, fmt.Errorf("change file line %d: the @x match part is empty", c.line)
//line common/common.w:1074
		}
//line common/common.w:1075
		changes = append(changes, c)
//line common/common.w:1076
	}
//line common/common.w:1077
	return changes, nil
//line common/common.w:1078
}

//line common/common.w:1114
func applyChanges(src string, changes []change, chFile string) (string, error) {
//line common/common.w:1115
	out, _, err := applyChangesMapped(splitLines(src), nil, changes, chFile)
//line common/common.w:1116
	if err != nil {
//line common/common.w:1117
		return "", err
//line common/common.w:1118
	}
//line common/common.w:1119
	return strings.Join(out, "\n"), nil
//line common/common.w:1120
}

//line common/common.w:1127
func applyChangesMapped(master []string, locs []srcLoc, changes []change, chFile string) (
//line common/common.w:1128
	[]string, []srcLoc, error,
//line common/common.w:1129
) {
//line common/common.w:1130
	loc := func(i int) srcLoc {
//line common/common.w:1131
		if locs != nil && i < len(locs) {
//line common/common.w:1132
			return locs[i]
//line common/common.w:1133
		}
//line common/common.w:1134
		return srcLoc{line: i + 1}
//line common/common.w:1135
	}
//line common/common.w:1136
	out := make([]string, 0, len(master))
//line common/common.w:1137
	var outLocs []srcLoc
//line common/common.w:1138
	ci := 0
//line common/common.w:1139
	for i := 0; i < len(master); {
//line common/common.w:1140
		if ci < len(changes) && sameLine(master[i], changes[ci].match[0]) {
//line common/common.w:1141
			if !blockMatches(master, i, changes[ci].match) {
//line common/common.w:1142
				return nil, nil, fmt.Errorf("%s:%d: change did not match the master source at %s",
//line common/common.w:1143
					chFile, changes[ci].line, loc(i))
//line common/common.w:1144
			}
//line common/common.w:1145
			for r, rl := range changes[ci].repl {
//line common/common.w:1146
				out = append(out, rl)
//line common/common.w:1147
				outLocs = append(outLocs, srcLoc{chFile, changes[ci].replLine + r})
//line common/common.w:1148
			}
//line common/common.w:1149
			i += len(changes[ci].match)
//line common/common.w:1150
			ci++
//line common/common.w:1151
			continue
//line common/common.w:1152
		}
//line common/common.w:1153
		out = append(out, master[i])
//line common/common.w:1154
		outLocs = append(outLocs, loc(i))
//line common/common.w:1155
		i++
//line common/common.w:1156
	}
//line common/common.w:1157
	if ci < len(changes) {
//line common/common.w:1158
		return nil, nil, fmt.Errorf("%s:%d: change was never matched (looking for %q)",
//line common/common.w:1159
			chFile, changes[ci].line, changes[ci].match[0])
//line common/common.w:1160
	}
//line common/common.w:1161
	return out, outLocs, nil
//line common/common.w:1162
}

//line common/common.w:1167
func blockMatches(master []string, at int, match []string) bool {
//line common/common.w:1168
	if at+len(match) > len(master) {
//line common/common.w:1169
		return false
//line common/common.w:1170
	}
//line common/common.w:1171
	for k, m := range match {
//line common/common.w:1172
		if !sameLine(master[at+k], m) {
//line common/common.w:1173
			return false
//line common/common.w:1174
		}
//line common/common.w:1175
	}
//line common/common.w:1176
	return true
//line common/common.w:1177
}
