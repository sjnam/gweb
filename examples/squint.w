@s Rat int
@s Context int
@s testing.T int

\def\title{Squinting at Power Series}
\datethis

@* Introduction.
A power series
$$F(x)=\sum_{i\ge0}F_i\,x^i=F_0+F_1x+F_2x^2+\cdots$$
is an infinite object, but its coefficients $F_0,F_1,F_2,\ldots$ arrive one at a
time, and almost every interesting operation on series --- sum, product,
composition, reciprocal, functional inverse, even the solution of a differential
equation --- can be computed coefficient by coefficient, never looking further
ahead than it must. That is exactly the shape of a {\it lazy stream,\/} and this
package follows M.~Douglas McIlroy's lovely paper {\sl Squinting at Power Series\/}
({\sl Software---Practice and Experience\/} {\bf 20} (1990), 661--683) in
realizing each series as a {\it stream of rational coefficients\/} and each
algebraic operation as a small {\it concurrent process.\/}

The representation is a {\it demand channel,\/} McIlroy's central device. A series is
a pair of channels: a consumer sends a token on the request channel, and only then
does the producer compute and send the next coefficient on the data channel. No
process ever runs ahead of demand, so even series defined in terms of themselves ---
$\exp$ as the solution of $X'=XF'$, or $\tan$ from $\tan'=1+\tan^2$ --- evaluate
without runaway. The whole arithmetic of series becomes a network of goroutines
joined by channels, and writing each operator is mostly a matter of transcribing
its defining equation.

Coefficients are exact rationals (\GO/'s |math/big.Rat|), so nothing is lost to
rounding. Each running series is a network of goroutines whose lifetime is governed
by a |context.Context|: generators take one, derived series inherit it, and
cancelling it shuts the whole network down.

@ A validation program is provided so that installers can tell if {\sc squint}
is working properly. To make the test, simply run \.{squint\_test.go}.
@(squint_test.go@>=
package squint

import (
	"context"
	"testing"
)

@<Test Helpers@>
@<Test |Deriv| routine@>
@<Test |Recip| routine@>

@ The \GO/ code for {\sc squint} doesn't have a main routine; it's just a bunch
of subroutines to be incorporated into programs at a higher level via the system
loading routine. Here is the general outline of \.{squint.go}:

@p
package squint

import (
	"context"
	"math/big"
)

@<Type definition@>
@<Private subroutines@>
@<Public subroutines@>

@ A |PS| value is a handle on a running series: the two channels of its demand
protocol, plus the context whose cancellation ends it. Coefficients (|dat|) are
|*big.Rat|; a request (|req|) carries no data, so it is an empty struct. |mkPS|
makes the channels; the goroutine that fills them is started by whichever
generator or operator created the series.
@<Type definition@>=
type PS struct {
	ctx context.Context
	req chan struct{}
	dat chan *big.Rat
}

@ @<Private subroutines@>=
func mkPS(ctx context.Context) PS {
	return PS{
		ctx: ctx,
		req: make(chan struct{}),
		dat: make(chan *big.Rat),
	}
}

@* The demand protocol.
Four little methods are the entire vocabulary of the network; every operator below
is built from them. A consumer drives a series with |get|: it places a demand and
then waits for the coefficient. Each half of the exchange races against the
context, so a cancelled network never blocks --- |get| simply reports |ok| false.
@<Private subroutines@>=
func (F PS) get() (v *big.Rat, ok bool) {
	select {
	case F.req <- struct{}{}:
	case <-F.ctx.Done():
		return nil, false
	}
	select {
	case v = <-F.dat:
		return v, true
	case <-F.ctx.Done():
		return nil, false
	}
}

@ A producer is the mirror image. |awaitReq| blocks until a demand arrives; |send|
hands over one coefficient to a demand already received; and |put| does both in
order --- wait, then deliver. The single most important fact about |put| is that a
process can {\it produce a term before demanding any input,\/} which is what lets the
self-referential definitions later on get started instead of deadlocking.
@<Private subroutines@>=
func (F PS) awaitReq() bool {
	select {
	case <-F.req:
		return true
	case <-F.ctx.Done():
		return false
	}
}

func (F PS) send(v *big.Rat) bool {
	select {
	case F.dat <- v:
		return true
	case <-F.ctx.Done():
		return false
	}
}

func (F PS) put(v *big.Rat) bool {
	return F.awaitReq() && F.send(v)
}

@ Two public accessors let a caller read a series. |Get| demands one coefficient
(|nil| once the context is cancelled).
@<Public subroutines@>=
func (F PS) Get() *big.Rat {
	v, _ := F.get()
	return v
}

@ |Take| collects the first |n|, stopping early if the network is shut down mid-stream.
@<Public subroutines@>=
func (F PS) Take(n int) []*big.Rat {
	cs := make([]*big.Rat, 0, n)
	for i := 0; i < n; i++ {
		v, ok := F.get()
		if !ok {
			break
		}
		cs = append(cs, v)
	}
	return cs
}

@ A handful of one-line wrappers keep the rational arithmetic readable: |rat| builds
$a/b$, and |radd|, |rmul|, |rneg|, |rinv| are the non-mutating arithmetic we need.
Each returns a fresh |*big.Rat| so that shared coefficients are never clobbered.
@<Private subroutines@>=
func rat(a, b int64) *big.Rat     { return big.NewRat(a, b) }
func radd(x, y *big.Rat) *big.Rat { return new(big.Rat).Add(x, y) }
func rmul(x, y *big.Rat) *big.Rat { return new(big.Rat).Mul(x, y) }
func rneg(x *big.Rat) *big.Rat    { return new(big.Rat).Neg(x) }
func rinv(x *big.Rat) *big.Rat    { return new(big.Rat).Inv(x) }

@* Generators.
The simplest series come from nothing but a context. |Series| streams the given
coefficients and then zeros forever, so any polynomial is a |Series|. Notice the
shape that recurs in every producer below: a goroutine that loops on |put|,
returning the moment |put| reports the network has been cancelled.
@<Public subroutines@>=
func Series(ctx context.Context, cs ...*big.Rat) PS {
	S := mkPS(ctx)
	go func() {
		for _, c := range cs {
			if !S.put(c) {
				return
			}
		}
		for S.put(rat(0, 1)) {
		}
	}()
	return S
}

@ |Ones| is $1+x+x^2+\cdots=1/(1-x)$, and |X| is the series $x$ itself. |X| is just
a two-term |Series|, which shows how the generators compose.
@<Public subroutines@>=
func Ones(ctx context.Context) PS {
	S := mkPS(ctx)
	go func() {
		for S.put(rat(1, 1)) {
		}
	}()
	return S
}

func X(ctx context.Context) PS { return Series(ctx, rat(0, 1), rat(1, 1)) }

@ |copyPS| forwards series |I| onto an output channel |S|, one term per demand. It
is the glue that lets a process, after computing a few leading terms by hand, defer
the rest of its output to some other series.
@<Private subroutines@>=
func copyPS(I, S PS) {
	for {
		if !S.awaitReq() {
			return
		}
		v, ok := I.get()
		if !ok {
			return
		}
		if !S.send(v) {
			return
		}
	}
}

@* Term-by-term operations.
Addition is the model operator: on each demand it pulls one term from each input
and sends their sum. If either input ends (cancellation), so does the output.
@<Public subroutines@>=
func Add(F, G PS) PS {
	S := mkPS(F.ctx)
	go func() {
		for {
			if !S.awaitReq() {
				return
			}
			f, ok := F.get()
			if !ok {
				return
			}
			g, ok := G.get()
			if !ok {
				return
			}
			if !S.send(radd(f, g)) {
				return
			}
		}
	}()
	return S
}

@ Scalar multiplication |Cmul| scales every term by a constant |c|. Subtraction is
then free: $F-G=F+(-1)\,G$.
@<Public subroutines@>=
func Cmul(c *big.Rat, F PS) PS {
	S := mkPS(F.ctx)
	go func() {
		for {
			if !S.awaitReq() {
				return
			}
			f, ok := F.get()
			if !ok {
				return
			}
			if !S.send(rmul(c, f)) {
				return
			}
		}
	}()
	return S
}

func Sub(F, G PS) PS { return Add(F, Cmul(rat(-1, 1), G)) }

@ Multiplication by~$x$ shifts every coefficient up one place: it emits a~$0$ and
then copies |F|. This one-term delay is how the ``$x\cdot{}$'' factors in the
product and composition recursions are realized.
@<Public subroutines@>=
func Xmul(F PS) PS {
	S := mkPS(F.ctx)
	go func() {
		if !S.put(rat(0, 1)) {
			return
		}
		copyPS(F, S)
	}()
	return S
}

@* Calculus.
Differentiation maps $\sum F_i x^i$ to $\sum i\,F_i x^{i-1}$: drop the constant
term, then multiply the $n$th surviving coefficient by~$n$.
@<Public subroutines@>=
func Deriv(F PS) PS {
	D := mkPS(F.ctx)
	go func() {
		if !D.awaitReq() {
			return
		}
		if _, ok := F.get(); !ok { // discard the constant term
			return
		}
		for n := int64(1); ; n++ {
			f, ok := F.get()
			if !ok {
				return
			}
			if !D.send(rmul(rat(n, 1), f)) {
				return
			}
			if !D.awaitReq() {
				return
			}
		}
	}()
	return D
}

@ ${d\over dx} 1/(1-x) = 1/(1-x)^2 = 1 + 2x + 3x^2 +\ldots.$
@<Test |Deriv| routine@>=
func TestDeriv(t *testing.T) {
	checkTerms(t, "deriv(Ones)", Deriv(Ones(newCtx(t))),
		[]string{"1", "2", "3", "4", "5", "6"})
}

@ Integration is the key to the self-referential definitions. $\int F\,dx$ has
constant term~|c| (the constant of integration) and $n$th coefficient
$F_{n-1}/n$. Crucially, |Integ| {\it emits |c| before demanding anything from
|F|;\/} that one free term is what makes a feedback loop productive rather than
deadlocked.
@<Public subroutines@>=
func Integ(c *big.Rat, F PS) PS {
	I := mkPS(F.ctx)
	go func() {
		if !I.put(c) {
			return
		}
		for n := int64(1); ; n++ {
			if !I.awaitReq() {
				return
			}
			f, ok := F.get()
			if !ok {
				return
			}
			if !I.send(rmul(rat(1, n), f)) {
				return
			}
		}
	}()
	return I
}

@* Splitting a series.
Several operations need to read one series in more than one place at once --- a
product feeds both of its tails back in, a reciprocal refers to itself. A bare
channel cannot be read twice, so |Split| turns one series into |n| independent
streams, each delivering every coefficient of |F|. The branches may run at
different speeds; coefficients already produced but not yet read by the slowest
branch are held in a queue.
@<Public subroutines@>=
func Split(F PS, n int) []PS {
	ctx := F.ctx
	outs := make([]PS, n)
	demand := make(chan int)
	@<Start a forwarder goroutine for each branch@>@;
	@<Run the buffering server@>@;
	return outs
}

@ Each branch has a tiny goroutine that turns ``a demand arrived on branch~|i|''
into the message |i| on the shared |demand| channel, so the single server below can
tell {\it which\/} branch is asking.
@<Start a forwarder goroutine for each branch@>=
for i := range outs {
	outs[i] = mkPS(ctx)
	go func(i int) {
		for {
			select {
			case <-outs[i].req:
			case <-ctx.Done():
				return
			}
			select {
			case demand <- i:
			case <-ctx.Done():
				return
			}
		}
	}(i)
}

@ The server keeps a sliding |buf| of coefficients from index |base| onward, and
remembers how far each branch has read in |pos|. Once every branch has passed an
index, that prefix is dropped, so the queue holds only what some lagging branch
still needs. To |serve| branch~|i| is to hand it |buf[pos[i]-base]| and advance it,
trimming the buffer when the slowest branch moves.
@<Run the buffering server@>=
go func() {
	var buf []*big.Rat // $F$'s coefficients from index base onward
	base := 0
	pos := make([]int, n) // next index each branch will read
	var waiting []int     // branches blocked on $F$'s next term
	pulling := false
	pulled := make(chan *big.Rat)
	serve := func(i int) bool {
		if !outs[i].send(buf[pos[i]-base]) {
			return false
		}
		pos[i]++
		min := pos[0]
		for _, p := range pos[1:] {
			if p < min {
				min = p
			}
		}
		if min > base { // every branch has consumed the prefix
			buf = buf[min-base:]
			base = min
		}
		return true
	}
	@<Serve demands, fetching new terms of |F| asynchronously@>@;
}()

@ A demand for an already-buffered term is served at once. A demand that runs off
the end of the buffer must wait for |F| to produce its next coefficient --- but we
{\it must not\/} block the server to fetch it, because in a recursive definition
(|Exp|, |Recip|, |Rev|) producing that very term will circle back and demand
earlier terms through this same splitter. So the fetch happens in its own goroutine;
meanwhile the server keeps serving buffered demands, and folds the new term in when
it arrives on |pulled|.
@<Serve demands, fetching new terms of |F| asynchronously@>=
for {
	select {
	case i := <-demand:
		if pos[i]-base < len(buf) {
			if !serve(i) {
				return
			}
			continue
		}
		waiting = append(waiting, i)
		if !pulling {
			pulling = true
			go func() {
				v, ok := F.get()
				if !ok {
					return
				}
				select {
				case pulled <- v:
				case <-ctx.Done():
				}
			}()
		}
	case v := <-pulled:
		pulling = false
		buf = append(buf, v)
		w := waiting
		waiting = nil
		for _, i := range w {
			if !serve(i) {
				return
			}
		}
	case <-ctx.Done():
		return
	}
}

@* Multiplication.
Write $F=F_0+x\bar F$, splitting off the head coefficient from the tail $\bar F$.
Then the product satisfies McIlroy's equation~(2),
$$FG=F_0G_0+x\,(F_0\bar G+G_0\bar F+x\,\bar F\bar G),$$
which is genuinely recursive: the last term needs the product of the two
tails. We compute it by reading $F_0$ and $G_0$ (after which each input channel
carries its own tail), splitting both tails, and assembling the three component
series; the inner |Mul| is the recursion, and |Split| is what makes feeding the
tails back in legal.
@<Public subroutines@>=
func Mul(F, G PS) PS {
	P := mkPS(F.ctx)
	go func() {
		if !P.awaitReq() {
			return
		}
		f, ok := F.get() // $F$, $G$ now carry their tails
		if !ok {
			return
		}
		g, ok := G.get()
		if !ok {
			return
		}
		if !P.send(rmul(f, g)) { // the $F0\cdot G0$ term
			return
		}
		FF := Split(F, 2)
		GG := Split(G, 2)
		fG := Cmul(f, GG[0])
		gF := Cmul(g, FF[0])
		xFG := Xmul(Mul(FF[1], GG[1])) // the recursion
		@<Emit the sum of the three tail series@>@;
	}()
	return P
}

@ After the head, every coefficient of the product is the sum of the corresponding
coefficients of $F_0\bar G$, $G_0\bar F$, and $x\,\bar F\bar G$.
@<Emit the sum of the three tail series@>=
for {
	if !P.awaitReq() {
		return
	}
	a, ok := fG.get()
	if !ok {
		return
	}
	b, ok := gF.get()
	if !ok {
		return
	}
	c, ok := xFG.get()
	if !ok {
		return
	}
	if !P.send(radd(radd(a, b), c)) {
		return
	}
}

@* Composition and substitution.
The composition $F(G)$, where $G_0=0$ so that it converges, obeys equation~(3),
$$F(G)=F_0+x\,\bar G\,\bar F(G).$$
Reading $F_0$ leaves |F| carrying $\bar F$, and discarding the (zero) head of |G|
leaves it carrying $\bar G$; the tail of the result is then $\bar G$ times the
recursive composition.
@<Public subroutines@>=
func Subst(F, G PS) PS {
	S := mkPS(F.ctx)
	go func() {
		GG := Split(G, 2)
		if !S.awaitReq() {
			return
		}
		f, ok := F.get()
		if !ok {
			return
		}
		if !S.send(f) {
			return
		}
		if _, ok := GG[0].get(); !ok { // drop $G0$ (must be 0)
			return
		}
		copyPS(Mul(GG[0], Subst(F, GG[1])), S)
	}()
	return S
}

@ A monomial substitution $F(c\,x^n)$ is cheaper and needs no recursion: the $i$th
coefficient is scaled by $c^i$, and $n-1$ zeros are inserted after each to spread
the powers out. With $c=-1$, $n=2$ it turns $1/(1-x)$ into $1/(1+x^2)$, the seed of
the $\arctan$ example.
@<Public subroutines@>=
func Msubst(F PS, c *big.Rat, n int) PS {
	S := mkPS(F.ctx)
	go func() {
		ci := rat(1, 1)
		for {
			if !S.awaitReq() {
				return
			}
			f, ok := F.get()
			if !ok {
				return
			}
			if !S.send(rmul(ci, f)) {
				return
			}
			ci = rmul(ci, c)
			for k := 0; k < n-1; k++ {
				if !S.put(rat(0, 1)) {
					return
				}
			}
		}
	}()
	return S
}

@* Tying recursive knots.
|Fix| returns the series $X$ satisfying $X=f(X)$: it splits a fresh $X$, lets |f|
build its network from one copy, and copies the result back onto $X$. For this to
work the definition must be {\it productive\/} --- |f|'s network must deliver its
first term before it demands one --- and beginning with |Integ|, which emits the
constant of integration up front, guarantees exactly that. This is how a
differential equation becomes a stream.
@<Public subroutines@>=
func Fix(ctx context.Context, f func(PS) PS) PS {
	X := mkPS(ctx)
	XX := Split(X, 2)
	go copyPS(f(XX[0]), X)
	return XX[1]
}

@ The exponential is the running example of the paper: $e^F$ (with $F_0=0$) is the
solution of $X'=X\,F'$, i.e. the fixed point
$$X=1+\int X\,F'\,dx.$$
Because |Integ| supplies the leading~$1$ before reading anything, the loop is
productive and converges term by term --- Picard's iteration, run as dataflow.
@<Public subroutines@>=
func Exp(F PS) PS {
	D := Deriv(F)
	return Fix(F.ctx, func(X PS) PS {
		return Integ(rat(1, 1), Mul(X, D))
	})
}

@* Reciprocal and reversion.
The reciprocal $1/F$ (with $F_0\ne0$) is itself a fixed point,
$$R={1\over F_0}\,(1-x\,\bar F\,R),$$
so |R| is split and one copy is multiplied back in. Reading $F_0$ leaves |F|
carrying $\bar F$; the first term is $1/F_0$, and the rest is $-1/F_0$ times
$\bar F\,R$ (the |Xmul| is hidden in the term offset of |copyPS|).
@<Public subroutines@>=
func Recip(F PS) PS {
	R := mkPS(F.ctx)
	RR := Split(R, 2)
	go func() {
		if !R.awaitReq() {
			return
		}
		f, ok := F.get() // $F$ now carries $\bar F$
		if !ok {
			return
		}
		r0 := rinv(f)
		if !R.send(r0) {
			return
		}
		copyPS(Cmul(rneg(r0), Mul(F, RR[0])), R)
	}()
	return RR[1]
}

@ @<Test |Recip| routine@>=
func TestRecip(t *testing.T) {
	// $1/(1/(1-x)) = 1 - x$
	checkTerms(t, "recip(Ones)", Recip(Ones(newCtx(t))),
		[]string{"1", "-1", "0", "0", "0", "0"})
}

@ Reversion finds the functional inverse: given $F$ with $F_0=0$ and $F_1\ne0$,
|Rev| returns the $R$ with $F(R(x))=x$. Writing $R=x\bar R$, equation~(8) gives
$$\bar R={1\over F_1}\,\bigl(1-x\,\bar R^2\,\bar{\bar F}(R)\bigr).$$
$R$ appears three times on the right (twice in $\bar R^2$, once inside the
composition), so the network splits it four ways --- three for its own definition,
one for the caller.
@<Public subroutines@>=
func Rev(F PS) PS {
	R := mkPS(F.ctx)
	RR := Split(R, 4)
	go func() {
		if !R.put(rat(0, 1)) { // $R0 = 0$
			return
		}
		if !R.awaitReq() {
			return
		}
		if _, ok := F.get(); !ok { // drop $F0$ (must be 0)
			return
		}
		v, ok := F.get() // $F$ now carries $\bar{\bar F}$
		if !ok {
			return
		}
		f1 := rinv(v)
		if !R.send(f1) { // $R1 = 1/F1$
			return
		}
		W := Mul(Mul(tail(RR[0]), tail(RR[1])), Subst(F, RR[2])) // ${\bar R}^2 \cdot \bar{\bar F}(R)$
		c := rneg(f1)
		@<Emit the remaining coefficients of |R|@>@;
	}()
	return RR[3]
}

@ With $R_0=0$ and $R_1=1/F_1$ already sent, each further coefficient is
$-1/F_1$ times the next term of $W=\bar R^2\,\bar{\bar F}(R)$.
@<Emit the remaining coefficients of |R|@>=
for {
	if !R.awaitReq() {
		return
	}
	w, ok := W.get()
	if !ok {
		return
	}
	if !R.send(rmul(c, w)) {
		return
	}
}

@ Finally, |tail| drops the constant term of |F|, yielding $\bar F$ --- the
operation McIlroy writes as an overbar, used above to form $\bar R$ from $R$.
@<Private subroutines@>=
func tail(F PS) PS {
	T := mkPS(F.ctx)
	go func() {
		if !T.awaitReq() {
			return
		}
		if _, ok := F.get(); !ok { // discard
			return
		}
		v, ok := F.get()
		if !ok {
			return
		}
		if !T.send(v) {
			return
		}
		copyPS(F, T)
	}()
	return T
}

@ |newCtx| returns a context cancelled at the end of the test, so each
test's process network is torn down.
@<Test Helpers@>=
func newCtx(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

@ |checkTerms| compares the first coefficients of F with the expected
rationals, given as strings like "1", "-1/3".
@<Test Helpers@>=
func checkTerms(t *testing.T, name string, F PS, want []string) {
	t.Helper()
	for i, w := range want {
		got := F.Get().RatString()
		if got != w {
			t.Fatalf("%s: term %d = %s, want %s", name, i, got, w)
		}
	}
}

@* Index.
