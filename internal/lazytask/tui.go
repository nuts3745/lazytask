package lazytask

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type Model struct {
	store       *Store
	now         func() time.Time
	view        ViewKind
	filter      Filter
	selected    int
	width       int
	height      int
	prompt      *linePrompt
	err         string
	status      string
	focusedPane paneKind
	navSelected int
	navMode     navMode
	helpOpen    bool
}

func NewModel(store *Store) Model {
	return Model{store: store, now: time.Now, view: KindInbox, focusedPane: paneList, navMode: navRoot}
}

type paneKind string

const (
	paneNav    paneKind = "nav"
	paneList   paneKind = "list"
	paneDetail paneKind = "detail"
)

type navMode string

const (
	navRoot     navMode = "root"
	navTags     navMode = "tags"
	navProjects navMode = "projects"
	navAreas    navMode = "areas"
)

type navItem struct {
	label string
	view  ViewKind
	mode  navMode
	value string
}

type copyTitleResultMsg struct {
	err error
}

var clipboardWriteAll = clipboard.WriteAll

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
		if !m.detailPaneVisible() && m.focusedPane == paneDetail {
			m.focusedPane = paneList
		}
		return m, nil
	}
	if msg, ok := msg.(copyTitleResultMsg); ok {
		if msg.err != nil {
			m.err = msg.err.Error()
			m.status = ""
			return m, nil
		}
		m.err = ""
		m.status = "copied title"
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
			m.status = ""
			m.prompt = nil
			m.clampSelection()
		}
		return m, cmd
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.helpOpen {
		switch key.String() {
		case "?", "esc":
			m.helpOpen = false
			return m, nil
		case "ctrl+c", "q":
			return m, tea.Quit
		default:
			return m, nil
		}
	}
	m.status = ""
	switch key.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "0":
		m.focusedPane = paneList
	case "1":
		m.switchFixedView(KindInbox, 0)
	case "2":
		m.switchFixedView(KindToday, 1)
	case "3":
		m.switchFixedView(KindWeekly, 2)
	case "?":
		m.helpOpen = true
	case "tab":
		m.focusNextPane()
	case "shift+tab":
		m.focusPrevPane()
	case "h", "left":
		m.moveLeft()
	case "l", "right":
		m.moveRight()
	case "esc":
		if m.navMode != navRoot {
			mode := m.navMode
			m.navMode = navRoot
			m.navSelected = m.rootIndexForMode(mode)
		}
	case "j", "down":
		m.moveSelection(1)
	case "k", "up":
		m.moveSelection(-1)
	case "enter":
		if m.focusedPane == paneNav {
			m.activateNav()
		}
	case "a":
		if m.listKeyActive() {
			m.prompt = newLinePrompt(promptAdd, "add")
		}
	case "/":
		if m.listKeyActive() {
			m.prompt = newLinePrompt(promptSearch, "search")
		}
	case ":":
		if m.listKeyActive() {
			m.prompt = newLinePrompt(promptCommand, "command")
		}
	case "c":
		if task, ok := m.selectedTask(); ok && m.listKeyActive() {
			return m, copyTitleCmd(task.Title)
		}
	case "e":
		if task, ok := m.selectedTask(); ok && m.listKeyActive() {
			m.prompt = newLinePrompt(promptCommand, "edit")
			m.prompt.input.SetValue("update " + encodeQuickTask(task.Input()))
		}
	case "t":
		if task, ok := m.selectedTask(); ok && m.listKeyActive() {
			input := task.Input()
			input.Start = StartDate
			input.StartDate = FormatLocalDate(m.now())
			_, err := m.store.Update(task.ID, input)
			m.err = errorString(err)
			m.clampSelection()
		}
	case "w":
		if task, ok := m.selectedTask(); ok && m.listKeyActive() {
			if task.WIP {
				m.err = errorString(m.store.ClearWIP(task.ID))
			} else {
				m.err = errorString(m.store.SetWIP(task.ID))
			}
		}
	case " ":
		if task, ok := m.selectedTask(); ok && m.listKeyActive() {
			if task.CompletedAt == "" {
				m.err = errorString(m.store.Complete(task.ID, FormatLocalDate(m.now())))
			} else {
				m.err = errorString(m.store.Uncomplete(task.ID))
			}
		}
	case "d", "delete", "backspace":
		if task, ok := m.selectedTask(); ok && m.listKeyActive() {
			m.err = errorString(m.store.Delete(task.ID))
			m.clampSelection()
		}
	}
	return m, nil
}

func (m Model) View() string {
	view := m.baseView()
	if m.prompt != nil {
		return overlayPromptView(view, m.prompt.popupView(m, m.promptWidth()), m.width, m.height)
	}
	return view
}

func (m Model) baseView() string {
	header := m.headerView()
	if m.err != "" {
		header += " " + errorStyle.Render(m.err)
	} else if m.status != "" {
		header += " " + statusStyle.Render(" "+m.status+" ")
	}
	wip := m.wipView()
	body := m.panesView(m.bodyHeight())
	help := m.statusBarView()
	if m.helpOpen {
		body = overlayHelpView(m.bodyHeight(), m.width)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, wip, "", body, "", help)
}

func (m Model) promptWidth() int {
	width := m.width
	if width <= 0 {
		width = 88
	}
	if width < 48 {
		return max(1, width-2)
	}
	return min(78, width-10)
}

func (m Model) headerView() string {
	title := headerStyle.Render(" LAZYTASK // OPS CONSOLE ")
	view := statusStyle.Render(" VIEW [" + strings.ToUpper(m.title()) + "] ")
	count := subtleStyle.Render(fmt.Sprintf(" TARGETS %02d ", len(m.visibleTasks())))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, " ", view, " ", count)
}

func (m Model) wipView() string {
	width := m.width
	if width <= 0 {
		width = 120
	}
	task, ok := m.wipTask()
	if !ok {
		return subtleStyle.Render(truncateDisplay("WIP none", width))
	}
	line := "WIP " + task.Title
	if meta := compactWIPMeta(task); meta != "" {
		line += "  " + meta
	}
	return metaStyle.Render(truncateDisplay(line, width))
}

func (m Model) wipTask() (Task, bool) {
	for _, task := range m.store.List() {
		if task.WIP {
			return task, true
		}
	}
	return Task{}, false
}

func (m Model) bodyHeight() int {
	if m.height <= 0 {
		return 0
	}
	return max(1, m.height-5)
}

func (m Model) panesView(height int) string {
	width := m.width
	if width <= 0 {
		width = 120
	}
	showDetail := width >= 100
	navWidth := 22
	if width < 72 {
		navWidth = 18
	}
	gap := 2
	detailWidth := 0
	if showDetail {
		detailWidth = max(26, width/3)
	}
	listWidth := width - navWidth - gap
	if showDetail {
		listWidth -= detailWidth + gap
	}
	if listWidth < 28 {
		listWidth = 28
	}
	contentHeight := paneContentHeight(height)
	navTextWidth := paneTextWidth(navWidth)
	listTextWidth := paneTextWidth(listWidth)
	detailTextWidth := paneTextWidth(detailWidth)

	nav := paneStyle(m.focusedPane == paneNav, navWidth, height).Render(m.navView(contentHeight, navTextWidth))
	listModel := m
	listModel.width = listTextWidth
	list := paneStyle(m.focusedPane == paneList, listWidth, height).Render(listModel.listView(contentHeight, listTextWidth))
	if !showDetail {
		return lipgloss.JoinHorizontal(lipgloss.Top, nav, strings.Repeat(" ", gap), list)
	}
	detail := paneStyle(m.focusedPane == paneDetail, detailWidth, height).Render(m.detailView(contentHeight, detailTextWidth))
	return lipgloss.JoinHorizontal(lipgloss.Top, nav, strings.Repeat(" ", gap), list, strings.Repeat(" ", gap), detail)
}

func (m Model) navView(height, width int) string {
	items := m.navItems()
	lines := make([]string, 0, len(items)+2)
	if m.navMode != navRoot {
		lines = append(lines, subtleStyle.Render(truncateDisplay("< "+strings.ToUpper(string(m.navMode)), width)))
	}
	if len(items) == 0 {
		lines = append(lines, subtleStyle.Render(truncateDisplay("No "+string(m.navMode), width)))
		return fillHeight(strings.Join(lines, "\n"), height)
	}
	for i, item := range items {
		prefix := "  "
		if i == m.navSelected {
			prefix = "> "
		}
		label := item.label
		if item.mode != "" && item.mode != navRoot {
			label += " >"
		}
		line := truncateDisplay(prefix+label, width)
		if i == m.navSelected {
			lines = append(lines, selectedStyle.Render(line))
			continue
		}
		if m.navItemActive(item) {
			lines = append(lines, statusStyle.Render(line))
			continue
		}
		lines = append(lines, line)
	}
	return fillHeight(strings.Join(lines, "\n"), height)
}

func (m Model) navItems() []navItem {
	switch m.navMode {
	case navTags:
		return valueNavItems(KnownTags(m.store.List()), navTags)
	case navProjects:
		return valueNavItems(KnownProjects(m.store.List()), navProjects)
	case navAreas:
		return valueNavItems(KnownAreas(m.store.List()), navAreas)
	default:
		return []navItem{
			{label: "[1] Inbox", view: KindInbox},
			{label: "[2] Today", view: KindToday},
			{label: "[3] Weekly", view: KindWeekly},
			{label: "Anytime", view: KindAnytime},
			{label: "Someday", view: KindSomeday},
			{label: "Logbook", view: KindLogbook},
			{label: "Tags", mode: navTags},
			{label: "Projects", mode: navProjects},
			{label: "Areas", mode: navAreas},
		}
	}
}

func valueNavItems(values []string, mode navMode) []navItem {
	items := make([]navItem, 0, len(values))
	for _, value := range values {
		label := value
		switch mode {
		case navTags:
			label = "#" + value
		case navProjects:
			label = ">" + value
		case navAreas:
			label = "/" + value
		}
		items = append(items, navItem{label: label, mode: mode, value: value})
	}
	return items
}

func (m Model) navItemActive(item navItem) bool {
	if m.navMode != navRoot {
		switch item.mode {
		case navTags:
			return m.view == KindFilter && strings.EqualFold(m.filter.Tag, item.value)
		case navProjects:
			return m.view == KindFilter && strings.EqualFold(m.filter.Project, item.value)
		case navAreas:
			return m.view == KindFilter && strings.EqualFold(m.filter.Area, item.value)
		}
		return false
	}
	return item.view != "" && m.view == item.view
}

func (m Model) detailView(height, width int) string {
	task, ok := m.selectedTask()
	if !ok {
		lines := []string{
			"VIEW",
			m.title(),
			"",
			fmt.Sprintf("tasks: %d", len(m.visibleTasks())),
			"",
			"Select a task in the list to inspect its metadata.",
		}
		return fillHeight(truncateLines(lines, width), height)
	}
	lines := []string{
		"DETAIL",
		task.Title,
		"",
		"status: " + taskStatus(task),
		"start: " + taskStart(task),
	}
	if task.Deadline != "" {
		lines = append(lines, "deadline: "+task.Deadline)
	}
	if task.Project != "" {
		lines = append(lines, "project: "+task.Project)
	}
	if task.Area != "" {
		lines = append(lines, "area: "+task.Area)
	}
	if len(task.Tags) > 0 {
		lines = append(lines, "tags: "+JoinTags(task.Tags))
	}
	if task.Notes != "" {
		lines = append(lines, "", "notes:", task.Notes)
	}
	lines = append(lines, "", "created: "+formatTaskTime(task.CreatedAt), "updated: "+formatTaskTime(task.UpdatedAt))
	lines = append(lines, "", helpStyle.Render("w wip  t today  space done  d delete  e edit"))
	return fillHeight(truncateLines(lines, width), height)
}

func (m Model) statusBarView() string {
	width := m.width
	if width <= 0 {
		width = 120
	}
	line := fmt.Sprintf("[0] list  [tab] pane  [j/k] select  [w] wip  [?] help  pane:%s  [a] capture  [/] scan  [:] command  [q] exit", m.focusedPane)
	return helpStyle.Render(truncateDisplay(line, width))
}

func overlayHelpView(height, width int) string {
	lines := []string{
		"HELP",
		"",
		"0                  jump to task list pane",
		"1 / 2 / 3          jump to Inbox, Today, Weekly",
		"tab / shift+tab    cycle panes",
		"h / l              move between panes",
		"j / k              move selection",
		"enter              select nav view or filter",
		"a                  quick add",
		"/                  search",
		":                  command palette",
		"w                  toggle selected task as WIP",
		"t                  schedule selected task today",
		"c                  copy selected task title",
		"space              complete or reopen selected task",
		"d                  delete selected task",
		"esc                close overlay or return nav to root",
		"q                  quit",
	}
	contentHeight := paneContentHeight(height)
	if width > 0 {
		return paneStyle(true, max(40, min(width, 72)), height).Render(fillHeight(strings.Join(lines, "\n"), contentHeight))
	}
	return fillHeight(strings.Join(lines, "\n"), contentHeight)
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

func (m Model) listView(height, width int) string {
	if m.view == KindWeekly {
		return m.weeklyView(height)
	}
	tasks := m.visibleTasks()
	if len(tasks) == 0 {
		return fillHeight(subtleStyle.Render(truncateDisplay("No tasks. Press a to capture one.", width)), height)
	}
	lines := make([]string, 0, len(tasks)+1)
	for _, i := range visibleIndexes(len(tasks), m.selected, height) {
		lines = append(lines, m.taskLine(tasks[i], i == m.selected, width))
	}
	if height > 0 && len(tasks) > len(lines) {
		lines = append(lines, subtleStyle.Render(truncateDisplay(fmt.Sprintf("+%d more", len(tasks)-len(lines)), width)))
	}
	return fillHeight(strings.Join(lines, "\n"), height)
}

func (m Model) weeklyView(height int) string {
	week := WorkWeek(m.store.List(), m.now())
	for i := range week {
		week[i].Tasks = activeTasksFirst(week[i].Tasks)
	}
	flat := activeTasksFirst(FlattenWeek(week))
	selectedID := ""
	if m.selected >= 0 && m.selected < len(flat) {
		selectedID = flat[m.selected].ID
	}
	width := 24
	if m.width > 20 {
		width = max(1, m.width/5-1)
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
		headerCells = append(headerCells, weeklyColumnStyle.Width(width).Render(dayStyle.Render(truncateDisplay("["+day.Label+" "+day.Date[5:]+"]", width))))
		separatorCells = append(separatorCells, weeklyColumnStyle.Width(width).Render(gridStyle.Render(strings.Repeat("═", max(1, width)))))

		lines := make([]string, 0, len(visibleTasks)+1)
		if len(day.Tasks) == 0 {
			lines = append(lines, subtleStyle.Render(truncateDisplay("standby", width)))
		}
		for _, task := range visibleTasks {
			lines = append(lines, m.weeklyTaskLine(task, task.ID == selectedID, width))
		}
		if len(visibleTasks) < len(day.Tasks) {
			lines = append(lines, subtleStyle.Render(truncateDisplay(fmt.Sprintf("+%d more", len(day.Tasks)-len(visibleTasks)), width)))
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

func (m Model) taskLine(task Task, selected bool, width int) string {
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
	line = truncateDisplay(line, width)
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
	line = truncateDisplay(line, width)
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

func compactWIPMeta(task Task) string {
	parts := make([]string, 0, 3)
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
	if len(task.Tags) > 0 {
		parts = append(parts, "#"+task.Tags[0])
	}
	return strings.Join(parts, " ")
}

func taskStatus(task Task) string {
	switch {
	case task.Deleted:
		return "deleted"
	case task.CompletedAt != "":
		return "completed " + task.CompletedAt
	case task.CanceledAt != "":
		return "canceled " + task.CanceledAt
	default:
		return "active"
	}
}

func taskStart(task Task) string {
	if task.Start == StartDate && task.StartDate != "" {
		return task.StartDate
	}
	if task.Start == "" {
		return string(StartInbox)
	}
	return string(task.Start)
}

func formatTaskTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format("2006-01-02 15:04")
}

func (m Model) visibleTasks() []Task {
	tasks := m.store.List()
	switch m.view {
	case KindInbox:
		return activeTasksFirst(InboxTasks(tasks))
	case KindToday:
		return activeTasksFirst(TodayTasks(tasks, m.now()))
	case KindWeekly:
		return activeTasksFirst(FlattenWeek(WorkWeek(tasks, m.now())))
	case KindAnytime:
		return activeTasksFirst(AnytimeTasks(tasks))
	case KindSomeday:
		return activeTasksFirst(SomedayTasks(tasks))
	case KindLogbook:
		return activeTasksFirst(LogbookTasks(tasks))
	case KindFilter:
		return activeTasksFirst(FilteredTasks(tasks, m.filter))
	default:
		return activeTasksFirst(InboxTasks(tasks))
	}
}

func activeTasksFirst(tasks []Task) []Task {
	if len(tasks) == 0 {
		return tasks
	}
	out := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		if task.Active() {
			out = append(out, task)
		}
	}
	for _, task := range tasks {
		if !task.Active() {
			out = append(out, task)
		}
	}
	return out
}

func copyTitleCmd(title string) tea.Cmd {
	return func() tea.Msg {
		return copyTitleResultMsg{err: clipboardWriteAll(title)}
	}
}

func (m Model) selectedTask() (Task, bool) {
	tasks := m.visibleTasks()
	if m.selected < 0 || m.selected >= len(tasks) {
		return Task{}, false
	}
	return tasks[m.selected], true
}

func (m Model) detailPaneVisible() bool {
	if m.width <= 0 {
		return true
	}
	return m.width >= 100
}

func (m *Model) focusNextPane() {
	switch m.focusedPane {
	case paneNav:
		m.focusedPane = paneList
	case paneList:
		if m.detailPaneVisible() {
			m.focusedPane = paneDetail
		} else {
			m.focusedPane = paneNav
		}
	default:
		m.focusedPane = paneNav
	}
}

func (m *Model) focusPrevPane() {
	switch m.focusedPane {
	case paneNav:
		if m.detailPaneVisible() {
			m.focusedPane = paneDetail
		} else {
			m.focusedPane = paneList
		}
	case paneDetail:
		m.focusedPane = paneList
	default:
		m.focusedPane = paneNav
	}
}

func (m *Model) moveLeft() {
	switch m.focusedPane {
	case paneNav:
		if m.navMode != navRoot {
			mode := m.navMode
			m.navMode = navRoot
			m.navSelected = m.rootIndexForMode(mode)
		}
	case paneList:
		m.focusedPane = paneNav
	case paneDetail:
		m.focusedPane = paneList
	}
}

func (m *Model) moveRight() {
	switch m.focusedPane {
	case paneNav:
		if m.navSelectedItemOpens() {
			m.openNavMode()
			return
		}
		m.focusedPane = paneList
	case paneList:
		if m.detailPaneVisible() {
			m.focusedPane = paneDetail
		}
	}
}

func (m *Model) moveSelection(delta int) {
	if m.focusedPane == paneNav {
		m.navSelected += delta
		m.clampNavSelection()
		m.applySelectedNav(false)
		return
	}
	if m.focusedPane == paneList {
		m.selected += delta
		m.clampSelection()
	}
}

func (m Model) listKeyActive() bool {
	return m.focusedPane == paneList
}

func (m *Model) activateNav() {
	m.applySelectedNav(true)
}

func (m *Model) applySelectedNav(focusList bool) {
	items := m.navItems()
	if len(items) == 0 || m.navSelected < 0 || m.navSelected >= len(items) {
		return
	}
	item := items[m.navSelected]
	if item.mode != "" && item.value == "" {
		if focusList {
			m.openNavMode()
		}
		return
	}
	if item.value != "" {
		m.view = KindFilter
		m.filter = Filter{}
		switch item.mode {
		case navTags:
			m.filter.Tag = item.value
		case navProjects:
			m.filter.Project = item.value
		case navAreas:
			m.filter.Area = item.value
		}
		m.selected = 0
		if focusList {
			m.focusedPane = paneList
		}
		m.err = ""
		m.status = ""
		return
	}
	if item.view != "" {
		m.applyFixedView(item.view, m.navSelected, focusList)
	}
}

func (m *Model) switchFixedView(view ViewKind, navSelected int) {
	m.applyFixedView(view, navSelected, true)
}

func (m *Model) applyFixedView(view ViewKind, navSelected int, focusList bool) {
	m.view = view
	m.filter = Filter{}
	m.selected = 0
	m.navMode = navRoot
	m.navSelected = navSelected
	if focusList {
		m.focusedPane = paneList
	}
	m.err = ""
	m.status = ""
}

func (m Model) navSelectedItemOpens() bool {
	items := m.navItems()
	return m.navSelected >= 0 && m.navSelected < len(items) && items[m.navSelected].mode != "" && items[m.navSelected].value == ""
}

func (m *Model) openNavMode() {
	items := m.navItems()
	if m.navSelected < 0 || m.navSelected >= len(items) || items[m.navSelected].mode == "" {
		return
	}
	m.navMode = items[m.navSelected].mode
	m.navSelected = 0
	m.clampNavSelection()
}

func (m *Model) clampNavSelection() {
	items := m.navItems()
	if len(items) == 0 {
		m.navSelected = 0
		return
	}
	if m.navSelected < 0 {
		m.navSelected = 0
	}
	if m.navSelected >= len(items) {
		m.navSelected = len(items) - 1
	}
}

func (m Model) rootIndexForMode(mode navMode) int {
	switch mode {
	case navTags:
		return 6
	case navProjects:
		return 7
	case navAreas:
		return 8
	default:
		return 0
	}
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

func (p *linePrompt) popupView(m Model, width int) string {
	contentWidth := paneTextWidth(width)
	input := p.input
	input.Width = max(8, contentWidth)
	lines := []string{headerStyle.Render(truncateDisplay(" "+strings.ToUpper(p.label)+" ", contentWidth)), input.View()}
	if p.kind == promptSearch {
		lines = append(lines, "", subtleStyle.Render(truncateDisplay("try: #urgent  >Work  /Home  today  weekly", contentWidth)))
		lines = append(lines, truncatePromptLines(m.searchHints(p.input.Value()), contentWidth)...)
	}
	if p.kind == promptCommand {
		lines = append(lines, "", subtleStyle.Render(truncateDisplay("add/tag/untag/move/area/when/deadline/done/undone/cancel/delete", contentWidth)))
	}
	lines = append(lines, "", subtleStyle.Render("enter apply  esc cancel"))
	style := focusBorderStyle
	if width > 2 {
		style = style.Width(width - 2)
	}
	return style.Render(fillWidthLines(strings.Join(lines, "\n"), contentWidth))
}

func truncatePromptLines(lines []string, width int) []string {
	for i, line := range lines {
		lines[i] = truncateDisplay(line, width)
	}
	return lines
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

func truncateDisplay(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width <= 3 {
		return ansi.Truncate(value, width, "")
	}
	return ansi.Truncate(value, width, "...")
}

func truncateLines(lines []string, width int) string {
	for i, line := range lines {
		lines[i] = truncateDisplay(line, width)
	}
	return strings.Join(lines, "\n")
}

func fillWidthLines(value string, width int) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = lipgloss.PlaceHorizontal(width, lipgloss.Left, truncateDisplay(line, width))
	}
	return strings.Join(lines, "\n")
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

func paneContentHeight(height int) int {
	if height <= 2 {
		return 0
	}
	return height - 2
}

func paneTextWidth(width int) int {
	if width <= 4 {
		return 0
	}
	return width - 4
}

func overlayPromptView(base, popup string, width, height int) string {
	baseLines := strings.Split(base, "\n")
	popupLines := strings.Split(popup, "\n")
	if width <= 0 {
		width = maxLineWidth(baseLines)
	}
	if height <= 0 {
		height = len(baseLines)
	}
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}
	if len(baseLines) > height {
		baseLines = baseLines[:height]
	}
	for i, line := range baseLines {
		baseLines[i] = lipgloss.PlaceHorizontal(width, lipgloss.Left, truncateDisplay(line, width))
	}
	top := max(0, (height-len(popupLines))/2)
	for i, line := range popupLines {
		row := top + i
		if row >= height {
			break
		}
		baseLines[row] = overlayLine(baseLines[row], truncateDisplay(line, width), width)
	}
	return strings.Join(baseLines, "\n")
}

func overlayLine(base, popup string, width int) string {
	popupWidth := lipgloss.Width(popup)
	if popupWidth >= width {
		return truncateDisplay(popup, width)
	}
	left := max(0, (width-popupWidth)/2)
	right := left + popupWidth
	return ansi.Cut(base, 0, left) + popup + ansi.Cut(base, right, width)
}

func maxLineWidth(lines []string) int {
	width := 0
	for _, line := range lines {
		width = max(width, lipgloss.Width(line))
	}
	return width
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
	paneBorderStyle   = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1)
	focusBorderStyle  = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("51")).Padding(0, 1)
)

func paneStyle(focused bool, width, height int) lipgloss.Style {
	style := paneBorderStyle
	if focused {
		style = focusBorderStyle
	}
	if width > 4 {
		style = style.Width(width - 2)
	}
	if height > 2 {
		style = style.Height(height - 2)
	}
	return style
}
