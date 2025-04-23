// ui/connect.go
package ui

import (
	"database/sql"
	"mysql-tui/db"

	"github.com/rivo/tview"
)

func ShowConnectionForm(app *tview.Application) {
	var form *tview.Form

	form = tview.NewForm().
		AddInputField("Host", "127.0.0.1", 20, nil, nil).
		AddInputField("Port", "3306", 6, nil, nil).
		AddInputField("User", "root", 20, nil, nil).
		AddPasswordField("Password", "", 20, '*', nil).
		AddButton("Connect", func() {
			host := form.GetFormItemByLabel("Host").(*tview.InputField).GetText()
			port := form.GetFormItemByLabel("Port").(*tview.InputField).GetText()
			user := form.GetFormItemByLabel("User").(*tview.InputField).GetText()
			pass := form.GetFormItemByLabel("Password").(*tview.InputField).GetText()

			conn, err := db.Connect(user, pass, host, port)
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
		AddButton("Quit", func() {
			app.Stop()
		})

	form.SetBorder(true).SetTitle("MySQL Connection").SetTitleAlign(tview.AlignCenter)
	app.SetRoot(form, true)
}

func ShowDatabaseList(app *tview.Application, db *sql.DB) {
	list := tview.NewList()

	rows, err := db.Query("SHOW DATABASES")
	if err != nil {
		list.AddItem("Error: "+err.Error(), "", 0, nil)
	} else {
		defer rows.Close()
		var dbName string
		for rows.Next() {
			rows.Scan(&dbName)
			// Add each DB with a handler that opens its tables view
			list.AddItem(dbName, "", 0, func(name string) func() {
				return func() {
					UseDatabase(app, db, name)
				}
			}(dbName))
		}
	}

	list.AddItem("Back", "Return to connection screen", 'b', func() {
		ShowConnectionForm(app)
	})

	app.SetRoot(list, true)
}
