\def\title{Pair Sums}

@* Introduction.
Define the {\it value\/} of an array to be the sum, over every pair of positions
$i<j$, of the products of their entries:
$$\mathop{\rm value}(A)=\sum_{i<j}A_iA_j.$$
(Equal numbers in different positions count as different pairs.) Given one array,
we want the largest value attained by any of its nonempty {\it subarrays\/} --- its
contiguous stretches. A single element has no pairs and so has value~$0$, which is
the floor of the answer.

The array can be long: up to $n=5\cdot10^5$ entries, each in $[-10^3,10^3]$. There
are $\Theta(n^2)$ subarrays, so trying them one by one is hopeless at the top end.
The pleasure of the problem is that a little algebra turns it into a question about
{\it the upper envelope of a family of straight lines}, which a {\sc Li Chao tree}
answers in $O(n\log n)$ time.

@ The input is $n$ on one line and the $n$ entries on the next;
with $n$ up to half a million we read through a buffered word scanner. The answer is
a single integer.
@c
package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
)

@<The line and the Li Chao tree@>

func main() {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	sc.Split(bufio.ScanWords)
	readInt := func() int64 {
		sc.Scan()
		v, _ := strconv.ParseInt(sc.Text(), 10, 64)
		return v
	}

	n := int(readInt())
	a := make([]int64, n)
	for i := range a {
		a[i] = readInt()
	}

	@<Solve with Li Chao Tree@>
}

@* The Li Chao tree.
Squaring a sum splits into its diagonal and off-diagonal parts,
$$\Bigl(\sum_i A_i\Bigr)^2=\sum_i A_i^2+2\sum_{i<j}A_iA_j,$$
so with $S=\sum A_i$ the sum of a stretch and $Q=\sum A_i^2$ its sum of squares,
$$\mathop{\rm value}=\sum_{i<j}A_iA_j={S^2-Q\over2}.$$
The numerator $S^2-Q$ equals $2\sum_{i<j}A_iA_j$, an even integer, so the value is
always an exact integer. We will maximize $S^2-Q$ and halve the winner at the end.

@ Let $P_k=A_0+\cdots+A_{k-1}$ and $R_k=A_0^2+\cdots+A_{k-1}^2$ be the prefix sums of
the entries and of their squares ($P_0=R_0=0$). The subarray spanning positions
$i,\ldots,j-1$ --- prefix indices $i<j$ --- has $S=P_j-P_i$ and $Q=R_j-R_i$, hence
$$S^2-Q=(P_j-P_i)^2-(R_j-R_i).$$
Now fix the right end $j$ and look at it as a function of the left end~$i$:
$$(P_j-P_i)^2-(R_j-R_i)=\underbrace{(P_j^2-R_j)}_{\hbox{\sevenrm fixed by }j}
   +\underbrace{\bigl[(-2P_i)\,P_j+(P_i^2+R_i)\bigr]}_{\hbox{\sevenrm line }\ell_i
   \hbox{\sevenrm\ evaluated at }P_j}.$$
Each candidate left end~$i$ thus contributes a straight line
$$\ell_i(t)=(-2P_i)\,t+(P_i^2+R_i),$$
and the best left end for a given~$j$ is the one whose line is highest at $t=P_j$.
Sweeping $j$ from left to right, we add $\ell_i$ the moment position~$i$ becomes a
legal left end, and ask for the maximum of all lines so far at the point $P_j$.

@ A |line| is $y=a\,x+b$. Coefficients and results stay within |int64|: a slope is
at most $2\cdot5\cdot10^8$ in magnitude, a query point at most $5\cdot10^8$, so a
product is below $5\cdot10^{17}$, comfortably inside the range.
@<The line and the Li Chao tree@>=
type line struct{ a, b int64 }
@#
func (l line) at(x int64) int64 { return l.a*x + l.b }

@ A node owns the segment $[lo,hi]$ of $x$ values, the line currently winning at its
midpoint, and two children for the halves. A fresh node starts with a sentinel line
lying at $-\infty$, so the first real line always beats it.
@<The line and the Li Chao tree@>=
type liChao struct {
	lo, hi      int64
	ln          line
	left, right *liChao
}
@#
func newLiChao(lo, hi int64) *liChao {
	return &liChao{lo: lo, hi: hi, ln: line{0, math.MinInt64 / 4}}
}

@ To |add| a line, compare it with the node's incumbent at the two ends $lo$ and
$mid$. If the newcomer wins at the midpoint, keep it here and demote the old line.
Whichever line is now the loser can still be best on at most one side --- the side
where it also lost the endpoint test --- so it recurses into exactly that child. The
midpoint is computed with an arithmetic shift, which floors toward $-\infty$ and so
behaves correctly for the negative coordinates we have here.
@<The line and the Li Chao tree@>=
func (t *liChao) add(nw line) {
	m := (t.lo + t.hi) >> 1
	bl := nw.at(t.lo) > t.ln.at(t.lo)
	bm := nw.at(m) > t.ln.at(m)
	if bm {
		t.ln, nw = nw, t.ln
	}
	switch {
	case t.lo == t.hi:
		return
	case bl != bm:
		if t.left == nil {
			t.left = newLiChao(t.lo, m)
		}
		t.left.add(nw)
	default:
		if t.right == nil {
			t.right = newLiChao(m+1, t.hi)
		}
		t.right.add(nw)
	}
}

@ A query takes the incumbent's height at $x$ and improves it with the one child
whose segment contains~$x$ --- the only place a better line could be hiding.
@<The line and the Li Chao tree@>=
func (t *liChao) query(x int64) int64 {
	best := t.ln.at(x)
	m := (t.lo + t.hi) >> 1
	if x <= m {
		if t.left != nil {
			best = max(best, t.left.query(x))
		}
	} else if t.right != nil {
		best = max(best, t.right.query(x))
	}
	return best
}

@ Because the array has negative entries, neither the slopes $-2P_i$ nor the query
points $P_j$ arrive in any sorted order, so the ordinary monotone convex-hull trick
will not do. A {\sc Li Chao tree} handles the general case: it is a segment tree
over the range of possible $x$ values in which each node keeps the single line that
is highest at the node's midpoint, pushing the loser down into whichever half it
can still win. Inserting a line and querying the maximum at a point each visit one
node per level, so both cost $O(\log V)$ where $V$ is the width of the coordinate
range. With $n$ insertions and $n$ queries the whole search is $O(n\log V)$.

@ It builds the prefix arrays,
opens a Li Chao tree spanning the observed range of $P$, then sweeps the right
end~$j$: at each step it reads off the best line at $P_j$, folds in the fixed part
$P_j^2-R_j$, and then admits position~$j$ as a future left end. The running maximum
is $S^2-Q$; halving it gives the answer.
@<Solve...@>=
@<Build the prefix sums |p| and |r|@>@;
@<Open a tree over the range of |p|@>@;
best := int64(math.MinInt64)
tree.add(line{-2 * p[0], p[0]*p[0] + r[0]})
for j := 1; j <= n; j++ {
	g := p[j]*p[j] - r[j] + tree.query(p[j])
	best = max(best, g)
	if j < n {
		tree.add(line{-2 * p[j], p[j]*p[j] + r[j]})
	}
}
fmt.Println(best / 2)

@ @<Build the prefix sums |p| and |r|@>=
p := make([]int64, n+1)
r := make([]int64, n+1)
for k := 1; k <= n; k++ {
	p[k] = p[k-1] + a[k-1]
	r[k] = r[k-1] + a[k-1]*a[k-1]
}

@ The query points and the line tests all happen at coordinates that are values of
$P$, so a tree spanning $[\min P,\max P]$ is wide enough.
@<Open a tree over the range of |p|@>=
lo, hi := p[0], p[0]
for _, v := range p {
	lo = min(lo, v)
	hi = max(hi, v)
}
tree := newLiChao(lo, hi)

@* Index.
