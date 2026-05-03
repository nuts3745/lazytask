package lazytask

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

type Store struct {
	mu    sync.RWMutex
	log   *EventLog
	tasks map[string]Task
	order []string
	now   func() time.Time
}

func NewStore(log *EventLog) (*Store, error) {
	store := &Store{
		log:   log,
		tasks: make(map[string]Task),
		order: make([]string, 0),
		now:   time.Now,
	}
	events, err := log.Load()
	if err != nil {
		return nil, err
	}
	for _, event := range events {
		if err := store.apply(event); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func NewMemoryStore(events ...Event) (*Store, error) {
	store := &Store{
		tasks: make(map[string]Task),
		order: make([]string, 0),
		now:   time.Now,
	}
	for _, event := range events {
		if err := store.apply(event); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func (s *Store) SetClock(now func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = now
}

func (s *Store) Create(input TaskInput) (Task, error) {
	input = input.normalized()
	if err := input.Validate(); err != nil {
		return Task{}, err
	}
	now := s.now().In(time.Local)
	task := Task{
		ID:        newID("task"),
		Title:     input.Title,
		Notes:     input.Notes,
		Start:     input.Start,
		StartDate: input.StartDate,
		Deadline:  input.Deadline,
		Project:   input.Project,
		Area:      input.Area,
		Tags:      input.Tags,
		CreatedAt: now,
		UpdatedAt: now,
	}
	event, err := newEvent(EventTaskCreated, task.ID, now, task)
	if err != nil {
		return Task{}, err
	}
	if err := s.commit(event); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *Store) Update(id string, input TaskInput) (Task, error) {
	input = input.normalized()
	if err := input.Validate(); err != nil {
		return Task{}, err
	}
	now := s.now().In(time.Local)
	payload := updatePayload{
		Title:     input.Title,
		Notes:     input.Notes,
		Start:     input.Start,
		StartDate: input.StartDate,
		Deadline:  input.Deadline,
		Project:   input.Project,
		Area:      input.Area,
		Tags:      input.Tags,
	}
	event, err := newEvent(EventTaskUpdated, id, now, payload)
	if err != nil {
		return Task{}, err
	}
	if err := s.commit(event); err != nil {
		return Task{}, err
	}
	task, _ := s.Get(id)
	return task, nil
}

func (s *Store) Complete(id, date string) error {
	if date == "" {
		date = FormatLocalDate(s.now())
	}
	if err := validateDate("completedAt", date); err != nil {
		return err
	}
	event, err := newEvent(EventTaskCompleted, id, s.now().In(time.Local), completePayload{CompletedAt: date})
	if err != nil {
		return err
	}
	return s.commit(event)
}

func (s *Store) Uncomplete(id string) error {
	event, err := newEvent(EventTaskUncompleted, id, s.now().In(time.Local), nil)
	if err != nil {
		return err
	}
	return s.commit(event)
}

func (s *Store) Cancel(id, date string) error {
	if date == "" {
		date = FormatLocalDate(s.now())
	}
	if err := validateDate("canceledAt", date); err != nil {
		return err
	}
	event, err := newEvent(EventTaskCanceled, id, s.now().In(time.Local), cancelPayload{CanceledAt: date})
	if err != nil {
		return err
	}
	return s.commit(event)
}

func (s *Store) Delete(id string) error {
	event, err := newEvent(EventTaskDeleted, id, s.now().In(time.Local), nil)
	if err != nil {
		return err
	}
	return s.commit(event)
}

func (s *Store) Get(id string) (Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[id]
	return task, ok && !task.Deleted
}

func (s *Store) List() []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tasks := make([]Task, 0, len(s.order))
	for _, id := range s.order {
		task := s.tasks[id]
		if !task.Deleted {
			tasks = append(tasks, task)
		}
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].StartDate != tasks[j].StartDate {
			if tasks[i].StartDate == "" {
				return false
			}
			if tasks[j].StartDate == "" {
				return true
			}
			return tasks[i].StartDate < tasks[j].StartDate
		}
		return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
	})
	return tasks
}

func (s *Store) commit(event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := s.clone()
	if err := next.apply(event); err != nil {
		return err
	}
	if s.log != nil {
		if err := s.log.Append(event); err != nil {
			return err
		}
	}
	s.tasks = next.tasks
	s.order = next.order
	return nil
}

func (s *Store) apply(event Event) error {
	switch event.Type {
	case EventTaskCreated:
		var task Task
		if err := json.Unmarshal(event.Payload, &task); err != nil {
			return err
		}
		if task.ID == "" {
			task.ID = event.TaskID
		}
		if task.Start == "" {
			return fmt.Errorf("unsupported old task payload for %s: remove or recreate the JSONL log", event.TaskID)
		}
		if err := task.Input().Validate(); err != nil {
			return err
		}
		if _, exists := s.tasks[task.ID]; !exists {
			s.order = append(s.order, task.ID)
		}
		s.tasks[task.ID] = task
	case EventTaskUpdated:
		task, ok := s.tasks[event.TaskID]
		if !ok || task.Deleted {
			return fmt.Errorf("task not found: %s", event.TaskID)
		}
		var payload updatePayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		task.Title = payload.Title
		task.Notes = payload.Notes
		task.Start = payload.Start
		task.StartDate = payload.StartDate
		task.Deadline = payload.Deadline
		task.Project = payload.Project
		task.Area = payload.Area
		task.Tags = normalizeTags(payload.Tags)
		task.UpdatedAt = event.Timestamp
		if err := task.Input().Validate(); err != nil {
			return err
		}
		s.tasks[event.TaskID] = task
	case EventTaskCompleted:
		task, ok := s.tasks[event.TaskID]
		if !ok || task.Deleted {
			return fmt.Errorf("task not found: %s", event.TaskID)
		}
		var payload completePayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		task.CompletedAt = payload.CompletedAt
		task.UpdatedAt = event.Timestamp
		s.tasks[event.TaskID] = task
	case EventTaskUncompleted:
		task, ok := s.tasks[event.TaskID]
		if !ok || task.Deleted {
			return fmt.Errorf("task not found: %s", event.TaskID)
		}
		task.CompletedAt = ""
		task.UpdatedAt = event.Timestamp
		s.tasks[event.TaskID] = task
	case EventTaskCanceled:
		task, ok := s.tasks[event.TaskID]
		if !ok || task.Deleted {
			return fmt.Errorf("task not found: %s", event.TaskID)
		}
		var payload cancelPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		task.CanceledAt = payload.CanceledAt
		task.UpdatedAt = event.Timestamp
		s.tasks[event.TaskID] = task
	case EventTaskDeleted:
		task, ok := s.tasks[event.TaskID]
		if !ok {
			return fmt.Errorf("task not found: %s", event.TaskID)
		}
		task.Deleted = true
		task.UpdatedAt = event.Timestamp
		s.tasks[event.TaskID] = task
	default:
		return fmt.Errorf("unknown event type: %s", event.Type)
	}
	return nil
}

func (s *Store) clone() *Store {
	next := &Store{
		log:   s.log,
		tasks: make(map[string]Task, len(s.tasks)),
		order: append([]string(nil), s.order...),
		now:   s.now,
	}
	for id, task := range s.tasks {
		task.Tags = append([]string(nil), task.Tags...)
		next.tasks[id] = task
	}
	return next
}

type updatePayload struct {
	Title     string   `json:"title"`
	Notes     string   `json:"notes,omitempty"`
	Start     Start    `json:"start"`
	StartDate string   `json:"startDate,omitempty"`
	Deadline  string   `json:"deadline,omitempty"`
	Project   string   `json:"project,omitempty"`
	Area      string   `json:"area,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

type completePayload struct {
	CompletedAt string `json:"completedAt"`
}

type cancelPayload struct {
	CanceledAt string `json:"canceledAt"`
}
