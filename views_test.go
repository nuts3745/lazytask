package lazytask

import "testing"

func TestTodayTasksIncludesScheduledAndCompletedToday(t *testing.T) {
	today, _ := ParseLocalDate("2026-05-04")
	tasks := []Task{
		{ID: "scheduled", Title: "Scheduled", Start: StartDate, StartDate: "2026-05-04"},
		{ID: "deadline", Title: "Deadline", Start: StartAnytime, Deadline: "2026-05-04"},
		{ID: "completed", Title: "Completed", CompletedAt: "2026-05-04"},
		{ID: "other", Title: "Other", Start: StartDate, StartDate: "2026-05-05"},
	}

	got := TodayTasks(tasks, today)
	if len(got) != 3 {
		t.Fatalf("expected 3 tasks, got %#v", got)
	}
	if got[0].ID != "scheduled" || got[1].ID != "deadline" || got[2].ID != "completed" {
		t.Fatalf("unexpected today tasks: %#v", got)
	}
}

func TestInboxTasksIncludesOnlyUnclassifiedTasks(t *testing.T) {
	tasks := []Task{
		{ID: "inbox", Title: "Inbox", Start: StartInbox},
		{ID: "anytime", Title: "Anytime", Start: StartAnytime},
		{ID: "done", Title: "Done", Start: StartInbox, CompletedAt: "2026-05-04"},
		{ID: "deleted", Title: "Deleted", Deleted: true},
	}

	got := InboxTasks(tasks)
	if len(got) != 1 || got[0].ID != "inbox" {
		t.Fatalf("expected only inbox task, got %#v", got)
	}
}

func TestWorkWeekStartsMondayAndEndsFriday(t *testing.T) {
	sunday, _ := ParseLocalDate("2026-05-10")
	tasks := []Task{
		{ID: "mon", Title: "Monday", Start: StartDate, StartDate: "2026-05-04"},
		{ID: "due", Title: "Due", Start: StartAnytime, Deadline: "2026-05-06"},
		{ID: "done-fri", Title: "Done Friday", CompletedAt: "2026-05-08"},
		{ID: "sat", Title: "Saturday", Start: StartDate, StartDate: "2026-05-09"},
	}

	week := WorkWeek(tasks, sunday)
	if len(week) != 5 {
		t.Fatalf("expected 5 weekdays, got %d", len(week))
	}
	if week[0].Date != "2026-05-04" || week[4].Date != "2026-05-08" {
		t.Fatalf("unexpected week range: %#v", week)
	}
	if len(week[0].Tasks) != 1 || week[0].Tasks[0].ID != "mon" {
		t.Fatalf("expected monday task, got %#v", week[0].Tasks)
	}
	if len(week[2].Tasks) != 1 || week[2].Tasks[0].ID != "due" {
		t.Fatalf("expected wednesday deadline task, got %#v", week[2].Tasks)
	}
	if len(week[4].Tasks) != 1 || week[4].Tasks[0].ID != "done-fri" {
		t.Fatalf("expected friday completed task, got %#v", week[4].Tasks)
	}
}

func TestWorkWeekIncludesCompletedTasksByCompletionDate(t *testing.T) {
	monday, _ := ParseLocalDate("2026-05-04")
	tasks := []Task{
		{ID: "old-plan", Title: "Old Plan", Start: StartDate, StartDate: "2026-04-30", CompletedAt: "2026-05-06"},
		{ID: "outside-week", Title: "Outside", Start: StartDate, StartDate: "2026-04-30", CompletedAt: "2026-05-09"},
	}

	week := WorkWeek(tasks, monday)
	if len(week[2].Tasks) != 1 || week[2].Tasks[0].ID != "old-plan" {
		t.Fatalf("expected completed task on wednesday, got %#v", week[2].Tasks)
	}
}

func TestWorkWeekKeepsCompletedTasksOnPlannedDate(t *testing.T) {
	monday, _ := ParseLocalDate("2026-05-04")
	tasks := []Task{
		{ID: "planned-done", Title: "Planned Done", Start: StartDate, StartDate: "2026-05-05", CompletedAt: "2026-05-10"},
		{ID: "due-done", Title: "Due Done", Start: StartAnytime, Deadline: "2026-05-07", CompletedAt: "2026-05-10"},
	}

	week := WorkWeek(tasks, monday)
	if len(week[1].Tasks) != 1 || week[1].Tasks[0].ID != "planned-done" {
		t.Fatalf("expected completed planned task on tuesday, got %#v", week[1].Tasks)
	}
	if len(week[3].Tasks) != 1 || week[3].Tasks[0].ID != "due-done" {
		t.Fatalf("expected completed due task on thursday, got %#v", week[3].Tasks)
	}
}

func TestFilteredTasksByTagProjectArea(t *testing.T) {
	tasks := []Task{
		{ID: "tag", Title: "Tagged", Start: StartAnytime, Tags: []string{"urgent"}, Project: "Work", Area: "Office"},
		{ID: "other", Title: "Other", Start: StartAnytime, Tags: []string{"home"}, Project: "Home", Area: "Life"},
	}
	if got := FilteredTasks(tasks, Filter{Tag: "urgent"}); len(got) != 1 || got[0].ID != "tag" {
		t.Fatalf("tag filter failed: %#v", got)
	}
	if got := FilteredTasks(tasks, Filter{Project: "Work"}); len(got) != 1 || got[0].ID != "tag" {
		t.Fatalf("project filter failed: %#v", got)
	}
	if got := FilteredTasks(tasks, Filter{Area: "Life"}); len(got) != 1 || got[0].ID != "other" {
		t.Fatalf("area filter failed: %#v", got)
	}
}
