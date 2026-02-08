package tui

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nathfavour/zdns/pkg/ipc"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type tickMsg time.Time

type model struct {
	table      table.Model
	socketPath string
	error      error
}

func NewModel(socketPath string) model {
	columns := []table.Column{
		{Title: "Peer", Width: 15},
		{Title: "Battery", Width: 10},
		{Title: "State", Width: 10},
		{Title: "Services", Width: 20},
		{Title: "Latency", Width: 10},
		{Title: "ID", Width: 10},
		{Title: "Last Seen", Width: 15},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return model{
		table:      t,
		socketPath: socketPath,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.fetchStatus(), tick())
}

func tick() tea.Cmd {
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) fetchStatus() tea.Cmd {
	return func() tea.Msg {
		conn, err := net.Dial("unix", m.socketPath)
		if err != nil {
			return err
		}
		defer conn.Close()

		req := ipc.Request{Command: "list_peers"}
		json.NewEncoder(conn).Encode(req)

		var resp ipc.Response
		if err := json.NewDecoder(conn).Decode(&resp); err != nil {
			return err
		}
		return resp
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tickMsg:
		return m, tea.Batch(m.fetchStatus(), tick())
	case ipc.Response:
		rows := []table.Row{}
		for _, p := range msg.Peers {
			batColor := "42" // Green
			if p.Battery < 20 {
				batColor = "196" // Red
			} else if p.Battery < 50 {
				batColor = "214" // Orange
			}
			
			batStr := lipgloss.NewStyle().Foreground(lipgloss.Color(batColor)).Render(fmt.Sprintf("%d%%", p.Battery))
			
			stateIcon := "🔓"
			if p.State == "LOCKED" {
				stateIcon = "🔒"
			}

			lastSeen := time.Since(time.Unix(p.LastSeen, 0)).Round(time.Second).String() + " ago"

			latency := "???"
			if p.Latency > 0 {
				latency = fmt.Sprintf("%dms", p.Latency)
			}

			rows = append(rows, table.Row{
				p.Name,
				batStr,
				stateIcon + " " + p.State,
				p.Tags,
				latency,
				p.PublicKey,
				lastSeen,
			})
		}
		m.table.SetRows(rows)
	case error:
		m.error = msg
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Render(" zDNS Stealth Dashboard ")

	body := baseStyle.Render(m.table.View())
	
	footer := " q: quit | "
	if m.error != nil {
		footer += lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(fmt.Sprintf("Error: %v", m.error))
	} else {
		footer += "Daemon Connected"
	}

	return fmt.Sprintf("\n%s\n\n%s\n\n%s\n", header, body, footer)
}

func Run(socketPath string) error {
	p := tea.NewProgram(NewModel(socketPath))
	_, err := p.Run()
	return err
}
