package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func newTestState() *state {
	app := tview.NewApplication()
	table := tview.NewTable().SetSelectable(true, false)
	input := tview.NewInputField().SetLabel("Add: ")
	pages := tview.NewPages()
	quitDialog := tview.NewFlex()

	mainLayout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(input, 1, 0, false)

	savingLabel := tview.NewTextView()
	savingOverlay := tview.NewFlex().AddItem(savingLabel, 0, 1, false)

	pages.AddPage("main", mainLayout, true, true)
	pages.AddPage("quit", quitDialog, true, false)
	pages.AddPage("saving", savingOverlay, true, false)
	app.SetRoot(pages, true)

	s := &state{
		app:               app,
		pages:             pages,
		table:             table,
		input:             input,
		quitDialog:        quitDialog,
		savingLabel:       savingLabel,
		items:             []Item{{text: "first"}, {text: "second"}, {text: "third"}},
		filePath:          "",
		mode:              inputModeAdd,
		editIndex:         -1,
		lastListSelection: 0,
	}
	s.refreshList()
	app.SetFocus(table)
	return s
}

func runeEvent(r rune) *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone)
}

func keyEvent(key tcell.Key) *tcell.EventKey {
	return tcell.NewEventKey(key, 0, tcell.ModNone)
}

func TestHandleGlobalInput_AFromList(t *testing.T) {
	s := newTestState()
	// Focus table (default) — 'A' should be consumed and start add mode
	s.app.SetFocus(s.table)

	result := s.handleGlobalInput(runeEvent('A'))
	if result != nil {
		t.Error("expected 'A' to be consumed (return nil) when table is focused")
	}
	if !s.input.HasFocus() {
		t.Error("expected focus to move to input field after pressing A")
	}
}

func TestHandleGlobalInput_AFromInput(t *testing.T) {
	s := newTestState()
	// Focus input — 'A' should pass through so the character can be typed
	s.app.SetFocus(s.input)

	event := runeEvent('A')
	result := s.handleGlobalInput(event)
	if result != event {
		t.Error("expected 'A' to pass through (not be consumed) when input is focused")
	}
}

func TestHandleGlobalInput_OtherRunePassesThrough(t *testing.T) {
	s := newTestState()
	s.app.SetFocus(s.table)

	event := runeEvent('z')
	result := s.handleGlobalInput(event)
	if result != event {
		t.Error("expected unhandled rune to pass through")
	}
}

func TestHandleListInput_ToggleEnter(t *testing.T) {
	s := newTestState()
	s.table.Select(0, 0)

	result := s.handleListInput(keyEvent(tcell.KeyEnter))
	if result != nil {
		t.Error("expected Enter to be consumed")
	}
	if !s.items[0].checked {
		t.Error("expected first item to be toggled to checked")
	}
}

func TestHandleListInput_Delete(t *testing.T) {
	s := newTestState()
	s.table.Select(1, 0)
	originalLen := len(s.items)

	result := s.handleListInput(runeEvent('d'))
	if result != nil {
		t.Error("expected 'd' to be consumed")
	}
	if len(s.items) != originalLen-1 {
		t.Errorf("expected %d items after delete, got %d", originalLen-1, len(s.items))
	}
}

func TestHandleListInput_EditMode(t *testing.T) {
	s := newTestState()
	s.table.Select(0, 0)

	result := s.handleListInput(runeEvent('c'))
	if result != nil {
		t.Error("expected 'c' to be consumed")
	}
	if s.mode != inputModeEdit {
		t.Error("expected mode to be inputModeEdit after pressing 'c'")
	}
	if s.editIndex != 0 {
		t.Errorf("expected editIndex 0, got %d", s.editIndex)
	}
}

func TestHandleListInput_JumpDigits(t *testing.T) {
	s := newTestState()

	s.handleListInput(runeEvent('2'))
	if s.jumpBuffer != "2" {
		t.Errorf("expected jumpBuffer '2', got %q", s.jumpBuffer)
	}

	s.handleListInput(keyEvent(tcell.KeyEnter))
	if s.jumpBuffer != "" {
		t.Error("expected jumpBuffer cleared after Enter")
	}
	if s.lastListSelection != 1 {
		t.Errorf("expected selection at index 1, got %d", s.lastListSelection)
	}
}

func TestHandleInputDone_EnterAddsItem(t *testing.T) {
	s := newTestState()
	s.app.SetFocus(s.input)
	s.input.SetText("new item")
	originalLen := len(s.items)

	s.handleInputDone(tcell.KeyEnter)

	if len(s.items) != originalLen+1 {
		t.Errorf("expected %d items, got %d", originalLen+1, len(s.items))
	}
	if s.items[len(s.items)-1].text != "new item" {
		t.Errorf("expected last item text 'new item', got %q", s.items[len(s.items)-1].text)
	}
}

func TestHandleInputDone_EscapeCancels(t *testing.T) {
	s := newTestState()
	s.app.SetFocus(s.input)
	s.input.SetText("should be discarded")
	originalLen := len(s.items)

	s.handleInputDone(tcell.KeyEscape)

	if len(s.items) != originalLen {
		t.Errorf("expected %d items after cancel, got %d", originalLen, len(s.items))
	}
	if s.input.GetText() != "" {
		t.Error("expected input text to be cleared after escape")
	}
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "todo-tui.conf")
	os.WriteFile(confPath, []byte(`# comment
$FILE = MyTodo
file-path = ~/Documents/
file-cmd-save = rclone copy "$TODO_PATH/$TODO_FILE.md" gdrive:docs/
file-cmd-load = rclone copy "gdrive:docs/$TODO_FILE.md" "$TODO_PATH/"
history-file = ~/Documents/todo-tui-history.txt
`), 0o644)

	cfg, err := loadConfigFrom(confPath)
	if err != nil {
		t.Fatalf("loadConfigFrom failed: %v", err)
	}

	if cfg.FileName != "MyTodo" {
		t.Errorf("FileName = %q, want %q", cfg.FileName, "MyTodo")
	}
	home, _ := os.UserHomeDir()
	wantPath := filepath.Join(home, "Documents")
	if cfg.FilePath != wantPath {
		t.Errorf("FilePath = %q, want %q", cfg.FilePath, wantPath)
	}
	wantResolved := filepath.Join(home, "Documents", "MyTodo.md")
	if got := resolveFilePath(cfg); got != wantResolved {
		t.Errorf("resolveFilePath = %q, want %q", got, wantResolved)
	}
	if cfg.FileCmdSave != `rclone copy "$TODO_PATH/$TODO_FILE.md" gdrive:docs/` {
		t.Errorf("FileCmdSave = %q", cfg.FileCmdSave)
	}
	if cfg.FileCmdLoad != `rclone copy "gdrive:docs/$TODO_FILE.md" "$TODO_PATH/"` {
		t.Errorf("FileCmdLoad = %q", cfg.FileCmdLoad)
	}
	wantHistory := filepath.Join(home, "Documents", "todo-tui-history.txt")
	if cfg.HistoryFile != wantHistory {
		t.Errorf("HistoryFile = %q, want %q", cfg.HistoryFile, wantHistory)
	}
}

func TestLoadConfigOnlySaveErrors(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "todo-tui.conf")
	os.WriteFile(confPath, []byte("file-cmd-save = cp $PATH /tmp/backup\n"), 0o644)

	_, err := loadConfigFrom(confPath)
	if err == nil {
		t.Fatal("expected error when only file-cmd-save is set")
	}
}

func TestLoadConfigOnlyLoadErrors(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "todo-tui.conf")
	os.WriteFile(confPath, []byte("file-cmd-load = cp /tmp/backup $PATH\n"), 0o644)

	_, err := loadConfigFrom(confPath)
	if err == nil {
		t.Fatal("expected error when only file-cmd-load is set")
	}
}

func TestLoadConfigMissing(t *testing.T) {
	cfg, err := loadConfigFrom("/nonexistent/path/todo-tui.conf")
	if err != nil {
		t.Fatalf("expected no error for missing config, got %v", err)
	}
	if cfg.FilePath != "" || cfg.FileCmdSave != "" || cfg.FileCmdLoad != "" {
		t.Errorf("expected zero-value config, got %+v", cfg)
	}
}

func TestSaveWithFileCmdSave(t *testing.T) {
	dir := t.TempDir()
	destDir := filepath.Join(dir, "dest")
	os.MkdirAll(destDir, 0o755)

	s := newTestState()
	s.cfg.FileName = "Test"
	s.cfg.FileCmdSave = `cp "$TODO_PATH/$TODO_FILE.md" ` + destDir + "/"

	s.save()

	// save() is async when FileCmdSave is set; wait for it to finish.
	select {
	case <-s.saveDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for async save to complete")
	}

	if s.dirty {
		t.Error("expected dirty to be false after save")
	}

	// No local file should be written.
	localFile := filepath.Join(dir, "Test.md")
	if _, err := os.Stat(localFile); err == nil {
		t.Error("expected no local file to be written when file-cmd-save is set")
	}

	// The command should have copied it to dest.
	items, err := loadItems(filepath.Join(destDir, "Test.md"))
	if err != nil {
		t.Fatalf("loadItems from dest failed: %v", err)
	}
	if len(items) != len(s.items) {
		t.Errorf("expected %d items at dest, got %d", len(s.items), len(items))
	}
}

func TestHandleListInput_QuitClean(t *testing.T) {
	s := newTestState()
	s.dirty = false

	result := s.handleListInput(runeEvent('q'))
	if result != nil {
		t.Error("expected 'q' to be consumed")
	}
	if !s.stopped {
		t.Error("expected app to be stopped when not dirty")
	}
}

func TestHandleListInput_QuitDirtyShowsDialog(t *testing.T) {
	s := newTestState()
	s.dirty = true

	result := s.handleListInput(runeEvent('q'))
	if result != nil {
		t.Error("expected 'q' to be consumed")
	}
	if !s.confirmQuit {
		t.Error("expected confirmQuit to be true when dirty")
	}
	if s.stopped {
		t.Error("expected app NOT to be stopped yet")
	}
}

func TestHandleQuitInput_Yes(t *testing.T) {
	s := newTestState()
	dir := t.TempDir()
	s.filePath = filepath.Join(dir, "test.md")
	s.dirty = true
	s.confirmQuit = true

	s.handleQuitInput(runeEvent('y'))
	if !s.stopped {
		t.Error("expected app to be stopped after 'y'")
	}
	if s.dirty {
		t.Error("expected dirty to be false after save")
	}
}

func TestHandleQuitInput_No(t *testing.T) {
	s := newTestState()
	s.dirty = true
	s.confirmQuit = true

	s.handleQuitInput(runeEvent('n'))
	if !s.stopped {
		t.Error("expected app to be stopped after 'n'")
	}
}

func TestHandleQuitInput_Escape(t *testing.T) {
	s := newTestState()
	s.dirty = true
	s.confirmQuit = true

	s.handleQuitInput(keyEvent(tcell.KeyEscape))
	if s.confirmQuit {
		t.Error("expected confirmQuit to be false after Escape")
	}
	if s.stopped {
		t.Error("expected app NOT to be stopped after Escape")
	}
}

func TestHandleGlobalInput_CtrlCDirtyShowsDialog(t *testing.T) {
	s := newTestState()
	s.dirty = true

	result := s.handleGlobalInput(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl))
	if result != nil {
		t.Error("expected Ctrl-C to be consumed")
	}
	if !s.confirmQuit {
		t.Error("expected confirmQuit to be true")
	}
	if s.stopped {
		t.Error("expected app NOT to be stopped yet")
	}
}

func TestHandleGlobalInput_CtrlCCleanStops(t *testing.T) {
	s := newTestState()
	s.dirty = false

	result := s.handleGlobalInput(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl))
	if result != nil {
		t.Error("expected Ctrl-C to be consumed")
	}
	if !s.stopped {
		t.Error("expected app to be stopped when not dirty")
	}
}

func TestLoadAndSaveItems(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte("# TODO\n- [x] done\n- [ ] pending\n"), 0o644)

	items, err := loadItems(path)
	if err != nil {
		t.Fatalf("loadItems failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if !items[0].checked || items[0].text != "done" {
		t.Errorf("unexpected first item: %+v", items[0])
	}
	if items[1].checked || items[1].text != "pending" {
		t.Errorf("unexpected second item: %+v", items[1])
	}

	items = append(items, Item{text: "new"})
	if err := saveItems(path, items); err != nil {
		t.Fatalf("saveItems failed: %v", err)
	}

	reloaded, err := loadItems(path)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if len(reloaded) != 3 {
		t.Fatalf("expected 3 items after save, got %d", len(reloaded))
	}
}

func TestLoadItems_AssignsMissingID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte("# TODO\n- [ ] no id here\n"), 0o644)

	items, err := loadItems(path)
	if err != nil {
		t.Fatalf("loadItems failed: %v", err)
	}
	if len(items) != 1 || items[0].id == "" {
		t.Fatalf("expected legacy item to receive an id, got %+v", items)
	}
}

func TestSaveItems_RoundTripsID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	original := []Item{{id: "abc1234567890def", text: "one", checked: false}, {id: "feedfacecafebabe", text: "two", checked: true}}
	if err := saveItems(path, original); err != nil {
		t.Fatalf("saveItems failed: %v", err)
	}
	reloaded, err := loadItems(path)
	if err != nil {
		t.Fatalf("loadItems failed: %v", err)
	}
	if len(reloaded) != 2 {
		t.Fatalf("expected 2, got %d", len(reloaded))
	}
	for i, it := range reloaded {
		if it.id != original[i].id || it.text != original[i].text || it.checked != original[i].checked {
			t.Errorf("round-trip mismatch at %d: got %+v want %+v", i, it, original[i])
		}
	}
}

func TestMerge_LocalOnlyEdit(t *testing.T) {
	base := []Item{{id: "x", text: "old", checked: false}}
	local := []Item{{id: "x", text: "new", checked: false}}
	remote := []Item{{id: "x", text: "old", checked: false}}
	auto, conflicts := merge(base, local, remote)
	if len(conflicts) != 0 {
		t.Fatalf("unexpected conflicts: %+v", conflicts)
	}
	if len(auto) != 1 || auto[0].text != "new" {
		t.Fatalf("expected local edit to win, got %+v", auto)
	}
}

func TestMerge_RemoteOnlyEdit(t *testing.T) {
	base := []Item{{id: "x", text: "old"}}
	local := []Item{{id: "x", text: "old"}}
	remote := []Item{{id: "x", text: "new"}}
	auto, conflicts := merge(base, local, remote)
	if len(conflicts) != 0 {
		t.Fatalf("unexpected conflicts: %+v", conflicts)
	}
	if len(auto) != 1 || auto[0].text != "new" {
		t.Fatalf("expected remote edit to win, got %+v", auto)
	}
}

func TestMerge_BothEditIdentical(t *testing.T) {
	base := []Item{{id: "x", text: "old"}}
	local := []Item{{id: "x", text: "same-new"}}
	remote := []Item{{id: "x", text: "same-new"}}
	auto, conflicts := merge(base, local, remote)
	if len(conflicts) != 0 {
		t.Fatalf("identical edits should not conflict, got %+v", conflicts)
	}
	if len(auto) != 1 || auto[0].text != "same-new" {
		t.Fatalf("got %+v", auto)
	}
}

func TestMerge_BothEditConflict(t *testing.T) {
	base := []Item{{id: "x", text: "old"}}
	local := []Item{{id: "x", text: "local-new"}}
	remote := []Item{{id: "x", text: "remote-new"}}
	auto, conflicts := merge(base, local, remote)
	if len(auto) != 0 {
		t.Fatalf("expected no auto items, got %+v", auto)
	}
	if len(conflicts) != 1 || conflicts[0].id != "x" {
		t.Fatalf("expected one conflict for x, got %+v", conflicts)
	}
	if conflicts[0].local == nil || conflicts[0].local.text != "local-new" {
		t.Errorf("local missing/wrong: %+v", conflicts[0].local)
	}
	if conflicts[0].remote == nil || conflicts[0].remote.text != "remote-new" {
		t.Errorf("remote missing/wrong: %+v", conflicts[0].remote)
	}
}

func TestMerge_EditVsDeleteConflict(t *testing.T) {
	base := []Item{{id: "x", text: "old"}}
	local := []Item{{id: "x", text: "edited"}}
	remote := []Item{} // x deleted on remote
	auto, conflicts := merge(base, local, remote)
	if len(auto) != 0 {
		t.Fatalf("expected no auto items, got %+v", auto)
	}
	if len(conflicts) != 1 || conflicts[0].remote != nil {
		t.Fatalf("expected edit-vs-delete with remote=nil, got %+v", conflicts)
	}
}

func TestMerge_IndependentAdds(t *testing.T) {
	base := []Item{}
	local := []Item{{id: "a", text: "alpha"}}
	remote := []Item{{id: "b", text: "beta"}}
	auto, conflicts := merge(base, local, remote)
	if len(conflicts) != 0 {
		t.Fatalf("unexpected conflicts: %+v", conflicts)
	}
	if len(auto) != 2 {
		t.Fatalf("expected 2 items, got %+v", auto)
	}
	if auto[0].text != "alpha" || auto[1].text != "beta" {
		t.Errorf("local-first ordering broken: %+v", auto)
	}
}

func TestMerge_BothDeleted(t *testing.T) {
	base := []Item{{id: "x", text: "gone"}}
	local := []Item{}
	remote := []Item{}
	auto, conflicts := merge(base, local, remote)
	if len(auto) != 0 || len(conflicts) != 0 {
		t.Fatalf("both-delete should drop, got auto=%+v conflicts=%+v", auto, conflicts)
	}
}

func TestApplyResolutions_KeepBothAssignsNewID(t *testing.T) {
	conflicts := []conflict{{id: "x", local: itemPtr(Item{id: "x", text: "L"}), remote: itemPtr(Item{id: "x", text: "R"})}}
	resolutions := []resolution{{id: "x", kind: resolutionBoth}}
	out := applyResolutions(nil, conflicts, resolutions)
	if len(out) != 2 {
		t.Fatalf("expected 2 items, got %+v", out)
	}
	if out[0].id == out[1].id {
		t.Errorf("keep-both must produce distinct IDs, got %s == %s", out[0].id, out[1].id)
	}
}

// --- sync end-to-end tests ---

func setupSyncFixture(t *testing.T) (s *state, fixtureDir, localDir, homeDir string) {
	t.Helper()
	homeDir = t.TempDir()
	t.Setenv("HOME", homeDir)

	fixtureDir = t.TempDir()
	localDir = t.TempDir()

	s = newTestState()
	s.items = nil
	s.cfg.FileName = "SyncTest"
	s.cfg.FileCmdLoad = "cp " + fixtureDir + `/SyncTest.md "$TODO_PATH/"`
	s.cfg.FileCmdSave = `cp "$TODO_PATH/$TODO_FILE.md" ` + fixtureDir + "/"
	s.filePath = filepath.Join(localDir, "SyncTest.md")
	return s, fixtureDir, localDir, homeDir
}

func waitSync(t *testing.T, s *state) {
	t.Helper()
	select {
	case <-s.syncDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for sync to complete")
	}
}

func TestSync_FirstRun_NoBase(t *testing.T) {
	s, fixtureDir, localDir, _ := setupSyncFixture(t)

	// remote has one item
	if err := saveItems(filepath.Join(fixtureDir, "SyncTest.md"), []Item{{id: "remote1", text: "from-remote"}}); err != nil {
		t.Fatal(err)
	}
	// local in-memory has one item
	s.items = []Item{{id: "local1", text: "from-local"}}

	s.sync()
	waitSync(t, s)

	if s.dirty {
		t.Errorf("expected clean state after successful sync")
	}
	if len(s.items) != 2 {
		t.Fatalf("expected merged 2 items, got %+v", s.items)
	}

	// local file written with merged
	saved, err := loadItems(filepath.Join(localDir, "SyncTest.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 2 {
		t.Errorf("local file should have 2 items, got %d", len(saved))
	}

	// remote file written with merged (push round-trip)
	pushed, err := loadItems(filepath.Join(fixtureDir, "SyncTest.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(pushed) != 2 {
		t.Errorf("remote file should have 2 items, got %d", len(pushed))
	}

	// base file written
	baseFile := cachePath(s.cfg)
	base, err := loadItems(baseFile)
	if err != nil {
		t.Fatalf("base file missing or unreadable: %v", err)
	}
	if len(base) != 2 {
		t.Errorf("base file should have 2 items, got %d", len(base))
	}
}

func TestSync_PushSuccessUpdatesBase(t *testing.T) {
	s, fixtureDir, _, _ := setupSyncFixture(t)

	// Pre-seed base with one item, matching local (no local edits)
	if err := ensureCacheDir(s.cfg); err != nil {
		t.Fatal(err)
	}
	seed := []Item{{id: "x", text: "shared"}}
	if err := saveItems(cachePath(s.cfg), seed); err != nil {
		t.Fatal(err)
	}
	s.items = []Item{{id: "x", text: "shared"}}

	// Remote added a new item
	if err := saveItems(filepath.Join(fixtureDir, "SyncTest.md"), []Item{{id: "x", text: "shared"}, {id: "y", text: "added-on-remote"}}); err != nil {
		t.Fatal(err)
	}

	s.sync()
	waitSync(t, s)

	if s.dirty {
		t.Errorf("expected clean state after sync")
	}

	base, err := loadItems(cachePath(s.cfg))
	if err != nil {
		t.Fatal(err)
	}
	if len(base) != 2 {
		t.Errorf("base should have 2 items after sync, got %d", len(base))
	}
}

func TestSync_PushFailureLeavesBaseUnchanged(t *testing.T) {
	s, fixtureDir, localDir, _ := setupSyncFixture(t)

	if err := ensureCacheDir(s.cfg); err != nil {
		t.Fatal(err)
	}
	seed := []Item{{id: "x", text: "before"}}
	if err := saveItems(cachePath(s.cfg), seed); err != nil {
		t.Fatal(err)
	}
	if err := saveItems(s.filePath, seed); err != nil {
		t.Fatal(err)
	}
	if err := saveItems(filepath.Join(fixtureDir, "SyncTest.md"), seed); err != nil {
		t.Fatal(err)
	}
	s.items = []Item{{id: "x", text: "before"}, {id: "y", text: "local-add"}}

	// Force the push to fail.
	s.cfg.FileCmdSave = "false"

	s.sync()
	waitSync(t, s)

	if !s.dirty {
		t.Errorf("expected dirty state after failed push")
	}

	// Base file must still match the seed.
	base, err := loadItems(cachePath(s.cfg))
	if err != nil {
		t.Fatal(err)
	}
	if len(base) != 1 || base[0].text != "before" {
		t.Errorf("base file should be unchanged after failed push, got %+v", base)
	}

	// Local file must also be unchanged.
	local, err := loadItems(s.filePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(local) != 1 || local[0].text != "before" {
		t.Errorf("local file should be unchanged after failed push, got %+v", local)
	}

	_ = localDir
}

func TestDeleteWritesHistory(t *testing.T) {
	s := newTestState()
	historyPath := filepath.Join(t.TempDir(), "sub", "history.log")
	s.cfg.HistoryFile = historyPath
	s.table.Select(1, 0) // "second"

	s.deleteSelected()

	if len(s.items) != 2 {
		t.Fatalf("expected 2 items after delete, got %d", len(s.items))
	}
	data, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	line := strings.TrimRight(string(data), "\n")
	re := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2} second$`)
	if !re.MatchString(line) {
		t.Errorf("history line %q does not match expected format", line)
	}
}

func TestDeleteWithoutHistoryFile(t *testing.T) {
	s := newTestState()
	s.table.Select(0, 0)
	s.deleteSelected()
	if len(s.items) != 2 {
		t.Errorf("expected 2 items after delete, got %d", len(s.items))
	}
}

// --- CLI argument parsing ---

func TestParseCLI_Empty(t *testing.T) {
	file, noHistory, err := parseCLIArgs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if file != "" || noHistory {
		t.Errorf("expected zero values, got file=%q noHistory=%v", file, noHistory)
	}
}

func TestParseCLI_PositionalOnly(t *testing.T) {
	file, noHistory, err := parseCLIArgs([]string{"notes.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if file != "notes.md" {
		t.Errorf("file = %q, want %q", file, "notes.md")
	}
	if noHistory {
		t.Errorf("noHistory = true, want false")
	}
}

func TestParseCLI_NoHistoryShort(t *testing.T) {
	file, noHistory, err := parseCLIArgs([]string{"-n"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !noHistory {
		t.Errorf("noHistory = false, want true")
	}
	if file != "" {
		t.Errorf("file = %q, want empty", file)
	}
}

func TestParseCLI_NoHistoryLong(t *testing.T) {
	file, noHistory, err := parseCLIArgs([]string{"--no-history"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !noHistory {
		t.Errorf("noHistory = false, want true")
	}
	if file != "" {
		t.Errorf("file = %q, want empty", file)
	}
}

func TestParseCLI_FlagThenPositional(t *testing.T) {
	file, noHistory, err := parseCLIArgs([]string{"-n", "notes.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !noHistory {
		t.Errorf("noHistory = false, want true")
	}
	if file != "notes.md" {
		t.Errorf("file = %q, want %q", file, "notes.md")
	}
}

// --- CLI override application ---

func TestApplyOverrides_NoArgs_PreservesConfig(t *testing.T) {
	cfg := config{
		FilePath:    "/etc/todo",
		FileCmdSave: "cp $PATH /tmp/",
		FileCmdLoad: "cp /tmp/ $PATH",
		HistoryFile: "/var/log/todo-history",
	}
	got, path := applyCLIOverrides(cfg, "", false)
	if got != cfg {
		t.Errorf("cfg modified unexpectedly: got %+v, want %+v", got, cfg)
	}
	if path != "" {
		t.Errorf("path = %q, want empty (caller should fall back)", path)
	}
}

func TestApplyOverrides_FileArgClearsRemoteAndPath(t *testing.T) {
	cfg := config{
		FileName:    "Foo",
		FilePath:    "/etc/todo",
		FileCmdSave: "cp $PATH /tmp/",
		FileCmdLoad: "cp /tmp/ $PATH",
	}
	got, path := applyCLIOverrides(cfg, "/tmp/foo.md", false)
	if got.FilePath != "" || got.FileName != "" || got.FileCmdSave != "" || got.FileCmdLoad != "" {
		t.Errorf("expected remote/path fields cleared, got %+v", got)
	}
	if path != "/tmp/foo.md" {
		t.Errorf("path = %q, want %q", path, "/tmp/foo.md")
	}
}

func TestApplyOverrides_FileArgPreservesHistory(t *testing.T) {
	cfg := config{
		FileCmdSave: "cp $PATH /tmp/",
		FileCmdLoad: "cp /tmp/ $PATH",
		HistoryFile: "/var/log/todo-history",
	}
	got, _ := applyCLIOverrides(cfg, "/tmp/foo.md", false)
	if got.HistoryFile != "/var/log/todo-history" {
		t.Errorf("HistoryFile = %q, want preserved", got.HistoryFile)
	}
}

func TestApplyOverrides_NoHistoryClearsHistoryFile(t *testing.T) {
	cfg := config{HistoryFile: "/var/log/todo-history"}
	got, _ := applyCLIOverrides(cfg, "/tmp/foo.md", true)
	if got.HistoryFile != "" {
		t.Errorf("HistoryFile = %q, want empty", got.HistoryFile)
	}
}

func TestApplyOverrides_NoHistoryWithoutFileArg(t *testing.T) {
	cfg := config{
		FileCmdSave: "cp $PATH /tmp/",
		FileCmdLoad: "cp /tmp/ $PATH",
		HistoryFile: "/var/log/todo-history",
	}
	got, path := applyCLIOverrides(cfg, "", true)
	if got.HistoryFile != "" {
		t.Errorf("HistoryFile = %q, want empty", got.HistoryFile)
	}
	if got.FileCmdSave == "" || got.FileCmdLoad == "" {
		t.Errorf("remote cmds should be preserved when no positional, got %+v", got)
	}
	if path != "" {
		t.Errorf("path = %q, want empty", path)
	}
}

func TestApplyOverrides_ExpandsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	_, path := applyCLIOverrides(config{}, "~/foo.md", false)
	want := filepath.Join(home, "foo.md")
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
}

// --- behavioral integration ---

func TestSave_AfterOverride_WritesLocalNotRemote(t *testing.T) {
	dir := t.TempDir()
	destDir := filepath.Join(dir, "dest")
	os.MkdirAll(destDir, 0o755)

	s := newTestState()
	// Pre-override: remote cmd is set (would normally route through async branch).
	s.cfg.FileName = "Test"
	s.cfg.FileCmdSave = `cp "$TODO_PATH/$TODO_FILE.md" ` + destDir + "/"

	argFile := filepath.Join(dir, "override.md")
	newCfg, newPath := applyCLIOverrides(s.cfg, argFile, false)
	s.cfg = newCfg
	s.filePath = newPath

	s.save()

	// Local branch is synchronous — saveDone should be untouched (nil).
	if s.saveDone != nil {
		t.Errorf("expected synchronous local save; saveDone was set")
	}
	if s.dirty {
		t.Error("expected dirty=false after save")
	}
	if _, err := os.Stat(filepath.Join(destDir, "Test.md")); err == nil {
		t.Error("remote dest file should NOT have been written after override")
	}
	items, err := loadItems(argFile)
	if err != nil {
		t.Fatalf("loadItems from override path failed: %v", err)
	}
	if len(items) != len(s.items) {
		t.Errorf("override file has %d items, want %d", len(items), len(s.items))
	}
}

func TestDelete_AfterNoHistory_SkipsHistoryFile(t *testing.T) {
	s := newTestState()
	historyPath := filepath.Join(t.TempDir(), "history.log")
	s.cfg.HistoryFile = historyPath

	newCfg, _ := applyCLIOverrides(s.cfg, "", true)
	s.cfg = newCfg

	s.table.Select(0, 0)
	s.deleteSelected()

	if _, err := os.Stat(historyPath); !os.IsNotExist(err) {
		t.Errorf("history file should not exist after delete with --no-history, err=%v", err)
	}
}

func TestDeleteAppendsMultipleHistoryLines(t *testing.T) {
	s := newTestState()
	historyPath := filepath.Join(t.TempDir(), "history.log")
	s.cfg.HistoryFile = historyPath

	s.table.Select(0, 0)
	s.deleteSelected()
	s.table.Select(0, 0)
	s.deleteSelected()

	data, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 history lines, got %d: %q", len(lines), string(data))
	}
	if !strings.HasSuffix(lines[0], " first") {
		t.Errorf("line 0 = %q, want suffix %q", lines[0], " first")
	}
	if !strings.HasSuffix(lines[1], " second") {
		t.Errorf("line 1 = %q, want suffix %q", lines[1], " second")
	}
}

// --- review-follow-up coverage ---

func TestSync_KeepBothResolution(t *testing.T) {
	s, fixtureDir, _, _ := setupSyncFixture(t)

	if err := ensureCacheDir(s.cfg); err != nil {
		t.Fatal(err)
	}
	base := []Item{{id: "x", text: "shared"}}
	if err := saveItems(cachePath(s.cfg), base); err != nil {
		t.Fatal(err)
	}
	if err := saveItems(filepath.Join(fixtureDir, "SyncTest.md"), []Item{{id: "x", text: "remote-edit"}}); err != nil {
		t.Fatal(err)
	}
	s.items = []Item{{id: "x", text: "local-edit"}}

	s.sync()
	s.syncResume <- resolution{id: "x", kind: resolutionBoth}
	waitSync(t, s)

	if len(s.items) != 2 {
		t.Fatalf("keep-both should yield 2 items, got %+v", s.items)
	}
	texts := map[string]bool{s.items[0].text: true, s.items[1].text: true}
	if !texts["local-edit"] || !texts["remote-edit"] {
		t.Errorf("expected both local-edit and remote-edit, got %+v", s.items)
	}
	if s.items[0].id == s.items[1].id {
		t.Errorf("keep-both must yield distinct ids, both = %q", s.items[0].id)
	}
}

func TestSync_AbortDuringConflict(t *testing.T) {
	s, fixtureDir, _, _ := setupSyncFixture(t)

	if err := ensureCacheDir(s.cfg); err != nil {
		t.Fatal(err)
	}
	base := []Item{{id: "x", text: "shared"}}
	if err := saveItems(cachePath(s.cfg), base); err != nil {
		t.Fatal(err)
	}
	remoteFile := filepath.Join(fixtureDir, "SyncTest.md")
	if err := saveItems(remoteFile, []Item{{id: "x", text: "remote-edit"}}); err != nil {
		t.Fatal(err)
	}
	remoteStat, _ := os.Stat(remoteFile)
	s.items = []Item{{id: "x", text: "local-edit"}}

	s.sync()
	s.syncResume <- resolution{id: "x", kind: resolutionAbort}
	waitSync(t, s)

	// Local in-memory items unchanged.
	if len(s.items) != 1 || s.items[0].text != "local-edit" {
		t.Errorf("abort should leave local items untouched, got %+v", s.items)
	}
	// Remote file unchanged.
	postStat, _ := os.Stat(remoteFile)
	if postStat.ModTime() != remoteStat.ModTime() {
		t.Error("remote file should not be touched after abort")
	}
	// Base unchanged.
	gotBase, _ := loadItems(cachePath(s.cfg))
	if len(gotBase) != 1 || gotBase[0].text != "shared" {
		t.Errorf("base should be untouched after abort, got %+v", gotBase)
	}
}

func TestSync_MalformedRemote(t *testing.T) {
	s, fixtureDir, _, _ := setupSyncFixture(t)

	if err := os.WriteFile(filepath.Join(fixtureDir, "SyncTest.md"), []byte("not a checklist\njust prose\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.items = []Item{{id: "local", text: "kept"}}

	s.sync()
	waitSync(t, s)

	if len(s.items) != 1 || s.items[0].text != "kept" {
		t.Errorf("malformed remote should leave local items intact, got %+v", s.items)
	}
	if s.dirty {
		t.Error("expected clean state after successful sync over malformed remote")
	}
}

func TestAppendHistory_PermissionDenied(t *testing.T) {
	s := newTestState()
	// /dev/null is not a directory, so MkdirAll on a subpath fails fast.
	s.cfg.HistoryFile = "/dev/null/nope/history.log"
	s.table.Select(1, 0)

	s.deleteSelected()

	if len(s.items) != 2 {
		t.Errorf("item should still be removed despite history failure, got %d items", len(s.items))
	}
	if !strings.Contains(s.status, "history write failed") {
		t.Errorf("expected status to surface history failure, got %q", s.status)
	}
}

func TestRunFileCmd_EnvVars(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out")
	// Use single quotes around output filename to avoid shell expansion in the path itself.
	cmd := "printf '%s|%s' \"$TODO_PATH\" \"$TODO_FILE\" > " + outFile
	if err := runFileCmd(cmd, "/some/dir", "MyTodo"); err != nil {
		t.Fatalf("runFileCmd failed: %v", err)
	}
	got, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	want := "/some/dir|MyTodo"
	if string(got) != want {
		t.Errorf("env passthrough = %q, want %q", string(got), want)
	}
}

func TestLoadConfig_RejectsBadFile(t *testing.T) {
	cases := []string{
		"../../etc/passwd",
		"foo/bar",
		"bad;name",
		"a&b",
		"x`whoami`",
	}
	for _, name := range cases {
		dir := t.TempDir()
		confPath := filepath.Join(dir, "todo-tui.conf")
		os.WriteFile(confPath, []byte("$FILE = "+name+"\n"), 0o644)
		if _, err := loadConfigFrom(confPath); err == nil {
			t.Errorf("expected error for $FILE = %q, got nil", name)
		}
	}
}

func TestParseCLI_RejectsExtraArgs(t *testing.T) {
	_, _, err := parseCLIArgs([]string{"a.md", "b.md"})
	if err == nil {
		t.Fatal("expected error when multiple positional args are passed")
	}
	if !strings.Contains(err.Error(), "unexpected") {
		t.Errorf("error = %q, want contains 'unexpected'", err.Error())
	}
}
