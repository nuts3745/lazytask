# LazyTask

LazyTask is a Lazygit-like terminal task manager inspired by Things 3. It stores state as a lightweight JSONL event log and rebuilds the current task list by replaying events.

## Run

```sh
go run ./cmd/lazytask
```

By default, LazyTask writes events to `./lazytask.jsonl`. Pass a path to use another log file:

```sh
go run ./cmd/lazytask ./demo.jsonl
```

## Views

- Today shows tasks scheduled for today plus tasks completed today.
- Week shows Monday through Friday and includes tasks scheduled or completed on each day.

## Keys

- `tab`: switch Today / Week
- `j` / `k`: move selection
- `a`: add task
- `e`: edit selected task
- `space`: complete or reopen selected task
- `d`: delete selected task
- `q`: quit

## Storage

Each line in the log is a JSON event such as `task_created`, `task_updated`, `task_completed`, `task_uncompleted`, or `task_deleted`. Deleted tasks are hidden by projection rather than removed from the log.
