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
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alarmfox/tesi/internal/pbench"
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
	arraySize           = flag.Int("array-size", 100, "Size of the buffer to be used")
	resultFile          = flag.String("write", "result.txt", "File path to write result")
)

type Config struct {
	addr                string
	resultFile          string
	nRequest            int
	arraySize           int
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
		arraySize:           *arraySize,
		resultFile:          *resultFile,
		concurrency:         *concurrency,
	}

	log.Printf("%+v", c)
	if err := run(c); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}

}

type Result struct {
	ResidenceTime int64
	WaitingTime   int64
	ExecutionTime int64
}

func run(c Config) error {
	ctx, canc := signal.NotifyContext(context.Background(), os.Kill, syscall.SIGTERM)

	errs := make(chan error)
	defer close(errs)

	rand.Seed(time.Now().Unix())
	jobs := make(chan pbench.Request, 2)

	nSlowRequest := math.Floor(float64(c.nRequest) * c.slowRequestPercent / 100)
	go func() {
		ticker := time.NewTicker(c.slowRequestInterval)
		defer ticker.Stop()

		for i := 0; i < int(nSlowRequest); i += 1 {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				jobs <- pbench.Request{
					Type:    pbench.SlowRequest,
					Payload: rand.Intn(math.MaxInt),
					Offset:  rand.Intn(c.arraySize),
				}
			}
		}
	}()

	nFastRequest := *nRequest - int(nSlowRequest)
	go func() {
		ticker := time.NewTicker(c.fastRequestInterval)
		defer ticker.Stop()

		for i := 0; i < nFastRequest; i += 1 {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				jobs <- pbench.Request{
					Type:   pbench.FastRequest,
					Offset: rand.Intn(c.arraySize),
				}

			}
		}
	}()

	go func() {
		<-ctx.Done()
		close(jobs)
	}()

	results := make(chan Result)
	defer close(results)

	sentRequests := make(chan struct{}, c.nRequest)
	defer close(sentRequests)

	pool := pbench.NewPool(func() []byte { b := make([]byte, bufferSize); return b })
	conns, err := pbench.CreateTcpConnPool(&pbench.TcpConfig{
		Address:      c.addr,
		MaxIdleConns: 512,
		MaxOpenConn:  10000,
	})

	if err != nil {
		return err
	}
	for i := 0; i < int(c.concurrency); i++ {
		go func() {
			var err error
			for job := range jobs {
				err = func() error {
					defer func() {
						sentRequests <- struct{}{}
					}()
					conn, err := conns.Get()

					if err != nil {
						return err
					}
					defer conns.Put(conn)

					if err := json.NewEncoder(conn).Encode(job); err != nil {
						return fmt.Errorf("encoding error: %v", err)
					}

					buffer := pool.Get()
					defer pool.Put(buffer)

					n, err := conn.Read(buffer)
					if err != nil {
						return fmt.Errorf("read error: %v", err)
					}

					var response pbench.Response
					if err := json.NewDecoder(bytes.NewReader(buffer[:n])).Decode(&response); err != nil {
						return fmt.Errorf("decoding error: %v", err)
					}

					results <- Result{
						ResidenceTime: response.Info.FinishedTs - response.Info.AcceptedTs,
						WaitingTime:   response.Info.RunningTs - response.Info.AcceptedTs,
						ExecutionTime: response.Info.FinishedTs - response.Info.RunningTs,
					}
					return nil
				}()
				if err != nil {
					log.Print(err)
				}

			}
		}()
	}

	go func() {
		f, err := os.Create(c.resultFile)
		if err != nil {
			errs <- err
		}
		defer f.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case result := <-results:
				_, err := fmt.Fprintf(f, "%d;%d;%d\n",
					result.ResidenceTime,
					result.WaitingTime,
					result.ExecutionTime,
				)
				if err != nil {
					log.Print(err)
					continue
				}

			}
		}
	}()

	n := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errs:
			return err
		case <-sentRequests:
			n += 1
			if n == c.nRequest {
				canc()
			}
		}
	}

}
