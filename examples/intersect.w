\input kotexgweb.tex
\def\title{순열의 교집합}

@* 들어가며.
코드포스 1093E 문제다. 길이 |n|의 두 순열 |a|와 |b|가 있고, |q|개의 질의를
처리한다.

\item{$\bullet$} {\bf 1 $l_a$ $r_a$ $l_b$ $r_b$}: 부분구간 $a[l_a\,..\,r_a]$와
$b[l_b\,..\,r_b]$에 {\it 동시에\/} 나타나는 값이 몇 개인지 센다.
\item{$\bullet$} {\bf 2 $x$ $y$}: $b$의 $x$번째와 $y$번째 원소를 맞바꾼다.

\noindent $n,q\le2\cdot10^5$이라 질의마다 두 구간을 훑으면 너무 느리다.

핵심은 각 값을 {\it 평면 위의 점\/}으로 바꾸는 것이다. 값 $v$가 $a$에서는
${\rm posA}(v)$번째, $b$에서는 ${\rm posB}(v)$번째에 있다고 하면, $v$에 점
$({\rm posA}(v),\,{\rm posB}(v))$를 대응시킨다. 그러면 ``$v$가
$a[l_a\,..\,r_a]$에 있다''는 것은 ${\rm posA}(v)\in[l_a,r_a]$와 같고,
``$b[l_b\,..\,r_b]$에 있다''는 것은 ${\rm posB}(v)\in[l_b,r_b]$와 같다. 따라서
질의 1은 직사각형 $[l_a,r_a]\times[l_b,r_b]$ 안의 점 개수를 세는 일이 된다.

질의 2는 어떤가. $b$에서 두 자리를 맞바꾸면 거기 있던 두 값의 ${\rm posB}$만
바뀌므로, ${\rm posA}$는 그대로인 채 두 점의 $y$좌표만 옮겨진다. 곧 {\it 점 두
개의 갱신\/}이다. 그러므로 문제는 ``점 갱신이 있는 2차원 영역 세기''로 귀착된다.

@ 프로그램의 뼈대다. 입력을 읽고, 값을 점으로 바꾸고, 모든 질의를 미리 한 번 훑어
좌표를 모은 뒤 자료구조를 세우고, 처음 점들을 넣고 나서, 질의를 차례로 처리한다.
@c
package main

import (
	"bufio"
	"os"
	"sort"
	"strconv"
)

@<유일화 도우미@>
@<이차원 BIT@>

func main() {
	@<입력을 읽는다@>
	@<값을 점으로 바꾼다@>
	@<모든 질의를 미리 훑어 좌표를 모은다@>
	@<처음 점들을 넣는다@>
	@<질의를 차례로 처리한다@>
}

@* 이차원 영역 세기.
$y$좌표가 갱신되는 2차원 영역 세기는 {\it 펜윅 트리 속의 펜윅 트리\/}(BIT of BITs)로
한다. 바깥 펜윅은 $x$좌표(곧 $a$에서의 자리)를 다스리고, 그 각 마디는 자기가
맡은 열들의 $y$좌표를 다스리는 안쪽 펜윅을 품는다. 갱신과 질의 모두
$O(\log^2 n)$이다.

$y$가 $1\ldots n$ 어디로든 바뀔 수 있으니 안쪽을 가득 채우면 $O(n^2)$ 메모리라 무리다.
그래서 {\it 오프라인\/}으로 간다. 질의를 미리 다 읽어, 각 점이 평생 가질 수 있는
$y$값들만 모아 안쪽 펜윅을 그만큼만 만든다. 한 점이 바깥 마디 $O(\log n)$개에
등록되므로 전체 메모리는 $O((n+q)\log n)$이다.
@<이차원 BIT@>=
type bit2d struct {
	n   int
	ys  [][]int // ys[i]: 바깥 마디 i를 거쳐 가는 모든 y값 (정렬·유일)
	bit [][]int // 안쪽 펜윅들
}

func newBIT2D(n int) *bit2d {
	return &bit2d{n: n, ys: make([][]int, n+1), bit: make([][]int, n+1)}
}

@ 만들기는 두 단계다. 먼저 일어날 수 있는 모든 점 $(x,y)$에 대해 |register|로 $y$를
바깥 마디들에 등록해 둔다. 그런 다음 |build|에서 각 마디의 $y$목록을 정렬·유일화하고
그 크기만큼 안쪽 펜윅을 만든다.
@<이차원 BIT@>=
func (t *bit2d) register(x, y int) {
	for i := x; i <= t.n; i += i & -i {
		t.ys[i] = append(t.ys[i], y)
	}
}

func (t *bit2d) build() {
	for i := 1; i <= t.n; i++ {
		sort.Ints(t.ys[i])
		t.ys[i] = uniq(t.ys[i])
		t.bit[i] = make([]int, len(t.ys[i])+1)
	}
}

@ 점 $(x,y)$의 무게를 |delta|만큼 바꾼다(넣을 땐 $+1$, 뺄 땐 $-1$). 바깥은 $x$에서
위로 오르고, 각 마디에서 $y$의 압축된 순위를 찾아 안쪽 펜윅을 갱신한다. 등록해
두었으므로 $y$는 반드시 목록 안에 있다.
@<이차원 BIT@>=
func (t *bit2d) update(x, y, delta int) {
	for i := x; i <= t.n; i += i & -i {
		ys := t.ys[i]
		r := sort.SearchInts(ys, y) + 1
		for j := r; j <= len(ys); j += j & -j {
			t.bit[i][j] += delta
		}
	}
}

@ |query| 는 $x'\le x$, $y'\le y$인 점의 개수다. 바깥은 $x$에서 아래로 내려가고,
각 마디에서 $y$ 이하인 값의 수를 이분 탐색으로 구해 안쪽 펜윅의 그만큼을 합한다.
@<이차원 BIT@>=
func (t *bit2d) query(x, y int) int {
	sum := 0
	for i := x; i > 0; i -= i & -i {
		ys := t.ys[i]
		c := sort.SearchInts(ys, y+1) // y 이하인 값의 개수
		for j := c; j > 0; j -= j & -j {
			sum += t.bit[i][j]
		}
	}
	return sum
}

@ 직사각형 $[l_a,r_a]\times[l_b,r_b]$의 점 개수는 누적 개수 넷의 포함배제다.
@<이차원 BIT@>=
func (t *bit2d) rect(la, ra, lb, rb int) int {
	return t.query(ra, rb) - t.query(la-1, rb) - t.query(ra, lb-1) + t.query(la-1, lb-1)
}

@ 정렬된 슬라이스에서 중복을 없앤다.
@<유일화 도우미@>=
func uniq(s []int) []int {
	j := 0
	for i, v := range s {
		if i == 0 || v != s[j-1] {
			s[j] = v
			j++
		}
	}
	return s[:j]
}

@* 점으로 바꾸기.
|a|와 |b|를 $1$부터 센다. |posA[v]|와 |posB[v]|는 값 $v$의 자리다. 열 $i$($a$에서의
자리)에 놓인 점의 현재 $y$좌표를 |curY[i]|로 들고 다닌다. 처음에는 $a[i]$의 $b$에서의
자리, 곧 |posB[a[i]]|이다.
@<값을 점으로 바꾼다@>=
posA := make([]int, n+1)
posB := make([]int, n+1)
for i := 1; i <= n; i++ {
	posA[a[i]] = i
	posB[b[i]] = i
}
curY := make([]int, n+1)
for i := 1; i <= n; i++ {
	curY[i] = posB[a[i]]
}
tree := newBIT2D(n)

@ 좌표를 모으려면 일어날 모든 점을 알아야 한다. 처음 점들을 등록하고, 이어 |b|의
사본을 두고 맞바꿈을 흉내 내며 새로 생길 $y$좌표들을 등록한다. 맞바꿈에서 값
$u=b[x]$는 $y$자리로, $v=b[y]$는 $x$자리로 가므로, 각각의 열 |posA[u]|, |posA[v]|에
새 $y$좌표 |y|와 |x|를 등록한다. 다 모았으면 |build|로 트리를 세운다.
@<모든 질의를 미리 훑어 좌표를 모은다@>=
for i := 1; i <= n; i++ {
	tree.register(i, curY[i])
}
bb := make([]int, n+1)
copy(bb, b)
for _, qu := range queries {
	if qu.kind == 2 {
		x, y := qu.arg[0], qu.arg[1]
		if x == y {
			continue
		}
		u, v := bb[x], bb[y]
		tree.register(posA[u], y)
		tree.register(posA[v], x)
		bb[x], bb[y] = v, u
	}
}
tree.build()

@ @<처음 점들을 넣는다@>=
for i := 1; i <= n; i++ {
	tree.update(i, curY[i], 1)
}

@ 이제 질의를 순서대로 처리한다. 종류 1이면 직사각형 안의 점을 세어 답에 적는다.
종류 2이면 두 값의 점을 옮긴다: 헌 $y$좌표의 점을 빼고($-1$), |curY|를 새 자리로
고친 뒤 새 점을 넣는다($+1$). 마지막으로 실제 |b|도 맞바꿔 다음 질의에 대비한다.
@<질의를 차례로 처리한다@>=
out := bufio.NewWriter(os.Stdout)
defer out.Flush()
for _, qu := range queries {
	if qu.kind == 1 {
		ans := tree.rect(qu.arg[0], qu.arg[1], qu.arg[2], qu.arg[3])
		out.WriteString(strconv.Itoa(ans))
		out.WriteByte('\n')
		continue
	}
	x, y := qu.arg[0], qu.arg[1]
	if x == y {
		continue
	}
	u, v := b[x], b[y]
	cu, cv := posA[u], posA[v]
	tree.update(cu, curY[cu], -1)
	curY[cu] = y
	tree.update(cu, y, 1)
	tree.update(cv, curY[cv], -1)
	curY[cv] = x
	tree.update(cv, x, 1)
	b[x], b[y] = v, u
}

@* 입출력.
원소와 질의가 각각 수십만이라 버퍼를 키운 단어 스캐너로 읽는다. 질의는 종류에
따라 인자 수가 다르니 종류를 먼저 읽고 나머지를 채운다.
@<입력을 읽는다@>=
sc := bufio.NewScanner(os.Stdin)
sc.Buffer(make([]byte, 1<<20), 1<<26)
sc.Split(bufio.ScanWords)
readInt := func() int {
	sc.Scan()
	v, _ := strconv.Atoi(sc.Text())
	return v
}

n := readInt()
q := readInt()
a := make([]int, n+1)
b := make([]int, n+1)
for i := 1; i <= n; i++ {
	a[i] = readInt()
}
for i := 1; i <= n; i++ {
	b[i] = readInt()
}

type query struct {
	kind int
	arg  [4]int
}
queries := make([]query, q)
for k := 0; k < q; k++ {
	kind := readInt()
	queries[k].kind = kind
	if kind == 1 {
		queries[k].arg[0] = readInt()
		queries[k].arg[1] = readInt()
		queries[k].arg[2] = readInt()
		queries[k].arg[3] = readInt()
	} else {
		queries[k].arg[0] = readInt()
		queries[k].arg[1] = readInt()
	}
}

@* 색인.
