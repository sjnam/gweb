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
		`\GID{xs}\mathord{[}\GID{i}\mathord{]}`: "index xs[i] should be tight",
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
	if !strings.Contains(out, `\GNS{chunk}{1}{Used in sections \Gs{2}, \Gs{3}}`) {
		t.Errorf("section-names entry malformed:\n%s", out)
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
