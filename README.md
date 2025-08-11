# gopipe

A lightweight, generic **pipeline framework** with built-in **Fan-out** and **Fan-in** utilities for Go. Designed to simplify concurrent data processing with stages, workers, and error handling modes.

## Features
- **Pipeline Stages**: Chain multiple processing stages (`T -> T`) with configurable workers.
- **Error Handling**: `FailFast` or `Collect` modes.
- **Metrics**: Per-stage timing and processed item counts.
- **Fan-out**: Broadcast or round-robin distribution to multiple channels.
- **Fan-in**: Merge multiple channels into one.
- **Generic**: Works with any data type via Go generics.

## Installation
```bash
go get github.com/aminofox/gopipe
```

## Usage Example
```go
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
        if in == 8 { return 0, fmt.Errorf("bad number: %d", in) }
        return in + 1, nil
    }, gopipe.WithStageName("plus1"))

    res := pipe.Run(ctx, []int{1, 2, 3, 4, 5})
    for v := range res.Out { fmt.Println("out:", v) }

    if err := gopipe.JoinErrors(res.Errors); err != nil {
        fmt.Println("errors:", err)
    }

    for _, m := range res.DrainMetrics() {
        fmt.Printf("stage=%s processed=%d duration=%s\n", m.Name, m.ItemsProcessed, m.Duration)
    }

    // Fan-out / Fan-in
    in := make(chan int)
    go func(){ for i:=1;i<=9;i++{ in<-i }; close(in) }()
    outs := gopipe.FanOutRoundRobin[int](ctx, in, 3)
    merged := gopipe.FanIn[int](ctx, outs...)
    for v := range merged { fmt.Println("merged:", v) }
}
```

## Run Tests
```bash
go test ./gopipe -v
```

## Project Structure
```
├── go.mod
│── fan_test.go
│── fan.go
│── helpers.go
│── pipeline_test.go
│── pipeline.go
└── examples
    └── main.go
```

## License
MIT