package pbench

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"log"
	"math"
	"net"
	"sync"
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
	Memory        uint64
}

type BenchResult struct {
	FastRate                float64
	SlowRate                float64
	TotRequests             int
	SlowRequestLoad         int
	AverageSlowRt           float64
	AverageSlowWt           float64
	AverageSlowRtt          float64
	AverageFastRt           float64
	AverageFastWt           float64
	AverageFastRtt          float64
	RequestsPerSecond       float64
	AverageMemoryConsuption float64
}

type BenchConfig struct {
	Algorithm       string
	ServerAddress   string
	TotRequests     int
	SlowRequestLoad int
	SlowRate        float64
	FastRate        float64
}

func Bench(ctx context.Context, c BenchConfig) (BenchResult, error) {

	g, ctx := errgroup.WithContext(ctx)

	requests := make(chan Request, c.TotRequests)
	results := make(chan requestResult, c.TotRequests)
	doneSendingJobs := make(chan struct{}, 2)
	doneSendingResult := make(chan struct{})
	terminationSignal := make(chan struct{})
	defer close(doneSendingJobs)
	defer close(doneSendingResult)

	nSlowRequest := math.Floor(float64(c.TotRequests) * float64(c.SlowRequestLoad) / 100)

	g.Go(func() error {
		defer func() {
			doneSendingJobs <- struct{}{}
		}()
		sendJobs(ctx, SlowRequest, int(nSlowRequest), c.SlowRate, requests)
		return nil
	})

	g.Go(func() error {
		nFastRequest := c.TotRequests - int(nSlowRequest)
		defer func() {
			doneSendingJobs <- struct{}{}
		}()
		sendJobs(ctx, FastRequest, nFastRequest, c.FastRate, requests)

		return nil
	})

	buffers := NewPool(func() []byte { b := make([]byte, 4); return b })

	g.Go(func() error {
		wg := sync.WaitGroup{}
		for request := range requests {
			r := request
			wg.Add(1)
			go func() {
				defer wg.Done()
				start := time.Now()

				// conn, err := conns.get()

				// if err != nil {
				// 	return err
				// }
				// defer conns.put(conn)

				conn, err := net.Dial("tcp4", c.ServerAddress)
				if err != nil {
					log.Print(err)
					return
				}
				defer conn.Close()

				wg.Add(1)
				go func() {
					defer wg.Done()
					select {
					case <-ctx.Done():
						conn.SetDeadline(time.Now())
					case <-terminationSignal:

					}
				}()
				buffer := buffers.Get()
				defer buffers.Put(buffer)

				binary.BigEndian.PutUint32(buffer, uint32(r))

				_, err = conn.Write(buffer)
				if err != nil {
					log.Print(err)
					return
				}
				var response Response
				if err := json.NewDecoder(conn).Decode(&response); err != nil {
					log.Print(err)
					return
				}

				results <- requestResult{
					Request:       r,
					ResidenceTime: response.FinishedTs.Sub(response.AcceptedTs),
					WaitingTime:   response.RunningTs.Sub(response.AcceptedTs),
					RoundTripTime: time.Since(start),
					Memory:        response.Memory,
				}

			}()
		}
		wg.Wait()
		doneSendingResult <- struct{}{}

		return nil
	})

	benchResult := make(chan BenchResult)
	defer close(benchResult)
	g.Go(func() error {

		var slowRt []float64 = make([]float64, 0)
		var slowWt []float64 = make([]float64, 0)
		var slowRtt []float64 = make([]float64, 0)
		var fastRt []float64 = make([]float64, 0)
		var fastWt []float64 = make([]float64, 0)
		var fastRtt []float64 = make([]float64, 0)
		var memoryAllocation []float64 = make([]float64, 0)

		start := time.Now()
		n := 0.0
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
			memoryAllocation = append(memoryAllocation, float64(result.Memory))
			n += 1
		}
		elapsed := time.Since(start)
		rps := n / elapsed.Seconds()

		benchResult <- BenchResult{
			FastRate:                c.FastRate,
			SlowRate:                c.SlowRate,
			TotRequests:             c.TotRequests,
			SlowRequestLoad:         c.SlowRequestLoad,
			AverageSlowRt:           stat.Mean(slowRt, nil),
			AverageSlowWt:           stat.Mean(slowWt, nil),
			AverageSlowRtt:          stat.Mean(slowRtt, nil),
			AverageFastRt:           stat.Mean(fastRt, nil),
			AverageFastWt:           stat.Mean(fastWt, nil),
			AverageFastRtt:          stat.Mean(fastRtt, nil),
			AverageMemoryConsuption: stat.Mean(memoryAllocation, nil),
			RequestsPerSecond:       rps,
		}

		return nil
	})

	g.Go(func() error {

		defer close(requests)
		defer close(terminationSignal)
		for i := 0; i < 2; i++ {
			<-doneSendingJobs
		}

		return nil
	})

	g.Go(func() error {

		defer close(results)
		<-doneSendingResult

		return nil
	})

	return <-benchResult, g.Wait()

}

func sendJobs(ctx context.Context, request Request, n int, rate float64, jobs chan<- Request) {
	exp := distuv.Exponential{
		Rate: rate,
	}
	for i := 0; i < n; i += 1 {
		select {
		case <-ctx.Done():
			return
		default:
			n := exp.Rand()
			d := n * float64(time.Second)
			select {
			case <-time.After(time.Duration(d)):
				jobs <- request
			case <-ctx.Done():
				return
			}
		}
	}
}
