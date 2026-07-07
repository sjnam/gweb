\def\title{Lazy Sequences in Go}

@s Seq int

@* Introduction.
This example builds a tiny library of {\it lazy sequences\/} and uses it to print
the first few even Fibonacci numbers. In barely fifty lines it exercises the Go
features that have no counterpart in C, and that a literate document most enjoys
showing off:

\smallskip
\item{$\bullet$} first-class functions and closures --- functions are
values that can capture and carry local state, be passed as arguments, and be
returned from other functions;
\item{$\bullet$} anonymous functions --- the |func(...) {...}| literals
that make the closures above;
\item{$\bullet$} generics --- type parameters like |[A, B any]|, so one
|Map| works for every element type;
\item{$\bullet$} range-over-func iterators (Go~1.23) --- a function can
{\it be\/} a sequence, driven directly by |for v := range seq|.
\smallskip

\noindent A sequence here is an |iter.Seq[V]|, which is just the function type
|func(yield func(V) bool)|: a {\it producer\/} that hands each value to a
|yield| callback and stops early if |yield| returns |false|. Because a producer
is an ordinary closure, sequences can be infinite and are computed only on
demand --- nothing is materialised into a slice.
@c
package main

import (
	"fmt"
	"iter"
)

@<The Fibonacci generator@>
@<The |Map| combinator@>
@<The |Filter| combinator@>
@<The |Take| combinator@>

func main() {
	@<Print the first even Fibonacci numbers@>
}

@* A sequence is a closure.
The Fibonacci numbers are an infinite sequence, so they could never be returned
as a slice; as a lazy |iter.Seq[int]| they are no trouble at all. |fibs| returns
a closure that keeps the running pair |a|,|b| as captured local state --- the
kind of stateful function value C simply cannot express --- and offers each |a|
to |yield| until the consumer asks it to stop. The loop condition {\it is\/} the
yield call: |for yield(a)| runs until |yield| returns |false|.
@<The Fibonacci generator@>=
func fibs() iter.Seq[int] {
	return func(yield func(int) bool) {
		a, b := 0, 1
		for yield(a) {
			a, b = b, a+b
		}
	}
}

@* Composing sequences.
The real payoff is composition. Each combinator takes a sequence and returns a
{\it new\/} sequence --- another closure --- that wraps the old one. None of them
does any work when called; the computation happens only when the final sequence
is ranged over. This is the same pipeline style as Unix filters, but type-safe
and built entirely from function values.

|Map| applies |f| to every element. Its type parameters |A| and |B| let it turn
a sequence of one type into a sequence of another; the returned closure ranges
over the input and yields the transformed values.
@<The |Map| combinator@>=
func Map[A, B any](s iter.Seq[A], f func(A) B) iter.Seq[B] {
	return func(yield func(B) bool) {
		for v := range s {
			if !yield(f(v)) {
				return
			}
		}
	}
}

@ |Filter| keeps only the elements satisfying a predicate. The |keep| function
is supplied by the caller as an anonymous function, so the same |Filter| serves
every purpose.
@<The |Filter| combinator@>=
func Filter[V any](s iter.Seq[V], keep func(V) bool) iter.Seq[V] {
	return func(yield func(V) bool) {
		for v := range s {
			if keep(v) && !yield(v) {
				return
			}
		}
	}
}

@ |Take| makes an infinite sequence finite by yielding at most |n| elements and
then returning, which breaks the producer's loop. Without it, ranging over
|fibs| would never end.
@<The |Take| combinator@>=
func Take[V any](s iter.Seq[V], n int) iter.Seq[V] {
	return func(yield func(V) bool) {
		i := 0
		for v := range s {
			if i >= n || !yield(v) {
				return
			}
			i++
		}
	}
}

@* Running the pipeline.
Now we assemble a pipeline and consume it. |Filter| narrows the infinite
Fibonacci stream to its even members using an anonymous predicate, |Take| caps
it at eight, and the |for n := range Take(even, 8)| loop pulls the values ---
each computed only as it is demanded. The output is the eight numbers
0, 2, 8, 34, 144, 610, 2584, 10946.
@<Print the first even Fibonacci numbers@>=
even := Filter(fibs(), func(n int) bool { return n%2 == 0 })
for n := range Take(even, 8) {
	fmt.Println(n)
}

@* Index.
