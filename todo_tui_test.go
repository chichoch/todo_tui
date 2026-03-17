package main

import (
	"os"
	"path/filepath"
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
file-cmd-save = rclone copy $PATH gdrive:docs/$FILE
file-cmd-load = rclone copy gdrive:docs/$FILE $PATH
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
	if cfg.FileCmdSave != "rclone copy $PATH gdrive:docs/$FILE" {
		t.Errorf("FileCmdSave = %q", cfg.FileCmdSave)
	}
	if cfg.FileCmdLoad != "rclone copy gdrive:docs/$FILE $PATH" {
		t.Errorf("FileCmdLoad = %q", cfg.FileCmdLoad)
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
	// cp copies files from $PATH dir to dest; $FILE is the raw config name.
	s.cfg.FileCmdSave = "cp $PATH/Test.md " + destDir + "/"

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
