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
					err := ExecuteQuery(app, db, query, dataTable)

					if err != nil {
						modal := tview.NewModal().
							SetText("Executing Fail: " + err.Error()).
							AddButtons([]string{"OK"}).
							SetDoneFunc(func(buttonIndex int, buttonLabel string) {
								layout := CreateLayoutWithFooter(app, mainFlex)
								app.SetRoot(layout, true)
							})
						app.SetRoot(modal, true)
					}

					if objType == "TABLE" {
						err := EnableCellEditing(app, dataTable, db, dbName, objName)
						if err != nil {
							modal := tview.NewModal().
								SetText("Failed to enable cell editing: " + err.Error()).
								AddButtons([]string{"OK"}).
								SetDoneFunc(func(buttonIndex int, buttonLabel string) {
									layout := CreateLayoutWithFooter(app, mainFlex)
									app.SetRoot(layout, true)
								})

							app.SetRoot(modal, true)
						}
					}
					app.SetFocus(dataTable)
				case "PROCEDURE":
					// query := `SELECT ROUTINE_DEFINITION
					// FROM INFORMATION_SCHEMA.ROUTINES
					// WHERE ROUTINE_NAME = '` + objName + `'
					// AND ROUTINE_SCHEMA = '` + dbName + `' AND ROUTINE_TYPE = 'PROCEDURE';`
					// queryBox.SetText(query, true)
					// app.SetFocus(queryBox)
					query := `SELECT   routine_name, data_type, is_deterministic, security_type, definer, routine_definition 
					FROM INFORMATION_SCHEMA.ROUTINES
					WHERE ROUTINE_NAME = '` + objName + `'
					AND ROUTINE_SCHEMA = '` + dbName + `' AND ROUTINE_TYPE = 'PROCEDURE';`

					routineDefinition, err := ExeQueryToData(db, objName, query, dbName, "PROCEDURE")
					if err != nil {
						modal := tview.NewModal().
							SetText("Failed to execute query: " + err.Error()).
							AddButtons([]string{"OK"}).
							SetDoneFunc(func(buttonIndex int, buttonLabel string) {
								layout := CreateLayoutWithFooter(app, mainFlex)
								app.SetRoot(layout, true)
							})
						app.SetRoot(modal, true)
						return
					}

					queryBox.SetText(routineDefinition, true)
					app.SetFocus(queryBox)
				case "FUNCTION":
					query := `SELECT   routine_name, data_type, is_deterministic, security_type, definer, routine_definition 
					FROM INFORMATION_SCHEMA.ROUTINES
					WHERE ROUTINE_NAME = '` + objName + `'
					AND ROUTINE_SCHEMA = '` + dbName + `' AND ROUTINE_TYPE = 'FUNCTION';`

					util.SaveLog("FUNCTION: " + query)
					routineDefinition, err := ExeQueryToData(db, objName, query, dbName, "FUNCTION")
					util.SaveLog("FUNCTION1: " + routineDefinition)

					if err != nil {
						util.SaveLog("FUNCTION1: " + err.Error())
						modal := tview.NewModal().
							SetText("Failed to execute query: " + err.Error()).
							AddButtons([]string{"OK"}).
							SetDoneFunc(func(buttonIndex int, buttonLabel string) {
								layout := CreateLayoutWithFooter(app, mainFlex)
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
	Mode     string // <-- NEW field (optional: IN, OUT, INOUT)
}

func ExeQueryToData(db *sql.DB, objName string, query string, dbName string, routineType string) (string, error) {
	// Execute the query to fetch routine metadata
	rows, err := db.Query(query)
	if err != nil {
		util.SaveLog("1.) Error executing query: " + err.Error())
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
			util.SaveLog("2.) Error executing query: " + err.Error())
			return "", err
		}
	} else {
		util.SaveLog("3.) No routine found")
		return "", fmt.Errorf("no routine found")
	}

	// Fetch parameters from information_schema.parameters
	paramsQuery := `
			SELECT 
				parameter_name, 
				CONCAT(
					data_type,
					CASE 
						WHEN data_type IN ('char', 'varchar', 'binary', 'varbinary') 
							THEN CONCAT('(', character_maximum_length, ')')
						WHEN data_type IN ('decimal', 'numeric', 'float', 'double') 
							THEN CONCAT('(', numeric_precision, ',', numeric_scale, ')')
						ELSE ''
					END
				) AS data_type,
				parameter_mode
			FROM 
				information_schema.parameters
			WHERE 	
				specific_name = ? 
				AND specific_schema = ? 
				AND routine_type = ?
			ORDER BY 
				ordinal_position;
		`

	paramRows, err := db.Query(paramsQuery, objName, dbName, routineType)

	if err != nil {
		util.SaveLog(paramsQuery)
		util.SaveLog("3.) Error executing query: " + err.Error())
		return "", err
	}
	defer paramRows.Close()

	// Scan all parameters
	for paramRows.Next() {
		var param Parameter
		var paramName sql.NullString
		var paramMode sql.NullString // NEW
		err := paramRows.Scan(&paramName, &param.DataType, &paramMode)
		util.SaveLog("paramName: " + paramName.String)
		util.SaveLog("paramMode: " + paramMode.String)

		if err != nil {
			return "", err
		}
		if paramName.Valid {
			param.Name = paramName.String
		}
		if paramMode.Valid {
			param.Mode = paramMode.String
		}
		params = append(params, param)
	}

	util.SaveLog("Routine Name: " + metadata.RoutineName)
	// Construct the CREATE FUNCTION SQL statement
	if routineType == "FUNCTION" {
		util.SaveLog("Function Routine Name: " + metadata.RoutineName)
		return buildCreateFunctionSQL(metadata, params, db, dbName), nil
	} else if routineType == "PROCEDURE" {
		util.SaveLog("Procedure Routine Name: " + metadata.RoutineName)
		return buildCreateProcedureSQL(metadata, params, db), nil
	} else {
		util.SaveLog("4.) Unsupported routine type: " + routineType)
		return "", fmt.Errorf("unsupported routine type: %s", routineType)
	}

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
		if param.Mode != "" {
			sqlStmt += fmt.Sprintf("    `%s` %s,\n", param.Name, param.DataType)
		}
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

func buildCreateProcedureSQL(metadata RoutineMetadata, params []Parameter, db *sql.DB) string {
	// Split the Definer into user and host
	definerParts := strings.SplitN(metadata.Definer, "@", 2)
	user := definerParts[0]
	host := ""
	if len(definerParts) > 1 {
		host = definerParts[1]
	}

	sqlStmt := fmt.Sprintf("CREATE DEFINER=`%s`@`%s` PROCEDURE `%s` (\n", user, host, metadata.RoutineName)

	// Add parameters
	for _, param := range params {
		// In procedures, parameters usually have a mode: IN, OUT, or INOUT
		// Assuming param.Mode is available. If not, default to IN.
		mode := param.Mode
		if mode == "" {
			mode = "IN"
		}
		sqlStmt += fmt.Sprintf("    %s `%s` %s,\n", mode, param.Name, param.DataType)
	}

	// Remove the last comma and newline
	if len(params) > 0 {
		sqlStmt = sqlStmt[:len(sqlStmt)-2] + "\n"
	}

	// Add characteristics and body
	sqlStmt += fmt.Sprintf(")\nLANGUAGE SQL\n") +
		"DETERMINISTIC\n" +
		"CONTAINS SQL\n" +
		fmt.Sprintf("SQL SECURITY %s\n", metadata.SecurityType) +
		"COMMENT ''\n" +
		metadata.RoutineDefinition + "\n"

	return sqlStmt
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
	util.SaveLog("queryAllStructure: " + queryAllStructure)
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
		allTables = []DBObject{}
		for rows.Next() {
			rows.Scan(&name, &objectType)
			// displayName := fmt.Sprintf("[%s] %s", objectType, name)
			dispalyName := objectType + " " + name
			allTables = append(allTables, DBObject{Name: name, Type: objectType})
			//rows.Scan(&tableName)
			currentName := name
			currentobjectType := objectType
			tableList.AddItem(dispalyName, "", 0, func() {
				switch currentobjectType {
				case "PROCEDURE":
					// query := `SELECT ROUTINE_DEFINITION
					// FROM INFORMATION_SCHEMA.ROUTINES
					// WHERE ROUTINE_NAME = '` + currentName + `'
					// AND ROUTINE_SCHEMA = '` + dbName + `' AND ROUTINE_TYPE = 'PROCEDURE';`
					// util.SaveLog("PROCEDURE: " + query)
					// queryBox.SetText(query, true)
					// app.SetFocus(queryBox)
					query := `SELECT routine_name, data_type, is_deterministic, security_type, definer, routine_definition 
					FROM INFORMATION_SCHEMA.ROUTINES
					WHERE ROUTINE_NAME = '` + currentName + `'
					AND ROUTINE_SCHEMA = '` + dbName + `' AND ROUTINE_TYPE = 'PROCEDURE';`
					util.SaveLog("FUNCTION: " + query)
					routineDefinition, err := ExeQueryToData(db, currentName, query, dbName, "PROCEDURE")
					if err != nil {
						modal := tview.NewModal().
							SetText("Failed to execute query: " + err.Error()).
							AddButtons([]string{"OK"}).
							SetDoneFunc(func(buttonIndex int, buttonLabel string) {
								layout := CreateLayoutWithFooter(app, mainFlex)
								app.SetRoot(layout, true)
							})
						app.SetRoot(modal, true)
						return
					}
					queryBox.SetText(routineDefinition, true)
					app.SetFocus(queryBox)
				case "FUNCTION":
					query := `SELECT routine_name, data_type, is_deterministic, security_type, definer, routine_definition 
					FROM INFORMATION_SCHEMA.ROUTINES
					WHERE ROUTINE_NAME = '` + currentName + `'
					AND ROUTINE_SCHEMA = '` + dbName + `' AND ROUTINE_TYPE = 'FUNCTION';`
					util.SaveLog("FUNCTION: " + query)
					routineDefinition, err := ExeQueryToData(db, currentName, query, dbName, "FUNCTION")
					if err != nil {
						modal := tview.NewModal().
							SetText("Failed to execute query: " + err.Error()).
							AddButtons([]string{"OK"}).
							SetDoneFunc(func(buttonIndex int, buttonLabel string) {
								layout := CreateLayoutWithFooter(app, mainFlex)
								app.SetRoot(layout, true)
							})
						app.SetRoot(modal, true)
						return
					}
					queryBox.SetText(routineDefinition, true)
					app.SetFocus(queryBox)
				case "TABLE", "VIEW":
					query := "SELECT * FROM " + currentName + " LIMIT 100"
					queryBox.SetText(query, true)
					util.SaveLog("TABLE,VIEW: " + query)
					err = ExecuteQuery(app, db, query, dataTable)
					if err != nil {
						modal := tview.NewModal().
							SetText("Executing Fail: " + err.Error()).
							AddButtons([]string{"OK"}).
							SetDoneFunc(func(buttonIndex int, buttonLabel string) {
								layout := CreateLayoutWithFooter(app, mainFlex)
								app.SetRoot(layout, true)
							})
						app.SetRoot(modal, true)
					}

					if currentobjectType == "TABLE" {
						err = EnableCellEditing(app, dataTable, db, dbName, currentName)
						if err != nil {
							modal := tview.NewModal().
								SetText("Failed to enable cell editing: " + err.Error()).
								AddButtons([]string{"OK"}).
								SetDoneFunc(func(buttonIndex int, buttonLabel string) {
									layout := CreateLayoutWithFooter(app, mainFlex)
									app.SetRoot(layout, true)
								})

							app.SetRoot(modal, true)
						}
					}
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
							layout := CreateLayoutWithFooter(app, mainFlex)
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
		queryBox.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			switch event.Key() {
			case tcell.KeyTab:
				app.SetFocus(runButton)
				return nil
			case tcell.KeyEscape:
				layout := CreateLayoutWithFooter(app, mainFlex)
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
									layout := CreateLayoutWithFooter(app, mainFlex)
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
				layout := CreateLayoutWithFooter(app, mainFlex)
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
				layout := CreateLayoutWithFooter(app, mainFlex)
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
		layout := CreateLayoutWithFooter(app, mainFlex)
		app.SetRoot(layout, true)
	}
}

// Get primary key column name dynamically
func GetPrimaryKeyColumn(db *sql.DB, dbName, tableName string) (string, error) {
	query := `
	SELECT COLUMN_NAME
	FROM INFORMATION_SCHEMA.COLUMNS
	WHERE TABLE_SCHEMA = ?
	  AND TABLE_NAME = ?
	  AND COLUMN_KEY = 'PRI'
	LIMIT 1
	`
	var primaryKey string
	err := db.QueryRow(query, dbName, tableName).Scan(&primaryKey)

	if err != nil {
		util.SaveLog(" KEYS error Error getting primary key column: " + err.Error())
		util.SaveLog("dbName: " + dbName)
		util.SaveLog("tableName: " + tableName)
		util.SaveLog("Query: " + query)
		util.SaveLog("PrimaryKey: " + primaryKey)
		util.SaveLog("Error: " + err.Error())
		return "", err

	}
	return primaryKey, nil
}

// Fetch data and show in table
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

	// Set headers
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

// Enable editing and database update
func EnableCellEditing(app *tview.Application, table *tview.Table, db *sql.DB, dbName, tableName string) error {
	primaryKeyColumn, err := GetPrimaryKeyColumn(db, dbName, tableName)
	if err != nil {
		util.SaveLog("tableName: " + tableName)
		util.SaveLog("dbName: " + dbName)
		util.SaveLog("Error getting primary key column: " + err.Error())
		return err
	}

	table.SetSelectable(true, true)

	table.SetSelectedFunc(func(row int, column int) {
		if row == 0 {
			return // Skip header row
		}

		cell := table.GetCell(row, column)
		currentValue := cell.Text

		// Get column name from header
		headerCell := table.GetCell(0, column)
		columnName := stripFormatting(headerCell.Text)

		// Now don't assume primary key is always 0 column
		var primaryKeyValue string
		for col := 0; col < table.GetColumnCount(); col++ {
			colHeader := stripFormatting(table.GetCell(0, col).Text)
			if colHeader == primaryKeyColumn {
				primaryKeyValue = table.GetCell(row, col).Text
				break
			}
		}

		// Use TextArea now
		textArea := tview.NewTextArea()
		textArea.
			SetBorder(true).
			SetTitle(fmt.Sprintf("Edit %s (Enter=Save, Esc=Cancel)", columnName))

		textArea.SetText(string(currentValue), true)
		// textArea.SetChangedFunc(func() {
		// 	app.Draw()
		// })

		textArea.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			switch event.Key() {
			case tcell.KeyEnter:
				newValue := textArea.GetText()

				// Update cell visually
				cell.SetText(newValue)

				// Update database
				query := fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s = ?", tableName, columnName, primaryKeyColumn)
				_, err := db.Exec(query, newValue, primaryKeyValue)
				if err != nil {
					fmt.Println("Update error:", err)
				}

				app.SetRoot(mainFlex, true)
				util.SetFocusWithBorder(app, table)
				return nil

			case tcell.KeyEscape:
				app.SetRoot(mainFlex, true)
				util.SetFocusWithBorder(app, table)
				return nil
			}
			return event
		})

		modal := tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(textArea, 0, 1, true)

		app.SetRoot(modal, true).SetFocus(textArea)
	})

	return nil
}

// Remove formatting codes like [::b]
func stripFormatting(s string) string {
	s = strings.ReplaceAll(s, "[::b]", "")
	s = strings.ReplaceAll(s, "[::u]", "")
	return s
}

// func EnableCellEditing(app *tview.Application, table *tview.Table, db *sql.DB, tableName string, primaryKeyColumn string) {
// 	table.SetSelectable(true, true)

// 	table.SetSelectedFunc(func(row int, column int) {
// 		// Do not allow editing of header row
// 		if row == 0 {
// 			return
// 		}

// 		cell := table.GetCell(row, column)
// 		currentValue := cell.Text

// 		// Get the column name from header
// 		columnName := table.GetCell(0, column).Text
// 		columnName = tview.Escape(columnName) // Remove formatting like [::b]

// 		// Assume primary key value is in column 0
// 		primaryKeyValue := table.GetCell(row, 0).Text

// 		input := tview.NewInputField()
// 		input.SetFieldBackgroundColor(tcell.ColorBlack).
// 			SetLabel(fmt.Sprintf("Edit %s: ", columnName)).
// 			SetText(currentValue).
// 			SetDoneFunc(func(key tcell.Key) {
// 				if key == tcell.KeyEnter {
// 					newValue := input.GetText()

// 					// Update the table cell visually
// 					cell.SetText(newValue)

// 					// UPDATE the database
// 					query := fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s = ?", tableName, columnName, primaryKeyColumn)
// 					_, err := db.Exec(query, newValue, primaryKeyValue)
// 					if err != nil {
// 						// Handle error if needed
// 						fmt.Println("Update failed:", err)
// 					}

// 					app.SetRoot(table, true)
// 				} else if key == tcell.KeyEscape {
// 					app.SetRoot(table, true)
// 				}
// 			})

// 		modal := tview.NewFlex().
// 			SetDirection(tview.FlexRow).
// 			AddItem(input, 3, 1, true)

// 		app.SetRoot(modal, true).SetFocus(input)
// 	})
// }

// func ExecuteQuery(app *tview.Application, db *sql.DB, query string, table *tview.Table) error {
// 	rows, err := db.Query(query)
// 	if err != nil {
// 		table.Clear()
// 		table.SetCell(0, 0, tview.NewTableCell("Error: "+err.Error()).SetTextColor(tcell.ColorRed))
// 		return err
// 	}
// 	defer rows.Close()

// 	columns, err := rows.Columns()
// 	if err != nil {
// 		table.Clear()
// 		table.SetCell(0, 0, tview.NewTableCell("Error: "+err.Error()).SetTextColor(tcell.ColorRed))
// 		return err
// 	}

// 	table.Clear()

// 	// Show column headers
// 	for i, col := range columns {
// 		table.SetCell(0, i, tview.NewTableCell(fmt.Sprintf("[::b]%s", col)).SetAlign(tview.AlignCenter))
// 	}

// 	values := make([]sql.RawBytes, len(columns))
// 	scanArgs := make([]interface{}, len(values))
// 	for i := range values {
// 		scanArgs[i] = &values[i]
// 	}

// 	rowIndex := 1
// 	for rows.Next() {
// 		err := rows.Scan(scanArgs...)
// 		if err != nil {
// 			continue
// 		}

// 		for i, col := range values {
// 			table.SetCell(rowIndex, i, tview.NewTableCell(string(col)).SetAlign(tview.AlignLeft))
// 		}
// 		rowIndex++
// 	}
// 	return nil
// }

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

	layout := CreateLayoutWithFooter(app, list)
	app.SetRoot(tview.NewFlex().AddItem(layout, 0, 1, true), true)
	app.SetFocus(layout)
}
