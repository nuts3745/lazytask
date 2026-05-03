package lazytask

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	store    *Store
	now      func() time.Time
	view     ViewKind
	filter   Filter
	selected int
	width    int
	height   int
	prompt   *linePrompt
	err      string
}

func NewModel(store *Store) Model {
	return Model{store: store, now: time.Now, view: KindInbox}
}

func RunTUI(path string) error {
	store, err := NewStore(NewEventLog(path))
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(NewModel(store), tea.WithAltScreen()).Run()
	return err
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}
	if m.prompt != nil {
		prompt, action, cmd := m.prompt.Update(msg)
		m.prompt = prompt
		switch action {
		case promptCancel:
			m.prompt = nil
		case promptSubmit:
			m.err = errorString(m.runPrompt())
			m.prompt = nil
			m.clampSelection()
		}
		return m, cmd
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "tab":
		m.nextView()
	case "j", "down":
		m.selected++
		m.clampSelection()
	case "k", "up":
		m.selected--
		m.clampSelection()
	case "a":
		m.prompt = newLinePrompt(promptAdd, "add")
	case "/":
		m.prompt = newLinePrompt(promptSearch, "search")
	case ":":
		m.prompt = newLinePrompt(promptCommand, "command")
	case "e":
		if task, ok := m.selectedTask(); ok {
			m.prompt = newLinePrompt(promptCommand, "edit")
			m.prompt.input.SetValue("update " + encodeQuickTask(task.Input()))
		}
	case "t":
		if task, ok := m.selectedTask(); ok {
			input := task.Input()
			input.Start = StartDate
			input.StartDate = FormatLocalDate(m.now())
			_, err := m.store.Update(task.ID, input)
			m.err = errorString(err)
			m.clampSelection()
		}
	case " ":
		if task, ok := m.selectedTask(); ok {
			if task.CompletedAt == "" {
				m.err = errorString(m.store.Complete(task.ID, FormatLocalDate(m.now())))
			} else {
				m.err = errorString(m.store.Uncomplete(task.ID))
			}
		}
	case "d", "delete", "backspace":
		if task, ok := m.selectedTask(); ok {
			m.err = errorString(m.store.Delete(task.ID))
			m.clampSelection()
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.prompt != nil {
		return m.prompt.View(m)
	}
	header := m.headerView()
	if m.err != "" {
		header += " " + errorStyle.Render(m.err)
	}
	help := helpStyle.Render("[tab] views  [a] capture  [/] scan  [:] command  [j/k] lock  [t] today  [space] done  [d] delete  [q] exit")
	body := m.listView(m.bodyHeight())
	return lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", help)
}

func (m Model) headerView() string {
	title := headerStyle.Render(" LAZYTASK // OPS CONSOLE ")
	view := statusStyle.Render(" VIEW [" + strings.ToUpper(m.title()) + "] ")
	count := subtleStyle.Render(fmt.Sprintf(" TARGETS %02d ", len(m.visibleTasks())))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, " ", view, " ", count)
}

func (m Model) bodyHeight() int {
	if m.height <= 0 {
		return 0
	}
	return max(1, m.height-4)
}

func (m Model) title() string {
	if m.view != KindFilter {
		return string(m.view)
	}
	if m.filter.Tag != "" {
		return "#" + m.filter.Tag
	}
	if m.filter.Project != "" {
		return ">" + m.filter.Project
	}
	if m.filter.Area != "" {
		return "/" + m.filter.Area
	}
	return "Search " + m.filter.Query
}

func (m Model) listView(height int) string {
	if m.view == KindWeekly {
		return m.weeklyView(height)
	}
	tasks := m.visibleTasks()
	if len(tasks) == 0 {
		return fillHeight(subtleStyle.Render("No tasks. Press a to capture one."), height)
	}
	lines := make([]string, 0, len(tasks)+1)
	for _, i := range visibleIndexes(len(tasks), m.selected, height) {
		lines = append(lines, m.taskLine(tasks[i], i == m.selected))
	}
	if height > 0 && len(tasks) > len(lines) {
		lines = append(lines, subtleStyle.Render(fmt.Sprintf("+%d more", len(tasks)-len(lines))))
	}
	return fillHeight(strings.Join(lines, "\n"), height)
}

func (m Model) weeklyView(height int) string {
	week := WorkWeek(m.store.List(), m.now())
	flat := FlattenWeek(week)
	selectedID := ""
	if m.selected >= 0 && m.selected < len(flat) {
		selectedID = flat[m.selected].ID
	}
	width := 24
	if m.width > 20 {
		width = max(16, (m.width-4)/5)
	}
	taskHeight := 0
	if height > 0 {
		taskHeight = max(0, height-2)
	}
	headerCells := make([]string, 0, len(week))
	separatorCells := make([]string, 0, len(week))
	bodyColumns := make([]string, 0, len(week))
	for _, day := range week {
		visibleTasks := day.Tasks
		if taskHeight > 0 {
			visibleTasks = visibleWeeklyTasks(day.Tasks, selectedID, taskHeight)
		}
		headerCells = append(headerCells, weeklyColumnStyle.Width(width).Render(dayStyle.Render("["+day.Label+" "+day.Date[5:]+"]")))
		separatorCells = append(separatorCells, weeklyColumnStyle.Width(width).Render(gridStyle.Render(strings.Repeat("═", max(1, width-1)))))

		lines := make([]string, 0, len(visibleTasks)+1)
		if len(day.Tasks) == 0 {
			lines = append(lines, subtleStyle.Render("standby"))
		}
		for _, task := range visibleTasks {
			lines = append(lines, m.weeklyTaskLine(task, task.ID == selectedID, width-1))
		}
		if len(visibleTasks) < len(day.Tasks) {
			lines = append(lines, subtleStyle.Render(fmt.Sprintf("+%d more", len(day.Tasks)-len(visibleTasks))))
		}
		body := strings.Join(lines, "\n")
		if taskHeight > 0 {
			body = fillHeight(body, taskHeight)
		}
		bodyColumns = append(bodyColumns, weeklyColumnStyle.Width(width).Render(body))
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top, headerCells...)
	separator := lipgloss.JoinHorizontal(lipgloss.Top, separatorCells...)
	body := lipgloss.JoinHorizontal(lipgloss.Top, bodyColumns...)
	return fillHeight(lipgloss.JoinVertical(lipgloss.Left, header, separator, body), height)
}

func (m Model) taskLine(task Task, selected bool) string {
	cursor := " "
	if selected {
		cursor = ">"
	}
	check := "[ ]"
	if task.CompletedAt != "" {
		check = "[x]"
	}
	if task.CanceledAt != "" {
		check = "[-]"
	}
	line := fmt.Sprintf("%s %s %s", cursor, check, task.Title)
	if meta := taskMeta(task); meta != "" {
		if task.CompletedAt != "" || task.CanceledAt != "" {
			line += "  " + meta
		} else {
			line += metaStyle.Render("  " + meta)
		}
	}
	if selected {
		return selectedStyle.Render(line)
	}
	if task.CompletedAt != "" || task.CanceledAt != "" {
		return doneStyle.Render(line)
	}
	return line
}

func (m Model) weeklyTaskLine(task Task, selected bool, width int) string {
	cursor := " "
	if selected {
		cursor = ">"
	}
	check := "[ ]"
	if task.CompletedAt != "" {
		check = "[x]"
	}
	if task.CanceledAt != "" {
		check = "[-]"
	}
	prefix := fmt.Sprintf("%s %s ", cursor, check)
	meta := compactWeeklyMeta(task)
	available := width - len(prefix)
	if meta != "" {
		available -= len(meta) + 1
	}
	if available < 4 {
		available = 4
	}
	line := prefix + truncateRunes(task.Title, available)
	if meta != "" {
		line += " " + meta
	}
	if selected {
		return selectedStyle.Render(line)
	}
	if task.CompletedAt != "" || task.CanceledAt != "" {
		return doneStyle.Render(line)
	}
	return line
}

func taskMeta(task Task) string {
	parts := make([]string, 0, 6)
	if task.Start == StartDate && task.StartDate != "" {
		parts = append(parts, "@"+task.StartDate)
	} else if task.Start != "" {
		parts = append(parts, "@"+string(task.Start))
	}
	if task.Deadline != "" {
		parts = append(parts, "!"+task.Deadline)
	}
	if task.Project != "" {
		parts = append(parts, ">"+task.Project)
	}
	if task.Area != "" {
		parts = append(parts, "/"+task.Area)
	}
	for _, tag := range task.Tags {
		parts = append(parts, "#"+tag)
	}
	return strings.Join(parts, " ")
}

func compactWeeklyMeta(task Task) string {
	if task.Deadline != "" {
		return "!" + task.Deadline[5:]
	}
	if len(task.Tags) > 0 {
		return "#" + task.Tags[0]
	}
	if task.Project != "" {
		return ">" + task.Project
	}
	return ""
}

func (m Model) visibleTasks() []Task {
	tasks := m.store.List()
	switch m.view {
	case KindInbox:
		return InboxTasks(tasks)
	case KindToday:
		return TodayTasks(tasks, m.now())
	case KindWeekly:
		return FlattenWeek(WorkWeek(tasks, m.now()))
	case KindAnytime:
		return AnytimeTasks(tasks)
	case KindSomeday:
		return SomedayTasks(tasks)
	case KindLogbook:
		return LogbookTasks(tasks)
	case KindFilter:
		return FilteredTasks(tasks, m.filter)
	default:
		return InboxTasks(tasks)
	}
}

func (m Model) selectedTask() (Task, bool) {
	tasks := m.visibleTasks()
	if m.selected < 0 || m.selected >= len(tasks) {
		return Task{}, false
	}
	return tasks[m.selected], true
}

func (m *Model) nextView() {
	order := []ViewKind{KindInbox, KindToday, KindWeekly, KindAnytime, KindSomeday, KindLogbook}
	for i, kind := range order {
		if m.view == kind {
			m.view = order[(i+1)%len(order)]
			m.filter = Filter{}
			m.selected = 0
			m.err = ""
			return
		}
	}
	m.view = KindInbox
	m.filter = Filter{}
	m.selected = 0
}

func (m *Model) clampSelection() {
	tasks := m.visibleTasks()
	if len(tasks) == 0 {
		m.selected = 0
		return
	}
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(tasks) {
		m.selected = len(tasks) - 1
	}
}

func (m *Model) runPrompt() error {
	value := strings.TrimSpace(m.prompt.input.Value())
	switch m.prompt.kind {
	case promptAdd:
		input, err := ParseQuickTask(value, m.now())
		if err != nil {
			return err
		}
		m.applyCreateDefaults(&input)
		_, err = m.store.Create(input)
		return err
	case promptSearch:
		m.applySearch(value)
		return nil
	case promptCommand:
		return m.runCommand(value)
	default:
		return nil
	}
}

func (m *Model) applySearch(value string) {
	value = strings.TrimSpace(value)
	m.view = KindFilter
	m.filter = Filter{}
	switch {
	case strings.HasPrefix(value, "#"):
		m.filter.Tag = strings.TrimPrefix(value, "#")
	case strings.HasPrefix(value, ">"):
		m.filter.Project = strings.TrimPrefix(value, ">")
	case strings.HasPrefix(value, "/"):
		m.filter.Area = strings.TrimPrefix(value, "/")
	default:
		switch strings.ToLower(value) {
		case "inbox":
			m.view = KindInbox
		case "today":
			m.view = KindToday
		case "weekly", "week":
			m.view = KindWeekly
		case "anytime":
			m.view = KindAnytime
		case "someday":
			m.view = KindSomeday
		case "logbook":
			m.view = KindLogbook
		default:
			m.filter.Query = value
		}
	}
	m.selected = 0
}

func (m *Model) runCommand(value string) error {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return nil
	}
	cmd := strings.ToLower(parts[0])
	rest := strings.TrimSpace(strings.TrimPrefix(value, parts[0]))
	switch cmd {
	case "add":
		input, err := ParseQuickTask(rest, m.now())
		if err != nil {
			return err
		}
		m.applyCreateDefaults(&input)
		_, err = m.store.Create(input)
		return err
	case "update":
		task, ok := m.selectedTask()
		if !ok {
			return errors.New("no selected task")
		}
		input, err := ParseQuickTask(rest, m.now())
		if err != nil {
			return err
		}
		_, err = m.store.Update(task.ID, input)
		return err
	case "tag", "untag", "move", "area", "when", "deadline":
		return m.updateSelected(cmd, rest)
	case "done":
		if task, ok := m.selectedTask(); ok {
			return m.store.Complete(task.ID, FormatLocalDate(m.now()))
		}
	case "undone":
		if task, ok := m.selectedTask(); ok {
			return m.store.Uncomplete(task.ID)
		}
	case "cancel":
		if task, ok := m.selectedTask(); ok {
			return m.store.Cancel(task.ID, FormatLocalDate(m.now()))
		}
	case "delete":
		if task, ok := m.selectedTask(); ok {
			return m.store.Delete(task.ID)
		}
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
	return errors.New("no selected task")
}

func (m Model) applyCreateDefaults(input *TaskInput) {
	if input.Start != StartInbox || input.StartDate != "" {
		return
	}
	switch m.view {
	case KindToday:
		input.Start = StartDate
		input.StartDate = FormatLocalDate(m.now())
	case KindAnytime:
		input.Start = StartAnytime
	case KindSomeday:
		input.Start = StartSomeday
	case KindFilter:
		if m.filter.Tag != "" {
			input.Tags = normalizeTags(append(input.Tags, m.filter.Tag))
		}
		if m.filter.Project != "" {
			input.Project = m.filter.Project
		}
		if m.filter.Area != "" {
			input.Area = m.filter.Area
		}
	}
}

func (m *Model) updateSelected(cmd, rest string) error {
	task, ok := m.selectedTask()
	if !ok {
		return errors.New("no selected task")
	}
	input := task.Input()
	switch cmd {
	case "tag":
		input.Tags = normalizeTags(append(input.Tags, SplitTags(rest)...))
	case "untag":
		remove := make(map[string]struct{})
		for _, tag := range SplitTags(rest) {
			remove[strings.ToLower(tag)] = struct{}{}
		}
		kept := input.Tags[:0]
		for _, tag := range input.Tags {
			if _, ok := remove[strings.ToLower(tag)]; !ok {
				kept = append(kept, tag)
			}
		}
		input.Tags = kept
	case "move":
		input.Project = strings.TrimPrefix(strings.TrimSpace(rest), ">")
		if input.Project != "" && input.Start == StartInbox {
			input.Start = StartAnytime
		}
	case "area":
		input.Area = strings.TrimPrefix(strings.TrimSpace(rest), "/")
	case "when":
		if err := ApplyWhen(&input, rest, m.now()); err != nil {
			return err
		}
	case "deadline":
		if err := ApplyDeadline(&input, rest); err != nil {
			return err
		}
	}
	_, err := m.store.Update(task.ID, input)
	return err
}

type promptKind int

const (
	promptAdd promptKind = iota
	promptSearch
	promptCommand
)

type promptAction int

const (
	promptNone promptAction = iota
	promptSubmit
	promptCancel
)

type linePrompt struct {
	kind  promptKind
	label string
	input textinput.Model
}

func newLinePrompt(kind promptKind, label string) *linePrompt {
	ti := textinput.New()
	ti.Focus()
	ti.Width = 72
	ti.CharLimit = 500
	return &linePrompt{kind: kind, label: label, input: ti}
}

func (p *linePrompt) Update(msg tea.Msg) (*linePrompt, promptAction, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return p, promptCancel, nil
		case "enter":
			return p, promptSubmit, nil
		case "ctrl+c":
			return p, promptCancel, tea.Quit
		}
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, promptNone, cmd
}

func (p *linePrompt) View(m Model) string {
	lines := []string{headerStyle.Render(" " + strings.ToUpper(p.label) + " "), p.input.View()}
	if p.kind == promptSearch {
		lines = append(lines, "", subtleStyle.Render("try: #urgent  >Work  /Home  today  weekly"))
		lines = append(lines, m.searchHints(p.input.Value())...)
	}
	if p.kind == promptCommand {
		lines = append(lines, "", subtleStyle.Render("add/tag/untag/move/area/when/deadline/done/undone/cancel/delete"))
	}
	lines = append(lines, "", subtleStyle.Render("enter apply  esc cancel"))
	return strings.Join(lines, "\n")
}

func (m Model) searchHints(query string) []string {
	query = strings.ToLower(strings.TrimSpace(query))
	items := []string{"inbox", "today", "weekly", "anytime", "someday", "logbook"}
	for _, tag := range KnownTags(m.store.List()) {
		items = append(items, "#"+tag)
	}
	for _, project := range KnownProjects(m.store.List()) {
		items = append(items, ">"+project)
	}
	for _, area := range KnownAreas(m.store.List()) {
		items = append(items, "/"+area)
	}
	for _, task := range m.store.List() {
		items = append(items, task.Title)
	}
	out := make([]string, 0, 8)
	for _, item := range items {
		if query == "" || strings.Contains(strings.ToLower(item), query) {
			out = append(out, subtleStyle.Render(item))
		}
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func encodeQuickTask(input TaskInput) string {
	parts := []string{input.Title}
	for _, tag := range input.Tags {
		parts = append(parts, "#"+tag)
	}
	switch input.Start {
	case StartDate:
		parts = append(parts, "@"+input.StartDate)
	case StartAnytime, StartSomeday, StartInbox:
		parts = append(parts, "@"+string(input.Start))
	}
	if input.Deadline != "" {
		parts = append(parts, "!"+input.Deadline)
	}
	if input.Project != "" {
		parts = append(parts, ">"+input.Project)
	}
	if input.Area != "" {
		parts = append(parts, "/"+input.Area)
	}
	return strings.Join(parts, " ")
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func truncate(value string, width int) string {
	if width <= 0 || len(value) <= width {
		return value
	}
	if width <= 3 {
		return value[:width]
	}
	return value[:width-3] + "..."
}

func truncateRunes(value string, width int) string {
	runes := []rune(value)
	if width <= 0 || len(runes) <= width {
		return value
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}

func fillHeight(value string, height int) string {
	if height <= 0 {
		return value
	}
	lines := strings.Split(value, "\n")
	if len(lines) >= height {
		return strings.Join(lines[:height], "\n")
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func visibleIndexes(total, selected, height int) []int {
	if total <= 0 {
		return nil
	}
	if height <= 0 || total <= height {
		indexes := make([]int, total)
		for i := range indexes {
			indexes[i] = i
		}
		return indexes
	}
	limit := max(1, height-1)
	if selected < 0 {
		selected = 0
	}
	if selected >= total {
		selected = total - 1
	}
	start := selected - limit/2
	if start < 0 {
		start = 0
	}
	if start+limit > total {
		start = total - limit
	}
	indexes := make([]int, 0, limit)
	for i := start; i < start+limit; i++ {
		indexes = append(indexes, i)
	}
	return indexes
}

func visibleWeeklyTasks(tasks []Task, selectedID string, height int) []Task {
	if len(tasks) == 0 {
		return tasks
	}
	if height <= 0 || len(tasks) <= height {
		return tasks
	}
	selected := -1
	for i, task := range tasks {
		if task.ID == selectedID {
			selected = i
			break
		}
	}
	if selected < 0 {
		return tasks[:height]
	}
	start := selected - height/2
	if start < 0 {
		start = 0
	}
	if start+height > len(tasks) {
		start = len(tasks) - height
	}
	return tasks[start : start+height]
}

var (
	headerStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231")).Background(lipgloss.Color("25"))
	statusStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("51")).Background(lipgloss.Color("236"))
	subtleStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	metaStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("109"))
	selectedStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("16")).Background(lipgloss.Color("51"))
	doneStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Strikethrough(true)
	errorStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203"))
	dayStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("51"))
	gridStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("25"))
	weeklyColumnStyle = lipgloss.NewStyle().PaddingRight(1)
)
