package pbench

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"runtime"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

type Server struct {
	highPrio chan<- Job
	lowPrio  chan<- Job
	isDRR    bool
	buffers  *Pool[[]byte]
	sync.Mutex
}

func NewServer(highPrio, lowPrio chan<- Job, isDRR bool) *Server {
	return &Server{
		highPrio: highPrio,
		lowPrio:  lowPrio,
		isDRR:    isDRR,
		buffers:  NewPool(func() []byte { b := make([]byte, 4); return b }),
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
		return nil
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
			if err := s.handleConnection(client); parseErr(err) != nil {
				log.Print(err)
			}
			return nil
		})

	}
	return g.Wait()
}

func (s *Server) handleConnection(conn net.Conn) error {

	defer conn.Close()

	for {

		buffer := s.buffers.Get()
		defer s.buffers.Put(buffer)

		n, err := conn.Read(buffer)

		if err != nil {
			return err
		} else if n != 4 {
			return fmt.Errorf("cannot read request type")
		}

		r := binary.BigEndian.Uint32(buffer)

		err = s.schedule(Job{
			Request: Request(r),
			Response: Response{
				AcceptedTs: time.Now(),
				Jobs:       runtime.NumGoroutine() - 4,
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
		if j.Request == SlowRequest {
			s.lowPrio <- j
		} else if j.Request == FastRequest {
			s.highPrio <- j
		} else {
			return fmt.Errorf("unknown request type")
		}
	} else {
		s.highPrio <- j
	}
	return nil
}

func parseErr(err error) error {
	if !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) && errors.Is(err, syscall.ECONNRESET) {
		return err
	}
	return nil
}
