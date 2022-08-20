package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"github.com/alarmfox/tesi/internal/pbench"
	"golang.org/x/sync/errgroup"
)

var (
	addr       = flag.String("listen-addr", "127.0.0.1:8000", "Listen address for TCP server")
	scheduler  = flag.String("scheduler", "fcfs", "Scheduler algorithm to be used")
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
)

type Config struct {
	addr      string
	scheduler string
}

func main() {
	flag.Parse()

	c := Config{
		addr:      *addr,
		scheduler: *scheduler,
	}

	log.Printf("%+v", c)
	if err := run(c); err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}

func run(c Config) error {

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}

		log.Print("starting cpu profile")
		defer func() {
			log.Print("stopping cpu profile")
			pprof.StopCPUProfile()
		}()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		<-ctx.Done()
		cancel()
		return ctx.Err()
	})

	jobs := make(chan pbench.Job)
	requests := make(chan pbench.Job)
	lo := make(chan pbench.Job)

	defer close(lo)
	defer close(requests)

	var isDRR bool
	g.Go(func() error {

		switch strings.ToLower(c.scheduler) {
		case "fcfs":
			isDRR = false
			scheduler := pbench.NewFCFS(requests, jobs)
			return scheduler.Start(ctx)
		case "drr":
			isDRR = true
			scheduler, err := pbench.NewDRR(jobs)
			if err != nil {
				return err
			}
			scheduler.Input(3, requests)
			scheduler.Input(2, lo)
			return scheduler.Start(ctx)
		default:
			return fmt.Errorf("unsupported scheduler: %s", c.scheduler)
		}
	})

	g.Go(func() error {
		return pbench.NewServer(requests, lo, isDRR).Start(ctx, c.addr)
	})

	g.Go(func() error {

		buffer := pbench.NewBuffer()
		for job := range jobs {
			job.Response.RunningTs = time.Now().UnixMicro()
			switch job.Request.Type {
			case pbench.SlowRequest:
				buffer.Slow()
			case pbench.FastRequest:
				buffer.Fast()
			}
			job.Response.FinishedTs = time.Now().UnixMicro()
			err := json.NewEncoder(job.Client).Encode(job.Response)
			if err != nil {
				log.Printf("response: %v", err)
			}
		}
		return nil
	})

	return g.Wait()
}
