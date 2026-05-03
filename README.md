# LazyTask

LazyTask is a fast, Things-inspired terminal task manager. It favors quick capture, tag-based sorting, a clear Today list, and a Monday-Friday Weekly view. State is stored as a lightweight JSONL event log.

## Run

```sh
go run ./cmd/lazytask
```

By default, LazyTask writes events to `./lazytask.jsonl`. Pass a path to use another log file:

```sh
go run ./cmd/lazytask ./demo.jsonl
```

## Quick Add

Press `a` and enter a single-line task:

```text
買い物 #errand #outside @today >Home /生活 !2026-05-05
```

- `#tag` adds tags.
- `@today`, `@tomorrow`, `@YYYY-MM-DD`, `@anytime`, `@someday`, `@inbox` set the start state.
- `!YYYY-MM-DD` sets the deadline.
- `>Project` and `/Area` add loose project and area metadata.

## Views

Use `tab` to cycle `Inbox -> Today -> Weekly -> Anytime -> Someday -> Logbook`.

- Today shows active tasks scheduled or due today, plus tasks completed today.
- Weekly shows Monday-Friday planned, due, completed, and canceled tasks.
- Inbox is for captured tasks; Anytime and Someday hold unscheduled active work.
- Logbook shows completed or canceled tasks.

## Search & Commands

- `/`: fzf-style search. Try `#urgent`, `>Work`, `/Home`, `today`, or `weekly`.
- `:`: command palette. Commands include `add`, `tag`, `untag`, `move`, `area`, `when`, `deadline`, `done`, `undone`, `cancel`, and `delete`.
- `j` / `k`: move selection.
- `t`: schedule the selected task for today.
- `space`: complete or reopen selected task.
- `d`: delete selected task.
- `q`: quit.

## Storage

Each log line is a JSON event such as `task_created`, `task_updated`, `task_completed`, `task_uncompleted`, `task_canceled`, or `task_deleted`. The current schema is intentionally not compatible with early LazyTask logs.
