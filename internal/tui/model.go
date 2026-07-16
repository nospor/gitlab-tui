package tui

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gitlab-tui/internal/config"
	"gitlab-tui/internal/gitlab"
)

// commentMode distinguishes what kind of comment we are composing.
type commentMode int

const (
	commentModeGeneral commentMode = iota
	commentModeInline
	commentModeReply
)

// ─── Tabs ─────────────────────────────────────────────────────────────────────

type tabID int

const (
	tabMRs tabID = iota
	tabPipelines
	tabIssues
	tabProjects
	tabCount
)

var tabLabels = [tabCount]string{
	"  1: Merge Requests",
	"  2: Pipelines",
	"  3: Issues",
	"  4: Projects",
}

// ─── App state ────────────────────────────────────────────────────────────────

type appState int

const (
	stateLoading appState = iota
	stateError
	stateMain
	stateDetail
	stateServerSelect
	stateConfirm
	stateComment
	stateLinkSelect
)

// ─── Messages ─────────────────────────────────────────────────────────────────

type (
	errMsg          struct{ err error }
	mrLoadedMsg     struct {
		items      []*gitlab.MRInfo
		totalPages int
	}
	mrDetailLoadedMsg struct{ item *gitlab.MRInfo }
	pipelineLoadedMsg struct {
		items      []*gitlab.PipelineInfo
		totalPages int
	}
	issueLoadedMsg struct {
		items      []*gitlab.IssueInfo
		totalPages int
	}
	projectLoadedMsg struct {
		items      []*gitlab.ProjectInfo
		totalPages int
	}
	actionDoneMsg   struct{ msg string }
	whoAmIMsg       struct{ username string }
	mrDiffsLoadedMsg struct {
		files   []*gitlab.DiffFile
		version *gitlab.MRVersion
	}
	mrDiscussionsLoadedMsg struct {
		discussions []*gitlab.MRDiscussion
	}
	pipelineJobsLoadedMsg struct {
		items []*gitlab.JobInfo
	}
	pipelineDetailLoadedMsg struct {
		item *gitlab.PipelineInfo
	}
	jobTraceLoadedMsg struct {
		job   *gitlab.JobInfo
		trace string
	}
	tickMsg struct{}
	jobPipelineIDMsg struct {
		pipelineID int
	}
)

// ─── Confirmation action ──────────────────────────────────────────────────────

type confirmAction struct {
	label   string
	perform tea.Cmd
}

// ─── Link selection ────────────────────────────────────────────────────────────

type linkItem struct {
	Label string
	URL   string
}

// ─── Root model ───────────────────────────────────────────────────────────────

// Model is the root bubbletea model.
type Model struct {
	width, height int

	cfg        *config.Config
	serverIdx  int
	client     *gitlab.Client
	project    *gitlab.ProjectInfo
	username   string
	initialMRIID      int
	initialPipelineID int
	initialJobID      int64

	state     appState
	tab       tabID
	errText   string
	loadMsg   string
	doneMsg   string

	// Spinner
	spin spinner.Model

	// MR view
	mrs         []*gitlab.MRInfo
	mrPage      int
	mrTotalPage int
	mrCursor    int
	mrState     gitlab.MRState
	mrDetail    *gitlab.MRInfo

	// MR details scroll offset and discussions
	mrDiscussions        []*gitlab.MRDiscussion
	mrDetailScrollOffset int

	// MR diff panel (shown in detail view)
	mrDiffFiles        []*gitlab.DiffFile
	mrDiffVersion      *gitlab.MRVersion
	mrDiffFileIdx      int    // which file is selected
	mrDiffLineCursor   int    // which line is highlighted within the file
	mrDiffScrollOffset int    // scroll offset for the diff viewport
	mrDiffPanelOpen    bool   // whether the diff panel is shown

	// Comment composer
	commentInput    textarea.Model
	commentMode     commentMode
	commentInlineFile *gitlab.DiffFile  // target file for inline comment
	commentInlineLine gitlab.DiffLine   // target line for inline comment
	commentReplyDiscussionID string     // target discussion ID for replies

	// Link selection
	linkItems  []linkItem
	linkCursor int

	// Pipeline view
	pipelines            []*gitlab.PipelineInfo
	pipelinePage         int
	pipelineTotalPage    int
	pipelineCursor       int
	pipelineDetail       *gitlab.PipelineInfo
	pipelineJobs         []*gitlab.JobInfo
	jobCursor            int
	jobTrace             string
	jobTraceJob          *gitlab.JobInfo
	jobTraceScrollOffset int
	jobTraceOpen         bool
	jobTraceFocus        bool

	// Issue view
	issues         []*gitlab.IssueInfo
	issuePage      int
	issueTotalPage int
	issueCursor    int
	issueDetail    *gitlab.IssueInfo

	// Project select view
	projects         []*gitlab.ProjectInfo
	projectPage      int
	projectTotalPage int
	projectCursor    int
	projectSearch    textinput.Model

	// Server select
	serverCursor int

	// Confirm dialog
	confirm     *confirmAction
	confirmYes  bool

	// Warning message from startup (e.g. detection failure reason)
	startupWarn string

	// Previous state for back navigation
	prevState appState
}

// ─── Init ─────────────────────────────────────────────────────────────────────

// New creates the root model.
func New(cfg *config.Config, serverIdx int, client *gitlab.Client, project *gitlab.ProjectInfo, startupWarn string, initialMRIID int, initialPipelineID int, initialJobID int64) Model {
	InitTheme(cfg.Theme)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorAccent)

	ti := textinput.New()
	ti.Placeholder = "Search projects..."
	ti.CharLimit = 100

	ci := textarea.New()
	ci.Placeholder = "Type your comment..."
	ci.SetWidth(58)
	ci.SetHeight(6)
	ci.CharLimit = 2000
	ci.Prompt = ""
	ci.ShowLineNumbers = false

	// Set background colors to match the panel background to prevent transparency
	ci.FocusedStyle.Base = ci.FocusedStyle.Base.Background(colorBgPanel)
	ci.FocusedStyle.Text = ci.FocusedStyle.Text.Background(colorBgPanel)
	ci.FocusedStyle.Placeholder = ci.FocusedStyle.Placeholder.Background(colorBgPanel)
	ci.FocusedStyle.CursorLine = ci.FocusedStyle.CursorLine.Background(colorBgPanel)
	ci.FocusedStyle.EndOfBuffer = ci.FocusedStyle.EndOfBuffer.Background(colorBgPanel)

	ci.BlurredStyle.Base = ci.BlurredStyle.Base.Background(colorBgPanel)
	ci.BlurredStyle.Text = ci.BlurredStyle.Text.Background(colorBgPanel)
	ci.BlurredStyle.Placeholder = ci.BlurredStyle.Placeholder.Background(colorBgPanel)
	ci.BlurredStyle.CursorLine = ci.BlurredStyle.CursorLine.Background(colorBgPanel)
	ci.BlurredStyle.EndOfBuffer = ci.BlurredStyle.EndOfBuffer.Background(colorBgPanel)

	m := Model{
		cfg:               cfg,
		serverIdx:         serverIdx,
		client:            client,
		project:           project,
		startupWarn:       startupWarn,
		initialMRIID:      initialMRIID,
		initialPipelineID: initialPipelineID,
		initialJobID:      initialJobID,
		state:             stateLoading,
		loadMsg:           "Connecting to GitLab...",
		tab:               tabMRs,
		spin:              sp,
		mrState:           gitlab.MRStateOpened,
		mrPage:            1,
		pipelinePage:      1,
		issuePage:         1,
		projectPage:       1,
		projectSearch:     ti,
		commentInput:      ci,
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spin.Tick,
		m.cmdWhoAmI(),
	)
}

// ─── Update ───────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateDiffScroll()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case errMsg:
		if m.state != stateError {
			m.prevState = m.state
		}
		m.state = stateError
		m.errText = msg.err.Error()
		return m, nil

	case whoAmIMsg:
		m.username = msg.username
		if m.project == nil {
			// No project detected — go straight to main so user can pick one
			m.state = stateMain
			m.tab = tabProjects
			m.projectSearch.Focus()
			return m, m.cmdLoadProjects()
		}
		if m.initialJobID > 0 {
			m.state = stateLoading
			m.loadMsg = fmt.Sprintf("Loading job #%d details...", m.initialJobID)
			m.tab = tabPipelines
			return m, m.cmdGetJobPipelineID(m.initialJobID)
		}
		if m.initialPipelineID > 0 {
			m.state = stateLoading
			m.loadMsg = fmt.Sprintf("Loading pipeline #%d...", m.initialPipelineID)
			m.tab = tabPipelines
			return m, tea.Batch(
				m.cmdLoadPipelineDetail(m.initialPipelineID),
				m.cmdLoadPipelineJobs(m.initialPipelineID),
			)
		}
		if m.initialMRIID > 0 {
			m.state = stateLoading
			m.loadMsg = fmt.Sprintf("Loading merge request !%d...", m.initialMRIID)
			return m, m.cmdLoadMRDetail(m.initialMRIID)
		}
		m.state = stateLoading
		m.loadMsg = "Loading merge requests..."
		return m, m.cmdLoadMRs()

	case jobPipelineIDMsg:
		m.initialPipelineID = msg.pipelineID
		m.loadMsg = fmt.Sprintf("Loading pipeline #%d...", m.initialPipelineID)
		return m, tea.Batch(
			m.cmdLoadPipelineDetail(m.initialPipelineID),
			m.cmdLoadPipelineJobs(m.initialPipelineID),
		)

	case mrLoadedMsg:
		m.mrs = msg.items
		m.mrTotalPage = msg.totalPages
		m.mrCursor = 0
		if m.state == stateLoading {
			m.state = stateMain
		}
		return m, nil

	case mrDetailLoadedMsg:
		m.mrDetail = msg.item
		var cmds []tea.Cmd
		if m.mrDiffFiles == nil {
			cmds = append(cmds,
				m.cmdLoadMRDiffs(m.mrDetail.IID),
				m.cmdLoadMRDiscussions(m.mrDetail.IID),
			)
		}
		if m.state == stateLoading {
			m.state = stateDetail
			m.prevState = stateMain
			m.tab = tabMRs
		}
		if len(cmds) > 0 {
			return m, tea.Batch(cmds...)
		}
		return m, nil

	case mrDiffsLoadedMsg:
		m.mrDiffFiles = msg.files
		m.mrDiffVersion = msg.version
		m.mrDiffFileIdx = 0
		m.mrDiffLineCursor = 0
		m.mrDiffScrollOffset = 0
		m.updateDiffScroll()
		return m, nil

	case mrDiscussionsLoadedMsg:
		m.mrDiscussions = msg.discussions
		return m, nil

	case pipelineLoadedMsg:
		m.pipelines = msg.items
		m.pipelineTotalPage = msg.totalPages
		m.pipelineCursor = 0
		return m, nil

	case pipelineJobsLoadedMsg:
		m.pipelineJobs = msg.items
		if m.initialJobID > 0 {
			jobIdx := -1
			for i, job := range m.pipelineJobs {
				if job.ID == m.initialJobID {
					jobIdx = i
					break
				}
			}
			if jobIdx >= 0 {
				m.jobCursor = jobIdx
				job := m.pipelineJobs[jobIdx]
				m.initialJobID = 0 // Clear it so we don't repeat this
				m.state = stateLoading
				m.loadMsg = "Loading job trace..."
				return m, m.cmdLoadJobTrace(job)
			}
			m.initialJobID = 0 // Clear even if not found
		}
		if m.jobCursor >= len(m.pipelineJobs) {
			m.jobCursor = len(m.pipelineJobs) - 1
		}
		if m.jobCursor < 0 {
			m.jobCursor = 0
		}
		if m.state == stateDetail && m.tab == tabPipelines && isPipelineOrJobsActive(m.pipelineDetail, m.pipelineJobs) {
			return m, tickCmd()
		}
		return m, nil

	case pipelineDetailLoadedMsg:
		m.pipelineDetail = msg.item
		if m.state == stateLoading && m.initialJobID == 0 && m.loadMsg != "Loading job trace..." {
			m.state = stateDetail
			m.prevState = stateMain
			m.tab = tabPipelines
		}
		if m.state == stateDetail && m.tab == tabPipelines && isPipelineOrJobsActive(m.pipelineDetail, m.pipelineJobs) {
			return m, tickCmd()
		}
		return m, nil

	case tickMsg:
		if m.state == stateDetail && m.tab == tabPipelines && m.pipelineDetail != nil && isPipelineOrJobsActive(m.pipelineDetail, m.pipelineJobs) {
			return m, tea.Batch(
				m.cmdLoadPipelineDetail(m.pipelineDetail.ID),
				m.cmdLoadPipelineJobs(m.pipelineDetail.ID),
			)
		}
		return m, nil

	case jobTraceLoadedMsg:
		m.jobTraceJob = msg.job
		traceStr := strings.ReplaceAll(msg.trace, "\r\n", "\n")
		traceStr = strings.ReplaceAll(traceStr, "\r", "\n")
		m.jobTrace = traceStr
		m.jobTraceScrollOffset = 0
		m.jobTraceOpen = true
		m.jobTraceFocus = true
		m.state = stateDetail
		return m, nil

	case issueLoadedMsg:
		m.issues = msg.items
		m.issueTotalPage = msg.totalPages
		m.issueCursor = 0
		return m, nil

	case projectLoadedMsg:
		m.projects = msg.items
		m.projectTotalPage = msg.totalPages
		m.projectCursor = 0
		return m, nil

	case actionDoneMsg:
		m.doneMsg = msg.msg
		m.state = m.prevState
		// When returning to MR detail, reload both details and discussions so vote counts and threads are fresh.
		if m.state == stateDetail && m.tab == tabMRs && m.mrDetail != nil {
			return m, tea.Batch(
				m.cmdLoadMRDetail(m.mrDetail.IID),
				m.cmdLoadMRDiscussions(m.mrDetail.IID),
			)
		}
		if m.state == stateDetail && m.tab == tabPipelines && m.pipelineDetail != nil {
			return m, tea.Batch(
				m.cmdLoadPipelineDetail(m.pipelineDetail.ID),
				m.cmdLoadPipelineJobs(m.pipelineDetail.ID),
			)
		}
		return m, m.reloadCurrent()

	case youtrackTuiFinishedMsg:
		return m, tea.ClearScreen

	case tea.KeyMsg:
		// Comment input captures all keys
		if m.state == stateComment {
			return m.handleCommentKey(msg)
		}
		if m.state == stateMain && m.tab == tabProjects && m.projectSearch.Focused() {
			key := msg.String()
			switch key {
			case "ctrl+c":
				return m, tea.Quit
			case "tab":
				m.tab = (m.tab + 1) % tabCount
				m.projectSearch.Blur()
				return m, m.reloadCurrent()
			case "shift+tab":
				m.tab = (m.tab - 1 + tabCount) % tabCount
				m.projectSearch.Blur()
				return m, m.reloadCurrent()
			case "up":
				if m.projectCursor > 0 {
					m.projectCursor--
				}
				return m, nil
			case "down":
				if m.projectCursor < len(m.projects)-1 {
					m.projectCursor++
				}
				return m, nil
			case "enter":
				if len(m.projects) > 0 && m.projectCursor < len(m.projects) {
					m.project = m.projects[m.projectCursor]
					m.tab = tabMRs
					m.mrPage = 1
					m.pipelinePage = 1
					m.issuePage = 1
					m.projectSearch.Blur()
					return m, m.cmdLoadMRs()
				}
				return m, nil
			case "esc":
				m.projectSearch.SetValue("")
				m.projectPage = 1
				return m, m.cmdLoadProjects()
			case "pgdown":
				return m.nextPage()
			case "pgup":
				return m.prevPage()
			default:
				var cmd tea.Cmd
				m.projectSearch, cmd = m.projectSearch.Update(msg)
				m.projectPage = 1
				return m, tea.Batch(cmd, m.cmdLoadProjects())
			}
		}
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global quit — works in every state
	if key == "ctrl+c" || (key == "q" && (m.tab != tabProjects || m.state == stateDetail || m.state == stateError || m.state == stateLoading)) {
		return m, tea.Quit
	}

	// Escape / back
	if key == "esc" {
		switch m.state {
		case stateDetail:
			if m.mrDiffPanelOpen {
				// Close diff panel first
				m.mrDiffPanelOpen = false
				return m, nil
			}
			if m.jobTraceOpen {
				m.jobTraceOpen = false
				m.jobTraceFocus = false
				return m, nil
			}
			m.state = stateMain
			m.mrDetail = nil
			m.mrDiffFiles = nil
			m.mrDiffVersion = nil
			m.mrDiffPanelOpen = false
			m.mrDiscussions = nil
			m.mrDetailScrollOffset = 0
			m.pipelineDetail = nil
			m.pipelineJobs = nil
			m.jobCursor = 0
			m.jobTrace = ""
			m.jobTraceJob = nil
			m.jobTraceOpen = false
			m.jobTraceFocus = false
			m.issueDetail = nil
			if len(m.mrs) == 0 {
				m.state = stateLoading
				m.loadMsg = "Loading merge requests..."
				return m, m.cmdLoadMRs()
			}
		case stateServerSelect:
			m.state = stateMain
		case stateConfirm:
			m.state = m.prevState
			m.confirm = nil
		case stateLinkSelect:
			m.state = m.prevState
			m.linkItems = nil
		case stateError:
			if m.prevState == stateLoading || m.prevState == stateError {
				m.state = stateMain
			} else {
				m.state = m.prevState
			}
		}
		return m, nil
	}

	switch m.state {
	case stateMain:
		return m.handleMainKey(key)
	case stateDetail:
		return m.handleDetailKey(key)
	case stateServerSelect:
		return m.handleServerSelectKey(key)
	case stateLinkSelect:
		return m.handleLinkSelectKey(key)
	case stateConfirm:
		return m.handleConfirmKey(key)
	case stateError:
		if key == "enter" || key == " " {
			if m.prevState == stateLoading || m.prevState == stateError {
				m.state = stateMain
			} else {
				m.state = m.prevState
			}
			return m, nil
		}
	}
	return m, nil
}

// ─── Main view key handler ────────────────────────────────────────────────────

func (m Model) handleMainKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "tab", "right", "l":
		m.tab = (m.tab + 1) % tabCount
		if m.tab == tabProjects {
			m.projectSearch.Focus()
		} else {
			m.projectSearch.Blur()
		}
		return m, m.reloadCurrent()
	case "shift+tab", "left", "h":
		m.tab = (m.tab - 1 + tabCount) % tabCount
		if m.tab == tabProjects {
			m.projectSearch.Focus()
		} else {
			m.projectSearch.Blur()
		}
		return m, m.reloadCurrent()
	case "1":
		m.tab = tabMRs
		m.projectSearch.Blur()
		return m, m.cmdLoadMRs()
	case "2":
		m.tab = tabPipelines
		m.projectSearch.Blur()
		return m, m.cmdLoadPipelines()
	case "3":
		m.tab = tabIssues
		m.projectSearch.Blur()
		return m, m.cmdLoadIssues()
	case "4":
		m.tab = tabProjects
		m.projectSearch.Focus()
		return m, m.cmdLoadProjects()
	case "j", "down":
		m.moveCursorDown()
	case "k", "up":
		m.moveCursorUp()
	case "enter":
		return m.openDetail()
	case "n":
		return m.nextPage()
	case "p":
		return m.prevPage()
	case "pgdown":
		return m.nextPage()
	case "pgup":
		return m.prevPage()
	case "r":
		return m, m.reloadCurrent()
	case "s":
		// Switch MR state filter
		if m.tab == tabMRs {
			switch m.mrState {
			case gitlab.MRStateOpened:
				m.mrState = gitlab.MRStateMerged
			case gitlab.MRStateMerged:
				m.mrState = gitlab.MRStateClosed
			case gitlab.MRStateClosed:
				m.mrState = gitlab.MRStateAll
			default:
				m.mrState = gitlab.MRStateOpened
			}
			m.mrPage = 1
			return m, m.cmdLoadMRs()
		}
	case "S":
		// Switch server
		m.serverCursor = m.serverIdx
		m.prevState = m.state
		m.state = stateServerSelect
	}
	return m, nil
}

// ─── Detail key handler ───────────────────────────────────────────────────────

func (m Model) handleDetailKey(key string) (tea.Model, tea.Cmd) {
	switch m.tab {
	case tabMRs:
		if m.mrDetail == nil {
			return m, nil
		}
		// Diff-panel navigation (active when panel is open)
		if m.mrDiffPanelOpen {
			switch key {
			case "tab":
				// Toggle panel off
				m.mrDiffPanelOpen = false
				return m, nil
			case "j", "down":
				m.diffLineCursorDown()
				m.updateDiffScroll()
				return m, nil
			case "k", "up":
				m.diffLineCursorUp()
				m.updateDiffScroll()
				return m, nil
			case "J":
				// Jump to next diff block
				m.diffNextHunk()
				m.updateDiffScroll()
				return m, nil
			case "K":
				// Jump to previous diff block
				m.diffPrevHunk()
				m.updateDiffScroll()
				return m, nil
			case "n":
				// Next file
				if m.mrDiffFileIdx < len(m.mrDiffFiles)-1 {
					m.mrDiffFileIdx++
					m.mrDiffLineCursor = 0
					m.mrDiffScrollOffset = 0
					m.updateDiffScroll()
				}
				return m, nil
			case "p":
				// Prev file
				if m.mrDiffFileIdx > 0 {
					m.mrDiffFileIdx--
					m.mrDiffLineCursor = 0
					m.mrDiffScrollOffset = 0
					m.updateDiffScroll()
				}
				return m, nil
			case "N":
				// Inline comment on current line
				if len(m.mrDiffFiles) == 0 {
					return m, nil
				}
				f := m.mrDiffFiles[m.mrDiffFileIdx]
				if m.mrDiffLineCursor >= len(f.Lines) {
					return m, nil
				}
				l := f.Lines[m.mrDiffLineCursor]
				if l.Type == "hunk" {
					return m, nil // can't comment on hunk headers
				}
				m.commentInlineFile = f
				m.commentInlineLine = l
				m.commentMode = commentModeInline
				m.commentInput.SetValue("")
				cmd := m.commentInput.Focus()
				m.prevState = m.state
				m.state = stateComment
				return m, cmd
			case "r":
				// Reply to inline comment on current line
				if len(m.mrDiffFiles) == 0 {
					return m, nil
				}
				f := m.mrDiffFiles[m.mrDiffFileIdx]
				if m.mrDiffLineCursor >= len(f.Lines) {
					return m, nil
				}
				l := f.Lines[m.mrDiffLineCursor]
				discs := m.getDiscussionsForLine(f, l)
				if len(discs) == 0 {
					return m, nil // no thread to reply to
				}
				m.commentReplyDiscussionID = discs[0].ID
				m.commentMode = commentModeReply
				m.commentInput.SetValue("")
				cmd := m.commentInput.Focus()
				m.prevState = m.state
				m.state = stateComment
				return m, cmd
			}
		}
		switch key {
		case "j", "down":
			m.mrDetailScrollOffset++
			return m, nil
		case "k", "up":
			if m.mrDetailScrollOffset > 0 {
				m.mrDetailScrollOffset--
			}
			return m, nil
		case "tab":
			// Toggle diff panel
			m.mrDiffPanelOpen = !m.mrDiffPanelOpen
			return m, nil
		case "C":
			// General comment on MR
			m.commentMode = commentModeGeneral
			m.commentInput.SetValue("")
			cmd := m.commentInput.Focus()
			m.prevState = m.state
			m.state = stateComment
			return m, cmd
		case "a":
			return m.promptConfirm("Approve MR", fmt.Sprintf("Approve MR !%d: %s?", m.mrDetail.IID, m.mrDetail.Title),
				m.cmdApproveMR(m.mrDetail.IID))
		case "m":
			if m.mrDetail.State == "opened" {
				return m.promptConfirm("Merge MR", fmt.Sprintf("Merge MR !%d: %s?", m.mrDetail.IID, m.mrDetail.Title),
					m.cmdMergeMR(m.mrDetail.IID))
			}
		case "x":
			if m.mrDetail.State == "opened" {
				return m.promptConfirm("Close MR", fmt.Sprintf("Close MR !%d?", m.mrDetail.IID),
					m.cmdCloseMR(m.mrDetail.IID))
			}
		case "+":
			m.state = stateLoading
			m.prevState = stateDetail
			m.loadMsg = "Voting..."
			return m, m.cmdVoteUpMR(m.mrDetail.IID)
		case "-":
			m.state = stateLoading
			m.prevState = stateDetail
			m.loadMsg = "Voting..."
			return m, m.cmdVoteDownMR(m.mrDetail.IID)
		case "o":
			m.linkItems = m.collectLinksForDetail()
			if len(m.linkItems) > 0 {
				m.linkCursor = 0
				m.prevState = m.state
				m.state = stateLinkSelect
			}
		}
	case tabPipelines:
		if m.pipelineDetail == nil {
			return m, nil
		}
		if m.jobTraceOpen {
			switch key {
			case "j", "down":
				m.jobTraceScrollOffset++
				return m, nil
			case "k", "up":
				if m.jobTraceScrollOffset > 0 {
					m.jobTraceScrollOffset--
				}
				return m, nil
			case "g":
				m.jobTraceScrollOffset = 0
				return m, nil
			case "G":
				bodyH := m.getBodyHeight()
				traceH := bodyH - 3
				traceLines := strings.Split(m.jobTrace, "\n")
				m.jobTraceScrollOffset = len(traceLines) - traceH
				if m.jobTraceScrollOffset < 0 {
					m.jobTraceScrollOffset = 0
				}
				return m, nil
			case "ctrl+g":
				return m, m.cmdOpenTraceInEditor()
			case "esc", "tab", "enter":
				m.jobTraceOpen = false
				m.jobTraceFocus = false
				return m, nil
			}
			return m, nil
		}
		switch key {
		case "j", "down":
			if len(m.pipelineJobs) > 0 && m.jobCursor < len(m.pipelineJobs)-1 {
				m.jobCursor++
			}
			return m, nil
		case "k", "up":
			if m.jobCursor > 0 {
				m.jobCursor--
			}
			return m, nil
		case "enter":
			if len(m.pipelineJobs) > 0 && m.jobCursor < len(m.pipelineJobs) {
				job := m.pipelineJobs[m.jobCursor]
				m.state = stateLoading
				m.prevState = stateDetail
				m.loadMsg = "Loading job trace..."
				return m, m.cmdLoadJobTrace(job)
			}
			return m, nil
		case "r":
			if len(m.pipelineJobs) > 0 && m.jobCursor < len(m.pipelineJobs) {
				job := m.pipelineJobs[m.jobCursor]
				if job.Status == "manual" {
					return m.promptConfirm("Play Job", fmt.Sprintf("Play manual job '%s'?", job.Name),
						m.cmdPlayJob(job.ID))
				}
				return m.promptConfirm("Retry Job", fmt.Sprintf("Retry job '%s'?", job.Name),
					m.cmdRetryJob(job.ID))
			}
			return m, nil
		case "R":
			return m.promptConfirm("Retry Pipeline", fmt.Sprintf("Retry pipeline #%d?", m.pipelineDetail.ID),
				m.cmdRetryPipeline(m.pipelineDetail.ID))
		case "c":
			if m.pipelineDetail.Status == "running" || m.pipelineDetail.Status == "pending" {
				return m.promptConfirm("Cancel Pipeline", fmt.Sprintf("Cancel pipeline #%d?", m.pipelineDetail.ID),
					m.cmdCancelPipeline(m.pipelineDetail.ID))
			}
		case "o":
			m.linkItems = m.collectLinksForDetail()
			if len(m.linkItems) > 0 {
				m.linkCursor = 0
				m.prevState = m.state
				m.state = stateLinkSelect
			}
		}
	case tabIssues:
		if m.issueDetail == nil {
			return m, nil
		}
		switch key {
		case "o":
			m.linkItems = m.collectLinksForDetail()
			if len(m.linkItems) > 0 {
				m.linkCursor = 0
				m.prevState = m.state
				m.state = stateLinkSelect
			}
		}
	}
	return m, nil
}

// ─── Comment key handler ──────────────────────────────────────────────────────

func (m Model) handleCommentKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.commentInput.Blur()
		m.state = m.prevState
		return m, nil
	case "alt+enter":
		m.commentInput.InsertRune('\n')
		return m, nil
	case "ctrl+s", "enter":
		body := strings.TrimSpace(m.commentInput.Value())
		if body == "" {
			m.state = m.prevState
			return m, nil
		}
		m.commentInput.Blur()
		m.state = stateLoading
		m.loadMsg = "Posting comment..."
		if m.commentMode == commentModeInline {
			return m, m.cmdCreateInlineComment(body)
		} else if m.commentMode == commentModeReply {
			return m, m.cmdReplyToDiscussion(body)
		}
		return m, m.cmdCreateMRComment(body)
	default:
		var cmd tea.Cmd
		m.commentInput, cmd = m.commentInput.Update(msg)
		return m, cmd
	}
}

// ─── Server select key handler ────────────────────────────────────────────────

func (m Model) handleServerSelectKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "j", "down":
		if m.serverCursor < len(m.cfg.Servers)-1 {
			m.serverCursor++
		}
	case "k", "up":
		if m.serverCursor > 0 {
			m.serverCursor--
		}
	case "enter":
		srv := m.cfg.Servers[m.serverCursor]
		c, err := gitlab.NewClient(srv.URL, srv.Token)
		if err != nil {
			m.prevState = m.state
			m.state = stateError
			m.errText = err.Error()
			return m, nil
		}
		m.client = c
		m.serverIdx = m.serverCursor
		m.project = nil
		m.state = stateMain
		m.tab = tabProjects
		m.projectPage = 1
		m.projectSearch.SetValue("")
		m.projectSearch.Focus()
		return m, m.cmdLoadProjects()
	}
	return m, nil
}

// ─── Link select key handler ──────────────────────────────────────────────────

func (m Model) handleLinkSelectKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "j", "down":
		if m.linkCursor < len(m.linkItems)-1 {
			m.linkCursor++
		}
	case "k", "up":
		if m.linkCursor > 0 {
			m.linkCursor--
		}
	case "enter":
		var cmd tea.Cmd
		if m.linkCursor >= 0 && m.linkCursor < len(m.linkItems) {
			cmd = m.openURL(m.linkItems[m.linkCursor].URL)
		}
		m.state = m.prevState
		m.linkItems = nil
		return m, cmd
	case "esc":
		m.state = m.prevState
		m.linkItems = nil
	}
	return m, nil
}

// ─── Confirm key handler ──────────────────────────────────────────────────────

func (m Model) handleConfirmKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "enter":
		if m.confirm != nil {
			m.state = stateLoading
			m.loadMsg = "Processing..."
			cmd := m.confirm.perform
			m.confirm = nil
			return m, cmd
		}
	case "n":
		m.state = m.prevState
		m.confirm = nil
	}
	return m, nil
}

// ─── Diff cursor helpers ──────────────────────────────────────────────────────

func (m *Model) diffLineCursorDown() {
	if len(m.mrDiffFiles) == 0 {
		return
	}
	f := m.mrDiffFiles[m.mrDiffFileIdx]
	if m.mrDiffLineCursor < len(f.Lines)-1 {
		m.mrDiffLineCursor++
	} else if m.mrDiffFileIdx < len(m.mrDiffFiles)-1 {
		// Move to next file
		m.mrDiffFileIdx++
		m.mrDiffLineCursor = 0
		m.mrDiffScrollOffset = 0
	}
}

func (m *Model) diffLineCursorUp() {
	if m.mrDiffLineCursor > 0 {
		m.mrDiffLineCursor--
	} else if m.mrDiffFileIdx > 0 {
		m.mrDiffFileIdx--
		m.mrDiffScrollOffset = 0
		if len(m.mrDiffFiles[m.mrDiffFileIdx].Lines) > 0 {
			m.mrDiffLineCursor = len(m.mrDiffFiles[m.mrDiffFileIdx].Lines) - 1
		}
	}
}

func (m *Model) diffNextHunk() {
	if len(m.mrDiffFiles) == 0 {
		return
	}
	f := m.mrDiffFiles[m.mrDiffFileIdx]
	for i := m.mrDiffLineCursor + 1; i < len(f.Lines); i++ {
		if f.Lines[i].Type == "hunk" {
			m.mrDiffLineCursor = i
			return
		}
	}
}

func (m *Model) diffPrevHunk() {
	if len(m.mrDiffFiles) == 0 {
		return
	}
	f := m.mrDiffFiles[m.mrDiffFileIdx]
	for i := m.mrDiffLineCursor - 1; i >= 0; i-- {
		if f.Lines[i].Type == "hunk" {
			m.mrDiffLineCursor = i
			return
		}
	}
}

func (m Model) diffPanelWidth() int {
	leftW := m.width * 2 / 5
	if leftW < 20 {
		leftW = 20
	}
	return m.width - leftW - 1
}

func (m Model) diffPanelHeight() int {
	header := lipgloss.NewStyle().Width(m.width).MaxHeight(1).Render(m.viewHeader())
	footer := lipgloss.NewStyle().Width(m.width).Render(m.viewDetailFooter())
	bodyH := m.height - lipgloss.Height(header) - lipgloss.Height(footer) - m.getHeightOffset()
	if bodyH < 1 {
		bodyH = 1
	}
	return bodyH
}

func (m Model) diffHeight() int {
	bodyH := m.diffPanelHeight()
	if len(m.mrDiffFiles) == 0 {
		return bodyH - 4
	}
	startIdx := m.mrDiffFileIdx - 1
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + 3
	if endIdx > len(m.mrDiffFiles) {
		endIdx = len(m.mrDiffFiles)
		startIdx = endIdx - 3
		if startIdx < 0 {
			startIdx = 0
		}
	}
	tabsLen := endIdx - startIdx
	dh := bodyH - (4 + tabsLen)
	if dh < 1 {
		dh = 1
	}
	return dh
}

func (m Model) getScreenLinesForRange(f *gitlab.DiffFile, start, end int, panelWidth int) int {
	lines := 0
	for i := start; i <= end && i < len(f.Lines); i++ {
		lines++ // for the code line itself
		discs := m.getDiscussionsForLine(f, f.Lines[i])
		for _, d := range discs {
			for _, note := range d.Notes {
				if note.System {
					continue
				}
				bodyStyle := lipgloss.NewStyle().Width(panelWidth - 10)
				bodyLines := strings.Split(bodyStyle.Render(note.Body), "\n")
				lines += len(bodyLines)
			}
		}
	}
	return lines
}

func (m Model) getDiscussionsForLine(f *gitlab.DiffFile, dl gitlab.DiffLine) []*gitlab.MRDiscussion {
	var matches []*gitlab.MRDiscussion
	for _, d := range m.mrDiscussions {
		if len(d.Notes) == 0 {
			continue
		}
		n0 := d.Notes[0]
		if n0.Position == nil {
			continue
		}
		pos := n0.Position
		pathMatch := false
		if f.NewPath != "" && pos.NewPath == f.NewPath {
			pathMatch = true
		} else if f.OldPath != "" && pos.OldPath == f.OldPath {
			pathMatch = true
		}
		if !pathMatch {
			continue
		}
		lineMatch := false
		if dl.Type == "added" && pos.NewLine > 0 && pos.NewLine == dl.NewLine {
			lineMatch = true
		} else if dl.Type == "removed" && pos.OldLine > 0 && pos.OldLine == dl.OldLine {
			lineMatch = true
		} else if dl.Type == "context" {
			if (pos.NewLine > 0 && pos.NewLine == dl.NewLine) || (pos.OldLine > 0 && pos.OldLine == dl.OldLine) {
				lineMatch = true
			}
		}
		if lineMatch {
			matches = append(matches, d)
		}
	}
	return matches
}

func (m *Model) updateDiffScroll() {
	if len(m.mrDiffFiles) == 0 {
		return
	}
	f := m.mrDiffFiles[m.mrDiffFileIdx]
	totalLines := len(f.Lines)
	diffHeight := m.diffHeight()
	w := m.diffPanelWidth()

	if m.mrDiffLineCursor < m.mrDiffScrollOffset {
		m.mrDiffScrollOffset = m.mrDiffLineCursor
	}

	for m.mrDiffScrollOffset < m.mrDiffLineCursor {
		screenLines := m.getScreenLinesForRange(f, m.mrDiffScrollOffset, m.mrDiffLineCursor, w)
		if screenLines <= diffHeight {
			break
		}
		m.mrDiffScrollOffset++
	}

	if m.mrDiffScrollOffset >= totalLines {
		m.mrDiffScrollOffset = totalLines - 1
	}
	if m.mrDiffScrollOffset < 0 {
		m.mrDiffScrollOffset = 0
	}
}

func (m Model) getHeightOffset() int {
	offset := 1
	if val := os.Getenv("GITLAB_TUI_HEIGHT_OFFSET"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			offset = parsed
		}
	}
	return offset
}

// ─── Cursor helpers ───────────────────────────────────────────────────────────

func (m *Model) listLen() int {
	switch m.tab {
	case tabMRs:
		return len(m.mrs)
	case tabPipelines:
		return len(m.pipelines)
	case tabIssues:
		return len(m.issues)
	case tabProjects:
		return len(m.projects)
	}
	return 0
}

func (m *Model) cursor() int {
	switch m.tab {
	case tabMRs:
		return m.mrCursor
	case tabPipelines:
		return m.pipelineCursor
	case tabIssues:
		return m.issueCursor
	case tabProjects:
		return m.projectCursor
	}
	return 0
}

func (m *Model) moveCursorDown() {
	n := m.listLen()
	switch m.tab {
	case tabMRs:
		if m.mrCursor < n-1 {
			m.mrCursor++
		}
	case tabPipelines:
		if m.pipelineCursor < n-1 {
			m.pipelineCursor++
		}
	case tabIssues:
		if m.issueCursor < n-1 {
			m.issueCursor++
		}
	case tabProjects:
		if m.projectCursor < n-1 {
			m.projectCursor++
		}
	}
}

func (m *Model) moveCursorUp() {
	switch m.tab {
	case tabMRs:
		if m.mrCursor > 0 {
			m.mrCursor--
		}
	case tabPipelines:
		if m.pipelineCursor > 0 {
			m.pipelineCursor--
		}
	case tabIssues:
		if m.issueCursor > 0 {
			m.issueCursor--
		}
	case tabProjects:
		if m.projectCursor > 0 {
			m.projectCursor--
		}
	}
}

func (m Model) openDetail() (Model, tea.Cmd) {
	switch m.tab {
	case tabMRs:
		if m.mrCursor < len(m.mrs) {
			m.mrDetail = m.mrs[m.mrCursor] // placeholder until fresh fetch arrives
			m.prevState = stateMain
			m.state = stateDetail
			return m, tea.Batch(
				m.cmdLoadMRDetail(m.mrDetail.IID),
				m.cmdLoadMRDiscussions(m.mrDetail.IID),
			)
		}
	case tabPipelines:
		if m.pipelineCursor < len(m.pipelines) {
			m.pipelineDetail = m.pipelines[m.pipelineCursor]
			m.prevState = stateMain
			m.state = stateDetail
			m.pipelineJobs = nil
			m.jobCursor = 0
			m.jobTrace = ""
			m.jobTraceJob = nil
			m.jobTraceOpen = false
			m.jobTraceFocus = false
			return m, tea.Batch(
				m.cmdLoadPipelineDetail(m.pipelineDetail.ID),
				m.cmdLoadPipelineJobs(m.pipelineDetail.ID),
			)
		}
	case tabIssues:
		if m.issueCursor < len(m.issues) {
			m.issueDetail = m.issues[m.issueCursor]
			m.prevState = stateMain
			m.state = stateDetail
		}
	case tabProjects:
		if m.projectCursor < len(m.projects) {
			m.project = m.projects[m.projectCursor]
			m.state = stateMain
			m.tab = tabMRs
			m.mrPage = 1
			m.projectSearch.Blur()
			return m, m.cmdLoadMRs()
		}
	}
	return m, nil
}

func (m Model) nextPage() (Model, tea.Cmd) {
	switch m.tab {
	case tabMRs:
		if m.mrPage < m.mrTotalPage {
			m.mrPage++
			return m, m.cmdLoadMRs()
		}
	case tabPipelines:
		if m.pipelinePage < m.pipelineTotalPage {
			m.pipelinePage++
			return m, m.cmdLoadPipelines()
		}
	case tabIssues:
		if m.issuePage < m.issueTotalPage {
			m.issuePage++
			return m, m.cmdLoadIssues()
		}
	case tabProjects:
		if m.projectPage < m.projectTotalPage {
			m.projectPage++
			return m, m.cmdLoadProjects()
		}
	}
	return m, nil
}

func (m Model) prevPage() (Model, tea.Cmd) {
	switch m.tab {
	case tabMRs:
		if m.mrPage > 1 {
			m.mrPage--
			return m, m.cmdLoadMRs()
		}
	case tabPipelines:
		if m.pipelinePage > 1 {
			m.pipelinePage--
			return m, m.cmdLoadPipelines()
		}
	case tabIssues:
		if m.issuePage > 1 {
			m.issuePage--
			return m, m.cmdLoadIssues()
		}
	case tabProjects:
		if m.projectPage > 1 {
			m.projectPage--
			return m, m.cmdLoadProjects()
		}
	}
	return m, nil
}

func (m Model) reloadCurrent() tea.Cmd {
	if m.project == nil {
		if m.tab == tabProjects {
			return m.cmdLoadProjects()
		}
		return nil
	}
	switch m.tab {
	case tabMRs:
		return m.cmdLoadMRs()
	case tabPipelines:
		return m.cmdLoadPipelines()
	case tabIssues:
		return m.cmdLoadIssues()
	case tabProjects:
		return m.cmdLoadProjects()
	}
	return nil
}

func (m Model) promptConfirm(label, message string, cmd tea.Cmd) (Model, tea.Cmd) {
	m.confirm = &confirmAction{label: message, perform: cmd}
	m.prevState = m.state
	m.state = stateConfirm
	m.confirmYes = true
	return m, nil
}

// ─── Commands ─────────────────────────────────────────────────────────────────

func (m Model) cmdWhoAmI() tea.Cmd {
	return func() tea.Msg {
		u, err := m.client.WhoAmI()
		if err != nil {
			return errMsg{err}
		}
		return whoAmIMsg{u}
	}
}

func (m Model) cmdLoadMRs() tea.Cmd {
	if m.project == nil {
		return nil
	}
	pid := m.project.ID
	state := m.mrState
	page := m.mrPage
	return func() tea.Msg {
		items, total, err := m.client.ListMRs(pid, state, page)
		if err != nil {
			return errMsg{err}
		}
		return mrLoadedMsg{items, total}
	}
}

func (m Model) cmdLoadPipelines() tea.Cmd {
	if m.project == nil {
		return nil
	}
	pid := m.project.ID
	page := m.pipelinePage
	return func() tea.Msg {
		items, total, err := m.client.ListPipelines(pid, page)
		if err != nil {
			return errMsg{err}
		}
		return pipelineLoadedMsg{items, total}
	}
}

func (m Model) cmdLoadIssues() tea.Cmd {
	if m.project == nil {
		return nil
	}
	pid := m.project.ID
	page := m.issuePage
	return func() tea.Msg {
		items, total, err := m.client.ListIssues(pid, gitlab.IssueStateOpened, page)
		if err != nil {
			return errMsg{err}
		}
		return issueLoadedMsg{items, total}
	}
}

func (m Model) cmdLoadProjects() tea.Cmd {
	search := m.projectSearch.Value()
	page := m.projectPage
	return func() tea.Msg {
		items, total, err := m.client.ListProjects(search, page)
		if err != nil {
			return errMsg{err}
		}
		return projectLoadedMsg{items, total}
	}
}

func (m Model) cmdApproveMR(iid int) tea.Cmd {
	pid := m.project.ID
	return func() tea.Msg {
		if err := m.client.ApproveMR(pid, iid); err != nil {
			return errMsg{err}
		}
		return actionDoneMsg{"MR approved!"}
	}
}

func (m Model) cmdMergeMR(iid int) tea.Cmd {
	pid := m.project.ID
	return func() tea.Msg {
		if err := m.client.MergeMR(pid, iid); err != nil {
			return errMsg{err}
		}
		return actionDoneMsg{"MR merged!"}
	}
}

func (m Model) cmdCloseMR(iid int) tea.Cmd {
	pid := m.project.ID
	return func() tea.Msg {
		if err := m.client.CloseMR(pid, iid); err != nil {
			return errMsg{err}
		}
		return actionDoneMsg{"MR closed!"}
	}
}

func (m Model) cmdRetryPipeline(id int) tea.Cmd {
	pid := m.project.ID
	return func() tea.Msg {
		if err := m.client.RetryPipeline(pid, id); err != nil {
			return errMsg{err}
		}
		return actionDoneMsg{"Pipeline retry triggered!"}
	}
}

func (m Model) cmdCancelPipeline(id int) tea.Cmd {
	pid := m.project.ID
	return func() tea.Msg {
		if err := m.client.CancelPipeline(pid, id); err != nil {
			return errMsg{err}
		}
		return actionDoneMsg{"Pipeline cancelled!"}
	}
}

func (m Model) cmdLoadPipelineDetail(pipelineID int) tea.Cmd {
	if m.project == nil {
		return nil
	}
	pid := m.project.ID
	return func() tea.Msg {
		item, err := m.client.GetPipeline(pid, pipelineID)
		if err != nil {
			return errMsg{err}
		}
		return pipelineDetailLoadedMsg{item}
	}
}

func (m Model) cmdLoadPipelineJobs(pipelineID int) tea.Cmd {
	if m.project == nil {
		return nil
	}
	pid := m.project.ID
	return func() tea.Msg {
		items, err := m.client.ListPipelineJobs(pid, pipelineID)
		if err != nil {
			return errMsg{err}
		}
		return pipelineJobsLoadedMsg{items}
	}
}

func (m Model) cmdLoadJobTrace(job *gitlab.JobInfo) tea.Cmd {
	if m.project == nil {
		return nil
	}
	pid := m.project.ID
	return func() tea.Msg {
		trace, err := m.client.GetJobTrace(pid, job.ID)
		if err != nil {
			return errMsg{err}
		}
		return jobTraceLoadedMsg{job, trace}
	}
}

func (m Model) cmdGetJobPipelineID(jobID int64) tea.Cmd {
	if m.project == nil {
		return nil
	}
	pid := m.project.ID
	return func() tea.Msg {
		pipelineID, err := m.client.GetJobPipelineID(pid, jobID)
		if err != nil {
			return errMsg{err}
		}
		return jobPipelineIDMsg{pipelineID}
	}
}

func (m Model) cmdRetryJob(jobID int64) tea.Cmd {
	if m.project == nil {
		return nil
	}
	pid := m.project.ID
	return func() tea.Msg {
		err := m.client.RetryJob(pid, jobID)
		if err != nil {
			return errMsg{err}
		}
		return actionDoneMsg{fmt.Sprintf("Job #%d retried successfully", jobID)}
	}
}

func (m Model) cmdPlayJob(jobID int64) tea.Cmd {
	if m.project == nil {
		return nil
	}
	pid := m.project.ID
	return func() tea.Msg {
		err := m.client.PlayJob(pid, jobID)
		if err != nil {
			return errMsg{err}
		}
		return actionDoneMsg{fmt.Sprintf("Job #%d started successfully", jobID)}
	}
}

func (m Model) cmdLoadMRDiffs(iid int) tea.Cmd {
	pid := m.project.ID
	return func() tea.Msg {
		files, err := m.client.GetMRDiffs(pid, iid)
		if err != nil {
			return errMsg{err}
		}
		ver, err := m.client.GetMRVersion(pid, iid)
		if err != nil {
			// Non-fatal: inline comments won't work but diffs still show
			ver = nil
		}
		return mrDiffsLoadedMsg{files: files, version: ver}
	}
}

func (m Model) cmdCreateMRComment(body string) tea.Cmd {
	pid := m.project.ID
	iid := m.mrDetail.IID
	return func() tea.Msg {
		if err := m.client.CreateMRComment(pid, iid, body); err != nil {
			return errMsg{err}
		}
		return actionDoneMsg{"💬 Comment posted!"}
	}
}

func (m Model) cmdCreateInlineComment(body string) tea.Cmd {
	pid := m.project.ID
	iid := m.mrDetail.IID
	ver := m.mrDiffVersion
	f := m.commentInlineFile
	l := m.commentInlineLine
	return func() tea.Msg {
		if ver == nil {
			return errMsg{fmt.Errorf("diff version SHAs not available; cannot post inline comment")}
		}
		err := m.client.CreateMRInlineComment(
			pid, iid, body,
			ver.BaseSHA, ver.StartSHA, ver.HeadSHA,
			f.OldPath, f.NewPath,
			l.OldLine, l.NewLine,
		)
		if err != nil {
			return errMsg{err}
		}
		return actionDoneMsg{"💬 Inline comment posted!"}
	}
}

func (m Model) cmdLoadMRDiscussions(iid int) tea.Cmd {
	pid := m.project.ID
	return func() tea.Msg {
		discs, err := m.client.GetMRDiscussions(pid, iid)
		if err != nil {
			return errMsg{err}
		}
		return mrDiscussionsLoadedMsg{discussions: discs}
	}
}

func (m Model) cmdReplyToDiscussion(body string) tea.Cmd {
	pid := m.project.ID
	iid := m.mrDetail.IID
	discussionID := m.commentReplyDiscussionID
	return func() tea.Msg {
		if err := m.client.ReplyToMRDiscussion(pid, iid, discussionID, body); err != nil {
			return errMsg{err}
		}
		return actionDoneMsg{"💬 Reply posted!"}
	}
}

func (m Model) cmdLoadMRDetail(iid int) tea.Cmd {
	pid := m.project.ID
	return func() tea.Msg {
		mr, err := m.client.GetMR(pid, iid)
		if err != nil {
			return errMsg{err}
		}
		return mrDetailLoadedMsg{mr}
	}
}

func (m Model) cmdVoteUpMR(iid int) tea.Cmd {
	pid := m.project.ID
	username := m.username
	return func() tea.Msg {
		added, err := m.client.ToggleVoteMR(pid, iid, "thumbsup", username)
		if err != nil {
			return errMsg{err}
		}
		if added {
			return actionDoneMsg{"👍 Vote up added!"}
		}
		return actionDoneMsg{"👍 Vote up removed."}
	}
}

func (m Model) cmdVoteDownMR(iid int) tea.Cmd {
	pid := m.project.ID
	username := m.username
	return func() tea.Msg {
		added, err := m.client.ToggleVoteMR(pid, iid, "thumbsdown", username)
		if err != nil {
			return errMsg{err}
		}
		if added {
			return actionDoneMsg{"👎 Vote down added!"}
		}
		return actionDoneMsg{"👎 Vote down removed."}
	}
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	switch m.state {
	case stateLoading:
		return m.viewLoading()
	case stateError:
		return m.viewError()
	case stateServerSelect:
		return m.viewServerSelect()
	case stateLinkSelect:
		return m.viewLinkSelect()
	case stateConfirm:
		return m.viewConfirm()
	case stateComment:
		return m.viewCommentComposer()
	case stateDetail:
		return m.viewDetail()
	default:
		return m.viewMain()
	}
}

// ─── Loading screen ───────────────────────────────────────────────────────────

func (m Model) viewBackground() string {
	state := m.prevState
	if state == stateLoading || state == stateError {
		state = stateMain
	}
	switch state {
	case stateServerSelect:
		return m.viewServerSelect()
	case stateConfirm:
		return m.viewConfirm()
	case stateComment:
		return m.viewCommentComposer()
	case stateDetail:
		return m.viewDetail()
	default:
		return m.viewMain()
	}
}

func (m Model) viewLoading() string {
	bg := m.viewBackground()

	box := panelStyle.Padding(1, 4).Render(lipgloss.JoinVertical(lipgloss.Center,
		accentStyle.Render("GitLab TUI"),
		"",
		m.spin.View()+" "+dimStyle.Render(m.loadMsg),
	))

	dlgWidth := lipgloss.Width(box)
	dlgHeight := lipgloss.Height(box)

	targetHeight := m.height - m.getHeightOffset()
	startX := (m.width - dlgWidth) / 2
	startY := (targetHeight - dlgHeight) / 2

	return overlay(bg, box, m.width, targetHeight, startX, startY)
}

// ─── Error screen ─────────────────────────────────────────────────────────────

func (m Model) viewError() string {
	bg := m.viewBackground()

	maxTextWidth := m.width - 12
	if maxTextWidth < 20 {
		maxTextWidth = 20
	}
	if maxTextWidth > 80 {
		maxTextWidth = 80
	}

	wrappedText := dimStyle.Width(maxTextWidth).Align(lipgloss.Center).Render(m.errText)

	box := panelStyle.Padding(1, 4).Render(lipgloss.JoinVertical(lipgloss.Center,
		errorStyle.Render("⚠  Error"),
		"",
		wrappedText,
		"",
		mutedStyle.Render("Press Esc or Enter to dismiss"),
	))

	dlgWidth := lipgloss.Width(box)
	dlgHeight := lipgloss.Height(box)

	targetHeight := m.height - m.getHeightOffset()
	startX := (m.width - dlgWidth) / 2
	startY := (targetHeight - dlgHeight) / 2

	return overlay(bg, box, m.width, targetHeight, startX, startY)
}

// ─── Main view ────────────────────────────────────────────────────────────────

func (m Model) viewMain() string {
	header := lipgloss.NewStyle().Width(m.width).MaxHeight(1).Render(m.viewHeader())
	tabs := lipgloss.NewStyle().Width(m.width).MaxHeight(2).Render(m.viewTabs())
	body := m.viewBody()
	footer := lipgloss.NewStyle().Width(m.width).Render(m.viewFooter())

	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(tabs) - lipgloss.Height(footer) - m.getHeightOffset() - 1
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	bodyPanel := lipgloss.NewStyle().
		Width(m.width).
		Height(bodyHeight).
		MaxHeight(bodyHeight).
		Render(body)

	if m.doneMsg != "" {
		// Show success notification briefly — just render it in footer area
		// It clears on next keypress via reloadCurrent
		_ = m.doneMsg
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, tabs, bodyPanel, footer)
}

func (m Model) viewHeader() string {
	left := lipgloss.JoinHorizontal(lipgloss.Center,
		titleBarStyle.Render("  GitLab TUI "),
	)

	var projectName string
	if m.project != nil {
		projectName = m.project.NameWithNamespace
	} else {
		projectName = "(no project)"
	}

	serverName := ""
	if m.serverIdx < len(m.cfg.Servers) {
		serverName = m.cfg.Servers[m.serverIdx].Name
	}

	right := lipgloss.JoinHorizontal(lipgloss.Center,
		dimStyle.Render(fmt.Sprintf("%s · %dx%d · ", serverName, m.width, m.height)),
		accentStyle.Render(projectName),
		dimStyle.Render("  @"+m.username),
	)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	return lipgloss.JoinHorizontal(lipgloss.Center,
		left,
		strings.Repeat(" ", gap),
		right,
	)
}

func (m Model) viewTabs() string {
	var tabs []string
	for i := tabID(0); i < tabCount; i++ {
		tabs = append(tabs, tabStyle(tabLabels[i], m.tab == i))
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)
	divider := lipgloss.NewStyle().Foreground(colorBorder).
		Render(strings.Repeat("─", m.width))

	if m.doneMsg != "" {
		bar += "  " + successStyle.Render("✓ "+m.doneMsg)
		m.doneMsg = "" // clear after render
	}

	return lipgloss.JoinVertical(lipgloss.Left, bar, divider)
}

func (m Model) viewBody() string {
	if m.project == nil && m.tab != tabProjects {
		lines := []string{dimStyle.Render("No project selected. Press ") + accentStyle.Render("4") + dimStyle.Render(" to select a project.")}
		if m.startupWarn != "" {
			lines = append(lines, "", warningStyle.Render("⚠ "+m.startupWarn))
		}
		return lipgloss.Place(m.width, 10, lipgloss.Center, lipgloss.Center,
			strings.Join(lines, "\n"))
	}

	switch m.tab {
	case tabMRs:
		return m.viewMRList()
	case tabPipelines:
		return m.viewPipelineList()
	case tabIssues:
		return m.viewIssueList()
	case tabProjects:
		return m.viewProjectList()
	}
	return ""
}

// ─── MR list ─────────────────────────────────────────────────────────────────

func (m Model) viewMRList() string {
	if len(m.mrs) == 0 {
		return dimStyle.Padding(2).Render("No merge requests found for state: " + string(m.mrState))
	}

	var rows []string
	for i, mr := range m.mrs {
		selected := i == m.mrCursor
		title := mr.Title
		if mr.Draft {
			title = "[DRAFT] " + title
		}

		draft := ""
		if mr.Draft {
			draft = warningStyle.Render(" DRAFT")
		}

		line := fmt.Sprintf("!%-4d  %-55s  %s  %-14s  %s%s",
			mr.IID,
			truncate(title, 55),
			statusBadge(mr.State),
			truncate(mr.Author, 14),
			dimStyle.Render(mr.UpdatedAt),
			draft,
		)

		if selected {
			rows = append(rows, selectedStyle.Width(m.width-2).Render("▶ "+line))
		} else {
			rows = append(rows, normalItemStyle.Width(m.width-2).Render("  "+line))
		}
	}

	header := lipgloss.NewStyle().
		Foreground(colorMuted).
		PaddingLeft(2).
		Render(fmt.Sprintf("%-6s  %-55s  %-12s  %-14s  %-16s",
			"IID", "Title", "State", "Author", "Updated"))
	header += "\n" + lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", m.width-2))

	return header + "\n" + strings.Join(rows, "\n")
}

// ─── Pipeline list ────────────────────────────────────────────────────────────

func (m Model) viewPipelineList() string {
	if len(m.pipelines) == 0 {
		return dimStyle.Padding(2).Render("No pipelines found.")
	}

	var rows []string
	for i, p := range m.pipelines {
		selected := i == m.pipelineCursor

		line := fmt.Sprintf("#%-6d  %-22s  %s  %-14s  %-12s  %s",
			p.ID,
			truncate(p.Ref, 22),
			statusBadge(p.Status),
			truncate(p.User, 14),
			truncate(p.Source, 12),
			dimStyle.Render(p.UpdatedAt),
		)

		if selected {
			rows = append(rows, selectedStyle.Width(m.width-2).Render("▶ "+line))
		} else {
			rows = append(rows, normalItemStyle.Width(m.width-2).Render("  "+line))
		}
	}

	header := lipgloss.NewStyle().Foreground(colorMuted).PaddingLeft(2).
		Render(fmt.Sprintf("%-8s  %-22s  %-12s  %-14s  %-12s  %-16s",
			"ID", "Ref", "Status", "Triggered by", "Source", "Updated"))
	header += "\n" + lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", m.width-2))

	return header + "\n" + strings.Join(rows, "\n")
}

// ─── Issue list ───────────────────────────────────────────────────────────────

func (m Model) viewIssueList() string {
	if len(m.issues) == 0 {
		return dimStyle.Padding(2).Render("No issues found.")
	}

	var rows []string
	for i, iss := range m.issues {
		selected := i == m.issueCursor

		line := fmt.Sprintf("#%-5d  %-55s  %s  %-14s  %s",
			iss.IID,
			truncate(iss.Title, 55),
			statusBadge(iss.State),
			truncate(iss.Author, 14),
			dimStyle.Render(iss.UpdatedAt),
		)

		if selected {
			rows = append(rows, selectedStyle.Width(m.width-2).Render("▶ "+line))
		} else {
			rows = append(rows, normalItemStyle.Width(m.width-2).Render("  "+line))
		}
	}

	header := lipgloss.NewStyle().Foreground(colorMuted).PaddingLeft(2).
		Render(fmt.Sprintf("%-7s  %-55s  %-12s  %-14s  %-16s",
			"IID", "Title", "State", "Author", "Updated"))
	header += "\n" + lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", m.width-2))

	return header + "\n" + strings.Join(rows, "\n")
}

// ─── Project list (inline, inside main view) ──────────────────────────────────

func (m Model) viewProjectList() string {
	var rows []string

	// Render the search box
	rows = append(rows, "  "+m.projectSearch.View(), "")

	header := lipgloss.NewStyle().Foreground(colorMuted).PaddingLeft(2).
		Render(fmt.Sprintf("%-45s  %-40s", "Project", "Path"))
	header += "\n" + lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", m.width-2))
	rows = append(rows, header)

	if len(m.projects) == 0 {
		rows = append(rows, dimStyle.Padding(2).Render("No projects found."))
	} else {
		for i, p := range m.projects {
			selected := i == m.projectCursor

			line := fmt.Sprintf("%-45s  %s",
				truncate(p.NameWithNamespace, 45),
				dimStyle.Render(p.PathWithNamespace),
			)

			if selected {
				rows = append(rows, selectedStyle.Width(m.width-2).Render("▶ "+line))
			} else {
				rows = append(rows, normalItemStyle.Width(m.width-2).Render("  "+line))
			}
		}
	}

	return strings.Join(rows, "\n")
}

// ─── Detail views ─────────────────────────────────────────────────────────────

func (m Model) viewDetail() string {
	header := lipgloss.NewStyle().Width(m.width).MaxHeight(1).Render(m.viewHeader())
	footer := lipgloss.NewStyle().Width(m.width).Render(m.viewDetailFooter())

	bodyH := m.getBodyHeight()

	var body string
	switch m.tab {
	case tabMRs:
		if m.mrDiffPanelOpen {
			body = m.viewMRDetailSplit(bodyH)
		} else {
			body = m.viewMRDetail(bodyH)
		}
	case tabPipelines:
		body = m.viewPipelineDetail(bodyH)
	case tabIssues:
		body = m.viewIssueDetail()
	}

	bodyPanel := lipgloss.NewStyle().Width(m.width).Height(bodyH).MaxHeight(bodyH).Render(body)
	return lipgloss.JoinVertical(lipgloss.Left, header, bodyPanel, footer)
}

// viewMRDetailSplit renders left=MR detail + right=diff panel side by side.
func (m Model) viewMRDetailSplit(bodyH int) string {
	leftW := m.width * 2 / 5
	rightW := m.width - leftW - 1 // -1 for separator

	if leftW < 20 {
		leftW = 20
	}

	// Left: existing MR detail (narrower), clipped to bodyH lines to prevent terminal scrolling
	leftContent := m.viewMRDetailForWidth(leftW)
	leftLines := strings.Split(leftContent, "\n")

	totalLines := len(leftLines)
	maxScroll := totalLines - bodyH
	if maxScroll < 0 {
		maxScroll = 0
	}
	offset := m.mrDetailScrollOffset
	if offset > maxScroll {
		offset = maxScroll
	}
	if offset < 0 {
		offset = 0
	}

	end := offset + bodyH
	if end > totalLines {
		end = totalLines
	}

	clippedLeftContent := strings.Join(leftLines[offset:end], "\n")
	left := lipgloss.NewStyle().Width(leftW).Height(bodyH).MaxHeight(bodyH).Render(clippedLeftContent)

	// Separator
	sepContent := strings.Repeat("│\n", bodyH)
	if bodyH > 0 {
		sepContent = sepContent[:len(sepContent)-1]
	}
	sep := lipgloss.NewStyle().Foreground(colorBorder).Render(sepContent)

	// Right: diff panel
	rightContent := m.viewDiffPanel(rightW, bodyH)
	right := lipgloss.NewStyle().Width(rightW).Height(bodyH).MaxHeight(bodyH).Render(rightContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, sep, right)
}

// viewDiffPanel renders the changes panel for the current MR.
func (m Model) viewDiffPanel(w, h int) string {
	var lines []string

	if len(m.mrDiffFiles) == 0 {
		lines = append(lines,
			subtitleStyle.Render("  Changes"),
			"",
			dimStyle.Render("  Loading diffs..."),
		)
		return strings.Join(lines, "\n")
	}

	// File list header
	fileCount := len(m.mrDiffFiles)
	headerLine := subtitleStyle.Render("  Changes ") +
		dimStyle.Render(fmt.Sprintf("(%d file(s))  n/p=file, J/K=hunk", fileCount))
	lines = append(lines, headerLine)

	// File tabs (show nearby files)
	var fileTabs []string
	startIdx := m.mrDiffFileIdx - 1
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + 3
	if endIdx > len(m.mrDiffFiles) {
		endIdx = len(m.mrDiffFiles)
		startIdx = endIdx - 3
		if startIdx < 0 {
			startIdx = 0
		}
	}

	for i := startIdx; i < endIdx; i++ {
		f := m.mrDiffFiles[i]
		name := f.NewPath
		if len(name) > 35 {
			name = "…" + name[len(name)-34:]
		}
		label := fmt.Sprintf("+%d -%d %s", f.Added, f.Deleted, name)
		if i == m.mrDiffFileIdx {
			fileTabs = append(fileTabs, accentStyle.Render(" ▶ "+label))
		} else {
			fileTabs = append(fileTabs, dimStyle.Render("   "+label))
		}
	}
	lines = append(lines, strings.Join(fileTabs, "\n"))
	lines = append(lines, lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", w-2)))

	tabsLen := endIdx - startIdx
	diffHeight := h - (4 + tabsLen)
	if diffHeight < 1 {
		diffHeight = 1
	}

	// Current file diff lines
	f := m.mrDiffFiles[m.mrDiffFileIdx]
	renderedCount := 0

	for i := m.mrDiffScrollOffset; i < len(f.Lines) && renderedCount < diffHeight; i++ {
		dl := f.Lines[i]
		selected := i == m.mrDiffLineCursor
		content := dl.Content
		// Clip to panel width
		avail := w - 5
		if avail < 1 {
			avail = 1
		}
		if len(content) > avail {
			content = content[:avail] + "…"
		}

		var rendered string
		switch dl.Type {
		case "added":
			st := lipgloss.NewStyle().Foreground(colorSuccess)
			if selected {
				st = st.Background(colorBgHover).Bold(true)
			}
			rendered = st.Render("▶ " + content)
		case "removed":
			st := lipgloss.NewStyle().Foreground(colorError)
			if selected {
				st = st.Background(colorBgHover).Bold(true)
			}
			rendered = st.Render("▶ " + content)
		case "hunk":
			st := lipgloss.NewStyle().Foreground(colorInfo).Italic(true)
			if selected {
				st = st.Background(colorBgHover).Bold(true)
			}
			rendered = st.Render("  " + content)
		default:
			st := lipgloss.NewStyle().Foreground(colorTextDim)
			if selected {
				st = st.Background(colorBgHover)
			}
			rendered = st.Render("  " + content)
		}
		lines = append(lines, rendered)
		renderedCount++

		// Under the line, render discussions if any
		discs := m.getDiscussionsForLine(f, dl)
		for _, d := range discs {
			for _, note := range d.Notes {
				if note.System {
					continue
				}
				if renderedCount >= diffHeight {
					break
				}

				// Wrap comment body using lipgloss
				bodyStyle := lipgloss.NewStyle().Foreground(colorTextDim).Width(w - 10)
				bodyLines := strings.Split(bodyStyle.Render(note.Body), "\n")

				commentStyle := lipgloss.NewStyle().Foreground(colorTeal).Italic(true)
				for idx, bl := range bodyLines {
					if renderedCount >= diffHeight {
						break
					}
					var lineStr string
					if idx == 0 {
						lineStr = commentStyle.Render("    💬 @"+note.Author+": ") + bl
					} else {
						lineStr = "        " + bl
					}
					lines = append(lines, lineStr)
					renderedCount++
				}
			}
		}
	}

	// Pad empty lines if we have fewer lines than diffHeight
	for renderedCount < diffHeight {
		lines = append(lines, "")
		renderedCount++
	}

	// Help hint at bottom
	hintsStr := "  " + keyHint("N", "new comment")
	if m.mrDiffLineCursor < len(f.Lines) {
		discs := m.getDiscussionsForLine(f, f.Lines[m.mrDiffLineCursor])
		if len(discs) > 0 {
			hintsStr += "  " + keyHint("r", "reply")
		}
	}
	hintsStr += "  " + keyHint("Tab", "close panel")
	lines = append(lines, "", dimStyle.Render(hintsStr))

	return strings.Join(lines, "\n")
}

func (m Model) viewMRDetail(bodyH int) string {
	content := m.viewMRDetailForWidth(m.width)
	lines := strings.Split(content, "\n")

	totalLines := len(lines)
	maxScroll := totalLines - bodyH
	if maxScroll < 0 {
		maxScroll = 0
	}
	offset := m.mrDetailScrollOffset
	if offset > maxScroll {
		offset = maxScroll
	}
	if offset < 0 {
		offset = 0
	}

	end := offset + bodyH
	if end > totalLines {
		end = totalLines
	}

	return strings.Join(lines[offset:end], "\n")
}

func (m Model) viewMRDetailForWidth(w int) string {
	mr := m.mrDetail
	if mr == nil {
		return ""
	}

	inner := w - 4
	if inner < 4 {
		inner = 4
	}

	title := boldStyle.Width(inner).Render(fmt.Sprintf("!%d  %s", mr.IID, mr.Title))
	status := statusBadge(mr.State)
	meta := lipgloss.JoinHorizontal(lipgloss.Center,
		status,
		"  ",
		dimStyle.Render("Author: "), accentStyle.Render(mr.Author),
		"  ",
		dimStyle.Render("Source: "), dimStyle.Render(mr.SourceBranch),
		" → ",
		dimStyle.Render(mr.TargetBranch),
	)

	var assignees, reviewers string
	if len(mr.Assignees) > 0 {
		assignees = dimStyle.Render("Assignees: ") + strings.Join(mr.Assignees, ", ")
	}
	if len(mr.Reviewers) > 0 {
		reviewers = dimStyle.Render("Reviewers: ") + strings.Join(mr.Reviewers, ", ")
	}

	var labels string
	if len(mr.Labels) > 0 {
		var lb []string
		for _, l := range mr.Labels {
			lb = append(lb, lipgloss.NewStyle().Background(colorBgHover).Foreground(colorTeal).Padding(0, 1).Render(l))
		}
		labels = strings.Join(lb, " ")
	}

	divider := lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", inner))

	desc := mr.Description
	if desc == "" {
		desc = dimStyle.Italic(true).Render("No description provided.")
	} else {
		desc = dimStyle.Width(inner).Render(truncateLines(desc, 20, inner))
	}

	diffBadge := ""
	if len(m.mrDiffFiles) > 0 {
		totalAdded, totalDeleted := 0, 0
		for _, f := range m.mrDiffFiles {
			totalAdded += f.Added
			totalDeleted += f.Deleted
		}
		diffBadge = "  " + successStyle.Render(fmt.Sprintf("+%d", totalAdded)) +
			" " + errorStyle.Render(fmt.Sprintf("-%d", totalDeleted)) +
			" " + dimStyle.Render(fmt.Sprintf("in %d file(s)", len(m.mrDiffFiles)))
	} else {
		diffBadge = "  " + dimStyle.Render("(loading changes...)")
	}

	lines := []string{
		"",
		lipgloss.NewStyle().PaddingLeft(2).Render(title),
		lipgloss.NewStyle().PaddingLeft(2).Render(meta),
	}
	if assignees != "" {
		lines = append(lines, lipgloss.NewStyle().PaddingLeft(2).Render(assignees))
	}
	if reviewers != "" {
		lines = append(lines, lipgloss.NewStyle().PaddingLeft(2).Render(reviewers))
	}
	if labels != "" {
		lines = append(lines, lipgloss.NewStyle().PaddingLeft(2).Render(labels))
	}
	lines = append(lines,
		lipgloss.NewStyle().PaddingLeft(2).Render(dimStyle.Render("Updated: "+mr.UpdatedAt+"  Created: "+mr.CreatedAt)),
		lipgloss.NewStyle().PaddingLeft(2).Render(diffBadge),
		"",
		lipgloss.NewStyle().PaddingLeft(2).Render(divider),
		"",
		lipgloss.NewStyle().PaddingLeft(2).Render(desc),
		"",
		lipgloss.NewStyle().PaddingLeft(2).Render(
			dimStyle.Render(fmt.Sprintf("👍 %d  👎 %d  💬 %d", mr.Upvotes, mr.Downvotes, mr.UserNotesCount))),
		"",
		lipgloss.NewStyle().PaddingLeft(2).Render(
			lipgloss.NewStyle().Foreground(colorInfo).Render("🔗 "+mr.WebURL)),
	)

	// Append discussions/comments
	if len(m.mrDiscussions) > 0 {
		lines = append(lines,
			"",
			lipgloss.NewStyle().PaddingLeft(2).Render(subtitleStyle.Render("💬 Discussions & Comments")),
			lipgloss.NewStyle().PaddingLeft(2).Render(divider),
		)

		for _, d := range m.mrDiscussions {
			if len(d.Notes) == 0 {
				continue
			}

			for noteIdx, note := range d.Notes {
				var noteContent string
				if note.System {
					noteContent = dimStyle.Italic(true).Render(fmt.Sprintf("• %s (%s)", note.Body, note.CreatedAt))
				} else {
					var threadHeader string
					if noteIdx == 0 {
						if note.Position != nil {
							fileInfo := note.Position.NewPath
							if fileInfo == "" {
								fileInfo = note.Position.OldPath
							}
							lineNum := note.Position.NewLine
							if lineNum == 0 {
								lineNum = note.Position.OldLine
							}
							threadHeader = accentStyle.Render(fmt.Sprintf("Thread on %s:L%d", fileInfo, lineNum))
						} else {
							threadHeader = accentStyle.Render("General Thread")
						}
					}

					authorPart := boldStyle.Render("@" + note.Author)
					timePart := dimStyle.Render(" on " + note.CreatedAt)

					bodyStyle := lipgloss.NewStyle().Foreground(colorText).Width(inner - 4)
					bodyWrapped := bodyStyle.Render(note.Body)

					indent := "  "
					if noteIdx > 0 {
						indent = "    "
					}

					var blockLines []string
					if threadHeader != "" {
						blockLines = append(blockLines, indent+threadHeader)
					}
					blockLines = append(blockLines, indent+authorPart+timePart)

					for _, bLine := range strings.Split(bodyWrapped, "\n") {
						blockLines = append(blockLines, indent+"  "+bLine)
					}

					noteContent = strings.Join(blockLines, "\n")
				}

				lines = append(lines, lipgloss.NewStyle().PaddingLeft(2).Render(noteContent), "")
			}
			lines = append(lines, lipgloss.NewStyle().PaddingLeft(2).Render(divider))
		}
	} else {
		lines = append(lines,
			"",
			lipgloss.NewStyle().PaddingLeft(2).Render(subtitleStyle.Render("💬 Discussions & Comments")),
			lipgloss.NewStyle().PaddingLeft(2).Render(divider),
			lipgloss.NewStyle().PaddingLeft(2).Render(dimStyle.Italic(true).Render("No comments yet or loading...")),
		)
	}

	return strings.Join(lines, "\n")
}

func (m Model) viewPipelineDetail(bodyH int) string {
	p := m.pipelineDetail
	if p == nil {
		return ""
	}

	leftW := m.width * 2 / 5
	if leftW < 30 {
		leftW = 30
	}
	rightW := m.width - leftW - 1 // -1 for separator

	// ─── Left Panel: Pipeline Info & Job List ───
	var leftLines []string

	// Pipeline Metadata
	leftLines = append(leftLines, "")
	leftLines = append(leftLines, boldStyle.Render(fmt.Sprintf("Pipeline #%d", p.ID)))
	leftLines = append(leftLines, lipgloss.JoinHorizontal(lipgloss.Center,
		statusBadge(p.Status), "  ",
		dimStyle.Render("Ref: "), accentStyle.Render(p.Ref),
	))
	leftLines = append(leftLines, dimStyle.Render("Source: ")+p.Source)
	leftLines = append(leftLines, dimStyle.Render("User: ")+p.User)
	leftLines = append(leftLines, dimStyle.Render("Created: ")+p.CreatedAt)
	leftLines = append(leftLines, dimStyle.Render("Updated: ")+p.UpdatedAt)
	leftLines = append(leftLines, lipgloss.NewStyle().Foreground(colorInfo).Render("🔗 "+truncate(p.WebURL, leftW-2)))

	leftLines = append(leftLines, lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", leftW-2)))
	leftLines = append(leftLines, subtitleStyle.Render(" Jobs"))

	metaHeight := len(leftLines)
	jobsH := bodyH - metaHeight - 1
	if jobsH < 2 {
		jobsH = 2
	}

	if m.pipelineJobs == nil {
		leftLines = append(leftLines, "", dimStyle.Italic(true).Render(" Loading jobs..."))
	} else if len(m.pipelineJobs) == 0 {
		leftLines = append(leftLines, "", dimStyle.Italic(true).Render(" No jobs found."))
	} else {
		// Calculate job scroll offset
		startJobIdx := m.jobCursor - jobsH/2
		if startJobIdx < 0 {
			startJobIdx = 0
		}
		if startJobIdx+jobsH > len(m.pipelineJobs) {
			startJobIdx = len(m.pipelineJobs) - jobsH
		}
		if startJobIdx < 0 {
			startJobIdx = 0
		}

		for i := 0; i < jobsH; i++ {
			idx := startJobIdx + i
			if idx >= len(m.pipelineJobs) {
				break
			}
			job := m.pipelineJobs[idx]
			selected := idx == m.jobCursor

			// Format row
			statusStr := statusBadge(job.Status)
			nameStr := truncate(job.Name, leftW-16) // truncate so it fits

			rowText := fmt.Sprintf("%s %s", statusStr, nameStr)
			if selected {
				leftLines = append(leftLines, selectedStyle.Width(leftW-2).Render("▶ "+rowText))
			} else {
				leftLines = append(leftLines, normalItemStyle.Width(leftW-2).Render("  "+rowText))
			}
		}
	}

	if len(leftLines) > bodyH {
		leftLines = leftLines[:bodyH]
	}
	leftContent := strings.Join(leftLines, "\n")
	leftPanel := lipgloss.NewStyle().Width(leftW).Height(bodyH).MaxHeight(bodyH).Render(leftContent)

	// ─── Separator ───
	sepContent := strings.Repeat("│\n", bodyH)
	if bodyH > 0 {
		sepContent = sepContent[:len(sepContent)-1]
	}
	sep := lipgloss.NewStyle().Foreground(colorBorder).Render(sepContent)

	// ─── Right Panel: Job Trace or Job Details ───
	var rightLines []string

	if m.jobTraceOpen {
		// Show Job Trace
		rightLines = append(rightLines, subtitleStyle.Render(fmt.Sprintf("  Trace Log: %s", m.jobTraceJob.Name)))
		rightLines = append(rightLines, dimStyle.Render("  (Esc/Enter=Close, j/k=Scroll, g/G=Top/Bottom, ctrl+g=Editor)"))
		rightLines = append(rightLines, lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", rightW-2)))

		traceH := bodyH - len(rightLines)
		if traceH < 1 {
			traceH = 1
		}

		traceLines := strings.Split(m.jobTrace, "\n")

		maxOffset := len(traceLines) - traceH
		if maxOffset < 0 {
			maxOffset = 0
		}
		offset := m.jobTraceScrollOffset
		if offset > maxOffset {
			offset = maxOffset
		}
		if offset < 0 {
			offset = 0
		}

		end := offset + traceH
		if end > len(traceLines) {
			end = len(traceLines)
		}

		for _, tl := range traceLines[offset:end] {
			rightLines = append(rightLines, truncate(tl, rightW-2))
		}
	} else {
		// Show Job details
		rightLines = append(rightLines, subtitleStyle.Render("  Job Details"))
		rightLines = append(rightLines, lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", rightW-2)))
		rightLines = append(rightLines, "")

		if m.pipelineJobs == nil {
			rightLines = append(rightLines, dimStyle.Italic(true).Render("  Loading jobs..."))
		} else if len(m.pipelineJobs) == 0 {
			rightLines = append(rightLines, dimStyle.Italic(true).Render("  No jobs to display details for."))
		} else if m.jobCursor < len(m.pipelineJobs) {
			job := m.pipelineJobs[m.jobCursor]

			rightLines = append(rightLines, boldStyle.Render(fmt.Sprintf("  Job: %s (ID: #%d)", job.Name, job.ID)))
			rightLines = append(rightLines, "")
			rightLines = append(rightLines, fmt.Sprintf("  Stage:          %s", job.Stage))
			rightLines = append(rightLines, fmt.Sprintf("  Status:         %s", statusBadge(job.Status)))

			durStr := "n/a"
			if job.Duration > 0 {
				durStr = fmt.Sprintf("%ds", job.Duration)
			}
			rightLines = append(rightLines, fmt.Sprintf("  Duration:       %s", durStr))
			rightLines = append(rightLines, fmt.Sprintf("  Created:        %s", job.CreatedAt))
			rightLines = append(rightLines, fmt.Sprintf("  Started:        %s", job.StartedAt))
			rightLines = append(rightLines, fmt.Sprintf("  Finished:       %s", job.FinishedAt))

			allowFailStr := "No"
			if job.AllowFailure {
				allowFailStr = "Yes"
			}
			rightLines = append(rightLines, fmt.Sprintf("  Allow Failure:  %s", allowFailStr))

			if job.FailureReason != "" {
				rightLines = append(rightLines, "")
				rightLines = append(rightLines, lipgloss.NewStyle().Foreground(colorError).Render(fmt.Sprintf("  Failure Reason: %s", job.FailureReason)))
			}

			rightLines = append(rightLines, "")
			rightLines = append(rightLines, lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", rightW-2)))
			rightLines = append(rightLines, "")
			rightLines = append(rightLines, accentStyle.Render("  Press [Enter] to view trace/logs output"))
			if job.Status == "manual" {
				rightLines = append(rightLines, dimStyle.Render("  Press [r] to play/trigger this manual job"))
			} else {
				rightLines = append(rightLines, dimStyle.Render("  Press [r] to restart/retry this job"))
			}
		}
	}

	if len(rightLines) > bodyH {
		rightLines = rightLines[:bodyH]
	}
	rightContent := strings.Join(rightLines, "\n")
	rightPanel := lipgloss.NewStyle().Width(rightW).Height(bodyH).MaxHeight(bodyH).Render(rightContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, sep, rightPanel)
}

func (m Model) viewIssueDetail() string {
	iss := m.issueDetail
	if iss == nil {
		return ""
	}
	w := m.width - 4

	title := boldStyle.Width(w).Render(fmt.Sprintf("#%d  %s", iss.IID, iss.Title))
	divider := lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", w))

	desc := iss.Description
	if desc == "" {
		desc = dimStyle.Italic(true).Render("No description provided.")
	} else {
		desc = dimStyle.Width(w).Render(truncateLines(desc, 20, w))
	}

	var labels string
	if len(iss.Labels) > 0 {
		var lb []string
		for _, l := range iss.Labels {
			lb = append(lb, lipgloss.NewStyle().Background(colorBgHover).Foreground(colorTeal).Padding(0, 1).Render(l))
		}
		labels = strings.Join(lb, " ")
	}

	lines := []string{
		"",
		lipgloss.NewStyle().PaddingLeft(2).Render(title),
		lipgloss.NewStyle().PaddingLeft(2).Render(lipgloss.JoinHorizontal(lipgloss.Center,
			statusBadge(iss.State), "  ",
			dimStyle.Render("Author: "), accentStyle.Render(iss.Author),
		)),
	}
	if labels != "" {
		lines = append(lines, lipgloss.NewStyle().PaddingLeft(2).Render(labels))
	}
	lines = append(lines,
		lipgloss.NewStyle().PaddingLeft(2).Render(dimStyle.Render("Updated: "+iss.UpdatedAt+"  Created: "+iss.CreatedAt)),
		"",
		lipgloss.NewStyle().PaddingLeft(2).Render(divider),
		"",
		lipgloss.NewStyle().PaddingLeft(2).Render(desc),
		"",
		lipgloss.NewStyle().PaddingLeft(2).Render(
			dimStyle.Render(fmt.Sprintf("👍 %d  👎 %d", iss.Upvotes, iss.Downvotes))),
		"",
		lipgloss.NewStyle().PaddingLeft(2).Render(
			lipgloss.NewStyle().Foreground(colorInfo).Render("🔗 "+iss.WebURL)),
	)
	return strings.Join(lines, "\n")
}

// ─── Server select overlay ────────────────────────────────────────────────────

func (m Model) viewServerSelect() string {
	var rows []string
	rows = append(rows, subtitleStyle.Render("Select GitLab Server"), "")
	for i, srv := range m.cfg.Servers {
		line := fmt.Sprintf("%-20s  %s", srv.Name, dimStyle.Render(srv.URL))
		if i == m.serverCursor {
			rows = append(rows, selectedStyle.Render("▶ "+line))
		} else {
			rows = append(rows, normalItemStyle.Render("  "+line))
		}
	}
	rows = append(rows, "", dimStyle.Render("↑↓ navigate  Enter select  Esc cancel"))

	box := panelStyle.Padding(1, 3).Render(strings.Join(rows, "\n"))
	
	bg := m.viewBackground()
	dlgWidth := lipgloss.Width(box)
	dlgHeight := lipgloss.Height(box)
	targetHeight := m.height - m.getHeightOffset()
	startX := (m.width - dlgWidth) / 2
	startY := (targetHeight - dlgHeight) / 2
	return overlay(bg, box, m.width, targetHeight, startX, startY)
}



// ─── Link select overlay ──────────────────────────────────────────────────────

func (m Model) viewLinkSelect() string {
	var rows []string
	rows = append(rows, subtitleStyle.Render("Open Link"), "")
	for i, item := range m.linkItems {
		line := fmt.Sprintf("%s  %s", item.Label, dimStyle.Render(truncate(item.URL, 80)))
		if i == m.linkCursor {
			rows = append(rows, selectedStyle.Render("▶ "+line))
		} else {
			rows = append(rows, normalItemStyle.Render("  "+line))
		}
	}
	rows = append(rows, "", dimStyle.Render("↑↓ navigate  Enter open  Esc cancel"))

	box := panelStyle.Padding(1, 3).Render(strings.Join(rows, "\n"))

	bg := m.viewBackground()
	dlgWidth := lipgloss.Width(box)
	dlgHeight := lipgloss.Height(box)
	targetHeight := m.height - m.getHeightOffset()
	startX := (m.width - dlgWidth) / 2
	startY := (targetHeight - dlgHeight) / 2
	return overlay(bg, box, m.width, targetHeight, startX, startY)
}

// ─── Comment composer overlay ─────────────────────────────────────────────────

func (m Model) viewCommentComposer() string {
	var title, hint string
	if m.commentMode == commentModeInline {
		l := m.commentInlineLine
		f := m.commentInlineFile
		fileName := f.NewPath
		lineInfo := ""
		if l.NewLine > 0 {
			lineInfo = fmt.Sprintf("line %d", l.NewLine)
		} else if l.OldLine > 0 {
			lineInfo = fmt.Sprintf("old line %d", l.OldLine)
		}
		snippet := l.Content
		if len(snippet) > 60 {
			snippet = snippet[:60] + "…"
		}
		title = subtitleStyle.Render("Inline Comment") + "  " +
			dimStyle.Render(fileName+" "+lineInfo)
		hint = lipgloss.NewStyle().Foreground(colorSuccess).Italic(true).Render(snippet)
	} else {
		title = subtitleStyle.Render("MR Comment")
		hint = dimStyle.Render(fmt.Sprintf("Commenting on MR !%d", m.mrDetail.IID))
	}

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Background(colorBgPanel).
		Padding(0, 1).
		Width(60).
		Render(m.commentInput.View())

	box := panelStyle.Padding(1, 3).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			title,
			hint,
			"",
			inputBox,
			"",
			dimStyle.Render(keyHint("Enter", "submit")+"  "+keyHint("Alt+Enter", "new line")+"  "+keyHint("Esc", "cancel")),
		),
	)

	bg := m.viewBackground()
	dlgWidth := lipgloss.Width(box)
	dlgHeight := lipgloss.Height(box)
	targetHeight := m.height - m.getHeightOffset()
	startX := (m.width - dlgWidth) / 2
	startY := (targetHeight - dlgHeight) / 2
	return overlay(bg, box, m.width, targetHeight, startX, startY)
}

// ─── Confirm dialog ───────────────────────────────────────────────────────────

func (m Model) viewConfirm() string {
	if m.confirm == nil {
		return ""
	}
	box := panelStyle.Padding(2, 4).Render(lipgloss.JoinVertical(lipgloss.Center,
		warningStyle.Render("⚠  Confirm"),
		"",
		dimStyle.Render(m.confirm.label),
		"",
		lipgloss.JoinHorizontal(lipgloss.Center,
			successStyle.Render("[y] Yes"),
			"   ",
			errorStyle.Render("[n] No"),
		),
	))

	bg := m.viewBackground()
	dlgWidth := lipgloss.Width(box)
	dlgHeight := lipgloss.Height(box)
	targetHeight := m.height - m.getHeightOffset()
	startX := (m.width - dlgWidth) / 2
	startY := (targetHeight - dlgHeight) / 2
	return overlay(bg, box, m.width, targetHeight, startX, startY)
}

// ─── Footer / help bar ────────────────────────────────────────────────────────

func (m Model) viewFooter() string {
	hints := []string{
		keyHint("Tab/1-4", "tabs"),
		keyHint("↑↓", "navigate"),
		keyHint("Enter", "open"),
		keyHint("r", "refresh"),
		keyHint("S", "server"),
		keyHint("q", "quit"),
	}

	if m.tab == tabMRs {
		hints = append(hints, keyHint("s", "state:"+string(m.mrState)))
	}

	if m.tab == tabProjects {
		if m.projectTotalPage > 1 {
			hints = append(hints, keyHint("PgUp/PgDn", "pages"))
		}
	} else {
		if m.mrTotalPage > 1 || m.pipelineTotalPage > 1 || m.issueTotalPage > 1 {
			hints = append(hints, keyHint("n/p", "pages"))
		}
	}

	bar := strings.Join(hints, "  ")
	div := lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", m.width))

	pageInfo := ""
	switch m.tab {
	case tabMRs:
		if m.mrTotalPage > 0 {
			pageInfo = dimStyle.Render(fmt.Sprintf("  Page %d/%d", m.mrPage, m.mrTotalPage))
		}
	case tabPipelines:
		if m.pipelineTotalPage > 0 {
			pageInfo = dimStyle.Render(fmt.Sprintf("  Page %d/%d", m.pipelinePage, m.pipelineTotalPage))
		}
	case tabIssues:
		if m.issueTotalPage > 0 {
			pageInfo = dimStyle.Render(fmt.Sprintf("  Page %d/%d", m.issuePage, m.issueTotalPage))
		}
	case tabProjects:
		if m.projectTotalPage > 0 {
			pageInfo = dimStyle.Render(fmt.Sprintf("  Page %d/%d", m.projectPage, m.projectTotalPage))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		div,
		lipgloss.NewStyle().PaddingLeft(1).Render(bar+pageInfo),
	)
}

func (m Model) viewDetailFooter() string {
	var hints []string
	switch m.tab {
	case tabMRs:
		if m.mrDiffPanelOpen {
			hints = []string{
				keyHint("j/k", "scroll lines"),
				keyHint("J/K", "prev/next hunk"),
				keyHint("n/p", "prev/next file"),
				keyHint("N", "inline comment"),
				keyHint("Tab", "close diff"),
				keyHint("Esc", "close diff"),
				keyHint("q", "quit"),
			}
		} else {
			hints = []string{
				keyHint("j/k", "scroll"),
				keyHint("Tab", "changes"),
				keyHint("C", "comment"),
				keyHint("a", "approve"),
				keyHint("m", "merge"),
				keyHint("x", "close"),
				keyHint("+", "vote up"),
				keyHint("-", "vote down"),
				keyHint("o", "open link"),
				keyHint("Esc", "back"),
				keyHint("q", "quit"),
			}
		}
	case tabPipelines:
		if m.jobTraceOpen {
			hints = []string{
				keyHint("j/k", "scroll trace"),
				keyHint("Esc/Enter", "close trace"),
				keyHint("q", "quit"),
			}
		} else {
			hints = []string{
				keyHint("j/k", "select job"),
				keyHint("Enter", "view trace"),
				keyHint("r", "retry job"),
				keyHint("R", "retry pipeline"),
				keyHint("c", "cancel pipeline"),
				keyHint("o", "open link"),
				keyHint("Esc", "back"),
				keyHint("q", "quit"),
			}
		}
	case tabIssues:
		hints = []string{
			keyHint("o", "open link"),
			keyHint("Esc", "back"),
			keyHint("q", "quit"),
		}
	default:
		hints = []string{
			keyHint("Esc", "back"),
			keyHint("q", "quit"),
		}
	}

	bar := strings.Join(hints, "  ")
	div := lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", m.width))
	return lipgloss.JoinVertical(lipgloss.Left, div, lipgloss.NewStyle().PaddingLeft(1).Render(bar))
}

func tickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func isPipelineOrJobsActive(pipeline *gitlab.PipelineInfo, jobs []*gitlab.JobInfo) bool {
	if pipeline == nil {
		return false
	}
	if isStatusActive(pipeline.Status) {
		return true
	}
	for _, j := range jobs {
		if isStatusActive(j.Status) {
			return true
		}
	}
	return false
}

func isStatusActive(status string) bool {
	switch status {
	case "running", "pending", "created", "waiting_for_resource", "preparing":
		return true
	default:
		return false
	}
}

func (m Model) getBodyHeight() int {
	header := lipgloss.NewStyle().Width(m.width).MaxHeight(1).Render(m.viewHeader())
	footer := lipgloss.NewStyle().Width(m.width).Render(m.viewDetailFooter())
	bodyH := m.height - lipgloss.Height(header) - lipgloss.Height(footer) - m.getHeightOffset()
	if bodyH < 1 {
		return 1
	}
	return bodyH
}

func (m Model) cmdOpenTraceInEditor() tea.Cmd {
	tmpFile, err := os.CreateTemp(".", "job-trace-*.log")
	if err != nil {
		return func() tea.Msg { return errMsg{err} }
	}
	if _, err := tmpFile.WriteString(m.jobTrace); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return func() tea.Msg { return errMsg{err} }
	}
	tmpFile.Close()

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	c := exec.Command(editor, tmpFile.Name())
	return tea.ExecProcess(c, func(err error) tea.Msg {
		os.Remove(tmpFile.Name())
		return youtrackTuiFinishedMsg{Err: err}
	})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

type youtrackTuiFinishedMsg struct {
	Err error
}

// openURL opens a URL in the user's preferred browser or returns a command to execute
// an interactive YouTrack command.
func (m Model) openURL(url string) tea.Cmd {
	if m.cfg.YouTrackCommand != "" && m.cfg.IsYouTrackURL(url) {
		c := exec.Command(m.cfg.YouTrackCommand, url)
		return tea.ExecProcess(c, func(err error) tea.Msg {
			return youtrackTuiFinishedMsg{Err: err}
		})
	}

	cmd := m.cfg.BrowserCommand
	if cmd == "" {
		cmd = "xdg-open"
	}
	exec.Command(cmd, url).Start()
	return nil
}

// urlRe matches http:// and https:// URLs.
var urlRe = regexp.MustCompile(`https?://[^\s<>"']+`)

// youTrackRe matches project keys (like MTEL-22122) in text.
var youTrackRe = regexp.MustCompile(`\b[a-zA-Z0-9]+-\d+\b`)

type youTrackLink struct {
	Key string
	URL string
}

// extractYouTrackLinks parses project keys and builds YouTrack URLs based on configuration.
func extractYouTrackLinks(text string, cfg *config.Config) []youTrackLink {
	if cfg == nil {
		return nil
	}
	seen := map[string]bool{}
	var links []youTrackLink
	for _, m := range youTrackRe.FindAllString(text, -1) {
		if u, ok := cfg.GetYouTrackURL(m); ok {
			if !seen[u] {
				seen[u] = true
				links = append(links, youTrackLink{Key: strings.ToUpper(m), URL: u})
			}
		}
	}
	return links
}

// extractURLs finds all unique URLs in a block of text.
func extractURLs(text string) []string {
	seen := map[string]bool{}
	var urls []string
	for _, m := range urlRe.FindAllString(text, -1) {
		// Strip trailing punctuation that's likely not part of the URL
		m = strings.TrimRight(m, ".,;:!?)]}>")
		if !seen[m] {
			seen[m] = true
			urls = append(urls, m)
		}
	}
	return urls
}

// collectLinksForDetail returns the linkItems for the current detail view.
func (m Model) collectLinksForDetail() []linkItem {
	var items []linkItem
	seen := map[string]bool{}

	add := func(label, rawURL string) {
		u := strings.TrimRight(rawURL, ".,;:!?)]}>")
		if seen[u] {
			return
		}
		seen[u] = true
		items = append(items, linkItem{Label: label, URL: u})
	}

	switch m.tab {
	case tabMRs:
		if m.mrDetail == nil {
			return nil
		}
		add("🔗 MR on GitLab", m.mrDetail.WebURL)
		for _, u := range extractURLs(m.mrDetail.Description) {
			add("📎 "+u, u)
		}
		for _, y := range extractYouTrackLinks(m.mrDetail.Description, m.cfg) {
			add("🎫 "+y.Key, y.URL)
		}
		for _, d := range m.mrDiscussions {
			for _, n := range d.Notes {
				for _, u := range extractURLs(n.Body) {
					add("💬 "+u, u)
				}
				for _, y := range extractYouTrackLinks(n.Body, m.cfg) {
					add("🎫 "+y.Key, y.URL)
				}
			}
		}
	case tabPipelines:
		if m.pipelineDetail == nil {
			return nil
		}
		add("🔗 Pipeline on GitLab", m.pipelineDetail.WebURL)
	case tabIssues:
		if m.issueDetail == nil {
			return nil
		}
		add("🔗 Issue on GitLab", m.issueDetail.WebURL)
		for _, u := range extractURLs(m.issueDetail.Description) {
			add("📎 "+u, u)
		}
		for _, y := range extractYouTrackLinks(m.issueDetail.Description, m.cfg) {
			add("🎫 "+y.Key, y.URL)
		}
	}

	return items
}

func truncateLines(s string, maxLines, maxWidth int) string {
	lines := strings.Split(s, "\n")
	var result []string
	for i, l := range lines {
		if i >= maxLines {
			result = append(result, dimStyle.Render("… (truncated)"))
			break
		}
		if len(l) > maxWidth {
			l = l[:maxWidth-1] + "…"
		}
		result = append(result, l)
	}
	return strings.Join(result, "\n")
}
