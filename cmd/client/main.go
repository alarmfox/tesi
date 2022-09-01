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
	"strings"
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
		"devstd_slow_rt",
		"var_slow_rt",

		"avg_slow_wt",
		"min_slow_wt",
		"max_slow_wt",
		"devstd_slow_wt",
		"var_slow_wt",

		"avg_slow_rtt",
		"min_slow_rtt",
		"max_slow_rtt",
		"devstd_slow_rtt",
		"var_slow_rtt",

		"avg_fast_rt",
		"min_fast_rt",
		"max_fast_rt",
		"devstd_fast_rt",
		"var_fast_rt",

		"avg_fast_wt",
		"min_fast_wt",
		"max_fast_wt",
		"devstd_fast_wt",
		"var_fast_wt",

		"avg_fast_rtt",
		"min_fast_rtt",
		"max_fast_rtt",
		"devstd_fast_rtt",
		"var_fast_rtt",

		"avg_memory",
		"min_memory",
		"max_memory",
		"devstd_memory",
		"var_memory",

		"avg_jobs",
		"min_jobs",
		"max_jobs",
		"devstd_jobs",
		"var_jobs",

		"avg_cpu",
		"min_cpu",
		"max_cpu",
		"devstd_cpu",
		"var_cpu",
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
				strings.Replace(fmt.Sprintf("%f", record.result.SlowRt.Average), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.SlowRt.Min), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.SlowRt.Max), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.SlowRt.DevStd), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.SlowRt.Var), ".", ",", 1),

				strings.Replace(fmt.Sprintf("%f", record.result.SlowWt.Average), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.SlowWt.Min), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.SlowWt.Max), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.SlowWt.DevStd), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.SlowRt.Var), ".", ",", 1),

				strings.Replace(fmt.Sprintf("%f", record.result.SlowRtt.Average), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.SlowRtt.Min), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.SlowRtt.Max), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.SlowRtt.DevStd), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.SlowRtt.Var), ".", ",", 1),

				strings.Replace(fmt.Sprintf("%f", record.result.FastRt.Average), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.FastRt.Min), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.FastRt.Max), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.FastRt.DevStd), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.FastRt.Var), ".", ",", 1),

				strings.Replace(fmt.Sprintf("%f", record.result.FastWt.Average), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.FastWt.Min), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.FastWt.Max), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.FastWt.DevStd), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.FastRt.Var), ".", ",", 1),

				strings.Replace(fmt.Sprintf("%f", record.result.FastRtt.Average), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.FastRtt.Min), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.FastRtt.Max), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.FastRtt.DevStd), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.FastRtt.Var), ".", ",", 1),

				strings.Replace(fmt.Sprintf("%f", record.result.Memory.Average), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.Memory.Min), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.Memory.Max), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.Memory.DevStd), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.Memory.Var), ".", ",", 1),

				strings.Replace(fmt.Sprintf("%f", record.result.Jobs.Average), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.Jobs.Min), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.Jobs.Max), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.Jobs.DevStd), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.Jobs.Var), ".", ",", 1),

				strings.Replace(fmt.Sprintf("%f", record.result.CPU.Average), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.CPU.Min), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.CPU.Max), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.CPU.DevStd), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.result.CPU.Var), ".", ",", 1),
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
