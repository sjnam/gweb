\input kotexgweb.tex
\def\title{흐르는 중앙값}

@* 들어가며.
해커랭크의 {\it 흐르는 중앙값\/}(Running Median) 문제다. 정수가 하나씩 흘러
들어올 때마다, 지금까지 받은 모든 수의 {\it 중앙값\/}을 소수점 한 자리로 출력한다.
수가 홀수 개이면 가운데 값이, 짝수 개이면 가운데 두 값의 평균이 중앙값이다.

이를테면 $12,4,5,3,8,7$이 차례로 들어오면, 매 단계의 모임은
$\{12\},\{4,12\},\{4,5,12\},\ldots$이고 중앙값은
$$12.0,\;8.0,\;5.0,\;4.5,\;5.0,\;6.0$$
이다. 들어올 때마다 다시 정렬하면 한 번에 $O(n\log n)$이라 전체가 너무 느리다.
대신 지금까지의 수를 {\it 크기로 반씩 나눠\/} 두 더미로 들고 있으면, 중앙값은 늘
두 더미의 경계에 있어 $O(\log n)$에 새 수를 받아넘길 수 있다.

@ 프로그램은 표준 입력에서 개수 |n|과 |n|개의 정수를 읽어, 받을 때마다 그 시점의
중앙값을 한 줄씩 출력한다.
@c
package main

import (
	"bufio"
	"container/heap"
	"fmt"
	"os"
	"strconv"
)

@<정수 힙@>
@<중앙값을 읽는다@>
@<수를 하나 더한다@>

func main() {
	@<입력을 읽어 매번 중앙값을 출력한다@>
}

@* 두 힙의 약속.
작은 절반은 {\it 최대 힙\/} |lo|에 담는다. 그러면 그 더미에서 가장 큰 수, 곧
가운데에 가장 가까운 수가 꼭대기에 온다. 큰 절반은 {\it 최소 힙\/} |hi|에 담아,
가장 작은 수가 꼭대기에 오게 한다. 두 힙의 크기를 늘 같거나 |lo|가 딱 하나 더
많도록 유지하면, 수가 홀수 개일 때 중앙값은 |lo|의 꼭대기 하나이고, 짝수 개일
때는 두 꼭대기의 평균이다.

힙은 표준 라이브러리 |container/heap|로 만든다. 최대 힙과 최소 힙은 비교 방향만
다르므로, 비교 함수 |less|를 품은 작은 힙 타입 하나로 둘 다 만든다. |heap| 패키지가
요구하는 다섯 메서드에, 꼭대기를 들여다보는 |top|을 더한다.
@<정수 힙@>=
type intHeap struct {
	data []int
	less func(a, b int) bool // 위로 올라갈 우선순위
}

func (h intHeap) Len() int           { return len(h.data) }
func (h intHeap) Less(i, j int) bool { return h.less(h.data[i], h.data[j]) }
func (h intHeap) Swap(i, j int)      { h.data[i], h.data[j] = h.data[j], h.data[i] }
func (h *intHeap) Push(x any)        { h.data = append(h.data, x.(int)) }

func (h *intHeap) Pop() any {
	n := len(h.data)
	x := h.data[n-1]
	h.data = h.data[:n-1]
	return x
}

func (h intHeap) top() int { return h.data[0] }

@ 크기 약속(|lo| 쪽이 같거나 하나 많음) 덕분에 중앙값은 곧장 읽힌다. |lo|가 더
크면 전체가 홀수 개이고 그 꼭대기가 중앙값이며, 같으면 두 꼭대기의 평균이다.
@<중앙값을 읽는다@>=
func median(lo, hi *intHeap) float64 {
	if lo.Len() > hi.Len() {
		return float64(lo.top())
	}
	return float64(lo.top()+hi.top()) / 2
}

@* 수를 더하고 균형을 맞춘다.
새 수 |x|는 먼저 어느 절반에 속하는지 정한다. |lo|가 비었거나 |x|가 |lo|의 꼭대기
이하이면 작은 절반(|lo|)에, 아니면 큰 절반(|hi|)에 넣는다. 그러고 나서 두 힙의
크기가 약속을 벗어났으면 꼭대기 하나를 반대편으로 옮겨 바로잡는다. 이렇게 해도
``|lo|의 모든 값 $\le$ |hi|의 모든 값''이라는 분할은 깨지지 않는다.
@<수를 하나 더한다@>=
func add(lo, hi *intHeap, x int) {
	@<x를 알맞은 절반에 넣는다@>
	@<두 힙의 크기를 다시 맞춘다@>
}

@ 경계는 |lo|의 꼭대기다. 새 수가 그보다 크지 않으면 작은 절반에 속한다.
@<x를 알맞은 절반에 넣는다@>=
if lo.Len() == 0 || x <= lo.top() {
	heap.Push(lo, x)
} else {
	heap.Push(hi, x)
}

@ 한쪽이 너무 커지면 그쪽 꼭대기를 떼어 반대편에 넣는다. 한 번에 많아야 하나만
어긋나므로 |if|\,--\,|else if| 한 번이면 약속이 회복된다.
@<두 힙의 크기를 다시 맞춘다@>=
if lo.Len() > hi.Len()+1 {
	heap.Push(hi, heap.Pop(lo))
} else if hi.Len() > lo.Len() {
	heap.Push(lo, heap.Pop(hi))
}

@* 입출력.
작은 절반은 최대 힙(큰 값이 위로), 큰 절반은 최소 힙(작은 값이 위로)으로 만든다.
수마다 더하고 곧바로 중앙값을 소수점 한 자리로 찍는다.
@<입력을 읽어 매번 중앙값을 출력한다@>=
sc := bufio.NewScanner(os.Stdin)
sc.Buffer(make([]byte, 1<<20), 1<<26)
sc.Split(bufio.ScanWords)
readInt := func() int {
	sc.Scan()
	v, _ := strconv.Atoi(sc.Text())
	return v
}

lo := &intHeap{less: func(a, b int) bool { return a > b }} // 작은 절반: 최대 힙
hi := &intHeap{less: func(a, b int) bool { return a < b }} // 큰 절반: 최소 힙

n := readInt()
out := bufio.NewWriter(os.Stdout)
defer out.Flush()
for i := 0; i < n; i++ {
	add(lo, hi, readInt())
	fmt.Fprintf(out, "%.1f\n", median(lo, hi))
}

@* 색인.
