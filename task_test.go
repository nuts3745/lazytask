package lazytask

import "testing"

func TestNewTask(t *testing.T) {
	task := NewTask("build", "Build", "go", "build", "./...")

	if task.ID != "build" {
		t.Fatalf("expected id build, got %s", task.ID)
	}
	if task.Status != StatusPending {
		t.Fatalf("expected pending status, got %s", task.Status)
	}
	if err := task.Validate(); err != nil {
		t.Fatalf("expected valid task: %v", err)
	}
}

func TestStoreListPreservesInsertionOrder(t *testing.T) {
	store := NewStore()

	if err := store.Add(NewTask("test", "Test", "go", "test", "./...")); err != nil {
		t.Fatalf("add test task: %v", err)
	}
	if err := store.Add(NewTask("build", "Build", "go", "build", "./...")); err != nil {
		t.Fatalf("add build task: %v", err)
	}

	tasks := store.List()
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].ID != "test" || tasks[1].ID != "build" {
		t.Fatalf("unexpected order: %#v", tasks)
	}
}
