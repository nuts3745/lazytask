package lazytask

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type EventType string

const (
	EventTaskCreated     EventType = "task_created"
	EventTaskUpdated     EventType = "task_updated"
	EventTaskCompleted   EventType = "task_completed"
	EventTaskUncompleted EventType = "task_uncompleted"
	EventTaskCanceled    EventType = "task_canceled"
	EventTaskDeleted     EventType = "task_deleted"
	EventTaskWIPSelected EventType = "task_wip_selected"
	EventTaskWIPCleared  EventType = "task_wip_cleared"
)

type Event struct {
	EventID   string          `json:"eventID"`
	Type      EventType       `json:"type"`
	TaskID    string          `json:"taskID"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type EventLog struct {
	path string
}

type CompactResult struct {
	Before int
	After  int
}

func NewEventLog(path string) *EventLog {
	return &EventLog{path: path}
}

func (l *EventLog) Path() string {
	return l.path
}

func (l *EventLog) Load() ([]Event, error) {
	file, err := os.Open(l.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var events []Event
	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		text := scanner.Bytes()
		if len(text) == 0 {
			continue
		}
		var event Event
		if err := json.Unmarshal(text, &event); err != nil {
			return nil, fmt.Errorf("read %s line %d: %w", l.path, line, err)
		}
		if event.EventID == "" || event.Type == "" || event.TaskID == "" || event.Timestamp.IsZero() {
			return nil, fmt.Errorf("read %s line %d: invalid event", l.path, line)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (l *EventLog) Append(event Event) error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return file.Sync()
}

func (l *EventLog) Compact() (CompactResult, error) {
	events, err := l.Load()
	if err != nil {
		return CompactResult{}, err
	}
	store, err := NewMemoryStore(events...)
	if err != nil {
		return CompactResult{}, err
	}

	now := time.Now().In(time.Local)
	compacted := make([]Event, 0, len(store.order))
	for _, id := range store.order {
		task := store.tasks[id]
		if task.Deleted {
			continue
		}
		event, err := newEvent(EventTaskCreated, task.ID, now, task)
		if err != nil {
			return CompactResult{}, err
		}
		compacted = append(compacted, event)
	}

	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return CompactResult{}, err
	}
	if err := copyFile(l.path, l.path+".bak"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return CompactResult{}, err
	}
	if err := l.replace(compacted); err != nil {
		return CompactResult{}, err
	}
	return CompactResult{Before: len(events), After: len(compacted)}, nil
}

func (l *EventLog) replace(events []Event) error {
	dir := filepath.Dir(l.path)
	tmp, err := os.CreateTemp(dir, filepath.Base(l.path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	encoder := json.NewEncoder(tmp)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			tmp.Close()
			return err
		}
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, l.path)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func newEvent(kind EventType, taskID string, now time.Time, payload any) (Event, error) {
	var raw json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return Event{}, err
		}
		raw = data
	}
	return Event{
		EventID:   newID("evt"),
		Type:      kind,
		TaskID:    taskID,
		Timestamp: now,
		Payload:   raw,
	}, nil
}

func newID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}
