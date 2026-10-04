package server

import (
	"sync"

	"github.com/anonymous-beta/nonchat/internal/protocol"
)

// GeneralRoomCode is the reserved code for the always-open general lobby.
const GeneralRoomCode = "0000"

// Room is a broadcast group. It owns its own mutex and client set.
type Room struct {
	Code    string
	Name    string
	general bool

	mu      sync.RWMutex
	clients map[*ClientConn]struct{}
}

// NewRoom creates a room with the given code and display name.
func NewRoom(code, name string, general bool) *Room {
	return &Room{
		Code:    code,
		Name:    name,
		general: general,
		clients: make(map[*ClientConn]struct{}),
	}
}

// Add registers a client with the room.
func (r *Room) Add(c *ClientConn) {
	r.mu.Lock()
	r.clients[c] = struct{}{}
	r.mu.Unlock()
}

// Remove deregisters a client from the room.
func (r *Room) Remove(c *ClientConn) {
	r.mu.Lock()
	delete(r.clients, c)
	r.mu.Unlock()
}

// Broadcast sends a message to every client currently in the room.
func (r *Room) Broadcast(t string, payload any) {
	data, err := protocol.Encode(t, payload)
	if err != nil {
		return
	}
	r.mu.RLock()
	targets := make([]*ClientConn, 0, len(r.clients))
	for c := range r.clients {
		targets = append(targets, c)
	}
	r.mu.RUnlock()
	for _, c := range targets {
		_ = c.SendRaw(data)
	}
}

// Users returns the current usernames in the room.
func (r *Room) Users() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.clients))
	for c := range r.clients {
		out = append(out, c.Username())
	}
	return out
}

// Count returns how many clients are currently in the room.
func (r *Room) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}

// IsGeneral reports whether this is the reserved general lobby.
func (r *Room) IsGeneral() bool { return r.general }
