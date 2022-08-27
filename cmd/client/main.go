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
	serverAddress      = flag.String("server-address", "127.0.0.1:8000", "Address for TCP server")
	scheduler          = flag.String("scheduler", "", "Scheduling algorithm used by the server")
	inputFile          = flag.String("input-file", "workload.json", "File path containing workload")
	outputFile         = flag.String("output-file", "", "File path to write result")
	maxIdleConnections = flag.Int("max-idle-connections", 256, "Number of idle connection to keep open to reuse")
	maxOpenConnections = flag.Int("max-open-connections", 256, "Max number of connection opened at same time")
)

var (
	header = []string{
		"sched",
		"fast_rate",
		"slow_rate",
		"tot_requests",
		"slow_percent",
		"average_slow_rt",
		"average_slow_wt",
		"average_slow_rtt",
		"average_fast_rt",
		"average_fast_wt",
		"average_fast_rtt",
	}
)

type Config struct {
	algorithm         string
	addr              string
	outputFile        string
	inputFile         string
	maxIdleConns      int
	maxOpenConnection int
}

type block struct {
	TotRequests int     `json:"tot_requests"`
	SlowRate    float64 `json:"slow_rate"`
	FastRate    float64 `json:"fast_rate"`
	SlowPercent int     `json:"slow_percent"`
}
type jsonData struct {
	Workload []block `json:"workload"`
}

func main() {
	flag.Parse()

	c := Config{
		addr:              *serverAddress,
		outputFile:        *outputFile,
		algorithm:         *scheduler,
		inputFile:         *inputFile,
		maxIdleConns:      *maxIdleConnections,
		maxOpenConnection: *maxOpenConnections,
	}
	if err := run(c); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}

}

func run(c Config) error {
	ctx, canc := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer canc()

	g, ctx := errgroup.WithContext(ctx)

	benches, err := getBenchesFromFile(c.inputFile)
	if err != nil {
		return err
	}

	records := make(chan pbench.BenchResult, len(benches))
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
				MaxIdleConns:    c.maxIdleConns,
				MaxOpenConns:    c.maxOpenConnection,
			}
			r, err = pbench.Bench(ctx, cfg)
			if err != nil {
				log.Print(err)
			} else {
				records <- r
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
				strings.Replace(fmt.Sprintf("%f", record.FastLambda), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.SlowLambda), ".", ",", 1),
				fmt.Sprintf("%d", record.TotRequests),
				fmt.Sprintf("%d", record.SlowRequestLoad),
				strings.Replace(fmt.Sprintf("%f", record.AverageSlowRt), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.AverageSlowWt), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.AverageSlowRtt), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.AverageFastRt), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.AverageFastWt), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.AverageFastRtt), ".", ",", 1),
			}
			if err := csvWriter.Write(row); err != nil {
				log.Print(err)
			}
		}
		return nil
	})

	return g.Wait()

}

func getBenchesFromFile(fname string) ([]block, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var workload jsonData
	if err := json.NewDecoder(f).Decode(&workload); err != nil {
		return nil, err
	}
	return workload.Workload, nil
}
