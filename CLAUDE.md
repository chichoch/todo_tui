# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

A short companion doc for human contributors lives in `AGENTS.md` — read it for the same conventions in plainer prose.

## Commands

```sh
go build -o todo-tui .          # build (module name `todo-tui`, go 1.26.1)
go test ./...                   # run all tests
go test -run TestSaveWithFileCmdSave   # run a single test
./todo-tui                      # run; reads ./TODO_tui.md unless config overrides
./install.sh [--local|--system] # default --local: ~/.todo-tui/bin + symlink in ~/.local/bin
./uninstall.sh [--local|--system] # mirror of install.sh — keep them in sync
```

`install.sh` also seeds `~/.config/todo-tui/todo-tui.conf` from the in-repo template if absent. When you change what `install.sh` creates, update `uninstall.sh` to remove it.

## Architecture

Single Go package `main` split by concern (not by layer). The shared `state` struct in `ui.go` is the spine — every handler is a method on `*state` and mutates it in place. There is no model/controller separation; the tview widgets and the domain data are both fields of `state`.

- `todo_tui.go` — `main()` only. Wires the tview widget tree (table, input, status bar, help/quit/saving overlays via `tview.Pages`), constructs `state`, and registers `handleGlobalInput` / `handleListInput` / `handleInputDone`.
- `model.go` — `Item` + `loadItems` / `saveItems`. The on-disk format is a Markdown checklist (`- [ ] text` / `- [x] text`) parsed by `checklistPattern`. `saveItems` always prepends `# TODO\n` and only writes lines for items in memory — non-checklist content in an existing file is **not preserved** on save.
- `config.go` — parses `~/.config/todo-tui/todo-tui.conf` (key=value, `#` comments). Keys: `$FILE` (basename without `.md`), `file-path`, `file-cmd-save`, `file-cmd-load`. **Invariant:** `file-cmd-save` and `file-cmd-load` must both be set or both empty — `loadConfigFrom` returns an error otherwise. `runFileCmd` substitutes `$PATH` (a temp dir) and `$FILE` (raw config name without extension) and runs the result via `sh -c`.
- `input.go` — input handling + item operations (add, edit, delete, toggle, jump, save). Three entry points:
  - `handleGlobalInput` — app-level keys (`A` to enter add mode, Ctrl-C, help dismiss).
  - `handleListInput` — table-focused keys (`q`, `c`, `d`, `s`/`w` save, `?`/`h` help, digits + Enter/Space for jump-to-index, Enter/Space toggle).
  - `handleInputDone` — Enter/Escape on the input field.
- `ui.go` — `state` struct, `refreshList` (table rendering), `updateChrome` (status bar), and the page-overlay show/hide helpers (help, unsaved-quit dialog, saving spinner).

### Save flow

`save()` has two branches:

1. **Local** (no `file-cmd-save`): synchronous `saveItems(s.filePath, ...)`.
2. **Remote** (`file-cmd-save` set): writes items to a temp dir, runs the shell command, removes the temp dir. Runs **in a goroutine** because the command can block; uses `s.app.QueueUpdateDraw` to flip back to the UI thread for status updates and `close(s.saveDone)` so tests can synchronize. The "Saving..." overlay is shown for the duration.

### Load flow

If `file-cmd-load` is set, `main` fetches into a temp dir before opening the local file. A failed load command is non-fatal — the app starts empty with a status message. Any other load error exits.

### Quit guard

`q` and Ctrl-C check `s.dirty`. If dirty, they show the quit overlay and route subsequent keys through `handleQuitInput` (`y` saves and stops, `n` stops without saving, Esc cancels). `s.stopped` is set before `app.Stop()` so tests can assert termination without actually running the event loop.

## Testing conventions

`newTestState()` in `todo_tui_test.go` builds a `*state` with three seed items and wires up just enough tview to keep handlers happy — no `app.Run()`. Tests call handler methods directly and assert on `state` fields. Use the `runeEvent(r)` / `keyEvent(k)` helpers to construct synthetic `tcell.EventKey` values. The async save path is tested via `<-s.saveDone` with a timeout.
