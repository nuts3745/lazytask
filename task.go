package lazytask

import (
	"errors"
	"strings"
	"time"
)

const DateLayout = "2006-01-02"

// Task is a Things-style todo item projected from the event log.
type Task struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Notes       string    `json:"notes,omitempty"`
	When        string    `json:"when,omitempty"`
	Deadline    string    `json:"deadline,omitempty"`
	CompletedAt string    `json:"completedAt,omitempty"`
	Canceled    bool      `json:"canceled,omitempty"`
	Project     string    `json:"project,omitempty"`
	Area        string    `json:"area,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Deleted     bool      `json:"-"`
}

type TaskInput struct {
	Title    string
	Notes    string
	When     string
	Deadline string
	Project  string
	Area     string
	Tags     []string
}

func (in TaskInput) Validate() error {
	if strings.TrimSpace(in.Title) == "" {
		return errors.New("task title is required")
	}
	if err := validateDate("when", in.When); err != nil {
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
	in.When = strings.TrimSpace(in.When)
	in.Deadline = strings.TrimSpace(in.Deadline)
	in.Project = strings.TrimSpace(in.Project)
	in.Area = strings.TrimSpace(in.Area)
	in.Tags = normalizeTags(in.Tags)
	return in
}

func (t Task) Input() TaskInput {
	return TaskInput{
		Title:    t.Title,
		Notes:    t.Notes,
		When:     t.When,
		Deadline: t.Deadline,
		Project:  t.Project,
		Area:     t.Area,
		Tags:     append([]string(nil), t.Tags...),
	}
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
