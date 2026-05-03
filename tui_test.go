package lazytask

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelCompletesSelectedTaskAndNavigatesPanes(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	store.SetClock(fixedClock("2026-05-04"))
	task, err := store.Create(TaskInput{Title: "Ship MVP", Start: StartInbox})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	model := NewModel(store)
	model.now = fixedClock("2026-05-04")
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeySpace})
	model = updated.(Model)

	got, ok := store.Get(task.ID)
	if !ok {
		t.Fatal("expected task")
	}
	if got.CompletedAt != "2026-05-04" {
		t.Fatalf("expected completed today, got %q", got.CompletedAt)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	if model.focusedPane != paneDetail {
		t.Fatalf("expected detail pane, got %s", model.focusedPane)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	if model.focusedPane != paneNav {
		t.Fatalf("expected nav pane, got %s", model.focusedPane)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model = updated.(Model)
	if model.focusedPane != paneDetail {
		t.Fatalf("expected detail pane after shift+tab, got %s", model.focusedPane)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	model = updated.(Model)
	if model.focusedPane != paneList {
		t.Fatalf("expected list pane after h, got %s", model.focusedPane)
	}
}

func TestRootNavSelectsFixedView(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	model := NewModel(store)
	model.focusedPane = paneNav
	model.navSelected = 2

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	if model.view != KindWeekly {
		t.Fatalf("expected weekly view, got %s", model.view)
	}
	if model.focusedPane != paneList {
		t.Fatalf("expected focus to move to list, got %s", model.focusedPane)
	}
}

func TestNavCategoryAppliesFilter(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	if _, err := store.Create(TaskInput{Title: "Tagged", Start: StartAnytime, Tags: []string{"urgent"}}); err != nil {
		t.Fatalf("create tagged task: %v", err)
	}
	if _, err := store.Create(TaskInput{Title: "Other", Start: StartAnytime, Project: "Work", Area: "Home"}); err != nil {
		t.Fatalf("create other task: %v", err)
	}
	model := NewModel(store)
	model.focusedPane = paneNav
	model.navSelected = 6

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	model = updated.(Model)
	if model.navMode != navTags {
		t.Fatalf("expected tags nav mode, got %s", model.navMode)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.view != KindFilter || model.filter.Tag != "urgent" {
		t.Fatalf("expected urgent tag filter, got view=%s filter=%#v", model.view, model.filter)
	}
	if got := model.visibleTasks(); len(got) != 1 || got[0].Title != "Tagged" {
		t.Fatalf("expected tagged task, got %#v", got)
	}
}

func TestHelpOverlayOpensAndCloses(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	model := NewModel(store)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	model = updated.(Model)
	if !model.helpOpen {
		t.Fatal("expected help overlay open")
	}
	if view := model.View(); !strings.Contains(view, "HELP") || !strings.Contains(view, "cycle panes") {
		t.Fatalf("expected help overlay content:\n%s", view)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.helpOpen {
		t.Fatal("expected help overlay closed")
	}
}

func TestRunCommandAddsAndOrganizesTask(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	model := NewModel(store)
	model.now = fixedClock("2026-05-04")

	if err := model.runCommand("add 買い物 #errand @today >Home /生活 !2026-05-05"); err != nil {
		t.Fatalf("add command: %v", err)
	}
	tasks := store.List()
	if len(tasks) != 1 {
		t.Fatalf("expected one task, got %#v", tasks)
	}
	task := tasks[0]
	if task.Title != "買い物" || task.Start != StartDate || task.StartDate != "2026-05-04" || task.Project != "Home" || task.Area != "生活" {
		t.Fatalf("unexpected task: %#v", task)
	}

	model.view = KindToday
	if err := model.runCommand("tag #urgent"); err != nil {
		t.Fatalf("tag command: %v", err)
	}
	got, _ := store.Get(task.ID)
	if len(got.Tags) != 2 || got.Tags[1] != "urgent" {
		t.Fatalf("expected added tag, got %#v", got.Tags)
	}
}

func TestSearchAppliesTagFilter(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	if _, err := store.Create(TaskInput{Title: "Tagged", Start: StartAnytime, Tags: []string{"urgent"}}); err != nil {
		t.Fatalf("create task: %v", err)
	}
	model := NewModel(store)

	model.applySearch("#urgent")
	if model.view != KindFilter || model.filter.Tag != "urgent" {
		t.Fatalf("expected tag filter, got view=%s filter=%#v", model.view, model.filter)
	}
	if len(model.visibleTasks()) != 1 {
		t.Fatalf("expected one filtered task, got %#v", model.visibleTasks())
	}
}

func TestInboxTKeySchedulesSelectedTaskToday(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	store.SetClock(fixedClock("2026-05-04"))
	task, err := store.Create(TaskInput{Title: "Plan me", Start: StartInbox})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	model := NewModel(store)
	model.now = fixedClock("2026-05-04")
	model.view = KindInbox

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	model = updated.(Model)

	got, ok := store.Get(task.ID)
	if !ok {
		t.Fatal("expected task")
	}
	if got.Start != StartDate || got.StartDate != "2026-05-04" {
		t.Fatalf("expected task scheduled today, got %#v", got)
	}
	if len(model.visibleTasks()) != 0 {
		t.Fatalf("expected task to leave inbox, got %#v", model.visibleTasks())
	}
	model.view = KindToday
	if len(model.visibleTasks()) != 1 {
		t.Fatalf("expected task in today, got %#v", model.visibleTasks())
	}
}

func TestTodayAddDefaultsTaskToToday(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	model := NewModel(store)
	model.now = fixedClock("2026-05-04")
	model.view = KindToday

	if err := model.runCommand("add Review plan #work"); err != nil {
		t.Fatalf("add command: %v", err)
	}
	tasks := store.List()
	if len(tasks) != 1 {
		t.Fatalf("expected one task, got %#v", tasks)
	}
	if tasks[0].Start != StartDate || tasks[0].StartDate != "2026-05-04" {
		t.Fatalf("expected today default, got %#v", tasks[0])
	}
	if len(TodayTasks(tasks, model.now())) != 1 {
		t.Fatalf("expected task visible today, got %#v", TodayTasks(tasks, model.now()))
	}
}

func TestTodayAddRespectsExplicitSomeday(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	model := NewModel(store)
	model.now = fixedClock("2026-05-04")
	model.view = KindToday

	if err := model.runCommand("add Maybe later @someday"); err != nil {
		t.Fatalf("add command: %v", err)
	}
	tasks := store.List()
	if tasks[0].Start != StartSomeday {
		t.Fatalf("expected explicit someday to be preserved, got %#v", tasks[0])
	}
}

func TestWeeklyViewDoesNotCollapseJapaneseTaskToEllipsis(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	if _, err := store.Create(TaskInput{
		Title:     "買い物をする",
		Start:     StartDate,
		StartDate: "2026-05-04",
	}); err != nil {
		t.Fatalf("create task: %v", err)
	}
	model := NewModel(store)
	model.now = fixedClock("2026-05-04")
	model.view = KindWeekly
	model.width = 80

	view := model.weeklyView(0)
	if !strings.Contains(view, "買い物") {
		t.Fatalf("expected weekly view to include readable title, got:\n%s", view)
	}
	if strings.Contains(view, "> [ ] ...") || strings.Contains(view, "  [ ] ...") {
		t.Fatalf("weekly view collapsed task to ellipsis:\n%s", view)
	}
}

func TestViewUsesAvailableHeight(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	model := NewModel(store)
	model.height = 18

	view := model.View()
	if got := len(strings.Split(view, "\n")); got < 18 {
		t.Fatalf("expected view to use available height, got %d lines:\n%s", got, view)
	}
}

func TestThreePaneViewRendersNavListAndDetail(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	if _, err := store.Create(TaskInput{
		Title:    "Inspect layout",
		Start:    StartInbox,
		Project:  "Ops",
		Area:     "Work",
		Tags:     []string{"urgent"},
		Deadline: "2026-05-05",
	}); err != nil {
		t.Fatalf("create task: %v", err)
	}
	model := NewModel(store)
	model.width = 130
	model.height = 24

	view := model.View()
	for _, want := range []string{"Inbox", "Weekly", "Inspect layout", "DETAIL", "project: Ops", "tags: urgent"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in three-pane view:\n%s", want, view)
		}
	}
}

func TestNarrowViewOmitsDetailPane(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	if _, err := store.Create(TaskInput{Title: "Narrow task", Start: StartInbox, Project: "Hidden"}); err != nil {
		t.Fatalf("create task: %v", err)
	}
	model := NewModel(store)
	model.width = 72
	model.height = 18

	view := model.View()
	if !strings.Contains(view, "Narrow task") {
		t.Fatalf("expected list task in narrow view:\n%s", view)
	}
	if strings.Contains(view, "DETAIL") || strings.Contains(view, "project: Hidden") {
		t.Fatalf("expected detail pane to be omitted in narrow view:\n%s", view)
	}
}

func TestWeeklyViewUsesAvailableHeight(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	if _, err := store.Create(TaskInput{Title: "Weekly task", Start: StartDate, StartDate: "2026-05-04"}); err != nil {
		t.Fatalf("create task: %v", err)
	}
	model := NewModel(store)
	model.now = fixedClock("2026-05-04")
	model.view = KindWeekly
	model.width = 100
	model.height = 18

	view := model.View()
	if got := len(strings.Split(view, "\n")); got < 18 {
		t.Fatalf("expected weekly view to use available height, got %d lines:\n%s", got, view)
	}
	if !strings.Contains(view, "Weekly task") {
		t.Fatalf("expected weekly task in view:\n%s", view)
	}
}

func TestWeeklyHeadersRemainVisibleWhenSelectedTaskScrolls(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	for i := 0; i < 6; i++ {
		if _, err := store.Create(TaskInput{
			Title:     "Monday task",
			Start:     StartDate,
			StartDate: "2026-05-04",
		}); err != nil {
			t.Fatalf("create task: %v", err)
		}
	}
	model := NewModel(store)
	model.now = fixedClock("2026-05-04")
	model.view = KindWeekly
	model.width = 100
	model.selected = 4

	view := model.weeklyView(5)
	if !strings.Contains(view, "Mon 05-04") {
		t.Fatalf("expected monday header to remain visible:\n%s", view)
	}
	if !strings.Contains(view, "════════") {
		t.Fatalf("expected header separator to remain visible:\n%s", view)
	}
}

func TestWeeklyViewRendersCompletedTask(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	task, err := store.Create(TaskInput{Title: "Done this week", Start: StartAnytime})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := store.Complete(task.ID, "2026-05-06"); err != nil {
		t.Fatalf("complete task: %v", err)
	}
	model := NewModel(store)
	model.now = fixedClock("2026-05-04")
	model.view = KindWeekly
	model.width = 100

	view := model.weeklyView(8)
	if !strings.Contains(view, "Done this") || !strings.Contains(view, "[x]") {
		t.Fatalf("expected completed task in weekly view:\n%s", view)
	}
}

func TestCompletedTaskLineDoesNotLeakANSISequences(t *testing.T) {
	model := NewModel(nil)
	line := model.taskLine(Task{
		Title:       "done task",
		Start:       StartDate,
		StartDate:   "2026-04-28",
		Deadline:    "2026-04-29",
		CompletedAt: "2026-05-03",
	}, false)

	if strings.Contains(line, "[38;5;") || strings.Contains(line, "[0m") {
		t.Fatalf("completed line leaked ansi sequence: %q", line)
	}
}
