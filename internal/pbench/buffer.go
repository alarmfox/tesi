package pbench

import (
	"errors"
	"time"
)

type Buffer struct {
	data []int
}

func NewBuffer(size int) *Buffer {
	return &Buffer{
		data: make([]int, size),
	}
}

var (
	ErrOutOfRange = errors.New("index out of range")
)

func (b *Buffer) Slow(v, pos int) error {

	time.Sleep(time.Millisecond)

	if pos >= len(b.data) {
		return ErrOutOfRange
	}
	b.data[pos] = v
	return nil
}

func (b *Buffer) Fast(pos int) (int, error) {

	if pos >= len(b.data) {
		return 0, ErrOutOfRange
	}
	return b.data[pos], nil
}
