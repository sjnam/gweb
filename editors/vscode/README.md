# GWEB language support for VS Code

Language support for [GWEB](https://github.com/sjnam/gweb) literate-programming
files (`.w`, and change files `.ch`): syntax highlighting, plus real IDE
features in the Go code parts, borrowed from your installed Go extension.

## Features

**Syntax highlighting** for the control codes — section openers (`@*`, `@*N`,
`@ `), code starts (`@c`/`@p`, `@<name@>=`, `@(file@>=`), references
`@<name@>`, directives `@d`/`@f`/`@s`, includes `@i`, index entries `@^…@>`
`@.…@>` `@:…@>`, verbatim `@=…@>`, `@t…@>`, the layout codes `@,@/@|@#@&@!`,
`@@`, `@%`, and change-file `@x`/`@y`/`@z` — with **Go** embedded in every code
part and in `|…|` inline spans, and light **TeX** highlighting in the
commentary.

**Go to Definition (F12) and Hover in code parts.** gtangle writes a
`//line file.w:N` directive for virtually every line of its output, so the
tangled `.go` sitting next to your `.w` carries a line-accurate source map back
to the web. The extension maps your cursor position through that table to the
tangled `.go`, asks the Go language support (gopls, via the standard VS Code
provider commands), and maps the answer back — including across webs: a jump
from `gtangle.w` into code defined in `common.w` lands in `common.w`.
Definitions in plain Go packages (the standard library, dependencies) open
normally.

**Section-name navigation.** F12 on a `@<section name@>` reference jumps to its
`@<section name@>=` definition (all of them, when a name is defined in several
pieces); `prefix...` abbreviations resolve. Hovering a name shows where it is
defined.

**Tangle on save.** Saving a `.w` re-runs gtangle so the source map stays
fresh. Only a web whose tangled `.go` already exists is re-tangled — the
extension never creates one behind your back; run the **GWEB: Tangle current
file** command for the first tangle (this also lets weave-only masters, like a
`doc/gweb.w` that only `@i`-includes other webs, sit untangled).

## Requirements

* **gtangle** on your `PATH` (or point `gweb.gtanglePath` at it).
* The **Go extension** (`golang.go`) for definition/hover in code parts —
  anything that provides Go language features works, since requests go through
  VS Code's provider commands.
* The tangled `.go` next to the `.w` (the normal GWEB workflow). Definition and
  hover answer from the **last saved** state of the web.

## Settings

| setting | default | meaning |
|---|---|---|
| `gweb.gtanglePath` | `gtangle` | gtangle executable |
| `gweb.tangleOnSave` | `true` | re-tangle a saved `.w` whose `.go` exists |

## Limitations

* Positions are mapped by line (exact, via `//line`) and by identifier
  occurrence within the line (gofmt re-indents, so raw columns differ); on rare
  heavily-transformed lines a jump may land at the start of the right line.
* Edits since the last save aren't in the tangled `.go`, so brand-new code
  resolves after you save.
* A file included with `@i` maps only where its lines land in the including
  web's output; rename and find-references are not forwarded (yet).

## Install (recommended: from a VSIX)

Installing the packaged `.vsix` registers the extension **globally**, so every
window and project gets it. Grab `gweb-<version>.vsix` from the
[releases page](https://github.com/sjnam/gweb/releases) (or build one — see
below), then either:

* **GUI** — Extensions view (`Cmd/Ctrl+Shift+X`) → the `⋯` menu →
  *Install from VSIX…* → pick the file; or
* **CLI** — `code --install-extension gweb-<version>.vsix`.

Then **fully quit and relaunch** VS Code (`Cmd/Ctrl+Q`, not just close the
window) so all windows pick it up.

Open any `.w` file; the language indicator at the bottom right should read
**GWEB**. If not, click it and choose *Change Language Mode → GWEB*.

## Build the VSIX

With [`vsce`](https://github.com/microsoft/vscode-vsce):

```sh
cd editors/vscode && npx @vscode/vsce package   # -> gweb-<version>.vsix
```

## Install by copying (fallback)

You can instead drop the folder into your extensions directory, but a manual
copy is only picked up after a **full restart** of VS Code, and is easy to
leave stale — prefer the VSIX above.

```sh
cp -r editors/vscode ~/.vscode/extensions/gweb-0.2.0   # macOS / Linux
# Windows: %USERPROFILE%\.vscode\extensions\gweb-0.2.0
```

If things work in one project but not another, you most likely have a stale
manual copy: uninstall **GWEB** from the Extensions view (or delete the
`~/.vscode/extensions/gweb-*` folder), install the VSIX, and relaunch.
