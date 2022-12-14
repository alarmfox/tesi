package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/alarmfox/tesi/internal/pbench"
	"github.com/shirou/gopsutil/load"
	"golang.org/x/sync/errgroup"
)

var (
	listenAddress = flag.String("listen-address", "127.0.0.1:8000", "Listen address for TCP server")
	scheduler     = flag.String("scheduler", "", "Scheduler algorithm to be used")
	slowTime      = flag.Duration("slow-time", time.Millisecond, "Time to sleep in slow requests")
)

type Config struct {
	listenAddress string
	scheduler     string
	slowTime      time.Duration
}

func main() {
	flag.Parse()

	c := Config{
		listenAddress: *listenAddress,
		scheduler:     *scheduler,
		slowTime:      *slowTime,
	}

	if err := run(c); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}

func run(c Config) error {

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		<-ctx.Done()
		cancel()
		return ctx.Err()
	})

	jobs := make(chan pbench.Job)
	hiPrio := make(chan pbench.Job)
	loPrio := make(chan pbench.Job)

	defer close(loPrio)
	defer close(hiPrio)

	var isDRR bool
	g.Go(func() error {

		defer close(jobs)
		switch strings.ToLower(c.scheduler) {
		case "fcfs":
			isDRR = false
			scheduler := pbench.NewFCFS(hiPrio, jobs)
			return scheduler.Start(ctx)
		case "drr":
			isDRR = true
			scheduler, err := pbench.NewDRR(jobs)
			if err != nil {
				return err
			}
			scheduler.Input(3, hiPrio)
			scheduler.Input(2, loPrio)
			return scheduler.Start(ctx)
		default:
			return fmt.Errorf("unsupported scheduler: %q", c.scheduler)
		}
	})

	g.Go(func() error {
		server := pbench.NewServer(hiPrio, loPrio, isDRR)
		return server.Start(ctx, c.listenAddress)
	})

	g.Go(func() error {
		buffer := pbench.NewBuffer(c.slowTime)
		var memory runtime.MemStats
		for job := range jobs {
			job.Response.RunningTs = time.Now()
			switch job.Request {
			case pbench.SlowRequest:
				buffer.Slow()
			case pbench.FastRequest:
				buffer.Fast()
			}
			runtime.ReadMemStats(&memory)
			job.Response.Memory = memory.Sys
			job.Response.FinishedTs = time.Now()

			if cpuAvg, err := load.Avg(); err != nil {
				log.Print(err)
			} else {
				job.Response.CPU = cpuAvg.Load1
			}

			if err := json.NewEncoder(job.Client).Encode(job.Response); err != nil {
				log.Printf("response: %v", err)
			}

		}
		return nil
	})

	return g.Wait()
}
