package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// clipboardWriteAll is indirected to allow stubbing in tests.
var clipboardWriteAll = clipboard.WriteAll

const statusMsgDuration = 4 * time.Second

// ─── Yank options ─────────────────────────────────────────────────────────────

type yankOption struct {
	Key   string
	Label string
}

// currentYankOptions returns the yank menu entries for the active detail view.
func (m Model) currentYankOptions() []yankOption {
	switch m.tab {
	case tabMRs:
		return []yankOption{
			{"i", "MR ID"},
			{"t", "Title"},
			{"d", "Description"},
			{"b", "Source Branch"},
			{"u", "URLs"},
		}
	case tabIssues:
		return []yankOption{
			{"i", "Issue ID"},
			{"t", "Title"},
			{"d", "Description"},
			{"u", "URLs"},
		}
	case tabPipelines:
		return []yankOption{
			{"i", "Pipeline ID"},
			{"t", "Ref"},
			{"u", "URL"},
		}
	default:
		return nil
	}
}

// yankText returns the text to copy for the given yank option key.
func (m Model) yankText(option string) string {
	switch m.tab {
	case tabMRs:
		d := m.mrDetail
		if d == nil {
			return ""
		}
		switch option {
		case "i":
			return fmt.Sprintf("!%d", d.IID)
		case "t":
			return d.Title
		case "d":
			return d.Description
		case "b":
			return d.SourceBranch
		}
	case tabIssues:
		d := m.issueDetail
		if d == nil {
			return ""
		}
		switch option {
		case "i":
			return fmt.Sprintf("#%d", d.IID)
		case "t":
			return d.Title
		case "d":
			return d.Description
		}
	case tabPipelines:
		d := m.pipelineDetail
		if d == nil {
			return ""
		}
		switch option {
		case "i":
			return fmt.Sprintf("#%d", d.ID)
		case "t":
			return d.Ref
		}
	}
	return ""
}

// ─── Yank key handling ────────────────────────────────────────────────────────

func (m Model) openYank() (tea.Model, tea.Cmd) {
	m.yankOpen = true
	return m, nil
}

func (m *Model) closeYank() {
	m.yankOpen = false
	m.yankURLSelect = false
	m.yankItems = nil
	m.yankCursor = 0
}

// handleYankPopupKey handles keys while the yank options popup is open.
// Any unrecognised key dismisses the popup (matching yt-tui behaviour).
func (m Model) handleYankPopupKey(key string) (tea.Model, tea.Cmd) {
	m.yankOpen = false
	for _, opt := range m.currentYankOptions() {
		if key == opt.Key {
			return m.performYank(opt.Key)
		}
	}
	return m, nil
}

// handleYankURLSelectKey handles keys while the URL pick list is open.
func (m Model) handleYankURLSelectKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "j", "down":
		if len(m.yankItems) > 0 {
			m.yankCursor++
			if m.yankCursor >= len(m.yankItems) {
				m.yankCursor = 0
			}
		}
	case "k", "up":
		if len(m.yankItems) > 0 {
			m.yankCursor--
			if m.yankCursor < 0 {
				m.yankCursor = len(m.yankItems) - 1
			}
		}
	case "enter":
		if m.yankCursor >= 0 && m.yankCursor < len(m.yankItems) {
			url := m.yankItems[m.yankCursor].URL
			m.closeYank()
			return m.copyYank(url, "URL")
		}
		m.closeYank()
	default:
		m.closeYank()
	}
	return m, nil
}

// performYank executes the selected yank option.
func (m Model) performYank(option string) (tea.Model, tea.Cmd) {
	if option == "u" {
		return m.yankURLs()
	}
	text := m.yankText(option)
	if text == "" {
		return m.setStatus("Nothing to copy")
	}
	label := ""
	for _, opt := range m.currentYankOptions() {
		if opt.Key == option {
			label = strings.ToLower(opt.Label)
			break
		}
	}
	return m.copyYank(text, label)
}

// yankURLs collects all links of the current detail view. A single URL is
// copied right away; multiple URLs open a pick list; none shows a hint.
func (m Model) yankURLs() (tea.Model, tea.Cmd) {
	items := m.collectLinksForDetail()
	switch len(items) {
	case 0:
		return m.setStatus("No URLs found")
	case 1:
		return m.copyYank(items[0].URL, "URL")
	default:
		m.yankItems = items
		m.yankCursor = 0
		m.yankURLSelect = true
		return m, nil
	}
}

// ─── Clipboard & status feedback ──────────────────────────────────────────────

// setStatus shows a transient status message without copying anything.
func (m Model) setStatus(msg string) (tea.Model, tea.Cmd) {
	m.statusMsg = msg
	m.statusMsgID++
	id := m.statusMsgID
	return m, tea.Tick(statusMsgDuration, func(time.Time) tea.Msg {
		return clearStatusMsg{id: id}
	})
}

// copyYank copies text to the clipboard and reports via the status message.
func (m Model) copyYank(text, label string) (tea.Model, tea.Cmd) {
	if err := clipboardWriteAll(text); err != nil {
		return m.setStatus(fmt.Sprintf("Copy failed: %v", err))
	}
	if label == "" {
		return m.setStatus("Copied to clipboard")
	}
	return m.setStatus(fmt.Sprintf("Copied %s to clipboard", label))
}

// ─── Rendering ────────────────────────────────────────────────────────────────

func yankBox(rows []string) string {
	return panelStyle.Padding(0, 1).Render(strings.Join(rows, "\n"))
}

// viewYankPopup renders the small yank options popup shown top-right.
func (m Model) viewYankPopup() string {
	rows := []string{subtitleStyle.Render("Yank")}
	for _, opt := range m.currentYankOptions() {
		key := lipgloss.NewStyle().Foreground(colorAccentAlt).Bold(true).Render("[" + opt.Key + "]")
		rows = append(rows, fmt.Sprintf("%s %s", key, dimStyle.Render(opt.Label)))
	}
	return yankBox(rows)
}

// viewYankURLSelect renders the URL pick list shown top-right when several
// URLs are available for yanking.
func (m Model) viewYankURLSelect() string {
	rows := []string{subtitleStyle.Render("Yank URL")}
	maxW := m.width - 12
	if maxW < 24 {
		maxW = 24
	}
	for idx, item := range m.yankItems {
		display := truncate(item.URL, maxW)
		if idx == m.yankCursor {
			rows = append(rows, selectedStyle.Render("▶ "+display))
		} else {
			rows = append(rows, normalItemStyle.Render("  "+display))
		}
	}
	rows = append(rows, "", dimStyle.Render("↑↓ select · Enter copy · Esc cancel"))
	return yankBox(rows)
}

// applyYankOverlays overlays the yank popups onto the rendered detail view,
// anchored at the top-right corner.
func (m Model) applyYankOverlays(view string) string {
	var popup string
	switch {
	case m.yankOpen:
		popup = m.viewYankPopup()
	case m.yankURLSelect:
		popup = m.viewYankURLSelect()
	default:
		return view
	}
	x := (m.width - 4) - lipgloss.Width(popup)
	if x < 0 {
		x = 0
	}
	return overlay(view, popup, m.width, lipgloss.Height(view), x, 0)
}
