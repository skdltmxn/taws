package route53

import (
	"context"
	"fmt"
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
	ViewZones ViewState = iota
	ViewRecords
)

type Model struct {
	app             *app.App
	table           table.Model
	state           ViewState
	zones           []domain.HostedZone
	visibleZone     []string
	selectedZone    map[string]struct{}
	records         []domain.Route53RecordSet
	visibleRecordID []string
	selectedRecord  map[string]struct{}
	currentZoneID   string
	currentZoneName string
	search          common.TableSearch
	loading         bool
	err             error
	width           int
	height          int
}

type zonesLoadedMsg []domain.HostedZone
type recordsLoadedMsg []domain.Route53RecordSet
type errMsg error

func NewModel(a *app.App) Model {
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Domain Name", Width: 40},
			{Title: "ID", Width: 20},
			{Title: "Record Count", Width: 15},
		}),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	t.SetStyles(common.DefaultTableStyles())

	return Model{
		app:            a,
		table:          t,
		state:          ViewZones,
		selectedZone:   make(map[string]struct{}),
		selectedRecord: make(map[string]struct{}),
		search:         common.NewTableSearch(),
	}
}

func (m Model) Init() tea.Cmd {
	return m.fetchZones
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case zonesLoadedMsg:
		m.zones = msg
		m.loading = false
		m.err = nil
		m.state = ViewZones
		m = m.SetSize(m.width, m.height)
		m = m.applyFilter()
		return m, nil

	case recordsLoadedMsg:
		m.records = msg
		m.loading = false
		m.err = nil
		m.state = ViewRecords
		m = m.SetSize(m.width, m.height)
		m = m.applyFilter()
		return m, nil

	case errMsg:
		m.err = msg
		m.loading = false
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
			m.loading = true
			if m.state == ViewZones {
				return m, m.fetchZones
			}
			return m, m.fetchRecords(m.currentZoneID)
		case "enter":
			if m.state == ViewZones {
				selected := m.table.SelectedRow()
				if len(selected) > 0 {
					m.currentZoneName = selected[0]
					m.currentZoneID = selected[1]
					m.state = ViewRecords
					m.loading = true
					m.table.SetRows(nil)
					m = m.SetSize(m.width, m.height)
					return m, m.fetchRecords(m.currentZoneID)
				}
			}
		case "backspace", "delete", "esc":
			if m.state == ViewRecords {
				m.state = ViewZones
				m.currentZoneID = ""
				m.currentZoneName = ""
				m.records = nil
				m.visibleRecordID = nil
				m.selectedRecord = make(map[string]struct{})
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

	if m.state == ViewRecords {
		// Records view: Name, Type, TTL, Value
		const (
			cellPadding = 8
			minName     = 25
			maxName     = 45
			minType     = 8
			minTTL      = 6
			minValue    = 20
		)
		available := width - cellPadding
		if available < minName+minType+minTTL+minValue {
			return m
		}

		nameWidth := maxName
		remaining := available - nameWidth - minType - minTTL - minValue
		if remaining < 0 {
			nameWidth = minName
			remaining = available - nameWidth - minType - minTTL - minValue
		}

		valueWidth := minValue + remaining
		m.table.SetColumns([]table.Column{
			{Title: "Name", Width: nameWidth},
			{Title: "Type", Width: minType},
			{Title: "TTL", Width: minTTL},
			{Title: "Value", Width: valueWidth},
		})
		return m
	}

	// Zones view: Domain Name, ID, Record Count
	const (
		cellPadding = 6
		minDomain   = 30
		maxDomain   = 45
		minID       = 18
		minCount    = 14
	)
	available := width - cellPadding
	if available < minDomain+minID+minCount {
		return m
	}

	domainWidth := maxDomain
	remaining := available - domainWidth - minID - minCount
	if remaining < 0 {
		domainWidth = minDomain
		remaining = available - domainWidth - minID - minCount
	}

	idWidth := minID + remaining/2
	countWidth := minCount + remaining - remaining/2

	m.table.SetColumns([]table.Column{
		{Title: "Domain Name", Width: domainWidth},
		{Title: "ID", Width: idWidth},
		{Title: "Record Count", Width: countWidth},
	})
	return m
}

func (m Model) View() string {
	if m.err != nil {
		return common.RenderError(m.err)
	}

	header := "Route53 Hosted Zones"
	if m.state == ViewRecords {
		header = fmt.Sprintf("Records in %s", m.currentZoneName)
	}

	if m.loading {
		return common.RenderLoading(header)
	}

	indicator := m.search.Indicator()
	tips := "r: refresh | /: search | space: select | j/k: navigate"
	if m.state == ViewZones {
		tips = "enter: view records | " + tips
	} else {
		tips = "backspace: back | " + tips
	}
	if indicator != "" {
		tips = indicator + " | " + tips
	}
	if m.search.Active {
		tips = m.search.Input.View()
	}

	title := header
	if m.state == ViewZones {
		if n := len(m.selectedZone); n > 0 {
			title = fmt.Sprintf("%s (%d selected)", title, n)
		}
	} else {
		if n := len(m.selectedRecord); n > 0 {
			title = fmt.Sprintf("%s (%d selected)", title, n)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		common.TitleStyle.Render(title),
		m.renderTable(),
		lipgloss.NewStyle().Foreground(common.InfoColor).Render(tips),
	)
}

func (m Model) renderTable() string {
	if m.state == ViewZones {
		return common.HighlightSelectedTableRows(m.table.View(), m.table.Cursor(), m.visibleZone, m.selectedZone)
	}
	return common.HighlightSelectedTableRows(m.table.View(), m.table.Cursor(), m.visibleRecordID, m.selectedRecord)
}

func (m Model) applyFilter() Model {
	prevCursor := m.table.Cursor()
	query := m.search.Query()

	if m.state == ViewRecords {
		valid := make(map[string]struct{}, len(m.records))
		rows := make([]table.Row, 0, len(m.records))
		visible := make([]string, 0, len(m.records))
		for _, r := range m.records {
			valid[r.ID] = struct{}{}
			ttl := "-"
			if r.TTL > 0 {
				ttl = fmt.Sprintf("%d", r.TTL)
			}
			raw := table.Row{
				r.Name,
				r.Type,
				ttl,
				r.Value,
			}
			if common.TableRowMatches(raw, query) {
				rows = append(rows, raw)
				visible = append(visible, r.ID)
			}
		}

		for k := range m.selectedRecord {
			if _, ok := valid[k]; !ok {
				delete(m.selectedRecord, k)
			}
		}

		m.visibleRecordID = visible
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

	valid := make(map[string]struct{}, len(m.zones))
	rows := make([]table.Row, 0, len(m.zones))
	visible := make([]string, 0, len(m.zones))
	for _, z := range m.zones {
		valid[z.ID] = struct{}{}
		raw := table.Row{
			z.Name,
			z.ID,
			fmt.Sprintf("%d", z.Count),
		}
		if common.TableRowMatches(raw, query) {
			rows = append(rows, raw)
			visible = append(visible, z.ID)
		}
	}

	for k := range m.selectedZone {
		if _, ok := valid[k]; !ok {
			delete(m.selectedZone, k)
		}
	}

	m.visibleZone = visible
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
	if m.state == ViewZones {
		if idx < 0 || idx >= len(m.visibleZone) {
			return
		}
		key := m.visibleZone[idx]
		if _, ok := m.selectedZone[key]; ok {
			delete(m.selectedZone, key)
			return
		}
		m.selectedZone[key] = struct{}{}
		return
	}

	if idx < 0 || idx >= len(m.visibleRecordID) {
		return
	}
	key := m.visibleRecordID[idx]
	if _, ok := m.selectedRecord[key]; ok {
		delete(m.selectedRecord, key)
		return
	}
	m.selectedRecord[key] = struct{}{}
}

func (m Model) fetchZones() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if m.app.AWSClient == nil {
		return errMsg(fmt.Errorf("AWS client not initialized"))
	}

	zones, err := m.app.AWSClient.Route53().ListHostedZones(ctx)
	if err != nil {
		return errMsg(err)
	}
	return zonesLoadedMsg(zones)
}

func (m Model) fetchRecords(hostedZoneID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if m.app.AWSClient == nil {
			return errMsg(fmt.Errorf("AWS client not initialized"))
		}
		records, err := m.app.AWSClient.Route53().ListRecordSets(ctx, hostedZoneID)
		if err != nil {
			return errMsg(err)
		}
		return recordsLoadedMsg(records)
	}
}
