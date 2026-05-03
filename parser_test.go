package lazytask

import "testing"

func TestParseQuickTask(t *testing.T) {
	now, _ := ParseLocalDate("2026-05-04")
	input, err := ParseQuickTask("買い物 #errand #outside @today >Home /生活 !2026-05-05", now)
	if err != nil {
		t.Fatalf("parse quick task: %v", err)
	}
	if input.Title != "買い物" {
		t.Fatalf("unexpected title: %q", input.Title)
	}
	if input.Start != StartDate || input.StartDate != "2026-05-04" {
		t.Fatalf("unexpected start: %#v", input)
	}
	if input.Deadline != "2026-05-05" || input.Project != "Home" || input.Area != "生活" {
		t.Fatalf("unexpected metadata: %#v", input)
	}
	if len(input.Tags) != 2 || input.Tags[0] != "errand" || input.Tags[1] != "outside" {
		t.Fatalf("unexpected tags: %#v", input.Tags)
	}
}

func TestParseQuickTaskSomeday(t *testing.T) {
	now, _ := ParseLocalDate("2026-05-04")
	input, err := ParseQuickTask("いつかやる #idea @someday", now)
	if err != nil {
		t.Fatalf("parse quick task: %v", err)
	}
	if input.Start != StartSomeday || input.StartDate != "" {
		t.Fatalf("unexpected someday input: %#v", input)
	}
}
