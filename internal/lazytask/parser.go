package lazytask

import (
	"strings"
	"time"
)

func ParseQuickTask(value string, now time.Time) (TaskInput, error) {
	fields := strings.Fields(value)
	input := TaskInput{Start: StartInbox}
	title := make([]string, 0, len(fields))
	for _, field := range fields {
		switch {
		case strings.HasPrefix(field, "#") && len(field) > 1:
			input.Tags = append(input.Tags, strings.TrimPrefix(field, "#"))
		case strings.HasPrefix(field, ">") && len(field) > 1:
			input.Project = strings.TrimPrefix(field, ">")
		case strings.HasPrefix(field, "/") && len(field) > 1:
			input.Area = strings.TrimPrefix(field, "/")
		case strings.HasPrefix(field, "!") && len(field) > 1:
			input.Deadline = strings.TrimPrefix(field, "!")
		case strings.HasPrefix(field, "@") && len(field) > 1:
			if err := applyStartToken(&input, strings.TrimPrefix(field, "@"), now); err != nil {
				return TaskInput{}, err
			}
		default:
			title = append(title, field)
		}
	}
	input.Title = strings.Join(title, " ")
	input = input.normalized()
	return input, input.Validate()
}

func applyStartToken(input *TaskInput, token string, now time.Time) error {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "today":
		input.Start = StartDate
		input.StartDate = FormatLocalDate(now)
	case "tomorrow":
		input.Start = StartDate
		input.StartDate = FormatLocalDate(now.AddDate(0, 0, 1))
	case "anytime":
		input.Start = StartAnytime
		input.StartDate = ""
	case "someday":
		input.Start = StartSomeday
		input.StartDate = ""
	case "inbox":
		input.Start = StartInbox
		input.StartDate = ""
	case "clear":
		input.Start = StartInbox
		input.StartDate = ""
	default:
		if err := validateDate("startDate", token); err != nil {
			return err
		}
		input.Start = StartDate
		input.StartDate = token
	}
	return nil
}

func ApplyWhen(input *TaskInput, token string, now time.Time) error {
	token = strings.TrimPrefix(strings.TrimSpace(token), "@")
	return applyStartToken(input, token, now)
}

func ApplyDeadline(input *TaskInput, token string) error {
	token = strings.TrimPrefix(strings.TrimSpace(token), "!")
	if token == "clear" {
		input.Deadline = ""
		return nil
	}
	if err := validateDate("deadline", token); err != nil {
		return err
	}
	input.Deadline = token
	return nil
}
