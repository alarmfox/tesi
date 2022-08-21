package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alarmfox/tesi/internal/pbench"
	"golang.org/x/sync/errgroup"
)

const (
	bufferSize = 4096
)

var (
	addr                = flag.String("server-addr", "127.0.0.1:8000", "Address for TCP server")
	nRequest            = flag.Int("n-request", 100, "Number of request")
	concurrency         = flag.Uint("concurrency", 1, "Number of requests to be sent concurrently")
	slowRequestPercent  = flag.Float64("slow-request-percent", 50, "Percent of slow request")
	slowRequestInterval = flag.Duration("slow-request-interval", 1000*time.Microsecond, "Period to send slow request")
	fastRequestInterval = flag.Duration("fast-request-interval", time.Microsecond, "Period to send fast request")
	resultFile          = flag.String("write", "result.txt", "File path to write result")
)

type Config struct {
	addr                string
	resultFile          string
	nRequest            int
	concurrency         uint
	slowRequestPercent  float64
	slowRequestInterval time.Duration
	fastRequestInterval time.Duration
}

func main() {
	flag.Parse()

	c := Config{
		addr:                *addr,
		nRequest:            *nRequest,
		slowRequestPercent:  *slowRequestPercent,
		slowRequestInterval: *slowRequestInterval,
		fastRequestInterval: *fastRequestInterval,
		resultFile:          *resultFile,
		concurrency:         *concurrency,
	}

	log.Printf("%+v", c)
	if err := run(c); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}

}

type Result struct {
	Request       pbench.RequestType
	ResidenceTime int64
	WaitingTime   int64
	RTT           int64
}

func run(c Config) error {
	ctx, canc := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer canc()
	g, ctx := errgroup.WithContext(ctx)

	jobs := make(chan pbench.Request, c.nRequest)

	done := make(chan struct{})

	nSlowRequest := math.Floor(float64(c.nRequest) * c.slowRequestPercent / 100)
	g.Go(func() error {
		ticker := time.NewTicker(c.slowRequestInterval)
		defer func() {
			ticker.Stop()
			done <- struct{}{}
		}()
		for i := 0; i < int(nSlowRequest); i += 1 {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				jobs <- pbench.Request{
					Type: pbench.SlowRequest,
				}
			}
		}
		return nil
	})

	g.Go(func() error {
		nFastRequest := c.nRequest - int(nSlowRequest)
		ticker := time.NewTicker(c.fastRequestInterval)
		defer func() {
			ticker.Stop()
			done <- struct{}{}
		}()
		for i := 0; i < nFastRequest; i += 1 {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				jobs <- pbench.Request{
					Type: pbench.FastRequest,
				}

			}
		}
		return nil
	})

	results := make(chan Result, c.nRequest)
	defer close(results)

	pool := pbench.NewPool(func() []byte { b := make([]byte, bufferSize); return b })
	conns, err := pbench.CreateTcpConnPool(&pbench.TcpConfig{
		Address:      c.addr,
		MaxIdleConns: 1024,
		MaxOpenConn:  3072,
	})

	if err != nil {
		return err
	}
	defer conns.Close()

	sentRequest := make(chan struct{}, c.nRequest)
	defer close(sentRequest)

	for i := 0; i < int(c.concurrency); i++ {
		g.Go(func() error {
			var err error
			for job := range jobs {
				err = func() error {
					defer func() {
						sentRequest <- struct{}{}
					}()
					start := time.Now()

					conn, err := conns.Get()

					if err != nil {
						return err
					}

					defer conns.Put(conn)

					g.Go(func() error {
						<-ctx.Done()
						conn.SetDeadline(time.Now())
						return nil
					})

					if err := json.NewEncoder(conn).Encode(job); err != nil {
						return err
					}

					buffer := pool.Get()
					defer pool.Put(buffer)

					n, err := conn.Read(buffer)
					if err != nil {
						return err
					}

					var response pbench.Response
					if err := json.NewDecoder(bytes.NewReader(buffer[:n])).Decode(&response); err != nil {
						return err
					}

					results <- Result{
						Request:       job.Type,
						ResidenceTime: response.FinishedTs - response.AcceptedTs,
						WaitingTime:   response.RunningTs - response.AcceptedTs,
						RTT:           time.Since(start).Microseconds(),
					}
					return nil
				}()
				if errors.Is(err, os.ErrDeadlineExceeded) {
					return nil
				} else if err != nil {
					log.Print(err)
				}
			}
			return nil
		})
	}

	g.Go(func() error {
		f, err := os.Create(c.resultFile)
		if err != nil {
			return err
		}
		defer f.Close()
		for {
			select {
			case <-ctx.Done():
				return nil
			case result := <-results:
				_, err := fmt.Fprintf(f, "%d;%d;%d;%d\n",
					result.Request,
					result.ResidenceTime,
					result.WaitingTime,
					result.RTT,
				)
				if err != nil {
					log.Print(err)
					continue
				}

			}
		}
	})

	g.Go(func() error {
		defer close(jobs)
		for i := 0; i < 2; i++ {
			<-done
		}
		return nil
	})

	g.Go(func() error {
		defer canc()
		n := 0
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-sentRequest:
				n += 1
				if n == c.nRequest {
					return nil
				}
			}
		}
	})

	return g.Wait()

}
