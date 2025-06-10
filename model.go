package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	main    lipgloss.Style
	footer  lipgloss.Style
	text    lipgloss.Style
	subtext lipgloss.Style
	accent  lipgloss.Style

	spinner   spinner.Model
	stopwatch stopwatch.Model
	width     *int
	height    *int

	retryChan chan int
	msgChan   chan Msg
	msgs      []Msg
}

func NewModel(msgChan chan Msg, retryChan chan int) Model {
	main := lipgloss.NewStyle().Padding(1)
	footer := lipgloss.NewStyle().Align(lipgloss.Center)
	text := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#4c4f69", Dark: "#cdd6f4"})
	subtext := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#6c6f85", Dark: "#a6adc8"})
	accent := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#04a5e5", Dark: "#89dceb"})
	s := spinner.New(spinner.WithSpinner(spinner.Dot))
	sw := stopwatch.New()

	return Model{
		main:    main,
		footer:  footer,
		text:    text,
		subtext: subtext,
		accent:  accent,

		spinner:   s,
		stopwatch: sw,
		retryChan: retryChan,
		msgChan:   msgChan,
		msgs:      []Msg{},
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.stopwatch.Init(),
		func() tea.Msg {
			return Msg(<-m.msgChan)
		},
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {

	case Msg:
		cmds := []tea.Cmd{}

		m.msgs = append(m.msgs, msg)

		switch msg.kind {
		case MsgLoading:
			cmds = append(cmds, m.stopwatch.Reset())
			cmds = append(cmds, m.stopwatch.Start())
		case MsgDone:
			cmds = append(cmds, m.stopwatch.Stop())
		}

		cmds = append(cmds, func() tea.Msg {
			return Msg(<-m.msgChan)
		})

		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		lastMsg := m.msgs[len(m.msgs)-1]

		switch msg.String() {

		case "n", "N":
			if lastMsg.kind == MsgDone {
				m.msgs = []Msg{} // empty msgs array
				m.retryChan <- 1
			}

		case "q", "esc", "ctrl+c":
			return m, tea.Quit

		default:
			if lastMsg.kind == MsgDone {
				m.retryChan <- 0
			}
		}

		return m, cmd

	case tea.WindowSizeMsg:
		m.width = &msg.Width
		m.height = &msg.Height
		return m, cmd

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	default:
		m.stopwatch, cmd = m.stopwatch.Update(msg)
		return m, cmd
	}
}

func (m Model) View() string {
	if m.width == nil || m.height == nil {
		return ""
	}

	main := m.main.Width(*m.width)

	// If no messages have been generated yet
	if len(m.msgs) == 0 {
		main := main.Height(*m.height).Align(lipgloss.Center, lipgloss.Center)
		return main.Render(m.spinner.View())
	}

	// Format messages
	msgs := ""
	for _, msg := range m.msgs {
		switch msg.kind {
		case MsgThought:
			msgs += m.subtext.Render(msg.text)
		case MsgCommit:
			msgs += m.accent.Render(msg.text)
		}
	}
	msgs += "\n\n"

	// Set width & height for footer
	main = main.Height(*m.height - 1)
	footer := m.footer.Width(*m.width)

	lastMsg := m.msgs[len(m.msgs)-1]
	switch lastMsg.kind {

	case MsgLoading:
		main = main.Align(lipgloss.Center, lipgloss.Center)
		elapsed := m.subtext.Render(fmt.Sprintf("elapsed %s", m.stopwatch.View()))
		return lipgloss.JoinVertical(lipgloss.Top, main.Render(m.spinner.View()), footer.Render(elapsed))

	case MsgDone:
		final := m.accent.Bold(true).Render(lastMsg.text) + "\n"
		retry := m.text.Render("looks good? (Y/n): ")
		elapsed := m.subtext.Render(fmt.Sprintf("took: %s", m.stopwatch.View()))
		return lipgloss.JoinVertical(lipgloss.Top, main.Render(msgs+final+retry), footer.Render(elapsed))

	default:
		elapsed := m.subtext.Render(fmt.Sprintf("elapsed: %s", m.stopwatch.View()))
		return lipgloss.JoinVertical(lipgloss.Top, main.Render(msgs), footer.Render(elapsed))
	}
}

var ErrStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#d20f39", Dark: "#f38ba8"})

func printErr(msg string, ext ...any) {
	fmt.Println(ErrStyle.Render(fmt.Sprintf(msg, ext...)))
	os.Exit(1)
}

var WarnStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#df8e1d", Dark: "#f9e2af"})

func printWarn(msg string, ext ...any) {
	fmt.Println(WarnStyle.Render(fmt.Sprintf(msg, ext...)))
}
