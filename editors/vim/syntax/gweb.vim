" Vim syntax file
" Language:    GWEB (literate programming for Go; the Go analogue of CWEB)
" Filenames:   *.w *.ch
" Maintainer:  GWEB -- https://github.com/sjnam/gweb
"
" A GWEB web is TeX commentary interleaved with Go code parts, tied together by
" @-control codes. This file embeds the Go grammar for the code parts and for
" |...| inline spans, lightly highlights the TeX commentary, and marks every
" control code.

if exists("b:current_syntax")
  finish
endif

" Embed Go for code parts and |...| inline spans.
syn include @gwebGo syntax/go.vim
unlet! b:current_syntax

" Regions span the file (a code part reaches to the next section), so sync from
" the start for correct highlighting.
syn sync fromstart

" A literal '@@' first, so it is never read as a control code.
syn match gwebAt "@@" containedin=ALL

" Argument-terminated groups: @=...@> (verbatim), @t...@> (TeX), and the index
" codes @^...@>, @....@>, @:...@>, and @q...@> (comment).
syn region gwebGroup matchgroup=gwebControl start="@[=t^.:q]" end="@>" keepend
      \ contains=gwebAt

" A named-section reference (no trailing '='): @<name@> or @(name@>.
syn match gwebNameDelim "@[<(]" contained
syn match gwebNameDelim "@>"    contained
syn match gwebName "@[<(][^@]*@>=\@!" contains=gwebNameDelim

" Code parts: a code-start control code up to (but not including) the next
" section break -- '@ ', '@\t', '@\n', or '@*'. The unnamed '@c'/'@p', a
" definition '@<name@>=', and a file output '@(file@>=' all begin one.
syn region gwebCode matchgroup=gwebControl
      \ start="@[cp]\>"
      \ start="@[<(][^@]*@>="
      \ end="@\ze[ \t\r\n*]"
      \ keepend contains=@gwebGo,gwebName,gwebGroup,gwebAt,gwebLayout

" Inline Go inside the TeX commentary: |...|.
syn region gwebInline matchgroup=gwebInlineDelim start="|" end="|" oneline
      \ contains=@gwebGo

" Layout and other in-code control codes (also valid in prose).
syn match gwebLayout "@[,/|#&!+;[\]]"

" Documentation-side control codes and section openers.
syn match gwebControl "@[dfsixyz]\>"
syn match gwebControl "@%"
syn match gwebSection "@\*\*\|@\*\d*\|@[ \t]\|@$"

" Light TeX highlighting for the commentary.
syn match  gwebTeX "\\[a-zA-Z@]\+"
syn match  gwebTeX "\\[^a-zA-Z@]"
syn region gwebMath matchgroup=gwebMathDelim start="\$" skip="\\\$" end="\$"
      \ oneline contains=gwebTeX,gwebAt

hi def link gwebControl     PreProc
hi def link gwebSection     Special
hi def link gwebLayout      PreProc
hi def link gwebAt          PreProc
hi def link gwebName        Identifier
hi def link gwebNameDelim   PreProc
hi def link gwebGroup       String
hi def link gwebInlineDelim Delimiter
hi def link gwebTeX         Statement
hi def link gwebMath        Constant
hi def link gwebMathDelim   Delimiter

let b:current_syntax = "gweb"
