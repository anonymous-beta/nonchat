package server

import (
	"bufio"
	"encoding/json"
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"github.com/anonymous-beta/nonchat/internal/protocol"
)

// ClientConn is a single connected client on the server side.
type ClientConn struct {
	conn net.Conn
	srv  *Server

	writeMu sync.Mutex
	closeMu sync.Mutex
	closed  bool

	mu       sync.RWMutex
	username string
	room     *Room
}

// NewClientConn wraps a net.Conn with nonchat client state.
func NewClientConn(conn net.Conn, srv *Server) *ClientConn {
	return &ClientConn{
		conn: conn,
		srv:  srv,
	}
}

// Username returns the current username.
func (c *ClientConn) Username() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.username
}

// SetUsername updates the client's username.
func (c *ClientConn) SetUsername(name string) {
	c.mu.Lock()
	c.username = name
	c.mu.Unlock()
}

// Room returns the room this client is currently in (may be nil).
func (c *ClientConn) Room() *Room {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.room
}

// SetRoom updates the client's current room.
func (c *ClientConn) SetRoom(r *Room) {
	c.mu.Lock()
	c.room = r
	c.mu.Unlock()
}

// SendRaw writes raw bytes plus a newline to the client. Thread-safe.
func (c *ClientConn) SendRaw(data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.closeMu.Lock()
	closed := c.closed
	c.closeMu.Unlock()
	if closed {
		return errors.New("connection closed")
	}

	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err := c.conn.Write(append(data, '\n'))
	return err
}

// Send encodes and writes a typed message.
func (c *ClientConn) Send(t string, payload any) error {
	data, err := protocol.Encode(t, payload)
	if err != nil {
		return err
	}
	return c.SendRaw(data)
}

// Close terminates the underlying connection exactly once.
func (c *ClientConn) Close() {
	c.closeMu.Lock()
	if c.closed {
		c.closeMu.Unlock()
		return
	}
	c.closed = true
	c.closeMu.Unlock()
	_ = c.conn.Close()
}

// ReadLoop reads newline-delimited JSON until the connection dies.
func (c *ClientConn) ReadLoop() {
	defer func() {
		c.srv.removeClient(c)
		c.Close()
	}()

	scanner := bufio.NewScanner(c.conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		msg, err := protocol.Decode(line)
		if err != nil {
			_ = c.Send(protocol.TypeError, protocol.ErrorMsg{Message: "malformed message"})
			continue
		}
		if err := c.handle(msg); err != nil {
			log.Printf("client %s handler error: %v", c.Username(), err)
		}
	}
}

func (c *ClientConn) handle(msg *protocol.Message) error {
	switch msg.Type {
	case protocol.TypeJoinRequest:
		var req protocol.JoinRequest
		if err := json.Unmarshal(msg.Payload, &req); err != nil {
			return err
		}
		return c.srv.handleJoin(c, req)

	case protocol.TypeChat:
		var chat protocol.Chat
		if err := json.Unmarshal(msg.Payload, &chat); err != nil {
			return err
		}
		return c.srv.handleChat(c, chat)

	case protocol.TypeCreateRoom:
		return c.srv.handleCreateRoom(c)

	case protocol.TypeLeaveRoom:
		return c.srv.handleLeaveRoom(c)

	case protocol.TypePing:
		return c.Send(protocol.TypePong, nil)

	default:
		return c.Send(protocol.TypeError, protocol.ErrorMsg{
			Message: "unknown message type: " + msg.Type,
		})
	}
}
