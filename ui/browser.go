// ui/browser.go
package ui

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"mysql-tui/phhistory"
	"mysql-tui/util"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var dataTable *tview.Table
var dataBaseList *tview.List
var allDatabases []string

// var allTables []string
type DBObject struct {
	Name string
	Type string
}

var allTables []DBObject

var mainFlex *tview.Flex
var fileNameInput *tview.InputField

var isEditingEnabled bool = false
var searchFiltertext string
var IsSearchStateEnabled = false

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

	var typeFilter string
	if strings.Contains(search, ":") {
		parts := strings.SplitN(search, ":", 2)
		typeFilter = strings.TrimSpace(parts[0])
		search = strings.TrimSpace(parts[1])
	}

	if typeFilter == "db" {
		if dataBaseList != nil {
			dataBaseList.Clear()
		}
		for _, filterDbName := range allDatabases {
			// if strings.ToLower(filterDbName)

			if strings.Contains(strings.ToLower(filterDbName), search) {
				dataBaseList.AddItem("📁 "+filterDbName, "Press Enter to use", 0, func() {
					IsSearchStateEnabled = false
					UseDatabase(app, db, filterDbName)
				})
			}

		}

	} else {
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

				list.AddItem("🧮 "+displayName, "Press Enter to use", 0, func() {
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
							isEditingEnabled = true
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
								return
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
						routineDefinition, err := ExeQueryToData(db, objName, query, dbName, "FUNCTION")
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
	Mode     string
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

var sqlTemplates = []string{
	// CREATE Commands
	"CREATE TABLE table_name (column1 datatype, column2 datatype, ...)",
	"CREATE DATABASE database_name",
	"CREATE INDEX index_name ON table_name (column_name)",
	"CREATE UNIQUE INDEX index_name ON table_name (column_name)",
	"CREATE VIEW view_name AS SELECT column1, column2 FROM table_name WHERE condition",

	// INSERT Commands
	"INSERT INTO table_name (column1, column2) VALUES (value1, value2)",
	"INSERT INTO table_name VALUES (value1, value2, ...)",
	"INSERT INTO table_name (col1, col2) SELECT col1, col2 FROM other_table",

	// ALTER Commands
	"ALTER TABLE table_name ADD column_name datatype",
	"ALTER TABLE table_name DROP COLUMN column_name",
	"ALTER TABLE table_name RENAME TO new_table_name",
	"ALTER TABLE table_name MODIFY column_name datatype",
	"ALTER TABLE table_name ADD CONSTRAINT constraint_name FOREIGN KEY (column_name) REFERENCES other_table(column_name)",

	// DROP/DELETE/TRUNCATE
	"DROP TABLE table_name",
	"DROP DATABASE database_name",
	"DROP INDEX index_name ON table_name",
	"TRUNCATE TABLE table_name",
	"DELETE FROM table_name WHERE condition",

	// UPDATE
	"UPDATE table_name SET column1 = value1, column2 = value2 WHERE condition",

	// SELECT Queries
	"SELECT * FROM table_name",
	"SELECT column1, column2 FROM table_name WHERE condition",
	"SELECT column1, COUNT(*) FROM table_name GROUP BY column1",
	"SELECT column1 FROM table_name ORDER BY column1 DESC",
	"SELECT DISTINCT column1 FROM table_name",
	"SELECT column1 FROM table_name LIMIT 10 OFFSET 5",

	// Conditions and Clauses
	"WHERE column_name = value",
	"WHERE column_name BETWEEN value1 AND value2",
	"WHERE column_name LIKE '%value%'",
	"ORDER BY column_name ASC",
	"GROUP BY column_name",
	"HAVING COUNT(column_name) > value",
	"LIMIT number",
	"OFFSET number",

	// Aggregate Functions
	"SELECT COUNT(*) FROM table_name",
	"SELECT SUM(column_name) FROM table_name",
	"SELECT AVG(column_name) FROM table_name",
	"SELECT MIN(column_name) FROM table_name",
	"SELECT MAX(column_name) FROM table_name",
}

var sqlKeywords = []string{
	// DML (Data Manipulation Language)
	"SELECT", "INSERT", "UPDATE", "DELETE", "MERGE", "CALL", "EXPLAIN", "LOCK",

	// DDL (Data Definition Language)
	"CREATE", "ALTER", "DROP", "TRUNCATE", "RENAME", "COMMENT",

	// DCL (Data Control Language)
	"GRANT", "REVOKE",

	// TCL (Transaction Control Language)
	"COMMIT", "ROLLBACK", "SAVEPOINT", "SET TRANSACTION",

	// Clauses and Operators
	"FROM", "WHERE", "HAVING", "GROUP BY", "ORDER BY", "LIMIT", "OFFSET",
	"VALUES", "INTO", "DISTINCT", "UNION", "UNION ALL", "INTERSECT", "EXCEPT",

	// Joins
	"JOIN", "INNER JOIN", "LEFT JOIN", "RIGHT JOIN", "FULL JOIN", "CROSS JOIN", "NATURAL JOIN", "ON", "USING",

	// Conditions
	"AND", "OR", "NOT", "IN", "LIKE", "IS NULL", "IS NOT NULL", "BETWEEN", "EXISTS",

	// Data Types (for completeness)
	"INT", "INTEGER", "VARCHAR", "CHAR", "TEXT", "DATE", "DATETIME", "BOOLEAN", "DECIMAL", "FLOAT",

	// Miscellaneous
	"AS", "DESC", "ASC", "CASE", "WHEN", "THEN", "ELSE", "END", "DEFAULT", "PRIMARY KEY", "FOREIGN KEY",
	"AUTO_INCREMENT", "INDEX", "CONSTRAINT", "REFERENCES", "CHECK", "IF", "ALL", "ANY", "SOME",

	// Functions (optional)
	"COUNT", "SUM", "AVG", "MIN", "MAX", "NOW", "COALESCE", "NULLIF", "ROUND", "LENGTH",
}

func getSQLSuggestions(prefix string) []string {

	util.SaveLog("prefix: " + prefix)
	prefix = strings.ToUpper(prefix)
	var suggestions []string
	for _, word := range sqlKeywords {
		if strings.HasPrefix(word, prefix) {
			suggestions = append(suggestions, word)
		}
	}
	return suggestions
}

func showSuggestionBox(app *tview.Application, mainFlex *tview.Flex, editor *tview.TextArea, suggestions []string, onSelect func(string)) {
	list := tview.NewList()
	for _, s := range suggestions {
		sugg := s // capture loop variable
		list.AddItem(s, "", 0, func() {
			onSelect(sugg)
			app.SetRoot(mainFlex, true)
			app.SetFocus(editor)
		})
	}

	modal := tview.NewFlex().AddItem(list, 30, 1, true)
	app.SetRoot(modal, true).SetFocus(list)
}

func UseDatabase(app *tview.Application, db *sql.DB, dbName string) {
	runIcon := "\n▶ Execute Query\n"
	saveIcon := "\n💾 Save Query\n"
	loadIcon := "\n📂 Load Query\n"
	exitIcon := "\n❌ Exit Application\n"

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

	dataBaseList = tview.NewList()
	dataBaseList.
		ShowSecondaryText(false).
		SetHighlightFullLine(true)

	dataBaseList.SetBorder(true).
		SetTitle(" 🗂️  Databases ").
		SetTitleAlign(tview.AlignLeft).
		SetBorderColor(tcell.ColorGreen)

	queryAllDB := `SHOW DATABASES;`

	dbRows, err := db.Query(queryAllDB)
	if err != nil {
		dataBaseList.AddItem("❌ "+"Error: "+err.Error(), "", 0, nil)
	} else {
		defer dbRows.Close()
		var dbNameli string
		for dbRows.Next() {
			if err := dbRows.Scan(&dbNameli); err != nil {
				log.Println("DB Fetch Error!")
				continue
			}
			allDatabases = append(allDatabases, dbNameli)
			currentDBName := dbNameli
			dataBaseList.AddItem("📁 "+currentDBName, "Press Enter to use", 0, func() {
				IsSearchStateEnabled = true
				UseDatabase(app, db, currentDBName)
			})
		}

	}

	// LEFT: Table list (using tview.List)
	tableList := tview.NewList()
	tableList.
		ShowSecondaryText(false).
		SetHighlightFullLine(true)

	tableList.
		SetBorder(true).
		SetTitle(" 🧮 Tables ").
		SetTitleAlign(tview.AlignLeft).
		SetBorderColor(tcell.ColorYellow)

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
			// rows.Scan(&name, &objectType)
			if err := rows.Scan(&name, &objectType); err != nil {
				log.Println("Scan error:", err)
				continue
			}

			// displayName := fmt.Sprintf("[%s] %s", objectType, name)
			dispalyName := objectType + " " + name
			allTables = append(allTables, DBObject{Name: name, Type: objectType})
			//rows.Scan(&tableName)
			currentName := name
			currentobjectType := objectType
			tableList.AddItem("🧮 "+dispalyName, "Press Enter to use", 0, func() {
				switch currentobjectType {
				case "PROCEDURE":
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

					phhistory.SaveQuery(query, dbName)

					if currentobjectType == "TABLE" {
						isEditingEnabled = true
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
							return
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
				phhistory.SaveQuery(query, dbName)
				isEditingEnabled = false
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

		// queryBox = tview.NewTextArea()
		// queryBox.
		// 	SetBorder(true).
		// 	SetTitle("Query- ctrl+R: Run, ctrl+F11: FullScreen, ctrl+T: Table, ctrl+S: SQL Keywords, ctrl+_: SQL Templates.").
		// 	SetTitleAlign(tview.AlignCenter).Blur()
		// queryBox.SetTitleAlign(tview.AlignLeft).
		// 	SetBorderColor(tcell.ColorWhite)

		queryBox = tview.NewTextArea()
		queryBox.
			SetBorder(true).
			SetTitle(" [::b]Query Editor[::-] - [green]Ctrl+R:[-]Run  [green]Ctrl+F11:[-]FullScreen  [green]Ctrl+T:[-]Table  [green]Ctrl+S:[-]Keywords  [green]Ctrl+_:[-]Templates").
			SetTitleAlign(tview.AlignCenter).
			SetBorderColor(tcell.ColorLightCyan).
			SetTitleColor(tcell.ColorAqua).
			Blur()

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
			case tcell.KeyCtrlR:
				query := queryBox.GetText()
				err := ExecuteQuery(app, db, query, dataTable)
				phhistory.SaveQuery(query, dbName)
				isEditingEnabled = false
				if err != nil {
					modal := tview.NewModal().
						SetText("Failed to execute query: " + err.Error()).
						AddButtons([]string{"OK"}).
						SetDoneFunc(func(buttonIndex int, buttonLabel string) {
							layout := CreateLayoutWithFooter(app, mainFlex)
							app.SetRoot(layout, true)
						})
					app.SetRoot(modal, true)
					return nil
				}
				app.SetRoot(mainFlex, true)
				app.SetFocus(dataTable)
				return nil

			case tcell.KeyCtrlUnderscore:
				// Get current word at cursor
				row, col, _, _ := queryBox.GetCursor()
				text := queryBox.GetText()
				lines := strings.Split(text, "\n")

				if row >= len(lines) {
					return nil
				}
				currentLine := lines[row]

				prefix := currentLine[:col]
				words := strings.Fields(prefix)
				if len(words) == 0 {
					return nil
				}
				currentWord := words[len(words)-1]
				matches := []string{}
				for _, kw := range sqlTemplates {
					if strings.HasPrefix(strings.ToUpper(kw), strings.ToUpper(currentWord)) {
						matches = append(matches, kw)
					}
				}
				if len(matches) == 0 {
					return nil
				}

				util.SaveLog("matches: " + fmt.Sprint(matches))
				suggestionList := tview.NewList()
				suggestionList.ShowSecondaryText(false).
					SetBorder(true).SetTitle("Suggestions")
				for _, match := range matches {
					kw := match
					suggestionList.AddItem(kw, "", 0, func() {
						before := currentLine[:col]
						after := currentLine[col:]
						idx := strings.LastIndex(before, currentWord)
						newLine := before[:idx] + kw + after
						lines[row] = newLine
						linesText := strings.Join(lines, "\n")

						queryBox.SetText(linesText, true)

						app.SetRoot(queryBox, true).SetFocus(queryBox)
					})
				}
				app.SetRoot(suggestionList, true).SetFocus(suggestionList)
				return nil

			case tcell.KeyCtrlT:
				// Get current word at cursor
				row, col, _, _ := queryBox.GetCursor()
				text := queryBox.GetText()
				lines := strings.Split(text, "\n")

				if row >= len(lines) {
					return nil
				}
				currentLine := lines[row]

				prefix := currentLine[:col]
				words := strings.Fields(prefix)
				if len(words) == 0 {
					return nil
				}
				currentWord := words[len(words)-1]

				// Find suggestions
				matches := []string{}

				for _, table := range allTables {
					if strings.HasPrefix(strings.ToUpper(table.Name), strings.ToUpper(currentWord)) {
						matches = append(matches, table.Name)
					}
				}
				if len(matches) == 0 {
					return nil
				}

				util.SaveLog("matches: " + fmt.Sprint(matches))
				// Show suggestions
				suggestionList := tview.NewList()
				suggestionList.ShowSecondaryText(false).
					SetBorder(true).SetTitle("Suggestions")
				for _, match := range matches {
					kw := match
					suggestionList.AddItem(kw, "", 0, func() {
						// Replace current word with selection
						before := currentLine[:col]
						after := currentLine[col:]

						// Replace last word in 'before' with selected keyword
						idx := strings.LastIndex(before, currentWord)
						newLine := before[:idx] + kw + after
						lines[row] = newLine
						linesText := strings.Join(lines, "\n")

						queryBox.SetText(linesText, true)

						app.SetRoot(queryBox, true).SetFocus(queryBox)
					})
				}
				app.SetRoot(suggestionList, true).SetFocus(suggestionList)
				return nil

			case tcell.KeyCtrlF:
				searchInput := tview.NewInputField()
				searchInput.
					SetLabel("Search: ").
					SetFieldWidth(30).
					SetDoneFunc(func(key tcell.Key) {
						searchTerm := searchInput.GetText()
						text := queryBox.GetText()

						// Highlight all matches (simplified: uppercase the matches)
						highlighted := strings.ReplaceAll(text, searchTerm, "[yellow::b]"+searchTerm+"[::-]")
						queryBox.SetText(highlighted, true)

						app.SetRoot(queryBox, true).SetFocus(queryBox)
					})
				searchInput.SetBorder(true).SetTitle("Search").SetTitleAlign(tview.AlignLeft)
				app.SetRoot(searchInput, true).SetFocus(searchInput)
				return nil

			case tcell.KeyCtrlS:
				// Get current word at cursor
				row, col, _, _ := queryBox.GetCursor()
				text := queryBox.GetText()
				lines := strings.Split(text, "\n")

				if row >= len(lines) {
					return nil
				}
				currentLine := lines[row]

				prefix := currentLine[:col]
				words := strings.Fields(prefix)
				if len(words) == 0 {
					return nil
				}
				currentWord := words[len(words)-1]

				// Find suggestions
				matches := []string{}
				for _, kw := range sqlKeywords {
					if strings.HasPrefix(strings.ToUpper(kw), strings.ToUpper(currentWord)) {
						matches = append(matches, kw)
					}
				}
				if len(matches) == 0 {
					return nil
				}

				util.SaveLog("matches: " + fmt.Sprint(matches))
				// Show suggestions
				suggestionList := tview.NewList()
				suggestionList.ShowSecondaryText(false).
					SetBorder(true).SetTitle("Suggestions")
				for _, match := range matches {
					kw := match
					suggestionList.AddItem(kw, "", 0, func() {
						// Replace current word with selection
						before := currentLine[:col]
						after := currentLine[col:]

						// Replace last word in 'before' with selected keyword
						idx := strings.LastIndex(before, currentWord)
						newLine := before[:idx] + kw + after
						lines[row] = newLine
						linesText := strings.Join(lines, "\n")

						queryBox.SetText(linesText, true)

						app.SetRoot(queryBox, true).SetFocus(queryBox)
					})
				}
				app.SetRoot(suggestionList, true).SetFocus(suggestionList)
				return nil
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
				searchFiltertext = text
				filterTableList(text, allTables, tableList, queryBox, dataTable, app, db, dbName)
			})

		if searchFiltertext != "" && IsSearchStateEnabled {
			searchInput.SetText(searchFiltertext)
			filterTableList(searchFiltertext, allTables, tableList, queryBox, dataTable, app, db, dbName)
			IsSearchStateEnabled = false
		}
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
				app.SetFocus(dataBaseList)
				return nil
			}
			if event.Key() == tcell.KeyEscape {
				app.SetFocus(searchInput)
				return nil
			}

			return event
		})
		dataBaseList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyTab {
				app.SetFocus(queryBox)
				return nil
			}
			return event
		})

		leftPanel := tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(searchInput, 1, 0, false).
			AddItem(tableList, 0, 1, true).
			AddItem(dataBaseList, 0, 1, true)

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
		table.SetCell(0, 0, tview.NewTableCell("[red::b]Error: "+err.Error()))
		return err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		table.Clear()
		table.SetCell(0, 0, tview.NewTableCell("[red::b]Error: "+err.Error()))
		return err
	}

	table.Clear()
	table.SetBorders(false)

	// Set header with styling
	for i, col := range columns {
		header := fmt.Sprintf("[::b][white::]%s", col)
		table.SetCell(0, i,
			tview.NewTableCell(header).
				SetTextColor(tcell.ColorWhite).
				SetAlign(tview.AlignCenter).
				SetSelectable(false))
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
			text := string(col)
			if text == "" {
				text = "[gray]NULL"
			}

			color := tcell.ColorWhite
			if rowIndex%2 == 0 {
				color = tcell.ColorLightGray
			}

			cell := tview.NewTableCell(text).
				SetTextColor(color).
				SetAlign(tview.AlignLeft)

			table.SetCell(rowIndex, i, cell)
		}
		rowIndex++
	}

	// Add a title row (optional)
	table.SetTitle(" [::b]Query Result ").SetTitleAlign(tview.AlignLeft).SetBorder(true)

	return nil
}

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

// 	// Set headers
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
	// table.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorLightYellow).Foreground(tcell.ColorBlack))
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorWhite).
		Foreground(tcell.ColorBlack))
	table.SetSelectedFunc(func(row int, column int) {
		if row == 0 {
			return // Skip header row
		}

		cell := table.GetCell(row, column)
		currentValue := cell.Text

		// Get column name from header
		headerCell := table.GetCell(0, column)
		columnName := util.StripFormatting(headerCell.Text)
		// columnName = util.StripFormatting(columnName)
		// Now don't assume primary key is always 0 column
		var primaryKeyValue string
		for col := 0; col < table.GetColumnCount(); col++ {
			colHeader := util.StripFormatting(table.GetCell(0, col).Text)
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

		textArea.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			switch event.Key() {
			case tcell.KeyEnter:
				if !isEditingEnabled {
					modal := tview.NewModal().
						SetText("Not allowed to update in Run Query mode").
						AddButtons([]string{"OK"}).
						SetDoneFunc(func(buttonIndex int, buttonLabel string) {
							app.SetRoot(mainFlex, true)
							util.SetFocusWithBorder(app, table)
						})
					app.SetRoot(modal, false)
					return nil
				}

				newValue := textArea.GetText()

				// Update cell visually
				cell.SetText(newValue)

				columnName = util.StripFormatting(columnName)
				// Update database
				query := fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s = ?", tableName, columnName, primaryKeyColumn)
				_, err := db.Exec(query, newValue, primaryKeyValue)
				if err != nil {
					fmt.Println("Update error:", err)
				}
				fullQuery := phhistory.ReplacePlaceholders(query, newValue, primaryKeyValue)
				phhistory.SaveQuery(fullQuery, dbName)
				util.SaveLog(fullQuery)
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
	list := tview.NewList().ShowSecondaryText(true)

	// Go up
	if currentDir != "/" {
		parent := filepath.Dir(currentDir)
		list.AddItem("[::b]<..>", "Go up a directory", 'u', func() {
			fileBrowser(button2, parent, app, queryBox, returnTo)
		})
	}

	// Read and sort entries
	entries, err := os.ReadDir(currentDir)
	if err != nil {
		log.Printf("Failed to read directory: %v", err)
		app.SetRoot(returnTo, true)
		return
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(currentDir, name)

		info, err := os.Stat(fullPath) // <- use os.Stat here
		if err != nil {
			continue
		}
		modTime := info.ModTime().Format("2006-01-02 15:04")
		size := fmt.Sprintf("%d bytes", info.Size())
		meta := fmt.Sprintf("%s | %s", size, modTime)

		if info.IsDir() {
			list.AddItem(fmt.Sprintf("%s", name), meta, 0, func(p string) func() {
				return func() {
					fileBrowser(button2, p, app, queryBox, returnTo)
				}
			}(fullPath))
		} else if strings.HasSuffix(name, ".sql") || strings.HasSuffix(name, ".go") {
			list.AddItem(fmt.Sprintf("[green]%s", name), meta, 0, func(p string) func() {
				return func() {
					content, err := os.ReadFile(p)
					if err != nil {
						log.Printf("Failed to read file: %v", err)
					} else {
						queryBox.SetText(string(content), true)
						app.SetFocus(queryBox)
					}
					app.SetRoot(returnTo, true)
				}
			}(fullPath))
		}
	}

	list.SetDoneFunc(func() {
		app.SetRoot(returnTo, true)
		app.SetFocus(button2)
	})

	// Footer: current directory
	statusBar := tview.NewTextView().
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true).
		SetText(fmt.Sprintf("[::b]Current Directory: [white]%s", currentDir))

	// Layout with footer
	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(statusBar, 1, 0, false)

	app.SetRoot(layout, true)
	app.SetFocus(list)
}
