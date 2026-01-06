package common

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/skdltmxn/taws/internal/config"
)

var (
	// Colors (initialized with defaults, can be updated by ApplyTheme)
	PrimaryColor   = lipgloss.Color("#7D56F4")
	SecondaryColor = lipgloss.Color("#6244C5")
	ErrorColor     = lipgloss.Color("#FF0000")
	SuccessColor   = lipgloss.Color("#3FB950")
	WarningColor   = lipgloss.Color("#F4BD2D")
	InfoColor      = lipgloss.Color("#3794FF")
	TextColor      = lipgloss.Color("#FFFFFF")
	SubTextColor   = lipgloss.Color("#888888")
	BorderColor    = lipgloss.Color("#888888")
	HeaderBgColor  = lipgloss.Color("#1a1a2e")

	// Styles
	DefaultStyle = lipgloss.NewStyle().Foreground(TextColor)

	TitleStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true).
			Padding(0, 1)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(TextColor).
			Background(SecondaryColor)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true)

	RenderError = func(err error) string {
		return ErrorStyle.Render(fmt.Sprintf("Error: %v", err))
	}

	RenderLoading = func(title string) string {
		return lipgloss.JoinVertical(lipgloss.Left,
			TitleStyle.Render(title),
			"Loading...",
		)
	}

	// Header styles
	HeaderStyle = lipgloss.NewStyle().
			Background(HeaderBgColor).
			Padding(0, 1)

	HeaderLabelStyle = lipgloss.NewStyle().
				Foreground(SubTextColor)

	HeaderValueStyle = lipgloss.NewStyle().
				Foreground(PrimaryColor).
				Bold(true)

	HeaderSeparatorStyle = lipgloss.NewStyle().
				Foreground(SubTextColor)

	// Command palette styles
	CmdPaletteStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(PrimaryColor).
			Padding(0, 1)

	CmdPaletteInputStyle = lipgloss.NewStyle().
				Foreground(TextColor)

	CmdPaletteSelectedStyle = lipgloss.NewStyle().
				Foreground(PrimaryColor).
				Bold(true)

	CmdPaletteItemStyle = lipgloss.NewStyle().
				Foreground(SubTextColor)

	CmdPaletteAliasStyle = lipgloss.NewStyle().
				Foreground(SubTextColor).
				Faint(true)

	// Content area style
	ContentStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(0, 1)
)

// ApplyTheme updates all colors and styles based on the given theme
func ApplyTheme(theme *config.Theme) {
	if theme == nil {
		theme = config.DefaultTheme()
	}

	// Update colors
	PrimaryColor = lipgloss.Color(theme.Colors.Primary)
	SecondaryColor = lipgloss.Color(theme.Colors.Secondary)
	ErrorColor = lipgloss.Color(theme.Colors.Error)
	SuccessColor = lipgloss.Color(theme.Colors.Success)
	WarningColor = lipgloss.Color(theme.Colors.Warning)
	InfoColor = lipgloss.Color(theme.Colors.Info)
	TextColor = lipgloss.Color(theme.Colors.Text)
	SubTextColor = lipgloss.Color(theme.Colors.SubText)
	BorderColor = lipgloss.Color(theme.Colors.Border)
	HeaderBgColor = lipgloss.Color(theme.Colors.HeaderBg)

	// Rebuild styles with new colors
	DefaultStyle = lipgloss.NewStyle().Foreground(TextColor)

	TitleStyle = lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true).
		Padding(0, 1)

	StatusBarStyle = lipgloss.NewStyle().
		Foreground(TextColor).
		Background(SecondaryColor)

	ErrorStyle = lipgloss.NewStyle().
		Foreground(ErrorColor).
		Bold(true)

	HeaderStyle = lipgloss.NewStyle().
		Background(HeaderBgColor).
		Padding(0, 1)

	HeaderLabelStyle = lipgloss.NewStyle().
		Foreground(SubTextColor)

	HeaderValueStyle = lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true)

	HeaderSeparatorStyle = lipgloss.NewStyle().
		Foreground(SubTextColor)

	CmdPaletteStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(0, 1)

	CmdPaletteInputStyle = lipgloss.NewStyle().
		Foreground(TextColor)

	CmdPaletteSelectedStyle = lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true)

	CmdPaletteItemStyle = lipgloss.NewStyle().
		Foreground(SubTextColor)

	CmdPaletteAliasStyle = lipgloss.NewStyle().
		Foreground(SubTextColor).
		Faint(true)

	ContentStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Padding(0, 1)
}

// DisplayValue returns "-" if the value is empty, otherwise returns the value
func DisplayValue(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

// SplitTitle extracts the first line as title and returns the remainder as body.
func SplitTitle(view string) (string, string) {
	if view == "" {
		return "", view
	}
	parts := strings.SplitN(view, "\n", 2)
	title := strings.TrimSpace(parts[0])
	if title == "" {
		return "", view
	}
	body := ""
	if len(parts) == 2 {
		body = strings.TrimPrefix(parts[1], "\n")
	}
	return title, body
}

// WithBorderTitle injects the given title into the top border of a rendered box.
func WithBorderTitle(rendered, title string) string {
	if title == "" {
		return rendered
	}

	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		return rendered
	}

	top := lines[0]
	border := lipgloss.RoundedBorder()
	contentWidth := lipgloss.Width(top)
	titleWidth := lipgloss.Width(title)

	usable := contentWidth - lipgloss.Width(border.TopLeft+border.TopRight)
	if usable <= titleWidth+2 {
		return rendered
	}

	leftFill := 2
	rightFill := usable - titleWidth - leftFill
	if rightFill < 1 {
		leftFill = 1
		rightFill = usable - titleWidth - leftFill
	}

	leftSeg := lipgloss.NewStyle().Foreground(BorderColor).Render(border.TopLeft + strings.Repeat(border.Top, leftFill))
	titleSeg := title
	rightSeg := lipgloss.NewStyle().Foreground(BorderColor).Render(strings.Repeat(border.Top, rightFill) + border.TopRight)

	lines[0] = lipgloss.JoinHorizontal(lipgloss.Left, leftSeg, titleSeg, rightSeg)
	return strings.Join(lines, "\n")
}

func DefaultTableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(BorderColor).
		Foreground(InfoColor).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(HeaderBgColor).
		Background(PrimaryColor).
		Bold(true)
	return s
}

func HandleTableNavKeys(t *table.Model, key string) bool {
	switch key {
	case "j":
		t.MoveDown(1)
		return true
	case "k":
		t.MoveUp(1)
		return true
	case "g":
		t.GotoTop()
		return true
	case "G":
		t.GotoBottom()
		return true
	default:
		return false
	}
}

func PlaceOverlay(x, y int, fg, bg string) string {
	fgLines := strings.Split(fg, "\n")
	bgLines := strings.Split(bg, "\n")

	for i, fgLine := range fgLines {
		bgY := y + i
		if bgY < 0 || bgY >= len(bgLines) {
			continue
		}
		bgLine := bgLines[bgY]
		bgRunes := []rune(bgLine)
		fgRunes := []rune(fgLine)

		for j, r := range fgRunes {
			bgX := x + j
			if bgX < 0 {
				continue
			}
			if bgX >= len(bgRunes) {
				bgRunes = append(bgRunes, make([]rune, bgX-len(bgRunes)+1)...)
			}
			bgRunes[bgX] = r
		}
		bgLines[bgY] = string(bgRunes)
	}

	return strings.Join(bgLines, "\n")
}
