// Command gweave turns a GWEB (.w) file into a TeX document.
//
// Usage:
//
//	gweave [-o dir] file[.w] [change[.ch]]
//
// The .w (and .ch) extension may be omitted. The woven document is written to
// <basename>.tex. Process it with a TeX engine that can find gwebmac.tex (e.g.
// "pdftex file.tex") to produce a PDF.
//
//line lit/gweave.w:8
//line lit/gweave.w:9
//line lit/gweave.w:10
//line lit/gweave.w:11
//line lit/gweave.w:12
//line lit/gweave.w:13
//line lit/gweave.w:14
//line lit/gweave.w:15
//line lit/gweave.w:16
//line lit/gweave.w:17
package main

//line lit/gweave.w:19
import (
//line lit/gweave.w:20
	"flag"
//line lit/gweave.w:21
	"fmt"
//line lit/gweave.w:22
	"os"
//line lit/gweave.w:23
	"path/filepath"
//line lit/gweave.w:24
	"strings"

//line lit/gweave.w:26
	"github.com/sjnam/gweb/internal/weave"
//line lit/gweave.w:27
	"github.com/sjnam/gweb/internal/web"
//line lit/gweave.w:28
)

//line lit/gweave.w:34
func main() {
//line lit/gweave.w:35
	outDir := flag.String("o", "", "output directory (default: input file's directory)")
//line lit/gweave.w:36
	showVersion := flag.Bool("version", false, "print version and exit")
//line lit/gweave.w:37
	flag.Usage = usage
//line lit/gweave.w:38
	flag.Parse()
//line lit/gweave.w:39
	if *showVersion {
//line lit/gweave.w:40
		fmt.Printf("gweave (GWEB) %s\n", web.Version)
//line lit/gweave.w:41
		return
//line lit/gweave.w:42
	}
//line lit/gweave.w:43
	if flag.NArg() < 1 || flag.NArg() > 2 {
//line lit/gweave.w:44
		usage()
//line lit/gweave.w:45
		os.Exit(2)
//line lit/gweave.w:46
	}
//line lit/gweave.w:47
	fmt.Fprintf(os.Stderr, "This is GWEAVE, Version %s\n", web.Version)
//line lit/gweave.w:48
	if err := run(flag.Arg(0), flag.Arg(1), *outDir); err != nil {
//line lit/gweave.w:49
		fmt.Fprintln(os.Stderr, "gweave:", err)
//line lit/gweave.w:50
		os.Exit(1)
//line lit/gweave.w:51
	}
//line lit/gweave.w:52
}

//line lit/gweave.w:56
func usage() {
//line lit/gweave.w:57
	fmt.Fprintln(os.Stderr, "usage: gweave [-o dir] file[.w] [change[.ch]]")
//line lit/gweave.w:58
	flag.PrintDefaults()
//line lit/gweave.w:59
}

//line lit/gweave.w:65
func reportProgress(w *web.Web) {
//line lit/gweave.w:66
	for _, s := range w.Sections {
//line lit/gweave.w:67
		if s.Starred {
//line lit/gweave.w:68
			fmt.Fprintf(os.Stderr, "*%d", s.Number)
//line lit/gweave.w:69
		}
//line lit/gweave.w:70
	}
//line lit/gweave.w:71
	fmt.Fprintln(os.Stderr)
//line lit/gweave.w:72
}

//line lit/gweave.w:78
func run(input, changeFile, outDir string) error {
//line lit/gweave.w:79
	input = web.DefaultExt(input, ".w")
//line lit/gweave.w:80
	changeFile = web.DefaultExt(changeFile, ".ch")
//line lit/gweave.w:81
	w, err := web.ParseWithChange(input, changeFile)
//line lit/gweave.w:82
	if err != nil {
//line lit/gweave.w:83
		return err
//line lit/gweave.w:84
	}
//line lit/gweave.w:85
	for _, warn := range w.Warnings {
//line lit/gweave.w:86
		fmt.Fprintln(os.Stderr, "gweave: warning:", warn)
//line lit/gweave.w:87
	}
//line lit/gweave.w:88
	reportProgress(w)
//line lit/gweave.w:89
	if outDir == "" {
//line lit/gweave.w:90
		outDir = filepath.Dir(input)
//line lit/gweave.w:91
	}
//line lit/gweave.w:92
	base := filepath.Base(input)
//line lit/gweave.w:93
	base = strings.TrimSuffix(base, filepath.Ext(base))
//line lit/gweave.w:94
	outPath := filepath.Join(outDir, base+".tex")

//line lit/gweave.w:96
	f, err := os.Create(outPath)
//line lit/gweave.w:97
	if err != nil {
//line lit/gweave.w:98
		return err
//line lit/gweave.w:99
	}
//line lit/gweave.w:100
	defer f.Close()

//line lit/gweave.w:102
	if err := weave.New(w).Weave(f); err != nil {
//line lit/gweave.w:103
		return err
//line lit/gweave.w:104
	}
//line lit/gweave.w:105
	fmt.Printf("gweave: wrote %s\n", outPath)
//line lit/gweave.w:106
	return nil
//line lit/gweave.w:107
}
