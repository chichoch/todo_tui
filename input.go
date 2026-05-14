package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

func (s *state) handleGlobalInput(event *tcell.EventKey) *tcell.EventKey {
	if s.resolvingConflict {
		return s.handleConflictInput(event)
	}
	if s.confirmQuit {
		return s.handleQuitInput(event)
	}
	// While an async save/sync is in flight, swallow input so the user can't
	// edit items between the goroutine's snapshot and its post-save dirty
	// flag flip (which would otherwise silently mark those edits clean).
	if s.savingActive {
		return nil
	}
	if event.Key() == tcell.KeyCtrlC {
		if s.isDirty() {
			s.showQuitDialog()
			return nil
		}
		s.stopped = true
		s.app.Stop()
		return nil
	}
	if s.helpVisible {
		s.toggleHelp()
		return nil
	}
	if s.input.HasFocus() {
		return event
	}
	if event.Key() == tcell.KeyRune && (event.Rune() == 'A' || event.Rune() == 'a') {
		s.startAddMode()
		return nil
	}
	return event
}

func (s *state) handleListInput(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyRune:
		r := event.Rune()
		if r >= '0' && r <= '9' {
			s.appendJumpDigit(r)
			return nil
		}

		switch r {
		case ' ':
			if s.jumpBuffer != "" {
				s.commitJump()
				return nil
			}
			s.toggleSelected()
			return nil
		case 'q':
			if s.isDirty() {
				s.showQuitDialog()
				return nil
			}
			s.stopped = true
			s.app.Stop()
			return nil
		case '?', 'h':
			s.toggleHelp()
			return nil
		case 'c':
			s.startEditMode()
			return nil
		case 'd':
			s.deleteSelected()
			return nil
		case 's', 'w':
			if s.cfg.FileCmdSave != "" {
				s.sync()
			} else {
				s.save()
			}
			return nil
		}
	case tcell.KeyEnter:
		if s.jumpBuffer != "" {
			s.commitJump()
			return nil
		}
		s.toggleSelected()
		return nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if s.backspaceJump() {
			return nil
		}
	case tcell.KeyEscape:
		if s.clearJumpBuffer() {
			s.updateChrome("Canceled jump")
			return nil
		}
	}

	return event
}

func (s *state) handleInputDone(key tcell.Key) {
	switch key {
	case tcell.KeyEnter:
		s.commitInput()
	case tcell.KeyEscape:
		s.cancelInput()
	}
}

func (s *state) startAddMode() {
	s.mode = inputModeAdd
	s.editIndex = -1
	s.input.SetLabel(" Add: ")
	s.input.SetText("")
	s.clearJumpBuffer()
	s.app.SetFocus(s.input)
	s.updateChrome("Add mode")
}

func (s *state) startEditMode() {
	index := s.selectedIndex()
	if index < 0 {
		s.updateChrome("Nothing selected to edit")
		return
	}

	s.mode = inputModeEdit
	s.editIndex = index
	s.input.SetLabel(fmt.Sprintf("Edit #%d: ", index+1))
	s.mu.Lock()
	text := ""
	if index < len(s.items) {
		text = s.items[index].text
	}
	s.mu.Unlock()
	s.input.SetText(text)
	s.clearJumpBuffer()
	s.app.SetFocus(s.input)
	s.updateChrome(fmt.Sprintf("Editing item %d", index+1))
}

func (s *state) commitInput() {
	text := strings.TrimSpace(s.input.GetText())
	if text == "" {
		if s.mode == inputModeAdd {
			s.input.SetText("")
			s.updateChrome("Ignored empty input")
			return
		}
		s.cancelInput()
		s.updateChrome("Ignored empty input")
		return
	}

	status := ""

	s.mu.Lock()
	switch s.mode {
	case inputModeEdit:
		if s.editIndex >= 0 && s.editIndex < len(s.items) {
			s.items[s.editIndex].text = text
			s.lastListSelection = s.editIndex
			s.dirty = true
			status = fmt.Sprintf("Updated item %d", s.editIndex+1)
		}
	default:
		s.items = append(s.items, Item{text: text})
		s.lastListSelection = len(s.items) - 1
		s.dirty = true
		status = fmt.Sprintf("Added item %d", len(s.items))
	}
	s.mu.Unlock()

	s.refreshList()
	if status == "" {
		status = "Input applied"
	}

	if s.mode == inputModeAdd {
		s.input.SetText("")
		s.updateChrome(status)
	} else {
		s.mode = inputModeAdd
		s.editIndex = -1
		s.input.SetLabel(" Add: ")
		s.input.SetText("")
		s.app.SetFocus(s.table)
		s.updateChrome(status)
	}
}

func (s *state) cancelInput() {
	s.mode = inputModeAdd
	s.editIndex = -1
	s.input.SetLabel(" (A)dd: ")
	s.input.SetText("")
	s.app.SetFocus(s.table)
	s.updateChrome("Input canceled")
}

func (s *state) selectedIndex() int {
	s.mu.Lock()
	n := len(s.items)
	s.mu.Unlock()
	if n == 0 {
		return -1
	}

	row, _ := s.table.GetSelection()
	if row < 0 || row >= n {
		return -1
	}

	s.lastListSelection = row
	return row
}

func (s *state) toggleSelected() {
	index := s.selectedIndex()
	if index < 0 {
		s.updateChrome("Nothing selected to toggle")
		return
	}

	s.mu.Lock()
	s.items[index].checked = !s.items[index].checked
	checked := s.items[index].checked
	s.dirty = true
	s.mu.Unlock()

	s.refreshList()
	if checked {
		s.updateChrome(fmt.Sprintf("Checked item %d", index+1))
	} else {
		s.updateChrome(fmt.Sprintf("Unchecked item %d", index+1))
	}
}

func (s *state) deleteSelected() {
	index := s.selectedIndex()
	if index < 0 {
		s.updateChrome("Nothing selected to delete")
		return
	}

	s.mu.Lock()
	deletedText := s.items[index].text
	s.items = append(s.items[:index], s.items[index+1:]...)
	s.dirty = true
	s.mu.Unlock()
	s.lastListSelection = index
	s.refreshList()

	if s.cfg.HistoryFile != "" {
		if err := appendHistory(s.cfg.HistoryFile, deletedText); err != nil {
			s.updateChrome(fmt.Sprintf("Deleted (history write failed: %v)", err))
			return
		}
	}
	s.updateChrome("Deleted selected item")
}

func (s *state) sync() {
	s.showSaving("Syncing...")

	s.mu.Lock()
	itemsCopy := make([]Item, len(s.items))
	copy(itemsCopy, s.items)
	s.mu.Unlock()

	syncDone := make(chan struct{})
	s.syncDone = syncDone
	s.syncResume = make(chan resolution, 1)

	cfg := s.cfg
	filePath := s.filePath

	go func() {
		// Close syncDone BEFORE any QueueUpdateDraw, since QueueUpdateDraw
		// blocks until the app event loop drains it (and tests don't run one).
		finish := func(status string, refreshUI bool) {
			close(syncDone)
			s.app.QueueUpdateDraw(func() {
				if refreshUI {
					s.refreshList()
				}
				s.hideConflict()
				s.hideSaving()
				s.updateChrome(status)
			})
		}

		pullDir, err := os.MkdirTemp("", "todo_tui_sync_pull_")
		if err != nil {
			finish(fmt.Sprintf("Sync failed: %v", err), false)
			return
		}
		defer os.RemoveAll(pullDir)

		if err := runFileCmd(cfg.FileCmdLoad, pullDir, configFileName(cfg)); err != nil {
			finish(fmt.Sprintf("Sync pull failed: %v", err), false)
			return
		}

		remote, _, err := loadItems(filepath.Join(pullDir, resolveFileName(cfg)))
		if err != nil {
			finish(fmt.Sprintf("Sync pull parse failed: %v", err), false)
			return
		}

		var base []Item
		basePath := cachePath(cfg)
		if basePath != "" {
			if _, statErr := os.Stat(basePath); statErr == nil {
				if b, _, err := loadItems(basePath); err == nil {
					base = b
				}
			}
		}

		auto, conflicts := merge(base, itemsCopy, remote)

		var resolutions []resolution
		if len(conflicts) > 0 {
			conflictsLocal := conflicts
			for i := range conflictsLocal {
				idx := i
				// QueueUpdateDraw waits on the event loop, which tests don't run;
				// dispatch the UI update from a helper goroutine so the sync
				// goroutine can immediately block on syncResume.
				go s.app.QueueUpdateDraw(func() {
					s.pendingConflicts = conflictsLocal
					s.pendingIndex = idx
					s.showConflict(idx, len(conflictsLocal), conflictsLocal[idx])
				})
				res := <-s.syncResume
				if res.kind == resolutionAbort {
					finish("Sync aborted", false)
					return
				}
				resolutions = append(resolutions, res)
			}
			go s.app.QueueUpdateDraw(func() {
				s.pendingConflicts = nil
				s.pendingIndex = 0
				s.hideConflict()
				s.showSaving("Syncing...")
			})
		}

		final := applyResolutions(auto, conflicts, resolutions)

		pushDir, err := os.MkdirTemp("", "todo_tui_sync_push_")
		if err != nil {
			s.mu.Lock()
			s.items = final
			s.dirty = true
			s.mu.Unlock()
			finish(fmt.Sprintf("Sync failed: %v", err), true)
			return
		}
		defer os.RemoveAll(pushDir)

		pushFile := filepath.Join(pushDir, resolveFileName(cfg))
		if err := saveItems(pushFile, final, nil); err != nil {
			s.mu.Lock()
			s.items = final
			s.dirty = true
			s.mu.Unlock()
			finish(fmt.Sprintf("Sync write failed: %v", err), true)
			return
		}

		if err := runFileCmd(cfg.FileCmdSave, pushDir, configFileName(cfg)); err != nil {
			s.mu.Lock()
			s.items = final
			s.dirty = true
			s.mu.Unlock()
			finish(fmt.Sprintf("Sync push failed: %v", err), true)
			return
		}

		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			s.mu.Lock()
			s.items = final
			s.dirty = true
			s.mu.Unlock()
			finish(fmt.Sprintf("Sync local write failed: %v", err), true)
			return
		}
		if err := saveItems(filePath, final, s.fileCtx); err != nil {
			s.mu.Lock()
			s.items = final
			s.dirty = true
			s.mu.Unlock()
			finish(fmt.Sprintf("Sync local write failed: %v", err), true)
			return
		}

		if basePath != "" {
			if err := ensureCacheDir(cfg); err == nil {
				_ = saveItems(basePath, final, nil)
			}
		}

		s.mu.Lock()
		s.items = final
		s.dirty = false
		s.mu.Unlock()
		finish(fmt.Sprintf("Synced (%d conflicts)", len(conflicts)), true)
	}()
}

func (s *state) save() {
	if s.cfg.FileCmdSave != "" {
		s.showSaving("Saving...")
		s.mu.Lock()
		itemsCopy := make([]Item, len(s.items))
		copy(itemsCopy, s.items)
		s.mu.Unlock()
		done := make(chan struct{})
		s.saveDone = done

		go func() {
			status := ""
			ok := true
			var tmpFile string

			tmpDir, err := os.MkdirTemp("", "todo_tui_save_")
			if err != nil {
				status = fmt.Sprintf("Save failed: %v", err)
				ok = false
			}

			if ok {
				tmpFile = filepath.Join(tmpDir, resolveFileName(s.cfg))
				if err := saveItems(tmpFile, itemsCopy, nil); err != nil {
					status = fmt.Sprintf("Save failed: %v", err)
					ok = false
				}
			}

			if ok {
				// Expose temp path so a quit-save timeout can surface it for
				// manual recovery before the goroutine cleans up.
				s.mu.Lock()
				s.saveTempPath = tmpFile
				s.mu.Unlock()

				if err := runFileCmd(s.cfg.FileCmdSave, tmpDir, configFileName(s.cfg)); err != nil {
					status = fmt.Sprintf("Save cmd failed: %v", err)
					ok = false
				}
			}

			if ok {
				status = "Saved via command"
				s.mu.Lock()
				s.dirty = false
				s.saveTempPath = ""
				s.mu.Unlock()
			}

			if tmpDir != "" {
				os.RemoveAll(tmpDir)
			}

			close(done)
			s.app.QueueUpdateDraw(func() {
				s.hideSaving()
				s.updateChrome(status)
			})
		}()
		return
	}

	s.mu.Lock()
	itemsCopy := make([]Item, len(s.items))
	copy(itemsCopy, s.items)
	s.mu.Unlock()
	if err := saveItems(s.filePath, itemsCopy, s.fileCtx); err != nil {
		s.updateChrome(fmt.Sprintf("Save failed: %v", err))
		return
	}
	s.mu.Lock()
	s.dirty = false
	s.mu.Unlock()
	s.updateChrome("Saved " + s.filePath)
}

func (s *state) appendJumpDigit(digit rune) {
	if len(s.jumpBuffer) >= 8 {
		return
	}

	s.jumpBuffer += string(digit)
	s.updateChrome(fmt.Sprintf("Jump: %s", s.jumpBuffer))
}

func (s *state) backspaceJump() bool {
	if s.jumpBuffer == "" {
		return false
	}

	s.jumpBuffer = s.jumpBuffer[:len(s.jumpBuffer)-1]
	if s.jumpBuffer == "" {
		s.updateChrome("Jump cleared")
	} else {
		s.updateChrome(fmt.Sprintf("Jump: %s", s.jumpBuffer))
	}
	return true
}

func (s *state) clearJumpBuffer() bool {
	if s.jumpBuffer == "" {
		return false
	}

	s.jumpBuffer = ""
	return true
}

func (s *state) commitJump() {
	raw := s.jumpBuffer
	s.clearJumpBuffer()

	s.mu.Lock()
	n := len(s.items)
	s.mu.Unlock()
	index, err := strconv.Atoi(raw)
	if err != nil || index < 1 || index > n {
		s.updateChrome(fmt.Sprintf("Invalid item number: %s", raw))
		return
	}

	s.lastListSelection = index - 1
	s.table.Select(index-1, 0)
	s.updateChrome(fmt.Sprintf("Selected item %d", index))
}
