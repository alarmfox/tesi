package pbench

import (
	"context"
)

type FCFS struct {
	out chan<- Job
	in  <-chan Job
}

func NewFCFS(in <-chan Job, out chan<- Job) *FCFS {
	return &FCFS{
		out: out,
		in:  in,
	}
}

func (f *FCFS) Start(ctx context.Context) error {
	defer close(f.out)
	for {
		select {
		case <-ctx.Done():
			return nil
		case r := <-f.in:
			f.out <- r
		}
	}
}
