package pbench

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// TCPConn is a wrapper for a single tcp connection
type TCPConn struct {
	id   string       // A unique id to identify a connection
	pool *TcpConnPool // The TCP connecion pool
	conn net.Conn     // The underlying TCP connection
}

func (tcp *TCPConn) Read(b []byte) (n int, err error) {
	return tcp.conn.Read(b)
}
func (tcp *TCPConn) Write(b []byte) (n int, err error) {
	return tcp.conn.Write(b)
}

func (tcp *TCPConn) SetDeadline(t time.Time) (err error) {
	return tcp.conn.SetDeadline(t)
}

// connRequest wraps a channel to receive a connection
// and a channel to receive an error
type connRequest struct {
	connChan chan *TCPConn
	errChan  chan error
}

// TcpConnPool represents a pool of tcp connections
type TcpConnPool struct {
	address      string
	mu           sync.Mutex          // mutex to prevent race conditions
	idleConns    map[string]*TCPConn // holds the idle connections
	numOpen      int                 // counter that tracks open connections
	maxOpenCount int
	maxIdleCount int
	requestChan  chan *connRequest
}

// Put() attempts to return a used connection back to the pool
// It closes the connection if it can't do so
func (p *TcpConnPool) Put(c *TCPConn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.maxIdleCount > 0 && p.maxIdleCount > len(p.idleConns) {
		p.idleConns[c.id] = c // put into the pool
	} else {
		c.conn.Close()
		c.pool.numOpen--
	}
}

// Get() retrieves a TCP connection
func (p *TcpConnPool) Get() (*TCPConn, error) {
	p.mu.Lock()

	// Case 1: Gets a free connection from the pool if any
	numIdle := len(p.idleConns)
	if numIdle > 0 {
		// Loop map to get one conn
		for _, c := range p.idleConns {
			// remove from pool
			delete(p.idleConns, c.id)
			p.mu.Unlock()
			return c, nil
		}
	}

	// Case 2: Queue a connection request
	if p.maxOpenCount > 0 && p.numOpen >= p.maxOpenCount {
		// Create the request
		req := &connRequest{
			connChan: make(chan *TCPConn, 1),
			errChan:  make(chan error, 1),
		}

		// Queue the request
		p.requestChan <- req

		p.mu.Unlock()

		// Waits for either
		// 1. Request fulfilled, or
		// 2. An error is returned
		select {
		case tcpConn := <-req.connChan:
			return tcpConn, nil
		case err := <-req.errChan:
			return nil, err
		}
	}

	// Case 3: Open a new connection
	p.numOpen++
	p.mu.Unlock()

	newTcpConn, err := p.openNewTcpConnection()
	if err != nil {
		p.mu.Lock()
		p.numOpen--
		p.mu.Unlock()
		return nil, err
	}

	return newTcpConn, nil

}

// openNewTcpConnection() creates a new TCP connection at p.address
func (p *TcpConnPool) openNewTcpConnection() (*TCPConn, error) {

	c, err := net.Dial("tcp", p.address)
	if err != nil {
		return nil, err
	}

	return &TCPConn{
		// Use unix time as id
		id:   fmt.Sprintf("%v", time.Now().UnixNano()),
		conn: c,
		pool: p,
	}, nil
}

var (
	errTimeout = errors.New("connection request timeout")
)

// handleConnectionRequest() listens to the request queue
// and attempts to fulfil any incoming requests
func (p *TcpConnPool) handleConnectionRequest() {
	for req := range p.requestChan {
		var (
			requestDone = false
			hasTimeout  = false

			// start a 3-second timeout
			timeoutChan = time.After(3 * time.Second)
		)

		for {
			if requestDone || hasTimeout {
				break
			}
			select {
			// request timeout
			case <-timeoutChan:
				hasTimeout = true
				req.errChan <- errTimeout
			default:
				// 1. get idle conn or open new conn
				// 2. if success, pass conn into req.conn. requestDone!
				// 3. if fail, we retry until timeout
				p.mu.Lock()

				// First, we try to get an idle conn.
				// If fail, we try to open a new conn.
				// If both does not work, we try again in the next loop until timeout.
				numIdle := len(p.idleConns)
				if numIdle > 0 {
					for _, c := range p.idleConns {
						delete(p.idleConns, c.id)
						p.mu.Unlock()
						req.connChan <- c // give conn
						requestDone = true
						break
					}
				} else if p.maxOpenCount > 0 && p.numOpen < p.maxOpenCount {
					p.numOpen++
					p.mu.Unlock()

					c, err := p.openNewTcpConnection()
					if err != nil {
						p.mu.Lock()
						p.numOpen--
						p.mu.Unlock()
					} else {
						req.connChan <- c // give conn
						requestDone = true
					}
				} else {
					p.mu.Unlock()
				}
			}
		}
	}
}

const maxQueueLength = 10_000

// TcpConfig is a set of configuration for a TCP connection pool
type TcpConfig struct {
	Address      string
	MaxIdleConns int
	MaxOpenConn  int
}

// CreateTcpConnPool() creates a connection pool
// and starts the worker that handles connection request
func CreateTcpConnPool(cfg *TcpConfig) (*TcpConnPool, error) {
	pool := &TcpConnPool{
		address:      cfg.Address,
		idleConns:    make(map[string]*TCPConn),
		requestChan:  make(chan *connRequest, maxQueueLength),
		maxOpenCount: cfg.MaxOpenConn,
		maxIdleCount: cfg.MaxIdleConns,
	}

	go pool.handleConnectionRequest()

	return pool, nil
}
