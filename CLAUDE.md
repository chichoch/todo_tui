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

`install.sh` also seeds `~/.config/todo-tui/todo-tui.conf` from the in-repo template if absent. **`uninstall.sh` deliberately leaves the user's config dir alone** — only the binary is removed. If you add new install-time files, decide explicitly whether `uninstall.sh` should touch them; user-edited config should never be wiped.

## Architecture

Single Go package `main` split by concern (not by layer). The shared `state` struct in `ui.go` is the spine — every handler is a method on `*state` and mutates it in place. There is no model/controller separation; the tview widgets and the domain data are both fields of `state`.

- `todo_tui.go` — `main()` + CLI parsing. `parseCLIArgs` handles `[file]` and `-n`/`--no-history`; `applyCLIOverrides` zeroes out the remote-cmd fields when a file argument is given. Wires the tview widget tree (table, input, status bar, help/quit/saving/conflict overlays via `tview.Pages`), constructs `state`, and registers `handleGlobalInput` / `handleListInput` / `handleInputDone`.
- `model.go` — `Item` + `loadItems` / `saveItems` + `appendHistory`. The on-disk format is a Markdown checklist (`- [ ] text` / `- [x] text`) with hidden id markers (`<!--id:HEX-->`) appended to each line by `saveItems`. Items without an id (legacy files / new entries) get a fresh one on load via `newItemID()`. `saveItems` always prepends `# TODO\n` and only writes lines for items in memory — non-checklist content in an existing file is **not preserved** on save. **Known limitation:** if item text contains a literal `<!--id:HEX-->` suffix, `checklistPattern` will parse it as the id and strip it from the text on round-trip.
- `config.go` — parses `~/.config/todo-tui/todo-tui.conf` (key=value, `#` comments). Keys: `$FILE` (basename without `.md`, validated via `validateFileName`), `file-path`, `file-cmd-save`, `file-cmd-load`, `history-file`. **Invariant:** `file-cmd-save` and `file-cmd-load` must both be set or both empty — `loadConfigFrom` returns an error otherwise. `runFileCmd` exports two env vars to the shell command (`TODO_PATH` = temp dir, `TODO_FILE` = config name without extension) and runs the template via `sh -c`. Older configs used `$PATH`/`$FILE` substitution — that was removed; users must update to `$TODO_PATH`/`$TODO_FILE`.
- `sync.go` — three-way merge (`merge(base, local, remote)`) keyed by item id. Produces auto-merged items + a list of `conflict`s that need user resolution. `applyResolutions` folds the user's choices back in; the "keep both" branch assigns a fresh id to the remote copy so the saved file has unique ids.
- `input.go` — input handling + item operations (add, edit, delete, toggle, jump, save, sync). Three entry points:
  - `handleGlobalInput` — app-level keys (`A`/`a` to enter add mode, Ctrl-C, help dismiss, conflict-resolution routing when `s.resolvingConflict` is set).
  - `handleListInput` — table-focused keys (`q`, `c`, `d`, `s`/`w` save-or-sync, `?`/`h` help, digits + Enter/Space for jump-to-index, Enter/Space toggle).
  - `handleInputDone` — Enter/Escape on the input field.
- `ui.go` — `state` struct, `refreshList` (table rendering), `updateChrome` (status bar), `isDirty` helper, and the page-overlay show/hide helpers (help, unsaved-quit dialog, saving spinner, conflict dialog). `state.mu` is a `sync.Mutex` that guards `items` and `dirty` across the sync/save background goroutines — handlers and `refreshList` lock briefly when reading/writing those fields, and the goroutines lock around their writes.

### Save flow

`save()` has two branches:

1. **Local** (no `file-cmd-save`): synchronous. Snapshots `s.items` under `s.mu`, writes via `saveItems`, then clears `s.dirty` under the lock.
2. **Remote** (`file-cmd-save` set): writes items to a temp dir, runs the shell command via `runFileCmd`, removes the temp dir. Runs **in a goroutine** because the command can block; uses `s.app.QueueUpdateDraw` to flip back to the UI thread for status updates and `close(s.saveDone)` so tests can synchronize. The "Saving..." overlay is shown for the duration.

### Sync flow

`s`/`w` routes to `sync()` instead of `save()` when `file-cmd-save` is set. `sync()`:

1. Snapshots local items under `s.mu`, shows the saving overlay, spawns a goroutine.
2. Runs `file-cmd-load` into a temp dir, parses remote items via `loadItems`.
3. Reads the merge-base from `~/.cache/todo-tui/<name>.base.md` (if present).
4. Calls `merge(base, local, remote)`. For each conflict, queues a `showConflict` overlay on the UI thread and blocks on `<-s.syncResume`. `handleConflictInput` (`l`/`r`/`b`/Esc) sends the resolution back. Esc aborts the whole sync.
5. `applyResolutions` produces the final item list.
6. Writes the final list to a push temp dir, runs `file-cmd-save`, then mirrors to the local file and updates the base cache.
7. Closes `s.syncDone` (for tests) and queues a final UI update with the status.

All `s.items` / `s.dirty` writes inside the goroutine are guarded by `s.mu`.

### Load flow

If `file-cmd-load` is set, `main` fetches into a temp dir before opening the local file. A failed load command is non-fatal — the app starts empty with a status message. On the first successful load, `cachePath(cfg)` is seeded as the merge base. Any other load error exits.

### History flow

When `history-file` is set in the config (or via env), `deleteSelected` appends a line `YYYY-MM-DD HH:MM <item text>` to that file via `appendHistory` after the item is removed. `-n` / `--no-history` clears `cfg.HistoryFile` for the session.

### CLI args

`parseCLIArgs` accepts at most one positional file path plus `-n`/`--no-history`. Extra positional args return an error. A positional file forces a plain-local edit on that path: `applyCLIOverrides` zeroes `FilePath`, `FileName`, `FileCmdSave`, `FileCmdLoad` so the remote-sync flow is bypassed even if the config sets it. History is preserved unless `-n` is also passed.

### Quit guard

`q` and Ctrl-C check `s.dirty`. If dirty, they show the quit overlay and route subsequent keys through `handleQuitInput` (`y` saves and stops, `n` stops without saving, Esc cancels). `s.stopped` is set before `app.Stop()` so tests can assert termination without actually running the event loop.

## Testing conventions

`newTestState()` in `todo_tui_test.go` builds a `*state` with three seed items and wires up just enough tview to keep handlers happy — no `app.Run()`. Tests call handler methods directly and assert on `state` fields. Use the `runeEvent(r)` / `keyEvent(k)` helpers to construct synthetic `tcell.EventKey` values. The async save and sync paths are tested via `<-s.saveDone` / `<-s.syncDone` with a timeout. `QueueUpdateDraw` is non-blocking (buffered channel) but its closures don't fire without an event loop — goroutines therefore mutate `s.items` / `s.dirty` directly under `s.mu` before signaling completion, so tests see the result the moment the channel closes. Always run `go test -race ./...` because the mutex is the only thing standing between the sync goroutine and the UI thread.
