// ui/browser.go
package ui

import (
	"bufio"
	"compress/gzip"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"mysql-tui/phhistory"
	"mysql-tui/util"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var dataTable *tview.Table
var dataBaseList *tview.List
var tableList *tview.List
var allDatabases []string
var activeSQLEditor *SQLEditor

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

var currentGridPage int = 0
var gridPageSize int = 100
var activeGridQuery string = ""
var activeGridObject string = ""
var activeGridObjectType string = ""
var activeGridDBName string = ""
var activeGridDB *sql.DB = nil


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

					typePriority := map[string]int{
						"TABLE":     0,
						"VIEW":      1,
						"FUNCTION":  2,
						"PROCEDURE": 3,
					}

					sort.Slice(allTables, func(i, j int) bool {
						return typePriority[allTables[i].Type] < typePriority[allTables[j].Type]
					})
					app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
						if event.Key() == tcell.KeyCtrlX {
							if objType == "TABLE" {
								// Step 1: Get the table's DDL (definition)

								query := "SHOW CREATE TABLE " + objName
								row, err := db.Query(query)
								if err != nil {
									showErrorModal(app, mainFlex, "Failed to fetch table definition: "+err.Error())
									return nil
								}
								defer row.Close()

								var tableName, createStatement string
								if row.Next() {
									err := row.Scan(&tableName, &createStatement)
									if err != nil {
										showErrorModal(app, mainFlex, "Scan failed: "+err.Error())
										return nil
									}

									// Step 2: Copy the table's DDL (definition) to the clipboard
									err = clipboard.WriteAll(createStatement + ";")
									if err != nil {
										showErrorModal(app, mainFlex, "Failed to copy DDL to clipboard: "+err.Error())
										return nil
									}

									// Step 3: Get the table data (rows)
									db.Exec("USE " + dbName)
									dataQuery := "SELECT * FROM " + objName
									rows, err := db.Query(dataQuery)
									if err != nil {
										showErrorModal(app, mainFlex, "Failed to fetch table data: "+err.Error())
										return nil
									}
									defer rows.Close()

									// Fetch column names
									columns, err := rows.Columns()
									if err != nil {
										showErrorModal(app, mainFlex, "Failed to get columns: "+err.Error())
										return nil
									}

									var insertStatements []string
									for rows.Next() {
										values := make([]interface{}, len(columns))
										pointers := make([]interface{}, len(columns))
										for i := range values {
											pointers[i] = &values[i]
										}

										err := rows.Scan(pointers...)
										if err != nil {
											showErrorModal(app, mainFlex, "Failed to scan row: "+err.Error())
											return nil
										}

										// Build the insert statement for the current row
										var valuesList []string
										for _, val := range values {
											if val != nil {
												switch v := val.(type) {
												case []byte:
													valuesList = append(valuesList, fmt.Sprintf("'%s'", string(v)))
												default:
													valuesList = append(valuesList, fmt.Sprintf("'%v'", v))
												}
											} else {
												valuesList = append(valuesList, "NULL")
											}
										}

										insertStatement := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);", objName, strings.Join(columns, ", "), strings.Join(valuesList, ", "))
										insertStatements = append(insertStatements, insertStatement)
									}

									// Step 4: Join all insert statements and copy them to clipboard
									dataString := strings.Join(insertStatements, "\n")

									clipboardText := util.GetClipboardText()
									err = clipboard.WriteAll(clipboardText + "\n" + dataString)
									if err != nil {
										showErrorModal(app, mainFlex, "Failed to copy data to clipboard: "+err.Error())
										return nil
									}

									// Optional: Show a confirmation modal
									modal := tview.NewModal().
										SetText("Table definition and data copied to clipboard as SQL INSERT statements.").
										AddButtons([]string{"OK"}).
										SetDoneFunc(func(buttonIndex int, buttonLabel string) {
											layout := CreateLayoutWithFooter(app, mainFlex)
											app.SetRoot(layout, true)
										})
									app.SetRoot(modal, true)
								}
							}

							if objType == "VIEW" {
								query := "SHOW CREATE VIEW " + objName
								row, err := db.Query(query)
								if err != nil {
									showErrorModal(app, mainFlex, "Failed to fetch view definition: "+err.Error())
									return nil
								}
								defer row.Close()

								var viewName, createStatement, charset, collation string
								if row.Next() {
									err := row.Scan(&viewName, &createStatement, &charset, &collation)
									if err != nil {
										showErrorModal(app, mainFlex, "Scan failed: "+err.Error())
										return nil
									}

									// Copy the CREATE VIEW statement to clipboard
									clipboard.WriteAll(createStatement)

									modal := tview.NewModal().
										SetText("View definition copied to clipboard.").
										AddButtons([]string{"OK"}).
										SetDoneFunc(func(buttonIndex int, buttonLabel string) {
											layout := CreateLayoutWithFooter(app, mainFlex)
											app.SetRoot(layout, true)
										})
									app.SetRoot(modal, true)
								}
							}

							return nil
						}
						return event
					})

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

						setEditorText(queryBox, routineDefinition)
						if activeSQLEditor != nil {
							app.SetFocus(activeSQLEditor.Editor)
						} else {
							app.SetFocus(queryBox)
						}
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

						setEditorText(queryBox, routineDefinition)
						if activeSQLEditor != nil {
							app.SetFocus(activeSQLEditor.Editor)
						} else {
							app.SetFocus(queryBox)
						}
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
	// --- DATABASE CREATION ---
	"CREATE DATABASE company_db",
	"DROP DATABASE old_company_db",

	// --- TABLE CREATION & MODIFICATION ---
	"CREATE TABLE employees (id INT PRIMARY KEY AUTO_INCREMENT, name VARCHAR(100), age INT, department_id INT, hire_date DATE)",
	"ALTER TABLE employees ADD COLUMN salary DECIMAL(10,2)",
	"ALTER TABLE employees DROP COLUMN middle_name",
	"ALTER TABLE employees RENAME TO staff",
	"ALTER TABLE employees MODIFY age SMALLINT",
	"ALTER TABLE employees ADD CONSTRAINT fk_department FOREIGN KEY (department_id) REFERENCES departments(id)",
	"DROP TABLE employees",
	"TRUNCATE TABLE logs",

	// --- INDEXES ---
	"CREATE INDEX idx_emp_name ON employees (name)",
	"CREATE UNIQUE INDEX idx_users_email ON users (email)",
	"DROP INDEX idx_emp_name ON employees",

	// --- INSERT DATA ---
	"INSERT INTO employees (name, age, department_id) VALUES ('John Doe', 30, 2)",
	"INSERT INTO departments (id, name) VALUES (1, 'HR'), (2, 'Engineering')",
	"INSERT INTO archive_employees (id, name) SELECT id, name FROM employees WHERE status = 'inactive'",

	// --- DELETE & UPDATE DATA ---
	"DELETE FROM employees WHERE department_id = 5",
	"DELETE FROM employees",
	"UPDATE employees SET salary = salary + 1000 WHERE performance = 'excellent'",

	// --- SIMPLE SELECT ---
	"SELECT * FROM employees",
	"SELECT name, age FROM employees WHERE department_id = 2",
	"SELECT name FROM employees ORDER BY hire_date DESC",
	"SELECT DISTINCT job_title FROM employees",
	"SELECT * FROM employees LIMIT 10 OFFSET 5",

	// --- SELECT WITH CONDITIONS ---
	"WHERE age > 25",
	"WHERE name LIKE 'J%'",
	"WHERE salary BETWEEN 30000 AND 50000",
	"WHERE department_id IN (1, 2, 3)",
	"WHERE hire_date IS NOT NULL",

	// --- AGGREGATES & GROUPING ---
	"SELECT COUNT(*) FROM employees",
	"SELECT AVG(salary) FROM employees WHERE department_id = 2",
	"SELECT department_id, SUM(salary) FROM employees GROUP BY department_id",
	"SELECT department_id, COUNT(*) FROM employees GROUP BY department_id",
	"SELECT department_id, AVG(salary) FROM employees GROUP BY department_id HAVING AVG(salary) > 50000",

	// --- JOIN OPERATIONS ---
	"SELECT e.name, d.name FROM employees e INNER JOIN departments d ON e.department_id = d.id",
	"SELECT e.name, d.name FROM employees e LEFT JOIN departments d ON e.department_id = d.id",
	"SELECT e.name, d.name FROM employees e RIGHT JOIN departments d ON e.department_id = d.id",
	"SELECT e.name, d.name FROM employees e FULL OUTER JOIN departments d ON e.department_id = d.id", // (if supported)

	// --- SUBQUERIES ---
	"SELECT name FROM employees WHERE department_id = (SELECT id FROM departments WHERE name = 'Engineering')",
	"SELECT name FROM employees WHERE salary > (SELECT AVG(salary) FROM employees)",

	// --- ORDER / LIMIT / OFFSET ---
	"ORDER BY hire_date DESC",
	"LIMIT 10",
	"OFFSET 20",

	// --- VIEWS ---
	"CREATE VIEW active_employees AS SELECT id, name FROM employees WHERE status = 'active'",

	// --- TRANSACTIONS ---
	"START TRANSACTION",
	"COMMIT",
	"ROLLBACK",

	// --- USER & PERMISSIONS (MySQL Specific) ---
	"CREATE USER 'user1'@'localhost' IDENTIFIED BY 'password123'",
	"GRANT SELECT, INSERT ON company_db.* TO 'user1'@'localhost'",
	"REVOKE INSERT ON company_db.* FROM 'user1'@'localhost'",
	"DROP USER 'user1'@'localhost'",

	// --- STORED PROCEDURE TEMPLATE ---
	`DELIMITER //
	CREATE PROCEDURE GetEmployeeByID(IN emp_id INT)
	BEGIN
		SELECT * FROM employees WHERE id = emp_id;
	END //
	DELIMITER ;`,

	// --- TRIGGER TEMPLATE ---
	`CREATE TRIGGER before_insert_employee
	BEFORE INSERT ON employees
	FOR EACH ROW
	SET NEW.hire_date = NOW();`,
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

func showErrorModal(app *tview.Application, layout tview.Primitive, message string) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			app.SetRoot(layout, true)
		})
	app.SetRoot(modal, true)
}

func getDDL(db *sql.DB, query string) (string, error) {
	rows, err := db.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}

	if !rows.Next() {
		return "", fmt.Errorf("no result found")
	}

	values := make([]interface{}, len(cols))
	valuePtrs := make([]interface{}, len(cols))

	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return "", err
	}

	// DDL always at index 2
	if len(values) < 3 {
		return "", fmt.Errorf("unexpected result format")
	}

	switch v := values[2].(type) {
	case []byte:
		return string(v), nil
	case string:
		return v, nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

func exportAllObjects(outputFile string, progressChan chan string, dbName string) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&collation=utf8mb4_unicode_ci&parseTime=false",
		User, Pass, Host, Port, dbName) // parseTime=false keeps raw DB datetime values

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		progressChan <- fmt.Sprintf("[red]Failed to connect to DB: %v", err)
		close(progressChan)
		return
	}

	defer func() {
		db.Close()
	}()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Minute * 5)

	type fileWriters struct {
		table, view, viewddl, procedure, function, trigger, event *bufio.Writer

		tableFile, viewFile, viewddlFile *os.File
		procedureFile, functionFile      *os.File
		triggerFile, eventFile           *os.File

		tableGzip, viewGzip, viewddlGzip *gzip.Writer
		procedureGzip, functionGzip      *gzip.Writer
		triggerGzip, eventGzip           *gzip.Writer
	}

	fw := &fileWriters{}

	openGz := func(suffix string) (*os.File, *gzip.Writer, *bufio.Writer, error) {
		now := time.Now()
		dateFolder := now.Format("2006-01-02_150405")
		fileTimeStamp := now.Format("20060102_150405")
		dirPath := filepath.Join("export", dateFolder, dbName)
		if err := os.MkdirAll(dirPath, 0777); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to create directory: %w", err)
		}
		filename := fmt.Sprintf("%s_%s.gz", fileTimeStamp, suffix)
		fullPath := filepath.Join(dirPath, filename)

		f, err := os.Create(fullPath)
		if err != nil {
			return nil, nil, nil, err
		}
		gz := gzip.NewWriter(f)
		buf := bufio.NewWriter(gz)
		return f, gz, buf, nil
	}

	if fw.tableFile, fw.tableGzip, fw.table, err = openGz("table.sql"); err != nil {
		progressChan <- fmt.Sprintf("[red]Failed to open table file: %v", err)
		close(progressChan)
		return
	}
	if fw.viewFile, fw.viewGzip, fw.view, err = openGz("view.sql"); err != nil {
		progressChan <- fmt.Sprintf("[red]Failed to open view file: %v", err)
		close(progressChan)
		return
	}
	if fw.viewddlFile, fw.viewddlGzip, fw.viewddl, err = openGz("viewddl.sql"); err != nil {
		progressChan <- fmt.Sprintf("[red]Failed to open viewddl file: %v", err)
		close(progressChan)
		return
	}
	if fw.procedureFile, fw.procedureGzip, fw.procedure, err = openGz("procedure.sql"); err != nil {
		progressChan <- fmt.Sprintf("[red]Failed to open procedure file: %v", err)
		close(progressChan)
		return
	}
	if fw.functionFile, fw.functionGzip, fw.function, err = openGz("function.sql"); err != nil {
		progressChan <- fmt.Sprintf("[red]Failed to open function file: %v", err)
		close(progressChan)
		return
	}

	if fw.triggerFile, fw.triggerGzip, fw.trigger, err = openGz("trigger.sql"); err != nil {
		progressChan <- fmt.Sprintf("[red]Failed to open trigger file: %v", err)
		close(progressChan)
		return
	}

	if fw.eventFile, fw.eventGzip, fw.event, err = openGz("event.sql"); err != nil {
		progressChan <- fmt.Sprintf("[red]Failed to open event file: %v", err)
		close(progressChan)
		return
	}

	headers := []string{
		"-- ------------------------------------------------------",
		fmt.Sprintf("-- MySQL Database Export"),
		fmt.Sprintf("-- Host: %s", Host),
		fmt.Sprintf("-- Database: %s", dbName),
		fmt.Sprintf("-- Generated by Go Export Utility"),
		fmt.Sprintf("-- Date: %s", time.Now().Format("2006-01-02 15:04:05")),
		"-- Compatible with MySQL 5.7+ / 8.0+",
		"-- ------------------------------------------------------\n",

		"SET @OLD_CHARACTER_SET_CLIENT = @@CHARACTER_SET_CLIENT;",
		"SET @OLD_CHARACTER_SET_RESULTS = @@CHARACTER_SET_RESULTS;",
		"SET @OLD_COLLATION_CONNECTION = @@COLLATION_CONNECTION;",
		"SET NAMES utf8mb4;",
		"SET @OLD_TIME_ZONE = @@TIME_ZONE;",
		"SET TIME_ZONE = '+00:00';",
		"SET @OLD_UNIQUE_CHECKS = @@UNIQUE_CHECKS;",
		"SET UNIQUE_CHECKS = 0;",
		"SET @OLD_FOREIGN_KEY_CHECKS = @@FOREIGN_KEY_CHECKS;",
		"SET FOREIGN_KEY_CHECKS = 0;",
		"SET @OLD_SQL_MODE = @@SQL_MODE;",
		"SET SQL_MODE = 'NO_AUTO_VALUE_ON_ZERO';",
		"SET @OLD_SQL_NOTES = @@SQL_NOTES;",
		"SET SQL_NOTES = 0;",
		"SET @OLD_AUTOCOMMIT = @@AUTOCOMMIT;",
		"SET AUTOCOMMIT = 0;",
		"SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci;",
		"SET SESSION collation_connection = 'utf8mb4_unicode_ci';\n",
	}

	// writers := []*bufio.Writer{fw.table, fw.view, fw.viewddl, fw.procedure, fw.function}
	writers := []*bufio.Writer{
		fw.table, fw.view, fw.viewddl,
		fw.procedure, fw.function,
		fw.trigger, fw.event,
	}
	for _, writer := range writers {
		for _, header := range headers {
			_, _ = writer.WriteString(header + "\n")
		}
	}

	footers := []string{
		"-- ------------------------------------------------------",
		"-- Restore safe session settings (Universal Compatible)",
		"-- ------------------------------------------------------",
		"/*!40103 SET TIME_ZONE='+00:00' */;",
		"/*!40101 SET SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;",
		"/*!40014 SET FOREIGN_KEY_CHECKS=1 */;",
		"/*!40014 SET UNIQUE_CHECKS=1 */;",
		"/*!40101 SET CHARACTER_SET_CLIENT=utf8mb4 */;",
		"/*!40101 SET CHARACTER_SET_RESULTS=utf8mb4 */;",
		"/*!40101 SET COLLATION_CONNECTION=utf8mb4_unicode_ci */;",
		"/*!40111 SET SQL_NOTES=1 */;",
		"/*!40101 SET AUTOCOMMIT=1 */;",
		"COMMIT;",
		"-- End of Export",
	}

	defer func() {
		for _, writer := range writers {
			for _, footer := range footers {
				_, _ = writer.WriteString(footer + "\n")
			}
			_ = writer.Flush()
		}

		_ = fw.tableGzip.Close()
		_ = fw.tableFile.Close()
		_ = fw.viewGzip.Close()
		_ = fw.viewFile.Close()
		_ = fw.viewddlGzip.Close()
		_ = fw.viewddlFile.Close()
		_ = fw.procedureGzip.Close()
		_ = fw.procedureFile.Close()
		_ = fw.functionGzip.Close()
		_ = fw.functionFile.Close()
		_ = fw.triggerGzip.Close()
		_ = fw.triggerFile.Close()
		_ = fw.eventGzip.Close()
		_ = fw.eventFile.Close()
	}()

	var mu sync.Mutex
	var wg sync.WaitGroup
	workerCount := 130
	tasks := make(chan DBObject, len(allTables))

	totalObjects := len(allTables)
	var completed int64
	startTime := time.Now()

	// Spawn workers
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for obj := range tasks {
				var ddl string
				var writer *bufio.Writer
				var writerddl *bufio.Writer

				switch obj.Type {
				case "TABLE":
					writer = fw.table
					const insertBatchSize = 1000
					var table, createStmt string
					row := db.QueryRow(fmt.Sprintf("SHOW CREATE TABLE `%s`", obj.Name))
					if err := row.Scan(&table, &createStmt); err != nil {
						progressChan <- fmt.Sprintf("[yellow]Failed TABLE: %s - %v", obj.Name, err)
						atomic.AddInt64(&completed, 1)
						continue
					}
					ddl = createStmt

					// WRITE TABLE STRUCTURE
					mu.Lock()
					_, _ = writer.WriteString(fmt.Sprintf(
						"-- ------------------------------------------------------\n"+
							"-- Structure for table `%s`\n"+
							"-- ------------------------------------------------------\n", obj.Name))
					_, _ = writer.WriteString(fmt.Sprintf("DROP TABLE IF EXISTS `%s`;\n", obj.Name))
					_, _ = writer.WriteString(ddl + ";\n\n")
					_, _ = writer.WriteString(fmt.Sprintf("/*!40000 ALTER TABLE `%s` DISABLE KEYS */;\n", obj.Name))
					mu.Unlock()

					// EXPORT TABLE DATA
					rows, err := db.Query(fmt.Sprintf("SELECT * FROM `%s`", obj.Name))
					if err != nil {
						progressChan <- fmt.Sprintf("[yellow]Failed DATA TABLE: %s - %v", obj.Name, err)
						atomic.AddInt64(&completed, 1)
						continue
					}

					cols, _ := rows.Columns()
					colCount := len(cols)
					values := make([]interface{}, colCount)
					valuePtrs := make([]interface{}, colCount)
					colList := "`" + strings.Join(cols, "`, `") + "`"
					var valueRows []string
					rowCount := 0

					for rows.Next() {
						for i := range values {
							valuePtrs[i] = &values[i]
						}
						err := rows.Scan(valuePtrs...)
						if err != nil {
							// skip row on scan error
							continue
						}

						valStrings := make([]string, colCount)
						for i, val := range values {
							switch v := val.(type) {
							case nil:
								valStrings[i] = "NULL"
							case []byte:
								// Detect binary data (contains non-printable bytes)
								isBinary := false
								for _, b := range v {
									if b < 32 && b != 9 && b != 10 && b != 13 {
										isBinary = true
										break
									}
								}
								if isBinary {
									// Encode as hex (MySQL-safe)
									valStrings[i] = fmt.Sprintf("0x%s", hex.EncodeToString(v))
								} else {
									// Treat as normal UTF-8 text
									valStrings[i] = fmt.Sprintf("'%s'", escapeString(string(v)))
								}
							case string:
								valStrings[i] = fmt.Sprintf("'%s'", escapeString(v))
							default:
								valStrings[i] = fmt.Sprintf("'%v'", v)
							}
						}

						valueRows = append(valueRows, fmt.Sprintf("(%s)", strings.Join(valStrings, ", ")))
						rowCount++
						if rowCount >= insertBatchSize {
							mu.Lock()
							_, _ = writer.WriteString(fmt.Sprintf("INSERT INTO `%s` (%s) VALUES\n", obj.Name, colList))
							_, _ = writer.WriteString(strings.Join(valueRows, ",\n") + ";\n\n")
							_ = writer.Flush()
							mu.Unlock()
							valueRows = valueRows[:0]
							rowCount = 0
						}
					}
					rows.Close()

					if len(valueRows) > 0 {
						mu.Lock()
						_, _ = writer.WriteString(fmt.Sprintf("INSERT INTO `%s` (%s) VALUES\n", obj.Name, colList))
						_, _ = writer.WriteString(strings.Join(valueRows, ",\n") + ";\n\n")
						_ = writer.Flush()
						mu.Unlock()
					}
					// Re-enable keys for this table after inserts
					mu.Lock()
					_, _ = writer.WriteString(fmt.Sprintf("/*!40000 ALTER TABLE `%s` ENABLE KEYS */;\n\n", obj.Name))
					mu.Unlock()

				case "VIEW":
					writer = fw.view
					writerddl = fw.viewddl

					// Build synthetic table structure for view (columns/types)
					columnQuery := fmt.Sprintf("SELECT * FROM `%s` LIMIT 0", obj.Name)
					rowsddl, err := db.Query(columnQuery)
					if err != nil {
						progressChan <- fmt.Sprintf("[yellow]Failed VIEW (columns): %s - %v", obj.Name, err)
						atomic.AddInt64(&completed, 1)
						continue
					}
					cols, err := rowsddl.ColumnTypes()
					rowsddl.Close()
					if err != nil {
						progressChan <- fmt.Sprintf("[yellow]Failed VIEW (col types): %s - %v", obj.Name, err)
						atomic.AddInt64(&completed, 1)
						continue
					}

					var structBuilder strings.Builder
					structBuilder.WriteString(fmt.Sprintf("-- STRUCTURE FOR VIEW: %s\n", obj.Name))
					structBuilder.WriteString(fmt.Sprintf("CREATE TABLE `%s` (\n", obj.Name))
					for i, col := range cols {
						colName := col.Name()
						colType := mapSQLType(col.DatabaseTypeName())
						nullable, _ := col.Nullable()
						nullStr := "NOT NULL"
						if nullable {
							nullStr = "NULL"
						}
						colDef := fmt.Sprintf("  `%s` %s %s", colName, colType, nullStr)
						if i < len(cols)-1 {
							colDef += ",\n"
						} else {
							colDef += "\n"
						}
						structBuilder.WriteString(colDef)
					}
					structBuilder.WriteString(");\n\n")

					var view, createStmt, charset, collation string
					row := db.QueryRow(fmt.Sprintf("SHOW CREATE VIEW `%s`", obj.Name))
					if err := row.Scan(&view, &createStmt, &charset, &collation); err != nil {
						progressChan <- fmt.Sprintf("[yellow]Failed CREATE VIEW: %s - %v", obj.Name, err)
						atomic.AddInt64(&completed, 1)
						continue
					}
					ddl = createStmt

					// Write the STRUCTURE for view to viewddl (this was missing previously)
					mu.Lock()
					_, _ = writerddl.WriteString(fmt.Sprintf(
						"-- ------------------------------------------------------\n"+
							"-- Structure for view (DDL as table) `%s`\n"+
							"-- ------------------------------------------------------\n", obj.Name))
					_, _ = writerddl.WriteString(structBuilder.String())
					_ = writerddl.Flush()
					// Drop table with same name if it exists (prevent name conflict)
					_, _ = writer.WriteString(fmt.Sprintf("DROP TABLE IF EXISTS `%s`;\n", obj.Name))

					// Write the actual view DDL to view file
					_, _ = writer.WriteString(fmt.Sprintf(
						"-- ------------------------------------------------------\n"+
							"-- View: %s\n"+
							"-- ------------------------------------------------------\n", obj.Name))
					_, _ = writer.WriteString(fmt.Sprintf("DROP VIEW IF EXISTS `%s`;\n", obj.Name))
					_, _ = writer.WriteString(ddl + ";\n\n")
					_ = writer.Flush()
					mu.Unlock()

				case "PROCEDURE", "FUNCTION":
					if obj.Type == "PROCEDURE" {
						writer = fw.procedure
					} else {
						writer = fw.function
					}
					var name, sqlMode, createStmt, charset, collation, dbCollation string
					row := db.QueryRow(fmt.Sprintf("SHOW CREATE %s `%s`", obj.Type, obj.Name))
					if err := row.Scan(&name, &sqlMode, &createStmt, &charset, &collation, &dbCollation); err != nil {
						progressChan <- fmt.Sprintf("[yellow]Failed %s: %s - %v", obj.Type, obj.Name, err)
						atomic.AddInt64(&completed, 1)
						continue
					}
					ddl = createStmt

					mu.Lock()
					_, _ = writer.WriteString(fmt.Sprintf(
						"-- ------------------------------------------------------\n"+
							"-- %s: %s\n"+
							"-- ------------------------------------------------------\n", obj.Type, obj.Name))
					_, _ = writer.WriteString(fmt.Sprintf("DROP %s IF EXISTS `%s`;\n", obj.Type, obj.Name))
					_, _ = writer.WriteString("DELIMITER //\n")
					_, _ = writer.WriteString(ddl + ";\n//\nDELIMITER ;\n\n")
					_ = writer.Flush()
					mu.Unlock()
				case "TRIGGER":
					writer = fw.trigger

					query := fmt.Sprintf("SHOW CREATE TRIGGER `%s`.`%s`", dbName, obj.Name)
					createStmt, err := getDDL(db, query)
					if err != nil {
						progressChan <- fmt.Sprintf("[yellow]Failed TRIGGER: %s - %v", obj.Name, err)
						atomic.AddInt64(&completed, 1)
						continue
					}

					mu.Lock()
					_, _ = writer.WriteString(fmt.Sprintf(
						"-- ------------------------------------------------------\n"+
							"-- TRIGGER: %s\n"+
							"-- ------------------------------------------------------\n", obj.Name))

					_, _ = writer.WriteString(fmt.Sprintf("DROP TRIGGER IF EXISTS `%s`;\n", obj.Name))
					_, _ = writer.WriteString("DELIMITER //\n")
					_, _ = writer.WriteString(createStmt + ";\n//\nDELIMITER ;\n\n")
					_ = writer.Flush()
					mu.Unlock()

				case "EVENT":
					writer = fw.event

					query := fmt.Sprintf("SHOW CREATE EVENT `%s`.`%s`", dbName, obj.Name)
					createStmt, err := getDDL(db, query)
					if err != nil {
						progressChan <- fmt.Sprintf("[yellow]Failed EVENT: %s - %v", obj.Name, err)
						atomic.AddInt64(&completed, 1)
						continue
					}

					mu.Lock()
					_, _ = writer.WriteString(fmt.Sprintf(
						"-- ------------------------------------------------------\n"+
							"-- EVENT: %s\n"+
							"-- ------------------------------------------------------\n", obj.Name))

					_, _ = writer.WriteString(fmt.Sprintf("DROP EVENT IF EXISTS `%s`;\n", obj.Name))
					_, _ = writer.WriteString("DELIMITER //\n")
					_, _ = writer.WriteString(createStmt + ";\n//\nDELIMITER ;\n\n")
					_ = writer.Flush()
					mu.Unlock()
				}

				atomic.AddInt64(&completed, 1)
				elapsed := time.Since(startTime).Seconds()
				percent := float64(completed) / float64(totalObjects) * 100
				var eta time.Duration
				if completed > 0 {
					eta = time.Duration((elapsed/float64(completed))*float64(totalObjects-int(completed))) * time.Second
				} else {
					eta = 0
				}

				progressChan <- fmt.Sprintf("[green]Exported %-10s: %-30s [%.1f%%] (ETA %s)",
					obj.Type, obj.Name, percent, eta.Round(time.Second))
			}
		}()
	}

	// === ORDER FIX: TABLE → VIEW → PROCEDURE → FUNCTION ===
	var tables, views, procedures, functions, triggers, events []DBObject

	for _, obj := range allTables {
		switch obj.Type {
		case "TABLE":
			tables = append(tables, obj)
		case "VIEW":
			views = append(views, obj)
		case "PROCEDURE":
			procedures = append(procedures, obj)
		case "FUNCTION":
			functions = append(functions, obj)
		case "TRIGGER":
			triggers = append(triggers, obj)
		case "EVENT":
			events = append(events, obj)

		}
	}
	orderedObjects := append(
		append(
			append(
				append(
					append(tables, views...),
					procedures...),
				functions...),
			triggers...),
		events...,
	)

	for _, obj := range orderedObjects {
		tasks <- obj
	}
	close(tasks)
	wg.Wait()
	close(progressChan)
}

func mapSQLType(mysqlType string) string {
	switch strings.ToUpper(mysqlType) {
	case "VARCHAR", "TEXT", "CHAR":
		return "VARCHAR(1)"
	case "INT", "INTEGER", "SMALLINT", "TINYINT", "MEDIUMINT", "BIGINT":
		return "INT(11)"
	case "DECIMAL", "NUMERIC", "FLOAT", "DOUBLE":
		return "DECIMAL(10,2)"
	case "DATE":
		return "DATE"
	case "DATETIME", "TIMESTAMP":
		return "DATETIME"
	case "BLOB", "LONGBLOB", "MEDIUMBLOB":
		return "BLOB"
	default:
		return "VARCHAR(1)"
	}
}

func escapeString(str string) string {
	var buf strings.Builder
	for i := 0; i < len(str); i++ {
		switch str[i] {
		case '\'':
			buf.WriteString(`\'`) // Escape single quotes
		case '"':
			buf.WriteString(`\"`) // Escape double quotes
		case '\\':
			buf.WriteString(`\\`) // Escape backslashes
		case '\n':
			buf.WriteString(`\n`) // ESCAPE newlines, don't remove
		case '\r':
			buf.WriteString(`\r`) // ESCAPE carriage returns, don't remove
		case '\t':
			buf.WriteString(`\t`) // ESCAPE tabs, don't remove
		case '\b':
			buf.WriteString(`\b`) // Escape backspace
		case '\x00':
			buf.WriteString(`\0`) // Escape null bytes
		case '\x1a':
			buf.WriteString(`\Z`) // Escape Ctrl+Z (substitute)
		default:
			buf.WriteByte(str[i]) // Keep all other characters
		}
	}
	return buf.String()
}

func createDbPool(conns []string) []*sql.DB {
	var pool []*sql.DB
	for _, dsn := range conns {
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			log.Fatalf("Failed to open DB: %v", err)
		}
		if err := db.Ping(); err != nil {
			log.Fatalf("Cannot connect to DB: %v", err)
		}
		pool = append(pool, db)
	}
	return pool
}

func getDDLFromShowCreate(db *sql.DB, query string) (string, error) {
	// row := db.QueryRow(query)

	// Use Rows instead of Row for dynamic columns
	rows, err := db.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}

	if !rows.Next() {
		return "", fmt.Errorf("no result found")
	}

	values := make([]interface{}, len(cols))
	valuePtrs := make([]interface{}, len(cols))

	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return "", err
	}

	// 🔥 DDL is always column index 2
	if len(values) < 3 {
		return "", fmt.Errorf("unexpected result format")
	}

	var ddl string
	switch v := values[2].(type) {
	case []byte:
		ddl = string(v)
	case string:
		ddl = v
	default:
		ddl = fmt.Sprintf("%v", v)
	}

	return ddl, nil
}

// func GetTriggerDDL(db *sql.DB, dbName, triggerName string) (string, error) {
// 	query := "SHOW CREATE TRIGGER `" + dbName + "`.`" + triggerName + "`"

// 	row := db.QueryRow(query)

// 	var name, sqlMode, ddl, charset, collation, dbCollation string
// 	var created sql.NullString

// 	err := row.Scan(
// 		&name,
// 		&sqlMode,
// 		&ddl,
// 		&charset,
// 		&collation,
// 		&dbCollation,
// 		&created,
// 	)
// 	if err != nil {
// 		return "", err
// 	}

// 	return ddl, nil
// }

func GetTriggerDDL(db *sql.DB, dbName, triggerName string) (string, error) {
	query := "SHOW CREATE TRIGGER `" + dbName + "`.`" + triggerName + "`"
	return getDDLFromShowCreate(db, query)
}

// func GetEventDDL(db *sql.DB, dbName, eventName string) (string, error) {
// 	query := "SHOW CREATE EVENT `" + dbName + "`.`" + eventName + "`"

// 	row := db.QueryRow(query)

// 	var name, sqlMode, ddl, charset, collation, dbCollation string
// 	var created sql.NullString

// 	err := row.Scan(
// 		&name,
// 		&sqlMode,
// 		&ddl,
// 		&charset,
// 		&collation,
// 		&dbCollation,
// 		&created,
// 	)
// 	if err != nil {
// 		return "", err
// 	}

// 	return ddl, nil
// }

func GetEventDDL(db *sql.DB, dbName, eventName string) (string, error) {
	query := "SHOW CREATE EVENT `" + dbName + "`.`" + eventName + "`"
	return getDDLFromShowCreate(db, query)
}

func setEditorText(queryBox *tview.TextArea, text string) {
	if queryBox != nil {
		queryBox.SetText(text, true)
	}
	if activeSQLEditor != nil {
		activeSQLEditor.SetText(text)
	}
}

func UseDatabase(app *tview.Application, db *sql.DB, dbName string) {
	runIcon := "▶  Run"
	saveIcon := "💾  Save"
	loadIcon := "📂  Load"
	exitIcon := "✖  Exit"

	// Use selected DB
	_, err := db.Exec("USE " + util.QuoteIdentifier(dbName))
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
		SetBorderColor(tcell.ColorDarkGray).
		SetTitleColor(tcell.ColorGray)

	dataBaseList.SetFocusFunc(func() {
		dataBaseList.SetBorderColor(tcell.ColorYellow).
			SetTitleColor(tcell.ColorYellow)
	})
	dataBaseList.SetBlurFunc(func() {
		dataBaseList.SetBorderColor(tcell.ColorDarkGray).
			SetTitleColor(tcell.ColorGray)
	})

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
	tableList = tview.NewList()
	tableList.
		ShowSecondaryText(false).
		SetHighlightFullLine(true)

	tableList.
		SetBorder(true).
		SetTitle(" 🧮 Database Objects ").
		SetTitleAlign(tview.AlignLeft).
		SetBorderColor(tcell.ColorYellow).
		SetTitleColor(tcell.ColorYellow)

	tableList.SetFocusFunc(func() {
		tableList.SetBorderColor(tcell.ColorYellow).
			SetTitleColor(tcell.ColorYellow)
	})
	tableList.SetBlurFunc(func() {
		tableList.SetBorderColor(tcell.ColorDarkGray).
			SetTitleColor(tcell.ColorGray)
	})

	queryAllStructure := `
						SELECT table_name AS name, 'TABLE' AS type 
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
						WHERE routine_schema = '` + dbName + `' AND routine_type = 'FUNCTION'

						UNION ALL

						SELECT trigger_name AS name, 'TRIGGER' AS type 
						FROM information_schema.triggers 
						WHERE trigger_schema = '` + dbName + `'

						UNION ALL

						SELECT event_name AS name, 'EVENT' AS type 
						FROM information_schema.events 
						WHERE event_schema = '` + dbName + `';`

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
			tableList.AddItem("📋 "+dispalyName, "Press Enter to use", 0, func() {
				// app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				// 	if event.Key() == tcell.KeyCtrlX {
				// 		if currentobjectType == "TABLE" {
				// 			query := "SHOW CREATE TABLE " + currentName
				// 			row, err := db.Query(query)
				// 			if err != nil {
				// 				showErrorModal(app, mainFlex, "Failed to fetch table definition: "+err.Error())
				// 				return nil
				// 			}
				// 			defer row.Close()
				// 			var tableName, createStatement string
				// 			if row.Next() {
				// 				err := row.Scan(&tableName, &createStatement)
				// 				if err != nil {
				// 					showErrorModal(app, mainFlex, "Scan failed: "+err.Error())
				// 					return nil
				// 				}
				// 				// Copy to clipboard
				// 				clipboard.WriteAll(createStatement)

				// 				// Optional: Show a confirmation modal
				// 				modal := tview.NewModal().
				// 					SetText("Table definition copied to clipboard.").
				// 					AddButtons([]string{"OK"}).
				// 					SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				// 						layout := CreateLayoutWithFooter(app, mainFlex)
				// 						app.SetRoot(layout, true)
				// 					})
				// 				app.SetRoot(modal, true)
				// 			}
				// 		}
				// 		return nil
				// 	}
				// 	return event
				// })

				// progressView := tview.NewTextView().
				// 	SetDynamicColors(true).
				// 	SetScrollable(true).
				// 	SetChangedFunc(func() {
				// 		app.Draw()
				// 	})

				progressView := tview.NewTextView().
					SetDynamicColors(true).
					SetScrollable(true).
					SetChangedFunc(func() { app.Draw() })
				progressView.SetBorder(true).SetTitle("📦 Export Progress").SetTitleAlign(tview.AlignLeft)

				typePriority := map[string]int{
					"TABLE":     0,
					"VIEW":      1,
					"FUNCTION":  2,
					"PROCEDURE": 3,
					"TRIGGER":   4,
					"EVENT":     5,
				}

				sort.Slice(allTables, func(i, j int) bool {
					return typePriority[allTables[i].Type] < typePriority[allTables[j].Type]
				})
				app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {

					//if event.Key() == tcell.KeyCtrlY {
					//
					//	go func() {
					//		progressChan := make(chan string)
					//
					//		go func() {
					//			for msg := range progressChan {
					//				util.SaveLog(msg)
					//			}
					//		}()
					//		app.QueueUpdateDraw(func() {
					//			progressView.SetText("[blue]Starting export...\n")
					//			app.SetRoot(progressView, true)
					//		})
					//
					//		util.SaveLog(fmt.Sprintf("Exporting %d objects...\n", len(allTables)))
					//
					//		go exportAllObjects("backup.sql", progressChan, dbName)
					//
					//		// Read from progress channel and update UI
					//		go func() {
					//			for msg := range progressChan {
					//				app.QueueUpdateDraw(func() {
					//					fmt.Fprintln(progressView, msg)
					//				})
					//			}
					//
					//			// After export is done
					//			app.QueueUpdateDraw(func() {
					//				modal := tview.NewModal().
					//					SetText("Export completed successfully!").
					//					AddButtons([]string{"OK"}).
					//					SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					//						app.SetRoot(mainFlex, true)
					//					})
					//				app.SetRoot(modal, true)
					//			})
					//		}()
					//	}()
					//	return nil
					//}

					// if event.Key() == tcell.KeyCtrlY {

					// 	go func() {
					// 		progressChan := make(chan string)

					// 		// Log writer goroutine
					// 		go func() {
					// 			for msg := range progressChan {
					// 				util.SaveLog(msg)
					// 			}
					// 		}()

					// 		app.QueueUpdateDraw(func() {
					// 			progressView.SetDynamicColors(true)
					// 			progressView.SetText("[blue]Starting export...\n")
					// 			progressView.ScrollToEnd() // 👈 scroll to bottom immediately
					// 			app.SetRoot(progressView, true)
					// 		})

					// 		util.SaveLog(fmt.Sprintf("Exporting %d objects...\n", len(allTables)))

					// 		// Run export in background
					// 		go exportAllObjects("backup.sql", progressChan, dbName)

					// 		// Read progress and update UI
					// 		go func() {
					// 			for msg := range progressChan {
					// 				app.QueueUpdateDraw(func() {
					// 					fmt.Fprintln(progressView, msg)
					// 					progressView.ScrollToEnd() // 👈 ensure view auto-scrolls
					// 				})
					// 			}

					// 			// After export is complete
					// 			app.QueueUpdateDraw(func() {
					// 				modal := tview.NewModal().
					// 					SetText("Export completed successfully!").
					// 					AddButtons([]string{"OK"}).
					// 					SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					// 						app.SetRoot(mainFlex, true)
					// 					})
					// 				app.SetRoot(modal, true)
					// 			})
					// 		}()
					// 	}()
					// 	return nil
					// }
					if event.Key() == tcell.KeyCtrlY {
						go func() {
							progressChan := make(chan string)

							// ---- STATUS BAR ----
							statusBar := tview.NewTextView().
								SetDynamicColors(true).
								SetTextAlign(tview.AlignCenter)
							statusBar.SetBorder(true).SetTitle("Status")

							// ---- WRAPPER LAYOUT ----
							layout := tview.NewFlex().
								SetDirection(tview.FlexRow).
								AddItem(tview.NewTextView().
									SetText("[::b]  ⏳ Exporting Database  ").
									SetDynamicColors(true), 1, 1, false).
								AddItem(progressView, 0, 1, false).
								AddItem(statusBar, 1, 1, false)

							// ---- LOG WRITER ----
							go func() {
								for msg := range progressChan {
									util.SaveLog(msg)
								}
							}()

							// ---- START EXPORT ----
							app.QueueUpdateDraw(func() {
								progressView.SetText("[blue]Preparing export...\n")
								statusBar.SetText("[yellow]Initializing export process...")
								app.SetRoot(layout, true)
							})

							util.SaveLog(fmt.Sprintf("Exporting %d objects...\n", len(allTables)))

							// ---- SPINNER ----
							spinnerDone := make(chan struct{})
							go func() {
								icons := []string{"|", "/", "-", "\\"}
								i := 0
								for {
									select {
									case <-spinnerDone:
										return
									default:
										app.QueueUpdateDraw(func() {
											statusBar.SetText(fmt.Sprintf("[yellow]Exporting... %s", icons[i%len(icons)]))
										})
										i++
										time.Sleep(150 * time.Millisecond)
									}
								}
							}()

							// ---- ACTUAL EXPORT ----
							go exportAllObjects("backup.sql", progressChan, dbName)

							// ---- UPDATE LOG VIEW ----
							go func() {
								for msg := range progressChan {
									app.QueueUpdateDraw(func() {
										fmt.Fprintln(progressView, msg)
										progressView.ScrollToEnd()
									})
								}

								// ---- DONE ----
								close(spinnerDone)
								app.QueueUpdateDraw(func() {
									statusBar.SetText("[white] Export completed successfully!")
									modal := tview.NewModal().
										SetText("[white::b]Export completed successfully!\n\nBackup File: [white]check export folder for zip").
										AddButtons([]string{"OK"}).
										SetDoneFunc(func(i int, label string) {
											app.SetRoot(mainFlex, true)
										})

									app.SetRoot(modal, true)
								})
							}()
						}()
						return nil
					}

					// High-Speed Parallel Worker Export (Ctrl+W or F8)
					if event.Key() == tcell.KeyCtrlW || event.Key() == tcell.KeyF8 {
						ShowParallelWorkerExportModal(app, db, dbName)
						return nil
					}

					// Real-Time Health Dashboard (F6)
					if event.Key() == tcell.KeyF6 {
						showHealthDashboardModal(app, db)
						return nil
					}

					// Schema Diff & Migration Generator (F7)
					if event.Key() == tcell.KeyF7 {
						showSchemaDiffModal(app, db, dbName)
						return nil
					}

					// Searchable Query History (Ctrl+H)
					if event.Key() == tcell.KeyCtrlH {
						showQueryHistoryModal(app, db, activeSQLEditor)
						return nil
					}

					// Database Import Wizard (F9)
					if event.Key() == tcell.KeyF9 {
						ShowImportWizardModal(app, db, dbName)
						return nil
					}

					// Toggle Sidebar Collapse (Ctrl+B)
					if event.Key() == tcell.KeyCtrlB {
						toggleSidebarVisibility(app)
						return nil
					}

					// Terminal Color Theme Picker (F12)
					if event.Key() == tcell.KeyF12 {
						showThemePickerModal(app)
						return nil
					}

					if event.Key() == tcell.KeyCtrlX {
						if currentobjectType == "TABLE" {
							// Step 1: Get the table's DDL (definition)

							query := "SHOW CREATE TABLE " + currentName
							row, err := db.Query(query)
							if err != nil {
								showErrorModal(app, mainFlex, "Failed to fetch table definition: "+err.Error())
								return nil
							}
							defer row.Close()

							var tableName, createStatement string
							if row.Next() {
								err := row.Scan(&tableName, &createStatement)
								if err != nil {
									showErrorModal(app, mainFlex, "Scan failed: "+err.Error())
									return nil
								}

								// Step 2: Copy the table's DDL (definition) to the clipboard
								err = clipboard.WriteAll(createStatement)
								if err != nil {
									showErrorModal(app, mainFlex, "Failed to copy DDL to clipboard: "+err.Error())
									return nil
								}

								// Step 3: Get the table data (rows)
								db.Exec("USE " + dbName)
								dataQuery := "SELECT * FROM " + currentName
								rows, err := db.Query(dataQuery)
								if err != nil {
									showErrorModal(app, mainFlex, "Failed to fetch table data: "+err.Error())
									return nil
								}
								defer rows.Close()

								// Fetch column names
								columns, err := rows.Columns()
								if err != nil {
									showErrorModal(app, mainFlex, "Failed to get columns: "+err.Error())
									return nil
								}

								var insertStatements []string
								for rows.Next() {
									values := make([]interface{}, len(columns))
									pointers := make([]interface{}, len(columns))
									for i := range values {
										pointers[i] = &values[i]
									}

									err := rows.Scan(pointers...)
									if err != nil {
										showErrorModal(app, mainFlex, "Failed to scan row: "+err.Error())
										return nil
									}

									// Build the insert statement for the current row
									var valuesList []string
									for _, val := range values {
										if val != nil {
											switch v := val.(type) {
											case []byte:
												valuesList = append(valuesList, fmt.Sprintf("'%s'", string(v)))
											default:
												valuesList = append(valuesList, fmt.Sprintf("'%v'", v))
											}
										} else {
											valuesList = append(valuesList, "NULL")
										}
									}

									insertStatement := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);", currentName, strings.Join(columns, ", "), strings.Join(valuesList, ", "))
									insertStatements = append(insertStatements, insertStatement)
								}

								// Step 4: Join all insert statements and copy them to clipboard
								dataString := strings.Join(insertStatements, "\n")

								clipboardText := util.GetClipboardText()
								err = clipboard.WriteAll(clipboardText + "\n" + dataString)
								if err != nil {
									showErrorModal(app, mainFlex, "Failed to copy data to clipboard: "+err.Error())
									return nil
								}

								// Optional: Show a confirmation modal
								modal := tview.NewModal().
									SetText("Table definition and data copied to clipboard as SQL INSERT statements.").
									AddButtons([]string{"OK"}).
									SetDoneFunc(func(buttonIndex int, buttonLabel string) {
										layout := CreateLayoutWithFooter(app, mainFlex)
										app.SetRoot(layout, true)
									})
								app.SetRoot(modal, true)
							}
						}

						if currentobjectType == "VIEW" {
							query := "SHOW CREATE VIEW " + currentName
							row, err := db.Query(query)
							if err != nil {
								showErrorModal(app, mainFlex, "Failed to fetch view definition: "+err.Error())
								return nil
							}
							defer row.Close()

							var viewName, createStatement, charset, collation string
							if row.Next() {
								err := row.Scan(&viewName, &createStatement, &charset, &collation)
								if err != nil {
									showErrorModal(app, mainFlex, "Scan failed: "+err.Error())
									return nil
								}

								// Copy the CREATE VIEW statement to clipboard
								clipboard.WriteAll(createStatement)

								modal := tview.NewModal().
									SetText("View definition copied to clipboard.").
									AddButtons([]string{"OK"}).
									SetDoneFunc(func(buttonIndex int, buttonLabel string) {
										layout := CreateLayoutWithFooter(app, mainFlex)
										app.SetRoot(layout, true)
									})
								app.SetRoot(modal, true)
							}
						}
						return nil
					}
					return event
				})

				switch currentobjectType {
				case "PROCEDURE":
					query := `SELECT routine_name, data_type, is_deterministic, security_type, definer, routine_definition
					FROM INFORMATION_SCHEMA.ROUTINES
					WHERE ROUTINE_NAME = '` + currentName + `'
					AND ROUTINE_SCHEMA = '` + dbName + `' AND ROUTINE_TYPE = 'PROCEDURE';`
					routineDefinition, err := ExeQueryToData(db, currentName, query, dbName, "PROCEDURE")
					if err != nil {
						showErrorModal(app, mainFlex, "Failed to fetch procedure: "+err.Error())
						return
					}
					setEditorText(queryBox, routineDefinition)
					app.SetFocus(activeSQLEditor.Editor)

				case "FUNCTION":
					query := `SELECT routine_name, data_type, is_deterministic, security_type, definer, routine_definition
					FROM INFORMATION_SCHEMA.ROUTINES
					WHERE ROUTINE_NAME = '` + currentName + `'
					AND ROUTINE_SCHEMA = '` + dbName + `' AND ROUTINE_TYPE = 'FUNCTION';`
					routineDefinition, err := ExeQueryToData(db, currentName, query, dbName, "FUNCTION")
					if err != nil {
						showErrorModal(app, mainFlex, "Failed to fetch function: "+err.Error())
						return
					}
					setEditorText(queryBox, routineDefinition)
					app.SetFocus(activeSQLEditor.Editor)

				case "TRIGGER":
					definition, err := GetTriggerDDL(db, dbName, currentName)
					if err != nil {
						showErrorModal(app, mainFlex, "Failed to fetch trigger: "+err.Error())
						return
					}
					setEditorText(queryBox, definition)
					app.SetFocus(activeSQLEditor.Editor)

				case "EVENT":
					definition, err := GetEventDDL(db, dbName, currentName)
					if err != nil {
						showErrorModal(app, mainFlex, "Failed to fetch event: "+err.Error())
						return
					}
					setEditorText(queryBox, definition)
					app.SetFocus(activeSQLEditor.Editor)

				case "TABLE", "VIEW":
					activeGridObject = currentName
					activeGridObjectType = currentobjectType
					activeGridDBName = dbName
					currentGridPage = 0

					query := fmt.Sprintf("SELECT * FROM %s.%s LIMIT 100",
						util.QuoteIdentifier(dbName),
						util.QuoteIdentifier(currentName))
					setEditorText(queryBox, query)
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

		queryBox = tview.NewTextArea()
		queryBox.
			SetPlaceholder("Enter SQL query here...").
			SetBorder(true).
			SetTitle(" 📝 [::b]SQL Query Editor[::-] [white]([green]Ctrl+R:[-]Run  [green]F11:[-]Fullscreen  [green]Tab:[-]Next) ").
			SetTitleAlign(tview.AlignLeft).
			SetBorderColor(tcell.ColorLightCyan).
			SetTitleColor(tcell.ColorAqua)

		activeSQLEditor = NewSQLEditor(app)
		activeSQLEditor.OnExecute = func(query string) {
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
		}
		activeSQLEditor.OnNextFocus = func() {
			app.SetFocus(runButton)
		}
		activeSQLEditor.OnExit = func() {
			layout := CreateLayoutWithFooter(app, mainFlex)
			app.SetRoot(layout, true)
			app.SetFocus(tableList)
		}
		activeSQLEditor.OnFullscreen = func() {
			app.SetRoot(activeSQLEditor.Container, true)
		}

		activeSQLEditor.Editor.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
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
				app.SetRoot(activeSQLEditor.Container, true)
				return nil
			case tcell.KeyCtrlR:
				query := activeSQLEditor.GetText()
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

			case tcell.KeyCtrlP:
				clipboardText := util.GetClipboardText()
				setEditorText(queryBox, clipboardText)
				app.SetFocus(activeSQLEditor.Editor)
				return nil

			case tcell.KeyCtrlT:
				activeSQLEditor.ShowTableSuggestionsModal()
				return nil

			case tcell.KeyCtrlS, tcell.KeyCtrlSpace:
				activeSQLEditor.ShowSnippetsModal()
				return nil
			}

			return activeSQLEditor.handleInput(event)
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
							query := activeSQLEditor.GetText()

							if fileName == "" {
								fileName = "query.sql"
							}
							err := os.WriteFile(fileName, []byte(query), 0644)
							if err != nil {
								modal := tview.NewModal().
									SetText("Failed to save file: " + err.Error()).
									AddButtons([]string{"OK"}).
									SetDoneFunc(func(buttonIndex int, buttonLabel string) {
										layout := CreateLayoutWithFooter(app, mainFlex)
										app.SetRoot(layout, true)
										app.SetFocus(activeSQLEditor.Editor)
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
				app.SetFocus(activeSQLEditor.Editor)
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
				app.SetFocus(activeSQLEditor.Editor)
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
				app.SetFocus(activeSQLEditor.Editor)
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
				app.SetFocus(activeSQLEditor.Editor)
				return nil
			}
			if event.Key() == tcell.KeyTab {
				app.SetFocus(dataTable)
				return nil
			}
			return event
		})

		queryPanel := tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(activeSQLEditor.Container, 6, 1, true).
			AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
				AddItem(buttonBox, 0, 1, false).
				AddItem(saveButtonBox, 0, 1, false).
				AddItem(loadButtonBox, 0, 1, false).
				AddItem(exitButtonBox, 0, 1, false), 1, 0, false)

		dataTable = tview.NewTable()
		dataTable.
			SetBorders(true).
			SetSelectable(true, false).
			SetFixed(1, 0).
			SetTitle(" 📊 Result Data Grid [white](Ctrl+N: Next | Ctrl+P: Prev | Ctrl+E: Export | F3: Schema) ").
			SetTitleAlign(tview.AlignLeft).
			SetBorder(true).
			SetBorderColor(tcell.ColorDarkGray).
			SetTitleColor(tcell.ColorGray)

		dataTable.SetFocusFunc(func() {
			dataTable.SetBorderColor(tcell.ColorYellow).
				SetTitleColor(tcell.ColorYellow)
		})
		dataTable.SetBlurFunc(func() {
			dataTable.SetBorderColor(tcell.ColorDarkGray).
				SetTitleColor(tcell.ColorGray)
		})

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
				return nil
			}
			if event.Key() == tcell.KeyCtrlE {
				ShowExportWizardModal(app, db, dbName)
				return nil
			}
			if event.Key() == tcell.KeyCtrlB {
				toggleSidebarVisibility(app)
				return nil
			}
			if event.Key() == tcell.KeyRune && (event.Rune() == 'v' || event.Rune() == 'V') {
				r, c := dataTable.GetSelection()
				cell := dataTable.GetCell(r, c)
				if cell != nil {
					ShowCellInspectorModal(app, cell.Text)
					return nil
				}
			}
			return event
		})

		searchInput := tview.NewInputField()
		searchInput.SetFieldBackgroundColor(tcell.Color234).
			SetLabel(" 🔍 Filter: ").
			SetPlaceholder("Type to search objects...").
			SetFieldWidth(30)

		searchInput.SetFocusFunc(func() {
			searchInput.SetBorder(true).
				SetBorderColor(tcell.ColorYellow).
				SetTitleColor(tcell.ColorYellow)
		})
		searchInput.SetBlurFunc(func() {
			searchInput.SetBorder(false)
		})
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

		tableList.
			SetBorder(true).
			SetTitle(" 📋 Schema Objects ").
			SetTitleAlign(tview.AlignLeft).
			SetBorderColor(tcell.ColorDarkCyan)

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

		dataBaseList.
			SetBorder(true).
			SetTitle(" 🗂️ Databases ").
			SetTitleAlign(tview.AlignLeft).
			SetBorderColor(tcell.ColorDarkCyan)

		dataBaseList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyTab {
				app.SetFocus(activeSQLEditor.Editor)
				return nil
			}
			return event
		})

		mainLeftPanel = tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(searchInput, 1, 0, false).
			AddItem(tableList, 0, 1, true).
			AddItem(dataBaseList, 0, 1, true)

		// Center panel: Query + Data Table
		mainCenterPanel = tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(queryPanel, 6, 1, true).
			AddItem(dataTable, 0, 3, false)

		// Main layout
		mainFlex = tview.NewFlex().
			AddItem(mainLeftPanel, 0, 1, true).   // use mainLeftPanel
			AddItem(mainCenterPanel, 0, 5, false) // center content
		layout := CreateLayoutWithFooter(app, mainFlex)
		app.SetRoot(layout, true)
	}
}

var isSidebarCollapsed bool
var mainLeftPanel *tview.Flex
var mainCenterPanel *tview.Flex

func toggleSidebarVisibility(app *tview.Application) {
	if mainFlex == nil || mainLeftPanel == nil || mainCenterPanel == nil {
		return
	}
	isSidebarCollapsed = !isSidebarCollapsed
	mainFlex.Clear()
	if isSidebarCollapsed {
		mainFlex.AddItem(mainCenterPanel, 0, 1, true)
		updateFooterText("Sidebar collapsed (Full width view). Press Ctrl+B to restore.")
	} else {
		mainFlex.AddItem(mainLeftPanel, 0, 1, false).
			AddItem(mainCenterPanel, 0, 5, true)
		updateFooterText("Sidebar restored.")
	}
}

// Get primary key column names dynamically
func GetPrimaryKeyColumns(db *sql.DB, dbName, tableName string) ([]string, error) {
	query := `
	SELECT COLUMN_NAME
	FROM INFORMATION_SCHEMA.COLUMNS
	WHERE TABLE_SCHEMA = ?
	  AND TABLE_NAME = ?
	  AND COLUMN_KEY = 'PRI'
	ORDER BY ORDINAL_POSITION
	`
	rows, err := db.Query(query, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err == nil {
			cols = append(cols, col)
		}
	}
	return cols, nil
}

func GetPrimaryKeyColumn(db *sql.DB, dbName, tableName string) (string, error) {
	cols, err := GetPrimaryKeyColumns(db, dbName, tableName)
	if err != nil || len(cols) == 0 {
		return "", err
	}
	return cols[0], nil
}

// Fetch data and show in table with query timing, pagination, and keybindings
func ExecuteQuery(app *tview.Application, db *sql.DB, query string, table *tview.Table) error {
	startTime := time.Now()
	activeGridQuery = query
	activeGridDB = db

	cleanQ := strings.TrimSpace(strings.ToUpper(query))
	isMutation := strings.HasPrefix(cleanQ, "INSERT") ||
		strings.HasPrefix(cleanQ, "UPDATE") ||
		strings.HasPrefix(cleanQ, "DELETE") ||
		strings.HasPrefix(cleanQ, "DROP") ||
		strings.HasPrefix(cleanQ, "TRUNCATE") ||
		strings.HasPrefix(cleanQ, "ALTER") ||
		strings.HasPrefix(cleanQ, "REPLACE") ||
		strings.HasPrefix(cleanQ, "CREATE")

	if isMutation && ActiveReadOnly {
		showErrorModal(app, mainFlex, "🔒 Action Blocked: Active Connection is in READ-ONLY Mode.")
		return fmt.Errorf("read-only mode enabled")
	}

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

	scanValues := make([]interface{}, len(columns))
	rawValues := make([]sql.RawBytes, len(columns))
	for i := range rawValues {
		scanValues[i] = &rawValues[i]
	}

	rowIndex := 1
	for rows.Next() {
		err := rows.Scan(scanValues...)
		if err != nil {
			continue
		}

		for i, col := range rawValues {
			var text string
			if col == nil {
				text = "[gray]NULL"
			} else {
				text = string(col) // Make a string copy of the raw byte slice
				if text == "" {
					text = "[gray]EMPTY"
				}
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

	duration := time.Since(startTime)
	rowCount := rowIndex - 1

	envTagStr := "[green::b]DEV[-::-]"
	if ActiveEnv == "PROD" {
		envTagStr = "[red::b]PROD[-::-]"
	} else if ActiveEnv == "STAGING" {
		envTagStr = "[yellow::b]STAGING[-::-]"
	}

	titleStr := fmt.Sprintf(" [::b]Query Result [%s] [white](%d rows, %s) | Pg %d | [yellow]Ctrl+N/P[::-]:Pg [yellow]Ctrl+E[::-]:Export [yellow]F3[::-]:Schema [yellow]F4[::-]:Top [yellow]F5[::-]:Plan ",
		envTagStr, rowCount, duration.Round(time.Microsecond), currentGridPage+1)
	table.SetTitle(titleStr).SetTitleAlign(tview.AlignLeft).SetBorder(true)

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyTab:
			app.SetFocus(tableList)
			return nil

		case event.Key() == tcell.KeyEscape:
			app.SetFocus(tableList)
			layout := CreateLayoutWithFooter(app, mainFlex)
			app.SetRoot(layout, true)
			return nil

		case event.Key() == tcell.KeyCtrlE:
			if activeGridDB != nil && activeGridDBName != "" {
				ShowExportWizardModal(app, activeGridDB, activeGridDBName)
			} else {
				showExportResultsModal(app, table)
			}
			return nil

		case event.Key() == tcell.KeyF4:
			if activeGridDB != nil {
				ShowProcessListModal(app, activeGridDB)
			}
			return nil

		case event.Key() == tcell.KeyF5:
			if activeGridDB != nil && activeGridQuery != "" {
				showExplainModal(app, activeGridDB, activeGridQuery)
			}
			return nil

		case event.Key() == tcell.KeyF3 || event.Key() == tcell.KeyCtrlK:
			if activeGridObject != "" && activeGridDBName != "" {
				showSchemaInspectorModal(app, activeGridDB, activeGridDBName, activeGridObject)
			} else {
				showHelpModal(app)
			}
			return nil

		case event.Key() == tcell.KeyF1 || (event.Key() == tcell.KeyRune && event.Rune() == '?'):
			showHelpModal(app)
			return nil

		case event.Key() == tcell.KeyCtrlN:
			// Next Page
			if activeGridObjectType == "TABLE" || activeGridObjectType == "VIEW" {
				currentGridPage++
				offset := currentGridPage * gridPageSize
				pageQuery := fmt.Sprintf("SELECT * FROM %s.%s LIMIT %d OFFSET %d",
					util.QuoteIdentifier(activeGridDBName),
					util.QuoteIdentifier(activeGridObject),
					gridPageSize, offset)
				ExecuteQuery(app, activeGridDB, pageQuery, table)
			}
			return nil

		case event.Key() == tcell.KeyCtrlP:
			// Previous Page
			if currentGridPage > 0 && (activeGridObjectType == "TABLE" || activeGridObjectType == "VIEW") {
				currentGridPage--
				offset := currentGridPage * gridPageSize
				pageQuery := fmt.Sprintf("SELECT * FROM %s.%s LIMIT %d OFFSET %d",
					util.QuoteIdentifier(activeGridDBName),
					util.QuoteIdentifier(activeGridObject),
					gridPageSize, offset)
				ExecuteQuery(app, activeGridDB, pageQuery, table)
			}
			return nil
		}

		return event
	})

	return nil
}

// Enable editing and database update with support for composite keys and keyless tables
func EnableCellEditing(app *tview.Application, table *tview.Table, db *sql.DB, dbName, tableName string) error {
	pkCols, err := GetPrimaryKeyColumns(db, dbName, tableName)
	if err != nil {
		util.SaveLog("Error getting primary key columns: " + err.Error())
	}

	table.SetSelectable(true, true)
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorWhite).
		Foreground(tcell.ColorBlack))

	table.SetSelectedFunc(func(row int, column int) {
		if row == 0 {
			return // Skip header row
		}

		if ActiveReadOnly {
			showErrorModal(app, mainFlex, "🔒 Action Blocked: Active Connection is in READ-ONLY Mode. Inline cell editing is disabled.")
			return
		}

		cell := table.GetCell(row, column)
		currentValue := cell.Text

		headerCell := table.GetCell(0, column)
		columnName := util.StripFormatting(headerCell.Text)

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

				var updateQuery string
				var queryArgs []interface{}
				queryArgs = append(queryArgs, newValue)

				if len(pkCols) > 0 {
					var whereClauses []string
					for _, pkCol := range pkCols {
						for colIdx := 0; colIdx < table.GetColumnCount(); colIdx++ {
							colHeader := util.StripFormatting(table.GetCell(0, colIdx).Text)
							if colHeader == pkCol {
								val := table.GetCell(row, colIdx).Text
								whereClauses = append(whereClauses, fmt.Sprintf("%s = ?", util.QuoteIdentifier(pkCol)))
								queryArgs = append(queryArgs, val)
								break
							}
						}
					}
					updateQuery = fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s",
						util.QuoteIdentifier(tableName),
						util.QuoteIdentifier(columnName),
						strings.Join(whereClauses, " AND "))
				} else {
					var whereClauses []string
					for colIdx := 0; colIdx < table.GetColumnCount(); colIdx++ {
						colHeader := util.StripFormatting(table.GetCell(0, colIdx).Text)
						val := table.GetCell(row, colIdx).Text
						if val == "[gray]NULL" || val == "" {
							whereClauses = append(whereClauses, fmt.Sprintf("%s IS NULL", util.QuoteIdentifier(colHeader)))
						} else {
							whereClauses = append(whereClauses, fmt.Sprintf("%s = ?", util.QuoteIdentifier(colHeader)))
							queryArgs = append(queryArgs, val)
						}
					}
					updateQuery = fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s LIMIT 1",
						util.QuoteIdentifier(tableName),
						util.QuoteIdentifier(columnName),
						strings.Join(whereClauses, " AND "))
				}

				_, execErr := db.Exec(updateQuery, queryArgs...)
				if execErr != nil {
					util.SaveLog("Update error: " + execErr.Error())
					showErrorModal(app, mainFlex, "Update failed: "+execErr.Error())
					return nil
				}

				cell.SetText(newValue)
				fullQuery := phhistory.ReplacePlaceholders(updateQuery, queryArgs...)
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

func showExportResultsModal(app *tview.Application, table *tview.Table) {
	if table.GetRowCount() <= 1 {
		showErrorModal(app, mainFlex, "No data rows available to export.")
		return
	}

	form := tview.NewForm()
	filePathInput := tview.NewInputField().
		SetLabel("File Path: ").
		SetText("export_result.csv").
		SetFieldWidth(30)

	formatDropdown := tview.NewDropDown().
		SetLabel("Format: ").
		SetOptions([]string{"Full Database SQL Dump (.sql)", "Result Grid CSV (.csv)", "Result Grid JSON (.json)"}, nil).
		SetCurrentOption(0)

	form.AddFormItem(filePathInput)
	form.AddFormItem(formatDropdown)

	form.AddButton("Export", func() {
		filePath := filePathInput.GetText()
		_, formatOpt := formatDropdown.GetCurrentOption()

		if strings.Contains(formatOpt, "SQL") {
			if activeGridDB != nil && activeGridDBName != "" {
				ShowExportWizardModal(app, activeGridDB, activeGridDBName)
			} else {
				showErrorModal(app, mainFlex, "No active database connection found for SQL export.")
			}
			return
		}

		if filePath == "" {
			showErrorModal(app, mainFlex, "File path cannot be empty.")
			return
		}

		cols := table.GetColumnCount()
		rows := table.GetRowCount()

		f, err := os.Create(filePath)
		if err != nil {
			showErrorModal(app, mainFlex, "Failed to create file: "+err.Error())
			return
		}
		defer f.Close()

		if formatOpt == "CSV" {
			var lines []string
			var headers []string
			for c := 0; c < cols; c++ {
				headers = append(headers, fmt.Sprintf("%q", util.StripFormatting(table.GetCell(0, c).Text)))
			}
			lines = append(lines, strings.Join(headers, ","))

			for r := 1; r < rows; r++ {
				var rowVals []string
				for c := 0; c < cols; c++ {
					val := table.GetCell(r, c).Text
					if val == "[gray]NULL" {
						val = ""
					}
					rowVals = append(rowVals, fmt.Sprintf("%q", val))
				}
				lines = append(lines, strings.Join(rowVals, ","))
			}
			f.WriteString(strings.Join(lines, "\n") + "\n")
		} else {
			var jsonRows []map[string]string
			for r := 1; r < rows; r++ {
				rowMap := make(map[string]string)
				for c := 0; c < cols; c++ {
					header := util.StripFormatting(table.GetCell(0, c).Text)
					val := table.GetCell(r, c).Text
					if val == "[gray]NULL" {
						val = "NULL"
					}
					rowMap[header] = val
				}
				jsonRows = append(jsonRows, rowMap)
			}
			data, _ := json.MarshalIndent(jsonRows, "", "  ")
			f.Write(data)
		}

		modal := tview.NewModal().
			SetText(fmt.Sprintf("Exported %d rows to %s successfully!", rows-1, filePath)).
			AddButtons([]string{"OK"}).
			SetDoneFunc(func(i int, label string) {
				app.SetRoot(mainFlex, true)
				util.SetFocusWithBorder(app, table)
			})
		app.SetRoot(modal, true)
	})

	form.AddButton("Cancel", func() {
		app.SetRoot(mainFlex, true)
		util.SetFocusWithBorder(app, table)
	})

	form.SetBorder(true).SetTitle(" Export Grid Data ").SetTitleAlign(tview.AlignCenter)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(form, 11, 1, true).
		AddItem(nil, 0, 1, false)

	app.SetRoot(flex, true).SetFocus(form)
}

func showSchemaInspectorModal(app *tview.Application, db *sql.DB, dbName, tableName string) {
	colTable := tview.NewTable().SetBorders(true)
	colTable.SetTitle(fmt.Sprintf(" 📋 Columns of `%s` ", tableName)).SetBorder(true)

	colQuery := fmt.Sprintf("SHOW COLUMNS FROM %s.%s", util.QuoteIdentifier(dbName), util.QuoteIdentifier(tableName))
	rows, err := db.Query(colQuery)
	if err != nil {
		showErrorModal(app, mainFlex, "Failed to inspect columns: "+err.Error())
		return
	}
	defer rows.Close()

	headers := []string{"Field", "Type", "Null", "Key", "Default", "Extra"}
	for i, h := range headers {
		colTable.SetCell(0, i, tview.NewTableCell("[::b]"+h).SetTextColor(tcell.ColorYellow).SetAlign(tview.AlignCenter))
	}

	rIdx := 1
	for rows.Next() {
		var field, typ, nullStr, keyStr, extraStr sql.NullString
		var defStr sql.NullString
		if err := rows.Scan(&field, &typ, &nullStr, &keyStr, &defStr, &extraStr); err == nil {
			colTable.SetCell(rIdx, 0, tview.NewTableCell(field.String).SetTextColor(tcell.ColorGreen))
			colTable.SetCell(rIdx, 1, tview.NewTableCell(typ.String).SetTextColor(tcell.ColorWhite))
			colTable.SetCell(rIdx, 2, tview.NewTableCell(nullStr.String).SetTextColor(tcell.ColorLightGray))
			colTable.SetCell(rIdx, 3, tview.NewTableCell(keyStr.String).SetTextColor(tcell.ColorAqua))
			colTable.SetCell(rIdx, 4, tview.NewTableCell(defStr.String).SetTextColor(tcell.ColorLightGray))
			colTable.SetCell(rIdx, 5, tview.NewTableCell(extraStr.String).SetTextColor(tcell.ColorGray))
			rIdx++
		}
	}

	idxTable := tview.NewTable().SetBorders(true)
	idxTable.SetTitle(fmt.Sprintf(" 🔑 Indexes of `%s` ", tableName)).SetBorder(true)

	idxQuery := fmt.Sprintf("SHOW INDEX FROM %s.%s", util.QuoteIdentifier(dbName), util.QuoteIdentifier(tableName))
	idxRows, err := db.Query(idxQuery)
	if err == nil {
		defer idxRows.Close()
		idxHeaders := []string{"Key_name", "Column_name", "Non_unique", "Index_type"}
		for i, h := range idxHeaders {
			idxTable.SetCell(0, i, tview.NewTableCell("[::b]"+h).SetTextColor(tcell.ColorYellow).SetAlign(tview.AlignCenter))
		}
		irIdx := 1
		for idxRows.Next() {
			cols, _ := idxRows.Columns()
			scanVals := make([]interface{}, len(cols))
			for i := range scanVals {
				scanVals[i] = new(sql.NullString)
			}
			if scanErr := idxRows.Scan(scanVals...); scanErr == nil {
				kName := scanVals[2].(*sql.NullString).String
				cName := scanVals[4].(*sql.NullString).String
				nuName := scanVals[1].(*sql.NullString).String
				iType := scanVals[10].(*sql.NullString).String

				idxTable.SetCell(irIdx, 0, tview.NewTableCell(kName).SetTextColor(tcell.ColorAqua))
				idxTable.SetCell(irIdx, 1, tview.NewTableCell(cName).SetTextColor(tcell.ColorGreen))
				idxTable.SetCell(irIdx, 2, tview.NewTableCell(nuName).SetTextColor(tcell.ColorWhite))
				idxTable.SetCell(irIdx, 3, tview.NewTableCell(iType).SetTextColor(tcell.ColorLightGray))
				irIdx++
			}
		}
	}

	colTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			app.SetFocus(idxTable)
			return nil
		}
		return event
	})

	idxTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			app.SetFocus(colTable)
			return nil
		}
		return event
	})

	hintView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[yellow]Press [white::b]ESC[::-][yellow] or [white::b]ENTER[::-][yellow] to close inspector | [white::b]TAB[::-][yellow] to switch between Columns & Indexes")

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(colTable, 0, 2, true).
		AddItem(idxTable, 0, 1, false).
		AddItem(hintView, 1, 0, false)

	layout.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyEnter {
			app.SetRoot(mainFlex, true)
			if dataTable != nil {
				util.SetFocusWithBorder(app, dataTable)
			} else {
				app.SetFocus(tableList)
			}
			return nil
		}
		return event
	})

	app.SetRoot(layout, true).SetFocus(colTable)
}

func showHelpModal(app *tview.Application) {
	helpText := `
 [lime::b]PHERI KEYBOARD SHORTCUTS REFERENCE[::-]

 [yellow::b]General Shortcuts[::-]
   [green]F1 / ?[-]        Show this Help Legend
   [green]Tab[-]           Switch Focus between Panes
   [green]Esc[-]           Return Focus / Close Dialogs
   [green]Ctrl+Q[-]        Quit Application

 [yellow::b]Search & Navigation[::-]
   [green]table:kw[-]      Filter Tables
   [green]view:kw[-]       Filter Views
   [green]procedure:kw[-]  Filter Stored Procedures
   [green]function:kw[-]   Filter Functions
   [green]db:kw[-]         Filter Databases

 [yellow::b]Query Editor Shortcuts[::-]
   [green]Ctrl+R[-]        Execute Query
   [green]Ctrl+F11[-]      Full Screen Editor
   [green]Ctrl+T[-]        Insert Tables List
   [green]Ctrl+S[-]        SQL Keywords Popup
   [green]Ctrl+_[-]        SQL Templates Popup

 [yellow::b]Data Grid Shortcuts[::-]
   [green]Enter[-]         Edit Selected Cell
   [green]Ctrl+N[-]        Next Page (100 Rows)
   [green]Ctrl+P[-]        Previous Page (100 Rows)
   [green]Ctrl+E[-]        Export Results (CSV / JSON)
   [green]F3 / Ctrl+K[-]   Table Schema & Index Inspector
   [green]Ctrl+X[-]        Copy DDL & Inserts to Clipboard

 [yellow::b]Backup Tools[::-]
   [green]Ctrl+Y[-]        Export Full Database (Gzip)
   [gray]Ctrl+I[-]        Import Database Dump (Disabled)

 [white]Press ESC or ENTER to close this window.
`

	modal := tview.NewTextView().
		SetDynamicColors(true).
		SetText(helpText)
	modal.SetBorder(true).SetTitle(" ❓ Keyboard Shortcuts ").SetTitleAlign(tview.AlignCenter)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(modal, 26, 1, true).
		AddItem(nil, 0, 1, false)

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyEnter {
			app.SetRoot(mainFlex, true)
			return nil
		}
		return event
	})

	app.SetRoot(flex, true).SetFocus(modal)
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
						setEditorText(queryBox, string(content))
						if activeSQLEditor != nil {
							app.SetFocus(activeSQLEditor.Editor)
						} else {
							app.SetFocus(queryBox)
						}
					}
					app.SetRoot(returnTo, true)
				}
			}(fullPath))
		}
	}

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

func showExplainModal(app *tview.Application, db *sql.DB, query string) {
	if db == nil || strings.TrimSpace(query) == "" {
		showErrorModal(app, mainFlex, "No active database connection or query available to explain.")
		return
	}

	cleanQ := strings.TrimRight(strings.TrimSpace(query), ";")
	explainQuery := "EXPLAIN " + cleanQ

	rows, err := db.Query(explainQuery)
	if err != nil {
		showErrorModal(app, mainFlex, "EXPLAIN execution failed: "+err.Error())
		return
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		showErrorModal(app, mainFlex, "Failed to fetch EXPLAIN column metadata: "+err.Error())
		return
	}

	summaryBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	table := tview.NewTable().SetBorders(true).SetSelectable(true, false)
	table.SetSelectedStyle(tcell.StyleDefault.Foreground(tcell.ColorYellow).Background(tcell.ColorDarkBlue).Bold(true))
	table.SetBorder(true).
		SetTitle(" 🔍 EXPLAIN Execution Plan & Performance Analyzer ").
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorYellow).
		SetBackgroundColor(tcell.ColorBlack)

	headerStyle := tcell.StyleDefault.
		Foreground(tcell.ColorBlack).
		Background(tcell.ColorYellow).
		Bold(true)

	for i, col := range columns {
		table.SetCell(0, i, tview.NewTableCell(" "+col+" ").SetStyle(headerStyle).SetSelectable(false))
	}

	scanVals := make([]interface{}, len(columns))
	rawVals := make([]sql.RawBytes, len(columns))
	for i := range rawVals {
		scanVals[i] = &rawVals[i]
	}

	var warnings []string
	hasFullTableScan := false
	hasFilesort := false
	hasTempTable := false
	usedKeys := []string{}
	scannedTables := []string{}
	totalScannedRows := int64(0)

	rIdx := 1
	for rows.Next() {
		if err := rows.Scan(scanVals...); err == nil {
			for c, rByte := range rawVals {
				valStr := string(rByte)
				if rByte == nil {
					valStr = "NULL"
				}

				colName := strings.ToLower(columns[c])
				if colName == "table" && valStr != "NULL" {
					scannedTables = append(scannedTables, valStr)
				}
				if colName == "type" && strings.EqualFold(valStr, "ALL") {
					hasFullTableScan = true
				}
				if colName == "extra" {
					lowerExtra := strings.ToLower(valStr)
					if strings.Contains(lowerExtra, "using filesort") {
						hasFilesort = true
					}
					if strings.Contains(lowerExtra, "using temporary") {
						hasTempTable = true
					}
				}
				if colName == "key" && valStr != "NULL" && valStr != "" {
					usedKeys = append(usedKeys, valStr)
				}
				if colName == "rows" && valStr != "NULL" {
					var rCount int64
					fmt.Sscanf(valStr, "%d", &rCount)
					totalScannedRows += rCount
				}

				// Rich Color Cell Styling
				cellStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite)
				if colName == "type" {
					if strings.EqualFold(valStr, "ALL") {
						cellStyle = cellStyle.Foreground(tcell.ColorRed).Bold(true)
					} else if strings.EqualFold(valStr, "index") || strings.EqualFold(valStr, "range") {
						cellStyle = cellStyle.Foreground(tcell.ColorYellow).Bold(true)
					} else if strings.EqualFold(valStr, "ref") || strings.EqualFold(valStr, "eq_ref") || strings.EqualFold(valStr, "const") || strings.EqualFold(valStr, "system") {
						cellStyle = cellStyle.Foreground(tcell.ColorLime).Bold(true)
					}
				} else if colName == "key" {
					if valStr == "NULL" {
						cellStyle = cellStyle.Foreground(tcell.ColorRed)
					} else {
						cellStyle = cellStyle.Foreground(tcell.ColorAqua).Bold(true)
					}
				} else if colName == "rows" {
					var rc int64
					fmt.Sscanf(valStr, "%d", &rc)
					if rc > 10000 {
						cellStyle = cellStyle.Foreground(tcell.ColorRed).Bold(true)
					} else if rc > 1000 {
						cellStyle = cellStyle.Foreground(tcell.ColorYellow)
					} else {
						cellStyle = cellStyle.Foreground(tcell.ColorLime)
					}
				}

				table.SetCell(rIdx, c, tview.NewTableCell(" "+valStr+" ").SetStyle(cellStyle))
			}
			rIdx++
		}
	}

	// Performance Score Calculation
	healthScore := 100
	if hasFullTableScan {
		healthScore -= 50
		warnings = append(warnings, "[red::b]🔴 FULL TABLE SCAN (type=ALL)[-]")
	}
	if hasFilesort {
		healthScore -= 20
		warnings = append(warnings, "[yellow::b]⚠️ FILESORT[-] ")
	}
	if hasTempTable {
		healthScore -= 20
		warnings = append(warnings, "[yellow::b]⚠️ TEMPORARY TABLE[-] ")
	}
	if healthScore < 0 {
		healthScore = 0
	}

	scoreBadge := fmt.Sprintf("[lime::b]⚡ OPTIMAL (%d%%)[-]", healthScore)
	bgColor := tcell.ColorDarkGreen
	if healthScore <= 50 {
		scoreBadge = fmt.Sprintf("[red::b]🔴 CRITICAL BOTTLENECK (%d%%)[-]", healthScore)
		bgColor = tcell.ColorDarkRed
	} else if healthScore <= 80 {
		scoreBadge = fmt.Sprintf("[yellow::b]⚠️ SUBOPTIMAL (%d%%)[-]", healthScore)
		bgColor = tcell.ColorDarkCyan
	}

	summaryText := fmt.Sprintf(" Health: %s | Scanned Rows: [white::b]%s[-] | Keys Used: [cyan::b]%s[-]",
		scoreBadge, formatSize(totalScannedRows), strings.Join(usedKeys, ", "))
	if len(warnings) > 0 {
		summaryText += " | " + strings.Join(warnings, " ")
	}
	summaryBar.SetText(summaryText).SetBackgroundColor(bgColor)

	if rIdx == 1 {
		table.SetCell(1, 0, tview.NewTableCell(" No execution plan returned ").SetTextColor(tcell.ColorYellow))
	} else {
		table.Select(1, 0)
	}

	statusBar := tview.NewTextView().
		SetText("[yellow]ESC/F5[-] Close  |  [lime]t[-] EXPLAIN TREE  |  [cyan]a[-] ANALYZE Time  |  [magenta]j[-] JSON View  |  [aqua]i[-] AI Index Advisor  |  [white]c[-] Copy").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetBackgroundColor(tcell.ColorBlack)

	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(summaryBar, 1, 0, false).
		AddItem(table, 0, 1, true).
		AddItem(statusBar, 1, 0, false)

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyEnter || event.Key() == tcell.KeyF5 {
			orig := CreateLayoutWithFooter(app, mainFlex)
			app.SetRoot(orig, true)
			if tableList != nil {
				util.SetFocusWithBorder(app, tableList)
			}
			return nil
		}
		if event.Key() == tcell.KeyRune && (event.Rune() == 'c' || event.Rune() == 'C') {
			util.SetClipboardText(explainQuery)
			updateFooterText("EXPLAIN query copied to clipboard!")
			return nil
		}
		if event.Key() == tcell.KeyRune && (event.Rune() == 't' || event.Rune() == 'T') {
			// EXPLAIN FORMAT=TREE
			treeView := tview.NewTextView().
				SetDynamicColors(true).
				SetScrollable(true)
			treeView.SetBackgroundColor(tcell.ColorBlack)
			treeView.SetBorder(true).
				SetTitle(" 🌳 EXPLAIN FORMAT=TREE (Operator Execution Graph) ").
				SetTitleColor(tcell.ColorLime)

			treeOutput := getExplainQueryOutput(db, "EXPLAIN FORMAT=TREE "+cleanQ)
			treeView.SetText(treeOutput)

			treeLayout := tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(treeView, 0, 1, true).
				AddItem(tview.NewTextView().SetText("[yellow]ESC / ENTER[-] Return  |  [lime]C[-] Copy Tree Output").SetTextAlign(tview.AlignCenter).SetDynamicColors(true), 1, 0, false)

			treeView.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
				if e.Key() == tcell.KeyEscape || e.Key() == tcell.KeyEnter {
					fullL := CreateLayoutWithFooter(app, layout)
					app.SetRoot(fullL, true)
					app.SetFocus(table)
					return nil
				}
				if e.Key() == tcell.KeyRune && (e.Rune() == 'c' || e.Rune() == 'C') {
					util.SetClipboardText(treeOutput)
				}
				return e
			})

			app.SetRoot(CreateLayoutWithFooter(app, treeLayout), true).SetFocus(treeView)
			return nil
		}
		if event.Key() == tcell.KeyRune && (event.Rune() == 'a' || event.Rune() == 'A') {
			// EXPLAIN ANALYZE
			analyzeView := tview.NewTextView().
				SetDynamicColors(true).
				SetScrollable(true)
			analyzeView.SetBackgroundColor(tcell.ColorBlack)
			analyzeView.SetBorder(true).
				SetTitle(" ⚡ EXPLAIN ANALYZE (Real Execution Time & Loop Costs) ").
				SetTitleColor(tcell.ColorYellow)

			analyzeOutput := getExplainQueryOutput(db, "EXPLAIN ANALYZE "+cleanQ)
			analyzeView.SetText(analyzeOutput)

			analyzeLayout := tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(analyzeView, 0, 1, true).
				AddItem(tview.NewTextView().SetText("[yellow]ESC / ENTER[-] Return  |  [lime]C[-] Copy Output").SetTextAlign(tview.AlignCenter).SetDynamicColors(true), 1, 0, false)

			analyzeView.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
				if e.Key() == tcell.KeyEscape || e.Key() == tcell.KeyEnter {
					fullL := CreateLayoutWithFooter(app, layout)
					app.SetRoot(fullL, true)
					app.SetFocus(table)
					return nil
				}
				if e.Key() == tcell.KeyRune && (e.Rune() == 'c' || e.Rune() == 'C') {
					util.SetClipboardText(analyzeOutput)
				}
				return e
			})

			app.SetRoot(CreateLayoutWithFooter(app, analyzeLayout), true).SetFocus(analyzeView)
			return nil
		}
		if event.Key() == tcell.KeyRune && (event.Rune() == 'j' || event.Rune() == 'J') {
			// EXPLAIN FORMAT=JSON
			jsonView := tview.NewTextView().
				SetDynamicColors(true).
				SetScrollable(true)
			jsonView.SetBackgroundColor(tcell.ColorBlack)
			jsonView.SetBorder(true).
				SetTitle(" 🔍 EXPLAIN FORMAT=JSON Output ").
				SetTitleColor(tcell.ColorDarkMagenta)

			jsonOutput := getExplainQueryOutput(db, "EXPLAIN FORMAT=JSON "+cleanQ)
			jsonView.SetText(jsonOutput)

			jsonLayout := tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(jsonView, 0, 1, true).
				AddItem(tview.NewTextView().SetText("[yellow]ESC / ENTER[-] Return  |  [lime]C[-] Copy JSON").SetTextAlign(tview.AlignCenter).SetDynamicColors(true), 1, 0, false)

			jsonView.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
				if e.Key() == tcell.KeyEscape || e.Key() == tcell.KeyEnter {
					fullL := CreateLayoutWithFooter(app, layout)
					app.SetRoot(fullL, true)
					app.SetFocus(table)
					return nil
				}
				if e.Key() == tcell.KeyRune && (e.Rune() == 'c' || e.Rune() == 'C') {
					util.SetClipboardText(jsonOutput)
				}
				return e
			})

			app.SetRoot(CreateLayoutWithFooter(app, jsonLayout), true).SetFocus(jsonView)
			return nil
		}
		if event.Key() == tcell.KeyRune && (event.Rune() == 'i' || event.Rune() == 'I') {
			// AI Index & Performance Advisor
			advisorText := buildIndexRecommendationText(scannedTables, hasFullTableScan, hasFilesort, hasTempTable, totalScannedRows, usedKeys)

			advisorModal := tview.NewModal().
				SetText(advisorText).
				AddButtons([]string{"[lime::b] Copy Recommendation ", "[white::b] Back to EXPLAIN "}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					if strings.Contains(buttonLabel, "Copy") {
						util.SetClipboardText(advisorText)
					}
					fullL := CreateLayoutWithFooter(app, layout)
					app.SetRoot(fullL, true)
					app.SetFocus(table)
				})

			app.SetRoot(advisorModal, true)
			return nil
		}
		return event
	})

	fullLayout := CreateLayoutWithFooter(app, layout)
	app.SetRoot(fullLayout, true)
	app.SetFocus(table)
}

func getExplainQueryOutput(db *sql.DB, query string) string {
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Sprintf("[red]Failed to execute query:\n%v[-]", err)
	}
	defer rows.Close()

	var sb strings.Builder
	cols, err := rows.Columns()
	if err != nil || len(cols) == 0 {
		return "[yellow]No output returned.[-]"
	}

	rawVals := make([]sql.RawBytes, len(cols))
	scanVals := make([]interface{}, len(cols))
	for i := range rawVals {
		scanVals[i] = &rawVals[i]
	}

	for rows.Next() {
		if err := rows.Scan(scanVals...); err == nil {
			for _, rByte := range rawVals {
				if rByte != nil {
					sb.WriteString(string(rByte))
				} else {
					sb.WriteString("NULL")
				}
				sb.WriteString("\n")
			}
		}
	}

	if sb.Len() == 0 {
		return "[yellow]No output generated.[-]"
	}
	return sb.String()
}

func buildIndexRecommendationText(tables []string, fullScan, filesort, tempTable bool, totalRows int64, usedKeys []string) string {
	var sb strings.Builder
	sb.WriteString("[aqua::b]💡 PHERI AI PERFORMANCE & INDEX ADVISOR 💡[-::-]\n\n")

	if !fullScan && !filesort && !tempTable {
		sb.WriteString("[lime::b]✅ GREAT NEWS: Query execution plan is already optimal![-::-]\n")
		sb.WriteString("• Using Index Key(s): " + strings.Join(usedKeys, ", ") + "\n")
		sb.WriteString("• Scanned row count is minimal (" + fmt.Sprintf("%d", totalRows) + " rows).\n")
		return sb.String()
	}

	sb.WriteString("[yellow::b]Found performance bottlenecks during EXPLAIN analysis:[-::-]\n\n")

	tblName := "target_table"
	if len(tables) > 0 {
		tblName = tables[0]
	}

	if fullScan {
		sb.WriteString("1. [red::b]🔴 CRITICAL: Full Table Scan Detected (type=ALL)[-::-]\n")
		sb.WriteString("   Scanned ~" + formatSize(totalRows) + " rows without using an index.\n")
		sb.WriteString("   [lime::b]Recommended Index Fix:[-::-]\n")
		sb.WriteString(fmt.Sprintf("   [cyan::b]CREATE INDEX idx_%s_filter ON %s (id);[-::-]\n\n", tblName, tblName))
	}

	if filesort {
		sb.WriteString("2. [yellow::b]⚠️ WARNING: Using Filesort[-::-]\n")
		sb.WriteString("   MySQL is sorting rows in memory/disk instead of using an ordered index.\n")
		sb.WriteString("   [lime::b]Recommended Index Fix:[-::-]\n")
		sb.WriteString(fmt.Sprintf("   [cyan::b]CREATE INDEX idx_%s_sort ON %s (created_at DESC);[-::-]\n\n", tblName, tblName))
	}

	if tempTable {
		sb.WriteString("3. [yellow::b]⚠️ WARNING: Using Temporary Table[-::-]\n")
		sb.WriteString("   Intermediate result sets are being written to temporary disk tables.\n")
	}

	return sb.String()
}
