# LazyTask

LazyTask is a Go package for building a Lazygit-like task runner UI.

This first package contains the core task model, an in-memory task store, and a command runner.

## Usage

```go
package main

import (
	"context"
	"log"

	"lazytask"
)

func main() {
	store := lazytask.NewStore()
	task := lazytask.NewTask("test", "Run tests", "go", "test", "./...")

	if err := store.Add(task); err != nil {
		log.Fatal(err)
	}

	runner := lazytask.NewRunner(store)
	result, err := runner.Run(context.Background(), "test")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("task %s exited with %d", result.TaskID, result.Code)
}
```

## Next Steps

- Add config loading for task definitions.
- Add a Bubble Tea based terminal UI.
- Stream command output instead of waiting for process completion.
