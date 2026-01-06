package header

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/skdltmxn/taws/internal/buildinfo"
	"github.com/skdltmxn/taws/internal/ui/common"
)

type Info struct {
	AccountID    string
	Region       string
	Profile      string
	UserARN      string
	ResourceName string
}

func Render(info Info, width int) string {
	labelStyle := lipgloss.NewStyle().
		Foreground(common.WarningColor)

	valueStyle := lipgloss.NewStyle().
		Foreground(common.TextColor)

	logoStyle := lipgloss.NewStyle().
		Foreground(common.PrimaryColor).
		Bold(true)

	rawRightLines := []string{
		" ______  ______  __     __  ______   ",
		"/\\__  _\\/\\  __ \\/\\ \\  _ \\ \\/\\  ___\\  ",
		"\\/_/\\ \\/\\ \\  __ \\ \\ \\/ \".\\ \\ \\___  \\ ",
		"   \\ \\_\\ \\ \\_\\ \\_\\ \\__/\".~\\_\\/\\_____\\",
		"    \\/_/  \\/_/\\/_/\\/_/   \\/_/\\/_____/",
	}

	maxRightWidth := max(width-1, 0)

	rightLines := make([]string, 0, 5)
	for _, l := range rawRightLines {
		rightLines = append(rightLines, logoStyle.Render(takeByWidth(l, maxRightWidth)))
	}

	rightWidth := 0
	for _, l := range rightLines {
		if w := lipgloss.Width(l); w > rightWidth {
			rightWidth = w
		}
	}

	maxLeftWidth := max(width-rightWidth-1, 0)

	line := func(label, value string) string {
		labelText := label + ": "
		labelWidth := lipgloss.Width(labelText)
		remaining := maxLeftWidth - labelWidth
		return labelStyle.Render(labelText) + valueStyle.Render(truncate(value, remaining))
	}

	leftLines := []string{
		line("Account", info.AccountID),
		line("Region", info.Region),
		line("Profile", info.Profile),
		line("User", info.UserARN),
		line("Version", buildinfo.UIString()),
	}

	rows := make([]string, 0, 5)
	for i := range 5 {
		l := leftLines[i]
		r := rightLines[i]
		gap := width - lipgloss.Width(l) - lipgloss.Width(r)
		if gap < 1 {
			gap = 1
		}
		rows = append(rows, l+strings.Repeat(" ", gap)+r)
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}

	if maxWidth <= 3 {
		return takeByWidth(s, maxWidth)
	}

	return takeByWidth(s, maxWidth-3) + "..."
}

func takeByWidth(s string, maxWidth int) string {
	if maxWidth <= 0 || s == "" {
		return ""
	}

	w := 0
	end := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			break
		}
		rw := lipgloss.Width(string(r))
		if w+rw > maxWidth {
			break
		}
		w += rw
		i += size
		end = i
	}
	return s[:end]
}
