package pbench

import (
	"net"
)

type Request uint32

const (
	SlowRequest Request = iota
	FastRequest
)

type Response struct {
	AcceptedTs int64 `json:"accepted_ts"`
	RunningTs  int64 `json:"running_ts"`
	FinishedTs int64 `json:"finished_ts"`
}

type Job struct {
	Request  Request
	Response Response
	Client   net.Conn
}
