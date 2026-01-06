package vpc

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
	ViewVPCs ViewState = iota
	ViewSubnets
)

type Model struct {
	app        *app.App
	table      table.Model
	state      ViewState
	vpcs       []domain.VPC
	visibleVPC []string
	selectedV  map[string]struct{}
	subnets    []domain.Subnet
	visibleSub []string
	selectedS  map[string]struct{}
	currentVPC string
	search     common.TableSearch
	loading    bool
	err        error
	width      int
	height     int
}

type vpcsLoadedMsg []domain.VPC
type subnetsLoadedMsg []domain.Subnet
type errMsg error

func NewModel(a *app.App) Model {
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Name", Width: 30},
			{Title: "ID", Width: 20},
			{Title: "CIDR", Width: 15},
			{Title: "State", Width: 10},
		}),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	t.SetStyles(common.DefaultTableStyles())

	return Model{
		app:       a,
		table:     t,
		state:     ViewVPCs,
		selectedV: make(map[string]struct{}),
		selectedS: make(map[string]struct{}),
		search:    common.NewTableSearch(),
	}
}

func (m Model) Init() tea.Cmd {
	return m.fetchVPCs
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case vpcsLoadedMsg:
		m.vpcs = msg
		m.loading = false
		m.err = nil
		m.state = ViewVPCs
		m = m.applyFilter()
		return m, nil

	case subnetsLoadedMsg:
		m.subnets = msg
		m.loading = false
		m.err = nil
		m.state = ViewSubnets
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
			if m.state == ViewVPCs {
				return m, m.fetchVPCs
			} else {
				return m, m.fetchSubnets(m.currentVPC)
			}
		case "enter":
			if m.state == ViewVPCs {
				selected := m.table.SelectedRow()
				if len(selected) > 0 {
					m.currentVPC = selected[1]
					m.loading = true
					m.table.SetRows(nil)
					return m, m.fetchSubnets(m.currentVPC)
				}
			}
		case "backspace", "delete", "esc":
			if m.state == ViewSubnets {
				m.state = ViewVPCs
				m.loading = true
				m.table.SetRows(nil)
				return m, m.fetchVPCs
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

	// Dynamically adjust column widths based on current view
	// Table cell padding: columns × 2 (left+right padding per cell)
	if m.state == ViewSubnets {
		// Subnets view: Name, ID, CIDR, AZ, IPs
		const (
			cellPadding = 10
			minName     = 20
			maxName     = 35
			minID       = 26
			minCIDR     = 18
			minAZ       = 15
			minIPs      = 8
		)
		available := width - cellPadding
		if available < minName+minID+minCIDR+minAZ+minIPs {
			return m
		}

		nameWidth := maxName
		remaining := available - nameWidth - minID - minCIDR - minAZ - minIPs
		if remaining < 0 {
			nameWidth = minName
			remaining = available - nameWidth - minID - minCIDR - minAZ - minIPs
		}

		// Distribute remaining to ID and AZ
		idWidth := minID + remaining/2
		azWidth := minAZ + remaining - remaining/2

		m.table.SetColumns([]table.Column{
			{Title: "Name", Width: nameWidth},
			{Title: "ID", Width: idWidth},
			{Title: "CIDR", Width: minCIDR},
			{Title: "AZ", Width: azWidth},
			{Title: "IPs", Width: minIPs},
		})
	} else {
		// VPCs view: Name, ID, CIDR, State
		const (
			cellPadding = 8
			minName     = 20
			maxName     = 35
			minID       = 24
			minCIDR     = 18
			minState    = 12
		)
		available := width - cellPadding
		if available < minName+minID+minCIDR+minState {
			return m
		}

		nameWidth := maxName
		remaining := available - nameWidth - minID - minCIDR - minState
		if remaining < 0 {
			nameWidth = minName
			remaining = available - nameWidth - minID - minCIDR - minState
		}

		// Distribute remaining to ID and CIDR
		idWidth := minID + remaining/2
		cidrWidth := minCIDR + remaining - remaining/2

		m.table.SetColumns([]table.Column{
			{Title: "Name", Width: nameWidth},
			{Title: "ID", Width: idWidth},
			{Title: "CIDR", Width: cidrWidth},
			{Title: "State", Width: minState},
		})
	}
	return m
}

func (m Model) View() string {
	if m.err != nil {
		return common.RenderError(m.err)
	}

	header := "VPCs"
	if m.state == ViewSubnets {
		header = fmt.Sprintf("Subnets in VPC %s", m.currentVPC)
	}

	if m.loading {
		return common.RenderLoading(header)
	}

	tips := "enter: view subnets | r: refresh | j/k: navigate"
	if m.state == ViewSubnets {
		tips = "backspace: back to VPCs | r: refresh | j/k: navigate"
	}
	tips = tips + " | space: select"
	indicator := m.search.Indicator()
	if indicator != "" {
		tips = indicator + " | " + tips
	}
	tips = tips + " | /: search"
	if m.search.Active {
		tips = m.search.Input.View()
	}

	title := header
	if n := m.selectedCount(); n > 0 {
		title = fmt.Sprintf("%s (%d selected)", title, n)
	}

	tableView := common.HighlightSelectedTableRows(m.table.View(), m.table.Cursor(), m.visibleVPC, m.selectedV)
	if m.state == ViewSubnets {
		tableView = common.HighlightSelectedTableRows(m.table.View(), m.table.Cursor(), m.visibleSub, m.selectedS)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		common.TitleStyle.Render(title),
		tableView,
		lipgloss.NewStyle().Foreground(common.InfoColor).Render(tips),
	)
}

func (m Model) applyFilter() Model {
	prevCursor := m.table.Cursor()
	query := m.search.Query()

	var rows []table.Row
	switch m.state {
	case ViewSubnets:
		valid := make(map[string]struct{}, len(m.subnets))
		rows = make([]table.Row, 0, len(m.subnets))
		visible := make([]string, 0, len(m.subnets))
		for _, s := range m.subnets {
			valid[s.ID] = struct{}{}
			raw := table.Row{
				common.DisplayValue(s.Name),
				s.ID,
				s.CidrBlock,
				s.AvailabilityZone,
				fmt.Sprintf("%d", s.AvailableIPs),
			}
			if common.TableRowMatches(raw, query) {
				rows = append(rows, raw)
				visible = append(visible, s.ID)
			}
		}
		for k := range m.selectedS {
			if _, ok := valid[k]; !ok {
				delete(m.selectedS, k)
			}
		}
		m.visibleSub = visible
	default:
		valid := make(map[string]struct{}, len(m.vpcs))
		rows = make([]table.Row, 0, len(m.vpcs))
		visible := make([]string, 0, len(m.vpcs))
		for _, v := range m.vpcs {
			valid[v.ID] = struct{}{}
			raw := table.Row{
				common.DisplayValue(v.Name),
				v.ID,
				v.CidrBlock,
				v.State,
			}
			if common.TableRowMatches(raw, query) {
				rows = append(rows, raw)
				visible = append(visible, v.ID)
			}
		}
		for k := range m.selectedV {
			if _, ok := valid[k]; !ok {
				delete(m.selectedV, k)
			}
		}
		m.visibleVPC = visible
	}

	m.table.SetRows(nil)
	m = m.SetSize(m.width, m.height)
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
	if idx < 0 {
		return
	}
	if m.state == ViewSubnets {
		if idx >= len(m.visibleSub) {
			return
		}
		key := m.visibleSub[idx]
		if _, ok := m.selectedS[key]; ok {
			delete(m.selectedS, key)
			return
		}
		m.selectedS[key] = struct{}{}
		return
	}

	if idx >= len(m.visibleVPC) {
		return
	}
	key := m.visibleVPC[idx]
	if _, ok := m.selectedV[key]; ok {
		delete(m.selectedV, key)
		return
	}
	m.selectedV[key] = struct{}{}
}

func (m Model) selectedCount() int {
	if m.state == ViewSubnets {
		return len(m.selectedS)
	}
	return len(m.selectedV)
}

func (m Model) fetchVPCs() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if m.app.AWSClient == nil {
		return errMsg(fmt.Errorf("AWS client not initialized"))
	}

	vpcs, err := m.app.AWSClient.VPC().ListVPCs(ctx)
	if err != nil {
		return errMsg(err)
	}
	return vpcsLoadedMsg(vpcs)
}

func (m Model) fetchSubnets(vpcID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if m.app.AWSClient == nil {
			return errMsg(fmt.Errorf("AWS client not initialized"))
		}

		subnets, err := m.app.AWSClient.VPC().ListSubnets(ctx, vpcID)
		if err != nil {
			return errMsg(err)
		}
		return subnetsLoadedMsg(subnets)
	}
}
