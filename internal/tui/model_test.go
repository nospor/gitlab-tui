package tui

import (
	"reflect"
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
