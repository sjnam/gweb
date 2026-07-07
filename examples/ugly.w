\def\x#1{x_{#1}}
@f x1 TeX
@f x2 TeX

@* Introduction.
This program implements a {\sl 1-indexed Fenwick tree.\/} However, to test
\.{gweave}, I intentionally made the code --- including the indentation and the
code on each line --- a mess, even though it compiles and runs. Of course, there
is almost no explanation in the \TEX/ part.
@c
package main

import "fmt"

@<Type Definition@>
@<Subroutines@>

func main() {
    var x1, x2 int
    x1, x2 = x2, x1
fw:=NewFenwick(10)

fw.Add(1,5)
fw.Add(3,              2)
fw.Add(5,7)
fw.Add(8,                    4)

fmt.Println(fw.Sum                (5))
fmt.Println              (fw.RangeSum(3,8))
}

@ @<Type...@>=
type Fenwick struct{
tree[]int
}

@ @<Sub...@>=
func NewFenwick(n int)*Fenwick{
return &Fenwick{
tree:make([]int,n+1),
}
}

@ @<Sub...@>=
func (f *Fenwick) Add(i, delta int) {
for i<len                   (f.tree)            {
f.tree[i] +=delta
i += i &           -i
}
}

func (f *Fenwick) Sum(i int) int {
sum := 0
for i            >0 {
sum+= f.tree[i]
i-=i&-i
}
return sum
}

@ @<Sub...@>=
func (f *Fenwick) RangeSum(l, r int) int {
if l > r {
return 0
}
return f.Sum(r)-f.Sum(l-1)
}

@* Index.
