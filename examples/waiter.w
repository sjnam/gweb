\input kotexgweb.tex
\def\title{웨이터}

@* 들어가며.
해커랭크의 {\it 웨이터\/} 문제다. 번호가 적힌 접시 여러 개가 하나의 더미에 쌓여
있다. 입력으로 먼저 주어진 접시가 맨 아래, 나중 것이 맨 위다. 모두 |q|번의 차례를
거치는데, $i$번째 차례에는 $i$번째 소수 $p_i$를 쓴다. 곧 $p_1=2$, $p_2=3$,
$p_3=5$, $\ldots$ 이다.

$i$번째 차례에는 현재 더미 $A_{i-1}$을 맨 위에서부터 한 장씩 꺼내, 번호가 $p_i$로
나누어떨어지면 더미 $B_i$에, 아니면 더미 $A_i$에 얹는다. 한 차례가 끝나면 그때
모인 $B_i$를 맨 위에서부터 꺼내 답에 차례로 적는다. |q|번이 다 끝나면 마지막까지
남은 $A_q$도 맨 위에서부터 꺼내 답 뒤에 붙인다.

이를테면 접시가 아래에서 위로 $3,4,7,6,5$로 쌓였고 |q|=1이면, $p_1=2$로 가른다.
맨 위 $5$부터 꺼내면 $5,6,7,4,3$ 순서이고, 짝수 $6,4$는 $B_1$으로, 홀수
$5,7,3$은 $A_1$으로 간다. $B_1$을 맨 위에서부터 꺼내면 $4,6$, 남은 $A_1$을 맨
위에서부터 꺼내면 $3,7,5$이니, 답은 $4,6,3,7,5$이다.

@ 프로그램은 표준 입력에서 접시 수 |n|과 차례 수 |q|, 이어서 |n|개의 접시 번호를
읽어, 답을 한 줄에 하나씩 출력한다.
@c
package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
)

@<처음 |q|개의 소수를 만든다@>
@<접시를 나누어 답을 만든다@>

func main() {
	@<입력을 읽어 답을 출력한다@>
}

@* 소수 만들기.
$q$번째 소수까지 있으면 된다. |q|가 커야 $1200$ 안팎이고 그 소수도 만 단위를 넘지
않으니, 정교한 체 없이 시험 나눗셈으로 충분하다. 이미 찾은 소수들로만 후보를
나눠 보되, 어떤 소수의 제곱이 후보를 넘어서면 더 볼 것이 없으므로 멈춘다.
@<처음 |q|개의 소수를 만든다@>=
func firstPrimes(q int) []int {
	primes := make([]int, 0, q)
	for cand := 2; len(primes) < q; cand++ {
		isPrime := true
		for _, p := range primes {
			if p*p > cand {
				break
			}
			if cand%p == 0 {
				isPrime = false
				break
			}
		}
		if isPrime {
			primes = append(primes, cand)
		}
	}
	return primes
}

@* 접시 나누기.
더미는 슬라이스로 나타내되 {\it 마지막 원소가 맨 위\/}라고 약속한다. 그러면 ``맨
위에서부터 꺼낸다''는 말은 슬라이스를 뒤에서 앞으로 훑는다는 뜻이 된다. 각 차례마다
현재 더미를 두 더미로 가르고, 나누어떨어진 쪽을 답에 옮긴다.
@<접시를 나누어 답을 만든다@>=
func waiter(number []int, q int) []int {
	primes := firstPrimes(q)
	answers := make([]int, 0, len(number))
	a := number
	for i := 0; i < q; i++ {
		p := primes[i]
		var b, next []int
		@<더미 |a|를 |p|로 |b|와 |next|로 가른다@>
		@<|b|를 맨 위에서부터 답에 적는다@>
		a = next
	}
	@<남은 더미 |a|를 맨 위에서부터 답에 붙인다@>
	return answers
}

@ 맨 위 |a[len(a)-1]|부터 아래로 내려가며, $p$로 나누어떨어지는 접시는 |b|에,
나머지는 |next|에 얹는다. 두 더미 모두 ``꺼낸 순서대로 쌓이므로'' 역시 마지막
원소가 맨 위가 된다.
@<더미 |a|를 |p|로 |b|와 |next|로 가른다@>=
for j := len(a) - 1; j >= 0; j-- {
	if a[j]%p == 0 {
		b = append(b, a[j])
	} else {
		next = append(next, a[j])
	}
}

@ 이번 차례에 걸러진 |b|를 맨 위에서부터 꺼내 답에 적는다.
@<|b|를 맨 위에서부터 답에 적는다@>=
for j := len(b) - 1; j >= 0; j-- {
	answers = append(answers, b[j])
}

@ |q|번이 끝나고 남은 더미를 같은 방식으로 답 뒤에 붙이면 끝이다.
@<남은 더미 |a|를 맨 위에서부터 답에 붙인다@>=
for j := len(a) - 1; j >= 0; j-- {
	answers = append(answers, a[j])
}

@* 입출력.
접시가 최대 오만 장이라 입력은 버퍼를 키운 단어 스캐너로, 출력은 버퍼 쓰기로
처리한다.
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
q := readInt()
number := make([]int, n)
for i := range number {
	number[i] = readInt()
}

out := bufio.NewWriter(os.Stdout)
defer out.Flush()
for _, v := range waiter(number, q) {
	fmt.Fprintln(out, v)
}

@* 색인.
