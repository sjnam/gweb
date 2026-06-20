@* Command \.{gtangle}.
This is the command-line front end to the |tangle| package. The unnamed \.{@@c}
sections are written to the input's base name with a \.{.go} extension (in the
|-o| directory, default the input's directory); \.{@@(file@@>=} sections are
written to their named files.
@(cmd/gtangle/main.go@>=
// Command gtangle extracts compilable Go source from a GWEB (.w) file.
//
// Usage:
//
//	gtangle [-o dir] file.w
//
// The unnamed @@c sections are written to <basename>.go (in -o dir, default the
// input's directory); @@(file@@>= sections are written to their named files.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sjnam/gweb/internal/tangle"
	"github.com/sjnam/gweb/internal/web"
)

@ The entry point parses the flags and arguments and dispatches to |run|.
@(cmd/gtangle/main.go@>=
func main() {
	outDir := flag.String("o", "", "output directory (default: input file's directory)")
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() < 1 || flag.NArg() > 2 {
		usage()
		os.Exit(2)
	}
	if err := run(flag.Arg(0), flag.Arg(1), *outDir); err != nil {
		fmt.Fprintln(os.Stderr, "gtangle:", err)
		os.Exit(1)
	}
}

@ Usage.
@(cmd/gtangle/main.go@>=
func usage() {
	fmt.Fprintln(os.Stderr, "usage: gtangle [-o dir] file.w [change.ch]")
	flag.PrintDefaults()
}

@ |run| parses the web (applying a change file if given), prints any warnings,
tangles, and writes each output file, creating its directory if necessary.
@(cmd/gtangle/main.go@>=
func run(input, changeFile, outDir string) error {
	w, err := web.ParseWithChange(input, changeFile)
	if err != nil {
		return err
	}
	for _, warn := range w.Warnings {
		fmt.Fprintln(os.Stderr, "gtangle: warning:", warn)
	}
	if outDir == "" {
		outDir = filepath.Dir(input)
	}

	base := filepath.Base(input)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	defaultFile := base + ".go"

	outs, err := tangle.New(w).Tangle(defaultFile)
	if err != nil {
		return err
	}

	for _, out := range outs {
		path := filepath.Join(outDir, out.File)
		if dir := filepath.Dir(path); dir != "." {
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
				return mkErr
			}
		}
		if writeErr := os.WriteFile(path, out.Content, 0o644); writeErr != nil {
			return writeErr
		}
		if out.Warning != "" {
			fmt.Fprintf(os.Stderr, "gtangle: warning: %s: %s\n", path, out.Warning)
		}
		fmt.Printf("gtangle: wrote %s (%d bytes)\n", path, len(out.Content))
	}
	return nil
}
