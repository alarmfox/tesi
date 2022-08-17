package pbench

import (
	"context"
)

type FCFS struct {
	out chan<- Job
	in  chan Job
}

func NewFCFS(out chan<- Job) *FCFS {
	return &FCFS{
		out: out,
		in:  make(chan Job),
	}
}

func (f *FCFS) Schedule(sr Job) error {
	f.in <- sr
	return nil
}

func (f *FCFS) Start(ctx context.Context) {
	defer close(f.in)
	for {
		select {
		case <-ctx.Done():
			return
		case r := <-f.in:
			f.out <- r
		}
	}
}
