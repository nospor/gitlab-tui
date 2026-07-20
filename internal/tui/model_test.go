package tui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

func TestCreateIssueKey(t *testing.T) {
	InitTheme("catppuccin")

	m := New(&config.Config{
		Servers: []config.Server{
			{Name: "GitLab", URL: "https://gitlab.com"},
		},
	}, 0, nil, &gitlab.ProjectInfo{ID: 1}, "", 0, 0, 0)
	m.tab = tabIssues
	m.state = stateMain
	m.width = 100
	m.height = 30

	footer := m.viewFooter()
	if !strings.Contains(footer, "c") || !strings.Contains(footer, "create issue") {
		t.Errorf("expected list footer to contain 'c create issue', got: %s", footer)
	}

	// Press 'c' to trigger create issue dialog
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	res, _ := m.Update(msg)

	newModel := res.(Model)
	if newModel.state != stateCreateIssue {
		t.Errorf("expected state to be stateCreateIssue, got %v", newModel.state)
	}

	// Render view and check box content
	plainView := ansi.Strip(newModel.View())
	if !strings.Contains(plainView, "Create Issue") || !strings.Contains(plainView, "Title:") || !strings.Contains(plainView, "Type:") {
		t.Errorf("expected View() to contain Create Issue form elements, got: %s", plainView)
	}
}

func TestEditIssueKeyFromList(t *testing.T) {
	InitTheme("catppuccin")

	m := New(&config.Config{
		Servers: []config.Server{
			{Name: "GitLab", URL: "https://gitlab.com"},
		},
	}, 0, nil, &gitlab.ProjectInfo{ID: 1}, "", 0, 0, 0)
	m.tab = tabIssues
	m.state = stateMain
	m.issues = []*gitlab.IssueInfo{
		{
			IID:         101,
			Title:       "Existing Issue Title",
			Description: "Existing description body",
			IssueType:   "incident",
			State:       "opened",
		},
	}
	m.issueCursor = 0
	m.width = 80
	m.height = 24

	footer := m.viewFooter()
	if !strings.Contains(footer, "e") || !strings.Contains(footer, "edit issue") {
		t.Errorf("expected list footer to contain 'e edit issue', got: %s", footer)
	}

	// Press 'e' to trigger edit issue dialog
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	res, _ := m.Update(msg)

	newModel := res.(Model)
	if newModel.state != stateEditIssue {
		t.Errorf("expected state to be stateEditIssue, got %v", newModel.state)
	}
	if newModel.issueEditIID != 101 {
		t.Errorf("expected issueEditIID to be 101, got %d", newModel.issueEditIID)
	}
	if newModel.issueFormTitle.Value() != "Existing Issue Title" {
		t.Errorf("expected pre-filled title 'Existing Issue Title', got '%s'", newModel.issueFormTitle.Value())
	}
	if newModel.issueFormDescription.Value() != "Existing description body" {
		t.Errorf("expected pre-filled description 'Existing description body', got '%s'", newModel.issueFormDescription.Value())
	}
	if newModel.issueFormTypeIdx != 1 { // incident is index 1
		t.Errorf("expected issueFormTypeIdx to be 1 for incident, got %d", newModel.issueFormTypeIdx)
	}

	// Render view and check box content
	viewStr := newModel.View()
	if !strings.Contains(viewStr, "Edit Issue #101") || !strings.Contains(viewStr, "incident") {
		t.Errorf("expected View() to contain Edit Issue #101 with type incident, got: %s", viewStr)
	}
}

func TestEditIssueKeyFromDetail(t *testing.T) {
	InitTheme("catppuccin")

	m := New(&config.Config{
		Servers: []config.Server{
			{Name: "GitLab", URL: "https://gitlab.com"},
		},
	}, 0, nil, &gitlab.ProjectInfo{ID: 1}, "", 0, 0, 0)
	m.tab = tabIssues
	m.state = stateDetail
	m.commentCursor = -1 // no comment selected
	m.issueDetail = &gitlab.IssueInfo{
		IID:         202,
		Title:       "Detail Issue",
		Description: "Detail desc",
		IssueType:   "task",
		State:       "opened",
	}
	m.width = 80
	m.height = 24

	footer := m.viewDetailFooter()
	if !strings.Contains(footer, "e") || !strings.Contains(footer, "edit issue") {
		t.Errorf("expected detail footer to contain 'e edit issue', got: %s", footer)
	}

	// Press 'e' on detail view without selected comment
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	res, _ := m.Update(msg)

	newModel := res.(Model)
	if newModel.state != stateEditIssue {
		t.Errorf("expected state to be stateEditIssue, got %v", newModel.state)
	}
	if newModel.issueEditIID != 202 {
		t.Errorf("expected issueEditIID to be 202, got %d", newModel.issueEditIID)
	}
	if newModel.issueFormTypeIdx != 0 { // task is index 0 in []string{"task", "issue"}
		t.Errorf("expected issueFormTypeIdx to be 0 for task, got %d", newModel.issueFormTypeIdx)
	}
}

func TestCreateEditIssueFormKeys(t *testing.T) {
	InitTheme("catppuccin")

	m := New(&config.Config{
		Servers: []config.Server{
			{Name: "GitLab", URL: "https://gitlab.com"},
		},
	}, 0, nil, &gitlab.ProjectInfo{ID: 1}, "", 0, 0, 0)
	m.tab = tabIssues
	m.state = stateMain
	m.width = 80
	m.height = 24

	// Start create issue
	m, _ = m.startCreateIssue()

	// Type 'A' into title
	msgA := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}}
	res, _ := m.Update(msgA)
	m = res.(Model)
	if m.issueFormTitle.Value() != "A" {
		t.Errorf("expected title to be 'A', got '%s'", m.issueFormTitle.Value())
	}

	// Press Tab to move to Type field
	msgTab := tea.KeyMsg{Type: tea.KeyTab}
	res, _ = m.Update(msgTab)
	m = res.(Model)
	if m.issueFormField != issueFieldType {
		t.Errorf("expected field to be issueFieldType (1), got %d", m.issueFormField)
	}

	// Cycle type with right arrow
	msgRight := tea.KeyMsg{Type: tea.KeyRight}
	res, _ = m.Update(msgRight)
	m = res.(Model)
	if m.issueFormTypeIdx != 1 {
		t.Errorf("expected type index to be 1 (incident), got %d", m.issueFormTypeIdx)
	}

	// Press Tab again to move to Description field
	res, _ = m.Update(msgTab)
	m = res.(Model)
	if m.issueFormField != issueFieldDescription {
		t.Errorf("expected field to be issueFieldDescription (2), got %d", m.issueFormField)
	}
}

func TestCreateBranchForIssueKeyFromList(t *testing.T) {
	InitTheme("catppuccin")

	m := New(&config.Config{
		Servers: []config.Server{
			{Name: "GitLab", URL: "https://gitlab.com"},
		},
	}, 0, nil, &gitlab.ProjectInfo{ID: 1, DefaultBranch: "main"}, "", 0, 0, 0)
	m.tab = tabIssues
	m.state = stateMain
	m.issues = []*gitlab.IssueInfo{
		{
			IID:   88,
			Title: "Fix Login Bug & Retry!",
			State: "opened",
		},
	}
	m.issueCursor = 0
	m.width = 80
	m.height = 24

	footer := m.viewFooter()
	if !strings.Contains(footer, "b") || !strings.Contains(footer, "create branch") {
		t.Errorf("expected list footer to contain 'b create branch', got: %s", footer)
	}

	// Press 'b' to trigger create branch for issue dialog
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}
	res, _ := m.Update(msg)

	newModel := res.(Model)
	if newModel.state != stateCreateIssueBranch {
		t.Errorf("expected state to be stateCreateIssueBranch, got %v", newModel.state)
	}
	if newModel.createIssueBranchIssue == nil || newModel.createIssueBranchIssue.IID != 88 {
		t.Errorf("expected target issue IID to be 88")
	}
	expectedBranchName := "88-fix-login-bug-retry"
	if newModel.createIssueBranchName.Value() != expectedBranchName {
		t.Errorf("expected suggested branch name '%s', got '%s'", expectedBranchName, newModel.createIssueBranchName.Value())
	}
	if newModel.createIssueBranchRef.Value() != "main" {
		t.Errorf("expected default ref 'main', got '%s'", newModel.createIssueBranchRef.Value())
	}

	// Render view and check box content
	plainView := ansi.Strip(newModel.View())
	if !strings.Contains(plainView, "Create Branch for Issue #88") || !strings.Contains(plainView, "Branch Name:") || !strings.Contains(plainView, "Source Ref") {
		t.Errorf("expected View() to contain Create Branch for Issue form elements, got: %s", plainView)
	}
}

func TestCreateBranchForIssueKeyFromDetail(t *testing.T) {
	InitTheme("catppuccin")

	m := New(&config.Config{
		Servers: []config.Server{
			{Name: "GitLab", URL: "https://gitlab.com"},
		},
	}, 0, nil, &gitlab.ProjectInfo{ID: 1, DefaultBranch: "master"}, "", 0, 0, 0)
	m.tab = tabIssues
	m.state = stateDetail
	m.commentCursor = -1 // no comment selected
	m.issueDetail = &gitlab.IssueInfo{
		IID:   42,
		Title: "Add Issue Branch Feature",
		State: "opened",
	}
	m.width = 80
	m.height = 24

	footer := m.viewDetailFooter()
	if !strings.Contains(footer, "b") || !strings.Contains(footer, "create branch") {
		t.Errorf("expected detail footer to contain 'b create branch', got: %s", footer)
	}

	// Press 'b' on detail view
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}
	res, _ := m.Update(msg)

	newModel := res.(Model)
	if newModel.state != stateCreateIssueBranch {
		t.Errorf("expected state to be stateCreateIssueBranch, got %v", newModel.state)
	}
	if newModel.createIssueBranchIssue == nil || newModel.createIssueBranchIssue.IID != 42 {
		t.Errorf("expected target issue IID to be 42")
	}
	expectedBranchName := "42-add-issue-branch-feature"
	if newModel.createIssueBranchName.Value() != expectedBranchName {
		t.Errorf("expected suggested branch name '%s', got '%s'", expectedBranchName, newModel.createIssueBranchName.Value())
	}
	if newModel.createIssueBranchRef.Value() != "master" {
		t.Errorf("expected default ref 'master', got '%s'", newModel.createIssueBranchRef.Value())
	}

	// Test Esc key cancel
	msgEsc := tea.KeyMsg{Type: tea.KeyEsc}
	resEsc, _ := newModel.Update(msgEsc)
	canceledModel := resEsc.(Model)
	if canceledModel.state != stateDetail {
		t.Errorf("expected state to revert to stateDetail after Esc, got %v", canceledModel.state)
	}
}

func TestCreateBranchForIssueFormNavigation(t *testing.T) {
	InitTheme("catppuccin")

	m := New(&config.Config{
		Servers: []config.Server{
			{Name: "GitLab", URL: "https://gitlab.com"},
		},
	}, 0, nil, &gitlab.ProjectInfo{ID: 1, DefaultBranch: "main"}, "", 0, 0, 0)
	m.tab = tabIssues
	m.state = stateMain
	m.issues = []*gitlab.IssueInfo{
		{
			IID:   1,
			Title: "Test Issue",
			State: "opened",
		},
	}
	m.issueCursor = 0

	m, _ = m.startCreateBranchForIssue()
	if m.createIssueBranchField != createIssueBranchFieldName {
		t.Errorf("expected initial field to be name (0), got %d", m.createIssueBranchField)
	}

	// Press Tab to switch to Ref field
	msgTab := tea.KeyMsg{Type: tea.KeyTab}
	res, _ := m.Update(msgTab)
	m = res.(Model)
	if m.createIssueBranchField != createIssueBranchFieldRef {
		t.Errorf("expected field to switch to ref (1), got %d", m.createIssueBranchField)
	}

	// Press Tab to switch back to Name field
	res, _ = m.Update(msgTab)
	m = res.(Model)
	if m.createIssueBranchField != createIssueBranchFieldName {
		t.Errorf("expected field to switch back to name (0), got %d", m.createIssueBranchField)
	}
}

func TestFormatSystemNoteStyled(t *testing.T) {
	input := `changed title from \*\*new tst2\*\* to \*\*new tst2{+ 222+}\*\*`
	got := formatSystemNoteStyled(input)

	// Verify that ** asterisks are stripped from output
	if strings.Contains(got, "**") {
		t.Errorf("expected asterisks ** to be stripped, got: %q", got)
	}
	// Verify that {+ +} brackets are stripped from output
	if strings.Contains(got, "{+") {
		t.Errorf("expected diff brackets {+ +} to be stripped, got: %q", got)
	}
	// Verify that text contents and diff addition are present
	if !strings.Contains(got, "new tst2") || !strings.Contains(got, "+222") {
		t.Errorf("expected text contents 'new tst2' and '+222' in output, got: %q", got)
	}
}
