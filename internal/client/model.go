package client

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/anonymous-beta/nonchat/internal/protocol"
)

// IncomingMsg wraps a decoded protocol message for the TUI event loop.
type IncomingMsg struct{ Msg *protocol.Message }

// ConnErrMsg is delivered when the underlying TCP connection dies.
type ConnErrMsg struct{ Err error }

type lineKind string

const (
	lineSystem lineKind = "system"
	lineChat   lineKind = "chat"
	lineSelf   lineKind = "self"
	lineError  lineKind = "error"
)

type line struct {
	kind lineKind
	from string
	body string
	at   time.Time
}

// Model is the bubbletea model driving the nonchat TUI.
type Model struct {
	conn     *Conn
	username string
	roomCode string
	roomName string
	users    []string

	viewport viewport.Model
	input    textinput.Model
	lines    []line

	width  int
	height int

	ready    bool
	quitting bool
}

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("62"))

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Background(lipgloss.Color("236"))

	systemStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true)
	chatFromStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	chatBodyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	selfStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("120")).Bold(true)
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	timeStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

// NewModel builds a fresh TUI model. roomCode/roomName may be empty — the
// join response will fill them in.
func NewModel(conn *Conn, username, roomCode, roomName string) Model {
	ti := textinput.New()
	ti.Placeholder = "type a message… (/help for commands)"
	ti.CharLimit = 4000
	ti.Prompt = "▸ "
	ti.Focus()

	return Model{
		conn:     conn,
		username: username,
		roomCode: roomCode,
		roomName: roomName,
		input:    ti,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		m.ready = true
		return m, nil

	case IncomingMsg:
		m.handleIncoming(msg.Msg)
		return m, nil

	case ConnErrMsg:
		m.appendLine(line{
			kind: lineError,
			body: "disconnected from server",
			at:   time.Now(),
		})
		return m, tea.Quit

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			value := strings.TrimSpace(m.input.Value())
			m.input.SetValue("")
			if value == "" {
				return m, nil
			}
			cmd := m.submit(value)
			return m, cmd
		}
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd

	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if !m.ready {
		return dimStyle.Render("nonchat — connecting…")
	}

	header := m.renderHeader()
	footer := m.renderFooter()
	body := m.viewport.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		body,
		m.input.View(),
		footer,
	)
}

func (m *Model) layout() {
	// rows: header(1) + body(viewport) + input(1) + footer(1)
	bodyH := m.height - 3
	if bodyH < 3 {
		bodyH = 3
	}

	if !m.ready {
		m.viewport = viewport.New(m.width, bodyH)
	} else {
		m.viewport.Width = m.width
		m.viewport.Height = bodyH
	}

	m.input.Width = m.width - 4
	if m.input.Width < 10 {
		m.input.Width = 10
	}

	m.viewport.SetContent(m.renderLines())
	m.viewport.GotoBottom()
}

func (m *Model) appendLine(l line) {
	m.lines = append(m.lines, l)
	if len(m.lines) > 2000 {
		m.lines = m.lines[len(m.lines)-2000:]
	}
	if m.ready {
		m.viewport.SetContent(m.renderLines())
		m.viewport.GotoBottom()
	}
}

func (m *Model) renderLines() string {
	var b strings.Builder
	for _, l := range m.lines {
		b.WriteString(m.renderLine(l))
		b.WriteString("\n")
	}
	return b.String()
}

func (m *Model) renderLine(l line) string {
	ts := timeStyle.Render(l.at.Format("15:04"))

	var raw string
	switch l.kind {
	case lineSystem:
		raw = fmt.Sprintf("%s %s", ts, systemStyle.Render("· "+l.body))
	case lineError:
		raw = fmt.Sprintf("%s %s", ts, errStyle.Render("! "+l.body))
	case lineChat:
		raw = fmt.Sprintf("%s %s %s", ts, chatFromStyle.Render(l.from+":"), chatBodyStyle.Render(l.body))
	case lineSelf:
		raw = fmt.Sprintf("%s %s %s", ts, selfStyle.Render(l.from+":"), chatBodyStyle.Render(l.body))
	default:
		raw = l.body
	}

	if m.viewport.Width > 0 {
		raw = lipgloss.NewStyle().Width(m.viewport.Width).Render(raw)
	}
	return raw
}

func (m *Model) renderHeader() string {
	name := m.roomName
	if name == "" {
		name = "connecting…"
	}
	code := m.roomCode
	if code == "" {
		code = "—"
	}

	left := " NONCHAT "
	mid := fmt.Sprintf("· %s [%s] ", name, code)
	right := fmt.Sprintf("· you: %s · %d online ", m.username, len(m.users))

	content := left + mid + right
	pad := m.width - lipgloss.Width(content)
	if pad < 0 {
		pad = 0
	}
	return headerStyle.Render(content + strings.Repeat(" ", pad))
}

func (m *Model) renderFooter() string {
	left := " credit: Anonymous-beta (chinedu) "
	right := " /help for commands "
	pad := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		pad = 1
	}
	return footerStyle.Render(left + strings.Repeat(" ", pad) + right)
}

func (m *Model) handleIncoming(msg *protocol.Message) {
	switch msg.Type {
	case protocol.TypeChatEcho:
		var e protocol.ChatEcho
		if err := json.Unmarshal(msg.Payload, &e); err != nil {
			return
		}
		kind := lineChat
		if e.From == m.username {
			kind = lineSelf
		}
		m.appendLine(line{
			kind: kind,
			from: e.From,
			body: e.Content,
			at:   time.Unix(e.Time, 0),
		})

	case protocol.TypeSystem:
		var s protocol.SystemNotice
		if err := json.Unmarshal(msg.Payload, &s); err != nil {
			return
		}
		m.appendLine(line{kind: lineSystem, body: s.Content, at: time.Now()})

	case protocol.TypeUserList:
		var u protocol.UserList
		if err := json.Unmarshal(msg.Payload, &u); err != nil {
			return
		}
		m.users = u.Users

	case protocol.TypeJoinResponse:
		var j protocol.JoinResponse
		if err := json.Unmarshal(msg.Payload, &j); err != nil {
			return
		}
		if j.Success {
			m.username = j.Username
			m.roomCode = j.RoomCode
			m.roomName = j.RoomName
			m.appendLine(line{
				kind: lineSystem,
				body: fmt.Sprintf("joined %s [%s] as %s", j.RoomName, j.RoomCode, j.Username),
				at:   time.Now(),
			})
		} else {
			m.appendLine(line{kind: lineError, body: j.Error, at: time.Now()})
		}

	case protocol.TypeRoomCreated:
		var r protocol.RoomCreated
		if err := json.Unmarshal(msg.Payload, &r); err != nil {
			return
		}
		m.appendLine(line{
			kind: lineSystem,
			body: fmt.Sprintf("room created: %s — share that code to invite others", r.Code),
			at:   time.Now(),
		})

	case protocol.TypeError:
		var e protocol.ErrorMsg
		if err := json.Unmarshal(msg.Payload, &e); err != nil {
			return
		}
		m.appendLine(line{kind: lineError, body: e.Message, at: time.Now()})

	case protocol.TypePong:
		// no-op
	}
}

func (m *Model) submit(input string) tea.Cmd {
	if strings.HasPrefix(input, "/") {
		return m.handleCommand(input)
	}
	conn := m.conn
	content := input
	return func() tea.Msg {
		_ = conn.Send(protocol.TypeChat, protocol.Chat{Content: content})
		return nil
	}
}

func (m *Model) handleCommand(input string) tea.Cmd {
	parts := strings.Fields(input)
	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "/help":
		m.appendLine(line{
			kind: lineSystem,
			body: "commands: /create  /join <code>  /leave  /nick <name>  /users  /quit",
			at:   time.Now(),
		})
		return nil

	case "/create":
		conn := m.conn
		return func() tea.Msg {
			_ = conn.Send(protocol.TypeCreateRoom, nil)
			return nil
		}

	case "/join":
		if len(args) == 0 {
			m.appendLine(line{kind: lineError, body: "usage: /join <code>", at: time.Now()})
			return nil
		}
		conn := m.conn
		code := args[0]
		name := m.username
		return func() tea.Msg {
			_ = conn.Send(protocol.TypeJoinRequest, protocol.JoinRequest{
				Username: name,
				RoomCode: code,
			})
			return nil
		}

	case "/leave":
		conn := m.conn
		return func() tea.Msg {
			_ = conn.Send(protocol.TypeLeaveRoom, nil)
			return nil
		}

	case "/nick":
		if len(args) == 0 {
			m.appendLine(line{kind: lineError, body: "usage: /nick <name>", at: time.Now()})
			return nil
		}
		conn := m.conn
		name := args[0]
		code := m.roomCode
		return func() tea.Msg {
			_ = conn.Send(protocol.TypeJoinRequest, protocol.JoinRequest{
				Username: name,
				RoomCode: code,
			})
			return nil
		}

	case "/users":
		users := strings.Join(m.users, ", ")
		if users == "" {
			users = "(nobody)"
		}
		m.appendLine(line{kind: lineSystem, body: "online: " + users, at: time.Now()})
		return nil

	case "/quit":
		m.quitting = true
		return tea.Quit

	default:
		m.appendLine(line{kind: lineError, body: "unknown command: " + cmd, at: time.Now()})
		return nil
	}
}
