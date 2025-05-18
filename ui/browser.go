// ui/browser.go
package ui

import (
	"bufio"
	"compress/gzip"
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
	"sync"
	"time"

	"github.com/atotto/clipboard"
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

func exportAllObjects(outputFile string, progressChan chan string, dbName string) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", User, Pass, Host, Port, dbName)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		progressChan <- fmt.Sprintf("[red]Failed to connect to DB: %v", err)
		close(progressChan)
		return
	}
	defer db.Close()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Minute * 5)

	// Set up multiple writers
	type fileWriters struct {
		table, view, viewddl, procedure, function *bufio.Writer
		tableFile, viewFile, viewddlFile          *os.File
		procedureFile, functionFile               *os.File
		tableGzip, viewGzip, viewddlGzip          *gzip.Writer
		procedureGzip, functionGzip               *gzip.Writer
	}

	fw := &fileWriters{}

	openGz := func(suffix string) (*os.File, *gzip.Writer, *bufio.Writer, error) {
		f, err := os.Create(fmt.Sprintf("%s_%s.gz", outputFile, suffix))
		if err != nil {
			return nil, nil, nil, err
		}
		gz := gzip.NewWriter(f)
		buf := bufio.NewWriter(gz)
		return f, gz, buf, nil
	}

	if fw.tableFile, fw.tableGzip, fw.table, err = openGz("table"); err != nil {
		progressChan <- fmt.Sprintf("[red]Failed to open table file: %v", err)
		close(progressChan)
		return
	}
	if fw.viewFile, fw.viewGzip, fw.view, err = openGz("view"); err != nil {
		progressChan <- fmt.Sprintf("[red]Failed to open view file: %v", err)
		close(progressChan)
		return
	}
	if fw.viewddlFile, fw.viewddlGzip, fw.viewddl, err = openGz("viewddl"); err != nil {
		progressChan <- fmt.Sprintf("[red]Failed to open viewddl file: %v", err)
		close(progressChan)
		return
	}

	if fw.procedureFile, fw.procedureGzip, fw.procedure, err = openGz("procedure"); err != nil {
		progressChan <- fmt.Sprintf("[red]Failed to open procedure file: %v", err)
		close(progressChan)
		return
	}
	if fw.functionFile, fw.functionGzip, fw.function, err = openGz("function"); err != nil {
		progressChan <- fmt.Sprintf("[red]Failed to open function file: %v", err)
		close(progressChan)
		return
	}

	defer func() {
		fw.table.Flush()
		fw.tableGzip.Close()
		fw.tableFile.Close()

		fw.view.Flush()
		fw.viewGzip.Close()
		fw.viewFile.Close()

		fw.viewddl.Flush()
		fw.viewddlGzip.Close()
		fw.viewddlFile.Close()

		fw.procedure.Flush()
		fw.procedureGzip.Close()
		fw.procedureFile.Close()

		fw.function.Flush()
		fw.functionGzip.Close()
		fw.functionFile.Close()
	}()

	var mu sync.Mutex
	var wg sync.WaitGroup

	workerCount := 10
	tasks := make(chan DBObject, len(allTables))

	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for obj := range tasks {
				var ddl string
				var writer *bufio.Writer
				var writerddl *bufio.Writer

				for i := 0; i < 3; i++ {
					if err := db.Ping(); err == nil {
						break
					}
					time.Sleep(2 * time.Second)
				}

				switch obj.Type {
				case "TABLE":
					writer = fw.table
					const insertBatchSize = 1000
					var table, createStmt string
					row := db.QueryRow(fmt.Sprintf("SHOW CREATE TABLE `%s`", obj.Name))
					if err := row.Scan(&table, &createStmt); err != nil {
						progressChan <- fmt.Sprintf("[yellow]Failed to export TABLE: %s - %v", obj.Name, err)
						continue
					}
					ddl = createStmt

					rows, err := db.Query(fmt.Sprintf("SELECT * FROM `%s`", obj.Name))
					if err != nil {
						progressChan <- fmt.Sprintf("[yellow]Failed to select data from TABLE: %s - %v", obj.Name, err)
						continue
					}
					defer rows.Close()

					cols, _ := rows.Columns()
					colCount := len(cols)
					values := make([]interface{}, colCount)
					valuePtrs := make([]interface{}, colCount)
					colList := "`" + strings.Join(cols, "`, `") + "`"

					var valueRows []string
					rowCount := 0

					mu.Lock()
					writer.WriteString("-- ----------------------------\n")
					writer.WriteString(fmt.Sprintf("-- TABLE: %s\n", obj.Name))
					writer.WriteString("-- ----------------------------\n")
					writer.WriteString(ddl + ";\n\n")
					writer.WriteString("-- DATA\n")
					mu.Unlock()

					for rows.Next() {
						for i := range values {
							valuePtrs[i] = &values[i]
						}
						err := rows.Scan(valuePtrs...)
						if err != nil {
							continue
						}

						var valStrings []string
						for _, val := range values {
							switch v := val.(type) {
							case nil:
								valStrings = append(valStrings, "NULL")
							case []byte:
								valStrings = append(valStrings, fmt.Sprintf("'%s'", escapeString(string(v))))
							case string:
								valStrings = append(valStrings, fmt.Sprintf("'%s'", escapeString(v)))
							default:
								valStrings = append(valStrings, fmt.Sprintf("'%v'", v))
							}
						}

						valueRows = append(valueRows, fmt.Sprintf("(%s)", strings.Join(valStrings, ", ")))
						rowCount++

						if rowCount >= insertBatchSize {
							mu.Lock()
							writer.WriteString(fmt.Sprintf("INSERT INTO `%s` (%s) VALUES\n", obj.Name, colList))
							writer.WriteString(strings.Join(valueRows, ",\n") + ";\n\n")
							mu.Unlock()

							valueRows = valueRows[:0]
							rowCount = 0
						}
					}

					if len(valueRows) > 0 {
						mu.Lock()
						writer.WriteString(fmt.Sprintf("INSERT INTO `%s` (%s) VALUES\n", obj.Name, colList))
						writer.WriteString(strings.Join(valueRows, ",\n") + ";\n\n")
						mu.Unlock()
					}

				case "VIEW":
					writerddl = fw.viewddl
					columnQuery := fmt.Sprintf("SELECT * FROM `%s` LIMIT 0", obj.Name)
					rowsddl, err := db.Query(columnQuery)
					if err != nil {
						progressChan <- fmt.Sprintf("[yellow]Failed to select data from VIEW: %s - %v", obj.Name, err)
						continue
					}
					cols, err := rowsddl.ColumnTypes()
					rowsddl.Close()
					if err != nil {
						progressChan <- fmt.Sprintf("[yellow]Failed to get column types from VIEW: %s - %v", obj.Name, err)
						continue
					}
					var structBuilder strings.Builder
					structBuilder.WriteString("-- ----------------------------\n")
					structBuilder.WriteString(fmt.Sprintf("--  STRUCTURE (DUMMY TABLE FOR VIEW): %s\n", obj.Name))
					structBuilder.WriteString("-- ----------------------------\n")
					structBuilder.WriteString(fmt.Sprintf("CREATE TABLE `%s` (\n", obj.Name))
					for i, col := range cols {
						colName := col.Name()
						colType := col.DatabaseTypeName()
						nullable, _ := col.Nullable()
						nullStr := "NOT NULL"
						if nullable {
							nullStr = "NULL"
						}

						colDef := fmt.Sprintf("  `%s` %s %s", colName, mapSQLType(colType), nullStr)

						if i < len(cols)-1 {
							colDef += ",\n"

						} else {
							colDef += "\n"
						}
						structBuilder.WriteString(colDef)
					}
					structBuilder.WriteString(");\n\n")
					writer = fw.view
					var view, createStmt, charset, collation string
					row := db.QueryRow(fmt.Sprintf("SHOW CREATE VIEW `%s`", obj.Name))
					if err := row.Scan(&view, &createStmt, &charset, &collation); err != nil {
						progressChan <- fmt.Sprintf("[yellow]Failed to export VIEW: %s - %v", obj.Name, err)
						continue
					}
					ddl = createStmt

					mu.Lock()
					writerddl.WriteString(structBuilder.String())
					writerddl.Flush()
					writer.WriteString("-- ----------------------------\n")
					writer.WriteString(fmt.Sprintf("-- VIEW: %s\n", obj.Name))
					writer.WriteString("-- ----------------------------\n")
					writer.WriteString("DROP TABLE IF EXISTS `" + obj.Name + "`;\n")
					writer.WriteString(ddl + ";\n\n")
					writer.Flush()
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
						progressChan <- fmt.Sprintf("[yellow]Failed to export %s: %s - %v", obj.Type, obj.Name, err)
						continue
					}
					ddl = createStmt

					mu.Lock()
					writer.WriteString("-- ----------------------------\n")
					writer.WriteString(fmt.Sprintf("-- %s: %s\n", obj.Type, obj.Name))
					writer.WriteString("-- ----------------------------\n")
					writer.WriteString("DELIMITER //\n")
					writer.WriteString(ddl + ";\n\n")
					writer.WriteString("// \nDELIMITER ;\n")
					mu.Unlock()
				}

				progressChan <- fmt.Sprintf("[green]Exported %s: %s", obj.Type, obj.Name)
				time.Sleep(50 * time.Millisecond)
			}
		}()
	}

	for _, obj := range allTables {
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

// func exportAllObjects(outputFile string, progressChan chan string, dbName string) {
// 	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", User, Pass, Host, Port, dbName)

// 	db, err := sql.Open("mysql", dsn)
// 	if err != nil {
// 		progressChan <- fmt.Sprintf("[red]Failed to connect to DB: %v", err)
// 		close(progressChan)
// 		return
// 	}
// 	defer db.Close()

// 	db.SetMaxOpenConns(10)
// 	db.SetMaxIdleConns(5)
// 	db.SetConnMaxLifetime(time.Minute * 5)

// 	f, err := os.Create(outputFile + ".gz")
// 	if err != nil {
// 		progressChan <- fmt.Sprintf("[red]Failed to create output file: %v", err)
// 		close(progressChan)
// 		return
// 	}
// 	gzipWriter := gzip.NewWriter(f)
// 	writer := bufio.NewWriter(gzipWriter)

// 	defer func() {
// 		writer.Flush()
// 		gzipWriter.Close()
// 		f.Close()
// 	}()

// 	var mu sync.Mutex
// 	var wg sync.WaitGroup

// 	workerCount := 10
// 	tasks := make(chan DBObject, len(allTables))

// 	for w := 0; w < workerCount; w++ {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			for obj := range tasks {
// 				var ddl string

// 				// Retry ping
// 				var err error
// 				for i := 0; i < 3; i++ {
// 					if err = db.Ping(); err == nil {
// 						break
// 					}
// 					time.Sleep(time.Second * 2)
// 				}
// 				if err != nil {
// 					progressChan <- fmt.Sprintf("[red]DB connection not alive for %s: %v", obj.Name, err)
// 					continue
// 				}

// 				switch obj.Type {
// 				case "TABLE":
// 					const insertBatchSize = 1000
// 					var table, createStmt string
// 					row := db.QueryRow(fmt.Sprintf("SHOW CREATE TABLE `%s`", obj.Name))
// 					if err := row.Scan(&table, &createStmt); err != nil {
// 						progressChan <- fmt.Sprintf("[yellow]Failed to export TABLE: %s - %v", obj.Name, err)
// 						continue
// 					}
// 					ddl = createStmt

// 					rows, err := db.Query(fmt.Sprintf("SELECT * FROM `%s`", obj.Name))
// 					if err != nil {
// 						progressChan <- fmt.Sprintf("[yellow]Failed to select data from TABLE: %s - %v", obj.Name, err)
// 						break
// 					}
// 					defer rows.Close()

// 					cols, _ := rows.Columns()
// 					colCount := len(cols)
// 					values := make([]interface{}, colCount)
// 					valuePtrs := make([]interface{}, colCount)

// 					colList := "`" + strings.Join(cols, "`, `") + "`"
// 					// util.SaveLog("colList: " + colList)
// 					var valueRows []string

// 					mu.Lock()
// 					writer.WriteString("-- ----------------------------\n")
// 					writer.WriteString(fmt.Sprintf("-- TABLE: %s\n", obj.Name))
// 					writer.WriteString("-- ----------------------------\n")
// 					writer.WriteString(ddl + ";\n\n")
// 					writer.WriteString("-- DATA\n")
// 					mu.Unlock()

// 					rowCount := 0
// 					batchCount := 0

// 					for rows.Next() {
// 						for i := range values {
// 							valuePtrs[i] = &values[i]
// 						}
// 						err := rows.Scan(valuePtrs...)
// 						if err != nil {
// 							continue
// 						}

// 						var valStrings []string
// 						for _, val := range values {
// 							switch v := val.(type) {
// 							case nil:
// 								valStrings = append(valStrings, "NULL")
// 							case []byte:
// 								valStrings = append(valStrings, fmt.Sprintf("'%s'", escapeString(string(v))))
// 							case string:
// 								valStrings = append(valStrings, fmt.Sprintf("'%s'", escapeString(v)))
// 							default:
// 								valStrings = append(valStrings, fmt.Sprintf("'%v'", v))
// 							}
// 						}

// 						valueRows = append(valueRows, fmt.Sprintf("(%s)", strings.Join(valStrings, ", ")))
// 						rowCount++

// 						// Write batch if limit is reached
// 						if rowCount >= insertBatchSize {
// 							mu.Lock()
// 							writer.WriteString(fmt.Sprintf("INSERT INTO `%s` (%s) VALUES\n", obj.Name, colList))
// 							writer.WriteString(strings.Join(valueRows, ",\n") + ";\n\n")
// 							mu.Unlock()

// 							valueRows = valueRows[:0] // Reset slice
// 							rowCount = 0
// 							batchCount++
// 						}
// 					}

// 					// Write remaining rows
// 					if len(valueRows) > 0 {
// 						mu.Lock()
// 						writer.WriteString(fmt.Sprintf("INSERT INTO `%s` (%s) VALUES\n", obj.Name, colList))
// 						writer.WriteString(strings.Join(valueRows, ",\n") + ";\n\n")
// 						mu.Unlock()
// 					}

// 				// case "TABLE":
// 				// 	var table, createStmt string
// 				// 	row := db.QueryRow(fmt.Sprintf("SHOW CREATE TABLE `%s`", obj.Name))
// 				// 	if err := row.Scan(&table, &createStmt); err != nil {
// 				// 		progressChan <- fmt.Sprintf("[yellow]Failed to export TABLE: %s - %v", obj.Name, err)
// 				// 		continue
// 				// 	}
// 				// 	ddl = createStmt

// 				// 	var inserts []string
// 				// 	rows, err := db.Query(fmt.Sprintf("SELECT * FROM `%s`", obj.Name))
// 				// 	if err != nil {
// 				// 		progressChan <- fmt.Sprintf("[yellow]Failed to select data from TABLE: %s - %v", obj.Name, err)
// 				// 		break
// 				// 	}
// 				// 	defer rows.Close()

// 				// 	cols, _ := rows.Columns()
// 				// 	colCount := len(cols)
// 				// 	values := make([]interface{}, colCount)
// 				// 	valuePtrs := make([]interface{}, colCount)

// 				// 	for rows.Next() {
// 				// 		for i := range values {
// 				// 			valuePtrs[i] = &values[i]
// 				// 		}
// 				// 		err := rows.Scan(valuePtrs...)
// 				// 		if err != nil {
// 				// 			continue
// 				// 		}

// 				// 		var valStrings []string
// 				// 		for _, val := range values {
// 				// 			switch v := val.(type) {
// 				// 			case nil:
// 				// 				valStrings = append(valStrings, "NULL")
// 				// 			case []byte:
// 				// 				valStrings = append(valStrings, fmt.Sprintf("'%s'", escapeString(string(v))))
// 				// 			case string:
// 				// 				valStrings = append(valStrings, fmt.Sprintf("'%s'", escapeString(v)))
// 				// 			default:
// 				// 				valStrings = append(valStrings, fmt.Sprintf("'%v'", v))
// 				// 			}
// 				// 		}
// 				// 		insert := fmt.Sprintf("INSERT INTO `%s` VALUES (%s);", obj.Name, strings.Join(valStrings, ", "))
// 				// 		inserts = append(inserts, insert)
// 				// 	}

// 				// 	mu.Lock()
// 				// 	writer.WriteString("-- ----------------------------\n")
// 				// 	writer.WriteString(fmt.Sprintf("-- TABLE: %s\n", obj.Name))
// 				// 	writer.WriteString("-- ----------------------------\n")
// 				// 	writer.WriteString(ddl + ";\n\n")

// 				// 	if len(inserts) > 0 {
// 				// 		writer.WriteString("-- DATA\n")
// 				// 		for _, ins := range inserts {
// 				// 			writer.WriteString(ins + "\n")
// 				// 		}
// 				// 		writer.WriteString("\n")
// 				// 	}
// 				// 	mu.Unlock()

// 				case "VIEW":
// 					var view, createStmt, charset, collation string
// 					row := db.QueryRow(fmt.Sprintf("SHOW CREATE VIEW `%s`", obj.Name))
// 					if err := row.Scan(&view, &createStmt, &charset, &collation); err != nil {
// 						progressChan <- fmt.Sprintf("[yellow]Failed to export VIEW: %s - %v", obj.Name, err)
// 						continue
// 					}
// 					ddl = createStmt

// 					mu.Lock()
// 					writer.WriteString("-- ----------------------------\n")
// 					writer.WriteString(fmt.Sprintf("-- VIEW: %s\n", obj.Name))
// 					writer.WriteString("-- ----------------------------\n")
// 					writer.WriteString(ddl + ";\n\n")
// 					mu.Unlock()

// 				case "PROCEDURE", "FUNCTION":
// 					var name, sqlMode, createStmt, charset, collation, dbCollation string
// 					row := db.QueryRow(fmt.Sprintf("SHOW CREATE %s `%s`", obj.Type, obj.Name))
// 					if err := row.Scan(&name, &sqlMode, &createStmt, &charset, &collation, &dbCollation); err != nil {
// 						progressChan <- fmt.Sprintf("[yellow]Failed to export %s: %s - %v", obj.Type, obj.Name, err)
// 						continue
// 					}
// 					ddl = createStmt

// 					mu.Lock()
// 					writer.WriteString("-- ----------------------------\n")
// 					writer.WriteString(fmt.Sprintf("-- %s: %s\n", obj.Type, obj.Name))
// 					writer.WriteString("-- ----------------------------\n")
// 					writer.WriteString(ddl + ";\n\n")
// 					mu.Unlock()

// 				}

// 				progressChan <- fmt.Sprintf("[green]Exported %s: %s", obj.Type, obj.Name)
// 			}
// 		}()
// 	}

// 	for _, obj := range allTables {
// 		tasks <- obj
// 	}
// 	close(tasks)

// 	wg.Wait()
// 	close(progressChan)
// }

// func escapeString(input string) string {
// 	// Escape single quotes and backslashes
// 	input = strings.ReplaceAll(input, "\\", "\\\\")
// 	input = strings.ReplaceAll(input, "'", "\\'")
// 	return input
// }

// func escapeString(str string) string {
// 	return strings.ReplaceAll(str, "'", "''")
// }

// func escapeString(str string) string {
// 	str = strings.ReplaceAll(str, "'", "''") // Escape single quotes
// 	str = strings.ReplaceAll(str, "\n", "")  // Remove newlines
// 	str = strings.ReplaceAll(str, "\r", "")  // Remove carriage returns (Windows-style newlines)
// 	return str
// }

func escapeString(str string) string {
	str = strings.ReplaceAll(str, "'", "''") // Escape single quotes
	str = strings.ReplaceAll(str, "\n", "")  // Remove newlines (Unix-style)
	str = strings.ReplaceAll(str, "\r", "")  // Remove carriage returns (Windows-style)
	str = strings.ReplaceAll(str, "\t", "")  // Optional: remove tabs if present
	return str
}

// func exportAllObjects(outputFile string, progressChan chan string, dbName string) {
// 	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", "root", "12345678", "127.0.0.1", "3306", dbName)

// 	// Create single shared DB connection pool
// 	db, err := sql.Open("mysql", dsn)
// 	if err != nil {
// 		progressChan <- fmt.Sprintf("[red]Failed to connect to DB: %v", err)
// 		close(progressChan)
// 		return
// 	}
// 	defer db.Close()

// 	// Set limits on DB connection pool
// 	db.SetMaxOpenConns(10)
// 	db.SetMaxIdleConns(5)
// 	db.SetConnMaxLifetime(time.Minute * 5)

// 	f, err := os.Create(outputFile)
// 	if err != nil {
// 		progressChan <- fmt.Sprintf("[red]Failed to create output file: %v", err)
// 		close(progressChan)
// 		return
// 	}
// 	defer func() {
// 		f.Sync()
// 		f.Close()
// 	}()

// 	var mu sync.Mutex
// 	var wg sync.WaitGroup

// 	workerCount := 10
// 	tasks := make(chan DBObject, len(allTables))

// 	// Worker pool
// 	for w := 0; w < workerCount; w++ {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			for obj := range tasks {
// 				var ddl string

// 				// Retry logic for Ping
// 				var err error
// 				for i := 0; i < 3; i++ {
// 					if err = db.Ping(); err == nil {
// 						break
// 					}
// 					time.Sleep(time.Second * 2)
// 				}
// 				if err != nil {
// 					progressChan <- fmt.Sprintf("[red]DB connection not alive for %s: %v", obj.Name, err)
// 					continue
// 				}

// 				switch obj.Type {
// 				case "TABLE":
// 					var table, createStmt string
// 					row := db.QueryRow(fmt.Sprintf("SHOW CREATE TABLE `%s`", obj.Name))
// 					if err := row.Scan(&table, &createStmt); err != nil {
// 						progressChan <- fmt.Sprintf("[yellow]Failed to export TABLE: %s - %v", obj.Name, err)
// 						continue
// 					}
// 					ddl = createStmt
// 				case "VIEW":
// 					var view, createStmt, charset, collation string
// 					row := db.QueryRow(fmt.Sprintf("SHOW CREATE VIEW `%s`", obj.Name))
// 					if err := row.Scan(&view, &createStmt, &charset, &collation); err != nil {
// 						progressChan <- fmt.Sprintf("[yellow]Failed to export VIEW: %s - %v", obj.Name, err)
// 						continue
// 					}
// 					ddl = createStmt
// 				case "PROCEDURE", "FUNCTION":
// 					var name, createStmt, charset string
// 					row := db.QueryRow(fmt.Sprintf("SHOW CREATE %s `%s`", obj.Type, obj.Name))
// 					if err := row.Scan(&name, &createStmt, &charset); err != nil {
// 						progressChan <- fmt.Sprintf("[yellow]Failed to export %s: %s - %v", obj.Type, obj.Name, err)
// 						continue
// 					}
// 					ddl = createStmt
// 				default:
// 					progressChan <- fmt.Sprintf("[red]Unknown object type: %s", obj.Type)
// 					continue
// 				}

// 				if ddl == "" {
// 					progressChan <- fmt.Sprintf("[red]Empty DDL for %s: %s", obj.Type, obj.Name)
// 					continue
// 				}

// 				// Write to file with lock
// 				mu.Lock()
// 				f.WriteString("-- ----------------------------\n")
// 				f.WriteString(fmt.Sprintf("-- %s: %s\n", obj.Type, obj.Name))
// 				f.WriteString("-- ----------------------------\n")
// 				f.WriteString(ddl + ";\n\n")
// 				mu.Unlock()

// 				progressChan <- fmt.Sprintf("[green]Exported %s: %s", obj.Type, obj.Name)
// 			}
// 		}()
// 	}

// 	// Enqueue tasks
// 	for _, obj := range allTables {
// 		tasks <- obj
// 	}
// 	close(tasks)

// 	// Wait for all workers
// 	wg.Wait()
// 	close(progressChan)
// }

// func exportAllObjects(outputFile string, progressChan chan string, dbName string) {

// 	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", "root", "12345678", "127.0.0.1", "3306", dbName)
// 	util.SaveLog(dsn)
// 	var dbPool []*sql.DB
// 	for i := 0; i < len(allTables)+50; i++ {
// 		conn1, err := sql.Open("mysql", dsn)
// 		if err != nil {
// 			log.Println("DB connection error:", err)
// 			continue
// 		}
// 		dbPool = append(dbPool, conn1)
// 	}
// 	defer func() {
// 		for _, db1 := range dbPool {
// 			db1.Close()
// 		}
// 	}()

// 	var wg sync.WaitGroup
// 	var mu sync.Mutex

// 	f, err := os.Create(outputFile)
// 	if err != nil {
// 		progressChan <- fmt.Sprintf("[red]Failed to create file: %v", err)
// 		close(progressChan)
// 		return
// 	}
// 	defer func() {
// 		f.Sync()
// 		f.Close()
// 	}()

// 	for i, obj := range allTables {
// 		wg.Add(1)

// 		go func(i int, obj DBObject) {
// 			defer wg.Done()
// 			db := dbPool[i%len(dbPool)]

// 			// Validate DB is alive
// 			if err := db.Ping(); err != nil {
// 				progressChan <- fmt.Sprintf("[red]DB connection is not alive for %s: %v", obj.Name, err)
// 				return
// 			}

// 			var ddl string
// 			switch obj.Type {
// 			case "TABLE", "VIEW":
// 				var table, createStmt string
// 				row := db.QueryRow(fmt.Sprintf("SHOW CREATE TABLE `%s`", obj.Name))
// 				if err := row.Scan(&table, &createStmt); err != nil {
// 					progressChan <- fmt.Sprintf("[yellow]Failed to export %s: %s - %v", obj.Type, obj.Name, err)
// 					return
// 				}
// 				ddl = createStmt

// 			case "PROCEDURE", "FUNCTION":
// 				var name, createStmt, charset string
// 				row := db.QueryRow(fmt.Sprintf("SHOW CREATE %s `%s`", obj.Type, obj.Name))
// 				if err := row.Scan(&name, &createStmt, &charset); err != nil {
// 					progressChan <- fmt.Sprintf("[yellow]Failed to export %s: %s - %v", obj.Type, obj.Name, err)
// 					return
// 				}
// 				ddl = createStmt
// 			}

// 			if ddl == "" {
// 				progressChan <- fmt.Sprintf("[red]WARNING: Empty DDL for %s: %s", obj.Type, obj.Name)
// 				return
// 			}

// 			mu.Lock()
// 			f.WriteString("-- ----------------------------\n")
// 			f.WriteString(fmt.Sprintf("-- %s: %s\n", obj.Type, obj.Name))
// 			f.WriteString("-- ----------------------------\n")
// 			f.WriteString(ddl + ";\n\n")
// 			mu.Unlock()

// 			progressChan <- fmt.Sprintf("[green]Exported %s: %s", obj.Type, obj.Name)
// 		}(i, obj)
// 	}

// 	wg.Wait()
// 	close(progressChan)
// }

// func exportAllObjects(outputFile string, progressChan chan string, dbName string) {
// 	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", "root", "12345678", "127.0.0.1", "3306", dbName)
// 	util.SaveLog(dsn)

// 	var dbPool []*sql.DB
// 	for i := 0; i < len(allTables)+50; i++ {
// 		conn1, err := sql.Open("mysql", dsn)

// 		if err != nil {
// 			log.Println("DB connection error:", err)
// 			continue
// 		}
// 		dbPool = append(dbPool, conn1)
// 	}
// 	countStr := fmt.Sprintf("%d", len(dbPool))
// 	util.SaveLog(countStr)
// 	defer func() {
// 		for _, db1 := range dbPool {
// 			db1.Close()
// 		}
// 	}()

// 	var wg sync.WaitGroup
// 	var mu sync.Mutex

// 	f, err := os.Create(outputFile)
// 	if err != nil {
// 		progressChan <- fmt.Sprintf("[red]Failed to create file: %v", err)
// 		close(progressChan)
// 		return
// 	}
// 	defer func() {
// 		f.Sync()
// 		f.Close()
// 	}()

// 	for i, obj := range allTables {
// 		wg.Add(1)

// 		go func(obj DBObject) {
// 			defer wg.Done()
// 			db := dbPool[i%len(dbPool)]

// 			defer db.Close()
// 			if err := db.Ping(); err != nil {
// 				for attempts := 0; attempts < 3; attempts++ {
// 					db, err = sql.Open("mysql", dsn)
// 					if err == nil {
// 						break // success
// 					}
// 					if attempts == 2 {
// 						progressChan <- fmt.Sprintf("[red]DB RE-connection not alive for %s: %v", obj.Name, err)
// 						return
// 					}
// 					time.Sleep(time.Second * 2)
// 				}
// 			}

// 			// Validate DB is alive
// 			if err := db.Ping(); err != nil {
// 				progressChan <- fmt.Sprintf("[red]DB connection not alive for %s: %v", obj.Name, err)
// 				return
// 			}

// 			var ddl string
// 			switch obj.Type {
// 			case "TABLE":
// 				var table, createStmt string
// 				row := db.QueryRow(fmt.Sprintf("SHOW CREATE TABLE `%s`", obj.Name))
// 				if err := row.Scan(&table, &createStmt); err != nil {
// 					progressChan <- fmt.Sprintf("[yellow]Failed to export TABLE: %s - %v", obj.Name, err)
// 					return
// 				}
// 				ddl = createStmt

// 			case "VIEW":
// 				var view, createStmt, charset, collation string
// 				row := db.QueryRow(fmt.Sprintf("SHOW CREATE VIEW `%s`", obj.Name))
// 				if err := row.Scan(&view, &createStmt, &charset, &collation); err != nil {
// 					progressChan <- fmt.Sprintf("[yellow]Failed to export VIEW: %s - %v", obj.Name, err)
// 					return
// 				}
// 				ddl = createStmt

// 			case "PROCEDURE", "FUNCTION":
// 				var name, createStmt, charset string
// 				row := db.QueryRow(fmt.Sprintf("SHOW CREATE %s `%s`", obj.Type, obj.Name))
// 				if err := row.Scan(&name, &createStmt, &charset); err != nil {
// 					progressChan <- fmt.Sprintf("[yellow]Failed to export %s: %s - %v", obj.Type, obj.Name, err)
// 					return
// 				}
// 				ddl = createStmt
// 			}

// 			if ddl == "" {
// 				progressChan <- fmt.Sprintf("[red]WARNING: Empty DDL for %s: %s", obj.Type, obj.Name)
// 				return
// 			}

// 			mu.Lock()
// 			f.WriteString("-- ----------------------------\n")
// 			f.WriteString(fmt.Sprintf("-- %s: %s\n", obj.Type, obj.Name))
// 			f.WriteString("-- ----------------------------\n")
// 			f.WriteString(ddl + ";\n\n")
// 			mu.Unlock()

// 			progressChan <- fmt.Sprintf("[green]Exported %s: %s", obj.Type, obj.Name)

// 		}(obj)
// 	}

// 	wg.Wait()
// 	close(progressChan)
// }

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

				progressView := tview.NewTextView().
					SetDynamicColors(true).
					SetScrollable(true).
					SetChangedFunc(func() {
						app.Draw()
					})

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

					if event.Key() == tcell.KeyCtrlY {

						go func() {
							progressChan := make(chan string)

							go func() {
								for msg := range progressChan {
									util.SaveLog(msg)
								}
							}()
							app.QueueUpdateDraw(func() {
								progressView.SetText("[blue]Starting export...\n")
								app.SetRoot(progressView, true)
							})

							util.SaveLog(fmt.Sprintf("Exporting %d objects...\n", len(allTables)))

							go exportAllObjects("backup.sql", progressChan, dbName)

							// Read from progress channel and update UI
							go func() {
								for msg := range progressChan {
									app.QueueUpdateDraw(func() {
										fmt.Fprintln(progressView, msg)
									})
								}

								// After export is done
								app.QueueUpdateDraw(func() {
									modal := tview.NewModal().
										SetText("Export completed successfully!").
										AddButtons([]string{"OK"}).
										SetDoneFunc(func(buttonIndex int, buttonLabel string) {
											app.SetRoot(mainFlex, true)
										})
									app.SetRoot(modal, true)
								})
							}()
						}()
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
			case tcell.KeyCtrlU:
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

			case tcell.KeyCtrlP:
				clipboardText := util.GetClipboardText()
				queryBox.SetText(clipboardText, false)
				app.SetFocus(queryBox)
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
