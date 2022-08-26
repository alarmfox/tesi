package pbench

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
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
			<-ctx.Done()
			client.Close()
			return nil
		})
		g.Go(func() error {
			s.handleConnection(client)
			return nil
		})

	}
	return g.Wait()
}

func (s *Server) handleConnection(conn net.Conn) {

	defer conn.Close()

	var err error
	for {
		err = func() error {

			buffer := s.buffers.Get()
			defer s.buffers.Put(buffer)

			n, err := conn.Read(buffer)

			if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) || errors.Is(err, syscall.ECONNRESET) {
				return nil
			} else if err != nil {
				return fmt.Errorf("error from %s: %v", conn.RemoteAddr(), err)
			} else if n != 4 {
				return fmt.Errorf("cannot read requests type")
			}

			r := binary.BigEndian.Uint32(buffer)

			return s.schedule(Job{
				Request: Request(r),
				Response: Response{
					AcceptedTs: time.Now(),
				},
				Client: conn,
			})
		}()

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
