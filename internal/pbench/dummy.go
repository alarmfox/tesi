package pbench

import (
	"errors"
	"time"
)

type Buffer struct {
	slowTime time.Duration
}

func NewBuffer(slowTime time.Duration) *Buffer {
	return &Buffer{slowTime: slowTime}
}

var (
	ErrOutOfRange = errors.New("index out of range")
)

func (b *Buffer) Slow() {
	time.Sleep(b.slowTime)
}

func (b *Buffer) Fast() {
}
