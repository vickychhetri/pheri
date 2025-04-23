// ui/browser.go
package ui

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var dataTable *tview.Table
var allTables []string

func filterTableList(
	search string,
	allTables []string,
	list *tview.List,
	queryBox *tview.InputField,
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
				queryBox.SetText(query)
				ExecuteQuery(app, db, query, dataTable)
				app.SetFocus(dataTable)
			})
		}
	}
}

func UseDatabase(app *tview.Application, db *sql.DB, dbName string) {
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
		var queryBox *tview.InputField
		var dataTable *tview.Table

		for rows.Next() {
			rows.Scan(&tableName)
			currentTable := tableName
			allTables = append(allTables, tableName)
			tableList.AddItem(currentTable, "", 0, func() {
				// Handle table selection logic here
				if queryBox != nil {
					query := "SELECT * FROM " + currentTable + " LIMIT 100"
					queryBox.SetText(query)
					ExecuteQuery(app, db, query, dataTable)
					app.SetFocus(dataTable)
				}
			})
		}

		// Initialize queryBox and dataText outside of the callback scope

		runButton := tview.NewButton("Run").
			SetSelectedFunc(func() {
				query := queryBox.GetText()
				ExecuteQuery(app, db, query, dataTable)
				app.SetFocus(dataTable) // 🔥 Move focus to table
			})

		queryBox = tview.NewInputField().
			SetLabel("SQL> ").
			SetFieldWidth(100).
			SetDoneFunc(func(key tcell.Key) {
				if key == tcell.KeyTab || key == tcell.KeyEnter {
					app.SetFocus(runButton)
				}
			})

		runButton.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape {
				app.SetFocus(queryBox)
				return nil
			}
			return event
		})

		queryPanel := tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(queryBox, 3, 1, true).
			AddItem(runButton, 1, 0, false)

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
				app.SetFocus(queryBox)
				return nil
			}

			if event.Key() == tcell.KeyEscape {
				app.SetFocus(tableList)
				return nil
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
				app.SetFocus(tableList)
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
			AddItem(queryPanel, 4, 1, true).
			AddItem(dataTable, 0, 3, false)

		// Main layout
		mainFlex := tview.NewFlex().
			AddItem(leftPanel, 0, 1, true).   // use leftPanel instead of just tableList
			AddItem(centerPanel, 0, 5, false) // center content

		// // Final layout
		// mainFlex := tview.NewFlex().
		// 	AddItem(tableList, 30, 1, true). // Left column
		// 	AddItem(middle, 0, 4, false)     // Right section

		// Set the root view with tableList, middle area, and queryBox in focus

		app.SetRoot(mainFlex, true)
	}
}

func ExecuteQuery(app *tview.Application, db *sql.DB, query string, table *tview.Table) {
	rows, err := db.Query(query)
	if err != nil {
		table.Clear()
		table.SetCell(0, 0, tview.NewTableCell("Error: "+err.Error()).SetTextColor(tcell.ColorRed))
		return
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		table.Clear()
		table.SetCell(0, 0, tview.NewTableCell("Error: "+err.Error()).SetTextColor(tcell.ColorRed))
		return
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
