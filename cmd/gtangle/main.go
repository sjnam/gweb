// Command gtangle extracts compilable Go source from a GWEB (.w) file.
//
// Usage:
//
//	gtangle [-o dir] file.w
//
// The unnamed @c sections are written to <basename>.go (in -o dir, default the
// input's directory); @(file@>= sections are written to their named files.
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

func main() {
	outDir := flag.String("o", "", "output directory (default: input file's directory)")
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 1 {
		usage()
		os.Exit(2)
	}
	if err := run(flag.Arg(0), *outDir); err != nil {
		fmt.Fprintln(os.Stderr, "gtangle:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: gtangle [-o dir] file.w")
	flag.PrintDefaults()
}

func run(input, outDir string) error {
	w, err := web.Parse(input)
	if err != nil {
		return err
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
