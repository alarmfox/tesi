package pbench

import "sync"

type Pool[T any] struct {
	syncPool sync.Pool
}

func NewPool[T any](newFn func() T) *Pool[T] {
	pool := &Pool[T]{
		syncPool: sync.Pool{
			New: func() interface{} { return newFn() },
		},
	}

	return pool
}

// Get returns an arbitrary item from the pool.
func (p *Pool[T]) Get() T {
	return p.syncPool.Get().(T)
}

// Put places an item in the pool
func (p *Pool[T]) Put(value T) {
	p.syncPool.Put(value)
}
