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

## Install (development copy)

No marketplace publish is needed — just drop the folder into your VS Code
extensions directory and reload:

```sh
# macOS / Linux
cp -r editors/vscode ~/.vscode/extensions/gweb-0.1.5
# then: Command Palette -> "Developer: Reload Window"
```

(On Windows use `%USERPROFILE%\.vscode\extensions\gweb-0.1.5`.)

Open any `.w` file; the language indicator at the bottom right should read
**GWEB**. If a file is not recognized, pick it manually with
"Change Language Mode → GWEB".

## Packaging (optional)

With [`vsce`](https://github.com/microsoft/vscode-vsce):

```sh
cd editors/vscode && vsce package      # -> gweb-0.1.5.vsix
code --install-extension gweb-0.1.5.vsix
```
