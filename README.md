# GWEB — Literate programming for Go

[![CI](https://github.com/sjnam/gweb/actions/workflows/ci.yml/badge.svg)](https://github.com/sjnam/gweb/actions/workflows/ci.yml)

GWEB is a [literate programming](https://www-cs-faculty.stanford.edu/~knuth/lp.html)
system for the Go programming language, modeled closely on Donald Knuth and Silvio Levy's
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
  automatically. Declared `type` names are set bold and `const` names typewriter;
  mark other names typewriter too (e.g. another package's `@d http.StatusOK`), and
  override any of it with `@f`/`@s`.

## Build

```sh
make build        # builds bin/gtangle and bin/gweave
make test         # runs the test suite
```

As in CWEB, only the Go needed to build `gtangle` is committed; `gweave` and the
weave engine are tangled from their `.w` sources on the fly. `make build` does
this for you: it builds `gtangle` from the committed sources, runs it to generate
the rest, then builds `gweave`. (`make generate` performs just the tangling step.)
So build through `make`, not a bare `go build ./...`, on a fresh checkout.

Producing a PDF additionally requires a TeX engine (e.g. `pdftex`, from TeX
Live or MacTeX).

## Install

`gtangle` installs the Go way (its sources are committed):

```sh
go install github.com/sjnam/gweb/cmd/gtangle@latest
```

`gweave` is generated rather than committed, so install both from a checkout
with `make install` (or `make install-tools` to put just the binaries in your
`GOBIN`). Point `TEXINPUTS` at a copy of [tex/gwebmac.tex](tex/gwebmac.tex) so
the TeX engine can find it.

For a full install — the commands **plus** `gwebmac.tex` (placed in your TeX tree
so `pdftex foo.tex` just works) **plus** the man pages — use the install script:

```sh
./install.sh                       # into ~ (TEXMFHOME) and /usr/local (may need sudo)
./install.sh --prefix="$HOME/.local"   # a writable prefix, no sudo
sudo ./install.sh                  # system-wide
./install.sh --uninstall           # remove everything it installed
```

`make install` / `make uninstall` call the script (pass options with
`ARGS=...`); run `./install.sh --help` for all options (`--bindir`, `--mandir`,
`--texmf`). Then `man gtangle` / `man gweave` describe the commands.

## Usage

```sh
gtangle foo.w     # -> foo.go (plus any @(file@>= outputs)
gweave  foo.w     # -> foo.tex
pdftex  foo.tex   # -> foo.pdf   (gwebmac.tex must be on TEXINPUTS)
```

The `.w` extension may be omitted, as in CWEB — `gtangle foo` reads `foo.w` (and
a bare change-file name gets `.ch`). Each command prints a brief cweb-style
progress line, one `*N` per starred (chapter) section.

For a hyperlinked PDF you can also go through DVI, exactly as CWEB does — request
the `\special{pdf:…}` back end with `\let\pdf+`, then convert with `dvipdfmx`:

```sh
tex "\let\pdf+ \input foo.tex"   # -> foo.dvi  (with pdf: specials)
dvipdfmx foo.dvi                 # -> foo.pdf  (links + bookmarks)
```

### Korean (and other non-English) documentation

The woven output can be written in Korean by processing it with **`luatex`**
(not `pdftex`) and putting one line in the `.w` file's limbo:

```tex
\input kotexgweb.tex
```

[tex/kotexgweb.tex](tex/kotexgweb.tex) loads [luatexko](https://ctan.org/pkg/luatexko)
and selects the Noto Serif/Sans CJK KR fonts (edit the `\sethangulfont` lines to
change typefaces), translates gweave's fixed wording into Korean, and supplies a
LuaTeX PDF back end so that blue cross-reference links and the PDF outline
(bookmark) pane work, with Korean bookmark titles. Then:

```sh
gweave foo.w           # -> foo.tex
luatex foo.tex         # -> foo.pdf   (kotexgweb.tex on TEXINPUTS)
```

gweave needs no flag; all the human-readable text it emits goes through macros
(`\GU`, `\GNused`, `\Gsectionword`, …) that `kotexgweb.tex` overrides, so the same
mechanism localizes to any language — write your own `\input` file modelled on it.

Both commands accept `-o <dir>` to choose an output directory, and an optional
**change file** as a second argument — `gtangle foo.w foo.ch` — which patches the
master source without editing it (CWEB's `.ch` mechanism; see
[doc/format.md](doc/format.md)). For example,
`gtangle examples/wc.w examples/wc.ch` builds a CSV variant of the word counter. `gwebmac.tex`
lives in [tex/](tex/); point `TEXINPUTS` at that directory, or copy the file
next to your document.

The tangled Go always carries `//line` directives, so the Go compiler, `go vet`,
and panic traces report errors at **`.w`** positions instead of `.go` ones — the
Go counterpart of CWEB's `#line`, which `ctangle` likewise emits unconditionally:

```sh
gtangle foo.w && go build .       # an error reads  foo.w:42: ...
```

GWEB tangles its own sources the same way, so editing a `.w` reshuffles the line
numbers in the bootstrap Go it commits — the price of keeping the generated code
honest about its literate origin.

## Try the examples

```sh
make example                 # tangle & weave every examples/*.w into .go and .pdf
make -C examples NAME=wc     # just one example
```

* [examples/wc.w](examples/wc.w) — a literate word-count program; its tangled
  output matches the system `wc`. It also shows `@f` setting a user type in bold.
* [examples/seq.w](examples/seq.w) — a tiny lazy-sequence library (`Map`,
  `Filter`, `Take` over infinite Fibonacci numbers), showing off the Go features
  C has no answer to: first-class functions and closures, anonymous functions,
  generics, and Go 1.23 range-over-func iterators.
* [examples/sham.w](examples/sham.w) — a GWEB port of Knuth's Stanford GraphBase
  demo `sham`: count the symmetric Hamiltonian cycles of the knight's graph on an
  8×9 board, by folding the graph in half and backtracking with `goto` labels. It
  builds on [go-sgb](https://github.com/sjnam/go-sgb), a Go port of the SGB, so
  running it needs that module (`go get github.com/sjnam/go-sgb`); the commentary
  is newly written. Shows GWEB handling an external dependency and a real Knuth
  program.
* [examples/floyd.w](examples/floyd.w) — Floyd's partition problem, the classic
  "toy problem" Knuth discusses in *Are Toy Problems Useful?*: partition
  √1…√50 into two nearly-equal halves. A worked literate solution
  (meet-in-the-middle search, Gray-code enumeration, compensated summation, and
  a `math/big` verification).
* [examples/hangul.w](examples/hangul.w) — a short Fibonacci program written in
  Korean, demonstrating `\input kotexgweb.tex`. Typeset it with **luatex**:
  `make -C examples NAME=hangul TEXENGINE=luatex` (or `make example`).

`make test` (the non-`-short` run) tangles every example and `go build`s the
result, so the examples are guaranteed to stay compilable.

You can find more GWEB examples at [gweb-examples](https://github.com/sjnam/gweb-examples).

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
| `\|Go code\|` | inline Go code inside prose                                |

The full list is in [doc/format.md](doc/format.md).

## How it is organized

Each `.w` source sits next to the Go it generates. Only the Go that bootstraps
`gtangle` is committed (marked ◇ below); the rest (✦) is tangled by `make`.

```text
gweb.w           the master web: @i-includes the three below (woven, not tangled)
cmd/gtangle      gtangle.w -> gtangle: front end + the tangle engine        ◇
cmd/gweave       gweave.w  -> gweave: front end + the weave engine (lexer,   ✦
                 pretty-printer, cross-references)
common           common.w -> the shared parser (CWEB's common.w)            ◇
tex/gwebmac.tex  TeX macros for woven output (CWEB's cwebmac.tex)
tex/kotexgweb.tex  Korean (luatexko) localization + fonts + LuaTeX PDF back end
man/             gtangle.1 and gweave.1 man pages
doc/             format reference and the gwebman.tex manual
examples/        worked examples
editors/vscode   VS Code syntax highlighting for .w files
install.sh       installer for the commands, gwebmac.tex, and man pages
```

## Editor support

A VS Code extension in [editors/vscode/](editors/vscode/) highlights `.w` files:
the control codes, embedded **Go** in code parts and `|…|` spans, and the TeX
commentary. To use it, install the `.vsix` from the
[releases page](https://github.com/sjnam/gweb/releases)
(Extensions view → *Install from VSIX…*), then fully relaunch.

See [editors/vscode/README.md](editors/vscode/README.md) for details.

## Self-hosting

Like CWEB, GWEB is written in itself. Each `.w` source sits next to its output:
[common/common.w](common/common.w) (the shared parser),
[cmd/gtangle/gtangle.w](cmd/gtangle/gtangle.w) (the command and the tangle
engine), and [cmd/gweave/gweave.w](cmd/gweave/gweave.w) (the command and the
weave engine) are the source of truth; the `.go` beside them is tangled from
them. GWEB is a set of commands, not a library, so each engine lives in its
command's `main` package rather than a separate importable one.

```sh
make tangle       # re-tangle the .w sources back into the Go tree
make bootstrap    # tangle into a scratch tree and verify it is byte-for-byte
                  # identical to the committed sources (the fixpoint)
```

Editing workflow: change a `.w`, run `make tangle`, then commit both. The
`bootstrap` target is the self-hosting proof — a freshly built `gtangle`
reproduces its own committed source exactly. The tests live in the `.w` sources
too (a `Tests` chapter per web), so they are tangled and git-ignored along with
the rest of the generated Go: no `_test.go` is committed.

`gweb.w` (at the repo doc) is a master that `@i`-includes the three component
webs in reading order, so `gweave` can typeset the whole system as one document:

```sh
make selfdoc      # -> build/gweb.pdf, GWEB woven as a literate program
```

The result is GWEB documenting itself — the same pretty-printed code, index, and
cross-references it produces for any other program.

The user manual, `doc/gwebman.tex` — a guide for readers who already know CWEB,
typeset in the small-book format of Knuth and Levy's `cwebman` — builds with:

```sh
make manual       # -> build/gwebman.pdf
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
  stops if it must actually expand an undefined reference. An origin map kept in
  step through `@i` includes and change-file edits makes every location point
  back to the file (and line) you actually wrote.
* **Page layout follows cweave.** `gwebmac.tex` gives the woven document the same
  furniture CWEB produces: run-in bold section headings (with a page break before
  each major starred section), `§sec  JOBNAME … GROUPTITLE  page` running heads, a
  title-less index that sits under your own `@* Index.` section, a *Names of the
  Sections* list, and a contents page (centered title, `Section`/`Page` columns,
  dotted leaders). The index is set in two columns. As in CWEB, the contents page
  is produced at the *end* in a single TeX pass; move it to the front when binding
  if you prefer. The font set mirrors `cwebmac.tex` (medium-caps `\mc`, the
  `cmtex10` string font, …), and a few `cwebmac` prose helpers are available in
  commentary: `\CEE/`, `\GO/`, `\UNIX/`, `\TEX/`, and `\\{id}` for an italic
  identifier (though the GWEB idiom for inline code is `|id|`).
* **Hyperlinks and bookmarks.** Every section number shown as a reference, in the
  index, in a cross-reference note, or on the contents page is a blue link to that
  section; clicking it jumps there (for the underlined index entries, to where the
  identifier is defined). The starred sections also become a PDF outline (bookmark
  tree), nested by their `@*`, `@*1`, `@*2` depths. Two back ends produce these:
  `pdftex`/`luatex` in PDF mode use the engine's own primitives, while the DVI
  route emits `\special{pdf:…}` commands for `dvipdfmx` when you ask for them with
  `\let\pdf+` (the same convention as CWEB). With a plain DVI engine and no
  `\let\pdf+`, the links and bookmarks are simply omitted.
