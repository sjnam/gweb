# GWEB syntax highlighting for Vim

Syntax highlighting for [GWEB](https://github.com/sjnam/gweb) literate-programming
files (`.w`, and change files `.ch`).

It highlights the `@`-control codes, named-section brackets `@<name@>` and
`@(file@>=`, and embeds Vim's built-in **Go** syntax in every code part and in
`|...|` inline spans; the TeX commentary gets light control-sequence and `$...$`
math highlighting.

## Install

Copy the two files into your Vim runtime path:

```sh
mkdir -p ~/.vim/syntax ~/.vim/ftdetect
cp editors/vim/syntax/gweb.vim   ~/.vim/syntax/
cp editors/vim/ftdetect/gweb.vim ~/.vim/ftdetect/
```

For Neovim use `~/.config/nvim/` in place of `~/.vim/`. With a plugin manager,
point it at the `editors/vim` directory instead.

Detection and highlighting must be on (usually already in your config):

```vim
filetype plugin on
syntax on
```

Open any `.w` file; the filetype should read `gweb` (`:set ft?`).

## Note on CWEB

Vim's built-in detection already claims `*.w` and `*.ch` for **CWEB**, so this
plugin sets the filetype unconditionally to win for GWEB. If you also edit CWEB
files, override per file with a modeline or `:setfiletype cweb`.
