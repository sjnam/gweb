# GWEB source format

A GWEB source file (`.w`) is a sequence of **sections**, mirroring CWEB. Everything
before the first section is **limbo** (TeX preamble for `gweave`, ignored by
`gtangle`). Each section has up to three parts, in order:

1. **TeX part** — commentary/documentation (plain TeX, copied to the woven output).
2. **Definition part** — `@d` / `@f` / `@s` directives (formatting hints).
3. **Code part** — the Go code of the section, introduced by `@c` (or `@p`) or
   `@<name@>=`.

All control codes begin with `@`. Use `@@` for a literal `@`.

## Structural control codes

| code            | meaning                                                          |
|-----------------|------------------------------------------------------------------|
| `@ ` `@\t` `@\n`| begin a normal (unstarred) section                               |
| `@*`            | begin a starred section (group/chapter), title runs to first `.` |
| `@*N`           | begin a starred section at depth `N` (`@*0` == `@*`)             |
| `@c`, `@p`      | begin the code part of an *unnamed* section (the program text)   |
| `@<name@>=`     | begin the code part of a *named* section (a "refinement")        |
| `@(file@>=`     | begin code that `gtangle` writes to the file `file`              |
| `@i file`       | include another `.w` file at this point                          |

## In-code control codes

| code            | meaning                                                          |
|-----------------|------------------------------------------------------------------|
| `@<name@>`      | reference to a named section (expanded by tangle, linked by weave)|
| `@=text@>`      | verbatim text passed literally to the tangled Go output           |
| `@t text@>`     | TeX text inserted into the woven code (ignored by tangle)         |
| `@&`            | paste: join the surrounding tokens with no space                  |
| `@@`            | a literal `@`                                                     |

## Woven-layout control codes (used by `gweave`, ignored by `gtangle`)

These fine-tune the woven output; the source's own spacing and line breaks
otherwise determine the layout.

| code  | meaning                                                          |
|-------|-----------------------------------------------------------------|
| `@,`  | insert a thin space                                             |
| `@/`  | force a line break (continuation at the same indentation)        |
| `@\|` | an optional line break (a place a long line may wrap)            |
| `@#`  | a line break followed by a blank line                           |

GWEB mirrors the source rather than reflowing it, so CWEB's prettyprinter hints
`@+` (cancel break), `@[` … `@]` (treat as an expression), and `@;` (invisible
semicolon) have no effect; they are accepted and ignored for portability.

## Formatting / index control codes (used by `gweave`)

| code            | meaning                                                          |
|-----------------|------------------------------------------------------------------|
| `@f a b`        | format identifier `a` like identifier/keyword `b`                 |
| `@s a b`        | like `@f` but the entry is omitted from the index                 |
| `@!`            | index the next identifier as a definition (underline it)          |
| `@^text@>`      | index entry, typeset in roman                                    |
| `@.text@>`      | index entry, typeset in typewriter                               |
| `@:text@>`      | index entry, typeset by the user macro `\9`                      |
| `@%`            | the rest of the line is a comment in the `.w` file (ignored)     |
| `@q text@>`     | quoted/ignored material (a comment for the web author)            |

## Name abbreviation

A section name may be abbreviated with a trailing `...`: `@<Set it up...@>`
matches the unique full name beginning with `Set it up`. The full (unabbreviated)
name need appear only once, at the definition **or** at any reference; all the
`...` forms then resolve to it.

A section name may also contain Go code between vertical bars, which `gweave`
typesets as code (e.g. `@<Update the counts for byte |b|@>`). The bars are part
of the name, so the definition and all references must spell it identically.

## Change files

Both tools accept an optional second argument, a **change file** (`.ch`), which
patches the master source without editing it (CWEB's mechanism):

```sh
gtangle foo.w foo.ch
gweave  foo.w foo.ch
```

A change file is a sequence of changes, each of the form

```
@x
<lines to find in the master source>
@y
<lines to substitute>
@z
```

Text outside an `@x`…`@z` group is ignored (commentary). The `@x`/`@y`/`@z`
controls must start in the first column. Changes are matched against the master
(after `@i` includes are expanded) in order: at the first line equal to a
change's first match line the whole block must match, and is then replaced.
Matching ignores trailing whitespace. It is an error if a change never matches,
or matches its first line but not the rest.

## Notes specific to Go

* Go has no preprocessor, so `@d` (macro definition) has no analogue: it is
  accepted for compatibility but its body is ignored by both tools. Use Go
  `const`/`func` instead.
* `@f a b` makes `gweave` typeset identifier `a` in the class of `b` — most
  usefully `@f MyType int` to set a user type in bold like a predeclared type.
  `@s` does the same but also omits `a` from the index. These directives apply
  globally and may appear in limbo or in a section's definition part.
* The default tangled output file is `<basename>.go`; additional files are
  produced with `@(file@>=`.
* `gtangle` runs `gofmt` on its output, so the emitted Go is canonically
  formatted as long as the assembled program is valid Go.
