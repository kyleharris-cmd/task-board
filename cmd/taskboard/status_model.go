package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

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

type statusRow struct {
	TaskID      string
	ParentID    *string
	Depth       int
	HasChildren bool
	TaskTitle   string
	State       domain.State
	UpdatedAt   time.Time
	LeaseOwner  string
	LeaseActive bool
	AgentActive bool
	Collapsed   bool
}

type statusModel struct {
	svc           *app.Service
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
}

func newStatusModel(svc *app.Service) statusModel {
	return statusModel{
		svc:       svc,
		status:    "Loading task status...",
		filter:    filterAll,
		collapsed: map[string]bool{},
	}
}

func (m statusModel) Init() tea.Cmd {
	return tea.Batch(loadStatusCmd(m.svc), statusTick())
}

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	case tea.KeyMsg:
		switch msg.String() {
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
		}
	}

	return m, nil
}

func (m statusModel) View() string {
	if m.width == 0 {
		m.width = 120
	}
	if m.height == 0 {
		m.height = 30
	}

	header := m.renderHeader()
	table := m.renderTable()
	footer := m.renderFooter()

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
	head := fmt.Sprintf("%-2s %-2s %-52s %-20s %-12s %-22s %-8s", "S", "[]", "Task", "Owner", "Lease", "State", "Updated")
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
			line := fmt.Sprintf("%s%s %-2s %-52s %-20s %-12s %-22s %-8s",
				prefix,
				statusIcon(row.State),
				doneBox(row.State),
				truncate(treeLabel, 52),
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

	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Height(m.height - 6).Render(strings.Join(lines, "\n"))
}

func (m statusModel) renderFooter() string {
	help := "j/k move  tab filter  space collapse parent  r refresh  q quit"
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
		statuses, err := svc.ListTaskStatus(context.Background(), nil)
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
