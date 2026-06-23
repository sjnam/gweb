\def\sq#1{{1\over#1^2}}

@* Introduction. Project Euler Problem 152 asks:
\smallskip
{\narrower\narrower\narrower\noindent
There are several ways to write the number $1\over2$ as a sum of square reciprocals
using distinct integers. For instance, numbers $\{2,3,4,5,7,12,15,20,28,35\}$ can
be used:
$$
{1\over2}=\sq2+\sq3+\sq4+\sq5+\sq7+\sq{12}+\sq{15}+\sq{20}+\sq{28}+\sq{35}
$$
In fact, only using integers between 2 and 45 inclusive, there are exactly three
ways to do it, the remaining two being:
${2,3,4,6,7,9,10,20,28,35,36,45}$ and ${2,3,4,6,7,9,12,15,28,30,35,36,45}$.

\noindent How many ways are there to write $1\over2$ as a sum of
 reciprocals of squares using distinct integers between 2 and 80 inclusive?
\smallskip
}

\noindent The official answer is 301. A naïve search would examine every subset of
$\{2,\ldots,80\}$, requiring $2^{79}$ possibilities, which is completely
infeasible.
The purpose of this program is to exploit several number-theoretic observations
that drastically reduce the search space and then apply a meet-in-the-middle
enumeration.

@c
package main

import (
	"flag"
	"fmt"
	"math/bits"
	"slices"
	"strings"
)

@<Type definition@>
@<Subroutines@>

func main() {
    all := flag.Bool("all", false, "Print all 301 years, each as a denominator set")
	flag.Parse()

    @<Construct the reduced search space@>
    @<Clear denominators@>
    @<Meet in the middle@>

	if count != 301 {
		panic(fmt.Sprintf("regression: got %d, want 301", count))
	}

	if !*all {
		return
	}

    @<Display solutions@>
}

@* Mathematical pruning. The key observation is that most denominators
can never participate in a valid solution. Suppose a prime $p > 7$ appears
in a denominator. After clearing denominators, the contribution of terms
containing the highest power of $p$ must cancel modulo that power.

For almost all primes larger than seven this is impossible. The modular constraints
eliminate nearly every candidate. After applying these arguments, only denominators
whose prime factors belong to $\{2,3,5,7\}$ survive.

Such numbers are called {\sl 7-smooth numbers\/}. Among the integers
$2\le n\le 80$, there are exactly 39 such values. One additional special block survives:
$$
\sq{13}+\sq{39}+\sq{52}.
$$
The three denominators must always occur together. Therefore they can be represented
as a single search item. Consequently the entire problem reduces to selecting among
only forty objects:

\item{$\bullet$} 39 individual 7-smooth denominators
\item{$\bullet$} one composite block $\{13,39,52\}$

This reduction is the essential number-theoretic step.

@ Let's represent the candidates. Each search item represents either a single denominator
or a block of denominators.
@<Type...@>=
type item struct {
    denoms []int64
}

@ To transform rational arithmetic into integer arithmetic, we compute a common denominator.
@<Sub...@>=
func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
@#
func lcm(a, b int64) int64 {
	return a / gcd(a, b) * b
}

@ A positive integer is 7-smooth if all of its prime factors belong to $\{2,3,5,7\}$.
@<Sub...@>=
func isSevenSmooth(a int64) bool {
    if a <= 0 {
        return false
    }
    for p := range slices.Values([]int64{2,3,5,7}) {
        for a%p == 0 {
            a /= p
        }
    }
    return a == 1
}

@ The first 39 items are the surviving 7-smooth denominators. The final item represents
the mandatory block $\{13,39,52\}.$

@<Construct...@>=
var items []item
for a := int64(2); a <= 80; a++ {
	if isSevenSmooth(a) {
		items = append(items, item{denoms: []int64{a}})
	}
}
items = append(items, item{denoms: []int64{13, 39, 52}})

@* Integer subset-sum formulation. Let $D = {\rm lcm}(n^2)$ taken over every denominator
occurring in every item. Then every reciprocal square can be represented as an
integer
$$\sq n = {D/n^2\over D}.$$
The original equation becomes an integer subset-sum problem.
@<Clear denominators@>=
D := int64(1)
for it := range slices.Values(items) {
    for a := range slices.Values(it.denoms) {
        D = lcm(D, a*a)
    }
}
target := D / 2

vals := make([]int64, len(items))
for i, it := range items {
    for a := range slices.Values(it.denoms) {
		vals[i] += D / (a * a)
	}
}

@* Meet in the middle. After pruning we still have 40 search objects. A direct search
requires $2^{40}\approx 10^{12}$ subset evaluations. Instead we split the items into
two groups of twenty. For every left subset we compute its sum. For every right subset
we look for $\hbox{target} - \hbox{sum}$ among the left sums. This reduces the complexity
to approximately $2^{20}+2^{20}$ which is entirely practical.

@ Enumerating all subsets usually requires recomputing every subset sum from scratch.
Gray codes avoid this. Consecutive Gray-code subsets differ by exactly one bit,
so the running sum can be updated by a single addition or subtraction.
This reduces subset generation from $O(n2^n)$ to $O(2^n).$

@<Sub...@>=
func forEachGray(vals []int64, visit func(mask uint32, sum int64)) {
	var s int64
	visit(0, 0) // empty set
	for k := 1; k < (1 << uint(len(vals))); k++ {
		b := bits.TrailingZeros(uint(k))
		if (k^(k>>1))&(1<<uint(b)) != 0 {
			s += vals[b]
		} else {
			s -= vals[b]
		}
		visit(uint32(k^(k>>1)), s)
	}
}

@ @<Meet in the middle@>=
mid := len(vals) / 2
left, right := vals[:mid], vals[mid:]

lm := make(map[int64]int64, 1<<uint(mid))
forEachGray(left, func(_ uint32, s int64) { lm[s]++ })

var count int64
forEachGray(right, func(_ uint32, s int64) { count += lm[target-s] })

fmt.Println(count)

@* Recovering solutions. The counting phase only determines the number of solutions.
When the \.{-all} flag is supplied we reconstruct the actual denominator sets producing
each solution.
@<Sub...@>=
func denomsOf(items []item, base int, mask uint32) []int64 {
	var ds []int64
	for i := 0; mask != 0; i, mask = i+1, mask>>1 {
		if mask&1 != 0 {
			ds = append(ds, items[base+i].denoms...)
		}
	}
	return ds
}

@ @<Sub...@>=
func findSolutions(items []item, vals []int64, target int64) [][]int64 {
	mid := len(vals) / 2
	left := make(map[int64][]uint32)
	forEachGray(vals[:mid], func(mask uint32, s int64) {
		left[s] = append(left[s], mask)
	})

	var sols [][]int64
	forEachGray(vals[mid:], func(rmask uint32, s int64) {
		for _, lmask := range left[target-s] {
			ds := append(denomsOf(items, 0, lmask), denomsOf(items, mid, rmask)...)
			slices.Sort(ds)
			sols = append(sols, ds)
		}
	})

	slices.SortFunc(sols, slices.Compare) // 사전식 정렬로 출력 결정적
	return sols
}

@ @<Display solutions@>=
sols := findSolutions(items, vals, target)
for i, ds := range sols {
	terms := make([]string, len(ds))
	for j, d := range ds {
		terms[j] = fmt.Sprintf("1/%d²", d)
	}
	fmt.Printf("%3d: 1/2 = %s\n", i+1, strings.Join(terms, " + "))
}

@* Index.
