# GWEB syntax highlighting for VS Code

Syntax highlighting for [GWEB](https://github.com/sjnam/gweb) literate-programming
files (`.w`, and change files `.ch`).

It highlights:

* the control codes — section openers (`@*`, `@*N`, `@ `), code starts
  (`@c`/`@p`, `@<name@>=`, `@(file@>=`), references `@<name@>`, directives
  `@d`/`@f`/`@s`, includes `@i`, index entries `@^…@>` `@.…@>` `@:…@>`, verbatim
  `@=…@>`, `@t…@>`, the layout codes `@,@/@|@#@&@!`, `@@`, `@%`, and change-file
  `@x`/`@y`/`@z`;
* **Go** code, embedded in every code part and in `|…|` inline spans (uses VS
  Code's built-in Go grammar);
* the **TeX** commentary (control sequences, `%` comments, `$…$` math, groups).

## Install (recommended: from a VSIX)

Installing the packaged `.vsix` registers the extension **globally**, so every
window and project gets `.w` highlighting. Grab `gweb-<version>.vsix` from the
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
copy is only picked up after a **full restart** of VS Code, and is easy to leave
stale — prefer the VSIX above.

```sh
cp -r editors/vscode ~/.vscode/extensions/gweb-0.1.5   # macOS / Linux
# Windows: %USERPROFILE%\.vscode\extensions\gweb-0.1.5
```

If highlighting works in one project but not another, you most likely have a
stale manual copy: uninstall **GWEB** from the Extensions view (or delete the
`~/.vscode/extensions/gweb-*` folder), install the VSIX, and relaunch.
