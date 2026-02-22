package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kyleharris/task-board/internal/app"
	"github.com/kyleharris/task-board/internal/domain"
)

type statusFilter int

const (
	filterAll statusFilter = iota
	filterAgentActive
)

type statusLoadedMsg struct {
	rows []statusRow
	err  error
	at   time.Time
}

type statusTickMsg time.Time

type statusOpMsg struct {
	status string
	err    error
}

type statusRow struct {
	TaskID      string
	ShortRef    string
	ParentID    *string
	Depth       int
	HasChildren bool
	TaskTitle   string
	State       domain.State
	UpdatedAt   time.Time
	LeaseOwner  string
	LeaseActive bool
	AgentActive bool
}

type statusModel struct {
	svc           *app.Service
	actor         domain.Actor
	editable      bool
	rows          []statusRow
	visible       []statusRow
	cursor        int
	width         int
	height        int
	status        string
	errText       string
	filter        statusFilter
	collapsed     map[string]bool
	lastRefreshed time.Time
	commandMode   bool
	commandInput  textinput.Model
	helpMode      bool
}

func newStatusModel(svc *app.Service, actor domain.Actor, editable bool) statusModel {
	in := textinput.New()
	in.Placeholder = "edit 1"
	in.Prompt = ":"
	in.CharLimit = 200
	in.Width = 40

	return statusModel{
		svc:          svc,
		actor:        actor,
		editable:     editable,
		status:       "Loading task status...",
		filter:       filterAll,
		collapsed:    map[string]bool{},
		commandInput: in,
	}
}

func (m statusModel) Init() tea.Cmd {
	return tea.Batch(loadStatusCmd(m.svc), statusTick())
}

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.helpMode {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			return m, nil
		case tea.KeyMsg:
			switch msg.String() {
			case "esc", "?":
				m.helpMode = false
				return m, nil
			case "q", "ctrl+c":
				return m, tea.Quit
			}
		}
		return m, nil
	}

	if m.commandMode {
		return m.updateCommandMode(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case statusTickMsg:
		return m, tea.Batch(loadStatusCmd(m.svc), statusTick())
	case statusLoadedMsg:
		if msg.err != nil {
			m.errText = msg.err.Error()
			m.status = "Refresh failed"
			return m, nil
		}
		m.rows = msg.rows
		m.lastRefreshed = msg.at
		m.recomputeVisible()
		m.status = fmt.Sprintf("%d tasks", len(m.visible))
		m.errText = ""
		return m, nil
	case statusOpMsg:
		if msg.err != nil {
			m.errText = msg.err.Error()
			m.status = "Operation failed"
			return m, nil
		}
		m.errText = ""
		m.status = msg.status
		return m, loadStatusCmd(m.svc)
	case tea.KeyMsg:
		switch msg.String() {
		case "?":
			m.helpMode = true
			return m, nil
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			m.status = "Refreshing..."
			return m, loadStatusCmd(m.svc)
		case "j", "down":
			if m.cursor < len(m.visible)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "tab":
			m.filter = (m.filter + 1) % 2
			m.recomputeVisible()
		case " ":
			row, ok := m.selected()
			if !ok || !row.HasChildren {
				return m, nil
			}
			m.collapsed[row.TaskID] = !m.collapsed[row.TaskID]
			m.recomputeVisible()
		case ":":
			if !m.editable {
				m.status = "Read-only mode"
				m.errText = "status command mode is disabled (run with --editable)"
				return m, nil
			}
			m.commandMode = true
			m.commandInput.SetValue("")
			m.commandInput.Focus()
			return m, nil
		}
	}

	return m, nil
}

func (m statusModel) updateCommandMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "?":
			m.helpMode = true
			m.commandMode = false
			m.commandInput.Blur()
			return m, nil
		case "esc":
			m.commandMode = false
			m.commandInput.Blur()
			return m, nil
		case "enter":
			cmdText := strings.TrimSpace(m.commandInput.Value())
			m.commandMode = false
			m.commandInput.Blur()
			if cmdText == "" {
				return m, nil
			}
			return m, runStatusCommand(m.svc, m.visible, m.cursor, m.actor, cmdText)
		}
	}
	var cmd tea.Cmd
	m.commandInput, cmd = m.commandInput.Update(msg)
	return m, cmd
}

func (m statusModel) View() string {
	if m.width == 0 {
		m.width = 140
	}
	if m.height == 0 {
		m.height = 34
	}

	header := m.renderHeader()
	table := m.renderTable()
	footer := m.renderFooter()

	if m.helpMode {
		overlay := m.renderHelpOverlay(header + "\n" + table + "\n" + footer)
		return overlay
	}

	if m.commandMode {
		cmdBox := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Render("Command: " + m.commandInput.View())
		return strings.Join([]string{header, table, footer, cmdBox}, "\n")
	}

	return strings.Join([]string{header, table, footer}, "\n")
}

func (m statusModel) renderHeader() string {
	activeAgents := 0
	for _, row := range m.visible {
		if row.AgentActive {
			activeAgents++
		}
	}
	filterText := "all"
	if m.filter == filterAgentActive {
		filterText = "agent-active"
	}
	line := fmt.Sprintf("Task Status  |  filter=%s  |  active-agents=%d  |  refreshed=%s", filterText, activeAgents, m.lastRefreshed.Format("15:04:05"))
	return lipgloss.NewStyle().Bold(true).Padding(0, 1).Render(line)
}

func (m statusModel) renderTable() string {
	head := fmt.Sprintf("%-3s %-2s %-2s %-8s %-44s %-20s %-12s %-22s %-8s", "#", "S", "[]", "Ref", "Task", "Owner", "Lease", "State", "Updated")
	lines := []string{lipgloss.NewStyle().Bold(true).Render(head)}

	if len(m.visible) == 0 {
		lines = append(lines, "(no tasks)")
	} else {
		for i, row := range m.visible {
			prefix := "  "
			if i == m.cursor {
				prefix = "> "
			}
			treeLabel := m.treeLabel(row)
			ref := row.ShortRef
			if ref == "" {
				ref = row.TaskID
			}
			line := fmt.Sprintf("%s%-3d %-2s %-2s %-8s %-44s %-20s %-12s %-22s %-8s",
				prefix,
				i+1,
				statusIcon(row.State),
				doneBox(row.State),
				truncate(ref, 8),
				truncate(treeLabel, 44),
				truncate(row.LeaseOwner, 20),
				leaseText(row),
				truncate(string(row.State), 22),
				row.UpdatedAt.Format("01-02"),
			)
			if i == m.cursor {
				line = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true).Render(line)
			}
			lines = append(lines, line)
		}
	}

	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Height(m.height - 7).Render(strings.Join(lines, "\n"))
}

func (m statusModel) renderFooter() string {
	help := "? help  q quit"
	if m.editable {
		help = "? help  : command  q quit"
	}
	status := m.status
	if m.errText != "" {
		status = fmt.Sprintf("%s: %s", m.status, m.errText)
	}
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	if m.errText != "" {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	}
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true, false, false, false).Padding(0, 1).Render(help + "\n" + style.Render(status))
}

func (m statusModel) renderHelpOverlay(background string) string {
	lines := []string{
		"Command Palette",
		"",
		"Navigation",
		"j/k : move cursor",
		"tab : toggle filter (all / agent-active)",
		"space : collapse/expand parent row",
		"r : refresh now",
		"q : quit",
		"",
		"Command Mode",
	}
	if m.editable {
		lines = append(lines,
			":(e)dit <row>   (examples: :e 1, :edit 1)",
			":cp \"task name\"  create parent task",
			":cc \"task name\"  create child task in selected parent context",
		)
	} else {
		lines = append(lines, "(disabled in read-only mode; run tb stat --editable)")
	}
	lines = append(lines,
		"",
		"Close this panel with Esc or ?",
	)

	dimmed := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(background)
	boxWidth := min(78, max(52, m.width-12))
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Foreground(lipgloss.Color("252")).
		Background(lipgloss.Color("236")).
		Padding(1, 2).
		Width(boxWidth).
		Render(strings.Join(lines, "\n"))

	topPad := max(1, (m.height-18)/2)
	leftPad := max(1, (m.width-boxWidth)/2)
	return dimmed + "\n" + strings.Repeat("\n", topPad) + lipgloss.NewStyle().PaddingLeft(leftPad).Render(box)
}

func (m *statusModel) recomputeVisible() {
	visible := make([]statusRow, 0, len(m.rows))
	collapsedParent := map[string]bool{}
	for _, row := range m.rows {
		if row.Depth == 0 {
			collapsedParent[row.TaskID] = m.collapsed[row.TaskID]
		}
		if row.ParentID != nil {
			if collapsedParent[*row.ParentID] {
				continue
			}
		}
		if m.filter == filterAgentActive && !row.AgentActive {
			continue
		}
		visible = append(visible, row)
	}
	m.visible = visible
	if m.cursor >= len(m.visible) {
		m.cursor = max(0, len(m.visible)-1)
	}
}

func (m statusModel) selected() (statusRow, bool) {
	if len(m.visible) == 0 || m.cursor < 0 || m.cursor >= len(m.visible) {
		return statusRow{}, false
	}
	return m.visible[m.cursor], true
}

func (m statusModel) treeLabel(row statusRow) string {
	indent := strings.Repeat("  ", row.Depth)
	prefix := ""
	if row.Depth > 0 {
		prefix = "↳ "
	}
	if row.HasChildren {
		if m.collapsed[row.TaskID] {
			prefix = "▸ "
		} else {
			prefix = "▾ "
		}
	}
	return indent + prefix + row.TaskTitle
}

func loadStatusCmd(svc *app.Service) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
		defer cancel()
		statuses, err := svc.ListTaskStatus(ctx, nil)
		if err != nil {
			return statusLoadedMsg{err: err, at: time.Now()}
		}
		return statusLoadedMsg{rows: buildStatusRows(statuses), at: time.Now()}
	}
}

func statusTick() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return statusTickMsg(t)
	})
}

func buildStatusRows(statuses []app.TaskStatus) []statusRow {
	parents := []app.TaskStatus{}
	childrenByParent := map[string][]app.TaskStatus{}
	orphans := []app.TaskStatus{}
	for _, s := range statuses {
		if s.Task.ParentID == nil {
			parents = append(parents, s)
			continue
		}
		childrenByParent[*s.Task.ParentID] = append(childrenByParent[*s.Task.ParentID], s)
	}

	sort.Slice(parents, func(i, j int) bool { return parents[i].Task.UpdatedAt.After(parents[j].Task.UpdatedAt) })
	rows := []statusRow{}
	seenParent := map[string]bool{}
	for _, p := range parents {
		seenParent[p.Task.ID] = true
		children := childrenByParent[p.Task.ID]
		sort.Slice(children, func(i, j int) bool { return children[i].Task.UpdatedAt.After(children[j].Task.UpdatedAt) })
		rows = append(rows, statusFromTaskStatus(p, 0, len(children) > 0))
		for _, c := range children {
			rows = append(rows, statusFromTaskStatus(c, 1, false))
		}
	}
	for parentID, kids := range childrenByParent {
		if seenParent[parentID] {
			continue
		}
		orphans = append(orphans, kids...)
	}
	sort.Slice(orphans, func(i, j int) bool { return orphans[i].Task.UpdatedAt.After(orphans[j].Task.UpdatedAt) })
	for _, orphan := range orphans {
		rows = append(rows, statusFromTaskStatus(orphan, 0, false))
	}

	return rows
}

func statusFromTaskStatus(s app.TaskStatus, depth int, hasChildren bool) statusRow {
	owner := "-"
	agentActive := false
	if s.Lease != nil {
		owner = string(s.Lease.ActorType) + ":" + s.Lease.ActorID
		agentActive = s.LeaseActive && s.Lease.ActorType == domain.ActorTypeAgent
	}
	return statusRow{
		TaskID:      s.Task.ID,
		ShortRef:    s.Task.ShortRef,
		ParentID:    s.Task.ParentID,
		Depth:       depth,
		HasChildren: hasChildren,
		TaskTitle:   s.Task.Title,
		State:       s.Task.State,
		UpdatedAt:   s.Task.UpdatedAt,
		LeaseOwner:  owner,
		LeaseActive: s.LeaseActive,
		AgentActive: agentActive,
	}
}

func statusIcon(state domain.State) string {
	switch state {
	case domain.StateDone:
		return "✓"
	case domain.StateInProgress, domain.StateTesting, domain.StateDocumented:
		return "◔"
	case domain.StateReadyForImplementation:
		return "⚑"
	default:
		return "○"
	}
}

func doneBox(state domain.State) string {
	if state == domain.StateDone {
		return "☑"
	}
	return "☐"
}

func leaseText(row statusRow) string {
	if row.LeaseOwner == "-" {
		return "-"
	}
	if row.LeaseActive {
		if row.AgentActive {
			return "active[AGENT]"
		}
		return "active"
	}
	return "expired"
}

func runStatusCommand(svc *app.Service, visible []statusRow, cursor int, actor domain.Actor, cmdText string) tea.Cmd {
	return func() tea.Msg {
		verb, arg, err := parseStatusCommand(cmdText)
		if err != nil {
			return statusOpMsg{status: "Invalid command", err: err}
		}

		switch verb {
		case "edit", "e":
			idx, convErr := strconv.Atoi(arg)
			if convErr != nil || idx < 1 || idx > len(visible) {
				return statusOpMsg{status: "Invalid command", err: fmt.Errorf("row index out of range")}
			}
			row := visible[idx-1]
			artifactType := domain.ArtifactDesign
			if row.ParentID == nil && row.HasChildren {
				artifactType = domain.ArtifactParentDesign
			} else if row.ParentID != nil {
				artifactType = domain.ArtifactChildDesign
			}

			initial := ""
			if snap, ok, lookupErr := svc.GetLatestArtifact(context.Background(), row.TaskID, artifactType); lookupErr == nil && ok {
				initial = snap.ContentSnapshot
			} else if lookupErr != nil {
				return statusOpMsg{status: "Edit failed", err: lookupErr}
			}
			if strings.TrimSpace(initial) == "" {
				initial = fmt.Sprintf("# %s\n\n", artifactType)
			}

			content, editErr := editContentWithEditor(initial)
			if editErr != nil {
				return statusOpMsg{status: "Edit failed", err: editErr}
			}
			if strings.TrimSpace(content) == "" {
				return statusOpMsg{status: "Edit canceled", err: fmt.Errorf("content was empty")}
			}
			if _, _, addErr := svc.AddArtifact(context.Background(), row.TaskID, artifactType, content, actor); addErr != nil {
				return statusOpMsg{status: "Edit failed", err: addErr}
			}
			ref := row.ShortRef
			if ref == "" {
				ref = row.TaskID
			}
			return statusOpMsg{status: fmt.Sprintf("updated %s for %s", artifactType, ref)}
		case "cp":
			parentID, createErr := svc.CreateTask(context.Background(), app.CreateTaskInput{
				Title:       arg,
				Description: "Created from status board command mode",
				TaskType:    "design",
				Priority:    2,
			})
			if createErr != nil {
				return statusOpMsg{status: "Create parent failed", err: createErr}
			}
			parentDesign := fmt.Sprintf("# Parent Design: %s\n\n## Goal\n- \n\n## Scope\n- \n\n## Components\n- \n", arg)
			if _, _, addErr := svc.AddArtifact(context.Background(), parentID, domain.ArtifactParentDesign, parentDesign, actor); addErr != nil {
				return statusOpMsg{status: "Create parent failed", err: addErr}
			}
			task, lookupErr := svc.GetTask(context.Background(), parentID)
			if lookupErr != nil {
				return statusOpMsg{status: "Create parent failed", err: lookupErr}
			}
			ref := task.ShortRef
			if ref == "" {
				ref = task.ID
			}
			return statusOpMsg{status: fmt.Sprintf("created parent %s (%s)", ref, arg)}
		case "cc":
			if len(visible) == 0 {
				return statusOpMsg{status: "Create child failed", err: errors.New("no rows visible")}
			}
			if cursor < 0 || cursor >= len(visible) {
				cursor = 0
			}
			base := visible[cursor]
			parentID := base.TaskID
			if base.ParentID != nil {
				parentID = *base.ParentID
			}

			childID, createErr := svc.CreateTask(context.Background(), app.CreateTaskInput{
				Title:             arg,
				Description:       "Created from status board command mode",
				TaskType:          "implementation",
				Priority:          3,
				ParentID:          &parentID,
				RequiredForParent: true,
			})
			if createErr != nil {
				return statusOpMsg{status: "Create child failed", err: createErr}
			}
			childDesign := fmt.Sprintf("# Child Design: %s\n\n## Objective\n- \n\n## Plan\n- \n", arg)
			if _, _, addErr := svc.AddArtifact(context.Background(), childID, domain.ArtifactChildDesign, childDesign, actor); addErr != nil {
				return statusOpMsg{status: "Create child failed", err: addErr}
			}
			childContext := fmt.Sprintf("# Context\n\nParent Task: %s\n\nFiles to read first:\n- (add files)\n", parentID)
			if _, _, addErr := svc.AddArtifact(context.Background(), childID, domain.ArtifactContext, childContext, actor); addErr != nil {
				return statusOpMsg{status: "Create child failed", err: addErr}
			}
			task, lookupErr := svc.GetTask(context.Background(), childID)
			if lookupErr != nil {
				return statusOpMsg{status: "Create child failed", err: lookupErr}
			}
			ref := task.ShortRef
			if ref == "" {
				ref = task.ID
			}
			return statusOpMsg{status: fmt.Sprintf("created child %s (%s)", ref, arg)}
		default:
			return statusOpMsg{status: "Invalid command", err: fmt.Errorf("unsupported command %q", verb)}
		}
	}
}

func parseStatusCommand(cmdText string) (verb, arg string, err error) {
	cmdText = strings.TrimSpace(cmdText)
	if cmdText == "" {
		return "", "", fmt.Errorf("empty command")
	}
	parts := strings.Fields(cmdText)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("expected command and argument")
	}
	verb = strings.ToLower(parts[0])
	switch verb {
	case "edit", "e":
		if len(parts) != 2 {
			return "", "", fmt.Errorf("expected format: (e)dit <row-number>")
		}
		return verb, parts[1], nil
	case "cp", "cc":
		title := strings.TrimSpace(cmdText[len(parts[0]):])
		if strings.HasPrefix(title, "\"") && strings.HasSuffix(title, "\"") && len(title) >= 2 {
			title = title[1 : len(title)-1]
		}
		title = strings.TrimSpace(title)
		if title == "" {
			return "", "", fmt.Errorf("task name cannot be empty")
		}
		return verb, title, nil
	default:
		return "", "", fmt.Errorf("unsupported command %q", verb)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
