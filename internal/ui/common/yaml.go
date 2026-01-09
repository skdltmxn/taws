package common

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type YAMLView struct {
	viewport   viewport.Model
	rawValue   any
	title      string
	width      int
	height     int
	fullWidth  int
	fullHeight int
	ready      bool
	fullscreen bool
}

func NewYAMLView() YAMLView {
	return YAMLView{}
}

func (y YAMLView) IsFullscreen() bool {
	return y.fullscreen
}

func (y YAMLView) SetContent(title string, v any) YAMLView {
	y.title = title
	y.rawValue = v
	if y.ready {
		w := y.width
		if y.fullscreen {
			w = y.fullWidth
		}
		y.viewport.SetContent(ToYAMLWrapped(v, w))
		y.viewport.GotoTop()
	}
	return y
}

func (y YAMLView) SetSize(width, height int) YAMLView {
	y.width = width
	y.height = height
	if !y.fullscreen {
		y = y.updateViewport()
	}
	return y
}

func (y YAMLView) SetFullscreenSize(width, height int) YAMLView {
	y.fullWidth = width
	y.fullHeight = height
	if y.fullscreen {
		y = y.updateViewport()
	}
	return y
}

func (y YAMLView) updateViewport() YAMLView {
	var viewportHeight, viewportWidth int
	if y.fullscreen {
		viewportHeight = y.fullHeight
		viewportWidth = y.fullWidth
	} else {
		headerHeight := 2
		footerHeight := 1
		viewportHeight = y.height - headerHeight - footerHeight
		viewportWidth = y.width
	}

	if !y.ready {
		y.viewport = viewport.New(viewportWidth, viewportHeight)
		y.viewport.SetContent(ToYAMLWrapped(y.rawValue, viewportWidth))
		y.ready = true
	} else {
		y.viewport.Width = viewportWidth
		y.viewport.Height = viewportHeight
		y.viewport.SetContent(ToYAMLWrapped(y.rawValue, viewportWidth))
	}
	return y
}

// Update handles key events. Returns (updated view, cmd, shouldExit).
// shouldExit is true when user pressed esc/backspace/q to exit YAML view.
func (y YAMLView) Update(msg tea.Msg) (YAMLView, tea.Cmd, bool) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace", "q":
			if y.fullscreen {
				y.fullscreen = false
				y = y.updateViewport()
				return y, nil, false
			}
			return y, nil, true
		case "f":
			y.fullscreen = !y.fullscreen
			y = y.updateViewport()
		case "j", "down":
			y.viewport.ScrollDown(1)
		case "k", "up":
			y.viewport.ScrollUp(1)
		case "g":
			y.viewport.GotoTop()
		case "G":
			y.viewport.GotoBottom()
		case "ctrl+d":
			y.viewport.HalfPageDown()
		case "ctrl+u":
			y.viewport.HalfPageUp()
		default:
			y.viewport, cmd = y.viewport.Update(msg)
		}
	default:
		y.viewport, cmd = y.viewport.Update(msg)
	}
	return y, cmd, false
}

func (y YAMLView) View() string {
	if !y.ready {
		return "Initializing..."
	}

	if y.fullscreen {
		return y.viewport.View()
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true).
		Padding(0, 1)

	footerStyle := lipgloss.NewStyle().
		Foreground(InfoColor)

	scrollInfo := fmt.Sprintf("%d/%d", y.viewport.YOffset+1, max(1, y.viewport.TotalLineCount()-y.viewport.Height+1))
	tips := fmt.Sprintf("j/k: scroll | g/G: top/bottom | ctrl+d/u: half page | f: fullscreen | esc: back | %s", scrollInfo)

	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(y.title),
		y.viewport.View(),
		footerStyle.Render(tips),
	)
}

func ToYAML(v any) string {
	var sb strings.Builder
	writeYAML(&sb, reflect.ValueOf(v), 0, 0)
	return sb.String()
}

func ToYAMLWrapped(v any, width int) string {
	var sb strings.Builder
	writeYAML(&sb, reflect.ValueOf(v), 0, width)
	return sb.String()
}

func writeYAML(sb *strings.Builder, v reflect.Value, indent, width int) {
	if !v.IsValid() {
		sb.WriteString("null")
		return
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			sb.WriteString("null")
			return
		}
		writeYAML(sb, v.Elem(), indent, width)

	case reflect.Struct:
		if t, ok := v.Interface().(time.Time); ok {
			if t.IsZero() {
				sb.WriteString("null")
			} else {
				sb.WriteString(t.Format(time.RFC3339))
			}
			return
		}

		t := v.Type()
		first := true
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}

			fieldValue := v.Field(i)
			if isZeroValue(fieldValue) {
				continue
			}

			if !first {
				sb.WriteString("\n")
			}
			first = false

			writeIndent(sb, indent)
			sb.WriteString(toYAMLKey(field.Name))
			sb.WriteString(":")

			if isScalar(fieldValue) {
				sb.WriteString(" ")
				writeYAML(sb, fieldValue, indent, width)
			} else {
				sb.WriteString("\n")
				writeYAML(sb, fieldValue, indent+2, width)
			}
		}

	case reflect.Map:
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool {
			return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface())
		})

		first := true
		for _, key := range keys {
			val := v.MapIndex(key)
			if !first {
				sb.WriteString("\n")
			}
			first = false

			writeIndent(sb, indent)
			fmt.Fprint(sb, key.Interface())
			sb.WriteString(":")

			if isScalar(val) {
				sb.WriteString(" ")
				writeYAML(sb, val, indent, width)
			} else {
				sb.WriteString("\n")
				writeYAML(sb, val, indent+2, width)
			}
		}

	case reflect.Slice, reflect.Array:
		if v.Len() == 0 {
			sb.WriteString("[]")
			return
		}

		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				sb.WriteString("\n")
			}
			writeIndent(sb, indent)
			sb.WriteString("- ")
			elem := v.Index(i)
			if isScalar(elem) {
				writeYAML(sb, elem, indent, width)
			} else {
				writeYAML(sb, elem, indent+2, width)
			}
		}

	case reflect.String:
		s := v.String()
		if s == "" {
			sb.WriteString(`""`)
		} else if needsQuoting(s) {
			quoted := fmt.Sprintf("%q", s)
			writeWrappedString(sb, quoted, indent, width)
		} else {
			writeWrappedString(sb, s, indent, width)
		}

	case reflect.Bool:
		if v.Bool() {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fmt.Fprintf(sb, "%d", v.Int())

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		fmt.Fprintf(sb, "%d", v.Uint())

	case reflect.Float32, reflect.Float64:
		fmt.Fprintf(sb, "%g", v.Float())

	default:
		fmt.Fprintf(sb, "%v", v.Interface())
	}
}

func writeWrappedString(sb *strings.Builder, s string, indent, width int) {
	if width <= 0 {
		sb.WriteString(s)
		return
	}

	runes := []rune(s)
	total := len(runes)

	currentLineStart := 0
	content := sb.String()
	for i := len(content) - 1; i >= 0; i-- {
		if content[i] == '\n' {
			currentLineStart = len(content) - 1 - i
			break
		}
	}
	if currentLineStart == 0 {
		currentLineStart = len([]rune(content))
	}

	firstLineWidth := max(width-currentLineStart, 10)
	contLineWidth := max(width-indent-2, 20)

	if total <= firstLineWidth {
		sb.WriteString(s)
		return
	}

	pos := 0
	firstLine := true

	for pos < total {
		var lineWidth int
		if firstLine {
			lineWidth = firstLineWidth
		} else {
			sb.WriteString("\n")
			writeIndent(sb, indent+2)
			lineWidth = contLineWidth
		}
		firstLine = false

		remaining := total - pos
		if remaining <= lineWidth {
			sb.WriteString(string(runes[pos:]))
			break
		}

		end := min(pos+lineWidth, total)

		breakAt := -1
		skipCount := 0
		searchStart := max(pos+lineWidth/2, pos)

		for i := end - 1; i >= searchStart; i-- {
			r := runes[i]
			if r == ' ' {
				breakAt = i
				skipCount = 1
				break
			}
			if r == '-' || r == '/' || r == ',' {
				breakAt = i + 1
				skipCount = 0
				break
			}
		}

		if breakAt == -1 || breakAt <= pos {
			sb.WriteString(string(runes[pos:end]))
			pos = end
		} else {
			sb.WriteString(string(runes[pos:breakAt]))
			pos = breakAt + skipCount
		}
	}
}

func writeIndent(sb *strings.Builder, indent int) {
	for range indent {
		sb.WriteByte(' ')
	}
}

var yamlKeyAcronyms = []string{"ID", "ARN", "URI", "URL", "VPC", "AWS", "EC2", "EKS", "ECR", "IAM", "S3", "AZ", "CIDR", "DNS", "IP", "TTL", "AMI", "EBS", "API", "CPU", "GB", "MB", "KB"}

func toYAMLKey(s string) string {
	for _, acr := range yamlKeyAcronyms {
		s = strings.ReplaceAll(s, acr, "_"+strings.ToLower(acr)+"_")
	}

	var result strings.Builder
	prevUnderscore := false
	for i, r := range s {
		if r == '_' {
			if i > 0 && !prevUnderscore && result.Len() > 0 {
				result.WriteByte('_')
			}
			prevUnderscore = true
			continue
		}
		if i > 0 && r >= 'A' && r <= 'Z' && !prevUnderscore {
			result.WriteByte('_')
		}
		if r >= 'A' && r <= 'Z' {
			result.WriteRune(r - 'A' + 'a')
		} else {
			result.WriteRune(r)
		}
		prevUnderscore = false
	}

	// Clean up leading/trailing underscores and double underscores
	out := result.String()
	for strings.Contains(out, "__") {
		out = strings.ReplaceAll(out, "__", "_")
	}
	out = strings.Trim(out, "_")
	return out
}

func isScalar(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return true
		}
		return isScalar(v.Elem())
	case reflect.Struct:
		_, ok := v.Interface().(time.Time)
		return ok
	case reflect.Slice, reflect.Array, reflect.Map:
		return false
	default:
		return true
	}
}

func isZeroValue(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	case reflect.Slice, reflect.Map:
		return v.IsNil() || v.Len() == 0
	case reflect.String:
		return v.String() == ""
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Struct:
		if t, ok := v.Interface().(time.Time); ok {
			return t.IsZero()
		}
		return false
	default:
		return false
	}
}

func needsQuoting(s string) bool {
	if s == "" {
		return true
	}
	if strings.ContainsAny(s, ":\n\r\t#[]{}|>&*!?'\"\\") {
		return true
	}
	if s[0] == ' ' || s[len(s)-1] == ' ' {
		return true
	}
	lower := strings.ToLower(s)
	if lower == "true" || lower == "false" || lower == "null" || lower == "yes" || lower == "no" {
		return true
	}
	return false
}
