// ui/connect.go
package ui

import (
	"database/sql"
	"log"
	"mysql-tui/dbs"

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
			AddButton("Quit", func() {
				app.Stop()
			})

		form.SetBorder(true).SetTitle("MySQL Connection").SetTitleAlign(tview.AlignCenter)
		layout := CreateLayoutWithFooter(app, form)
		app.SetRoot(layout, true).SetFocus(form)
	}

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
		ShowConnectionForm(app, user, pass, host, port)
	})

	layout := CreateLayoutWithFooter(app, list)
	app.SetRoot(layout, true)
}
