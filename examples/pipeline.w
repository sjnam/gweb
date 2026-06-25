\def\title{A Hybrid Pipeline}

@s Seq int
@s Context int
@s WaitGroup int
@s Once int
@s CancelFunc int

@* Introduction.
Go gives us two rather different ways to build a {\it pipeline}, and they each shine
at a different job.

The first is the {\it sequential, lazy\/} world of {\bf iterators}. Since Go~1.23 a
function can be ranged over with |for v := range s|, and a chain of |map| and
|filter| stages composes into a single pull-driven loop --- no buffering, no
goroutines, nothing computed until the consumer asks for it. It is wonderfully
simple to reason about, but it runs on one thread.

The second is the {\it concurrent\/} world of {\bf channels} and {\bf goroutines}. Here we
can {\it fan out\/} a stage across several workers running at once, cancel the whole
network the instant something goes wrong, and let the first error propagate out. It
is powerful, but every stage now has to think about cancellation and shutdown.

Real programs want both: cheap sequential transforms before and after an expensive
step that deserves parallelism. This example is a small, complete tour of how to
join the two worlds with {\it two little boundary adapters}, so a value can flow
$$\hbox{count}\to\hbox{map}\to\hbox{filter}\;\Longrightarrow\;
  \hbox{fan out to workers}\;\Longrightarrow\;
  \hbox{filter}\to\hbox{collect},$$
where the single arrows are lazy iterator stages and the double arrows are the two
crossings between the iterator world and the channel world. Cancellation threads all
the way through: if a worker fails, the upstream feeder and the downstream consumer
both stop promptly.

@ Here is the whole program in outline. We meet the sequential stages first, then a
pocket-sized error group, the two adapters that bridge the worlds, the parallel
stage itself, a toy ``scoring'' task to run through it, and finally the assembly.
@c
package main

import (
	"context"
	"fmt"
	"iter"
	"sort"
	"sync"
	"time"
)

@<Sequential transforms over iterators@>@;
@<A pocket error group@>@;
@<The two boundary adapters@>@;
@<The parallel fan-out stage@>@;
@<The example domain@>@;
@<Running one pass of the pipeline@>@;
@<Entry point@>@;

@* The sequential world.
An |iter.Seq[V]| is just a function that, given a |yield| callback, calls it once
per element and stops early if |yield| returns false. |count| is the simplest
producer: the integers $0,1,\ldots,n-1$. The early-exit check is what lets a
downstream stage break the whole chain.
@<Sequential transforms over iterators@>=
func count(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

@ |mapSeq| and |filterSeq| are the classic transformers, written once for every
element type thanks to generics. Each wraps a source sequence in a new one; nothing
runs until the final consumer ranges over the result, and a |false| from |yield|
unwinds cleanly back through every stage. Note how |filterSeq| only forwards an
element that passes |keep|, and stops if |yield| asks it to.
@<Sequential transforms over iterators@>=
func mapSeq[A, B any](s iter.Seq[A], f func(A) B) iter.Seq[B] {
	return func(yield func(B) bool) {
		for v := range s {
			if !yield(f(v)) {
				return
			}
		}
	}
}

func filterSeq[A any](s iter.Seq[A], keep func(A) bool) iter.Seq[A] {
	return func(yield func(A) bool) {
		for v := range s {
			if keep(v) && !yield(v) {
				return
			}
		}
	}
}

@* A pocket error group.
The parallel stage needs three things from its workers: wait for them all, cancel
everybody the moment one fails, and remember that first error. The standard library
package \.{golang.org/x/sync/errgroup} does exactly this, but to keep the example
dependency-free we reproduce its essence in a dozen lines. (In real code, import
\.{errgroup} and delete this section.)

|withContext| returns a group and a child context; the group holds the |cancel|
that ends it.
@<A pocket error group@>=
type errGroup struct {
	wg     sync.WaitGroup
	once   sync.Once
	cancel context.CancelFunc
	err    error
}

func withContext(ctx context.Context) (*errGroup, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	return &errGroup{cancel: cancel}, ctx
}

@ |Go| launches a worker. If it returns an error, |once| ensures that only the very
first failure is recorded and only it cancels the context --- later failures (often
just |ctx.Err()| from the cancellation itself) are ignored. |Wait| blocks for all
workers, then cancels so nothing is left running, and reports the saved error.
(|wg.Go| is the Go~1.25 shorthand for ``add one, run the function, mark done.'')
@<A pocket error group@>=
func (g *errGroup) Go(f func() error) {
	g.wg.Go(func() {
		if err := f(); err != nil {
			g.once.Do(func() { g.err = err; g.cancel() })
		}
	})
}

func (g *errGroup) Wait() error {
	g.wg.Wait()
	g.cancel()
	return g.err
}

@* Crossing between the worlds.
Two adapters are the whole trick. |seqToChan| goes from the sequential world to the
concurrent one: a goroutine pulls the iterator and pushes each value onto a channel,
giving up the moment the context is cancelled. This is the feeder that hands work to
the parallel stage.
@<The two boundary adapters@>=
func seqToChan[V any](ctx context.Context, s iter.Seq[V]) <-chan V {
	ch := make(chan V)
	go func() {
		defer close(ch)
		for v := range s {
			select {
			case ch <- v:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

@ |chanToSeq| goes the other way, presenting a channel as a cancellable
|iter.Seq| --- the ``or-done'' pattern. The returned sequence yields values as they
arrive, and ends when either the channel closes or the |done| signal fires, so the
downstream sequential transforms can simply range over it.
@<The two boundary adapters@>=
func chanToSeq[V any](done <-chan struct{}, c <-chan V) iter.Seq[V] {
	return func(yield func(V) bool) {
		for {
			select {
			case <-done:
				return
			case v, ok := <-c:
				if !ok || !yield(v) {
					return
				}
			}
		}
	}
}

@* The parallel stage.
|parallelMap| is the expensive middle of the pipeline. It starts |workers|
goroutines that all range over the same input channel |in| --- that shared receive
is the {\it fan-out}: whichever worker is free grabs the next job. Each applies
|work| and forwards the result on |out|, and the error group ties their fates
together.
@<The parallel fan-out stage@>=
func parallelMap[A, B any](
	ctx context.Context, workers int, in <-chan A,
	work func(context.Context, A) (B, error),
) (<-chan B, <-chan error) {
	out := make(chan B)
	errc := make(chan error, 1)
	g, ctx := withContext(ctx)
	@<Launch the workers@>@;
	@<Close the outputs once the workers finish@>@;
	return out, errc
}

@ A worker loops until |in| is drained. If |work| fails it returns the error, which
makes the group cancel |ctx|; the |select| on the send means a worker also stops at
once when some {\it other\/} worker has triggered cancellation, rather than blocking
forever on |out|.
@<Launch the workers@>=
for range workers {
	g.Go(func() error {
		for a := range in {
			b, err := work(ctx, a)
			if err != nil {
				return err
			}
			select {
			case out <- b:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})
}

@ A single closer goroutine waits for every worker, then publishes the group's
error on the buffered |errc| and closes both channels. Closing |out| is what lets
the downstream |chanToSeq| range loop terminate normally on success.
@<Close the outputs once the workers finish@>=
go func() {
	errc <- g.Wait()
	close(out)
	close(errc)
}()

@* The example task.
To have something to run, we score ``documents'' by id. |score| stands in for a
piece of cancellable I/O: it waits 100\thinspace ms (but abandons the wait if the
context is cancelled), fails on one chosen id to demonstrate error propagation, and
otherwise returns a small deterministic score.
@<The example domain@>=
type Result struct {
	ID    int
	Score int
}

func score(ctx context.Context, id, failOn int) (Result, error) {
	select {
	case <-time.After(100 * time.Millisecond):
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
	if id == failOn {
		return Result{}, fmt.Errorf("doc %d: fetch failed", id)
	}
	return Result{ID: id, Score: (id * id) % 97}, nil
}

@* Putting it together.
|run| wires the five stages into one pipeline and consumes it. A cancellable
context governs the whole run; the deferred |cancel| guarantees that if we ever
leave early, the feeder and the workers are torn down too.
@<Running one pass of the pipeline@>=
func run(label string, failOn int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	start := time.Now()

	@<Build the input sequence@>@;
	@<Fan out through the scorer@>@;
	@<Filter and collect the results@>@;
	@<Report what happened@>@;
}

@ The source is purely sequential: the ids $1\ldots12$ with the multiples of five
removed. Being an |iter.Seq|, nothing here runs yet --- it is a recipe the feeder
will pull on.
@<Build the input sequence@>=
ids := filterSeq(mapSeq(count(12), func(i int) int { return i + 1 }),
	func(i int) bool { return i%5 != 0 })

@ Now cross into the concurrent world: |seqToChan| turns the recipe into a stream of
jobs, and |parallelMap| scores them four at a time.
@<Fan out through the scorer@>=
jobs := seqToChan(ctx, ids)
out, errc := parallelMap(ctx, 4, jobs, func(ctx context.Context, id int) (Result, error) {
	return score(ctx, id, failOn)
})

@ Crossing back, |chanToSeq| presents the results as a sequence again, so a final
sequential |filterSeq| keeps only the high scores. Ranging over |kept| is what
actually drives the entire machine --- pulling here pulls jobs through the workers,
which pulls values from the feeder, which pulls the source iterator. Results arrive
in worker-completion order, so we sort before reporting.
@<Filter and collect the results@>=
kept := filterSeq(chanToSeq(ctx.Done(), out), func(r Result) bool { return r.Score >= 20 })

var got []Result
for r := range kept {
	got = append(got, r)
}
err := <-errc

sort.Slice(got, func(i, j int) bool { return got[i].ID < got[j].ID })

@ @<Report what happened@>=
fmt.Printf("[%s] elapsed=%v  err=%v\n", label, time.Since(start).Round(10*time.Millisecond), err)
fmt.Printf("    kept %d results: %v\n", len(got), got)

@* Two runs.
The happy path lets every document succeed: with four workers and about nine jobs
the scoring takes roughly three rounds of 100\thinspace ms. The cancel path makes
document~7 fail; its error cancels the context, the other workers and the feeder
stop where they are, and the run ends early --- a first-error shutdown rippling
across both worlds.
@<Entry point@>=
func main() {
	run("happy ", -1)
	run("cancel", 7)
}

@* Index.
