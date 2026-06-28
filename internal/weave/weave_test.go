package weave

import (
	"strings"
	"testing"

	"github.com/sjnam/gweb/internal/web"
)

func weaveString(t *testing.T, src string) string {
	t.Helper()
	var b strings.Builder
	if err := New(web.ParseString(src)).Weave(&b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestWeaveHighlighting(t *testing.T) {
	out := weaveString(t, `\input gwebmac
@* Demo.
The |main| entry.
@c
package main

func main() {
	@<body@>
}

@ Body.
@<body@>=
println(x)
`)
	checks := []string{
		`\GN{0}{1}{Demo}`, // starred section with title
		`$\GID{main}$`,    // inline code
		`\GKW{package}`,   // keyword bold
		`\GKW{func}`,      // keyword bold
		`\GID{main}`,      // identifier italic
		`\GX{2}{body}`,    // reference resolved to defining section 2
		`\GD{2}{body}`,    // definition headline
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("woven output missing %q\n---\n%s", c, out)
		}
	}
}

func TestWeaveEscaping(t *testing.T) {
	out := weaveString(t, `@ x
@c
m := map[string]int{}
s := "a\tb"
`)
	if !strings.Contains(out, `\GID{m}`) || !strings.Contains(out, `\GKW{map}`) {
		t.Errorf("expected identifier/keyword highlighting:\n%s", out)
	}
	// Braces in code must be escaped for math mode.
	if !strings.Contains(out, `\{`) || !strings.Contains(out, `\}`) {
		t.Errorf("braces not escaped:\n%s", out)
	}
}

func TestWeaveCommentSlashKern(t *testing.T) {
	// The leading "//" of a comment is tightened with \Gcommentkern.
	out := weaveString(t, "@ x\n@c\nx := 1 // hi\n")
	if !strings.Contains(out, `\GCM{/\kern\Gcommentkern/ hi}`) {
		t.Errorf("comment // not kerned:\n%s", out)
	}
}

func TestWeaveUnderscoreIdent(t *testing.T) {
	out := weaveString(t, `@ x
@c
var my_var int
`)
	if !strings.Contains(out, `\GID{my\_var}`) {
		t.Errorf("underscore not escaped in identifier:\n%s", out)
	}
}

func TestWeaveIndexAndXref(t *testing.T) {
	out := weaveString(t, `@ Program.
@c
package main

func main() {
	x := compute()
	@<use x@>
}

@ A refinement.
@<use x@>=
println(x)

@ Another definition site.
@<use x@>=
println(x + 1)
`)
	checks := []string{
		`\GII{\GID{main}}{\GsD{1}}`, // main defined (underlined) in section 1
		`\GII{\GID{x}}{`,            // x indexed
		`\GsD{1}`,                   // x defined via := in section 1
		`\GNS{use x}`,               // named section in the list
		`\GU{`,                      // "used in" note
		`\GA{`,                      // "also defined in" note (two def sites)
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("woven output missing %q\n---\n%s", c, out)
		}
	}
}

func TestWeaveOperators(t *testing.T) {
	out := weaveString(t, `@ x
@c
func f(ch chan int) {
	for i := 0; i != 3; i++ {
		ch <- i
	}
	if !done && i >= 1 {
	}
	switch x {
	default:
	}
}
`)
	checks := map[string]string{
		`\neq`:                     "!= should render as \\neq",
		`\geq`:                     ">= should render as \\geq",
		`\mathord{\leftarrow}`:     "<- should render as a left arrow",
		`\mathord{+}\mathord{+}`:   "++ should render tight",
		`$\GKW{if}$\GS `:           "a source space after if becomes a breakable \\GS",
		`\GKW{default}\mathord{:}`: "default: should be tight (no space before colon)",
	}
	for sub, msg := range checks {
		if !strings.Contains(out, sub) {
			t.Errorf("%s\nwant substring %q in:\n%s", msg, sub, out)
		}
	}
}

func TestWeaveTypeNamesAreBold(t *testing.T) {
	// A name declared with `type' is a user type and renders bold (\GKW)
	// everywhere, like a predeclared type -- as cweave does.
	out := weaveString(t, `@ x
@c
type entry struct {
	frac float64
}

type (
	Graph = int
	Vertex = int
)

func use() {
	var e entry
	var g Graph
	_ = e
	_ = g
}
`)
	for _, want := range []string{`\GKW{entry}`, `\GKW{Graph}`, `\GKW{Vertex}`} {
		if !strings.Contains(out, want) {
			t.Errorf("want declared type bold %q in:\n%s", want, out)
		}
	}
	// frac is a struct field, not a type, so it stays an italic identifier.
	if strings.Contains(out, `\GKW{frac}`) {
		t.Errorf("a struct field must not be bolded as a type:\n%s", out)
	}
}

func TestWeaveThinSpaceBeforeParen(t *testing.T) {
	// As in cweave, a "(" directly after a word (a function name or a keyword
	// like func) gets a thin space, so it does not jam against it.
	out := weaveString(t, "@ x\n@c\nvar _ = f(a)\nvar g = func(n int) {}\n")
	for _, want := range []string{`\GID{f}\Gthin \mathord{(}`, `\GKW{func}\Gthin \mathord{(}`} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in:\n%s", want, out)
		}
	}
}

func TestWeaveShiftOperators(t *testing.T) {
	// << and >> render as the tight double-angle symbols \ll and \gg (as cweb),
	// not two separate less-than/greater-than signs.
	out := weaveString(t, "@ x\n@c\nvar a = b<<2 | c>>3\n")
	for _, want := range []string{`\mathord{\ll}`, `\mathord{\gg}`} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, `\mathord{<}\mathord{<}`) {
		t.Errorf("<< should not render as two less-than signs:\n%s", out)
	}
}

func TestWeaveFormatDirective(t *testing.T) {
	out := weaveString(t, `\input gwebmac
@f Counts int
@s hidden int
@ x
@c
type Counts struct{}

var c Counts
var hidden int
`)
	if !strings.Contains(out, `\GKW{Counts}`) {
		t.Errorf("@f should typeset Counts bold like a type:\n%s", out)
	}
	if !strings.Contains(out, `\GKW{hidden}`) {
		t.Errorf("@s should also change the typeset class:\n%s", out)
	}
	if !strings.Contains(out, `\GII{\GID{Counts}}`) {
		t.Errorf("@f keeps the identifier in the index:\n%s", out)
	}
	if strings.Contains(out, `\GII{\GID{hidden}}`) {
		t.Errorf("@s should omit the identifier from the index:\n%s", out)
	}
}

func TestWeaveSourceSpacing(t *testing.T) {
	out := weaveString(t, `@ x
@c
func f(p *int) {
	r := a*b + c
	s := xs[i]
}
`)
	checks := map[string]string{
		`\mathord{*}\GKW{int}`:                  "pointer *int should be tight (one chunk)",
		`\GID{a}\mathord{*}\GID{b}`:             "multiplication a*b should be tight, matching gofmt",
		`\GID{xs}\mathord{[}\GID{i}\mathord{]}`: "index xs[i] should be tight (no thin space before [)",
		`\GS `:                                  "spaced operands should be separated by a breakable \\GS",
	}
	for sub, msg := range checks {
		if !strings.Contains(out, sub) {
			t.Errorf("%s\nwant substring %q in:\n%s", msg, sub, out)
		}
	}
}

func TestWeaveCodeInSectionName(t *testing.T) {
	out := weaveString(t, `@ use
@c
package main

var _ = @<Compute |area| now@>

@ def
@<Compute |area| now@>=
w * h
`)
	want := `Compute $\GID{area}$ now`
	if !strings.Contains(out, `\GX{2}{`+want+`}`) {
		t.Errorf("reference name should typeset |area| as code; missing %q in:\n%s", want, out)
	}
	if !strings.Contains(out, `\GD{2}{`+want+`}`) {
		t.Errorf("definition headline should typeset |area| as code; missing %q in:\n%s", want, out)
	}
}

func TestWeaveLayoutCodes(t *testing.T) {
	out := weaveString(t, "@ x\n@c\nvar y = a@,b\nvar z = c@/d\nvar w = e@|f\nvar v = g@#h\n")
	checks := map[string]string{
		`\GID{a}\,\GID{b}`:  "@, should insert a thin space within the chunk",
		`\GL{0}{$\GID{d}$}`: "@/ should force a new line",
		`\GSO `:             "@| should emit an optional break",
		`\GBL`:              "@# should emit a blank line",
	}
	for sub, msg := range checks {
		if !strings.Contains(out, sub) {
			t.Errorf("%s\nwant substring %q in:\n%s", msg, sub, out)
		}
	}
}

func TestWeaveForceDefinition(t *testing.T) {
	// foo is only *used* (inside a call), but @! forces it to be indexed as a
	// definition, so its section number is underlined.
	out := weaveString(t, "@ x\n@c\nfunc f() { use(@!foo) }\n")
	if !strings.Contains(out, `\GII{\GID{foo}}{\GsD{1}}`) {
		t.Errorf("@! should index foo as a definition (underlined):\n%s", out)
	}
}

func TestWeaveIndexExcludesBlankAndPluralizes(t *testing.T) {
	out := weaveString(t, `@ def
@<chunk@>=
println(1)

@ first user
@c
package main

func f() {
	for _, x := range xs {
		@<chunk@>
	}
}

@ second user
@c
func g() { @<chunk@> }
`)
	if strings.Contains(out, `\GII{\GID{_}}`) {
		t.Errorf("the blank identifier _ should not be indexed:\n%s", out)
	}
	// chunk is used in two different sections (2 and 3), so the plural notes apply.
	if !strings.Contains(out, `\GUs{\Gs{2}, \Gs{3}}`) {
		t.Errorf("uses in two sections should emit \\GUs:\n%s", out)
	}
	if !strings.Contains(out, `\GNS{chunk}{1}{\GNuseds{\Gs{2}, \Gs{3}}}`) {
		t.Errorf("section-names entry malformed:\n%s", out)
	}
}

func TestWrappedSectionName(t *testing.T) {
	// A section name wrapped across lines (a newline inside @<...@>) must match
	// the same name written on one line, as in CWEB. Otherwise the reference
	// resolves to section 0, which also crashes luatex's PDF backend.
	out := weaveString(t, "@* Start.\n@c\nfunc main() { @<do the\nthing@> }\n@ @<do the thing@>=\nx := 1\n")
	if strings.Contains(out, `\GX{0}`) {
		t.Errorf("wrapped section name failed to resolve (got \\GX{0}):\n%s", out)
	}
	if !strings.Contains(out, `\GX{2}`) {
		t.Errorf("wrapped reference should resolve to defining section 2:\n%s", out)
	}
}

func TestCommentInlineCode(t *testing.T) {
	// A |...| span inside a code comment is set as the Go code it names (as in
	// cweb), not printed literally; an unmatched bar stays literal.
	out := weaveString(t, "@ x\n@c\nx := 1 // set |x| now\n")
	if !strings.Contains(out, `\GCM{`) {
		t.Fatalf("no comment emitted:\n%s", out)
	}
	if !strings.Contains(out, `\GID{x}`) {
		t.Errorf("|x| in a comment should render as code \\GID{x}:\n%s", out)
	}
	if strings.Contains(out, "|x|") {
		t.Errorf("the bars should be consumed, not printed literally:\n%s", out)
	}
	// A \.{...} typewriter span in a comment passes through verbatim (cweb-style),
	// rather than being escaped character by character.
	out3 := weaveString(t, "@ x\n@c\nx := 1 // see \\.{foo.go}\n")
	if !strings.Contains(out3, `\.{foo.go}`) {
		t.Errorf("\\.{...} in a comment should pass through verbatim:\n%s", out3)
	}

	// An unmatched bar is left literal (no closing |), not swallowed to end: it
	// is escaped as a roman bar (\vert), and the following word is not code.
	out2 := weaveString(t, "@ x\n@c\nx := 1 // a | b\n")
	if !strings.Contains(out2, `\vert`) {
		t.Errorf("unmatched bar should stay a literal bar (\\vert):\n%s", out2)
	}
	if strings.Contains(out2, `\GID{b}`) {
		t.Errorf("unmatched bar must not turn the rest into code:\n%s", out2)
	}
}

func TestWeaveEmptyBrackets(t *testing.T) {
	// The empty brackets of a slice type get a thin space so they don't jam.
	out := weaveString(t, "@ x\n@c\nvar s []byte\n")
	if !strings.Contains(out, `\mathord{[}\,\mathord{]}`) {
		t.Errorf("slice brackets [] should get a thin space:\n%s", out)
	}
	// Indexing a[i] must stay tight (the brackets are not empty).
	out2 := weaveString(t, "@ x\n@c\nvar v = a[i]\n")
	if strings.Contains(out2, `\mathord{[}\,`) {
		t.Errorf("index brackets a[i] should not get a thin space:\n%s", out2)
	}
	// Empty braces (struct{}, ...) get a thin space; non-empty braces do not.
	out3 := weaveString(t, "@ x\n@c\ntype E struct{}\n")
	if !strings.Contains(out3, `\mathord{\{}\,\mathord{\}}`) {
		t.Errorf("empty braces {} should get a thin space:\n%s", out3)
	}
	out4 := weaveString(t, "@ x\n@c\nv := T{x}\n")
	if strings.Contains(out4, `\mathord{\{}\,`) {
		t.Errorf("non-empty braces should not get a thin space:\n%s", out4)
	}
}

func TestWeaveBookmarks(t *testing.T) {
	out := weaveString(t, `@* Chapter one. intro.
@c
package main
@*1 Sub A. first.
@<a@>=
1
@*1 Sub B. second.
@<b@>=
2
@* Chapter two. more.
@c
var _ = 0
`)
	// Chapter one (depth 0, section 1) has two direct children: the @*1
	// subsections (depth 1). \Gbookmark is {depth}{secNum}{children}{title}.
	for _, want := range []string{
		`\Gbookmark{0}{1}{2}{Chapter one}`,
		`\Gbookmark{1}{2}{0}{Sub A}`,
		`\Gbookmark{1}{3}{0}{Sub B}`,
		`\Gbookmark{0}{4}{0}{Chapter two}`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing bookmark %q:\n%s", want, out)
		}
	}
}

func TestBookmarkTitle(t *testing.T) {
	cases := map[string]string{
		"The scanner":        "The scanner",
		"Update for |b| now": "Update for b now",
		"Foo \\& Bar":        "Foo  Bar",
		"a @@ b":             "a @ b",
	}
	for in, want := range cases {
		if got := bookmarkTitle(in); got != want {
			t.Errorf("bookmarkTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWeaveInjectsGwebmac(t *testing.T) {
	// gweave supplies \input gwebmac; the .w file need not.
	out := weaveString(t, "@ x\n@c\npackage main\n")
	if !strings.HasPrefix(out, "\\input gwebmac\n") {
		t.Errorf("woven output should start with \\input gwebmac, got:\n%.30q", out)
	}
	// A stray copy in the limbo is stripped, never duplicated.
	out2 := weaveString(t, "\\input gwebmac\n@ x\n@c\npackage main\n")
	if n := strings.Count(out2, "\\input gwebmac"); n != 1 {
		t.Errorf("want exactly one \\input gwebmac, got %d", n)
	}
}
