package gopipe

import (
	"context"
	"testing"
	"time"
)

func TestFanOutAndIn(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	in := make(chan int)
	go func() {
		for i := 1; i <= 5; i++ {
			in <- i
		}
		close(in)
	}()

	outs := FanOut[int](ctx, in, 3)
	merged := FanIn[int](ctx, outs...)

	count := 0
	for range merged {
		count++
	}
	// broadcast mode: each of 5 values replicated to 3 chans => 15 merged
	if count != 15 {
		t.Fatalf("want 15 items, got %d", count)
	}
}

func TestFanOutRoundRobin(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	in := make(chan int)
	go func() {
		for i := 1; i <= 9; i++ {
			in <- i
		}
		close(in)
	}()

	outs := FanOutRoundRobin[int](ctx, in, 3)
	// Count items per channel should be balanced: 3,3,3
	counts := make([]int, 3)
	done := make(chan struct{})
	for idx, ch := range outs {
		go func(i int, c <-chan int) {
			for range c {
				counts[i]++
			}
			done <- struct{}{}
		}(idx, ch)
	}
	<-done
	<-done
	<-done

	for i := 0; i < 3; i++ {
		if counts[i] != 3 {
			t.Fatalf("channel %d got %d items, want 3", i, counts[i])
		}
	}
}
