package ecr

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
	ViewRepos ViewState = iota
	ViewTags
)

type Model struct {
	app           *app.App
	table         table.Model
	state         ViewState
	repos         []domain.Repository
	visibleRepoID []string
	selectedRepo  map[string]struct{}
	tags          []domain.ECRImageTag
	visibleTagID  []string
	selectedTag   map[string]struct{}
	currentRepo   string
	search        common.TableSearch
	loading       bool
	err           error
	width         int
	height        int
}

type reposLoadedMsg []domain.Repository
type tagsLoadedMsg []domain.ECRImageTag
type errMsg error

func NewModel(a *app.App) Model {
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Name", Width: 35},
			{Title: "URI", Width: 50},
			{Title: "Enc", Width: 8},
			{Title: "Mut", Width: 9},
			{Title: "Created", Width: 19},
		}),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	t.SetStyles(common.DefaultTableStyles())

	return Model{
		app:          a,
		table:        t,
		state:        ViewRepos,
		selectedRepo: make(map[string]struct{}),
		selectedTag:  make(map[string]struct{}),
		search:       common.NewTableSearch(),
	}
}

func (m Model) Init() tea.Cmd {
	return m.fetchRepos
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case reposLoadedMsg:
		m.repos = msg
		m.loading = false
		m.err = nil
		m.state = ViewRepos
		m = m.SetSize(m.width, m.height)
		m = m.applyFilter()
		return m, nil

	case tagsLoadedMsg:
		m.tags = msg
		m.loading = false
		m.err = nil
		m.state = ViewTags
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
			if m.state == ViewRepos {
				return m, m.fetchRepos
			}
			return m, m.fetchTags(m.currentRepo)
		case "enter":
			if m.state == ViewRepos {
				selected := m.table.SelectedRow()
				if len(selected) > 0 {
					m.currentRepo = selected[0]
					m.state = ViewTags
					m.loading = true
					m.table.SetRows(nil)
					m = m.SetSize(m.width, m.height)
					return m, m.fetchTags(m.currentRepo)
				}
			}
		case "backspace", "delete", "esc":
			if m.state == ViewTags {
				m.state = ViewRepos
				m.currentRepo = ""
				m.tags = nil
				m.visibleTagID = nil
				m.selectedTag = make(map[string]struct{})
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

	if m.state == ViewTags {
		// Tags view: Tag, Digest, Size, Pushed At
		const (
			cellPadding = 8
			minTag      = 18
			maxTag      = 35
			minDigest   = 16
			minSize     = 10
			minPushed   = 19
		)
		available := width - cellPadding
		if available < minTag+minDigest+minSize+minPushed {
			return m
		}

		tagWidth := maxTag
		remaining := available - tagWidth - minDigest - minSize - minPushed
		if remaining < 0 {
			tagWidth = minTag
			remaining = available - tagWidth - minDigest - minSize - minPushed
		}

		digestWidth := minDigest + remaining
		m.table.SetColumns([]table.Column{
			{Title: "Tag", Width: tagWidth},
			{Title: "Digest", Width: digestWidth},
			{Title: "Size", Width: minSize},
			{Title: "Pushed", Width: minPushed},
		})
		return m
	}

	// Dynamically adjust column widths
	// Table cell padding: 5 columns × 2 = 10
	const (
		cellPadding  = 10
		minName      = 30
		maxName      = 40
		minURI       = 30
		encWidth     = 8
		mutWidth     = 9
		createdWidth = 19
	)
	available := width - cellPadding
	if available < minName+minURI+encWidth+mutWidth+createdWidth {
		return m
	}

	nameWidth := maxName
	remaining := available - nameWidth - minURI - encWidth - mutWidth - createdWidth
	if remaining < 0 {
		nameWidth = minName
		remaining = available - nameWidth - minURI - encWidth - mutWidth - createdWidth
	}

	// Distribute remaining to URI
	uriWidth := minURI + remaining

	m.table.SetColumns([]table.Column{
		{Title: "Name", Width: nameWidth},
		{Title: "URI", Width: uriWidth},
		{Title: "Enc", Width: encWidth},
		{Title: "Mut", Width: mutWidth},
		{Title: "Created", Width: createdWidth},
	})
	return m
}

func (m Model) View() string {
	if m.err != nil {
		return common.RenderError(m.err)
	}

	header := "ECR Repositories"
	if m.state == ViewTags {
		header = fmt.Sprintf("Image Tags in %s", m.currentRepo)
	}

	if m.loading {
		return common.RenderLoading(header)
	}

	indicator := m.search.Indicator()
	tips := "r: refresh | /: search | space: select | j/k: navigate"
	if m.state == ViewRepos {
		tips = "enter: view tags | " + tips
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
	if m.state == ViewRepos {
		if n := len(m.selectedRepo); n > 0 {
			title = fmt.Sprintf("%s (%d selected)", title, n)
		}
	} else {
		if n := len(m.selectedTag); n > 0 {
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
	if m.state == ViewRepos {
		return common.HighlightSelectedTableRows(m.table.View(), m.table.Cursor(), m.visibleRepoID, m.selectedRepo)
	}
	return common.HighlightSelectedTableRows(m.table.View(), m.table.Cursor(), m.visibleTagID, m.selectedTag)
}

func (m Model) applyFilter() Model {
	prevCursor := m.table.Cursor()
	query := m.search.Query()

	if m.state == ViewTags {
		valid := make(map[string]struct{}, len(m.tags))
		rows := make([]table.Row, 0, len(m.tags))
		visible := make([]string, 0, len(m.tags))
		for _, t := range m.tags {
			valid[t.ID] = struct{}{}
			pushed := "-"
			if !t.PushedAt.IsZero() {
				pushed = t.PushedAt.Format("2006-01-02 15:04:05")
			}
			raw := table.Row{
				t.Tag,
				shortDigest(t.Digest),
				formatBytes(t.SizeBytes),
				pushed,
			}
			if common.TableRowMatches(raw, query) {
				rows = append(rows, raw)
				visible = append(visible, t.ID)
			}
		}

		for k := range m.selectedTag {
			if _, ok := valid[k]; !ok {
				delete(m.selectedTag, k)
			}
		}

		m.visibleTagID = visible
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

	valid := make(map[string]struct{}, len(m.repos))
	rows := make([]table.Row, 0, len(m.repos))
	visible := make([]string, 0, len(m.repos))
	for _, r := range m.repos {
		valid[r.Name] = struct{}{}

		created := "-"
		if !r.CreatedAt.IsZero() {
			created = r.CreatedAt.Format("2006-01-02 15:04:05")
		}

		raw := table.Row{
			r.Name,
			r.URI,
			common.DisplayValue(r.Encryption),
			common.DisplayValue(r.Mutability),
			created,
		}
		if common.TableRowMatches(raw, query) {
			rows = append(rows, raw)
			visible = append(visible, r.Name)
		}
	}

	for k := range m.selectedRepo {
		if _, ok := valid[k]; !ok {
			delete(m.selectedRepo, k)
		}
	}

	m.visibleRepoID = visible
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
	if m.state == ViewRepos {
		if idx < 0 || idx >= len(m.visibleRepoID) {
			return
		}
		key := m.visibleRepoID[idx]
		if _, ok := m.selectedRepo[key]; ok {
			delete(m.selectedRepo, key)
			return
		}
		m.selectedRepo[key] = struct{}{}
		return
	}

	if idx < 0 || idx >= len(m.visibleTagID) {
		return
	}
	key := m.visibleTagID[idx]
	if _, ok := m.selectedTag[key]; ok {
		delete(m.selectedTag, key)
		return
	}
	m.selectedTag[key] = struct{}{}
}

func (m Model) fetchRepos() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if m.app.AWSClient == nil {
		return errMsg(fmt.Errorf("AWS client not initialized"))
	}

	repos, err := m.app.AWSClient.ECR().ListRepositories(ctx)
	if err != nil {
		return errMsg(err)
	}
	return reposLoadedMsg(repos)
}

func (m Model) fetchTags(repo string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if m.app.AWSClient == nil {
			return errMsg(fmt.Errorf("AWS client not initialized"))
		}

		tags, err := m.app.AWSClient.ECR().ListImageTags(ctx, repo)
		if err != nil {
			return errMsg(err)
		}
		return tagsLoadedMsg(tags)
	}
}

func shortDigest(d string) string {
	d = strings.TrimSpace(d)
	if d == "" {
		return "-"
	}
	const prefix = "sha256:"
	d = strings.TrimPrefix(d, prefix)
	if len(d) <= 12 {
		return d
	}
	return d[:12]
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
