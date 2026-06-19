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

Both commands accept `-o <dir>` to choose an output directory, and an optional
**change file** as a second argument — `gtangle foo.w foo.ch` — which patches the
master source without editing it (CWEB's `.ch` mechanism; see
[doc/format.md](doc/format.md)). For example,
`gtangle examples/wc.w examples/wc.ch` builds a CSV variant of the word counter. `gwebmac.tex`
lives in [tex/](tex/); point `TEXINPUTS` at that directory, or copy the file
next to your document.

## Try the examples

```sh
make example                 # tangle & weave every examples/*.w into .go and .pdf
make -C examples NAME=pmap   # just one example
```

* [examples/wc.w](examples/wc.w) — a literate word-count program; its tangled
  output matches the system `wc`. It also shows `@f` setting a user type in bold.
* [examples/pmap.w](examples/pmap.w) — a generic concurrent `map` over a slice,
  exercising generics, goroutines, channels, and `sync.WaitGroup`.

`make test` (the non-`-short` run) tangles every example and `go build`s the
result, so the examples are guaranteed to stay compilable.

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
  identifiers, typewriter strings, real math symbols for `≤ ≥ ≠ ←`, …) and
  *mirrors the source's own spacing* rather than re-deriving layout from a full
  Go grammar the way CWEB does for C. Because gofmt-formatted Go already encodes
  the grammar in its spacing, mirroring it reproduces gofmt exactly — including
  the tricky cases (`*T` vs `a * b`, `[]T` vs `a[i]`, precedence spacing like
  `a*b + c`) — without any parsing. Long code lines wrap at the inter-token
  spaces, with continuation lines indented one step deeper. Write the code in
  your sections in gofmt style for the best-looking output.
* **Definition detection** in the index is heuristic (an identifier following
  `func`/`var`/`const`/`type`, or just left of `:=`), not a full type check.
* **Diagnostics.** Both tools report, with `file:line` locations, unterminated
  control codes, references to undefined sections, ambiguous `...` abbreviations,
  and named sections defined but never used. These are warnings; `gtangle` still
  stops if it must actually expand an undefined reference. (With `@i` includes,
  line numbers refer to the concatenated source.)
* **Table of contents.** Starred sections record themselves (number, title, page)
  as the document is shaped, and `gwebmac.tex` typesets a contents page from that
  data in a single TeX pass. As in CWEB, the contents page is emitted at the *end*
  of the document; move it to the front when binding if you prefer.
