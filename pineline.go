package gopipe

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type ErrorMode int

const (
	FailFast ErrorMode = iota
	Collect
)

type StageFn[T any] func(ctx context.Context, in T) (T, error)

type stageConfig struct {
	workers int
	name    string
}

type StageOption func(*stageConfig)

func WithWorkers(n int) StageOption {
	if n <= 0 {
		n = 1
	}
	return func(c *stageConfig) { c.workers = n }
}

func WithStageName(name string) StageOption { return func(c *stageConfig) { c.name = name } }

type stageWrap[T any] struct {
	fn   StageFn[T]
	conf stageConfig
}

type StageMetrics struct {
	Name           string
	ItemsProcessed uint64
	Duration       time.Duration
}

type Result[T any] struct {
	Out     <-chan T
	Errors  <-chan error
	metrics []StageMetrics
}

type Pipeline[T any] struct {
	stages  []stageWrap[T]
	errMode ErrorMode
	bufSize int
	metrics []StageMetrics
}

func New[T any]() *Pipeline[T] {
	return &Pipeline[T]{
		errMode: FailFast,
		bufSize: 0,
	}
}

func (p *Pipeline[T]) SetErrorMode(m ErrorMode) { p.errMode = m }
func (p *Pipeline[T]) SetBufferSize(n int)      { p.bufSize = n }

func (p *Pipeline[T]) AddStage(fn StageFn[T], opts ...StageOption) {
	conf := stageConfig{workers: runtime.GOMAXPROCS(0), name: fmt.Sprintf("stage#%d", len(p.stages))}
	for _, o := range opts {
		o(&conf)
	}
	p.stages = append(p.stages, stageWrap[T]{fn: fn, conf: conf})
}

func (p *Pipeline[T]) Run(ctx context.Context, in []T) Result[T] {
	input := make(chan T)
	go func() {
		defer close(input)
		for _, v := range in {
			select {
			case <-ctx.Done():
				return
			case input <- v:
			}
		}
	}()
	return p.RunChan(ctx, input)
}

func (p *Pipeline[T]) RunChan(ctx context.Context, in <-chan T) Result[T] {
	curr := in
	errs := make(chan error, 64)
	ctx, cancel := context.WithCancel(ctx)
	p.metrics = make([]StageMetrics, 0, len(p.stages))

	for i, s := range p.stages {
		out := make(chan T, p.bufSize)
		m := StageMetrics{Name: s.conf.name}
		var wg sync.WaitGroup
		wg.Add(s.conf.workers)
		start := time.Now()
		var processed uint64

		for w := 0; w < s.conf.workers; w++ {
			go func(stage stageWrap[T]) {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					case v, ok := <-curr:
						if !ok {
							return
						}
						res, err := stage.fn(ctx, v)
						if err != nil {
							if p.errMode == FailFast {
								select {
								case errs <- err:
								default:
								}
								cancel()
								return
							}
							select {
							case errs <- err:
							default:
							}
							continue
						}
						atomic.AddUint64(&processed, 1)
						select {
						case <-ctx.Done():
							return
						case out <- res:
						}
					}
				}
			}(s)
		}

		go func() {
			wg.Wait()
			close(out)
		}()

		// finalize metrics when this stage closes
		doneCh := make(chan struct{})
		go func(ix int, outc <-chan T) {
			for range outc { /* drained by consumer */
			}
			m.Duration = time.Since(start)
			m.ItemsProcessed = atomic.LoadUint64(&processed)
			p.metrics = append(p.metrics, m)
			close(doneCh)
		}(i, out)
		<-doneCh // ensure metrics appended in order

		curr = out
	}

	go func() {
		for range curr { /* consumer drains */
		}
		cancel()
		close(errs)
	}()

	return Result[T]{Out: curr, Errors: errs, metrics: p.metrics}
}

func (r Result[T]) Drain() ([]T, error) {
	var out []T
	for v := range r.Out {
		out = append(out, v)
	}
	return out, FirstError(r.Errors)
}

func (r Result[T]) DrainMetrics() []StageMetrics { return r.metrics }
