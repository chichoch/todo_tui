package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func newTestState() *state {
	app := tview.NewApplication()
	table := tview.NewTable().SetSelectable(true, false)
	input := tview.NewInputField().SetLabel("Add: ")

	root := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(input, 1, 0, false)
	app.SetRoot(root, true)

	s := &state{
		app:               app,
		table:             table,
		input:             input,
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
