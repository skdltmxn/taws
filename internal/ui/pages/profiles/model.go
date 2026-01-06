package profiles

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/skdltmxn/taws/internal/app"
	"github.com/skdltmxn/taws/internal/config"
	"github.com/skdltmxn/taws/internal/domain"
	"github.com/skdltmxn/taws/internal/ui/common"
)

type Tab int

const (
	TabProfiles Tab = iota
	TabRegions
)

type Model struct {
	app             *app.App
	profileTable    table.Model
	regionTable     table.Model
	profiles        []domain.AWSProfile
	visibleProfiles []string
	selectedProfile map[string]struct{}
	visibleRegions  []string
	selectedRegion  map[string]struct{}
	activeTab       Tab
	search          common.TableSearch
	loading         bool
	err             error
	message         string
	width           int
	height          int
}

type profilesLoadedMsg []domain.AWSProfile
type profileSwitchedMsg struct {
	profile  string
	identity *domain.AWSIdentity
}
type regionSwitchedMsg struct {
	region   string
	identity *domain.AWSIdentity
}
type errMsg struct{ error }

// ProfileChangedMsg is sent when profile or region is successfully changed
type ProfileChangedMsg struct {
	Profile  string
	Identity *domain.AWSIdentity
}

// RegionChangedMsg is sent when region is successfully changed
type RegionChangedMsg struct {
	Region   string
	Identity *domain.AWSIdentity
}

func NewModel(a *app.App) Model {
	// Profile table
	pt := table.New(
		table.WithColumns([]table.Column{
			{Title: "Profile", Width: 25},
			{Title: "Region", Width: 15},
			{Title: "Type", Width: 10},
			{Title: "Status", Width: 15},
		}),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	pt.SetStyles(common.DefaultTableStyles())

	// Region table
	rt := table.New(
		table.WithColumns([]table.Column{
			{Title: "Region", Width: 20},
			{Title: "Status", Width: 15},
		}),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	rt.SetStyles(common.DefaultTableStyles())

	// Initialize region rows
	currentRegion := ""
	if a.Config != nil {
		currentRegion = a.Config.Region
	}
	regionRows := make([]table.Row, len(config.AWSRegions))
	for i, r := range config.AWSRegions {
		status := ""
		if r == currentRegion {
			status = "* Active"
		}
		regionRows[i] = table.Row{r, status}
	}
	rt.SetRows(regionRows)

	return Model{
		app:             a,
		profileTable:    pt,
		regionTable:     rt,
		activeTab:       TabProfiles,
		selectedProfile: make(map[string]struct{}),
		selectedRegion:  make(map[string]struct{}),
		search:          common.NewTableSearch(),
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadProfiles
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case profilesLoadedMsg:
		m.profiles = msg
		m.loading = false
		m.err = nil

		currentProfile := ""
		if m.app.Config != nil {
			currentProfile = m.app.Config.Profile
		}

		rows := make([]table.Row, len(m.profiles))
		for i, p := range m.profiles {
			profileType := "IAM"
			if p.IsSSO {
				profileType = "SSO"
			}

			status := ""
			if p.Name == currentProfile || (currentProfile == "" && p.IsDefault) {
				status = "* Active"
			}

			region := p.Region
			if region == "" {
				region = "-"
			}

			rows[i] = table.Row{
				p.Name,
				region,
				profileType,
				status,
			}
		}
		m.profileTable.SetRows(rows)
		if len(rows) > 0 {
			m.profileTable.SetCursor(0)
		}
		m = m.applyFilter()
		return m, nil

	case profileSwitchedMsg:
		m.loading = false
		m.message = fmt.Sprintf("Switched to profile: %s", msg.profile)
		// Refresh profile list to update status
		return m, tea.Batch(
			m.loadProfiles,
			func() tea.Msg {
				return ProfileChangedMsg{
					Profile:  msg.profile,
					Identity: msg.identity,
				}
			},
		)

	case regionSwitchedMsg:
		m.loading = false
		m.message = fmt.Sprintf("Switched to region: %s", msg.region)
		m = m.applyFilter()
		return m, func() tea.Msg {
			return RegionChangedMsg{
				Region:   msg.region,
				Identity: msg.identity,
			}
		}

	case errMsg:
		m.err = msg.error
		m.loading = false
		return m, nil

	case tea.KeyMsg:
		m.message = ""
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

		if m.activeTab == TabProfiles {
			if common.HandleTableNavKeys(&m.profileTable, msg.String()) {
				return m, nil
			}
		} else {
			if common.HandleTableNavKeys(&m.regionTable, msg.String()) {
				return m, nil
			}
		}

		switch msg.String() {
		case "/":
			m.search, cmd = m.search.Start()
			return m, cmd
		case "tab":
			// Switch between tabs
			if m.activeTab == TabProfiles {
				m.activeTab = TabRegions
				m.profileTable.Blur()
				m.regionTable.Focus()
			} else {
				m.activeTab = TabProfiles
				m.regionTable.Blur()
				m.profileTable.Focus()
			}
			m = m.applyFilter()
			return m, nil
		case "r":
			m.loading = true
			return m, m.loadProfiles
		case "enter":
			if m.activeTab == TabProfiles {
				selected := m.profileTable.SelectedRow()
				if len(selected) > 0 {
					profileName := selected[0]
					m.loading = true
					m.message = fmt.Sprintf("Switching to %s...", profileName)
					return m, m.switchProfile(profileName)
				}
			} else {
				selected := m.regionTable.SelectedRow()
				if len(selected) > 0 {
					regionName := selected[0]
					m.loading = true
					m.message = fmt.Sprintf("Switching to %s...", regionName)
					return m, m.switchRegion(regionName)
				}
			}
		}
	}

	if m.activeTab == TabProfiles {
		m.profileTable, cmd = m.profileTable.Update(msg)
	} else {
		m.regionTable, cmd = m.regionTable.Update(msg)
	}
	return m, cmd
}

func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height
	m.profileTable.SetHeight(height - 7)
	m.regionTable.SetHeight(height - 7)
	m.search = m.search.SetWidth(width)

	// Profile table: Profile, Region, Type, Status
	// Table cell padding: 4 columns × 2 = 8
	const (
		profileCellPadding = 8
		minProfile         = 25
		maxProfile         = 35
		minRegion          = 15
		minType            = 8
		minStatus          = 12
	)
	available := width - profileCellPadding
	if available >= minProfile+minRegion+minType+minStatus {
		profileWidth := maxProfile
		remaining := available - profileWidth - minRegion - minType - minStatus
		if remaining < 0 {
			profileWidth = minProfile
			remaining = available - profileWidth - minRegion - minType - minStatus
		}

		// Distribute remaining to Region and Status
		regionWidth := minRegion + remaining/2
		statusWidth := minStatus + remaining - remaining/2

		m.profileTable.SetColumns([]table.Column{
			{Title: "Profile", Width: profileWidth},
			{Title: "Region", Width: regionWidth},
			{Title: "Type", Width: minType},
			{Title: "Status", Width: statusWidth},
		})
	}

	// Region table: Region, Status
	// Table cell padding: 2 columns × 2 = 4
	const (
		regionCellPadding = 4
		minRegionCol      = 20
		maxRegionCol      = 30
		minStatusCol      = 15
	)
	available2 := width - regionCellPadding
	if available2 >= minRegionCol+minStatusCol {
		regionColWidth := maxRegionCol
		remaining := available2 - regionColWidth - minStatusCol
		if remaining < 0 {
			regionColWidth = minRegionCol
			remaining = available2 - regionColWidth - minStatusCol
		}

		statusColWidth := minStatusCol + remaining

		m.regionTable.SetColumns([]table.Column{
			{Title: "Region", Width: regionColWidth},
			{Title: "Status", Width: statusColWidth},
		})
	}
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

	profilesTab := inactiveTabStyle.Render("Profiles")
	regionsTab := inactiveTabStyle.Render("Regions")

	if m.activeTab == TabProfiles {
		profilesTab = activeTabStyle.Render("Profiles")
	} else {
		regionsTab = activeTabStyle.Render("Regions")
	}

	tabs := lipgloss.JoinHorizontal(lipgloss.Left, profilesTab, regionsTab)

	if m.loading && len(m.profiles) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left,
			common.TitleStyle.Render("AWS Settings"),
			tabs,
			"Loading...",
		)
	}

	var statusLine string
	if m.message != "" {
		statusLine = lipgloss.NewStyle().Foreground(common.SuccessColor).Render(m.message)
	}

	var tableView string
	var tips string
	if m.activeTab == TabProfiles {
		tableView = common.HighlightSelectedTableRows(m.profileTable.View(), m.profileTable.Cursor(), m.visibleProfiles, m.selectedProfile)
		tips = "tab: switch tab | enter: switch profile | r: refresh | /: search | space: select | j/k: navigate"
	} else {
		tableView = common.HighlightSelectedTableRows(m.regionTable.View(), m.regionTable.Cursor(), m.visibleRegions, m.selectedRegion)
		tips = "tab: switch tab | enter: switch region | /: search | space: select | j/k: navigate"
	}
	indicator := m.search.Indicator()
	if indicator != "" {
		tips = indicator + " | " + tips
	}
	if m.search.Active {
		tips = m.search.Input.View()
	}

	title := "AWS Settings"
	if n := m.selectedCount(); n > 0 {
		title = fmt.Sprintf("%s (%d selected)", title, n)
	}

	elements := []string{
		common.TitleStyle.Render(title),
		tabs,
	}
	if statusLine != "" {
		elements = append(elements, statusLine)
	}
	elements = append(elements,
		tableView,
		lipgloss.NewStyle().Foreground(common.InfoColor).Render(tips),
	)

	return lipgloss.JoinVertical(lipgloss.Left, elements...)
}

func (m Model) applyFilter() Model {
	query := m.search.Query()
	if m.activeTab == TabProfiles {
		prevCursor := m.profileTable.Cursor()
		currentProfile := ""
		if m.app.Config != nil {
			currentProfile = m.app.Config.Profile
		}

		valid := make(map[string]struct{}, len(m.profiles))
		rows := make([]table.Row, 0, len(m.profiles))
		visible := make([]string, 0, len(m.profiles))
		for _, p := range m.profiles {
			valid[p.Name] = struct{}{}
			profileType := "IAM"
			if p.IsSSO {
				profileType = "SSO"
			}

			status := ""
			if p.Name == currentProfile || (currentProfile == "" && p.IsDefault) {
				status = "* Active"
			}

			region := p.Region
			if region == "" {
				region = "-"
			}

			raw := table.Row{
				p.Name,
				region,
				profileType,
				status,
			}
			if common.TableRowMatches(raw, query) {
				rows = append(rows, raw)
				visible = append(visible, p.Name)
			}
		}

		for k := range m.selectedProfile {
			if _, ok := valid[k]; !ok {
				delete(m.selectedProfile, k)
			}
		}

		m.visibleProfiles = visible
		m.profileTable.SetRows(rows)
		if len(rows) > 0 {
			if prevCursor < 0 {
				prevCursor = 0
			}
			if prevCursor >= len(rows) {
				prevCursor = len(rows) - 1
			}
			m.profileTable.SetCursor(prevCursor)
		}
		return m
	}

	prevCursor := m.regionTable.Cursor()
	currentRegion := ""
	if m.app.Config != nil {
		currentRegion = m.app.Config.Region
	}

	valid := make(map[string]struct{}, len(config.AWSRegions))
	rows := make([]table.Row, 0, len(config.AWSRegions))
	visible := make([]string, 0, len(config.AWSRegions))
	for _, r := range config.AWSRegions {
		valid[r] = struct{}{}
		status := ""
		if r == currentRegion {
			status = "* Active"
		}
		raw := table.Row{r, status}
		if common.TableRowMatches(raw, query) {
			rows = append(rows, raw)
			visible = append(visible, r)
		}
	}

	for k := range m.selectedRegion {
		if _, ok := valid[k]; !ok {
			delete(m.selectedRegion, k)
		}
	}

	m.visibleRegions = visible
	m.regionTable.SetRows(rows)
	if len(rows) > 0 {
		if prevCursor < 0 {
			prevCursor = 0
		}
		if prevCursor >= len(rows) {
			prevCursor = len(rows) - 1
		}
		m.regionTable.SetCursor(prevCursor)
	}
	return m
}

func (m *Model) toggleSelected() {
	if m.activeTab == TabProfiles {
		idx := m.profileTable.Cursor()
		if idx < 0 || idx >= len(m.visibleProfiles) {
			return
		}
		key := m.visibleProfiles[idx]
		if _, ok := m.selectedProfile[key]; ok {
			delete(m.selectedProfile, key)
			return
		}
		m.selectedProfile[key] = struct{}{}
		return
	}

	idx := m.regionTable.Cursor()
	if idx < 0 || idx >= len(m.visibleRegions) {
		return
	}
	key := m.visibleRegions[idx]
	if _, ok := m.selectedRegion[key]; ok {
		delete(m.selectedRegion, key)
		return
	}
	m.selectedRegion[key] = struct{}{}
}

func (m Model) selectedCount() int {
	if m.activeTab == TabProfiles {
		return len(m.selectedProfile)
	}
	return len(m.selectedRegion)
}

func (m Model) loadProfiles() tea.Msg {
	profiles, err := config.ListProfiles()
	if err != nil {
		return errMsg{err}
	}
	return profilesLoadedMsg(profiles)
}

func (m Model) switchProfile(profileName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Get region from profile or use current
		var region string
		for _, p := range m.profiles {
			if p.Name == profileName {
				region = p.Region
				break
			}
		}
		if region == "" && m.app.Config != nil {
			region = m.app.Config.Region
		}

		// Re-initialize AWS client with new profile
		if err := m.app.SwitchProfile(ctx, profileName, region); err != nil {
			return errMsg{err}
		}

		return profileSwitchedMsg{
			profile:  profileName,
			identity: m.app.Identity,
		}
	}
}

func (m Model) switchRegion(regionName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := m.app.SwitchRegion(ctx, regionName); err != nil {
			return errMsg{err}
		}

		return regionSwitchedMsg{
			region:   regionName,
			identity: m.app.Identity,
		}
	}
}
