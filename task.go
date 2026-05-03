package lazytask

import (
	"errors"
	"fmt"
)

// Status represents the lifecycle state of a task.
type Status string

const (
	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusDone    Status = "done"
	StatusFailed  Status = "failed"
)

// Task is the core task model used by LazyTask.
type Task struct {
	ID          string
	Name        string
	Description string
	Command     []string
	Status      Status
}

// Validate checks whether the task has the minimum fields needed to run.
func (t Task) Validate() error {
	if t.ID == "" {
		return errors.New("task id is required")
	}
	if t.Name == "" {
		return errors.New("task name is required")
	}
	if len(t.Command) == 0 {
		return errors.New("task command is required")
	}
	if t.Command[0] == "" {
		return errors.New("task command executable is required")
	}
	if t.Status == "" {
		return errors.New("task status is required")
	}
	if !t.Status.Valid() {
		return fmt.Errorf("invalid task status: %s", t.Status)
	}
	return nil
}

// Valid reports whether the status is known.
func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusRunning, StatusDone, StatusFailed:
		return true
	default:
		return false
	}
}

// NewTask creates a pending task.
func NewTask(id, name string, command ...string) Task {
	return Task{
		ID:      id,
		Name:    name,
		Command: command,
		Status:  StatusPending,
	}
}
