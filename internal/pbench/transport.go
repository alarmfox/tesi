package pbench

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
)

const (
	bufferSize = 2048
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
	for {
		client, err := conn.Accept()

		if errors.Is(err, net.ErrClosed) {
			break
		} else if err != nil {
			log.Print(err)
			continue
		}

		buffer := make([]byte, bufferSize)

		n, err := client.Read(buffer)

		if err != nil {
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

		err = s.scheduler.Schedule(Job{
			Request: request,
			Client:  client,
		})

		if err != nil {
			log.Printf("cannot schedule: %v", err)
			client.Close()
		}
	}
	return nil
}
