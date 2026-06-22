# GWEB syntax highlighting for Emacs

`gweb-mode` is a major mode for [GWEB](https://github.com/sjnam/gweb)
literate-programming files (`.w`, and change files `.ch`).

It highlights the `@`-control codes, named-section brackets `@<name@>` and
`@(file@>=`, `|...|` inline code, and the TeX control sequences of the
commentary.

## Install

Put `gweb-mode.el` on your `load-path` and require it:

```elisp
(add-to-list 'load-path "/path/to/gweb/editors/emacs")
(require 'gweb-mode)
```

`gweb-mode` then opens automatically for `.w` and `.ch` files. With `use-package`:

```elisp
(use-package gweb-mode
  :load-path "/path/to/gweb/editors/emacs"
  :mode ("\\.w\\'" "\\.ch\\'"))
```

## Full Go + TeX highlighting (optional)

This standalone mode highlights GWEB's structure, but does not fontify the code
parts as full Go or the commentary as full TeX. For that, layer a multi-mode
package such as [polymode](https://polymode.github.io/) or `mmm-mode` on top.
