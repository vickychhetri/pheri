// ui/browser.go
package ui

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"mysql-tui/dbs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var dataTable *tview.Table
var allTables []string
var mainFlex *tview.Flex
var fileNameInput *tview.InputField

func filterTableList(
	search string,
	allTables []string,
	list *tview.List,
	queryBox *tview.TextArea,
	dataTable *tview.Table,
	app *tview.Application,
	db *sql.DB,
) {
	list.Clear()
	search = strings.ToLower(search)
	for _, tableName := range allTables {
		if strings.Contains(strings.ToLower(tableName), search) {
			// Closure-safe name
			name := tableName
			list.AddItem(name, "", 0, func() {
				query := "SELECT * FROM " + name + " LIMIT 100"
				queryBox.SetText(query, true)
				ExecuteQuery(app, db, query, dataTable)
				app.SetFocus(dataTable)
			})
		}
	}
}

func UseDatabase(app *tview.Application, db *sql.DB, dbName string) {
	runIcon := "\n➢ Run\n"
	saveIcon := "\n〄 Save\n"
	loadIcon := "\n⌘ Load\n"
	exitIcon := "\n ✘ Exit\n"

	// Use selected DB
	_, err := db.Exec("USE " + dbName)
	if err != nil {
		modal := tview.NewModal().
			SetText("Failed to use DB: " + err.Error()).
			AddButtons([]string{"Back"}).
			SetDoneFunc(func(i int, label string) {
				ShowDatabaseList(app, db)
			})
		app.SetRoot(modal, true)
		return
	}

	// LEFT: Table list (using tview.List)
	tableList := tview.NewList()
	tableList.SetBorder(true).SetTitle("Tables").SetTitleAlign(tview.AlignLeft).
		SetBorderColor(tcell.ColorWhite)
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		tableList.AddItem("Error: "+err.Error(), "", 0, nil)
	} else {
		defer rows.Close()
		var tableName string
		// Define queryBox and dataText outside the callback functions so they are in the scope
		var queryBox *tview.TextArea
		var dataTable *tview.Table

		for rows.Next() {
			rows.Scan(&tableName)
			currentTable := tableName
			allTables = append(allTables, tableName)
			tableList.AddItem(currentTable, "", 0, func() {
				// Handle table selection logic here
				if queryBox != nil {
					query := "SELECT * FROM " + currentTable + " LIMIT 100"
					queryBox.SetText(query, true)
					ExecuteQuery(app, db, query, dataTable)
					app.SetFocus(dataTable)
				}
			})
		}

		// Initialize queryBox and dataText outside of the callback scope

		runButton := tview.NewButton(runIcon).
			SetSelectedFunc(func() {
				query := queryBox.GetText()
				err := ExecuteQuery(app, db, query, dataTable)
				if err != nil {
					modal := tview.NewModal().
						SetText("Failed to execute query: " + err.Error()).
						AddButtons([]string{"OK"}).
						SetDoneFunc(func(buttonIndex int, buttonLabel string) {
							layout := CreateLayoutWithFooter(mainFlex)
							app.SetRoot(layout, true)
						})
					app.SetRoot(modal, true)
					return
				}
				app.SetFocus(dataTable)
			})

		buttonBox := tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(nil, 2, 0, false).      // Left padding
			AddItem(runButton, 0, 1, true). // Button
			AddItem(nil, 2, 0, false)       // Right padding

		// queryBox = tview.NewInputField().
		// 	SetLabel("SQL> ").
		// 	SetFieldWidth(100).
		// 	SetDoneFunc(func(key tcell.Key) {
		// 		if key == tcell.KeyTab || key == tcell.KeyEnter {
		// 			app.SetFocus(runButton)
		// 		}
		// 	})

		queryBox = tview.NewTextArea()
		queryBox.
			SetBorder(true).
			SetTitle("SQL Editor")
		queryBox.SetTitleAlign(tview.AlignLeft).
			SetBorderColor(tcell.ColorWhite)

		// queryBox.SetSize(5, 0)

		queryBox.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			switch event.Key() {
			case tcell.KeyTab:
				app.SetFocus(runButton)
				return nil
			case tcell.KeyEscape:
				layout := CreateLayoutWithFooter(mainFlex)
				app.SetRoot(layout, true)
				app.SetFocus(tableList)
				return nil

			case tcell.KeyF11:
				app.SetRoot(queryBox, true)
			}

			return event
		})

		button1 := tview.NewButton(saveIcon)
		button1.
			SetSelectedFunc(func() {
				fileNameInput = tview.NewInputField().
					SetLabel("File Name: ").
					SetFieldWidth(20).
					SetFieldBackgroundColor(tcell.ColorBlack).
					SetFieldTextColor(tcell.ColorWhite).
					SetPlaceholder("query.sql").
					SetDoneFunc(func(key tcell.Key) {
						if key == tcell.KeyEnter {
							fileName := fileNameInput.GetText()
							query := queryBox.GetText()

							if fileName == "" {
								fileName = "query.sql"
							}
							err := os.WriteFile(fileName, []byte(query), 0644)
							if err != nil {
								modal := tview.NewModal().
									SetText("Failed to save file: " + err.Error()).
									AddButtons([]string{"OK"}).
									SetDoneFunc(func(buttonIndex int, buttonLabel string) {
										app.SetRoot(queryBox, true)
									})
								app.SetRoot(modal, true)
								return
							}
							modal := tview.NewModal().
								SetText("Query saved to " + fileName).
								AddButtons([]string{"OK"}).
								SetDoneFunc(func(buttonIndex int, buttonLabel string) {
									layout := CreateLayoutWithFooter(mainFlex)
									app.SetRoot(layout, true)
								})
							app.SetRoot(modal, true)
						}
					})

				flexSaveFilenName := tview.NewFlex().
					AddItem(fileNameInput, 0, 1, true)
				flexSaveFilenName.SetDirection(tview.FlexRow).
					SetTitle("Save Query").
					SetTitleAlign(tview.AlignLeft).
					SetBorder(true).
					SetBorderColor(tcell.ColorWhite)
				flexSaveFilenName.SetBorderPadding(0, 0, 1, 1)

				flexSaveFilenName.SetBorder(true).
					SetTitle("Save Query").
					SetTitleAlign(tview.AlignCenter).
					SetBorderColor(tcell.ColorWhite)

				app.SetRoot(flexSaveFilenName, true).SetFocus(fileNameInput)
			})

		saveButtonBox := tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(nil, 2, 0, false).    // Left padding
			AddItem(button1, 0, 1, true). // Button
			AddItem(nil, 2, 0, false)     // Right padding

		button1.SetBorderPadding(0, 0, 1, 1)

		button2 := tview.NewButton(loadIcon).SetSelectedFunc(func() {
		})

		button2.SetBorderPadding(0, 0, 1, 1)

		loadButtonBox := tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(nil, 2, 0, false).    // Left padding
			AddItem(button2, 0, 1, true). // Button
			AddItem(nil, 2, 0, false)     // Right padding

		exitButton := tview.NewButton(exitIcon).SetSelectedFunc(func() {
			app.Stop()
		})

		exitButton.SetBorderPadding(0, 0, 5, 5)

		exitButtonBox := tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(nil, 1, 0, false).       // Left padding
			AddItem(exitButton, 0, 1, true). // Button
			AddItem(nil, 1, 0, false)        // Right padding

		// Create a dropdown.
		// dropdown := tview.NewDropDown().
		// 	SetLabel("Select Option: ").
		// 	SetOptions([]string{"Export", "About", "Help"}, func(option string, index int) {
		// 		_ = index // Ignore the index for now
		// 		//fmt.Printf("Dropdown selected: %s\n", option)
		// 	})

		runButton.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape {
				app.SetFocus(queryBox)
				return nil
			}
			if event.Key() == tcell.KeyTab {
				app.SetFocus(button1)
				return nil
			}
			return event
		})

		button1.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape {
				app.SetFocus(queryBox)
				return nil
			}
			if event.Key() == tcell.KeyTab {
				app.SetFocus(button2)
				return nil
			}
			return event
		})
		button2.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape {
				app.SetFocus(queryBox)
				return nil
			}
			if event.Key() == tcell.KeyTab {
				app.SetFocus(exitButton)
				return nil
			}
			if event.Key() == tcell.KeyEnter {
				// Show a suggestion list of files
				startDir, err := os.Getwd()
				if err != nil {
					startDir = "."
				}
				layout := CreateLayoutWithFooter(mainFlex)
				fileBrowser(button2, startDir, app, queryBox, layout)
			}

			return event
		})

		exitButton.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape {
				app.SetFocus(queryBox)
				return nil
			}
			if event.Key() == tcell.KeyTab {
				app.SetFocus(dataTable)
				return nil
			}
			return event
		})

		// dropdown.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// 	if event.Key() == tcell.KeyEscape {
		// 		app.SetFocus(queryBox)
		// 		return nil

		// 	}
		// 	if event.Key() == tcell.KeyTab {
		// 		app.SetFocus(runButton)
		// 		return nil
		// 	}
		// 	return event
		// })

		queryPanel := tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(queryBox, 4, 1, true).
			AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
				AddItem(buttonBox, 0, 1, false).
				AddItem(saveButtonBox, 0, 1, false).
				AddItem(loadButtonBox, 0, 1, false).
				AddItem(exitButtonBox, 0, 1, false), 1, 0, false)

		// queryPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		// 	AddItem(queryBox, 6, 1, true).
		// 	AddItem(runButton, 1, 0, false)

		// BOTTOM: Query Result (dataText)
		dataTable = tview.NewTable()
		dataTable.SetBorders(true).
			SetSelectable(true, false). // Allow vertical navigation only
			SetFixed(1, 0).             // Fix the first row (header)
			SetTitle("Result").
			SetBorder(true)
		dataTable.SetBorders(true).SetBorderColor(tcell.ColorWhite)

		dataTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyTab {
				app.SetFocus(tableList)
				return nil
			}
			if event.Key() == tcell.KeyEscape {
				app.SetFocus(tableList)
				layout := CreateLayoutWithFooter(mainFlex)
				app.SetRoot(layout, true)
				return nil
			}

			if event.Key() == tcell.KeyF11 {
				app.SetRoot(dataTable, true)
			}

			return event
		})

		searchInput := tview.NewInputField()
		searchInput.SetFieldBackgroundColor(tcell.ColorBlack).
			SetLabel("Search: ").
			SetFieldWidth(30)
		searchInput.
			SetChangedFunc(func(text string) {
				filterTableList(text, allTables, tableList, queryBox, dataTable, app, db)
			})

		searchInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyTab {
				app.SetFocus(queryBox)
				return nil
			}
			if event.Key() == tcell.KeyEscape {
				conn, err := dbs.Connect(user, pass, host, port)
				if err != nil {
					modal := tview.NewModal().
						SetText("Connection failed: " + err.Error()).
						AddButtons([]string{"OK"}).
						SetDoneFunc(func(buttonIndex int, buttonLabel string) {
							app.SetRoot(queryBox, true)
						})
					app.SetRoot(modal, true)
					return nil
				}

				ShowDatabaseList(app, conn)
				return nil
			}
			return event
		})

		tableList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyTab {
				app.SetFocus(queryBox)
				return nil
			}
			if event.Key() == tcell.KeyEscape {
				app.SetFocus(searchInput)
				return nil
			}

			return event
		})

		// Combine middle part (query + result)
		// Left panel: Search + Table List
		leftPanel := tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(searchInput, 1, 0, false). // small fixed height for search input
			AddItem(tableList, 0, 1, true)     // fills remaining space

		// Center panel: Query + Data Table
		centerPanel := tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(queryPanel, 6, 1, true).
			AddItem(dataTable, 0, 3, false)

		// Main layout
		mainFlex = tview.NewFlex().
			AddItem(leftPanel, 0, 1, true).   // use leftPanel instead of just tableList
			AddItem(centerPanel, 0, 5, false) // center content

		// // Final layout
		// mainFlex := tview.NewFlex().
		// 	AddItem(tableList, 30, 1, true). // Left column
		// 	AddItem(middle, 0, 4, false)     // Right section

		// Set the root view with tableList, middle area, and queryBox in focus
		layout := CreateLayoutWithFooter(mainFlex)
		app.SetRoot(layout, true)
	}
}

func ExecuteQuery(app *tview.Application, db *sql.DB, query string, table *tview.Table) error {
	rows, err := db.Query(query)
	if err != nil {
		table.Clear()
		table.SetCell(0, 0, tview.NewTableCell("Error: "+err.Error()).SetTextColor(tcell.ColorRed))
		return err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		table.Clear()
		table.SetCell(0, 0, tview.NewTableCell("Error: "+err.Error()).SetTextColor(tcell.ColorRed))
		return err
	}

	table.Clear()

	// Show column headers
	for i, col := range columns {
		table.SetCell(0, i, tview.NewTableCell(fmt.Sprintf("[::b]%s", col)).SetAlign(tview.AlignCenter))
	}

	values := make([]sql.RawBytes, len(columns))
	scanArgs := make([]interface{}, len(values))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	rowIndex := 1
	for rows.Next() {
		err := rows.Scan(scanArgs...)
		if err != nil {
			continue
		}

		for i, col := range values {
			table.SetCell(rowIndex, i, tview.NewTableCell(string(col)).SetAlign(tview.AlignLeft))
		}
		rowIndex++
	}
	return nil
}

func listFilesWithExtensions(dir string, exts []string) ([]string, error) {
	var matched []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			for _, ext := range exts {
				if strings.HasSuffix(d.Name(), ext) {
					matched = append(matched, path)
				}
			}
		}
		return nil
	})
	return matched, err
}

// Browse files in a directory
// This function will be called when the user clicks the "Load" button
// It will show a list of files with .sql and .go extensions
// When a file is selected, its content will be loaded into the queryBox
// and the user will be returned to the main screen
// The function will also handle the case when the user clicks "Cancel" or "Back"
// It will return to the main screen without loading any file
func fileBrowser(button2 *tview.Button, currentDir string, app *tview.Application, queryBox *tview.TextArea, returnTo tview.Primitive) {
	list := tview.NewList().
		ShowSecondaryText(false)

	// ".." to go up a directory
	if currentDir != "/" {
		parent := filepath.Dir(currentDir)
		list.AddItem("..", "Go up", 0, func() {
			fileBrowser(button2, parent, app, queryBox, returnTo)
		})
	}

	// Read directory
	entries, err := os.ReadDir(currentDir)
	if err != nil {
		log.Printf("Failed to read directory: %v", err)
		app.SetRoot(returnTo, true)
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(currentDir, name)

		if entry.IsDir() {
			list.AddItem("[::b][DIR] "+name, "", 0, func() {
				fileBrowser(button2, fullPath, app, queryBox, returnTo)
			})
		} else if strings.HasSuffix(name, ".sql") || strings.HasSuffix(name, ".go") {
			list.AddItem(name, "", 0, func() {
				content, err := os.ReadFile(fullPath)
				if err != nil {
					log.Printf("Failed to read file: %v", err)
				} else {
					queryBox.SetText(string(content), true)
					app.SetFocus(queryBox)
				}
				app.SetRoot(returnTo, true)
			})
		}
	}

	list.SetDoneFunc(func() {
		app.SetRoot(returnTo, true)
		app.SetFocus(button2)
	})

	layout := CreateLayoutWithFooter(list)
	app.SetRoot(tview.NewFlex().AddItem(layout, 0, 1, true), true)
	app.SetFocus(layout)
}

// func ExecuteQuery(app *tview.Application, db *sql.DB, query string, output *tview.TextView) {
// 	rows, err := db.Query(query)
// 	if err != nil {
// 		output.SetText("[red]Error: " + err.Error())
// 		return
// 	}
// 	defer rows.Close()

// 	columns, _ := rows.Columns()
// 	values := make([]sql.RawBytes, len(columns))
// 	scanArgs := make([]interface{}, len(values))
// 	for i := range values {
// 		scanArgs[i] = &values[i]
// 	}

// 	var result strings.Builder
// 	result.WriteString("[yellow]" + strings.Join(columns, " | ") + "\n")

// 	for rows.Next() {
// 		err = rows.Scan(scanArgs...)
// 		if err != nil {
// 			output.SetText("[red]Scan error: " + err.Error())
// 			return
// 		}
// 		for _, val := range values {
// 			result.WriteString(string(val) + " | ")
// 		}
// 		result.WriteString("\n")
// 	}

// 	output.SetText(result.String())
// }
