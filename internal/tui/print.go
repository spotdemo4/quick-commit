package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	ErrTextStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#d20f39", Dark: "#f38ba8"})
	WarnTextStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#df8e1d", Dark: "#f9e2af"})
)

func Print(msg string, ext ...any) {
	fmt.Println(TextStyle.Render(fmt.Sprintf(msg, ext...)))
}

func PrintErr(msg string, ext ...any) {
	fmt.Println(ErrTextStyle.Render(fmt.Sprintf(msg, ext...)))
}

func PrintWarn(msg string, ext ...any) {
	fmt.Println(WarnTextStyle.Render(fmt.Sprintf(msg, ext...)))
}
