// ui/connect.go
package ui

import (
	"database/sql"
	"fmt"
	"log"
	"mysql-tui/dbs"
	"mysql-tui/phhistory"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var user, pass, host, port string
var User, Pass, Host, Port string

func ShowConnectionForm(app *tview.Application, userArg, passArg, hostArg, portArg string) {
	if userArg != "" && hostArg != "" && portArg != "" {
		User = userArg
		Pass = passArg
		Host = hostArg
		Port = portArg
		conn, err := dbs.Connect(userArg, passArg, hostArg, portArg)
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
[white::-]  ⚡ MySQL Terminal Client ⚡`
	asciiBanner.SetText(asciiArt)
	asciiBanner.SetBackgroundColor(tcell.ColorBlack)

	profiles, _ := phhistory.LoadSavedConnections()

	form := tview.NewForm()

	profileOptions := []string{"-- Custom / New Connection --"}
	for _, p := range profiles {
		profileOptions = append(profileOptions, fmt.Sprintf("🔑 %s@%s:%s", p.User, p.Host, p.Port))
	}

	form.AddDropDown("Quick Saved Profile", profileOptions, 0, func(optionText string, optionIndex int) {
		if optionIndex > 0 && optionIndex-1 < len(profiles) {
			p := profiles[optionIndex-1]
			form.GetFormItem(1).(*tview.InputField).SetText(p.Host)
			form.GetFormItem(2).(*tview.InputField).SetText(p.Port)
			form.GetFormItem(3).(*tview.InputField).SetText(p.User)
			form.GetFormItem(4).(*tview.InputField).SetText(p.Pass)
		}
	})

	form.
		AddInputField("Host", "127.0.0.1", 26, nil, nil).
		AddInputField("Port", "3306", 8, nil, nil).
		AddInputField("User", "root", 26, nil, nil).
		AddPasswordField("Password", "", 26, '*', nil).
		AddCheckbox("Remember Credentials", true, nil)

	connectAction := func() {
		host = form.GetFormItem(1).(*tview.InputField).GetText()
		port = form.GetFormItem(2).(*tview.InputField).GetText()
		user = form.GetFormItem(3).(*tview.InputField).GetText()
		pass = form.GetFormItem(4).(*tview.InputField).GetText()
		remember := form.GetFormItem(5).(*tview.Checkbox).IsChecked()

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

		if remember {
			_ = phhistory.SaveConnectionProfile(host, port, user, pass)
		}

		ShowDatabaseList(app, conn)
	}

	form.
		AddButton("[lime::b] ▶ CONNECT ", connectAction).
		AddButton("[yellow::b] ⟳ CLEAR ", func() {
			for i := 1; i <= 4; i++ {
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
		SetBorderPadding(1, 1, 3, 3)

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
		AddItem(asciiBanner, 7, 0, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(form, 64, 1, true).
			AddItem(nil, 0, 1, false), 18, 1, true).
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

	statusBar := tview.NewTextView().
		SetText(" [cyan]↑/↓ / TAB[-] Navigate List • [lime]ENTER[-] Open Database • [yellow]ESC[-] Back to Login").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	searchInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyDown {
			app.SetFocus(list)
			return nil
		}
		if event.Key() == tcell.KeyEnter {
			if list.GetItemCount() > 0 {
				cur := list.GetCurrentItem()
				if cur >= 0 {
					mainText, _ := list.GetItemText(cur)
					if strings.Contains(mainText, "Back to Login") {
						ShowConnectionForm(app, user, pass, host, port)
					} else {
						for _, dbName := range dbNames {
							if strings.Contains(mainText, dbName) {
								UseDatabase(app, db, dbName)
								return nil
							}
						}
					}
				}
			}
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			ShowConnectionForm(app, user, pass, host, port)
			return nil
		}
		return event
	})

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyBacktab {
			app.SetFocus(searchInput)
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			ShowConnectionForm(app, user, pass, host, port)
			return nil
		}
		return event
	})

	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(searchInput, 3, 0, true).
		AddItem(list, 0, 1, false).
		AddItem(statusBar, 1, 0, false)

	app.SetRoot(layout, true).SetFocus(searchInput)
}
