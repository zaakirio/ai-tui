// PAI Agent Dashboard v0.2.0 — Real-time orchestration & observability TUI
//
// Enhancements over v0.1.0:
//   - PAI Algorithm phase tracking (OBSERVE→THINK→PLAN→BUILD→EXECUTE→VERIFY→LEARN)
//   - Tokens/sec throughput per agent
//   - Visual progress bars with percentage
//   - Task descriptions per agent
//   - 2-second tick for snappy real-time feel
//   - Enhanced detail pane with token stats, phase timeline, ISC pass/fail
//   - Faster tick (2s) for more responsive updates
//
// Dependencies (go.mod):
//   module pai-tui
//   go 1.22
//   require (
//     github.com/charmbracelet/bubbletea v1.2.4
//     github.com/charmbracelet/lipgloss  v1.0.0
//     github.com/charmbracelet/bubbles   v0.20.0
//   )
//
// Run: go mod tidy && go run main.go

package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Color palette — Tokyo Night, matching PAI conventions
// ---------------------------------------------------------------------------

var (
	colorTitle   = lipgloss.Color("#7aa2f7")
	colorRunning = lipgloss.Color("#e0af68")
	colorIdle    = lipgloss.Color("#9ece6a")
	colorPaused  = lipgloss.Color("#7dcfff")
	colorError   = lipgloss.Color("#f7768e")
	colorStopped = lipgloss.Color("#565f89")
	colorBorder  = lipgloss.Color("#3b4261")
	colorFg      = lipgloss.Color("#c0caf5")
	colorDim     = lipgloss.Color("#565f89")
	colorSelBg   = lipgloss.Color("#283457")
	colorAccent  = lipgloss.Color("#bb9af7") // purple accent for phases
	colorBar     = lipgloss.Color("#9ece6a") // progress bar fill
	colorBarBg   = lipgloss.Color("#1a1b26") // progress bar empty
)

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------

type AgentStatus int

const (
	StatusRunning AgentStatus = iota
	StatusIdle
	StatusPaused
	StatusError
	StatusStopped
)

func (s AgentStatus) String() string {
	return [...]string{"Running", "Idle", "Paused", "Error", "Stopped"}[s]
}

func (s AgentStatus) Color() lipgloss.Color {
	return [...]lipgloss.Color{colorRunning, colorIdle, colorPaused, colorError, colorStopped}[s]
}

// Phase represents a PAI Algorithm phase (OBSERVE through LEARN).
type Phase int

const (
	PhaseObserve Phase = iota
	PhaseThink
	PhasePlan
	PhaseBuild
	PhaseExecute
	PhaseVerify
	PhaseLearn
	PhaseDone // completed all phases
)

var phaseNames = [...]string{"OBSERVE", "THINK", "PLAN", "BUILD", "EXECUTE", "VERIFY", "LEARN", "DONE"}
var phaseIcons = [...]string{"👁️", "🧠", "📋", "🔨", "⚡", "✅", "📚", "🏁"}

func (p Phase) String() string  { return phaseNames[p] }
func (p Phase) Icon() string    { return phaseIcons[p] }

// Agent represents a PAI agent with full real-time metrics.
type Agent struct {
	ID            string
	Name          string
	Status        AgentStatus
	StartedAt     time.Time
	LastActTime   time.Time
	LastActivity  string
	Model         string
	ISCItems      []ISCCriterion
	EventLog      []string
	// New real-time fields
	Phase         Phase
	Progress      int     // 0-100 percentage
	TokensPerSec  float64 // current tok/s throughput
	TotalTokensIn int     // cumulative input tokens
	TotalTokensOut int    // cumulative output tokens
	TaskDesc      string  // what this agent is working on
	ToolsUsed     int     // total tool invocations
	CurrentTool   string  // currently executing tool
}

// ISCCriterion tracks individual success criteria with pass/fail state.
type ISCCriterion struct {
	Text   string
	Passed bool
}

// ---------------------------------------------------------------------------
// Data pools for realistic simulation
// ---------------------------------------------------------------------------

var agentNames = []string{
	"Engineer", "Architect", "ClaudeResearcher", "GeminiResearcher",
	"GrokResearcher", "QATester", "Designer", "Pentester",
	"Explore", "Algorithm", "Intern",
}

var taskDescs = []string{
	"Implement auth middleware for API",
	"Design database schema for users",
	"Research best practices for caching",
	"Security audit of payment flow",
	"Explore codebase for dead imports",
	"Evaluate ISC criteria satisfaction",
	"Build React component library",
	"Test checkout E2E flow in browser",
	"Analyze API response time patterns",
	"Refactor state management layer",
}

var toolNames = []string{
	"Read", "Write", "Edit", "Bash", "Grep", "Glob",
	"WebSearch", "Task", "WebFetch", "Skill", "AskUserQuestion",
}

var activities = []string{
	"Read src/auth/middleware.ts",
	"Bash: npm run test",
	"Write api/routes.go",
	"Edit config/database.yaml",
	"Grep: 'async function'",
	"Glob: **/*.test.ts",
	"WebSearch: Go TUI frameworks",
	"Task: spawned Intern agent",
	"WebFetch: API docs",
	"ISC verified: tests pass",
	"Browser: screenshot captured",
	"Bash: go build ./...",
}

var models = []string{"claude-opus-4-6", "claude-sonnet-4-5", "claude-haiku-4-5", "gemini-2.5-pro", "grok-3"}

// Model-specific token throughput ranges (tok/s) — realistic values
var modelTokRanges = map[string][2]float64{
	"claude-opus-4-6":    {25, 65},
	"claude-sonnet-4-5":  {80, 160},
	"claude-haiku-4-5":   {150, 300},
	"gemini-2.5-pro":     {60, 130},
	"grok-3":             {70, 140},
}

var iscPool = []string{
	"Tests pass for auth module",
	"No security vulnerabilities detected",
	"API response time under 200ms",
	"All lint checks green",
	"Code coverage above 80 percent",
	"E2E login flow verified in browser",
	"No regressions in CI pipeline",
	"Database migrations reversible",
	"No credentials exposed in code",
	"Component renders without errors",
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func randHex4() string {
	return fmt.Sprintf("%04x", rand.Intn(0xFFFF+1))
}

func fmtDuration(d time.Duration) string {
	if d < 0 {
		return "--"
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func fmtAgo(t time.Time) string {
	d := time.Since(t)
	sec := int(d.Seconds())
	if sec < 1 {
		sec = 1
	}
	if sec < 60 {
		return fmt.Sprintf("%ds ago", sec)
	}
	return fmt.Sprintf("%dm%ds ago", sec/60, sec%60)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func pickRand[T any](sl []T) T { return sl[rand.Intn(len(sl))] }

// renderProgressBar draws a visual bar like ████░░░░ 45%
func renderProgressBar(pct, width int) string {
	if width < 8 {
		width = 8
	}
	barW := width - 5 // leave room for " XXX%"
	if barW < 4 {
		barW = 4
	}
	filled := barW * pct / 100
	empty := barW - filled

	fillStyle := lipgloss.NewStyle().Foreground(colorBar)
	emptyStyle := lipgloss.NewStyle().Foreground(colorBarBg)
	pctStyle := lipgloss.NewStyle().Foreground(colorFg)

	bar := fillStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", empty))
	return bar + pctStyle.Render(fmt.Sprintf(" %3d%%", pct))
}

func makeAgent() Agent {
	name := pickRand(agentNames)
	model := pickRand(models)
	now := time.Now()
	status := AgentStatus(rand.Intn(3)) // Running, Idle, or Paused
	phase := Phase(rand.Intn(7))

	// ISC criteria with random pass/fail
	iscCount := 3 + rand.Intn(4)
	isc := make([]ISCCriterion, 0, iscCount)
	for i := 0; i < iscCount; i++ {
		isc = append(isc, ISCCriterion{
			Text:   pickRand(iscPool),
			Passed: rand.Float32() < 0.6,
		})
	}

	// Seed event log
	log := make([]string, 0, 8)
	for i := 0; i < 4+rand.Intn(5); i++ {
		t := now.Add(-time.Duration(rand.Intn(300)) * time.Second)
		tool := pickRand(toolNames)
		log = append(log, fmt.Sprintf("[%s] %s: %s", t.Format("15:04:05"), tool, pickRand(activities)))
	}

	// Token throughput based on model
	tokRange := modelTokRanges[model]
	tokps := tokRange[0] + rand.Float64()*(tokRange[1]-tokRange[0])

	// Progress tied to phase
	baseProgress := int(phase) * 14 // ~14% per phase
	progress := clamp(baseProgress+rand.Intn(14), 0, 100)
	if status == StatusIdle {
		progress = 100
		phase = PhaseDone
	}
	if status == StatusStopped {
		progress = 0
	}

	return Agent{
		ID:             "pai-" + randHex4(),
		Name:           name,
		Status:         status,
		StartedAt:      now.Add(-time.Duration(rand.Intn(600)) * time.Second),
		LastActTime:    now.Add(-time.Duration(rand.Intn(20)) * time.Second),
		LastActivity:   pickRand(activities),
		Model:          model,
		ISCItems:       isc,
		EventLog:       log,
		Phase:          phase,
		Progress:       progress,
		TokensPerSec:   tokps,
		TotalTokensIn:  5000 + rand.Intn(50000),
		TotalTokensOut: 1000 + rand.Intn(20000),
		TaskDesc:       pickRand(taskDescs),
		ToolsUsed:      rand.Intn(40),
		CurrentTool:    pickRand(toolNames),
	}
}

// ---------------------------------------------------------------------------
// Bubble Tea messages
// ---------------------------------------------------------------------------

type tickMsg time.Time
type loadedMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func loadCmd() tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(_ time.Time) tea.Msg { return loadedMsg{} })
}

// ---------------------------------------------------------------------------
// Keybindings
// ---------------------------------------------------------------------------

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	Refresh key.Binding
	Toggle  key.Binding
	Quit    key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Refresh, k.Toggle, k.Quit}
}
func (k keyMap) FullHelp() [][]key.Binding { return [][]key.Binding{k.ShortHelp()} }

var keys = keyMap{
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Enter:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "detail")),
	Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Toggle:  key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start/stop")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

type model struct {
	agents      []Agent
	cursor      int
	detailOpen  bool
	loading     bool
	spinner     spinner.Model
	help        help.Model
	lastRefresh time.Time
	width       int
	height      int
	totalTicks  int
}

func initialModel() model {
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = lipgloss.NewStyle().Foreground(colorTitle)

	agents := make([]Agent, 0, 10)
	for i := 0; i < 10; i++ {
		agents = append(agents, makeAgent())
	}

	return model{
		agents:      agents,
		loading:     true,
		spinner:     sp,
		help:        help.New(),
		lastRefresh: time.Now(),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadCmd(), tickCmd())
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		return m, nil

	case loadedMsg:
		m.loading = false
		return m, nil

	case tickMsg:
		m.simulateTick()
		m.lastRefresh = time.Now()
		m.totalTicks++
		return m, tickCmd()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.cursor < len(m.agents)-1 {
				m.cursor++
			}
		case key.Matches(msg, keys.Enter):
			if len(m.agents) > 0 {
				m.detailOpen = !m.detailOpen
			}
		case key.Matches(msg, keys.Refresh):
			m.simulateTick()
			m.lastRefresh = time.Now()
		case key.Matches(msg, keys.Toggle):
			if len(m.agents) > 0 {
				a := &m.agents[m.cursor]
				if a.Status == StatusStopped {
					a.Status = StatusRunning
					a.StartedAt = time.Now()
					a.Phase = PhaseObserve
					a.Progress = 0
				} else {
					a.Status = StatusStopped
					a.TokensPerSec = 0
				}
			}
		}
	}
	return m, nil
}

// simulateTick mutates agent state every 2 seconds for real-time feel.
func (m *model) simulateTick() {
	now := time.Now()

	// Transition 1-2 agent statuses
	transitions := 1 + rand.Intn(2)
	for t := 0; t < transitions && len(m.agents) > 0; t++ {
		idx := rand.Intn(len(m.agents))
		a := &m.agents[idx]
		switch a.Status {
		case StatusRunning:
			if rand.Float32() < 0.15 {
				a.Status = []AgentStatus{StatusIdle, StatusPaused, StatusError}[rand.Intn(3)]
				if a.Status == StatusIdle {
					a.Phase = PhaseDone
					a.Progress = 100
					a.TokensPerSec = 0
				}
			}
		case StatusIdle:
			if rand.Float32() < 0.3 {
				a.Status = StatusRunning
				a.Phase = PhaseObserve
				a.Progress = 0
				a.TaskDesc = pickRand(taskDescs)
			}
		case StatusPaused:
			if rand.Float32() < 0.4 {
				a.Status = StatusRunning
			}
		case StatusError:
			if rand.Float32() < 0.3 {
				a.Status = StatusRunning
				a.Phase = PhaseObserve
				a.Progress = 0
			}
		}
	}

	// Update all running agents: advance phase, progress, tokens, activity
	for i := range m.agents {
		a := &m.agents[i]
		if a.Status != StatusRunning {
			continue
		}

		// Advance phase probabilistically
		if a.Phase < PhaseDone && rand.Float32() < 0.25 {
			a.Phase++
			if a.Phase == PhaseDone {
				a.Status = StatusIdle
				a.Progress = 100
				a.TokensPerSec = 0
				continue
			}
		}

		// Progress: advance toward phase-appropriate percentage
		targetPct := clamp(int(a.Phase+1)*14+rand.Intn(5), 0, 99)
		if a.Progress < targetPct {
			a.Progress += 1 + rand.Intn(4)
			if a.Progress > targetPct {
				a.Progress = targetPct
			}
		}

		// Token throughput: fluctuate around model baseline
		tokRange := modelTokRanges[a.Model]
		base := (tokRange[0] + tokRange[1]) / 2
		jitter := (rand.Float64() - 0.5) * (tokRange[1] - tokRange[0]) * 0.6
		a.TokensPerSec = base + jitter
		if a.TokensPerSec < 0 {
			a.TokensPerSec = tokRange[0]
		}

		// Accumulate tokens (simulate ~2 seconds of throughput)
		newOut := int(a.TokensPerSec * 2)
		a.TotalTokensOut += newOut
		a.TotalTokensIn += newOut * (2 + rand.Intn(3)) // input usually 2-4x output

		// Activity & tool usage
		a.CurrentTool = pickRand(toolNames)
		a.LastActivity = pickRand(activities)
		a.LastActTime = now.Add(-time.Duration(rand.Intn(3)) * time.Second)
		a.ToolsUsed++
		entry := fmt.Sprintf("[%s] %s → %s",
			now.Format("15:04:05"), a.CurrentTool, a.LastActivity)
		a.EventLog = append(a.EventLog, entry)
		if len(a.EventLog) > 20 {
			a.EventLog = a.EventLog[len(a.EventLog)-20:]
		}

		// Occasionally flip an ISC criterion
		if rand.Float32() < 0.2 && len(a.ISCItems) > 0 {
			idx := rand.Intn(len(a.ISCItems))
			a.ISCItems[idx].Passed = !a.ISCItems[idx].Passed
		}
	}

	// Occasionally spawn or garbage-collect
	if rand.Float32() < 0.12 && len(m.agents) < 14 {
		m.agents = append(m.agents, makeAgent())
	}
	if rand.Float32() < 0.06 && len(m.agents) > 6 {
		idx := rand.Intn(len(m.agents))
		if m.agents[idx].Status == StatusStopped {
			m.agents = append(m.agents[:idx], m.agents[idx+1:]...)
			if m.cursor >= len(m.agents) {
				m.cursor = clamp(len(m.agents)-1, 0, len(m.agents))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m model) View() string {
	w := m.width
	if w == 0 {
		w = 140
	}

	// --- Loading ---
	if m.loading {
		s := lipgloss.NewStyle().
			Width(w).Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(colorTitle)
		return s.Render(m.spinner.View() + "  Connecting to PAI orchestration layer...")
	}

	// --- Empty ---
	if len(m.agents) == 0 {
		s := lipgloss.NewStyle().
			Width(w).Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(colorDim)
		return s.Render("No agents active. Press 'n' to spawn a new agent.")
	}

	var sections []string

	// --- Title bar ---
	titleStyle := lipgloss.NewStyle().
		Bold(true).Foreground(colorTitle).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(0, 2).Width(w - 2).
		Align(lipgloss.Center)
	sections = append(sections, titleStyle.Render(
		fmt.Sprintf("⚡ PAI Agent Dashboard v0.2.0  │  %d agents  │  %s",
			len(m.agents), time.Now().Format("15:04:05"))))

	// --- Table ---
	sections = append(sections, m.renderTable(w))

	// --- Detail pane ---
	if m.detailOpen && m.cursor < len(m.agents) {
		sections = append(sections, m.renderDetail(w))
	}

	// --- Status bar ---
	sections = append(sections, m.renderStatusBar(w))

	// --- Help ---
	helpStyle := lipgloss.NewStyle().Foreground(colorDim).Width(w).Align(lipgloss.Center)
	sections = append(sections, helpStyle.Render(m.help.View(keys)))

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderTable draws the main agent table with phase, progress, tok/s columns.
func (m model) renderTable(w int) string {
	// Column widths: ID(11) Name(16) Status(9) Phase(9) Progress(16) Tok/s(8) Uptime(8) Process(rest)
	cID, cName, cStatus, cPhase, cProg, cTok, cUp := 11, 16, 9, 9, 16, 8, 8
	cProc := w - cID - cName - cStatus - cPhase - cProg - cTok - cUp - 10
	if cProc < 15 {
		cProc = 15
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(colorFg).Underline(true)
	header := fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s",
		cID, "AGENT ID", cName, "NAME", cStatus, "STATUS",
		cPhase, "PHASE", cProg, "PROGRESS", cTok, "TOK/S",
		cUp, "UPTIME", cProc, "CURRENT PROCESS")

	rows := []string{headerStyle.Render(header)}

	for i, a := range m.agents {
		// Status (colored)
		stStyle := lipgloss.NewStyle().Foreground(a.Status.Color())
		stStr := stStyle.Render(fmt.Sprintf("%-*s", cStatus, a.Status.String()))

		// Phase (colored with icon)
		phStr := lipgloss.NewStyle().Foreground(colorDim).Render(fmt.Sprintf("%-*s", cPhase, "--"))
		if a.Status == StatusRunning && a.Phase < PhaseDone {
			phStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
			phStr = phStyle.Render(fmt.Sprintf("%-*s", cPhase, a.Phase.Icon()+" "+a.Phase.String()[:3]))
		} else if a.Phase == PhaseDone {
			phStr = lipgloss.NewStyle().Foreground(colorIdle).Render(fmt.Sprintf("%-*s", cPhase, "🏁 DONE"))
		}

		// Progress bar
		progStr := renderProgressBar(a.Progress, cProg)
		if a.Status == StatusStopped {
			progStr = lipgloss.NewStyle().Foreground(colorDim).Render(fmt.Sprintf("%-*s", cProg, "   --"))
		}

		// Tok/s
		tokStr := lipgloss.NewStyle().Foreground(colorDim).Render(fmt.Sprintf("%-*s", cTok, "--"))
		if a.Status == StatusRunning && a.TokensPerSec > 0 {
			tokColor := colorIdle // green for good throughput
			if a.TokensPerSec < 50 {
				tokColor = colorRunning // yellow for slower
			}
			tokStr = lipgloss.NewStyle().Foreground(tokColor).
				Render(fmt.Sprintf("%-*s", cTok, fmt.Sprintf("%.0f", a.TokensPerSec)))
		}

		// Uptime
		upStr := "--"
		if a.Status != StatusStopped {
			upStr = fmtDuration(time.Since(a.StartedAt))
		}

		// Current process (tool + activity)
		procStr := lipgloss.NewStyle().Foreground(colorDim).Render("--")
		if a.Status == StatusRunning {
			proc := fmt.Sprintf("%s → %s", a.CurrentTool, a.LastActivity)
			if len(proc) > cProc-1 {
				proc = proc[:cProc-2] + "…"
			}
			procStr = proc
		} else if a.Status == StatusPaused {
			procStr = lipgloss.NewStyle().Foreground(colorPaused).Render("⏳ Awaiting input")
		} else if a.Status == StatusError {
			procStr = lipgloss.NewStyle().Foreground(colorError).Render("✗ Error — see detail")
		}

		line := fmt.Sprintf(" %-*s %-*s %s %s %s %s %-*s %s",
			cID, a.ID, cName, a.Name, stStr, phStr, progStr, tokStr,
			cUp, upStr, procStr)

		if i == m.cursor {
			rowStyle := lipgloss.NewStyle().Background(colorSelBg).Width(w)
			line = rowStyle.Render(line)
		}
		rows = append(rows, line)
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderDetail shows comprehensive agent information.
func (m model) renderDetail(w int) string {
	a := m.agents[m.cursor]

	border := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1).Width(w - 4)

	title := lipgloss.NewStyle().Bold(true).Foreground(colorTitle)
	label := lipgloss.NewStyle().Bold(true).Foreground(colorFg)
	dim := lipgloss.NewStyle().Foreground(colorDim)
	pass := lipgloss.NewStyle().Foreground(colorIdle)
	fail := lipgloss.NewStyle().Foreground(colorError)

	var b strings.Builder

	// ── Header ──
	b.WriteString(title.Render(fmt.Sprintf("Agent Detail — %s", a.ID)) + "\n")

	// ── Metadata (two-column layout) ──
	uptime := "--"
	if a.Status != StatusStopped {
		uptime = fmtDuration(time.Since(a.StartedAt))
	}
	stColored := lipgloss.NewStyle().Foreground(a.Status.Color()).Render(a.Status.String())

	col1 := fmt.Sprintf("%s %s\n%s %s\n%s %s\n%s %s",
		label.Render("Type:"), a.Name,
		label.Render("Model:"), a.Model,
		label.Render("Status:"), stColored,
		label.Render("Phase:"), a.Phase.Icon()+" "+a.Phase.String())

	col2 := fmt.Sprintf("%s %s\n%s %s\n%s %d\n%s %s",
		label.Render("Uptime:"), uptime,
		label.Render("Task:"), a.TaskDesc,
		label.Render("Tools used:"), a.ToolsUsed,
		label.Render("Progress:"), renderProgressBar(a.Progress, 20))

	// Side by side
	halfW := (w - 8) / 2
	c1Style := lipgloss.NewStyle().Width(halfW)
	c2Style := lipgloss.NewStyle().Width(halfW)
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, c1Style.Render(col1), c2Style.Render(col2)))
	b.WriteString("\n")

	// ── Token Stats ──
	// TODO: Replace with real PAI API — read from agent session's token usage endpoint
	b.WriteString(title.Render("Token Metrics") + "\n")
	b.WriteString(fmt.Sprintf("  %s %.1f tok/s   %s %s in   %s %s out   %s %s total\n",
		label.Render("Throughput:"), a.TokensPerSec,
		label.Render("Input:"), fmtTokens(a.TotalTokensIn),
		label.Render("Output:"), fmtTokens(a.TotalTokensOut),
		label.Render("Total:"), fmtTokens(a.TotalTokensIn+a.TotalTokensOut)))

	// ── Phase Timeline ──
	b.WriteString(title.Render("Phase Timeline") + "\n  ")
	for p := PhaseObserve; p <= PhaseLearn; p++ {
		icon := p.Icon()
		name := p.String()[:3]
		if p < a.Phase {
			b.WriteString(pass.Render(icon+" "+name) + " → ")
		} else if p == a.Phase {
			b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Render("▶"+icon+" "+name) + " → ")
		} else {
			b.WriteString(dim.Render(icon+" "+name) + " → ")
		}
	}
	b.WriteString("\n")

	// ── ISC Criteria ──
	// TODO: Replace with real ISC.json from agent's WORK directory
	b.WriteString(title.Render("ISC Criteria") + "\n")
	passed, total := 0, len(a.ISCItems)
	for _, c := range a.ISCItems {
		if c.Passed {
			passed++
			b.WriteString(pass.Render("  ✓ ") + c.Text + "\n")
		} else {
			b.WriteString(fail.Render("  ✗ ") + c.Text + "\n")
		}
	}
	b.WriteString(dim.Render(fmt.Sprintf("  [%d/%d passed]\n", passed, total)))

	// ── Recent Events ──
	// TODO: Replace with real JSONL event stream from ~/.claude/history/raw-outputs/
	b.WriteString(title.Render("Recent Events") + "\n")
	start := len(a.EventLog) - 8
	if start < 0 {
		start = 0
	}
	for _, entry := range a.EventLog[start:] {
		b.WriteString(dim.Render("  ") + entry + "\n")
	}

	return border.Render(b.String())
}

func fmtTokens(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// renderStatusBar shows aggregate metrics.
func (m model) renderStatusBar(w int) string {
	counts := map[AgentStatus]int{}
	var totalTok float64
	for _, a := range m.agents {
		counts[a.Status]++
		totalTok += a.TokensPerSec
	}

	parts := []string{
		fmt.Sprintf("Agents: %d", len(m.agents)),
		lipgloss.NewStyle().Foreground(colorRunning).Render(fmt.Sprintf("⚡%d running", counts[StatusRunning])),
		lipgloss.NewStyle().Foreground(colorIdle).Render(fmt.Sprintf("✓%d idle", counts[StatusIdle])),
		lipgloss.NewStyle().Foreground(colorError).Render(fmt.Sprintf("✗%d err", counts[StatusError])),
		fmt.Sprintf("Σ %.0f tok/s", totalTok),
	}
	left := strings.Join(parts, "  │  ")
	right := lipgloss.NewStyle().Foreground(colorDim).Render("⟳ " + m.lastRefresh.Format("15:04:05"))

	gap := w - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if gap < 1 {
		gap = 1
	}

	barStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorBorder).
		BorderTop(true).
		Width(w - 2).Padding(0, 1)

	return barStyle.Render(left + strings.Repeat(" ", gap) + right)
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	rand.Seed(time.Now().UnixNano())

	// --screenshot flag: render one frame to stdout and exit (for captures)
	if len(os.Args) > 1 && os.Args[1] == "--screenshot" {
		rand.Seed(42) // fixed seed for consistent output
		m := initialModel()
		m.loading = false
		m.width = 160
		m.height = 50
		m.detailOpen = true
		// Stage interesting agent states for the screenshot
		if len(m.agents) > 7 {
			a := &m.agents[0]; a.Name = "Engineer"; a.Status = StatusRunning; a.Phase = PhaseBuild; a.Progress = 58; a.TokensPerSec = 42; a.CurrentTool = "Edit"; a.LastActivity = "Edit config/database.yaml"; a.Model = "claude-opus-4-6"; a.TaskDesc = "Implement auth middleware for API"
			a = &m.agents[1]; a.Name = "ClaudeResearcher"; a.Status = StatusRunning; a.Phase = PhaseExecute; a.Progress = 72; a.TokensPerSec = 135; a.CurrentTool = "WebSearch"; a.LastActivity = "WebSearch: Go TUI frameworks"; a.Model = "claude-sonnet-4-5"
			a = &m.agents[2]; a.Name = "Architect"; a.Status = StatusIdle; a.Phase = PhaseDone; a.Progress = 100
			a = &m.agents[3]; a.Name = "GeminiResearcher"; a.Status = StatusRunning; a.Phase = PhaseObserve; a.Progress = 12; a.TokensPerSec = 245; a.CurrentTool = "Read"; a.LastActivity = "Read src/auth/middleware.ts"; a.Model = "claude-haiku-4-5"
			a = &m.agents[4]; a.Name = "QATester"; a.Status = StatusError; a.Progress = 45
			a = &m.agents[5]; a.Name = "Pentester"; a.Status = StatusRunning; a.Phase = PhaseVerify; a.Progress = 88; a.TokensPerSec = 98; a.CurrentTool = "Bash"; a.LastActivity = "Bash: npm run test"; a.Model = "gemini-2.5-pro"
			a = &m.agents[6]; a.Name = "Designer"; a.Status = StatusPaused; a.Phase = PhasePlan; a.Progress = 35
			a = &m.agents[7]; a.Name = "Algorithm"; a.Status = StatusRunning; a.Phase = PhaseThink; a.Progress = 28; a.TokensPerSec = 112; a.CurrentTool = "Task"; a.LastActivity = "Task: spawned Intern agent"; a.Model = "claude-sonnet-4-5"
		}
		fmt.Println(m.View())
		return
	}

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
