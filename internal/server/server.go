package server

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net"
	"strings"
	"sync"
	"unicode"

	"github.com/anonymous-beta/nonchat/internal/protocol"
)

// Server is the nonchat TCP server.
type Server struct {
	addr     string
	listener net.Listener

	mu     sync.RWMutex
	rooms  map[string]*Room
	closed bool
}

// New creates a server bound to the given address and pre-seeds the general lobby.
func New(addr string) *Server {
	s := &Server{
		addr:  addr,
		rooms: make(map[string]*Room),
	}
	s.rooms[GeneralRoomCode] = NewRoom(GeneralRoomCode, "general", true)
	return s
}

// ListenAndServe blocks, accepting connections until Shutdown is called.
func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.listener = ln
	log.Printf("nonchat server listening on %s", s.addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			s.mu.RLock()
			closed := s.closed
			s.mu.RUnlock()
			if closed {
				return nil
			}
			log.Printf("accept error: %v", err)
			continue
		}
		c := NewClientConn(conn, s)
		go c.ReadLoop()
	}
}

// Shutdown stops the listener and lets existing connections drain naturally.
func (s *Server) Shutdown() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	if s.listener != nil {
		_ = s.listener.Close()
	}
}

func (s *Server) removeClient(c *ClientConn) {
	room := c.Room()
	if room == nil {
		return
	}
	username := c.Username()
	room.Remove(c)

	if username == "" {
		return
	}
	room.Broadcast(protocol.TypeSystem, protocol.SystemNotice{
		Content: username + " left the room",
	})
	room.Broadcast(protocol.TypeUserList, protocol.UserList{
		Users: room.Users(),
		Room:  room.Code,
	})
}

func (s *Server) handleJoin(c *ClientConn, req protocol.JoinRequest) error {
	name := sanitizeUsername(req.Username)
	if name == "" {
		name = randomGuestName()
	}

	code := strings.TrimSpace(req.RoomCode)
	if code == "" {
		code = GeneralRoomCode
	}

	s.mu.RLock()
	room, ok := s.rooms[code]
	s.mu.RUnlock()

	if !ok {
		return c.Send(protocol.TypeJoinResponse, protocol.JoinResponse{
			Success: false,
			Error:   "no such room: " + code,
		})
	}

	// If the client was already somewhere else, leave the old room first.
	if prev := c.Room(); prev != nil && prev != room {
		oldName := c.Username()
		prev.Remove(c)
		if oldName != "" {
			prev.Broadcast(protocol.TypeSystem, protocol.SystemNotice{
				Content: oldName + " left the room",
			})
			prev.Broadcast(protocol.TypeUserList, protocol.UserList{
				Users: prev.Users(),
				Room:  prev.Code,
			})
		}
	}

	c.SetUsername(name)
	c.SetRoom(room)
	room.Add(c)

	if err := c.Send(protocol.TypeJoinResponse, protocol.JoinResponse{
		Success:  true,
		Username: name,
		RoomCode: room.Code,
		RoomName: room.Name,
	}); err != nil {
		return err
	}

	room.Broadcast(protocol.TypeSystem, protocol.SystemNotice{
		Content: name + " joined the room",
	})
	room.Broadcast(protocol.TypeUserList, protocol.UserList{
		Users: room.Users(),
		Room:  room.Code,
	})
	return nil
}

func (s *Server) handleChat(c *ClientConn, chat protocol.Chat) error {
	room := c.Room()
	if room == nil {
		return c.Send(protocol.TypeError, protocol.ErrorMsg{Message: "you are not in a room"})
	}

	content := strings.TrimSpace(chat.Content)
	if content == "" {
		return nil
	}
	if len(content) > 4000 {
		content = content[:4000]
	}

	room.Broadcast(protocol.TypeChatEcho, protocol.ChatEcho{
		From:    c.Username(),
		Content: content,
		Room:    room.Code,
		Time:    nowUnix(),
	})
	return nil
}

func (s *Server) handleCreateRoom(c *ClientConn) error {
	code, err := s.generateRoomCode()
	if err != nil {
		return c.Send(protocol.TypeError, protocol.ErrorMsg{
			Message: "could not allocate a room code, try again",
		})
	}
	room := NewRoom(code, "room-"+code, false)

	s.mu.Lock()
	s.rooms[code] = room
	s.mu.Unlock()

	return c.Send(protocol.TypeRoomCreated, protocol.RoomCreated{
		Code: code,
		Name: room.Name,
	})
}

func (s *Server) handleLeaveRoom(c *ClientConn) error {
	current := c.Room()
	if current == nil {
		return c.Send(protocol.TypeError, protocol.ErrorMsg{Message: "not in a room"})
	}
	if current.IsGeneral() {
		return c.Send(protocol.TypeError, protocol.ErrorMsg{Message: "you are already in general"})
	}
	return s.handleJoin(c, protocol.JoinRequest{
		Username: c.Username(),
		RoomCode: GeneralRoomCode,
	})
}

func (s *Server) generateRoomCode() (string, error) {
	for i := 0; i < 200; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10000))
		if err != nil {
			return "", err
		}
		code := fmt.Sprintf("%04d", n.Int64())
		if code == GeneralRoomCode {
			continue
		}
		s.mu.RLock()
		_, exists := s.rooms[code]
		s.mu.RUnlock()
		if !exists {
			return code, nil
		}
	}
	return "", fmt.Errorf("no free room codes")
}

func sanitizeUsername(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' {
			b.WriteRune(r)
		}
		if b.Len() >= 24 {
			break
		}
	}
	return b.String()
}

func randomGuestName() string {
	n, err := rand.Int(rand.Reader, big.NewInt(9999))
	if err != nil {
		return "guest"
	}
	return fmt.Sprintf("guest%04d", n.Int64())
}
