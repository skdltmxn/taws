package eks

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
	ViewClusters ViewState = iota
	ViewNodegroups
)

type Model struct {
	app             *app.App
	table           table.Model
	state           ViewState
	clusters        []domain.EKSCluster
	visibleCluster  []string
	selectedCluster map[string]struct{}
	nodegroups      []domain.EKSNodegroup
	visibleNG       []string
	selectedNG      map[string]struct{}
	currentCluster  string
	search          common.TableSearch
	loading         bool
	err             error
	width           int
	height          int
}

type clustersLoadedMsg []domain.EKSCluster
type nodegroupsLoadedMsg []domain.EKSNodegroup
type errMsg error

func NewModel(a *app.App) Model {
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Name", Width: 30},
			{Title: "Version", Width: 10},
			{Title: "Status", Width: 15},
			{Title: "Created At", Width: 25},
		}),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	t.SetStyles(common.DefaultTableStyles())

	return Model{
		app:             a,
		table:           t,
		state:           ViewClusters,
		selectedCluster: make(map[string]struct{}),
		selectedNG:      make(map[string]struct{}),
		search:          common.NewTableSearch(),
	}
}

func (m Model) Init() tea.Cmd {
	return m.fetchClusters
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case clustersLoadedMsg:
		m.clusters = msg
		m.loading = false
		m.err = nil
		m.state = ViewClusters
		m = m.SetSize(m.width, m.height)
		m = m.applyFilter()
		return m, nil

	case nodegroupsLoadedMsg:
		m.nodegroups = msg
		m.loading = false
		m.err = nil
		m.state = ViewNodegroups
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
			if m.state == ViewClusters {
				return m, m.fetchClusters
			}
			return m, m.fetchNodegroups(m.currentCluster)
		case "enter":
			if m.state == ViewClusters {
				selected := m.table.SelectedRow()
				if len(selected) > 0 {
					m.currentCluster = selected[0]
					m.state = ViewNodegroups
					m.loading = true
					m.table.SetRows(nil)
					m = m.SetSize(m.width, m.height)
					return m, m.fetchNodegroups(m.currentCluster)
				}
			}
		case "backspace", "delete", "esc":
			if m.state == ViewNodegroups {
				m.state = ViewClusters
				m.currentCluster = ""
				m.nodegroups = nil
				m.visibleNG = nil
				m.selectedNG = make(map[string]struct{})
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

	if m.state == ViewNodegroups {
		// Nodegroups view: Name, Version, Status, Created At
		const (
			cellPadding = 8
			minName     = 25
			maxName     = 40
			minVersion  = 10
			minStatus   = 12
			minCreated  = 22
		)
		available := width - cellPadding
		if available < minName+minVersion+minStatus+minCreated {
			return m
		}

		nameWidth := maxName
		remaining := available - nameWidth - minVersion - minStatus - minCreated
		if remaining < 0 {
			nameWidth = minName
			remaining = available - nameWidth - minVersion - minStatus - minCreated
		}

		statusWidth := minStatus + remaining/2
		createdWidth := minCreated + remaining - remaining/2
		m.table.SetColumns([]table.Column{
			{Title: "Name", Width: nameWidth},
			{Title: "Version", Width: minVersion},
			{Title: "Status", Width: statusWidth},
			{Title: "Created At", Width: createdWidth},
		})
		return m
	}

	// Dynamically adjust column widths
	// Table cell padding: 4 columns × 2 = 8
	const (
		cellPadding = 8
		minName     = 25
		maxName     = 40
		minVersion  = 10
		minStatus   = 12
		minCreated  = 22
	)
	available := width - cellPadding
	if available < minName+minVersion+minStatus+minCreated {
		return m
	}

	nameWidth := maxName
	remaining := available - nameWidth - minVersion - minStatus - minCreated
	if remaining < 0 {
		nameWidth = minName
		remaining = available - nameWidth - minVersion - minStatus - minCreated
	}

	// Distribute remaining to Status and Created
	statusWidth := minStatus + remaining/2
	createdWidth := minCreated + remaining - remaining/2

	m.table.SetColumns([]table.Column{
		{Title: "Name", Width: nameWidth},
		{Title: "Version", Width: minVersion},
		{Title: "Status", Width: statusWidth},
		{Title: "Created At", Width: createdWidth},
	})
	return m
}

func (m Model) View() string {
	if m.err != nil {
		return common.RenderError(m.err)
	}

	header := "EKS Clusters"
	if m.state == ViewNodegroups {
		header = fmt.Sprintf("Nodegroups in %s", m.currentCluster)
	}

	if m.loading {
		return common.RenderLoading(header)
	}

	indicator := m.search.Indicator()
	tips := "r: refresh | /: search | space: select | j/k: navigate"
	if m.state == ViewClusters {
		tips = "enter: view nodegroups | " + tips
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
	if m.state == ViewClusters {
		if n := len(m.selectedCluster); n > 0 {
			title = fmt.Sprintf("%s (%d selected)", title, n)
		}
	} else {
		if n := len(m.selectedNG); n > 0 {
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
	if m.state == ViewClusters {
		return common.HighlightSelectedTableRows(m.table.View(), m.table.Cursor(), m.visibleCluster, m.selectedCluster)
	}
	return common.HighlightSelectedTableRows(m.table.View(), m.table.Cursor(), m.visibleNG, m.selectedNG)
}

func (m Model) applyFilter() Model {
	prevCursor := m.table.Cursor()
	query := m.search.Query()

	if m.state == ViewNodegroups {
		valid := make(map[string]struct{}, len(m.nodegroups))
		rows := make([]table.Row, 0, len(m.nodegroups))
		visible := make([]string, 0, len(m.nodegroups))
		for _, ng := range m.nodegroups {
			valid[ng.Name] = struct{}{}
			created := "-"
			if !ng.CreatedAt.IsZero() {
				created = ng.CreatedAt.Format("2006-01-02 15:04:05")
			}
			raw := table.Row{
				ng.Name,
				common.DisplayValue(ng.Version),
				common.DisplayValue(ng.Status),
				created,
			}
			if common.TableRowMatches(raw, query) {
				rows = append(rows, raw)
				visible = append(visible, ng.Name)
			}
		}

		for k := range m.selectedNG {
			if _, ok := valid[k]; !ok {
				delete(m.selectedNG, k)
			}
		}

		m.visibleNG = visible
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

	valid := make(map[string]struct{}, len(m.clusters))
	rows := make([]table.Row, 0, len(m.clusters))
	visible := make([]string, 0, len(m.clusters))
	for _, c := range m.clusters {
		valid[c.Name] = struct{}{}
		raw := table.Row{
			c.Name,
			c.Version,
			c.Status,
			c.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		if common.TableRowMatches(raw, query) {
			rows = append(rows, raw)
			visible = append(visible, c.Name)
		}
	}

	for k := range m.selectedCluster {
		if _, ok := valid[k]; !ok {
			delete(m.selectedCluster, k)
		}
	}

	m.visibleCluster = visible
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
	if m.state == ViewClusters {
		if idx < 0 || idx >= len(m.visibleCluster) {
			return
		}
		key := m.visibleCluster[idx]
		if _, ok := m.selectedCluster[key]; ok {
			delete(m.selectedCluster, key)
			return
		}
		m.selectedCluster[key] = struct{}{}
		return
	}

	if idx < 0 || idx >= len(m.visibleNG) {
		return
	}
	key := m.visibleNG[idx]
	if _, ok := m.selectedNG[key]; ok {
		delete(m.selectedNG, key)
		return
	}
	m.selectedNG[key] = struct{}{}
}

func (m Model) fetchClusters() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if m.app.AWSClient == nil {
		return errMsg(fmt.Errorf("AWS client not initialized"))
	}

	clusters, err := m.app.AWSClient.EKS().ListClusters(ctx)
	if err != nil {
		return errMsg(err)
	}
	return clustersLoadedMsg(clusters)
}

func (m Model) fetchNodegroups(clusterName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if m.app.AWSClient == nil {
			return errMsg(fmt.Errorf("AWS client not initialized"))
		}
		nodegroups, err := m.app.AWSClient.EKS().ListNodegroups(ctx, clusterName)
		if err != nil {
			return errMsg(err)
		}
		return nodegroupsLoadedMsg(nodegroups)
	}
}
