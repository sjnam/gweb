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
		`\GII{\GID{main}}{\GUL{1}}`, // main defined (underlined) in section 1
		`\GII{\GID{x}}{`,            // x indexed
		`\GUL{1}`,                   // x defined via := in section 1
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
