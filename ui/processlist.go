// ui/processlist.go
package ui

import (
	"database/sql"
	"fmt"
	"mysql-tui/util"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ProcessItem struct {
	ID      string
	User    string
	Host    string
	DB      string
	Command string
	Time    string
	State   string
	Info    string
}

func ShowProcessListModal(app *tview.Application, db *sql.DB) {
	if db == nil {
		showErrorModal(app, mainFlex, "No active database connection available.")
		return
	}

	table := tview.NewTable().
		SetBorders(true).
		SetSelectable(true, false)

	table.SetBorder(true).
		SetTitle(" ⚡ MySQL Live Process Manager (SHOW FULL PROCESSLIST) ").
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorDarkCyan)

	table.SetBackgroundColor(tcell.ColorBlack)

	headerStyle := tcell.StyleDefault.
		Foreground(tcell.ColorBlack).
		Background(tcell.ColorAqua).
		Bold(true)

	headers := []string{"ID", "User", "Host", "DB", "Command", "Time (s)", "State", "Query Info"}

	loadProcesses := func() {
		table.Clear()
		for col, h := range headers {
			cell := tview.NewTableCell(fmt.Sprintf(" %s ", h)).
				SetStyle(headerStyle).
				SetAlign(tview.AlignCenter).
				SetSelectable(false)
			table.SetCell(0, col, cell)
		}

		rows, err := db.Query("SHOW FULL PROCESSLIST")
		if err != nil {
			showErrorModal(app, mainFlex, "Failed to fetch processlist: "+err.Error())
			return
		}
		defer rows.Close()

		rowIdx := 1
		for rows.Next() {
			var id, user, host, dbName, command, timeVal, state, info sql.NullString
			err := rows.Scan(&id, &user, &host, &dbName, &command, &timeVal, &state, &info)
			if err != nil {
				continue
			}

			colVals := []string{
				id.String,
				user.String,
				host.String,
				dbName.String,
				command.String,
				timeVal.String,
				state.String,
				info.String,
			}

			rowStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite)
			if strings.EqualFold(command.String, "Query") {
				rowStyle = rowStyle.Foreground(tcell.ColorGreen).Bold(true)
			}

			for c, val := range colVals {
				displayVal := val
				if len(displayVal) > 40 && c == 7 {
					displayVal = displayVal[:37] + "..."
				}
				cell := tview.NewTableCell(" " + displayVal + " ").
					SetStyle(rowStyle).
					SetAlign(tview.AlignLeft)
				table.SetCell(rowIdx, c, cell)
			}
			rowIdx++
		}

		if rowIdx == 1 {
			table.SetCell(1, 0, tview.NewTableCell(" No active processes found ").SetTextColor(tcell.ColorYellow))
		} else {
			table.Select(1, 0)
		}
	}

	loadProcesses()

	statusBar := tview.NewTextView().
		SetText("[cyan]↑/↓[-] Move | [lime]r[-] Refresh | [red]k / DEL[-] Kill Query | [yellow]ESC[-] Close").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetBackgroundColor(tcell.ColorBlack)

	killProcess := func(procID string, killQueryOnly bool) {
		cmd := "KILL " + procID
		if killQueryOnly {
			cmd = "KILL QUERY " + procID
		}
		_, err := db.Exec(cmd)
		if err != nil {
			showErrorModal(app, mainFlex, fmt.Sprintf("Failed to kill process %s: %s", procID, err.Error()))
			return
		}
		loadProcesses()
	}

	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(statusBar, 1, 0, false)

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			orig := CreateLayoutWithFooter(app, mainFlex)
			app.SetRoot(orig, true)
			if tableList != nil {
				util.SetFocusWithBorder(app, tableList)
			}
			return nil
		}
		if event.Key() == tcell.KeyRune && (event.Rune() == 'r' || event.Rune() == 'R') {
			loadProcesses()
			return nil
		}
		if event.Key() == tcell.KeyDelete || (event.Key() == tcell.KeyRune && (event.Rune() == 'k' || event.Rune() == 'K')) {
			r, _ := table.GetSelection()
			if r >= 1 && table.GetCell(r, 0) != nil {
				procID := strings.TrimSpace(table.GetCell(r, 0).Text)
				if procID != "" && procID != "ID" {
					modal := tview.NewModal().
						SetText(fmt.Sprintf("[yellow::b]⚡ Kill MySQL Connection / Query\n\n[white]Target Thread ID: [cyan]%s\nSelect action:", procID)).
						AddButtons([]string{"[red]Kill Connection", "[yellow]Kill Query Only", "[white]Cancel"}).
						SetBackgroundColor(tcell.ColorDarkBlue).
						SetButtonBackgroundColor(tcell.ColorDarkCyan).
						SetDoneFunc(func(buttonIndex int, buttonLabel string) {
							if buttonIndex == 0 {
								killProcess(procID, false)
							} else if buttonIndex == 1 {
								killProcess(procID, true)
							}
							fullL := CreateLayoutWithFooter(app, layout)
							app.SetRoot(fullL, true)
							app.SetFocus(table)
						})
					app.SetRoot(modal, true)
				}
			}
			return nil
		}
		return event
	})

	fullLayout := CreateLayoutWithFooter(app, layout)
	app.SetRoot(fullLayout, true)
	app.SetFocus(table)
}
