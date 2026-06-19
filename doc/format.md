# GWEB source format

A GWEB source file (`.w`) is a sequence of **sections**, mirroring CWEB. Everything
before the first section is **limbo** (TeX preamble for `gweave`, ignored by
`gtangle`). Each section has up to three parts, in order:

1. **TeX part** — commentary/documentation (plain TeX, copied to the woven output).
2. **Definition part** — `@d` / `@f` / `@s` directives (formatting hints).
3. **Code part** — the Go code of the section, introduced by `@c` or `@<name@>=`.

All control codes begin with `@`. Use `@@` for a literal `@`.

## Structural control codes

| code            | meaning                                                          |
|-----------------|------------------------------------------------------------------|
| `@ ` `@\t` `@\n`| begin a normal (unstarred) section                               |
| `@*`            | begin a starred section (group/chapter), title runs to first `.` |
| `@*N`           | begin a starred section at depth `N` (`@*0` == `@*`)             |
| `@c`            | begin the code part of an *unnamed* section (the program text)   |
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

## Formatting / index control codes (used by `gweave`)

| code            | meaning                                                          |
|-----------------|------------------------------------------------------------------|
| `@f a b`        | format identifier `a` like identifier/keyword `b`                 |
| `@s a b`        | like `@f` but the entry is omitted from the index                 |
| `@^text@>`      | index entry, typeset in roman                                    |
| `@.text@>`      | index entry, typeset in typewriter                               |
| `@:text@>`      | index entry, typeset by the user macro `\9`                      |
| `@%`            | the rest of the line is a comment in the `.w` file (ignored)     |
| `@q text@>`     | quoted/ignored material (a comment for the web author)            |

## Name abbreviation

A section name may be abbreviated with a trailing `...`: `@<Set it up...@>`
matches the unique full name beginning with `Set it up`.

## Notes specific to Go

* Go has no preprocessor, so `@d` (macro definition) is accepted but currently
  emitted verbatim into the code stream; prefer Go `const`/`func`.
* The default tangled output file is `<basename>.go`; additional files are
  produced with `@(file@>=`.
* `gtangle` runs `gofmt` on its output, so the emitted Go is canonically
  formatted as long as the assembled program is valid Go.
