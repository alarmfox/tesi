package pbench

import (
	"net"
	"time"
)

type Request uint32

const (
	SlowRequest Request = iota
	FastRequest
)

type Response struct {
	AcceptedTs time.Time `json:"accepted_ts"`
	RunningTs  time.Time `json:"running_ts"`
	FinishedTs time.Time `json:"finished_ts"`
}

type Job struct {
	Request  Request
	Response Response
	Client   net.Conn
}
