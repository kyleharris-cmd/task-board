package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
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

const (
	keymapDefault = "default"
	keymapVim     = "vim"
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

type pendingEditorMode int

const (
	pendingEditorModeEdit pendingEditorMode = iota
	pendingEditorModeCreateChild
	pendingEditorModeCreateParent
)

type pendingEditSession struct {
	mode         pendingEditorMode
	taskID       string
	shortRef     string
	artifactType domain.ArtifactType
	parentID     string
}

type editorCompletion struct {
	prefix   string
	matches  []string
	selected int
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
	LeaseActor  domain.ActorType
	LeaseActive bool
	AgentActive bool
}

type statusModel struct {
	svc           *app.Service
	actor         domain.Actor
	editable      bool
	keymapMode    string
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
	editorMode    bool
	editorInsert  bool
	pendingG      bool
	editorInput   textarea.Model
	completion    *editorCompletion
	pendingEdit   *pendingEditSession
	repoFiles     []string
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
		keymapMode:   resolveKeymapMode(os.Getenv("TB_KEYMAP")),
		status:       "Loading task status...",
		filter:       filterAll,
		collapsed:    map[string]bool{},
		commandInput: in,
		repoFiles:    nil,
	}
}

func (m statusModel) Init() tea.Cmd {
	return tea.Batch(loadStatusCmd(m.svc), statusTick())
}

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.editorMode {
		return m.updateEditorMode(msg)
	}

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
		if m.keymapMode == keymapVim {
			switch msg.String() {
			case "g":
				if m.pendingG {
					m.cursor = 0
					m.pendingG = false
					return m, nil
				}
				m.pendingG = true
				return m, nil
			case "G":
				if len(m.visible) > 0 {
					m.cursor = len(m.visible) - 1
				}
				m.pendingG = false
				return m, nil
			default:
				m.pendingG = false
			}
		}
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
		case "h":
			if m.keymapMode == keymapVim {
				row, ok := m.selected()
				if ok && row.HasChildren {
					m.collapsed[row.TaskID] = true
					m.recomputeVisible()
				}
				return m, nil
			}
		case "l":
			if m.keymapMode == keymapVim {
				row, ok := m.selected()
				if ok && row.HasChildren {
					m.collapsed[row.TaskID] = false
					m.recomputeVisible()
				}
				return m, nil
			}
		case "enter":
			if !m.editable {
				m.status = "Read-only mode"
				m.errText = "opening editor is disabled (run without --read-only)"
				return m, nil
			}
			row, ok := m.selected()
			if !ok {
				return m, nil
			}
			return m.beginEditForRow(row)
		case ":":
			if !m.editable {
				m.status = "Read-only mode"
				m.errText = "status command mode is disabled (run without --read-only)"
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
			verb, arg, parseErr := parseStatusCommand(cmdText)
			if parseErr != nil {
				return m, func() tea.Msg {
					return statusOpMsg{status: "Invalid command", err: parseErr}
				}
			}
			if verb == "edit" || verb == "e" {
				idx, convErr := strconv.Atoi(arg)
				if convErr != nil || idx < 1 || idx > len(m.visible) {
					return m, func() tea.Msg {
						return statusOpMsg{status: "Invalid command", err: fmt.Errorf("row index out of range")}
					}
				}
				return m.beginEditForRow(m.visible[idx-1])
			}
			if verb == "cc" {
				if len(m.visible) == 0 {
					return m, func() tea.Msg {
						return statusOpMsg{status: "Create child failed", err: errors.New("no rows visible")}
					}
				}
				cursor := m.cursor
				if cursor < 0 || cursor >= len(m.visible) {
					cursor = 0
				}
				base := m.visible[cursor]
				parentID := base.TaskID
				if base.ParentID != nil {
					parentID = *base.ParentID
				}

				initial := "Title: \n\n"
				if strings.TrimSpace(arg) != "" {
					initial = "Title: " + strings.TrimSpace(arg) + "\n\n"
				}
				m.pendingEdit = &pendingEditSession{
					mode:     pendingEditorModeCreateChild,
					parentID: parentID,
				}
				m.openInlineEditor(initial)
				m.status = "Creating child..."
				m.errText = ""
				return m, nil
			}
			if verb == "cp" {
				initial := "Title: \n\n"
				if strings.TrimSpace(arg) != "" {
					initial = "Title: " + strings.TrimSpace(arg) + "\n\n"
				}
				m.pendingEdit = &pendingEditSession{
					mode: pendingEditorModeCreateParent,
				}
				m.openInlineEditor(initial)
				m.status = "Creating parent..."
				m.errText = ""
				return m, nil
			}
			return m, runStatusCommand(m.svc, m.visible, m.cursor, m.actor, cmdText)
		}
	}
	var cmd tea.Cmd
	m.commandInput, cmd = m.commandInput.Update(msg)
	return m, cmd
}

func (m statusModel) beginEditForRow(row statusRow) (tea.Model, tea.Cmd) {
	artifactType := domain.ArtifactDesign
	if row.ParentID == nil && row.HasChildren {
		artifactType = domain.ArtifactParentDesign
	} else if row.ParentID != nil {
		artifactType = domain.ArtifactChildDesign
	}

	initial := ""
	if snap, ok, lookupErr := m.svc.GetLatestArtifact(context.Background(), row.TaskID, artifactType); lookupErr == nil && ok {
		initial = snap.ContentSnapshot
	} else if lookupErr != nil {
		return m, func() tea.Msg {
			return statusOpMsg{status: "Edit failed", err: lookupErr}
		}
	}
	if strings.TrimSpace(initial) == "" {
		initial = fmt.Sprintf("# %s\n\n", artifactType)
	}

	m.pendingEdit = &pendingEditSession{
		mode:         pendingEditorModeEdit,
		taskID:       row.TaskID,
		shortRef:     row.ShortRef,
		artifactType: artifactType,
	}
	m.openInlineEditor(initial)
	m.status = "Editing..."
	m.errText = ""
	return m, nil
}

func (m statusModel) updateEditorMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.editorInput.SetWidth(max(50, m.width-16))
		m.editorInput.SetHeight(max(10, m.height-12))
		return m, nil
	case tea.KeyMsg:
		if m.keymapMode == keymapVim {
			return m.updateEditorModeVim(msg)
		}
		switch msg.String() {
		case "esc":
			if m.completion != nil {
				m.completion = nil
				return m, nil
			}
			return m.saveInlineEditor()
		case "ctrl+q":
			m.editorMode = false
			m.pendingEdit = nil
			m.completion = nil
			m.status = "Edit canceled"
			m.errText = ""
			return m, nil
		case "ctrl+s":
			return m.saveInlineEditor()
		case "enter":
			if m.completion != nil {
				m.applySelectedEditorCompletion()
				return m, nil
			}
		case "j", "down":
			if m.completion != nil && len(m.completion.matches) > 0 {
				m.completion.selected = (m.completion.selected + 1) % len(m.completion.matches)
				return m, nil
			}
		case "k", "up":
			if m.completion != nil && len(m.completion.matches) > 0 {
				m.completion.selected--
				if m.completion.selected < 0 {
					m.completion.selected = len(m.completion.matches) - 1
				}
				return m, nil
			}
		case "tab":
			m.advanceEditorCompletion()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.editorInput, cmd = m.editorInput.Update(msg)
	if _, ok := msg.(tea.KeyMsg); ok && m.completion != nil {
		m.refreshEditorCompletion(false)
	}
	return m, cmd
}

func (m statusModel) updateEditorModeVim(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.editorInsert {
		switch msg.String() {
		case "esc":
			if m.completion != nil {
				m.completion = nil
				return m, nil
			}
			m.editorInsert = false
			return m, nil
		case "ctrl+s":
			return m.saveInlineEditor()
		case "ctrl+q":
			m.editorMode = false
			m.pendingEdit = nil
			m.completion = nil
			m.editorInsert = false
			m.status = "Edit canceled"
			m.errText = ""
			return m, nil
		case "tab":
			m.advanceEditorCompletion()
			return m, nil
		case "enter":
			if m.completion != nil {
				m.applySelectedEditorCompletion()
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.editorInput, cmd = m.editorInput.Update(msg)
		if m.completion != nil {
			m.refreshEditorCompletion(false)
		}
		return m, cmd
	}

	switch msg.String() {
	case "i", "a", "o":
		if msg.String() == "o" {
			m.editorInput.InsertString("\n")
		}
		m.editorInsert = true
		return m, nil
	case "j", "down":
		if m.completion != nil && len(m.completion.matches) > 0 {
			m.completion.selected = (m.completion.selected + 1) % len(m.completion.matches)
			return m, nil
		}
		m.editorInput.CursorDown()
		return m, nil
	case "k", "up":
		if m.completion != nil && len(m.completion.matches) > 0 {
			m.completion.selected--
			if m.completion.selected < 0 {
				m.completion.selected = len(m.completion.matches) - 1
			}
			return m, nil
		}
		m.editorInput.CursorUp()
		return m, nil
	case "h", "left":
		// We only expose line-based movement in this first vim pass.
		return m, nil
	case "l", "right":
		// We only expose line-based movement in this first vim pass.
		return m, nil
	case "tab":
		m.advanceEditorCompletion()
		return m, nil
	case "enter":
		if m.completion != nil {
			m.applySelectedEditorCompletion()
			return m, nil
		}
		return m.saveInlineEditor()
	case "esc":
		if m.completion != nil {
			m.completion = nil
		}
		return m, nil
	case "q", "ctrl+q":
		m.editorMode = false
		m.pendingEdit = nil
		m.completion = nil
		m.status = "Edit canceled"
		m.errText = ""
		return m, nil
	}
	return m, nil
}

func (m *statusModel) saveInlineEditor() (tea.Model, tea.Cmd) {
	if m.pendingEdit == nil {
		m.editorMode = false
		m.status = "Edit canceled"
		return m, nil
	}
	session := m.pendingEdit
	m.pendingEdit = nil
	m.editorMode = false

	content := m.editorInput.Value()
	if strings.TrimSpace(content) == "" {
		m.errText = "content was empty"
		m.status = "Edit canceled"
		return m, nil
	}

	switch session.mode {
	case pendingEditorModeCreateChild:
		title, body, parseErr := parseTitleAndBody(content)
		if parseErr != nil {
			m.errText = parseErr.Error()
			m.status = "Create child failed"
			return m, nil
		}
		childID, createErr := m.svc.CreateTask(context.Background(), app.CreateTaskInput{
			Title:             title,
			Description:       body,
			TaskType:          "implementation",
			Priority:          3,
			ParentID:          &session.parentID,
			RequiredForParent: true,
		})
		if createErr != nil {
			m.errText = createErr.Error()
			m.status = "Create child failed"
			return m, nil
		}
		if strings.TrimSpace(body) != "" {
			if _, _, addErr := m.svc.AddArtifact(context.Background(), childID, domain.ArtifactChildDesign, body, m.actor); addErr != nil {
				m.errText = addErr.Error()
				m.status = "Create child failed"
				return m, nil
			}
		}
		task, lookupErr := m.svc.GetTask(context.Background(), childID)
		if lookupErr != nil {
			m.errText = lookupErr.Error()
			m.status = "Create child failed"
			return m, nil
		}
		ref := task.ShortRef
		if ref == "" {
			ref = task.ID
		}
		m.errText = ""
		m.status = fmt.Sprintf("created child %s (%s)", ref, title)
		return m, loadStatusCmd(m.svc)
	case pendingEditorModeCreateParent:
		title, body, parseErr := parseTitleAndBody(content)
		if parseErr != nil {
			m.errText = parseErr.Error()
			m.status = "Create parent failed"
			return m, nil
		}
		parentID, createErr := m.svc.CreateTask(context.Background(), app.CreateTaskInput{
			Title:       title,
			Description: body,
			TaskType:    "design",
			Priority:    2,
		})
		if createErr != nil {
			m.errText = createErr.Error()
			m.status = "Create parent failed"
			return m, nil
		}
		if strings.TrimSpace(body) != "" {
			if _, _, addErr := m.svc.AddArtifact(context.Background(), parentID, domain.ArtifactParentDesign, body, m.actor); addErr != nil {
				m.errText = addErr.Error()
				m.status = "Create parent failed"
				return m, nil
			}
		}
		task, lookupErr := m.svc.GetTask(context.Background(), parentID)
		if lookupErr != nil {
			m.errText = lookupErr.Error()
			m.status = "Create parent failed"
			return m, nil
		}
		ref := task.ShortRef
		if ref == "" {
			ref = task.ID
		}
		m.errText = ""
		m.status = fmt.Sprintf("created parent %s (%s)", ref, title)
		return m, loadStatusCmd(m.svc)
	default:
		if _, _, addErr := m.svc.AddArtifact(context.Background(), session.taskID, session.artifactType, content, m.actor); addErr != nil {
			m.errText = addErr.Error()
			m.status = "Edit failed"
			return m, nil
		}
		ref := session.shortRef
		if ref == "" {
			ref = session.taskID
		}
		m.errText = ""
		m.status = fmt.Sprintf("updated %s for %s", session.artifactType, ref)
		return m, loadStatusCmd(m.svc)
	}
}

func (m *statusModel) advanceEditorCompletion() {
	if m.completion == nil {
		if !m.refreshEditorCompletion(true) {
			m.status = "No matching paths"
			return
		}
		m.status = fmt.Sprintf("Path suggestions: %d", len(m.completion.matches))
		return
	}
	if len(m.completion.matches) == 0 {
		m.completion = nil
		return
	}
	m.completion.selected = (m.completion.selected + 1) % len(m.completion.matches)
}

func (m *statusModel) applySelectedEditorCompletion() {
	if m.completion == nil || len(m.completion.matches) == 0 {
		return
	}
	selected := m.completion.matches[m.completion.selected]
	prefix := m.completion.prefix
	if !strings.HasPrefix(selected, prefix) {
		m.completion = nil
		return
	}
	suffix := selected[len(prefix):]
	if suffix != "" {
		m.editorInput.InsertString(suffix)
	}
	if strings.HasSuffix(selected, "/") {
		m.refreshEditorCompletion(true)
	} else {
		m.completion = nil
	}
}

func (m *statusModel) refreshEditorCompletion(forceShow bool) bool {
	if len(m.repoFiles) == 0 {
		m.repoFiles = collectRepoFiles(m.svc.RepoRoot())
	}
	token := m.editorCurrentTokenPrefix()
	typed := strings.TrimLeft(token, "\"'")
	slashMode := strings.HasPrefix(typed, "/")
	query := strings.TrimPrefix(typed, "/")
	if query == "" && !forceShow {
		m.completion = nil
		return false
	}

	matches := listPathSuggestions(query, m.repoFiles)
	if slashMode {
		for i := range matches {
			matches[i] = "/" + matches[i]
		}
	}
	if len(matches) == 0 {
		m.completion = nil
		return false
	}

	sel := 0
	if m.completion != nil && m.completion.prefix == typed {
		if m.completion.selected >= 0 && m.completion.selected < len(matches) {
			sel = m.completion.selected
		}
	}
	m.completion = &editorCompletion{
		prefix:   typed,
		matches:  matches,
		selected: sel,
	}
	return true
}

func (m statusModel) editorCurrentTokenPrefix() string {
	value := m.editorInput.Value()
	lines := strings.Split(value, "\n")
	lineIdx := m.editorInput.Line()
	if lineIdx < 0 || lineIdx >= len(lines) {
		return ""
	}
	line := lines[lineIdx]
	col := m.editorInput.LineInfo().CharOffset
	if col < 0 {
		col = 0
	}
	r := []rune(line)
	if col > len(r) {
		col = len(r)
	}
	start := col
	for start > 0 && isPathTokenRune(r[start-1]) {
		start--
	}
	return string(r[start:col])
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

	if m.editorMode {
		editor := m.renderEditorOverlay(header + "\n" + table + "\n" + footer)
		return editor
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
	line := fmt.Sprintf("Task Status  |  filter=%s  |  active-agents=%d  |  checkout(🔒H human, 🔒A agent)  |  refreshed=%s", filterText, activeAgents, m.lastRefreshed.Format("15:04:05"))
	return lipgloss.NewStyle().Bold(true).Padding(0, 1).Render(line)
}

func (m statusModel) renderTable() string {
	head := fmt.Sprintf("%-3s %-2s %-3s %-2s %-8s %-42s %-20s %-12s %-22s %-8s", "#", "S", "CO", "[]", "Ref", "Task", "Owner", "Lease", "State", "Updated")
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
			line := fmt.Sprintf("%s%-3d %-2s %-3s %-2s %-8s %-42s %-20s %-12s %-22s %-8s",
				prefix,
				i+1,
				statusIcon(row.State),
				checkoutIcon(row),
				doneBox(row.State),
				truncate(ref, 8),
				truncate(treeLabel, 42),
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
		"enter : open highlighted task",
		"r : refresh now",
		"q : quit",
		"",
		"Command Mode",
	}
	if m.editable {
		lines = append(lines,
			":(e)dit <row>   (examples: :e1, :edit1, :e 1, :edit 1)",
			":cp [optional title]  create parent from editor (line 1: Title: ..., rest=content)",
			":cc [optional title]  create child from editor (line 1: Title: ..., rest=content)",
			"",
			"Inline Editor",
			"tab : open/cycle path suggestions",
			"j/k : move suggestion selection",
			"enter : accept selected suggestion",
			"ctrl+s or esc : save",
			"ctrl+q : cancel",
		)
		if m.keymapMode == keymapVim {
			lines = append(lines,
				"",
				"Vim Mode (TB_KEYMAP=vim)",
				"status: gg/G jump, h/l collapse/expand",
				"editor: starts NORMAL, i/a/o enter INSERT, Esc returns NORMAL",
			)
		}
	} else {
		lines = append(lines, "(disabled in read-only mode; run tb stat without --read-only)")
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

func (m *statusModel) openInlineEditor(initial string) {
	ta := textarea.New()
	ta.SetValue(initial)
	ta.Focus()
	ta.CharLimit = 0
	ta.Prompt = "│ "
	ta.ShowLineNumbers = true
	ta.SetWidth(max(50, m.width-16))
	ta.SetHeight(max(10, m.height-12))
	m.editorInput = ta
	m.editorMode = true
	m.editorInsert = m.keymapMode != keymapVim
	m.completion = nil
}

func (m statusModel) renderEditorOverlay(background string) string {
	dimmed := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(background)
	title := "Inline Editor  |  tab suggestions  |  enter accept  |  esc/ctrl+s save"
	if m.keymapMode == keymapVim {
		mode := "NORMAL"
		if m.editorInsert {
			mode = "INSERT"
		}
		title = fmt.Sprintf("Inline Editor [%s]  |  i/a/o insert  |  enter save  |  ctrl+q cancel", mode)
	}
	content := m.editorInput.View()
	if m.completion != nil && len(m.completion.matches) > 0 {
		content += "\n\n" + m.renderCompletionList()
	}
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Foreground(lipgloss.Color("252")).
		Background(lipgloss.Color("236")).
		Padding(1, 1).
		Width(max(60, m.width-10)).
		Height(max(14, m.height-8)).
		Render(lipgloss.NewStyle().Bold(true).Render(title) + "\n\n" + content)
	return dimmed + "\n" + lipgloss.NewStyle().PaddingLeft(3).PaddingTop(1).Render(panel)
}

func (m statusModel) renderCompletionList() string {
	if m.completion == nil || len(m.completion.matches) == 0 {
		return ""
	}
	lines := []string{"Path Suggestions (Tab/j/k=move, Enter=insert, Esc=close list)"}
	for i, match := range m.completion.matches {
		prefix := "  "
		if i == m.completion.selected {
			prefix = "> "
		}
		line := prefix + truncate(match, max(20, m.width-28))
		if i == m.completion.selected {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true).Render(line)
		}
		lines = append(lines, line)
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Render(strings.Join(lines, "\n"))
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
	leaseActor := domain.ActorType("")
	agentActive := false
	if s.Lease != nil {
		owner = string(s.Lease.ActorType) + ":" + s.Lease.ActorID
		leaseActor = s.Lease.ActorType
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
		LeaseActor:  leaseActor,
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

func checkoutIcon(row statusRow) string {
	if row.LeaseOwner == "-" {
		return "-"
	}
	actor := "?"
	switch row.LeaseActor {
	case domain.ActorTypeHuman:
		actor = "H"
	case domain.ActorTypeAgent:
		actor = "A"
	}
	if row.LeaseActive {
		return "🔒" + actor
	}
	return "⏳" + actor
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
	cmdText = strings.TrimPrefix(cmdText, ":")
	if cmdText == "" {
		return "", "", fmt.Errorf("empty command")
	}

	lower := strings.ToLower(cmdText)
	for _, prefix := range []string{"edit", "e"} {
		if strings.HasPrefix(lower, prefix) {
			arg := strings.TrimSpace(cmdText[len(prefix):])
			if arg == "" {
				return "", "", fmt.Errorf("expected format: (e)dit <row-number>")
			}
			return prefix, arg, nil
		}
	}

	parts := strings.Fields(cmdText)
	if len(parts) == 0 {
		return "", "", fmt.Errorf("empty command")
	}
	verb = strings.ToLower(parts[0])
	switch verb {
	case "edit", "e":
		if len(parts) != 2 {
			return "", "", fmt.Errorf("expected format: (e)dit <row-number>")
		}
		return verb, parts[1], nil
	case "cc":
		title := strings.TrimSpace(cmdText[len(parts[0]):])
		if strings.HasPrefix(title, "\"") && strings.HasSuffix(title, "\"") && len(title) >= 2 {
			title = title[1 : len(title)-1]
		}
		return verb, strings.TrimSpace(title), nil
	case "cp":
		title := strings.TrimSpace(cmdText[len(parts[0]):])
		if strings.HasPrefix(title, "\"") && strings.HasSuffix(title, "\"") && len(title) >= 2 {
			title = title[1 : len(title)-1]
		}
		return verb, strings.TrimSpace(title), nil
	default:
		return "", "", fmt.Errorf("unsupported command %q", verb)
	}
}

func collectRepoFiles(repoRoot string) []string {
	out := make([]string, 0, 256)
	_ = filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".taskboard", "bin", "node_modules":
				if path != repoRoot {
					return filepath.SkipDir
				}
			}
			return nil
		}
		rel, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(out)
	return out
}

func listPathSuggestions(query string, files []string) []string {
	baseDir := ""
	partial := ""
	if strings.HasSuffix(query, "/") {
		baseDir = strings.TrimSuffix(query, "/")
		partial = ""
	} else if idx := strings.LastIndex(query, "/"); idx >= 0 {
		baseDir = query[:idx]
		partial = query[idx+1:]
	} else {
		partial = query
	}
	if baseDir == "." {
		baseDir = ""
	}

	type entry struct {
		name  string
		isDir bool
	}
	entries := map[string]entry{}
	for _, f := range files {
		rest := f
		if baseDir != "" {
			prefix := baseDir + "/"
			if !strings.HasPrefix(f, prefix) {
				continue
			}
			rest = strings.TrimPrefix(f, prefix)
		}
		if rest == "" {
			continue
		}
		parts := strings.SplitN(rest, "/", 2)
		child := parts[0]
		if child == "" || !strings.HasPrefix(child, partial) {
			continue
		}
		isDir := len(parts) > 1
		existing, ok := entries[child]
		if !ok || isDir {
			entries[child] = entry{name: child, isDir: existing.isDir || isDir}
		}
	}

	dirs := make([]string, 0, len(entries))
	regular := make([]string, 0, len(entries))
	for _, e := range entries {
		full := e.name
		if baseDir != "" {
			full = baseDir + "/" + full
		}
		if e.isDir {
			dirs = append(dirs, full+"/")
		} else {
			regular = append(regular, full)
		}
	}
	sort.Strings(dirs)
	sort.Strings(regular)
	return append(dirs, regular...)
}

func bestPathCompletion(prefix string, candidates []string) (string, bool) {
	matches := make([]string, 0, 8)
	for _, c := range candidates {
		if strings.HasPrefix(c, prefix) {
			matches = append(matches, c)
			if len(matches) > 100 {
				break
			}
		}
	}
	if len(matches) == 0 {
		return "", false
	}
	if len(matches) == 1 {
		return matches[0], true
	}
	common := matches[0]
	for _, m := range matches[1:] {
		for !strings.HasPrefix(m, common) {
			if len(common) == 0 {
				return "", false
			}
			common = common[:len(common)-1]
		}
	}
	if len(common) <= len(prefix) {
		return "", false
	}
	return common, true
}

func isPathTokenRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return true
	case r >= 'A' && r <= 'Z':
		return true
	case r >= '0' && r <= '9':
		return true
	}
	switch r {
	case '/', '.', '_', '-', '"', '\'':
		return true
	default:
		return false
	}
}

func parseTitleAndBody(content string) (string, string, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return "", "", fmt.Errorf("first line must contain a task title")
	}
	title := strings.TrimSpace(lines[0])
	lower := strings.ToLower(title)
	if strings.HasPrefix(lower, "title:") {
		title = strings.TrimSpace(title[len("title:"):])
	}
	if title == "" {
		return "", "", fmt.Errorf("first line must contain a task title")
	}
	body := ""
	if len(lines) > 1 {
		body = strings.TrimSpace(strings.Join(lines[1:], "\n"))
	}
	return title, body, nil
}

func resolveKeymapMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case keymapVim:
		return keymapVim
	default:
		return keymapDefault
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
