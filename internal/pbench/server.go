package pbench

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"time"
)

const (
	bufferSize = 2048
)

type Scheduler interface {
	Start(context.Context)
	Schedule(Job) error
}

type Server struct {
	scheduler Scheduler
	buffers   *Pool[[]byte]
}

func NewServer(scheduler Scheduler) *Server {
	return &Server{
		scheduler: scheduler,
		buffers:   NewPool(func() []byte { b := make([]byte, bufferSize); return b }),
	}
}

func (s *Server) Start(ctx context.Context, addr string) error {

	conn, err := net.Listen("tcp4", addr)

	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	for {
		client, err := conn.Accept()

		if errors.Is(err, net.ErrClosed) {
			break
		} else if err != nil {
			log.Print(err)
			continue
		}
		go s.handleConnection(ctx, client)

	}
	return nil
}

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {

	defer conn.Close()

	go func() {
		<-ctx.Done()
		conn.SetDeadline(time.Now())
	}()
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

		err = s.scheduler.Schedule(Job{
			Request: request,
			Response: Response{
				Info: Info{
					AcceptedTs: time.Now().UnixMicro(),
				},
			},
			Client: conn,
		})

		if err != nil {
			log.Printf("cannot schedule: %v", err)
			continue
		}
	}

}
