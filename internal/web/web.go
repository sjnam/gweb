// Package web parses GWEB source files (.w) into a sequence of sections that
// gtangle and gweave consume. It is the shared front end of the GWEB system,
// playing the role of CWEB's common.w.
//
//line internal/web/web.w:11
//line internal/web/web.w:12
//line internal/web/web.w:13
//line internal/web/web.w:14
package web

//line internal/web/web.w:16
import (
//line internal/web/web.w:17
	"fmt"
//line internal/web/web.w:18
	"os"
//line internal/web/web.w:19
	"path/filepath"
//line internal/web/web.w:20
	"strings"
//line internal/web/web.w:21
)

// Version is the GWEB release, shared by gtangle and gweave for their startup
// banner and their --version output.
//
//line internal/web/web.w:23
//line internal/web/web.w:24
//line internal/web/web.w:25
const Version = "0.3.0"

// Format records an "@f a b" (or "@s a b") directive: typeset identifier
// Original the way identifier/keyword Like is typeset. NoIndex is true for @s.
//
//line internal/web/web.w:27
//line internal/web/web.w:28
//line internal/web/web.w:29
type Format struct {
//line internal/web/web.w:30
	Original string
//line internal/web/web.w:31
	Like string
//line internal/web/web.w:32
	NoIndex bool
//line internal/web/web.w:33
	Macro bool // @d: typeset Original in typewriter (a CWEB-style macro)
//line internal/web/web.w:34
}

// Section is one numbered section of the web.
//
//line internal/web/web.w:40
//line internal/web/web.w:41
type Section struct {
//line internal/web/web.w:42
	Number int // 1-based section number
//line internal/web/web.w:43
	Line int // 1-based source line where the section begins
//line internal/web/web.w:44
	Starred bool // true for @* sections
//line internal/web/web.w:45
	Depth int // group depth for starred sections (-1 == @**, 0 == @*, n == @*n)
//line internal/web/web.w:46
	Title string // starred-section title (text up to the first period)
//line internal/web/web.w:47
	Tex string // commentary, raw TeX with in-text @-codes still embedded
//line internal/web/web.w:48
	Formats []Format
//line internal/web/web.w:49
	HasCode bool // true if the section contributes code
//line internal/web/web.w:50
	Name string // named-section name, or "" for an unnamed @c section
//line internal/web/web.w:51
	IsFile bool // true if the name is an output file (@(file@>=)
//line internal/web/web.w:52
	Code string // raw code text with in-code @-codes still embedded
//line internal/web/web.w:53
	CodeLine int // 1-based combined-source line where Code begins (0 if none)
//line internal/web/web.w:54
}

// Web is a fully parsed GWEB document.
//
//line internal/web/web.w:61
//line internal/web/web.w:62
type Web struct {
//line internal/web/web.w:63
	Limbo string
//line internal/web/web.w:64
	Formats []Format // @f / @s directives found in limbo (apply globally)
//line internal/web/web.w:65
	Sections []*Section
//line internal/web/web.w:66
	Warnings []string // non-fatal diagnostics gathered while parsing/checking
//line internal/web/web.w:67
	file string // source filename, for diagnostics ("" if unknown)
//line internal/web/web.w:68
	locs []srcLoc // origin (file, line) of each combined-source line
//line internal/web/web.w:69
	full []string // canonical (non-abbreviated) section names
//line internal/web/web.w:70
}

// Parse reads filename, expands @i includes, and parses the result.
//
//line internal/web/web.w:77
//line internal/web/web.w:78
func Parse(filename string) (*Web, error) {
//line internal/web/web.w:79
	return ParseWithChange(filename, "")
//line internal/web/web.w:80
}

// ParseWithChange reads the master file, expands @i includes, applies the change
// file (CWEB's ".ch" mechanism) if changeFile is non-empty, and parses the
// result. Diagnostics point back to the original file and line via an origin map
// kept in step through include expansion and change application.
//
//line internal/web/web.w:82
//line internal/web/web.w:83
//line internal/web/web.w:84
//line internal/web/web.w:85
//line internal/web/web.w:86
func ParseWithChange(filename, changeFile string) (*Web, error) {
//line internal/web/web.w:87
	lines, locs, err := expandIncludes(filename, 0)
//line internal/web/web.w:88
	if err != nil {
//line internal/web/web.w:89
		return nil, err
//line internal/web/web.w:90
	}
//line internal/web/web.w:91
	if changeFile != "" {
//line internal/web/web.w:92
		chData, err := os.ReadFile(changeFile)
//line internal/web/web.w:93
		if err != nil {
//line internal/web/web.w:94
			return nil, err
//line internal/web/web.w:95
		}
//line internal/web/web.w:96
		changes, err := parseChangeFile(string(chData))
//line internal/web/web.w:97
		if err != nil {
//line internal/web/web.w:98
			return nil, err
//line internal/web/web.w:99
		}
//line internal/web/web.w:100
		lines, locs, err = applyChangesMapped(lines, locs, changes, changeFile)
//line internal/web/web.w:101
		if err != nil {
//line internal/web/web.w:102
			return nil, err
//line internal/web/web.w:103
		}
//line internal/web/web.w:104
	}
//line internal/web/web.w:105
	src := strings.Join(lines, "\n")
//line internal/web/web.w:106
	w := parse(src)
//line internal/web/web.w:107
	w.file = filename
//line internal/web/web.w:108
	w.locs = locs
//line internal/web/web.w:109
	w.finish(src)
//line internal/web/web.w:110
	return w, nil
//line internal/web/web.w:111
}

// ParseString parses already-loaded source (used by tests; no includes).
//
//line internal/web/web.w:113
//line internal/web/web.w:114
func ParseString(src string) *Web {
//line internal/web/web.w:115
	w := parse(src)
//line internal/web/web.w:116
	w.finish(src)
//line internal/web/web.w:117
	return w
//line internal/web/web.w:118
}

// finish runs post-parse bookkeeping: name collection and diagnostics.
//
//line internal/web/web.w:120
//line internal/web/web.w:121
func (w *Web) finish(src string) {
//line internal/web/web.w:122
	w.collectNames()
//line internal/web/web.w:123
	w.Warnings = append(w.Warnings, w.scanDiagnostics(src)...)
//line internal/web/web.w:124
	w.Warnings = append(w.Warnings, w.checkNames()...)
//line internal/web/web.w:125
}

// at formats a combined-source line for a diagnostic, mapping it back to the
// original file and line when an origin map is available.
//
//line internal/web/web.w:127
//line internal/web/web.w:128
//line internal/web/web.w:129
func (w *Web) at(line int) string {
//line internal/web/web.w:130
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line internal/web/web.w:131
		return w.locs[i].String()
//line internal/web/web.w:132
	}
//line internal/web/web.w:133
	if w.file != "" {
//line internal/web/web.w:134
		return fmt.Sprintf("%s:%d", w.file, line)
//line internal/web/web.w:135
	}
//line internal/web/web.w:136
	return fmt.Sprintf("line %d", line)
//line internal/web/web.w:137
}

// Origin maps a 1-based combined-source line back to the original file and line.
// When no origin map is available it falls back to the web's own filename.
//
//line internal/web/web.w:144
//line internal/web/web.w:145
//line internal/web/web.w:146
func (w *Web) Origin(line int) (file string, ln int) {
//line internal/web/web.w:147
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line internal/web/web.w:148
		return w.locs[i].file, w.locs[i].line
//line internal/web/web.w:149
	}
//line internal/web/web.w:150
	return w.file, line
//line internal/web/web.w:151
}

// DefaultExt returns name with ext appended when name has no extension of its
// own (and is non-empty), so "wc" becomes "wc.w". A name that already carries an
// extension is left alone.
//
//line internal/web/web.w:157
//line internal/web/web.w:158
//line internal/web/web.w:159
//line internal/web/web.w:160
func DefaultExt(name, ext string) string {
//line internal/web/web.w:161
	if name == "" || filepath.Ext(name) != "" {
//line internal/web/web.w:162
		return name
//line internal/web/web.w:163
	}
//line internal/web/web.w:164
	return name + ext
//line internal/web/web.w:165
}

// expandIncludes reads file and splices in @i includes, returning the combined
// lines together with a parallel origin map. As in CWEB, @i is line-oriented: a
// line whose first non-blank text is "@i" (followed by whitespace) names a file
// whose expansion replaces that line. A final newline does not produce a
// trailing blank line.
//
//line internal/web/web.w:171
//line internal/web/web.w:172
//line internal/web/web.w:173
//line internal/web/web.w:174
//line internal/web/web.w:175
//line internal/web/web.w:176
func expandIncludes(file string, depth int) ([]string, []srcLoc, error) {
//line internal/web/web.w:177
	if depth > 25 {
//line internal/web/web.w:178
		return nil, nil, fmt.Errorf("gweb: @i include nesting too deep at %q", file)
//line internal/web/web.w:179
	}
//line internal/web/web.w:180
	data, err := os.ReadFile(file)
//line internal/web/web.w:181
	if err != nil {
//line internal/web/web.w:182
		return nil, nil, err
//line internal/web/web.w:183
	}
//line internal/web/web.w:184
	raw := splitLines(string(data))
//line internal/web/web.w:185
	if n := len(raw); n > 0 && raw[n-1] == "" {
//line internal/web/web.w:186
		raw = raw[:n-1]
//line internal/web/web.w:187
	}

//line internal/web/web.w:189
	var lines []string
//line internal/web/web.w:190
	var locs []srcLoc
//line internal/web/web.w:191
	dir := filepath.Dir(file)
//line internal/web/web.w:192
	for i, line := range raw {
//line internal/web/web.w:193
		if name, ok := includeDirective(line); ok {
//line internal/web/web.w:194
			path := name
//line internal/web/web.w:195
			if !filepath.IsAbs(path) {
//line internal/web/web.w:196
				path = filepath.Join(dir, name)
//line internal/web/web.w:197
			}
//line internal/web/web.w:198
			sub, subLocs, err := expandIncludes(path, depth+1)
//line internal/web/web.w:199
			if err != nil {
//line internal/web/web.w:200
				return nil, nil, fmt.Errorf("%s:%d: %w", file, i+1, err)
//line internal/web/web.w:201
			}
//line internal/web/web.w:202
			lines = append(lines, sub...)
//line internal/web/web.w:203
			locs = append(locs, subLocs...)
//line internal/web/web.w:204
			continue
//line internal/web/web.w:205
		}
//line internal/web/web.w:206
		lines = append(lines, line)
//line internal/web/web.w:207
		locs = append(locs, srcLoc{file, i + 1})
//line internal/web/web.w:208
	}
//line internal/web/web.w:209
	return lines, locs, nil
//line internal/web/web.w:210
}

// includeDirective returns the file named by an "@i" line, or ok=false. The @i
// must be the first non-blank text on the line and be followed by whitespace.
//
//line internal/web/web.w:212
//line internal/web/web.w:213
//line internal/web/web.w:214
func includeDirective(line string) (name string, ok bool) {
//line internal/web/web.w:215
	t := strings.TrimLeft(line, " \t")
//line internal/web/web.w:216
	if !strings.HasPrefix(t, "@i") {
//line internal/web/web.w:217
		return "", false
//line internal/web/web.w:218
	}
//line internal/web/web.w:219
	rest := t[2:]
//line internal/web/web.w:220
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
//line internal/web/web.w:221
		return "", false
//line internal/web/web.w:222
	}
//line internal/web/web.w:223
	name = strings.Trim(strings.TrimSpace(rest), "\"")
//line internal/web/web.w:224
	return name, name != ""
//line internal/web/web.w:225
}

// collectNames records the set of canonical (non-abbreviated) section names so
// abbreviations ending in "..." can be resolved. A full name may appear at a
// definition (@<name@>=) or at any reference (@<name@>), in code or in
// commentary, so all of those are scanned. Output files (@(...@>=) are roots,
// not referable refinements, so their names are excluded.
//
//line internal/web/web.w:232
//line internal/web/web.w:233
//line internal/web/web.w:234
//line internal/web/web.w:235
//line internal/web/web.w:236
//line internal/web/web.w:237
func (w *Web) collectNames() {
//line internal/web/web.w:238
	seen := map[string]bool{}
//line internal/web/web.w:239
	add := func(name string) {
//line internal/web/web.w:240
		if name != "" && !strings.HasSuffix(name, "...") && !seen[name] {
//line internal/web/web.w:241
			seen[name] = true
//line internal/web/web.w:242
			w.full = append(w.full, name)
//line internal/web/web.w:243
		}
//line internal/web/web.w:244
	}
//line internal/web/web.w:245
	for _, s := range w.Sections {
//line internal/web/web.w:246
		if !s.IsFile {
//line internal/web/web.w:247
			add(s.Name) // a definition's name
//line internal/web/web.w:248
		}
//line internal/web/web.w:249
		for _, raw := range []string{s.Code, s.Tex} {
//line internal/web/web.w:250
			for _, a := range ScanCode(raw) {
//line internal/web/web.w:251
				if a.Kind == ARef {
//line internal/web/web.w:252
					add(a.Text) // a reference's name
//line internal/web/web.w:253
				}
//line internal/web/web.w:254
			}
//line internal/web/web.w:255
		}
//line internal/web/web.w:256
	}
//line internal/web/web.w:257
}

// prefixMatches counts canonical names beginning with prefix.
//
//line internal/web/web.w:259
//line internal/web/web.w:260
func (w *Web) prefixMatches(prefix string) int {
//line internal/web/web.w:261
	n := 0
//line internal/web/web.w:262
	for _, full := range w.full {
//line internal/web/web.w:263
		if strings.HasPrefix(full, prefix) {
//line internal/web/web.w:264
			n++
//line internal/web/web.w:265
		}
//line internal/web/web.w:266
	}
//line internal/web/web.w:267
	return n
//line internal/web/web.w:268
}

// checkNames validates @<...@> references: it reports ambiguous or unmatched
// abbreviations, references to undefined sections, and named sections that are
// defined but never used. All are warnings (gtangle still fails hard if it
// actually meets an undefined reference while expanding).
//
//line internal/web/web.w:275
//line internal/web/web.w:276
//line internal/web/web.w:277
//line internal/web/web.w:278
//line internal/web/web.w:279
func (w *Web) checkNames() []string {
//line internal/web/web.w:280
	// "defined" is the set of sections that actually have a definition (not just
//line internal/web/web.w:281
	// the full names known for abbreviation resolution, which include references).
//line internal/web/web.w:282
	defined := map[string]bool{}
//line internal/web/web.w:283
	for _, s := range w.Sections {
//line internal/web/web.w:284
		if s.Name != "" && !s.IsFile {
//line internal/web/web.w:285
			defined[w.Resolve(s.Name)] = true
//line internal/web/web.w:286
		}
//line internal/web/web.w:287
	}
//line internal/web/web.w:288
	used := map[string]bool{}
//line internal/web/web.w:289
	var warns []string

//line internal/web/web.w:291
	for _, s := range w.Sections {
//line internal/web/web.w:292
		scan := func(raw string) {
//line internal/web/web.w:293
			for _, a := range ScanCode(raw) {
//line internal/web/web.w:294
				if a.Kind != ARef {
//line internal/web/web.w:295
					continue
//line internal/web/web.w:296
				}
//line internal/web/web.w:297
				canon := w.Resolve(a.Text)
//line internal/web/web.w:298
				if strings.HasSuffix(a.Text, "...") && canon == a.Text {
//line internal/web/web.w:299
					prefix := strings.TrimSpace(strings.TrimSuffix(a.Text, "..."))
//line internal/web/web.w:300
					if m := w.prefixMatches(prefix); m == 0 {
//line internal/web/web.w:301
						warns = append(warns, fmt.Sprintf("%s: no section name matches <%s>", w.at(s.Line), a.Text))
//line internal/web/web.w:302
					} else {
//line internal/web/web.w:303
						warns = append(warns, fmt.Sprintf("%s: ambiguous prefix <%s> matches %d section names", w.at(s.Line), a.Text, m))
//line internal/web/web.w:304
					}
//line internal/web/web.w:305
					continue
//line internal/web/web.w:306
				}
//line internal/web/web.w:307
				if !defined[canon] {
//line internal/web/web.w:308
					warns = append(warns, fmt.Sprintf("%s: reference to undefined section <%s>", w.at(s.Line), a.Text))
//line internal/web/web.w:309
				}
//line internal/web/web.w:310
				used[canon] = true
//line internal/web/web.w:311
			}
//line internal/web/web.w:312
		}
//line internal/web/web.w:313
		scan(s.Code)
//line internal/web/web.w:314
		scan(s.Tex)
//line internal/web/web.w:315
	}

//line internal/web/web.w:317
	warned := map[string]bool{}
//line internal/web/web.w:318
	for _, s := range w.Sections {
//line internal/web/web.w:319
		if s.Name == "" || s.IsFile {
//line internal/web/web.w:320
			continue
//line internal/web/web.w:321
		}
//line internal/web/web.w:322
		canon := w.Resolve(s.Name)
//line internal/web/web.w:323
		if !used[canon] && !warned[canon] {
//line internal/web/web.w:324
			warned[canon] = true
//line internal/web/web.w:325
			warns = append(warns, fmt.Sprintf("%s: section <%s> is defined but never used", w.at(s.Line), s.Name))
//line internal/web/web.w:326
		}
//line internal/web/web.w:327
	}
//line internal/web/web.w:328
	return warns
//line internal/web/web.w:329
}

// lineAt returns the 1-based line number of byte offset off in src.
//
//line internal/web/web.w:335
//line internal/web/web.w:336
func lineAt(src string, off int) int {
//line internal/web/web.w:337
	if off > len(src) {
//line internal/web/web.w:338
		off = len(src)
//line internal/web/web.w:339
	}
//line internal/web/web.w:340
	return 1 + strings.Count(src[:off], "\n")
//line internal/web/web.w:341
}

// canonName canonicalizes a section name's whitespace: every run of spaces,
// tabs, and newlines becomes a single space, and leading/trailing space is
// dropped. As in CWEB, this lets a long name that is wrapped across lines in one
// place still match the same name written on a single line elsewhere.
//
//line internal/web/web.w:343
//line internal/web/web.w:344
//line internal/web/web.w:345
//line internal/web/web.w:346
//line internal/web/web.w:347
func canonName(name string) string {
//line internal/web/web.w:348
	return strings.Join(strings.Fields(name), " ")
//line internal/web/web.w:349
}

// Resolve maps a (possibly abbreviated) name to its canonical form. An
// abbreviation "Prefix..." matches the unique full name starting with Prefix.
//
//line internal/web/web.w:351
//line internal/web/web.w:352
//line internal/web/web.w:353
func (w *Web) Resolve(name string) string {
//line internal/web/web.w:354
	name = canonName(name)
//line internal/web/web.w:355
	if !strings.HasSuffix(name, "...") {
//line internal/web/web.w:356
		return name
//line internal/web/web.w:357
	}
//line internal/web/web.w:358
	prefix := strings.TrimSpace(strings.TrimSuffix(name, "..."))
//line internal/web/web.w:359
	var match string
//line internal/web/web.w:360
	count := 0
//line internal/web/web.w:361
	for _, full := range w.full {
//line internal/web/web.w:362
		if strings.HasPrefix(full, prefix) {
//line internal/web/web.w:363
			match = full
//line internal/web/web.w:364
			count++
//line internal/web/web.w:365
		}
//line internal/web/web.w:366
	}
//line internal/web/web.w:367
	if count == 1 {
//line internal/web/web.w:368
		return match
//line internal/web/web.w:369
	}
//line internal/web/web.w:370
	return name // unresolved or ambiguous; leave as-is for caller to report
//line internal/web/web.w:371
}

//line internal/web/web.w:373
func indexFrom(s, sub string, from int) int {
//line internal/web/web.w:374
	if from >= len(s) {
//line internal/web/web.w:375
		return -1
//line internal/web/web.w:376
	}
//line internal/web/web.w:377
	idx := strings.Index(s[from:], sub)
//line internal/web/web.w:378
	if idx < 0 {
//line internal/web/web.w:379
		return -1
//line internal/web/web.w:380
	}
//line internal/web/web.w:381
	return from + idx
//line internal/web/web.w:382
}

// ctrlKind classifies a structural control code found while scanning.
//
//line internal/web/web.w:389
//line internal/web/web.w:390
type ctrlKind int

//line internal/web/web.w:392
const (
//line internal/web/web.w:393
	cEOF ctrlKind = iota
//line internal/web/web.w:394
	cSection
//line internal/web/web.w:395
	cCode // @c (or its synonym @p)
//line internal/web/web.w:396
	cNamed // @<name@>= or @(file@>=
//line internal/web/web.w:397
	cDefn // @d
//line internal/web/web.w:398
	cFormat

//line internal/web/web.w:399
)

//line internal/web/web.w:401
type ctrl struct {
//line internal/web/web.w:402
	kind ctrlKind
//line internal/web/web.w:403
	pos int // index of the leading '@'
//line internal/web/web.w:404
	end int // index just past the control token
//line internal/web/web.w:405
	depth int // for cSection: -1 unstarred (or @** top level), else starred depth
//line internal/web/web.w:406
	starred bool // for cSection (distinguishes @** from an unstarred section)
//line internal/web/web.w:407
	name string // for cNamed
//line internal/web/web.w:408
	isFile bool // for cNamed (@( vs @<)
//line internal/web/web.w:409
	noIndex bool // for cFormat (@s)
//line internal/web/web.w:410
}

// scanStruct finds the next structural control at or after i. It skips literal
// "@@" and argument-terminated codes (@<...@>, @=...@>, etc.) so their contents
// never trigger a false section break. A "@<...@>" not followed by "=" is a
// reference, not a definition, and is skipped.
//
//line internal/web/web.w:417
//line internal/web/web.w:418
//line internal/web/web.w:419
//line internal/web/web.w:420
//line internal/web/web.w:421
func scanStruct(src string, i int) ctrl {
//line internal/web/web.w:422
	n := len(src)
//line internal/web/web.w:423
	for i < n {
//line internal/web/web.w:424
		if src[i] != '@' {
//line internal/web/web.w:425
			i++
//line internal/web/web.w:426
			continue
//line internal/web/web.w:427
		}
//line internal/web/web.w:428
		if i+1 >= n {
//line internal/web/web.w:429
			break
//line internal/web/web.w:430
		}
//line internal/web/web.w:431
		switch c := src[i+1]; {
//line internal/web/web.w:432
		case c == '@':
//line internal/web/web.w:433
			i += 2
//line internal/web/web.w:434
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line internal/web/web.w:435
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line internal/web/web.w:436
		case c == '*':
//line internal/web/web.w:437
			j := i + 2
//line internal/web/web.w:438
			depth := 0
//line internal/web/web.w:439
			if j < n && src[j] == '*' {
//line internal/web/web.w:440
				j++
//line internal/web/web.w:441
				depth = -1 // "@**" is the top level: bold in the contents, as cweb
//line internal/web/web.w:442
			} else {
//line internal/web/web.w:443
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line internal/web/web.w:444
					depth = depth*10 + int(src[j]-'0')
//line internal/web/web.w:445
					j++
//line internal/web/web.w:446
				}
//line internal/web/web.w:447
			}
//line internal/web/web.w:448
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}
//line internal/web/web.w:449
		case c == 'c' || c == 'p':
//line internal/web/web.w:450
			return ctrl{kind: cCode, pos: i, end: i + 2}
//line internal/web/web.w:451
		case c == 'd':
//line internal/web/web.w:452
			return ctrl{kind: cDefn, pos: i, end: i + 2}
//line internal/web/web.w:453
		case c == 'f':
//line internal/web/web.w:454
			return ctrl{kind: cFormat, pos: i, end: i + 2}
//line internal/web/web.w:455
		case c == 's':
//line internal/web/web.w:456
			return ctrl{kind: cFormat, pos: i, end: i + 2, noIndex: true}
//line internal/web/web.w:457
		case c == '<' || c == '(':
//line internal/web/web.w:458
			end := indexFrom(src, "@>", i+2)
//line internal/web/web.w:459
			if end < 0 {
//line internal/web/web.w:460
				return ctrl{kind: cEOF, pos: n, end: n}
//line internal/web/web.w:461
			}
//line internal/web/web.w:462
			after := end + 2
//line internal/web/web.w:463
			k := after
//line internal/web/web.w:464
			for k < n && (src[k] == ' ' || src[k] == '\t') {
//line internal/web/web.w:465
				k++
//line internal/web/web.w:466
			}
//line internal/web/web.w:467
			if k < n && src[k] == '=' {
//line internal/web/web.w:468
				return ctrl{kind: cNamed, pos: i, end: k + 1,
//line internal/web/web.w:469
					name: canonName(src[i+2 : end]), isFile: c == '('}
//line internal/web/web.w:470
			}
//line internal/web/web.w:471
			i = after // a reference, not a definition
//line internal/web/web.w:472
		case c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line internal/web/web.w:473
			end := indexFrom(src, "@>", i+2)
//line internal/web/web.w:474
			if end < 0 {
//line internal/web/web.w:475
				return ctrl{kind: cEOF, pos: n, end: n}
//line internal/web/web.w:476
			}
//line internal/web/web.w:477
			i = end + 2
//line internal/web/web.w:478
		case c == '%':
//line internal/web/web.w:479
			j := i + 2
//line internal/web/web.w:480
			for j < n && src[j] != '\n' {
//line internal/web/web.w:481
				j++
//line internal/web/web.w:482
			}
//line internal/web/web.w:483
			i = j
//line internal/web/web.w:484
		default:
//line internal/web/web.w:485
			i += 2
//line internal/web/web.w:486
		}
//line internal/web/web.w:487
	}
//line internal/web/web.w:488
	return ctrl{kind: cEOF, pos: n, end: n}
//line internal/web/web.w:489
}

// findNextSection scans forward to the next section break (@ or @*), skipping
// everything else including argument-terminated codes. Used inside code parts,
// where @c/@d/@f never legitimately appear.
//
//line internal/web/web.w:495
//line internal/web/web.w:496
//line internal/web/web.w:497
//line internal/web/web.w:498
func findNextSection(src string, i int) ctrl {
//line internal/web/web.w:499
	n := len(src)
//line internal/web/web.w:500
	for i < n {
//line internal/web/web.w:501
		if src[i] != '@' {
//line internal/web/web.w:502
			i++
//line internal/web/web.w:503
			continue
//line internal/web/web.w:504
		}
//line internal/web/web.w:505
		if i+1 >= n {
//line internal/web/web.w:506
			break
//line internal/web/web.w:507
		}
//line internal/web/web.w:508
		switch c := src[i+1]; {
//line internal/web/web.w:509
		case c == '@':
//line internal/web/web.w:510
			i += 2
//line internal/web/web.w:511
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
//line internal/web/web.w:512
			return ctrl{kind: cSection, pos: i, end: i + 2, depth: -1}
//line internal/web/web.w:513
		case c == '*':
//line internal/web/web.w:514
			j := i + 2
//line internal/web/web.w:515
			depth := 0
//line internal/web/web.w:516
			if j < n && src[j] == '*' {
//line internal/web/web.w:517
				j++
//line internal/web/web.w:518
				depth = -1 // "@**" is the top level: bold in the contents, as cweb
//line internal/web/web.w:519
			} else {
//line internal/web/web.w:520
				for j < n && src[j] >= '0' && src[j] <= '9' {
//line internal/web/web.w:521
					depth = depth*10 + int(src[j]-'0')
//line internal/web/web.w:522
					j++
//line internal/web/web.w:523
				}
//line internal/web/web.w:524
			}
//line internal/web/web.w:525
			return ctrl{kind: cSection, pos: i, end: j, depth: depth, starred: true}
//line internal/web/web.w:526
		case c == '<' || c == '(' || c == '=' || c == 't' || c == '^' || c == '.' || c == ':' || c == 'q':
//line internal/web/web.w:527
			end := indexFrom(src, "@>", i+2)
//line internal/web/web.w:528
			if end < 0 {
//line internal/web/web.w:529
				return ctrl{kind: cEOF, pos: n, end: n}
//line internal/web/web.w:530
			}
//line internal/web/web.w:531
			i = end + 2
//line internal/web/web.w:532
		case c == '%':
//line internal/web/web.w:533
			j := i + 2
//line internal/web/web.w:534
			for j < n && src[j] != '\n' {
//line internal/web/web.w:535
				j++
//line internal/web/web.w:536
			}
//line internal/web/web.w:537
			i = j
//line internal/web/web.w:538
		default:
//line internal/web/web.w:539
			i += 2
//line internal/web/web.w:540
		}
//line internal/web/web.w:541
	}
//line internal/web/web.w:542
	return ctrl{kind: cEOF, pos: n, end: n}
//line internal/web/web.w:543
}

// parse splits source into limbo and sections.
//
//line internal/web/web.w:548
//line internal/web/web.w:549
func parse(src string) *Web {
//line internal/web/web.w:550
	w := &Web{}
//line internal/web/web.w:551
	n := len(src)

//line internal/web/web.w:553
	// Limbo runs until the first section break. Format directives placed there
//line internal/web/web.w:554
	// (@f / @s, a common CWEB idiom) are extracted and removed from the copied
//line internal/web/web.w:555
	// TeX so they apply globally rather than printing literally.
//line internal/web/web.w:556
	first := findNextSection(src, 0)
//line internal/web/web.w:557
	w.Limbo, w.Formats = extractLimboFormats(src[:first.pos])
//line internal/web/web.w:558
	i := first.pos

//line internal/web/web.w:560
	num := 0
//line internal/web/web.w:561
	for i < n {
//line internal/web/web.w:562
		// We are positioned at a section break.
//line internal/web/web.w:563
		hdr := src[i+1]
//line internal/web/web.w:564
		num++
//line internal/web/web.w:565
		sec := &Section{Number: num, Line: lineAt(src, i)}
//line internal/web/web.w:566
		if hdr == '*' {
//line internal/web/web.w:567
			h := findSectionHeaderEnd(src, i)
//line internal/web/web.w:568
			sec.Starred = true
//line internal/web/web.w:569
			sec.Depth = h.depth
//line internal/web/web.w:570
			i = h.end
//line internal/web/web.w:571
		} else {
//line internal/web/web.w:572
			i += 2
//line internal/web/web.w:573
		}

//line internal/web/web.w:575
		// TeX part: from here to the next structural control.
//line internal/web/web.w:576
		ct := scanStruct(src, i)
//line internal/web/web.w:577
		sec.Tex = src[i:ct.pos]
//line internal/web/web.w:578
		if sec.Starred {
//line internal/web/web.w:579
			sec.Title = extractTitle(sec.Tex)
//line internal/web/web.w:580
		}

//line internal/web/web.w:582
		// Definition part: a run of @d / @f / @s.
//line internal/web/web.w:583
		for ct.kind == cDefn || ct.kind == cFormat {
//line internal/web/web.w:584
			nx := scanStruct(src, ct.end)
//line internal/web/web.w:585
			seg := src[ct.end:nx.pos]
//line internal/web/web.w:586
			// @d has no Go analogue (Go has no preprocessor), so it never tangles
//line internal/web/web.w:587
			// to code; gweave uses it only to set the named identifier in
//line internal/web/web.w:588
			// typewriter, as cweave sets a macro. @f/@s format like another word.
//line internal/web/web.w:589
			if ct.kind == cDefn {
//line internal/web/web.w:590
				if f, ok := parseMacro(seg); ok {
//line internal/web/web.w:591
					sec.Formats = append(sec.Formats, f)
//line internal/web/web.w:592
				}
//line internal/web/web.w:593
			} else if f, ok := parseFormat(seg, ct.noIndex); ok {
//line internal/web/web.w:594
				sec.Formats = append(sec.Formats, f)
//line internal/web/web.w:595
			}
//line internal/web/web.w:596
			ct = nx
//line internal/web/web.w:597
		}

//line internal/web/web.w:599
		switch ct.kind {
//line internal/web/web.w:600
		case cCode:
//line internal/web/web.w:601
			sec.HasCode = true
//line internal/web/web.w:602
			sec.CodeLine = lineAt(src, ct.end)
//line internal/web/web.w:603
			nx := findNextSection(src, ct.end)
//line internal/web/web.w:604
			sec.Code = src[ct.end:nx.pos]
//line internal/web/web.w:605
			i = nx.pos
//line internal/web/web.w:606
		case cNamed:
//line internal/web/web.w:607
			sec.HasCode = true
//line internal/web/web.w:608
			sec.Name = ct.name
//line internal/web/web.w:609
			sec.IsFile = ct.isFile
//line internal/web/web.w:610
			sec.CodeLine = lineAt(src, ct.end)
//line internal/web/web.w:611
			nx := findNextSection(src, ct.end)
//line internal/web/web.w:612
			sec.Code = src[ct.end:nx.pos]
//line internal/web/web.w:613
			i = nx.pos
//line internal/web/web.w:614
		default: // cSection or cEOF: a documentation-only section
//line internal/web/web.w:615
			i = ct.pos
//line internal/web/web.w:616
		}

//line internal/web/web.w:618
		w.Sections = append(w.Sections, sec)
//line internal/web/web.w:619
		if ct.kind == cEOF && sec.Code == "" {
//line internal/web/web.w:620
			break
//line internal/web/web.w:621
		}
//line internal/web/web.w:622
		if i >= n {
//line internal/web/web.w:623
			break
//line internal/web/web.w:624
		}
//line internal/web/web.w:625
	}
//line internal/web/web.w:626
	return w
//line internal/web/web.w:627
}

//line internal/web/web.w:631
func findSectionHeaderEnd(src string, i int) ctrl {
//line internal/web/web.w:632
	n := len(src)
//line internal/web/web.w:633
	j := i + 2
//line internal/web/web.w:634
	depth := 0
//line internal/web/web.w:635
	if j < n && src[j] == '*' {
//line internal/web/web.w:636
		j++
//line internal/web/web.w:637
		depth = -1 // "@**" is the top level: bold in the contents, as cweb
//line internal/web/web.w:638
	} else {
//line internal/web/web.w:639
		for j < n && src[j] >= '0' && src[j] <= '9' {
//line internal/web/web.w:640
			depth = depth*10 + int(src[j]-'0')
//line internal/web/web.w:641
			j++
//line internal/web/web.w:642
		}
//line internal/web/web.w:643
	}
//line internal/web/web.w:644
	return ctrl{end: j, depth: depth}
//line internal/web/web.w:645
}

// extractTitle returns the text of a starred section up to its terminating
// period, with whitespace collapsed, for use in the table of contents. The
// terminator is the first period at end of text or followed by whitespace, so a
// period inside a control sequence such as \.{web} does not end the title early.
//
//line internal/web/web.w:650
//line internal/web/web.w:651
//line internal/web/web.w:652
//line internal/web/web.w:653
//line internal/web/web.w:654
func extractTitle(tex string) string {
//line internal/web/web.w:655
	t := strings.TrimLeft(tex, " \t\n")
//line internal/web/web.w:656
	if i := titleEnd(t); i >= 0 {
//line internal/web/web.w:657
		t = t[:i]
//line internal/web/web.w:658
	}
//line internal/web/web.w:659
	return strings.Join(strings.Fields(t), " ")
//line internal/web/web.w:660
}

// titleEnd returns the index of the period that ends a starred-section title --
// the first '.' at end of s or followed by whitespace -- or -1 if there is none.
//
//line internal/web/web.w:662
//line internal/web/web.w:663
//line internal/web/web.w:664
func titleEnd(s string) int {
//line internal/web/web.w:665
	for i := 0; i < len(s); i++ {
//line internal/web/web.w:666
		if s[i] == '.' && (i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t' ||
//line internal/web/web.w:667
			s[i+1] == '\n' || s[i+1] == '\r') {
//line internal/web/web.w:668
			return i
//line internal/web/web.w:669
		}
//line internal/web/web.w:670
	}
//line internal/web/web.w:671
	return -1
//line internal/web/web.w:672
}

// scanDiagnostics walks the source looking for malformed control codes —
// currently argument-terminated codes (@<, @(, @=, @t, @^, @., @:, @q) that are
// missing their closing @> — and returns one warning per problem.
//
//line internal/web/web.w:677
//line internal/web/web.w:678
//line internal/web/web.w:679
//line internal/web/web.w:680
func (w *Web) scanDiagnostics(src string) []string {
//line internal/web/web.w:681
	var warns []string
//line internal/web/web.w:682
	n := len(src)
//line internal/web/web.w:683
	i := 0
//line internal/web/web.w:684
	for i < n {
//line internal/web/web.w:685
		if src[i] != '@' || i+1 >= n {
//line internal/web/web.w:686
			i++
//line internal/web/web.w:687
			continue
//line internal/web/web.w:688
		}
//line internal/web/web.w:689
		switch c := src[i+1]; c {
//line internal/web/web.w:690
		case '@':
//line internal/web/web.w:691
			i += 2
//line internal/web/web.w:692
		case '<', '(', '=', 't', '^', '.', ':', 'q':
//line internal/web/web.w:693
			if end := indexFrom(src, "@>", i+2); end < 0 {
//line internal/web/web.w:694
				warns = append(warns, fmt.Sprintf("%s: unterminated `@%c ... @>'", w.at(lineAt(src, i)), c))
//line internal/web/web.w:695
				i = n
//line internal/web/web.w:696
			} else {
//line internal/web/web.w:697
				i = end + 2
//line internal/web/web.w:698
			}
//line internal/web/web.w:699
		default:
//line internal/web/web.w:700
			i += 2
//line internal/web/web.w:701
		}
//line internal/web/web.w:702
	}
//line internal/web/web.w:703
	return warns
//line internal/web/web.w:704
}

// parseFormat parses the body of an @f/@s directive: two identifiers.
//
//line internal/web/web.w:708
//line internal/web/web.w:709
func parseFormat(seg string, noIndex bool) (Format, bool) {
//line internal/web/web.w:710
	fields := strings.Fields(seg)
//line internal/web/web.w:711
	if len(fields) < 2 {
//line internal/web/web.w:712
		return Format{}, false
//line internal/web/web.w:713
	}
//line internal/web/web.w:714
	return Format{Original: fields[0], Like: fields[1], NoIndex: noIndex}, true
//line internal/web/web.w:715
}

// parseMacro parses an @d directive: its first word names a constant to set in
// typewriter; any value after it is ignored (Go has no preprocessor). A
// qualified name keeps its final component, so "@d http.StatusOK" and
// "@d StatusOK" both register StatusOK.
//
//line internal/web/web.w:723
//line internal/web/web.w:724
//line internal/web/web.w:725
//line internal/web/web.w:726
//line internal/web/web.w:727
func parseMacro(seg string) (Format, bool) {
//line internal/web/web.w:728
	fields := strings.Fields(seg)
//line internal/web/web.w:729
	if len(fields) == 0 {
//line internal/web/web.w:730
		return Format{}, false
//line internal/web/web.w:731
	}
//line internal/web/web.w:732
	name := fields[0]
//line internal/web/web.w:733
	if k := strings.LastIndex(name, "."); k >= 0 {
//line internal/web/web.w:734
		name = name[k+1:]
//line internal/web/web.w:735
	}
//line internal/web/web.w:736
	if name == "" {
//line internal/web/web.w:737
		return Format{}, false
//line internal/web/web.w:738
	}
//line internal/web/web.w:739
	return Format{Original: name, Macro: true}, true
//line internal/web/web.w:740
}

// extractLimboFormats pulls @d/@f/@s directives out of the limbo text
// (consuming each to end of line) and returns the cleaned text together with the
// formats. Other control codes and argument-terminated groups are copied through.
//
//line internal/web/web.w:746
//line internal/web/web.w:747
//line internal/web/web.w:748
//line internal/web/web.w:749
func extractLimboFormats(src string) (string, []Format) {
//line internal/web/web.w:750
	var b strings.Builder
//line internal/web/web.w:751
	var formats []Format
//line internal/web/web.w:752
	n := len(src)
//line internal/web/web.w:753
	i := 0
//line internal/web/web.w:754
	for i < n {
//line internal/web/web.w:755
		if src[i] != '@' || i+1 >= n {
//line internal/web/web.w:756
			b.WriteByte(src[i])
//line internal/web/web.w:757
			i++
//line internal/web/web.w:758
			continue
//line internal/web/web.w:759
		}
//line internal/web/web.w:760
		switch c := src[i+1]; c {
//line internal/web/web.w:761
		case '@':
//line internal/web/web.w:762
			b.WriteString("@@")
//line internal/web/web.w:763
			i += 2
//line internal/web/web.w:764
		case 'd', 'f', 's':
//line internal/web/web.w:765
			j := i + 2
//line internal/web/web.w:766
			for j < n && src[j] != '\n' {
//line internal/web/web.w:767
				j++
//line internal/web/web.w:768
			}
//line internal/web/web.w:769
			var f Format
//line internal/web/web.w:770
			var ok bool
//line internal/web/web.w:771
			if c == 'd' {
//line internal/web/web.w:772
				f, ok = parseMacro(src[i+2 : j])
//line internal/web/web.w:773
			} else {
//line internal/web/web.w:774
				f, ok = parseFormat(src[i+2:j], c == 's')
//line internal/web/web.w:775
			}
//line internal/web/web.w:776
			if ok {
//line internal/web/web.w:777
				formats = append(formats, f)
//line internal/web/web.w:778
			}
//line internal/web/web.w:779
			if j < n {
//line internal/web/web.w:780
				j++ // also drop the newline that ended the directive
//line internal/web/web.w:781
			}
//line internal/web/web.w:782
			i = j
//line internal/web/web.w:783
		case '<', '(', '=', 't', '^', '.', ':', 'q':
//line internal/web/web.w:784
			end := indexFrom(src, "@>", i+2)
//line internal/web/web.w:785
			if end < 0 {
//line internal/web/web.w:786
				b.WriteString(src[i:])
//line internal/web/web.w:787
				i = n
//line internal/web/web.w:788
			} else {
//line internal/web/web.w:789
				b.WriteString(src[i : end+2])
//line internal/web/web.w:790
				i = end + 2
//line internal/web/web.w:791
			}
//line internal/web/web.w:792
		default:
//line internal/web/web.w:793
			b.WriteString(src[i : i+2])
//line internal/web/web.w:794
			i += 2
//line internal/web/web.w:795
		}
//line internal/web/web.w:796
	}
//line internal/web/web.w:797
	return b.String(), formats
//line internal/web/web.w:798
}

// AtomKind classifies a piece of a code part.
//
//line internal/web/web.w:805
//line internal/web/web.w:806
type AtomKind int

//line internal/web/web.w:808
const (
//line internal/web/web.w:809
	AText AtomKind = iota // ordinary Go source text
//line internal/web/web.w:810
	ARef // @<name@> reference to a named section
//line internal/web/web.w:811
	AVerbatim // @=text@> passed verbatim to tangled output
//line internal/web/web.w:812
	ATeX // @t text@> TeX text for the woven output
//line internal/web/web.w:813
	AIndex // @^/@./@: index entry
//line internal/web/web.w:814
	APaste // @& join (delete surrounding whitespace)
//line internal/web/web.w:815
	ALayout // @, @/ @| @# woven-output layout hints
//line internal/web/web.w:816
	AIndexDef // @! force the next identifier to index as a definition
//line internal/web/web.w:817
)

// Atom is one element of a scanned code part.
//
//line internal/web/web.w:819
//line internal/web/web.w:820
type Atom struct {
//line internal/web/web.w:821
	Kind AtomKind
//line internal/web/web.w:822
	Text string // payload for AText/AVerbatim/ATeX/AIndex; name for ARef
//line internal/web/web.w:823
	Index byte // '^','.',':' for AIndex; ',' '/' '|' '#' for ALayout
//line internal/web/web.w:824
}

// ScanCode splits a raw code part into atoms, interpreting in-code control
// codes. "@@" becomes a literal '@' folded into the surrounding text.
//
//line internal/web/web.w:830
//line internal/web/web.w:831
//line internal/web/web.w:832
func ScanCode(code string) []Atom {
//line internal/web/web.w:833
	var atoms []Atom
//line internal/web/web.w:834
	var buf strings.Builder
//line internal/web/web.w:835
	flush := func() {
//line internal/web/web.w:836
		if buf.Len() > 0 {
//line internal/web/web.w:837
			atoms = append(atoms, Atom{Kind: AText, Text: buf.String()})
//line internal/web/web.w:838
			buf.Reset()
//line internal/web/web.w:839
		}
//line internal/web/web.w:840
	}
//line internal/web/web.w:841
	n := len(code)
//line internal/web/web.w:842
	i := 0
//line internal/web/web.w:843
	for i < n {
//line internal/web/web.w:844
		c := code[i]
//line internal/web/web.w:845
		if c != '@' || i+1 >= n {
//line internal/web/web.w:846
			buf.WriteByte(c)
//line internal/web/web.w:847
			i++
//line internal/web/web.w:848
			continue
//line internal/web/web.w:849
		}
//line internal/web/web.w:850
		switch d := code[i+1]; d {
//line internal/web/web.w:851
		case '@':
//line internal/web/web.w:852
			buf.WriteByte('@')
//line internal/web/web.w:853
			i += 2
//line internal/web/web.w:854
		case '&':
//line internal/web/web.w:855
			flush()
//line internal/web/web.w:856
			atoms = append(atoms, Atom{Kind: APaste})
//line internal/web/web.w:857
			i += 2
//line internal/web/web.w:858
		case '<':
//line internal/web/web.w:859
			end := indexFrom(code, "@>", i+2)
//line internal/web/web.w:860
			if end < 0 {
//line internal/web/web.w:861
				buf.WriteString(code[i:])
//line internal/web/web.w:862
				i = n
//line internal/web/web.w:863
				continue
//line internal/web/web.w:864
			}
//line internal/web/web.w:865
			flush()
//line internal/web/web.w:866
			atoms = append(atoms, Atom{Kind: ARef, Text: canonName(code[i+2 : end])})
//line internal/web/web.w:867
			i = end + 2
//line internal/web/web.w:868
		case '=':
//line internal/web/web.w:869
			end := indexFrom(code, "@>", i+2)
//line internal/web/web.w:870
			if end < 0 {
//line internal/web/web.w:871
				i = n
//line internal/web/web.w:872
				continue
//line internal/web/web.w:873
			}
//line internal/web/web.w:874
			flush()
//line internal/web/web.w:875
			atoms = append(atoms, Atom{Kind: AVerbatim, Text: code[i+2 : end]})
//line internal/web/web.w:876
			i = end + 2
//line internal/web/web.w:877
		case 't':
//line internal/web/web.w:878
			end := indexFrom(code, "@>", i+2)
//line internal/web/web.w:879
			if end < 0 {
//line internal/web/web.w:880
				i = n
//line internal/web/web.w:881
				continue
//line internal/web/web.w:882
			}
//line internal/web/web.w:883
			flush()
//line internal/web/web.w:884
			atoms = append(atoms, Atom{Kind: ATeX, Text: code[i+2 : end]})
//line internal/web/web.w:885
			i = end + 2
//line internal/web/web.w:886
		case '^', '.', ':':
//line internal/web/web.w:887
			end := indexFrom(code, "@>", i+2)
//line internal/web/web.w:888
			if end < 0 {
//line internal/web/web.w:889
				i = n
//line internal/web/web.w:890
				continue
//line internal/web/web.w:891
			}
//line internal/web/web.w:892
			flush()
//line internal/web/web.w:893
			atoms = append(atoms, Atom{Kind: AIndex, Text: code[i+2 : end], Index: d})
//line internal/web/web.w:894
			i = end + 2
//line internal/web/web.w:895
		case 'q':
//line internal/web/web.w:896
			end := indexFrom(code, "@>", i+2)
//line internal/web/web.w:897
			if end < 0 {
//line internal/web/web.w:898
				i = n
//line internal/web/web.w:899
				continue
//line internal/web/web.w:900
			}
//line internal/web/web.w:901
			i = end + 2 // ignored material
//line internal/web/web.w:902
		case '%':
//line internal/web/web.w:903
			j := i + 2
//line internal/web/web.w:904
			for j < n && code[j] != '\n' {
//line internal/web/web.w:905
				j++
//line internal/web/web.w:906
			}
//line internal/web/web.w:907
			i = j
//line internal/web/web.w:908
		case '>':
//line internal/web/web.w:909
			i += 2 // stray terminator
//line internal/web/web.w:910
		case ',', '/', '|', '#':
//line internal/web/web.w:911
			// Woven-output layout hints: thin space, line break, optional line
//line internal/web/web.w:912
			// break, and break-plus-blank-line. Ignored by gtangle.
//line internal/web/web.w:913
			flush()
//line internal/web/web.w:914
			atoms = append(atoms, Atom{Kind: ALayout, Index: d})
//line internal/web/web.w:915
			i += 2
//line internal/web/web.w:916
		case '!':
//line internal/web/web.w:917
			// Force the next identifier's index entry to be a definition,
//line internal/web/web.w:918
			// overriding the heuristic. Produces no output by itself.
//line internal/web/web.w:919
			flush()
//line internal/web/web.w:920
			atoms = append(atoms, Atom{Kind: AIndexDef})
//line internal/web/web.w:921
			i += 2
//line internal/web/web.w:922
		case '+', '[', ']', ';':
//line internal/web/web.w:923
			// CWEB prettyprinter hints (cancel break, expression brackets,
//line internal/web/web.w:924
			// invisible semicolon). GWEB mirrors the source instead of reflowing
//line internal/web/web.w:925
			// it, so these have no effect; accept and drop them for portability.
//line internal/web/web.w:926
			i += 2
//line internal/web/web.w:927
		default:
//line internal/web/web.w:928
			i += 2 // unknown @x: drop it rather than corrupt the output
//line internal/web/web.w:929
		}
//line internal/web/web.w:930
	}
//line internal/web/web.w:931
	flush()
//line internal/web/web.w:932
	return atoms
//line internal/web/web.w:933
}

//line internal/web/web.w:940
// A change file (CWEB's ".ch" mechanism) patches the master source without
//line internal/web/web.w:941
// editing it. It is a sequence of changes, each of the form
//line internal/web/web.w:942
//
//line internal/web/web.w:943
//	@x
//line internal/web/web.w:944
//	<lines to find in the master source>
//line internal/web/web.w:945
//	@y
//line internal/web/web.w:946
//	<lines to substitute>
//line internal/web/web.w:947
//	@z
//line internal/web/web.w:948
//
//line internal/web/web.w:949
// Text outside an @x...@z group is ignored (it serves as commentary). Changes
//line internal/web/web.w:950
// are matched against the master source — after @i includes are expanded — in
//line internal/web/web.w:951
// the order they appear: GWEB scans the master line by line, and at the first
//line internal/web/web.w:952
// line equal to a change's first match line it requires the whole match block
//line internal/web/web.w:953
// to match, then substitutes the replacement lines.

//line internal/web/web.w:959
type change struct {
//line internal/web/web.w:960
	match []string // lines to find in the master source
//line internal/web/web.w:961
	repl []string // lines to substitute for them
//line internal/web/web.w:962
	line int // 1-based line of the @x in the change file (for diagnostics)
//line internal/web/web.w:963
	replLine int // 1-based change-file line of the first replacement line
//line internal/web/web.w:964
}

// srcLoc identifies the origin (file and 1-based line) of a line of the
// includes-expanded, change-applied source, so diagnostics can point back to
// the file the user actually wrote.
//
//line internal/web/web.w:966
//line internal/web/web.w:967
//line internal/web/web.w:968
//line internal/web/web.w:969
type srcLoc struct {
//line internal/web/web.w:970
	file string
//line internal/web/web.w:971
	line int
//line internal/web/web.w:972
}

//line internal/web/web.w:974
func (l srcLoc) String() string {
//line internal/web/web.w:975
	if l.file == "" {
//line internal/web/web.w:976
		return fmt.Sprintf("line %d", l.line)
//line internal/web/web.w:977
	}
//line internal/web/web.w:978
	return fmt.Sprintf("%s:%d", l.file, l.line)
//line internal/web/web.w:979
}

// isChangeCtrl reports whether line begins with the change control "@<c>"
// (c is 'x', 'y', or 'z'), which must start in the first column.
//
//line internal/web/web.w:984
//line internal/web/web.w:985
//line internal/web/web.w:986
func isChangeCtrl(line string, c byte) bool {
//line internal/web/web.w:987
	return len(line) >= 2 && line[0] == '@' && line[1] == c
//line internal/web/web.w:988
}

// splitLines splits text into lines, normalizing CRLF, so that joining the
// result with "\n" reproduces the (LF-normalized) input.
//
//line internal/web/web.w:990
//line internal/web/web.w:991
//line internal/web/web.w:992
func splitLines(s string) []string {
//line internal/web/web.w:993
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
//line internal/web/web.w:994
}

// sameLine compares two source lines for change matching, ignoring trailing
// whitespace (as WEB does).
//
//line internal/web/web.w:996
//line internal/web/web.w:997
//line internal/web/web.w:998
func sameLine(a, b string) bool {
//line internal/web/web.w:999
	return strings.TrimRight(a, " \t") == strings.TrimRight(b, " \t")
//line internal/web/web.w:1000
}

// parseChangeFile parses change-file text into an ordered list of changes.
//
//line internal/web/web.w:1005
//line internal/web/web.w:1006
func parseChangeFile(src string) ([]change, error) {
//line internal/web/web.w:1007
	lines := splitLines(src)
//line internal/web/web.w:1008
	var changes []change
//line internal/web/web.w:1009
	n := len(lines)
//line internal/web/web.w:1010
	for i := 0; i < n; {
//line internal/web/web.w:1011
		if !isChangeCtrl(lines[i], 'x') {
//line internal/web/web.w:1012
			i++ // commentary between changes
//line internal/web/web.w:1013
			continue
//line internal/web/web.w:1014
		}
//line internal/web/web.w:1015
		c := change{line: i + 1}
//line internal/web/web.w:1016
		i++
//line internal/web/web.w:1017
		for i < n && !isChangeCtrl(lines[i], 'y') {
//line internal/web/web.w:1018
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'z') {
//line internal/web/web.w:1019
				return nil, fmt.Errorf("change file line %d: expected @y to close the @x match part", c.line)
//line internal/web/web.w:1020
			}
//line internal/web/web.w:1021
			c.match = append(c.match, lines[i])
//line internal/web/web.w:1022
			i++
//line internal/web/web.w:1023
		}
//line internal/web/web.w:1024
		if i >= n {
//line internal/web/web.w:1025
			return nil, fmt.Errorf("change file line %d: @x without a matching @y", c.line)
//line internal/web/web.w:1026
		}
//line internal/web/web.w:1027
		i++ // skip @y
//line internal/web/web.w:1028
		c.replLine = i + 1
//line internal/web/web.w:1029
		for i < n && !isChangeCtrl(lines[i], 'z') {
//line internal/web/web.w:1030
			if isChangeCtrl(lines[i], 'x') || isChangeCtrl(lines[i], 'y') {
//line internal/web/web.w:1031
				return nil, fmt.Errorf("change file line %d: expected @z to close the change", c.line)
//line internal/web/web.w:1032
			}
//line internal/web/web.w:1033
			c.repl = append(c.repl, lines[i])
//line internal/web/web.w:1034
			i++
//line internal/web/web.w:1035
		}
//line internal/web/web.w:1036
		if i >= n {
//line internal/web/web.w:1037
			return nil, fmt.Errorf("change file line %d: change has no @z", c.line)
//line internal/web/web.w:1038
		}
//line internal/web/web.w:1039
		i++ // skip @z
//line internal/web/web.w:1040
		if len(c.match) == 0 {
//line internal/web/web.w:1041
			return nil, fmt.Errorf("change file line %d: the @x match part is empty", c.line)
//line internal/web/web.w:1042
		}
//line internal/web/web.w:1043
		changes = append(changes, c)
//line internal/web/web.w:1044
	}
//line internal/web/web.w:1045
	return changes, nil
//line internal/web/web.w:1046
}

// applyChanges returns src with the changes applied (string convenience form,
// used by tests). See applyChangesMapped for the origin-tracking version.
//
//line internal/web/web.w:1050
//line internal/web/web.w:1051
//line internal/web/web.w:1052
func applyChanges(src string, changes []change, chFile string) (string, error) {
//line internal/web/web.w:1053
	out, _, err := applyChangesMapped(splitLines(src), nil, changes, chFile)
//line internal/web/web.w:1054
	if err != nil {
//line internal/web/web.w:1055
		return "", err
//line internal/web/web.w:1056
	}
//line internal/web/web.w:1057
	return strings.Join(out, "\n"), nil
//line internal/web/web.w:1058
}

// applyChangesMapped applies changes to master, keeping a parallel origin map in
// step: passed-through lines keep their origin, and replacement lines are
// attributed to the change file. locs may be nil if origins are not tracked.
// chFile names the change file for diagnostics. It is an error if a change's
// first line is never found, or is found but the rest of the block does not
// match.
//
//line internal/web/web.w:1065
//line internal/web/web.w:1066
//line internal/web/web.w:1067
//line internal/web/web.w:1068
//line internal/web/web.w:1069
//line internal/web/web.w:1070
//line internal/web/web.w:1071
func applyChangesMapped(master []string, locs []srcLoc, changes []change, chFile string) ([]string, []srcLoc, error) {
//line internal/web/web.w:1072
	loc := func(i int) srcLoc {
//line internal/web/web.w:1073
		if locs != nil && i < len(locs) {
//line internal/web/web.w:1074
			return locs[i]
//line internal/web/web.w:1075
		}
//line internal/web/web.w:1076
		return srcLoc{line: i + 1}
//line internal/web/web.w:1077
	}
//line internal/web/web.w:1078
	out := make([]string, 0, len(master))
//line internal/web/web.w:1079
	var outLocs []srcLoc
//line internal/web/web.w:1080
	ci := 0
//line internal/web/web.w:1081
	for i := 0; i < len(master); {
//line internal/web/web.w:1082
		if ci < len(changes) && sameLine(master[i], changes[ci].match[0]) {
//line internal/web/web.w:1083
			if !blockMatches(master, i, changes[ci].match) {
//line internal/web/web.w:1084
				return nil, nil, fmt.Errorf("%s:%d: change did not match the master source at %s",
//line internal/web/web.w:1085
					chFile, changes[ci].line, loc(i))
//line internal/web/web.w:1086
			}
//line internal/web/web.w:1087
			for r, rl := range changes[ci].repl {
//line internal/web/web.w:1088
				out = append(out, rl)
//line internal/web/web.w:1089
				outLocs = append(outLocs, srcLoc{chFile, changes[ci].replLine + r})
//line internal/web/web.w:1090
			}
//line internal/web/web.w:1091
			i += len(changes[ci].match)
//line internal/web/web.w:1092
			ci++
//line internal/web/web.w:1093
			continue
//line internal/web/web.w:1094
		}
//line internal/web/web.w:1095
		out = append(out, master[i])
//line internal/web/web.w:1096
		outLocs = append(outLocs, loc(i))
//line internal/web/web.w:1097
		i++
//line internal/web/web.w:1098
	}
//line internal/web/web.w:1099
	if ci < len(changes) {
//line internal/web/web.w:1100
		return nil, nil, fmt.Errorf("%s:%d: change was never matched (looking for %q)",
//line internal/web/web.w:1101
			chFile, changes[ci].line, changes[ci].match[0])
//line internal/web/web.w:1102
	}
//line internal/web/web.w:1103
	return out, outLocs, nil
//line internal/web/web.w:1104
}

// blockMatches reports whether match lines up with master starting at index at.
//
//line internal/web/web.w:1109
//line internal/web/web.w:1110
func blockMatches(master []string, at int, match []string) bool {
//line internal/web/web.w:1111
	if at+len(match) > len(master) {
//line internal/web/web.w:1112
		return false
//line internal/web/web.w:1113
	}
//line internal/web/web.w:1114
	for k, m := range match {
//line internal/web/web.w:1115
		if !sameLine(master[at+k], m) {
//line internal/web/web.w:1116
			return false
//line internal/web/web.w:1117
		}
//line internal/web/web.w:1118
	}
//line internal/web/web.w:1119
	return true
//line internal/web/web.w:1120
}
