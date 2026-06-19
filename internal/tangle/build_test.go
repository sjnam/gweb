package tangle

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sjnam/gweb/internal/web"
)

// TestExamplesBuild tangles every bundled example and runs `go build` on the
// result, proving end to end that the tools emit real, compilable Go. It is
// skipped in -short mode and when the go tool is unavailable.
func TestExamplesBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping go build of examples in -short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not found in PATH")
	}

	examples, err := filepath.Glob(filepath.Join("..", "..", "examples", "*.w"))
	if err != nil {
		t.Fatal(err)
	}
	if len(examples) == 0 {
		t.Fatal("no example .w files found")
	}

	for _, ex := range examples {
		t.Run(filepath.Base(ex), func(t *testing.T) {
			t.Parallel()
			buildExample(t, ex)
		})
	}
}

func buildExample(t *testing.T, path string) {
	t.Helper()

	w, err := web.Parse(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	outs, err := New(w).Tangle(base + ".go")
	if err != nil {
		t.Fatalf("tangle: %v", err)
	}

	dir := t.TempDir()
	haveMod := false
	for _, o := range outs {
		if o.Warning != "" {
			t.Fatalf("%s: %s", o.File, o.Warning)
		}
		if o.File == "go.mod" {
			haveMod = true
		}
		if err := os.WriteFile(filepath.Join(dir, o.File), o.Content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !haveMod {
		const mod = "module gwebexample\n\ngo 1.21\n"
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command("go", "build", "-o", os.DevNull, ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
}

// TestChangeFileBuilds applies the wc.ch change file to wc.w and confirms the
// patched program still tangles to compilable Go (and was actually changed).
func TestChangeFileBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping go build in -short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not found in PATH")
	}
	w, err := web.ParseWithChange(
		filepath.Join("..", "..", "examples", "wc.w"),
		filepath.Join("..", "..", "examples", "wc.ch"),
	)
	if err != nil {
		t.Fatal(err)
	}
	outs, err := New(w).Tangle("wc.go")
	if err != nil {
		t.Fatal(err)
	}
	got := string(outs[0].Content)
	if !strings.Contains(got, `%d,%d,%d`) || strings.Contains(got, `%8d`) {
		t.Fatalf("change file not applied to tangled output:\n%s", got)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wc.go"), outs[0].Content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module gwebexample\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", os.DevNull, ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
}
