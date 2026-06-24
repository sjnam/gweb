\def\title{Topswops, Forwards}

@* Introduction.
This is a companion to \.{topswops.w}. Both programs compute the same quantity ---
the largest number of flips that Conway's {\it topswops\/} game can take on a deck
of $n$ cards,
$$f(n)=\hbox{the largest score among all $n!$ permutations of $\{1,\ldots,n\}$}$$
--- but by opposite strategies. The companion runs the game {\it backwards\/} from
its ending position; this one runs it {\it forwards\/}, and prunes hard. (For the
rules of topswops and the meaning of a {\it score}, see \.{topswops.w}.) It is a Go
transcription of Knuth's \.{CWEB} program \.{topswops-fwd.w}.

The forward idea is bolder than the backward one. Rather than fix a starting deal
and play it, we play with a deck whose cards are mostly {\it unknown}, deciding a
card's value only at the moment the game first turns it face up on top. Each such
decision is a branch in a search tree, and --- crucially --- a sharp upper bound
lets us abandon most branches early. This is what lets the forward search reach
much larger~$n$ than the backward one; the original runs at $n=16$.

@ {\bf A deck of placeholders.}\enspace
The deck |a| holds $n$ slots. A slot is either a {\it settled\/} card, a positive
value in $1\ldots n$, or an {\it unknown}, a negative {\it placeholder\/} $-i$
standing for ``the $i$th card we have not yet committed to.'' We start with
everything unknown:
$$a = (-1,\,-2,\,\ldots,\,-(n-1),\,0).$$
Now play forward. To flip the top $v$ cards we need to know $v$, the top card's
value; as long as the top is settled we keep flipping and counting steps. The
moment an {\it unknown\/} card reaches the top, the game cannot proceed until we
say what that card is --- so we stop and {\it branch}, trying each still-available
value in turn. Committing a value may settle several cards at once (the flip that
follows shuffles them into place), and the game runs on until the next unknown
surfaces, or until a~$1$ appears and the game ends.

A node of the search tree is thus a partially-decided deck together with the
number of steps the game has taken so far; its children are the legal values for
the unknown now on top. A leaf is a fully-decided deck --- a genuine starting
permutation --- whose step count is a candidate for the record.

@ {\bf The bound that makes it practical.}\enspace
Here is the key observation. At any node, the cards still unknown form, among
themselves, a {\it smaller topswops puzzle}: whatever else happens, the flips still
to come can number at most $f(m)$, where $m$ is how many cards remain unknown. So
if |c| steps have been taken and the best deal found so far has score~$r$, then
this node is worth pursuing only when
$$c + f(m)\ \ge\ r.$$
Otherwise no descendant can beat the record, and we prune the whole subtree.

The bound feeds on itself: the smaller answers $f(0),f(1),\ldots$ are exactly the
table |score[0]|, |score[1]|, $\ldots$ that we know in advance. We seed |score|
with the published values and keep |score[n]| as the running record~$r$, raising it
each time a better leaf turns up. Tighter records prune harder, so good deals found early pay
for themselves many times over.

@ {\bf Enumerating the choices.}\enspace
When an unknown surfaces we must try every value not yet placed, each exactly once.
The program does this with an inversion-table scheme of the kind Knuth uses for
{\it genlex\/} permutation generation: the array |p| keeps the values chosen so far
in $p[0\ldots l-1]$ and the still-available ones after them, the array |v| records,
for each level, which candidate was taken, and moving from one candidate to the
next costs a single swap. We never build a permutation from scratch; we edit the
previous one. The details are terse --- this is faithful to Knuth's original --- but
the effect is simple: at level |l| the loop sweeps an index |j| down through the
remaining candidates, the feasibility-and-bound test filters them, and |v| and |p|
let us undo a choice to reach the next.

@ {\bf The shape of the search.}\enspace
The recursion is written, as in the original, not with a recursive function but as
an explicit state machine with five labels: \.{advance} steps down to the next
candidate slot; \.{tryit} picks a candidate and tests feasibility and the bound;
\.{infeas} handles a rejected candidate, trying another slot or backing up;
\.{backup} pops one level; and \.{nextv} moves on to the next candidate, restoring
the saved deck. A snapshot |s[l]| of the deck and the running counts |d[l]|, |h[l]| are saved at
each level so that backtracking is a plain assignment --- and here Go helps,
because its arrays are values: saving and restoring a deck is just |s[l] = a| and
|a = s[l]|, where C needed an explicit copy. The array |profile| tallies how many
nodes are visited at each depth, a useful gauge of how well the bound is working.

@* The program.
The whole search lives in |main|. After setting up the placeholder deck it threads
through the five labelled states until it backs all the way out, then prints the
node profile.
@c
package main

import "fmt"

@<The known records, and the deck size@>@;
@<The global search state@>@;

func main() {
	@<Local scratch variables@>@;
	@<Set up the placeholder deck and the first level@>@;

advance:
	j--
tryit:
	@<Choose a candidate and reject it unless it can still beat the record@>@;
	@<Play the game forward as far as the settled cards allow@>@;
	@<At a leaf record a champion deal; otherwise descend one level@>@;
infeas:
	if j != 0 {
		goto advance
	}
backup:
	l--
nextv:
	@<Step to the next candidate, or back up; at the root, finish@>@;
}

@ The deck size~|n| is at most~$16$. The original uses $16$, whose search space is
so vast the program effectively never finishes; lower |n| to watch it complete and
to check the answers against the table below. |score| holds the known records
$f(0),\ldots,f(16)$ (sequence \.{A000375}); $score[n]$ doubles as the current
target, and $score[m]$ for $m<n$ is the pruning bound.
@<The known records, and the deck size@>=
const n = 16

var score = []int{0, 0, 1, 2, 4, 7, 10, 16, 22, 30, 38, 51, 65, 80, 101, 113, 114}

@ The state machine carries its working storage in package-level arrays, so that a
|goto| never jumps across a declaration. |a| is the live deck; |s[l]| snapshots it
at level~|l|; |d[l]| and |h[l]| are the step count and the boundary of the unknown
region saved there. |p| and |v| drive the candidate enumeration, and |b|, |bb|
serve only when reconstructing a winning deal.
@<The global search state@>=
var (
	p, v, h, b, bb [16]int
	s              [16][16]int
	a              [16]int
	d, profile     [16]int
)

@ @<Local scratch variables@>=
var j, k, l, t, c int

@ Initially every card is an unknown placeholder, the candidate pool |p| lists the
values $2,3,\ldots,n,1$, and we are about to choose the top card of level~$1$.
@<Set up the placeholder deck and the first level@>=
for k = 1; k < n; k++ {
	p[k-1] = k + 1
	a[k-1] = -k
}
p[n-1] = 1
v[0] = 1
h[1] = n
profile[0] = 1
l = 1
s[l] = a
j = n - 1

@ At |tryit| we take the next candidate value |k| from the pool. Two cheap tests
can reject it without any work. First, a card may not be assigned the value of the
placeholder now on top (that would make the move a no-op against itself). Second
comes the branch-and-bound test of section~3: the unknown region has size |t| (or,
in the boundary case |k == t|, size |k-t| after skipping the run of already-settled
cards), and unless the steps so far plus the best the remainder could yield, $c +
score[\cdot]$, can match the record, we give up on |k|. A surviving candidate is
recorded in |v| and swapped to the front of the chosen prefix of |p|.
@<Choose a candidate and reject it unless it can still beat the record@>=
k = p[n-2-j]
if k == -a[0] {
	goto infeas
}
t = h[l]
c = d[l] + 1
if k == t {
	for t = 1; a[t] == k-t; t++ {
	}
	if c+score[k-t] < score[n] {
		goto infeas
	}
} else if c+score[t] < score[n] {
	goto infeas
}
v[l] = j
p[n-2-j] = p[l-1]
p[l-1] = k

@ Having committed the surfaced card to |k|, we play the game forward. Each pass
flips the top |k| cards --- the old top moves to position $k-1$, the old position
$k-1$ becomes the new top, and the cards between are reversed --- and then reads the
new top. While that top is a settled (positive) value we keep going, counting a
step each time; the instant it is an unknown ($\le 0$) we stop, the next branch
point reached.
@<Play the game forward as far as the settled cards allow@>=
for {
	a[0] = a[k-1]
	a[k-1] = k
	for j, k = 1, k-2; j < k; j, k = j+1, k-1 {
		t = a[j]
		a[j] = a[k]
		a[k] = t
	}
	k = a[0]
	if k <= 0 {
		break
	}
	c++
}

@ One node is now done. If we are at the deepest level, every card is settled and
the deck is a real starting permutation; should its score |c| match or beat the
record we record it (raising the bar for everyone after). Otherwise we descend:
shrink the boundary |t| past the cards already in their final place, save the deck
and counts for this new level, and loop back to |advance|.
@<At a leaf record a champion deal; otherwise descend one level@>=
profile[l]++
if l == n-1 {
	if c >= score[n] {
		@<Reconstruct and print the winning deal@>@;
	}
	goto nextv
}
for t = h[l]; a[t-1] == t; t-- {
}
l++
s[l] = a
d[l] = c
h[l] = t
j = n - l
goto advance

@ A leaf gives us the {\it order\/} in which values were chosen, |p|, not the
opening deal itself. To recover the deal we replay the game's flips in reverse onto
a fresh placeholder deck |b|: each chosen value, fed back through the inverse flip,
lands in the position it must have occupied at the start, filling in |bb|. We print
the score, the reconstructed opening permutation, and (after \.{->\ 1}) the settled
tail of the final deck as a check.
@<Reconstruct and print the winning deal@>=
score[n] = c
fmt.Printf("%d:", c)
for k = 1; k <= n; k++ {
	b[k-1] = -k
}
for k = 1; k <= n; k++ {
	for b[0] > 0 {
		j = b[0]
		b[0] = b[j-1]
		b[j-1] = j
		c = 1
		j -= 2
		for ; c < j; c, j = c+1, j-1 {
			t = b[c]
			b[c] = b[j]
			b[j] = t
		}
	}
	bb[-b[0]-1] = p[k-1]
	b[0] = p[k-1]
}
for k = 0; k < n; k++ {
	fmt.Printf(" %d", bb[k])
}
fmt.Printf(" -> 1")
for k = 1; k < n; k++ {
	fmt.Printf(" %d", a[k])
}
fmt.Printf("\n")

@ The |infeas| and |backup| labels in |main| handle the two ways forward progress
can stall: a rejected candidate (try the next slot, or back up if none remain) and
the end of a level (pop). The |nextv| state ties them together. If this level's
candidates are exhausted ($v[l]=0$) we undo its swap and back up further; otherwise
we restore the saved deck and the previous swap and jump to |tryit| with the next
candidate. When the backing-up reaches the root, the search is over and we print the
node counts.
@<Step to the next candidate, or back up; at the root, finish@>=
if v[l] == 0 {
	t = p[l-1]
	p[l-1] = p[n-2]
	p[n-2] = t
	goto backup
}
if l != 0 {
	j = v[l] - 1
	t = p[l-1]
	p[l-1] = p[n-3-j]
	p[n-3-j] = t
	a = s[l]
	goto tryit
}
for k = range n {
	fmt.Printf("%9d nodes at level %d.\n", profile[k], k)
}

@* Index.
