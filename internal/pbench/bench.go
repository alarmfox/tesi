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
)

type requestResult struct {
	Request       Request
	ResidenceTime int64
	WaitingTime   int64
	RTT           int64
}

type BenchResult struct {
	FastRequestInterval time.Duration
	SlowRequestInterval time.Duration
	TotRequests         int
	SlowRequestLoad     int
	AverageSlowRt       float64
	AverageSlowWt       float64
	AverageSlowRtt      float64
	AverageFastRt       float64
	AverageFastWt       float64
	AverageFastRtt      float64
}

type BenchConfig struct {
	Algorithm           string
	ServerAddress       string
	TotRequests         int
	Concurrency         int
	SlowRequestLoad     int
	SlowRequestInterval time.Duration
	FastRequestInterval time.Duration
	MaxIdleConns        int
	MaxOpenConns        int
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
		ticker := time.NewTicker(c.SlowRequestInterval)
		defer func() {
			ticker.Stop()
			doneSendingJobs <- struct{}{}
		}()
		for i := 0; i < int(nSlowRequest); i += 1 {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				jobs <- SlowRequest
			}
		}
		return nil
	})

	g.Go(func() error {
		nFastRequest := c.TotRequests - int(nSlowRequest)
		ticker := time.NewTicker(c.FastRequestInterval)
		defer func() {
			ticker.Stop()
			doneSendingJobs <- struct{}{}

		}()
		for i := 0; i < nFastRequest; i += 1 {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				jobs <- FastRequest

			}
		}

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

	benchResult := make(chan BenchResult, 1)
	defer close(benchResult)
	g.Go(func() error {

		var totSlowRt, totFastRt, totFastRtt, totFastWt, totSlowRtt, totSlowWt, slowCount, fastCount int64 = 0, 0, 0, 0, 0, 0, 0, 0
		for result := range results {
			switch result.Request {
			case SlowRequest:
				slowCount += 1
				totSlowRt += result.ResidenceTime
				totSlowRtt += result.RTT
				totSlowWt += result.WaitingTime
			case FastRequest:
				fastCount += 1
				totFastRt += result.ResidenceTime
				totFastRtt += result.RTT
				totFastWt += result.WaitingTime
			default:
				log.Printf("unknown request type: %d", result.Request)
			}
		}
		avgSlowRt := float64(totSlowRt) / float64(slowCount)
		avgSlowWt := float64(totSlowWt) / float64(slowCount)
		avgSlowRtt := float64(totSlowRtt) / float64(slowCount)

		avgFastRt := float64(totFastRt) / float64(fastCount)
		avgFastWt := float64(totFastWt) / float64(fastCount)
		avgFastRtt := float64(totFastRtt) / float64(fastCount)

		benchResult <- BenchResult{
			FastRequestInterval: c.FastRequestInterval,
			SlowRequestInterval: c.SlowRequestInterval,
			TotRequests:         c.TotRequests,
			SlowRequestLoad:     c.SlowRequestLoad,
			AverageSlowRt:       avgSlowRt,
			AverageSlowWt:       avgSlowWt,
			AverageSlowRtt:      avgSlowRtt,
			AverageFastRt:       avgFastRt,
			AverageFastWt:       avgFastWt,
			AverageFastRtt:      avgFastRtt,
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

	g.Wait()

	return <-benchResult, nil

}
