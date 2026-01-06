package s3

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
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
	ViewBuckets ViewState = iota
	ViewObjects
)

type Model struct {
	app            *app.App
	table          table.Model
	state          ViewState
	buckets        []domain.Bucket
	visibleBucket  []string
	selectedBuck   map[string]struct{}
	objects        []domain.S3Object
	visibleObject  []string
	selectedObj    map[string]struct{}
	currentBucket  string
	prefix         string
	search         common.TableSearch
	downloadInput  textinput.Model
	downloadOpen   bool
	downloadIsDir  bool
	downloadKeys   []string
	downloadCh     chan tea.Msg
	downloading    bool
	downloadCancel context.CancelFunc
	dlKey          string
	dlWritten      int64
	dlTotal        int64
	dlDone         int
	dlTotalN       int
	loading        bool
	message        string
	messageIsErr   bool
	err            error
	width          int
	height         int
}

type bucketsLoadedMsg []domain.Bucket
type objectsLoadedMsg []domain.S3Object
type errMsg error
type statusMsg struct {
	text    string
	isError bool
}

type downloadProgressMsg struct {
	key     string
	written int64
	total   int64
	done    int
	totalN  int
}

type downloadDoneMsg struct {
	text    string
	isError bool
}

func NewModel(a *app.App) Model {
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Name", Width: 40},
			{Title: "Creation Date", Width: 30},
		}),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	t.SetStyles(common.DefaultTableStyles())

	di := textinput.New()
	di.Prompt = "save to: "
	di.CharLimit = 512
	di.Width = 50
	di.PromptStyle = lipgloss.NewStyle().Foreground(common.InfoColor)
	di.TextStyle = lipgloss.NewStyle().Foreground(common.TextColor)

	return Model{
		app:            a,
		table:          t,
		state:          ViewBuckets,
		selectedBuck:   make(map[string]struct{}),
		selectedObj:    make(map[string]struct{}),
		search:         common.NewTableSearch(),
		downloadInput:  di,
		downloadKeys:   nil,
		downloadOpen:   false,
		downloadIsDir:  false,
		downloadCh:     nil,
		downloading:    false,
		downloadCancel: nil,
	}
}

func (m Model) Init() tea.Cmd {
	return m.fetchBuckets
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case bucketsLoadedMsg:
		m.buckets = msg
		m.loading = false
		m.err = nil
		m.state = ViewBuckets
		m = m.applyFilter()
		return m, nil

	case objectsLoadedMsg:
		m.objects = msg
		m.loading = false
		m.err = nil
		m.state = ViewObjects
		m = m.applyFilter()
		return m, nil

	case errMsg:
		m.err = msg
		m.loading = false
		return m, nil

	case statusMsg:
		m.message = msg.text
		m.messageIsErr = msg.isError
		return m, nil

	case downloadProgressMsg:
		m.downloading = true
		m.dlKey = msg.key
		m.dlWritten = msg.written
		m.dlTotal = msg.total
		m.dlDone = msg.done
		m.dlTotalN = msg.totalN
		if m.downloadCh != nil {
			return m, listenDownload(m.downloadCh)
		}
		return m, nil

	case downloadDoneMsg:
		m.downloading = false
		m.downloadCh = nil
		if m.downloadCancel != nil {
			m.downloadCancel()
		}
		m.downloadCancel = nil
		m.dlKey = ""
		m.dlWritten = 0
		m.dlTotal = 0
		m.dlDone = 0
		m.dlTotalN = 0
		m.message = msg.text
		m.messageIsErr = msg.isError
		return m, nil

	case tea.KeyMsg:
		m.message = ""
		m.messageIsErr = false

		if m.downloading && msg.String() == "c" && m.downloadCancel != nil {
			m.message = "Cancelling..."
			m.messageIsErr = false
			m.downloadCancel()
			return m, nil
		}

		if m.downloadOpen {
			switch msg.String() {
			case "esc":
				m.downloadOpen = false
				m.downloadKeys = nil
				m.downloadIsDir = false
				m.downloadInput.Blur()
				m.downloadInput.Reset()
				return m, nil
			case "enter":
				p := strings.TrimSpace(m.downloadInput.Value())
				keys := append([]string(nil), m.downloadKeys...)
				isDir := m.downloadIsDir
				m.downloadOpen = false
				m.downloadKeys = nil
				m.downloadIsDir = false
				m.downloadInput.Blur()
				m.downloadInput.Reset()
				m.message = "Downloading..."
				ch := make(chan tea.Msg, 128)
				ctx, cancel := context.WithCancel(context.Background())
				m.downloadCh = ch
				m.downloading = true
				m.downloadCancel = cancel
				return m, m.downloadObjectsToPath(ctx, ch, keys, p, isDir)
			default:
				var c tea.Cmd
				m.downloadInput, c = m.downloadInput.Update(msg)
				return m, c
			}
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
			if m.state == ViewBuckets {
				return m, m.fetchBuckets
			} else {
				return m, m.fetchObjects(m.currentBucket, m.prefix)
			}
		case "enter":
			if m.state == ViewBuckets {
				selected := m.table.SelectedRow()
				if len(selected) > 0 {
					m.currentBucket = selected[0]
					m.prefix = ""
					m.loading = true
					m.table.SetRows(nil)
					return m, m.fetchObjects(m.currentBucket, m.prefix)
				}
			} else if m.state == ViewObjects {
				selected := m.table.SelectedRow()
				if len(selected) > 0 {
					name := selected[0]
					// Check if folder (ends with /)
					if strings.HasSuffix(name, "/") {
						m.prefix = m.prefix + name
						m.loading = true
						m.table.SetRows(nil)
						return m, m.fetchObjects(m.currentBucket, m.prefix)
					}
				}
			}
		case "d":
			if m.state == ViewObjects {
				m2, c := m.openDownloadPrompt()
				return m2, c
			}
		case "backspace", "delete", "esc":
			if m.state == ViewObjects {
				if m.prefix == "" {
					// Go back to buckets
					m.state = ViewBuckets
					m.loading = true
					m.table.SetRows(nil)
					return m, m.fetchBuckets
				} else {
					// Go up one level
					p := strings.TrimSuffix(m.prefix, "/")
					i := strings.LastIndex(p, "/")
					if i == -1 {
						m.prefix = ""
					} else {
						m.prefix = p[:i+1]
					}
					m.loading = true
					m.table.SetRows(nil)
					return m, m.fetchObjects(m.currentBucket, m.prefix)
				}
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
	m.downloadInput.Width = max(min(80, width-10), 20)

	// Dynamically adjust column widths based on current view
	// Table cell padding: columns × 2 (left+right padding per cell)
	if m.state == ViewObjects {
		// Objects view: Name, Size, Last Modified
		const (
			cellPadding = 6
			minName     = 30
			maxName     = 50
			minSize     = 12
			minModified = 22
		)
		available := width - cellPadding
		if available < minName+minSize+minModified {
			return m
		}

		nameWidth := maxName
		remaining := available - nameWidth - minSize - minModified
		if remaining < 0 {
			nameWidth = minName
			remaining = available - nameWidth - minSize - minModified
		}

		// Distribute remaining to Size and Modified
		sizeWidth := minSize + remaining/3
		modifiedWidth := minModified + remaining - remaining/3

		m.table.SetColumns([]table.Column{
			{Title: "Name", Width: nameWidth},
			{Title: "Size", Width: sizeWidth},
			{Title: "Last Modified", Width: modifiedWidth},
		})
	} else {
		// Buckets view: Name, Creation Date
		const (
			cellPadding = 4
			minName     = 30
			maxName     = 50
			minDate     = 22
		)
		available := width - cellPadding
		if available < minName+minDate {
			return m
		}

		nameWidth := maxName
		remaining := available - nameWidth - minDate
		if remaining < 0 {
			nameWidth = minName
			remaining = available - nameWidth - minDate
		}

		// Distribute remaining to date
		dateWidth := minDate + remaining

		m.table.SetColumns([]table.Column{
			{Title: "Name", Width: nameWidth},
			{Title: "Creation Date", Width: dateWidth},
		})
	}
	return m
}

func (m Model) View() string {
	if m.err != nil {
		return common.RenderError(m.err)
	}

	header := "S3 Buckets"
	if m.state == ViewObjects {
		header = fmt.Sprintf("S3://%s/%s", m.currentBucket, m.prefix)
	}

	if m.loading {
		return common.RenderLoading(header)
	}

	tips := "enter: open | esc/backspace: back | r: refresh | j/k: navigate | /: search | space: select"
	if m.state == ViewObjects {
		tips = tips + " | d: download"
	}
	if m.downloading {
		tips = tips + " | c: cancel"
	}
	indicator := m.search.Indicator()
	if indicator != "" {
		tips = indicator + " | " + tips
	}
	if m.search.Active {
		tips = m.search.Input.View()
	}
	if m.downloadOpen {
		tips = m.downloadInput.View() + " | enter: download | esc: cancel"
	}

	title := header
	if n := m.selectedCount(); n > 0 {
		title = fmt.Sprintf("%s (%d selected)", title, n)
	}

	var statusLine string
	if m.message != "" {
		color := common.SuccessColor
		if m.messageIsErr {
			color = common.ErrorColor
		}
		statusLine = lipgloss.NewStyle().Foreground(color).Render(m.message)
	}
	if m.downloading {
		statusLine = lipgloss.NewStyle().Foreground(common.InfoColor).Render(m.downloadStatus())
	}

	tableView := m.table.View()
	if m.state == ViewObjects {
		tableView = common.HighlightSelectedTableRows(tableView, m.table.Cursor(), m.visibleObject, m.selectedObj)
	} else {
		tableView = common.HighlightSelectedTableRows(tableView, m.table.Cursor(), m.visibleBucket, m.selectedBuck)
	}

	elements := []string{common.TitleStyle.Render(title)}
	if statusLine != "" {
		elements = append(elements, statusLine)
	}
	elements = append(elements,
		tableView,
		lipgloss.NewStyle().Foreground(common.InfoColor).Render(tips),
	)
	return lipgloss.JoinVertical(lipgloss.Left, elements...)
}

func (m Model) openDownloadPrompt() (Model, tea.Cmd) {
	if m.app == nil || m.app.AWSClient == nil {
		m.message = "AWS client not initialized"
		m.messageIsErr = true
		return m, nil
	}
	if m.state != ViewObjects {
		m.message = "Not in objects view"
		m.messageIsErr = true
		return m, nil
	}
	if m.currentBucket == "" {
		m.message = "No bucket selected"
		m.messageIsErr = true
		return m, nil
	}

	keys := make([]string, 0)
	if len(m.selectedObj) > 0 {
		for k := range m.selectedObj {
			keys = append(keys, k)
		}
	} else {
		idx := m.table.Cursor()
		if idx >= 0 && idx < len(m.visibleObject) {
			keys = append(keys, m.visibleObject[idx])
		}
	}
	if len(keys) == 0 {
		m.message = "Select an object to download"
		m.messageIsErr = true
		return m, nil
	}

	fileKeys := make([]string, 0, len(keys))
	for _, k := range keys {
		if m.isFolderKey(k) {
			continue
		}
		fileKeys = append(fileKeys, k)
	}
	if len(fileKeys) == 0 {
		m.message = "No file objects selected"
		m.messageIsErr = true
		return m, nil
	}

	isDir := len(fileKeys) > 1
	defaultPath := ""
	if isDir {
		base, err := common.DefaultDownloadBaseDir()
		if err != nil {
			m.message = err.Error()
			m.messageIsErr = true
			return m, nil
		}
		defaultPath = filepath.Join(base, m.currentBucket)
	} else {
		p, err := common.S3DownloadPath(m.currentBucket, fileKeys[0])
		if err != nil {
			m.message = err.Error()
			m.messageIsErr = true
			return m, nil
		}
		defaultPath = p
	}

	m.downloadIsDir = isDir
	m.downloadKeys = fileKeys
	m.downloadOpen = true
	m.downloadInput.SetValue(defaultPath)
	m.downloadInput.CursorEnd()
	return m, m.downloadInput.Focus()
}

func (m Model) downloadObjectsToPath(parent context.Context, ch chan tea.Msg, keys []string, userPath string, isDir bool) tea.Cmd {
	appRef := m.app
	bucket := m.currentBucket
	return func() tea.Msg {
		go func() {
			defer close(ch)

			if appRef == nil || appRef.AWSClient == nil {
				ch <- downloadDoneMsg{text: "AWS client not initialized", isError: true}
				return
			}
			if bucket == "" {
				ch <- downloadDoneMsg{text: "No bucket selected", isError: true}
				return
			}
			if len(keys) == 0 {
				ch <- downloadDoneMsg{text: "Select an object to download", isError: true}
				return
			}

			p, err := common.ExpandUserPath(userPath)
			if err != nil {
				ch <- downloadDoneMsg{text: err.Error(), isError: true}
				return
			}
			p = strings.TrimSpace(p)
			if p == "" {
				ch <- downloadDoneMsg{text: "Destination path is empty", isError: true}
				return
			}

			ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
			defer cancel()

			downloaded := 0
			totalN := len(keys)
			var lastPath string
			for _, key := range keys {
				dst, err := m.resolveDestPath(p, bucket, key, isDir)
				if err != nil {
					ch <- downloadDoneMsg{text: err.Error(), isError: true}
					return
				}

				err = appRef.AWSClient.S3().DownloadObject(ctx, bucket, key, dst, func(w, t int64) {
					msg := downloadProgressMsg{
						key:     key,
						written: w,
						total:   t,
						done:    downloaded,
						totalN:  totalN,
					}
					select {
					case ch <- msg:
					default:
					}
				})
				if err != nil {
					if errors.Is(err, context.Canceled) {
						ch <- downloadDoneMsg{text: "Cancelled", isError: false}
						return
					}
					ch <- downloadDoneMsg{text: err.Error(), isError: true}
					return
				}

				downloaded++
				lastPath = dst
			}

			if downloaded == 1 {
				ch <- downloadDoneMsg{text: fmt.Sprintf("Downloaded: %s", lastPath), isError: false}
				return
			}
			ch <- downloadDoneMsg{text: fmt.Sprintf("Downloaded %d objects", downloaded), isError: false}
		}()

		return <-ch
	}
}

func listenDownload(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func (m Model) resolveDestPath(base string, bucket string, key string, isDir bool) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return "", fmt.Errorf("destination path is empty")
	}

	looksDir := isDir || strings.HasSuffix(base, "/") || strings.HasSuffix(base, "\\") || strings.HasSuffix(base, string(os.PathSeparator))
	if !looksDir {
		if st, err := os.Stat(base); err == nil && st.IsDir() {
			looksDir = true
		}
	}

	if looksDir {
		dst := filepath.Join(base, bucket, filepath.FromSlash(key))
		return common.UniquePath(dst), nil
	}
	return common.UniquePath(base), nil
}

func (m Model) downloadStatus() string {
	if m.dlTotalN <= 0 {
		return "Downloading..."
	}
	name := path.Base(m.dlKey)
	if name == "" || name == "." || name == "/" {
		name = m.dlKey
	}
	if m.dlTotal > 0 {
		pct := int(float64(m.dlWritten) * 100 / float64(m.dlTotal))
		return fmt.Sprintf("Downloading %s %d%% (%s/%s) [%d/%d]", name, pct, formatBytes(m.dlWritten), formatBytes(m.dlTotal), m.dlDone+1, m.dlTotalN)
	}
	return fmt.Sprintf("Downloading %s (%s) [%d/%d]", name, formatBytes(m.dlWritten), m.dlDone+1, m.dlTotalN)
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	suffix := "KMGTPE"[exp : exp+1]
	return fmt.Sprintf("%.1f%siB", float64(n)/float64(div), suffix)
}

func (m Model) isFolderKey(key string) bool {
	if strings.HasSuffix(key, "/") {
		return true
	}
	for _, o := range m.objects {
		if o.Key == key {
			return o.IsFolder
		}
	}
	return false
}

func (m Model) applyFilter() Model {
	prevCursor := m.table.Cursor()
	query := m.search.Query()

	var rows []table.Row
	switch m.state {
	case ViewObjects:
		valid := make(map[string]struct{}, len(m.objects))
		rows = make([]table.Row, 0, len(m.objects))
		visible := make([]string, 0, len(m.objects))
		for _, o := range m.objects {
			valid[o.Key] = struct{}{}
			name := path.Base(o.Key)
			if o.IsFolder {
				name = name + "/"
			}

			size := ""
			if !o.IsFolder {
				size = fmt.Sprintf("%d", o.Size)
			}

			date := ""
			if !o.IsFolder {
				date = o.LastModified.Format("2006-01-02 15:04:05")
			}

			raw := table.Row{name, size, date}
			if common.TableRowMatches(raw, query) {
				rows = append(rows, raw)
				visible = append(visible, o.Key)
			}
		}
		for k := range m.selectedObj {
			if _, ok := valid[k]; !ok {
				delete(m.selectedObj, k)
			}
		}
		m.visibleObject = visible
	default:
		valid := make(map[string]struct{}, len(m.buckets))
		rows = make([]table.Row, 0, len(m.buckets))
		visible := make([]string, 0, len(m.buckets))
		for _, b := range m.buckets {
			valid[b.Name] = struct{}{}
			raw := table.Row{
				b.Name,
				b.CreationDate.Format("2006-01-02 15:04:05"),
			}
			if common.TableRowMatches(raw, query) {
				rows = append(rows, raw)
				visible = append(visible, b.Name)
			}
		}
		for k := range m.selectedBuck {
			if _, ok := valid[k]; !ok {
				delete(m.selectedBuck, k)
			}
		}
		m.visibleBucket = visible
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
	if m.state == ViewObjects {
		if idx >= len(m.visibleObject) {
			return
		}
		key := m.visibleObject[idx]
		if _, ok := m.selectedObj[key]; ok {
			delete(m.selectedObj, key)
			return
		}
		m.selectedObj[key] = struct{}{}
		return
	}

	if idx >= len(m.visibleBucket) {
		return
	}
	key := m.visibleBucket[idx]
	if _, ok := m.selectedBuck[key]; ok {
		delete(m.selectedBuck, key)
		return
	}
	m.selectedBuck[key] = struct{}{}
}

func (m Model) selectedCount() int {
	if m.state == ViewObjects {
		return len(m.selectedObj)
	}
	return len(m.selectedBuck)
}

func (m Model) fetchBuckets() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if m.app.AWSClient == nil {
		return errMsg(fmt.Errorf("AWS client not initialized"))
	}

	buckets, err := m.app.AWSClient.S3().ListBuckets(ctx)
	if err != nil {
		return errMsg(err)
	}
	return bucketsLoadedMsg(buckets)
}

func (m Model) fetchObjects(bucket, prefix string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if m.app.AWSClient == nil {
			return errMsg(fmt.Errorf("AWS client not initialized"))
		}

		objects, err := m.app.AWSClient.S3().ListObjects(ctx, bucket, prefix)
		if err != nil {
			return errMsg(err)
		}
		return objectsLoadedMsg(objects)
	}
}
