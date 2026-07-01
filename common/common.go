//line common/common.w:15
package common

//line common/common.w:17
import (
//line common/common.w:18
	"fmt"
//line common/common.w:19
	"os"
//line common/common.w:20
	"path/filepath"
//line common/common.w:21
	"strings"
//line common/common.w:22
)

//line common/common.w:24
const Version = "0.3.0"

//line common/common.w:26
type Format struct {
//line common/common.w:27
	Original string
//line common/common.w:28
	Like string
//line common/common.w:29
	NoIndex bool
//line common/common.w:30
	Macro bool // \.{@d}: typeset Original in typewriter (a \.{CWEB}-style macro)
//line common/common.w:31
}

//line common/common.w:37
type Section struct {
//line common/common.w:38
	Number int // 1-based section number
//line common/common.w:39
	Line int // 1-based source line where the section begins
//line common/common.w:40
	Starred bool // true for \.{@*} sections
//line common/common.w:41
	Depth int // group depth for starred sections (-1 |==| \.{@**}, 0 |==| \.{@*}, n |==| \.{@*n})
//line common/common.w:42
	Title string // starred-section title (text up to the first period)
//line common/common.w:43
	Tex string // commentary, raw \TEX/ with in-text\.{@}-codes still embedded
//line common/common.w:44
	Formats []Format
//line common/common.w:45
	HasCode bool // true if the section contributes code
//line common/common.w:46
	Name string // named-section name, or \.{""} for an unnamed @c section
//line common/common.w:47
	IsFile bool // true if the name is an output file (\.{@(file@>=})
//line common/common.w:48
	Code string // raw code text with in-code \.{@}-codes still embedded
//line common/common.w:49
	CodeLine int // 1-based combined-source line where Code begins (0 if none)
//line common/common.w:50
}

//line common/common.w:57
type Web struct {
//line common/common.w:58
	Limbo string
//line common/common.w:59
	Formats []Format // \.{@f}/\.{@s} directives found in limbo (apply globally)
//line common/common.w:60
	Sections []*Section
//line common/common.w:61
	Warnings []string // non-fatal diagnostics gathered while parsing/checking
//line common/common.w:62
	file string // source filename, for diagnostics (\.{""} if unknown)
//line common/common.w:63
	locs []srcLoc // origin (file, line) of each combined-source line
//line common/common.w:64
	full []string // canonical (non-abbreviated) section names
//line common/common.w:65
}

//line common/common.w:72
func Parse(filename string) (*Web, error) {
//line common/common.w:73
	return ParseWithChange(filename, "")
//line common/common.w:74
}

//line common/common.w:76
func ParseWithChange(filename, changeFile string) (*Web, error) {
//line common/common.w:77
	lines, locs, err := expandIncludes(filename, 0)
//line common/common.w:78
	if err != nil {
//line common/common.w:79
		return nil, err
//line common/common.w:80
	}
//line common/common.w:81
	if changeFile != "" {
//line common/common.w:82
		chData, err := os.ReadFile(changeFile)
//line common/common.w:83
		if err != nil {
//line common/common.w:84
			return nil, err
//line common/common.w:85
		}
//line common/common.w:86
		changes, err := parseChangeFile(string(chData))
//line common/common.w:87
		if err != nil {
//line common/common.w:88
			return nil, err
//line common/common.w:89
		}
//line common/common.w:90
		lines, locs, err = applyChangesMapped(lines, locs, changes, changeFile)
//line common/common.w:91
		if err != nil {
//line common/common.w:92
			return nil, err
//line common/common.w:93
		}
//line common/common.w:94
	}
//line common/common.w:95
	src := strings.Join(lines, "\n")
//line common/common.w:96
	w := parse(src)
//line common/common.w:97
	w.file = filename
//line common/common.w:98
	w.locs = locs
//line common/common.w:99
	w.finish(src)
//line common/common.w:100
	return w, nil
//line common/common.w:101
}

//line common/common.w:104
func ParseString(src string) *Web {
//line common/common.w:105
	w := parse(src)
//line common/common.w:106
	w.finish(src)
//line common/common.w:107
	return w
//line common/common.w:108
}

//line common/common.w:110
func (w *Web) finish(src string) {
//line common/common.w:111
	w.collectNames()
//line common/common.w:112
	w.Warnings = append(w.Warnings, w.scanDiagnostics(src)...)
//line common/common.w:113
	w.Warnings = append(w.Warnings, w.checkNames()...)
//line common/common.w:114
}

//line common/common.w:116
func (w *Web) at(line int) string {
//line common/common.w:117
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line common/common.w:118
		return w.locs[i].String()
//line common/common.w:119
	}
//line common/common.w:120
	if w.file != "" {
//line common/common.w:121
		return fmt.Sprintf("%s:%d", w.file, line)
//line common/common.w:122
	}
//line common/common.w:123
	return fmt.Sprintf("line %d", line)
//line common/common.w:124
}

//line common/common.w:131
func (w *Web) Origin(line int) (file string, ln int) {
//line common/common.w:132
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line common/common.w:133
		return w.locs[i].file, w.locs[i].line
//line common/common.w:134
	}
//line common/common.w:135
	return w.file, line
//line common/common.w:136
}

//line common/common.w:142
func DefaultExt(name, ext string) string {
//line common/common.w:143
	if name == "" || filepath.Ext(name) != "" {
//line common/common.w:144
		return name
//line common/common.w:145
	}
//line common/common.w:146
	return name + ext
//line common/common.w:147
}

//line common/common.w:153
func expandIncludes(file string, depth int) ([]string, []srcLoc, error) {
//line common/common.w:154
	if depth > 25 {
//line common/common.w:155
		return nil, nil, fmt.Errorf("gweb: @i include nesting too deep at %q", file)
//line common/common.w:156
	}
//line common/common.w:157
	data, err := os.ReadFile(file)
//line common/common.w:158
	if err != nil {
//line common/common.w:159
		return nil, nil, err
//line common/common.w:160
	}
//line common/common.w:161
	raw := splitLines(string(data))
//line common/common.w:162
	if n := len(raw); n > 0 && raw[n-1] == "" {
//line common/common.w:163
		raw = raw[:n-1]
//line common/common.w:164
	}

//line common/common.w:166
	var lines []string
//line common/common.w:167
	var locs []srcLoc
//line common/common.w:168
	dir := filepath.Dir(file)
//line common/common.w:169
	for i, line := range raw {
//line common/common.w:170
		if name, ok := includeDirective(line); ok {
//line common/common.w:171
			path := name
//line common/common.w:172
			if !filepath.IsAbs(path) {
//line common/common.w:173
				path = filepath.Join(dir, name)
//line common/common.w:174
			}
//line common/common.w:175
			sub, subLocs, err := expandIncludes(path, depth+1)
//line common/common.w:176
			if err != nil {
//line common/common.w:177
				return nil, nil, fmt.Errorf("%s:%d: %w", file, i+1, err)
//line common/common.w:178
			}
//line common/common.w:179
			lines = append(lines, sub...)
//line common/common.w:180
			locs = append(locs, subLocs...)
//line common/common.w:181
			continue
//line common/common.w:182
		}
//line common/common.w:183
		lines = append(lines, line)
//line common/common.w:184
		locs = append(locs, srcLoc{file, i + 1})
//line common/common.w:185
	}
//line common/common.w:186
	return lines, locs, nil
//line common/common.w:187
}

//line common/common.w:190
func includeDirective(line string) (name string, ok bool) {
//line common/common.w:191
	t := strings.TrimLeft(line, " \t")
//line common/common.w:192
	if !strings.HasPrefix(t, "@i") {
//line common/common.w:193
		return "", false
//line common/common.w:194
	}
//line common/common.w:195
	rest := t[2:]
//line common/common.w:196
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
//line common/common.w:197
		return "", false
//line common/common.w:198
	}
//line common/common.w:199
	name = strings.Trim(strings.TrimSpace(rest), "\"")
//line common/common.w:200
	return name, name != ""
//line common/common.w:201
}

//line common/common.w:208
func (w *Web) collectNames() {
//line common/common.w:209
	seen := map[string]bool{}
//line common/common.w:210
	add := func(name string) {
//line common/common.w:211
		if name != "" && !strings.HasSuffix(name, "...") && !seen[name] {
//line common/common.w:212
			seen[name] = true
//line common/common.w:213
			w.full = append(w.full, name)
//line common/common.w:214
		}
//line common/common.w:215
	}
//line common/common.w:216
	for _, s := range w.Sections {
//line common/common.w:217
		if !s.IsFile {
//line common/common.w:218
			add(s.Name) // a definition's name
//line common/common.w:219
		}
//line common/common.w:220
		for _, raw := range []string{s.Code, s.Tex} {
//line common/common.w:221
			for _, a := range ScanCode(raw) {
//line common/common.w:222
				if a.Kind == ARef {
//line common/common.w:223
					add(a.Text) // a reference's name
//line common/common.w:224
				}
//line common/common.w:225
			}
//line common/common.w:226
		}
//line common/common.w:227
	}
//line common/common.w:228
}

//line common/common.w:230
func (w *Web) prefixMatches(prefix string) int {
//line common/common.w:231
	n := 0
//line common/common.w:232
	for _, full := range w.full {
//line common/common.w:233
		if strings.HasPrefix(full, prefix) {
//line common/common.w:234
			n++
//line common/common.w:235
		}
//line common/common.w:236
	}
//line common/common.w:237
	return n
//line common/common.w:238
}

//line common/common.w:245
func (w *Web) checkNames() []string {
//line common/common.w:246
	// |defined| is the set of sections that actually have a definition (not just
//line common/common.w:247
	// the full names known for abbreviation resolution, which include references).
//line common/common.w:248
	defined := map[string]bool{}
//line common/common.w:249
	for _, s := range w.Sections {
//line common/common.w:250
		if s.Name != "" && !s.IsFile {
//line common/common.w:251
			defined[w.Resolve(s.Name)] = true
//line common/common.w:252
		}
//line common/common.w:253
	}
//line common/common.w:254
	used := map[string]bool{}
//line common/common.w:255
	var warns []string

//line common/common.w:257
	for _, s := range w.Sections {
//line common/common.w:258

//line common/common.w:278
		scan := func(raw string) {
//line common/common.w:279
			for _, a := range ScanCode(raw) {
//line common/common.w:280
				if a.Kind != ARef {
//line common/common.w:281
					continue
//line common/common.w:282
				}
//line common/common.w:283
				canon := w.Resolve(a.Text)
//line common/common.w:284
				if strings.HasSuffix(a.Text, "...") && canon == a.Text {
//line common/common.w:285
					prefix := strings.TrimSpace(strings.TrimSuffix(a.Text, "..."))
//line common/common.w:286
					if m := w.prefixMatches(prefix); m == 0 {
//line common/common.w:287
						warns = append(warns, fmt.Sprintf("%s: no section name matches <%s>", w.at(s.Line), a.Text))
//line common/common.w:288
					} else {
//line common/common.w:289
						warns = append(warns, fmt.Sprintf("%s: ambiguous prefix <%s> matches %d section names", w.at(s.Line), a.Text, m))
//line common/common.w:290
					}
//line common/common.w:291
					continue
//line common/common.w:292
				}
//line common/common.w:293
				if !defined[canon] {
//line common/common.w:294
					warns = append(warns, fmt.Sprintf("%s: reference to undefined section <%s>", w.at(s.Line), a.Text))
//line common/common.w:295
				}
//line common/common.w:296
				used[canon] = true
//line common/common.w:297
			}
//line common/common.w:298
		}

//line common/common.w:259
		scan(s.Code)
//line common/common.w:260
		scan(s.Tex)
//line common/common.w:261
	}

//line common/common.w:263
	warned := map[string]bool{}
//line common/common.w:264
	for _, s := range w.Sections {
//line common/common.w:265
		if s.Name == "" || s.IsFile {
//line common/common.w:266
			continue
//line common/common.w:267
		}
//line common/common.w:268
		canon := w.Resolve(s.Name)
//line common/common.w:269
		if !used[canon] && !warned[canon] {
//line common/common.w:270
			warned[canon] = true
//line common/common.w:271
			warns = append(warns, fmt.Sprintf("%s: section <%s> is defined but never used", w.at(s.Line), s.Name))
//line common/common.w:272
		}
//line common/common.w:273
	}
//line common/common.w:274
	return warns
//line common/common.w:275
}

//line common/common.w:305
func lineAt(src string, off int) int {
//line common/common.w:306
	if off > len(src) {
//line common/common.w:307
		off = len(src)
//line common/common.w:308
	}
//line common/common.w:309
	return 1 + strings.Count(src[:off], "\n")
//line common/common.w:310
}

//line common/common.w:312
func canonName(name string) string {
//line common/common.w:313
	return strings.Join(strings.Fields(name), " ")
//line common/common.w:314
}

//line common/common.w:317
func (w *Web) Resolve(name string) string {
//line common/common.w:318
	name = canonName(name)
//line common/common.w:319
	if !strings.HasSuffix(name, "...") {
//line common/common.w:320
		return name
//line common/common.w:321
	}
//line common/common.w:322
	prefix := strings.TrimSpace(strings.TrimSuffix(name, "..."))
//line common/common.w:323
	var match string
//line common/common.w:324
	count := 0
//line common/common.w:325
	for _, full := range w.full {
//line common/common.w:326
		if strings.HasPrefix(full, prefix) {
//line common/common.w:327
			match = full
//line common/common.w:328
			count++
//line common/common.w:329
		}
//line common/common.w:330
	}
//line common/common.w:331
	if count == 1 {
//line common/common.w:332
		return match
//line common/common.w:333
	}
//line common/common.w:334
	return name // unresolved or ambiguous; leave as-is for caller to report
//line common/common.w:335
}

//line common/common.w:337
func indexFrom(s, sub string, from int) int {
//line common/common.w:338
	if from >= len(s) {
//line common/common.w:339
		return -1
//line common/common.w:340
	}
//line common/common.w:341
	idx := strings.Index(s[from:], sub)
//line common/common.w:342
	if idx < 0 {
//line common/common.w:343
		return -1
//line common/common.w:344
	}
//line common/common.w:345
	return from + idx
//line common/common.w:346
}

//line common/common.w:353
type ctrlKind int

//line common/common.w:355
const (
//line common/common.w:356
	cEOF ctrlKind = iota
//line common/common.w:357
	cSection
//line common/common.w:358
	cCode // \.{@c} (or its synonym \.{@p})
//line common/common.w:359
	cNamed // \.{@<name@>=} or \.{@(file@>=}
//line common/common.w:360
	cDefn // \.{@d}
//line common/common.w:361
	cFormat

//line common/common.w:362
)

//line common/common.w:364
type ctrl struct {
//line common/common.w:365
	kind ctrlKind
//line common/common.w:366
	pos int // index of the leading `\.{@}'
//line common/common.w:367
	end int // index just past the control token
//line common/common.w:368
	depth int // for cSection: -1 unstarred (or \.{@**} top level), else starred depth
//line common/common.w:369
	starred bool // for cSection (distinguishes \.{@**} from an unstarred section)
//line common/common.w:370
	name string // for cNamed
//line common/common.w:371
	isFile bool // for cNamed (\.{@(} vs \.{@<})
//line common/common.w:372
	noIndex bool // for cFormat (\.{@s})
//line common/common.w:373
}

//line common/common.w:380
func scanStruct(src string, i int) ctrl {
//line common/common.w:381
	n := len(src)
//line common/common.w:382
	for i < n {
//line common/common.w:383
		if src[i] != '@' {
//line common/common.w:384
			i++
//line common/common.w:385
			continue
//line common/common.w:386
		}
//line common/common.w:387
		if i+1 >= n {
//line common/common.w:388
			break
//line common/common.w:389
		}
//line common/common.w:390
		switch c := src[i+1]; {
//line common/common.w:391
		case c == '@':
//line common/common.w:392
			i += 2
//line common/common.w:393
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line common/common.w:394
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line common/common.w:395
		case c == '*':
//line common/common.w:396
			j := i + 2
//line common/common.w:397
			depth := 0
//line common/common.w:398
			if j < n && src[j] == '*' {
//line common/common.w:399
				j++
//line common/common.w:400
				depth = -1 // ``\.{@**}" is the top level: bold in the contents, as \.{CWEB}
//line common/common.w:401
			} else {
//line common/common.w:402
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:403
					depth = depth*10 + int(src[j]-'0')
//line common/common.w:404
					j++
//line common/common.w:405
				}
//line common/common.w:406
			}
//line common/common.w:407
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}
//line common/common.w:408
		case c == 'c' || c == 'p':
//line common/common.w:409
			return ctrl{kind: cCode, pos: i, end: i + 2}
//line common/common.w:410
		case c == 'd':
//line common/common.w:411
			return ctrl{kind: cDefn, pos: i, end: i + 2}
//line common/common.w:412
		case c == 'f':
//line common/common.w:413
			return ctrl{kind: cFormat, pos: i, end: i + 2}
//line common/common.w:414
		case c == 's':
//line common/common.w:415
			return ctrl{kind: cFormat, pos: i, end: i + 2, noIndex: true}
//line common/common.w:416
		case c == '<' || c == '(':
//line common/common.w:417
			end := indexFrom(src, "@>", i+2)
//line common/common.w:418
			if end < 0 {
//line common/common.w:419
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:420
			}
//line common/common.w:421
			after := end + 2
//line common/common.w:422
			k := after
//line common/common.w:423
			for k < n && (src[k] == ' ' || src[k] == '\t') {
//line common/common.w:424
				k++
//line common/common.w:425
			}
//line common/common.w:426
			if k < n && src[k] == '=' {
//line common/common.w:427
				return ctrl{kind: cNamed, pos: i, end: k + 1,
//line common/common.w:428
					name: canonName(src[i+2 : end]), isFile: c == '('}
//line common/common.w:429
			}
//line common/common.w:430
			i = after // a reference, not a definition
//line common/common.w:431
		case c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line common/common.w:432
			end := indexFrom(src, "@>", i+2)
//line common/common.w:433
			if end < 0 {
//line common/common.w:434
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:435
			}
//line common/common.w:436
			i = end + 2
//line common/common.w:437
		case c == '%':
//line common/common.w:438
			j := i + 2
//line common/common.w:439
			for j < n && src[j] != '\n' {
//line common/common.w:440
				j++
//line common/common.w:441
			}
//line common/common.w:442
			i = j
//line common/common.w:443
		default:
//line common/common.w:444
			i += 2
//line common/common.w:445
		}
//line common/common.w:446
	}
//line common/common.w:447
	return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:448
}

//line common/common.w:454
func findNextSection(src string, i int) ctrl {
//line common/common.w:455
	n := len(src)
//line common/common.w:456
	for i < n {
//line common/common.w:457
		if src[i] != '@' {
//line common/common.w:458
			i++
//line common/common.w:459
			continue
//line common/common.w:460
		}
//line common/common.w:461
		if i+1 >= n {
//line common/common.w:462
			break
//line common/common.w:463
		}
//line common/common.w:464
		switch c := src[i+1]; {
//line common/common.w:465
		case c == '@':
//line common/common.w:466
			i += 2
//line common/common.w:467
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line common/common.w:468
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line common/common.w:469
		case c == '*':
//line common/common.w:470
			j := i + 2
//line common/common.w:471
			depth := 0
//line common/common.w:472
			if j < n && src[j] == '*' {
//line common/common.w:473
				j++
//line common/common.w:474
				depth = -1 // "\.{@**}" is the top level: bold in the contents, as \.{CWEB}
//line common/common.w:475
			} else {
//line common/common.w:476
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:477
					depth = depth*10 + int(src[j]-'0')
//line common/common.w:478
					j++
//line common/common.w:479
				}
//line common/common.w:480
			}
//line common/common.w:481
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}
//line common/common.w:482
		case c == '<' || c == '(' || c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line common/common.w:483
			end := indexFrom(src, "@>", i+2)
//line common/common.w:484
			if end < 0 {
//line common/common.w:485
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:486
			}
//line common/common.w:487
			i = end + 2
//line common/common.w:488
		case c == '%':
//line common/common.w:489
			j := i + 2
//line common/common.w:490
			for j < n && src[j] != '\n' {
//line common/common.w:491
				j++
//line common/common.w:492
			}
//line common/common.w:493
			i = j
//line common/common.w:494
		default:
//line common/common.w:495
			i += 2
//line common/common.w:496
		}
//line common/common.w:497
	}
//line common/common.w:498
	return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:499
}

//line common/common.w:507
func parse(src string) *Web {
//line common/common.w:508
	w := &Web{}
//line common/common.w:509
	n := len(src)

//line common/common.w:511
	first := findNextSection(src, 0)
//line common/common.w:512
	w.Limbo, w.Formats = extractLimboFormats(src[:first.pos])
//line common/common.w:513
	i := first.pos

//line common/common.w:515
	num := 0
//line common/common.w:516
	for i < n {
//line common/common.w:517
		// We are positioned at a section break.
//line common/common.w:518
		hdr := src[i+1]
//line common/common.w:519
		num++
//line common/common.w:520
		sec := &Section{Number: num, Line: lineAt(src, i)}
//line common/common.w:521
		if hdr == '*' {
//line common/common.w:522
			h := findSectionHeaderEnd(src, i)
//line common/common.w:523
			sec.Starred = true
//line common/common.w:524
			sec.Depth = h.depth
//line common/common.w:525
			i = h.end
//line common/common.w:526
		} else {
//line common/common.w:527
			i += 2
//line common/common.w:528
		}

//line common/common.w:530
		// \TEX/ part: from here to the next structural control.
//line common/common.w:531
		ct := scanStruct(src, i)
//line common/common.w:532
		sec.Tex = src[i:ct.pos]
//line common/common.w:533
		if sec.Starred {
//line common/common.w:534
			sec.Title = extractTitle(sec.Tex)
//line common/common.w:535
		}

//line common/common.w:537
		// Definition part: a run of \.{@d} / \.{@f} / \.{@s}.
//line common/common.w:538
		for ct.kind == cDefn || ct.kind == cFormat {
//line common/common.w:539
			nx := scanStruct(src, ct.end)
//line common/common.w:540
			seg := src[ct.end:nx.pos]
//line common/common.w:541
			// \.{@d} has no \GO/ analogue (\GO/ has no preprocessor), so it never tangles
//line common/common.w:542
			// to code; gweave uses it only to set the named identifier in
//line common/common.w:543
			// typewriter, as cweave sets a macro. \.{@f}/\.{@s} format like another word.
//line common/common.w:544
			if ct.kind == cDefn {
//line common/common.w:545
				if f, ok := parseMacro(seg); ok {
//line common/common.w:546
					sec.Formats = append(sec.Formats, f)
//line common/common.w:547
				}
//line common/common.w:548
			} else if f, ok := parseFormat(seg, ct.noIndex); ok {
//line common/common.w:549
				sec.Formats = append(sec.Formats, f)
//line common/common.w:550
			}
//line common/common.w:551
			ct = nx
//line common/common.w:552
		}

//line common/common.w:554
		switch ct.kind {
//line common/common.w:555
		case cCode:
//line common/common.w:556
			sec.HasCode = true
//line common/common.w:557
			sec.CodeLine = lineAt(src, ct.end)
//line common/common.w:558
			nx := findNextSection(src, ct.end)
//line common/common.w:559
			sec.Code = src[ct.end:nx.pos]
//line common/common.w:560
			i = nx.pos
//line common/common.w:561
		case cNamed:
//line common/common.w:562
			sec.HasCode = true
//line common/common.w:563
			sec.Name = ct.name
//line common/common.w:564
			sec.IsFile = ct.isFile
//line common/common.w:565
			sec.CodeLine = lineAt(src, ct.end)
//line common/common.w:566
			nx := findNextSection(src, ct.end)
//line common/common.w:567
			sec.Code = src[ct.end:nx.pos]
//line common/common.w:568
			i = nx.pos
//line common/common.w:569
		default: // cSection or cEOF: a documentation-only section
//line common/common.w:570
			i = ct.pos
//line common/common.w:571
		}

//line common/common.w:573
		w.Sections = append(w.Sections, sec)
//line common/common.w:574
		if ct.kind == cEOF && sec.Code == "" {
//line common/common.w:575
			break
//line common/common.w:576
		}
//line common/common.w:577
		if i >= n {
//line common/common.w:578
			break
//line common/common.w:579
		}
//line common/common.w:580
	}
//line common/common.w:581
	return w
//line common/common.w:582
}

//line common/common.w:586
func findSectionHeaderEnd(src string, i int) ctrl {
//line common/common.w:587
	n := len(src)
//line common/common.w:588
	j := i + 2
//line common/common.w:589
	depth := 0
//line common/common.w:590
	if j < n && src[j] == '*' {
//line common/common.w:591
		j++
//line common/common.w:592
		depth = -1 // ``\.{@**}'' is the top level: bold in the contents, as \.{CWEB}
//line common/common.w:593
	} else {
//line common/common.w:594
		for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:595
			depth = depth*10 + int(src[j]-'0')
//line common/common.w:596
			j++
//line common/common.w:597
		}
//line common/common.w:598
	}
//line common/common.w:599
	return ctrl{end: j, depth: depth}
//line common/common.w:600
}

//line common/common.w:605
func extractTitle(tex string) string {
//line common/common.w:606
	t := strings.TrimLeft(tex, " \t\n")
//line common/common.w:607
	if i := titleEnd(t); i >= 0 {
//line common/common.w:608
		t = t[:i]
//line common/common.w:609
	}
//line common/common.w:610
	return strings.Join(strings.Fields(t), " ")
//line common/common.w:611
}

//line common/common.w:613
func titleEnd(s string) int {
//line common/common.w:614
	for i := 0; i < len(s); i++ {
//line common/common.w:615
		if s[i] == '.' && (i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t' ||
//line common/common.w:616
			s[i+1] == '\n' || s[i+1] == '\r') {
//line common/common.w:617
			return i
//line common/common.w:618
		}
//line common/common.w:619
	}
//line common/common.w:620
	return -1
//line common/common.w:621
}

//line common/common.w:626
func (w *Web) scanDiagnostics(src string) []string {
//line common/common.w:627
	var warns []string
//line common/common.w:628
	n := len(src)
//line common/common.w:629
	i := 0
//line common/common.w:630
	for i < n {
//line common/common.w:631
		if src[i] != '@' || i+1 >= n {
//line common/common.w:632
			i++
//line common/common.w:633
			continue
//line common/common.w:634
		}
//line common/common.w:635
		switch c := src[i+1]; c {
//line common/common.w:636
		case '@':
//line common/common.w:637
			i += 2
//line common/common.w:638
		case '<', '(', '=', 't', '^', '.', ':', 'q':
//line common/common.w:639
			if end := indexFrom(src, "@>", i+2); end < 0 {
//line common/common.w:640
				warns = append(warns, fmt.Sprintf("%s: unterminated `@%c ... @>'", w.at(lineAt(src, i)), c))
//line common/common.w:641
				i = n
//line common/common.w:642
			} else {
//line common/common.w:643
				i = end + 2
//line common/common.w:644
			}
//line common/common.w:645
		default:
//line common/common.w:646
			i += 2
//line common/common.w:647
		}
//line common/common.w:648
	}
//line common/common.w:649
	return warns
//line common/common.w:650
}

//line common/common.w:654
func parseFormat(seg string, noIndex bool) (Format, bool) {
//line common/common.w:655
	fields := strings.Fields(seg)
//line common/common.w:656
	if len(fields) < 2 {
//line common/common.w:657
		return Format{}, false
//line common/common.w:658
	}
//line common/common.w:659
	return Format{Original: fields[0], Like: fields[1], NoIndex: noIndex}, true
//line common/common.w:660
}

//line common/common.w:668
func parseMacro(seg string) (Format, bool) {
//line common/common.w:669
	fields := strings.Fields(seg)
//line common/common.w:670
	if len(fields) == 0 {
//line common/common.w:671
		return Format{}, false
//line common/common.w:672
	}
//line common/common.w:673
	name := fields[0]
//line common/common.w:674
	if k := strings.LastIndex(name, "."); k >= 0 {
//line common/common.w:675
		name = name[k+1:]
//line common/common.w:676
	}
//line common/common.w:677
	if name == "" {
//line common/common.w:678
		return Format{}, false
//line common/common.w:679
	}
//line common/common.w:680
	return Format{Original: name, Macro: true}, true
//line common/common.w:681
}

//line common/common.w:687
func extractLimboFormats(src string) (string, []Format) {
//line common/common.w:688
	var b strings.Builder
//line common/common.w:689
	var formats []Format
//line common/common.w:690
	n := len(src)
//line common/common.w:691
	i := 0
//line common/common.w:692
	for i < n {
//line common/common.w:693
		if src[i] != '@' || i+1 >= n {
//line common/common.w:694
			b.WriteByte(src[i])
//line common/common.w:695
			i++
//line common/common.w:696
			continue
//line common/common.w:697
		}
//line common/common.w:698
		switch c := src[i+1]; c {
//line common/common.w:699
		case '@':
//line common/common.w:700
			b.WriteString("@@")
//line common/common.w:701
			i += 2
//line common/common.w:702
		case 'd', 'f', 's':
//line common/common.w:703
			j := i + 2
//line common/common.w:704
			for j < n && src[j] != '\n' {
//line common/common.w:705
				j++
//line common/common.w:706
			}
//line common/common.w:707
			var f Format
//line common/common.w:708
			var ok bool
//line common/common.w:709
			if c == 'd' {
//line common/common.w:710
				f, ok = parseMacro(src[i+2 : j])
//line common/common.w:711
			} else {
//line common/common.w:712
				f, ok = parseFormat(src[i+2:j], c == 's')
//line common/common.w:713
			}
//line common/common.w:714
			if ok {
//line common/common.w:715
				formats = append(formats, f)
//line common/common.w:716
			}
//line common/common.w:717
			if j < n {
//line common/common.w:718
				j++ // also drop the newline that ended the directive
//line common/common.w:719
			}
//line common/common.w:720
			i = j
//line common/common.w:721
		case '<', '(', '=', 't', '^', '.', ':', 'q':
//line common/common.w:722
			end := indexFrom(src, "@>", i+2)
//line common/common.w:723
			if end < 0 {
//line common/common.w:724
				b.WriteString(src[i:])
//line common/common.w:725
				i = n
//line common/common.w:726
			} else {
//line common/common.w:727
				b.WriteString(src[i : end+2])
//line common/common.w:728
				i = end + 2
//line common/common.w:729
			}
//line common/common.w:730
		default:
//line common/common.w:731
			b.WriteString(src[i : i+2])
//line common/common.w:732
			i += 2
//line common/common.w:733
		}
//line common/common.w:734
	}
//line common/common.w:735
	return b.String(), formats
//line common/common.w:736
}

//line common/common.w:743
type AtomKind int

//line common/common.w:745
const (
//line common/common.w:746
	AText AtomKind = iota // ordinary \GO/ source text
//line common/common.w:747
	ARef // \.{@<name@>} reference to a named section
//line common/common.w:748
	AVerbatim // \.{@=text@>} passed verbatim to tangled output
//line common/common.w:749
	ATeX // \.{@t text@>} \TEX/ text for the woven output
//line common/common.w:750
	AIndex // \.{@\^/@./@}: index entry
//line common/common.w:751
	APaste // \.{@\&} join (delete surrounding whitespace)
//line common/common.w:752
	ALayout // \.{@}, \.{@/} \.{@|} \.{@\#} woven-output layout hints
//line common/common.w:753
	AIndexDef // \.{@!} force the next identifier to index as a definition
//line common/common.w:754
)

//line common/common.w:756
type Atom struct {
//line common/common.w:757
	Kind AtomKind
//line common/common.w:758
	Text string // payload for |AText|/|AVerbatim|/|ATeX|/|AIndex|; name for |ARef|
//line common/common.w:759
	Index byte // '\.{\^}','\.{.}','\.{:}' for AIndex; '\.{,}' '\.{/}' '\.{|}' '\.{\#}' for |ALayout|
//line common/common.w:760
}

//line common/common.w:766
func ScanCode(code string) []Atom {
//line common/common.w:767
	var atoms []Atom
//line common/common.w:768
	var buf strings.Builder
//line common/common.w:769
	flush := func() {
//line common/common.w:770
		if buf.Len() > 0 {
//line common/common.w:771
			atoms = append(atoms, Atom{Kind: AText, Text: buf.String()})
//line common/common.w:772
			buf.Reset()
//line common/common.w:773
		}
//line common/common.w:774
	}
//line common/common.w:775
	n := len(code)
//line common/common.w:776
	i := 0
//line common/common.w:777
	for i < n {
//line common/common.w:778
		c := code[i]
//line common/common.w:779
		if c != '@' || i+1 >= n {
//line common/common.w:780
			buf.WriteByte(c)
//line common/common.w:781
			i++
//line common/common.w:782
			continue
//line common/common.w:783
		}
//line common/common.w:784
		switch d := code[i+1]; d {
//line common/common.w:785
		case '@':
//line common/common.w:786
			buf.WriteByte('@')
//line common/common.w:787
			i += 2
//line common/common.w:788
		case '&':
//line common/common.w:789
			flush()
//line common/common.w:790
			atoms = append(atoms, Atom{Kind: APaste})
//line common/common.w:791
			i += 2
//line common/common.w:792
		case '<':
//line common/common.w:793
			end := indexFrom(code, "@>", i+2)
//line common/common.w:794
			if end < 0 {
//line common/common.w:795
				buf.WriteString(code[i:])
//line common/common.w:796
				i = n
//line common/common.w:797
				continue
//line common/common.w:798
			}
//line common/common.w:799
			flush()
//line common/common.w:800
			atoms = append(atoms, Atom{Kind: ARef, Text: canonName(code[i+2 : end])})
//line common/common.w:801
			i = end + 2
//line common/common.w:802
		case '=':
//line common/common.w:803
			end := indexFrom(code, "@>", i+2)
//line common/common.w:804
			if end < 0 {
//line common/common.w:805
				i = n
//line common/common.w:806
				continue
//line common/common.w:807
			}
//line common/common.w:808
			flush()
//line common/common.w:809
			atoms = append(atoms, Atom{Kind: AVerbatim, Text: code[i+2 : end]})
//line common/common.w:810
			i = end + 2
//line common/common.w:811
		case 't':
//line common/common.w:812
			end := indexFrom(code, "@>", i+2)
//line common/common.w:813
			if end < 0 {
//line common/common.w:814
				i = n
//line common/common.w:815
				continue
//line common/common.w:816
			}
//line common/common.w:817
			flush()
//line common/common.w:818
			atoms = append(atoms, Atom{Kind: ATeX, Text: code[i+2 : end]})
//line common/common.w:819
			i = end + 2
//line common/common.w:820
		case '^', '.', ':':
//line common/common.w:821
			end := indexFrom(code, "@>", i+2)
//line common/common.w:822
			if end < 0 {
//line common/common.w:823
				i = n
//line common/common.w:824
				continue
//line common/common.w:825
			}
//line common/common.w:826
			flush()
//line common/common.w:827
			atoms = append(atoms, Atom{Kind: AIndex, Text: code[i+2 : end], Index: d})
//line common/common.w:828
			i = end + 2
//line common/common.w:829
		case 'q':
//line common/common.w:830
			end := indexFrom(code, "@>", i+2)
//line common/common.w:831
			if end < 0 {
//line common/common.w:832
				i = n
//line common/common.w:833
				continue
//line common/common.w:834
			}
//line common/common.w:835
			i = end + 2 // ignored material
//line common/common.w:836
		case '%':
//line common/common.w:837
			j := i + 2
//line common/common.w:838
			for j < n && code[j] != '\n' {
//line common/common.w:839
				j++
//line common/common.w:840
			}
//line common/common.w:841
			i = j
//line common/common.w:842
		case '>':
//line common/common.w:843
			i += 2 // stray terminator
//line common/common.w:844
		case ',', '/', '|', '#':
//line common/common.w:845
			// Woven-output layout hints: thin space, line break, optional line
//line common/common.w:846
			// break, and break-plus-blank-line. Ignored by gtangle.
//line common/common.w:847
			flush()
//line common/common.w:848
			atoms = append(atoms, Atom{Kind: ALayout, Index: d})
//line common/common.w:849
			i += 2
//line common/common.w:850
		case '!':
//line common/common.w:851
			// Force the next identifier's index entry to be a definition,
//line common/common.w:852
			// overriding the heuristic. Produces no output by itself.
//line common/common.w:853
			flush()
//line common/common.w:854
			atoms = append(atoms, Atom{Kind: AIndexDef})
//line common/common.w:855
			i += 2
//line common/common.w:856
		case '+', '[', ']', ';':
//line common/common.w:857
			// \.{CWEB} prettyprinter hints (cancel break, expression brackets,
//line common/common.w:858
			// invisible semicolon). \.{GWEB} mirrors the source instead of reflowing
//line common/common.w:859
			// it, so these have no effect; accept and drop them for portability.
//line common/common.w:860
			i += 2
//line common/common.w:861
		default:
//line common/common.w:862
			i += 2 // unknown \.{@x}: drop it rather than corrupt the output
//line common/common.w:863
		}
//line common/common.w:864
	}
//line common/common.w:865
	flush()
//line common/common.w:866
	return atoms
//line common/common.w:867
}

//line common/common.w:884
type change struct {
//line common/common.w:885
	match []string // lines to find in the master source
//line common/common.w:886
	repl []string // lines to substitute for them
//line common/common.w:887
	line int // 1-based line of the \.{@x} in the change file (for diagnostics)
//line common/common.w:888
	replLine int // 1-based change-file line of the first replacement line
//line common/common.w:889
}

//line common/common.w:891
type srcLoc struct {
//line common/common.w:892
	file string
//line common/common.w:893
	line int
//line common/common.w:894
}

//line common/common.w:896
func (l srcLoc) String() string {
//line common/common.w:897
	if l.file == "" {
//line common/common.w:898
		return fmt.Sprintf("line %d", l.line)
//line common/common.w:899
	}
//line common/common.w:900
	return fmt.Sprintf("%s:%d", l.file, l.line)
//line common/common.w:901
}

//line common/common.w:906
func isChangeCtrl(line string, c byte) bool {
//line common/common.w:907
	return len(line) >= 2 && line[0] == '@' && line[1] == c
//line common/common.w:908
}

//line common/common.w:910
func splitLines(s string) []string {
//line common/common.w:911
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
//line common/common.w:912
}

//line common/common.w:914
func sameLine(a, b string) bool {
//line common/common.w:915
	return strings.TrimRight(a, " \t") == strings.TrimRight(b, " \t")
//line common/common.w:916
}

//line common/common.w:921
func parseChangeFile(src string) ([]change, error) {
//line common/common.w:922
	lines := splitLines(src)
//line common/common.w:923
	var changes []change
//line common/common.w:924
	n := len(lines)
//line common/common.w:925
	for i := 0; i < n; {
//line common/common.w:926
		if !isChangeCtrl(lines[i], 'x') {
//line common/common.w:927
			i++ // commentary between changes
//line common/common.w:928
			continue
//line common/common.w:929
		}
//line common/common.w:930
		c := change{line: i + 1}
//line common/common.w:931
		i++
//line common/common.w:932
		for i < n && !isChangeCtrl(lines[i], 'y') {
//line common/common.w:933
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'z') {
//line common/common.w:934
				return nil, fmt.Errorf("change file line %d: expected @y to close the @x match part", c.line)
//line common/common.w:935
			}
//line common/common.w:936
			c.match = append(c.match, lines[i])
//line common/common.w:937
			i++
//line common/common.w:938
		}
//line common/common.w:939
		if i >= n {
//line common/common.w:940
			return nil, fmt.Errorf("change file line %d: @x without a matching @y", c.line)
//line common/common.w:941
		}
//line common/common.w:942
		i++ // skip \.{@y}
//line common/common.w:943
		c.replLine = i + 1
//line common/common.w:944
		for i < n && !isChangeCtrl(lines[i], 'z') {
//line common/common.w:945
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'y') {
//line common/common.w:946
				return nil, fmt.Errorf("change file line %d: expected @z to close the change", c.line)
//line common/common.w:947
			}
//line common/common.w:948
			c.repl = append(c.repl, lines[i])
//line common/common.w:949
			i++
//line common/common.w:950
		}
//line common/common.w:951
		if i >= n {
//line common/common.w:952
			return nil, fmt.Errorf("change file line %d: change has no @z", c.line)
//line common/common.w:953
		}
//line common/common.w:954
		i++ // skip \.{@z}
//line common/common.w:955
		if len(c.match) == 0 {
//line common/common.w:956
			return nil, fmt.Errorf("change file line %d: the @x match part is empty", c.line)
//line common/common.w:957
		}
//line common/common.w:958
		changes = append(changes, c)
//line common/common.w:959
	}
//line common/common.w:960
	return changes, nil
//line common/common.w:961
}

//line common/common.w:965
func applyChanges(src string, changes []change, chFile string) (string, error) {
//line common/common.w:966
	out, _, err := applyChangesMapped(splitLines(src), nil, changes, chFile)
//line common/common.w:967
	if err != nil {
//line common/common.w:968
		return "", err
//line common/common.w:969
	}
//line common/common.w:970
	return strings.Join(out, "\n"), nil
//line common/common.w:971
}

//line common/common.w:978
func applyChangesMapped(master []string, locs []srcLoc, changes []change, chFile string) ([]string, []srcLoc, error) {
//line common/common.w:979
	loc := func(i int) srcLoc {
//line common/common.w:980
		if locs != nil && i < len(locs) {
//line common/common.w:981
			return locs[i]
//line common/common.w:982
		}
//line common/common.w:983
		return srcLoc{line: i + 1}
//line common/common.w:984
	}
//line common/common.w:985
	out := make([]string, 0, len(master))
//line common/common.w:986
	var outLocs []srcLoc
//line common/common.w:987
	ci := 0
//line common/common.w:988
	for i := 0; i < len(master); {
//line common/common.w:989
		if ci < len(changes) && sameLine(master[i], changes[ci].match[0]) {
//line common/common.w:990
			if !blockMatches(master, i, changes[ci].match) {
//line common/common.w:991
				return nil, nil, fmt.Errorf("%s:%d: change did not match the master source at %s",
//line common/common.w:992
					chFile, changes[ci].line, loc(i))
//line common/common.w:993
			}
//line common/common.w:994
			for r, rl := range changes[ci].repl {
//line common/common.w:995
				out = append(out, rl)
//line common/common.w:996
				outLocs = append(outLocs, srcLoc{chFile, changes[ci].replLine + r})
//line common/common.w:997
			}
//line common/common.w:998
			i += len(changes[ci].match)
//line common/common.w:999
			ci++
//line common/common.w:1000
			continue
//line common/common.w:1001
		}
//line common/common.w:1002
		out = append(out, master[i])
//line common/common.w:1003
		outLocs = append(outLocs, loc(i))
//line common/common.w:1004
		i++
//line common/common.w:1005
	}
//line common/common.w:1006
	if ci < len(changes) {
//line common/common.w:1007
		return nil, nil, fmt.Errorf("%s:%d: change was never matched (looking for %q)",
//line common/common.w:1008
			chFile, changes[ci].line, changes[ci].match[0])
//line common/common.w:1009
	}
//line common/common.w:1010
	return out, outLocs, nil
//line common/common.w:1011
}

//line common/common.w:1016
func blockMatches(master []string, at int, match []string) bool {
//line common/common.w:1017
	if at+len(match) > len(master) {
//line common/common.w:1018
		return false
//line common/common.w:1019
	}
//line common/common.w:1020
	for k, m := range match {
//line common/common.w:1021
		if !sameLine(master[at+k], m) {
//line common/common.w:1022
			return false
//line common/common.w:1023
		}
//line common/common.w:1024
	}
//line common/common.w:1025
	return true
//line common/common.w:1026
}
