package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"gitlab-tui/internal/config"
	"gitlab-tui/internal/gitlab"
)

func TestYankOptionsPerTab(t *testing.T) {
	tests := []struct {
		name    string
		tab     tabID
		wantLen int
		hasURLs bool
	}{
		{"MR detail", tabMRs, 5, true},
		{"Issue detail", tabIssues, 4, true},
		{"Pipeline detail", tabPipelines, 3, true},
		{"Branch detail", tabBranches, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{tab: tt.tab}
			opts := m.currentYankOptions()
			if len(opts) != tt.wantLen {
				t.Fatalf("expected %d options, got %d", tt.wantLen, len(opts))
			}
			found := false
			for _, o := range opts {
				if o.Key == "u" {
					found = true
				}
			}
			if found != tt.hasURLs {
				t.Fatalf("expected URLs option presence %v, got %v", tt.hasURLs, found)
			}
		})
	}
}

func TestYankTextMR(t *testing.T) {
	m := Model{
		tab: tabMRs,
		mrDetail: &gitlab.MRInfo{
			IID:          42,
			Title:        "Add yank support",
			Description:  "Copies things",
			SourceBranch: "feature/yank",
		},
	}
	tests := []struct {
		option string
		want   string
	}{
		{"i", "!42"},
		{"t", "Add yank support"},
		{"d", "Copies things"},
		{"b", "feature/yank"},
		{"z", ""},
	}
	for _, tt := range tests {
		if got := m.yankText(tt.option); got != tt.want {
			t.Errorf("yankText(%q) = %q, want %q", tt.option, got, tt.want)
		}
	}
}

func TestYankTextIssueAndPipeline(t *testing.T) {
	issue := Model{
		tab:         tabIssues,
		issueDetail: &gitlab.IssueInfo{IID: 7, Title: "Broken link", Description: "It broke"},
	}
	if got := issue.yankText("i"); got != "#7" {
		t.Errorf("issue ID = %q, want %q", got, "#7")
	}
	if got := issue.yankText("t"); got != "Broken link" {
		t.Errorf("issue title = %q", got)
	}

	pipe := Model{
		tab:            tabPipelines,
		pipelineDetail: &gitlab.PipelineInfo{ID: 991, Ref: "main"},
	}
	if got := pipe.yankText("i"); got != "#991" {
		t.Errorf("pipeline ID = %q, want %q", got, "#991")
	}
	if got := pipe.yankText("t"); got != "main" {
		t.Errorf("pipeline ref = %q", got)
	}
	if got := pipe.yankText("d"); got != "" {
		t.Errorf("pipeline description should be empty, got %q", got)
	}
}

func TestOpenYankPopupWithYKey(t *testing.T) {
	m := Model{
		state:    stateDetail,
		tab:      tabMRs,
		width:    100,
		height:   30,
		mrDetail: &gitlab.MRInfo{IID: 1, Title: "T"},
	}
	newM, _ := m.handleDetailKey("y")
	got := newM.(Model)
	if !got.yankOpen {
		t.Fatal("expected yank popup to open after 'y'")
	}

	// Any other key closes it
	newM2, _ := got.handleDetailKey("x")
	got2 := newM2.(Model)
	if got2.yankOpen {
		t.Fatal("expected yank popup to close on unrelated key")
	}
}

func TestHandleYankPopupKeyCopies(t *testing.T) {
	var captured string
	var calls int
	orig := clipboardWriteAll
	clipboardWriteAll = func(s string) error { captured = s; calls++; return nil }
	defer func() { clipboardWriteAll = orig }()

	m := Model{
		tab:      tabMRs,
		mrDetail: &gitlab.MRInfo{IID: 5, Title: "My MR"},
		yankOpen: true,
	}
	newM, _ := m.handleDetailKey("i")
	got := newM.(Model)
	if got.yankOpen {
		t.Fatal("popup should close after copying")
	}
	if calls != 1 || captured != "!5" {
		t.Fatalf("expected clipboard to receive %q once, got %q (%d calls)", "!5", captured, calls)
	}
	if !strings.Contains(got.statusMsg, "Copied") {
		t.Fatalf("expected status message about copy, got %q", got.statusMsg)
	}
}

func TestCopyFailureShowsErrorStatus(t *testing.T) {
	orig := clipboardWriteAll
	clipboardWriteAll = func(string) error { return errTestClipboard }
	defer func() { clipboardWriteAll = orig }()

	m := Model{tab: tabMRs, mrDetail: &gitlab.MRInfo{IID: 5}, yankOpen: true}
	newM, _ := m.handleDetailKey("i")
	got := newM.(Model)
	if !strings.Contains(got.statusMsg, "failed") {
		t.Fatalf("expected failure status message, got %q", got.statusMsg)
	}
}

type testErr struct{}

func (testErr) Error() string { return "no clipboard tool" }

var errTestClipboard = testErr{}

func TestYankURLSelectFlow(t *testing.T) {
	var captured []string
	orig := clipboardWriteAll
	clipboardWriteAll = func(s string) error { captured = append(captured, s); return nil }
	defer func() { clipboardWriteAll = orig }()

	cfg := testConfigForLinks()
	m := Model{
		tab: tabMRs,
		mrDetail: &gitlab.MRInfo{
			IID:         3,
			WebURL:      "https://gitlab.com/group/proj/-/merge_requests/3",
			Description: "See https://example.com/a and https://example.com/b",
		},
		cfg: cfg,
	}

	newM, _ := m.performYank("u")
	got := newM.(Model)
	if !got.yankURLSelect {
		t.Fatal("expected URL select mode with multiple URLs")
	}
	if len(got.yankItems) < 2 {
		t.Fatalf("expected at least 2 items in select list, got %d", len(got.yankItems))
	}

	// navigate down to the last item, then copy it
	newM2, _ := got.handleYankURLSelectKey("j")
	newM3, _ := newM2.(Model).handleYankURLSelectKey("j")
	newM4, _ := newM3.(Model).handleYankURLSelectKey("enter")
	got4 := newM4.(Model)
	if len(captured) != 1 {
		t.Fatalf("expected one copy after Enter, got %d", len(captured))
	}
	if captured[0] != "https://example.com/b" {
		t.Fatalf("expected second URL copied, got %q", captured[0])
	}
	if got4.yankURLSelect {
		t.Fatal("URL select should close after Enter")
	}
}

func TestYankSingleURLCopiesDirectly(t *testing.T) {
	var captured []string
	orig := clipboardWriteAll
	clipboardWriteAll = func(s string) error { captured = append(captured, s); return nil }
	defer func() { clipboardWriteAll = orig }()

	m := Model{
		tab:            tabPipelines,
		pipelineDetail: &gitlab.PipelineInfo{ID: 9, WebURL: "https://gitlab.com/group/proj/-/pipelines/9"},
	}
	newM, _ := m.performYank("u")
	got := newM.(Model)
	if got.yankURLSelect {
		t.Fatal("single URL should not open the select list")
	}
	if len(captured) != 1 || captured[0] != "https://gitlab.com/group/proj/-/pipelines/9" {
		t.Fatalf("expected pipeline URL copied, got %v", captured)
	}
}

func TestClearStatusMsg(t *testing.T) {
	m := Model{statusMsg: "Copied", statusMsgID: 3}

	stale := clearStatusMsg{id: 2}
	newM, _ := m.Update(stale)
	if got := newM.(Model).statusMsg; got != "Copied" {
		t.Fatalf("stale clear should not erase message, got %q", got)
	}

	current := clearStatusMsg{id: 3}
	newM2, _ := m.Update(current)
	if got := newM2.(Model).statusMsg; got != "" {
		t.Fatalf("current clear should erase message, got %q", got)
	}
}

func TestEscClosesYankBeforeLeavingDetail(t *testing.T) {
	m := Model{
		state:    stateDetail,
		tab:      tabMRs,
		mrs:      []*gitlab.MRInfo{{IID: 1}},
		mrDetail: &gitlab.MRInfo{IID: 1},
		yankOpen: true,
	}
	newM, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	got := newM.(Model)
	if got.yankOpen {
		t.Fatal("esc should close yank popup")
	}
	if got.state != stateDetail {
		t.Fatalf("esc should stay in detail view, got state %v", got.state)
	}
}

// testConfigForLinks returns a minimal config for collectLinksForDetail.
func testConfigForLinks() *config.Config { return nil }
