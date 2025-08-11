package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aminofox/gopipe"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pipe := gopipe.New[int]()
	pipe.SetErrorMode(gopipe.Collect)
	pipe.SetBufferSize(16)

	pipe.AddStage(func(ctx context.Context, in int) (int, error) { return in * 2, nil }, gopipe.WithStageName("double"), gopipe.WithWorkers(4))
	pipe.AddStage(func(ctx context.Context, in int) (int, error) {
		if in == 8 {
			return 0, fmt.Errorf("bad number: %d", in)
		}
		return in + 1, nil
	}, gopipe.WithStageName("plus1"))

	res := pipe.Run(ctx, []int{1, 2, 3, 4, 5})
	for v := range res.Out {
		fmt.Println("out:", v)
	}
	if err := gopipe.JoinErrors(res.Errors); err != nil {
		fmt.Println("errors:", err)
	}
	for _, m := range res.DrainMetrics() {
		fmt.Printf("stage=%s processed=%d duration=%s", m.Name, m.ItemsProcessed, m.Duration)
	}

	in := make(chan int)
	go func() {
		for i := 1; i <= 9; i++ {
			in <- i
		}
		close(in)
	}()
	outs := gopipe.FanOutRoundRobin(ctx, in, 3)
	merged := gopipe.FanIn(ctx, outs...)
	for v := range merged {
		fmt.Println("merged:", v)
	}
}
