package main

import (
	"path/filepath"
	"testing"
)

func TestLogPathInConfigDir(t *testing.T) {
	got := logPathInConfigDir(filepath.Join("home", "config"))
	want := filepath.Join("home", "config", "lazytask", "lazytask.jsonl")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
