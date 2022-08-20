package pbench

import (
	"errors"
	"time"
)

type Buffer struct {
}

func NewBuffer() *Buffer {
	return &Buffer{}
}

var (
	ErrOutOfRange = errors.New("index out of range")
)

func (b *Buffer) Slow() {
	time.Sleep(time.Millisecond)
}

func (b *Buffer) Fast() {

}
