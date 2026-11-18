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

func CreateLayoutWithFooter(app *tview.Application, mainContent tview.Primitive) tview.Primitive {
	footerBar = tview.NewTextView()
	footerBar.SetDynamicColors(true)
	footerBar.SetTextAlign(tview.AlignCenter)
	footerBar.SetBackgroundColor(tcell.Color234)

	updateFooterText("Ready")

	commandInput = tview.NewInputField().
		SetLabel(" [lime::b]: [white]").
		SetFieldWidth(40).
		SetPlaceholder("Enter command (:help, :use db, :export csv, :quit)...").
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetFieldTextColor(tcell.ColorWhite).
		SetLabelColor(tcell.ColorLime)

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
	envBadge := "[green::b]DEV[-::-]"
	if ActiveEnv == "PROD" {
		envBadge = "[red::b]PROD[-::-]"
	} else if ActiveEnv == "STAGING" {
		envBadge = "[yellow::b]STAGING[-::-]"
	}

	text := fmt.Sprintf("[white::b]PHERI[-] [%s] | [lime]e:[-] Edit SQL | [lime]Ctrl+R:[-] Run | [yellow]F2:[-] History | [cyan]F3:[-] Schema | [yellow]F4:[-] Top | [yellow]Ctrl+E:[-] Export | [white]Tab:[-] Switch [gray]| %s", envBadge, status)
	footerBar.SetText(text)
}

func UpdateFooterRunningQuery(elapsed time.Duration) {
	if footerBar == nil {
		return
	}
	envBadge := "[green::b]DEV[-::-]"
	if ActiveEnv == "PROD" {
		envBadge = "[red::b]PROD[-::-]"
	} else if ActiveEnv == "STAGING" {
		envBadge = "[yellow::b]STAGING[-::-]"
	}

	text := fmt.Sprintf("[white::b]PHERI[-] [%s] | [yellow::b]⚡ Executing Query... (%0.1fs)[-::-] | [lime::b]Ctrl+C / Esc:[-] Cancel Query", envBadge, elapsed.Seconds())
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
		app.Stop()
	case "help", "h":
		showGlobalHelpModal(app)
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
	aboutText := `[white::b]Pheri - Terminal Database Management TUI[::-]
Version: 1.0.0
Author: Vicky Chhetri
License: MIT
Repository: https://github.com/vickychhetri/pheri

About:
Pheri is a fast, terminal-based database management interface built for MySQL in Go.
It offers developers an interactive environment with asynchronous query execution,
real-time process management, EXPLAIN performance analyzer, schema diff generator,
and high-speed compressed database backups directly from the terminal.

Features:
- Asynchronous query execution with real-time Ctrl+C / Esc cancellation
- Interactive multiline SQL code editor with autocomplete and snippets
- Live process manager and query killer (F4)
- EXPLAIN query execution plan analyzer (F5)
- Server health and metrics dashboard (F6)
- Database schema diff generator (F7)
- Searchable query history (F2)
- High-speed compressed export and import (.sql.gz)`

	modal := tview.NewModal().
		SetText(aboutText).
		AddButtons([]string{"Close"}).
		SetBackgroundColor(tcell.ColorDarkBlue).
		SetButtonBackgroundColor(tcell.ColorDarkCyan).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if mainFlex != nil {
				layout := CreateLayoutWithFooter(app, mainFlex)
				app.SetRoot(layout, true)
			}
		})
	app.SetRoot(modal, true)
}

func showGlobalHelpModal(app *tview.Application) {
	helpText := `[aqua::b]PHERI MYSQL TUI DEVELOPER SHORTCUTS[::-]

[lime::b]DevOps & Process Manager:[::-]
  * [white]F4 / :process / :top[-]: Open Live Process Manager & Query Killer
  * [white]F5 / :explain[-]: View Query Execution Plan (EXPLAIN ANALYZE)
  * [white]F6 / :health[-]: Real-Time Server Health & Performance Dashboard
  * [white]F7 / :diff[-]: Database Schema Diff & Migration Generator

[lime::b]Editor Controls:[::-]
  * [white]e[-]: Jump Focus Directly to SQL Editor
  * [white]Ctrl+R / Ctrl+Enter[-]: Execute SQL Query
  * [white]F2 / :history[-]: Searchable Execution History & Restorer
  * [white]:format / :fmt[-]: Auto-Format SQL Keywords & Clauses
  * [white]F11[-]: Toggle Fullscreen Code Editor
  * [white]Ctrl+S / Ctrl+T[-]: Open SQL Snippets & Table Autocomplete Modal

[lime::b]Navigation & Grid:[::-]
  * [white]Tab / Shift+Tab[-]: Cycle focus between Sidebar, Editor, and Grid
  * [white]Esc[-]: Back to main layout / Clear focus
  * [white]F3[-]: Inspect Table Columns & Indexes Schema
  * [white]Ctrl+E / :export[-]: Multi-Object Selective Database Export
  * [white]F9 / :import[-]: Database Import Wizard (Requires confirmation)

[lime::b]Command Launcher:[::-]
  * Type [yellow]:help[-], [yellow]:about[-], [yellow]:health[-], [yellow]:diff[-], [yellow]:history[-], [yellow]:import[-], [yellow]:export[-], or [yellow]:quit[-]`

	modal := tview.NewModal().
		SetText(helpText).
		AddButtons([]string{"Close"}).
		SetBackgroundColor(tcell.ColorDarkBlue).
		SetButtonBackgroundColor(tcell.ColorDarkCyan).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if mainFlex != nil {
				layout := CreateLayoutWithFooter(app, mainFlex)
				app.SetRoot(layout, true)
			}
		})
	app.SetRoot(modal, true)
}
