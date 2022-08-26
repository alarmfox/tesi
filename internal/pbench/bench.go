package pbench

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"log"
	"math"
	"os"
	"time"

	"golang.org/x/sync/errgroup"
	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/gonum/stat/distuv"
)

type requestResult struct {
	Request       Request
	ResidenceTime time.Duration
	WaitingTime   time.Duration
	RoundTripTime time.Duration
}

type BenchResult struct {
	FastLambda      float64
	SlowLambda      float64
	TotRequests     int
	SlowRequestLoad int
	AverageSlowRt   float64
	AverageSlowWt   float64
	AverageSlowRtt  float64
	AverageFastRt   float64
	AverageFastWt   float64
	AverageFastRtt  float64
}

type BenchConfig struct {
	Algorithm       string
	ServerAddress   string
	TotRequests     int
	Concurrency     int
	SlowRequestLoad int
	SlowLambda      float64
	FastLambda      float64
	MaxIdleConns    int
	MaxOpenConns    int
	TimeUnit        time.Duration
}

func Bench(ctx context.Context, c BenchConfig) (BenchResult, error) {

	conns, err := createTcpConnPool(&tcpConfig{
		Address:      c.ServerAddress,
		MaxIdleConns: c.MaxIdleConns,
		MaxOpenConn:  c.MaxOpenConns,
	})

	if err != nil {
		return BenchResult{}, err
	}
	defer conns.close()

	g, ctx := errgroup.WithContext(ctx)

	jobs := make(chan Request, c.TotRequests)
	results := make(chan requestResult, c.TotRequests)
	doneSendingJobs := make(chan struct{}, 2)
	doneSendingResult := make(chan struct{}, c.Concurrency)
	terminationSignal := make(chan struct{})
	defer close(doneSendingJobs)
	defer close(doneSendingResult)

	nSlowRequest := math.Floor(float64(c.TotRequests) * float64(c.SlowRequestLoad) / 100)

	g.Go(func() error {
		defer func() {
			doneSendingJobs <- struct{}{}
		}()
		sendJobs(ctx, SlowRequest, int(nSlowRequest), c.TimeUnit, c.SlowLambda, jobs)
		return nil
	})

	g.Go(func() error {
		nFastRequest := c.TotRequests - int(nSlowRequest)
		defer func() {
			doneSendingJobs <- struct{}{}

		}()
		sendJobs(ctx, FastRequest, nFastRequest, c.TimeUnit, c.FastLambda, jobs)

		return nil
	})

	buffers := NewPool(func() []byte { b := make([]byte, 4); return b })

	for i := 0; i < c.Concurrency; i++ {
		g.Go(func() error {
			defer func() {
				doneSendingResult <- struct{}{}
			}()
			var err error
			for job := range jobs {
				err = func() error {
					start := time.Now()

					conn, err := conns.get()

					if err != nil {
						return err
					}

					defer conns.put(conn)

					g.Go(func() error {
						select {
						case <-ctx.Done():
							conn.conn.SetDeadline(time.Now())
						case <-terminationSignal:
						}
						return nil
					})
					buffer := buffers.Get()
					defer buffers.Put(buffer)

					binary.BigEndian.PutUint32(buffer, uint32(job))

					_, err = conn.conn.Write(buffer)
					if err != nil {
						return err
					}

					var response Response
					if err := json.NewDecoder(conn.conn).Decode(&response); err != nil {
						return err
					}

					results <- requestResult{
						Request:       job,
						ResidenceTime: response.FinishedTs.Sub(response.AcceptedTs),
						WaitingTime:   response.RunningTs.Sub(response.AcceptedTs),
						RoundTripTime: time.Since(start),
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

	benchResult := make(chan BenchResult)
	defer close(benchResult)
	g.Go(func() error {

		var slowRt []float64 = make([]float64, 0)
		var slowWt []float64 = make([]float64, 0)
		var slowRtt []float64 = make([]float64, 0)
		var fastRt []float64 = make([]float64, 0)
		var fastWt []float64 = make([]float64, 0)
		var fastRtt []float64 = make([]float64, 0)

		for result := range results {
			switch result.Request {
			case SlowRequest:
				slowRt = append(slowRt, float64(result.ResidenceTime))
				slowWt = append(slowWt, float64(result.WaitingTime))
				slowRtt = append(slowRtt, float64(result.RoundTripTime))
			case FastRequest:
				fastRt = append(fastRt, float64(result.ResidenceTime))
				fastWt = append(fastWt, float64(result.WaitingTime))
				fastRtt = append(fastRtt, float64(result.RoundTripTime))
			default:
				log.Printf("unknown request type: %d", result.Request)
			}
		}

		benchResult <- BenchResult{
			FastLambda:      c.FastLambda,
			SlowLambda:      c.SlowLambda,
			TotRequests:     c.TotRequests,
			SlowRequestLoad: c.SlowRequestLoad,
			AverageSlowRt:   stat.Mean(slowRt, nil),
			AverageSlowWt:   stat.Mean(slowWt, nil),
			AverageSlowRtt:  stat.Mean(slowRtt, nil),
			AverageFastRt:   stat.Mean(fastRt, nil),
			AverageFastWt:   stat.Mean(fastWt, nil),
			AverageFastRtt:  stat.Mean(fastRtt, nil),
		}

		return nil
	})

	g.Go(func() error {

		defer close(jobs)
		defer close(terminationSignal)
		for i := 0; i < 2; i++ {
			<-doneSendingJobs
		}

		return nil
	})

	g.Go(func() error {

		defer close(results)
		for i := 0; i < c.Concurrency; i++ {
			<-doneSendingResult
		}

		return nil
	})

	return <-benchResult, g.Wait()

}

func sendJobs(ctx context.Context, request Request, n int, timeUnit time.Duration, mean float64, jobs chan<- Request) {
	poisson := distuv.Poisson{
		Lambda: mean,
	}
	for i := 0; i < n; i += 1 {
		select {
		case <-ctx.Done():
			return
		default:
			n := poisson.Rand()
			select {
			case <-time.After(time.Duration(n) * timeUnit):
				jobs <- request
			case <-ctx.Done():
				return
			}
		}
	}
}
