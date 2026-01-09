package ec2

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
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
	ViewConfirm
	ViewYAML
)

type ConfirmAction int

const (
	ActionNone ConfirmAction = iota
	ActionStart
	ActionStop
	ActionReboot
	ActionTerminate
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
	fullWidth    int
	fullHeight   int

	confirmAction    ConfirmAction
	confirmInput     textinput.Model
	confirmTargetIDs []string
	prevState        ViewState

	yamlView common.YAMLView
}

type instancesLoadedMsg []domain.EC2Instance
type errMsg struct{ error }
type actionSuccessMsg string
type actionErrorMsg struct{ error }
type autoRefreshTickMsg time.Time

const (
	autoRefreshFast = 3 * time.Second
	autoRefreshSlow = 10 * time.Second
)

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

	ci := textinput.New()
	ci.Placeholder = ""
	ci.CharLimit = 50
	ci.Width = 30

	return Model{
		app:          a,
		table:        t,
		state:        ViewList,
		selectedIDs:  make(map[string]struct{}),
		search:       common.NewTableSearch(),
		confirmInput: ci,
		yamlView:     common.NewYAMLView(),
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
		return m, m.scheduleAutoRefresh()

	case autoRefreshTickMsg:
		if m.loading || m.state == ViewConfirm {
			return m, m.scheduleAutoRefresh()
		}
		return m, m.fetchInstances

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

		if m.state == ViewYAML {
			var exit bool
			m.yamlView, cmd, exit = m.yamlView.Update(msg)
			if exit {
				m.state = ViewList
				return m, nil
			}
			return m, cmd
		}

		if m.state == ViewConfirm {
			return m.handleConfirmInput(msg)
		}

		if m.state == ViewDetail {
			switch msg.String() {
			case "backspace", "esc", "q":
				m.state = ViewList
				m.selected = nil
				return m, nil
			case "s":
				if m.selected != nil && m.selected.State == domain.InstanceStatusRunning {
					return m.enterConfirmMode(ActionStop, []string{m.selected.ID})
				}
			case "S":
				if m.selected != nil && m.selected.State == domain.InstanceStatusStopped {
					m.loading = true
					return m, m.executeInstanceAction(ActionStart, []string{m.selected.ID})
				}
			case "R":
				if m.selected != nil && m.selected.State == domain.InstanceStatusRunning {
					m.loading = true
					return m, m.executeInstanceAction(ActionReboot, []string{m.selected.ID})
				}
			case "T":
				if m.selected != nil && m.selected.State != domain.InstanceStatusTerminated && m.selected.State != domain.InstanceStatusShuttingDown {
					return m.enterConfirmMode(ActionTerminate, []string{m.selected.ID})
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
		case "s":
			return m.handleListAction(ActionStop, domain.InstanceStatusRunning)
		case "S":
			return m.handleListStartAction()
		case "R":
			return m.handleListRebootAction()
		case "T":
			return m.handleListAction(ActionTerminate, "")
		case "y":
			return m.showYAMLView()
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
	m.yamlView = m.yamlView.SetSize(width, height)

	// Dynamically adjust column widths to fill available space
	// Table cell padding: 10 columns × 2 (left+right) = 20
	const (
		cellPadding  = 20
		minName      = 18
		maxName      = 32
		minID        = 19
		minState     = 13
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

func (m Model) SetFullscreenSize(width, height int) Model {
	m.fullWidth = width
	m.fullHeight = height
	m.yamlView = m.yamlView.SetFullscreenSize(width, height)
	return m
}

func (m Model) IsFullscreen() bool {
	return m.state == ViewYAML && m.yamlView.IsFullscreen()
}

func (m Model) FullscreenView() string {
	return m.yamlView.View()
}

func (m Model) showYAMLView() (Model, tea.Cmd) {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.instances) {
		return m, nil
	}

	inst := &m.instances[idx]
	title := fmt.Sprintf("EC2 Instance: %s", common.DisplayValue(inst.Name))
	m.yamlView = m.yamlView.SetContent(title, inst)
	m.state = ViewYAML
	return m, nil
}

func (m Model) View() string {
	if m.err != nil {
		return common.RenderError(m.err)
	}
	if m.loading && !m.loaded {
		return "Loading EC2 instances..."
	}

	if m.state == ViewYAML {
		return m.yamlView.View()
	}

	if m.state == ViewConfirm {
		var base string
		if m.prevState == ViewDetail && m.selected != nil {
			base = m.renderDetailView()
		} else {
			base = m.renderListView()
		}
		return m.overlayConfirmModal(base)
	}

	if m.state == ViewDetail && m.selected != nil {
		return m.renderDetailView()
	}

	return m.renderListView()
}

func (m Model) renderListView() string {
	var statusLine string
	if m.actionMsg != "" {
		statusLine = lipgloss.NewStyle().Foreground(common.SuccessColor).Render(m.actionMsg)
	}

	indicator := m.search.Indicator()
	tips := "enter: detail | s: stop | S: start | R: reboot | T: terminate | y: yaml | r: refresh | /: search | space: select"
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
		actions = "s: stop | R: reboot | T: terminate"
	case domain.InstanceStatusStopped:
		actions = "S: start | T: terminate"
	case domain.InstanceStatusTerminated, domain.InstanceStatusShuttingDown:
		actions = "(no actions available)"
	default:
		actions = "T: terminate"
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

func (m Model) hasTransitionalState() bool {
	for _, inst := range m.allInstances {
		switch inst.State {
		case domain.InstanceStatusPending,
			domain.InstanceStatusStopping,
			domain.InstanceStatusShuttingDown:
			return true
		}
	}
	return false
}

func (m Model) scheduleAutoRefresh() tea.Cmd {
	interval := autoRefreshSlow
	if m.hasTransitionalState() {
		interval = autoRefreshFast
	}
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return autoRefreshTickMsg(t)
	})
}

func (m Model) executeInstanceAction(action ConfirmAction, instanceIDs []string) tea.Cmd {
	return func() tea.Msg {
		if m.app.AWSClient == nil {
			return actionErrorMsg{fmt.Errorf("AWS client not initialized")}
		}

		ec2Client := m.app.AWSClient.EC2()
		var actionFn func(context.Context, string) error
		var actionVerb string

		switch action {
		case ActionStart:
			actionFn = ec2Client.StartInstance
			actionVerb = "starting"
		case ActionStop:
			actionFn = ec2Client.StopInstance
			actionVerb = "stopping"
		case ActionReboot:
			actionFn = ec2Client.RebootInstance
			actionVerb = "rebooting"
		case ActionTerminate:
			actionFn = ec2Client.TerminateInstance
			actionVerb = "terminating"
		default:
			return actionErrorMsg{fmt.Errorf("unknown action")}
		}

		var succeeded []string
		for _, id := range instanceIDs {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			err := actionFn(ctx, id)
			cancel()

			if err != nil {
				if len(succeeded) > 0 {
					return actionErrorMsg{fmt.Errorf("%d instance(s) succeeded, but failed on %s: %w", len(succeeded), id, err)}
				}
				return actionErrorMsg{err}
			}
			succeeded = append(succeeded, id)
		}

		if len(instanceIDs) == 1 {
			return actionSuccessMsg(fmt.Sprintf("Instance %s is %s...", instanceIDs[0], actionVerb))
		}
		return actionSuccessMsg(fmt.Sprintf("%d instance(s) %s...", len(instanceIDs), actionVerb))
	}
}

func (m Model) getTargetInstanceIDs(requiredState domain.InstanceStatus, excludeStates ...domain.InstanceStatus) []string {
	excludeMap := make(map[domain.InstanceStatus]struct{})
	for _, s := range excludeStates {
		excludeMap[s] = struct{}{}
	}

	var ids []string
	if len(m.selectedIDs) > 0 {
		for _, inst := range m.instances {
			if _, ok := m.selectedIDs[inst.ID]; !ok {
				continue
			}
			if _, excluded := excludeMap[inst.State]; excluded {
				continue
			}
			if requiredState != "" && inst.State != requiredState {
				continue
			}
			ids = append(ids, inst.ID)
		}
	} else {
		idx := m.table.Cursor()
		if idx >= 0 && idx < len(m.instances) {
			inst := m.instances[idx]
			if _, excluded := excludeMap[inst.State]; !excluded {
				if requiredState == "" || inst.State == requiredState {
					ids = append(ids, inst.ID)
				}
			}
		}
	}
	return ids
}

func (m Model) handleListAction(action ConfirmAction, requiredState domain.InstanceStatus) (Model, tea.Cmd) {
	var ids []string
	if action == ActionTerminate {
		ids = m.getTargetInstanceIDs("", domain.InstanceStatusTerminated, domain.InstanceStatusShuttingDown)
	} else {
		ids = m.getTargetInstanceIDs(requiredState)
	}
	if len(ids) == 0 {
		return m, nil
	}
	return m.enterConfirmMode(action, ids)
}

func (m Model) handleListStartAction() (Model, tea.Cmd) {
	ids := m.getTargetInstanceIDs(domain.InstanceStatusStopped)
	if len(ids) == 0 {
		return m, nil
	}
	m.loading = true
	return m, m.executeInstanceAction(ActionStart, ids)
}

func (m Model) handleListRebootAction() (Model, tea.Cmd) {
	ids := m.getTargetInstanceIDs(domain.InstanceStatusRunning)
	if len(ids) == 0 {
		return m, nil
	}
	m.loading = true
	return m, m.executeInstanceAction(ActionReboot, ids)
}

func (m Model) confirmKeyword() string {
	switch m.confirmAction {
	case ActionStop:
		return "stop"
	case ActionTerminate:
		return "terminate"
	default:
		return ""
	}
}

func (m Model) confirmActionName() string {
	switch m.confirmAction {
	case ActionStop:
		return "STOP"
	case ActionTerminate:
		return "TERMINATE"
	default:
		return ""
	}
}

func (m Model) enterConfirmMode(action ConfirmAction, instanceIDs []string) (Model, tea.Cmd) {
	m.prevState = m.state
	m.state = ViewConfirm
	m.confirmAction = action
	m.confirmTargetIDs = instanceIDs
	m.confirmInput.Reset()
	m.confirmInput.Focus()
	return m, textinput.Blink
}

func (m Model) exitConfirmMode() Model {
	m.state = m.prevState
	m.confirmAction = ActionNone
	m.confirmTargetIDs = nil
	m.confirmInput.Reset()
	m.confirmInput.Blur()
	return m
}

func (m Model) handleConfirmInput(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m = m.exitConfirmMode()
		return m, nil
	case "enter":
		if m.confirmInput.Value() == m.confirmKeyword() {
			m.loading = true
			targetIDs := m.confirmTargetIDs
			action := m.confirmAction
			m = m.exitConfirmMode()
			return m, m.executeInstanceAction(action, targetIDs)
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.confirmInput, cmd = m.confirmInput.Update(msg)
		return m, cmd
	}
}

func (m Model) overlayConfirmModal(base string) string {
	modal := m.renderConfirmModal()
	modalWidth := lipgloss.Width(modal)
	modalHeight := lipgloss.Height(modal)
	x := (m.width - modalWidth) / 2
	y := (m.height - modalHeight) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return common.PlaceOverlay(x, y, modal, base)
}

func (m Model) renderConfirmModal() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(common.WarningColor).
		Padding(1, 2)

	warningStyle := lipgloss.NewStyle().
		Foreground(common.WarningColor).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(common.TextColor)

	keyword := m.confirmKeyword()
	actionName := m.confirmActionName()

	var instanceLines []string
	count := len(m.confirmTargetIDs)
	if count == 1 {
		id := m.confirmTargetIDs[0]
		name := id
		for _, inst := range m.allInstances {
			if inst.ID == id && inst.Name != "" {
				name = inst.Name
				break
			}
		}
		instanceLines = append(instanceLines,
			labelStyle.Render(fmt.Sprintf("Instance: %s", name)),
			labelStyle.Render(fmt.Sprintf("ID: %s", id)),
		)
	} else {
		instanceLines = append(instanceLines,
			labelStyle.Render(fmt.Sprintf("Instances: %d selected", count)),
		)
		for i, id := range m.confirmTargetIDs {
			if i >= 5 {
				instanceLines = append(instanceLines,
					labelStyle.Render(fmt.Sprintf("  ... and %d more", count-5)),
				)
				break
			}
			name := id
			for _, inst := range m.allInstances {
				if inst.ID == id && inst.Name != "" {
					name = inst.Name
					break
				}
			}
			instanceLines = append(instanceLines,
				labelStyle.Render(fmt.Sprintf("  - %s (%s)", name, id)),
			)
		}
	}

	elements := []string{
		warningStyle.Render(fmt.Sprintf("⚠ %s Instance(s)", actionName)),
		"",
	}
	elements = append(elements, instanceLines...)
	elements = append(elements,
		"",
		labelStyle.Render(fmt.Sprintf("Type '%s' to confirm:", keyword)),
		m.confirmInput.View(),
		"",
		lipgloss.NewStyle().Foreground(common.InfoColor).Render("esc: cancel | enter: confirm"),
	)

	content := lipgloss.JoinVertical(lipgloss.Left, elements...)
	return boxStyle.Render(content)
}
