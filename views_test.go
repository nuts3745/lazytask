package lazytask

import "testing"

func TestTodayTasksIncludesScheduledAndCompletedToday(t *testing.T) {
	today, _ := ParseLocalDate("2026-05-04")
	tasks := []Task{
		{ID: "scheduled", Title: "Scheduled", When: "2026-05-04"},
		{ID: "completed", Title: "Completed", CompletedAt: "2026-05-04"},
		{ID: "other", Title: "Other", When: "2026-05-05"},
	}

	got := TodayTasks(tasks, today)
	if len(got) != 2 {
		t.Fatalf("expected 2 tasks, got %#v", got)
	}
	if got[0].ID != "scheduled" || got[1].ID != "completed" {
		t.Fatalf("unexpected today tasks: %#v", got)
	}
}

func TestWorkWeekStartsMondayAndEndsFriday(t *testing.T) {
	sunday, _ := ParseLocalDate("2026-05-10")
	tasks := []Task{
		{ID: "mon", Title: "Monday", When: "2026-05-04"},
		{ID: "done-fri", Title: "Done Friday", CompletedAt: "2026-05-08"},
		{ID: "sat", Title: "Saturday", When: "2026-05-09"},
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
	if len(week[4].Tasks) != 1 || week[4].Tasks[0].ID != "done-fri" {
		t.Fatalf("expected friday completed task, got %#v", week[4].Tasks)
	}
}
