package lazytask

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const DateLayout = "2006-01-02"

type Start string

const (
	StartInbox   Start = "inbox"
	StartAnytime Start = "anytime"
	StartSomeday Start = "someday"
	StartDate    Start = "date"
)

// Task is a Things-style todo item projected from the event log.
type Task struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Notes       string    `json:"notes,omitempty"`
	Start       Start     `json:"start"`
	StartDate   string    `json:"startDate,omitempty"`
	Deadline    string    `json:"deadline,omitempty"`
	CompletedAt string    `json:"completedAt,omitempty"`
	CanceledAt  string    `json:"canceledAt,omitempty"`
	WIP         bool      `json:"wip,omitempty"`
	Project     string    `json:"project,omitempty"`
	Area        string    `json:"area,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Deleted     bool      `json:"-"`
}

type TaskInput struct {
	Title     string
	Notes     string
	Start     Start
	StartDate string
	Deadline  string
	Project   string
	Area      string
	Tags      []string
}

func (in TaskInput) Validate() error {
	if strings.TrimSpace(in.Title) == "" {
		return errors.New("task title is required")
	}
	if in.Start == "" {
		in.Start = StartInbox
	}
	if !in.Start.Valid() {
		return fmt.Errorf("invalid start: %s", in.Start)
	}
	if in.Start == StartDate && strings.TrimSpace(in.StartDate) == "" {
		return errors.New("start date is required")
	}
	if in.Start != StartDate && strings.TrimSpace(in.StartDate) != "" {
		return errors.New("start date requires date start")
	}
	if err := validateDate("startDate", in.StartDate); err != nil {
		return err
	}
	if err := validateDate("deadline", in.Deadline); err != nil {
		return err
	}
	return nil
}

func (in TaskInput) normalized() TaskInput {
	in.Title = strings.TrimSpace(in.Title)
	in.Notes = strings.TrimSpace(in.Notes)
	if in.Start == "" {
		in.Start = StartInbox
	}
	in.StartDate = strings.TrimSpace(in.StartDate)
	in.Deadline = strings.TrimSpace(in.Deadline)
	in.Project = strings.TrimSpace(in.Project)
	in.Area = strings.TrimSpace(in.Area)
	in.Tags = normalizeTags(in.Tags)
	return in
}

func (t Task) Input() TaskInput {
	return TaskInput{
		Title:     t.Title,
		Notes:     t.Notes,
		Start:     t.Start,
		StartDate: t.StartDate,
		Deadline:  t.Deadline,
		Project:   t.Project,
		Area:      t.Area,
		Tags:      append([]string(nil), t.Tags...),
	}
}

func (s Start) Valid() bool {
	switch s {
	case StartInbox, StartAnytime, StartSomeday, StartDate:
		return true
	default:
		return false
	}
}

func (t Task) Active() bool {
	return !t.Deleted && t.CompletedAt == "" && t.CanceledAt == ""
}

func ParseLocalDate(value string) (time.Time, error) {
	return time.ParseInLocation(DateLayout, value, time.Local)
}

func FormatLocalDate(t time.Time) string {
	return t.In(time.Local).Format(DateLayout)
}

func validateDate(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if _, err := ParseLocalDate(strings.TrimSpace(value)); err != nil {
		return errors.New(name + " must use YYYY-MM-DD")
	}
	return nil
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.TrimPrefix(tag, "#"))
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func SplitTags(value string) []string {
	return normalizeTags(strings.Split(value, ","))
}

func JoinTags(tags []string) string {
	return strings.Join(normalizeTags(tags), ", ")
}
