package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/alarmfox/tesi/internal/pbench"
	"golang.org/x/sync/errgroup"
)

var (
	addr      = flag.String("listen-addr", "127.0.0.1:8000", "Listen address for TCP server")
	scheduler = flag.String("scheduler", "fcfs", "Scheduler algorithm to be used")
	arraySize = flag.Int("array-size", 100, "Size of the buffer to be used")
)

type Config struct {
	addr      string
	scheduler string
	arraySize int
}

func main() {
	flag.Parse()

	c := Config{
		addr:      *addr,
		scheduler: *scheduler,
		arraySize: *arraySize,
	}

	if err := run(c); err != nil {
		log.Fatal(err)
	}
}

func run(c Config) error {
	jobs := make(chan pbench.Job)
	var scheduler pbench.Scheduler
	switch strings.ToLower(c.scheduler) {
	case "fcfs":
		scheduler = pbench.NewFCFS(jobs)
	case "drr":
		drr := pbench.NewDRR(jobs)
		drr.Input(1)
		drr.Input(2)
		scheduler = drr
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Kill, syscall.SIGTERM)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		<-ctx.Done()
		close(jobs)
		return nil
	})

	g.Go(func() error {
		scheduler.Start(ctx)
		return nil
	})

	g.Go(func() error {
		server := pbench.NewServer(scheduler)
		return server.Start(ctx, c.addr)
	})

	g.Go(func() error {
		buffer := pbench.NewBuffer(c.arraySize)
		var response pbench.Response
		var err error
		for job := range jobs {
			switch job.Request.Type {
			case pbench.SlowRequest:
				err = buffer.Slow(job.Request.Payload, job.Request.Offset)
				if err != nil {
					response.Error = -1
				} else {
					response.Error = 0
				}
			case pbench.FastRequest:
				n, err := buffer.Fast(job.Request.Offset)
				if err != nil {
					response.Error = -1
				} else {
					response.Error = 0
				}
				response.Result = n
			}
			b, err := json.Marshal(response)
			if err != nil {
				log.Print(err)
			} else if _, err := job.Client.Write(b); err != nil {
				log.Print(err)
			}
			job.Client.Close()
		}
		return nil
	})

	return g.Wait()
}
