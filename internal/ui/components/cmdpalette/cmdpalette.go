package cmdpalette

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/skdltmxn/taws/internal/ui/common"
)

type Command struct {
	Alias string
	Name  string
	Page  int
}

type SelectMsg struct {
	Page int
}

type CloseMsg struct{}

type Model struct {
	input    textinput.Model
	commands []Command
	filtered []Command
	cursor   int
	width    int
}

func NewModel(commands []Command) Model {
	ti := textinput.New()
	ti.Placeholder = "Type command or resource name..."
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 50
	ti.Prompt = ": "
	ti.PromptStyle = common.CmdPaletteInputStyle
	ti.TextStyle = common.CmdPaletteInputStyle

	return Model{
		input:    ti,
		commands: commands,
		filtered: commands,
		cursor:   0,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return CloseMsg{} }
		case "enter":
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				selected := m.filtered[m.cursor]
				return m, func() tea.Msg { return SelectMsg{Page: selected.Page} }
			}
			return m, nil
		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "ctrl+n":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	// Filter commands based on input
	m.filtered = m.filterCommands(m.input.Value())
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}

	return m, cmd
}

func (m Model) filterCommands(query string) []Command {
	if query == "" {
		return m.commands
	}

	query = strings.ToLower(query)
	var result []Command

	for _, cmd := range m.commands {
		// Exact alias match (highest priority)
		if strings.ToLower(cmd.Alias) == query {
			result = append([]Command{cmd}, result...)
			continue
		}

		// Alias prefix match
		if strings.HasPrefix(strings.ToLower(cmd.Alias), query) {
			result = append(result, cmd)
			continue
		}

		// Fuzzy match on name
		if fuzzyMatch(strings.ToLower(cmd.Name), query) {
			result = append(result, cmd)
		}
	}

	return result
}

func fuzzyMatch(str, pattern string) bool {
	pIdx := 0
	for sIdx := 0; sIdx < len(str) && pIdx < len(pattern); sIdx++ {
		if str[sIdx] == pattern[pIdx] {
			pIdx++
		}
	}
	return pIdx == len(pattern)
}

func (m Model) SetWidth(width int) Model {
	m.width = width
	inputWidth := min(50, width-10)
	m.input.Width = inputWidth
	return m
}

func (m Model) View() string {
	var items []string

	for i, cmd := range m.filtered {
		alias := common.CmdPaletteAliasStyle.Render("[" + cmd.Alias + "]")
		name := cmd.Name

		line := alias + " " + name

		if i == m.cursor {
			line = "> " + common.CmdPaletteSelectedStyle.Render(cmd.Name) + " " + alias
		} else {
			line = "  " + common.CmdPaletteItemStyle.Render(cmd.Name) + " " + alias
		}

		items = append(items, line)
	}

	itemsList := strings.Join(items, "\n")
	if len(items) == 0 {
		itemsList = common.CmdPaletteItemStyle.Render("  No matching commands")
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		m.input.View(),
		"",
		itemsList,
	)

	paletteWidth := min(60, m.width-4)
	if paletteWidth < 40 {
		paletteWidth = 40
	}

	return common.CmdPaletteStyle.Width(paletteWidth).Render(content)
}

func (m Model) Reset() Model {
	m.input.Reset()
	m.filtered = m.commands
	m.cursor = 0
	return m
}

func (m Model) Focus() tea.Cmd {
	return m.input.Focus()
}
