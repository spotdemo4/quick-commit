package tui

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	BodyStyle   = lipgloss.NewStyle().Padding(1)
	FooterStyle = lipgloss.NewStyle().Align(lipgloss.Center)

	TextStyle       = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#4c4f69", Dark: "#cdd6f4"})
	SubtextStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#6c6f85", Dark: "#a6adc8"})
	AltTextStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#5c5f77", Dark: "#bac2de"})
	AccentTextStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#04a5e5", Dark: "#89dceb"})

	ButtonStyle = lipgloss.NewStyle().
			PaddingLeft(1).
			PaddingRight(1).
			Background(lipgloss.AdaptiveColor{Light: "#ccd0da", Dark: "#313244"}).
			Foreground(lipgloss.AdaptiveColor{Light: "#4c4f69", Dark: "#cdd6f4"})
	AccentButtonStyle = lipgloss.NewStyle().
				PaddingLeft(1).
				PaddingRight(1).
				Background(lipgloss.AdaptiveColor{Light: "#04a5e5", Dark: "#89dceb"}).
				Foreground(lipgloss.AdaptiveColor{Light: "#dce0e8", Dark: "#11111b"})
)

type Tui struct {
	spinner   spinner.Model
	stopwatch stopwatch.Model
	picker    Picker
	width     *int
	height    *int

	input     chan string
	output    chan Msg
	msgs      []Msg
	version   string
	keyword   string
	looksGood bool
}

func New(version string, keywords []string, input chan string, output chan Msg) Tui {
	s := spinner.New(spinner.WithSpinner(spinner.Dot))
	sw := stopwatch.New()
	p := NewPicker(keywords)

	return Tui{
		spinner:   s,
		stopwatch: sw,
		picker:    p,

		input:   input,
		output:  output,
		msgs:    []Msg{},
		version: version,
	}
}

func (m Tui) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.stopwatch.Init(),
		textinput.Blink,
		func() tea.Msg {
			return Msg(<-m.output)
		},
	)
}

func (m Tui) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	cmds := []tea.Cmd{}

	switch msg := msg.(type) {

	case Msg:
		m.msgs = append(m.msgs, msg)

		switch msg.Type {

		case MsgLoading:
			cmds = append(cmds, m.stopwatch.Reset())
			cmds = append(cmds, m.stopwatch.Start())

		case MsgCommit:
			m.looksGood = true
			cmds = append(cmds, m.stopwatch.Stop())
		}

		cmds = append(cmds, func() tea.Msg {
			return Msg(<-m.output)
		})

	case tea.KeyMsg:
		// If this is a keyword selection
		if len(m.msgs) == 0 && msg.String() == "enter" {
			i, ok := m.picker.List.SelectedItem().(item)
			if ok {
				m.keyword = string(i)
				m.input <- string(i)
				break
			}
		}

		// If this is a confirmation message
		if len(m.msgs) != 0 {
			lastMsg := m.msgs[len(m.msgs)-1]
			if lastMsg.Type == MsgCommit {
				switch msg.String() {
				case "enter":
					if !m.looksGood {
						m.msgs = []Msg{} // clear prior messages
					}

					m.input <- strconv.FormatBool(m.looksGood)

				case "left":
					m.looksGood = true

				case "right":
					m.looksGood = false
				}

				break
			}
		}

		switch msg.String() {
		case "q", "esc", "ctrl+c":
			cmds = append(cmds, tea.Quit)
		}

	case tea.WindowSizeMsg:
		m.width = &msg.Width
		m.height = &msg.Height
		m.picker.Update(msg.Width, msg.Height)

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	m.stopwatch, cmd = m.stopwatch.Update(msg)
	cmds = append(cmds, cmd)

	m.picker.List, cmd = m.picker.List.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Tui) View() string {
	// If the keyword has not been set yet
	if m.keyword == "" {
		return m.render(renderParams{
			body:   m.picker.List.View(),
			footer: AltTextStyle.Render(fmt.Sprintf("quick commit v%s", m.version)),
			center: true,
		})
	}

	// If no messages have been generated yet
	if len(m.msgs) == 0 {
		return m.render(renderParams{
			body:   m.spinner.View(),
			center: true,
		})
	}

	lastMsg := m.msgs[len(m.msgs)-1]
	switch lastMsg.Type {

	case MsgLoading:
		return m.render(renderParams{
			body:   m.spinner.View(),
			footer: AltTextStyle.Render(fmt.Sprintf("%s elapsed", m.stopwatch.View())),
			center: true,
		})

	case MsgCommit:
		commitMsg := AccentTextStyle.Render(lastMsg.Text) + "\n\n"
		retry := TextStyle.Render("looks good?") + "\n\n"

		var leftButton string
		var rightButton string
		if m.looksGood {
			leftButton = AccentButtonStyle.Render("Yes")
			rightButton = ButtonStyle.Render("No")
		} else {
			leftButton = ButtonStyle.Render("Yes")
			rightButton = AccentButtonStyle.Render("No")
		}
		buttons := lipgloss.JoinHorizontal(lipgloss.Center, leftButton+" ", rightButton)

		return m.render(renderParams{
			body:   commitMsg + retry + buttons,
			footer: AltTextStyle.Render(fmt.Sprintf("took %s", m.stopwatch.View())),
			center: true,
		})

	default:
		// Format messages
		msgs := ""
		for _, msg := range m.msgs {
			switch msg.Type {
			case MsgThought:
				msgs += SubtextStyle.Render(msg.Text)
			case MsgResponse:
				msgs += TextStyle.Render(msg.Text)
			}
		}

		return m.render(renderParams{
			body:   msgs,
			footer: AltTextStyle.Render(fmt.Sprintf("%s elapsed", m.stopwatch.View())),
		})
	}
}

type renderParams struct {
	body   string
	footer string
	center bool
}

func (m Tui) render(p renderParams) string {
	if m.width == nil || m.height == nil {
		return ""
	}

	bodyStyle := BodyStyle.Width(*m.width).Height(*m.height - 1)
	if p.center {
		bodyStyle = bodyStyle.Align(lipgloss.Center, lipgloss.Center)
	}

	footerStyle := FooterStyle.Width(*m.width)

	return lipgloss.JoinVertical(lipgloss.Top, bodyStyle.Render(p.body), footerStyle.Render(p.footer))
}
