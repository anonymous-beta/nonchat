package client

import (
	"bufio"
	"net"
	"sync"
	"time"

	"github.com/anonymous-beta/nonchat/internal/protocol"
)

// Conn is a nonchat client-side TCP connection.
type Conn struct {
	conn net.Conn
	mu   sync.Mutex
}

// Dial opens a TCP connection to the given address and starts the read pump.
// onMsg is called for every decoded message. onClose is called once when the
// connection terminates (err may be nil for a clean close).
func Dial(addr string, onMsg func(*protocol.Message), onClose func(error)) (*Conn, error) {
	c, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	conn := &Conn{conn: c}
	go conn.readLoop(onMsg, onClose)
	return conn, nil
}

func (c *Conn) readLoop(onMsg func(*protocol.Message), onClose func(error)) {
	scanner := bufio.NewScanner(c.conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		m, err := protocol.Decode(line)
		if err != nil {
			continue
		}
		if onMsg != nil {
			onMsg(m)
		}
	}

	if onClose != nil {
		onClose(scanner.Err())
	}
}

// Send encodes and writes a typed message to the server.
func (c *Conn) Send(t string, payload any) error {
	data, err := protocol.Encode(t, payload)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err = c.conn.Write(append(data, '\n'))
	return err
}

// Close terminates the connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}
