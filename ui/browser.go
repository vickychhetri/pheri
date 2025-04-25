// ui/browser.go
package ui

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"mysql-tui/util"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var dataTable *tview.Table

// var allTables []string
type DBObject struct {
	Name string
	Type string
}

var allTables []DBObject

var mainFlex *tview.Flex
var fileNameInput *tview.InputField

func filterTableList(
	search string,
	allTable []DBObject,
	list *tview.List,
	queryBox *tview.TextArea,
	dataTable *tview.Table,
	app *tview.Application,
	db *sql.DB,
	dbName string,
) {
	list.Clear()
	search = strings.ToLower(search)

	// Optional: detect type filter prefix like "table:", "view:"
	var typeFilter string
	if strings.Contains(search, ":") {
		parts := strings.SplitN(search, ":", 2)
		typeFilter = strings.TrimSpace(parts[0])
		search = strings.TrimSpace(parts[1])
	}

	for _, obj := range allTable {
		// Match type filter if present
		if typeFilter != "" && strings.ToLower(obj.Type) != typeFilter {
			continue
		}

		// Match name
		if strings.Contains(strings.ToLower(obj.Name), search) {
			//displayName := fmt.Sprintf("[%s] %s", obj.Type, obj.Name)
			displayName := obj.Type + " " + obj.Name
			objName := obj.Name
			objType := obj.Type

			list.AddItem(displayName, "", 0, func() {
				switch objType {
				case "TABLE", "VIEW":
					query := "SELECT * FROM " + objName + " LIMIT 100"
					queryBox.SetText(query, true)
					ExecuteQuery(app, db, query, dataTable)
					app.SetFocus(dataTable)
				case "PROCEDURE":
					query := `SELECT ROUTINE_DEFINITION
					FROM INFORMATION_SCHEMA.ROUTINES
					WHERE ROUTINE_NAME = '` + objName + `'
					AND ROUTINE_SCHEMA = '` + dbName + `' AND ROUTINE_TYPE = 'PROCEDURE';`
					queryBox.SetText(query, true)
					app.SetFocus(queryBox)
				case "FUNCTION":
					query := `SELECT   routine_name, data_type, is_deterministic, security_type, definer, routine_definition 
					FROM INFORMATION_SCHEMA.ROUTINES
					WHERE ROUTINE_NAME = '` + objName + `'
					AND ROUTINE_SCHEMA = '` + dbName + `' AND ROUTINE_TYPE = 'FUNCTION';`

					routineDefinition, err := ExeQueryToData(db, objName, query, dbName, "FUNCTION")
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

					queryBox.SetText(routineDefinition, true)
					app.SetFocus(queryBox)
				}
			})
		}
	}
}

type RoutineMetadata struct {
	Definer           string
	RoutineName       string
	ReturnType        string
	RoutineDefinition string
	IsDeterministic   string
	SecurityType      string
}

type Parameter struct {
	Name     string
	DataType string
}

func ExeQueryToData(db *sql.DB, objName string, query string, dbName string, routineType string) (string, error) {
	// Execute the query to fetch routine metadata
	rows, err := db.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var metadata RoutineMetadata
	var params []Parameter

	// Fetch routine metadata from information_schema.routines
	if rows.Next() {
		err := rows.Scan(
			&metadata.RoutineName,
			&metadata.ReturnType,
			&metadata.IsDeterministic,
			&metadata.SecurityType,
			&metadata.Definer,
			&metadata.RoutineDefinition,
		)
		if err != nil {
			return "", err
		}
	} else {
		return "", fmt.Errorf("no routine found")
	}

	// Fetch parameters from information_schema.parameters
	paramsQuery := `
		SELECT parameter_name, data_type
		FROM information_schema.parameters
		WHERE specific_name = ? AND specific_schema = ?
		AND routine_type = ?	
		ORDER BY ordinal_position;`

	paramRows, err := db.Query(paramsQuery, objName, dbName, routineType)
	if err != nil {
		return "", err
	}
	defer paramRows.Close()

	// Scan all parameters
	for paramRows.Next() {
		var param Parameter
		var paramName sql.NullString // This allows NULL handling for parameter_name
		err := paramRows.Scan(&paramName, &param.DataType)
		if err != nil {
			return "", err
		}
		// If parameter_name is NULL, skip it or handle as needed
		if paramName.Valid {
			param.Name = paramName.String
			params = append(params, param)
		}

	}

	// Construct the CREATE FUNCTION SQL statement
	return buildCreateFunctionSQL(metadata, params, db, dbName), nil
}

func buildCreateFunctionSQL(metadata RoutineMetadata, params []Parameter, db *sql.DB, dbName string) string {
	// Split the Definer into user and host
	definerParts := strings.SplitN(metadata.Definer, "@", 2)
	user := definerParts[0]
	host := ""
	if len(definerParts) > 1 {
		host = definerParts[1]
	}
	sqlStmt := fmt.Sprintf("CREATE DEFINER=`%s`@`%s` FUNCTION `%s` (\n", user, host, metadata.RoutineName)

	// Add parameters
	for _, param := range params {
		sqlStmt += fmt.Sprintf("    `%s` %s,\n", param.Name, param.DataType)
	}
	// Remove the last comma and newline
	if len(params) > 0 {
		sqlStmt = sqlStmt[:len(sqlStmt)-2] + "\n"
	}
	return_type, err := util.GetFullReturnType(db, metadata.RoutineName, dbName)
	if err != nil {
		return fmt.Sprintf("Error fetching return type: %v", err)
	}

	// Add return type, language, deterministic, security, and comment
	sqlStmt += fmt.Sprintf(") RETURNS %s\n", return_type) +
		"LANGUAGE SQL\n" +
		"DETERMINISTIC\n" +
		"CONTAINS SQL\n" +
		fmt.Sprintf("SQL SECURITY %s\n", metadata.SecurityType) +
		"COMMENT ''\n" +
		metadata.RoutineDefinition + "\n"
	return sqlStmt
}

// func filterTableList(
// 	search string,
// 	allTables []string,
// 	list *tview.List,
// 	queryBox *tview.TextArea,
// 	dataTable *tview.Table,
// 	app *tview.Application,
// 	db *sql.DB,
// ) {
// 	list.Clear()
// 	search = strings.ToLower(search)
// 	for _, tableName := range allTables {
// 		if strings.Contains(strings.ToLower(tableName), search) {
// 			// Closure-safe name
// 			name := tableName
// 			list.AddItem(name, "", 0, func() {
// 				query := "SELECT * FROM " + name + " LIMIT 100"
// 				queryBox.SetText(query, true)
// 				ExecuteQuery(app, db, query, dataTable)
// 				app.SetFocus(dataTable)
// 			})
// 		}
// 	}
// }

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

	queryAllStructure := `SELECT table_name AS name, 'TABLE' AS type 
						FROM information_schema.tables 
						WHERE table_schema = '` + dbName + `' AND table_type = 'BASE TABLE'
						UNION ALL
						SELECT table_name AS name, 'VIEW' AS type 
						FROM information_schema.tables 
						WHERE table_schema = '` + dbName + `' AND table_type = 'VIEW'
						UNION ALL
						SELECT routine_name AS name, 'PROCEDURE' AS type 
						FROM information_schema.routines 
						WHERE routine_schema = '` + dbName + `' AND routine_type = 'PROCEDURE'
						UNION ALL
						SELECT routine_name AS name, 'FUNCTION' AS type 
						FROM information_schema.routines 
						WHERE routine_schema = '` + dbName + `' AND routine_type = 'FUNCTION';`
	rows, err := db.Query(queryAllStructure)
	if err != nil {
		tableList.AddItem("Error: "+err.Error(), "", 0, nil)
	} else {
		defer rows.Close()
		// var tableName string
		// Define queryBox and dataText outside the callback functions so they are in the scope
		var queryBox *tview.TextArea
		var dataTable *tview.Table

		var name, objectType string

		for rows.Next() {
			rows.Scan(&name, &objectType)
			// displayName := fmt.Sprintf("[%s] %s", objectType, name)
			dispalyName := objectType + " " + name
			allTables = append(allTables, DBObject{Name: name, Type: objectType})
			//rows.Scan(&tableName)
			currentName := name
			tableList.AddItem(dispalyName, "", 0, func() {
				switch objectType {
				case "TABLE", "VIEW":
					query := "SELECT * FROM " + currentName + " LIMIT 100"
					queryBox.SetText(query, true)
					ExecuteQuery(app, db, query, dataTable)
					app.SetFocus(dataTable)
				case "PROCEDURE":
					query := `SELECT ROUTINE_DEFINITION
					FROM INFORMATION_SCHEMA.ROUTINES
					WHERE ROUTINE_NAME = '` + currentName + `'
					AND ROUTINE_SCHEMA = '` + dbName + `' AND ROUTINE_TYPE = 'PROCEDURE';`

					queryBox.SetText(query, true)
					app.SetFocus(queryBox)
				case "FUNCTION":
					query := `SELECT routine_name, data_type, is_deterministic, security_type, definer, routine_definition 
					FROM INFORMATION_SCHEMA.ROUTINES
					WHERE ROUTINE_NAME = '` + currentName + `'
					AND ROUTINE_SCHEMA = '` + dbName + `' AND ROUTINE_TYPE = 'FUNCTION';`

					routineDefinition, err := ExeQueryToData(db, currentName, query, dbName, "FUNCTION")
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
					queryBox.SetText(routineDefinition, true)
					app.SetFocus(queryBox)
				}
			})

			// allTables = append(allTables, tableName)
			// tableList.AddItem(currentTable, "", 0, func() {
			// 	// Handle table selection logic here
			// 	if queryBox != nil {
			// 		query := "SELECT * FROM " + currentTable + " LIMIT 100"
			// 		queryBox.SetText(query, true)
			// 		ExecuteQuery(app, db, query, dataTable)
			// 		app.SetFocus(dataTable)
			// 	}
			// })
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

		queryPanel := tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(queryBox, 4, 1, true).
			AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
				AddItem(buttonBox, 0, 1, false).
				AddItem(saveButtonBox, 0, 1, false).
				AddItem(loadButtonBox, 0, 1, false).
				AddItem(exitButtonBox, 0, 1, false), 1, 0, false)

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
				filterTableList(text, allTables, tableList, queryBox, dataTable, app, db, dbName)
			})

		searchInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyTab {
				app.SetFocus(tableList)
				return nil
			}
			if event.Key() == tcell.KeyEscape {
				ShowDatabaseList(app, db)
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
