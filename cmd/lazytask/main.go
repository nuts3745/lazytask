package main

import (
	"fmt"
	"os"

	"lazytask"
)

func main() {
	path := "lazytask.jsonl"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	if err := lazytask.RunTUI(path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
