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
	"github.com/sjnam/gweb/internal/web"
//line lit/gtangle.w:29
)

//line lit/gtangle.w:35
func main() {
//line lit/gtangle.w:36
	outDir := flag.String("o", "", "output directory (default: input file's directory)")
//line lit/gtangle.w:37
	showVersion := flag.Bool("version", false, "print version and exit")
//line lit/gtangle.w:38
	flag.Usage = usage
//line lit/gtangle.w:39
	flag.Parse()
//line lit/gtangle.w:40
	if *showVersion {
//line lit/gtangle.w:41
		fmt.Printf("gtangle (GWEB) %s\n", web.Version)
//line lit/gtangle.w:42
		return
//line lit/gtangle.w:43
	}
//line lit/gtangle.w:44
	if flag.NArg() < 1 || flag.NArg() > 2 {
//line lit/gtangle.w:45
		usage()
//line lit/gtangle.w:46
		os.Exit(2)
//line lit/gtangle.w:47
	}
//line lit/gtangle.w:48
	fmt.Fprintf(os.Stderr, "This is GTANGLE, Version %s\n", web.Version)
//line lit/gtangle.w:49
	if err := run(flag.Arg(0), flag.Arg(1), *outDir); err != nil {
//line lit/gtangle.w:50
		fmt.Fprintln(os.Stderr, "gtangle:", err)
//line lit/gtangle.w:51
		os.Exit(1)
//line lit/gtangle.w:52
	}
//line lit/gtangle.w:53
}

//line lit/gtangle.w:57
func usage() {
//line lit/gtangle.w:58
	fmt.Fprintln(os.Stderr, "usage: gtangle [-o dir] file[.w] [change[.ch]]")
//line lit/gtangle.w:59
	flag.PrintDefaults()
//line lit/gtangle.w:60
}

//line lit/gtangle.w:66
func reportProgress(w *web.Web) {
//line lit/gtangle.w:67
	for _, s := range w.Sections {
//line lit/gtangle.w:68
		if s.Starred {
//line lit/gtangle.w:69
			fmt.Fprintf(os.Stderr, "*%d", s.Number)
//line lit/gtangle.w:70
		}
//line lit/gtangle.w:71
	}
//line lit/gtangle.w:72
	fmt.Fprintln(os.Stderr)
//line lit/gtangle.w:73
}

//line lit/gtangle.w:80
func run(input, changeFile, outDir string) error {
//line lit/gtangle.w:81
	input = web.DefaultExt(input, ".w")
//line lit/gtangle.w:82
	changeFile = web.DefaultExt(changeFile, ".ch")
//line lit/gtangle.w:83
	w, err := web.ParseWithChange(input, changeFile)
//line lit/gtangle.w:84
	if err != nil {
//line lit/gtangle.w:85
		return err
//line lit/gtangle.w:86
	}
//line lit/gtangle.w:87
	for _, warn := range w.Warnings {
//line lit/gtangle.w:88
		fmt.Fprintln(os.Stderr, "gtangle: warning:", warn)
//line lit/gtangle.w:89
	}
//line lit/gtangle.w:90
	reportProgress(w)
//line lit/gtangle.w:91
	if outDir == "" {
//line lit/gtangle.w:92
		outDir = filepath.Dir(input)
//line lit/gtangle.w:93
	}

//line lit/gtangle.w:95
	base := filepath.Base(input)
//line lit/gtangle.w:96
	base = strings.TrimSuffix(base, filepath.Ext(base))
//line lit/gtangle.w:97
	defaultFile := base + ".go"

//line lit/gtangle.w:99
	outs, err := New(w).Tangle(defaultFile)
//line lit/gtangle.w:100
	if err != nil {
//line lit/gtangle.w:101
		return err
//line lit/gtangle.w:102
	}

//line lit/gtangle.w:104
	for _, out := range outs {
//line lit/gtangle.w:105
		path := filepath.Join(outDir, out.File)
//line lit/gtangle.w:106
		if dir := filepath.Dir(path); dir != "." {
//line lit/gtangle.w:107
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
//line lit/gtangle.w:108
				return mkErr
//line lit/gtangle.w:109
			}
//line lit/gtangle.w:110
		}
//line lit/gtangle.w:111
		if writeErr := os.WriteFile(path, out.Content, 0o644); writeErr != nil {
//line lit/gtangle.w:112
			return writeErr
//line lit/gtangle.w:113
		}
//line lit/gtangle.w:114
		if out.Warning != "" {
//line lit/gtangle.w:115
			fmt.Fprintf(os.Stderr, "gtangle: warning: %s: %s\n", path, out.Warning)
//line lit/gtangle.w:116
		}
//line lit/gtangle.w:117
		fmt.Printf("gtangle: wrote %s (%d bytes)\n", path, len(out.Content))
//line lit/gtangle.w:118
	}
//line lit/gtangle.w:119
	return nil
//line lit/gtangle.w:120
}
