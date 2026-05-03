# Repository Guidelines

## Project Structure & Module Organization

LazyTask is a Go 1.24 terminal task manager. Library source files live at the repository root, and the CLI entrypoint lives under `cmd/lazytask`:

- `task.go` defines the Things-style task model and input validation.
- `event.go` reads and appends JSONL events.
- `store.go` replays events into the current task projection.
- `views.go` contains Today and Monday-Friday week view logic.
- `tui.go` contains the Bubble Tea model and key handling.

Keep reusable package code in root-level `.go` files. Put executable commands under `cmd/`. Add tests beside the code with `_test.go` suffixes.

## Build, Test, and Development Commands

- `go test ./...` runs all tests in the module.
- `go test -race ./...` runs tests with the race detector; use this for changes touching `Store` concurrency.
- `go test -cover ./...` reports package test coverage.
- `go build ./...` verifies the package compiles.
- `go run ./cmd/lazytask` starts the TUI using `./lazytask.jsonl`.
- `gofmt -w *.go` formats root Go files before committing.

There is no asset build step.

## Coding Style & Naming Conventions

Follow standard Go style and let `gofmt` handle indentation and spacing. Use exported names only for package API types and functions intended for consumers, such as `Task`, `Store`, `EventLog`, and `RunTUI`. Keep error messages lowercase and specific, for example `task title is required`. Prefer small, focused methods that preserve the event-sourced data flow.

## Testing Guidelines

Use Go’s built-in `testing` package. Name tests with `Test<Behavior>` and make assertions directly with `t.Fatalf` or `t.Errorf`. Add tests for event replay, malformed JSONL logs, date projections, TUI state transitions, and concurrency-sensitive store behavior. Run `go test ./...` before opening a pull request; use `go test -race ./...` when shared state is involved.

## Commit & Pull Request Guidelines

The current Git history is minimal, so use clear imperative commit subjects such as `Add runner failure test` or `Validate empty task command`. Keep commits scoped to one logical change.

Pull requests should include a short description, the reason for the change, and the test commands run. Link related issues when available. Include terminal output or screenshots only when behavior is user-visible, such as future terminal UI work.

## Agent-Specific Instructions

Do not introduce generated files or broad refactors unless the task requires them. Preserve JSONL event compatibility where possible, and update `README.md` when public usage or key bindings change.
