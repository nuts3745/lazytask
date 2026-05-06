package main

import (
	"fmt"
	"os"

	"github.com/nuts3745/lazytask/internal/lazytask"
)

func main() {
	path := "lazytask.jsonl"
	if len(os.Args) > 1 {
		if os.Args[1] == "compact" {
			if len(os.Args) > 2 {
				path = os.Args[2]
			}
			if len(os.Args) > 3 {
				fmt.Fprintln(os.Stderr, "usage: lazytask compact [path]")
				os.Exit(2)
			}
			result, err := lazytask.NewEventLog(path).Compact()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Printf("compacted %d events to %d events\n", result.Before, result.After)
			return
		}
		if len(os.Args) > 2 {
			fmt.Fprintln(os.Stderr, "usage: lazytask [path]")
			os.Exit(2)
		}
		path = os.Args[1]
	}
	if err := lazytask.RunTUI(path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
