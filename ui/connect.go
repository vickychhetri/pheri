// ui/connect.go
package ui

import (
	"database/sql"
	"log"
	"mysql-tui/dbs"
	"mysql-tui/phhistory"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var user, pass, host, port string

func ShowConnectionForm(app *tview.Application, user, pass, host, port string) {
	var form *tview.Form

	if user != "" && host != "" && port != "" {
		conn, err := dbs.Connect(user, pass, host, port)
		if err != nil {
			log.Printf("Error in db Connection: %v", err)
			app.Stop()
		}
		ShowDatabaseList(app, conn)

	} else {
		form = tview.NewForm().
			AddInputField("Host", "127.0.0.1", 20, nil, nil).
			AddInputField("Port", "3306", 6, nil, nil).
			AddInputField("User", "root", 20, nil, nil).
			AddPasswordField("Password", "", 20, '*', nil).
			AddButton("Connect", func() {
				host = form.GetFormItemByLabel("Host").(*tview.InputField).GetText()
				port = form.GetFormItemByLabel("Port").(*tview.InputField).GetText()
				user = form.GetFormItemByLabel("User").(*tview.InputField).GetText()
				pass = form.GetFormItemByLabel("Password").(*tview.InputField).GetText()

				phhistory.SetUser(user)
				phhistory.SetHost(host)
				phhistory.SetPort(port)
				conn, err := dbs.Connect(user, pass, host, port)
				if err != nil {
					modal := tview.NewModal().
						SetText("Connection failed: " + err.Error()).
						AddButtons([]string{"OK"}).
						SetDoneFunc(func(buttonIndex int, buttonLabel string) {
							app.SetRoot(form, true)
						})
					app.SetRoot(modal, true)
					return
				}

				ShowDatabaseList(app, conn)
			}).
			AddButton("Clear", func() {
				form.GetFormItemByLabel("Host").(*tview.InputField).SetText("")
				form.GetFormItemByLabel("Port").(*tview.InputField).SetText("")
				form.GetFormItemByLabel("User").(*tview.InputField).SetText("")
				form.GetFormItemByLabel("Password").(*tview.InputField).SetText("")

			}).
			AddButton("Quit", func() {
				app.Stop()
			})
		form.SetFieldBackgroundColor(tcell.ColorLightGray)
		form.SetBorder(true).SetTitle("MySQL Connection")
		form.SetBorderPadding(1, 1, 2, 2) // Top, bottom, left, right padding

		layout := CreateLayoutWithFooter(app, form)
		app.SetRoot(layout, true).SetFocus(form)
	}

}

// func ShowDatabaseList(app *tview.Application, db *sql.DB) {
// 	table := tview.NewTable().
// 		SetSelectable(true, false).
// 		SetFixed(1, 0)

// 	// Set table header
// 	table.SetCell(0, 0, tview.NewTableCell("Database Name").
// 		SetTextColor(tcell.ColorYellow).
// 		SetAlign(tview.AlignCenter).
// 		SetSelectable(false))

// 	// Query databases
// 	rows, err := db.Query("SHOW DATABASES")
// 	if err != nil {
// 		table.SetCell(1, 0, tview.NewTableCell("Error: "+err.Error()).
// 			SetTextColor(tcell.ColorRed).
// 			SetAlign(tview.AlignCenter))
// 	} else {
// 		defer rows.Close()
// 		var dbName string
// 		rowIndex := 1
// 		for rows.Next() {
// 			rows.Scan(&dbName)

// 			// Add database name to table
// 			table.SetCell(rowIndex, 0, tview.NewTableCell(dbName).
// 				SetAlign(tview.AlignLeft).
// 				SetSelectable(true))
// 			rowIndex++
// 		}

// 		// Set selected function to open UseDatabase view
// 		table.SetSelectedFunc(func(row, column int) {
// 			cell := table.GetCell(row, 0)
// 			if cell != nil {
// 				dbName := cell.Text
// 				UseDatabase(app, db, dbName)
// 			}
// 		})
// 	}

// 	// Add keybinding to go back
// 	table.SetDoneFunc(func(key tcell.Key) {
// 		if key == tcell.KeyEscape {
// 			ShowConnectionForm(app, user, pass, host, port)
// 		}
// 	})

// 	// Wrap table in a layout
// 	layout := CreateLayoutWithFooter(app, table)
// 	app.SetRoot(layout, true)
// }

// func ShowDatabaseList(app *tview.Application, db *sql.DB) {
// 	list := tview.NewList()

// 	rows, err := db.Query("SHOW DATABASES")
// 	if err != nil {
// 		list.AddItem("Error: "+err.Error(), "", 0, nil)
// 	} else {
// 		defer rows.Close()
// 		var dbName string
// 		for rows.Next() {
// 			rows.Scan(&dbName)
// 			// Add each DB with a handler that opens its tables view
// 			list.AddItem(dbName, "", 0, func(name string) func() {
// 				return func() {
// 					UseDatabase(app, db, name)
// 				}
// 			}(dbName))
// 		}
// 	}

// 	list.AddItem("Back", "Return to connection screen", 'b', func() {
// 		ShowConnectionForm(app, user, pass, host, port)
// 	})

// 	layout := CreateLayoutWithFooter(app, list)
// 	app.SetRoot(layout, true)
// }

func ShowDatabaseList(app *tview.Application, db *sql.DB) {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true).SetTitle(" 🗄️ Databases ").SetTitleAlign(tview.AlignLeft)

	searchInput := tview.NewInputField()
	searchInput.
		SetLabel("🔍 Search: ").
		SetFieldWidth(30).
		SetPlaceholder("Start typing...").
		SetBorder(true)

	statusBar := tview.NewTextView().
		SetText(" ↑/↓ Navigate • Enter: Select DB • b: Back ").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	var dbNames []string
	filtered := func(filter string) {
		list.Clear()
		for _, name := range dbNames {
			if strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
				dbCopy := name
				list.AddItem("[::b]"+dbCopy+"[::-]", "", 0, func() {
					UseDatabase(app, db, dbCopy)
				})
			}
		}
		list.AddItem("Back", "Return to connection screen", 'b', func() {
			ShowConnectionForm(app, user, pass, host, port)
		})
	}

	rows, err := db.Query("SHOW DATABASES")
	if err != nil {
		list.AddItem("Error: "+err.Error(), "", 0, nil)
	} else {
		defer rows.Close()
		var dbName string
		for rows.Next() {
			rows.Scan(&dbName)
			dbNames = append(dbNames, dbName)
		}
		filtered("")
	}

	searchInput.SetChangedFunc(func(text string) {
		filtered(text)
	})

	searchInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTAB {
			app.SetFocus(list)
			return nil
		}
		return event
	})

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTAB {
			app.SetFocus(searchInput)
			return nil
		}
		return event
	})

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(searchInput, 3, 0, true).
		AddItem(list, 0, 1, false).
		AddItem(statusBar, 1, 0, false)

	app.SetRoot(layout, true).SetFocus(searchInput)
}
