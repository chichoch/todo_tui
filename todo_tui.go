package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const todoFileName = "TODO_tui.md"

func main() {
	items, err := loadItems(todoFileName)
	if err != nil {
		panic(err)
	}

	app := tview.NewApplication()
	table := tview.NewTable().
		SetSelectable(true, false)
	table.SetBorder(true).
		SetBorderColor(tcell.ColorGreen).
		SetTitleColor(tcell.ColorGreen).
		SetBackgroundColor(tcell.ColorBlack)
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorGreen).
		Foreground(tcell.ColorBlack))

	input := tview.NewInputField().SetLabel("Add: ").
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetFieldTextColor(tcell.ColorGreen).
		SetLabelColor(tcell.ColorGreen)
	input.SetBackgroundColor(tcell.ColorBlack)

	statusBar := tview.NewTextView().
		SetTextAlign(tview.AlignRight).
		SetTextColor(tcell.ColorGreen).
		SetDynamicColors(false)
	statusBar.SetBackgroundColor(tcell.ColorBlack)

	helpBox := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetTextColor(tcell.ColorGreen).
		SetDynamicColors(false)
	helpBox.SetBackgroundColor(tcell.ColorBlack).
		SetBorder(true).
		SetBorderColor(tcell.ColorGreen).
		SetTitle(" Keys ").
		SetTitleColor(tcell.ColorGreen)
	helpBox.SetText("A: add\ndigits+Enter: jump\nEnter: toggle\nc: edit\nd: delete\ns: save\nEsc: cancel")

	s := &state{
		app:               app,
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

	table.SetInputCapture(s.handleListInput)
	input.SetDoneFunc(func(key tcell.Key) {
		s.handleInputDone(key)
	})

	// Content area: table + help box side by side
	contentRow := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(table, 0, 3, true).
		AddItem(helpBox, 22, 0, false)
	contentRow.SetBackgroundColor(tcell.ColorBlack)

	// Bottom row: input + status bar inline
	bottomRow := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(input, 0, 1, false).
		AddItem(statusBar, 40, 0, false)
	bottomRow.SetBackgroundColor(tcell.ColorBlack)

	// Root: content + bottom row
	root := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(contentRow, 0, 1, true).
		AddItem(bottomRow, 1, 0, false)
	root.SetBackgroundColor(tcell.ColorBlack)

	app.SetInputCapture(s.handleGlobalInput)
	s.refreshList()
	s.updateChrome("Loaded TODO_tui.md")

	if err := app.SetRoot(root, true).SetFocus(table).Run(); err != nil {
		panic(err)
	}
}
