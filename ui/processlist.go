// ui/processlist.go
package ui

import (
	"database/sql"
	"fmt"
	"mysql-tui/util"
	"strconv"
	"strings"
	"time"

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

	var allProcesses []ProcessItem
	filterText := ""
	autoRefresh := false
	var ticker *time.Ticker
	var stopChan chan struct{}

	metricsBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	metricsBar.SetBackgroundColor(tcell.ColorDarkBlue)

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

	renderTable := func() {
		table.Clear()
		for col, h := range headers {
			cell := tview.NewTableCell(fmt.Sprintf(" %s ", h)).
				SetStyle(headerStyle).
				SetAlign(tview.AlignCenter).
				SetSelectable(false)
			table.SetCell(0, col, cell)
		}

		activeCount := 0
		sleepCount := 0
		maxTime := 0

		rowIdx := 1
		for _, proc := range allProcesses {
			tVal, _ := strconv.Atoi(proc.Time)
			if tVal > maxTime {
				maxTime = tVal
			}
			if strings.EqualFold(proc.Command, "Query") {
				activeCount++
			} else if strings.EqualFold(proc.Command, "Sleep") {
				sleepCount++
			}

			// Apply Filter
			if filterText != "" {
				s := strings.ToLower(filterText)
				if !strings.Contains(strings.ToLower(proc.ID), s) &&
					!strings.Contains(strings.ToLower(proc.User), s) &&
					!strings.Contains(strings.ToLower(proc.Host), s) &&
					!strings.Contains(strings.ToLower(proc.DB), s) &&
					!strings.Contains(strings.ToLower(proc.Command), s) &&
					!strings.Contains(strings.ToLower(proc.State), s) &&
					!strings.Contains(strings.ToLower(proc.Info), s) {
					continue
				}
			}

			colVals := []string{
				proc.ID,
				proc.User,
				proc.Host,
				proc.DB,
				proc.Command,
				proc.Time,
				proc.State,
				proc.Info,
			}

			// Color-coding based on execution time and command
			rowStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite)
			if tVal > 5 && strings.EqualFold(proc.Command, "Query") {
				rowStyle = rowStyle.Foreground(tcell.ColorRed).Bold(true)
			} else if tVal > 2 && strings.EqualFold(proc.Command, "Query") {
				rowStyle = rowStyle.Foreground(tcell.ColorYellow).Bold(true)
			} else if strings.EqualFold(proc.Command, "Query") {
				rowStyle = rowStyle.Foreground(tcell.ColorGreen).Bold(true)
			} else if strings.EqualFold(proc.Command, "Sleep") {
				rowStyle = rowStyle.Foreground(tcell.ColorGray)
			}

			for c, val := range colVals {
				displayVal := val
				if len(displayVal) > 45 && c == 7 {
					displayVal = displayVal[:42] + "..."
				}
				cell := tview.NewTableCell(" " + displayVal + " ").
					SetStyle(rowStyle).
					SetAlign(tview.AlignLeft)
				table.SetCell(rowIdx, c, cell)
			}
			rowIdx++
		}

		if rowIdx == 1 {
			table.SetCell(1, 0, tview.NewTableCell(" No matching processes found ").SetTextColor(tcell.ColorYellow))
		} else {
			table.Select(1, 0)
		}

		refreshStatus := "[red]OFF[-]"
		if autoRefresh {
			refreshStatus = "[lime]ON (2s)[-]"
		}

		metricsText := fmt.Sprintf(
			" [white::b]Total Threads: [cyan]%d[-] | [white::b]Active Queries: [lime]%d[-] | [white::b]Sleeping: [gray]%d[-] | [white::b]Max Time: [yellow]%ds[-] | [white::b]Auto-Refresh: %s ",
			len(allProcesses), activeCount, sleepCount, maxTime, refreshStatus,
		)
		metricsBar.SetText(metricsText)
	}

	loadProcesses := func() {
		rows, err := db.Query("SHOW FULL PROCESSLIST")
		if err != nil {
			showErrorModal(app, mainFlex, "Failed to fetch processlist: "+err.Error())
			return
		}
		defer rows.Close()

		allProcesses = nil
		for rows.Next() {
			var id, user, host, dbName, command, timeVal, state, info sql.NullString
			if err := rows.Scan(&id, &user, &host, &dbName, &command, &timeVal, &state, &info); err == nil {
				allProcesses = append(allProcesses, ProcessItem{
					ID:      id.String,
					User:    user.String,
					Host:    host.String,
					DB:      dbName.String,
					Command: command.String,
					Time:    timeVal.String,
					State:   state.String,
					Info:    info.String,
				})
			}
		}
		renderTable()
	}

	loadProcesses()

	statusBar := tview.NewTextView().
		SetText("[cyan]↑/↓[-] Move | [lime]r[-] Refresh | [yellow]a[-] Auto-Refresh | [aqua]/[-] Filter | [lime]SPACE/ENTER[-] View SQL | [red]k/DEL[-] Kill | [yellow]ESC[-] Exit").
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
		AddItem(metricsBar, 1, 0, false).
		AddItem(table, 0, 1, true).
		AddItem(statusBar, 1, 0, false)

	closeModal := func() {
		if stopChan != nil {
			close(stopChan)
			stopChan = nil
		}
		orig := CreateLayoutWithFooter(app, mainFlex)
		app.SetRoot(orig, true)
		if tableList != nil {
			util.SetFocusWithBorder(app, tableList)
		}
	}

	toggleAutoRefresh := func() {
		autoRefresh = !autoRefresh
		if autoRefresh {
			stopChan = make(chan struct{})
			ticker = time.NewTicker(2 * time.Second)
			go func() {
				for {
					select {
					case <-stopChan:
						ticker.Stop()
						return
					case <-ticker.C:
						app.QueueUpdateDraw(func() {
							loadProcesses()
						})
					}
				}
			}()
		} else {
			if stopChan != nil {
				close(stopChan)
				stopChan = nil
			}
		}
		renderTable()
	}

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			closeModal()
			return nil
		}
		if event.Key() == tcell.KeyRune && (event.Rune() == 'r' || event.Rune() == 'R') {
			loadProcesses()
			return nil
		}
		if event.Key() == tcell.KeyRune && (event.Rune() == 'a' || event.Rune() == 'A') {
			toggleAutoRefresh()
			return nil
		}
		if event.Key() == tcell.KeyRune && event.Rune() == '/' {
			filterInput := tview.NewInputField().
				SetLabel("Filter Process List: ").
				SetText(filterText).
				SetFieldWidth(30)
			filterInput.SetBorder(true).SetTitle(" Search Processes ")
			filterInput.SetDoneFunc(func(key tcell.Key) {
				filterText = filterInput.GetText()
				renderTable()
				fullL := CreateLayoutWithFooter(app, layout)
				app.SetRoot(fullL, true)
				app.SetFocus(table)
			})
			app.SetRoot(filterInput, true).SetFocus(filterInput)
			return nil
		}
		if event.Key() == tcell.KeyRune && event.Rune() == ' ' || event.Key() == tcell.KeyEnter {
			r, _ := table.GetSelection()
			if r >= 1 && r <= len(allProcesses) {
				proc := allProcesses[r-1]
				sqlView := tview.NewTextView().
					SetDynamicColors(true).
					SetText(fmt.Sprintf("[yellow::b]Thread ID: [white]%s | [yellow::b]User: [white]%s@%s | [yellow::b]DB: [white]%s\n[yellow::b]State: [white]%s | [yellow::b]Time: [white]%ss\n\n[lime::b]Query SQL:\n[white]%s",
						proc.ID, proc.User, proc.Host, proc.DB, proc.State, proc.Time, proc.Info)).
					SetScrollable(true)
				sqlView.SetBorder(true).SetTitle(" 🔍 Full Query Details ")

				sqlModal := tview.NewFlex().SetDirection(tview.FlexRow).
					AddItem(sqlView, 0, 1, true).
					AddItem(tview.NewTextView().SetText("[yellow]ESC / ENTER[-] Close Detail View  |  [lime]C[-] Copy SQL").SetTextAlign(tview.AlignCenter).SetDynamicColors(true), 1, 0, false)

				sqlView.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
					if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyEnter {
						fullL := CreateLayoutWithFooter(app, layout)
						app.SetRoot(fullL, true)
						app.SetFocus(table)
						return nil
					}
					if ev.Key() == tcell.KeyRune && (ev.Rune() == 'c' || ev.Rune() == 'C') {
						util.SetClipboardText(proc.Info)
					}
					return ev
				})

				app.SetRoot(sqlModal, true).SetFocus(sqlView)
				return nil
			}
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
