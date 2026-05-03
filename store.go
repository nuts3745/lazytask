package lazytask

import (
	"fmt"
	"sync"
)

// Store keeps tasks in memory.
type Store struct {
	mu    sync.RWMutex
	tasks map[string]Task
	order []string
}

// NewStore creates an empty in-memory task store.
func NewStore() *Store {
	return &Store{
		tasks: make(map[string]Task),
		order: make([]string, 0),
	}
}

// Add validates and stores a task.
func (s *Store) Add(task Task) error {
	if err := task.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID]; exists {
		return fmt.Errorf("task already exists: %s", task.ID)
	}

	s.tasks[task.ID] = task
	s.order = append(s.order, task.ID)
	return nil
}

// Get returns a task by id.
func (s *Store) Get(id string) (Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[id]
	return task, ok
}

// List returns tasks in insertion order.
func (s *Store) List() []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]Task, 0, len(s.order))
	for _, id := range s.order {
		tasks = append(tasks, s.tasks[id])
	}
	return tasks
}

// UpdateStatus changes a task status.
func (s *Store) UpdateStatus(id string, status Status) error {
	if !status.Valid() {
		return fmt.Errorf("invalid task status: %s", status)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}

	task.Status = status
	s.tasks[id] = task
	return nil
}
