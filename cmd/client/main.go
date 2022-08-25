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
	"time"

	"github.com/alarmfox/tesi/internal/pbench"
	"golang.org/x/sync/errgroup"
)

var (
	serverAddress      = flag.String("server-address", "127.0.0.1:8000", "Address for TCP server")
	scheduler          = flag.String("scheduler", "", "Scheduling algorithm used by the server")
	inputFile          = flag.String("input-file", "workload.json", "File path containing workload")
	outputFile         = flag.String("output-file", "", "File path to write result")
	concurrency        = flag.Int("concurrency", 1, "Number of request to send concurrently")
	maxIdleConnections = flag.Int("max-idle-connections", 256, "Number of idle connection to keep open to reuse")
	maxOpenConnections = flag.Int("max-open-connections", 256, "Max number of connection opened at same time")
)

var (
	header = []string{
		"sched",
		"fast_int",
		"slow_int",
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
	concurrency       int
	outputFile        string
	inputFile         string
	maxIdleConns      int
	maxOpenConnection int
}

type block struct {
	TotRequests int    `json:"tot_requests"`
	SlowInt     string `json:"slow_int"`
	FastInt     string `json:"fast_int"`
	SlowPercent int    `json:"slow_percent"`
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
		concurrency:       *concurrency,
		inputFile:         *inputFile,
		maxIdleConns:      *maxIdleConnections,
		maxOpenConnection: *maxOpenConnections,
	}
	log.Print(c)

	if err := run(c); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}

}

func run(c Config) error {
	ctx, canc := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer canc()

	g, ctx := errgroup.WithContext(ctx)

	benchs := make(chan pbench.BenchConfig)
	var totJobs int
	g.Go(func() error {
		defer close(benchs)
		f, err := os.Open(c.inputFile)
		if err != nil {
			return err
		}
		defer f.Close()

		var workload jsonData
		if err := json.NewDecoder(f).Decode(&workload); err != nil {
			return err
		}
		totJobs = len(workload.Workload)
		for _, w := range workload.Workload {
			slowInt, err := time.ParseDuration(w.SlowInt)
			if err != nil {
				log.Print(err)
				continue
			}
			fastInt, err := time.ParseDuration(w.FastInt)
			if err != nil {
				log.Print(err)
				continue
			}
			benchs <- pbench.BenchConfig{
				ServerAddress:       c.addr,
				TotRequests:         w.TotRequests,
				SlowRequestInterval: slowInt,
				FastRequestInterval: fastInt,
				SlowRequestLoad:     w.SlowPercent,
				Concurrency:         c.concurrency,
				MaxIdleConns:        c.maxIdleConns,
				MaxOpenConns:        c.maxOpenConnection,
				Algorithm:           c.algorithm,
			}

		}
		return nil
	})

	records := make(chan pbench.BenchResult)
	g.Go(func() error {
		defer close(records)
		done := 0
		var err error
		var r pbench.BenchResult
		for bench := range benchs {
			r, err = pbench.Bench(ctx, bench)
			if err != nil {
				log.Print(err)
			} else {
				records <- r
			}

			done += 1
			log.Printf("done %d/%d: %+v", done, totJobs, bench)
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
		csvWriter.Flush()

		for record := range records {
			row := []string{
				c.algorithm,
				record.FastRequestInterval.String(),
				record.SlowRequestInterval.String(),
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
