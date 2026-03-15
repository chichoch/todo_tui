package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

func (s *state) handleGlobalInput(event *tcell.EventKey) *tcell.EventKey {
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
		case 's':
			s.save()
			return nil
		}
	case tcell.KeyEnter, tcell.KeySpace:
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

	s.mode = inputModeAdd
	s.editIndex = -1
	s.input.SetLabel(" (A)dd: ")
	s.input.SetText("")
	s.app.SetFocus(s.table)
	s.refreshList()
	if status == "" {
		status = "Input applied"
	}
	s.updateChrome(status)
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

func (s *state) save() {
	if err := saveItems(s.filePath, s.items); err != nil {
		s.updateChrome(fmt.Sprintf("Save failed: %v", err))
		return
	}

	s.dirty = false
	s.updateChrome("Saved TODO_tui.md")
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
