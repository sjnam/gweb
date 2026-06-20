\input gwebmac

@* Introduction.
This is the GWEB version of the classic \.{wc} word-counting program, modeled on
the example that accompanies CWEB. It reads each file named on the command line
(or standard input when no files are given) and reports the number of lines,
words, and characters it contains, followed by a grand total when more than one
file is processed.

The program illustrates the main ingredients of a literate program: starred and
ordinary sections, prose interleaved with code, and named sections (here called
{\it refinements\/}) that let us present the code in the order that best explains
it rather than the order the compiler needs.

@ The whole program is a single Go file in package |main|. Its skeleton names
three refinements that are filled in below; |gtangle| assembles them into the
order Go expects, while |gweave| typesets them in the order shown here.
@c
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

@<Type definitions@>
@<Subroutines@>

func main() {
	@<Count each file and print a grand total@>
}

@* Counting.
For every input we accumulate three integers, kept together in one struct. The
format directive on the next line asks |gweave| to typeset the type |counts| in
bold, the way it sets predeclared types such as |int|.
@f counts int
@<Type definitions@>=
type counts struct {
	lines, words, chars int
}

@ It is handy to fold one file's tally into a running total.
@<Subroutines@>=
func (c *counts) add(d counts) {
	c.lines += d.lines
	c.words += d.words
	c.chars += d.chars
}

@ The driver decides between filter mode and file mode, then prints a total.
@<Count each file and print a grand total@>=
files := os.Args[1:]
var total counts
@<Count the standard input if no files were given@>
for _, name := range files {
	@<Count one named file@>
}
if len(files) > 1 {
	report(total, "total")
}

@ With no arguments we behave like a Unix filter, reading standard input.
@<Count the standard input if no files were given@>=
if len(files) == 0 {
	report(countReader(os.Stdin), "")
	return
}

@ Otherwise each named file is opened, counted, closed, and added to the total.
A file that cannot be opened produces a diagnostic but does not stop the run.
@<Count one named file@>=
f, err := os.Open(name)
if err != nil {
	fmt.Fprintln(os.Stderr, "wc:", err)
	continue
}
c := countReader(f)
f.Close()
report(c, name)
total.add(c)

@* The scanner.
The heart of the program reads one |io.Reader| byte by byte. A {\it word\/} is a
maximal run of characters that are not white space, so we remember whether we are
currently inside a word.
@<Subroutines@>=
func countReader(r io.Reader) counts {
	var c counts
	in := bufio.NewReader(r)
	inWord := false
	for {
		b, err := in.ReadByte()
		if err != nil {
			break
		}
		c.chars++
		@<Update the counts for byte |b|@>
	}
	return c
}

@ Every byte is a character; a newline additionally ends a line; and any white
space ends the current word, while the first non-space after a gap starts a new
one.
@<Update the counts for byte |b|@>=
if b == '\n' {
	c.lines++
}
switch b {
case ' ', '\t', '\n', '\r', '\f', '\v':
	inWord = false
default:
	if !inWord {
		inWord = true
		c.words++
	}
}

@* Output.
Finally, one helper prints a single row of the report. The grand-total row and
the standard-input row are distinguished only by their trailing label.
@<Subroutines@>=
func report(c counts, name string) {
	if name == "" {
		fmt.Printf("%8d %8d %8d\n", c.lines, c.words, c.chars)
	} else {
		fmt.Printf("%8d %8d %8d %s\n", c.lines, c.words, c.chars, name)
	}
}

@* Index.
Names and refinements are cross-referenced automatically below.
