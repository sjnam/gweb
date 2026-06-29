package web

import (
	"os"
	"path/filepath"
	"testing"
)

const sample = `\input gwebmac
This is limbo text.

@* Introduction.
This program prints a greeting.
@f println foo

@ Here is the main function.
@c
package main

func main() {
	@<Print the greeting@>
}

@ The greeting itself.
@<Print the greeting@>=
println("hello, world")

@ A section with no code, just prose.

@*1 A deeper group.
@<Print the greeting@>=
println("again")
`

func TestParseStructure(t *testing.T) {
	w := ParseString(sample)

	if got := len(w.Sections); got != 5 {
		t.Fatalf("section count = %d, want 5", got)
	}

	s1 := w.Sections[0]
	if !s1.Starred || s1.Depth != 0 {
		t.Errorf("section 1: starred=%v depth=%d, want starred depth 0", s1.Starred, s1.Depth)
	}
	if s1.Title != "Introduction" {
		t.Errorf("section 1 title = %q, want %q", s1.Title, "Introduction")
	}
	if len(s1.Formats) != 1 || s1.Formats[0].Original != "println" || s1.Formats[0].Like != "foo" {
		t.Errorf("section 1 formats = %+v", s1.Formats)
	}

	s2 := w.Sections[1]
	if s2.Name != "" || !s2.HasCode {
		t.Errorf("section 2 should be unnamed code, got name=%q hasCode=%v", s2.Name, s2.HasCode)
	}
	if !contains(s2.Code, "package main") || !contains(s2.Code, "@<Print the greeting@>") {
		t.Errorf("section 2 code missing pieces:\n%s", s2.Code)
	}

	s3 := w.Sections[2]
	if s3.Name != "Print the greeting" {
		t.Errorf("section 3 name = %q", s3.Name)
	}

	s4 := w.Sections[3]
	if s4.HasCode {
		t.Errorf("section 4 should be prose-only, got code %q", s4.Code)
	}

	s5 := w.Sections[4]
	if !s5.Starred || s5.Depth != 1 {
		t.Errorf("section 5: starred=%v depth=%d, want starred depth 1", s5.Starred, s5.Depth)
	}
	if s5.Name != "Print the greeting" {
		t.Errorf("section 5 name = %q", s5.Name)
	}
}

func TestDoubleStarDepth(t *testing.T) {
	// @** is the top-level group (depth -1), printed bold in the contents, as
	// cweb does; @* stays depth 0 and @*n stays depth n.
	w := ParseString("@** Top.\n@c\npackage main\n@* Ordinary.\n@ x\n@*2 Deep.\n@ y\n")
	want := []int{-1, 0, 2}
	var got []int
	for _, s := range w.Sections {
		if s.Starred {
			got = append(got, s.Depth)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("starred sections = %v, want depths %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("starred section %d: depth=%d, want %d", i, got[i], want[i])
		}
	}
}

func TestResolveAbbrev(t *testing.T) {
	w := ParseString(sample)
	if got := w.Resolve("Print the..."); got != "Print the greeting" {
		t.Errorf("Resolve abbrev = %q, want %q", got, "Print the greeting")
	}
}

func TestCodePragmaP(t *testing.T) {
	// @p is a synonym for @c (CWEB compatibility).
	w := ParseString("@ x\n@p\npackage main\n")
	if len(w.Sections) != 1 || !w.Sections[0].HasCode {
		t.Fatalf("@p should begin a code section, got %+v", w.Sections)
	}
	if w.Sections[0].Name != "" {
		t.Errorf("@p section should be unnamed, got name %q", w.Sections[0].Name)
	}
	if !contains(w.Sections[0].Code, "package main") {
		t.Errorf("@p code missing: %q", w.Sections[0].Code)
	}
}

func TestDefaultExt(t *testing.T) {
	cases := []struct{ name, ext, want string }{
		{"wc", ".w", "wc.w"},         // bare name gets the extension
		{"wc.w", ".w", "wc.w"},       // already has one: unchanged
		{"foo.bar", ".w", "foo.bar"}, // a different extension is respected
		{"dir/wc", ".w", "dir/wc.w"}, // path components are fine
		{"", ".ch", ""},              // empty (e.g. no change file) stays empty
	}
	for _, c := range cases {
		if got := DefaultExt(c.name, c.ext); got != c.want {
			t.Errorf("DefaultExt(%q, %q) = %q, want %q", c.name, c.ext, got, c.want)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexFrom(s, sub, 0) >= 0
}

func TestLimboFormats(t *testing.T) {
	w := ParseString(`\input gwebmac
@f Counts int
@s hidden int
@ x
@c
package main
`)
	if len(w.Formats) != 2 {
		t.Fatalf("limbo formats = %d, want 2: %+v", len(w.Formats), w.Formats)
	}
	if w.Formats[0].Original != "Counts" || w.Formats[0].Like != "int" || w.Formats[0].NoIndex {
		t.Errorf("format[0] = %+v", w.Formats[0])
	}
	if w.Formats[1].Original != "hidden" || !w.Formats[1].NoIndex {
		t.Errorf("format[1] = %+v", w.Formats[1])
	}
	if contains(w.Limbo, "@f") || contains(w.Limbo, "@s") {
		t.Errorf("directives not stripped from limbo: %q", w.Limbo)
	}
	if !contains(w.Limbo, "\\input gwebmac") {
		t.Errorf("limbo lost its TeX: %q", w.Limbo)
	}
}

func hasWarning(ws []string, sub string) bool {
	for _, w := range ws {
		if indexFrom(w, sub, 0) >= 0 {
			return true
		}
	}
	return false
}

func TestSectionLines(t *testing.T) {
	w := ParseString("limbo\n\n@ first\n@c\nx\n\n@ second\n@c\ny\n")
	if w.Sections[0].Line != 3 {
		t.Errorf("section 1 line = %d, want 3", w.Sections[0].Line)
	}
	if w.Sections[1].Line != 7 {
		t.Errorf("section 2 line = %d, want 7", w.Sections[1].Line)
	}
}

func TestDiagnostics(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"unterminated", "@ x\n@c\ny := @<oops\n", "unterminated"},
		{"undefined ref", "@ x\n@c\n@<nope@>\n", "undefined section <nope>"},
		{"never used", "@ x\n@<helper@>=\ndoit()\n@ y\n@c\npackage main\n", "defined but never used"},
		{
			"ambiguous",
			"@ a\n@<Set X@>=\n1\n@ b\n@<Set Y@>=\n2\n@ c\n@c\n@<Set...@>\n",
			"ambiguous prefix <Set...>",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := ParseString(c.src)
			if !hasWarning(w.Warnings, c.want) {
				t.Errorf("want a warning containing %q, got %v", c.want, w.Warnings)
			}
		})
	}
}

func TestChangeFileApply(t *testing.T) {
	master := "@ greet\n@c\npackage main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	chSrc := "Ignored commentary.\n@x\n\tprintln(\"hello\")\n@y\n\tprintln(\"goodbye\")\n@z\n"
	changes, err := parseChangeFile(chSrc)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || len(changes[0].match) != 1 || len(changes[0].repl) != 1 {
		t.Fatalf("bad parse: %+v", changes)
	}
	out, err := applyChanges(master, changes, "c.ch")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(out, `println("goodbye")`) || contains(out, `println("hello")`) {
		t.Errorf("change not applied:\n%s", out)
	}
}

func TestChangeFileNoMatch(t *testing.T) {
	master := "@ x\n@c\npackage main\n"
	changes, _ := parseChangeFile("@x\nnonexistent line\n@y\nwhatever\n@z\n")
	if _, err := applyChanges(master, changes, "c.ch"); err == nil ||
		!contains(err.Error(), "never matched") {
		t.Errorf("want never-matched error, got %v", err)
	}
}

func TestChangeFilePartialMismatch(t *testing.T) {
	master := "alpha\nbeta\ngamma\n"
	changes, _ := parseChangeFile("@x\nbeta\nWRONG\n@y\nx\n@z\n")
	if _, err := applyChanges(master, changes, "c.ch"); err == nil ||
		!contains(err.Error(), "did not match") {
		t.Errorf("want did-not-match error, got %v", err)
	}
}

func TestChangeFileMalformed(t *testing.T) {
	if _, err := parseChangeFile("@x\nfind\n@z\n"); err == nil {
		t.Error("want error for @x without @y")
	}
}

func TestIncludeLineMapping(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	mustWrite("part.w", "@ A section in the included file.\n@c\nx := @<undef@>\n")
	main := mustWrite("main.w", "@* Main.\n@c\npackage main\n\n@i part.w\n")

	w, err := Parse(main)
	if err != nil {
		t.Fatal(err)
	}
	// The undefined reference lives in part.w; the diagnostic must cite it
	// (not a line number in the includes-expanded master).
	if !hasWarning(w.Warnings, "part.w:1") {
		t.Errorf("want a warning citing part.w:1, got %v", w.Warnings)
	}
	// Section 2 (the @c in part.w) should map back to part.w.
	if got := w.at(w.Sections[1].Line); !contains(got, "part.w") {
		t.Errorf("section 2 origin = %q, want a part.w location", got)
	}
}

func TestResolveAbbrevEitherSide(t *testing.T) {
	// The full name may appear only at a reference, with the definition
	// abbreviated -- and vice versa. Neither should warn.
	srcs := []string{
		"@ x\n@c\nvar _ = @<The parallel-map function@>\n@ d\n@<The parallel...@>=\n1\n",
		"@ x\n@c\nvar _ = @<The parallel...@>\n@ d\n@<The parallel-map function@>=\n1\n",
	}
	for _, src := range srcs {
		w := ParseString(src)
		for _, bad := range []string{"undefined", "ambiguous", "never"} {
			if hasWarning(w.Warnings, bad) {
				t.Errorf("unexpected %q warning for %q: %v", bad, src, w.Warnings)
			}
		}
	}
}
