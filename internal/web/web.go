// Package web parses GWEB source files (.w) into a sequence of sections that
// gtangle and gweave consume. It is the shared front end of the GWEB system,
// playing the role of CWEB's common.w.
//
//line lit/web.w:11
//line lit/web.w:12
//line lit/web.w:13
//line lit/web.w:14
package web

//line lit/web.w:16
import (
//line lit/web.w:17
	"fmt"
//line lit/web.w:18
	"os"
//line lit/web.w:19
	"path/filepath"
//line lit/web.w:20
	"strings"
//line lit/web.w:21
)

// Version is the GWEB release, shared by gtangle and gweave for their startup
// banner and their --version output.
//
//line lit/web.w:23
//line lit/web.w:24
//line lit/web.w:25
const Version = "0.2.0"

// Format records an "@f a b" (or "@s a b") directive: typeset identifier
// Original the way identifier/keyword Like is typeset. NoIndex is true for @s.
//
//line lit/web.w:27
//line lit/web.w:28
//line lit/web.w:29
type Format struct {
//line lit/web.w:30
	Original string
//line lit/web.w:31
	Like string
//line lit/web.w:32
	NoIndex bool
//line lit/web.w:33
	Macro bool // : typeset Original in typewriter (a CWEB-style macro)
//line lit/web.w:34
}

// Section is one numbered section of the web.
//
//line lit/web.w:40
//line lit/web.w:41
type Section struct {
//line lit/web.w:42
	Number int // 1-based section number
//line lit/web.w:43
	Line int // 1-based source line where the section begins
//line lit/web.w:44
	Starred bool // true for @* sections
//line lit/web.w:45
	Depth int // group depth for starred sections (-1 == @**, 0 == @*, n == @*n)
//line lit/web.w:46
	Title string // starred-section title (text up to the first period)
//line lit/web.w:47
	Tex string // commentary, raw TeX with in-text @-codes still embedded
//line lit/web.w:48
	Formats []Format
//line lit/web.w:49
	HasCode bool // true if the section contributes code
//line lit/web.w:50
	Name string // named-section name, or "" for an unnamed @c section
//line lit/web.w:51
	IsFile bool // true if the name is an output file (@(file@>=)
//line lit/web.w:52
	Code string // raw code text with in-code @-codes still embedded
//line lit/web.w:53
	CodeLine int // 1-based combined-source line where Code begins (0 if none)
//line lit/web.w:54
}

// Web is a fully parsed GWEB document.
//
//line lit/web.w:61
//line lit/web.w:62
type Web struct {
//line lit/web.w:63
	Limbo string
//line lit/web.w:64
	Formats []Format // @f / @s directives found in limbo (apply globally)
//line lit/web.w:65
	Sections []*Section
//line lit/web.w:66
	Warnings []string // non-fatal diagnostics gathered while parsing/checking
//line lit/web.w:67
	file string // source filename, for diagnostics ("" if unknown)
//line lit/web.w:68
	locs []srcLoc // origin (file, line) of each combined-source line
//line lit/web.w:69
	full []string // canonical (non-abbreviated) section names
//line lit/web.w:70
}

// Parse reads filename, expands @i includes, and parses the result.
//
//line lit/web.w:77
//line lit/web.w:78
func Parse(filename string) (*Web, error) {
//line lit/web.w:79
	return ParseWithChange(filename, "")
//line lit/web.w:80
}

// ParseWithChange reads the master file, expands @i includes, applies the change
// file (CWEB's ".ch" mechanism) if changeFile is non-empty, and parses the
// result. Diagnostics point back to the original file and line via an origin map
// kept in step through include expansion and change application.
//
//line lit/web.w:82
//line lit/web.w:83
//line lit/web.w:84
//line lit/web.w:85
//line lit/web.w:86
func ParseWithChange(filename, changeFile string) (*Web, error) {
//line lit/web.w:87
	lines, locs, err := expandIncludes(filename, 0)
//line lit/web.w:88
	if err != nil {
//line lit/web.w:89
		return nil, err
//line lit/web.w:90
	}
//line lit/web.w:91
	if changeFile != "" {
//line lit/web.w:92
		chData, err := os.ReadFile(changeFile)
//line lit/web.w:93
		if err != nil {
//line lit/web.w:94
			return nil, err
//line lit/web.w:95
		}
//line lit/web.w:96
		changes, err := parseChangeFile(string(chData))
//line lit/web.w:97
		if err != nil {
//line lit/web.w:98
			return nil, err
//line lit/web.w:99
		}
//line lit/web.w:100
		lines, locs, err = applyChangesMapped(lines, locs, changes, changeFile)
//line lit/web.w:101
		if err != nil {
//line lit/web.w:102
			return nil, err
//line lit/web.w:103
		}
//line lit/web.w:104
	}
//line lit/web.w:105
	src := strings.Join(lines, "\n")
//line lit/web.w:106
	w := parse(src)
//line lit/web.w:107
	w.file = filename
//line lit/web.w:108
	w.locs = locs
//line lit/web.w:109
	w.finish(src)
//line lit/web.w:110
	return w, nil
//line lit/web.w:111
}

// ParseString parses already-loaded source (used by tests; no includes).
//
//line lit/web.w:113
//line lit/web.w:114
func ParseString(src string) *Web {
//line lit/web.w:115
	w := parse(src)
//line lit/web.w:116
	w.finish(src)
//line lit/web.w:117
	return w
//line lit/web.w:118
}

// finish runs post-parse bookkeeping: name collection and diagnostics.
//
//line lit/web.w:120
//line lit/web.w:121
func (w *Web) finish(src string) {
//line lit/web.w:122
	w.collectNames()
//line lit/web.w:123
	w.Warnings = append(w.Warnings, w.scanDiagnostics(src)...)
//line lit/web.w:124
	w.Warnings = append(w.Warnings, w.checkNames()...)
//line lit/web.w:125
}

// at formats a combined-source line for a diagnostic, mapping it back to the
// original file and line when an origin map is available.
//
//line lit/web.w:127
//line lit/web.w:128
//line lit/web.w:129
func (w *Web) at(line int) string {
//line lit/web.w:130
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line lit/web.w:131
		return w.locs[i].String()
//line lit/web.w:132
	}
//line lit/web.w:133
	if w.file != "" {
//line lit/web.w:134
		return fmt.Sprintf("%s:%d", w.file, line)
//line lit/web.w:135
	}
//line lit/web.w:136
	return fmt.Sprintf("line %d", line)
//line lit/web.w:137
}

// Origin maps a 1-based combined-source line back to the original file and line.
// When no origin map is available it falls back to the web's own filename.
//
//line lit/web.w:144
//line lit/web.w:145
//line lit/web.w:146
func (w *Web) Origin(line int) (file string, ln int) {
//line lit/web.w:147
	if i := line - 1; i >= 0 && i < len(w.locs) {
//line lit/web.w:148
		return w.locs[i].file, w.locs[i].line
//line lit/web.w:149
	}
//line lit/web.w:150
	return w.file, line
//line lit/web.w:151
}

// DefaultExt returns name with ext appended when name has no extension of its
// own (and is non-empty), so "wc" becomes "wc.w". A name that already carries an
// extension is left alone.
//
//line lit/web.w:157
//line lit/web.w:158
//line lit/web.w:159
//line lit/web.w:160
func DefaultExt(name, ext string) string {
//line lit/web.w:161
	if name == "" || filepath.Ext(name) != "" {
//line lit/web.w:162
		return name
//line lit/web.w:163
	}
//line lit/web.w:164
	return name + ext
//line lit/web.w:165
}

// expandIncludes reads file and splices in @i includes, returning the combined
// lines together with a parallel origin map. As in CWEB, @i is line-oriented: a
// line whose first non-blank text is "@i" (followed by whitespace) names a file
// whose expansion replaces that line. A final newline does not produce a
// trailing blank line.
//
//line lit/web.w:171
//line lit/web.w:172
//line lit/web.w:173
//line lit/web.w:174
//line lit/web.w:175
//line lit/web.w:176
func expandIncludes(file string, depth int) ([]string, []srcLoc, error) {
//line lit/web.w:177
	if depth > 25 {
//line lit/web.w:178
		return nil, nil, fmt.Errorf("gweb: @i include nesting too deep at %q", file)
//line lit/web.w:179
	}
//line lit/web.w:180
	data, err := os.ReadFile(file)
//line lit/web.w:181
	if err != nil {
//line lit/web.w:182
		return nil, nil, err
//line lit/web.w:183
	}
//line lit/web.w:184
	raw := splitLines(string(data))
//line lit/web.w:185
	if n := len(raw); n > 0 && raw[n-1] == "" {
//line lit/web.w:186
		raw = raw[:n-1]
//line lit/web.w:187
	}

//line lit/web.w:189
	var lines []string
//line lit/web.w:190
	var locs []srcLoc
//line lit/web.w:191
	dir := filepath.Dir(file)
//line lit/web.w:192
	for i, line := range raw {
//line lit/web.w:193
		if name, ok := includeDirective(line); ok {
//line lit/web.w:194
			path := name
//line lit/web.w:195
			if !filepath.IsAbs(path) {
//line lit/web.w:196
				path = filepath.Join(dir, name)
//line lit/web.w:197
			}
//line lit/web.w:198
			sub, subLocs, err := expandIncludes(path, depth+1)
//line lit/web.w:199
			if err != nil {
//line lit/web.w:200
				return nil, nil, fmt.Errorf("%s:%d: %w", file, i+1, err)
//line lit/web.w:201
			}
//line lit/web.w:202
			lines = append(lines, sub...)
//line lit/web.w:203
			locs = append(locs, subLocs...)
//line lit/web.w:204
			continue
//line lit/web.w:205
		}
//line lit/web.w:206
		lines = append(lines, line)
//line lit/web.w:207
		locs = append(locs, srcLoc{file, i + 1})
//line lit/web.w:208
	}
//line lit/web.w:209
	return lines, locs, nil
//line lit/web.w:210
}

// includeDirective returns the file named by an "@i" line, or ok=false. The @i
// must be the first non-blank text on the line and be followed by whitespace.
//
//line lit/web.w:212
//line lit/web.w:213
//line lit/web.w:214
func includeDirective(line string) (name string, ok bool) {
//line lit/web.w:215
	t := strings.TrimLeft(line, " \t")
//line lit/web.w:216
	if !strings.HasPrefix(t, "@i") {
//line lit/web.w:217
		return "", false
//line lit/web.w:218
	}
//line lit/web.w:219
	rest := t[2:]
//line lit/web.w:220
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
//line lit/web.w:221
		return "", false
//line lit/web.w:222
	}
//line lit/web.w:223
	name = strings.Trim(strings.TrimSpace(rest), "\"")
//line lit/web.w:224
	return name, name != ""
//line lit/web.w:225
}

// collectNames records the set of canonical (non-abbreviated) section names so
// abbreviations ending in "..." can be resolved. A full name may appear at a
// definition (@<name@>=) or at any reference (@<name@>), in code or in
// commentary, so all of those are scanned. Output files (@(...@>=) are roots,
// not referable refinements, so their names are excluded.
//
//line lit/web.w:232
//line lit/web.w:233
//line lit/web.w:234
//line lit/web.w:235
//line lit/web.w:236
//line lit/web.w:237
func (w *Web) collectNames() {
//line lit/web.w:238
	seen := map[string]bool{}
//line lit/web.w:239
	add := func(name string) {
//line lit/web.w:240
		if name != "" && !strings.HasSuffix(name, "...") && !seen[name] {
//line lit/web.w:241
			seen[name] = true
//line lit/web.w:242
			w.full = append(w.full, name)
//line lit/web.w:243
		}
//line lit/web.w:244
	}
//line lit/web.w:245
	for _, s := range w.Sections {
//line lit/web.w:246
		if !s.IsFile {
//line lit/web.w:247
			add(s.Name) // a definition's name
//line lit/web.w:248
		}
//line lit/web.w:249
		for _, raw := range []string{s.Code, s.Tex} {
//line lit/web.w:250
			for _, a := range ScanCode(raw) {
//line lit/web.w:251
				if a.Kind == ARef {
//line lit/web.w:252
					add(a.Text) // a reference's name
//line lit/web.w:253
				}
//line lit/web.w:254
			}
//line lit/web.w:255
		}
//line lit/web.w:256
	}
//line lit/web.w:257
}

// prefixMatches counts canonical names beginning with prefix.
//
//line lit/web.w:259
//line lit/web.w:260
func (w *Web) prefixMatches(prefix string) int {
//line lit/web.w:261
	n := 0
//line lit/web.w:262
	for _, full := range w.full {
//line lit/web.w:263
		if strings.HasPrefix(full, prefix) {
//line lit/web.w:264
			n++
//line lit/web.w:265
		}
//line lit/web.w:266
	}
//line lit/web.w:267
	return n
//line lit/web.w:268
}

// checkNames validates @<...@> references: it reports ambiguous or unmatched
// abbreviations, references to undefined sections, and named sections that are
// defined but never used. All are warnings (gtangle still fails hard if it
// actually meets an undefined reference while expanding).
//
//line lit/web.w:275
//line lit/web.w:276
//line lit/web.w:277
//line lit/web.w:278
//line lit/web.w:279
func (w *Web) checkNames() []string {
//line lit/web.w:280
	// "defined" is the set of sections that actually have a definition (not just
//line lit/web.w:281
	// the full names known for abbreviation resolution, which include references).
//line lit/web.w:282
	defined := map[string]bool{}
//line lit/web.w:283
	for _, s := range w.Sections {
//line lit/web.w:284
		if s.Name != "" && !s.IsFile {
//line lit/web.w:285
			defined[w.Resolve(s.Name)] = true
//line lit/web.w:286
		}
//line lit/web.w:287
	}
//line lit/web.w:288
	used := map[string]bool{}
//line lit/web.w:289
	var warns []string

//line lit/web.w:291
	for _, s := range w.Sections {
//line lit/web.w:292
		scan := func(raw string) {
//line lit/web.w:293
			for _, a := range ScanCode(raw) {
//line lit/web.w:294
				if a.Kind != ARef {
//line lit/web.w:295
					continue
//line lit/web.w:296
				}
//line lit/web.w:297
				canon := w.Resolve(a.Text)
//line lit/web.w:298
				if strings.HasSuffix(a.Text, "...") && canon == a.Text {
//line lit/web.w:299
					prefix := strings.TrimSpace(strings.TrimSuffix(a.Text, "..."))
//line lit/web.w:300
					if m := w.prefixMatches(prefix); m == 0 {
//line lit/web.w:301
						warns = append(warns, fmt.Sprintf("%s: no section name matches <%s>", w.at(s.Line), a.Text))
//line lit/web.w:302
					} else {
//line lit/web.w:303
						warns = append(warns, fmt.Sprintf("%s: ambiguous prefix <%s> matches %d section names", w.at(s.Line), a.Text, m))
//line lit/web.w:304
					}
//line lit/web.w:305
					continue
//line lit/web.w:306
				}
//line lit/web.w:307
				if !defined[canon] {
//line lit/web.w:308
					warns = append(warns, fmt.Sprintf("%s: reference to undefined section <%s>", w.at(s.Line), a.Text))
//line lit/web.w:309
				}
//line lit/web.w:310
				used[canon] = true
//line lit/web.w:311
			}
//line lit/web.w:312
		}
//line lit/web.w:313
		scan(s.Code)
//line lit/web.w:314
		scan(s.Tex)
//line lit/web.w:315
	}

//line lit/web.w:317
	warned := map[string]bool{}
//line lit/web.w:318
	for _, s := range w.Sections {
//line lit/web.w:319
		if s.Name == "" || s.IsFile {
//line lit/web.w:320
			continue
//line lit/web.w:321
		}
//line lit/web.w:322
		canon := w.Resolve(s.Name)
//line lit/web.w:323
		if !used[canon] && !warned[canon] {
//line lit/web.w:324
			warned[canon] = true
//line lit/web.w:325
			warns = append(warns, fmt.Sprintf("%s: section <%s> is defined but never used", w.at(s.Line), s.Name))
//line lit/web.w:326
		}
//line lit/web.w:327
	}
//line lit/web.w:328
	return warns
//line lit/web.w:329
}

// lineAt returns the 1-based line number of byte offset off in src.
//
//line lit/web.w:335
//line lit/web.w:336
func lineAt(src string, off int) int {
//line lit/web.w:337
	if off > len(src) {
//line lit/web.w:338
		off = len(src)
//line lit/web.w:339
	}
//line lit/web.w:340
	return 1 + strings.Count(src[:off], "\n")
//line lit/web.w:341
}

// canonName canonicalizes a section name's whitespace: every run of spaces,
// tabs, and newlines becomes a single space, and leading/trailing space is
// dropped. As in CWEB, this lets a long name that is wrapped across lines in one
// place still match the same name written on a single line elsewhere.
//
//line lit/web.w:343
//line lit/web.w:344
//line lit/web.w:345
//line lit/web.w:346
//line lit/web.w:347
func canonName(name string) string {
//line lit/web.w:348
	return strings.Join(strings.Fields(name), " ")
//line lit/web.w:349
}

// Resolve maps a (possibly abbreviated) name to its canonical form. An
// abbreviation "Prefix..." matches the unique full name starting with Prefix.
//
//line lit/web.w:351
//line lit/web.w:352
//line lit/web.w:353
func (w *Web) Resolve(name string) string {
//line lit/web.w:354
	name = canonName(name)
//line lit/web.w:355
	if !strings.HasSuffix(name, "...") {
//line lit/web.w:356
		return name
//line lit/web.w:357
	}
//line lit/web.w:358
	prefix := strings.TrimSpace(strings.TrimSuffix(name, "..."))
//line lit/web.w:359
	var match string
//line lit/web.w:360
	count := 0
//line lit/web.w:361
	for _, full := range w.full {
//line lit/web.w:362
		if strings.HasPrefix(full, prefix) {
//line lit/web.w:363
			match = full
//line lit/web.w:364
			count++
//line lit/web.w:365
		}
//line lit/web.w:366
	}
//line lit/web.w:367
	if count == 1 {
//line lit/web.w:368
		return match
//line lit/web.w:369
	}
//line lit/web.w:370
	return name // unresolved or ambiguous; leave as-is for caller to report
//line lit/web.w:371
}

//line lit/web.w:373
func indexFrom(s, sub string, from int) int {
//line lit/web.w:374
	if from >= len(s) {
//line lit/web.w:375
		return -1
//line lit/web.w:376
	}
//line lit/web.w:377
	idx := strings.Index(s[from:], sub)
//line lit/web.w:378
	if idx < 0 {
//line lit/web.w:379
		return -1
//line lit/web.w:380
	}
//line lit/web.w:381
	return from + idx
//line lit/web.w:382
}
