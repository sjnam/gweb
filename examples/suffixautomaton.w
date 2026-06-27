\input kotexgweb.tex
\def\title{접미 오토마톤}

@* 들어가며.
{\it 접미 오토마톤\/}(suffix automaton, SAM)은 한 문자열 |s|의 {\it 모든
부분문자열\/}을 받아들이는 가장 작은 결정적 유한 오토마톤이다. 시작 상태에서
간선을 따라 글자를 읽어 갈 때, 지나온 글자열은 언제나 |s|의 부분문자열이고,
거꾸로 |s|의 모든 부분문자열이 그런 경로로 꼭 한 가지로 나타난다. 놀랍게도 상태
수와 간선 수가 모두 길이에 {\it 선형\/}($O(n)$)이며, 글자를 하나씩 덧붙이는
{\it 온라인\/} 방식으로 전체를 $O(n)$에 세울 수 있다.

한 번 만들어 두면 서로 다른 부분문자열의 개수, 두 문자열의 가장 긴 공통 부분문자열,
각 부분문자열의 출현 횟수 따위를 모두 빠르게 답할 수 있다. 이 글은
{\tt cp-algorithms.com}의 설명을 따라 SAM을 세우고, 그 첫 응용으로 {\it 서로 다른
부분문자열의 개수\/}를 세어 본다.

@ 프로그램은 표준 입력에서 한 줄짜리 문자열을 읽어, 그 문자열의 비어 있지 않은
서로 다른 부분문자열이 몇 개인지 출력한다.
@c
package main

import (
	"bufio"
	"fmt"
	"maps"
	"os"
)

@<상태와 자동자@>
@<글자 하나 덧붙이기@>
@<서로 다른 부분문자열의 개수@>

func main() {
	@<문자열을 읽어 개수를 출력한다@>
}

@* 상태와 접미 링크.
SAM의 상태는 부분문자열들을 {\it 끝위치 집합\/}(endpos)으로 묶은 동치류다. 어떤
부분문자열이 |s| 안에서 끝나는 위치들의 집합이 서로 같은 부분문자열끼리 한 상태에
모인다. 한 상태가 품은 문자열들은 길이순으로 한 줄로 늘어서는데, 그중 가장 긴 것의
길이를 |length|로 적는다.

상태 |v|의 {\it 접미 링크\/} |link|는, |v|의 가장 긴 문자열에서 앞 글자를 하나씩
떼다가 끝위치 집합이 처음으로 달라지는(더 커지는) 순간의 상태를 가리킨다. 이
링크들은 시작 상태를 뿌리로 하는 트리를 이루며, 구축의 뼈대가 된다. 상태 하나는
|length|, |link|, 그리고 글자별 전이 |next|를 가진다.
@<상태와 자동자@>=
type state struct {
	length int          // 이 상태가 품은 가장 긴 문자열의 길이
	link   int          // 접미 링크 (시작 상태는 -1)
	next   map[byte]int // 글자 -> 다음 상태
}

type sam struct {
	st   []state // 상태 0번이 시작 상태
	last int     // 지금까지의 문자열 전체에 대응하는 상태
}

func newSAM() *sam {
	s := &sam{}
	s.st = append(s.st, state{length: 0, link: -1, next: map[byte]int{}})
	s.last = 0
	return s
}

@* 한 글자씩 키우기.
글자 |c|를 문자열 끝에 덧붙이는 |extend| 하나가 구축의 전부다. 먼저 새 문자열
전체를 대표할 상태 |cur|를 만든다(길이는 직전 길이$+1$). 그런 다음 |last|에서
접미 링크를 타고 오르며, 아직 |c| 전이가 없는 상태마다 |cur|로 가는 |c| 전이를
달아 준다. 어딘가에서 이미 |c| 전이가 있는 상태 |p|를 만나거나 뿌리 위로 벗어나면
멈춘다. 멈춘 자리에 따라 |cur|의 접미 링크가 세 갈래로 정해진다.
@<글자 하나 덧붙이기@>=
func (s *sam) extend(c byte) {
	cur := len(s.st)
	s.st = append(s.st, state{length: s.st[s.last].length + 1, link: -1, next: map[byte]int{}})
	@<last에서 접미 링크를 타며 c 전이를 단다@>
	@<멈춘 자리에 따라 cur의 접미 링크를 정한다@>
	s.last = cur
}

@ |c| 전이가 없는 상태들에 |cur|로 가는 전이를 달며 접미 링크를 타고 오른다.
@<last에서 접미 링크를 타며 c 전이를 단다@>=
p := s.last
for p != -1 {
	if _, ok := s.st[p].next[c]; ok {
		break
	}
	s.st[p].next[c] = cur
	p = s.st[p].link
}

@ 세 갈래다. {\bf (1)} 뿌리 위로 벗어났으면(|p == -1|) |cur|의 링크는 시작 상태다.
{\bf (2)} |p|의 |c| 전이가 가리키는 |q|가 ``바로 다음 길이''이면(|len(p)+1 ==
len(q)|) |q|가 그대로 |cur|의 접미 링크다. {\bf (3)} 그렇지 않으면 |q|는 길이가
|len(p)+1|보다 긴 문자열까지 한 상태에 품고 있어, 새 글자 때문에 끝위치가 둘로
갈라진다. 이때는 |q|를 {\it 쪼개야\/} 한다.
@<멈춘 자리에 따라 cur의 접미 링크를 정한다@>=
if p == -1 {
	s.st[cur].link = 0
} else if q := s.st[p].next[c]; s.st[p].length+1 == s.st[q].length {
	s.st[cur].link = q
} else {
	@<q를 복제해 둘로 쪼갠다@>
}

@ 길이 |len(p)+1|짜리 복제 |clone|을 만들어 |q|의 전이와 접미 링크를 그대로
물려준다. 그러면 |clone|은 짧은(끝위치가 더 많은) 쪽을, |q|는 긴 쪽을 맡는다.
다음으로 |p|에서 접미 링크를 타며 여태 |q|로 가던 |c| 전이들을 모두 |clone|으로
돌린다. 마지막으로 |q|와 |cur|의 접미 링크를 |clone|으로 맞춘다.
@<q를 복제해 둘로 쪼갠다@>=
clone := len(s.st)
s.st = append(s.st, state{
	length: s.st[p].length + 1,
	link:   s.st[q].link,
	next:   maps.Clone(s.st[q].next),
})
for p != -1 && s.st[p].next[c] == q {
	s.st[p].next[c] = clone
	p = s.st[p].link
}
s.st[q].link = clone
s.st[cur].link = clone

@* 서로 다른 부분문자열 세기.
구축이 끝나면 첫 응용은 거저 얻는다. 시작 상태를 뺀 각 상태 |v|는 자기가 품은
문자열들, 곧 길이가 |len(link(v))+1|부터 |len(v)|까지인 문자열들을 대표하는데,
이들은 모두 서로 다르고 다른 상태와 겹치지 않는다. 따라서 상태마다
|len(v) - len(link(v))|개씩 더하면 서로 다른 부분문자열의 총수가 된다.
@<서로 다른 부분문자열의 개수@>=
func (s *sam) distinctSubstrings() int {
	total := 0
	for v := 1; v < len(s.st); v++ {
		total += s.st[v].length - s.st[s.st[v].link].length
	}
	return total
}

@ 한 줄을 읽어 글자마다 |extend|를 부르고, 다 세운 자동자에 개수를 묻는다. 긴
문자열도 받도록 스캐너의 버퍼를 키운다.
@<문자열을 읽어 개수를 출력한다@>=
sc := bufio.NewScanner(os.Stdin)
sc.Buffer(make([]byte, 1<<20), 1<<26)
sc.Scan()
s := sc.Text()

sa := newSAM()
for i := 0; i < len(s); i++ {
	sa.extend(s[i])
}

fmt.Println(sa.distinctSubstrings())

@* 색인.
