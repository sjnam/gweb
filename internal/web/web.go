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
