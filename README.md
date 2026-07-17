# GWEB — Literate programming for Go

[![CI](https://github.com/sjnam/gweb/actions/workflows/ci.yml/badge.svg)](https://github.com/sjnam/gweb/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**GWEB** is a [literate programming](https://www-cs-faculty.stanford.edu/~knuth/lp.html)
system for the Go programming language, modeled closely on Donald Knuth and Silvio Levy's
**CWEB**. You write a single `.w` source file that interleaves TeX
documentation with Go code, and two tools turn it into either a program or a
typeset document — exactly as **CWEB** does for C, with C replaced by Go:

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

## Prerequisites

**GWEB** assumes you already know **Go** and are comfortable with **TeX** — in
particular *plain* TeX, since `gwebmac.tex` and this repository's macros are
plain TeX, not LaTeX. Beyond that:

* A **TeX distribution** (e.g. [TeX Live](https://tug.org/texlive/) or MacTeX)
  must be installed, providing the `tex`, `pdftex`, and `luatex` engines the
  woven output is typeset with.
* For Korean output (`\input kotexgweb`; see [Usage](#usage)), the
  **Noto Serif/Sans CJK KR** fonts must be installed locally — `kotexgweb.tex`
  selects them by name.

## Build

```sh
make build        # builds bin/gtangle and bin/gweave
make test         # runs the test suite
```

As in **CWEB**, only the Go needed to build `gtangle` is committed; `gweave` and the
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
`GOBIN`). Point `TEXINPUTS` at a copy of [gwebmac.tex](gwebmac.tex) so
the TeX engine can find it.

For a full install — the commands **plus** `gwebmac.tex` (placed in your TeX tree
so `pdftex foo.tex` just works) **plus** the man page — use the install script:

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

The `.w` extension may be omitted, as in **CWEB** — `gtangle foo` reads `foo.w` (and
a bare change-file name gets `.ch`). Each command prints a brief cweb-style
progress line, one `*N` per starred (chapter) section.

For a hyperlinked PDF you can also go through DVI, exactly as **CWEB** does — request
the `\special{pdf:…}` back end with `\let\pdf+`, then convert with `dvipdfmx`:

```sh
tex "\let\pdf+ \input foo.tex"   # -> foo.dvi  (with pdf: specials)
dvipdfmx foo.dvi                 # -> foo.pdf  (links + bookmarks)
```

Both commands accept `-o <dir>` to choose an output directory, and an optional
**change file** as a second argument — `gtangle foo.w foo.ch` — which patches the
master source without editing it (**CWEB**'s `.ch` mechanism; see
[format.md](format.md)). For example,
`gtangle examples/wc.w examples/wc.ch` builds a CSV variant of the word counter. `gwebmac.tex`
lives at the repository root; point `TEXINPUTS` there, or copy the file
next to your document.

The tangled Go always carries `//line` directives, so the Go compiler, `go vet`,
and panic traces report errors at **`.w`** positions instead of `.go` ones — the
Go counterpart of **CWEB**'s `#line`, which `ctangle` likewise emits unconditionally:

```sh
gtangle foo.w && go build .       # an error reads  foo.w:42: ...
```

**GWEB** tangles its own sources the same way, so editing a `.w` reshuffles the line
numbers in the bootstrap Go it commits — the price of keeping the generated code
honest about its literate origin.

For a tour of the bundled examples, the full `.w` file-format reference, and
non-English (e.g. Korean) documentation, see the manual — `gwebman.tex`
(`make manual`).

## How it is organized

Each `.w` source sits next to the Go it generates. Only the Go that bootstraps
`gtangle` is committed (◇ below); the rest (✦) is tangled by `make`.

```text
gweb.w             the master web: @i-includes the three below (woven, not tangled)
cmd/
├── gtangle/       gtangle.w -> gtangle: front end + the tangle engine        ◇
└── gweave/        gweave.w -> gweave: front end + the weave engine (lexer,        ✦
                   pretty-printer, cross-references)
common/            common.w -> the shared parser (CWEB's common.w)        ◇
gwebmac.tex        TeX macros for woven output (CWEB's cwebmac.tex)
kotexgweb.tex      Korean (luatexko) localization + fonts + LuaTeX PDF back end
gweb.1             the man page for both commands (CWEB's cweb.1)
format.md          the .w file-format reference
gwebman.tex        the GWEB manual
examples/          worked examples
editors/
└── vscode/        VS Code language support for .w files
install.sh         installer for the commands, gwebmac.tex, and the man page
```

For editor support and how **GWEB**'s self-hosting (`make tangle`, `make
bootstrap`, `make selfdoc`) works, see the manual as well.

## Design notes and limitations

* **Lexing for free.** Tangle relies on `gofmt` (`go/format`) to canonicalize
  the assembled program, so the emitted Go is always tidily formatted as long as
  the web assembles into valid Go.
* **Pretty-printing.** `gweave` highlights tokens (bold keywords, italic
  identifiers, typewriter strings, real math symbols for `≤ ≥ ≠ ←`, …) and
  *mirrors the source's own spacing* rather than re-deriving layout from a full
  Go grammar the way **CWEB** does for C. Because gofmt-formatted Go already encodes
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
  furniture **CWEB** produces: run-in bold section headings (with a page break before
  each major starred section), `§sec  JOBNAME … GROUPTITLE  page` running heads, a
  title-less index that sits under your own `@* Index.` section, a *Names of the
  Sections* list, and a contents page (centered title, `Section`/`Page` columns,
  dotted leaders). The index is set in two columns. As in **CWEB**, the contents page
  is produced at the *end* in a single TeX pass; move it to the front when binding
  if you prefer. The font set mirrors `cwebmac.tex` (medium-caps `\mc`, the
  `cmtex10` string font, …), and a few `cwebmac` prose helpers are available in
  commentary: `\CEE/`, `\GO/`, `\UNIX/`, `\TEX/`, and `\\{id}` for an italic
  identifier (though the **GWEB** idiom for inline code is `|id|`).
* **Hyperlinks and bookmarks.** Every section number shown as a reference, in the
  index, in a cross-reference note, or on the contents page is a blue link to that
  section; clicking it jumps there (for the underlined index entries, to where the
  identifier is defined). The starred sections also become a PDF outline (bookmark
  tree), nested by their `@*`, `@*1`, `@*2` depths. Two back ends produce these:
  `pdftex`/`luatex` in PDF mode use the engine's own primitives, while the DVI
  route emits `\special{pdf:…}` commands for `dvipdfmx` when you ask for them with
  `\let\pdf+` (the same convention as **CWEB**). With a plain DVI engine and no
  `\let\pdf+`, the links and bookmarks are simply omitted.

## License

**GWEB** is released under the [MIT License](LICENSE).
