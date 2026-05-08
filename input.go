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
	if event.Key() == tcell.KeyCtrlC {
		if s.dirty {
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
	if event.Key() == tcell.KeyRune && event.Rune() == 'A' {
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
			if s.dirty {
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
	s.input.SetText(s.items[index].text)
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
	if len(s.items) == 0 {
		return -1
	}

	row, _ := s.table.GetSelection()
	if row < 0 || row >= len(s.items) {
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

	s.items[index].checked = !s.items[index].checked
	s.dirty = true
	s.refreshList()
	if s.items[index].checked {
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

	s.items = append(s.items[:index], s.items[index+1:]...)
	s.lastListSelection = index
	s.dirty = true
	s.refreshList()
	s.updateChrome("Deleted selected item")
}

func (s *state) sync() {
	s.showSaving("Syncing...")

	itemsCopy := make([]Item, len(s.items))
	copy(itemsCopy, s.items)

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

		remote, err := loadItems(filepath.Join(pullDir, resolveFileName(cfg)))
		if err != nil {
			finish(fmt.Sprintf("Sync pull parse failed: %v", err), false)
			return
		}

		var base []Item
		basePath := cachePath(cfg)
		if basePath != "" {
			if _, statErr := os.Stat(basePath); statErr == nil {
				if b, err := loadItems(basePath); err == nil {
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
				s.app.QueueUpdateDraw(func() {
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
			s.app.QueueUpdateDraw(func() {
				s.pendingConflicts = nil
				s.pendingIndex = 0
				s.hideConflict()
				s.showSaving("Syncing...")
			})
		}

		final := applyResolutions(auto, conflicts, resolutions)

		pushDir, err := os.MkdirTemp("", "todo_tui_sync_push_")
		if err != nil {
			s.items = final
			s.dirty = true
			finish(fmt.Sprintf("Sync failed: %v", err), true)
			return
		}
		defer os.RemoveAll(pushDir)

		pushFile := filepath.Join(pushDir, resolveFileName(cfg))
		if err := saveItems(pushFile, final); err != nil {
			s.items = final
			s.dirty = true
			finish(fmt.Sprintf("Sync write failed: %v", err), true)
			return
		}

		if err := runFileCmd(cfg.FileCmdSave, pushDir, configFileName(cfg)); err != nil {
			s.items = final
			s.dirty = true
			finish(fmt.Sprintf("Sync push failed: %v", err), true)
			return
		}

		if err := saveItems(filePath, final); err != nil {
			s.items = final
			s.dirty = true
			finish(fmt.Sprintf("Sync local write failed: %v", err), true)
			return
		}

		if basePath != "" {
			if err := ensureCacheDir(cfg); err == nil {
				_ = saveItems(basePath, final)
			}
		}

		s.items = final
		s.dirty = false
		finish(fmt.Sprintf("Synced (%d conflicts)", len(conflicts)), true)
	}()
}

func (s *state) save() {
	if s.cfg.FileCmdSave != "" {
		s.showSaving("Saving...")
		itemsCopy := make([]Item, len(s.items))
		copy(itemsCopy, s.items)
		done := make(chan struct{})
		s.saveDone = done

		go func() {
			status := ""
			ok := true

			tmpDir, err := os.MkdirTemp("", "todo_tui_save_")
			if err != nil {
				status = fmt.Sprintf("Save failed: %v", err)
				ok = false
			}

			if ok {
				tmpFile := filepath.Join(tmpDir, resolveFileName(s.cfg))
				if err := saveItems(tmpFile, itemsCopy); err != nil {
					status = fmt.Sprintf("Save failed: %v", err)
					ok = false
				}
			}

			if ok {
				if err := runFileCmd(s.cfg.FileCmdSave, tmpDir, configFileName(s.cfg)); err != nil {
					status = fmt.Sprintf("Save cmd failed: %v", err)
					ok = false
				}
			}

			if tmpDir != "" {
				os.RemoveAll(tmpDir)
			}

			if ok {
				status = "Saved via command"
				s.dirty = false
			}

			close(done)
			s.app.QueueUpdateDraw(func() {
				s.hideSaving()
				s.updateChrome(status)
			})
		}()
		return
	}

	if err := saveItems(s.filePath, s.items); err != nil {
		s.updateChrome(fmt.Sprintf("Save failed: %v", err))
		return
	}
	s.dirty = false
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

	index, err := strconv.Atoi(raw)
	if err != nil || index < 1 || index > len(s.items) {
		s.updateChrome(fmt.Sprintf("Invalid item number: %s", raw))
		return
	}

	s.lastListSelection = index - 1
	s.table.Select(index-1, 0)
	s.updateChrome(fmt.Sprintf("Selected item %d", index))
}
