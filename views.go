package lazytask

import (
	"sort"
	"strings"
	"time"
)

type ViewKind string

const (
	KindInbox   ViewKind = "Inbox"
	KindToday   ViewKind = "Today"
	KindWeekly  ViewKind = "Weekly"
	KindAnytime ViewKind = "Anytime"
	KindSomeday ViewKind = "Someday"
	KindLogbook ViewKind = "Logbook"
	KindFilter  ViewKind = "Filter"
)

type Filter struct {
	Tag     string
	Project string
	Area    string
	Query   string
}

type WeekDay struct {
	Date  string
	Label string
	Tasks []Task
}

func InboxTasks(tasks []Task) []Task {
	return filterTasks(tasks, func(task Task) bool {
		return task.Active() && task.Start == StartInbox
	})
}

func TodayTasks(tasks []Task, day time.Time) []Task {
	date := FormatLocalDate(day)
	return filterTasks(tasks, func(task Task) bool {
		if task.Deleted {
			return false
		}
		if task.CompletedAt == date || task.CanceledAt == date {
			return true
		}
		return task.Active() && (task.StartDate == date || task.Deadline == date)
	})
}

func AnytimeTasks(tasks []Task) []Task {
	return filterTasks(tasks, func(task Task) bool {
		return task.Active() && task.Start == StartAnytime
	})
}

func SomedayTasks(tasks []Task) []Task {
	return filterTasks(tasks, func(task Task) bool {
		return task.Active() && task.Start == StartSomeday
	})
}

func LogbookTasks(tasks []Task) []Task {
	return filterTasks(tasks, func(task Task) bool {
		return !task.Deleted && (task.CompletedAt != "" || task.CanceledAt != "")
	})
}

func FilteredTasks(tasks []Task, filter Filter) []Task {
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	tag := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(filter.Tag)), "#")
	project := strings.ToLower(strings.TrimSpace(filter.Project))
	area := strings.ToLower(strings.TrimSpace(filter.Area))
	return filterTasks(tasks, func(task Task) bool {
		if task.Deleted {
			return false
		}
		if tag != "" && !hasTag(task, tag) {
			return false
		}
		if project != "" && strings.ToLower(task.Project) != project {
			return false
		}
		if area != "" && strings.ToLower(task.Area) != area {
			return false
		}
		if query != "" && !strings.Contains(strings.ToLower(searchText(task)), query) {
			return false
		}
		return true
	})
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
			date := week[i].Date
			if task.StartDate == date || task.Deadline == date || task.CompletedAt == date || task.CanceledAt == date {
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

func KnownTags(tasks []Task) []string {
	set := make(map[string]struct{})
	for _, task := range tasks {
		for _, tag := range task.Tags {
			set[tag] = struct{}{}
		}
	}
	return sortedKeys(set)
}

func KnownProjects(tasks []Task) []string {
	set := make(map[string]struct{})
	for _, task := range tasks {
		if task.Project != "" {
			set[task.Project] = struct{}{}
		}
	}
	return sortedKeys(set)
}

func KnownAreas(tasks []Task) []string {
	set := make(map[string]struct{})
	for _, task := range tasks {
		if task.Area != "" {
			set[task.Area] = struct{}{}
		}
	}
	return sortedKeys(set)
}

func filterTasks(tasks []Task, keep func(Task) bool) []Task {
	out := make([]Task, 0)
	for _, task := range tasks {
		if keep(task) {
			out = append(out, task)
		}
	}
	return out
}

func hasTag(task Task, tag string) bool {
	for _, value := range task.Tags {
		if strings.ToLower(value) == tag {
			return true
		}
	}
	return false
}

func searchText(task Task) string {
	return strings.Join([]string{
		task.Title,
		task.Notes,
		task.Project,
		task.Area,
		JoinTags(task.Tags),
		task.StartDate,
		task.Deadline,
	}, " ")
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
