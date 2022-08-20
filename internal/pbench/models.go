package pbench

import (
	"net"
)

type RequestType uint8

const (
	SlowRequest RequestType = iota
	FastRequest
)

type Request struct {
	Type RequestType `json:"type"`
}

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
