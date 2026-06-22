" Detect GWEB webs and change files.
" Vim's built-in detection already claims *.w (and *.ch) for CWEB, so we set the
" filetype unconditionally to win for GWEB users. If you also edit CWEB files,
" override per file with a modeline or  :setfiletype cweb .
autocmd BufRead,BufNewFile *.w  set filetype=gweb
autocmd BufRead,BufNewFile *.ch set filetype=gweb
