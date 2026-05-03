package lazytask

import "time"

type WeekDay struct {
	Date  string
	Label string
	Tasks []Task
}

func TodayTasks(tasks []Task, day time.Time) []Task {
	date := FormatLocalDate(day)
	out := make([]Task, 0)
	for _, task := range tasks {
		if task.Deleted {
			continue
		}
		if task.When == date || task.CompletedAt == date {
			out = append(out, task)
		}
	}
	return out
}

func WorkWeek(tasks []Task, day time.Time) []WeekDay {
	monday := MondayOf(day)
	labels := []string{"Mon", "Tue", "Wed", "Thu", "Fri"}
	week := make([]WeekDay, 5)
	for i := range week {
		date := monday.AddDate(0, 0, i)
		week[i] = WeekDay{
			Date:  FormatLocalDate(date),
			Label: labels[i],
			Tasks: make([]Task, 0),
		}
	}
	for _, task := range tasks {
		if task.Deleted {
			continue
		}
		for i := range week {
			if task.When == week[i].Date || task.CompletedAt == week[i].Date {
				week[i].Tasks = append(week[i].Tasks, task)
				break
			}
		}
	}
	return week
}

func MondayOf(day time.Time) time.Time {
	day = day.In(time.Local)
	weekday := int(day.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return day.AddDate(0, 0, 1-weekday)
}

func FlattenWeek(week []WeekDay) []Task {
	var out []Task
	seen := make(map[string]struct{})
	for _, day := range week {
		for _, task := range day.Tasks {
			if _, ok := seen[task.ID]; ok {
				continue
			}
			seen[task.ID] = struct{}{}
			out = append(out, task)
		}
	}
	return out
}
