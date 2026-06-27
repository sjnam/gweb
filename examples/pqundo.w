\input kotexgweb.tex
\def\title{우선순위 큐 undo 트릭}

@* 들어가며.
코드포스 603E 문제다. 정점 |n|개의 그래프에 가중치 간선이 하나씩 |m|번 더해진다.
간선이 더해질 때마다, {\it 모든 정점의 차수가 홀수\/}인 부분그래프를 고를 때 그
부분그래프의 최대 간선 가중치를 얼마나 작게 만들 수 있는지를 출력한다(불가능하면
$-1$).

먼저 문제를 다시 본다. ``모든 정점이 홀수 차수''인 간선 부분집합은 $T$-조인($T=$
모든 정점)이고, 그런 집합은 그래프의 {\it 모든 연결요소의 크기가 짝수\/}일 때만
존재한다. 게다가 임계값 $W$를 정해 가중치 $\le W$인 간선만 쓸 때 모든 연결요소가
짝수이면, 그 안에서 원하는 부분그래프를 최대 가중치 $\le W$로 고를 수 있다. 따라서
답은 {\it 가중치 $\le W$인 간선들로 모든 연결요소가 짝수가 되는 가장 작은 $W$\/}이다.
``모든 연결요소 짝수''는 $W$가 커질수록 한 번 성립하면 계속 성립하므로(짝수 요소끼리
합쳐도 짝수), 가장 작은 $W$는 {\it 가벼운 간선부터 더하다 처음 모두 짝수가 되는
순간의 가중치\/}와 같다.

이를 동적으로 풀려면 간선 집합을 들고 있다가, 새 간선이 들어올 때마다 ``현재 가장
무거운 간선을 빼도 모두 짝수면 빼기''를 반복해 임계값을 끌어내려야 한다. 이때
필요한 것이 {\it 우선순위 큐 undo 트릭\/}이다: 되돌리기(rollback)만 지원하는 자료
구조(스택)에서, 스택 한가운데 묻힌 {\it 가장 우선순위 높은\/} 원소를 amortized
$O(T(n)\log n)$에 빼낸다.
@^참고: infossm.github.io priority-queue-undo-trick@>

@ 프로그램은 |n|, |m|과 |m|개의 간선 $(u,v,w)$를 읽어, 각 간선을 더한 뒤의 답을
한 줄씩 출력한다.
@c
package main

import (
	"bufio"
	"os"
	"strconv"
)

type edge struct{ u, v, w int }

@<롤백 유니온-파인드@>
@<우선순위 큐 undo@>
@<문제 풀이@>

func main() {
	@<입력을 읽어 답을 출력한다@>
}

@* 되돌릴 수 있는 유니온-파인드.
경로 압축을 쓰지 않고 {\it 크기로 union\/}하면, 각 union은 부모 하나만 바꾸므로
변경 이력을 스택에 쌓아 정확히 되돌릴 수 있다. 우리는 {\it 홀수 크기 연결요소의
수\/} |odd|를 함께 관리한다. |odd == 0|이면 모든 요소가 짝수다. 같은 요소를 잇는
union은 아무것도 바꾸지 않지만, 되돌리기 짝을 맞추려 빈 이력(|a == -1|)을 남긴다.
@<롤백 유니온-파인드@>=
type histItem struct{ a, b, oddBefore int } // a==-1이면 빈 union

type dsu struct {
	parent, size []int
	odd          int
	h            []histItem
}

func newDSU(n int) *dsu {
	d := &dsu{parent: make([]int, n+1), size: make([]int, n+1), odd: n}
	for i := 1; i <= n; i++ {
		d.parent[i] = i
		d.size[i] = 1
	}
	return d
}

func (d *dsu) find(x int) int {
	for d.parent[x] != x {
		x = d.parent[x]
	}
	return x
}

@ |version|은 되돌릴 지점을 가리키는 이력 길이다. |union|은 작은 쪽을 큰 쪽에 붙이고
|odd|를 갱신하며 이력을 남긴다.
@<롤백 유니온-파인드@>=
func (d *dsu) version() int { return len(d.h) }

func (d *dsu) union(x, y int) {
	rx, ry := d.find(x), d.find(y)
	if rx == ry {
		d.h = append(d.h, histItem{a: -1})
		return
	}
	if d.size[rx] < d.size[ry] {
		rx, ry = ry, rx
	}
	ob := d.odd
	d.odd -= d.size[rx] & 1
	d.odd -= d.size[ry] & 1
	d.parent[ry] = rx
	d.size[rx] += d.size[ry]
	d.odd += d.size[rx] & 1
	d.h = append(d.h, histItem{a: ry, b: rx, oddBefore: ob})
}

@ |rollback|은 이력을 지정한 길이까지 되감아, 그동안의 union을 정확히 거꾸로 푼다.
@<롤백 유니온-파인드@>=
func (d *dsu) rollback(to int) {
	for len(d.h) > to {
		e := d.h[len(d.h)-1]
		d.h = d.h[:len(d.h)-1]
		if e.a == -1 {
			continue
		}
		d.size[e.b] -= d.size[e.a]
		d.parent[e.a] = e.a
		d.odd = e.oddBefore
	}
}

@* 우선순위 큐 undo.
간선들을 더한 {\it 순서대로\/} 스택에 쌓되, 각 간선을 더하기 직전의 |version|을
함께 적어 둔다(그 간선만 되돌릴 지점). 연결 상태와 |odd|는 간선 {\it 집합\/}에만
달려 있고 순서와 무관하므로, 빼낼 때 남은 것들을 어떤 순서로 다시 쌓아도 결과는
같다. 이 자유가 트릭의 핵심이다.

간선마다 고정된 번호 |id|를 주고, 그 현재 스택 위치를 |pos[id]|로 들고 다닌다(빠지면
$-1$). 가장 무거운 간선을 빨리 찾으려고 최대 힙을 둔다. 같은 간선이 재배치로 같은
{\it 위치\/}에 다시 쌓일 수도 있으므로, 위치가 아니라 push마다 증가하는 {\it 토큰\/}
|tok[id]|으로 최신성을 본다: 힙 항목은 자기 토큰이 그 간선의 최신 토큰과 같을 때만
유효하다(지연 삭제). 또 가중치가 같은 간선이 여럿이면 어느 것을 뺄지가 결과를 가르므로,
힙 순위를 {\it 가중치, 같으면 더 작은 |id|\/}로 정해 항상 같은 선택을 한다.
@<우선순위 큐 undo@>=
type hitem struct{ w, id, p, tok int } // 가중치, 번호, 위치, 토큰

type pqUndo struct {
	d                    *dsu
	gtok                 int
	eu, ev, ew, pos, tok []int // 번호별 양끝/가중치/현재 위치/최신 토큰
	items, ver           []int  // 스택: 간선 번호와 적용 직전 version
	heap                 []hitem
}

func newPQUndo(d *dsu) *pqUndo { return &pqUndo{d: d} }

func (p *pqUndo) addEdge(u, v, w int) int {
	id := len(p.eu)
	p.eu = append(p.eu, u)
	p.ev = append(p.ev, v)
	p.ew = append(p.ew, w)
	p.pos = append(p.pos, -1)
	p.tok = append(p.tok, 0)
	return id
}

@ 손으로 짠 최대 힙이다(표준 라이브러리 대신 군더더기 없이). 순위 |higher|는 가중치가
크면, 같으면 |id|가 작으면 위로 간다.
@<우선순위 큐 undo@>=
func higher(a, b hitem) bool {
	return a.w > b.w || (a.w == b.w && a.id < b.id)
}

func (p *pqUndo) hup(i int) {
	for i > 0 {
		par := (i - 1) / 2
		if !higher(p.heap[i], p.heap[par]) {
			break
		}
		p.heap[par], p.heap[i] = p.heap[i], p.heap[par]
		i = par
	}
}

func (p *pqUndo) hdown(i int) {
	n := len(p.heap)
	for {
		l, r, b := 2*i+1, 2*i+2, i
		if l < n && higher(p.heap[l], p.heap[b]) {
			b = l
		}
		if r < n && higher(p.heap[r], p.heap[b]) {
			b = r
		}
		if b == i {
			break
		}
		p.heap[i], p.heap[b] = p.heap[b], p.heap[i]
		i = b
	}
}

func (p *pqUndo) hpush(e hitem) { p.heap = append(p.heap, e); p.hup(len(p.heap) - 1) }

func (p *pqUndo) hpop() hitem {
	top := p.heap[0]
	last := len(p.heap) - 1
	p.heap[0] = p.heap[last]
	p.heap = p.heap[:last]
	if last > 0 {
		p.hdown(0)
	}
	return top
}

@ |push|는 간선 하나를 스택 맨 위에 쌓아 DSU에 적용한다. |valid|는 힙 항목이
아직 그 위치에 살아 있는지 본다. |maxW|는 무효 항목을 걷어내며 현재 가장 무거운
간선의 가중치를 돌려준다(없으면 $-1$).
@<우선순위 큐 undo@>=
func (p *pqUndo) push(id int) {
	p.gtok++
	p.pos[id] = len(p.items)
	p.tok[id] = p.gtok
	p.ver = append(p.ver, p.d.version())
	p.d.union(p.eu[id], p.ev[id])
	p.items = append(p.items, id)
	p.hpush(hitem{p.ew[id], id, p.pos[id], p.gtok})
}

func (p *pqUndo) valid(e hitem) bool { return p.tok[e.id] == e.tok }

func (p *pqUndo) maxW() int {
	for len(p.heap) > 0 {
		if p.valid(p.heap[0]) {
			return p.heap[0].w
		}
		p.hpop()
	}
	return -1
}

@ 이제 핵심인 |deleteMax|다. 스택 맨 위 |k|개만 되돌리면 가장 무거운 간선을 끄집어낼
수 있도록, {\it 가장 무거운 $\lceil k/2\rceil$개가 모두 위 $k$칸 안에 들어오는 가장
작은 $k$\/}를 고른다. 힙에서 무거운 것부터 꺼내며, 그중 가장 깊은 위치 |minpos|로
$k=\max(2t-1,\;n-minpos)$를 정하고, $n-minpos\le 2t$가 되는 순간 멈춘다($t$는 꺼낸
개수).
@<우선순위 큐 undo@>=
func (p *pqUndo) deleteMax() int {
	n := len(p.items)
	popped := make([]hitem, 0, 8)
	minpos, k := n, 0
	for {
		for len(p.heap) > 0 && !p.valid(p.heap[0]) {
			p.hpop()
		}
		e := p.hpop()
		popped = append(popped, e)
		if e.p < minpos {
			minpos = e.p
		}
		t := len(popped)
		if need := n - minpos; need <= 2*t {
			k = max(2*t-1, need)
			break
		}
	}
	@<위 k개를 되돌리고 최댓값을 뺀 채 다시 쌓는다@>
	return rid
}

@ 위 |k|개를 되돌린 뒤, 가장 무거운 간선 |rid|만 버린다. 나머지를 다시 쌓되,
{\it 무거운 절반\/}(꺼낸 것들)은 가중치 오름차순으로 맨 위에 두어, 다음번
|deleteMax|가 작은 |k|로 끝나게 만든다. 이 재배치가 amortized 비용을 보장한다:
두 삭제 $i<j$에 대해 항상 $2(j-i)\ge\min(C_i,C_j)$가 성립해, 전체 비용이
$O(m\log m)$에 묶인다.
@<위 k개를 되돌리고 최댓값을 뺀 채 다시 쌓는다@>=
base := n - k
rid := popped[0].id // 가장 무거운 간선
high := make(map[int]bool, len(popped))
for _, e := range popped {
	high[e.id] = true
}
p.d.rollback(p.ver[base])
window := append([]int(nil), p.items[base:]...)
p.items = p.items[:base]
p.ver = p.ver[:base]
p.pos[rid] = -1
for _, id := range window { // 가벼운 절반: 원래 순서대로
	if !high[id] {
		p.push(id)
	}
}
up := append([]hitem(nil), popped[1:]...) // 무거운 절반(최댓값 제외): 오름차순
for i := 1; i < len(up); i++ {
	for j := i; j > 0 && up[j-1].w > up[j].w; j-- {
		up[j-1], up[j] = up[j], up[j-1]
	}
}
for _, e := range up {
	p.push(e.id)
}

@* 문제 풀이.
모든 현재 간선을 스택에 올려 둔다. |odd > 0|이면 모든 간선을 써도 짝수가 아니므로
답은 $-1$이다. |odd == 0|이면 임계값을 끌어내린다: 가장 무거운 간선을 빼 보아
여전히 |odd == 0|이면 그 간선은 필요 없으니 영영 버리고, |odd|가 깨지면 그 간선이
바로 필요한 가장 무거운 간선이므로 도로 넣고 멈춘다. 남은 가장 무거운 간선의
가중치가 곧 답이다.
@<문제 풀이@>=
func solve(n int, edges []edge) []int {
	d := newDSU(n)
	p := newPQUndo(d)
	out := make([]int, 0, len(edges))
	for _, e := range edges {
		id := p.addEdge(e.u, e.v, e.w)
		p.push(id)
		ans := -1
		if d.odd == 0 {
			for len(p.items) > 0 {
				rem := p.deleteMax()
				if d.odd != 0 {
					p.push(rem)
					break
				}
			}
			ans = p.maxW()
		}
		out = append(out, ans)
	}
	return out
}

@* 입출력.
간선이 수만 개라 버퍼를 키운 단어 스캐너로 읽고 버퍼 쓰기로 낸다.
@<입력을 읽어 답을 출력한다@>=
sc := bufio.NewScanner(os.Stdin)
sc.Buffer(make([]byte, 1<<20), 1<<26)
sc.Split(bufio.ScanWords)
readInt := func() int {
	sc.Scan()
	v, _ := strconv.Atoi(sc.Text())
	return v
}

n := readInt()
m := readInt()
edges := make([]edge, m)
for i := range edges {
	edges[i] = edge{readInt(), readInt(), readInt()}
}

out := bufio.NewWriter(os.Stdout)
defer out.Flush()
for _, v := range solve(n, edges) {
	out.WriteString(strconv.Itoa(v))
	out.WriteByte('\n')
}

@* 색인.
