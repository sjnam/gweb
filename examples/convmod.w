\input kotexgweb.tex
\def\title{합성곱 (mod 998244353)}

@* 들어가며.
두 정수열 $a_0,\ldots,a_{N-1}$과 $b_0,\ldots,b_{M-1}$의 {\it 합성곱\/}은
$$c_i=\sum_{j+k=i} a_j\,b_k\qquad(0\le i\le N+M-2)$$
로 정의된다. 이 문제(Library Checker {\tt convolution\_mod})는 $c_i$를 소수
$p=998244353$으로 나눈 나머지로 구해 출력하는 것이다.

합성곱은 곧 {\it 다항식 곱셈\/}이다. $a$를 계수로 갖는 다항식
$A(x)=\sum a_j x^j$와 $B(x)=\sum b_k x^k$를 곱하면, $A(x)B(x)$의 $x^i$ 계수가 바로
$c_i$이기 때문이다. 정의대로 더하면 $O(NM)$인데, $N,M$이 각각 $2^{19}$까지라 너무
느리다. 여기서는 {\it 수론적 변환\/}(NTT)으로 $O((N+M)\log(N+M))$에 푼다.

@ 프로그램은 표준 입력에서 |n|과 |m|, 이어 |a|와 |b|의 원소들을 읽어, 합성곱
|n+m-1|개를 공백으로 띄워 한 줄에 출력한다.
@c
package main

import (
	"bufio"
	"os"
	"strconv"
)

const (
	mod   = 998244353 // 998244353 = 119*2^23 + 1, 소수
	proot = 3         // mod의 원시근
)

@<모듈러 거듭제곱@>
@<수론적 변환@>
@<합성곱@>

func main() {
	@<입력을 읽어 합성곱을 출력한다@>
}

@* 수론적 변환.
빠른 곱셈의 발상은 {\it 평가--곱--보간\/}이다. 다항식 $A,B$를 서로 다른 점
$x_0,\ldots,x_{n-1}$에서 값을 구해 두면($n\ge N+M-1$), 곱 $C=AB$의 각 점 값은 단지
$C(x_t)=A(x_t)\,B(x_t)$이다. 점 값들로부터 계수를 되찾는 것이 보간이다. 평가와
보간을 점만 잘 고르면 분할정복으로 각각 $O(n\log n)$에 할 수 있는데, 그 점이 바로
$n$차 {\it 단위근\/}이고, 이것이 고속 푸리에 변환(FFT)이다.

실수 FFT는 부동소수 오차를 안고 가지만, 나머지 연산만 쓰는 정수판이 NTT다.
열쇠는 $p=998244353$이 $p-1=119\cdot2^{23}$이라 $2^{23}$까지의 $2$의 거듭제곱마다
$mod\ p$ 안에 원시 단위근이 있다는 점이다. $p$의 원시근은 $g=3$이고,
$\omega=g^{(p-1)/n}$은 $n$차 원시 단위근 노릇을 한다. 그래서 길이를 $2$의
거듭제곱 $n$으로 맞추면 실수 FFT와 똑같은 골격을 정수 위에서 그대로 쓸 수 있다.

@ 단위근과 보간에 쓸 역원은 모두 거듭제곱으로 얻는다(페르마 소정리로
$x^{-1}=x^{p-2}$). 빠른 거듭제곱이다.
@<모듈러 거듭제곱@>=
func power(base, exp int) int {
	base %= mod
	result := 1
	for exp > 0 {
		if exp&1 == 1 {
			result = result * base % mod
		}
		base = base * base % mod
		exp >>= 1
	}
	return result
}

@* 나비 연산.
반복형 NTT는 두 단계다. 먼저 계수들을 {\it 비트 역순\/}으로 재배열한다. 그러면
분할정복에서 짝수/홀수 첨자로 갈라지던 것이 자리만 옮기면 이웃끼리 묶이게 되어,
제자리에서 아래로부터 합쳐 올라갈 수 있다. 다음으로 묶음의 길이를 $2,4,\ldots,n$로
키우며 {\it 나비 연산\/}으로 두 반쪽을 합친다.
@<수론적 변환@>=
func ntt(a []int, invert bool) {
	n := len(a)
	@<비트 역순으로 재배열한다@>
	@<묶음을 키우며 나비 연산을 한다@>
	@<역변환이면 |n|으로 나눈다@>
}

@ 표준적인 제자리 비트 역순 치환이다. |i|를 $1$부터 키우며 그 비트 역순 |j|를
증분으로 유지하고, |i < j|일 때만 맞바꿔 한 번씩만 교환한다.
@<비트 역순으로 재배열한다@>=
for i, j := 1, 0; i < n; i++ {
	bit := n >> 1
	for ; j&bit != 0; bit >>= 1 {
		j ^= bit
	}
	j ^= bit
	if i < j {
		a[i], a[j] = a[j], a[i]
	}
}

@ 길이 |length|인 묶음마다 $\omega=g^{(p-1)/length}$이 그 길이의 원시 단위근이다
(역변환에서는 그 역원을 쓴다). 두 반쪽 |u|와 $\omega^j\,$|v|를 더하고 빼는 것이
나비 연산이고, 더한 값이 앞 반쪽, 뺀 값이 뒤 반쪽이 된다.
@<묶음을 키우며 나비 연산을 한다@>=
for length := 2; length <= n; length <<= 1 {
	w := power(proot, (mod-1)/length)
	if invert {
		w = power(w, mod-2)
	}
	for i := 0; i < n; i += length {
		wn := 1
		for j := 0; j < length/2; j++ {
			u := a[i+j]
			v := a[i+j+length/2] * wn % mod
			a[i+j] = (u + v) % mod
			a[i+j+length/2] = (u - v + mod) % mod
			wn = wn * w % mod
		}
	}
}

@ 역변환은 정방향과 같은 골격에 단위근의 역원을 쓴 것이라, 마지막에 전체를 |n|으로
나눠 주어야(곧 $n^{-1}$을 곱해야) 제 계수로 돌아온다.
@<역변환이면 |n|으로 나눈다@>=
if invert {
	ninv := power(n, mod-2)
	for i := range a {
		a[i] = a[i] * ninv % mod
	}
}

@* 합성곱.
이제 조립은 간단하다. 결과 길이는 |len(a)+len(b)-1|이다. 그 이상이 되는 가장 작은
$2$의 거듭제곱 |n|으로 두 배열을 |0| 채워 늘리고, 각각 정방향 NTT를 한 뒤 점마다
곱하고, 역 NTT로 계수를 되찾아 필요한 만큼 잘라 돌려준다.
@<합성곱@>=
func convolution(a, b []int) []int {
	size := len(a) + len(b) - 1
	n := 1
	for n < size {
		n <<= 1
	}
	fa := make([]int, n)
	fb := make([]int, n)
	copy(fa, a)
	copy(fb, b)
	ntt(fa, false)
	ntt(fb, false)
	for i := range fa {
		fa[i] = fa[i] * fb[i] % mod
	}
	ntt(fa, true)
	return fa[:size]
}

@* 입출력.
원소가 백만 개에 이를 수 있어 입력은 버퍼를 키운 단어 스캐너로, 출력은 버퍼
쓰기로 빠르게 처리한다. 곱 한 번이 |int|(64비트)에 들어가도록 $mod<2^{30}$임을
이용한다($mod^2<2^{60}$).
@<입력을 읽어 합성곱을 출력한다@>=
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
a := make([]int, n)
for i := range a {
	a[i] = readInt()
}
b := make([]int, m)
for i := range b {
	b[i] = readInt()
}

c := convolution(a, b)
out := bufio.NewWriter(os.Stdout)
defer out.Flush()
for i, v := range c {
	if i > 0 {
		out.WriteByte(' ')
	}
	out.WriteString(strconv.Itoa(v))
}
out.WriteByte('\n')

@* 색인.
