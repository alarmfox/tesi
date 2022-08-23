package main

import (
	"context"
	"encoding/csv"
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
	serverAddress = flag.String("server-address", "127.0.0.1:8000", "Address for TCP server")
	alg           = flag.String("algorithm", "fcfs", "Scheduling algorithm used by the server")
	outputFile    = flag.String("output-file", "", "File path to write result")
	concurrency   = flag.Int("concurrency", 1, "Number of request to send concurrently")
)

var (
	header = []string{
		"alg",
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
	algorithm   string
	addr        string
	concurrency int
	outputFile  string
}

func main() {
	flag.Parse()

	c := Config{
		addr:        *serverAddress,
		outputFile:  *outputFile,
		algorithm:   *alg,
		concurrency: *concurrency,
	}

	if err := run(c); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}

}

func run(c Config) error {
	ctx, canc := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer canc()

	g, ctx := errgroup.WithContext(ctx)

	records := make(chan pbench.BenchResult)

	g.Go(func() error {
		defer close(records)
		cfg := pbench.BenchConfig{
			ServerAddress:       c.addr,
			TotRequests:         100,
			Concurrency:         1,
			SlowRequestLoad:     50,
			SlowRequestInterval: time.Microsecond * 500,
			FastRequestInterval: time.Microsecond * 500,
			MaxIdleConns:        1024,
			MaxOpenConns:        3072,
			Algorithm:           c.algorithm,
		}

		r, err := pbench.Bench(ctx, cfg)
		if err != nil {
			return err
		}
		records <- r
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
				record.Algorithm,
				record.FasRequestInterval.String(),
				record.SlowRequestInterval.String(),
				fmt.Sprintf("%d", record.TotRequests),
				fmt.Sprintf("%d", record.SlowRequestInterval),
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
