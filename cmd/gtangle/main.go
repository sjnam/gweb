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
//line cmd/gtangle/gtangle.w:8
//line cmd/gtangle/gtangle.w:9
//line cmd/gtangle/gtangle.w:10
//line cmd/gtangle/gtangle.w:11
//line cmd/gtangle/gtangle.w:12
//line cmd/gtangle/gtangle.w:13
//line cmd/gtangle/gtangle.w:14
//line cmd/gtangle/gtangle.w:15
//line cmd/gtangle/gtangle.w:16
//line cmd/gtangle/gtangle.w:17
//line cmd/gtangle/gtangle.w:18
//line cmd/gtangle/gtangle.w:19
package main

//line cmd/gtangle/gtangle.w:21
import (
//line cmd/gtangle/gtangle.w:22
	"flag"
//line cmd/gtangle/gtangle.w:23
	"fmt"
//line cmd/gtangle/gtangle.w:24
	"os"
//line cmd/gtangle/gtangle.w:25
	"path/filepath"
//line cmd/gtangle/gtangle.w:26
	"strings"

//line cmd/gtangle/gtangle.w:28
	"github.com/sjnam/gweb/internal/web"
//line cmd/gtangle/gtangle.w:29
)

//line cmd/gtangle/gtangle.w:35
func main() {
//line cmd/gtangle/gtangle.w:36
	outDir := flag.String("o", "", "output directory (default: input file's directory)")
//line cmd/gtangle/gtangle.w:37
	showVersion := flag.Bool("version", false, "print version and exit")
//line cmd/gtangle/gtangle.w:38
	flag.Usage = usage
//line cmd/gtangle/gtangle.w:39
	flag.Parse()
//line cmd/gtangle/gtangle.w:40
	if *showVersion {
//line cmd/gtangle/gtangle.w:41
		fmt.Printf("gtangle (GWEB) %s\n", web.Version)
//line cmd/gtangle/gtangle.w:42
		return
//line cmd/gtangle/gtangle.w:43
	}
//line cmd/gtangle/gtangle.w:44
	if flag.NArg() < 1 || flag.NArg() > 2 {
//line cmd/gtangle/gtangle.w:45
		usage()
//line cmd/gtangle/gtangle.w:46
		os.Exit(2)
//line cmd/gtangle/gtangle.w:47
	}
//line cmd/gtangle/gtangle.w:48
	fmt.Fprintf(os.Stderr, "This is GTANGLE, Version %s\n", web.Version)
//line cmd/gtangle/gtangle.w:49
	if err := run(flag.Arg(0), flag.Arg(1), *outDir); err != nil {
//line cmd/gtangle/gtangle.w:50
		fmt.Fprintln(os.Stderr, "gtangle:", err)
//line cmd/gtangle/gtangle.w:51
		os.Exit(1)
//line cmd/gtangle/gtangle.w:52
	}
//line cmd/gtangle/gtangle.w:53
}

//line cmd/gtangle/gtangle.w:57
func usage() {
//line cmd/gtangle/gtangle.w:58
	fmt.Fprintln(os.Stderr, "usage: gtangle [-o dir] file[.w] [change[.ch]]")
//line cmd/gtangle/gtangle.w:59
	flag.PrintDefaults()
//line cmd/gtangle/gtangle.w:60
}

//line cmd/gtangle/gtangle.w:66
func reportProgress(w *web.Web) {
//line cmd/gtangle/gtangle.w:67
	for _, s := range w.Sections {
//line cmd/gtangle/gtangle.w:68
		if s.Starred {
//line cmd/gtangle/gtangle.w:69
			fmt.Fprintf(os.Stderr, "*%d", s.Number)
//line cmd/gtangle/gtangle.w:70
		}
//line cmd/gtangle/gtangle.w:71
	}
//line cmd/gtangle/gtangle.w:72
	fmt.Fprintln(os.Stderr)
//line cmd/gtangle/gtangle.w:73
}

//line cmd/gtangle/gtangle.w:80
func run(input, changeFile, outDir string) error {
//line cmd/gtangle/gtangle.w:81
	input = web.DefaultExt(input, ".w")
//line cmd/gtangle/gtangle.w:82
	changeFile = web.DefaultExt(changeFile, ".ch")
//line cmd/gtangle/gtangle.w:83
	w, err := web.ParseWithChange(input, changeFile)
//line cmd/gtangle/gtangle.w:84
	if err != nil {
//line cmd/gtangle/gtangle.w:85
		return err
//line cmd/gtangle/gtangle.w:86
	}
//line cmd/gtangle/gtangle.w:87
	for _, warn := range w.Warnings {
//line cmd/gtangle/gtangle.w:88
		fmt.Fprintln(os.Stderr, "gtangle: warning:", warn)
//line cmd/gtangle/gtangle.w:89
	}
//line cmd/gtangle/gtangle.w:90
	reportProgress(w)
//line cmd/gtangle/gtangle.w:91
	if outDir == "" {
//line cmd/gtangle/gtangle.w:92
		outDir = filepath.Dir(input)
//line cmd/gtangle/gtangle.w:93
	}

//line cmd/gtangle/gtangle.w:95
	base := filepath.Base(input)
//line cmd/gtangle/gtangle.w:96
	base = strings.TrimSuffix(base, filepath.Ext(base))
//line cmd/gtangle/gtangle.w:97
	defaultFile := base + ".go"

//line cmd/gtangle/gtangle.w:99
	outs, err := New(w).Tangle(defaultFile)
//line cmd/gtangle/gtangle.w:100
	if err != nil {
//line cmd/gtangle/gtangle.w:101
		return err
//line cmd/gtangle/gtangle.w:102
	}

//line cmd/gtangle/gtangle.w:104
	for _, out := range outs {
//line cmd/gtangle/gtangle.w:105
		path := filepath.Join(outDir, out.File)
//line cmd/gtangle/gtangle.w:106
		if dir := filepath.Dir(path); dir != "." {
//line cmd/gtangle/gtangle.w:107
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
//line cmd/gtangle/gtangle.w:108
				return mkErr
//line cmd/gtangle/gtangle.w:109
			}
//line cmd/gtangle/gtangle.w:110
		}
//line cmd/gtangle/gtangle.w:111
		if writeErr := os.WriteFile(path, out.Content, 0o644); writeErr != nil {
//line cmd/gtangle/gtangle.w:112
			return writeErr
//line cmd/gtangle/gtangle.w:113
		}
//line cmd/gtangle/gtangle.w:114
		if out.Warning != "" {
//line cmd/gtangle/gtangle.w:115
			fmt.Fprintf(os.Stderr, "gtangle: warning: %s: %s\n", path, out.Warning)
//line cmd/gtangle/gtangle.w:116
		}
//line cmd/gtangle/gtangle.w:117
		fmt.Printf("gtangle: wrote %s (%d bytes)\n", path, len(out.Content))
//line cmd/gtangle/gtangle.w:118
	}
//line cmd/gtangle/gtangle.w:119
	return nil
//line cmd/gtangle/gtangle.w:120
}
