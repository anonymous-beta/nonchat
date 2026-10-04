// nonchat client entrypoint — the TUI.
//
// Credit: Anonymous-beta (chinedu)
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/anonymous-beta/nonchat/internal/client"
	"github.com/anonymous-beta/nonchat/internal/protocol"
)

func main() {
	addr := flag.String("server", "127.0.0.1:7777", "nonchat server address")
	nick := flag.String("nick", "", "your username (prompted if empty)")
	room := flag.String("room", "", "room code to join (empty = general)")
	flag.Parse()

	if *nick == "" {
		fmt.Print("username: ")
		_, _ = fmt.Scanln(nick)
	}
	if *nick == "" {
		fmt.Fprintln(os.Stderr, "username required")
		os.Exit(1)
	}

	msgCh := make(chan *protocol.Message, 256)
	closeCh := make(chan error, 1)

	conn, err := client.Dial(
		*addr,
		func(m *protocol.Message) {
			select {
			case msgCh <- m:
			default:
			}
		},
		func(e error) {
			select {
			case closeCh <- e:
			default:
			}
		},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not connect to %s: %v\n", *addr, err)
		os.Exit(1)
	}
	defer conn.Close()

	// initial join
	if err := conn.Send(protocol.TypeJoinRequest, protocol.JoinRequest{
		Username: *nick,
		RoomCode: *room,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "join failed: %v\n", err)
		os.Exit(1)
	}

	model := client.NewModel(conn, *nick, *room, "")
	p := tea.NewProgram(model, tea.WithAltScreen())

	go func() {
		for m := range msgCh {
			p.Send(client.IncomingMsg{Msg: m})
		}
	}()

	go func() {
		err := <-closeCh
		p.Send(client.ConnErrMsg{Err: err})
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		os.Exit(1)
	}
}
