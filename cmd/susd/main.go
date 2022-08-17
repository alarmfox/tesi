package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	bufferSize = 2048
)

var (
	addr = flag.String("listen-addr", "127.0.0.1:8000", "Listen address for TCP server")
)

type RequestType uint8

const (
	SlowRequest RequestType = iota
	FastRequest
)

type Request struct {
	Type    RequestType `json:"type"`
	Payload int         `json:"payload,omitempty"`
	Offset  int         `json:"offset,omitempty"`
}

type Response struct {
	Error  int `json:"error"`
	Result int `json:"result"`
}

type Buffer struct {
	data []int
}

func NewBuffer(size int) *Buffer {
	return &Buffer{
		data: make([]int, size),
	}
}

func (b *Buffer) Slow(v, pos int) error {

	time.Sleep(100 * time.Millisecond)

	if pos >= len(b.data) {
		return errOutOfRange
	}
	b.data[pos] += v
	return nil
}

func (b *Buffer) Fast() int {
	return b.data[len(b.data)-1]
}

func main() {
	flag.Parse()

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	conn, err := net.Listen("tcp", *addr)

	if err != nil {
		return err
	}

	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, os.Kill, os.Interrupt)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	requests := make(chan SchedRequest)
	defer close(requests)
	g.Go(func() error {
		for {
			client, err := conn.Accept()

			if errors.Is(err, net.ErrClosed) {
				break
			} else if err != nil {
				log.Print(err)
				continue
			}
			go func() {
				<-ctx.Done()
				client.SetReadDeadline(time.Now())
			}()
			buffer := make([]byte, bufferSize)

			n, err := client.Read(buffer)

			if errors.Is(err, os.ErrDeadlineExceeded) {
				client.Close()
				continue
			} else if err != nil {
				log.Printf("error from %s: %v", client.RemoteAddr(), err)
				client.Close()
				continue
			}
			var request Request
			if err := json.Unmarshal(buffer[:n], &request); err != nil {
				log.Printf("error from %s: %v", client.RemoteAddr(), err)
				client.Close()
				continue
			}

			requests <- SchedRequest{
				Request: request,
				Client:  client,
			}
		}

		return nil
	})

	jobs := make(chan SchedRequest)
	g.Go(func() error {
		fcfs := NewFCFS(requests, jobs)
		fcfs.Start(ctx)
		return nil
	})

	g.Go(func() error {
		<-ctx.Done()
		conn.Close()
		close(jobs)
		return nil
	})

	g.Go(func() error {
		buffer := NewBuffer(100)
		var response Response
		var err error
		for job := range jobs {
			switch job.Request.Type {
			case SlowRequest:
				err = buffer.Slow(job.Request.Payload, job.Request.Offset)
				if errors.Is(err, errOutOfRange) {
					response.Error = -1
				} else if err != nil {
					response.Error = -2
				}
			case FastRequest:
				v := buffer.Fast()
				response.Error = 0
				response.Result = v

			}
			b, err := json.Marshal(response)
			if err != nil {
				log.Print(err)
			} else if _, err := job.Client.Write(b); err != nil {
				log.Print(err)
			}
			job.Client.Close()
		}
		return nil
	})

	return g.Wait()

}

var (
	errOutOfRange = errors.New("out of range index")
)

type FCFS struct {
	out chan<- SchedRequest
	in  <-chan SchedRequest
}

type SchedRequest struct {
	Request Request
	Client  net.Conn
}

func NewFCFS(in <-chan SchedRequest, out chan<- SchedRequest) *FCFS {
	return &FCFS{
		out: out,
		in:  in,
	}
}

func (f *FCFS) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case r := <-f.in:
			f.out <- r
		}
	}
}
