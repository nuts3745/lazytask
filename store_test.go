package lazytask

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEventLogReplaysTaskLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lazytask.jsonl")
	store, err := NewStore(NewEventLog(path))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	store.SetClock(fixedClock("2026-05-04"))

	task, err := store.Create(TaskInput{
		Title:   "Plan week",
		When:    "2026-05-04",
		Project: "Work",
		Tags:    []string{"planning", "planning"},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := store.Complete(task.ID, "2026-05-05"); err != nil {
		t.Fatalf("complete task: %v", err)
	}

	reloaded, err := NewStore(NewEventLog(path))
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	got, ok := reloaded.Get(task.ID)
	if !ok {
		t.Fatal("expected task after replay")
	}
	if got.CompletedAt != "2026-05-05" {
		t.Fatalf("expected completed date, got %q", got.CompletedAt)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "planning" {
		t.Fatalf("expected normalized tags, got %#v", got.Tags)
	}
}

func TestEventLogReportsMalformedJSONLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lazytask.jsonl")
	if err := os.WriteFile(path, []byte("{bad json}\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	if _, err := NewStore(NewEventLog(path)); err == nil {
		t.Fatal("expected malformed log error")
	}
}

func TestTaskInputValidation(t *testing.T) {
	if err := (TaskInput{Title: "  "}).Validate(); err == nil {
		t.Fatal("expected missing title error")
	}
	if err := (TaskInput{Title: "Task", When: "05-04-2026"}).Validate(); err == nil {
		t.Fatal("expected invalid when date error")
	}
}

func TestDeleteUsesTombstoneProjection(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	task, err := store.Create(TaskInput{Title: "Remove me", When: "2026-05-04"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := store.Delete(task.ID); err != nil {
		t.Fatalf("delete task: %v", err)
	}
	if _, ok := store.Get(task.ID); ok {
		t.Fatal("deleted task should not be visible")
	}
	if len(store.List()) != 0 {
		t.Fatalf("deleted task should be excluded from list: %#v", store.List())
	}
}

func fixedClock(date string) func() time.Time {
	return func() time.Time {
		t, err := ParseLocalDate(date)
		if err != nil {
			panic(err)
		}
		return t
	}
}
