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
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/gonum/stat/distuv"
)

type requestResult struct {
	Request       Request
	ResidenceTime time.Duration
	WaitingTime   time.Duration
	RoundTripTime time.Duration
	Memory        uint64
	JobNumber     int
	CPU           float64
}

type benchResult struct {
	Average float64
	Min     float64
	Max     float64
}

type BenchResult struct {
	SlowRt  benchResult
	SlowWt  benchResult
	SlowRtt benchResult
	FastRt  benchResult
	FastWt  benchResult
	FastRtt benchResult
	Memory  benchResult
	Jobs    benchResult
	CPU     benchResult
	Rps     float64
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

	nFastRequest := c.TotRequests - int(nSlowRequest)
	g.Go(func() error {
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
					JobNumber:     response.Jobs,
					CPU:           response.CPU,
				}

			}()
		}
		wg.Wait()
		doneSendingResult <- struct{}{}

		return nil
	})

	toReturn := make(chan BenchResult)
	defer close(toReturn)
	g.Go(func() error {

		var slowRt []float64 = make([]float64, int(nSlowRequest))
		var slowWt []float64 = make([]float64, int(nSlowRequest))
		var slowRtt []float64 = make([]float64, int(nSlowRequest))
		var fastRt []float64 = make([]float64, nFastRequest)
		var fastWt []float64 = make([]float64, nFastRequest)
		var fastRtt []float64 = make([]float64, nFastRequest)
		var memory []float64 = make([]float64, c.TotRequests)
		var jobs []float64 = make([]float64, c.TotRequests)
		var cpu []float64 = make([]float64, c.TotRequests)

		start := time.Now()
		n := 0
		idxSlow := 0
		idxFast := 0
		for result := range results {
			switch result.Request {
			case SlowRequest:
				slowRt[idxSlow] = float64(result.ResidenceTime)
				slowWt[idxSlow] = float64(result.WaitingTime)
				slowRtt[idxSlow] = float64(result.RoundTripTime)
				idxSlow += 1
			case FastRequest:
				fastRt[idxFast] = float64(result.ResidenceTime)
				fastWt[idxFast] = float64(result.WaitingTime)
				fastRtt[idxFast] = float64(result.RoundTripTime)
				idxFast += 1
			default:
				log.Printf("unknown request type: %d", result.Request)
			}
			memory[n] = float64(result.Memory)
			jobs[n] = float64(result.JobNumber)
			cpu[n] = float64(result.CPU)
			n += 1
		}
		elapsed := time.Since(start)
		rps := float64(n) / elapsed.Seconds()

		toReturn <- BenchResult{
			Rps: rps,
			SlowRt: benchResult{
				Average: stat.Mean(slowRt, nil),
				Min:     floats.Min(slowRt),
				Max:     floats.Max(slowRt),
			},
			SlowWt: benchResult{
				Average: stat.Mean(slowWt, nil),
				Min:     floats.Min(slowWt),
				Max:     floats.Max(slowWt),
			},
			SlowRtt: benchResult{
				Average: stat.Mean(slowRtt, nil),
				Min:     floats.Min(slowRtt),
				Max:     floats.Max(slowRtt),
			},
			FastRt: benchResult{
				Average: stat.Mean(fastRt, nil),
				Min:     floats.Min(fastRt),
				Max:     floats.Max(fastRt),
			},
			FastWt: benchResult{
				Average: stat.Mean(fastWt, nil),
				Min:     floats.Min(fastWt),
				Max:     floats.Max(fastWt),
			},
			FastRtt: benchResult{
				Average: stat.Mean(fastRtt, nil),
				Min:     floats.Min(fastRtt),
				Max:     floats.Max(fastRtt),
			},
			Memory: benchResult{
				Average: stat.Mean(memory, nil),
				Min:     floats.Min(memory),
				Max:     floats.Max(memory),
			},
			Jobs: benchResult{
				Average: stat.Mean(jobs, nil),
				Min:     floats.Min(jobs),
				Max:     floats.Max(jobs),
			},
			CPU: benchResult{
				Average: stat.Mean(cpu, nil),
				Min:     floats.Min(cpu),
				Max:     floats.Max(cpu),
			},
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

	return <-toReturn, g.Wait()

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
