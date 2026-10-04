# NONCHAT

> terminal-based ephemeral chat rooms. anonymous by default.
> **credit: Anonymous-beta (chinedu)**

nonchat is a lightweight, self-hostable chat system with a real TUI client.
spin up a server, share the address, and start talking. create private rooms
with a 4-digit code, or hang out in the general lobby.

## features

- **real TUI** — built with [bubbletea](https://github.com/charmbracelet/bubbletea) + [lipgloss](https://github.com/charmbracelet/lipgloss). not ascii, not a prompt loop.
- **ephemeral rooms** — 4-digit codes, generated on demand, freed when empty.
- **general lobby** — always-on `0000` room for public chat.
- **anonymous** — pick any username you want. nothing is stored.
- **single binary per role** — server and client, no runtime deps.
- **pure Go stdlib net + JSON protocol** — inspectable with `nc` if you're bored.

## build

```sh
git clone https://github.com/anonymous-beta/nonchat
cd nonchat
go mod tidy
go build -o nonchat-server ./cmd/server
go build -o nonchat-client ./cmd/client
```

## run

start the server:

```sh
./nonchat-server -addr :7777
```

connect a client:

```sh
./nonchat-client -server 127.0.0.1:7777 -nick he
```

connect straight into a room:

```sh
./nonchat-client -server 127.0.0.1:7777 -nick he -room 1234
```

## in-app commands

| command | effect |
|---|---|
| `/help` | show the command list |
| `/create` | allocate a new 4-digit room, get the code |
| `/join <code>` | join an existing room by code |
| `/leave` | return to general |
| `/nick <name>` | change your username |
| `/users` | list who's in the room |
| `/quit` | disconnect |

## protocol

newline-delimited JSON. one object per line. example frames:

```json
{"type":"join_request","payload":{"username":"he","room_code":"1234"}}
{"type":"chat","payload":{"content":"hello"}}
{"type":"chat_echo","payload":{"from":"he","content":"hello","room":"1234","time":1730000000}}
```

see `internal/protocol/protocol.go` for the full schema.

## architecture

```
cmd/server   → tcp listener, one goroutine per connection
cmd/client   → bubbletea tui, one read-pump goroutine
internal/
  protocol   → wire types
  server     → room registry, broadcast, join/leave
  client     → conn wrapper + tui model
```

rooms live in memory only. when the server dies, the rooms die with it.
that's the point.

## license

do what you want. keep the credit.
