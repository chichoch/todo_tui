package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type inputMode int

const (
	inputModeAdd inputMode = iota
	inputModeEdit
)

type state struct {
	app        *tview.Application
	pages      *tview.Pages
	table      *tview.Table
	input      *tview.InputField
	statusBar  *tview.TextView
	helpBox    *tview.TextView
	items      []Item
	fileCtx    *fileContext
	filePath   string
	cfg        config
	dirty      bool
	mode       inputMode
	editIndex  int
	jumpBuffer string
	status     string

	// mu guards items and dirty across the sync/save background goroutines.
	mu sync.Mutex

	lastListSelection int
	helpVisible       bool
	confirmQuit       bool
	quitDialog        *tview.Flex
	stopped           bool

	savingLabel  *tview.TextView
	saveDone     chan struct{}
	savingActive bool
	// saveTempPath holds the in-flight async-save temp file path while a remote
	// save command is running; cleared on success. Guarded by mu. Used by the
	// quit-save timeout path to point the user at unsaved content.
	saveTempPath string
	// quitSaveTimedOut is set when handleQuitInput's wait on saveDone exceeds
	// the timeout; main() inspects it after app.Run returns to print a recovery
	// hint to stderr.
	quitSaveTimedOut bool
	quitSaveTempPath string

	conflictOverlay   *tview.Flex
	conflictLabel     *tview.TextView
	resolvingConflict bool
	pendingConflicts  []conflict
	pendingIndex      int
	syncResume        chan resolution
	syncDone          chan struct{}
}

func (s *state) isDirty() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dirty
}

func (s *state) refreshList() {
	s.mu.Lock()
	items := make([]Item, len(s.items))
	copy(items, s.items)
	s.mu.Unlock()

	s.table.Clear()

	if len(items) == 0 {
		s.table.SetCell(0, 0, tview.NewTableCell("No TODO items yet. Press A to add one.").SetSelectable(false))
		return
	}

	for i, item := range items {
		color := tcell.ColorWhite
		if item.checked {
			color = tcell.ColorGreen
		}

		indexCell := tview.NewTableCell(fmt.Sprintf("%d", i+1)).
			SetTextColor(color).
			SetAlign(tview.AlignRight)

		prefix := "- [ ] "
		if item.checked {
			prefix = "- [X] "
		}

		textCell := tview.NewTableCell(tview.Escape(fmt.Sprintf("%s%s", prefix, item.text))).
			SetTextColor(color).
			SetExpansion(1)

		s.table.SetCell(i, 0, indexCell)
		s.table.SetCell(i, 1, textCell)
	}

	if s.lastListSelection >= len(items) {
		s.lastListSelection = len(items) - 1
	}
	if s.lastListSelection < 0 {
		s.lastListSelection = 0
	}
	s.table.Select(s.lastListSelection, 0)
}

func (s *state) toggleHelp() {
	if s.helpVisible {
		s.pages.HidePage("help")
		s.helpVisible = false
		s.app.SetFocus(s.table)
	} else {
		s.pages.ShowPage("help")
		s.helpVisible = true
		s.app.SetFocus(s.helpBox)
	}
}

func (s *state) showQuitDialog() {
	s.confirmQuit = true
	s.pages.ShowPage("quit")
	s.app.SetFocus(s.quitDialog)
}

func (s *state) hideQuitDialog() {
	s.confirmQuit = false
	s.pages.HidePage("quit")
	s.app.SetFocus(s.table)
}

func (s *state) handleQuitInput(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyRune {
		switch event.Rune() {
		case 'y', 'Y':
			s.confirmQuit = false
			s.save()
			// If save() spawned the async remote-save goroutine, wait up to 10s
			// for completion so the process doesn't exit mid-write. On timeout
			// stash the temp path; main() prints a recovery hint after the
			// event loop unwinds.
			done := s.saveDone
			if done != nil {
				select {
				case <-done:
				case <-time.After(10 * time.Second):
					s.mu.Lock()
					s.quitSaveTimedOut = true
					s.quitSaveTempPath = s.saveTempPath
					s.mu.Unlock()
				}
			}
			s.stopped = true
			s.app.Stop()
			return nil
		case 'n', 'N':
			s.confirmQuit = false
			s.stopped = true
			s.app.Stop()
			return nil
		}
	}
	if event.Key() == tcell.KeyEscape {
		s.hideQuitDialog()
		return nil
	}
	return nil
}

func (s *state) showSaving(msg string) {
	s.savingActive = true
	s.savingLabel.SetText(msg)
	s.pages.ShowPage("saving")
}

func (s *state) hideSaving() {
	s.savingActive = false
	s.pages.HidePage("saving")
	s.app.SetFocus(s.table)
}

func (s *state) showConflict(index, total int, c conflict) {
	s.resolvingConflict = true
	local := "(deleted)"
	if c.local != nil {
		mark := " "
		if c.local.checked {
			mark = "x"
		}
		local = fmt.Sprintf("- [%s] %s", mark, c.local.text)
	}
	remote := "(deleted)"
	if c.remote != nil {
		mark := " "
		if c.remote.checked {
			mark = "x"
		}
		remote = fmt.Sprintf("- [%s] %s", mark, c.remote.text)
	}
	s.conflictLabel.SetText(fmt.Sprintf(
		"Conflict %d of %d\n\nLocal : %s\nRemote: %s\n\n(l) keep local   (r) keep remote\n(b) keep both    (Esc) abort sync",
		index+1, total, local, remote,
	))
	s.pages.HidePage("saving")
	s.pages.ShowPage("conflict")
	s.app.SetFocus(s.conflictOverlay)
}

func (s *state) hideConflict() {
	s.resolvingConflict = false
	s.pages.HidePage("conflict")
	s.app.SetFocus(s.table)
}

func (s *state) handleConflictInput(event *tcell.EventKey) *tcell.EventKey {
	var kind resolutionKind
	switch event.Key() {
	case tcell.KeyEscape:
		kind = resolutionAbort
	case tcell.KeyRune:
		switch event.Rune() {
		case 'l', 'L':
			kind = resolutionLocal
		case 'r', 'R':
			kind = resolutionRemote
		case 'b', 'B':
			kind = resolutionBoth
		default:
			return nil
		}
	default:
		return nil
	}
	if s.pendingIndex >= len(s.pendingConflicts) {
		return nil
	}
	c := s.pendingConflicts[s.pendingIndex]
	if s.syncResume != nil {
		// Non-blocking: if the goroutine hasn't read the prior send yet
		// (channel buffer=1 already full), drop this keypress instead of
		// hanging the UI thread. The user can press again once the overlay
		// re-renders for the next conflict.
		select {
		case s.syncResume <- resolution{id: c.id, kind: kind}:
		default:
		}
	}
	return nil
}

func (s *state) updateChrome(status string) {
	s.status = status

	mode := "LIST"
	if s.input.HasFocus() {
		if s.mode == inputModeEdit {
			mode = "EDIT"
		} else {
			mode = "ADD"
		}
	}

	s.mu.Lock()
	isDirty := s.dirty
	s.mu.Unlock()
	dirty := "clean"
	if isDirty {
		dirty = "unsaved"
	}

	jump := ""
	if s.jumpBuffer != "" {
		jump = fmt.Sprintf(" | jump:%s", s.jumpBuffer)
	}

	statusText := ""
	if s.status != "" {
		statusText = fmt.Sprintf(" | %s", s.status)
	}

	s.table.SetTitle(" TODO TUI ")

	if s.statusBar != nil {
		s.statusBar.SetText(fmt.Sprintf("mode:%s | %s%s%s", mode, dirty, jump, statusText))
	}
}
