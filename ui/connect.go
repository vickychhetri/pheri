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
var User, Pass, Host, Port string

func ShowConnectionForm(app *tview.Application, user, pass, host, port string) {
	if user != "" && host != "" && port != "" {
		User = user
		Pass = pass
		Host = host
		Port = port
		conn, err := dbs.Connect(user, pass, host, port)
		if err != nil {
			log.Printf("Error in db Connection: %v", err)
			app.Stop()
			return
		}
		ShowDatabaseList(app, conn)
		return
	}

	// ASCII Logo Banner
	asciiBanner := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	
	asciiArt := `[cyan::b]
 ██████╗ ██╗  ██╗███████╗██████╗ ██╗
 ██╔══██╗██║  ██║██╔════╝██╔══██╗██║
 ██████╔╝███████║█████╗  ██████╔╝██║
 ██╔═══╝ ██╔══██║██╔══╝  ██╔══██╗██║
 ██║     ██║  ██║███████╗██║  ██║██║
[white::-]  ⚡ Production-Grade MySQL Terminal Client ⚡`
	asciiBanner.SetText(asciiArt)
	asciiBanner.SetBackgroundColor(tcell.ColorBlack)

	// Connection Form
	form := tview.NewForm()
	form.
		AddInputField("Host", "127.0.0.1", 24, nil, nil).
		AddInputField("Port", "3306", 8, nil, nil).
		AddInputField("User", "root", 24, nil, nil).
		AddPasswordField("Password", "", 24, '*', nil)

	form.
		AddButton("[lime::b] ▶ CONNECT ", func() {
			host = form.GetFormItem(0).(*tview.InputField).GetText()
			port = form.GetFormItem(1).(*tview.InputField).GetText()
			user = form.GetFormItem(2).(*tview.InputField).GetText()
			pass = form.GetFormItem(3).(*tview.InputField).GetText()

			User = user
			Pass = pass
			Host = host
			Port = port

			phhistory.SetUser(user)
			phhistory.SetHost(host)
			phhistory.SetPort(port)

			conn, err := dbs.Connect(user, pass, host, port)
			if err != nil {
				modal := tview.NewModal().
					SetText("[red::b]✖ Connection Failed:\n\n[white]" + err.Error()).
					AddButtons([]string{"[green]Try Again"}).
					SetBackgroundColor(tcell.ColorDarkBlue).
					SetButtonBackgroundColor(tcell.ColorDarkCyan).
					SetDoneFunc(func(buttonIndex int, buttonLabel string) {
						app.SetRoot(form, true)
					})
				app.SetRoot(modal, true)
				return
			}

			ShowDatabaseList(app, conn)
		}).
		AddButton("[yellow::b] ⟳ CLEAR ", func() {
			for i := 0; i < 4; i++ {
				form.GetFormItem(i).(*tview.InputField).SetText("")
			}
		}).
		AddButton("[red::b] ✖ QUIT ", func() {
			app.Stop()
		})

	form.SetBorder(true).
		SetTitle(" 🔒 MySQL Server Authentication ").
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorDarkCyan).
		SetTitleColor(tcell.ColorAqua).
		SetBorderPadding(1, 1, 4, 4)

	form.SetBackgroundColor(tcell.ColorBlack)
	form.SetFieldBackgroundColor(tcell.Color234)
	form.SetFieldTextColor(tcell.ColorWhite)
	form.SetLabelColor(tcell.ColorAqua)
	form.SetButtonBackgroundColor(tcell.Color236)
	form.SetButtonTextColor(tcell.ColorWhite)

	// Status Footer
	footer := tview.NewTextView().
		SetText("[aqua]TAB[-] Move Field | [lime]ENTER[-] Connect | [yellow]ESC[-] Quit").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetBackgroundColor(tcell.ColorBlack)

	// Layout Flex
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(asciiBanner, 8, 0, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(form, 54, 1, true).
			AddItem(nil, 0, 1, false), 15, 1, true).
		AddItem(footer, 1, 0, false)

	app.SetRoot(flex, true).SetFocus(form)
}

func ShowDatabaseList(app *tview.Application, db *sql.DB) {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true).
		SetTitle(" 🗄️ Available Databases ").
		SetTitleAlign(tview.AlignLeft).
		SetBorderColor(tcell.ColorDarkCyan)

	searchInput := tview.NewInputField()
	searchInput.
		SetLabel("🔍 Filter: ").
		SetFieldWidth(30).
		SetPlaceholder("Type database name...").
		SetBorder(true).
		SetBorderColor(tcell.ColorYellow)

	statusBar := tview.NewTextView().
		SetText("[cyan]↑/↓[-] Navigate • [lime]ENTER[-] Select Database • [yellow]ESC[-] Back to Login").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	var dbNames []string
	filtered := func(filter string) {
		list.Clear()
		for _, name := range dbNames {
			if strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
				dbCopy := name
				list.AddItem("  🗂️  [white::b]"+dbCopy+"[::-]", "Press Enter to open database", 0, func() {
					UseDatabase(app, db, dbCopy)
				})
			}
		}
		list.AddItem("  ⬅️  [yellow::b]Back to Login[::-]", "Return to connection screen", 'b', func() {
			ShowConnectionForm(app, user, pass, host, port)
		})
	}

	rows, err := db.Query("SHOW DATABASES")
	if err != nil {
		list.AddItem("❌ Error: "+err.Error(), "", 0, nil)
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

	mainPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(searchInput, 3, 0, false).
		AddItem(list, 0, 1, true).
		AddItem(statusBar, 1, 0, false)

	layout := CreateLayoutWithFooter(app, mainPanel)
	app.SetRoot(layout, true).SetFocus(list)
}
