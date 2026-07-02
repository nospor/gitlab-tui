package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gitlab-tui/internal/config"
	"gitlab-tui/internal/gitlab"
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
	"  Merge Requests",
	"  Pipelines",
	"  Issues",
	"  Projects",
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
)

// ─── Messages ─────────────────────────────────────────────────────────────────

type (
	errMsg          struct{ err error }
	mrLoadedMsg     struct {
		items      []*gitlab.MRInfo
		totalPages int
	}
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
	actionDoneMsg struct{ msg string }
	whoAmIMsg     struct{ username string }
)

// ─── Confirmation action ──────────────────────────────────────────────────────

type confirmAction struct {
	label   string
	perform tea.Cmd
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

	// Pipeline view
	pipelines       []*gitlab.PipelineInfo
	pipelinePage    int
	pipelineTotalPage int
	pipelineCursor  int
	pipelineDetail  *gitlab.PipelineInfo

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
func New(cfg *config.Config, serverIdx int, client *gitlab.Client, project *gitlab.ProjectInfo, startupWarn string) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorAccent)

	ti := textinput.New()
	ti.Placeholder = "Search projects..."
	ti.CharLimit = 100

	m := Model{
		cfg:         cfg,
		serverIdx:   serverIdx,
		client:      client,
		project:     project,
		startupWarn: startupWarn,
		state:       stateLoading,
		loadMsg:     "Connecting to GitLab...",
		tab:       tabMRs,
		spin:      sp,
		mrState:   gitlab.MRStateOpened,
		mrPage:    1,
		pipelinePage: 1,
		issuePage:    1,
		projectPage:  1,
		projectSearch: ti,
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
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case errMsg:
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
		m.state = stateLoading
		m.loadMsg = "Loading merge requests..."
		return m, m.cmdLoadMRs()

	case mrLoadedMsg:
		m.mrs = msg.items
		m.mrTotalPage = msg.totalPages
		m.mrCursor = 0
		if m.state == stateLoading {
			m.state = stateMain
		}
		return m, nil

	case pipelineLoadedMsg:
		m.pipelines = msg.items
		m.pipelineTotalPage = msg.totalPages
		m.pipelineCursor = 0
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
		return m, m.reloadCurrent()

	case tea.KeyMsg:
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
	if key == "ctrl+c" || (key == "q" && (m.tab != tabProjects || m.state == stateDetail)) {
		return m, tea.Quit
	}

	// Escape / back
	if key == "esc" {
		switch m.state {
		case stateDetail:
			m.state = stateMain
			m.mrDetail = nil
			m.pipelineDetail = nil
			m.issueDetail = nil
		case stateServerSelect:
			m.state = stateMain
		case stateConfirm:
			m.state = m.prevState
			m.confirm = nil
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
	case stateConfirm:
		return m.handleConfirmKey(key)
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
		switch key {
		case "a":
			return m.promptConfirm("Approve MR", fmt.Sprintf("Approve MR !%d: %s?", m.mrDetail.IID, m.mrDetail.Title),
				m.cmdApproveMR(m.mrDetail.IID))
		case "m":
			if m.mrDetail.State == "opened" {
				return m.promptConfirm("Merge MR", fmt.Sprintf("Merge MR !%d: %s?", m.mrDetail.IID, m.mrDetail.Title),
					m.cmdMergeMR(m.mrDetail.IID))
			}
		case "c":
			if m.mrDetail.State == "opened" {
				return m.promptConfirm("Close MR", fmt.Sprintf("Close MR !%d?", m.mrDetail.IID),
					m.cmdCloseMR(m.mrDetail.IID))
			}
		}
	case tabPipelines:
		if m.pipelineDetail == nil {
			return m, nil
		}
		switch key {
		case "r":
			return m.promptConfirm("Retry Pipeline", fmt.Sprintf("Retry pipeline #%d?", m.pipelineDetail.ID),
				m.cmdRetryPipeline(m.pipelineDetail.ID))
		case "c":
			if m.pipelineDetail.Status == "running" || m.pipelineDetail.Status == "pending" {
				return m.promptConfirm("Cancel Pipeline", fmt.Sprintf("Cancel pipeline #%d?", m.pipelineDetail.ID),
					m.cmdCancelPipeline(m.pipelineDetail.ID))
			}
		}
	}
	return m, nil
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
			m.mrDetail = m.mrs[m.mrCursor]
			m.prevState = stateMain
			m.state = stateDetail
		}
	case tabPipelines:
		if m.pipelineCursor < len(m.pipelines) {
			m.pipelineDetail = m.pipelines[m.pipelineCursor]
			m.prevState = stateMain
			m.state = stateDetail
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
	case stateConfirm:
		return m.viewConfirm()
	case stateDetail:
		return m.viewDetail()
	default:
		return m.viewMain()
	}
}

// ─── Loading screen ───────────────────────────────────────────────────────────

func (m Model) viewLoading() string {
	center := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center,
			accentStyle.Render("GitLab TUI"),
			"",
			m.spin.View()+" "+dimStyle.Render(m.loadMsg),
		),
	)
	return baseStyle.Width(m.width).Height(m.height).Render(center)
}

// ─── Error screen ─────────────────────────────────────────────────────────────

func (m Model) viewError() string {
	box := lipgloss.JoinVertical(lipgloss.Center,
		errorStyle.Render("⚠ Error"),
		"",
		dimStyle.Width(m.width-10).Render(m.errText),
		"",
		mutedStyle.Render("Press q to quit, or check your config at ~/.config/gitlab-tui/config.json"),
	)
	center := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	return baseStyle.Width(m.width).Height(m.height).Render(center)
}

// ─── Main view ────────────────────────────────────────────────────────────────

func (m Model) viewMain() string {
	header := m.viewHeader()
	tabs := m.viewTabs()
	body := m.viewBody()
	footer := m.viewFooter()

	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(tabs) - lipgloss.Height(footer) - 2
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	bodyPanel := lipgloss.NewStyle().
		Width(m.width).
		Height(bodyHeight).
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
		dimStyle.Render(serverName+" · "),
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
	header := m.viewHeader()
	footer := m.viewDetailFooter()

	var body string
	switch m.tab {
	case tabMRs:
		body = m.viewMRDetail()
	case tabPipelines:
		body = m.viewPipelineDetail()
	case tabIssues:
		body = m.viewIssueDetail()
	}

	bodyH := m.height - lipgloss.Height(header) - lipgloss.Height(footer) - 1
	if bodyH < 1 {
		bodyH = 1
	}

	bodyPanel := lipgloss.NewStyle().Width(m.width).Height(bodyH).Render(body)
	return lipgloss.JoinVertical(lipgloss.Left, header, bodyPanel, footer)
}

func (m Model) viewMRDetail() string {
	mr := m.mrDetail
	if mr == nil {
		return ""
	}

	w := m.width - 4

	title := boldStyle.Width(w).Render(fmt.Sprintf("!%d  %s", mr.IID, mr.Title))
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

	divider := lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", w))

	desc := mr.Description
	if desc == "" {
		desc = dimStyle.Italic(true).Render("No description provided.")
	} else {
		desc = dimStyle.Width(w).Render(truncateLines(desc, 20, w))
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

	return strings.Join(lines, "\n")
}

func (m Model) viewPipelineDetail() string {
	p := m.pipelineDetail
	if p == nil {
		return ""
	}
	w := m.width - 4

	title := boldStyle.Render(fmt.Sprintf("Pipeline #%d", p.ID))
	divider := lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", w))

	return strings.Join([]string{
		"",
		lipgloss.NewStyle().PaddingLeft(2).Render(title),
		lipgloss.NewStyle().PaddingLeft(2).Render(lipgloss.JoinHorizontal(lipgloss.Center,
			statusBadge(p.Status), "  ",
			dimStyle.Render("Ref: "), accentStyle.Render(p.Ref), "  ",
			dimStyle.Render("Source: "), dimStyle.Render(p.Source),
		)),
		lipgloss.NewStyle().PaddingLeft(2).Render(
			dimStyle.Render("Triggered by: ") + p.User),
		lipgloss.NewStyle().PaddingLeft(2).Render(
			dimStyle.Render("Created: " + p.CreatedAt + "  Updated: " + p.UpdatedAt)),
		"",
		lipgloss.NewStyle().PaddingLeft(2).Render(divider),
		"",
		lipgloss.NewStyle().PaddingLeft(2).Render(
			lipgloss.NewStyle().Foreground(colorInfo).Render("🔗 " + p.WebURL)),
	}, "\n")
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
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
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
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// ─── Footer / help bar ────────────────────────────────────────────────────────

func (m Model) viewFooter() string {
	hints := []string{
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
		hints = []string{
			keyHint("a", "approve"),
			keyHint("m", "merge"),
			keyHint("c", "close"),
			keyHint("Esc", "back"),
			keyHint("q", "quit"),
		}
	case tabPipelines:
		hints = []string{
			keyHint("r", "retry"),
			keyHint("c", "cancel"),
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

// ─── Helpers ──────────────────────────────────────────────────────────────────

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
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
