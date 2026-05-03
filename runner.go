package lazytask

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// Result contains the process output from a task run.
type Result struct {
	TaskID string
	Code   int
	Output []byte
}

// Runner executes tasks from a store.
type Runner struct {
	store *Store
}

// NewRunner creates a task runner backed by store.
func NewRunner(store *Store) *Runner {
	return &Runner{store: store}
}

// Run executes a task command and records its final status.
func (r *Runner) Run(ctx context.Context, id string) (Result, error) {
	task, ok := r.store.Get(id)
	if !ok {
		return Result{}, fmt.Errorf("task not found: %s", id)
	}
	if err := task.Validate(); err != nil {
		return Result{}, err
	}

	if err := r.store.UpdateStatus(id, StatusRunning); err != nil {
		return Result{}, err
	}

	cmd := exec.CommandContext(ctx, task.Command[0], task.Command[1:]...)
	output, err := cmd.CombinedOutput()

	result := Result{
		TaskID: id,
		Code:   exitCode(err),
		Output: output,
	}

	if err != nil {
		_ = r.store.UpdateStatus(id, StatusFailed)
		return result, err
	}

	if err := r.store.UpdateStatus(id, StatusDone); err != nil {
		return result, err
	}
	return result, nil
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if ok := errors.As(err, &exitErr); ok {
		return exitErr.ExitCode()
	}
	return -1
}
