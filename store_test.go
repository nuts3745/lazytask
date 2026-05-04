package lazytask

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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
		Title:     "Plan week",
		Start:     StartDate,
		StartDate: "2026-05-04",
		Project:   "Work",
		Tags:      []string{"planning", "planning"},
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

func TestEventLogReportsOldTaskPayload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lazytask.jsonl")
	old := `{"eventID":"evt_old","type":"task_created","taskID":"task_old","timestamp":"2026-05-04T00:00:00Z","payload":{"id":"task_old","title":"Old","when":"2026-05-04","createdAt":"2026-05-04T00:00:00Z","updatedAt":"2026-05-04T00:00:00Z"}}` + "\n"
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	if _, err := NewStore(NewEventLog(path)); err == nil {
		t.Fatal("expected old payload error")
	}
}

func TestEventLogCompactPreservesCurrentProjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lazytask.jsonl")
	log := NewEventLog(path)
	store, err := NewStore(log)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	store.SetClock(fixedClock("2026-05-04"))

	active, err := store.Create(TaskInput{Title: "Draft plan", Start: StartInbox, Project: "Work"})
	if err != nil {
		t.Fatalf("create active: %v", err)
	}
	active, err = store.Update(active.ID, TaskInput{
		Title:     "Plan week",
		Start:     StartDate,
		StartDate: "2026-05-05",
		Project:   "Work",
		Tags:      []string{"planning"},
	})
	if err != nil {
		t.Fatalf("update active: %v", err)
	}
	if err := store.Complete(active.ID, "2026-05-05"); err != nil {
		t.Fatalf("complete active: %v", err)
	}
	if err := store.Uncomplete(active.ID); err != nil {
		t.Fatalf("uncomplete active: %v", err)
	}
	if err := store.SetWIP(active.ID); err != nil {
		t.Fatalf("set active wip: %v", err)
	}

	done, err := store.Create(TaskInput{Title: "Done task", Start: StartAnytime})
	if err != nil {
		t.Fatalf("create done: %v", err)
	}
	if err := store.Complete(done.ID, "2026-05-04"); err != nil {
		t.Fatalf("complete done: %v", err)
	}

	canceled, err := store.Create(TaskInput{Title: "Canceled task", Start: StartSomeday})
	if err != nil {
		t.Fatalf("create canceled: %v", err)
	}
	if err := store.Cancel(canceled.ID, "2026-05-04"); err != nil {
		t.Fatalf("cancel task: %v", err)
	}

	deleted, err := store.Create(TaskInput{Title: "Deleted task", Start: StartInbox})
	if err != nil {
		t.Fatalf("create deleted: %v", err)
	}
	if err := store.Delete(deleted.ID); err != nil {
		t.Fatalf("delete task: %v", err)
	}

	beforeRaw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read original log: %v", err)
	}
	beforeEvents, err := log.Load()
	if err != nil {
		t.Fatalf("load original events: %v", err)
	}
	beforeList := store.List()
	beforeTasks := tasksByID(t, store, beforeList)

	result, err := log.Compact()
	if err != nil {
		t.Fatalf("compact log: %v", err)
	}
	if result.Before != len(beforeEvents) || result.After != 3 {
		t.Fatalf("unexpected compact result: %#v before events=%d", result, len(beforeEvents))
	}

	backupRaw, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("read backup log: %v", err)
	}
	if string(backupRaw) != string(beforeRaw) {
		t.Fatal("backup should preserve original log content")
	}

	afterEvents, err := log.Load()
	if err != nil {
		t.Fatalf("load compacted events: %v", err)
	}
	if len(afterEvents) != 3 {
		t.Fatalf("expected one event per non-deleted task, got %d", len(afterEvents))
	}
	for _, event := range afterEvents {
		if event.Type != EventTaskCreated {
			t.Fatalf("expected compacted task_created event, got %#v", event)
		}
		var task Task
		if err := json.Unmarshal(event.Payload, &task); err != nil {
			t.Fatalf("unmarshal compacted task payload: %v", err)
		}
		if event.TaskID != task.ID {
			t.Fatalf("expected taskID to match payload id, event=%#v task=%#v", event, task)
		}
	}

	reloaded, err := NewStore(log)
	if err != nil {
		t.Fatalf("reload compacted store: %v", err)
	}
	if _, ok := reloaded.Get(deleted.ID); ok {
		t.Fatal("deleted task should not exist after compaction")
	}
	if got := reloaded.List(); !reflect.DeepEqual(got, beforeList) {
		t.Fatalf("list changed after compaction:\nbefore=%#v\nafter=%#v", beforeList, got)
	}
	if got := tasksByID(t, reloaded, beforeList); !reflect.DeepEqual(got, beforeTasks) {
		t.Fatalf("tasks changed after compaction:\nbefore=%#v\nafter=%#v", beforeTasks, got)
	}

	gotActive, ok := reloaded.Get(active.ID)
	if !ok {
		t.Fatal("expected active task after compaction")
	}
	if !gotActive.WIP {
		t.Fatalf("expected active WIP to survive compaction: %#v", gotActive)
	}
	if gotLogbook := LogbookTasks(reloaded.List()); len(gotLogbook) != 2 {
		t.Fatalf("expected completed and canceled tasks in Logbook, got %#v", gotLogbook)
	}
}

func TestEventLogCompactRejectsMalformedLogWithoutReplacingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lazytask.jsonl")
	original := []byte("{bad json}\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	if _, err := NewEventLog(path).Compact(); err == nil {
		t.Fatal("expected compact malformed log to fail")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if string(got) != string(original) {
		t.Fatalf("malformed log should not be replaced, got %q", got)
	}
}

func TestTaskInputValidation(t *testing.T) {
	if err := (TaskInput{Title: "  "}).Validate(); err == nil {
		t.Fatal("expected missing title error")
	}
	if err := (TaskInput{Title: "Task", Start: StartDate, StartDate: "05-04-2026"}).Validate(); err == nil {
		t.Fatal("expected invalid start date error")
	}
}

func TestDeleteUsesTombstoneProjection(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	task, err := store.Create(TaskInput{Title: "Remove me", Start: StartDate, StartDate: "2026-05-04"})
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

func TestWIPReplayMaintainsSingleActiveTask(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	first, err := store.Create(TaskInput{Title: "First", Start: StartInbox})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	second, err := store.Create(TaskInput{Title: "Second", Start: StartAnytime})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}

	if err := store.SetWIP(first.ID); err != nil {
		t.Fatalf("set first wip: %v", err)
	}
	gotFirst, _ := store.Get(first.ID)
	if !gotFirst.WIP {
		t.Fatalf("expected first task to be WIP: %#v", gotFirst)
	}

	if err := store.SetWIP(second.ID); err != nil {
		t.Fatalf("set second wip: %v", err)
	}
	gotFirst, _ = store.Get(first.ID)
	gotSecond, _ := store.Get(second.ID)
	if gotFirst.WIP || !gotSecond.WIP {
		t.Fatalf("expected only second WIP, first=%#v second=%#v", gotFirst, gotSecond)
	}

	if err := store.ClearWIP(second.ID); err != nil {
		t.Fatalf("clear second wip: %v", err)
	}
	gotSecond, _ = store.Get(second.ID)
	if gotSecond.WIP {
		t.Fatalf("expected second WIP cleared: %#v", gotSecond)
	}
}

func TestWIPClearsOnTerminalTaskEvents(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*Store, string) error
	}{
		{name: "complete", run: func(store *Store, id string) error { return store.Complete(id, "2026-05-04") }},
		{name: "cancel", run: func(store *Store, id string) error { return store.Cancel(id, "2026-05-04") }},
		{name: "delete", run: func(store *Store, id string) error { return store.Delete(id) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, err := NewMemoryStore()
			if err != nil {
				t.Fatalf("new memory store: %v", err)
			}
			task, err := store.Create(TaskInput{Title: "Marked", Start: StartInbox})
			if err != nil {
				t.Fatalf("create task: %v", err)
			}
			if err := store.SetWIP(task.ID); err != nil {
				t.Fatalf("set wip: %v", err)
			}
			if err := tc.run(store, task.ID); err != nil {
				t.Fatalf("%s task: %v", tc.name, err)
			}
			for _, got := range store.tasks {
				if got.WIP {
					t.Fatalf("expected terminal event to clear WIP: %#v", got)
				}
			}
		})
	}
}

func TestSetWIPRejectsInactiveTask(t *testing.T) {
	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	task, err := store.Create(TaskInput{Title: "Done", Start: StartInbox})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := store.Complete(task.ID, "2026-05-04"); err != nil {
		t.Fatalf("complete task: %v", err)
	}
	if err := store.SetWIP(task.ID); err == nil {
		t.Fatal("expected completed task WIP selection to fail")
	}
}

func TestOldTaskPayloadWithoutWIPReplays(t *testing.T) {
	payload := struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		Start     Start     `json:"start"`
		CreatedAt time.Time `json:"createdAt"`
		UpdatedAt time.Time `json:"updatedAt"`
	}{
		ID:        "task_old",
		Title:     "Old task",
		Start:     StartInbox,
		CreatedAt: time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	store, err := NewMemoryStore(Event{
		EventID:   "evt_old",
		Type:      EventTaskCreated,
		TaskID:    "task_old",
		Timestamp: time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		Payload:   raw,
	})
	if err != nil {
		t.Fatalf("replay old payload: %v", err)
	}
	task, ok := store.Get("task_old")
	if !ok {
		t.Fatal("expected old task")
	}
	if task.WIP {
		t.Fatalf("expected missing wip field to default false: %#v", task)
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

func tasksByID(t *testing.T, store *Store, tasks []Task) map[string]Task {
	t.Helper()
	out := make(map[string]Task, len(tasks))
	for _, task := range tasks {
		got, ok := store.Get(task.ID)
		if !ok {
			t.Fatalf("task not found: %s", task.ID)
		}
		out[task.ID] = got
	}
	return out
}
