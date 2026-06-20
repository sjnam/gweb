@* Command \.{gweave}.
This is the command-line front end to the |weave| package. The woven document is
written to the input's base name with a \.{.tex} extension; process it with a
TeX engine that can find \.{gwebmac.tex} to produce a PDF.
@(cmd/gweave/main.go@>=
// Command gweave turns a GWEB (.w) file into a TeX document.
//
// Usage:
//
//	gweave [-o dir] file.w
//
// The woven document is written to <basename>.tex. Process it with a TeX engine
// that can find gwebmac.tex (e.g. "pdftex file.tex") to produce a PDF.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sjnam/gweb/internal/weave"
	"github.com/sjnam/gweb/internal/web"
)

@ The entry point parses the flags and arguments and dispatches to |run|.
@(cmd/gweave/main.go@>=
func main() {
	outDir := flag.String("o", "", "output directory (default: input file's directory)")
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() < 1 || flag.NArg() > 2 {
		usage()
		os.Exit(2)
	}
	if err := run(flag.Arg(0), flag.Arg(1), *outDir); err != nil {
		fmt.Fprintln(os.Stderr, "gweave:", err)
		os.Exit(1)
	}
}

@ Usage.
@(cmd/gweave/main.go@>=
func usage() {
	fmt.Fprintln(os.Stderr, "usage: gweave [-o dir] file.w [change.ch]")
	flag.PrintDefaults()
}

@ |run| parses the web (applying a change file if given), prints any warnings,
and writes the woven TeX.
@(cmd/gweave/main.go@>=
func run(input, changeFile, outDir string) error {
	w, err := web.ParseWithChange(input, changeFile)
	if err != nil {
		return err
	}
	for _, warn := range w.Warnings {
		fmt.Fprintln(os.Stderr, "gweave: warning:", warn)
	}
	if outDir == "" {
		outDir = filepath.Dir(input)
	}
	base := filepath.Base(input)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	outPath := filepath.Join(outDir, base+".tex")

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := weave.New(w).Weave(f); err != nil {
		return err
	}
	fmt.Printf("gweave: wrote %s\n", outPath)
	return nil
}
