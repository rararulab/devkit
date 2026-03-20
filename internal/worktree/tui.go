// tui.go implements an interactive terminal UI for worktree management.
package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Color palette — muted, modern terminal aesthetic.
var (
	colorPurple  = lipgloss.Color("99")
	colorGreen   = lipgloss.Color("42")
	colorYellow  = lipgloss.Color("214")
	colorRed     = lipgloss.Color("196")
	colorDim     = lipgloss.Color("241")
	colorFaint   = lipgloss.Color("238")
	colorCyan    = lipgloss.Color("80")
	colorWhite   = lipgloss.Color("255")
	colorSubtle  = lipgloss.Color("245")
	colorHotPink = lipgloss.Color("205")

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorPurple).
			Padding(0, 1).
			MarginBottom(1)

	styleStatus = map[Status]lipgloss.Style{
		StatusActive:   lipgloss.NewStyle().Foreground(colorGreen),
		StatusMerged:   lipgloss.NewStyle().Foreground(colorYellow),
		StatusDetached: lipgloss.NewStyle().Foreground(colorSubtle),
		StatusPrunable: lipgloss.NewStyle().Foreground(colorRed),
	}

	// Indicator icons per status
	statusIcon = map[Status]string{
		StatusActive:   "●",
		StatusMerged:   "◆",
		StatusDetached: "○",
		StatusPrunable: "✖",
	}

	styleCheck   = lipgloss.NewStyle().Foreground(colorHotPink).Bold(true)
	styleMain    = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	stylePath    = lipgloss.NewStyle().Foreground(colorWhite)
	styleBranch  = lipgloss.NewStyle().Foreground(colorCyan)
	styleDimPath = lipgloss.NewStyle().Foreground(colorDim)

	styleMessage = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	styleBusy    = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)

	styleHelpKey  = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	styleHelpDesc = lipgloss.NewStyle().Foreground(colorDim)
	styleHelpSep  = lipgloss.NewStyle().Foreground(colorFaint)

	styleCount     = lipgloss.NewStyle().Foreground(colorSubtle)
	styleLock      = lipgloss.NewStyle().Foreground(colorDim)
	styleToastBox  = lipgloss.NewStyle().Foreground(colorWhite).Background(lipgloss.Color("52")).Padding(0, 1)
	styleToastText = lipgloss.NewStyle().Foreground(lipgloss.Color("217"))
)

const toastDuration = 4 * time.Second

// Messages returned by async commands.
type deleteResultMsg struct {
	removed    int
	errors     []string // per-worktree errors
	freedBytes int64    // total bytes freed by successful removals
}

// sizeResultMsg delivers asynchronously computed disk sizes.
// The generation field prevents stale results from overwriting a newer entry list.
type sizeResultMsg struct {
	generation int           // must match tuiModel.generation to apply
	sizes      map[int]int64 // index → bytes
}

type pruneResultMsg struct{ err error }
type reloadResultMsg struct {
	entries []Entry
	err     error
}

// dismissToastMsg is sent by tea.Tick to auto-dismiss a toast.
type dismissToastMsg struct{ id int }

// dismissMessageMsg clears the status bar message after a delay.
type dismissMessageMsg struct{ seq int }

// toast represents a floating notification that auto-dismisses.
type toast struct {
	id   int
	text string
}

type tuiModel struct {
	table        table.Model
	entries      []Entry
	selected     map[int]bool
	message      string // status message after an action
	messageSeq   int    // incremented on each new message, used for auto-dismiss
	busy         bool   // true while an async operation is running
	quitting     bool
	toasts       []toast // active toast notifications (errors)
	toastSeq     int     // auto-incrementing toast ID
	sizesLoaded  bool    // true once async size computation has completed
	generation   int     // incremented on each reload, guards against stale sizeResultMsg
	windowHeight int     // terminal height from tea.WindowSizeMsg
	showHelp     bool    // toggle extended help overlay
	confirmForce bool    // true = pending force-delete confirmation
	confirmClean bool    // true = pending clean confirmation
}

// RunTUI launches the interactive worktree manager.
func RunTUI() error {
	entries, err := List()
	if err != nil {
		return err
	}

	m := newTUIModel(entries)
	p := tea.NewProgram(&m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}

// humanSize formats a byte count into a human-readable string.
func humanSize(bytes int64) string {
	switch {
	case bytes == 0:
		return "-"
	case bytes < 1024:
		return fmt.Sprintf("%d B", bytes)
	case bytes < 1024*1024:
		return fmt.Sprintf("%.0f KB", float64(bytes)/1024)
	case bytes < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	default:
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
	}
}

// relativeTime formats a timestamp as a human-readable relative duration.
func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		months := int(d.Hours() / 24 / 30)
		if months < 1 {
			months = 1
		}
		return fmt.Sprintf("%d mo ago", months)
	}
}

func newTUIModel(entries []Entry) tuiModel {
	// Wider columns to accommodate ANSI color codes in cell values
	columns := []table.Column{
		{Title: " ", Width: 4},
		{Title: "Path", Width: 32},
		{Title: "Branch", Width: 24},
		{Title: "Status", Width: 18},
		{Title: "Dirty", Width: 8},
		{Title: "Last Active", Width: 12},
		{Title: "Size", Width: 8},
	}

	rows := make([]table.Row, len(entries))
	for i := range entries {
		rows[i] = entryToRow(&entries[i], false, false)
	}

	totalWidth := 0
	for _, c := range columns {
		totalWidth += c.Width + 2
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithWidth(totalWidth),
		table.WithFocused(true),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		Bold(true).
		Foreground(colorSubtle)
	s.Selected = s.Selected.
		Foreground(colorWhite).
		Background(lipgloss.Color("236")).
		Bold(true)
	t.SetStyles(s)

	t.SetRows(rows)
	// Initial height before WindowSizeMsg; will be recalculated on resize
	t.SetHeight(min(len(entries)+1, 25))

	m := tuiModel{
		table:    t,
		entries:  entries,
		selected: make(map[int]bool),
	}
	return m
}

func entryToRow(e *Entry, selected, sizesLoaded bool) table.Row {
	// Selection indicator column
	check := " "
	if selected {
		check = styleCheck.Render("✓")
	}
	if e.IsMain {
		check = styleMain.Render("★")
	} else if e.Locked {
		check = styleLock.Render("🔒")
	} else if e.IsCurrent {
		check = styleMain.Render("▸")
	}

	// Path with dimmed prefix
	path := shortenPath(e.Path)
	if strings.HasPrefix(path, ".worktrees/") {
		path = styleDimPath.Render(".worktrees/") + stylePath.Render(strings.TrimPrefix(path, ".worktrees/"))
	} else {
		path = stylePath.Render(path)
	}

	// Branch with color
	branch := e.Branch
	if branch == "" {
		branch = styleDimPath.Render("(detached)")
	} else {
		branch = styleBranch.Render(branch)
	}

	// Status with icon and color, add lock/current tag
	icon := statusIcon[e.Status]
	stStyle := styleStatus[e.Status]
	statusText := icon + " " + e.Status.String()
	if e.Locked {
		statusText += styleLock.Render(" 🔒")
	} else if e.IsCurrent {
		statusText += styleLock.Render(" cwd")
	}
	status := stStyle.Render(statusText)

	// Dirty column — highlight if there are uncommitted changes
	var dirty string
	if e.Dirty > 0 {
		dirty = lipgloss.NewStyle().Foreground(colorRed).Bold(true).Render(fmt.Sprintf("%d", e.Dirty))
	} else {
		dirty = styleDimPath.Render("-")
	}

	// Last active column
	lastActive := styleDimPath.Render(relativeTime(e.LastActive))

	// Size column — show placeholder until async computation finishes
	var size string
	if !sizesLoaded {
		size = styleDimPath.Render("...")
	} else if e.DiskSize == 0 {
		size = styleDimPath.Render("-")
	} else {
		size = styleDimPath.Render(humanSize(e.DiskSize))
	}

	return table.Row{check, path, branch, status, dirty, lastActive, size}
}

func shortenPath(p string) string {
	home, err := os.UserHomeDir()
	if err == nil {
		p = strings.Replace(p, home, "~", 1)
	}
	// further shorten .worktrees/ prefix
	if idx := strings.Index(p, ".worktrees/"); idx >= 0 {
		p = ".worktrees/" + filepath.Base(p)
	}
	return p
}

// setMessage sets the status bar message and returns a Cmd to auto-dismiss it.
func (m *tuiModel) setMessage(text string) tea.Cmd {
	m.messageSeq++
	seq := m.messageSeq
	m.message = text
	return tea.Tick(toastDuration, func(time.Time) tea.Msg {
		return dismissMessageMsg{seq: seq}
	})
}

// pushToast adds an error toast and returns a Cmd to auto-dismiss it.
func (m *tuiModel) pushToast(text string) tea.Cmd {
	m.toastSeq++
	id := m.toastSeq
	m.toasts = append(m.toasts, toast{id: id, text: text})
	return tea.Tick(toastDuration, func(time.Time) tea.Msg {
		return dismissToastMsg{id: id}
	})
}

// chromeLines is the number of vertical lines consumed by non-table UI elements:
// title (1) + margin (1) + blank (2) + post-table blank (2) + help (1) + trailing newline (1) = 8.
const chromeLines = 8

// tableHeight computes the ideal table height based on terminal size and entry count.
// Falls back to a sensible default when the terminal size is not yet known.
func (m *tuiModel) tableHeight() int {
	rows := len(m.entries) + 1 // +1 for header
	if m.windowHeight > 0 {
		available := m.windowHeight - chromeLines
		if available < 5 {
			available = 5
		}
		return min(rows, available)
	}
	// Before first WindowSizeMsg, use a conservative default
	return min(rows, 25)
}

func (m *tuiModel) Init() tea.Cmd {
	return m.computeSizesCmd()
}

// computeSizesCmd returns a tea.Cmd that computes disk sizes for all entries in the background.
// Captures the current generation to discard stale results after a reload.
func (m *tuiModel) computeSizesCmd() tea.Cmd {
	entries := m.entries
	gen := m.generation
	return func() tea.Msg {
		sizes := make(map[int]int64, len(entries))
		for i, e := range entries {
			if e.Prunable {
				continue
			}
			if _, err := os.Stat(e.Path); err != nil {
				continue
			}
			sizes[i] = dirSize(e.Path)
		}
		return sizeResultMsg{generation: gen, sizes: sizes}
	}
}

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case dismissToastMsg:
		m.handleDismissToast(msg)
	case dismissMessageMsg:
		m.handleDismissMessage(msg)
	case sizeResultMsg:
		m.handleSizeResult(msg)
	case deleteResultMsg:
		cmd = m.handleDeleteResult(msg)
	case pruneResultMsg:
		cmd = m.handlePruneResult(msg)
	case reloadResultMsg:
		cmd = m.handleReloadResult(msg)
	case tea.WindowSizeMsg:
		m.windowHeight = msg.Height
		m.table.SetHeight(m.tableHeight())
	case tea.KeyPressMsg:
		if !m.busy {
			return m.handleKeyPress(msg)
		}
	default:
		m.table, cmd = m.table.Update(msg)
	}
	return m, cmd
}

func (m *tuiModel) handleDismissToast(msg dismissToastMsg) {
	for i, t := range m.toasts {
		if t.id == msg.id {
			m.toasts = append(m.toasts[:i], m.toasts[i+1:]...)
			break
		}
	}
}

func (m *tuiModel) handleDismissMessage(msg dismissMessageMsg) {
	if msg.seq == m.messageSeq {
		m.message = ""
	}
}

func (m *tuiModel) handleSizeResult(msg sizeResultMsg) {
	if msg.generation != m.generation {
		return
	}
	for i, sz := range msg.sizes {
		if i < len(m.entries) {
			m.entries[i].DiskSize = sz
		}
	}
	m.sizesLoaded = true
	m.refreshRows()
}

func (m *tuiModel) handleDeleteResult(msg deleteResultMsg) tea.Cmd {
	m.busy = false
	var cmds []tea.Cmd
	cmds = append(cmds, m.setMessage(fmt.Sprintf("Removed %d worktree(s), freed %s", msg.removed, humanSize(msg.freedBytes))))
	for _, errText := range msg.errors {
		cmds = append(cmds, m.pushToast(errText))
	}
	cmds = append(cmds, m.reloadCmd())
	return tea.Batch(cmds...)
}

func (m *tuiModel) handlePruneResult(msg pruneResultMsg) tea.Cmd {
	m.busy = false
	if msg.err != nil {
		return m.pushToast(msg.err.Error())
	}
	return tea.Batch(m.setMessage("Pruned stale worktree references"), m.reloadCmd())
}

func (m *tuiModel) handleReloadResult(msg reloadResultMsg) tea.Cmd {
	m.busy = false
	if msg.err != nil {
		return m.pushToast(msg.err.Error())
	}
	m.entries = msg.entries
	m.selected = make(map[int]bool)
	m.sizesLoaded = false
	m.generation++
	m.refreshRows()
	m.table.SetHeight(m.tableHeight())
	sizeCmd := m.computeSizesCmd()
	if m.message == "Refreshing..." {
		return tea.Batch(m.setMessage("Refreshed"), sizeCmd)
	}
	return sizeCmd
}

// handleKeyPress processes keyboard input, extracted from Update to reduce cyclomatic complexity.
func (m *tuiModel) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle confirmation prompts first
	if m.confirmForce || m.confirmClean {
		return m.handleConfirmKey(key)
	}

	// Help overlay dismissal
	if m.showHelp {
		m.showHelp = false
		return m, nil
	}

	switch key {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "j", "down":
		m.table.MoveDown(1)
		return m, nil
	case "k", "up":
		m.table.MoveUp(1)
		return m, nil
	case "g":
		m.table.GotoTop()
		return m, nil
	case "G":
		m.table.GotoBottom()
		return m, nil

	case "enter":
		cmd := m.showSelectedPath()
		return m, cmd

	case "?":
		m.showHelp = !m.showHelp
		return m, nil

	case "space":
		m.toggleSelection()
		return m, nil

	case "a":
		m.selectAllMerged()
		return m, nil

	case "d":
		m.requestDelete(true)
		return m, nil

	case "c":
		m.requestDelete(false)
		return m, nil

	case "C":
		m.selectAllMerged()
		m.requestDelete(false)
		return m, nil

	case "p":
		m.busy = true
		m.message = "Pruning..."
		return m, func() tea.Msg {
			err := Prune()
			return pruneResultMsg{err: err}
		}

	case "r":
		m.busy = true
		m.message = "Refreshing..."
		cmd := m.reloadCmd()
		return m, cmd
	}

	return m, nil
}

// handleConfirmKey handles y/n input during delete confirmation.
func (m *tuiModel) handleConfirmKey(key string) (tea.Model, tea.Cmd) {
	force := m.confirmForce
	m.confirmForce = false
	m.confirmClean = false

	switch key {
	case "y", "Y":
		cmd := m.deleteSelectedCmd(force)
		return m, cmd
	default:
		m.selected = make(map[int]bool)
		m.refreshRows()
		m.message = ""
		return m, nil
	}
}

// toggleSelection toggles the selection state of the cursor entry.
func (m *tuiModel) toggleSelection() {
	idx := m.table.Cursor()
	if idx < len(m.entries) && !m.entries[idx].Protected() {
		if m.selected[idx] {
			delete(m.selected, idx)
		} else {
			m.selected[idx] = true
			m.table.MoveDown(1)
		}
		m.refreshRows()
	}
}

// selectAllMerged selects all merged, non-protected worktrees.
func (m *tuiModel) selectAllMerged() {
	for i, e := range m.entries {
		if e.Status == StatusMerged && !e.Protected() {
			m.selected[i] = true
		}
	}
	m.refreshRows()
}

// requestDelete enters confirmation mode before deleting selected worktrees.
func (m *tuiModel) requestDelete(force bool) {
	count := len(m.selected)
	if count == 0 {
		return
	}
	if force {
		m.confirmForce = true
	} else {
		m.confirmClean = true
	}
	action := "clean"
	if force {
		action = "force-delete"
	}
	m.message = fmt.Sprintf("%s %d worktree(s)? (y/n)", action, count)
}

// showSelectedPath displays the full path of the cursor entry in the status bar.
func (m *tuiModel) showSelectedPath() tea.Cmd {
	idx := m.table.Cursor()
	if idx >= len(m.entries) {
		return nil
	}
	return m.setMessage(m.entries[idx].Path)
}

// deleteSelectedCmd returns a tea.Cmd that removes selected worktrees in the background.
func (m *tuiModel) deleteSelectedCmd(force bool) tea.Cmd {
	type target struct {
		path     string
		branch   string
		diskSize int64 // cached size from entry, or computed fresh if not loaded
	}
	var targets []target
	for idx, sel := range m.selected {
		if !sel || idx >= len(m.entries) {
			continue
		}
		e := m.entries[idx]
		if e.Protected() {
			continue
		}
		targets = append(targets, target{path: e.Path, branch: e.Branch, diskSize: e.DiskSize})
	}
	if len(targets) == 0 {
		return nil
	}

	m.busy = true
	m.message = fmt.Sprintf("Removing %d worktree(s)...", len(targets))
	m.selected = make(map[int]bool)
	m.refreshRows()

	return func() tea.Msg {
		removed := 0
		var freedBytes int64
		var errors []string
		for _, t := range targets {
			// Use cached size if available, otherwise compute fresh
			sz := t.diskSize
			if sz == 0 {
				sz = dirSize(t.path)
			}
			if err := Remove(t.path, force); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %s", shortenPath(t.path), err))
				continue
			}
			freedBytes += sz
			if t.branch != "" {
				if branchErr := DeleteBranch(t.branch, force); branchErr != nil {
					errors = append(errors, fmt.Sprintf("branch %s: %s", t.branch, branchErr))
				}
			}
			removed++
		}
		if pruneErr := Prune(); pruneErr != nil {
			errors = append(errors, fmt.Sprintf("prune: %s", pruneErr))
		}
		return deleteResultMsg{removed: removed, errors: errors, freedBytes: freedBytes}
	}
}

// reloadCmd returns a tea.Cmd that refreshes the worktree list in the background.
func (m *tuiModel) reloadCmd() tea.Cmd {
	m.busy = true
	return func() tea.Msg {
		entries, err := List()
		return reloadResultMsg{entries: entries, err: err}
	}
}

func (m *tuiModel) refreshRows() {
	rows := make([]table.Row, len(m.entries))
	for i := range m.entries {
		rows[i] = entryToRow(&m.entries[i], m.selected[i], m.sizesLoaded)
	}
	m.table.SetRows(rows)
}

// helpItem renders a single "key desc" help entry with styled key.
func helpItem(key, desc string) string {
	return styleHelpKey.Render(key) + " " + styleHelpDesc.Render(desc)
}

// renderFullHelp returns the extended help overlay text.
func (m *tuiModel) renderFullHelp() string {
	_ = m // receiver used for consistency
	title := styleTitle.Render("Keyboard Shortcuts")
	lines := []struct{ key, desc string }{
		{"j/k ↑/↓", "Move cursor up/down"},
		{"g/G", "Jump to top/bottom"},
		{"space", "Toggle selection on cursor entry"},
		{"a", "Select all merged worktrees"},
		{"enter", "Show full path of cursor entry"},
		{"c", "Clean (remove) selected worktrees"},
		{"C", "Clean ALL merged worktrees at once"},
		{"d", "Force-delete selected worktrees"},
		{"p", "Prune stale worktree references"},
		{"r", "Refresh worktree list"},
		{"?", "Toggle this help"},
		{"q", "Quit"},
	}
	var b strings.Builder
	b.WriteString("  " + title + "\n\n")
	for _, l := range lines {
		fmt.Fprintf(&b, "  %s  %s\n",
			styleHelpKey.Render(fmt.Sprintf("%-12s", l.key)),
			styleHelpDesc.Render(l.desc))
	}
	b.WriteString("\n  " + styleHelpDesc.Render("Press any key to dismiss") + "\n")
	return b.String()
}

func (m *tuiModel) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}

	var b strings.Builder

	// Title bar with worktree count
	selected := len(m.selected)
	title := "Worktree Manager"
	counter := styleCount.Render(fmt.Sprintf("  %d worktrees", len(m.entries)))
	if selected > 0 {
		counter = styleCheck.Render(fmt.Sprintf("  %d selected", selected))
	}
	b.WriteString(styleTitle.Render(title) + counter)
	b.WriteString("\n\n")

	// Table
	b.WriteString(m.table.View())
	b.WriteString("\n\n")

	// Status message
	if m.busy {
		b.WriteString(styleBusy.Render("  " + m.message))
		b.WriteString("\n\n")
	} else if m.message != "" {
		b.WriteString(styleMessage.Render("  " + m.message))
		b.WriteString("\n\n")
	}

	// Floating toast notifications (errors)
	for _, t := range m.toasts {
		b.WriteString(styleToastBox.Render(styleToastText.Render("  " + t.text)))
		b.WriteString("\n")
	}
	if len(m.toasts) > 0 {
		b.WriteString("\n")
	}

	// Help bar or full help overlay
	if m.showHelp {
		b.WriteString(m.renderFullHelp())
	} else {
		sep := styleHelpSep.Render(" · ")
		helpLine := strings.Join([]string{
			helpItem("space", "select") + sep + helpItem("a", "all merged"),
			helpItem("c", "clean") + sep + helpItem("d", "force del"),
			helpItem("enter", "path") + sep + helpItem("?", "help") + sep + helpItem("q", "quit"),
		}, styleHelpSep.Render("  │  "))
		b.WriteString("  " + helpLine)
		b.WriteString("\n")
	}

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}
