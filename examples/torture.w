\noatl
\nocon
\nosecs

@s MaxInt true
\def\x#1{x_{#1}} @f x1 TeX @f x2 TeX
\let\tagkern=\quad

\ifx\pdfextension\undefined % pdftex: the primitive is \pdfliteral
  \def\redon{\pdfliteral{1 0 0 rg}}\def\redoff{\pdfliteral{0 g}}
\else                       % luatex: it is \pdfextension literal
  \def\redon{\pdfextension literal{1 0 0 rg}}\def\redoff{\pdfextension literal{0 g}}
\fi

@* Introduction. @q torture @>
This program implements a {\sl 1-indexed Fenwick tree.\/} However, to test
\.{gweave}, I intentionally made the code---including the indentation and the
code on each line---a mess, even though it compiles and runs. Of course, there
is almost no explanation in the \TEX/ part. @^ Fenwick@> @.tree@>
\pdfURL{codeforces 1093E}{https://codeforces.com/problemset/problem/1093/E} 
@c
package main

import "fmt"

@<Type Definition@>
@<Subroutines@>

func main(){
	var x1, x2 int
    x1, x2 = x2, x1
fw:=NewFenwick(10)

const MaxInt = 23
fw.Add(1,5)
fw.Add(2,@,5)
fw.Add(3,@/2) 
fw.Add(5,7)
fw.Add(8,                    4)

fmt.Println(fw.Sum                (5))
fmt.Println              (fw.RangeSum(3,8))
}

@ @<Type...@>=
type Fenwick struct{
tree[]int
}

type edge [2][2]int

var knuthCorners = [4][]edge{
	@<0 corner@>
}

type Action struct {
	Actor    int `json:"player"` // who plays this move
	FieldNo  int `json:"field"`  // the field to sow or to reap
	FaceUpNo int `json:"card"`   // which face-up card, in |TakeRevealed|
	HandNo   int `json:"hand"`   // which card of the hand, in |TossHand|
	To       int `json:"to"`     // the other party to a trade
}

@ @<0 corner@>=
{
	{{0, 0}, {1, 2}}, {{0, 0}, {2, 1}}, {{0, 1}, {1, 3}}, {{0, 1}, {2, 2}},
	@t\redon@>{{0, 2}, {1, 0}},@t\redoff@> {{0, 2}, {1, 4}}, {{0, 3}, {2, 2}}, {{0, 3}, {2, 4}},
	{{0, 4}, {1, 6}}, {{0, 4}, {2, 5}}, {{0, 5}, {2, 4}}, {{0, 6}, {1, 4}},
},

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

@ \.{r1 r2 r3} \.{r1\ r2\ r3}
@<Sub...@>=
func (f *Fenwick) RangeSum(l, r int) int {
if l > r {return 0}
return f.Sum(r)-f.Sum(l-1)
}

@* Index.
