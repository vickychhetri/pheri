package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var footerBar *tview.TextView
var commandInput *tview.InputField
var currentApp *tview.Application

func CreateLayoutWithFooter(app *tview.Application, mainContent tview.Primitive) tview.Primitive {
	currentApp = app
	footerBar = tview.NewTextView()
	footerBar.SetDynamicColors(true)
	footerBar.SetTextAlign(tview.AlignLeft)
	footerBar.SetBackgroundColor(CurrentTheme.HeaderBg)

	updateFooterText("Ready")

	commandInput = tview.NewInputField().
		SetLabel(" [white::b]: [-::-]").
		SetFieldWidth(40).
		SetPlaceholder("Enter command (:help, :use <db>, :export, :quit)...").
		SetFieldBackgroundColor(tcell.ColorReset).
		SetFieldTextColor(tcell.ColorWhite).
		SetLabelColor(CurrentTheme.BorderFocused)

	commandInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			cmd := strings.TrimSpace(commandInput.GetText())
			commandInput.SetText("")
			if cmd != "" {
				handleGlobalCommand(app, cmd)
			}
			app.SetFocus(mainContent)
		} else if key == tcell.KeyEscape {
			commandInput.SetText("")
			app.SetFocus(mainContent)
		}
	})

	bottomFlex := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(footerBar, 0, 1, false)

	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(mainContent, 0, 1, true).
		AddItem(bottomFlex, 1, 0, false)

	return layout
}

func updateFooterText(status string) {
	if footerBar == nil {
		return
	}
	envColor := "green"
	if ActiveEnv == "PROD" {
		envColor = "red"
	} else if ActiveEnv == "STAGING" {
		envColor = "yellow"
	}

	txBadge := "[lime::b]AUTOCOMMIT: ON[-::-]"
	if !IsAutoCommitMode() {
		dirtyCount := GetUncommittedTxCount()
		if dirtyCount == 0 {
			txBadge = "[yellow::b]TX: READY[-::-]"
		} else {
			txBadge = fmt.Sprintf("[orange::b]TX: %d PENDING (F10 Commit │ Shift+F10 Rollback │ :review)[-::-]", dirtyCount)
		}
	}

	text := fmt.Sprintf(" [white::b]PHERI[-::-] [%s::b]%s[-::-] %s [gray]│[-] [aqua::b]F1[-::-] Help  [aqua::b]F2[-::-] AutoCommit  [aqua::b]Ctrl+H[-::-] History  [aqua::b]e[-::-] Edit  [aqua::b]Ctrl+R[-::-] Run  [aqua::b]F3[-::-] Schema  [aqua::b]F4[-::-] Top  [aqua::b]Ctrl+E[-::-] Export  [aqua::b]Tab[-::-] Switch [gray]│[-] [white]%s", envColor, ActiveEnv, txBadge, status)
	footerBar.SetText(text)
}

func UpdateFooterRunningQuery(elapsed time.Duration) {
	if footerBar == nil {
		return
	}
	envColor := "green"
	if ActiveEnv == "PROD" {
		envColor = "red"
	} else if ActiveEnv == "STAGING" {
		envColor = "yellow"
	}

	threadInfo := ""
	if threadID := GetActiveQueryConnectionID(); threadID > 0 {
		threadInfo = fmt.Sprintf(" [gray](Thread %d)[-::-]", threadID)
	}

	text := fmt.Sprintf(" [white::b]PHERI[-::-] [%s::b]%s[-::-] [gray]│[-] [yellow::b]Executing Query... (%0.1fs)%s[-::-] [gray]│[-] [aqua::b]Ctrl+C / Esc:[-] Cancel (Server Kill)", envColor, ActiveEnv, elapsed.Seconds(), threadInfo)
	footerBar.SetText(text)
}

func handleGlobalCommand(app *tview.Application, cmd string) {
	cleanCmd := strings.ToLower(strings.TrimPrefix(cmd, ":"))
	parts := strings.Fields(cleanCmd)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "quit", "exit", "q":
		if GetUncommittedTxCount() > 0 {
			PromptUncommittedChangesModal(app, "exit Pheri",
				func() {
					_, _ = CommitActiveTx(app)
					app.Stop()
				},
				func() {
					_, _ = RollbackActiveTx(app)
					app.Stop()
				},
				func() {
					updateFooterText("Exit cancelled.")
				},
			)
			return
		}
		app.Stop()
	case "tx":
		if GetUncommittedTxCount() > 0 {
			ShowTransactionReviewModal(app, activeGridDB,
				func() {
					if count, err := CommitActiveTx(app); err == nil && count > 0 {
						updateFooterText(fmt.Sprintf("Committed %d mutation(s) to database.", count))
					}
				},
				func() {
					if count, err := RollbackActiveTx(app); err == nil && count > 0 {
						updateFooterText(fmt.Sprintf("Rolled back %d mutation(s).", count))
						if activeGridDB != nil && activeGridQuery != "" && dataTable != nil {
							ExecuteQueryAsync(app, activeGridDB, activeGridQuery, dataTable, nil)
						}
					}
				},
			)
		} else {
			ToggleAutoCommit(app, activeGridDB)
		}
	case "autocommit":
		ToggleAutoCommit(app, activeGridDB)
	case "review", "audit", "txstatus":
		ShowTransactionReviewModal(app, activeGridDB,
			func() {
				if count, err := CommitActiveTx(app); err == nil && count > 0 {
					updateFooterText(fmt.Sprintf("Committed %d mutation(s) to database.", count))
				}
			},
			func() {
				if count, err := RollbackActiveTx(app); err == nil && count > 0 {
					updateFooterText(fmt.Sprintf("Rolled back %d mutation(s).", count))
					if activeGridDB != nil && activeGridQuery != "" && dataTable != nil {
						ExecuteQueryAsync(app, activeGridDB, activeGridQuery, dataTable, nil)
					}
				}
			},
		)
	case "commit", "c":
		if count, err := CommitActiveTx(app); err == nil && count > 0 {
			updateFooterText(fmt.Sprintf("Committed %d mutation(s) to database.", count))
		}
	case "rollback", "rb", "abort":
		if count, err := RollbackActiveTx(app); err == nil && count > 0 {
			updateFooterText(fmt.Sprintf("Rolled back %d mutation(s).", count))
			if activeGridDB != nil && activeGridQuery != "" && dataTable != nil {
				ExecuteQueryAsync(app, activeGridDB, activeGridQuery, dataTable, nil)
			}
		}
	case "help", "h":
		ShowComprehensiveHelpModal(app)
	case "clear", "cls":
		if activeSQLEditor != nil {
			activeSQLEditor.SetText("")
		}
	case "export", "exp":
		if activeGridDB != nil && activeGridDBName != "" {
			ShowExportWizardModal(app, activeGridDB, activeGridDBName)
		} else {
			updateFooterText("No active database selected to export")
		}
	case "import", "imp":
		if activeGridDB != nil && activeGridDBName != "" {
			ShowImportWizardModal(app, activeGridDB, activeGridDBName)
		} else {
			updateFooterText("No active database selected to import")
		}
	case "process", "top", "proc":
		if activeGridDB != nil {
			ShowProcessListModal(app, activeGridDB)
		} else {
			updateFooterText("No active database connection for Process Manager")
		}
	case "explain", "expq", "plan":
		if activeGridDB != nil && activeGridQuery != "" {
			showExplainModal(app, activeGridDB, activeGridQuery)
		} else {
			updateFooterText("No active query available to explain")
		}
	case "health", "metrics", "stats":
		if activeGridDB != nil {
			showHealthDashboardModal(app, activeGridDB)
		} else {
			updateFooterText("No active connection for health metrics")
		}
	case "diff", "migrate":
		if activeGridDB != nil && activeGridDBName != "" {
			showSchemaDiffModal(app, activeGridDB, activeGridDBName)
		} else {
			updateFooterText("No active database selected for schema comparison")
		}
	case "history", "hist":
		if activeGridDB != nil {
			showQueryHistoryModal(app, activeGridDB, activeSQLEditor)
		} else {
			updateFooterText("No history available")
		}
	case "format", "fmt":
		if activeSQLEditor != nil {
			raw := activeSQLEditor.GetText()
			activeSQLEditor.SetText(FormatSQLQuery(raw))
			updateFooterText("SQL formatted and keywords normalized!")
		}
	case "about", "info":
		showAboutModal(app)
	case "theme", "colors":
		showThemePickerModal(app)
	default:
		updateFooterText("Unknown command: " + cmd)
	}
}

func showAboutModal(app *tview.Application) {
	aboutText := ` [yellow::b]PHERI - MODERN TERMINAL DATABASE CLIENT[-::-]
 [gray]Version 3.1.0 │ MIT License │ Author: Vicky Chhetri[-::-]
 [gray]Repository: https://github.com/vickychhetri/pheri[-::-]

 [aqua::b]About Pheri[-::-]
 Pheri is a high-performance terminal user interface (TUI) for MySQL written in Go.
 It gives software engineers and DBAs an interactive, keyboard-driven console
 with instant query cancellation, real-time DevOps diagnostics, execution plan
 analysis, schema migrations, and high-speed backups without leaving the terminal.

 [aqua::b]Key Architecture Highlights[-::-]
  * [white]Asynchronous Execution:[-] Non-blocking queries with real-time Ctrl+C / Esc abort
  * [white]Interactive SQL Editor:[-] Multiline editing, table autocomplete, and SQL snippets
  * [white]Live Process Manager:[-] Real-time connection monitor and runaway query killer (F4)
  * [white]Execution Plan Analyzer:[-] EXPLAIN and EXPLAIN ANALYZE visual tree diagrams (F5)
  * [white]Server Health Dashboard:[-] Live QPS, connection pools, and cache hit metrics (F6)
  * [white]Schema Diff Generator:[-] Cross-database schema comparisons and ALTER generators (F7)
  * [white]Parallel Backups:[-] High-speed multi-threaded .sql.gz compressed exports (Ctrl+W)
  * [white]Enterprise Safety:[-] Read-only production safety guards and import verification (F9)`

	view := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true).
		SetText(aboutText)
	view.SetBackgroundColor(CurrentTheme.SurfaceBg)
	StyleModalBox(view.Box, "About Pheri")

	statusBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(" [yellow::b]ESC / ENTER[-::-] Close About ")
	statusBar.SetBackgroundColor(CurrentTheme.HeaderBg)

	contentFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(view, 0, 1, true).
		AddItem(statusBar, 1, 0, false)

	outerFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 2, 0, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 4, 0, false).
			AddItem(contentFlex, 0, 1, true).
			AddItem(nil, 4, 0, false),
			0, 1, true).
		AddItem(nil, 2, 0, false)

	closeModal := func() {
		if mainFlex != nil {
			orig := CreateLayoutWithFooter(app, mainFlex)
			app.SetRoot(orig, true)
		}
	}

	view.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if e.Key() == tcell.KeyEscape || e.Key() == tcell.KeyEnter {
			closeModal()
			return nil
		}
		return e
	})

	outerFlex.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if e.Key() == tcell.KeyEscape || e.Key() == tcell.KeyEnter {
			closeModal()
			return nil
		}
		return e
	})

	app.SetRoot(CreateLayoutWithFooter(app, outerFlex), true).SetFocus(view)
}

func showGlobalHelpModal(app *tview.Application) {
	ShowComprehensiveHelpModal(app)
}

// ShowCommandPaletteModal opens an interactive command prompt dialog for ':' commands.
func ShowCommandPaletteModal(app *tview.Application) {
	if app == nil {
		return
	}

	input := tview.NewInputField().
		SetLabel(" [aqua::b]: [-::-]").
		SetFieldWidth(45).
		SetPlaceholder("commit, rollback, review, autocommit, help, quit...").
		SetFieldBackgroundColor(tcell.ColorReset).
		SetFieldTextColor(tcell.ColorWhite).
		SetLabelColor(CurrentTheme.BorderFocused)

	hintView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetText(" [gray]Commands: [aqua]commit[-] │ [aqua]rollback[-] │ [aqua]review[-] │ [aqua]autocommit[-] │ [aqua]help[-] │ [aqua]quit[-]")
	hintView.SetBackgroundColor(CurrentTheme.SurfaceBg)

	box := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(input, 1, 0, true).
		AddItem(hintView, 1, 0, false)
	box.SetBorder(true)
	box.SetTitle(" Command Launcher (:command) ")
	box.SetTitleColor(CurrentTheme.TitleFocused)
	box.SetBorderColor(CurrentTheme.BorderFocused)
	box.SetBackgroundColor(CurrentTheme.SurfaceBg)

	modal := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(box, 55, 0, true).
			AddItem(nil, 0, 1, false),
			4, 0, true).
		AddItem(nil, 0, 1, false)

	closePalette := func() {
		if mainFlex != nil {
			orig := CreateLayoutWithFooter(app, mainFlex)
			app.SetRoot(orig, true)
		}
	}

	input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			cmd := strings.TrimSpace(input.GetText())
			closePalette()
			if cmd != "" {
				handleGlobalCommand(app, cmd)
			}
		} else if key == tcell.KeyEscape {
			closePalette()
		}
	})

	app.SetRoot(modal, true).SetFocus(input)
}
