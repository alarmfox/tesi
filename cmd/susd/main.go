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
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	bufferSize = 2048
)

var (
	addr = flag.String("listen-addr", "127.0.0.1:8000", "Listen address for TCP server")
)

type Scheduler interface {
	Schedule(connections <-chan net.Conn)
}

type RequestType uint8

const (
	SlowRequest RequestType = iota
	FastRequest
)

type Request struct {
	Type    RequestType `json:"type"`
	Payload uint        `json:"payload"`
	Offset  int         `json:"offset"`
}

type Response struct {
	Error  int  `json:"type"`
	Result uint `json:"result"`
}

type Server struct {
	queue []uint
	mu    sync.Mutex
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

	connections := make(chan net.Conn)

	g.Go(func() error {
		for {
			client, err := conn.Accept()

			if errors.Is(err, net.ErrClosed) {
				break
			} else if err != nil {
				log.Print(err)
				continue
			}
			connections <- client
		}

		return nil
	})

	// scheduler
	s := Server{
		queue: make([]uint, bufferSize),
	}
	g.Go(func() error {
		for connection := range connections {
			go s.handleConnection(ctx, connection)
		}
		return nil
	})

	g.Go(func() error {
		<-ctx.Done()
		conn.Close()
		close(connections)
		return nil
	})

	return g.Wait()

}

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	buffer := make([]byte, bufferSize)

	n, err := conn.Read(buffer)

	if err != nil {
		log.Printf("error from %s: %v", conn.RemoteAddr(), err)
		return
	}
	var message Request
	if err := json.Unmarshal(buffer[:n], &message); err != nil {
		log.Printf("error from %s: %v", conn.RemoteAddr(), err)
		return
	}

	var response Response
	switch message.Type {
	case FastRequest:
		n, err := s.handleFastRequest(ctx)

		if err != nil {
			response.Error = -1
		} else {
			response.Error = 0
			response.Result = n
		}

		b, _ := json.Marshal(response)
		conn.Write(b)
	case SlowRequest:
		err := s.handleSlowRequest(ctx, message.Payload, message.Offset)

		if err != nil {
			response.Error = -2
		} else {
			response.Error = 0
		}

		b, _ := json.Marshal(response)
		conn.Write(b)
	default:
		log.Printf("unsupported request type: %d", message.Type)
	}

}

func (s *Server) handleFastRequest(ctx context.Context) (uint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.queue[len(s.queue)-1], nil
}

var (
	errTimeout    = errors.New("timeout occurred")
	errOutOfRange = errors.New("out of range index")
)

func (s *Server) handleSlowRequest(ctx context.Context, payload uint, offset int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-time.After(100 * time.Millisecond):
		if offset >= len(s.queue) {
			return errOutOfRange
		}
		s.queue[offset] = payload
		return nil
	case <-ctx.Done():
		return errTimeout
	}
}
