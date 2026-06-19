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
		`\GN{0}{1}{Demo}`,             // starred section with title
		`$\GID{main}$`,                // inline code
		`\GKW{package}`,               // keyword bold
		`\GKW{func}`,                  // keyword bold
		`\GID{main}`,                  // identifier italic
		`\GX{2}{body}`,                // reference resolved to defining section 2
		`\GD{2}{body}`,                // definition headline
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
