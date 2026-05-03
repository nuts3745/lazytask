# Repository Guidelines

## Project Structure & Module Organization

LazyTask is a small Go 1.22 package named `lazytask`. Source files live at the repository root:

- `task.go` defines task models, statuses, validation, and constructors.
- `store.go` contains the in-memory task store and ordering behavior.
- `runner.go` executes stored task commands and records results.
- `task_test.go` contains unit tests for task creation and store behavior.
- `README.md` provides package usage examples and near-term roadmap notes.

Keep new package code in root-level `.go` files unless a feature grows large enough to justify a subpackage. Add tests beside the code with `_test.go` suffixes.

## Build, Test, and Development Commands

- `go test ./...` runs all tests in the module.
- `go test -race ./...` runs tests with the race detector; use this for changes touching `Store` concurrency.
- `go test -cover ./...` reports package test coverage.
- `go build ./...` verifies the package compiles.
- `gofmt -w *.go` formats root Go files before committing.

There is no separate application binary or asset build step yet.

## Coding Style & Naming Conventions

Follow standard Go style and let `gofmt` handle indentation and spacing. Use exported names only for package API types and functions intended for consumers, such as `Task`, `Store`, `Runner`, and `NewTask`. Keep error messages lowercase and specific, for example `task id is required`. Prefer small, focused methods that preserve the current package API.

## Testing Guidelines

Use Go’s built-in `testing` package. Name tests with `Test<Behavior>` and make assertions directly with `t.Fatalf` or `t.Errorf`. Add tests for validation errors, task ordering, status transitions, command execution outcomes, and concurrency-sensitive store behavior. Run `go test ./...` before opening a pull request; use `go test -race ./...` when shared state is involved.

## Commit & Pull Request Guidelines

The current Git history is minimal, so use clear imperative commit subjects such as `Add runner failure test` or `Validate empty task command`. Keep commits scoped to one logical change.

Pull requests should include a short description, the reason for the change, and the test commands run. Link related issues when available. Include terminal output or screenshots only when behavior is user-visible, such as future terminal UI work.

## Agent-Specific Instructions

Do not introduce dependencies, subpackages, generated files, or broad refactors unless the task requires them. Preserve the library-first shape of the repository and update `README.md` when public usage changes.
