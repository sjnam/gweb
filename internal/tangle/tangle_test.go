package tangle

import (
	"strings"
	"testing"

	"github.com/sjnam/gweb/internal/web"
)

func TestTangleExpandsAndConcatenates(t *testing.T) {
	const src = `@ main
@c
package main

func main() {
	@<body@>
}

@ helper
@<body@>=
greet()

@ more program text
@c
func greet() { println("hi") }
`
	outs, err := New(web.ParseString(src)).Tangle("prog.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(outs) != 1 {
		t.Fatalf("got %d outputs, want 1", len(outs))
	}
	got := string(outs[0].Content)
	if outs[0].Warning != "" {
		t.Errorf("unexpected warning: %s", outs[0].Warning)
	}
	for _, want := range []string{"func main()", "greet()", `func greet()`} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

func TestTangleFileSections(t *testing.T) {
	const src = `@ first
@(extra.go@>=
package main

@ second
@c
package main
`
	outs, err := New(web.ParseString(src)).Tangle("main.go")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]bool{}
	for _, o := range outs {
		files[o.File] = true
	}
	if !files["main.go"] || !files["extra.go"] {
		t.Errorf("expected main.go and extra.go, got %v", files)
	}
}

func TestTangleUndefinedReference(t *testing.T) {
	const src = `@ x
@c
package main
var _ = @<missing@>
`
	_, err := New(web.ParseString(src)).Tangle("p.go")
	if err == nil || !strings.Contains(err.Error(), "undefined") {
		t.Errorf("want undefined-section error, got %v", err)
	}
}

func TestTangleCircularReference(t *testing.T) {
	const src = `@ a
@<a@>=
@<b@>
@ b
@<b@>=
@<a@>
@ root
@c
package main
var _ = @<a@>
`
	_, err := New(web.ParseString(src)).Tangle("p.go")
	if err == nil || !strings.Contains(err.Error(), "circular") {
		t.Errorf("want circular-reference error, got %v", err)
	}
}

func TestTangleCodeInName(t *testing.T) {
	const src = `@ root
@c
package main

var area = @<the |x| value@>

@ helper
@<the |x| value@>=
42
`
	outs, err := New(web.ParseString(src)).Tangle("p.go")
	if err != nil {
		t.Fatal(err)
	}
	got := string(outs[0].Content)
	if !strings.Contains(got, "var area = 42") {
		t.Errorf("name containing |x| should still match for tangling:\n%s", got)
	}
}

func TestTangleIgnoresLayoutCodes(t *testing.T) {
	const src = "@ x\n@c\npackage main\n\nvar n = 1@,@/@|@#@+@[@]@;2\n"
	outs, err := New(web.ParseString(src)).Tangle("p.go")
	if err != nil {
		t.Fatal(err)
	}
	got := string(outs[0].Content)
	if !strings.Contains(got, "var n = 12") {
		t.Errorf("layout/hint codes must not leak into tangled output:\n%s", got)
	}
}

func TestTangleDropsUnknownCode(t *testing.T) {
	// An unknown @x must drop exactly its two characters, not corrupt the rest
	// (guards against a former double-skip bug).
	const src = "@ x\n@c\npackage main\n\nvar a@?bc = 1\n"
	outs, err := New(web.ParseString(src)).Tangle("p.go")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(outs[0].Content); !strings.Contains(got, "var abc = 1") {
		t.Errorf("unknown @x should drop exactly two chars:\n%s", got)
	}
}
