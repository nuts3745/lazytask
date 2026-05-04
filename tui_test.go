package lazytask

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

func TestRootNavDisplaysShortcutLabels(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	model := NewModel(store)

	view := model.navView(12, 40)
	for _, want := range []string{"[1] Inbox", "[2] Today", "[3] Weekly"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected shortcut label %q in nav:\n%s", want, view)
		}
	}
}

func TestRootNavMoveAppliesFixedViewWithoutMovingFocus(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	model := NewModel(store)
	model.focusedPane = paneNav
	model.navSelected = 0
	model.selected = 3
	model.err = "old error"

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(Model)

	if model.view != KindToday {
		t.Fatalf("expected today view, got %s", model.view)
	}
	if model.focusedPane != paneNav {
		t.Fatalf("expected focus to stay on nav, got %s", model.focusedPane)
	}
	if model.navMode != navRoot || model.navSelected != 1 {
		t.Fatalf("expected root nav item 1, got mode=%s selected=%d", model.navMode, model.navSelected)
	}
	if model.selected != 0 || model.filter != (Filter{}) || model.err != "" {
		t.Fatalf("expected reset selection/filter/error, got selected=%d filter=%#v err=%q", model.selected, model.filter, model.err)
	}
}

func TestVisibleTasksKeepsInactiveTasksBelowActiveTasks(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	store.SetClock(fixedClock("2026-05-04"))
	done, err := store.Create(TaskInput{Title: "Done first", Start: StartDate, StartDate: "2026-05-04"})
	if err != nil {
		t.Fatalf("create done task: %v", err)
	}
	if err := store.Complete(done.ID, "2026-05-04"); err != nil {
		t.Fatalf("complete task: %v", err)
	}
	active, err := store.Create(TaskInput{Title: "Active second", Start: StartDate, StartDate: "2026-05-04"})
	if err != nil {
		t.Fatalf("create active task: %v", err)
	}
	canceled, err := store.Create(TaskInput{Title: "Canceled third", Start: StartDate, StartDate: "2026-05-04"})
	if err != nil {
		t.Fatalf("create canceled task: %v", err)
	}
	if err := store.Cancel(canceled.ID, "2026-05-04"); err != nil {
		t.Fatalf("cancel task: %v", err)
	}
	model := NewModel(store)
	model.now = fixedClock("2026-05-04")

	for _, view := range []ViewKind{KindToday, KindWeekly, KindFilter} {
		model.view = view
		model.filter = Filter{}
		got := model.visibleTasks()
		if len(got) != 3 {
			t.Fatalf("%s expected 3 tasks, got %#v", view, got)
		}
		if got[0].ID != active.ID || got[1].ID != done.ID || got[2].ID != canceled.ID {
			t.Fatalf("%s expected active then inactive in stable order, got %#v", view, got)
		}
	}
}

func TestWeeklyViewKeepsInactiveTasksBelowActiveTasksInColumns(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	store.SetClock(fixedClock("2026-05-04"))
	done, err := store.Create(TaskInput{Title: "Done first", Start: StartDate, StartDate: "2026-05-04"})
	if err != nil {
		t.Fatalf("create done task: %v", err)
	}
	if err := store.Complete(done.ID, "2026-05-04"); err != nil {
		t.Fatalf("complete task: %v", err)
	}
	if _, err := store.Create(TaskInput{Title: "Active second", Start: StartDate, StartDate: "2026-05-04"}); err != nil {
		t.Fatalf("create active task: %v", err)
	}
	model := NewModel(store)
	model.now = fixedClock("2026-05-04")
	model.view = KindWeekly
	model.width = 100

	view := model.weeklyView(8)
	activeIndex := strings.Index(view, "Active")
	doneIndex := strings.Index(view, "Done first")
	if activeIndex < 0 || doneIndex < 0 {
		t.Fatalf("expected both weekly tasks:\n%s", view)
	}
	if activeIndex > doneIndex {
		t.Fatalf("expected active task before completed task in weekly column:\n%s", view)
	}
}

func TestCKeyCopiesSelectedTaskTitle(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	if _, err := store.Create(TaskInput{Title: "Copy only this title", Start: StartInbox, Notes: "not copied"}); err != nil {
		t.Fatalf("create task: %v", err)
	}
	oldWrite := clipboardWriteAll
	defer func() { clipboardWriteAll = oldWrite }()
	var copied []string
	clipboardWriteAll = func(value string) error {
		copied = append(copied, value)
		return nil
	}
	model := NewModel(store)
	model.err = "old error"

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected copy command")
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)

	if len(copied) != 1 || copied[0] != "Copy only this title" {
		t.Fatalf("expected selected title copied, got %#v", copied)
	}
	if model.err != "" {
		t.Fatalf("expected copy success to clear error, got %q", model.err)
	}
	if model.status != "copied title" {
		t.Fatalf("expected copy status, got %q", model.status)
	}
}

func TestCKeyShowsClipboardWriteError(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	if _, err := store.Create(TaskInput{Title: "Copy fails", Start: StartInbox}); err != nil {
		t.Fatalf("create task: %v", err)
	}
	oldWrite := clipboardWriteAll
	defer func() { clipboardWriteAll = oldWrite }()
	clipboardWriteAll = func(string) error {
		return errors.New("clipboard unavailable")
	}
	model := NewModel(store)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected copy command")
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)

	if model.err != "clipboard unavailable" {
		t.Fatalf("expected clipboard error, got %q", model.err)
	}
	if model.status != "" {
		t.Fatalf("expected failed copy to clear status, got %q", model.status)
	}
}

func TestCKeyDoesNotCopyWithoutListSelection(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	if _, err := store.Create(TaskInput{Title: "Do not copy from nav", Start: StartInbox}); err != nil {
		t.Fatalf("create task: %v", err)
	}
	oldWrite := clipboardWriteAll
	defer func() { clipboardWriteAll = oldWrite }()
	clipboardWriteAll = func(string) error {
		t.Fatal("clipboard write should not be called")
		return nil
	}

	model := NewModel(store)
	model.focusedPane = paneNav
	if _, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}); cmd != nil {
		t.Fatal("expected no copy command while nav is focused")
	}
	model.focusedPane = paneList
	model.selected = 10
	if _, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}); cmd != nil {
		t.Fatal("expected no copy command without selected task")
	}
}

func TestNumberKeysJumpToCommonViews(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	model := NewModel(store)
	model.view = KindSomeday
	model.focusedPane = paneNav
	model.navMode = navTags
	model.navSelected = 3
	model.selected = 4
	model.filter = Filter{Tag: "urgent"}
	model.err = "old error"

	for _, tc := range []struct {
		key         string
		wantView    ViewKind
		wantNavItem int
	}{
		{key: "1", wantView: KindInbox, wantNavItem: 0},
		{key: "2", wantView: KindToday, wantNavItem: 1},
		{key: "3", wantView: KindWeekly, wantNavItem: 2},
	} {
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)})
		model = updated.(Model)

		if model.view != tc.wantView {
			t.Fatalf("key %s expected view %s, got %s", tc.key, tc.wantView, model.view)
		}
		if model.focusedPane != paneList || model.navMode != navRoot || model.navSelected != tc.wantNavItem {
			t.Fatalf("key %s expected list/root nav item %d, got pane=%s mode=%s nav=%d", tc.key, tc.wantNavItem, model.focusedPane, model.navMode, model.navSelected)
		}
		if model.selected != 0 || model.filter != (Filter{}) || model.err != "" {
			t.Fatalf("key %s expected reset selection/filter/error, got selected=%d filter=%#v err=%q", tc.key, model.selected, model.filter, model.err)
		}
		model.view = KindSomeday
		model.focusedPane = paneNav
		model.navMode = navTags
		model.navSelected = 3
		model.selected = 4
		model.filter = Filter{Tag: "urgent"}
		model.err = "old error"
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
	if model.focusedPane != paneList {
		t.Fatalf("expected focus to move to list, got %s", model.focusedPane)
	}
}

func TestNavMoveAppliesSubmenuFilterWithoutMovingFocus(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	if _, err := store.Create(TaskInput{Title: "Alpha", Start: StartAnytime, Tags: []string{"alpha"}}); err != nil {
		t.Fatalf("create alpha task: %v", err)
	}
	if _, err := store.Create(TaskInput{Title: "Urgent", Start: StartAnytime, Tags: []string{"urgent"}}); err != nil {
		t.Fatalf("create urgent task: %v", err)
	}
	model := NewModel(store)
	model.focusedPane = paneNav
	model.navMode = navTags
	model.navSelected = 0
	model.selected = 5
	model.err = "old error"

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(Model)

	if model.view != KindFilter || model.filter.Tag != "urgent" {
		t.Fatalf("expected urgent tag filter, got view=%s filter=%#v", model.view, model.filter)
	}
	if got := model.visibleTasks(); len(got) != 1 || got[0].Title != "Urgent" {
		t.Fatalf("expected urgent task, got %#v", got)
	}
	if model.focusedPane != paneNav || model.navMode != navTags || model.navSelected != 1 {
		t.Fatalf("expected focus to stay on tag nav item 1, got pane=%s mode=%s selected=%d", model.focusedPane, model.navMode, model.navSelected)
	}
	if model.selected != 0 || model.err != "" {
		t.Fatalf("expected reset selection/error, got selected=%d err=%q", model.selected, model.err)
	}
}

func TestNavMoveToCategoryHeaderDoesNotOpenSubmenu(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	model := NewModel(store)
	model.focusedPane = paneNav
	model.view = KindLogbook
	model.navSelected = 5
	model.selected = 2
	model.err = "old error"

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(Model)

	if model.navMode != navRoot || model.navSelected != 6 {
		t.Fatalf("expected root nav category selected, got mode=%s selected=%d", model.navMode, model.navSelected)
	}
	if model.view != KindLogbook || model.focusedPane != paneNav {
		t.Fatalf("expected category move to leave view/focus unchanged, got view=%s pane=%s", model.view, model.focusedPane)
	}
	if model.selected != 2 || model.err != "old error" {
		t.Fatalf("expected category move to leave selection/error unchanged, got selected=%d err=%q", model.selected, model.err)
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

func TestPromptRendersAsPopupOverMainView(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	model := NewModel(store)
	model.width = 100
	model.height = 20

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model = updated.(Model)

	view := model.View()
	if !strings.Contains(view, "LAZYTASK") || !strings.Contains(view, "ADD") || !strings.Contains(view, "enter apply") {
		t.Fatalf("expected add prompt popup over main view:\n%s", view)
	}
	if !strings.Contains(view, "pane:list") {
		t.Fatalf("expected main status bar to remain visible behind popup:\n%s", view)
	}
	if got := len(strings.Split(view, "\n")); got != model.height {
		t.Fatalf("expected prompt popup to preserve height %d, got %d:\n%s", model.height, got, view)
	}
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, " ADD ") && strings.TrimSpace(ansi.Cut(line, 0, 1)) == "" {
			t.Fatalf("expected popup row to preserve background beside popup:\n%s", view)
		}
	}
	assertViewWidth(t, view, model.width)
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

func TestWKeyTogglesSelectedActiveTaskAsWIP(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	task, err := store.Create(TaskInput{Title: "Focus task", Start: StartInbox, Project: "Work"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	model := NewModel(store)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	model = updated.(Model)
	got, _ := store.Get(task.ID)
	if !got.WIP {
		t.Fatalf("expected selected task to be WIP: %#v", got)
	}
	if view := model.View(); !strings.Contains(view, "WIP Focus task") || !strings.Contains(view, ">Work") {
		t.Fatalf("expected WIP row with compact metadata:\n%s", view)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	model = updated.(Model)
	got, _ = store.Get(task.ID)
	if got.WIP {
		t.Fatalf("expected second w press to clear WIP: %#v", got)
	}
	if view := model.View(); !strings.Contains(view, "WIP none") {
		t.Fatalf("expected empty WIP state:\n%s", view)
	}
}

func TestWKeyRejectsInactiveTask(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*Store, string) error
	}{
		{name: "completed", run: func(store *Store, id string) error { return store.Complete(id, "2026-05-04") }},
		{name: "canceled", run: func(store *Store, id string) error { return store.Cancel(id, "2026-05-04") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, err := NewMemoryStore()
			if err != nil {
				t.Fatalf("new memory store: %v", err)
			}
			task, err := store.Create(TaskInput{Title: "Inactive task", Start: StartAnytime})
			if err != nil {
				t.Fatalf("create task: %v", err)
			}
			if err := tc.run(store, task.ID); err != nil {
				t.Fatalf("%s task: %v", tc.name, err)
			}
			model := NewModel(store)
			model.now = fixedClock("2026-05-04")
			model.view = KindLogbook

			updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
			model = updated.(Model)

			got, _ := store.Get(task.ID)
			if got.WIP {
				t.Fatalf("%s task should not become WIP: %#v", tc.name, got)
			}
			if !strings.Contains(model.err, "task is not active") {
				t.Fatalf("expected clear inactive-task error, got %q", model.err)
			}
		})
	}
}

func TestWIPRowAppearsInFixedViewsAndPreservesHeight(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	model := NewModel(store)
	model.now = fixedClock("2026-05-04")
	model.width = 100
	model.height = 18

	for _, viewKind := range []ViewKind{KindInbox, KindToday, KindWeekly, KindAnytime, KindSomeday, KindLogbook} {
		model.view = viewKind
		view := model.View()
		if !strings.Contains(view, "WIP none") {
			t.Fatalf("expected WIP row in %s view:\n%s", viewKind, view)
		}
		if got := len(strings.Split(view, "\n")); got != model.height {
			t.Fatalf("expected %s view height %d, got %d:\n%s", viewKind, model.height, got, view)
		}
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
	if got := len(strings.Split(view, "\n")); got != 18 {
		t.Fatalf("expected view to use available height, got %d lines:\n%s", got, view)
	}
}

func TestTodayViewDoesNotExceedAvailableHeight(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	if _, err := store.Create(TaskInput{Title: "Today task", Start: StartDate, StartDate: "2026-05-04"}); err != nil {
		t.Fatalf("create task: %v", err)
	}
	model := NewModel(store)
	model.now = fixedClock("2026-05-04")
	model.view = KindToday
	model.width = 100
	model.height = 18

	view := model.View()
	if got := len(strings.Split(view, "\n")); got != 18 {
		t.Fatalf("expected today view to fit available height, got %d lines:\n%s", got, view)
	}
	assertViewWidth(t, view, model.width)
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
	if got := len(strings.Split(view, "\n")); got != 18 {
		t.Fatalf("expected weekly view to use available height, got %d lines:\n%s", got, view)
	}
	if !strings.Contains(view, "Weekly task") {
		t.Fatalf("expected weekly task in view:\n%s", view)
	}
	assertViewWidth(t, view, model.width)
}

func assertViewWidth(t *testing.T, view string, width int) {
	t.Helper()
	for i, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("expected line %d to fit width %d, got %d:\n%s", i+1, width, got, view)
		}
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
	}, false, 80)

	if strings.Contains(line, "[38;5;") || strings.Contains(line, "[0m") {
		t.Fatalf("completed line leaked ansi sequence: %q", line)
	}
}
