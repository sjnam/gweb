// Package common parses GWEB source files (.w) into a sequence of sections that
// gtangle and gweave consume. It is the shared front end of the GWEB system,
// playing the role of CWEB's common.w.
//
//line common/common.w:11
//line common/common.w:12
//line common/common.w:13
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

// Version is the GWEB release, shared by gtangle and gweave for their startup
// banner and their --version output.
//
//line common/common.w:23
//line common/common.w:24
//line common/common.w:25
const Version = "0.3.0"

// Format records an "@f a b" (or "@s a b") directive: typeset identifier
// Original the way identifier/keyword Like is typeset. NoIndex is true for @s.
//
//line common/common.w:27
//line common/common.w:28
//line common/common.w:29
type Format struct {
//line common/common.w:30
	Original string
//line common/common.w:31
	Like string
//line common/common.w:32
	NoIndex bool
//line common/common.w:33
	Macro bool // @d: typeset Original in typewriter (a CWEB-style macro)
//line common/common.w:34
}

// Section is one numbered section of the web.
//
//line common/common.w:40
//line common/common.w:41
type Section struct {
//line common/common.w:42
	Number int // 1-based section number
//line common/common.w:43
	Line int // 1-based source line where the section begins
//line common/common.w:44
	Starred bool // true for @* sections
//line common/common.w:45
	Depth int // group depth for starred sections (-1 == @**, 0 == @*, n == @*n)
//line common/common.w:46
	Title string // starred-section title (text up to the first period)
//line common/common.w:47
	Tex string // commentary, raw TeX with in-text @-codes still embedded
//line common/common.w:48
	Formats []Format
//line common/common.w:49
	HasCode bool // true if the section contributes code
//line common/common.w:50
	Name string // named-section name, or "" for an unnamed @c section
//line common/common.w:51
	IsFile bool // true if the name is an output file (@(file@>=)
//line common/common.w:52
	Code string // raw code text with in-code @-codes still embedded
//line common/common.w:53
	CodeLine int // 1-based combined-source line where Code begins (0 if none)
//line common/common.w:54
}

// Web is a fully parsed GWEB document.
//
//line common/common.w:61
//line common/common.w:62
type Web struct {
//line common/common.w:63
	Limbo string
//line common/common.w:64
	Formats []Format // @f / @s directives found in limbo (apply globally)
//line common/common.w:65
	Sections []*Section
//line common/common.w:66
	Warnings []string // non-fatal diagnostics gathered while parsing/checking
//line common/common.w:67
	file string // source filename, for diagnostics ("" if unknown)
//line common/common.w:68
	locs []srcLoc // origin (file, line) of each combined-source line
//line common/common.w:69
	full []string // canonical (non-abbreviated) section names
//line common/common.w:70
}

// Parse reads filename, expands @i includes, and parses the result.
//
//line common/common.w:77
//line common/common.w:78
func Parse(filename string) (*Web, error) {
//line common/common.w:79
	return ParseWithChange(filename, "")
//line common/common.w:80
}

// ParseWithChange reads the master file, expands @i includes, applies the change
// file (CWEB's ".ch" mechanism) if changeFile is non-empty, and parses the
// result. Diagnostics point back to the original file and line via an origin map
// kept in step through include expansion and change application.
//
//line common/common.w:82
//line common/common.w:83
//line common/common.w:84
//line common/common.w:85
//line common/common.w:86
func ParseWithChange(filename, changeFile string) (*Web, error) {
//line common/common.w:87
	lines, locs, err := expandIncludes(filename, 0)
//line common/common.w:88
	if err != nil {
//line common/common.w:89
		return nil, err
//line common/common.w:90
	}
//line common/common.w:91
	if changeFile != "" {
//line common/common.w:92
		chData, err := os.ReadFile(changeFile)
//line common/common.w:93
		if err != nil {
//line common/common.w:94
			return nil, err
//line common/common.w:95
		}
//line common/common.w:96
		changes, err := parseChangeFile(string(chData))
//line common/common.w:97
		if err != nil {
//line common/common.w:98
			return nil, err
//line common/common.w:99
		}
//line common/common.w:100
		lines, locs, err = applyChangesMapped(lines, locs, changes, changeFile)
//line common/common.w:101
		if err != nil {
//line common/common.w:102
			return nil, err
//line common/common.w:103
		}
//line common/common.w:104
	}
//line common/common.w:105
	src := strings.Join(lines, "\n")
//line common/common.w:106
	w := parse(src)
//line common/common.w:107
	w.file = filename
//line common/common.w:108
	w.locs = locs
//line common/common.w:109
	w.finish(src)
//line common/common.w:110
	return w, nil
//line common/common.w:111
}

// ParseString parses already-loaded source (used by tests; no includes).
//
//line common/common.w:113
//line common/common.w:114
func ParseString(src string) *Web {
//line common/common.w:115
	w := parse(src)
//line common/common.w:116
	w.finish(src)
//line common/common.w:117
	return w
//line common/common.w:118
}

// finish runs post-parse bookkeeping: name collection and diagnostics.
//
//line common/common.w:120
//line common/common.w:121
func (w *Web) finish(src string) {
//line common/common.w:122
	w.collectNames()
//line common/common.w:123
	w.Warnings = append(w.Warnings, w.scanDiagnostics(src)...)
//line common/common.w:124
	w.Warnings = append(w.Warnings, w.checkNames()...)
//line common/common.w:125
}

// at formats a combined-source line for a diagnostic, mapping it back to the
// original file and line when an origin map is available.
//
//line common/common.w:127
//line common/common.w:128
//line common/common.w:129
func (w *Web) at(line int) string {
//line common/common.w:130
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line common/common.w:131
		return w.locs[i].String()
//line common/common.w:132
	}
//line common/common.w:133
	if w.file != "" {
//line common/common.w:134
		return fmt.Sprintf("%s:%d", w.file, line)
//line common/common.w:135
	}
//line common/common.w:136
	return fmt.Sprintf("line %d", line)
//line common/common.w:137
}

// Origin maps a 1-based combined-source line back to the original file and line.
// When no origin map is available it falls back to the web's own filename.
//
//line common/common.w:144
//line common/common.w:145
//line common/common.w:146
func (w *Web) Origin(line int) (file string, ln int) {
//line common/common.w:147
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line common/common.w:148
		return w.locs[i].file, w.locs[i].line
//line common/common.w:149
	}
//line common/common.w:150
	return w.file, line
//line common/common.w:151
}

// DefaultExt returns name with ext appended when name has no extension of its
// own (and is non-empty), so "wc" becomes "wc.w". A name that already carries an
// extension is left alone.
//
//line common/common.w:157
//line common/common.w:158
//line common/common.w:159
//line common/common.w:160
func DefaultExt(name, ext string) string {
//line common/common.w:161
	if name == "" || filepath.Ext(name) != "" {
//line common/common.w:162
		return name
//line common/common.w:163
	}
//line common/common.w:164
	return name + ext
//line common/common.w:165
}

// expandIncludes reads file and splices in @i includes, returning the combined
// lines together with a parallel origin map. As in CWEB, @i is line-oriented: a
// line whose first non-blank text is "@i" (followed by whitespace) names a file
// whose expansion replaces that line. A final newline does not produce a
// trailing blank line.
//
//line common/common.w:171
//line common/common.w:172
//line common/common.w:173
//line common/common.w:174
//line common/common.w:175
//line common/common.w:176
func expandIncludes(file string, depth int) ([]string, []srcLoc, error) {
//line common/common.w:177
	if depth > 25 {
//line common/common.w:178
		return nil, nil, fmt.Errorf("gweb: @i include nesting too deep at %q", file)
//line common/common.w:179
	}
//line common/common.w:180
	data, err := os.ReadFile(file)
//line common/common.w:181
	if err != nil {
//line common/common.w:182
		return nil, nil, err
//line common/common.w:183
	}
//line common/common.w:184
	raw := splitLines(string(data))
//line common/common.w:185
	if n := len(raw); n > 0 && raw[n-1] == "" {
//line common/common.w:186
		raw = raw[:n-1]
//line common/common.w:187
	}

//line common/common.w:189
	var lines []string
//line common/common.w:190
	var locs []srcLoc
//line common/common.w:191
	dir := filepath.Dir(file)
//line common/common.w:192
	for i, line := range raw {
//line common/common.w:193
		if name, ok := includeDirective(line); ok {
//line common/common.w:194
			path := name
//line common/common.w:195
			if !filepath.IsAbs(path) {
//line common/common.w:196
				path = filepath.Join(dir, name)
//line common/common.w:197
			}
//line common/common.w:198
			sub, subLocs, err := expandIncludes(path, depth+1)
//line common/common.w:199
			if err != nil {
//line common/common.w:200
				return nil, nil, fmt.Errorf("%s:%d: %w", file, i+1, err)
//line common/common.w:201
			}
//line common/common.w:202
			lines = append(lines, sub...)
//line common/common.w:203
			locs = append(locs, subLocs...)
//line common/common.w:204
			continue
//line common/common.w:205
		}
//line common/common.w:206
		lines = append(lines, line)
//line common/common.w:207
		locs = append(locs, srcLoc{file, i + 1})
//line common/common.w:208
	}
//line common/common.w:209
	return lines, locs, nil
//line common/common.w:210
}

// includeDirective returns the file named by an "@i" line, or ok=false. The @i
// must be the first non-blank text on the line and be followed by whitespace.
//
//line common/common.w:212
//line common/common.w:213
//line common/common.w:214
func includeDirective(line string) (name string, ok bool) {
//line common/common.w:215
	t := strings.TrimLeft(line, " \t")
//line common/common.w:216
	if !strings.HasPrefix(t, "@i") {
//line common/common.w:217
		return "", false
//line common/common.w:218
	}
//line common/common.w:219
	rest := t[2:]
//line common/common.w:220
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
//line common/common.w:221
		return "", false
//line common/common.w:222
	}
//line common/common.w:223
	name = strings.Trim(strings.TrimSpace(rest), "\"")
//line common/common.w:224
	return name, name != ""
//line common/common.w:225
}

// collectNames records the set of canonical (non-abbreviated) section names so
// abbreviations ending in "..." can be resolved. A full name may appear at a
// definition (@<name@>=) or at any reference (@<name@>), in code or in
// commentary, so all of those are scanned. Output files (@(...@>=) are roots,
// not referable refinements, so their names are excluded.
//
//line common/common.w:232
//line common/common.w:233
//line common/common.w:234
//line common/common.w:235
//line common/common.w:236
//line common/common.w:237
func (w *Web) collectNames() {
//line common/common.w:238
	seen := map[string]bool{}
//line common/common.w:239
	add := func(name string) {
//line common/common.w:240
		if name != "" && !strings.HasSuffix(name, "...") && !seen[name] {
//line common/common.w:241
			seen[name] = true
//line common/common.w:242
			w.full = append(w.full, name)
//line common/common.w:243
		}
//line common/common.w:244
	}
//line common/common.w:245
	for _, s := range w.Sections {
//line common/common.w:246
		if !s.IsFile {
//line common/common.w:247
			add(s.Name) // a definition's name
//line common/common.w:248
		}
//line common/common.w:249
		for _, raw := range []string{s.Code, s.Tex} {
//line common/common.w:250
			for _, a := range ScanCode(raw) {
//line common/common.w:251
				if a.Kind == ARef {
//line common/common.w:252
					add(a.Text) // a reference's name
//line common/common.w:253
				}
//line common/common.w:254
			}
//line common/common.w:255
		}
//line common/common.w:256
	}
//line common/common.w:257
}

// prefixMatches counts canonical names beginning with prefix.
//
//line common/common.w:259
//line common/common.w:260
func (w *Web) prefixMatches(prefix string) int {
//line common/common.w:261
	n := 0
//line common/common.w:262
	for _, full := range w.full {
//line common/common.w:263
		if strings.HasPrefix(full, prefix) {
//line common/common.w:264
			n++
//line common/common.w:265
		}
//line common/common.w:266
	}
//line common/common.w:267
	return n
//line common/common.w:268
}

// checkNames validates @<...@> references: it reports ambiguous or unmatched
// abbreviations, references to undefined sections, and named sections that are
// defined but never used. All are warnings (gtangle still fails hard if it
// actually meets an undefined reference while expanding).
//
//line common/common.w:275
//line common/common.w:276
//line common/common.w:277
//line common/common.w:278
//line common/common.w:279
func (w *Web) checkNames() []string {
//line common/common.w:280
	// "defined" is the set of sections that actually have a definition (not just
//line common/common.w:281
	// the full names known for abbreviation resolution, which include references).
//line common/common.w:282
	defined := map[string]bool{}
//line common/common.w:283
	for _, s := range w.Sections {
//line common/common.w:284
		if s.Name != "" && !s.IsFile {
//line common/common.w:285
			defined[w.Resolve(s.Name)] = true
//line common/common.w:286
		}
//line common/common.w:287
	}
//line common/common.w:288
	used := map[string]bool{}
//line common/common.w:289
	var warns []string

//line common/common.w:291
	for _, s := range w.Sections {
//line common/common.w:292
		scan := func(raw string) {
//line common/common.w:293
			for _, a := range ScanCode(raw) {
//line common/common.w:294
				if a.Kind != ARef {
//line common/common.w:295
					continue
//line common/common.w:296
				}
//line common/common.w:297
				canon := w.Resolve(a.Text)
//line common/common.w:298
				if strings.HasSuffix(a.Text, "...") && canon == a.Text {
//line common/common.w:299
					prefix := strings.TrimSpace(strings.TrimSuffix(a.Text, "..."))
//line common/common.w:300
					if m := w.prefixMatches(prefix); m == 0 {
//line common/common.w:301
						warns = append(warns, fmt.Sprintf("%s: no section name matches <%s>", w.at(s.Line), a.Text))
//line common/common.w:302
					} else {
//line common/common.w:303
						warns = append(warns, fmt.Sprintf("%s: ambiguous prefix <%s> matches %d section names", w.at(s.Line), a.Text, m))
//line common/common.w:304
					}
//line common/common.w:305
					continue
//line common/common.w:306
				}
//line common/common.w:307
				if !defined[canon] {
//line common/common.w:308
					warns = append(warns, fmt.Sprintf("%s: reference to undefined section <%s>", w.at(s.Line), a.Text))
//line common/common.w:309
				}
//line common/common.w:310
				used[canon] = true
//line common/common.w:311
			}
//line common/common.w:312
		}
//line common/common.w:313
		scan(s.Code)
//line common/common.w:314
		scan(s.Tex)
//line common/common.w:315
	}

//line common/common.w:317
	warned := map[string]bool{}
//line common/common.w:318
	for _, s := range w.Sections {
//line common/common.w:319
		if s.Name == "" || s.IsFile {
//line common/common.w:320
			continue
//line common/common.w:321
		}
//line common/common.w:322
		canon := w.Resolve(s.Name)
//line common/common.w:323
		if !used[canon] && !warned[canon] {
//line common/common.w:324
			warned[canon] = true
//line common/common.w:325
			warns = append(warns, fmt.Sprintf("%s: section <%s> is defined but never used", w.at(s.Line), s.Name))
//line common/common.w:326
		}
//line common/common.w:327
	}
//line common/common.w:328
	return warns
//line common/common.w:329
}

// lineAt returns the 1-based line number of byte offset off in src.
//
//line common/common.w:335
//line common/common.w:336
func lineAt(src string, off int) int {
//line common/common.w:337
	if off > len(src) {
//line common/common.w:338
		off = len(src)
//line common/common.w:339
	}
//line common/common.w:340
	return 1 + strings.Count(src[:off], "\n")
//line common/common.w:341
}

// canonName canonicalizes a section name's whitespace: every run of spaces,
// tabs, and newlines becomes a single space, and leading/trailing space is
// dropped. As in CWEB, this lets a long name that is wrapped across lines in one
// place still match the same name written on a single line elsewhere.
//
//line common/common.w:343
//line common/common.w:344
//line common/common.w:345
//line common/common.w:346
//line common/common.w:347
func canonName(name string) string {
//line common/common.w:348
	return strings.Join(strings.Fields(name), " ")
//line common/common.w:349
}

// Resolve maps a (possibly abbreviated) name to its canonical form. An
// abbreviation "Prefix..." matches the unique full name starting with Prefix.
//
//line common/common.w:351
//line common/common.w:352
//line common/common.w:353
func (w *Web) Resolve(name string) string {
//line common/common.w:354
	name = canonName(name)
//line common/common.w:355
	if !strings.HasSuffix(name, "...") {
//line common/common.w:356
		return name
//line common/common.w:357
	}
//line common/common.w:358
	prefix := strings.TrimSpace(strings.TrimSuffix(name, "..."))
//line common/common.w:359
	var match string
//line common/common.w:360
	count := 0
//line common/common.w:361
	for _, full := range w.full {
//line common/common.w:362
		if strings.HasPrefix(full, prefix) {
//line common/common.w:363
			match = full
//line common/common.w:364
			count++
//line common/common.w:365
		}
//line common/common.w:366
	}
//line common/common.w:367
	if count == 1 {
//line common/common.w:368
		return match
//line common/common.w:369
	}
//line common/common.w:370
	return name // unresolved or ambiguous; leave as-is for caller to report
//line common/common.w:371
}

//line common/common.w:373
func indexFrom(s, sub string, from int) int {
//line common/common.w:374
	if from >= len(s) {
//line common/common.w:375
		return -1
//line common/common.w:376
	}
//line common/common.w:377
	idx := strings.Index(s[from:], sub)
//line common/common.w:378
	if idx < 0 {
//line common/common.w:379
		return -1
//line common/common.w:380
	}
//line common/common.w:381
	return from + idx
//line common/common.w:382
}

// ctrlKind classifies a structural control code found while scanning.
//
//line common/common.w:389
//line common/common.w:390
type ctrlKind int

//line common/common.w:392
const (
//line common/common.w:393
	cEOF ctrlKind = iota
//line common/common.w:394
	cSection
//line common/common.w:395
	cCode // @c (or its synonym @p)
//line common/common.w:396
	cNamed // @<name@>= or @(file@>=
//line common/common.w:397
	cDefn // @d
//line common/common.w:398
	cFormat

//line common/common.w:399
)

//line common/common.w:401
type ctrl struct {
//line common/common.w:402
	kind ctrlKind
//line common/common.w:403
	pos int // index of the leading '@'
//line common/common.w:404
	end int // index just past the control token
//line common/common.w:405
	depth int // for cSection: -1 unstarred (or @** top level), else starred depth
//line common/common.w:406
	starred bool // for cSection (distinguishes @** from an unstarred section)
//line common/common.w:407
	name string // for cNamed
//line common/common.w:408
	isFile bool // for cNamed (@( vs @<)
//line common/common.w:409
	noIndex bool // for cFormat (@s)
//line common/common.w:410
}

// scanStruct finds the next structural control at or after i. It skips literal
// "@@" and argument-terminated codes (@<...@>, @=...@>, etc.) so their contents
// never trigger a false section break. A "@<...@>" not followed by "=" is a
// reference, not a definition, and is skipped.
//
//line common/common.w:417
//line common/common.w:418
//line common/common.w:419
//line common/common.w:420
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
			j := i + 2
//line common/common.w:438
			depth := 0
//line common/common.w:439
			if j < n && src[j] == '*' {
//line common/common.w:440
				j++
//line common/common.w:441
				depth = -1 // "@**" is the top level: bold in the contents, as cweb
//line common/common.w:442
			} else {
//line common/common.w:443
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:444
					depth = depth*10 + int(src[j]-'0')
//line common/common.w:445
					j++
//line common/common.w:446
				}
//line common/common.w:447
			}
//line common/common.w:448
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}
//line common/common.w:449
		case c == 'c' || c == 'p':
//line common/common.w:450
			return ctrl{kind: cCode, pos: i, end: i + 2}
//line common/common.w:451
		case c == 'd':
//line common/common.w:452
			return ctrl{kind: cDefn, pos: i, end: i + 2}
//line common/common.w:453
		case c == 'f':
//line common/common.w:454
			return ctrl{kind: cFormat, pos: i, end: i + 2}
//line common/common.w:455
		case c == 's':
//line common/common.w:456
			return ctrl{kind: cFormat, pos: i, end: i + 2, noIndex: true}
//line common/common.w:457
		case c == '<' || c == '(':
//line common/common.w:458
			end := indexFrom(src, "@>", i+2)
//line common/common.w:459
			if end < 0 {
//line common/common.w:460
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:461
			}
//line common/common.w:462
			after := end + 2
//line common/common.w:463
			k := after
//line common/common.w:464
			for k < n && (src[k] == ' ' || src[k] == '\t') {
//line common/common.w:465
				k++
//line common/common.w:466
			}
//line common/common.w:467
			if k < n && src[k] == '=' {
//line common/common.w:468
				return ctrl{kind: cNamed, pos: i, end: k + 1,
//line common/common.w:469
					name: canonName(src[i+2 : end]), isFile: c == '('}
//line common/common.w:470
			}
//line common/common.w:471
			i = after // a reference, not a definition
//line common/common.w:472
		case c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line common/common.w:473
			end := indexFrom(src, "@>", i+2)
//line common/common.w:474
			if end < 0 {
//line common/common.w:475
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:476
			}
//line common/common.w:477
			i = end + 2
//line common/common.w:478
		case c == '%':
//line common/common.w:479
			j := i + 2
//line common/common.w:480
			for j < n && src[j] != '\n' {
//line common/common.w:481
				j++
//line common/common.w:482
			}
//line common/common.w:483
			i = j
//line common/common.w:484
		default:
//line common/common.w:485
			i += 2
//line common/common.w:486
		}
//line common/common.w:487
	}
//line common/common.w:488
	return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:489
}

// findNextSection scans forward to the next section break (@ or @*), skipping
// everything else including argument-terminated codes. Used inside code parts,
// where @c/@d/@f never legitimately appear.
//
//line common/common.w:495
//line common/common.w:496
//line common/common.w:497
//line common/common.w:498
func findNextSection(src string, i int) ctrl {
//line common/common.w:499
	n := len(src)
//line common/common.w:500
	for i < n {
//line common/common.w:501
		if src[i] != '@' {
//line common/common.w:502
			i++
//line common/common.w:503
			continue
//line common/common.w:504
		}
//line common/common.w:505
		if i+1 >= n {
//line common/common.w:506
			break
//line common/common.w:507
		}
//line common/common.w:508
		switch c := src[i+1]; {
//line common/common.w:509
		case c == '@':
//line common/common.w:510
			i += 2
//line common/common.w:511
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line common/common.w:512
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line common/common.w:513
		case c == '*':
//line common/common.w:514
			j := i + 2
//line common/common.w:515
			depth := 0
//line common/common.w:516
			if j < n && src[j] == '*' {
//line common/common.w:517
				j++
//line common/common.w:518
				depth = -1 // "@**" is the top level: bold in the contents, as cweb
//line common/common.w:519
			} else {
//line common/common.w:520
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:521
					depth = depth*10 + int(src[j]-'0')
//line common/common.w:522
					j++
//line common/common.w:523
				}
//line common/common.w:524
			}
//line common/common.w:525
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}
//line common/common.w:526
		case c == '<' || c == '(' || c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line common/common.w:527
			end := indexFrom(src, "@>", i+2)
//line common/common.w:528
			if end < 0 {
//line common/common.w:529
				return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:530
			}
//line common/common.w:531
			i = end + 2
//line common/common.w:532
		case c == '%':
//line common/common.w:533
			j := i + 2
//line common/common.w:534
			for j < n && src[j] != '\n' {
//line common/common.w:535
				j++
//line common/common.w:536
			}
//line common/common.w:537
			i = j
//line common/common.w:538
		default:
//line common/common.w:539
			i += 2
//line common/common.w:540
		}
//line common/common.w:541
	}
//line common/common.w:542
	return ctrl{kind: cEOF, pos: n, end: n}
//line common/common.w:543
}

// parse splits source into limbo and sections.
//
//line common/common.w:548
//line common/common.w:549
func parse(src string) *Web {
//line common/common.w:550
	w := &Web{}
//line common/common.w:551
	n := len(src)

//line common/common.w:553
	// Limbo runs until the first section break. Format directives placed there
//line common/common.w:554
	// (@f / @s, a common CWEB idiom) are extracted and removed from the copied
//line common/common.w:555
	// TeX so they apply globally rather than printing literally.
//line common/common.w:556
	first := findNextSection(src, 0)
//line common/common.w:557
	w.Limbo, w.Formats = extractLimboFormats(src[:first.pos])
//line common/common.w:558
	i := first.pos

//line common/common.w:560
	num := 0
//line common/common.w:561
	for i < n {
//line common/common.w:562
		// We are positioned at a section break.
//line common/common.w:563
		hdr := src[i+1]
//line common/common.w:564
		num++
//line common/common.w:565
		sec := &Section{Number: num, Line: lineAt(src, i)}
//line common/common.w:566
		if hdr == '*' {
//line common/common.w:567
			h := findSectionHeaderEnd(src, i)
//line common/common.w:568
			sec.Starred = true
//line common/common.w:569
			sec.Depth = h.depth
//line common/common.w:570
			i = h.end
//line common/common.w:571
		} else {
//line common/common.w:572
			i += 2
//line common/common.w:573
		}

//line common/common.w:575
		// TeX part: from here to the next structural control.
//line common/common.w:576
		ct := scanStruct(src, i)
//line common/common.w:577
		sec.Tex = src[i:ct.pos]
//line common/common.w:578
		if sec.Starred {
//line common/common.w:579
			sec.Title = extractTitle(sec.Tex)
//line common/common.w:580
		}

//line common/common.w:582
		// Definition part: a run of @d / @f / @s.
//line common/common.w:583
		for ct.kind == cDefn || ct.kind == cFormat {
//line common/common.w:584
			nx := scanStruct(src, ct.end)
//line common/common.w:585
			seg := src[ct.end:nx.pos]
//line common/common.w:586
			// @d has no Go analogue (Go has no preprocessor), so it never tangles
//line common/common.w:587
			// to code; gweave uses it only to set the named identifier in
//line common/common.w:588
			// typewriter, as cweave sets a macro. @f/@s format like another word.
//line common/common.w:589
			if ct.kind == cDefn {
//line common/common.w:590
				if f, ok := parseMacro(seg); ok {
//line common/common.w:591
					sec.Formats = append(sec.Formats, f)
//line common/common.w:592
				}
//line common/common.w:593
			} else if f, ok := parseFormat(seg, ct.noIndex); ok {
//line common/common.w:594
				sec.Formats = append(sec.Formats, f)
//line common/common.w:595
			}
//line common/common.w:596
			ct = nx
//line common/common.w:597
		}

//line common/common.w:599
		switch ct.kind {
//line common/common.w:600
		case cCode:
//line common/common.w:601
			sec.HasCode = true
//line common/common.w:602
			sec.CodeLine = lineAt(src, ct.end)
//line common/common.w:603
			nx := findNextSection(src, ct.end)
//line common/common.w:604
			sec.Code = src[ct.end:nx.pos]
//line common/common.w:605
			i = nx.pos
//line common/common.w:606
		case cNamed:
//line common/common.w:607
			sec.HasCode = true
//line common/common.w:608
			sec.Name = ct.name
//line common/common.w:609
			sec.IsFile = ct.isFile
//line common/common.w:610
			sec.CodeLine = lineAt(src, ct.end)
//line common/common.w:611
			nx := findNextSection(src, ct.end)
//line common/common.w:612
			sec.Code = src[ct.end:nx.pos]
//line common/common.w:613
			i = nx.pos
//line common/common.w:614
		default: // cSection or cEOF: a documentation-only section
//line common/common.w:615
			i = ct.pos
//line common/common.w:616
		}

//line common/common.w:618
		w.Sections = append(w.Sections, sec)
//line common/common.w:619
		if ct.kind == cEOF && sec.Code == "" {
//line common/common.w:620
			break
//line common/common.w:621
		}
//line common/common.w:622
		if i >= n {
//line common/common.w:623
			break
//line common/common.w:624
		}
//line common/common.w:625
	}
//line common/common.w:626
	return w
//line common/common.w:627
}

//line common/common.w:631
func findSectionHeaderEnd(src string, i int) ctrl {
//line common/common.w:632
	n := len(src)
//line common/common.w:633
	j := i + 2
//line common/common.w:634
	depth := 0
//line common/common.w:635
	if j < n && src[j] == '*' {
//line common/common.w:636
		j++
//line common/common.w:637
		depth = -1 // "@**" is the top level: bold in the contents, as cweb
//line common/common.w:638
	} else {
//line common/common.w:639
		for j < n && src[j] >= '0' && src[j] <= '9' {
//line common/common.w:640
			depth = depth*10 + int(src[j]-'0')
//line common/common.w:641
			j++
//line common/common.w:642
		}
//line common/common.w:643
	}
//line common/common.w:644
	return ctrl{end: j, depth: depth}
//line common/common.w:645
}

// extractTitle returns the text of a starred section up to its terminating
// period, with whitespace collapsed, for use in the table of contents. The
// terminator is the first period at end of text or followed by whitespace, so a
// period inside a control sequence such as \.{web} does not end the title early.
//
//line common/common.w:650
//line common/common.w:651
//line common/common.w:652
//line common/common.w:653
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

// titleEnd returns the index of the period that ends a starred-section title --
// the first '.' at end of s or followed by whitespace -- or -1 if there is none.
//
//line common/common.w:662
//line common/common.w:663
//line common/common.w:664
func titleEnd(s string) int {
//line common/common.w:665
	for i := 0; i < len(s); i++ {
//line common/common.w:666
		if s[i] == '.' && (i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t' ||
//line common/common.w:667
			s[i+1] == '\n' || s[i+1] == '\r') {
//line common/common.w:668
			return i
//line common/common.w:669
		}
//line common/common.w:670
	}
//line common/common.w:671
	return -1
//line common/common.w:672
}

// scanDiagnostics walks the source looking for malformed control codes —
// currently argument-terminated codes (@<, @(, @=, @t, @^, @., @:, @q) that are
// missing their closing @> — and returns one warning per problem.
//
//line common/common.w:677
//line common/common.w:678
//line common/common.w:679
//line common/common.w:680
func (w *Web) scanDiagnostics(src string) []string {
//line common/common.w:681
	var warns []string
//line common/common.w:682
	n := len(src)
//line common/common.w:683
	i := 0
//line common/common.w:684
	for i < n {
//line common/common.w:685
		if src[i] != '@' || i+1 >= n {
//line common/common.w:686
			i++
//line common/common.w:687
			continue
//line common/common.w:688
		}
//line common/common.w:689
		switch c := src[i+1]; c {
//line common/common.w:690
		case '@':
//line common/common.w:691
			i += 2
//line common/common.w:692
		case '<', '(', '=', 't', '^', '.', ':', 'q':
//line common/common.w:693
			if end := indexFrom(src, "@>", i+2); end < 0 {
//line common/common.w:694
				warns = append(warns, fmt.Sprintf("%s: unterminated `@%c ... @>'", w.at(lineAt(src, i)), c))
//line common/common.w:695
				i = n
//line common/common.w:696
			} else {
//line common/common.w:697
				i = end + 2
//line common/common.w:698
			}
//line common/common.w:699
		default:
//line common/common.w:700
			i += 2
//line common/common.w:701
		}
//line common/common.w:702
	}
//line common/common.w:703
	return warns
//line common/common.w:704
}

// parseFormat parses the body of an @f/@s directive: two identifiers.
//
//line common/common.w:708
//line common/common.w:709
func parseFormat(seg string, noIndex bool) (Format, bool) {
//line common/common.w:710
	fields := strings.Fields(seg)
//line common/common.w:711
	if len(fields) < 2 {
//line common/common.w:712
		return Format{}, false
//line common/common.w:713
	}
//line common/common.w:714
	return Format{Original: fields[0], Like: fields[1], NoIndex: noIndex}, true
//line common/common.w:715
}

// parseMacro parses an @d directive: its first word names a constant to set in
// typewriter; any value after it is ignored (Go has no preprocessor). A
// qualified name keeps its final component, so "@d http.StatusOK" and
// "@d StatusOK" both register StatusOK.
//
//line common/common.w:723
//line common/common.w:724
//line common/common.w:725
//line common/common.w:726
//line common/common.w:727
func parseMacro(seg string) (Format, bool) {
//line common/common.w:728
	fields := strings.Fields(seg)
//line common/common.w:729
	if len(fields) == 0 {
//line common/common.w:730
		return Format{}, false
//line common/common.w:731
	}
//line common/common.w:732
	name := fields[0]
//line common/common.w:733
	if k := strings.LastIndex(name, "."); k >= 0 {
//line common/common.w:734
		name = name[k+1:]
//line common/common.w:735
	}
//line common/common.w:736
	if name == "" {
//line common/common.w:737
		return Format{}, false
//line common/common.w:738
	}
//line common/common.w:739
	return Format{Original: name, Macro: true}, true
//line common/common.w:740
}

// extractLimboFormats pulls @d/@f/@s directives out of the limbo text
// (consuming each to end of line) and returns the cleaned text together with the
// formats. Other control codes and argument-terminated groups are copied through.
//
//line common/common.w:746
//line common/common.w:747
//line common/common.w:748
//line common/common.w:749
func extractLimboFormats(src string) (string, []Format) {
//line common/common.w:750
	var b strings.Builder
//line common/common.w:751
	var formats []Format
//line common/common.w:752
	n := len(src)
//line common/common.w:753
	i := 0
//line common/common.w:754
	for i < n {
//line common/common.w:755
		if src[i] != '@' || i+1 >= n {
//line common/common.w:756
			b.WriteByte(src[i])
//line common/common.w:757
			i++
//line common/common.w:758
			continue
//line common/common.w:759
		}
//line common/common.w:760
		switch c := src[i+1]; c {
//line common/common.w:761
		case '@':
//line common/common.w:762
			b.WriteString("@@")
//line common/common.w:763
			i += 2
//line common/common.w:764
		case 'd', 'f', 's':
//line common/common.w:765
			j := i + 2
//line common/common.w:766
			for j < n && src[j] != '\n' {
//line common/common.w:767
				j++
//line common/common.w:768
			}
//line common/common.w:769
			var f Format
//line common/common.w:770
			var ok bool
//line common/common.w:771
			if c == 'd' {
//line common/common.w:772
				f, ok = parseMacro(src[i+2 : j])
//line common/common.w:773
			} else {
//line common/common.w:774
				f, ok = parseFormat(src[i+2:j], c == 's')
//line common/common.w:775
			}
//line common/common.w:776
			if ok {
//line common/common.w:777
				formats = append(formats, f)
//line common/common.w:778
			}
//line common/common.w:779
			if j < n {
//line common/common.w:780
				j++ // also drop the newline that ended the directive
//line common/common.w:781
			}
//line common/common.w:782
			i = j
//line common/common.w:783
		case '<', '(', '=', 't', '^', '.', ':', 'q':
//line common/common.w:784
			end := indexFrom(src, "@>", i+2)
//line common/common.w:785
			if end < 0 {
//line common/common.w:786
				b.WriteString(src[i:])
//line common/common.w:787
				i = n
//line common/common.w:788
			} else {
//line common/common.w:789
				b.WriteString(src[i : end+2])
//line common/common.w:790
				i = end + 2
//line common/common.w:791
			}
//line common/common.w:792
		default:
//line common/common.w:793
			b.WriteString(src[i : i+2])
//line common/common.w:794
			i += 2
//line common/common.w:795
		}
//line common/common.w:796
	}
//line common/common.w:797
	return b.String(), formats
//line common/common.w:798
}

// AtomKind classifies a piece of a code part.
//
//line common/common.w:805
//line common/common.w:806
type AtomKind int

//line common/common.w:808
const (
//line common/common.w:809
	AText AtomKind = iota // ordinary Go source text
//line common/common.w:810
	ARef // @<name@> reference to a named section
//line common/common.w:811
	AVerbatim // @=text@> passed verbatim to tangled output
//line common/common.w:812
	ATeX // @t text@> TeX text for the woven output
//line common/common.w:813
	AIndex // @^/@./@: index entry
//line common/common.w:814
	APaste // @& join (delete surrounding whitespace)
//line common/common.w:815
	ALayout // @, @/ @| @# woven-output layout hints
//line common/common.w:816
	AIndexDef // @! force the next identifier to index as a definition
//line common/common.w:817
)

// Atom is one element of a scanned code part.
//
//line common/common.w:819
//line common/common.w:820
type Atom struct {
//line common/common.w:821
	Kind AtomKind
//line common/common.w:822
	Text string // payload for AText/AVerbatim/ATeX/AIndex; name for ARef
//line common/common.w:823
	Index byte // '^','.',':' for AIndex; ',' '/' '|' '#' for ALayout
//line common/common.w:824
}

// ScanCode splits a raw code part into atoms, interpreting in-code control
// codes. "@@" becomes a literal '@' folded into the surrounding text.
//
//line common/common.w:830
//line common/common.w:831
//line common/common.w:832
func ScanCode(code string) []Atom {
//line common/common.w:833
	var atoms []Atom
//line common/common.w:834
	var buf strings.Builder
//line common/common.w:835
	flush := func() {
//line common/common.w:836
		if buf.Len() > 0 {
//line common/common.w:837
			atoms = append(atoms, Atom{Kind: AText, Text: buf.String()})
//line common/common.w:838
			buf.Reset()
//line common/common.w:839
		}
//line common/common.w:840
	}
//line common/common.w:841
	n := len(code)
//line common/common.w:842
	i := 0
//line common/common.w:843
	for i < n {
//line common/common.w:844
		c := code[i]
//line common/common.w:845
		if c != '@' || i+1 >= n {
//line common/common.w:846
			buf.WriteByte(c)
//line common/common.w:847
			i++
//line common/common.w:848
			continue
//line common/common.w:849
		}
//line common/common.w:850
		switch d := code[i+1]; d {
//line common/common.w:851
		case '@':
//line common/common.w:852
			buf.WriteByte('@')
//line common/common.w:853
			i += 2
//line common/common.w:854
		case '&':
//line common/common.w:855
			flush()
//line common/common.w:856
			atoms = append(atoms, Atom{Kind: APaste})
//line common/common.w:857
			i += 2
//line common/common.w:858
		case '<':
//line common/common.w:859
			end := indexFrom(code, "@>", i+2)
//line common/common.w:860
			if end < 0 {
//line common/common.w:861
				buf.WriteString(code[i:])
//line common/common.w:862
				i = n
//line common/common.w:863
				continue
//line common/common.w:864
			}
//line common/common.w:865
			flush()
//line common/common.w:866
			atoms = append(atoms, Atom{Kind: ARef, Text: canonName(code[i+2 : end])})
//line common/common.w:867
			i = end + 2
//line common/common.w:868
		case '=':
//line common/common.w:869
			end := indexFrom(code, "@>", i+2)
//line common/common.w:870
			if end < 0 {
//line common/common.w:871
				i = n
//line common/common.w:872
				continue
//line common/common.w:873
			}
//line common/common.w:874
			flush()
//line common/common.w:875
			atoms = append(atoms, Atom{Kind: AVerbatim, Text: code[i+2 : end]})
//line common/common.w:876
			i = end + 2
//line common/common.w:877
		case 't':
//line common/common.w:878
			end := indexFrom(code, "@>", i+2)
//line common/common.w:879
			if end < 0 {
//line common/common.w:880
				i = n
//line common/common.w:881
				continue
//line common/common.w:882
			}
//line common/common.w:883
			flush()
//line common/common.w:884
			atoms = append(atoms, Atom{Kind: ATeX, Text: code[i+2 : end]})
//line common/common.w:885
			i = end + 2
//line common/common.w:886
		case '^', '.', ':':
//line common/common.w:887
			end := indexFrom(code, "@>", i+2)
//line common/common.w:888
			if end < 0 {
//line common/common.w:889
				i = n
//line common/common.w:890
				continue
//line common/common.w:891
			}
//line common/common.w:892
			flush()
//line common/common.w:893
			atoms = append(atoms, Atom{Kind: AIndex, Text: code[i+2 : end], Index: d})
//line common/common.w:894
			i = end + 2
//line common/common.w:895
		case 'q':
//line common/common.w:896
			end := indexFrom(code, "@>", i+2)
//line common/common.w:897
			if end < 0 {
//line common/common.w:898
				i = n
//line common/common.w:899
				continue
//line common/common.w:900
			}
//line common/common.w:901
			i = end + 2 // ignored material
//line common/common.w:902
		case '%':
//line common/common.w:903
			j := i + 2
//line common/common.w:904
			for j < n && code[j] != '\n' {
//line common/common.w:905
				j++
//line common/common.w:906
			}
//line common/common.w:907
			i = j
//line common/common.w:908
		case '>':
//line common/common.w:909
			i += 2 // stray terminator
//line common/common.w:910
		case ',', '/', '|', '#':
//line common/common.w:911
			// Woven-output layout hints: thin space, line break, optional line
//line common/common.w:912
			// break, and break-plus-blank-line. Ignored by gtangle.
//line common/common.w:913
			flush()
//line common/common.w:914
			atoms = append(atoms, Atom{Kind: ALayout, Index: d})
//line common/common.w:915
			i += 2
//line common/common.w:916
		case '!':
//line common/common.w:917
			// Force the next identifier's index entry to be a definition,
//line common/common.w:918
			// overriding the heuristic. Produces no output by itself.
//line common/common.w:919
			flush()
//line common/common.w:920
			atoms = append(atoms, Atom{Kind: AIndexDef})
//line common/common.w:921
			i += 2
//line common/common.w:922
		case '+', '[', ']', ';':
//line common/common.w:923
			// CWEB prettyprinter hints (cancel break, expression brackets,
//line common/common.w:924
			// invisible semicolon). GWEB mirrors the source instead of reflowing
//line common/common.w:925
			// it, so these have no effect; accept and drop them for portability.
//line common/common.w:926
			i += 2
//line common/common.w:927
		default:
//line common/common.w:928
			i += 2 // unknown @x: drop it rather than corrupt the output
//line common/common.w:929
		}
//line common/common.w:930
	}
//line common/common.w:931
	flush()
//line common/common.w:932
	return atoms
//line common/common.w:933
}

//line common/common.w:940
// A change file (CWEB's ".ch" mechanism) patches the master source without
//line common/common.w:941
// editing it. It is a sequence of changes, each of the form
//line common/common.w:942
//
//line common/common.w:943
//	@x
//line common/common.w:944
//	<lines to find in the master source>
//line common/common.w:945
//	@y
//line common/common.w:946
//	<lines to substitute>
//line common/common.w:947
//	@z
//line common/common.w:948
//
//line common/common.w:949
// Text outside an @x...@z group is ignored (it serves as commentary). Changes
//line common/common.w:950
// are matched against the master source — after @i includes are expanded — in
//line common/common.w:951
// the order they appear: GWEB scans the master line by line, and at the first
//line common/common.w:952
// line equal to a change's first match line it requires the whole match block
//line common/common.w:953
// to match, then substitutes the replacement lines.

//line common/common.w:959
type change struct {
//line common/common.w:960
	match []string // lines to find in the master source
//line common/common.w:961
	repl []string // lines to substitute for them
//line common/common.w:962
	line int // 1-based line of the @x in the change file (for diagnostics)
//line common/common.w:963
	replLine int // 1-based change-file line of the first replacement line
//line common/common.w:964
}

// srcLoc identifies the origin (file and 1-based line) of a line of the
// includes-expanded, change-applied source, so diagnostics can point back to
// the file the user actually wrote.
//
//line common/common.w:966
//line common/common.w:967
//line common/common.w:968
//line common/common.w:969
type srcLoc struct {
//line common/common.w:970
	file string
//line common/common.w:971
	line int
//line common/common.w:972
}

//line common/common.w:974
func (l srcLoc) String() string {
//line common/common.w:975
	if l.file == "" {
//line common/common.w:976
		return fmt.Sprintf("line %d", l.line)
//line common/common.w:977
	}
//line common/common.w:978
	return fmt.Sprintf("%s:%d", l.file, l.line)
//line common/common.w:979
}

// isChangeCtrl reports whether line begins with the change control "@<c>"
// (c is 'x', 'y', or 'z'), which must start in the first column.
//
//line common/common.w:984
//line common/common.w:985
//line common/common.w:986
func isChangeCtrl(line string, c byte) bool {
//line common/common.w:987
	return len(line) >= 2 && line[0] == '@' && line[1] == c
//line common/common.w:988
}

// splitLines splits text into lines, normalizing CRLF, so that joining the
// result with "\n" reproduces the (LF-normalized) input.
//
//line common/common.w:990
//line common/common.w:991
//line common/common.w:992
func splitLines(s string) []string {
//line common/common.w:993
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
//line common/common.w:994
}

// sameLine compares two source lines for change matching, ignoring trailing
// whitespace (as WEB does).
//
//line common/common.w:996
//line common/common.w:997
//line common/common.w:998
func sameLine(a, b string) bool {
//line common/common.w:999
	return strings.TrimRight(a, " \t") == strings.TrimRight(b, " \t")
//line common/common.w:1000
}

// parseChangeFile parses change-file text into an ordered list of changes.
//
//line common/common.w:1005
//line common/common.w:1006
func parseChangeFile(src string) ([]change, error) {
//line common/common.w:1007
	lines := splitLines(src)
//line common/common.w:1008
	var changes []change
//line common/common.w:1009
	n := len(lines)
//line common/common.w:1010
	for i := 0; i < n; {
//line common/common.w:1011
		if !isChangeCtrl(lines[i], 'x') {
//line common/common.w:1012
			i++ // commentary between changes
//line common/common.w:1013
			continue
//line common/common.w:1014
		}
//line common/common.w:1015
		c := change{line: i + 1}
//line common/common.w:1016
		i++
//line common/common.w:1017
		for i < n && !isChangeCtrl(lines[i], 'y') {
//line common/common.w:1018
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'z') {
//line common/common.w:1019
				return nil, fmt.Errorf("change file line %d: expected @y to close the @x match part", c.line)
//line common/common.w:1020
			}
//line common/common.w:1021
			c.match = append(c.match, lines[i])
//line common/common.w:1022
			i++
//line common/common.w:1023
		}
//line common/common.w:1024
		if i >= n {
//line common/common.w:1025
			return nil, fmt.Errorf("change file line %d: @x without a matching @y", c.line)
//line common/common.w:1026
		}
//line common/common.w:1027
		i++ // skip @y
//line common/common.w:1028
		c.replLine = i + 1
//line common/common.w:1029
		for i < n && !isChangeCtrl(lines[i], 'z') {
//line common/common.w:1030
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'y') {
//line common/common.w:1031
				return nil, fmt.Errorf("change file line %d: expected @z to close the change", c.line)
//line common/common.w:1032
			}
//line common/common.w:1033
			c.repl = append(c.repl, lines[i])
//line common/common.w:1034
			i++
//line common/common.w:1035
		}
//line common/common.w:1036
		if i >= n {
//line common/common.w:1037
			return nil, fmt.Errorf("change file line %d: change has no @z", c.line)
//line common/common.w:1038
		}
//line common/common.w:1039
		i++ // skip @z
//line common/common.w:1040
		if len(c.match) == 0 {
//line common/common.w:1041
			return nil, fmt.Errorf("change file line %d: the @x match part is empty", c.line)
//line common/common.w:1042
		}
//line common/common.w:1043
		changes = append(changes, c)
//line common/common.w:1044
	}
//line common/common.w:1045
	return changes, nil
//line common/common.w:1046
}

// applyChanges returns src with the changes applied (string convenience form,
// used by tests). See applyChangesMapped for the origin-tracking version.
//
//line common/common.w:1050
//line common/common.w:1051
//line common/common.w:1052
func applyChanges(src string, changes []change, chFile string) (string, error) {
//line common/common.w:1053
	out, _, err := applyChangesMapped(splitLines(src), nil, changes, chFile)
//line common/common.w:1054
	if err != nil {
//line common/common.w:1055
		return "", err
//line common/common.w:1056
	}
//line common/common.w:1057
	return strings.Join(out, "\n"), nil
//line common/common.w:1058
}

// applyChangesMapped applies changes to master, keeping a parallel origin map in
// step: passed-through lines keep their origin, and replacement lines are
// attributed to the change file. locs may be nil if origins are not tracked.
// chFile names the change file for diagnostics. It is an error if a change's
// first line is never found, or is found but the rest of the block does not
// match.
//
//line common/common.w:1065
//line common/common.w:1066
//line common/common.w:1067
//line common/common.w:1068
//line common/common.w:1069
//line common/common.w:1070
//line common/common.w:1071
func applyChangesMapped(master []string, locs []srcLoc, changes []change, chFile string) ([]string, []srcLoc, error) {
//line common/common.w:1072
	loc := func(i int) srcLoc {
//line common/common.w:1073
		if locs != nil && i < len(locs) {
//line common/common.w:1074
			return locs[i]
//line common/common.w:1075
		}
//line common/common.w:1076
		return srcLoc{line: i + 1}
//line common/common.w:1077
	}
//line common/common.w:1078
	out := make([]string, 0, len(master))
//line common/common.w:1079
	var outLocs []srcLoc
//line common/common.w:1080
	ci := 0
//line common/common.w:1081
	for i := 0; i < len(master); {
//line common/common.w:1082
		if ci < len(changes) && sameLine(master[i], changes[ci].match[0]) {
//line common/common.w:1083
			if !blockMatches(master, i, changes[ci].match) {
//line common/common.w:1084
				return nil, nil, fmt.Errorf("%s:%d: change did not match the master source at %s",
//line common/common.w:1085
					chFile, changes[ci].line, loc(i))
//line common/common.w:1086
			}
//line common/common.w:1087
			for r, rl := range changes[ci].repl {
//line common/common.w:1088
				out = append(out, rl)
//line common/common.w:1089
				outLocs = append(outLocs, srcLoc{chFile, changes[ci].replLine + r})
//line common/common.w:1090
			}
//line common/common.w:1091
			i += len(changes[ci].match)
//line common/common.w:1092
			ci++
//line common/common.w:1093
			continue
//line common/common.w:1094
		}
//line common/common.w:1095
		out = append(out, master[i])
//line common/common.w:1096
		outLocs = append(outLocs, loc(i))
//line common/common.w:1097
		i++
//line common/common.w:1098
	}
//line common/common.w:1099
	if ci < len(changes) {
//line common/common.w:1100
		return nil, nil, fmt.Errorf("%s:%d: change was never matched (looking for %q)",
//line common/common.w:1101
			chFile, changes[ci].line, changes[ci].match[0])
//line common/common.w:1102
	}
//line common/common.w:1103
	return out, outLocs, nil
//line common/common.w:1104
}

// blockMatches reports whether match lines up with master starting at index at.
//
//line common/common.w:1109
//line common/common.w:1110
func blockMatches(master []string, at int, match []string) bool {
//line common/common.w:1111
	if at+len(match) > len(master) {
//line common/common.w:1112
		return false
//line common/common.w:1113
	}
//line common/common.w:1114
	for k, m := range match {
//line common/common.w:1115
		if !sameLine(master[at+k], m) {
//line common/common.w:1116
			return false
//line common/common.w:1117
		}
//line common/common.w:1118
	}
//line common/common.w:1119
	return true
//line common/common.w:1120
}
