package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/skdltmxn/taws/internal/app"
	"github.com/skdltmxn/taws/internal/ui/common"
	"github.com/skdltmxn/taws/internal/ui/components/cmdpalette"
	"github.com/skdltmxn/taws/internal/ui/components/header"
	"github.com/skdltmxn/taws/internal/ui/pages/cloudwatch"
	"github.com/skdltmxn/taws/internal/ui/pages/ec2"
	"github.com/skdltmxn/taws/internal/ui/pages/ecr"
	"github.com/skdltmxn/taws/internal/ui/pages/eks"
	"github.com/skdltmxn/taws/internal/ui/pages/iam"
	"github.com/skdltmxn/taws/internal/ui/pages/profiles"
	"github.com/skdltmxn/taws/internal/ui/pages/route53"
	"github.com/skdltmxn/taws/internal/ui/pages/s3"
	"github.com/skdltmxn/taws/internal/ui/pages/vpc"
)

type Page int

const (
	PageDashboard Page = iota
	PageEC2
	PageVPC
	PageS3
	PageEKS
	PageECR
	PageIAM
	PageRoute53
	PageCloudWatch
	PageProfiles
)

var pageNames = map[Page]string{
	PageDashboard:  "Dashboard",
	PageEC2:        "EC2 Instances",
	PageVPC:        "VPC",
	PageS3:         "S3 Buckets",
	PageEKS:        "EKS Clusters",
	PageECR:        "ECR Repositories",
	PageIAM:        "IAM Users",
	PageRoute53:    "Route53 Hosted Zones",
	PageCloudWatch: "CloudWatch Logs",
	PageProfiles:   "AWS Profiles",
}

var pageAliases = map[string]Page{
	"ec2":       PageEC2,
	"vpc":       PageVPC,
	"s3":        PageS3,
	"eks":       PageEKS,
	"ecr":       PageECR,
	"iam":       PageIAM,
	"route53":   PageRoute53,
	"logs":      PageCloudWatch,
	"profiles":  PageProfiles,
	"home":      PageDashboard,
	"dashboard": PageDashboard,
}

var commands = []cmdpalette.Command{
	{Alias: "ec2", Name: "EC2 Instances", Page: int(PageEC2)},
	{Alias: "vpc", Name: "VPC", Page: int(PageVPC)},
	{Alias: "s3", Name: "S3 Buckets", Page: int(PageS3)},
	{Alias: "eks", Name: "EKS Clusters", Page: int(PageEKS)},
	{Alias: "ecr", Name: "ECR Repositories", Page: int(PageECR)},
	{Alias: "iam", Name: "IAM Users", Page: int(PageIAM)},
	{Alias: "route53", Name: "Route53 Hosted Zones", Page: int(PageRoute53)},
	{Alias: "logs", Name: "CloudWatch Logs", Page: int(PageCloudWatch)},
	{Alias: "profiles", Name: "AWS Profiles", Page: int(PageProfiles)},
	{Alias: "home", Name: "Dashboard", Page: int(PageDashboard)},
}

type Model struct {
	app            *app.App
	currentPage    Page
	width          int
	height         int
	quitting       bool
	err            error
	showHelp       bool
	showCmdPalette bool
	cmdPalette     cmdpalette.Model

	// Sub-models
	ec2Model        ec2.Model
	s3Model         s3.Model
	vpcModel        vpc.Model
	eksModel        eks.Model
	ecrModel        ecr.Model
	iamModel        iam.Model
	route53Model    route53.Model
	cloudwatchModel cloudwatch.Model
	profilesModel   profiles.Model
}

func NewModel(a *app.App) Model {
	// Restore last resource from config
	initialPage := PageDashboard
	if a.Config != nil && a.Config.LastResource != "" {
		if page, ok := pageAliases[a.Config.LastResource]; ok {
			initialPage = page
		}
	}

	return Model{
		app:             a,
		currentPage:     initialPage,
		cmdPalette:      cmdpalette.NewModel(commands),
		ec2Model:        ec2.NewModel(a),
		s3Model:         s3.NewModel(a),
		vpcModel:        vpc.NewModel(a),
		eksModel:        eks.NewModel(a),
		ecrModel:        ecr.NewModel(a),
		iamModel:        iam.NewModel(a),
		route53Model:    route53.NewModel(a),
		cloudwatchModel: cloudwatch.NewModel(a),
		profilesModel:   profiles.NewModel(a),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.ec2Model.Init(),
		m.s3Model.Init(),
		m.vpcModel.Init(),
		m.eksModel.Init(),
		m.ecrModel.Init(),
		m.iamModel.Init(),
		m.route53Model.Init(),
		m.cloudwatchModel.Init(),
		m.profilesModel.Init(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle command palette messages
	switch msg := msg.(type) {
	case cmdpalette.SelectMsg:
		m.showCmdPalette = false
		m.currentPage = Page(msg.Page)
		m.cmdPalette = m.cmdPalette.Reset()
		// Save last resource to config
		m.saveCurrentState()
		return m, m.getPageInitCmd()
	case cmdpalette.CloseMsg:
		m.showCmdPalette = false
		m.cmdPalette = m.cmdPalette.Reset()
		return m, nil
	case profiles.ProfileChangedMsg, profiles.RegionChangedMsg:
		// Profile or region was changed, save state and refresh all page models
		m.saveCurrentState()
		return m, tea.Batch(
			m.ec2Model.Init(),
			m.s3Model.Init(),
			m.vpcModel.Init(),
			m.eksModel.Init(),
			m.ecrModel.Init(),
			m.iamModel.Init(),
			m.route53Model.Init(),
			m.cloudwatchModel.Init(),
		)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If help is shown, any key closes it
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		// If command palette is shown, route keys to it
		if m.showCmdPalette {
			var cmd tea.Cmd
			m.cmdPalette, cmd = m.cmdPalette.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "?":
			m.showHelp = true
			return m, nil
		case ":":
			m.showCmdPalette = true
			m.cmdPalette = m.cmdPalette.Reset()
			return m, m.cmdPalette.Focus()
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Full width layout (header takes 5 lines, status bar takes 1 line)
		// Content area: width - 2 (border) - 2 (padding) - 2 (table row prefix) = width - 6
		// Height: total - 5 (header) - 1 (status) - 2 (border) - 1 (padding) = height - 9
		contentWidth := m.width - 6 // border(2) + padding(2) + table row prefix(2)
		contentHeight := m.height - 9

		m.cmdPalette = m.cmdPalette.SetWidth(m.width)
		m.ec2Model = m.ec2Model.SetSize(contentWidth, contentHeight)
		m.s3Model = m.s3Model.SetSize(contentWidth, contentHeight)
		m.vpcModel = m.vpcModel.SetSize(contentWidth, contentHeight)
		m.eksModel = m.eksModel.SetSize(contentWidth, contentHeight)
		m.ecrModel = m.ecrModel.SetSize(contentWidth, contentHeight)
		m.iamModel = m.iamModel.SetSize(contentWidth, contentHeight)
		m.route53Model = m.route53Model.SetSize(contentWidth, contentHeight)
		m.cloudwatchModel = m.cloudwatchModel.SetSize(contentWidth, contentHeight)
		m.profilesModel = m.profilesModel.SetSize(contentWidth, contentHeight)

		m.ec2Model = m.ec2Model.SetFullscreenSize(m.width, m.height)
		m.s3Model = m.s3Model.SetFullscreenSize(m.width, m.height)
		m.vpcModel = m.vpcModel.SetFullscreenSize(m.width, m.height)
		m.eksModel = m.eksModel.SetFullscreenSize(m.width, m.height)
		m.ecrModel = m.ecrModel.SetFullscreenSize(m.width, m.height)
		m.iamModel = m.iamModel.SetFullscreenSize(m.width, m.height)
		m.route53Model = m.route53Model.SetFullscreenSize(m.width, m.height)
		m.cloudwatchModel = m.cloudwatchModel.SetFullscreenSize(m.width, m.height)
	}

	switch m.currentPage {
	case PageEC2:
		var ec2Cmd tea.Cmd
		m.ec2Model, ec2Cmd = m.ec2Model.Update(msg)
		cmds = append(cmds, ec2Cmd)
	case PageS3:
		var s3Cmd tea.Cmd
		m.s3Model, s3Cmd = m.s3Model.Update(msg)
		cmds = append(cmds, s3Cmd)
	case PageVPC:
		var vpcCmd tea.Cmd
		m.vpcModel, vpcCmd = m.vpcModel.Update(msg)
		cmds = append(cmds, vpcCmd)
	case PageEKS:
		var eksCmd tea.Cmd
		m.eksModel, eksCmd = m.eksModel.Update(msg)
		cmds = append(cmds, eksCmd)
	case PageECR:
		var ecrCmd tea.Cmd
		m.ecrModel, ecrCmd = m.ecrModel.Update(msg)
		cmds = append(cmds, ecrCmd)
	case PageIAM:
		var iamCmd tea.Cmd
		m.iamModel, iamCmd = m.iamModel.Update(msg)
		cmds = append(cmds, iamCmd)
	case PageRoute53:
		var route53Cmd tea.Cmd
		m.route53Model, route53Cmd = m.route53Model.Update(msg)
		cmds = append(cmds, route53Cmd)
	case PageCloudWatch:
		var cwCmd tea.Cmd
		m.cloudwatchModel, cwCmd = m.cloudwatchModel.Update(msg)
		cmds = append(cmds, cwCmd)
	case PageProfiles:
		var profilesCmd tea.Cmd
		m.profilesModel, profilesCmd = m.profilesModel.Update(msg)
		cmds = append(cmds, profilesCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) getPageInitCmd() tea.Cmd {
	switch m.currentPage {
	case PageEC2:
		return m.ec2Model.Init()
	case PageS3:
		return m.s3Model.Init()
	case PageVPC:
		return m.vpcModel.Init()
	case PageEKS:
		return m.eksModel.Init()
	case PageECR:
		return m.ecrModel.Init()
	case PageIAM:
		return m.iamModel.Init()
	case PageRoute53:
		return m.route53Model.Init()
	case PageCloudWatch:
		return m.cloudwatchModel.Init()
	case PageProfiles:
		return m.profilesModel.Init()
	}
	return nil
}

// saveCurrentState saves the current state to user config
func (m Model) saveCurrentState() {
	if m.app == nil || m.app.Config == nil {
		return
	}

	// Find alias for current page
	var resourceAlias string
	for alias, page := range pageAliases {
		if page == m.currentPage {
			resourceAlias = alias
			break
		}
	}

	m.app.Config.LastResource = resourceAlias
	_ = m.app.Config.SaveUserState()
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 {
		return "Loading..."
	}

	if m.showHelp {
		return m.renderHelp()
	}

	if fs := m.getFullscreenView(); fs != "" {
		return fs
	}

	headerView := m.renderHeader()
	content := m.renderContent()
	statusBar := m.renderStatusBar()

	mainView := lipgloss.JoinVertical(lipgloss.Left, headerView, content, statusBar)

	// Overlay command palette if shown
	if m.showCmdPalette {
		palette := m.cmdPalette.View()
		mainView = lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			palette,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
		)
	}

	return mainView
}

func (m Model) getFullscreenView() string {
	switch m.currentPage {
	case PageEC2:
		if m.ec2Model.IsFullscreen() {
			return m.ec2Model.FullscreenView()
		}
	case PageS3:
		if m.s3Model.IsFullscreen() {
			return m.s3Model.FullscreenView()
		}
	case PageVPC:
		if m.vpcModel.IsFullscreen() {
			return m.vpcModel.FullscreenView()
		}
	case PageEKS:
		if m.eksModel.IsFullscreen() {
			return m.eksModel.FullscreenView()
		}
	case PageECR:
		if m.ecrModel.IsFullscreen() {
			return m.ecrModel.FullscreenView()
		}
	case PageIAM:
		if m.iamModel.IsFullscreen() {
			return m.iamModel.FullscreenView()
		}
	case PageRoute53:
		if m.route53Model.IsFullscreen() {
			return m.route53Model.FullscreenView()
		}
	case PageCloudWatch:
		if m.cloudwatchModel.IsFullscreen() {
			return m.cloudwatchModel.FullscreenView()
		}
	}
	return ""
}

func (m Model) renderHeader() string {
	info := header.Info{
		AccountID:    "N/A",
		Region:       "N/A",
		Profile:      "default",
		UserARN:      "N/A",
		ResourceName: pageNames[m.currentPage],
	}

	if m.app.Config != nil {
		if m.app.Config.Region != "" {
			info.Region = m.app.Config.Region
		}
		if m.app.Config.Profile != "" {
			info.Profile = m.app.Config.Profile
		}
	}

	if m.app.Identity != nil {
		info.AccountID = m.app.Identity.AccountID
		info.UserARN = m.app.Identity.Arn
	}

	return header.Render(info, m.width)
}

func (m Model) renderHelp() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(common.PrimaryColor).
		Bold(true).
		MarginBottom(1)

	sectionStyle := lipgloss.NewStyle().
		Foreground(common.TextColor).
		MarginBottom(1)

	keyStyle := lipgloss.NewStyle().
		Foreground(common.PrimaryColor).
		Width(15)

	descStyle := lipgloss.NewStyle().
		Foreground(common.SubTextColor)

	helpLine := func(key, desc string) string {
		return lipgloss.JoinHorizontal(lipgloss.Left,
			keyStyle.Render(key),
			descStyle.Render(desc),
		)
	}

	globalKeys := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("Global Keys"),
		helpLine("?", "Show this help"),
		helpLine(":", "Open command palette"),
		helpLine("ctrl+c", "Quit"),
	)

	navKeys := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("Navigation"),
		helpLine("j / down", "Move down"),
		helpLine("k / up", "Move up"),
		helpLine("g", "Go to top"),
		helpLine("G", "Go to bottom"),
		helpLine("/", "Search/filter list"),
		helpLine("enter", "Select / Open"),
		helpLine("backspace", "Go back"),
		helpLine("r", "Refresh"),
	)

	cmdPaletteKeys := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("Command Palette"),
		helpLine("up/ctrl+p", "Previous item"),
		helpLine("down/ctrl+n", "Next item"),
		helpLine("enter", "Select"),
		helpLine("esc", "Close"),
	)

	ec2Keys := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("EC2 Actions (in detail view)"),
		helpLine("S", "Start instance"),
		helpLine("s", "Stop instance"),
		helpLine("R", "Reboot instance"),
	)

	content := lipgloss.JoinVertical(lipgloss.Left,
		sectionStyle.Render(globalKeys),
		sectionStyle.Render(navKeys),
		sectionStyle.Render(cmdPaletteKeys),
		sectionStyle.Render(ec2Keys),
		"",
		descStyle.Render("Press any key to close help"),
	)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(common.PrimaryColor).
		Padding(1, 2).
		Width(50)

	helpBox := boxStyle.Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		helpBox,
	)
}

func (m Model) renderContent() string {
	if m.app.Identity == nil {
		title := common.TitleStyle.Render("Welcome to taws")
		errMsg := "No AWS credentials found."
		if m.app.AuthError != nil {
			errMsg = m.app.AuthError.Error()
		}
		hints := []string{
			"Please configure your AWS credentials:",
			"",
			"  1. Set environment variables:",
			"     export AWS_ACCESS_KEY_ID=<key>",
			"     export AWS_SECRET_ACCESS_KEY=<secret>",
			"",
			"  2. Or use AWS SSO profile:",
			"     taws --profile <profile-name>",
			"",
			"  3. Or configure default credentials:",
			"     aws configure",
		}
		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			common.ErrorStyle.Render(errMsg),
			"",
			lipgloss.JoinVertical(lipgloss.Left, hints...),
		)
		return lipgloss.NewStyle().Padding(1).Height(m.height - 4).Render(content)
	}

	var pageContent string
	switch m.currentPage {
	case PageDashboard:
		pageContent = m.renderDashboard()
	case PageEC2:
		pageContent = m.ec2Model.View()
	case PageS3:
		pageContent = m.s3Model.View()
	case PageVPC:
		pageContent = m.vpcModel.View()
	case PageEKS:
		pageContent = m.eksModel.View()
	case PageECR:
		pageContent = m.ecrModel.View()
	case PageIAM:
		pageContent = m.iamModel.View()
	case PageRoute53:
		pageContent = m.route53Model.View()
	case PageCloudWatch:
		pageContent = m.cloudwatchModel.View()
	case PageProfiles:
		pageContent = m.profilesModel.View()
	default:
		pageContent = fmt.Sprintf("Page %d is under construction.", m.currentPage)
	}

	// Content area with border (subtract 2 for border only, padding is inside)
	contentHeight := m.height - 9
	contentWidth := m.width - 2
	title, body := common.SplitTitle(pageContent)
	contentBody := pageContent
	if title != "" {
		contentBody = body
	}

	box := common.ContentStyle.
		Width(contentWidth).
		Height(contentHeight).
		Render(contentBody)

	return common.WithBorderTitle(box, title)
}

func (m Model) renderDashboard() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(common.PrimaryColor).
		Bold(true).
		MarginBottom(1)

	descStyle := lipgloss.NewStyle().
		Foreground(common.SubTextColor)

	aliasStyle := lipgloss.NewStyle().
		Foreground(common.PrimaryColor).
		Width(12)

	nameStyle := lipgloss.NewStyle().
		Foreground(common.TextColor)

	resourceLine := func(alias, name string) string {
		return lipgloss.JoinHorizontal(lipgloss.Left,
			aliasStyle.Render(":"+alias),
			nameStyle.Render(name),
		)
	}

	resources := lipgloss.JoinVertical(lipgloss.Left,
		resourceLine("ec2", "EC2 Instances"),
		resourceLine("vpc", "VPC"),
		resourceLine("s3", "S3 Buckets"),
		resourceLine("eks", "EKS Clusters"),
		resourceLine("ecr", "ECR Repositories"),
		resourceLine("iam", "IAM Users"),
		resourceLine("route53", "Route53 Hosted Zones"),
		resourceLine("logs", "CloudWatch Logs"),
		resourceLine("profiles", "AWS Profiles"),
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("Dashboard"),
		"",
		descStyle.Render("Press : to open command palette, or use shortcuts below:"),
		"",
		resources,
		"",
		descStyle.Render("Press ? for help"),
	)
}

func (m Model) renderStatusBar() string {
	status := "Ready"
	if m.err != nil {
		status = common.ErrorStyle.Render(m.err.Error())
	}

	w := m.width
	if w == 0 {
		w = 80
	}

	tips := "?: help | :: command palette | ctrl+c: quit"
	tips = tips + " | /: search"

	return common.StatusBarStyle.
		Width(w).
		Render(fmt.Sprintf(" %s | %s", status, tips))
}
