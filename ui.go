package main

import (
	"fmt"

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
	filePath   string
	dirty      bool
	mode       inputMode
	editIndex  int
	jumpBuffer string
	status     string

	lastListSelection int
	helpVisible       bool
	confirmQuit       bool
	quitDialog        *tview.Flex
	stopped           bool
}

func (s *state) refreshList() {
	s.table.Clear()

	if len(s.items) == 0 {
		s.table.SetCell(0, 0, tview.NewTableCell("No TODO items yet. Press A to add one.").SetSelectable(false))
		return
	}

	for i, item := range s.items {
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

	if s.lastListSelection >= len(s.items) {
		s.lastListSelection = len(s.items) - 1
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
			s.save()
			s.confirmQuit = false
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

	dirty := "clean"
	if s.dirty {
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
