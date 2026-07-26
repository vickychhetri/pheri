package ui

import (
	"fmt"
	"strings"

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

	text := fmt.Sprintf("[white::b]PHERI[-] [%s] | [green]F1:[-] Help | [cyan]F3:[-] Schema | [yellow]F4:[-] Top | [lime]Ctrl+R:[-] Run | [yellow]Ctrl+E:[-] Export | [white]Tab:[-] Switch [gray]| %s", envBadge, status)
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
	default:
		updateFooterText("Unknown command: " + cmd)
	}
}

func showGlobalHelpModal(app *tview.Application) {
	helpText := `[aqua::b]⚡ PHERI MYSQL TUI DEVELOPER SHORTCUTS ⚡[::-]

[lime::b]DevOps & Process Manager:[::-]
  • [white]F4 / :process / :top[-]: Open Live Process Manager & Query Killer
  • [white]F5 / :explain[-]: View Query Execution Plan (EXPLAIN ANALYZE)

[lime::b]Editor Controls:[::-]
  • [white]Ctrl+R / Ctrl+Enter[-]: Execute SQL Query
  • [white]F11[-]: Toggle Fullscreen Code Editor
  • [white]Ctrl+S / Ctrl+T[-]: Open SQL Snippets & Template Modal

[lime::b]Navigation & Grid:[::-]
  • [white]Tab / Shift+Tab[-]: Cycle focus between Sidebar, Editor, and Grid
  • [white]Esc[-]: Back to main layout / Clear focus
  • [white]F3[-]: Inspect Table Columns & Indexes Schema
  • [white]Ctrl+E / :export[-]: Multi-Object Selective Database Export

[lime::b]Command Launcher:[::-]
  • Type [yellow]:help[-], [yellow]:process[-], [yellow]:explain[-], [yellow]:export[-], or [yellow]:quit[-]`

	modal := tview.NewModal().
		SetText(helpText).
		AddButtons([]string{"[green]Close"}).
		SetBackgroundColor(tcell.ColorDarkBlue).
		SetButtonBackgroundColor(tcell.ColorDarkCyan).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			// Dismiss modal
		})
	app.SetRoot(modal, true)
}
