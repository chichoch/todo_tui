package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func parseCLIArgs(args []string) (file string, noHistory bool, err error) {
	fs := flag.NewFlagSet("todo-tui", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&noHistory, "no-history", false, "skip appending deletions to history file")
	fs.BoolVar(&noHistory, "n", false, "skip appending deletions to history file (shorthand)")
	if err := fs.Parse(args); err != nil {
		return "", false, err
	}
	if fs.NArg() > 1 {
		return "", false, fmt.Errorf("unexpected extra arguments: %v", fs.Args()[1:])
	}
	file = fs.Arg(0)
	if file != "" && !strings.HasSuffix(strings.ToLower(file), ".md") {
		return "", false, fmt.Errorf("file must have a .md extension: %s", file)
	}
	return file, noHistory, nil
}

func applyCLIOverrides(cfg config, argFile string, noHistory bool) (config, string) {
	var filePath string
	if argFile != "" {
		cfg.FilePath = ""
		cfg.FileName = ""
		cfg.FileCmdSave = ""
		cfg.FileCmdLoad = ""
		filePath = expandHome(argFile)
	}
	if noHistory {
		cfg.HistoryFile = ""
	}
	return cfg, filePath
}

func main() {
	argFile, noHistory, err := parseCLIArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "args: %v\n", err)
		os.Exit(2)
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	cfg, filePath := applyCLIOverrides(cfg, argFile, noHistory)
	if filePath == "" {
		filePath = resolveFilePath(cfg)
	}

	// If a load command is configured, fetch the file to a temp directory.
	var loadStatus string
	var items []Item
	var fileCtx *fileContext
	var loadedFromRemote bool
	if cfg.FileCmdLoad != "" {
		tmpDir, err := os.MkdirTemp("", "todo_tui_load_")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
			os.Exit(1)
		}

		if err := runFileCmd(cfg.FileCmdLoad, tmpDir, configFileName(cfg)); err != nil {
			os.RemoveAll(tmpDir)
			loadStatus = fmt.Sprintf("file-cmd-load failed: %v (starting empty)", err)
		} else {
			tmpFile := filepath.Join(tmpDir, resolveFileName(cfg))
			items, fileCtx, err = loadItems(tmpFile)
			os.RemoveAll(tmpDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "load: %v\n", err)
				os.Exit(1)
			}
			loadedFromRemote = true
			// Seed the merge base on first run if missing.
			if base := cachePath(cfg); base != "" {
				if _, statErr := os.Stat(base); os.IsNotExist(statErr) {
					if mkErr := ensureCacheDir(cfg); mkErr == nil {
						_ = saveItems(base, items, nil)
					}
				}
			}
		}
	}

	if !loadedFromRemote {
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "load: %v\n", err)
			os.Exit(1)
		}
		var err error
		items, fileCtx, err = loadItems(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load: %v\n", err)
			os.Exit(1)
		}
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
	helpBox.SetText("A: add\ndigits+Enter: jump\nEnter: toggle\nc: edit\nd: delete\ns/w: save/sync\nEsc: cancel\n?/h: this help\n\nq: exit")

	pages := tview.NewPages()

	s := &state{
		app:               app,
		pages:             pages,
		table:             table,
		input:             input,
		statusBar:         statusBar,
		helpBox:           helpBox,
		items:             items,
		fileCtx:           fileCtx,
		filePath:          filePath,
		cfg:               cfg,
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

	savingLabel := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetTextColor(tcell.ColorGreen).
		SetDynamicColors(false)
	savingLabel.SetBackgroundColor(tcell.ColorDefault).
		SetBorder(true).
		SetBorderColor(tcell.ColorGreen).
		SetTitleColor(tcell.ColorGreen)

	savingOverlay := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(savingLabel, 20, 0, false).
			AddItem(nil, 0, 1, false),
			3, 0, false).
		AddItem(nil, 0, 1, false)

	s.savingLabel = savingLabel

	conflictLabel := tview.NewTextView().
		SetTextAlign(tview.AlignLeft).
		SetTextColor(tcell.ColorGreen).
		SetDynamicColors(false)
	conflictLabel.SetBackgroundColor(tcell.ColorDefault).
		SetBorder(true).
		SetBorderColor(tcell.ColorGreen).
		SetTitle(" Sync Conflict ").
		SetTitleColor(tcell.ColorGreen)

	conflictOverlay := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(conflictLabel, 60, 0, true).
			AddItem(nil, 0, 1, false),
			10, 0, true).
		AddItem(nil, 0, 1, false)

	s.conflictLabel = conflictLabel
	s.conflictOverlay = conflictOverlay

	pages.AddPage("main", mainLayout, true, true)
	pages.AddPage("help", helpOverlay, true, false)
	pages.AddPage("quit", quitOverlay, true, false)
	pages.AddPage("saving", savingOverlay, true, false)
	pages.AddPage("conflict", conflictOverlay, true, false)

	app.SetInputCapture(s.handleGlobalInput)
	s.refreshList()
	if loadStatus != "" {
		s.updateChrome(loadStatus)
	} else {
		s.updateChrome("Loaded " + filePath)
	}

	if err := app.SetRoot(pages, true).SetFocus(table).Run(); err != nil {
		panic(err)
	}

	if s.quitSaveTimedOut {
		path := s.quitSaveTempPath
		if path == "" {
			path = "(unknown; check $TMPDIR for todo_tui_save_*)"
		}
		fmt.Fprintf(os.Stderr,
			"Warning: save command did not finish within 10s on quit.\n"+
				"Your unsaved checklist is in: %s\n"+
				"Copy it out before the OS cleans the temp directory.\n",
			path)
	}
}
