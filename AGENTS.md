# TODO TUI

A terminal-based TODO list manager built with Go, using [tview](https://github.com/rivo/tview) and [tcell](https://github.com/gdamore/tcell).

## Project Structure

- `todo_tui.go` — Application entry point (`main`) and CLI arg parsing.
- `model.go` — Data model (`Item`) and file I/O (`loadItems`, `saveItems`, `appendHistory`). Items carry hidden id markers in the saved file for sync stability.
- `config.go` — Config-file parser. Reads `~/.config/todo-tui/todo-tui.conf` and runs `file-cmd-save` / `file-cmd-load` via `sh -c` with `$TODO_PATH` and `$TODO_FILE` exported as env vars.
- `sync.go` — Three-way merge keyed by item id.
- `input.go` — Input handling and item operations (add, edit, delete, toggle, jump, save, sync, conflict resolution).
- `ui.go` — State struct, list rendering (`refreshList`), status bar (`updateChrome`), overlays. State mutations that cross goroutine boundaries are protected by `state.mu`.
- `todo_tui_test.go` — Tests for input handlers, file I/O, merge, sync, and CLI.
- `TODO-tui.md` — The checklist data file (Markdown checkboxes).

## Build & Run

```sh
go build -o todo_tui .
./todo_tui
```

## Install & Uninstall

```sh
./install.sh    # builds and installs to ~/.todo-tui/bin/ with a symlink in ~/.local/bin/
./uninstall.sh  # removes the binary; leaves the user config dir alone on purpose
```

`uninstall.sh` does NOT delete `~/.config/todo-tui/` — user-edited config is preserved across reinstall/uninstall cycles. If you add new install-time files, decide explicitly whether uninstall should touch them.

## Testing

```sh
go test ./...
go test -race ./...   # required when touching sync/save paths
```

## Conventions

- State is held in the `state` struct; handler methods are on `*state`.
- Input handling is split into `handleGlobalInput` (app-level keys), `handleListInput` (table-focused keys), and `handleInputDone` (input field submission).
- Tests construct a `state` via `newTestState()` and call handler methods directly — no full app startup needed.
- Use `tcell.NewEventKey` helpers (`runeEvent`, `keyEvent`) to create synthetic key events in tests.
