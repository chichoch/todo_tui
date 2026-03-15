package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const todoFileName = "TODO-tui.md"

func main() {
	items, err := loadItems(todoFileName)
	if err != nil {
		panic(err)
	}

	app := tview.NewApplication()
	table := tview.NewTable().
		SetSelectable(true, false)
	table.SetBorder(true).
		SetBorderColor(tcell.ColorDefault).
		SetTitleColor(tcell.ColorGreen).
		SetBackgroundColor(tcell.ColorDefault)
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorGreen).
		Foreground(tcell.ColorBlack))

	input := tview.NewInputField().SetLabel(" (A)dd: ").
		SetFieldBackgroundColor(tcell.ColorDefault).
		SetFieldTextColor(tcell.ColorGreen).
		SetLabelColor(tcell.ColorGreen)
	input.SetBackgroundColor(tcell.ColorDefault)

	statusBar := tview.NewTextView().
		SetTextAlign(tview.AlignRight).
		SetTextColor(tcell.ColorGreen).
		SetDynamicColors(false)
	statusBar.SetBackgroundColor(tcell.ColorDefault)

	helpBox := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetTextColor(tcell.ColorGreen).
		SetDynamicColors(false)
	helpBox.SetBackgroundColor(tcell.ColorDefault).
		SetBorder(true).
		SetBorderColor(tcell.ColorGreen).
		SetTitle(" Keys ").
		SetTitleColor(tcell.ColorGreen)
	helpBox.SetText("A: add\ndigits+Enter: jump\nEnter: toggle\nc: edit\nd: delete\ns: save\nEsc: cancel\n?/h: this help")

	pages := tview.NewPages()

	s := &state{
		app:               app,
		pages:             pages,
		table:             table,
		input:             input,
		statusBar:         statusBar,
		helpBox:           helpBox,
		items:             items,
		filePath:          todoFileName,
		mode:              inputModeAdd,
		editIndex:         -1,
		lastListSelection: 0,
	}

	// quitDialog is set after quitOverlay is built (below)

	table.SetInputCapture(s.handleListInput)
	input.SetDoneFunc(func(key tcell.Key) {
		s.handleInputDone(key)
	})

	// Bottom row: input + status bar inline
	bottomRow := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(input, 0, 1, false).
		AddItem(statusBar, 40, 0, false)
	bottomRow.SetBackgroundColor(tcell.ColorDefault)

	// Main layout: table + bottom row
	mainLayout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(bottomRow, 1, 0, false)
	mainLayout.SetBackgroundColor(tcell.ColorDefault)

	// Help overlay: centered popup
	helpOverlay := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(helpBox, 24, 0, true).
			AddItem(nil, 0, 1, false),
			12, 0, true).
		AddItem(nil, 0, 1, false)

	quitLabel := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetTextColor(tcell.ColorGreen).
		SetDynamicColors(false)
	quitLabel.SetBackgroundColor(tcell.ColorDefault).
		SetBorder(true).
		SetBorderColor(tcell.ColorGreen).
		SetTitle(" Unsaved Changes ").
		SetTitleColor(tcell.ColorGreen)
	quitLabel.SetText("Save before quitting?\n\n(y)es  (n)o  (Esc) cancel")

	quitOverlay := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(quitLabel, 32, 0, true).
			AddItem(nil, 0, 1, false),
			7, 0, true).
		AddItem(nil, 0, 1, false)

	s.quitDialog = quitOverlay

	pages.AddPage("main", mainLayout, true, true)
	pages.AddPage("help", helpOverlay, true, false)
	pages.AddPage("quit", quitOverlay, true, false)

	app.SetInputCapture(s.handleGlobalInput)
	s.refreshList()
	s.updateChrome("Loaded TODO-tui.md")

	if err := app.SetRoot(pages, true).SetFocus(table).Run(); err != nil {
		panic(err)
	}
}
