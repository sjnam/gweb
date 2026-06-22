;;; gweb-mode.el --- Major mode for GWEB literate programs  -*- lexical-binding: t; -*-

;; Author: GWEB -- https://github.com/sjnam/gweb
;; Keywords: languages, literate programming
;; Version: 0.1.5

;;; Commentary:

;; A major mode for editing GWEB (.w) literate-programming files for Go -- the
;; Go analogue of CWEB.  It highlights the @-control codes, named-section
;; brackets, |...| inline code, and the TeX control sequences of the
;; commentary.
;;
;; Full dual Go+TeX sub-grammar highlighting is out of scope for this
;; self-contained mode; layer `polymode' or `mmm-mode' on top if you want the
;; code parts fontified as Go and the commentary as TeX.

;;; Code:

(defgroup gweb nil
  "Major mode for GWEB literate programs."
  :group 'languages
  :prefix "gweb-")

(defface gweb-control-face '((t :inherit font-lock-preprocessor-face))
  "Face for GWEB @-control codes."
  :group 'gweb)

(defface gweb-section-face
  '((t :inherit font-lock-function-name-face :weight bold))
  "Face for GWEB section openers (@*, @ )."
  :group 'gweb)

(defface gweb-name-face '((t :inherit font-lock-constant-face))
  "Face for GWEB section names inside @<...@>."
  :group 'gweb)

(defconst gweb-font-lock-keywords
  `(;; A literal @@ -- keep it from being read as a control code.
    ("@@" . 'gweb-control-face)
    ;; Section openers: starred (@**, @*, @*N) and ordinary (@ , @\t, @<eol>).
    ("@\\*\\*\\|@\\*[0-9]*" . 'gweb-section-face)
    ("@[ \t]\\|@$" . 'gweb-section-face)
    ;; Named-section brackets/references: @<name@>, @(file@>, with optional '='.
    ("\\(@[<(]\\)\\([^@]*\\)\\(@>=?\\)"
     (1 'gweb-control-face) (2 'gweb-name-face) (3 'gweb-control-face))
    ;; Argument-terminated groups: @=...@>, @t...@>, @^/@./@:/@q ... @>.
    ("\\(@[=t^.:q]\\)\\(?:.\\|\n\\)*?\\(@>\\)"
     (1 'gweb-control-face) (2 'gweb-control-face))
    ;; Code starts and other word-like control codes.
    ("@[cpdfsixyz]\\>" . 'gweb-control-face)
    ;; Single-character symbolic control codes: @, @/ @| @# @& @! @+ @; @[ @] @%.
    ("@[]&!+;,/|#%[]" . 'gweb-control-face)
    ;; Inline Go inside the commentary: |...|.
    ("\\(|\\)\\([^|]*\\)\\(|\\)"
     (1 'font-lock-delimiter-face) (3 'font-lock-delimiter-face))
    ;; TeX control sequences in the commentary.
    ("\\\\[A-Za-z@]+\\|\\\\." . 'font-lock-keyword-face))
  "Font-lock rules for `gweb-mode'.")

(defvar gweb-mode-syntax-table
  (let ((st (make-syntax-table)))
    (modify-syntax-entry ?@ "." st)
    (modify-syntax-entry ?\\ "\\" st)
    st)
  "Syntax table for `gweb-mode'.")

;;;###autoload
(define-derived-mode gweb-mode prog-mode "GWEB"
  "Major mode for editing GWEB (.w) literate-programming files for Go."
  :syntax-table gweb-mode-syntax-table
  (setq-local font-lock-defaults '(gweb-font-lock-keywords))
  (setq-local comment-start "@q ")
  (setq-local comment-end "@>"))

;;;###autoload
(add-to-list 'auto-mode-alist '("\\.w\\'" . gweb-mode))
;;;###autoload
(add-to-list 'auto-mode-alist '("\\.ch\\'" . gweb-mode))

(provide 'gweb-mode)

;;; gweb-mode.el ends here
