package cloudwatch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/skdltmxn/taws/internal/app"
	"github.com/skdltmxn/taws/internal/domain"
	"github.com/skdltmxn/taws/internal/ui/common"
)

type ViewState int

const (
	ViewLogGroups ViewState = iota
	ViewTail
)

type Model struct {
	app *app.App

	table table.Model
	state ViewState

	logGroups       []domain.CloudWatchLogGroup
	visibleGroupKey []string
	selectedGroup   map[string]struct{}

	events         []domain.CloudWatchLogEvent
	visibleEventID []string
	selectedEvent  map[string]struct{}
	seenEvent      map[string]struct{}

	currentGroup string

	search       common.TableSearch
	loading      bool
	err          error
	width        int
	height       int
	tailing      bool
	tailFetching bool
	lastEventAt  time.Time
}

type logGroupsLoadedMsg []domain.CloudWatchLogGroup
type eventsLoadedMsg []domain.CloudWatchLogEvent
type tailTickMsg struct{}
type errMsg error

func NewModel(a *app.App) Model {
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Name", Width: 60},
			{Title: "Retention", Width: 12},
			{Title: "Stored", Width: 12},
		}),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	t.SetStyles(common.DefaultTableStyles())

	return Model{
		app:           a,
		table:         t,
		state:         ViewLogGroups,
		selectedGroup: make(map[string]struct{}),
		selectedEvent: make(map[string]struct{}),
		seenEvent:     make(map[string]struct{}),
		search:        common.NewTableSearch(),
	}
}

func (m Model) Init() tea.Cmd {
	return m.fetchLogGroups
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case logGroupsLoadedMsg:
		m.logGroups = msg
		m.loading = false
		m.err = nil
		m.state = ViewLogGroups
		m = m.SetSize(m.width, m.height)
		m = m.applyFilter()
		return m, nil

	case eventsLoadedMsg:
		m.loading = false
		m.err = nil
		m.tailFetching = false

		if m.state != ViewTail {
			return m, nil
		}

		maxAt := m.lastEventAt
		for _, e := range msg {
			key := eventKey(e)
			if _, ok := m.seenEvent[key]; ok {
				continue
			}
			m.seenEvent[key] = struct{}{}
			m.events = append(m.events, e)
			if e.Timestamp.After(maxAt) {
				maxAt = e.Timestamp
			}
		}
		m.lastEventAt = maxAt
		m = m.capEvents(500)
		m = m.applyFilter()

		if m.tailing {
			return m, m.tickTail()
		}
		return m, nil

	case tailTickMsg:
		if m.state != ViewTail || !m.tailing {
			return m, nil
		}
		if m.tailFetching {
			return m, m.tickTail()
		}

		start := m.lastEventAt
		if start.IsZero() {
			start = time.Now().Add(-5 * time.Minute)
		} else {
			start = start.Add(-2 * time.Second)
		}
		m.tailFetching = true
		return m, tea.Batch(
			m.fetchEvents(m.currentGroup, start),
			m.tickTail(),
		)

	case errMsg:
		m.err = msg
		m.loading = false
		m.tailFetching = false
		m.tailing = false
		return m, nil

	case tea.KeyMsg:
		if m.search.Active {
			switch msg.String() {
			case "esc":
				m.search = m.search.Cancel()
				m = m.applyFilter()
				return m, nil
			case "enter":
				m.search = m.search.Stop()
				return m, nil
			default:
				m.search, cmd = m.search.Update(msg)
				m = m.applyFilter()
				return m, cmd
			}
		}

		if msg.String() == " " || msg.String() == "space" {
			m.toggleSelected()
			m = m.applyFilter()
			return m, nil
		}

		if msg.String() == "esc" && m.search.Query() != "" {
			m.search = m.search.Reset()
			m = m.applyFilter()
			return m, nil
		}

		if common.HandleTableNavKeys(&m.table, msg.String()) {
			return m, nil
		}

		switch msg.String() {
		case "/":
			m.search, cmd = m.search.Start()
			return m, cmd
		case "r":
			if m.state == ViewLogGroups {
				m.loading = true
				return m, m.fetchLogGroups
			}
			start := m.lastEventAt
			if start.IsZero() {
				start = time.Now().Add(-5 * time.Minute)
			} else {
				start = start.Add(-2 * time.Second)
			}
			m.tailFetching = true
			return m, m.fetchEvents(m.currentGroup, start)
		case "enter":
			if m.state == ViewLogGroups {
				selected := m.table.SelectedRow()
				if len(selected) > 0 {
					m.currentGroup = selected[0]
					m.state = ViewTail
					m.loading = true
					m.tailing = true
					m.tailFetching = true
					m.lastEventAt = time.Now().Add(-5 * time.Minute)
					m.events = nil
					m.visibleEventID = nil
					m.selectedEvent = make(map[string]struct{})
					m.seenEvent = make(map[string]struct{})
					m.table.SetRows(nil)
					m = m.SetSize(m.width, m.height)
					return m, tea.Batch(
						m.fetchEvents(m.currentGroup, m.lastEventAt),
						m.tickTail(),
					)
				}
			}
		case "c", "backspace", "delete", "esc":
			if m.state == ViewTail {
				m.state = ViewLogGroups
				m.currentGroup = ""
				m.events = nil
				m.visibleEventID = nil
				m.selectedEvent = make(map[string]struct{})
				m.seenEvent = make(map[string]struct{})
				m.tailing = false
				m.tailFetching = false
				m.lastEventAt = time.Time{}
				m.table.SetRows(nil)
				m = m.SetSize(m.width, m.height)
				m = m.applyFilter()
				return m, nil
			}
		}
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 5)
	m.search = m.search.SetWidth(width)

	if m.state == ViewTail {
		const (
			cellPadding = 6
			minTime     = 19
			minStream   = 18
			minMessage  = 20
		)
		available := width - cellPadding
		if available < minTime+minStream+minMessage {
			return m
		}

		timeWidth := minTime
		streamWidth := minStream
		messageWidth := available - timeWidth - streamWidth

		m.table.SetColumns([]table.Column{
			{Title: "Time", Width: timeWidth},
			{Title: "Stream", Width: streamWidth},
			{Title: "Message", Width: messageWidth},
		})
		return m
	}

	const (
		cellPadding  = 6
		minName      = 30
		minRetention = 10
		minStored    = 10
	)
	available := width - cellPadding
	if available < minName+minRetention+minStored {
		return m
	}

	nameWidth := available - minRetention - minStored
	m.table.SetColumns([]table.Column{
		{Title: "Name", Width: nameWidth},
		{Title: "Retention", Width: minRetention},
		{Title: "Stored", Width: minStored},
	})
	return m
}

func (m Model) View() string {
	if m.err != nil {
		return common.RenderError(m.err)
	}

	header := "CloudWatch Logs"
	if m.state == ViewTail {
		header = fmt.Sprintf("Tail: %s", m.currentGroup)
	}

	if m.loading {
		return common.RenderLoading(header)
	}

	indicator := m.search.Indicator()
	tips := "r: refresh | /: search | space: select | j/k: navigate"
	if m.state == ViewLogGroups {
		tips = "enter: tail | " + tips
	} else {
		tips = "c/backspace: back | " + tips
		if m.tailing {
			tips = "tailing | " + tips
		}
	}
	if indicator != "" {
		tips = indicator + " | " + tips
	}
	if m.search.Active {
		tips = m.search.Input.View()
	}

	title := header
	if m.state == ViewLogGroups {
		if n := len(m.selectedGroup); n > 0 {
			title = fmt.Sprintf("%s (%d selected)", title, n)
		}
	} else {
		if n := len(m.selectedEvent); n > 0 {
			title = fmt.Sprintf("%s (%d selected)", title, n)
		}
	}

	var highlighted string
	if m.state == ViewLogGroups {
		highlighted = common.HighlightSelectedTableRows(m.table.View(), m.table.Cursor(), m.visibleGroupKey, m.selectedGroup)
	} else {
		highlighted = common.HighlightSelectedTableRows(m.table.View(), m.table.Cursor(), m.visibleEventID, m.selectedEvent)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		common.TitleStyle.Render(title),
		highlighted,
		lipgloss.NewStyle().Foreground(common.InfoColor).Render(tips),
	)
}

func (m Model) applyFilter() Model {
	query := m.search.Query()

	if m.state == ViewLogGroups {
		prevCursor := m.table.Cursor()
		valid := make(map[string]struct{}, len(m.logGroups))
		rows := make([]table.Row, 0, len(m.logGroups))
		visible := make([]string, 0, len(m.logGroups))
		for _, g := range m.logGroups {
			valid[g.Name] = struct{}{}
			raw := table.Row{
				g.Name,
				formatRetention(g.RetentionDays),
				formatBytes(g.StoredBytes),
			}
			if common.TableRowMatches(raw, query) {
				rows = append(rows, raw)
				visible = append(visible, g.Name)
			}
		}
		for k := range m.selectedGroup {
			if _, ok := valid[k]; !ok {
				delete(m.selectedGroup, k)
			}
		}
		m.visibleGroupKey = visible
		m.table.SetRows(rows)
		if len(rows) > 0 {
			if prevCursor < 0 {
				prevCursor = 0
			}
			if prevCursor >= len(rows) {
				prevCursor = len(rows) - 1
			}
			m.table.SetCursor(prevCursor)
		}
		return m
	}

	prevCursor := m.table.Cursor()
	rows := make([]table.Row, 0, len(m.events))
	visible := make([]string, 0, len(m.events))
	valid := make(map[string]struct{}, len(m.events))
	for _, e := range m.events {
		id := eventKey(e)
		valid[id] = struct{}{}
		raw := table.Row{
			formatEventTime(e.Timestamp),
			shortStreamName(e.LogStreamName),
			singleLine(e.Message),
		}
		if common.TableRowMatches(raw, query) {
			rows = append(rows, raw)
			visible = append(visible, id)
		}
	}
	for k := range m.selectedEvent {
		if _, ok := valid[k]; !ok {
			delete(m.selectedEvent, k)
		}
	}
	m.visibleEventID = visible
	m.table.SetRows(rows)
	if len(rows) > 0 {
		if prevCursor < 0 {
			prevCursor = 0
		}
		if prevCursor >= len(rows) {
			prevCursor = len(rows) - 1
		}
		m.table.SetCursor(prevCursor)
	}
	return m
}

func (m *Model) toggleSelected() {
	idx := m.table.Cursor()
	if m.state == ViewLogGroups {
		if idx < 0 || idx >= len(m.visibleGroupKey) {
			return
		}
		key := m.visibleGroupKey[idx]
		if _, ok := m.selectedGroup[key]; ok {
			delete(m.selectedGroup, key)
			return
		}
		m.selectedGroup[key] = struct{}{}
		return
	}

	if idx < 0 || idx >= len(m.visibleEventID) {
		return
	}
	key := m.visibleEventID[idx]
	if _, ok := m.selectedEvent[key]; ok {
		delete(m.selectedEvent, key)
		return
	}
	m.selectedEvent[key] = struct{}{}
}

func (m Model) fetchLogGroups() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if m.app.AWSClient == nil {
		return errMsg(fmt.Errorf("AWS client not initialized"))
	}

	groups, err := m.app.AWSClient.CloudWatch().ListLogGroups(ctx)
	if err != nil {
		return errMsg(err)
	}
	return logGroupsLoadedMsg(groups)
}

func (m Model) fetchEvents(group string, start time.Time) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if m.app.AWSClient == nil {
			return errMsg(fmt.Errorf("AWS client not initialized"))
		}

		events, err := m.app.AWSClient.CloudWatch().FilterLogEvents(ctx, group, start)
		if err != nil {
			return errMsg(err)
		}
		return eventsLoadedMsg(events)
	}
}

func (m Model) tickTail() tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(time.Time) tea.Msg {
		return tailTickMsg{}
	})
}

func (m Model) capEvents(max int) Model {
	if max <= 0 || len(m.events) <= max {
		return m
	}

	m.events = m.events[len(m.events)-max:]
	m.seenEvent = make(map[string]struct{}, len(m.events))
	for _, e := range m.events {
		m.seenEvent[eventKey(e)] = struct{}{}
	}
	return m
}

func eventKey(e domain.CloudWatchLogEvent) string {
	if e.EventID != "" {
		return e.EventID
	}
	ts := e.Timestamp.UnixMilli()
	return fmt.Sprintf("%d|%s|%s", ts, e.LogStreamName, e.Message)
}

func singleLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

func shortStreamName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	if len(s) <= 18 {
		return s
	}
	return s[:18]
}

func formatEventTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}

func formatRetention(days int32) string {
	if days <= 0 {
		return "Never"
	}
	return fmt.Sprintf("%dd", days)
}

func formatBytes(b int64) string {
	if b <= 0 {
		return "-"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}

	suffixes := []string{"KB", "MB", "GB", "TB", "PB"}
	f := float64(b)
	i := 0
	for f >= unit && i < len(suffixes) {
		f /= unit
		i++
	}
	return fmt.Sprintf("%.1f %s", f, suffixes[i-1])
}

