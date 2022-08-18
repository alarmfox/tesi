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
	"net"
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
	concurrency         = flag.Int("concurrency", 1, "Number of request to send in parallel")
	nRequest            = flag.Int("n-request", 100, "Number of request")
	slowRequestPercent  = flag.Float64("slow-request-percent", 50, "Percent of slow request")
	slowRequestInterval = flag.Duration("slow-request-interval", 10*time.Microsecond, "Period to send slow request")
	fastRequestInterval = flag.Duration("fast-request-interval", time.Microsecond, "Period to send fast request")
	arraySize           = flag.Int("array-size", 100, "Size of the buffer to be used")
	resultFile          = flag.String("write", "result.txt", "File path to write result")
)

type Config struct {
	addr                string
	resultFile          string
	nRequest            int
	arraySize           int
	concurrency         int
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
	Elapsed time.Duration
}

func run(c Config) error {
	ctx, canc := signal.NotifyContext(context.Background(), os.Kill, syscall.SIGTERM)

	errs := make(chan error)
	defer close(errs)

	rand.Seed(time.Now().Unix())
	jobs := make(chan pbench.Request, c.concurrency)

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
	for i := 0; i < c.concurrency; i++ {
		go func() {
			var err error
			for job := range jobs {
				err = func() error {
					defer func() {
						sentRequests <- struct{}{}
					}()
					start := time.Now()
					conn, err := net.Dial("tcp4", c.addr)
					if err != nil {
						return fmt.Errorf("dial error: %v", err)
					}
					defer conn.Close()

					err = json.NewEncoder(conn).Encode(job)
					if err != nil {
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
					elapsed := time.Since(start)
					results <- Result{elapsed}
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
				_, err := fmt.Fprintf(f, "%d\n", result.Elapsed.Microseconds())
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
