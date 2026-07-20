package tui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gitlab-tui/internal/config"
	"gitlab-tui/internal/gitlab"
)

func TestExtractYouTrackLinks(t *testing.T) {
	cfg := &config.Config{
		YouTrackServers: []config.YouTrackServer{
			{
				Name:     "Mediatel YouTrack",
				URL:      "https://youtrack.mediatel.co.uk/",
				Projects: []string{"MTEL", "BARB"},
			},
		},
	}

	tests := []struct {
		name     string
		text     string
		expected []youTrackLink
	}{
		{
			name: "single match",
			text: "Closes MTEL-22122 in description",
			expected: []youTrackLink{
				{Key: "MTEL-22122", URL: "https://youtrack.mediatel.co.uk/issue/MTEL-22122"},
			},
		},
		{
			name: "multiple matches and duplicates",
			text: "Fixes barb-123 and also BARB-123. Note that MTEL-99 is related.",
			expected: []youTrackLink{
				{Key: "BARB-123", URL: "https://youtrack.mediatel.co.uk/issue/BARB-123"},
				{Key: "MTEL-99", URL: "https://youtrack.mediatel.co.uk/issue/MTEL-99"},
			},
		},
		{
			name:     "no config",
			text:     "MTEL-123 is here but cfg is nil",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var testCfg *config.Config
			if tt.name != "no config" {
				testCfg = cfg
			}
			got := extractYouTrackLinks(tt.text, testCfg)
			if len(got) == 0 && len(tt.expected) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("extractYouTrackLinks() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestInitTheme(t *testing.T) {
	// Test Catppuccin theme (default)
	InitTheme("catppuccin")
	if colorBg != lipgloss.Color("#0d1117") {
		t.Errorf("expected Catppuccin bg to be #0d1117, got %v", colorBg)
	}
	if colorAccent != lipgloss.Color("#7c3aed") {
		t.Errorf("expected Catppuccin accent to be #7c3aed, got %v", colorAccent)
	}

	// Test Teams theme
	InitTheme("teams")
	if colorBg != lipgloss.Color("#202020") {
		t.Errorf("expected Teams bg to be #202020, got %v", colorBg)
	}
	if colorAccent != lipgloss.Color("#00d75f") {
		t.Errorf("expected Teams accent to be #00d75f, got %v", colorAccent)
	}

	// Reset to default
	InitTheme("catppuccin")
}

func TestCompareBranchKey(t *testing.T) {
	// Initialize theme colors to prevent nil styles/panics in View
	InitTheme("catppuccin")

	m := Model{
		cfg: &config.Config{
			Servers: []config.Server{
				{Name: "GitLab", URL: "https://gitlab.com"},
			},
		},
		tab:          tabBranches,
		state:        stateMain,
		project:      &gitlab.ProjectInfo{ID: 1, DefaultBranch: "main"},
		branches:     []string{"main", "feature"},
		branchCursor: 0,
		width:        80,
		height:       24,
	}

	// Update with key "C"
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}}
	res, _ := m.Update(msg)
	
	newModel := res.(Model)
	if newModel.state != stateCompareBranchSelect {
		t.Errorf("expected state to be stateCompareBranchSelect, got %v", newModel.state)
	}

	// Call View to check if it panics or crashes
	_ = newModel.View()
}

func TestReopenMRKey(t *testing.T) {
	InitTheme("catppuccin")

	m := Model{
		cfg: &config.Config{
			Servers: []config.Server{
				{Name: "GitLab", URL: "https://gitlab.com"},
			},
		},
		tab:     tabMRs,
		state:   stateDetail,
		project: &gitlab.ProjectInfo{ID: 1},
		mrDetail: &gitlab.MRInfo{
			IID:   42,
			Title: "Test MR",
			State: "closed",
		},
		width:  80,
		height: 24,
	}

	footer := m.viewDetailFooter()
	if !strings.Contains(footer, "O") || !strings.Contains(footer, "reopen") {
		t.Errorf("expected footer to contain 'O reopen', got: %s", footer)
	}

	// Update with key "O"
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'O'}}
	res, _ := m.Update(msg)

	newModel := res.(Model)
	if newModel.state != stateConfirm {
		t.Errorf("expected state to be stateConfirm, got %v", newModel.state)
	}
	if newModel.confirm == nil || newModel.confirm.label != "Reopen MR !42?" {
		t.Errorf("expected confirmation prompt for MR 42, got %v", newModel.confirm)
	}

	// Call View to check if it panics or crashes
	_ = newModel.View()
}

func TestReopenMRKeyFromList(t *testing.T) {
	InitTheme("catppuccin")

	m := Model{
		cfg: &config.Config{
			Servers: []config.Server{
				{Name: "GitLab", URL: "https://gitlab.com"},
			},
		},
		tab:     tabMRs,
		state:   stateMain,
		project: &gitlab.ProjectInfo{ID: 1},
		mrs: []*gitlab.MRInfo{
			{
				IID:   100,
				Title: "Closed List MR",
				State: "closed",
			},
		},
		mrCursor: 0,
		width:    80,
		height:   24,
	}

	footer := m.viewFooter()
	if !strings.Contains(footer, "O") || !strings.Contains(footer, "reopen") {
		t.Errorf("expected list footer to contain 'O reopen', got: %s", footer)
	}

	// Update with key "O"
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'O'}}
	res, _ := m.Update(msg)

	newModel := res.(Model)
	if newModel.state != stateConfirm {
		t.Errorf("expected state to be stateConfirm, got %v", newModel.state)
	}
	if newModel.confirm == nil || newModel.confirm.label != "Reopen MR !100?" {
		t.Errorf("expected confirmation prompt for MR 100, got %v", newModel.confirm)
	}

	// Call View to check if it panics or crashes
	_ = newModel.View()
}

func TestIssueStateToggleKey(t *testing.T) {
	InitTheme("catppuccin")

	m := Model{
		cfg: &config.Config{
			Servers: []config.Server{
				{Name: "GitLab", URL: "https://gitlab.com"},
			},
		},
		tab:        tabIssues,
		state:      stateMain,
		project:    &gitlab.ProjectInfo{ID: 1},
		issueState: gitlab.IssueStateOpened,
		width:      80,
		height:     24,
	}

	footer := m.viewFooter()
	if !strings.Contains(footer, "s") || !strings.Contains(footer, "state:opened") {
		t.Errorf("expected list footer to contain 's state:opened', got: %s", footer)
	}

	// Press 's': opened -> closed
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
	res, _ := m.Update(msg)
	m = res.(Model)
	if m.issueState != gitlab.IssueStateClosed {
		t.Errorf("expected issueState to be closed, got %v", m.issueState)
	}
	footer = m.viewFooter()
	if !strings.Contains(footer, "state:closed") {
		t.Errorf("expected list footer to contain 'state:closed', got: %s", footer)
	}

	// Press 's': closed -> all
	res, _ = m.Update(msg)
	m = res.(Model)
	if m.issueState != gitlab.IssueStateAll {
		t.Errorf("expected issueState to be all, got %v", m.issueState)
	}
	footer = m.viewFooter()
	if !strings.Contains(footer, "state:all") {
		t.Errorf("expected list footer to contain 'state:all', got: %s", footer)
	}

	// Press 's': all -> opened
	res, _ = m.Update(msg)
	m = res.(Model)
	if m.issueState != gitlab.IssueStateOpened {
		t.Errorf("expected issueState to be opened, got %v", m.issueState)
	}
}

func TestCloseAndReopenIssueKey(t *testing.T) {
	InitTheme("catppuccin")

	m := Model{
		cfg: &config.Config{
			Servers: []config.Server{
				{Name: "GitLab", URL: "https://gitlab.com"},
			},
		},
		tab:     tabIssues,
		state:   stateDetail,
		project: &gitlab.ProjectInfo{ID: 1},
		issueDetail: &gitlab.IssueInfo{
			IID:   55,
			Title: "Test Issue",
			State: "opened",
		},
		width:  80,
		height: 24,
	}

	footer := m.viewDetailFooter()
	if !strings.Contains(footer, "x") || !strings.Contains(footer, "close") {
		t.Errorf("expected detail footer to contain 'x close', got: %s", footer)
	}

	// Update with key "x" to close opened issue
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	res, _ := m.Update(msg)

	newModel := res.(Model)
	if newModel.state != stateConfirm {
		t.Errorf("expected state to be stateConfirm, got %v", newModel.state)
	}
	if newModel.confirm == nil || newModel.confirm.label != "Close Issue #55?" {
		t.Errorf("expected confirmation prompt for Close Issue #55, got %v", newModel.confirm)
	}

	// Change state to closed and test reopening with 'O'
	m.issueDetail.State = "closed"
	footer = m.viewDetailFooter()
	if !strings.Contains(footer, "O") || !strings.Contains(footer, "reopen") {
		t.Errorf("expected detail footer to contain 'O reopen', got: %s", footer)
	}

	msgO := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'O'}}
	resO, _ := m.Update(msgO)

	newModelO := resO.(Model)
	if newModelO.state != stateConfirm {
		t.Errorf("expected state to be stateConfirm, got %v", newModelO.state)
	}
	if newModelO.confirm == nil || newModelO.confirm.label != "Reopen Issue #55?" {
		t.Errorf("expected confirmation prompt for Reopen Issue #55, got %v", newModelO.confirm)
	}
}

func TestCloseAndReopenIssueKeyFromList(t *testing.T) {
	InitTheme("catppuccin")

	m := Model{
		cfg: &config.Config{
			Servers: []config.Server{
				{Name: "GitLab", URL: "https://gitlab.com"},
			},
		},
		tab:     tabIssues,
		state:   stateMain,
		project: &gitlab.ProjectInfo{ID: 1},
		issues: []*gitlab.IssueInfo{
			{
				IID:   88,
				Title: "Opened List Issue",
				State: "opened",
			},
		},
		issueCursor: 0,
		width:       80,
		height:      24,
	}

	footer := m.viewFooter()
	if !strings.Contains(footer, "x") || !strings.Contains(footer, "close") {
		t.Errorf("expected list footer to contain 'x close', got: %s", footer)
	}

	// Update with key "x"
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	res, _ := m.Update(msg)

	newModel := res.(Model)
	if newModel.state != stateConfirm {
		t.Errorf("expected state to be stateConfirm, got %v", newModel.state)
	}
	if newModel.confirm == nil || newModel.confirm.label != "Close Issue #88?" {
		t.Errorf("expected confirmation prompt for Close Issue #88, got %v", newModel.confirm)
	}

	// Change state to closed and test 'O'
	m.issues[0].State = "closed"
	footer = m.viewFooter()
	if !strings.Contains(footer, "O") || !strings.Contains(footer, "reopen") {
		t.Errorf("expected list footer to contain 'O reopen', got: %s", footer)
	}

	msgO := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'O'}}
	resO, _ := m.Update(msgO)

	newModelO := resO.(Model)
	if newModelO.state != stateConfirm {
		t.Errorf("expected state to be stateConfirm, got %v", newModelO.state)
	}
	if newModelO.confirm == nil || newModelO.confirm.label != "Reopen Issue #88?" {
		t.Errorf("expected confirmation prompt for Reopen Issue #88, got %v", newModelO.confirm)
	}
}
