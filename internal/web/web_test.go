package web

import "testing"

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

func TestResolveAbbrev(t *testing.T) {
	w := ParseString(sample)
	if got := w.Resolve("Print the..."); got != "Print the greeting" {
		t.Errorf("Resolve abbrev = %q, want %q", got, "Print the greeting")
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
