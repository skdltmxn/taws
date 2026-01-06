package iam

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

type Tab int

const (
	TabUsers Tab = iota
	TabRoles
)

type Model struct {
	app *app.App

	userTable table.Model
	roleTable table.Model

	users        []domain.IAMUser
	roles        []domain.IAMRole
	visibleUsers []string
	visibleRoles []string
	selectedUser map[string]struct{}
	selectedRole map[string]struct{}

	activeTab    Tab
	search       common.TableSearch
	loadingUsers bool
	loadingRoles bool
	err          error
	width        int
	height       int
}

type usersLoadedMsg []domain.IAMUser
type rolesLoadedMsg []domain.IAMRole
type errMsg error

func NewModel(a *app.App) Model {
	ut := table.New(
		table.WithColumns([]table.Column{
			{Title: "User Name", Width: 20},
			{Title: "User ID", Width: 20},
			{Title: "Created Date", Width: 25},
			{Title: "ARN", Width: 50},
		}),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	ut.SetStyles(common.DefaultTableStyles())

	rt := table.New(
		table.WithColumns([]table.Column{
			{Title: "Role Name", Width: 25},
			{Title: "Role ID", Width: 22},
			{Title: "Created Date", Width: 25},
			{Title: "ARN", Width: 50},
		}),
		table.WithFocused(false),
		table.WithHeight(10),
	)

	rt.SetStyles(common.DefaultTableStyles())

	return Model{
		app:          a,
		userTable:    ut,
		roleTable:    rt,
		activeTab:    TabUsers,
		selectedUser: make(map[string]struct{}),
		selectedRole: make(map[string]struct{}),
		search:       common.NewTableSearch(),
	}
}

func (m Model) Init() tea.Cmd {
	return m.fetchUsers
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case usersLoadedMsg:
		m.users = msg
		m.loadingUsers = false
		m.err = nil
		m = m.applyFilter()
		return m, nil

	case rolesLoadedMsg:
		m.roles = msg
		m.loadingRoles = false
		m.err = nil
		m = m.applyFilter()
		return m, nil

	case errMsg:
		m.err = msg
		m.loadingUsers = false
		m.loadingRoles = false
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

		if m.activeTab == TabUsers {
			if common.HandleTableNavKeys(&m.userTable, msg.String()) {
				return m, nil
			}
		} else {
			if common.HandleTableNavKeys(&m.roleTable, msg.String()) {
				return m, nil
			}
		}
		switch msg.String() {
		case "/":
			m.search, cmd = m.search.Start()
			return m, cmd
		case "tab":
			if m.activeTab == TabUsers {
				m.activeTab = TabRoles
				m.userTable.Blur()
				m.roleTable.Focus()
				if len(m.roles) == 0 && !m.loadingRoles {
					m.loadingRoles = true
					return m, m.fetchRoles
				}
			} else {
				m.activeTab = TabUsers
				m.roleTable.Blur()
				m.userTable.Focus()
			}
			m = m.applyFilter()
			return m, nil
		case "r":
			if m.activeTab == TabUsers {
				m.loadingUsers = true
				return m, m.fetchUsers
			}
			m.loadingRoles = true
			return m, m.fetchRoles
		}
	}

	if m.activeTab == TabUsers {
		m.userTable, cmd = m.userTable.Update(msg)
		return m, cmd
	}
	m.roleTable, cmd = m.roleTable.Update(msg)
	return m, cmd
}

func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height
	m.userTable.SetHeight(height - 6)
	m.roleTable.SetHeight(height - 6)
	m.search = m.search.SetWidth(width)

	// Dynamically adjust column widths
	// Table cell padding: 4 columns × 2 = 8
	const (
		cellPadding = 8
		minName    = 18
		maxName    = 25
		minID      = 22
		minCreated = 22
		minARN     = 40
	)
	available := width - cellPadding
	if available < minName+minID+minCreated+minARN {
		return m
	}

	nameWidth := maxName
	remaining := available - nameWidth - minID - minCreated - minARN
	if remaining < 0 {
		nameWidth = minName
		remaining = available - nameWidth - minID - minCreated - minARN
	}

	// Distribute remaining mostly to ARN
	arnWidth := minARN + remaining*2/3
	createdWidth := minCreated + remaining - remaining*2/3

	m.userTable.SetColumns([]table.Column{
		{Title: "User Name", Width: nameWidth},
		{Title: "User ID", Width: minID},
		{Title: "Created Date", Width: createdWidth},
		{Title: "ARN", Width: arnWidth},
	})

	m.roleTable.SetColumns([]table.Column{
		{Title: "Role Name", Width: nameWidth},
		{Title: "Role ID", Width: minID},
		{Title: "Created Date", Width: createdWidth},
		{Title: "ARN", Width: arnWidth},
	})
	return m
}

func (m Model) View() string {
	if m.err != nil {
		return common.RenderError(m.err)
	}

	// Tab headers
	tabStyle := lipgloss.NewStyle().Padding(0, 2)
	activeTabStyle := tabStyle.Foreground(common.PrimaryColor).Bold(true).Underline(true)
	inactiveTabStyle := tabStyle.Foreground(common.SubTextColor)

	usersTab := inactiveTabStyle.Render("Users")
	rolesTab := inactiveTabStyle.Render("Roles")
	if m.activeTab == TabUsers {
		usersTab = activeTabStyle.Render("Users")
	} else {
		rolesTab = activeTabStyle.Render("Roles")
	}
	tabs := lipgloss.JoinHorizontal(lipgloss.Left, usersTab, rolesTab)

	if m.activeTab == TabUsers && m.loadingUsers && len(m.users) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left,
			common.TitleStyle.Render("IAM"),
			tabs,
			"Loading...",
		)
	}
	if m.activeTab == TabRoles && m.loadingRoles && len(m.roles) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left,
			common.TitleStyle.Render("IAM"),
			tabs,
			"Loading...",
		)
	}

	indicator := m.search.Indicator()
	tips := "tab: switch tab | r: refresh | /: search | space: select | j/k: navigate"
	if indicator != "" {
		tips = indicator + " | " + tips
	}
	if m.search.Active {
		tips = m.search.Input.View()
	}

	title := "IAM"
	if n := m.selectedCount(); n > 0 {
		title = fmt.Sprintf("%s (%d selected)", title, n)
	}

	var tableView string
	if m.activeTab == TabUsers {
		tableView = common.HighlightSelectedTableRows(m.userTable.View(), m.userTable.Cursor(), m.visibleUsers, m.selectedUser)
	} else {
		tableView = common.HighlightSelectedTableRows(m.roleTable.View(), m.roleTable.Cursor(), m.visibleRoles, m.selectedRole)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		common.TitleStyle.Render(title),
		tabs,
		tableView,
		lipgloss.NewStyle().Foreground(common.InfoColor).Render(tips),
	)
}

func (m Model) applyFilter() Model {
	query := m.search.Query()

	if m.activeTab == TabUsers {
		prevCursor := m.userTable.Cursor()
		valid := make(map[string]struct{}, len(m.users))
		rows := make([]table.Row, 0, len(m.users))
		visible := make([]string, 0, len(m.users))
		for _, u := range m.users {
			valid[u.UserName] = struct{}{}
			raw := table.Row{
				u.UserName,
				u.UserID,
				u.CreateDate.Format("2006-01-02 15:04:05"),
				u.Arn,
			}
			if common.TableRowMatches(raw, query) {
				rows = append(rows, raw)
				visible = append(visible, u.UserName)
			}
		}

		for k := range m.selectedUser {
			if _, ok := valid[k]; !ok {
				delete(m.selectedUser, k)
			}
		}

		m.visibleUsers = visible
		m.userTable.SetRows(rows)
		if len(rows) > 0 {
			if prevCursor < 0 {
				prevCursor = 0
			}
			if prevCursor >= len(rows) {
				prevCursor = len(rows) - 1
			}
			m.userTable.SetCursor(prevCursor)
		}
		return m
	}

	prevCursor := m.roleTable.Cursor()
	valid := make(map[string]struct{}, len(m.roles))
	rows := make([]table.Row, 0, len(m.roles))
	visible := make([]string, 0, len(m.roles))
	for _, r := range m.roles {
		valid[r.RoleName] = struct{}{}
		raw := table.Row{
			r.RoleName,
			r.RoleID,
			r.CreateDate.Format("2006-01-02 15:04:05"),
			r.Arn,
		}
		if common.TableRowMatches(raw, query) {
			rows = append(rows, raw)
			visible = append(visible, r.RoleName)
		}
	}

	for k := range m.selectedRole {
		if _, ok := valid[k]; !ok {
			delete(m.selectedRole, k)
		}
	}

	m.visibleRoles = visible
	m.roleTable.SetRows(rows)
	if len(rows) > 0 {
		if prevCursor < 0 {
			prevCursor = 0
		}
		if prevCursor >= len(rows) {
			prevCursor = len(rows) - 1
		}
		m.roleTable.SetCursor(prevCursor)
	}
	return m
}

func (m *Model) toggleSelected() {
	if m.activeTab == TabUsers {
		idx := m.userTable.Cursor()
		if idx < 0 || idx >= len(m.visibleUsers) {
			return
		}
		key := m.visibleUsers[idx]
		if _, ok := m.selectedUser[key]; ok {
			delete(m.selectedUser, key)
			return
		}
		m.selectedUser[key] = struct{}{}
		return
	}

	idx := m.roleTable.Cursor()
	if idx < 0 || idx >= len(m.visibleRoles) {
		return
	}
	key := m.visibleRoles[idx]
	if _, ok := m.selectedRole[key]; ok {
		delete(m.selectedRole, key)
		return
	}
	m.selectedRole[key] = struct{}{}
}

func (m Model) fetchUsers() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if m.app.AWSClient == nil {
		return errMsg(fmt.Errorf("AWS client not initialized"))
	}

	users, err := m.app.AWSClient.IAM().ListUsers(ctx)
	if err != nil {
		return errMsg(err)
	}
	return usersLoadedMsg(users)
}

func (m Model) fetchRoles() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if m.app.AWSClient == nil {
		return errMsg(fmt.Errorf("AWS client not initialized"))
	}

	roles, err := m.app.AWSClient.IAM().ListRoles(ctx)
	if err != nil {
		return errMsg(err)
	}
	return rolesLoadedMsg(roles)
}

func (m Model) selectedCount() int {
	return len(m.selectedUser) + len(m.selectedRole)
}
