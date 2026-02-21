package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kyleharris/task-board/internal/app"
	"github.com/kyleharris/task-board/internal/domain"
)

type tasksLoadedMsg struct {
	tasks []domain.Task
	err   error
}

type opResultMsg struct {
	status string
	err    error
}

type tuiModel struct {
	svc         *app.Service
	actor       domain.Actor
	leaseTTL    int
	autoRenew   bool
	tasks       []domain.Task
	cursor      int
	status      string
	errText     string
	width       int
	height      int
	inputMode   bool
	inputType   domain.ArtifactType
	inputPrompt string
	input       textinput.Model
}

func newTUIModel(svc *app.Service, actor domain.Actor, leaseTTL int, autoRenew bool) tuiModel {
	in := textinput.New()
	in.Placeholder = "Write markdown artifact content..."
	in.CharLimit = 4000
	in.Width = 80

	return tuiModel{
		svc:       svc,
		actor:     actor,
		leaseTTL:  leaseTTL,
		autoRenew: autoRenew,
		status:    "Loading tasks...",
		input:     in,
	}
}

func (m tuiModel) Init() tea.Cmd {
	return loadTasksCmd(m.svc)
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.inputMode {
		return m.updateInputMode(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tasksLoadedMsg:
		if msg.err != nil {
			m.errText = msg.err.Error()
			m.status = "Failed to load tasks"
			return m, nil
		}
		m.tasks = msg.tasks
		if m.cursor >= len(m.tasks) {
			m.cursor = max(0, len(m.tasks)-1)
		}
		m.errText = ""
		m.status = fmt.Sprintf("Loaded %d task(s)", len(m.tasks))
		return m, nil
	case opResultMsg:
		if msg.err != nil {
			m.errText = msg.err.Error()
			m.status = "Operation failed"
			return m, nil
		}
		m.errText = ""
		m.status = msg.status
		return m, loadTasksCmd(m.svc)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "r":
			m.status = "Refreshing..."
			return m, loadTasksCmd(m.svc)
		case "j", "down":
			if m.cursor < len(m.tasks)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "c":
			task, ok := m.selectedTask()
			if !ok {
				return m, nil
			}
			return m, claimTaskCmd(m.svc, task.ID, m.actor, m.leaseTTL, m.autoRenew)
		case "n":
			task, ok := m.selectedTask()
			if !ok {
				return m, nil
			}
			return m, renewTaskCmd(m.svc, task.ID, m.actor, m.leaseTTL)
		case "u":
			task, ok := m.selectedTask()
			if !ok {
				return m, nil
			}
			return m, releaseTaskCmd(m.svc, task.ID, m.actor)
		case ">", "l":
			task, ok := m.selectedTask()
			if !ok {
				return m, nil
			}
			next, ok := nextState(task.State)
			if !ok {
				m.status = "Already at final state"
				return m, nil
			}
			return m, transitionTaskCmd(m.svc, task.ID, next, m.actor)
		case "<", "h":
			task, ok := m.selectedTask()
			if !ok {
				return m, nil
			}
			prev, ok := prevState(task.State)
			if !ok {
				m.status = "Already at initial state"
				return m, nil
			}
			return m, transitionTaskCmd(m.svc, task.ID, prev, m.actor)
		case "x":
			task, ok := m.selectedTask()
			if !ok {
				return m, nil
			}
			return m, readyCheckCmd(m.svc, task.ID, m.actor)
		case "a":
			return m.startInput(domain.ArtifactContext), nil
		case "d":
			return m.startInput(domain.ArtifactDesign), nil
		case "b":
			return m.startInput(domain.ArtifactRubricReview), nil
		}
	}

	return m, nil
}

func (m tuiModel) updateInputMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.inputMode = false
			m.input.SetValue("")
			m.status = "Artifact input canceled"
			return m, nil
		case "enter":
			content := strings.TrimSpace(m.input.Value())
			if content == "" {
				m.status = "Artifact content cannot be empty"
				return m, nil
			}
			task, ok := m.selectedTask()
			if !ok {
				m.inputMode = false
				m.input.SetValue("")
				return m, nil
			}
			artifactType := m.inputType
			m.inputMode = false
			m.input.SetValue("")
			return m, addArtifactCmd(m.svc, task.ID, artifactType, content, m.actor)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m tuiModel) View() string {
	if m.width == 0 {
		m.width = 120
	}
	if m.height == 0 {
		m.height = 36
	}

	listWidth := max(40, m.width/2)
	detailWidth := max(40, m.width-listWidth-4)

	left := m.renderTaskList(listWidth)
	right := m.renderTaskDetail(detailWidth)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	footer := m.renderFooter()

	if m.inputMode {
		inputBox := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(m.width - 2).
			Render(fmt.Sprintf("%s\n%s", m.inputPrompt, m.input.View()))
		return strings.Join([]string{body, footer, inputBox}, "\n")
	}

	return strings.Join([]string{body, footer}, "\n")
}

func (m tuiModel) renderTaskList(width int) string {
	header := lipgloss.NewStyle().Bold(true).Render("Tasks")
	lines := []string{header, ""}
	if len(m.tasks) == 0 {
		lines = append(lines, "No tasks found. Use CLI: taskboard task create ...")
	} else {
		for i, t := range m.tasks {
			prefix := "  "
			if i == m.cursor {
				prefix = "> "
			}
			line := fmt.Sprintf("%s%s  [%s]", prefix, t.ID, t.State)
			if i == m.cursor {
				line = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true).Render(line)
			}
			lines = append(lines, line)
			lines = append(lines, fmt.Sprintf("   %s", truncate(t.Title, max(10, width-6))))
		}
	}

	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(width).Height(m.height - 6).Render(strings.Join(lines, "\n"))
}

func (m tuiModel) renderTaskDetail(width int) string {
	header := lipgloss.NewStyle().Bold(true).Render("Task Details")
	lines := []string{header, ""}
	task, ok := m.selectedTask()
	if !ok {
		lines = append(lines, "Select a task to inspect details")
		return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(width).Height(m.height - 6).Render(strings.Join(lines, "\n"))
	}

	parent := "-"
	if task.ParentID != nil {
		parent = *task.ParentID
	}
	lines = append(lines,
		fmt.Sprintf("ID: %s", task.ID),
		fmt.Sprintf("Title: %s", task.Title),
		fmt.Sprintf("State: %s", task.State),
		fmt.Sprintf("Type: %s", task.TaskType),
		fmt.Sprintf("Priority: %d", task.Priority),
		fmt.Sprintf("Parent: %s", parent),
		fmt.Sprintf("Rubric Passed: %t", task.RubricPassed),
		"",
		"Description:",
		truncate(task.Description, max(20, width-4)),
	)

	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(width).Height(m.height - 6).Render(strings.Join(lines, "\n"))
}

func (m tuiModel) renderFooter() string {
	help := "j/k move  r refresh  c claim  n renew  u release  </> state  x ready-check  a context  d design  b rubric  q quit"
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	if m.errText != "" {
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	}
	status := m.status
	if m.errText != "" {
		status = fmt.Sprintf("%s: %s", m.status, m.errText)
	}

	return lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true, false, false, false).Padding(0, 1).
		Render(fmt.Sprintf("%s\n%s", help, statusStyle.Render(status)))
}

func (m tuiModel) selectedTask() (domain.Task, bool) {
	if len(m.tasks) == 0 || m.cursor < 0 || m.cursor >= len(m.tasks) {
		return domain.Task{}, false
	}
	return m.tasks[m.cursor], true
}

func (m tuiModel) startInput(artifactType domain.ArtifactType) tuiModel {
	m.inputMode = true
	m.inputType = artifactType
	m.inputPrompt = fmt.Sprintf("Add %s artifact (Enter to save, Esc to cancel)", artifactType)
	m.input.SetValue("")
	m.input.Focus()
	return m
}

func loadTasksCmd(svc *app.Service) tea.Cmd {
	return func() tea.Msg {
		tasks, err := svc.ListTasks(context.Background(), nil)
		return tasksLoadedMsg{tasks: tasks, err: err}
	}
}

func claimTaskCmd(svc *app.Service, taskID string, actor domain.Actor, ttl int, autoRenew bool) tea.Cmd {
	return func() tea.Msg {
		expiresAt, err := svc.ClaimTask(context.Background(), app.ClaimTaskInput{TaskID: taskID, Actor: actor, TTLMinutes: ttl, AutoRenew: autoRenew})
		if err != nil {
			return opResultMsg{status: "claim failed", err: err}
		}
		return opResultMsg{status: fmt.Sprintf("claimed %s until %s", taskID, expiresAt.Format("2006-01-02 15:04:05Z07:00"))}
	}
}

func renewTaskCmd(svc *app.Service, taskID string, actor domain.Actor, ttl int) tea.Cmd {
	return func() tea.Msg {
		expiresAt, err := svc.RenewTaskLease(context.Background(), taskID, actor, ttl)
		if err != nil {
			return opResultMsg{status: "renew failed", err: err}
		}
		return opResultMsg{status: fmt.Sprintf("renewed %s until %s", taskID, expiresAt.Format("2006-01-02 15:04:05Z07:00"))}
	}
}

func releaseTaskCmd(svc *app.Service, taskID string, actor domain.Actor) tea.Cmd {
	return func() tea.Msg {
		if err := svc.ReleaseTaskLease(context.Background(), taskID, actor); err != nil {
			return opResultMsg{status: "release failed", err: err}
		}
		return opResultMsg{status: fmt.Sprintf("released lease for %s", taskID)}
	}
}

func transitionTaskCmd(svc *app.Service, taskID string, toState domain.State, actor domain.Actor) tea.Cmd {
	return func() tea.Msg {
		if err := svc.TransitionTask(context.Background(), app.TransitionInput{TaskID: taskID, ToState: toState, Actor: actor}); err != nil {
			return opResultMsg{status: "transition failed", err: err}
		}
		return opResultMsg{status: fmt.Sprintf("transitioned %s to %s", taskID, toState)}
	}
}

func readyCheckCmd(svc *app.Service, taskID string, actor domain.Actor) tea.Cmd {
	return func() tea.Msg {
		if err := svc.ReadyCheck(context.Background(), taskID, actor); err != nil {
			return opResultMsg{status: "ready-check failed", err: err}
		}
		return opResultMsg{status: fmt.Sprintf("task %s is ready for implementation", taskID)}
	}
}

func addArtifactCmd(svc *app.Service, taskID string, artifactType domain.ArtifactType, content string, actor domain.Actor) tea.Cmd {
	return func() tea.Msg {
		path, version, err := svc.AddArtifact(context.Background(), taskID, artifactType, content, actor)
		if err != nil {
			return opResultMsg{status: "artifact write failed", err: err}
		}
		return opResultMsg{status: fmt.Sprintf("artifact %s v%d written (%s)", artifactType, version, path)}
	}
}

func nextState(s domain.State) (domain.State, bool) {
	for i, state := range domain.AllStates {
		if state == s && i < len(domain.AllStates)-1 {
			return domain.AllStates[i+1], true
		}
	}
	return "", false
}

func prevState(s domain.State) (domain.State, bool) {
	for i, state := range domain.AllStates {
		if state == s && i > 0 {
			return domain.AllStates[i-1], true
		}
	}
	return "", false
}

func truncate(s string, maxChars int) string {
	if maxChars <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s
	}
	if maxChars < 2 {
		return string(runes[:maxChars])
	}
	return string(runes[:maxChars-1]) + "..."
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
