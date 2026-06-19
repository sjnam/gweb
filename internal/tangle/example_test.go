package tangle

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/sjnam/gweb/internal/web"
)

// TestExampleTanglesToValidGo tangles the bundled wc.w example and confirms the
// result parses as valid Go, exercising the whole front end on a realistic web.
func TestExampleTanglesToValidGo(t *testing.T) {
	path := filepath.Join("..", "..", "examples", "wc.w")
	w, err := web.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	outs, err := New(w).Tangle("wc.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(outs) != 1 {
		t.Fatalf("got %d outputs, want 1", len(outs))
	}
	if outs[0].Warning != "" {
		t.Fatalf("gofmt warning (output is not valid Go): %s", outs[0].Warning)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "wc.go", outs[0].Content, parser.AllErrors); err != nil {
		t.Fatalf("tangled output does not parse as Go: %v", err)
	}
}
