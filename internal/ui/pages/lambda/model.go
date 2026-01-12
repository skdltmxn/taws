package lambda

import (
	"context"
	"fmt"
	"strings"
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
	ViewCode
)

type ConfirmAction int

const (
	ActionNone ConfirmAction = iota
	ActionDelete
)

type Model struct {
	app          *app.App
	table        table.Model
	allFunctions []domain.LambdaFunction
	functions    []domain.LambdaFunction
	rowFunction  []string
	selectedIDs  map[string]struct{}
	search       common.TableSearch
	loading      bool
	err          error
	loaded       bool
	state        ViewState
	selected     *domain.LambdaFunction
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

	// Code view
	codeFiles        []domain.LambdaCodeFile
	codeFileIndex    int
	codeScrollY      int
	codeLoading      bool
	codeFunctionName string
	codeFullscreen   bool
}

type functionsLoadedMsg []domain.LambdaFunction
type errMsg struct{ error }
type actionSuccessMsg string
type actionErrorMsg struct{ error }
type autoRefreshTickMsg time.Time
type codeLoadedMsg struct {
	files        []domain.LambdaCodeFile
	functionName string
}
type codeErrorMsg struct{ error }

const (
	autoRefreshInterval = 30 * time.Second
)

func NewModel(a *app.App) Model {
	columns := []table.Column{
		{Title: "Name", Width: 30},
		{Title: "Runtime", Width: 15},
		{Title: "Handler", Width: 25},
		{Title: "Memory", Width: 8},
		{Title: "Timeout", Width: 8},
		{Title: "State", Width: 10},
		{Title: "Last Modified", Width: 19},
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
	return m.fetchFunctions
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case functionsLoadedMsg:
		m.allFunctions = msg
		m.loading = false
		m.loaded = true
		m.err = nil
		m = m.applyFilter()
		return m, m.scheduleAutoRefresh()

	case autoRefreshTickMsg:
		if m.loading || m.state == ViewConfirm {
			return m, m.scheduleAutoRefresh()
		}
		return m, m.fetchFunctions

	case actionSuccessMsg:
		m.actionMsg = string(msg)
		m.loading = true
		return m, m.fetchFunctions

	case actionErrorMsg:
		m.err = msg
		m.loading = false
		return m, nil

	case errMsg:
		m.err = msg
		m.loading = false
		return m, nil

	case codeLoadedMsg:
		m.codeLoading = false
		m.codeFiles = msg.files
		m.codeFunctionName = msg.functionName
		m.codeFileIndex = 0
		m.codeScrollY = 0
		m.codeFullscreen = true
		m.state = ViewCode
		return m, nil

	case codeErrorMsg:
		m.codeLoading = false
		m.err = msg
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

		if m.state == ViewCode {
			return m.handleCodeViewInput(msg)
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
			case "D":
				if m.selected != nil {
					return m.enterConfirmMode(ActionDelete, []string{m.selected.Name})
				}
			case "c":
				if m.selected != nil {
					m.codeLoading = true
					return m, m.fetchCode(m.selected.Name)
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
			if idx >= 0 && idx < len(m.functions) {
				key := m.functions[idx].Name
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
			return m, m.fetchFunctions
		case "enter":
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.functions) {
				m.selected = &m.functions[idx]
				m.state = ViewDetail
			}
			return m, nil
		case "D":
			return m.handleListDeleteAction()
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

	const cellPadding = 14
	available := width - cellPadding

	minName := 20
	minRuntime := 12
	minHandler := 20
	minMemory := 8
	minTimeout := 8
	minState := 10
	minLastMod := 19
	minTotal := minName + minRuntime + minHandler + minMemory + minTimeout + minState + minLastMod

	if available < minTotal {
		return m
	}

	extra := available - minTotal
	nameWidth := minName + extra/2
	handlerWidth := minHandler + extra - extra/2

	m.table.SetColumns([]table.Column{
		{Title: "Name", Width: nameWidth},
		{Title: "Runtime", Width: minRuntime},
		{Title: "Handler", Width: handlerWidth},
		{Title: "Memory", Width: minMemory},
		{Title: "Timeout", Width: minTimeout},
		{Title: "State", Width: minState},
		{Title: "Last Modified", Width: minLastMod},
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
	return m.state == ViewYAML && m.yamlView.IsFullscreen() || m.state == ViewCode && m.codeFullscreen
}

func (m Model) FullscreenView() string {
	if m.state == ViewCode {
		return m.renderCodeView()
	}
	return m.yamlView.View()
}

func (m Model) showYAMLView() (Model, tea.Cmd) {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.functions) {
		return m, nil
	}

	fn := &m.functions[idx]
	title := fmt.Sprintf("Lambda Function: %s", fn.Name)
	m.yamlView = m.yamlView.SetContent(title, fn)
	m.state = ViewYAML
	return m, nil
}

func (m Model) View() string {
	if m.err != nil {
		return common.RenderError(m.err)
	}
	if m.loading && !m.loaded {
		return "Loading Lambda functions..."
	}

	if m.state == ViewYAML {
		return m.yamlView.View()
	}

	if m.state == ViewCode && !m.codeFullscreen {
		return m.renderCodeView()
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
	tips := "enter: detail | D: delete | y: yaml | r: refresh | /: search | space: select"
	if indicator != "" {
		tips = indicator + " | " + tips
	}
	if m.search.Active {
		tips = m.search.Input.View()
	}

	title := "Lambda Functions"
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
		common.HighlightSelectedTableRows(m.table.View(), m.table.Cursor(), m.rowFunction, m.selectedIDs),
		lipgloss.NewStyle().Foreground(common.InfoColor).Render(tips),
	)

	return lipgloss.JoinVertical(lipgloss.Left, elements...)
}

func (m Model) applyFilter() Model {
	prevCursor := m.table.Cursor()
	query := m.search.Query()

	valid := make(map[string]struct{}, len(m.allFunctions))
	m.functions = nil
	m.rowFunction = nil
	rows := make([]table.Row, 0, len(m.allFunctions))
	for _, fn := range m.allFunctions {
		valid[fn.Name] = struct{}{}
		raw := table.Row{
			fn.Name,
			fn.Runtime,
			fn.Handler,
			fmt.Sprintf("%d MB", fn.MemorySize),
			fmt.Sprintf("%ds", fn.Timeout),
			string(fn.State),
			fn.LastModified.Format("2006-01-02 15:04"),
		}
		if common.TableRowMatches(raw, query) {
			m.functions = append(m.functions, fn)
			m.rowFunction = append(m.rowFunction, fn.Name)
			rows = append(rows, raw)
		}
	}

	for k := range m.selectedIDs {
		if _, ok := valid[k]; !ok {
			delete(m.selectedIDs, k)
		}
	}

	if m.state == ViewDetail && m.selected != nil {
		selectedName := m.selected.Name
		for _, fn := range m.allFunctions {
			if fn.Name == selectedName {
				fnCopy := fn
				m.selected = &fnCopy
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
	fn := m.selected

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
	switch fn.State {
	case domain.FunctionStateActive:
		stateColor = common.SuccessColor
	case domain.FunctionStatePending:
		stateColor = common.WarningColor
	case domain.FunctionStateFailed, domain.FunctionStateInactive:
		stateColor = common.ErrorColor
	}

	stateValue := lipgloss.NewStyle().Foreground(stateColor).Render(string(fn.State))

	details := lipgloss.JoinVertical(lipgloss.Left,
		row("Name", fn.Name),
		row("ARN", fn.ARN),
		lipgloss.JoinHorizontal(lipgloss.Left,
			labelStyle.Render("State:"),
			stateValue,
		),
		row("Runtime", fn.Runtime),
		row("Handler", fn.Handler),
		row("Memory", fmt.Sprintf("%d MB", fn.MemorySize)),
		row("Timeout", fmt.Sprintf("%d seconds", fn.Timeout)),
		row("Code Size", formatBytes(fn.CodeSize)),
		row("Role", fn.Role),
		row("Description", common.DisplayValue(fn.Description)),
		row("Last Modified", fn.LastModified.Format("2006-01-02 15:04:05")),
	)

	var statusLine string
	if m.actionMsg != "" {
		statusLine = lipgloss.NewStyle().Foreground(common.SuccessColor).Render(m.actionMsg) + "\n"
	}

	tips := "backspace/esc: back | c: view code | D: delete"

	return lipgloss.JoinVertical(lipgloss.Left,
		common.TitleStyle.Render("Lambda Function Detail"),
		statusLine,
		detailStyle.Render(details),
		lipgloss.NewStyle().Foreground(common.InfoColor).Render(tips),
	)
}

func (m Model) fetchFunctions() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if m.app.AWSClient == nil {
		return errMsg{fmt.Errorf("AWS client not initialized")}
	}

	functions, err := m.app.AWSClient.Lambda().ListFunctions(ctx)
	if err != nil {
		return errMsg{err}
	}
	return functionsLoadedMsg(functions)
}

func (m Model) scheduleAutoRefresh() tea.Cmd {
	return tea.Tick(autoRefreshInterval, func(t time.Time) tea.Msg {
		return autoRefreshTickMsg(t)
	})
}

func (m Model) executeAction(action ConfirmAction, functionNames []string) tea.Cmd {
	return func() tea.Msg {
		if m.app.AWSClient == nil {
			return actionErrorMsg{fmt.Errorf("AWS client not initialized")}
		}

		lambdaClient := m.app.AWSClient.Lambda()

		switch action {
		case ActionDelete:
			var succeeded []string
			for _, name := range functionNames {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				err := lambdaClient.DeleteFunction(ctx, name)
				cancel()

				if err != nil {
					if len(succeeded) > 0 {
						return actionErrorMsg{fmt.Errorf("%d function(s) deleted, but failed on %s: %w", len(succeeded), name, err)}
					}
					return actionErrorMsg{err}
				}
				succeeded = append(succeeded, name)
			}

			if len(functionNames) == 1 {
				return actionSuccessMsg(fmt.Sprintf("Function %s deleted", functionNames[0]))
			}
			return actionSuccessMsg(fmt.Sprintf("%d function(s) deleted", len(functionNames)))
		}

		return actionErrorMsg{fmt.Errorf("unknown action")}
	}
}

func (m Model) getTargetFunctionNames() []string {
	var names []string
	if len(m.selectedIDs) > 0 {
		for _, fn := range m.functions {
			if _, ok := m.selectedIDs[fn.Name]; ok {
				names = append(names, fn.Name)
			}
		}
	} else {
		idx := m.table.Cursor()
		if idx >= 0 && idx < len(m.functions) {
			names = append(names, m.functions[idx].Name)
		}
	}
	return names
}

func (m Model) handleListDeleteAction() (Model, tea.Cmd) {
	names := m.getTargetFunctionNames()
	if len(names) == 0 {
		return m, nil
	}
	return m.enterConfirmMode(ActionDelete, names)
}

func (m Model) enterConfirmMode(action ConfirmAction, functionNames []string) (Model, tea.Cmd) {
	m.prevState = m.state
	m.state = ViewConfirm
	m.confirmAction = action
	m.confirmTargetIDs = functionNames
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
		if m.confirmInput.Value() == "delete" {
			m.loading = true
			targetIDs := m.confirmTargetIDs
			action := m.confirmAction
			m = m.exitConfirmMode()
			return m, m.executeAction(action, targetIDs)
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

	var functionLines []string
	count := len(m.confirmTargetIDs)
	if count == 1 {
		functionLines = append(functionLines,
			labelStyle.Render(fmt.Sprintf("Function: %s", m.confirmTargetIDs[0])),
		)
	} else {
		functionLines = append(functionLines,
			labelStyle.Render(fmt.Sprintf("Functions: %d selected", count)),
		)
		for i, name := range m.confirmTargetIDs {
			if i >= 5 {
				functionLines = append(functionLines,
					labelStyle.Render(fmt.Sprintf("  ... and %d more", count-5)),
				)
				break
			}
			functionLines = append(functionLines,
				labelStyle.Render(fmt.Sprintf("  - %s", name)),
			)
		}
	}

	elements := []string{
		warningStyle.Render("DELETE Function(s)"),
		"",
	}
	elements = append(elements, functionLines...)
	elements = append(elements,
		"",
		labelStyle.Render("Type 'delete' to confirm:"),
		m.confirmInput.View(),
		"",
		lipgloss.NewStyle().Foreground(common.InfoColor).Render("esc: cancel | enter: confirm"),
	)

	content := lipgloss.JoinVertical(lipgloss.Left, elements...)
	return boxStyle.Render(content)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func (m Model) fetchCode(functionName string) tea.Cmd {
	return func() tea.Msg {
		if m.app.AWSClient == nil {
			return codeErrorMsg{fmt.Errorf("AWS client not initialized")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		files, err := m.app.AWSClient.Lambda().GetFunctionCode(ctx, functionName)
		if err != nil {
			return codeErrorMsg{err}
		}

		if len(files) == 0 {
			return codeErrorMsg{fmt.Errorf("no source files found in function package")}
		}

		return codeLoadedMsg{files: files, functionName: functionName}
	}
}

func (m Model) codeViewHeight() int {
	if m.codeFullscreen {
		return m.fullHeight - 6
	}
	return m.height - 6
}

func (m Model) handleCodeViewInput(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "backspace", "esc", "q":
		if !m.codeFullscreen {
			m.state = ViewDetail
			m.codeFiles = nil
			m.codeFileIndex = 0
			m.codeScrollY = 0
		} else {
			m.codeFullscreen = false
		}
		return m, nil
	case "f":
		m.codeFullscreen = !m.codeFullscreen
		return m, nil
	case "tab", "l", "right":
		if len(m.codeFiles) > 1 {
			m.codeFileIndex = (m.codeFileIndex + 1) % len(m.codeFiles)
			m.codeScrollY = 0
		}
		return m, nil
	case "shift+tab", "h", "left":
		if len(m.codeFiles) > 1 {
			m.codeFileIndex = (m.codeFileIndex - 1 + len(m.codeFiles)) % len(m.codeFiles)
			m.codeScrollY = 0
		}
		return m, nil
	case "j", "down":
		m.codeScrollY++
		return m, nil
	case "k", "up":
		if m.codeScrollY > 0 {
			m.codeScrollY--
		}
		return m, nil
	case "g":
		m.codeScrollY = 0
		return m, nil
	case "G":
		if len(m.codeFiles) > 0 && m.codeFileIndex < len(m.codeFiles) {
			lines := countLines(m.codeFiles[m.codeFileIndex].Content)
			viewHeight := m.codeViewHeight()
			if lines > viewHeight {
				m.codeScrollY = lines - viewHeight
			}
		}
		return m, nil
	case "ctrl+d":
		m.codeScrollY += m.codeViewHeight() / 2
		return m, nil
	case "ctrl+u":
		m.codeScrollY -= m.codeViewHeight() / 2
		if m.codeScrollY < 0 {
			m.codeScrollY = 0
		}
		return m, nil
	}
	return m, nil
}

func (m Model) renderCodeView() string {
	if m.codeLoading {
		return "Loading function code..."
	}

	if len(m.codeFiles) == 0 {
		return "No source files available"
	}

	file := m.codeFiles[m.codeFileIndex]
	viewHeight := m.codeViewHeight()
	lines := splitLines(file.Content)

	if m.codeScrollY >= len(lines) && len(lines) > 0 {
		m.codeScrollY = len(lines) - 1
	}

	endLine := m.codeScrollY + viewHeight
	if endLine > len(lines) {
		endLine = len(lines)
	}

	visibleLines := lines[m.codeScrollY:endLine]

	if m.codeFullscreen {
		return strings.Join(visibleLines, "\n")
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(common.PrimaryColor).
		Bold(true)

	fileTabStyle := lipgloss.NewStyle().
		Foreground(common.SubTextColor).
		Padding(0, 1)

	activeTabStyle := lipgloss.NewStyle().
		Foreground(common.TextColor).
		Background(common.PrimaryColor).
		Padding(0, 1)

	var tabs []string
	for i, f := range m.codeFiles {
		if i == m.codeFileIndex {
			tabs = append(tabs, activeTabStyle.Render(f.Path))
		} else {
			tabs = append(tabs, fileTabStyle.Render(f.Path))
		}
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Left, tabs...)

	title := titleStyle.Render(fmt.Sprintf("Code: %s", m.codeFunctionName))

	lineNumStyle := lipgloss.NewStyle().
		Foreground(common.SubTextColor).
		Width(5).
		Align(lipgloss.Right)

	codeStyle := lipgloss.NewStyle().
		Foreground(common.TextColor)

	var codeLines []string
	for i, line := range visibleLines {
		lineNum := m.codeScrollY + i + 1
		codeLines = append(codeLines,
			lipgloss.JoinHorizontal(lipgloss.Left,
				lineNumStyle.Render(fmt.Sprintf("%d", lineNum)),
				" ",
				codeStyle.Render(line),
			),
		)
	}

	codeContent := lipgloss.JoinVertical(lipgloss.Left, codeLines...)

	scrollInfo := fmt.Sprintf("Line %d-%d of %d", m.codeScrollY+1, endLine, len(lines))
	tips := "esc/q: back | f: fullscreen | tab/h/l: switch file | j/k: scroll | g/G: top/bottom | ctrl+d/u: page"

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		tabBar,
		"",
		codeContent,
		"",
		lipgloss.NewStyle().Foreground(common.SubTextColor).Render(scrollInfo),
		lipgloss.NewStyle().Foreground(common.InfoColor).Render(tips),
	)
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func countLines(s string) int {
	count := 1
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			count++
		}
	}
	return count
}
