package gopipe

import (
	"context"
	"sync"
)

func FanOut[T any](ctx context.Context, in <-chan T, n int) []<-chan T {
	if n <= 0 {
		n = 1
	}
	outs := make([]chan T, n)
	for i := range outs {
		outs[i] = make(chan T)
	}

	go func() {
		defer func() {
			for _, ch := range outs {
				close(ch)
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-in:
				if !ok {
					return
				}
				for _, ch := range outs {
					select {
					case <-ctx.Done():
						return
					case ch <- v:
					}
				}
			}
		}
	}()

	ro := make([]<-chan T, n)
	for i := range outs {
		ro[i] = outs[i]
	}
	return ro
}

func FanOutRoundRobin[T any](ctx context.Context, in <-chan T, n int) []<-chan T {
	if n <= 0 {
		n = 1
	}
	outs := make([]chan T, n)
	for i := range outs {
		outs[i] = make(chan T)
	}

	go func() {
		defer func() {
			for _, ch := range outs {
				close(ch)
			}
		}()
		i := 0
		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-in:
				if !ok {
					return
				}
				ch := outs[i%n]
				i++
				select {
				case <-ctx.Done():
					return
				case ch <- v:
				}
			}
		}
	}()

	ro := make([]<-chan T, n)
	for i := range outs {
		ro[i] = outs[i]
	}
	return ro
}

func FanIn[T any](ctx context.Context, channel ...<-chan T) <-chan T {
	out := make(chan T)
	var wg sync.WaitGroup
	wg.Add(len(channel))

	for _, ch := range channel {
		go func(c <-chan T) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case v, ok := <-c:
					if !ok {
						return
					}
					select {
					case <-ctx.Done():
						return
					case out <- v:
					}
				}
			}
		}(ch)
	}

	go func() { wg.Wait(); close(out) }()
	return out
}
