package common

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type TableSearch struct {
	Input  textinput.Model
	Active bool
}

func NewTableSearch() TableSearch {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 50
	ti.Width = 30
	ti.Prompt = "/ "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(InfoColor)
	ti.TextStyle = lipgloss.NewStyle().Foreground(TextColor)
	return TableSearch{Input: ti}
}

func (s TableSearch) SetWidth(width int) TableSearch {
	inputWidth := max(min(50, width-10), 10)
	s.Input.Width = inputWidth
	return s
}

func (s TableSearch) Query() string {
	return strings.TrimSpace(s.Input.Value())
}

func (s TableSearch) Start() (TableSearch, tea.Cmd) {
	s.Active = true
	return s, s.Input.Focus()
}

func (s TableSearch) Stop() TableSearch {
	s.Active = false
	s.Input.Blur()
	return s
}

func (s TableSearch) Reset() TableSearch {
	s.Input.Reset()
	return s
}

func (s TableSearch) Cancel() TableSearch {
	s.Input.Reset()
	return s.Stop()
}

func (s TableSearch) Update(msg tea.Msg) (TableSearch, tea.Cmd) {
	var cmd tea.Cmd
	s.Input, cmd = s.Input.Update(msg)
	return s, cmd
}

func (s TableSearch) Indicator() string {
	q := s.Query()
	if q == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(InfoColor).Render(fmt.Sprintf("filter: %s", q))
}

func TableRowMatches(row table.Row, query string) bool {
	q := strings.TrimSpace(query)
	if q == "" {
		return true
	}
	q = strings.ToLower(q)
	for _, cell := range row {
		if strings.Contains(strings.ToLower(cell), q) {
			return true
		}
	}
	return false
}

func MultiSelectLineStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(WarningColor)
}

func HighlightSelectedTableRows(view string, cursor int, rowKeys []string, selected map[string]struct{}) string {
	if view == "" || len(rowKeys) == 0 || len(selected) == 0 {
		return view
	}

	lines := strings.Split(view, "\n")
	if len(lines) < 3 {
		return view
	}

	sepIdx := -1
	for i := range lines {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			continue
		}
		all := true
		for _, r := range t {
			if r != '─' {
				all = false
				break
			}
		}
		if all {
			sepIdx = i
			break
		}
	}
	if sepIdx == -1 {
		sepIdx = 1
	}

	dataStart := sepIdx + 1
	if dataStart >= len(lines) {
		return view
	}

	dataLines := lines[dataStart:]
	visibleCount := len(dataLines)
	if visibleCount <= 0 {
		return view
	}

	maxStart := 0
	if len(rowKeys) > visibleCount {
		maxStart = len(rowKeys) - visibleCount
	}
	start := cursor - visibleCount + 1
	if start < 0 {
		start = 0
	}
	if start > maxStart {
		start = maxStart
	}

	style := MultiSelectLineStyle()
	for i := 0; i < visibleCount; i++ {
		global := start + i
		if global < 0 || global >= len(rowKeys) {
			continue
		}
		key := rowKeys[global]
		if _, ok := selected[key]; ok {
			dataLines[i] = style.Render(dataLines[i])
		}
	}

	out := append([]string{}, lines[:dataStart]...)
	out = append(out, dataLines...)
	return strings.Join(out, "\n")
}
