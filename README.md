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

To compact a long event log into the current task state without starting the TUI, run:

```sh
go run ./cmd/lazytask compact [path]
```

If `path` is omitted, LazyTask compacts `./lazytask.jsonl`.

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

Use `1`, `2`, and `3` to jump to Inbox, Today, and Weekly. Use `tab` / `shift+tab` to cycle panes.

- Today shows active tasks scheduled or due today, plus tasks completed today.
- Weekly shows Monday-Friday planned, due, completed, and canceled tasks.
- Inbox is for captured tasks; Anytime and Someday hold unscheduled active work.
- Logbook shows completed or canceled tasks.

## Search & Commands

- `/`: fzf-style search. Try `#urgent`, `>Work`, `/Home`, `today`, or `weekly`.
- `:`: command palette. Commands include `add`, `tag`, `untag`, `move`, `area`, `when`, `deadline`, `done`, `undone`, `cancel`, and `delete`.
- `1` / `2` / `3`: jump to Inbox, Today, or Weekly.
- `j` / `k`: move selection.
- `w`: toggle the selected active task as the global WIP.
- `t`: schedule the selected task for today.
- `space`: complete or reopen selected task.
- `d`: delete selected task.
- `q`: quit.

## Storage

Each log line is a JSON event such as `task_created`, `task_updated`, `task_completed`, `task_uncompleted`, `task_canceled`, or `task_deleted`. The current schema is intentionally not compatible with early LazyTask logs.

`lazytask compact [path]` rewrites the log to one `task_created` event per current non-deleted task. Deleted tasks are omitted, while completed and canceled tasks are retained so Logbook history remains visible. Before replacing the JSONL file, LazyTask writes the original contents to `<path>.bak`; an existing backup is overwritten.
