// Package protocol defines the wire format for nonchat.
// Every message is a single newline-terminated JSON object.
//
// Credit: Anonymous-beta (chinedu)
package protocol

import "encoding/json"

// Message is the envelope for every wire message.
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Message type constants.
const (
	TypeJoinRequest  = "join_request"
	TypeJoinResponse = "join_response"
	TypeChat         = "chat"
	TypeChatEcho     = "chat_echo"
	TypeCreateRoom   = "create_room"
	TypeRoomCreated  = "room_created"
	TypeLeaveRoom    = "leave_room"
	TypeSystem       = "system"
	TypeUserList     = "user_list"
	TypeError        = "error"
	TypePing         = "ping"
	TypePong         = "pong"
)

// JoinRequest is sent by the client to enter a room.
type JoinRequest struct {
	Username string `json:"username"`
	RoomCode string `json:"room_code,omitempty"`
}

// JoinResponse is the server's reply to a JoinRequest.
type JoinResponse struct {
	Success  bool   `json:"success"`
	Username string `json:"username"`
	RoomCode string `json:"room_code"`
	RoomName string `json:"room_name"`
	Error    string `json:"error,omitempty"`
}

// Chat is a message from a client to be broadcast to the room.
type Chat struct {
	Content string `json:"content"`
}

// ChatEcho is a message broadcast from the server to room members.
type ChatEcho struct {
	From    string `json:"from"`
	Content string `json:"content"`
	Room    string `json:"room"`
	Time    int64  `json:"time"`
}

// RoomCreated is sent when a new room is allocated.
type RoomCreated struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// SystemNotice is an out-of-band message shown to the user (joins, leaves).
type SystemNotice struct {
	Content string `json:"content"`
}

// UserList is the current list of usernames in a room.
type UserList struct {
	Users []string `json:"users"`
	Room  string   `json:"room"`
}

// ErrorMsg is a generic server-side error.
type ErrorMsg struct {
	Message string `json:"message"`
}

// Encode marshals a type + payload into a single wire-ready byte slice.
func Encode(t string, payload any) ([]byte, error) {
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		raw = b
	}
	msg := Message{Type: t, Payload: raw}
	return json.Marshal(msg)
}

// Decode parses a single wire line into a Message.
func Decode(data []byte) (*Message, error) {
	var m Message
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
