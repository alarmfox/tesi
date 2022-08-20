package pbench

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	bufferSize = 2048
)

type Server struct {
	highPrio chan<- Job
	lowPrio  chan<- Job
	isDRR    bool
	buffers  *Pool[[]byte]
}

func NewServer(highPrio, lowPrio chan<- Job, isDRR bool) *Server {
	return &Server{
		highPrio: highPrio,
		lowPrio:  lowPrio,
		isDRR:    isDRR,
		buffers:  NewPool(func() []byte { b := make([]byte, bufferSize); return b }),
	}
}

func (s *Server) Start(ctx context.Context, addr string) error {

	conn, err := net.Listen("tcp4", addr)

	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		<-ctx.Done()
		conn.Close()
		return ctx.Err()
	})

	for {
		client, err := conn.Accept()

		if errors.Is(err, net.ErrClosed) {
			break
		} else if err != nil {
			log.Print(err)
			continue
		}
		g.Go(func() error {
			g.Go(func() error {
				<-ctx.Done()
				client.SetDeadline(time.Now())
				return nil
			})
			s.handleConnection(ctx, client)
			return nil
		})

	}
	return g.Wait()
}

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {

	defer conn.Close()

	for {

		buffer := s.buffers.Get()
		defer s.buffers.Put(buffer)

		n, err := conn.Read(buffer)

		if errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(err, io.EOF) {
			return
		} else if err != nil {
			log.Printf("error from %s: %v", conn.RemoteAddr(), err)
			continue
		}

		var request Request
		if err := json.NewDecoder(bytes.NewReader(buffer[:n])).Decode(&request); err != nil {
			log.Printf("error from %s: %v", conn.RemoteAddr(), err)
			conn.Close()
			continue
		}

		err = s.schedule(Job{
			Request: request,
			Response: Response{
				AcceptedTs: time.Now().UnixMicro(),
			},
			Client: conn,
		})

		if err != nil {
			log.Printf("cannot schedule: %v", err)
			continue
		}
	}
}

func (s *Server) schedule(j Job) error {
	if s.isDRR {
		if j.Request.Type == SlowRequest {
			s.lowPrio <- j
		} else if j.Request.Type == FastRequest {
			s.highPrio <- j
		} else {
			return fmt.Errorf("unknown request type")
		}
	} else {
		s.highPrio <- j
	}
	return nil
}
