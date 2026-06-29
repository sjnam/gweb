// Command gtangle extracts compilable Go source from a GWEB (.w) file.
//
// Usage:
//
//	gtangle [-o dir] file[.w] [change[.ch]]
//
// The .w (and .ch) extension may be omitted. The unnamed @c sections are
// written to <basename>.go (in -o dir, default the input's directory);
// @(file@>= sections are written to their named files. As in cweb's ctangle,
// the Go output always carries //line directives so the compiler reports errors
// at .w positions.
//
//line lit/gtangle.w:8
//line lit/gtangle.w:9
//line lit/gtangle.w:10
//line lit/gtangle.w:11
//line lit/gtangle.w:12
//line lit/gtangle.w:13
//line lit/gtangle.w:14
//line lit/gtangle.w:15
//line lit/gtangle.w:16
//line lit/gtangle.w:17
//line lit/gtangle.w:18
//line lit/gtangle.w:19
package main

//line lit/gtangle.w:21
import (
//line lit/gtangle.w:22
	"flag"
//line lit/gtangle.w:23
	"fmt"
//line lit/gtangle.w:24
	"os"
//line lit/gtangle.w:25
	"path/filepath"
//line lit/gtangle.w:26
	"strings"

//line lit/gtangle.w:28
	"github.com/sjnam/gweb/internal/tangle"
//line lit/gtangle.w:29
	"github.com/sjnam/gweb/internal/web"
//line lit/gtangle.w:30
)

//line lit/gtangle.w:36
func main() {
//line lit/gtangle.w:37
	outDir := flag.String("o", "", "output directory (default: input file's directory)")
//line lit/gtangle.w:38
	showVersion := flag.Bool("version", false, "print version and exit")
//line lit/gtangle.w:39
	flag.Usage = usage
//line lit/gtangle.w:40
	flag.Parse()
//line lit/gtangle.w:41
	if *showVersion {
//line lit/gtangle.w:42
		fmt.Printf("gtangle (GWEB) %s\n", web.Version)
//line lit/gtangle.w:43
		return
//line lit/gtangle.w:44
	}
//line lit/gtangle.w:45
	if flag.NArg() < 1 || flag.NArg() > 2 {
//line lit/gtangle.w:46
		usage()
//line lit/gtangle.w:47
		os.Exit(2)
//line lit/gtangle.w:48
	}
//line lit/gtangle.w:49
	fmt.Fprintf(os.Stderr, "This is GTANGLE, Version %s\n", web.Version)
//line lit/gtangle.w:50
	if err := run(flag.Arg(0), flag.Arg(1), *outDir); err != nil {
//line lit/gtangle.w:51
		fmt.Fprintln(os.Stderr, "gtangle:", err)
//line lit/gtangle.w:52
		os.Exit(1)
//line lit/gtangle.w:53
	}
//line lit/gtangle.w:54
}

//line lit/gtangle.w:58
func usage() {
//line lit/gtangle.w:59
	fmt.Fprintln(os.Stderr, "usage: gtangle [-o dir] file[.w] [change[.ch]]")
//line lit/gtangle.w:60
	flag.PrintDefaults()
//line lit/gtangle.w:61
}

//line lit/gtangle.w:67
func reportProgress(w *web.Web) {
//line lit/gtangle.w:68
	for _, s := range w.Sections {
//line lit/gtangle.w:69
		if s.Starred {
//line lit/gtangle.w:70
			fmt.Fprintf(os.Stderr, "*%d", s.Number)
//line lit/gtangle.w:71
		}
//line lit/gtangle.w:72
	}
//line lit/gtangle.w:73
	fmt.Fprintln(os.Stderr)
//line lit/gtangle.w:74
}

//line lit/gtangle.w:81
func run(input, changeFile, outDir string) error {
//line lit/gtangle.w:82
	input = web.DefaultExt(input, ".w")
//line lit/gtangle.w:83
	changeFile = web.DefaultExt(changeFile, ".ch")
//line lit/gtangle.w:84
	w, err := web.ParseWithChange(input, changeFile)
//line lit/gtangle.w:85
	if err != nil {
//line lit/gtangle.w:86
		return err
//line lit/gtangle.w:87
	}
//line lit/gtangle.w:88
	for _, warn := range w.Warnings {
//line lit/gtangle.w:89
		fmt.Fprintln(os.Stderr, "gtangle: warning:", warn)
//line lit/gtangle.w:90
	}
//line lit/gtangle.w:91
	reportProgress(w)
//line lit/gtangle.w:92
	if outDir == "" {
//line lit/gtangle.w:93
		outDir = filepath.Dir(input)
//line lit/gtangle.w:94
	}

//line lit/gtangle.w:96
	base := filepath.Base(input)
//line lit/gtangle.w:97
	base = strings.TrimSuffix(base, filepath.Ext(base))
//line lit/gtangle.w:98
	defaultFile := base + ".go"

//line lit/gtangle.w:100
	outs, err := tangle.New(w).Tangle(defaultFile)
//line lit/gtangle.w:101
	if err != nil {
//line lit/gtangle.w:102
		return err
//line lit/gtangle.w:103
	}

//line lit/gtangle.w:105
	for _, out := range outs {
//line lit/gtangle.w:106
		path := filepath.Join(outDir, out.File)
//line lit/gtangle.w:107
		if dir := filepath.Dir(path); dir != "." {
//line lit/gtangle.w:108
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
//line lit/gtangle.w:109
				return mkErr
//line lit/gtangle.w:110
			}
//line lit/gtangle.w:111
		}
//line lit/gtangle.w:112
		if writeErr := os.WriteFile(path, out.Content, 0o644); writeErr != nil {
//line lit/gtangle.w:113
			return writeErr
//line lit/gtangle.w:114
		}
//line lit/gtangle.w:115
		if out.Warning != "" {
//line lit/gtangle.w:116
			fmt.Fprintf(os.Stderr, "gtangle: warning: %s: %s\n", path, out.Warning)
//line lit/gtangle.w:117
		}
//line lit/gtangle.w:118
		fmt.Printf("gtangle: wrote %s (%d bytes)\n", path, len(out.Content))
//line lit/gtangle.w:119
	}
//line lit/gtangle.w:120
	return nil
//line lit/gtangle.w:121
}
