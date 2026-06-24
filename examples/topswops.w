\def\title{Topswops}

@* Introduction.
{\it Topswops\/} is a game of patience invented by John~H. Conway. Shuffle a
packet of cards numbered $1$ through $n$ into a pile, face up, and then repeat
one move for as long as you can: read the number $k$ on the top card, and if
$k>1$, turn the top $k$ cards over as a single block. The game ends the instant
a~$1$ comes to the top.

With $n=3$, for instance, the deal $3\,1\,2$ plays out as
$$3\,1\,2\;\longrightarrow\;2\,1\,3\;\longrightarrow\;1\,2\,3,$$
ending after two flips. (First the top card~$3$ reverses all three cards; then
the top card~$2$ reverses the top two, and the $1$ surfaces.)

A short argument of Conway's shows that the game always halts, so the number of
flips it takes --- the {\it score\/} of the starting permutation --- is a
well-defined number. The natural question is how large that score can get:
$$f(n)=\hbox{the largest score among all $n!$ permutations of $\{1,\ldots,n\}$.}$$
These numbers grow briskly. The first of them, $f(1),f(2),\ldots$, are
$$0,\;1,\;2,\;4,\;7,\;10,\;16,\;22,\;30,\;38,\;51,\;65,\;80,\;101,\;113,\;114$$
(this is sequence \.{A000375} in the {\sl On-Line Encyclopedia of Integer
Sequences\/}), and they are known only for small~$n$.

@ How can we find $f(n)$, and a permutation that achieves it, without playing all
$n!$ games? This program uses the elegant idea of A.~Pepperdine ({\sl
Mathematical Gazette\/} {\bf 73} (1989), 131--133): {\it run the game
backwards.}

Instead of starting from a deal and playing forward, start from the unique
{\it ending\/} position --- a~$1$ on top --- and undo moves. Undoing one move at
a time, in every possible way, grows a tree of permutations whose depth is
exactly the score: a permutation reached at depth~$l$ is one from which the
forward game takes exactly $l$ flips to finish. The deepest leaf we can reach
therefore has score $f(n)$, and the search is a plain depth-first walk of that
tree. It does no pruning, so it is simple to write but slows down sharply as $n$
grows; here we take $n=15$.

This is a Go transcription of Knuth's \.{CWEB} program \.{topswops.w}, which used
a |goto|-driven backtrack. Go's arrays are values, not pointers, so the
per-level snapshots that the original kept by hand are made for us, for free, by
ordinary argument passing.

@ {\bf Undoing a flip.}\enspace
To walk backwards we must invert a forward move. Number the positions
$0,1,\ldots,n-1$ from the top. A forward move on a permutation~$p$ whose top card
is $v=p[0]$ reverses positions $0$ through $v-1$; afterwards the card that was on
top, namely~$v$, sits at position $v-1$.

Run that in reverse. Suppose the last forward move flipped the top $k+1$ cards
(so $v=k+1$). Then the permutation~$prev$ {\it before\/} the move is obtained from
the current permutation~$cur$ by reversing positions $0$ through~$k$ again, and
its top card must be $k+1$. Because reversing sends the old top to position~$k$,
this move could have produced~$cur$ only if
$$cur[k]=k+1.$$
That single equation is the whole feasibility test for ``the previous move
flipped $k+1$ cards.''

@ {\bf Partial permutations.}\enspace
A subtle economy makes the search far smaller. Many positions of a deal are never
looked at before the game ends; their values cannot affect the score, so there is
no reason to commit to them. We therefore work with {\it partial\/} permutations,
writing a~$0$ for ``not yet decided,'' and we fill in a position only when undoing
a move forces a specific card to have been there.

The only bookkeeping this needs is to avoid using one card value twice. A second
array, |used|, records which values have already been placed as the top card of
some earlier (deeper) state; position~$0$'s eternal~$1$ is implicit. When undoing
a flip of $k+1$ cards we must put card $k+1$ on top, so:

\item{$\bullet$} if position~$k$ of~$cur$ is still undecided, we are free to say
that card $k+1$ sat there --- {\it provided\/} $k+1$ has not been used elsewhere;
\item{$\bullet$} if position~$k$ already holds a value, the move is reversible
only when that value is exactly $k+1$.

\noindent A position left $0$ at the end is a genuinely free slot: any of the
unused card values may go there.

@* The program.
The program is a single file that needs only |fmt| for its output. It defines the
data, the recursive search, and a one-line reporter.
@c
package main

import "fmt"

@<Data and global state@>@;
@<The driver |main|@>@;
@<The backward search@>@;
@<Report a record permutation@>@;

@ The deck size~$n$ must be at most~$15$, so that a |perm| --- an array of $16$
bytes, one spare --- can hold both a permutation and, elsewhere, a $0/1$ ``used''
map. A value of~$0$ in a permutation means ``undecided.'' The global |best|
remembers the greatest depth reached so far, so that we print a permutation only
when its score sets a new record.
@<Data and global state@>=
const n = 15

type perm [16]byte

var best = -1

@ The search begins at the ending position: card~$1$ on top, every other position
undecided, and nothing yet marked used. That permutation has score~$0$.
@<The driver |main|@>=
func main() {
	var start perm
	start[0] = 1
	search(0, start, perm{})
}

@ |search| is the heart of the matter. Its argument |cur| is a permutation of
score~$l$; |used| flags the card values already committed. For every flip length
$k+1$ that {\it could\/} have been the last forward move, it reconstructs the
previous permutation and recurses one level deeper.
@<The backward search@>=
func search(l int, cur, used perm) {
	for k := 1; k < n; k++ {
		@<Skip |k| unless flipping $k+1$ cards could have produced |cur|@>@;
		@<Build the previous permutation |prev|@>@;
		@<Note a record, then recurse@>@;
	}
}

@ Here is the feasibility test derived above. If position~$k$ is undecided we may
commit it to $k+1$, unless that value is already spoken for; if it is decided, it
must already equal $k+1$.
@<Skip |k| unless flipping $k+1$ cards could have produced |cur|@>=
switch {
case cur[k] == 0:
	if used[k+1] != 0 {
		continue
	}
case cur[k] != byte(k+1):
	continue
}

@ Reversing positions $0$ through~$k$ of~$cur$, and setting the recovered top card
to $k+1$, yields the earlier permutation. Card $k+1$ is now used.
@<Build the previous permutation |prev|@>=
prev := cur
for j := 1; j <= k; j++ {
	prev[j] = cur[k-j]
}
prev[0] = byte(k + 1)

nextUsed := used
nextUsed[k+1] = 1

@ |prev| has score $l+1$. Whenever we first reach a new record depth we print the
permutation that achieves it; then we keep digging.
@<Note a record, then recurse@>=
if l >= best {
	best = l
	report(l+1, prev)
}
search(l+1, prev, nextUsed)

@ Finally, the reporter. It prints the score and the $n$ cards of a
record-setting permutation; a~$0$ marks a free position, which the reader may fill
with any unused value.
@<Report a record permutation@>=
func report(step int, p perm) {
	fmt.Printf("%d:", step)
	for j := range n {
		fmt.Printf(" %d", p[j])
	}
	fmt.Println()
}

@* Index.
