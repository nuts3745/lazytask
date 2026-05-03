package lazytask

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelSwitchesViewsAndCompletesSelectedTask(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	store.SetClock(fixedClock("2026-05-04"))
	task, err := store.Create(TaskInput{Title: "Ship MVP", When: "2026-05-04"})
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
	if model.mode != ViewWeek {
		t.Fatalf("expected week view, got %v", model.mode)
	}
}

func TestTaskFormRejectsEmptyTitle(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	model := NewModel(store)
	model.form = newTaskForm("", TaskInput{})

	err = model.saveForm()
	if err == nil {
		t.Fatal("expected validation error")
	}
}
