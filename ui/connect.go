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
var ActiveEnv string = "DEV"
var ActiveReadOnly bool = false

func ShowConnectionForm(app *tview.Application, userArg, passArg, hostArg, portArg string) {
	ActiveScreen = "login"
	ActiveLoginDB = nil
	ActiveEnv = "DEV"
	ActiveReadOnly = false

	if userArg != "" && hostArg != "" && portArg != "" {
		User = userArg
		Pass = passArg
		Host = hostArg
		Port = portArg
		conn, err := dbs.Connect(userArg, passArg, hostArg, portArg)
		if err != nil {
			errInfo := dbs.CategorizeConnectionError(err)
			log.Printf("Connection error (%s): %s - %s", errInfo.Category, errInfo.Title, errInfo.Message)
			if errInfo.Hint != "" {
				log.Printf("Remediation: %s", errInfo.Hint)
			}
			app.Stop()
			return
		}
		ShowDatabaseList(app, conn)
		return
	}

	// 1. Top Brand Header Bar
	header := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	header.SetText(" [white::b]PHERI DATABASE MANAGER[-::-]  [gray]v3.2.0[-]  [gray]│[-]  [aqua::b]Terminal MySQL Client[-::-]")
	header.SetBackgroundColor(CurrentTheme.HeaderBg)

	// 2. Left Panel: Saved Connection Profiles List
	profileList := tview.NewList().ShowSecondaryText(true)
	StyleBox(profileList.Box, "Saved Connection Profiles", true)
	profileList.SetBackgroundColor(CurrentTheme.SurfaceBg)
	StyleList(profileList)

	// 3. Right Panel: Connection Details Form
	form := tview.NewForm()
	StyleBox(form.Box, "Connection Details & Authentication", false)
	form.SetBorderPadding(0, 0, 2, 2)
	form.SetBackgroundColor(CurrentTheme.SurfaceBg)
	form.SetFieldBackgroundColor(tcell.ColorReset)
	form.SetFieldTextColor(CurrentTheme.TextPrimary)
	form.SetLabelColor(CurrentTheme.AccentPrimary)
	form.SetButtonBackgroundColor(CurrentTheme.SelectedBg)
	form.SetButtonTextColor(CurrentTheme.SelectedFg)

	form.
		AddInputField("Profile Name", "", 30, nil, nil).
		AddInputField("Host / IP", "127.0.0.1", 30, nil, nil).
		AddInputField("Port", "3306", 8, nil, nil).
		AddInputField("User", "root", 30, nil, nil).
		AddPasswordField("Password", "", 30, '*', nil).
		AddDropDown("Environment Tag", []string{"DEV (Development)", "STAGING (Test/Staging)", "PROD (Production)"}, 0, nil).
		AddCheckbox("Read-Only Mode (Block Write Queries)", false, nil).
		AddCheckbox("Save Credentials (AES-256-GCM)", true, nil)

	// 4. Live Feedback / Test Status Box
	testStatusView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	testStatusView.SetBackgroundColor(CurrentTheme.HeaderBg)
	testStatusView.SetText(" [gray]Ready. Select a profile or enter connection credentials.[-]")

	var profiles []phhistory.ConnectionProfile
	var mainLayout *tview.Flex
	var connectAction func()
	var testConnectionAction func()
	var reloadProfiles func(targetName string)

	populateFormWithProfile := func(p phhistory.ConnectionProfile) {
		form.GetFormItem(0).(*tview.InputField).SetText(p.Name)
		form.GetFormItem(1).(*tview.InputField).SetText(p.Host)
		form.GetFormItem(2).(*tview.InputField).SetText(p.Port)
		form.GetFormItem(3).(*tview.InputField).SetText(p.User)
		form.GetFormItem(4).(*tview.InputField).SetText(p.Pass)

		envIdx := 0
		if p.Environment == "STAGING" {
			envIdx = 1
		} else if p.Environment == "PROD" {
			envIdx = 2
		}
		form.GetFormItem(5).(*tview.DropDown).SetCurrentOption(envIdx)
		form.GetFormItem(6).(*tview.Checkbox).SetChecked(p.ReadOnly)
		testStatusView.SetText(fmt.Sprintf(" [gray]Loaded profile '%s'. Press [aqua]Connect[gray] or [yellow]Test Connection[gray].[-]", p.Name))
	}

	clearForm := func() {
		form.GetFormItem(0).(*tview.InputField).SetText("")
		form.GetFormItem(1).(*tview.InputField).SetText("127.0.0.1")
		form.GetFormItem(2).(*tview.InputField).SetText("3306")
		form.GetFormItem(3).(*tview.InputField).SetText("root")
		form.GetFormItem(4).(*tview.InputField).SetText("")
		form.GetFormItem(5).(*tview.DropDown).SetCurrentOption(0)
		form.GetFormItem(6).(*tview.Checkbox).SetChecked(false)
		testStatusView.SetText(" [gray]Configuring new connection. Enter credentials and connect.[-]")
	}

	reloadProfiles = func(targetName string) {
		profileList.Clear()
		var err error
		profiles, err = phhistory.LoadSavedConnections()
		if err != nil {
			profiles = []phhistory.ConnectionProfile{}
		}

		profileList.AddItem("  + [ New Connection ]", "    Configure clean credentials", 'n', func() {
			clearForm()
			app.SetFocus(form)
		})

		selectedIdx := 0
		for idx, p := range profiles {
			pCopy := p
			envBadge := fmt.Sprintf("[green::b]%s[-::-]", tview.Escape("[DEV]"))
			if pCopy.Environment == "STAGING" {
				envBadge = fmt.Sprintf("[yellow::b]%s[-::-]", tview.Escape("[STAGE]"))
			} else if pCopy.Environment == "PROD" {
				envBadge = fmt.Sprintf("[red::b]%s[-::-]", tview.Escape("[PROD]"))
			}

			displayName := pCopy.Name
			subText := fmt.Sprintf("    %s@%s:%s", pCopy.User, pCopy.Host, pCopy.Port)
			if displayName == "" {
				displayName = fmt.Sprintf("%s@%s:%s", pCopy.User, pCopy.Host, pCopy.Port)
				subText = fmt.Sprintf("    Port: %s │ User: %s", pCopy.Port, pCopy.User)
			} else if displayName == fmt.Sprintf("%s@%s:%s", pCopy.User, pCopy.Host, pCopy.Port) {
				subText = fmt.Sprintf("    Port: %s │ User: %s", pCopy.Port, pCopy.User)
			}

			mainText := fmt.Sprintf("  %s %s", envBadge, displayName)

			profileList.AddItem(mainText, subText, 0, func() {
				populateFormWithProfile(pCopy)
				connectAction()
			})

			if targetName != "" && (pCopy.Name == targetName || pCopy.ID == targetName) {
				selectedIdx = idx + 1
			}
		}

		if selectedIdx > 0 && selectedIdx < profileList.GetItemCount() {
			profileList.SetCurrentItem(selectedIdx)
		}
	}

	profileList.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		if index == 0 {
			clearForm()
		} else if index-1 < len(profiles) {
			populateFormWithProfile(profiles[index-1])
		}
	})

	connectAction = func() {
		profileName := form.GetFormItem(0).(*tview.InputField).GetText()
		host = form.GetFormItem(1).(*tview.InputField).GetText()
		port = form.GetFormItem(2).(*tview.InputField).GetText()
		user = form.GetFormItem(3).(*tview.InputField).GetText()
		pass = form.GetFormItem(4).(*tview.InputField).GetText()
		_, envOpt := form.GetFormItem(5).(*tview.DropDown).GetCurrentOption()
		readOnly := form.GetFormItem(6).(*tview.Checkbox).IsChecked()
		remember := form.GetFormItem(7).(*tview.Checkbox).IsChecked()

		envTag := "DEV"
		if strings.Contains(envOpt, "PROD") {
			envTag = "PROD"
		} else if strings.Contains(envOpt, "STAGING") {
			envTag = "STAGING"
		}

		ActiveEnv = envTag
		ActiveReadOnly = readOnly

		User = user
		Pass = pass
		Host = host
		Port = port

		phhistory.SetUser(user)
		phhistory.SetHost(host)
		phhistory.SetPort(port)

		testStatusView.SetText(fmt.Sprintf(" [yellow::b]Connecting to %s@%s:%s...[-::-]", user, host, port))

		go func() {
			conn, err := dbs.Connect(user, pass, host, port)
			if app != nil {
				app.QueueUpdateDraw(func() {
					if err != nil {
						errInfo := dbs.CategorizeConnectionError(err)
						errBody := fmt.Sprintf("[red::b]Connection Failed: %s[::-]\n\n[white]%s\n\n[yellow]Suggestion:[::-] %s",
							errInfo.Title,
							errInfo.Message,
							errInfo.Hint,
						)
						if errInfo.RawErr != nil {
							errBody += fmt.Sprintf("\n\n[gray::d]Details: %s[::-]", errInfo.RawErr.Error())
						}
						testStatusView.SetText(fmt.Sprintf(" [red::b]Connection Failed: %s[::-] [white]%s[-]", errInfo.Title, errInfo.Message))

						modal := tview.NewModal().
							SetText(errBody).
							AddButtons([]string{"Try Again"}).
							SetBackgroundColor(tcell.ColorDarkBlue).
							SetButtonBackgroundColor(tcell.ColorDarkCyan).
							SetDoneFunc(func(buttonIndex int, buttonLabel string) {
								if mainLayout != nil {
									app.SetRoot(mainLayout, true).SetFocus(form)
								} else {
									app.SetRoot(form, true)
								}
							})
						app.SetRoot(modal, true)
						return
					}

					if remember {
						_ = phhistory.SaveNamedConnectionProfile(profileName, host, port, user, pass, envTag, readOnly)
					}

					ShowDatabaseList(app, conn)
				})
			}
		}()
	}

	testConnectionAction = func() {
		h := form.GetFormItem(1).(*tview.InputField).GetText()
		p := form.GetFormItem(2).(*tview.InputField).GetText()
		u := form.GetFormItem(3).(*tview.InputField).GetText()
		pw := form.GetFormItem(4).(*tview.InputField).GetText()

		testStatusView.SetText(fmt.Sprintf(" [yellow::b]Testing connection to %s:%s...[-::-]", h, p))

		go func() {
			latency, version, err := dbs.TestConnection(u, pw, h, p)
			if app != nil {
				app.QueueUpdateDraw(func() {
					if err != nil {
						errInfo := dbs.CategorizeConnectionError(err)
						hintStr := ""
						if errInfo.Hint != "" {
							hintStr = fmt.Sprintf(" (Hint: %s)", errInfo.Hint)
						}
						testStatusView.SetText(fmt.Sprintf(" [red::b]Connection Failed: %s[::-] [white]%s%s[-]", errInfo.Title, errInfo.Message, hintStr))
					} else {
						verStr := ""
						if version != "" {
							verStr = fmt.Sprintf(" │ MySQL %s", version)
						}
						testStatusView.SetText(fmt.Sprintf(" [lime::b]Connection Successful![-::-] [white]Latency: %dms%s[-]", latency.Milliseconds(), verStr))
					}
				})
			}
		}()
	}

	saveProfileAction := func() {
		name := form.GetFormItem(0).(*tview.InputField).GetText()
		h := form.GetFormItem(1).(*tview.InputField).GetText()
		p := form.GetFormItem(2).(*tview.InputField).GetText()
		u := form.GetFormItem(3).(*tview.InputField).GetText()
		pw := form.GetFormItem(4).(*tview.InputField).GetText()
		_, envOpt := form.GetFormItem(5).(*tview.DropDown).GetCurrentOption()
		readOnly := form.GetFormItem(6).(*tview.Checkbox).IsChecked()

		envTag := "DEV"
		if strings.Contains(envOpt, "PROD") {
			envTag = "PROD"
		} else if strings.Contains(envOpt, "STAGING") {
			envTag = "STAGING"
		}

		if err := phhistory.SaveNamedConnectionProfile(name, h, p, u, pw, envTag, readOnly); err != nil {
			testStatusView.SetText(fmt.Sprintf(" [red::b]Error saving profile: %v[-::-]", err))
		} else {
			testStatusView.SetText(" [lime::b]Profile saved successfully![-::-]")
			reloadProfiles(name)
		}
	}

	deleteProfileAction := func() {
		cur := profileList.GetCurrentItem()
		if cur > 0 && cur-1 < len(profiles) {
			target := profiles[cur-1]
			_ = phhistory.DeleteConnectionProfile(target.ID)
			testStatusView.SetText(fmt.Sprintf(" [yellow::b]Deleted profile '%s'[-::-]", target.Name))
			reloadProfiles("")
		} else {
			testStatusView.SetText(" [gray]Select a saved profile from the left list to delete.[-]")
		}
	}

	form.
		AddButton(" Connect ", connectAction).
		AddButton(" Test Ping ", testConnectionAction).
		AddButton(" Save Profile ", saveProfileAction).
		AddButton(" Theme (F12) ", func() { showThemePickerModal(app) }).
		AddButton(" Delete ", deleteProfileAction).
		AddButton(" Clear ", clearForm).
		AddButton(" Quit ", func() {
			app.Stop()
		})

	profileList.SetFocusFunc(func() {
		StyleBox(profileList.Box, "Saved Connection Profiles", true)
		StyleBox(form.Box, "Connection Details & Authentication", false)
	})
	form.SetFocusFunc(func() {
		StyleBox(profileList.Box, "Saved Connection Profiles", false)
		StyleBox(form.Box, "Connection Details & Authentication", true)
	})

	profileList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab, tcell.KeyRight:
			app.SetFocus(form)
			return nil
		case tcell.KeyRune:
			if event.Rune() == 'n' || event.Rune() == 'N' {
				profileList.SetCurrentItem(0)
				app.SetFocus(form)
				return nil
			}
			if event.Rune() == 'd' || event.Rune() == 'D' {
				deleteProfileAction()
				return nil
			}
			if event.Rune() == 't' || event.Rune() == 'T' {
				testConnectionAction()
				return nil
			}
		}
		return event
	})

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyBacktab {
			app.SetFocus(profileList)
			return nil
		}
		return event
	})

	reloadProfiles("")

	// 5. Status Footer
	footer := tview.NewTextView().
		SetText(" [white::b]PHERI[-::-] [gray]│[-] [aqua::b]Tab / Shift+Tab[-::-] Switch Panels  [gray]│[-] [aqua::b]↑/↓[-::-] Select Profile  [gray]│[-] [aqua::b]Enter[-::-] Connect  [gray]│[-] [aqua::b]t[-::-] Test Ping  [gray]│[-] [aqua::b]n[-::-] New  [gray]│[-] [aqua::b]F12[-::-] Theme  [gray]│[-] [aqua::b]F1[-::-] Help  [gray]│[-] [aqua::b]Esc[-::-] Quit").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetBackgroundColor(CurrentTheme.HeaderBg)

	rightFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(form, 0, 1, true).
		AddItem(testStatusView, 1, 0, false)

	splitFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(profileList, 38, 0, true).
		AddItem(rightFlex, 0, 1, false)

	mainLayout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 1, 0, false).
		AddItem(splitFlex, 0, 1, true).
		AddItem(footer, 1, 0, false)

	mainLayout.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyF1 {
			ShowComprehensiveHelpModal(app)
			return nil
		}
		if event.Key() == tcell.KeyF12 {
			showThemePickerModal(app)
			return nil
		}
		return event
	})

	app.SetRoot(mainLayout, true).SetFocus(profileList)
}

func ShowDatabaseList(app *tview.Application, db *sql.DB) {
	ActiveScreen = "dblist"
	ActiveLoginDB = db

	list := tview.NewList().ShowSecondaryText(false)
	StyleBox(list.Box, "Available Databases", true)
	list.SetBackgroundColor(CurrentTheme.SurfaceBg)
	StyleList(list)

	searchInput := tview.NewInputField()
	searchInput.
		SetLabel(" Filter: ").
		SetLabelColor(CurrentTheme.AccentPrimary).
		SetFieldWidth(30).
		SetPlaceholder("Type database name...").
		SetFieldBackgroundColor(CurrentTheme.SurfaceBg).
		SetFieldTextColor(CurrentTheme.TextPrimary)
	StyleBox(searchInput.Box, "Search", false)

	var dbNames []string
	backToLogin := func() {
		if db != nil {
			_ = db.Close()
		}
		User = ""
		Pass = ""
		Host = ""
		Port = ""
		user = ""
		pass = ""
		host = ""
		port = ""
		ShowConnectionForm(app, "", "", "", "")
	}

	filtered := func(filter string) {
		list.Clear()
		for _, name := range dbNames {
			if strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
				dbCopy := name
				list.AddItem("  "+dbCopy, "Press Enter to open database", 0, func() {
					UseDatabase(app, db, dbCopy)
				})
			}
		}
		list.AddItem("  [ Back to Login ]", "Return to connection screen", 'b', func() {
			backToLogin()
		})
	}

	rows, err := db.Query("SHOW DATABASES")
	if err != nil {
		list.AddItem("  Error: "+err.Error(), "", 0, nil)
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
		SetText(" [cyan]↑/↓ / TAB[-] Navigate List • [lime]ENTER[-] Open Database • [aqua]F12[-] Theme • [yellow]ESC[-] Back to Login").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetBackgroundColor(CurrentTheme.HeaderBg)

	searchInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyF12 {
			showThemePickerModal(app)
			return nil
		}
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
						backToLogin()
						return nil
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
			backToLogin()
			return nil
		}
		return event
	})

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyF12 {
			showThemePickerModal(app)
			return nil
		}
		if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyBacktab {
			app.SetFocus(searchInput)
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			backToLogin()
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
