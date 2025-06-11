package tui

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	titleStyle        = TextStyle.MarginLeft(2)
	itemStyle         = AltTextStyle.PaddingLeft(4)
	selectedItemStyle = AccentTextStyle.PaddingLeft(2)
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
)

func NewPicker(values []string) picker {
	// Turn list values into items
	items := []list.Item{}
	minWidth := 9
	for _, keyword := range values {
		c := utf8.RuneCountInString(keyword)
		if c > minWidth {
			minWidth = c
		}

		items = append(items, item(keyword))
	}
	minHeight := len(items) + 4

	// Create list
	l := list.New(items, itemDelegate{}, minWidth, minHeight)
	l.Title = "Type"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle

	return picker{
		List:      l,
		minWidth:  minWidth,
		minHeight: minHeight,
	}
}

type picker struct {
	List list.Model

	minWidth  int
	minHeight int
}

func (p *picker) Update(width int, height int) {
	if width < p.List.Width() {
		p.List.SetWidth(width)
	} else {
		p.List.SetWidth(p.minWidth)
	}

	if height < p.List.Height()-1 {
		p.List.SetHeight(height - 1)
	} else {
		p.List.SetHeight(p.minHeight)
	}
}

type item string

func (i item) FilterValue() string { return "" }

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%s", i)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("> " + strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}
