package pbench

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
)

const (
	bufferSize = 4096
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

type Job struct {
	Request Request
	Client  net.Conn
}

type Scheduler interface {
	Start(context.Context)
	Schedule(Job) error
}

type Server struct {
	scheduler Scheduler
}

func NewServer(scheduler Scheduler) *Server {
	return &Server{
		scheduler: scheduler,
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

	pool := NewPool(func() []byte { b := make([]byte, bufferSize); return b })
	for {
		client, err := conn.Accept()

		if errors.Is(err, net.ErrClosed) {
			break
		} else if err != nil {
			log.Print(err)
			continue
		}

		buffer := pool.Get()

		n, err := client.Read(buffer)

		if err != nil {
			log.Printf("error from %s: %v", client.RemoteAddr(), err)
			client.Close()
			continue
		}

		var request Request
		if err := json.NewDecoder(bytes.NewReader(buffer[:n])).Decode(&request); err != nil {
			log.Printf("error from %s: %v", client.RemoteAddr(), err)
			client.Close()
			continue
		}

		err = s.scheduler.Schedule(Job{
			Request: request,
			Client:  client,
		})

		if err != nil {
			log.Printf("cannot schedule: %v", err)
			client.Close()
		}
		pool.Put(buffer)
	}
	return nil
}
