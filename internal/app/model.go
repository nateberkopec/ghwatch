package app

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/nateberkopec/ghwatch/internal/githubclient"
	"github.com/nateberkopec/ghwatch/internal/githuburl"
	"github.com/nateberkopec/ghwatch/internal/persistence"
	"github.com/nateberkopec/ghwatch/internal/watch"
)

// githubAPI captures the subset of client functionality the model needs. This
// makes it easy to stub in tests without reaching GitHub.
type githubAPI interface {
	WorkflowRunByID(ctx context.Context, owner, repo string, runID int64) (githubclient.WorkflowRun, error)
	RunsByPullRequest(ctx context.Context, owner, repo string, number int) ([]githubclient.WorkflowRun, error)
	RunsByCommit(ctx context.Context, owner, repo, sha string) ([]githubclient.WorkflowRun, error)
}

type focusArea int

const (
	focusRuns focusArea = iota
	focusInput
)

type statusKind int

const (
	statusNeutral statusKind = iota
	statusError
	statusSuccess
)

type statusMessage struct {
	text    string
	kind    statusKind
	expires time.Time
}

type area struct {
	top    int
	height int
}

// Config wires external dependencies for the app.
type Config struct {
	Client       githubAPI
	PollInterval time.Duration
	BellEnabled  bool
}

// Model implements the Bubble Tea program.
type Model struct {
	client       githubAPI
	tracker      *watch.Tracker
	pollInterval time.Duration

	focus        focusArea
	showArchived bool
	bellEnabled  bool

	selectedIndex int
	scrollOffset  int
	width         int
	height        int

	input textinput.Model
	spin  spinner.Model

	status       statusMessage
	pendingFetch bool
	refreshing   bool

	listArea  area
	inputArea area

	history      []string
	historyIndex int
	tempInput    string
}

// New creates a Bubble Tea model for the watcher.
func New(cfg Config) *Model {
	var client githubAPI = cfg.Client
	if client == nil {
		client = githubclient.New("")
	}

	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = 10 * time.Second
	}

	ti := textinput.New()
	ti.Placeholder = "Paste a GitHub workflow/run URL"
	ti.Prompt = ""
	ti.CharLimit = 256
	ti.Blur()

	sp := spinner.New(spinner.WithSpinner(spinner.Ellipsis))

	tracker := watch.NewTracker()
	if err := persistence.LoadTracker(tracker); err != nil {
	}

	history, err := persistence.LoadHistory()
	if err != nil {
		history = []string{}
	}

	return &Model{
		client:       client,
		tracker:      tracker,
		pollInterval: pollInterval,
		bellEnabled:  cfg.BellEnabled,
		input:        ti,
		spin:         sp,
		history:      history,
		historyIndex: len(history),
	}
}

// Init satisfies the tea.Model interface.
func (m *Model) Init() tea.Cmd {
	spinCmd := func() tea.Msg { return m.spin.Tick() }
	return tea.Batch(textinput.Blink, m.scheduleRefresh(), spinCmd)
}

// Update drives the Bubble Tea state machine.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.maybeExpireStatus()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.configureLayout()
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	case fetchResultMsg:
		m.pendingFetch = false
		cmd := m.absorbRuns(msg.Runs, msg.Source)
		return m, cmd
	case fetchErrMsg:
		m.pendingFetch = false
		m.setStatus(msg.Err.Error(), statusError)
	case openErrMsg:
		m.setStatus(msg.Err.Error(), statusError)
	case refreshTickMsg:
		cmds := []tea.Cmd{m.scheduleRefresh()}
		if refreshCmd := m.refreshCmd(true); refreshCmd != nil {
			cmds = append(cmds, refreshCmd)
		}
		return m, tea.Batch(cmds...)
	case refreshResultMsg:
		m.refreshing = false
		if msg.Err != nil {
			m.setStatus(msg.Err.Error(), statusError)
		}
		// Absorb individual run refreshes (preserve existing sources)
		cmd := m.absorbRuns(msg.Runs, githuburl.Parsed{})
		// Absorb PR runs with their respective sources (for new runs)
		for prSource, runs := range msg.PRRuns {
			prCmd := m.absorbRuns(runs, prSource)
			if prCmd != nil {
				cmd = tea.Batch(cmd, prCmd)
			}
		}
		return m, cmd
	}

	if m.focus == focusInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the TUI.
func (m *Model) View() string {
	return renderView(m)
}

func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.MouseLeft:
		if m.listArea.contains(msg.Y) {
			row := msg.Y - m.listArea.top
			if row <= 0 {
				return m, nil
			}
			index := m.scrollOffset + row - 1
			if index >= 0 && index < len(m.tracker.VisibleRuns(m.showArchived)) {
				m.selectedIndex = index
				m.setFocus(focusRuns)
				m.ensureSelectionBounds()
			}
		} else if m.inputArea.contains(msg.Y) {
			m.setFocus(focusInput)
		}
	case tea.MouseWheelUp:
		m.moveSelection(-1)
	case tea.MouseWheelDown:
		m.moveSelection(1)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "ctrl+c", "ctrl+d", "q":
		persistence.SaveTracker(m.tracker)
		persistence.SaveHistory(m.history)
		return m, tea.Quit
	case "tab", "shift+tab":
		m.toggleFocus()
		if m.focus == focusInput {
			return m, nil
		}
	case "esc":
		m.setFocus(focusRuns)
	}

	if m.focus == focusInput {
		switch key {
		case "enter":
			return m.submitURL()
		case "up":
			m.navigateHistoryUp()
			return m, nil
		case "down":
			m.navigateHistoryDown()
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch key {
	case "j", "down":
		m.moveSelection(1)
	case "k", "up":
		m.moveSelection(-1)
	case "pgdown", "ctrl+f":
		m.moveSelection(m.dataRows())
	case "pgup", "ctrl+b":
		m.moveSelection(-m.dataRows())
	case "g", "home":
		m.selectedIndex = 0
		m.scrollOffset = 0
	case "G", "end":
		m.selectedIndex = len(m.tracker.VisibleRuns(m.showArchived)) - 1
		if m.selectedIndex < 0 {
			m.selectedIndex = 0
		}
	case "o", "enter":
		return m, m.openSelected()
	case "a":
		if m.showArchived {
			if cmd := m.unarchiveSelected(); cmd != nil {
				return m, cmd
			}
		} else {
			m.archiveSelected()
		}
	case "A":
		m.showArchived = !m.showArchived
		m.selectedIndex = 0
		m.scrollOffset = 0
		if m.showArchived {
			m.setStatus("Viewing archived runs", statusNeutral)
		} else {
			m.setStatus("Viewing active runs", statusNeutral)
		}
	case "b":
		m.bellEnabled = !m.bellEnabled
		if m.bellEnabled {
			m.setStatus("Bell enabled", statusSuccess)
		} else {
			m.setStatus("Bell muted", statusNeutral)
		}
	}

	return m, nil
}

func (m *Model) submitURL() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())
	if value == "" {
		m.setStatus("Enter a GitHub Actions, PR, or commit URL", statusNeutral)
		return m, nil
	}

	parsed, err := githuburl.Parse(value)
	if err != nil {
		m.setStatus(err.Error(), statusError)
		return m, nil
	}

	// Add to history (avoid duplicates of the most recent command)
	if len(m.history) == 0 || m.history[len(m.history)-1] != value {
		m.history = append(m.history, value)
	}
	m.historyIndex = len(m.history)
	m.tempInput = ""

	m.input.SetValue("")
	m.pendingFetch = true
	m.setStatus(fmt.Sprintf("Watching %s …", parsed.String()), statusNeutral)
	return m, fetchRunsCmd(m.client, parsed)
}

func (m *Model) archiveSelected() {
	run := m.selectedRun()
	if run == nil {
		return
	}
	m.tracker.Archive(run.Run.ID)
	m.ensureSelectionBounds()
	persistence.SaveTracker(m.tracker)
	m.setStatus(fmt.Sprintf("Archived %s", runLabel(run.Run)), statusNeutral)
}

func (m *Model) unarchiveSelected() tea.Cmd {
	run := m.selectedRun()
	if run == nil {
		return nil
	}
	if ok := m.tracker.Unarchive(run.Run.ID); ok {
		m.showArchived = false
		persistence.SaveTracker(m.tracker)
		m.setStatus(fmt.Sprintf("Restored %s", runLabel(run.Run)), statusSuccess)
		return m.refreshCmd(false)
	}
	return nil
}

func (m *Model) openSelected() tea.Cmd {
	run := m.selectedRun()
	if run == nil {
		return nil
	}
	target := run.Run.PRURL
	if target == "" {
		target = run.Run.HTMLURL
	}
	m.setStatus(fmt.Sprintf("Opening %s", target), statusNeutral)
	return openURLCmd(target)
}

func (m *Model) selectedRun() *watch.TrackedRun {
	runs := m.tracker.VisibleRuns(m.showArchived)
	if len(runs) == 0 {
		return nil
	}
	if m.selectedIndex < 0 {
		m.selectedIndex = 0
	}
	if m.selectedIndex >= len(runs) {
		m.selectedIndex = len(runs) - 1
	}
	return runs[m.selectedIndex]
}

func (m *Model) moveSelection(delta int) {
	runs := m.tracker.VisibleRuns(m.showArchived)
	if len(runs) == 0 {
		m.selectedIndex = 0
		m.scrollOffset = 0
		return
	}
	m.selectedIndex += delta
	if m.selectedIndex < 0 {
		m.selectedIndex = 0
	}
	if m.selectedIndex >= len(runs) {
		m.selectedIndex = len(runs) - 1
	}
	m.ensureSelectionBounds()
}

func (m *Model) ensureSelectionBounds() {
	dataRows := m.dataRows()
	if dataRows <= 0 {
		return
	}
	if m.selectedIndex < m.scrollOffset {
		m.scrollOffset = m.selectedIndex
	}
	if m.selectedIndex >= m.scrollOffset+dataRows {
		m.scrollOffset = m.selectedIndex - dataRows + 1
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	maxScroll := len(m.tracker.VisibleRuns(m.showArchived)) - dataRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}
}

func (m *Model) dataRows() int {
	rows := m.listArea.height - 1 // header consumes one row
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m *Model) toggleFocus() {
	if m.focus == focusRuns {
		m.setFocus(focusInput)
	} else {
		m.setFocus(focusRuns)
	}
}

func (m *Model) setFocus(area focusArea) {
	if m.focus == area {
		return
	}
	m.focus = area
	if area == focusInput {
		m.input.Focus()
	} else {
		m.input.Blur()
	}
}

func (m *Model) navigateHistoryUp() {
	if len(m.history) == 0 {
		return
	}

	// Save current input if we're at the bottom
	if m.historyIndex == len(m.history) {
		m.tempInput = m.input.Value()
	}

	// Navigate up in history
	if m.historyIndex > 0 {
		m.historyIndex--
		m.input.SetValue(m.history[m.historyIndex])
		m.input.CursorEnd()
	}
}

func (m *Model) navigateHistoryDown() {
	if len(m.history) == 0 {
		return
	}

	// Navigate down in history
	if m.historyIndex < len(m.history) {
		m.historyIndex++
		if m.historyIndex == len(m.history) {
			// Back to current input
			m.input.SetValue(m.tempInput)
		} else {
			m.input.SetValue(m.history[m.historyIndex])
		}
		m.input.CursorEnd()
	}
}

func (m *Model) absorbRuns(runs []githubclient.WorkflowRun, source githuburl.Parsed) tea.Cmd {
	if len(runs) == 0 {
		label := "No workflow runs found"
		if source.Kind != githuburl.KindUnknown {
			label = fmt.Sprintf("No workflow runs found for %s", source.String())
		}
		m.setStatus(label, statusNeutral)
		return nil
	}
	shouldRing := false
	added := false
	for _, run := range runs {
		isNew, changed := m.tracker.Upsert(run, source)
		if isNew {
			added = true
		}
		if changed {
			shouldRing = true
		}
	}
	if added {
		m.selectedIndex = 0
		m.scrollOffset = 0
		persistence.SaveTracker(m.tracker)
		m.setStatus(fmt.Sprintf("Watching %d run(s)", len(runs)), statusSuccess)
	}
	if shouldRing && m.bellEnabled {
		return tea.Printf("\a")
	}
	return nil
}

func (m *Model) configureLayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	const (
		titleHeight  = 1
		statusHeight = 2
		inputHeight  = 3
	)
	listHeight := m.height - (titleHeight + statusHeight + inputHeight)
	if listHeight < 5 {
		listHeight = 5
	}
	m.listArea = area{
		top:    titleHeight,
		height: listHeight,
	}
	m.inputArea = area{
		top:    m.height - inputHeight,
		height: inputHeight,
	}
	m.input.Width = max(10, m.width-2)
}

func (m *Model) scheduleRefresh() tea.Cmd {
	if m.pollInterval <= 0 {
		return nil
	}
	return tea.Tick(m.pollInterval, func(time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

func (m *Model) refreshCmd(auto bool) tea.Cmd {
	active := m.tracker.VisibleRuns(false)
	if len(active) == 0 {
		if auto {
			m.refreshing = false
		}
		return nil
	}
	inputs := make([]refreshInput, 0, len(active))

	// Collect unique PR sources to re-fetch for new workflow runs
	prSources := make(map[string]githuburl.Parsed)
	for _, run := range active {
		owner, repo := splitRepo(run.Run.RepoFullName)
		if owner == "" {
			continue
		}
		inputs = append(inputs, refreshInput{
			RunID: run.Run.ID, Owner: owner, Repo: repo,
		})

		// Track PR sources to check for new runs on those PRs
		if run.Source.Kind == githuburl.KindPullRequest {
			key := fmt.Sprintf("%s/%s/%d", run.Source.Owner, run.Source.Repo, run.Source.PRNumber)
			if _, exists := prSources[key]; !exists {
				prSources[key] = run.Source
			}
		}
	}

	if len(inputs) == 0 {
		if auto {
			m.refreshing = false
		}
		return nil
	}
	if auto {
		m.refreshing = true
	}
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		refreshed := make([]githubclient.WorkflowRun, 0, len(inputs))
		prRuns := make(map[githuburl.Parsed][]githubclient.WorkflowRun)
		var errs []string

		// Refresh individual workflow runs by ID
		for _, target := range inputs {
			run, err := client.WorkflowRunByID(ctx, target.Owner, target.Repo, target.RunID)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s/%s #%d: %v", target.Owner, target.Repo, target.RunID, err))
				continue
			}
			refreshed = append(refreshed, run)
		}

		// Re-fetch PR runs to catch new workflow runs on watched PRs
		for _, prSource := range prSources {
			runs, err := client.RunsByPullRequest(ctx, prSource.Owner, prSource.Repo, prSource.PRNumber)
			if err != nil {
				errs = append(errs, fmt.Sprintf("PR %s/%s #%d: %v", prSource.Owner, prSource.Repo, prSource.PRNumber, err))
				continue
			}
			prRuns[prSource] = runs
		}

		var err error
		if len(errs) > 0 {
			err = errors.New(strings.Join(errs, "; "))
		}
		return refreshResultMsg{Runs: refreshed, PRRuns: prRuns, Err: err}
	}
}

func (m *Model) setStatus(text string, kind statusKind) {
	if text == "" {
		m.status = statusMessage{}
		return
	}
	m.status = statusMessage{
		text:    text,
		kind:    kind,
		expires: time.Now().Add(10 * time.Second),
	}
}

func (m *Model) maybeExpireStatus() {
	if m.status.text == "" {
		return
	}
	if time.Now().After(m.status.expires) {
		m.status = statusMessage{}
	}
}

func (a area) contains(y int) bool {
	return y >= a.top && y < a.top+a.height
}

type refreshTickMsg struct{}

type refreshResultMsg struct {
	Runs   []githubclient.WorkflowRun
	PRRuns map[githuburl.Parsed][]githubclient.WorkflowRun // Runs fetched from PR sources
	Err    error
}

type fetchResultMsg struct {
	Runs   []githubclient.WorkflowRun
	Source githuburl.Parsed
}

type fetchErrMsg struct {
	Err error
}

type refreshInput struct {
	RunID int64
	Owner string
	Repo  string
}

type openErrMsg struct {
	Err error
}

func fetchRunsCmd(client githubAPI, parsed githuburl.Parsed) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var (
			runs []githubclient.WorkflowRun
			err  error
		)
		switch parsed.Kind {
		case githuburl.KindWorkflowRun:
			run, runErr := client.WorkflowRunByID(ctx, parsed.Owner, parsed.Repo, parsed.RunID)
			if runErr != nil {
				err = runErr
			} else {
				runs = []githubclient.WorkflowRun{run}
			}
		case githuburl.KindPullRequest:
			runs, err = client.RunsByPullRequest(ctx, parsed.Owner, parsed.Repo, parsed.PRNumber)
		case githuburl.KindCommit:
			runs, err = client.RunsByCommit(ctx, parsed.Owner, parsed.Repo, parsed.SHA)
		default:
			err = fmt.Errorf("unsupported GitHub URL")
		}
		if err != nil {
			return fetchErrMsg{Err: err}
		}
		return fetchResultMsg{Runs: runs, Source: parsed}
	}
}

func openURLCmd(target string) tea.Cmd {
	return func() tea.Msg {
		name, args := openCommand(target)
		if name == "" {
			return openErrMsg{Err: fmt.Errorf("opening URLs is not supported on this platform")}
		}
		cmd := exec.Command(name, args...)
		if err := cmd.Start(); err != nil {
			return openErrMsg{Err: err}
		}
		return nil
	}
}

func openCommand(target string) (string, []string) {
	switch runtime.GOOS {
	case "darwin":
		return "open", []string{target}
	case "linux":
		return "xdg-open", []string{target}
	default:
		return "", nil
	}
}

func splitRepo(full string) (string, string) {
	parts := strings.Split(full, "/")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func runLabel(run githubclient.WorkflowRun) string {
	if run.Target != "" {
		return fmt.Sprintf("%s • %s", run.RepoFullName, run.Target)
	}
	return run.RepoFullName
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
