# TODO TUI

A terminal-based TODO list manager built with Go, using [tview](https://github.com/rivo/tview) and [tcell](https://github.com/gdamore/tcell).

## Project Structure

- `todo_tui.go` — Application entry point (`main`).
- `model.go` — Data model (`Item`) and file I/O (`loadItems`, `saveItems`).
- `input.go` — Input handling and item operations (add, edit, delete, toggle, jump, save).
- `ui.go` — State struct, list rendering (`refreshList`), and status bar (`updateChrome`).
- `todo_tui_test.go` — Tests for input handlers and file I/O.
- `TODO-tui.md` — The checklist data file (Markdown checkboxes).

## Build & Run

```sh
go build -o todo_tui .
./todo_tui
```

## Install & Uninstall

```sh
./install.sh    # builds and installs to ~/.todo-tui/bin/ with a symlink in ~/.local/bin/
./uninstall.sh  # removes all files and directories created by install.sh
```

When modifying `install.sh` to create new files or directories, always update `uninstall.sh` to remove them so that uninstall fully reverses an install.

## Testing

```sh
go test ./...
```

## Conventions

- State is held in the `state` struct; handler methods are on `*state`.
- Input handling is split into `handleGlobalInput` (app-level keys), `handleListInput` (table-focused keys), and `handleInputDone` (input field submission).
- Tests construct a `state` via `newTestState()` and call handler methods directly — no full app startup needed.
- Use `tcell.NewEventKey` helpers (`runeEvent`, `keyEvent`) to create synthetic key events in tests.
