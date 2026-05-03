package lazytask

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ViewMode int

const (
	ViewToday ViewMode = iota
	ViewWeek
)

type Model struct {
	store    *Store
	now      func() time.Time
	mode     ViewMode
	selected int
	width    int
	height   int
	form     *taskForm
	err      string
}

func NewModel(store *Store) Model {
	return Model{
		store: store,
		now:   time.Now,
		mode:  ViewToday,
	}
}

func RunTUI(path string) error {
	store, err := NewStore(NewEventLog(path))
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(NewModel(store), tea.WithAltScreen()).Run()
	return err
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	if m.form != nil {
		form, action, cmd := m.form.Update(msg)
		m.form = form
		switch action {
		case formCancel:
			m.form = nil
			m.err = ""
		case formSave:
			if err := m.saveForm(); err != nil {
				m.err = err.Error()
				m.form.err = m.err
			} else {
				m.form = nil
				m.err = ""
				m.clampSelection()
			}
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
		if m.mode == ViewToday {
			m.mode = ViewWeek
		} else {
			m.mode = ViewToday
		}
		m.selected = 0
		m.err = ""
	case "j", "down":
		m.selected++
		m.clampSelection()
	case "k", "up":
		m.selected--
		m.clampSelection()
	case "a":
		m.form = newTaskForm("", TaskInput{When: FormatLocalDate(m.now())})
	case "e":
		if task, ok := m.selectedTask(); ok {
			m.form = newTaskForm(task.ID, task.Input())
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
	if m.form != nil {
		return m.form.View(m.width)
	}

	title := "LazyTask"
	tab := "Today"
	if m.mode == ViewWeek {
		tab = "Week"
	}
	header := headerStyle.Render(title) + " " + subtleStyle.Render(tab)
	if m.err != "" {
		header += " " + errorStyle.Render(m.err)
	}

	var body string
	if m.mode == ViewToday {
		body = m.todayView()
	} else {
		body = m.weekView()
	}
	help := subtleStyle.Render("tab switch  j/k move  a add  e edit  space complete  d delete  q quit")
	return lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", help)
}

func (m Model) todayView() string {
	tasks := TodayTasks(m.store.List(), m.now())
	if len(tasks) == 0 {
		return subtleStyle.Render("No tasks for today. Press a to add one.")
	}
	lines := make([]string, 0, len(tasks))
	for i, task := range tasks {
		lines = append(lines, m.taskLine(task, i == m.selected))
	}
	return strings.Join(lines, "\n")
}

func (m Model) weekView() string {
	week := WorkWeek(m.store.List(), m.now())
	flat := FlattenWeek(week)
	selectedID := ""
	if m.selected >= 0 && m.selected < len(flat) {
		selectedID = flat[m.selected].ID
	}
	width := 24
	if m.width > 20 {
		width = max(18, (m.width-8)/5)
	}
	columns := make([]string, 0, len(week))
	for _, day := range week {
		lines := []string{dayStyle.Render(day.Label + " " + day.Date)}
		if len(day.Tasks) == 0 {
			lines = append(lines, subtleStyle.Render("No tasks"))
		}
		for _, task := range day.Tasks {
			lines = append(lines, truncate(m.taskLine(task, task.ID == selectedID), width-1))
		}
		columns = append(columns, lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n")))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, columns...)
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
	meta := taskMeta(task)
	line := fmt.Sprintf("%s %s %s", cursor, check, task.Title)
	if meta != "" {
		line += subtleStyle.Render("  " + meta)
	}
	if selected {
		return selectedStyle.Render(line)
	}
	if task.CompletedAt != "" {
		return doneStyle.Render(line)
	}
	return line
}

func taskMeta(task Task) string {
	parts := make([]string, 0, 4)
	if task.Project != "" {
		parts = append(parts, task.Project)
	}
	if task.Area != "" {
		parts = append(parts, task.Area)
	}
	if task.Deadline != "" {
		parts = append(parts, "due "+task.Deadline)
	}
	for _, tag := range task.Tags {
		parts = append(parts, "#"+tag)
	}
	return strings.Join(parts, " ")
}

func (m *Model) saveForm() error {
	input := m.form.Input()
	if m.form.taskID == "" {
		_, err := m.store.Create(input)
		return err
	}
	_, err := m.store.Update(m.form.taskID, input)
	return err
}

func (m Model) selectedTask() (Task, bool) {
	tasks := m.visibleTasks()
	if m.selected < 0 || m.selected >= len(tasks) {
		return Task{}, false
	}
	return tasks[m.selected], true
}

func (m Model) visibleTasks() []Task {
	if m.mode == ViewWeek {
		return FlattenWeek(WorkWeek(m.store.List(), m.now()))
	}
	return TodayTasks(m.store.List(), m.now())
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

type formAction int

const (
	formNone formAction = iota
	formSave
	formCancel
)

type taskForm struct {
	taskID string
	inputs []textinput.Model
	focus  int
	err    string
}

func newTaskForm(taskID string, input TaskInput) *taskForm {
	labels := []struct {
		placeholder string
		value       string
	}{
		{"title", input.Title},
		{"notes", input.Notes},
		{"when YYYY-MM-DD", input.When},
		{"deadline YYYY-MM-DD", input.Deadline},
		{"project", input.Project},
		{"area", input.Area},
		{"tags comma,separated", JoinTags(input.Tags)},
	}
	inputs := make([]textinput.Model, len(labels))
	for i, field := range labels {
		ti := textinput.New()
		ti.Placeholder = field.placeholder
		ti.SetValue(field.value)
		ti.CharLimit = 240
		ti.Width = 48
		if i == 0 {
			ti.Focus()
		}
		inputs[i] = ti
	}
	return &taskForm{taskID: taskID, inputs: inputs}
}

func (f *taskForm) Update(msg tea.Msg) (*taskForm, formAction, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if ok {
		switch key.String() {
		case "ctrl+c":
			return f, formCancel, tea.Quit
		case "esc":
			return f, formCancel, nil
		case "enter":
			if f.focus == len(f.inputs)-1 {
				return f, formSave, nil
			}
			f.next()
			return f, formNone, nil
		case "tab", "down":
			f.next()
			return f, formNone, nil
		case "shift+tab", "up":
			f.prev()
			return f, formNone, nil
		}
	}
	var cmd tea.Cmd
	f.inputs[f.focus], cmd = f.inputs[f.focus].Update(msg)
	return f, formNone, cmd
}

func (f *taskForm) View(width int) string {
	title := "Add Task"
	if f.taskID != "" {
		title = "Edit Task"
	}
	lines := []string{headerStyle.Render(title)}
	names := []string{"Title", "Notes", "When", "Deadline", "Project", "Area", "Tags"}
	for i, input := range f.inputs {
		label := names[i]
		if i == f.focus {
			label = selectedStyle.Render(label)
		} else {
			label = subtleStyle.Render(label)
		}
		lines = append(lines, label)
		lines = append(lines, input.View())
	}
	if f.err != "" {
		lines = append(lines, errorStyle.Render(f.err))
	}
	lines = append(lines, subtleStyle.Render("enter next/save  tab move  esc cancel"))
	return strings.Join(lines, "\n")
}

func (f *taskForm) Input() TaskInput {
	return TaskInput{
		Title:    f.inputs[0].Value(),
		Notes:    f.inputs[1].Value(),
		When:     f.inputs[2].Value(),
		Deadline: f.inputs[3].Value(),
		Project:  f.inputs[4].Value(),
		Area:     f.inputs[5].Value(),
		Tags:     SplitTags(f.inputs[6].Value()),
	}
}

func (f *taskForm) next() {
	f.inputs[f.focus].Blur()
	f.focus = (f.focus + 1) % len(f.inputs)
	f.inputs[f.focus].Focus()
}

func (f *taskForm) prev() {
	f.inputs[f.focus].Blur()
	f.focus--
	if f.focus < 0 {
		f.focus = len(f.inputs) - 1
	}
	f.inputs[f.focus].Focus()
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

var (
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	subtleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("57"))
	doneStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Strikethrough(true)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	dayStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
)
