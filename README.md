# GWEB — literate programming for Go

GWEB is a [literate programming](https://en.wikipedia.org/wiki/Literate_programming)
system for the Go language, modeled closely on Donald Knuth and Silvio Levy's
**CWEB**. You write a single `.w` source file that interleaves TeX
documentation with Go code, and two tools turn it into either a program or a
typeset document — exactly as CWEB does for C, with C replaced by Go:

| CWEB       | GWEB       | purpose                                             |
|------------|------------|-----------------------------------------------------|
| `ctangle`  | `gtangle`  | extract compilable source (`.go`) for the machine   |
| `cweave`   | `gweave`   | produce a typeset document (`.tex` → PDF) for people |

* **`gtangle`** strips the documentation, reassembles the named code sections in
  the order the Go compiler needs, and writes `gofmt`-formatted `.go` files.
* **`gweave`** produces a TeX file in which reserved words are bold, identifiers
  are italic, strings are typewriter, named sections are linked by number, and a
  cross-referenced **index** and **list of refinements** are generated
  automatically.

## Build

```sh
make build        # builds bin/gtangle and bin/gweave
make test         # runs the test suite
make install      # go install both commands
```

Producing a PDF additionally requires a TeX engine (e.g. `pdftex`, from TeX
Live or MacTeX).

## Usage

```sh
gtangle foo.w     # -> foo.go (plus any @(file@>= outputs)
gweave  foo.w     # -> foo.tex
pdftex  foo.tex   # -> foo.pdf   (gwebmac.tex must be on TEXINPUTS)
```

Both commands accept `-o <dir>` to choose an output directory. `gwebmac.tex`
lives in [tex/](tex/); point `TEXINPUTS` at that directory, or copy the file
next to your document.

## Try the example

```sh
make example      # tangles & weaves examples/wc.w into wc.go and wc.pdf
```

[examples/wc.w](examples/wc.w) is a literate word-count program. Its tangled
output matches the system `wc`, and its woven output is a small illustrated
document.

## The `.w` file format

A source file is a sequence of **sections**. Text before the first section is
*limbo* (TeX preamble for `gweave`, ignored by `gtangle`). Control codes begin
with `@`; write `@@` for a literal `@`. The most important codes:

| code          | meaning                                                    |
|---------------|------------------------------------------------------------|
| `@ `          | begin an ordinary section                                  |
| `@*`, `@*N`   | begin a starred (chapter) section at depth 0 / N           |
| `@c`          | begin the Go code of an unnamed section (the program)      |
| `@<name@>=`   | define a named section ("refinement")                      |
| `@<name@>`    | reference a named section (tangled in place, linked woven) |
| `@(file@>=`   | send the following code to an additional output file       |
| `@i file`     | include another `.w` file                                  |
| `@d`,`@f`,`@s`| definition / formatting directives                         |
| `@^ @. @:`    | index entries (roman / typewriter / custom)                |
| <code>&#124;Go code&#124;</code> | inline Go code inside prose                 |

The full list is in [doc/format.md](doc/format.md).

## How it is organized

```
cmd/gtangle      gtangle command
cmd/gweave       gweave command
internal/web     shared front end: parses .w into sections (CWEB's common.w)
internal/tangle  the tangle engine
internal/weave   the weave engine: Go lexer, pretty-printer, cross-references
tex/gwebmac.tex  TeX macros for woven output (CWEB's cwebmac.tex)
doc/             format reference and the gwebman.tex manual
examples/        a worked example
```

## Design notes and limitations

* **Lexing for free.** Tangle relies on `gofmt` (`go/format`) to canonicalize
  the assembled program, so the emitted Go is always tidily formatted as long as
  the web assembles into valid Go.
* **Pretty-printing.** `gweave` highlights tokens (bold keywords, italic
  identifiers, …) and *preserves the source's own line structure and
  indentation* rather than re-deriving layout from a full Go grammar the way
  CWEB does for C. Because literate Go is normally written `gofmt`-style, the
  result reads cleanly; it is a deliberately simpler model than CWEB's
  scrap-reduction prettyprinter.
* **Definition detection** in the index is heuristic (an identifier following
  `func`/`var`/`const`/`type`, or just left of `:=`), not a full type check.
* A running table of contents with page numbers is not yet generated.
