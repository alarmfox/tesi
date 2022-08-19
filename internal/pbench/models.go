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
	Type    RequestType `json:"type"`
	Payload int         `json:"payload,omitempty"`
	Offset  int         `json:"offset,omitempty"`
}

type Response struct {
	Error  int  `json:"error"`
	Result int  `json:"result"`
	Info   Info `json:"info"`
}

type Job struct {
	Request  Request
	Response Response
	Client   net.Conn
}

type Info struct {
	AcceptedTs int64 `json:"accepted_ts"`
	RunningTs  int64 `json:"running_ts"`
	FinishedTs int64 `json:"finished_ts"`
}
