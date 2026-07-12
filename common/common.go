//line common/common.w:23
package common

//line common/common.w:25
import (
//line common/common.w:26
	"fmt"
//line common/common.w:27
	"os"
//line common/common.w:28
	"path/filepath"
//line common/common.w:29
	"strings"
//line common/common.w:30
)

//line common/common.w:32
const Version = "0.6.5"

//line common/common.w:47
type Format struct {
//line common/common.w:48
	Original string
//line common/common.w:49
	Like string
//line common/common.w:50
	NoIndex bool
//line common/common.w:51
	Macro bool // \.{@d}: typeset Original in \.{typewriter} (a \.{CWEB}-style macro)
//line common/common.w:52
}

//line common/common.w:58
type Section struct {
//line common/common.w:59
	Number int // 1-based section number
//line common/common.w:60
	Line int // 1-based source line where the section begins
//line common/common.w:61
	Starred bool // |true| for \.{@*} sections
//line common/common.w:62
	Depth int // group depth for starred sections ($-1\equiv{}$\.{@**}, $0\equiv{}$\.{@*}, $n\equiv{}$\.{@*n})
//line common/common.w:63
	Title string // starred-section title (text up to the first period)
//line common/common.w:64
	Tex string // commentary, raw \TEX/ with in-text \.{@}-codes still embedded
//line common/common.w:65
	Formats []Format
//line common/common.w:66
	HasCode bool // |true| if the section contributes code
//line common/common.w:67
	Name string // named-section name, or \.{""} for an unnamed @c section
//line common/common.w:68
	IsFile bool // |true| if the name is an output file (\.{@(file@>=})
//line common/common.w:69
	Code string // raw code text with in-code \.{@}-codes still embedded
//line common/common.w:70
	CodeLine int // 1-based combined-source line where |Code| begins (0 if none)
//line common/common.w:71
}

//line common/common.w:78
type Web struct {
//line common/common.w:79
	Limbo string
//line common/common.w:80
	Formats []Format // \.{@f}/\.{@s} directives found in limbo (apply globally)
//line common/common.w:81
	Sections []*Section
//line common/common.w:82
	Warnings []string // non-fatal diagnostics gathered while parsing/checking
//line common/common.w:83
	file string // source filename, for diagnostics (\.{""} if unknown)
//line common/common.w:84
	locs []srcLoc // origin (file, line) of each combined-source line
//line common/common.w:85
	full []string // canonical (non-abbreviated) section names
//line common/common.w:86
}

//line common/common.w:93
func Parse(filename string) (*Web, error) {
//line common/common.w:94
	return ParseWithChange(filename, "")
//line common/common.w:95
}

//line common/common.w:97
func ParseWithChange(filename, changeFile string) (*Web, error) {
//line common/common.w:98
	lines, locs, err := expandIncludes(filename, 0)
//line common/common.w:99
	if err != nil {
//line common/common.w:100
		return nil, err
//line common/common.w:101
	}
//line common/common.w:102

//line common/common.w:115
	if changeFile != "" {
//line common/common.w:116
		chData, err := os.ReadFile(changeFile)
//line common/common.w:117
		if err != nil {
//line common/common.w:118
			return nil, err
//line common/common.w:119
		}
//line common/common.w:120
		changes, err := parseChangeFile(string(chData))
//line common/common.w:121
		if err != nil {
//line common/common.w:122
			return nil, err
//line common/common.w:123
		}
//line common/common.w:124
		lines, locs, err = applyChangesMapped(lines, locs, changes, changeFile)
//line common/common.w:125
		if err != nil {
//line common/common.w:126
			return nil, err
//line common/common.w:127
		}
//line common/common.w:128
	}

//line common/common.w:103
	src := strings.Join(lines, "\n")
//line common/common.w:104
	w := parse(src)
//line common/common.w:105
	w.file = filename
//line common/common.w:106
	w.locs = locs
//line common/common.w:107
	w.finish(src)
//line common/common.w:108
	return w, nil
//line common/common.w:109
}

//line common/common.w:134
func ParseString(src string) *Web {
//line common/common.w:135
	w := parse(src)
//line common/common.w:136
	w.finish(src)
//line common/common.w:137
	return w
//line common/common.w:138
}

//line common/common.w:140
func (w *Web) finish(src string) {
//line common/common.w:141
	w.collectNames()
//line common/common.w:142
	w.Warnings = append(w.Warnings, w.scanDiagnostics(src)...)
//line common/common.w:143
	w.Warnings = append(w.Warnings, w.checkNames()...)
//line common/common.w:144
}

//line common/common.w:152
func (w *Web) Origin(line int) (file string, ln int) {
//line common/common.w:153
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line common/common.w:154
		return w.locs[i].file, w.locs[i].line
//line common/common.w:155
	}
//line common/common.w:156
	return w.file, line
//line common/common.w:157
}

//line common/common.w:159
func (w *Web) at(line int) string {
//line common/common.w:160
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line common/common.w:161
		return w.locs[i].String()
//line common/common.w:162
	}
//line common/common.w:163
	if w.file != "" {
//line common/common.w:164
		return fmt.Sprintf("%s:%d", w.file, line)
//line common/common.w:165
	}
//line common/common.w:166
	return fmt.Sprintf("line %d", line)
//line common/common.w:167
}

//line common/common.w:173
func DefaultExt(name, ext string) string {
//line common/common.w:174
	if name == "" || filepath.Ext(name) != "" {
//line common/common.w:175
		return name
//line common/common.w:176
	}
//line common/common.w:177
	return name + ext
//line common/common.w:178
}

//line common/common.w:184
func expandIncludes(file string, depth int) ([]string, []srcLoc, error) {
//line common/common.w:185
	if depth > 25 {
//line common/common.w:186
		return nil, nil, fmt.Errorf("gweb: @i include nesting too deep at %q", file)
//line common/common.w:187
	}
//line common/common.w:188
	data, err := os.ReadFile(file)
//line common/common.w:189
	if err != nil {
//line common/common.w:190
		return nil, nil, err
//line common/common.w:191
	}
//line common/common.w:192
	raw := splitLines(string(data))
//line common/common.w:193
	if n := len(raw); n > 0 && raw[n-1] == "" {
//line common/common.w:194
		raw = raw[:n-1]
//line common/common.w:195
	}

//line common/common.w:197
	var lines []string
//line common/common.w:198
	var locs []srcLoc
//line common/common.w:199
	dir := filepath.Dir(file)
//line common/common.w:200
	for i, line := range raw {
//line common/common.w:201
		if name, ok := includeDirective(line); ok {
//line common/common.w:202
			path := name
//line common/common.w:203
			if !filepath.IsAbs(path) {
//line common/common.w:204
				path = filepath.Join(dir, name)
//line common/common.w:205
			}
//line common/common.w:206
			sub, subLocs, err := expandIncludes(path, depth+1)
//line common/common.w:207
			if err != nil {
//line common/common.w:208
				return nil, nil, fmt.Errorf("%s:%d: %w", file, i+1, err)
//line common/common.w:209
			}
//line common/common.w:210
			lines = append(lines, sub...)
//line common/common.w:211
			locs = append(locs, subLocs...)
//line common/common.w:212
			continue
//line common/common.w:213
		}
//line common/common.w:214
		lines = append(lines, line)
//line common/common.w:215
		locs = append(locs, srcLoc{file, i + 1})
//line common/common.w:216
	}
//line common/common.w:217
	return lines, locs, nil
//line common/common.w:218
}

//line common/common.w:221
func includeDirective(line string) (name string, ok bool) {
//line common/common.w:222
	t := strings.TrimLeft(line, " \t")
//line common/common.w:223
	if !strings.HasPrefix(t, "@i") {
//line common/common.w:224
		return "", false
//line common/common.w:225
	}
//line common/common.w:226
	rest := t[2:]
//line common/common.w:227
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
//line common/common.w:228
		return "", false
//line common/common.w:229
	}
//line common/common.w:230
	name = strings.Trim(strings.TrimSpace(rest), "\"")
//line common/common.w:231
	return name, name != ""
//line common/common.w:232
}

//line common/common.w:239
func (w *Web) collectNames() {
//line common/common.w:240
	seen := map[string]bool{}
//line common/common.w:241
	add := func(name string) {
//line common/common.w:242
		if name != "" && !strings.HasSuffix(name, "...") && !seen[name] {
//line common/common.w:243
			seen[name] = true
//line common/common.w:244
			w.full = append(w.full, name)
//line common/common.w:245
		}
//line common/common.w:246
	}
//line common/common.w:247
	for _, s := range w.Sections {
//line common/common.w:248
		if !s.IsFile {
//line common/common.w:249
			add(s.Name) // a definition's name
//line common/common.w:250
		}
//line common/common.w:251
		for _, raw := range []string{s.Code, s.Tex} {
//line common/common.w:252
			for _, a := range ScanCode(raw) {
//line common/common.w:253
				if a.Kind == ARef {
//line common/common.w:254
					add(a.Text) // a reference's name
//line common/common.w:255
				}
//line common/common.w:256
			}
//line common/common.w:257
		}
//line common/common.w:258
	}
//line common/common.w:259
}

//line common/common.w:264
func (w *Web) prefixMatches(prefix string) int {
//line common/common.w:265
	n := 0
//line common/common.w:266
	for _, full := range w.full {
//line common/common.w:267
		if strings.HasPrefix(full, prefix) {
//line common/common.w:268
			n++
//line common/common.w:269
		}
//line common/common.w:270
	}
//line common/common.w:271
	return n
//line common/common.w:272
}

//line common/common.w:283
func (w *Web) checkNames() []string {
//line common/common.w:284
	defined := map[string]bool{}
//line common/common.w:285
	for _, s := range w.Sections {
//line common/common.w:286
		if s.Name != "" && !s.IsFile {
//line common/common.w:287
			defined[w.Resolve(s.Name)] = true
//line common/common.w:288
		}
//line common/common.w:289
	}
//line common/common.w:290
	used := map[string]bool{}
//line common/common.w:291
	var warns []string

//line common/common.w:293
	for _, s := range w.Sections {
//line common/common.w:294

//line common/common.w:314
		scan := func(raw string) {
//line common/common.w:315
			for _, a := range ScanCode(raw) {
//line common/common.w:316
				if a.Kind != ARef {
//line common/common.w:317
					continue
//line common/common.w:318
				}
//line common/common.w:319
				canon := w.Resolve(a.Text)
//line common/common.w:320
				if strings.HasSuffix(a.Text, "...") && canon == a.Text {
//line common/common.w:321
					prefix := strings.TrimSpace(strings.TrimSuffix(a.Text, "..."))
//line common/common.w:322
					if m := w.prefixMatches(prefix); m == 0 {
//line common/common.w:323
						warns = append(warns, fmt.Sprintf("%s: no section name matches <%s>", w.at(s.Line), a.Text))
//line common/common.w:324
					} else {
//line common/common.w:325
						warns = append(warns, fmt.Sprintf("%s: ambiguous prefix <%s> matches %d section names", w.at(s.Line), a.Text, m))
//line common/common.w:326
					}
//line common/common.w:327
					continue
//line common/common.w:328
				}
//line common/common.w:329
				if !defined[canon] {
//line common/common.w:330
					warns = append(warns, fmt.Sprintf("%s: reference to undefined section <%s>", w.at(s.Line), a.Text))
//line common/common.w:331
				}
//line common/common.w:332
				used[canon] = true
//line common/common.w:333
			}
//line common/common.w:334
		}

//line common/common.w:295
		scan(s.Code)
//line common/common.w:296
		scan(s.Tex)
//line common/common.w:297
	}

//line common/common.w:299
	warned := map[string]bool{}
//line common/common.w:300
	for _, s := range w.Sections {
//line common/common.w:301
		if s.Name == "" || s.IsFile {
//line common/common.w:302
			continue
//line common/common.w:303
		}
//line common/common.w:304
		canon := w.Resolve(s.Name)
//line common/common.w:305
		if !used[canon] && !warned[canon] {
//line common/common.w:306
			warned[canon] = true
//line common/common.w:307
			warns = append(warns, fmt.Sprintf("%s: section <%s> is defined but never used", w.at(s.Line), s.Name))
//line common/common.w:308
		}
//line common/common.w:309
	}
//line common/common.w:310
	return warns
//line common/common.w:311
}

//line common/common.w:368
func (w *Web) Resolve(name string) string {
//line common/common.w:369
	name = canonName(name)
//line common/common.w:370
	if !strings.HasSuffix(name, "...") {
//line common/common.w:371
		return name
//line common/common.w:372
	}
//line common/common.w:373
	prefix := strings.TrimSpace(strings.TrimSuffix(name, "..."))
//line common/common.w:374
	var match string
//line common/common.w:375
	count := 0
//line common/common.w:376
	for _, full := range w.full {
//line common/common.w:377
		if strings.HasPrefix(full, prefix) {
//line common/common.w:378
			match = full
//line common/common.w:379
			count++
//line common/common.w:380
		}
//line common/common.w:381
	}
//line common/common.w:382
	if count == 1 {
//line common/common.w:383
		return match
//line common/common.w:384
	}
//line common/common.w:385
	return name // unresolved or ambiguous; leave as-is for caller to report
//line common/common.w:386
}

//line common/common.w:341
func lineAt(src string, off int) int {
//line common/common.w:342
	if off > len(src) {
//line common/common.w:343
		off = len(src)
//line common/common.w:344
	}
//line common/common.w:345
	return 1 + strings.Count(src[:off], "\n")
//line common/common.w:346
}

//line common/common.w:348
func canonName(name string) string {
//line common/common.w:349
	return strings.Join(strings.Fields(name), " ")
//line common/common.w:350
}

//line common/common.w:352
func indexFrom(s, sub string, from int) int {
//line common/common.w:353
	if from >= len(s) {
//line common/common.w:354
		return -1
//line common/common.w:355
	}
//line common/common.w:356
	idx := strings.Index(s[from:], sub)
//line common/common.w:357
	if idx < 0 {
//line common/common.w:358
		return -1
//line common/common.w:359
	}
//line common/common.w:360
	return from + idx
//line common/common.w:361
}

//line common/common.w:403
type ctrlKind int

//line common/common.w:405
const (
//line common/common.w:406
	cEOF ctrlKind = iota
//line common/common.w:407
	cSection
//line common/common.w:408
	cCode // \.{@c} (or its synonym \.{@p})
//line common/common.w:409
	cNamed // \.{@<name@>=} or \.{@(file@>=}
//line common/common.w:410
	cDefn // \.{@d}
//line common/common.w:411
	cFormat

//line common/common.w:412
)

//line common/common.w:414
type ctrl struct {
//line common/common.w:415
	kind ctrlKind
//line common/common.w:416
	pos int // index of the leading `\.{@}'
//line common/common.w:417
	end int // index just past the control token
//line common/common.w:418
	depth int // for |cSection|: -1 unstarred (or \.{@**} top level), else starred depth
//line common/common.w:419
	starred bool // for |cSection| (distinguishes \.{@**} from an unstarred section)
//line common/common.w:420
	name string // for |cNamed|
//line common/common.w:421
	isFile bool // for |cNamed| (\.{@(} vs \.{@<})
//line common/common.w:422
	noIndex bool // for |cFormat| (\.{@s})
//line common/common.w:423
}

//line common/common.w:430
func scanStruct(src string, i int) ctrl {
//line common/common.w:431
	n := len(src)
//line common/common.w:432
	for i < n {
//line common/common.w:433
		if src[i] != '@' {
//line common/common.w:434
			i++
//line common/common.w:435
			continue
//line common/common.w:436
		}
//line common/common.w:437
		if i+1 >= n {
//line common/common.w:438
			break
//line common/common.w:439
		}
//line common/common.w:440
		switch c := src[i+1]; {
//line common/common.w:441
		case c == '@':
//line common/common.w:442
			i += 2
//line common/common.w:443
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line common/common.w:444
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line common/common.w:445
		case c == '*':
//line common/common.w:446

//line common/common.w:521
			j := i + 2
//line common/common.w:522
			depth := 0
//line common/common.w:523
			if j < n && src[j] == '*' {
//line common/common.w:524
				j++
//line common/common.w:525
				depth = -1
//line common/common.w:526
			} else {
//line common/common.w:527
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:528
					depth = depth*10 + int(src[j]-'0')
//line common/common.w:529
					j++
//line common/common.w:530
				}
//line common/common.w:531
			}
//line common/common.w:532
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}

//line common/common.w:447
		case c == 'c' || c == 'p':
//line common/common.w:448
			return ctrl{kind: cCode, pos: i, end: i + 2}
//line common/common.w:449
		case c == 'd':
//line common/common.w:450
			return ctrl{kind: cDefn, pos: i, end: i + 2}
//line common/common.w:451
		case c == 'f':
//line common/common.w:452
			return ctrl{kind: cFormat, pos: i, end: i + 2}
//line common/common.w:453
		case c == 's':
//line common/common.w:454
			return ctrl{kind: cFormat, pos: i, end: i + 2, noIndex: true}
//line common/common.w:455
		case c == '<' || c == '(':
//line common/common.w:456

//line common/common.w:538
			end := indexFrom(src, "@>", i+2)
//line common/common.w:539
			if end < 0 {
//line common/common.w:540
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:541
			}
//line common/common.w:542
			after := end + 2
//line common/common.w:543
			k := after
//line common/common.w:544
			for k < n && (src[k] == ' ' || src[k] == '\t') {
//line common/common.w:545
				k++
//line common/common.w:546
			}
//line common/common.w:547
			if k < n && src[k] == '=' {
//line common/common.w:548
				return ctrl{kind: cNamed, pos: i, end: k + 1,
//line common/common.w:549
					name: canonName(src[i+2 : end]), isFile: c == '('}
//line common/common.w:550
			}
//line common/common.w:551
			i = after // a reference, not a definition

//line common/common.w:457
		case c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line common/common.w:458
			end := indexFrom(src, "@>", i+2)
//line common/common.w:459
			if end < 0 {
//line common/common.w:460
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:461
			}
//line common/common.w:462
			i = end + 2
//line common/common.w:463
		case c == '%':
//line common/common.w:464
			j := i + 2
//line common/common.w:465
			for j < n && src[j] != '\n' {
//line common/common.w:466
				j++
//line common/common.w:467
			}
//line common/common.w:468
			i = j
//line common/common.w:469
		default:
//line common/common.w:470
			i += 2
//line common/common.w:471
		}
//line common/common.w:472
	}
//line common/common.w:473
	return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:474
}

//line common/common.w:480
func findNextSection(src string, i int) ctrl {
//line common/common.w:481
	n := len(src)
//line common/common.w:482
	for i < n {
//line common/common.w:483
		if src[i] != '@' {
//line common/common.w:484
			i++
//line common/common.w:485
			continue
//line common/common.w:486
		}
//line common/common.w:487
		if i+1 >= n {
//line common/common.w:488
			break
//line common/common.w:489
		}
//line common/common.w:490
		switch c := src[i+1]; {
//line common/common.w:491
		case c == '@':
//line common/common.w:492
			i += 2
//line common/common.w:493
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line common/common.w:494
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line common/common.w:495
		case c == '*':
//line common/common.w:496

//line common/common.w:521
			j := i + 2
//line common/common.w:522
			depth := 0
//line common/common.w:523
			if j < n && src[j] == '*' {
//line common/common.w:524
				j++
//line common/common.w:525
				depth = -1
//line common/common.w:526
			} else {
//line common/common.w:527
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:528
					depth = depth*10 + int(src[j]-'0')
//line common/common.w:529
					j++
//line common/common.w:530
				}
//line common/common.w:531
			}
//line common/common.w:532
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}

//line common/common.w:497
		case c == '<' || c == '(' || c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line common/common.w:498
			end := indexFrom(src, "@>", i+2)
//line common/common.w:499
			if end < 0 {
//line common/common.w:500
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:501
			}
//line common/common.w:502
			i = end + 2
//line common/common.w:503
		case c == '%':
//line common/common.w:504
			j := i + 2
//line common/common.w:505
			for j < n && src[j] != '\n' {
//line common/common.w:506
				j++
//line common/common.w:507
			}
//line common/common.w:508
			i = j
//line common/common.w:509
		default:
//line common/common.w:510
			i += 2
//line common/common.w:511
		}
//line common/common.w:512
	}
//line common/common.w:513
	return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:514
}

//line common/common.w:559
func parse(src string) *Web {
//line common/common.w:560
	w := &Web{}
//line common/common.w:561
	n := len(src)

//line common/common.w:563
	first := findNextSection(src, 0)
//line common/common.w:564
	w.Limbo, w.Formats = extractLimboFormats(src[:first.pos])
//line common/common.w:565
	i := first.pos

//line common/common.w:567
	num := 0
//line common/common.w:568
	for i < n {
//line common/common.w:569
		// We are positioned at a section break.
//line common/common.w:570
		hdr := src[i+1]
//line common/common.w:571
		num++
//line common/common.w:572
		sec := &Section{Number: num, Line: lineAt(src, i)}
//line common/common.w:573
		if hdr == '*' {
//line common/common.w:574
			h := findSectionHeaderEnd(src, i)
//line common/common.w:575
			sec.Starred = true
//line common/common.w:576
			sec.Depth = h.depth
//line common/common.w:577
			i = h.end
//line common/common.w:578
		} else {
//line common/common.w:579
			i += 2
//line common/common.w:580
		}

//line common/common.w:582
		// \TEX/ part: from here to the next structural control.
//line common/common.w:583
		ct := scanStruct(src, i)
//line common/common.w:584
		sec.Tex = src[i:ct.pos]
//line common/common.w:585
		if sec.Starred {
//line common/common.w:586
			sec.Title = extractTitle(sec.Tex)
//line common/common.w:587
		}

//line common/common.w:589

//line common/common.w:607
		for ct.kind == cDefn || ct.kind == cFormat {
//line common/common.w:608
			nx := scanStruct(src, ct.end)
//line common/common.w:609
			seg := src[ct.end:nx.pos]
//line common/common.w:610
			if ct.kind == cDefn {
//line common/common.w:611
				sec.Formats = append(sec.Formats, parseMacro(seg)...)
//line common/common.w:612
			} else if f, ok := parseFormat(seg, ct.noIndex); ok {
//line common/common.w:613
				sec.Formats = append(sec.Formats, f)
//line common/common.w:614
			}
//line common/common.w:615
			ct = nx
//line common/common.w:616
		}

//line common/common.w:590

//line common/common.w:622
		switch ct.kind {
//line common/common.w:623
		case cCode:
//line common/common.w:624
			sec.HasCode = true
//line common/common.w:625
			sec.CodeLine = lineAt(src, ct.end)
//line common/common.w:626
			nx := findNextSection(src, ct.end)
//line common/common.w:627
			sec.Code = src[ct.end:nx.pos]
//line common/common.w:628
			i = nx.pos
//line common/common.w:629
		case cNamed:
//line common/common.w:630
			sec.HasCode = true
//line common/common.w:631
			sec.Name = ct.name
//line common/common.w:632
			sec.IsFile = ct.isFile
//line common/common.w:633
			sec.CodeLine = lineAt(src, ct.end)
//line common/common.w:634
			nx := findNextSection(src, ct.end)
//line common/common.w:635
			sec.Code = src[ct.end:nx.pos]
//line common/common.w:636
			i = nx.pos
//line common/common.w:637
		default: // |cSection| or |cEOF|: a documentation-only section
//line common/common.w:638
			i = ct.pos
//line common/common.w:639
		}

//line common/common.w:592
		w.Sections = append(w.Sections, sec)
//line common/common.w:593
		if ct.kind == cEOF && sec.Code == "" {
//line common/common.w:594
			break
//line common/common.w:595
		}
//line common/common.w:596
		if i >= n {
//line common/common.w:597
			break
//line common/common.w:598
		}
//line common/common.w:599
	}
//line common/common.w:600
	return w
//line common/common.w:601
}

//line common/common.w:643
func findSectionHeaderEnd(src string, i int) ctrl {
//line common/common.w:644
	n := len(src)
//line common/common.w:645
	j := i + 2
//line common/common.w:646
	depth := 0
//line common/common.w:647
	if j < n && src[j] == '*' {
//line common/common.w:648
		j++
//line common/common.w:649
		depth = -1 // ``\.{@**}'' is the top level: bold in the contents, as \.{CWEB}
//line common/common.w:650
	} else {
//line common/common.w:651
		for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:652
			depth = depth*10 + int(src[j]-'0')
//line common/common.w:653
			j++
//line common/common.w:654
		}
//line common/common.w:655
	}
//line common/common.w:656
	return ctrl{end: j, depth: depth}
//line common/common.w:657
}

//line common/common.w:662
func extractTitle(tex string) string {
//line common/common.w:663
	t := strings.TrimLeft(tex, " \t\n")
//line common/common.w:664
	if i := titleEnd(t); i >= 0 {
//line common/common.w:665
		t = t[:i]
//line common/common.w:666
	}
//line common/common.w:667
	return strings.Join(strings.Fields(t), " ")
//line common/common.w:668
}

//line common/common.w:670
func titleEnd(s string) int {
//line common/common.w:671
	for i := 0; i < len(s); i++ {
//line common/common.w:672
		if s[i] == '.' && (i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t' ||
//line common/common.w:673
			s[i+1] == '\n' || s[i+1] == '\r') {
//line common/common.w:674
			return i
//line common/common.w:675
		}
//line common/common.w:676
	}
//line common/common.w:677
	return -1
//line common/common.w:678
}

//line common/common.w:683
func (w *Web) scanDiagnostics(src string) []string {
//line common/common.w:684
	var warns []string
//line common/common.w:685
	n := len(src)
//line common/common.w:686
	i := 0
//line common/common.w:687
	for i < n {
//line common/common.w:688
		if src[i] != '@' || i+1 >= n {
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
			i += 2
//line common/common.w:695
		case '<', '(', '=', 't', '^', '.', ':', 'q':
//line common/common.w:696
			if end := indexFrom(src, "@>", i+2); end < 0 {
//line common/common.w:697
				warns = append(warns, fmt.Sprintf("%s: unterminated `@%c ... @>'", w.at(lineAt(src, i)), c))
//line common/common.w:698
				i = n
//line common/common.w:699
			} else {
//line common/common.w:700
				i = end + 2
//line common/common.w:701
			}
//line common/common.w:702
		default:
//line common/common.w:703
			i += 2
//line common/common.w:704
		}
//line common/common.w:705
	}
//line common/common.w:706
	return warns
//line common/common.w:707
}

//line common/common.w:711
func parseFormat(seg string, noIndex bool) (Format, bool) {
//line common/common.w:712
	fields := strings.Fields(seg)
//line common/common.w:713
	if len(fields) < 2 {
//line common/common.w:714
		return Format{}, false
//line common/common.w:715
	}
//line common/common.w:716
	return Format{Original: fields[0], Like: fields[1], NoIndex: noIndex}, true
//line common/common.w:717
}

//line common/common.w:727
func parseMacro(seg string) []Format {
//line common/common.w:728
	var fs []Format
//line common/common.w:729
	for _, field := range strings.Fields(seg) {
//line common/common.w:730
		name := field
//line common/common.w:731
		if k := strings.LastIndex(name, "."); k >= 0 {
//line common/common.w:732
			name = name[k+1:]
//line common/common.w:733
		}
//line common/common.w:734
		if name != "" {
//line common/common.w:735
			fs = append(fs, Format{Original: name, Macro: true})
//line common/common.w:736
		}
//line common/common.w:737
	}
//line common/common.w:738
	return fs
//line common/common.w:739
}

//line common/common.w:746
func extractLimboFormats(src string) (string, []Format) {
//line common/common.w:747
	var b strings.Builder
//line common/common.w:748
	var formats []Format
//line common/common.w:749
	n := len(src)
//line common/common.w:750
	i := 0
//line common/common.w:751
	for i < n {
//line common/common.w:752
		if src[i] != '@' || i+1 >= n {
//line common/common.w:753
			b.WriteByte(src[i])
//line common/common.w:754
			i++
//line common/common.w:755
			continue
//line common/common.w:756
		}
//line common/common.w:757
		switch c := src[i+1]; c {
//line common/common.w:758
		case '@':
//line common/common.w:759
			b.WriteString("@@")
//line common/common.w:760
			i += 2
//line common/common.w:761
		case 'd', 'f', 's':
//line common/common.w:762

//line common/common.w:795
			var fs []Format
//line common/common.w:796
			var j int
//line common/common.w:797
			if c == 'd' {
//line common/common.w:798
				j = i + 2
//line common/common.w:799
				for j < n && src[j] != '@' {
//line common/common.w:800
					j++ // the body runs to the next control code
//line common/common.w:801
				}
//line common/common.w:802
				fs = parseMacro(src[i+2 : j])
//line common/common.w:803
			} else {
//line common/common.w:804
				j = endOfFormatArgs(src, i+2, n)
//line common/common.w:805
				if f, ok := parseFormat(src[i+2:j], c == 's'); ok {
//line common/common.w:806
					fs = []Format{f}
//line common/common.w:807
				}
//line common/common.w:808
			}
//line common/common.w:809
			formats = append(formats, fs...)
//line common/common.w:810
			if k := skipBlanks(src, j, n); k < n && src[k] == '\n' {
//line common/common.w:811
				j = k + 1 // the directive ended its line; drop the blanks and the newline
//line common/common.w:812
			}
//line common/common.w:813
			i = j

//line common/common.w:763
		case 'q':
//line common/common.w:764
			if end := indexFrom(src, "@>", i+2); end < 0 {
//line common/common.w:765
				i = n // unterminated: drop the rest of limbo
//line common/common.w:766
			} else {
//line common/common.w:767
				i = end + 2 // drop the source-only comment
//line common/common.w:768
			}
//line common/common.w:769
		case '<', '(', '=', 't', '^', '.', ':':
//line common/common.w:770
			end := indexFrom(src, "@>", i+2)
//line common/common.w:771
			if end < 0 {
//line common/common.w:772
				b.WriteString(src[i:])
//line common/common.w:773
				i = n
//line common/common.w:774
			} else {
//line common/common.w:775
				b.WriteString(src[i : end+2])
//line common/common.w:776
				i = end + 2
//line common/common.w:777
			}
//line common/common.w:778
		default:
//line common/common.w:779
			b.WriteString(src[i : i+2])
//line common/common.w:780
			i += 2
//line common/common.w:781
		}
//line common/common.w:782
	}
//line common/common.w:783
	return b.String(), formats
//line common/common.w:784
}

//line common/common.w:819
func endOfFormatArgs(src string, p, n int) int {
//line common/common.w:820
	for word := 0; word < 2; word++ {
//line common/common.w:821
		p = skipBlanks(src, p, n)
//line common/common.w:822
		for p < n && src[p] != ' ' && src[p] != '\t' && src[p] != '\n' {
//line common/common.w:823
			p++
//line common/common.w:824
		}
//line common/common.w:825
	}
//line common/common.w:826
	return p
//line common/common.w:827
}

//line common/common.w:829
func skipBlanks(src string, p, n int) int {
//line common/common.w:830
	for p < n && (src[p] == ' ' || src[p] == '\t') {
//line common/common.w:831
		p++
//line common/common.w:832
	}
//line common/common.w:833
	return p
//line common/common.w:834
}

//line common/common.w:845
type AtomKind int

//line common/common.w:847
const (
//line common/common.w:848
	AText AtomKind = iota // ordinary \GO/ source text
//line common/common.w:849
	ARef // \.{@<name@>} reference to a named section
//line common/common.w:850
	AVerbatim // \.{@=text@>} passed verbatim to tangled output
//line common/common.w:851
	ATeX // \.{@t text@>} \TEX/ text for the woven output
//line common/common.w:852
	AIndex // \.{@\^/@./@}: index entry
//line common/common.w:853
	APaste // \.{@\&} join (delete surrounding whitespace)
//line common/common.w:854
	ALayout // \.{@}, \.{@/} \.{@|} \.{@\#} woven-output layout hints
//line common/common.w:855
	AIndexDef // \.{@!} force the next identifier to index as a definition
//line common/common.w:856
)

//line common/common.w:858
type Atom struct {
//line common/common.w:859
	Kind AtomKind
//line common/common.w:860
	Text string // payload for |AText|/|AVerbatim|/|ATeX|/|AIndex|; name for |ARef|
//line common/common.w:861
	Index byte // '\.{\^}','\.{.}','\.{:}' for AIndex; '\.{,}' '\.{/}' '\.{|}' '\.{\#}' for |ALayout|
//line common/common.w:862
}

//line common/common.w:868
func ScanCode(code string) []Atom {
//line common/common.w:869
	var atoms []Atom
//line common/common.w:870
	var buf strings.Builder
//line common/common.w:871
	flush := func() {
//line common/common.w:872
		if buf.Len() > 0 {
//line common/common.w:873
			atoms = append(atoms, Atom{Kind: AText, Text: buf.String()})
//line common/common.w:874
			buf.Reset()
//line common/common.w:875
		}
//line common/common.w:876
	}
//line common/common.w:877
	n := len(code)
//line common/common.w:878
	i := 0
//line common/common.w:879
	for i < n {
//line common/common.w:880
		c := code[i]
//line common/common.w:881
		if c != '@' || i+1 >= n {
//line common/common.w:882
			buf.WriteByte(c)
//line common/common.w:883
			i++
//line common/common.w:884
			continue
//line common/common.w:885
		}
//line common/common.w:886

//line common/common.w:898
		switch d := code[i+1]; d {

//line common/common.w:908
		case '@':
//line common/common.w:909
			buf.WriteByte('@')
//line common/common.w:910
			i += 2
//line common/common.w:911
		case '&':
//line common/common.w:912
			flush()
//line common/common.w:913
			atoms = append(atoms, Atom{Kind: APaste})
//line common/common.w:914
			i += 2

//line common/common.w:921
		case '<':
//line common/common.w:922
			end := indexFrom(code, "@>", i+2)
//line common/common.w:923
			if end < 0 {
//line common/common.w:924
				buf.WriteString(code[i:])
//line common/common.w:925
				i = n
//line common/common.w:926
				continue
//line common/common.w:927
			}
//line common/common.w:928
			flush()
//line common/common.w:929
			atoms = append(atoms, Atom{Kind: ARef, Text: canonName(code[i+2 : end])})
//line common/common.w:930
			i = end + 2
//line common/common.w:931
		case '=':
//line common/common.w:932
			end := indexFrom(code, "@>", i+2)
//line common/common.w:933
			if end < 0 {
//line common/common.w:934
				i = n
//line common/common.w:935
				continue
//line common/common.w:936
			}
//line common/common.w:937
			flush()
//line common/common.w:938
			atoms = append(atoms, Atom{Kind: AVerbatim, Text: code[i+2 : end]})
//line common/common.w:939
			i = end + 2
//line common/common.w:940
		case 't':
//line common/common.w:941
			end := indexFrom(code, "@>", i+2)
//line common/common.w:942
			if end < 0 {
//line common/common.w:943
				i = n
//line common/common.w:944
				continue
//line common/common.w:945
			}
//line common/common.w:946
			flush()
//line common/common.w:947
			atoms = append(atoms, Atom{Kind: ATeX, Text: code[i+2 : end]})
//line common/common.w:948
			i = end + 2
//line common/common.w:949
		case '^', '.', ':':
//line common/common.w:950
			end := indexFrom(code, "@>", i+2)
//line common/common.w:951
			if end < 0 {
//line common/common.w:952
				i = n
//line common/common.w:953
				continue
//line common/common.w:954
			}
//line common/common.w:955
			flush()
//line common/common.w:956
			atoms = append(atoms, Atom{Kind: AIndex, Text: code[i+2 : end], Index: d})
//line common/common.w:957
			i = end + 2
//line common/common.w:958
		case 'q':
//line common/common.w:959
			end := indexFrom(code, "@>", i+2)
//line common/common.w:960
			if end < 0 {
//line common/common.w:961
				i = n
//line common/common.w:962
				continue
//line common/common.w:963
			}
//line common/common.w:964
			i = end + 2 // ignored material

//line common/common.w:977
		case '%':
//line common/common.w:978
			j := i + 2
//line common/common.w:979
			for j < n && code[j] != '\n' {
//line common/common.w:980
				j++
//line common/common.w:981
			}
//line common/common.w:982
			i = j
//line common/common.w:983
		case '>':
//line common/common.w:984
			i += 2 // stray terminator
//line common/common.w:985
		case ',', '/', '|', '#':
//line common/common.w:986
			flush()
//line common/common.w:987
			atoms = append(atoms, Atom{Kind: ALayout, Index: d})
//line common/common.w:988
			i += 2
//line common/common.w:989
		case '!':
//line common/common.w:990
			flush()
//line common/common.w:991
			atoms = append(atoms, Atom{Kind: AIndexDef})
//line common/common.w:992
			i += 2
//line common/common.w:993
		case '+', '[', ']', ';':
//line common/common.w:994
			i += 2 // \.{CWEB} prettyprinter hints, dropped
//line common/common.w:995
		default:
//line common/common.w:996
			i += 2 // unknown \.{@x}: drop it rather than corrupt the output

//line common/common.w:902
		}

//line common/common.w:887
	}
//line common/common.w:888
	flush()
//line common/common.w:889
	return atoms
//line common/common.w:890
}

//line common/common.w:1018
type change struct {
//line common/common.w:1019
	match []string // lines to find in the master source
//line common/common.w:1020
	repl []string // lines to substitute for them
//line common/common.w:1021
	line int // 1-based line of the \.{@x} in the change file (for diagnostics)
//line common/common.w:1022
	replLine int // 1-based change-file line of the first replacement line
//line common/common.w:1023
}

//line common/common.w:1025
type srcLoc struct {
//line common/common.w:1026
	file string
//line common/common.w:1027
	line int
//line common/common.w:1028
}

//line common/common.w:1030
func (l srcLoc) String() string {
//line common/common.w:1031
	if l.file == "" {
//line common/common.w:1032
		return fmt.Sprintf("line %d", l.line)
//line common/common.w:1033
	}
//line common/common.w:1034
	return fmt.Sprintf("%s:%d", l.file, l.line)
//line common/common.w:1035
}

//line common/common.w:1040
func isChangeCtrl(line string, c byte) bool {
//line common/common.w:1041
	return len(line) >= 2 && line[0] == '@' && line[1] == c
//line common/common.w:1042
}

//line common/common.w:1044
func splitLines(s string) []string {
//line common/common.w:1045
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
//line common/common.w:1046
}

//line common/common.w:1048
func sameLine(a, b string) bool {
//line common/common.w:1049
	return strings.TrimRight(a, " \t") == strings.TrimRight(b, " \t")
//line common/common.w:1050
}

//line common/common.w:1055
func parseChangeFile(src string) ([]change, error) {
//line common/common.w:1056
	lines := splitLines(src)
//line common/common.w:1057
	var changes []change
//line common/common.w:1058
	n := len(lines)
//line common/common.w:1059
	for i := 0; i < n; {
//line common/common.w:1060
		if !isChangeCtrl(lines[i], 'x') {
//line common/common.w:1061
			i++ // commentary between changes
//line common/common.w:1062
			continue
//line common/common.w:1063
		}
//line common/common.w:1064
		c := change{line: i + 1}
//line common/common.w:1065
		i++
//line common/common.w:1066

//line common/common.w:1080
		for i < n && !isChangeCtrl(lines[i], 'y') {
//line common/common.w:1081
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'z') {
//line common/common.w:1082
				return nil, fmt.Errorf("change file line %d: expected @y to close the @x match part", c.line)
//line common/common.w:1083
			}
//line common/common.w:1084
			c.match = append(c.match, lines[i])
//line common/common.w:1085
			i++
//line common/common.w:1086
		}
//line common/common.w:1087
		if i >= n {
//line common/common.w:1088
			return nil, fmt.Errorf("change file line %d: @x without a matching @y", c.line)
//line common/common.w:1089
		}
//line common/common.w:1090
		i++ // skip \.{@y}
//line common/common.w:1091
		c.replLine = i + 1

//line common/common.w:1067

//line common/common.w:1096
		for i < n && !isChangeCtrl(lines[i], 'z') {
//line common/common.w:1097
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'y') {
//line common/common.w:1098
				return nil, fmt.Errorf("change file line %d: expected @z to close the change", c.line)
//line common/common.w:1099
			}
//line common/common.w:1100
			c.repl = append(c.repl, lines[i])
//line common/common.w:1101
			i++
//line common/common.w:1102
		}
//line common/common.w:1103
		if i >= n {
//line common/common.w:1104
			return nil, fmt.Errorf("change file line %d: change has no @z", c.line)
//line common/common.w:1105
		}
//line common/common.w:1106
		i++ // skip \.{@z}

//line common/common.w:1068
		if len(c.match) == 0 {
//line common/common.w:1069
			return nil, fmt.Errorf("change file line %d: the @x match part is empty", c.line)
//line common/common.w:1070
		}
//line common/common.w:1071
		changes = append(changes, c)
//line common/common.w:1072
	}
//line common/common.w:1073
	return changes, nil
//line common/common.w:1074
}

//line common/common.w:1110
func applyChanges(src string, changes []change, chFile string) (string, error) {
//line common/common.w:1111
	out, _, err := applyChangesMapped(splitLines(src), nil, changes, chFile)
//line common/common.w:1112
	if err != nil {
//line common/common.w:1113
		return "", err
//line common/common.w:1114
	}
//line common/common.w:1115
	return strings.Join(out, "\n"), nil
//line common/common.w:1116
}

//line common/common.w:1123
func applyChangesMapped(master []string, locs []srcLoc, changes []change, chFile string) (
//line common/common.w:1124
	[]string, []srcLoc, error,
//line common/common.w:1125
) {
//line common/common.w:1126
	loc := func(i int) srcLoc {
//line common/common.w:1127
		if locs != nil && i < len(locs) {
//line common/common.w:1128
			return locs[i]
//line common/common.w:1129
		}
//line common/common.w:1130
		return srcLoc{line: i + 1}
//line common/common.w:1131
	}
//line common/common.w:1132
	out := make([]string, 0, len(master))
//line common/common.w:1133
	var outLocs []srcLoc
//line common/common.w:1134
	ci := 0
//line common/common.w:1135
	for i := 0; i < len(master); {
//line common/common.w:1136
		if ci < len(changes) && sameLine(master[i], changes[ci].match[0]) {
//line common/common.w:1137
			if !blockMatches(master, i, changes[ci].match) {
//line common/common.w:1138
				return nil, nil, fmt.Errorf("%s:%d: change did not match the master source at %s",
//line common/common.w:1139
					chFile, changes[ci].line, loc(i))
//line common/common.w:1140
			}
//line common/common.w:1141
			for r, rl := range changes[ci].repl {
//line common/common.w:1142
				out = append(out, rl)
//line common/common.w:1143
				outLocs = append(outLocs, srcLoc{chFile, changes[ci].replLine + r})
//line common/common.w:1144
			}
//line common/common.w:1145
			i += len(changes[ci].match)
//line common/common.w:1146
			ci++
//line common/common.w:1147
			continue
//line common/common.w:1148
		}
//line common/common.w:1149
		out = append(out, master[i])
//line common/common.w:1150
		outLocs = append(outLocs, loc(i))
//line common/common.w:1151
		i++
//line common/common.w:1152
	}
//line common/common.w:1153
	if ci < len(changes) {
//line common/common.w:1154
		return nil, nil, fmt.Errorf("%s:%d: change was never matched (looking for %q)",
//line common/common.w:1155
			chFile, changes[ci].line, changes[ci].match[0])
//line common/common.w:1156
	}
//line common/common.w:1157
	return out, outLocs, nil
//line common/common.w:1158
}

//line common/common.w:1163
func blockMatches(master []string, at int, match []string) bool {
//line common/common.w:1164
	if at+len(match) > len(master) {
//line common/common.w:1165
		return false
//line common/common.w:1166
	}
//line common/common.w:1167
	for k, m := range match {
//line common/common.w:1168
		if !sameLine(master[at+k], m) {
//line common/common.w:1169
			return false
//line common/common.w:1170
		}
//line common/common.w:1171
	}
//line common/common.w:1172
	return true
//line common/common.w:1173
}
