@* First Error Cancellation.
Runs 10 goroutines in parallel where each finishes after a random delay (1–9999 ms).
If any goroutine encounters an error, it signals all others to stop immediately
via context cancellation.
@c
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

func main() {
    var wg sync.WaitGroup

    @<Make |errChan| channels@>

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    wg.Add(10)
    for i := range 10 {
        @<Create a goroutine that finishes after a random delay@>
    }

    done := make(chan struct{})
    go func() {
		defer close(done)
		for err := range errChan {
			if err != nil {
				cancel()
				// Drain the channel fully instead of returning early to avoid missing errors
			}
		}
	}()

    @<Close after |wg.Wait()| @>
}

@ Make buffer size 10 channels. All goroutines can send without blocking even with no receiver.
@<Make |err...@>=
errChan := make(chan error, 10)

@ Create and Runs 10 goroutines in parallel where each finishes after a random
delay (1–9999 ms). It simulates an error occurring at any time while the goroutine
is running and sends a signal to the error channel.

Goroutines that had not yet fired their ticker print \.{"context canceled"};
those that completed normally before the error print \.{"finished"};
the goroutine that caused the cancellation prints its error.
@<Create a ...@>=
go func() {
	eStr := "finished"
	start := time.Now()

	defer func() {
		log.Printf("goroutine[%d] %v %s", i, time.Since(start), eStr)
		wg.Done()
	}()

	// Use NewTicker instead of time.Tick() to prevent goroutine leak
	t := time.NewTicker(time.Duration(1+rand.Intn(9999)) * time.Millisecond)
	defer t.Stop()

	select {
	case <-t.C:
		// Decide whether to report an error after the actual work completes
		var err error
		if rand.Intn(4) == 0 {
			err = fmt.Errorf("ERROR[%d]", i)
		}
		errChan <- err
	case <-ctx.Done():
		eStr = ctx.Err().Error()
	}
}()

@ Close |errChan| channel after |wg.Wait()| ensures all sends are done.
Wait for consumer to finish draining
@<Close...@>=
wg.Wait()
close(errChan) // Close after wg.Wait() ensures all sends are done
<-done         // Wait for consumer to finish draining

@* Index.
