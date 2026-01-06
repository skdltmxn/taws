package ec2

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
	ViewList ViewState = iota
	ViewDetail
)

type Model struct {
	app          *app.App
	table        table.Model
	allInstances []domain.EC2Instance
	instances    []domain.EC2Instance
	rowInstance  []string
	selectedIDs  map[string]struct{}
	search       common.TableSearch
	loading      bool
	err          error
	loaded       bool
	state        ViewState
	selected     *domain.EC2Instance
	actionMsg    string
	width        int
	height       int
}

type instancesLoadedMsg []domain.EC2Instance
type errMsg struct{ error }
type actionSuccessMsg string
type actionErrorMsg struct{ error }

func NewModel(a *app.App) Model {
	columns := []table.Column{
		{Title: "Name", Width: 20},
		{Title: "ID", Width: 15},
		{Title: "State", Width: 10},
		{Title: "Type", Width: 20},
		{Title: "AZ", Width: 15},
		{Title: "Platform", Width: 8},
		{Title: "Arch", Width: 7},
		{Title: "Public IP", Width: 15},
		{Title: "Private IP", Width: 15},
		{Title: "Launch Time", Width: 16},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	t.SetStyles(common.DefaultTableStyles())

	return Model{
		app:         a,
		table:       t,
		state:       ViewList,
		selectedIDs: make(map[string]struct{}),
		search:      common.NewTableSearch(),
	}
}

func (m Model) Init() tea.Cmd {
	return m.fetchInstances
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case instancesLoadedMsg:
		m.allInstances = msg
		m.loading = false
		m.loaded = true
		m.err = nil
		m = m.applyFilter()
		return m, nil

	case actionSuccessMsg:
		m.actionMsg = string(msg)
		m.loading = true
		return m, m.fetchInstances

	case actionErrorMsg:
		m.err = msg
		m.loading = false
		return m, nil

	case errMsg:
		m.err = msg
		m.loading = false
		return m, nil

	case tea.KeyMsg:
		m.actionMsg = ""

		if m.state == ViewDetail {
			switch msg.String() {
			case "backspace", "esc", "q":
				m.state = ViewList
				m.selected = nil
				return m, nil
			case "s":
				if m.selected != nil && m.selected.State == domain.InstanceStatusRunning {
					m.loading = true
					return m, m.stopInstance(m.selected.ID)
				}
			case "S":
				if m.selected != nil && m.selected.State == domain.InstanceStatusStopped {
					m.loading = true
					return m, m.startInstance(m.selected.ID)
				}
			case "R":
				if m.selected != nil && m.selected.State == domain.InstanceStatusRunning {
					m.loading = true
					return m, m.rebootInstance(m.selected.ID)
				}
			}
			return m, nil
		}

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
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.instances) {
				key := m.instances[idx].ID
				if _, ok := m.selectedIDs[key]; ok {
					delete(m.selectedIDs, key)
				} else {
					m.selectedIDs[key] = struct{}{}
				}
				m = m.applyFilter()
			}
			return m, nil
		}

		if msg.String() == "esc" && m.search.Query() != "" {
			m.search = m.search.Reset()
			m = m.applyFilter()
			return m, nil
		}

		switch msg.String() {
		case "/":
			m.search, cmd = m.search.Start()
			return m, cmd
		case "r":
			m.loading = true
			return m, m.fetchInstances
		case "enter":
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.instances) {
				m.selected = &m.instances[idx]
				m.state = ViewDetail
			}
			return m, nil
		default:
			if common.HandleTableNavKeys(&m.table, msg.String()) {
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

	// Dynamically adjust column widths to fill available space
	// Table cell padding: 10 columns × 2 (left+right) = 20
	const (
		cellPadding  = 20
		minName      = 18
		maxName      = 28
		minID        = 19
		minState     = 9
		minType      = 13
		maxType      = 18
		minAZ        = 14
		minPlatform  = 8
		minArch      = 6
		minPublicIP  = 15
		minPrivateIP = 15
		minLaunch    = 16
	)
	minTotal := minName + minID + minState + minType + minAZ + minPlatform + minArch + minPublicIP + minPrivateIP + minLaunch
	available := width - cellPadding
	if available < minTotal {
		return m
	}

	// Calculate balanced widths
	nameWidth := maxName
	typeWidth := maxType
	remaining := available - nameWidth - minID - minState - typeWidth - minAZ - minPlatform - minArch - minPublicIP - minPrivateIP - minLaunch
	if remaining < 0 {
		nameWidth = minName
		typeWidth = minType
		remaining = available - nameWidth - minID - minState - typeWidth - minAZ - minPlatform - minArch - minPublicIP - minPrivateIP - minLaunch
	}

	// Distribute remaining space to AZ and Launch Time
	azWidth := minAZ + remaining/2
	launchWidth := minLaunch + remaining - remaining/2

	m.table.SetColumns([]table.Column{
		{Title: "Name", Width: nameWidth},
		{Title: "ID", Width: minID},
		{Title: "State", Width: minState},
		{Title: "Type", Width: typeWidth},
		{Title: "AZ", Width: azWidth},
		{Title: "Platform", Width: minPlatform},
		{Title: "Arch", Width: minArch},
		{Title: "Public IP", Width: minPublicIP},
		{Title: "Private IP", Width: minPrivateIP},
		{Title: "Launch Time", Width: launchWidth},
	})
	return m
}

func (m Model) View() string {
	if m.err != nil {
		return common.RenderError(m.err)
	}
	if m.loading && !m.loaded {
		return "Loading EC2 instances..."
	}

	if m.state == ViewDetail && m.selected != nil {
		return m.renderDetailView()
	}

	var statusLine string
	if m.actionMsg != "" {
		statusLine = lipgloss.NewStyle().Foreground(common.SuccessColor).Render(m.actionMsg)
	}

	indicator := m.search.Indicator()
	tips := "enter: detail | r: refresh | /: search | space: select | j/k: navigate | g/G: top/bottom"
	if indicator != "" {
		tips = indicator + " | " + tips
	}
	if m.search.Active {
		tips = m.search.Input.View()
	}

	title := "EC2 Instances"
	if n := len(m.selectedIDs); n > 0 {
		title = fmt.Sprintf("%s (%d selected)", title, n)
	}

	elements := []string{
		common.TitleStyle.Render(title),
	}
	if statusLine != "" {
		elements = append(elements, statusLine)
	}
	elements = append(elements,
		common.HighlightSelectedTableRows(m.table.View(), m.table.Cursor(), m.rowInstance, m.selectedIDs),
		lipgloss.NewStyle().Foreground(common.InfoColor).Render(tips),
	)

	return lipgloss.JoinVertical(lipgloss.Left, elements...)
}

func (m Model) applyFilter() Model {
	prevCursor := m.table.Cursor()
	query := m.search.Query()

	valid := make(map[string]struct{}, len(m.allInstances))
	m.instances = nil
	m.rowInstance = nil
	rows := make([]table.Row, 0, len(m.allInstances))
	for _, instance := range m.allInstances {
		valid[instance.ID] = struct{}{}
		raw := table.Row{
			common.DisplayValue(instance.Name),
			instance.ID,
			string(instance.State),
			instance.Type,
			instance.AvailabilityZone,
			instance.Platform,
			instance.Architecture,
			common.DisplayValue(instance.PublicIP),
			instance.PrivateIP,
			instance.LaunchTime.Format("2006-01-02 15:04"),
		}
		if common.TableRowMatches(raw, query) {
			m.instances = append(m.instances, instance)
			m.rowInstance = append(m.rowInstance, instance.ID)
			rows = append(rows, raw)
		}
	}

	for k := range m.selectedIDs {
		if _, ok := valid[k]; !ok {
			delete(m.selectedIDs, k)
		}
	}

	if m.state == ViewDetail && m.selected != nil {
		selectedID := m.selected.ID
		for _, inst := range m.allInstances {
			if inst.ID == selectedID {
				instCopy := inst
				m.selected = &instCopy
				break
			}
		}
	}

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

func (m Model) renderDetailView() string {
	inst := m.selected

	detailStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(common.PrimaryColor).
		Padding(1, 2)

	labelStyle := lipgloss.NewStyle().
		Foreground(common.SubTextColor).
		Width(15)

	valueStyle := lipgloss.NewStyle().
		Foreground(common.TextColor)

	row := func(label, value string) string {
		return lipgloss.JoinHorizontal(lipgloss.Left,
			labelStyle.Render(label+":"),
			valueStyle.Render(value),
		)
	}

	stateColor := common.SubTextColor
	switch inst.State {
	case domain.InstanceStatusRunning:
		stateColor = common.SuccessColor
	case domain.InstanceStatusStopped:
		stateColor = common.ErrorColor
	case domain.InstanceStatusPending, domain.InstanceStatusStopping:
		stateColor = common.WarningColor
	}

	stateValue := lipgloss.NewStyle().Foreground(stateColor).Render(string(inst.State))

	details := lipgloss.JoinVertical(lipgloss.Left,
		row("Name", inst.Name),
		row("Instance ID", inst.ID),
		lipgloss.JoinHorizontal(lipgloss.Left,
			labelStyle.Render("State:"),
			stateValue,
		),
		row("Type", inst.Type),
		row("Public IP", inst.PublicIP),
		row("Private IP", inst.PrivateIP),
		row("VPC ID", inst.VpcID),
		row("Subnet ID", inst.SubnetID),
		row("Key Name", inst.KeyName),
		row("Architecture", inst.Architecture),
		row("Platform", inst.Platform),
		row("Launch Time", inst.LaunchTime.Format("2006-01-02 15:04:05")),
	)

	var actions string
	switch inst.State {
	case domain.InstanceStatusRunning:
		actions = "s: stop | R: reboot"
	case domain.InstanceStatusStopped:
		actions = "S: start"
	default:
		actions = "(no actions available)"
	}

	var statusLine string
	if m.actionMsg != "" {
		statusLine = lipgloss.NewStyle().Foreground(common.SuccessColor).Render(m.actionMsg) + "\n"
	}

	tips := fmt.Sprintf("backspace/esc: back | %s", actions)

	return lipgloss.JoinVertical(lipgloss.Left,
		common.TitleStyle.Render("EC2 Instance Detail"),
		statusLine,
		detailStyle.Render(details),
		lipgloss.NewStyle().Foreground(common.InfoColor).Render(tips),
	)
}

func (m Model) fetchInstances() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if m.app.AWSClient == nil {
		return errMsg{fmt.Errorf("AWS client not initialized")}
	}

	instances, err := m.app.AWSClient.EC2().ListInstances(ctx)
	if err != nil {
		return errMsg{err}
	}
	return instancesLoadedMsg(instances)
}

func (m Model) startInstance(instanceID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if m.app.AWSClient == nil {
			return actionErrorMsg{fmt.Errorf("AWS client not initialized")}
		}

		err := m.app.AWSClient.EC2().StartInstance(ctx, instanceID)
		if err != nil {
			return actionErrorMsg{err}
		}
		return actionSuccessMsg(fmt.Sprintf("Instance %s is starting...", instanceID))
	}
}

func (m Model) stopInstance(instanceID string) tea.Cmd {
	return func() tea.Msg {
		if m.app.AWSClient == nil {
			return actionErrorMsg{fmt.Errorf("AWS client not initialized")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := m.app.AWSClient.EC2().StopInstance(ctx, instanceID)
		if err != nil {
			return actionErrorMsg{err}
		}
		return actionSuccessMsg(fmt.Sprintf("Instance %s is stopping...", instanceID))
	}
}

func (m Model) rebootInstance(instanceID string) tea.Cmd {
	return func() tea.Msg {
		if m.app.AWSClient == nil {
			return actionErrorMsg{fmt.Errorf("AWS client not initialized")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := m.app.AWSClient.EC2().RebootInstance(ctx, instanceID)
		if err != nil {
			return actionErrorMsg{err}
		}
		return actionSuccessMsg(fmt.Sprintf("Instance %s is rebooting...", instanceID))
	}
}
