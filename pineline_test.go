package gopipe

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
)

func TestPipeline_Basic(t *testing.T) {
	ctx := context.Background()
	p := New[int]()
	p.SetBufferSize(8)
	p.AddStage(func(ctx context.Context, in int) (int, error) { return in * 2, nil }, WithStageName("double"), WithWorkers(2))
	p.AddStage(func(ctx context.Context, in int) (int, error) { return in + 1, nil }, WithStageName("plus1"))

	res := p.Run(ctx, []int{1, 2, 3, 4})
	out, err := res.Drain()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	slices.Sort(out)
	expected := []int{3, 5, 7, 9}
	if !slices.Equal(out, expected) {
		t.Fatalf("got %v want %v", out, expected)
	}
}

func TestPipeline_FailFast(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	p := New[int]()
	p.SetErrorMode(FailFast)
	p.AddStage(func(ctx context.Context, in int) (int, error) { return in, nil })
	p.AddStage(func(ctx context.Context, in int) (int, error) {
		if in == 3 {
			return 0, errors.New("boom")
		}
		return in, nil
	})

	in := []int{1, 2, 3, 4, 5, 6}
	res := p.Run(ctx, in)
	// Expect at least one error and early cancel; outputs may be partial
	if err := FirstError(res.Errors); err == nil {
		t.Fatal("expected an error in FailFast mode")
	}
}

func TestPipeline_Collect(t *testing.T) {
	ctx := context.Background()
	p := New[int]()
	p.SetErrorMode(Collect)
	p.AddStage(func(ctx context.Context, in int) (int, error) {
		if in%2 == 0 {
			return 0, fmt.Errorf("bad %d", in)
		}
		return in * 10, nil
	})

	res := p.Run(ctx, []int{1, 2, 3, 4, 5})
	var outs []int
	for v := range res.Out {
		outs = append(outs, v)
	}
	if len(outs) != 3 {
		t.Fatalf("expected 3 outputs, got %d", len(outs))
	}
	if err := JoinErrors(res.Errors); err == nil {
		t.Fatalf("expected aggregated errors")
	}
}

func TestPipeline_Metrics(t *testing.T) {
	ctx := context.Background()
	p := New[int]()
	p.AddStage(func(ctx context.Context, in int) (int, error) { return in + 1, nil }, WithStageName("inc"))
	res := p.Run(ctx, []int{1, 2, 3})
	for range res.Out { /* drain */
	}
	_ = JoinErrors(res.Errors)
	ms := res.DrainMetrics()
	if len(ms) != 1 {
		t.Fatalf("want 1 stage metrics, got %d", len(ms))
	}
	if ms[0].ItemsProcessed == 0 {
		t.Fatalf("metrics ItemsProcessed should be > 0")
	}
}
