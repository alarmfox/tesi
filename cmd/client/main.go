package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/alarmfox/tesi/internal/pbench"
	"golang.org/x/sync/errgroup"
)

var (
	serverAddress = flag.String("server-address", "127.0.0.1:8000", "Address for TCP server")
	scheduler     = flag.String("scheduler", "", "Scheduling algorithm used by the server")
	inputFile     = flag.String("input-file", "workload.json", "File path containing workload")
	outputFile    = flag.String("output-file", "", "File path to write result")
)

var (
	header = []string{
		"sched",
		"fast_rate",
		"slow_rate",
		"tot_requests",
		"slow_load",
		"rps",

		"avg_slow_rt",
		"min_slow_rt",
		"max_slow_rt",

		"avg_slow_wt",
		"min_slow_wt",
		"max_slow_wt",

		"avg_slow_rtt",
		"min_slow_rtt",
		"max_slow_rtt",

		"avg_fast_rt",
		"min_fast_rt",
		"max_fast_rt",

		"avg_fast_wt",
		"min_fast_wt",
		"max_fast_wt",

		"avg_fast_rtt",
		"min_fast_rtt",
		"max_fast_rtt",

		"avg_memory",
		"min_memory",
		"max_memory",

		"avg_jobs",
		"min_jobs",
		"max_jobs",

		"avg_cpu",
		"min_cpu",
		"max_cpu",
	}
)

type Config struct {
	algorithm  string
	addr       string
	outputFile string
	inputFile  string
}

type block struct {
	TotRequests int     `json:"tot_requests"`
	SlowRate    float64 `json:"slow_rate"`
	FastRate    float64 `json:"fast_rate"`
	SlowPercent int     `json:"slow_percent"`
}

func main() {
	flag.Parse()

	c := Config{
		addr:       *serverAddress,
		outputFile: *outputFile,
		algorithm:  *scheduler,
		inputFile:  *inputFile,
	}
	if err := run(c); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}

type record struct {
	result  pbench.BenchResult
	request pbench.BenchConfig
}

func run(c Config) error {
	ctx, canc := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer canc()

	g, ctx := errgroup.WithContext(ctx)

	benches, err := readBenchesFromFile(c.inputFile)
	if err != nil {
		return err
	}

	records := make(chan record, len(benches))
	g.Go(func() error {
		defer close(records)
		done := 0
		var err error
		var r pbench.BenchResult
		for i := range benches {
			cfg := pbench.BenchConfig{
				Algorithm:       c.algorithm,
				ServerAddress:   c.addr,
				TotRequests:     benches[i].TotRequests,
				SlowRequestLoad: benches[i].SlowPercent,
				SlowRate:        benches[i].SlowRate,
				FastRate:        benches[i].FastRate,
			}
			r, err = pbench.Bench(ctx, cfg)
			if err != nil {
				log.Print(err)
			} else {
				records <- record{
					result:  r,
					request: cfg,
				}
			}

			done += 1
			log.Printf("done %d/%d: %+v", done, len(benches), cfg)
		}

		return nil
	})

	g.Go(func() error {
		var writer io.Writer
		if c.outputFile != "" {
			f, err := os.Create(c.outputFile)
			if err != nil {
				return err
			}
			defer f.Close()
			writer = f
		} else {
			writer = os.Stdout
		}
		csvWriter := csv.NewWriter(writer)
		csvWriter.Comma = ';'
		defer csvWriter.Flush()

		csvWriter.Write(header)
		for record := range records {
			row := []string{
				c.algorithm,
				fmt.Sprintf("%d", int(record.request.FastRate)),
				fmt.Sprintf("%d", int(record.request.SlowRate)),
				fmt.Sprintf("%d", record.request.TotRequests),
				fmt.Sprintf("%d", record.request.SlowRequestLoad),
				fmt.Sprintf("%f", record.result.Rps),

				fmt.Sprintf("%f", record.result.SlowRt.Average),
				fmt.Sprintf("%f", record.result.SlowRt.Min),
				fmt.Sprintf("%f", record.result.SlowRt.Max),

				fmt.Sprintf("%f", record.result.SlowWt.Average),
				fmt.Sprintf("%f", record.result.SlowWt.Min),
				fmt.Sprintf("%f", record.result.SlowWt.Max),

				fmt.Sprintf("%f", record.result.SlowRtt.Average),
				fmt.Sprintf("%f", record.result.SlowRtt.Min),
				fmt.Sprintf("%f", record.result.SlowRtt.Max),

				fmt.Sprintf("%f", record.result.FastRt.Average),
				fmt.Sprintf("%f", record.result.FastRt.Min),
				fmt.Sprintf("%f", record.result.FastRt.Max),

				fmt.Sprintf("%f", record.result.FastWt.Average),
				fmt.Sprintf("%f", record.result.FastWt.Min),
				fmt.Sprintf("%f", record.result.FastWt.Max),

				fmt.Sprintf("%f", record.result.FastRtt.Average),
				fmt.Sprintf("%f", record.result.FastRtt.Min),
				fmt.Sprintf("%f", record.result.FastRtt.Max),

				fmt.Sprintf("%f", record.result.Memory.Average),
				fmt.Sprintf("%f", record.result.Memory.Min),
				fmt.Sprintf("%f", record.result.Memory.Max),

				fmt.Sprintf("%f", record.result.Jobs.Average),
				fmt.Sprintf("%f", record.result.Jobs.Min),
				fmt.Sprintf("%f", record.result.Jobs.Max),

				fmt.Sprintf("%f", record.result.CPU.Average),
				fmt.Sprintf("%f", record.result.CPU.Min),
				fmt.Sprintf("%f", record.result.CPU.Max),
			}
			if err := csvWriter.Write(row); err != nil {
				log.Print(err)
			}
		}
		return nil
	})

	return g.Wait()

}

func readBenchesFromFile(fname string) ([]block, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	type jsonData struct {
		Workload []block `json:"workload"`
	}

	var workload jsonData
	if err := json.NewDecoder(f).Decode(&workload); err != nil {
		return nil, err
	}
	return workload.Workload, nil
}
