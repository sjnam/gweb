// Command gtangle extracts compilable Go source from a GWEB (.w) file.
//
// Usage:
//
//	gtangle [-o dir] [-line] file[.w] [change[.ch]]
//
// The .w (and .ch) extension may be omitted. The unnamed @c sections are
// written to <basename>.go (in -o dir, default the input's directory);
// @(file@>= sections are written to their named files. With -line, the Go
// output carries //line directives so the compiler reports errors at .w
// positions.
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
	lineDirs := flag.Bool("line", false, "emit //line directives mapping Go back to .w source")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = usage
	flag.Parse()
	if *showVersion {
		fmt.Printf("gtangle (GWEB) %s\n", web.Version)
		return
	}
	if flag.NArg() < 1 || flag.NArg() > 2 {
		usage()
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "This is GTANGLE, Version %s.\n", web.Version)
	if err := run(flag.Arg(0), flag.Arg(1), *outDir, *lineDirs); err != nil {
		fmt.Fprintln(os.Stderr, "gtangle:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: gtangle [-o dir] [-line] file[.w] [change[.ch]]")
	flag.PrintDefaults()
}

func reportProgress(w *web.Web) {
	for _, s := range w.Sections {
		if s.Starred {
			fmt.Fprintf(os.Stderr, "*%d", s.Number)
		}
	}
	fmt.Fprintln(os.Stderr)
}

func run(input, changeFile, outDir string, lineDirs bool) error {
	input = web.DefaultExt(input, ".w")
	changeFile = web.DefaultExt(changeFile, ".ch")
	w, err := web.ParseWithChange(input, changeFile)
	if err != nil {
		return err
	}
	for _, warn := range w.Warnings {
		fmt.Fprintln(os.Stderr, "gtangle: warning:", warn)
	}
	reportProgress(w)
	if outDir == "" {
		outDir = filepath.Dir(input)
	}

	base := filepath.Base(input)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	defaultFile := base + ".go"

	outs, err := tangle.New(w).WithLineDirectives(lineDirs).Tangle(defaultFile)
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
