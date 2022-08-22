package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

var (
	inputDirectory = flag.String("input-directory", "", "Directory of input files")
	outputFile     = flag.String("output-file", "", "Output file")
	concurrency    = flag.Uint("concurrency", 1, "Number of files to analyze concurrently")
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
	inputDirectory string
	outputFile     string
	concurrency    uint
}

type Record struct {
	alg            string
	fastInt        time.Duration
	slowInt        time.Duration
	totRequests    int
	slowPercent    int
	averageSlowRt  float64
	averageSlowWt  float64
	averageSlowRtt float64
	averageFastRt  float64
	averageFastWt  float64
	averageFastRtt float64
}

func main() {
	flag.Parse()
	c := Config{
		inputDirectory: *inputDirectory,
		outputFile:     *outputFile,
		concurrency:    *concurrency,
	}
	if err := run(c); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}

}

func run(c Config) error {
	directory, err := os.ReadDir(c.inputDirectory)
	if err != nil {
		return err
	}

	var inFiles []string
	for _, content := range directory {
		if !content.IsDir() && content.Type().IsRegular() {
			fpath := filepath.Join(c.inputDirectory, content.Name())
			inFiles = append(inFiles, fpath)
		}
	}

	files := make(chan string, len(inFiles))

	ctx, canc := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer canc()
	g, ctx := errgroup.WithContext(ctx)

	records := make(chan Record, len(inFiles))

	done := make(chan struct{}, c.concurrency)
	defer close(done)

	g.Go(func() error {
		for _, file := range inFiles {
			files <- file
		}
		close(files)
		return nil
	})
	g.Go(func() error {
		defer close(records)
		for i := 0; i < int(c.concurrency); i++ {
			g.Go(func() error {
				for file := range files {
					if err := process(ctx, file, records); err != nil {
						log.Print(err)
					}
				}
				done <- struct{}{}
				return nil
			})
		}

		for i := 0; i < int(c.concurrency); i++ {
			<-done
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
				record.alg,
				record.fastInt.String(),
				record.slowInt.String(),
				fmt.Sprintf("%d", record.totRequests),
				fmt.Sprintf("%d", record.slowPercent),
				strings.Replace(fmt.Sprintf("%f", record.averageSlowRt), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.averageSlowWt), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.averageSlowRtt), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.averageFastRt), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.averageFastWt), ".", ",", 1),
				strings.Replace(fmt.Sprintf("%f", record.averageFastRtt), ".", ",", 1),
			}
			if err := csvWriter.Write(row); err != nil {
				log.Print(err)
			}
		}

		return nil

	})

	return g.Wait()
}

func process(ctx context.Context, file string, records chan<- Record) error {

	alg, fastInt, slowInt, nRequests, slowLoad, err := parseFilename(filepath.Base(file))

	if err != nil {
		return err
	}

	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("cannot open %q: %v", file, err)
	}
	defer f.Close()

	var totSlowRt, totFastRt, totFastRtt, totFastWt, totSlowRtt, totSlowWt, slowCount, fastCount int64 = 0, 0, 0, 0, 0, 0, 0, 0

	r := bufio.NewScanner(f)

	for r.Scan() {
		select {
		case <-ctx.Done():
			return nil
		default:
			l := r.Text()

			rqType, rt, wt, rtt, err := parseRow(l)
			if err != nil {
				log.Print(err)
				continue
			}

			if rqType == 0 {
				slowCount += 1
				totSlowRt += rt
				totSlowRtt += rtt
				totSlowWt += wt
			} else if rqType == 1 {
				fastCount += 1
				totFastRt += rt
				totFastRtt += rtt
				totFastWt += wt
			}
		}

	}

	avgSlowRt := float64(totSlowRt) / float64(slowCount)
	avgSlowWt := float64(totSlowWt) / float64(slowCount)
	avgSlowRtt := float64(totSlowRtt) / float64(slowCount)

	avgFastRt := float64(totFastRt) / float64(fastCount)
	avgFastWt := float64(totFastWt) / float64(fastCount)
	avgFastRtt := float64(totFastRtt) / float64(fastCount)

	records <- Record{
		alg:            alg,
		fastInt:        fastInt,
		slowInt:        slowInt,
		totRequests:    nRequests,
		slowPercent:    slowLoad,
		averageSlowRt:  avgSlowRt,
		averageSlowWt:  avgSlowWt,
		averageSlowRtt: avgSlowRtt,
		averageFastRt:  avgFastRt,
		averageFastWt:  avgFastWt,
		averageFastRtt: avgFastRtt,
	}
	return nil
}

func parseRow(row string) (uint32, int64, int64, int64, error) {
	parts := strings.Split(row, ";")
	if len(parts) != 4 {
		return 0, 0, 0, 0, fmt.Errorf("bad line: %s", row)
	}

	rqType, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	rt, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	wt, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	rtt, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	return uint32(rqType), rt, wt, rtt, nil
}

func parseFilename(fname string) (string, time.Duration, time.Duration, int, int, error) {

	parts := strings.Split(fname, "_")

	if len(parts) != 5 {
		return "", 0, 0, 0, 0, fmt.Errorf("bad filename: %q", fname)
	}

	nRequests, err := strconv.Atoi(parts[3])
	if err != nil {
		return "", 0, 0, 0, 0, fmt.Errorf("bad request number %q: %v", parts[3], nRequests)
	}

	slowLoad, err := strconv.Atoi(parts[4])
	if err != nil {
		return "", 0, 0, 0, 0, fmt.Errorf("bad slow percent load number %q: %v", parts[4], err)
	}

	fastInt, err := time.ParseDuration(parts[1])
	if err != nil {
		return "", 0, 0, 0, 0, fmt.Errorf("bad fast interval %q: %v", parts[1], err)
	}

	slowInt, err := time.ParseDuration(parts[2])
	if err != nil {
		return "", 0, 0, 0, 0, fmt.Errorf("bad fast interval %q: %v", parts[2], err)
	}

	alg := strings.ToLower(parts[0])

	if alg != "fcfs" && alg != "drr" {
		return "", 0, 0, 0, 0, fmt.Errorf("unknown algoritm %q", alg)

	}

	return alg, fastInt, slowInt, nRequests, slowLoad, nil

}
