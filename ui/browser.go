// ui/browser.go
package ui

import (
	"bufio"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"mysql-tui/dbs"
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

var (
	isQueryRunning    atomic.Bool
	activeQueryCancel context.CancelFunc
	activeQueryConnID atomic.Uint64
	activeQueryDB     *sql.DB
	activeQueryMutex  sync.Mutex
)

type QueryResult struct {
	Columns       []string
	Rows          [][]string
	RawRows       [][]*string
	ForeignKeys   map[string]util.ForeignKeyRef
	Duration      time.Duration
	RowCount      int
	RowsAffected  int64
	LastInsertID  int64
	IsMutation    bool
	StatementType string
	ErrorMessage  string
	IsCanceled    bool
	ServerThread  uint64
}

func CancelActiveQuery() {
	activeQueryMutex.Lock()
	cancel := activeQueryCancel
	connID := activeQueryConnID.Swap(0)
	db := activeQueryDB
	activeQueryCancel = nil
	activeQueryDB = nil
	isQueryRunning.Store(false)
	activeQueryMutex.Unlock()

	if cancel != nil {
		cancel()
	}

	if connID > 0 && db != nil {
		go func(targetID uint64, targetDB *sql.DB) {
			ctxKill, cancelKill := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancelKill()
			_, _ = targetDB.ExecContext(ctxKill, fmt.Sprintf("KILL QUERY %d", targetID))
		}(connID, db)
	}
}

// IsQueryInterruptedError determines whether an error indicates that the query was canceled or interrupted
func IsQueryInterruptedError(err error, ctx context.Context) bool {
	if ctx != nil && ctx.Err() == context.Canceled {
		return true
	}
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	errLower := strings.ToLower(err.Error())
	return strings.Contains(errLower, "interrupted") ||
		strings.Contains(errLower, "1317") ||
		strings.Contains(errLower, "operation was canceled") ||
		strings.Contains(errLower, "query execution was interrupted")
}

func IsQueryRunning() bool {
	return isQueryRunning.Load()
}

func GetActiveQueryConnectionID() uint64 {
	return activeQueryConnID.Load()
}

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
				dataBaseList.AddItem("  "+filterDbName, "Press Enter to open database", 0, func() {
					IsSearchStateEnabled = false
					UseDatabase(app, db, filterDbName)
				})
			}

		}

	} else {
		for _, obj := range allTable {
			// Match type filter if present (supports proc, func, trig, view, table)
			if typeFilter != "" {
				tf := strings.ToLower(typeFilter)
				ot := strings.ToLower(obj.Type)
				match := (ot == tf) ||
					(tf == "proc" && ot == "procedure") ||
					(tf == "func" && ot == "function") ||
					(tf == "trig" && ot == "trigger") ||
					(tf == "view" && ot == "view") ||
					(tf == "table" && ot == "table")
				if !match {
					continue
				}
			}

			// Match name
			if strings.Contains(strings.ToLower(obj.Name), search) {
				objName := obj.Name
				objType := obj.Type

				cleanName := fmt.Sprintf(" %s %s", FormatSchemaObjectBadge(obj.Type), obj.Name)
				list.AddItem(cleanName, "Press Enter to use", 0, func() {
					if isQueryRunning.Load() {
						CancelActiveQuery()
					}
					activeGridObject = objName
					activeGridObjectType = objType
					activeGridDBName = dbName
					currentGridPage = 0

					switch objType {
					case "TABLE", "VIEW":
						openTableOrView(app, db, queryBox, dataTable, dbName, objName, objType)

					case "PROCEDURE":
						query := `SELECT routine_name, data_type, is_deterministic, security_type, definer, routine_definition 
					FROM INFORMATION_SCHEMA.ROUTINES
					WHERE ROUTINE_NAME = '` + objName + `'
					AND ROUTINE_SCHEMA = '` + dbName + `' AND ROUTINE_TYPE = 'PROCEDURE';`

						routineDefinition, err := ExeQueryToData(db, objName, query, dbName, "PROCEDURE")
						if err != nil {
							showErrorModal(app, mainFlex, "Failed to execute query: "+err.Error())
							return
						}

						setEditorText(queryBox, routineDefinition)
						if activeSQLEditor != nil {
							app.SetFocus(activeSQLEditor.Editor)
						} else {
							app.SetFocus(queryBox)
						}

					case "FUNCTION":
						query := `SELECT routine_name, data_type, is_deterministic, security_type, definer, routine_definition 
					FROM INFORMATION_SCHEMA.ROUTINES
					WHERE ROUTINE_NAME = '` + objName + `'
					AND ROUTINE_SCHEMA = '` + dbName + `' AND ROUTINE_TYPE = 'FUNCTION';`
						routineDefinition, err := ExeQueryToData(db, objName, query, dbName, "FUNCTION")
						if err != nil {
							showErrorModal(app, mainFlex, "Failed to execute query: "+err.Error())
							return
						}

						setEditorText(queryBox, routineDefinition)
						if activeSQLEditor != nil {
							app.SetFocus(activeSQLEditor.Editor)
						} else {
							app.SetFocus(queryBox)
						}

					case "TRIGGER":
						definition, err := GetTriggerDDL(db, dbName, objName)
						if err != nil {
							showErrorModal(app, mainFlex, "Failed to fetch trigger: "+err.Error())
							return
						}
						setEditorText(queryBox, definition)
						if activeSQLEditor != nil {
							app.SetFocus(activeSQLEditor.Editor)
						} else {
							app.SetFocus(queryBox)
						}

					case "EVENT":
						definition, err := GetEventDDL(db, dbName, objName)
						if err != nil {
							showErrorModal(app, mainFlex, "Failed to fetch event: "+err.Error())
							return
						}
						setEditorText(queryBox, definition)
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
			layout := CreateLayoutWithFooter(app, mainFlex)
			app.SetRoot(layout, true)
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
			if layout == mainFlex || layout == nil {
				layout = CreateLayoutWithFooter(app, mainFlex)
			}
			app.SetRoot(layout, true)
			if dataTable != nil {
				util.SetFocusWithBorder(app, dataTable)
			}
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

	dbs.ConfigurePoolLimits(db, 10, 5, 5*time.Minute, 2*time.Minute)

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
	if app != nil {
		app.EnableMouse(true)
	}
	if GetUncommittedTxCount() > 0 {
		PromptUncommittedChangesModal(app, "switch databases",
			func() {
				if _, err := CommitActiveTx(app); err == nil && activeTx == nil {
					UseDatabase(app, db, dbName)
				}
			},
			func() {
				if _, err := RollbackActiveTx(app); err == nil && activeTx == nil {
					UseDatabase(app, db, dbName)
				}
			},
			func() {
				updateFooterText("Database switch cancelled.")
			},
		)
		return
	}

	runIcon := " Run "
	saveIcon := " Save "
	loadIcon := " Load "
	exitIcon := " Exit "

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
	activeGridDB = db
	activeGridDBName = dbName
	ActiveLoginDB = db
	ActiveScreen = "browser"

	dataBaseList = tview.NewList()
	dataBaseList.ShowSecondaryText(false).SetHighlightFullLine(true)
	StyleList(dataBaseList)
	StyleBox(dataBaseList.Box, "Databases", false)

	dataBaseList.SetFocusFunc(func() {
		StyleBox(dataBaseList.Box, "Databases", true)
	})
	dataBaseList.SetBlurFunc(func() {
		StyleBox(dataBaseList.Box, "Databases", false)
	})

	queryAllDB := `SHOW DATABASES;`

	allDatabases = nil
	dbRows, err := db.Query(queryAllDB)
	if err != nil {
		dataBaseList.AddItem("  Error: "+err.Error(), "", 0, nil)
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
			dbItemLabel := "  " + currentDBName
			if currentDBName == dbName {
				dbItemLabel = " [aqua::b]▶ " + currentDBName + "[-::-]"
			}
			dataBaseList.AddItem(dbItemLabel, "Press Enter to use", 0, func() {
				IsSearchStateEnabled = true
				UseDatabase(app, db, currentDBName)
			})
		}
	}

	// LEFT: Table list (using tview.List)
	tableList = tview.NewList()
	tableList.ShowSecondaryText(false).SetHighlightFullLine(true)
	StyleList(tableList)
	StyleBox(tableList.Box, "Schema Objects", false)

	tableList.SetFocusFunc(func() {
		StyleBox(tableList.Box, "Schema Objects", true)
	})
	tableList.SetBlurFunc(func() {
		StyleBox(tableList.Box, "Schema Objects", false)
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
		// Define queryBox outside the callback functions so it is in the scope
		var queryBox *tview.TextArea

		var name, objectType string
		allTables = []DBObject{}
		for rows.Next() {
			// rows.Scan(&name, &objectType)
			if err := rows.Scan(&name, &objectType); err != nil {
				log.Println("Scan error:", err)
				continue
			}

			allTables = append(allTables, DBObject{Name: name, Type: objectType})
			//rows.Scan(&tableName)
			currentName := name
			currentobjectType := objectType
			tableLabel := fmt.Sprintf(" %s %s", FormatSchemaObjectBadge(objectType), name)
			tableList.AddItem(tableLabel, "Press Enter to use", 0, func() {
				if isQueryRunning.Load() {
					CancelActiveQuery()
				}
				activeGridObject = currentName
				activeGridObjectType = currentobjectType
				activeGridDBName = dbName
				currentGridPage = 0

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
					openTableOrView(app, db, queryBox, dataTable, dbName, currentName, currentobjectType)
				}
			})
		}

		// Initialize queryBox and dataText outside of the callback scope
		runButton := tview.NewButton(runIcon).
			SetSelectedFunc(func() {
				query := ""
				if activeSQLEditor != nil {
					query = strings.TrimSpace(activeSQLEditor.GetText())
				} else if queryBox != nil {
					query = strings.TrimSpace(queryBox.GetText())
				}
				if query == "" {
					updateFooterText("SQL query is empty. Press 'e' or click in editor to type a query.")
					return
				}
				isEditingEnabled = false
				activeGridObjectType = ""
				activeGridSort.Reset(query, false)
				ClearNavHistory()
				ExecuteQueryAsync(app, db, query, dataTable, func(res *QueryResult) {
					phhistory.SaveQuery(query, dbName)
					app.SetFocus(dataTable)
				})
			})

		queryBox = tview.NewTextArea()
		queryBox.SetPlaceholder("Enter SQL query here...")
		StyleBox(queryBox.Box, "SQL Query Editor", false)

		activeSQLEditor = NewSQLEditor(app)
		activeSQLEditor.OnExecute = func(query string) {
			cleanQuery := strings.TrimSpace(query)
			if cleanQuery == "" {
				updateFooterText("SQL query is empty. Type a query to execute.")
				return
			}
			isEditingEnabled = false
			activeGridObjectType = ""
			activeGridSort.Reset(cleanQuery, false)
			ClearNavHistory()
			ExecuteQueryAsync(app, db, cleanQuery, dataTable, func(res *QueryResult) {
				phhistory.SaveQuery(cleanQuery, dbName)
				app.SetFocus(dataTable)
			})
		}
		activeSQLEditor.OnNextFocus = func() {
			app.SetFocus(runButton)
		}
		activeSQLEditor.OnPrevFocus = func() {
			if tableList != nil {
				app.SetFocus(tableList)
			}
		}
		var isEditorFullscreen bool = false
		activeSQLEditor.OnExit = func() {
			if isEditorFullscreen {
				layout := CreateLayoutWithFooter(app, mainFlex)
				app.SetRoot(layout, true)
				app.SetFocus(activeSQLEditor.Editor)
				isEditorFullscreen = false
				return
			}
			app.SetFocus(tableList)
		}
		activeSQLEditor.OnFullscreen = func() {
			if isEditorFullscreen {
				layout := CreateLayoutWithFooter(app, mainFlex)
				app.SetRoot(layout, true)
				app.SetFocus(activeSQLEditor.Editor)
				isEditorFullscreen = false
			} else {
				app.SetRoot(activeSQLEditor.Container, true)
				app.SetFocus(activeSQLEditor.Editor)
				isEditorFullscreen = true
			}
		}

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

		button1.SetBorderPadding(0, 0, 1, 1)

		button2 := tview.NewButton(loadIcon).SetSelectedFunc(func() {
		})
		button2.SetBorderPadding(0, 0, 1, 1)

		exitButton := tview.NewButton(exitIcon).SetSelectedFunc(func() {
			if GetUncommittedTxCount() > 0 {
				PromptUncommittedChangesModal(app, "exit Pheri",
					func() {
						_, _ = CommitActiveTx(app)
						app.Stop()
					},
					func() {
						_, _ = RollbackActiveTx(app)
						app.Stop()
					},
					func() {
						updateFooterText("Exit cancelled.")
					},
				)
				return
			}
			app.Stop()
		})
		exitButton.SetBorderPadding(0, 0, 2, 2)

		commitButton := tview.NewButton(" Commit ")
		commitButton.SetBorderPadding(0, 0, 1, 1)

		rollbackButton := tview.NewButton(" Rollback ")
		rollbackButton.SetBorderPadding(0, 0, 1, 1)

		updateTransactionToolbarButtons := func() {
			if IsAutoCommitMode() {
				commitButton.SetLabel(" Commit ")
				rollbackButton.SetLabel(" Rollback ")
				commitButton.SetBackgroundColor(tcell.ColorDarkGray)
				rollbackButton.SetBackgroundColor(tcell.ColorDarkGray)
			} else {
				count := GetUncommittedTxCount()
				if count == 0 {
					commitButton.SetLabel(" Commit (0) ")
					rollbackButton.SetLabel(" Rollback (0) ")
					commitButton.SetBackgroundColor(tcell.ColorDarkCyan)
					rollbackButton.SetBackgroundColor(tcell.ColorDarkCyan)
				} else {
					commitButton.SetLabel(fmt.Sprintf(" Commit (%d) ", count))
					rollbackButton.SetLabel(fmt.Sprintf(" Rollback (%d) ", count))
					commitButton.SetBackgroundColor(tcell.ColorDarkGreen)
					rollbackButton.SetBackgroundColor(tcell.ColorDarkRed)
				}
			}
		}

		updateTransactionToolbarButtons()

		SetOnTxStateChangedHook(func() {
			if app != nil {
				go func() {
					app.QueueUpdateDraw(func() {
						updateTransactionToolbarButtons()
					})
				}()
			}
		})

		commitButton.SetSelectedFunc(func() {
			if IsAutoCommitMode() {
				updateFooterText("Auto-Commit is ON. Press F2 or run :autocommit to enable manual transactions.")
				return
			}
			if GetUncommittedTxCount() == 0 {
				updateFooterText("No active uncommitted changes to commit.")
				return
			}
			if count, err := CommitActiveTx(app); err == nil && count > 0 {
				updateTransactionToolbarButtons()
			}
		})

		rollbackButton.SetSelectedFunc(func() {
			if IsAutoCommitMode() {
				updateFooterText("Auto-Commit is ON. Press F2 or run :autocommit to enable manual transactions.")
				return
			}
			if GetUncommittedTxCount() == 0 {
				updateFooterText("No active uncommitted changes to rollback.")
				return
			}
			if count, err := RollbackActiveTx(app); err == nil && count > 0 {
				updateTransactionToolbarButtons()
				if activeGridDB != nil && activeGridQuery != "" && dataTable != nil {
					ExecuteQueryAsync(app, activeGridDB, activeGridQuery, dataTable, nil)
				}
			}
		})

		runButton.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyBacktab {
				if activeSQLEditor != nil && activeSQLEditor.Editor != nil {
					app.SetFocus(activeSQLEditor.Editor)
				}
				return nil
			}
			if event.Key() == tcell.KeyTab {
				app.SetFocus(commitButton)
				return nil
			}
			return event
		})

		commitButton.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape {
				if activeSQLEditor != nil && activeSQLEditor.Editor != nil {
					app.SetFocus(activeSQLEditor.Editor)
				}
				return nil
			}
			if event.Key() == tcell.KeyBacktab {
				app.SetFocus(runButton)
				return nil
			}
			if event.Key() == tcell.KeyTab {
				app.SetFocus(rollbackButton)
				return nil
			}
			return event
		})

		rollbackButton.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape {
				if activeSQLEditor != nil && activeSQLEditor.Editor != nil {
					app.SetFocus(activeSQLEditor.Editor)
				}
				return nil
			}
			if event.Key() == tcell.KeyBacktab {
				app.SetFocus(commitButton)
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
				if activeSQLEditor != nil && activeSQLEditor.Editor != nil {
					app.SetFocus(activeSQLEditor.Editor)
				}
				return nil
			}
			if event.Key() == tcell.KeyBacktab {
				app.SetFocus(rollbackButton)
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
				if activeSQLEditor != nil && activeSQLEditor.Editor != nil {
					app.SetFocus(activeSQLEditor.Editor)
				}
				return nil
			}
			if event.Key() == tcell.KeyBacktab {
				app.SetFocus(button1)
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
				if activeSQLEditor != nil && activeSQLEditor.Editor != nil {
					app.SetFocus(activeSQLEditor.Editor)
				}
				return nil
			}
			if event.Key() == tcell.KeyBacktab {
				app.SetFocus(button2)
				return nil
			}
			if event.Key() == tcell.KeyTab {
				if dataTable != nil {
					app.SetFocus(dataTable)
				}
				return nil
			}
			return event
		})

		queryPanel := tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(activeSQLEditor.Container, 0, 1, true).
			AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
				AddItem(runButton, 10, 0, false).
				AddItem(nil, 1, 0, false).
				AddItem(commitButton, 12, 0, false).
				AddItem(nil, 1, 0, false).
				AddItem(rollbackButton, 13, 0, false).
				AddItem(nil, 1, 0, false).
				AddItem(button1, 10, 0, false).
				AddItem(nil, 1, 0, false).
				AddItem(button2, 10, 0, false).
				AddItem(nil, 1, 0, false).
				AddItem(exitButton, 9, 0, false).
				AddItem(nil, 0, 1, false), 1, 0, false)

		dataTable = tview.NewTable()
		dataTable.
			SetBorders(true).
			SetSelectable(true, false).
			SetFixed(1, 0)
		StyleTable(dataTable)
		StyleBox(dataTable.Box, "Query Results", false)

		dataTable.SetFocusFunc(func() {
			StyleBox(dataTable.Box, "Query Results", true)
		})
		dataTable.SetBlurFunc(func() {
			StyleBox(dataTable.Box, "Query Results", false)
		})

		dataTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyRune && (event.Rune() == 'e' || event.Rune() == 'E' || event.Rune() == 'i' || event.Rune() == 'I') {
				if activeSQLEditor != nil {
					app.SetFocus(activeSQLEditor.Editor)
					return nil
				}
			}
			if event.Key() == tcell.KeyCtrlQ || event.Key() == tcell.KeyCtrlI {
				if activeSQLEditor != nil {
					app.SetFocus(activeSQLEditor.Editor)
					return nil
				}
			}
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
					headerCell := dataTable.GetCell(0, c)
					colName := ""
					if headerCell != nil {
						colName = util.StripFormatting(headerCell.Text)
					}
					gc := GetGridCellFromTableCell(cell, colName)
					inspectText := gc.Value()
					if gc.IsNull() {
						inspectText = "(NULL)"
					}
					ShowCellInspectorModal(app, inspectText)
					return nil
				}
			}
			return event
		})

		searchInput := tview.NewInputField()
		SchemaSearchInput = searchInput
		searchInput.SetFieldBackgroundColor(CurrentTheme.SurfaceBg).
			SetFieldTextColor(CurrentTheme.TextPrimary).
			SetLabel(" Filter: ").
			SetLabelColor(CurrentTheme.AccentPrimary).
			SetPlaceholder("Search objects...").
			SetPlaceholderTextColor(CurrentTheme.TextSecondary).
			SetFieldWidth(22)

		searchInput.SetFocusFunc(func() {
			searchInput.SetLabelColor(CurrentTheme.BorderFocused)
		})
		searchInput.SetBlurFunc(func() {
			searchInput.SetLabelColor(CurrentTheme.AccentPrimary)
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
			if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyDown {
				if tableList != nil {
					app.SetFocus(tableList)
				}
				return nil
			}
			if event.Key() == tcell.KeyBacktab {
				if dataTable != nil {
					app.SetFocus(dataTable)
				}
				return nil
			}
			if event.Key() == tcell.KeyEscape {
				if GetUncommittedTxCount() > 0 {
					PromptUncommittedChangesModal(app, "return to database list",
						func() {
							_, _ = CommitActiveTx(app)
							ShowDatabaseList(app, db)
						},
						func() {
							_, _ = RollbackActiveTx(app)
							ShowDatabaseList(app, db)
						},
						func() {
							updateFooterText("Navigation to database list cancelled.")
							app.SetFocus(searchInput)
						},
					)
					return nil
				}
				ShowDatabaseList(app, db)
				return nil
			}
			return event
		})

		StyleList(tableList)
		StyleBox(tableList.Box, "Schema Objects", false)

		tableList.SetFocusFunc(func() {
			StyleBox(tableList.Box, "Schema Objects", true)
		})
		tableList.SetBlurFunc(func() {
			StyleBox(tableList.Box, "Schema Objects", false)
		})

		tableList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyRune && (event.Rune() == 'e' || event.Rune() == 'E') {
				if activeSQLEditor != nil && activeSQLEditor.Editor != nil {
					app.SetFocus(activeSQLEditor.Editor)
					return nil
				}
			}
			if event.Key() == tcell.KeyTab {
				if dataBaseList != nil {
					app.SetFocus(dataBaseList)
				}
				return nil
			}
			if event.Key() == tcell.KeyBacktab {
				if dataTable != nil {
					app.SetFocus(dataTable)
				}
				return nil
			}
			if event.Key() == tcell.KeyRight {
				if dataTable != nil {
					app.SetFocus(dataTable)
				}
				return nil
			}
			if event.Key() == tcell.KeyEscape {
				app.SetFocus(searchInput)
				return nil
			}
			return event
		})

		StyleList(dataBaseList)
		StyleBox(dataBaseList.Box, "Databases", false)

		dataBaseList.SetFocusFunc(func() {
			StyleBox(dataBaseList.Box, "Databases", true)
		})
		dataBaseList.SetBlurFunc(func() {
			StyleBox(dataBaseList.Box, "Databases", false)
		})

		dataBaseList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyRune && (event.Rune() == 'e' || event.Rune() == 'E') {
				if activeSQLEditor != nil && activeSQLEditor.Editor != nil {
					app.SetFocus(activeSQLEditor.Editor)
					return nil
				}
			}
			if event.Key() == tcell.KeyTab {
				if activeSQLEditor != nil && activeSQLEditor.Editor != nil {
					app.SetFocus(activeSQLEditor.Editor)
					return nil
				}
			}
			if event.Key() == tcell.KeyBacktab {
				if tableList != nil {
					app.SetFocus(tableList)
				}
				return nil
			}
			if event.Key() == tcell.KeyRight {
				if dataTable != nil {
					app.SetFocus(dataTable)
				}
				return nil
			}
			if event.Key() == tcell.KeyEscape {
				if tableList != nil {
					app.SetFocus(tableList)
				}
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
			AddItem(queryPanel, 9, 0, true).
			AddItem(dataTable, 0, 1, false)

		// Main layout
		mainFlex = tview.NewFlex().
			AddItem(mainLeftPanel, 32, 0, true).
			AddItem(mainCenterPanel, 0, 1, false)
		layout := CreateLayoutWithFooter(app, mainFlex)
		app.SetRoot(layout, true)
		setupGlobalInputCapture(app, db, dbName)
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
		mainFlex.AddItem(mainLeftPanel, 32, 0, false).
			AddItem(mainCenterPanel, 0, 1, true)
		updateFooterText("Sidebar restored.")
	}
}

var (
	pkCache      = make(map[string][]string)
	pkCacheMutex sync.RWMutex
)

// GetPrimaryKeyColumns dynamically fetches and caches primary key column names
func GetPrimaryKeyColumns(db *sql.DB, dbName, tableName string) ([]string, error) {
	if db == nil || dbName == "" || tableName == "" {
		return nil, nil
	}

	cacheKey := dbName + "." + tableName
	pkCacheMutex.RLock()
	if cols, ok := pkCache[cacheKey]; ok {
		pkCacheMutex.RUnlock()
		return cols, nil
	}
	pkCacheMutex.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	query := `
	SELECT COLUMN_NAME
	FROM INFORMATION_SCHEMA.COLUMNS
	WHERE TABLE_SCHEMA = ?
	  AND TABLE_NAME = ?
	  AND COLUMN_KEY = 'PRI'
	ORDER BY ORDINAL_POSITION
	`
	rows, err := db.QueryContext(ctx, query, dbName, tableName)
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

	pkCacheMutex.Lock()
	pkCache[cacheKey] = cols
	pkCacheMutex.Unlock()

	return cols, nil
}

func GetPrimaryKeyColumn(db *sql.DB, dbName, tableName string) (string, error) {
	cols, err := GetPrimaryKeyColumns(db, dbName, tableName)
	if err != nil || len(cols) == 0 {
		return "", err
	}
	return cols[0], nil
}

// openTableOrView initializes browse/sort state for a selected table or view and loads page 0.
func openTableOrView(app *tview.Application, db *sql.DB, queryBox *tview.TextArea, dataTable *tview.Table, dbName, objName, objType string) {
	activeGridObject = objName
	activeGridObjectType = objType
	activeGridDBName = dbName
	currentGridPage = 0

	query := BuildBrowseQuery(dbName, objName, "", SortNone, gridPageSize, 0)
	activeGridSort.Reset(query, true)
	ClearNavHistory()
	setEditorText(queryBox, query)
	util.SaveLog(objType + ": " + query)
	phhistory.SaveQuery(query, dbName)

	err := ExecuteQuery(app, db, query, dataTable)
	if err != nil {
		showErrorModal(app, mainFlex, "Executing Fail: "+err.Error())
	}

	if objType == "TABLE" {
		isEditingEnabled = true
		_ = EnableCellEditing(app, dataTable, db, dbName, objName)
	} else {
		isEditingEnabled = false
	}
	if app != nil {
		app.SetFocus(dataTable)
	}
}

// Fetch data and show in table asynchronously with cancellation, live timing, and pagination
func ExecuteQuery(app *tview.Application, db *sql.DB, query string, table *tview.Table) error {
	ExecuteQueryAsync(app, db, query, table, nil)
	return nil
}

var activeQuerySequence uint64

// ExecuteQueryAsync executes a database query off the UI thread with context cancellation
func ExecuteQueryAsync(app *tview.Application, db *sql.DB, query string, table *tview.Table, onComplete func(res *QueryResult)) {
	if isQueryRunning.Load() {
		updateFooterText("A query is already executing. Press Ctrl+C or Esc to cancel it first.")
		return
	}
	thisSeq := atomic.AddUint64(&activeQuerySequence, 1)

	if db == nil {
		updateFooterText("Database connection is not initialized.")
		if onComplete != nil {
			onComplete(&QueryResult{ErrorMessage: "database connection is not initialized"})
		}
		return
	}

	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		updateFooterText("SQL query is empty. Type a query to execute.")
		if onComplete != nil {
			onComplete(&QueryResult{ErrorMessage: "empty query"})
		}
		return
	}

	activeGridQuery = query
	activeGridDB = db

	isMutation, stmtType := ClassifySQLStatement(query)

	if isMutation && ActiveReadOnly {
		showErrorModal(app, mainFlex, fmt.Sprintf("Action Blocked: %s statement not allowed in READ-ONLY Mode.", stmtType))
		if onComplete != nil {
			onComplete(&QueryResult{ErrorMessage: "read-only mode enabled", IsMutation: true, StatementType: stmtType})
		}
		return
	}

	activeQueryMutex.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	activeQueryCancel = cancel
	activeQueryDB = db
	isQueryRunning.Store(true)
	activeQueryMutex.Unlock()

	table.Clear()
	table.SetBorders(false)
	table.SetCell(0, 0, tview.NewTableCell("[yellow::b]Executing query in background... [aqua](Ctrl+C / Esc: Cancel)[-::-]").
		SetAlign(tview.AlignCenter).
		SetSelectable(false))
	UpdateFooterRunningQuery(0)

	startTime := time.Now()
	fkTargetDB := activeGridDBName
	fkTargetTable := activeGridObject

	// Live progress ticker
	tickerDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-tickerDone:
				return
			case t := <-ticker.C:
				elapsed := t.Sub(startTime)
				app.QueueUpdateDraw(func() {
					if isQueryRunning.Load() {
						UpdateFooterRunningQuery(elapsed)
						table.SetCell(0, 0, tview.NewTableCell(fmt.Sprintf("[yellow::b]Executing query... (%0.1fs) [aqua::b](Ctrl+C / Esc: Cancel)[-::-]", elapsed.Seconds())).
							SetAlign(tview.AlignCenter).
							SetSelectable(false))
					}
				})
			}
		}
	}()

	// Background worker
	go func() {
		defer func() {
			if r := recover(); r != nil {
				util.SaveLog(fmt.Sprintf("Query execution panic recovered: %v", r))
			}
			close(tickerDone)
			activeQueryMutex.Lock()
			if thisSeq == atomic.LoadUint64(&activeQuerySequence) {
				activeQueryCancel = nil
				activeQueryDB = nil
				isQueryRunning.Store(false)
			}
			activeQueryMutex.Unlock()
			activeQueryConnID.Store(0)
		}()

		result := &QueryResult{
			IsMutation:    isMutation,
			StatementType: stmtType,
		}

		var tx *sql.Tx
		var conn *sql.Conn
		if !IsAutoCommitMode() {
			var errTx error
			tx, errTx = EnsureActiveTx(ctx, db)
			if errTx != nil {
				result.Duration = time.Since(startTime)
				result.ErrorMessage = "Failed to acquire active transaction: " + errTx.Error()
				app.QueueUpdateDraw(func() {
					renderQueryResult(app, result, table)
					if onComplete != nil {
						onComplete(result)
					}
				})
				return
			}
			var threadID uint64
			idRow := tx.QueryRowContext(ctx, "SELECT CONNECTION_ID()")
			if err := idRow.Scan(&threadID); err == nil && threadID > 0 {
				activeQueryConnID.Store(threadID)
				result.ServerThread = threadID
			}
		} else {
			var errConn error
			conn, errConn = db.Conn(ctx)
			if errConn != nil {
				result.Duration = time.Since(startTime)
				if IsQueryInterruptedError(errConn, ctx) {
					result.IsCanceled = true
				} else {
					result.ErrorMessage = "Failed to acquire database connection: " + errConn.Error()
				}
				app.QueueUpdateDraw(func() {
					renderQueryResult(app, result, table)
					if onComplete != nil {
						onComplete(result)
					}
				})
				return
			}
			defer conn.Close()

			var threadID uint64
			idRow := conn.QueryRowContext(ctx, "SELECT CONNECTION_ID()")
			if err := idRow.Scan(&threadID); err == nil && threadID > 0 {
				activeQueryConnID.Store(threadID)
				result.ServerThread = threadID
			}
		}

		if isMutation {
			var execRes sql.Result
			var err error
			if tx != nil {
				execRes, err = tx.ExecContext(ctx, query)
				if err == nil {
					RecordMutationInTx(query, "SQL Editor")
				}
			} else {
				execRes, err = conn.ExecContext(ctx, query)
			}
			duration := time.Since(startTime)
			result.Duration = duration

			if err != nil {
				if IsQueryInterruptedError(err, ctx) {
					result.IsCanceled = true
				} else {
					result.ErrorMessage = err.Error()
				}
			} else {
				if rowsAff, errAff := execRes.RowsAffected(); errAff == nil {
					result.RowsAffected = rowsAff
				}
				if lastID, errID := execRes.LastInsertId(); errID == nil && lastID > 0 {
					result.LastInsertID = lastID
				}
			}
		} else {
			var rows *sql.Rows
			var err error
			if tx != nil {
				rows, err = tx.QueryContext(ctx, query)
			} else {
				rows, err = conn.QueryContext(ctx, query)
			}
			duration := time.Since(startTime)
			result.Duration = duration

			if err != nil {
				if IsQueryInterruptedError(err, ctx) {
					result.IsCanceled = true
				} else {
					result.ErrorMessage = err.Error()
				}
			} else {
				defer rows.Close()

				cols, err := rows.Columns()
				if err != nil {
					result.ErrorMessage = err.Error()
				} else {
					result.Columns = cols
					scanValues := make([]interface{}, len(cols))
					rawValues := make([]sql.RawBytes, len(cols))
					for i := range rawValues {
						scanValues[i] = &rawValues[i]
					}

					for rows.Next() {
						if ctx.Err() == context.Canceled {
							result.IsCanceled = true
							break
						}
						if scanErr := rows.Scan(scanValues...); scanErr != nil {
							if IsQueryInterruptedError(scanErr, ctx) {
								result.IsCanceled = true
								break
							}
							continue
						}

						rowStrings := make([]string, len(cols))
						rawRow := make([]*string, len(cols))
						for i, col := range rawValues {
							if col == nil {
								rawRow[i] = nil
								rowStrings[i] = "[gray::i]NULL[-::-]"
							} else {
								sVal := string(col)
								rawRow[i] = &sVal
								if sVal == "" {
									rowStrings[i] = "[gray::i]EMPTY[-::-]"
								} else {
									rowStrings[i] = tview.Escape(sVal)
								}
							}
						}
						result.Rows = append(result.Rows, rowStrings)
						result.RawRows = append(result.RawRows, rawRow)
					}
					if rows.Err() != nil {
						if IsQueryInterruptedError(rows.Err(), ctx) {
							result.IsCanceled = true
						}
					}
					result.RowCount = len(result.Rows)
					if fkTargetDB != "" && fkTargetTable != "" && !result.IsCanceled {
						result.ForeignKeys = GetCachedForeignKeys(ctx, db, fkTargetDB, fkTargetTable)
					}
				}
			}
		}

		// Update UI on main thread safely
		app.QueueUpdateDraw(func() {
			if thisSeq != atomic.LoadUint64(&activeQuerySequence) {
				return
			}
			renderQueryResult(app, result, table)
			if onComplete != nil {
				onComplete(result)
			}
		})
	}()
}

func renderQueryResult(app *tview.Application, res *QueryResult, table *tview.Table) {
	table.Clear()
	table.SetBorders(false)

	if res.IsCanceled {
		titleStr := fmt.Sprintf(" [ Query Result: Aborted (%s) ] ", res.Duration.Round(time.Millisecond))
		table.SetTitle(titleStr).SetTitleAlign(tview.AlignLeft).SetBorder(true)
		var threadInfo string
		if res.ServerThread > 0 {
			threadInfo = fmt.Sprintf(" [gray](MySQL Thread %d killed on server)[-]", res.ServerThread)
		}
		table.SetCell(0, 0, tview.NewTableCell(fmt.Sprintf("[yellow::b]Query execution canceled by user (Ctrl+C / Esc) [white](aborted after %s)[-::-]%s", res.Duration.Round(time.Millisecond), threadInfo)).
			SetAlign(tview.AlignCenter).
			SetSelectable(false))
		updateFooterText(fmt.Sprintf("Query canceled after %s%s", res.Duration.Round(time.Millisecond), strings.TrimPrefix(threadInfo, " ")))
		setupTableInputCapture(app, table)
		return
	}

	if res.ErrorMessage != "" {
		titleStr := " [ Query Result: Error ] "
		table.SetTitle(titleStr).SetTitleAlign(tview.AlignLeft).SetBorder(true)
		table.SetCell(0, 0, tview.NewTableCell("[red::b]Error: "+res.ErrorMessage).
			SetAlign(tview.AlignLeft).
			SetSelectable(false))
		updateFooterText("Query failed: " + res.ErrorMessage)
		setupTableInputCapture(app, table)
		return
	}

	if res.IsMutation {
		titleType := res.StatementType
		if titleType == "" {
			titleType = "Mutation"
		}
		titleStr := fmt.Sprintf(" [ %s: %d rows affected (%s) ] ",
			titleType, res.RowsAffected, res.Duration.Round(time.Microsecond))
		table.SetTitle(titleStr).SetTitleAlign(tview.AlignLeft).SetBorder(true)

		insertInfo := ""
		if res.LastInsertID > 0 {
			insertInfo = fmt.Sprintf("  [gray]│[-]  Last Insert ID: [aqua::b]%d[-::-]", res.LastInsertID)
		}
		table.SetCell(0, 0, tview.NewTableCell(fmt.Sprintf("[lime::b]Query OK, %d row(s) affected [white](Execution time: %s)[-::-]%s",
			res.RowsAffected, res.Duration.Round(time.Microsecond), insertInfo)).
			SetAlign(tview.AlignCenter).
			SetSelectable(false))

		footerInsertInfo := ""
		if res.LastInsertID > 0 {
			footerInsertInfo = fmt.Sprintf(", Last Insert ID: %d", res.LastInsertID)
		}
		updateFooterText(fmt.Sprintf("Query OK, %d row(s) affected (%s)%s",
			res.RowsAffected, res.Duration.Round(time.Microsecond), footerInsertInfo))
		setupTableInputCapture(app, table)
		return
	}

	// Cache original unsorted result for fallback in-memory sorting (e.g. SHOW, DESCRIBE)
	activeGridSort.mu.Lock()
	if activeGridSort.Direction == SortNone {
		activeGridSort.OriginalResult = cloneQueryResultForSort(res)
	}
	sortCol := activeGridSort.Column
	sortDir := activeGridSort.Direction
	restoreRow := activeGridSort.PendingRestoreRow
	restoreCol := activeGridSort.PendingRestoreCol
	activeGridSort.PendingRestoreRow = -1
	activeGridSort.PendingRestoreCol = -1
	activeGridSort.mu.Unlock()

	if res.ForeignKeys != nil {
		activeGridForeignKeys = res.ForeignKeys
	} else {
		activeGridForeignKeys = nil
	}

	fkNavMutex.Lock()
	navRestoreRow := pendingNavRestoreRow
	navRestoreCol := pendingNavRestoreCol
	pendingNavRestoreRow = -1
	pendingNavRestoreCol = -1
	fkNavMutex.Unlock()

	// Set column headers with professional styling, sort & FK indicators
	headerStyle := tcell.StyleDefault.
		Background(CurrentTheme.HeaderBg).
		Foreground(CurrentTheme.HeaderFg).
		Bold(true)
	sortedHeaderStyle := tcell.StyleDefault.
		Background(CurrentTheme.HeaderBg).
		Foreground(CurrentTheme.AccentPrimary).
		Bold(true)
	fkHeaderStyle := tcell.StyleDefault.
		Background(CurrentTheme.HeaderBg).
		Foreground(CurrentTheme.AccentInfo).
		Bold(true)

	for i, col := range res.Columns {
		_, isFK := activeGridForeignKeys[col]
		label := " " + col + " "
		style := headerStyle
		if isFK {
			label = fmt.Sprintf(" %s ↗ ", col)
			style = fkHeaderStyle
		}
		if sortCol != "" && col == sortCol && sortDir != SortNone {
			if isFK {
				label = fmt.Sprintf(" %s ↗ %s ", col, sortDir.Indicator())
			} else {
				label = fmt.Sprintf(" %s %s ", col, sortDir.Indicator())
			}
			style = sortedHeaderStyle
		}
		table.SetCell(0, i,
			tview.NewTableCell(label).
				SetStyle(style).
				SetAlign(tview.AlignCenter).
				SetSelectable(true).
				SetReference(col))
	}

	// Set rows with clean theme colors & FK reference highlights
	for rIdx, row := range res.Rows {
		for cIdx, val := range row {
			cellStyle := tcell.StyleDefault.
				Foreground(CurrentTheme.TextPrimary).
				Background(CurrentTheme.SurfaceBg)
			if (rIdx+1)%2 == 0 {
				if CurrentTheme.IsLight {
					cellStyle = cellStyle.Background(tcell.NewRGBColor(241, 245, 249)).
						Foreground(CurrentTheme.TextPrimary)
				} else {
					cellStyle = cellStyle.Foreground(CurrentTheme.TextSecondary)
				}
			}

			var rawVal *string
			if rIdx < len(res.RawRows) && cIdx < len(res.RawRows[rIdx]) {
				rawVal = res.RawRows[rIdx][cIdx]
			}
			colName := ""
			if cIdx < len(res.Columns) {
				colName = res.Columns[cIdx]
			}

			_, isFKCol := activeGridForeignKeys[colName]
			isCellNull := (rIdx < len(res.RawRows) && cIdx < len(res.RawRows[rIdx]) && rawVal == nil) ||
				(rIdx >= len(res.RawRows) && val == "[gray::i]NULL[-::-]")

			displayText := " " + val + " "
			if isFKCol && !isCellNull {
				displayText = " " + val + " ↗ "
				cellStyle = cellStyle.Foreground(CurrentTheme.AccentInfo)
			}

			cell := tview.NewTableCell(displayText).
				SetStyle(cellStyle).
				SetAlign(tview.AlignLeft)

			cell.SetReference(NewGridCell(colName, rawVal))
			table.SetCell(rIdx+1, cIdx, cell)
		}
	}

	table.SetSelectable(true, true)
	if !isEditingEnabled {
		table.SetSelectedFunc(func(row int, column int) {
			if row == 0 {
				triggerColumnSort(app, table, column)
			}
		})
	}

	// Restore selection after FK navigation return or column sort
	if navRestoreCol >= 0 && navRestoreRow >= 0 && navRestoreCol < len(res.Columns) {
		restoreRow = navRestoreRow
		restoreCol = navRestoreCol
	}
	if restoreCol >= 0 && restoreCol < len(res.Columns) {
		maxRow := table.GetRowCount() - 1
		if restoreRow < 0 {
			restoreRow = 0
		} else if restoreRow > maxRow {
			restoreRow = maxRow
		}
		table.Select(restoreRow, restoreCol)
	}

	sortBadge := ""
	if sortCol != "" && sortDir != SortNone {
		sortBadge = fmt.Sprintf(" | Sort: %s %s", sortCol, sortDir.Indicator())
	}
	navBadge := ""
	if depth := NavHistoryDepth(); depth > 0 {
		navBadge = fmt.Sprintf(" | Backspace: Return (%d)", depth)
	}
	titleStr := fmt.Sprintf(" [ Query Results: %d rows | %s | Page %d%s%s ] ",
		res.RowCount, res.Duration.Round(time.Microsecond), currentGridPage+1, sortBadge, navBadge)
	table.SetTitle(titleStr).SetTitleAlign(tview.AlignLeft).SetBorder(true)

	footerMsg := fmt.Sprintf("Fetched %d row(s) in %s", res.RowCount, res.Duration.Round(time.Microsecond))
	if sortCol != "" && sortDir != SortNone {
		footerMsg += fmt.Sprintf(" │ Sorted by %s (%s) — 's'/Enter on header", sortCol, sortDir.SQLKeyword())
	}
	if len(activeGridForeignKeys) > 0 {
		footerMsg += " │ FK '↗': f/Ctrl+J Jump"
	}
	if NavHistoryDepth() > 0 {
		footerMsg += " │ Backspace: Return"
	}
	updateFooterText(footerMsg)
	setupTableInputCapture(app, table)
}

// getHeaderColumnName resolves the underlying SQL column identifier from a header TableCell,
// ignoring directional sort indicators (▲ / ▼), FK markers (↗), and tview formatting tags.
func getHeaderColumnName(headerCell *tview.TableCell) string {
	if headerCell == nil {
		return ""
	}
	if ref, ok := headerCell.GetReference().(string); ok && ref != "" {
		return ref
	}
	clean := strings.TrimSpace(util.StripFormatting(headerCell.Text))
	for _, suffix := range []string{" ▲", " ▼", "▲", "▼", " ↗", "↗"} {
		clean = strings.TrimSuffix(clean, suffix)
	}
	return strings.TrimSpace(clean)
}

// triggerForeignKeyJump follows the foreign key relationship of the currently selected Data Grid cell.
// Returns true if the selected column is an FK column (even if NULL/blocked), or false if not an FK column.
func triggerForeignKeyJump(app *tview.Application, table *tview.Table) bool {
	if table == nil || len(activeGridForeignKeys) == 0 {
		return false
	}
	r, c := table.GetSelection()
	if r <= 0 || c < 0 {
		return false
	}
	colName := getHeaderColumnName(table.GetCell(0, c))
	ref, isFK := activeGridForeignKeys[colName]
	if !isFK {
		return false
	}

	if isQueryRunning.Load() {
		updateFooterText("A query is currently running. Cancel it before following foreign key.")
		return true
	}

	cell := table.GetCell(r, c)
	gc := GetGridCellFromTableCell(cell, colName)
	if gc.IsNull() {
		updateFooterText(fmt.Sprintf("Cannot follow NULL foreign key in column '%s'.", colName))
		return true
	}

	if activeGridDB == nil {
		updateFooterText("Database connection is not active.")
		return true
	}

	sortCol, sortDir, _, _ := activeGridSort.Snapshot()
	PushNavBreadcrumb(NavBreadcrumb{
		DBName:        activeGridDBName,
		TableName:     activeGridObject,
		ObjectType:    activeGridObjectType,
		Query:         activeGridQuery,
		Page:          currentGridPage,
		CursorRow:     r,
		CursorCol:     c,
		EditingActive: isEditingEnabled,
		SortColumn:    sortCol,
		SortDir:       sortDir,
	})

	targetQuery := util.BuildForeignKeyTargetQuery(ref, gc.Value())
	targetDB := ref.ReferencedSchema
	if targetDB == "" {
		targetDB = activeGridDBName
	}

	activeGridDBName = targetDB
	activeGridObject = ref.ReferencedTable
	activeGridObjectType = "TABLE"
	currentGridPage = 0
	activeGridSort.Reset(targetQuery, false)

	if activeSQLEditor != nil {
		activeSQLEditor.SetText(targetQuery)
	}

	isEditingEnabled = true
	_ = EnableCellEditing(app, table, activeGridDB, activeGridDBName, activeGridObject)

	ExecuteQueryAsync(app, activeGridDB, targetQuery, table, func(res *QueryResult) {
		if app != nil {
			app.SetFocus(table)
		}
	})
	return true
}

// navigateBackFromFK pops the top FK navigation breadcrumb and restores the prior table, query, and cursor cell.
func navigateBackFromFK(app *tview.Application, table *tview.Table) bool {
	crumb, ok := PopNavBreadcrumb()
	if !ok {
		return false
	}
	if isQueryRunning.Load() {
		CancelActiveQuery()
	}

	activeGridDBName = crumb.DBName
	activeGridObject = crumb.TableName
	activeGridObjectType = crumb.ObjectType
	currentGridPage = crumb.Page

	isBrowse := crumb.ObjectType == "TABLE" || crumb.ObjectType == "VIEW"
	activeGridSort.mu.Lock()
	activeGridSort.Column = crumb.SortColumn
	activeGridSort.Direction = crumb.SortDir
	activeGridSort.BaseQuery = crumb.Query
	activeGridSort.IsTableBrowse = isBrowse
	activeGridSort.OriginalResult = nil
	activeGridSort.mu.Unlock()

	fkNavMutex.Lock()
	pendingNavRestoreRow = crumb.CursorRow
	pendingNavRestoreCol = crumb.CursorCol
	fkNavMutex.Unlock()

	if activeSQLEditor != nil {
		activeSQLEditor.SetText(crumb.Query)
	}

	isEditingEnabled = crumb.EditingActive
	if crumb.EditingActive && activeGridDB != nil {
		_ = EnableCellEditing(app, table, activeGridDB, activeGridDBName, activeGridObject)
	}

	if activeGridDB != nil && crumb.Query != "" {
		ExecuteQueryAsync(app, activeGridDB, crumb.Query, table, func(res *QueryResult) {
			if app != nil {
				app.SetFocus(table)
			}
		})
	}
	return true
}

// triggerColumnSort cycles sorting (Unsorted -> ASC -> DESC -> Unsorted) for the specified column
// and re-executes the active query or performs a safe in-memory fallback sort for non-SELECT outputs.
func triggerColumnSort(app *tview.Application, table *tview.Table, col int) {
	if table == nil || col < 0 {
		return
	}
	if isQueryRunning.Load() {
		updateFooterText("A query is currently running. Cancel it before sorting.")
		return
	}
	headerCell := table.GetCell(0, col)
	colName := getHeaderColumnName(headerCell)
	if colName == "" {
		return
	}

	curRow, _ := table.GetSelection()
	activeGridSort.mu.Lock()
	activeGridSort.PendingRestoreRow = curRow
	activeGridSort.PendingRestoreCol = col
	activeGridSort.mu.Unlock()

	newCol, newDir := activeGridSort.Toggle(colName)
	_, _, baseQuery, isBrowse := activeGridSort.Snapshot()

	if isBrowse && activeGridDB != nil && activeGridDBName != "" && activeGridObject != "" {
		currentGridPage = 0
		sortedQuery := BuildBrowseQuery(activeGridDBName, activeGridObject, newCol, newDir, gridPageSize, 0)
		if activeSQLEditor != nil {
			activeSQLEditor.SetText(sortedQuery)
		}
		ExecuteQueryAsync(app, activeGridDB, sortedQuery, table, func(res *QueryResult) {
			if app != nil {
				app.SetFocus(table)
			}
		})
		return
	}

	if rewritten, ok := ApplySortToSQLQuery(baseQuery, newCol, newDir); ok && activeGridDB != nil {
		if activeSQLEditor != nil {
			activeSQLEditor.SetText(rewritten)
		}
		ExecuteQueryAsync(app, activeGridDB, rewritten, table, func(res *QueryResult) {
			if app != nil {
				app.SetFocus(table)
			}
		})
		return
	}

	// Fallback in-memory sort for non-SELECT results (e.g. SHOW, DESCRIBE, EXPLAIN) or offline test tables
	activeGridSort.mu.Lock()
	origRes := activeGridSort.OriginalResult
	activeGridSort.mu.Unlock()
	if origRes != nil {
		sortedRes := SortQueryResultInMemory(origRes, col, newDir)
		renderQueryResult(app, sortedRes, table)
	}
}

func setupTableInputCapture(app *tview.Application, table *tview.Table) {
	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if isQueryRunning.Load() {
			if event.Key() == tcell.KeyCtrlC || event.Key() == tcell.KeyEscape {
				CancelActiveQuery()
				return nil
			}
		}

		switch event.Key() {
		case tcell.KeyTab, tcell.KeyEscape:
			if tableList != nil {
				app.SetFocus(tableList)
			}
			return nil

		case tcell.KeyBacktab:
			if activeSQLEditor != nil && activeSQLEditor.Editor != nil {
				app.SetFocus(activeSQLEditor.Editor)
			}
			return nil

		case tcell.KeyCtrlB:
			toggleSidebarVisibility(app)
			return nil

		case tcell.KeyCtrlE:
			if activeGridDB != nil && activeGridDBName != "" {
				ShowExportWizardModal(app, activeGridDB, activeGridDBName)
			}
			return nil

		case tcell.KeyF11:
			app.SetRoot(table, true)
			return nil

		case tcell.KeyCtrlN:
			// Next Page (preserving active sort order)
			if activeGridObjectType == "TABLE" || activeGridObjectType == "VIEW" {
				currentGridPage++
				offset := currentGridPage * gridPageSize
				sortCol, sortDir, _, _ := activeGridSort.Snapshot()
				pageQuery := BuildBrowseQuery(activeGridDBName, activeGridObject, sortCol, sortDir, gridPageSize, offset)
				ExecuteQuery(app, activeGridDB, pageQuery, table)
			}
			return nil

		case tcell.KeyCtrlP:
			// Previous Page (preserving active sort order)
			if currentGridPage > 0 && (activeGridObjectType == "TABLE" || activeGridObjectType == "VIEW") {
				currentGridPage--
				offset := currentGridPage * gridPageSize
				sortCol, sortDir, _, _ := activeGridSort.Snapshot()
				pageQuery := BuildBrowseQuery(activeGridDBName, activeGridObject, sortCol, sortDir, gridPageSize, offset)
				ExecuteQuery(app, activeGridDB, pageQuery, table)
			}
			return nil
		}

		// Cycle sort on active column with 's' or 'S'
		if event.Key() == tcell.KeyRune && (event.Rune() == 's' || event.Rune() == 'S') {
			_, c := table.GetSelection()
			triggerColumnSort(app, table, c)
			return nil
		}

		// Follow Foreign Key relationship with 'f' or 'F'
		if event.Key() == tcell.KeyRune && (event.Rune() == 'f' || event.Rune() == 'F') {
			if !triggerForeignKeyJump(app, table) {
				r, c := table.GetSelection()
				if r <= 0 {
					updateFooterText("Select a non-NULL data cell to follow foreign key.")
				} else {
					colName := getHeaderColumnName(table.GetCell(0, c))
					updateFooterText(fmt.Sprintf("Column '%s' is not a foreign key.", colName))
				}
			}
			return nil
		}

		// Return from FK traversal with Backspace
		if event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
			if navigateBackFromFK(app, table) {
				return nil
			}
		}

		// Insert new row modal with '+' or 'a' / 'A'
		if event.Key() == tcell.KeyRune && (event.Rune() == '+' || event.Rune() == 'a' || event.Rune() == 'A') {
			if ActiveReadOnly {
				showErrorModal(app, mainFlex, "Action Blocked: Active Connection is in READ-ONLY Mode. Row insertion is disabled.")
				return nil
			}
			if activeGridDB == nil || activeGridDBName == "" || activeGridObject == "" || activeGridObjectType != "TABLE" {
				updateFooterText("Select a base TABLE in the sidebar before inserting a new row.")
				return nil
			}
			ShowInsertRowModal(app, activeGridDB, activeGridDBName, activeGridObject, table)
			return nil
		}

		// Delete highlighted row with 'd', 'D', or Delete key
		if event.Key() == tcell.KeyDelete || (event.Key() == tcell.KeyRune && (event.Rune() == 'd' || event.Rune() == 'D')) {
			if ActiveReadOnly {
				showErrorModal(app, mainFlex, "Action Blocked: Active Connection is in READ-ONLY Mode. Row deletion is disabled.")
				return nil
			}
			if activeGridDB == nil || activeGridDBName == "" || activeGridObject == "" || activeGridObjectType != "TABLE" {
				updateFooterText("Select a base TABLE in the sidebar before deleting a row.")
				return nil
			}
			r, _ := table.GetSelection()
			TriggerRowDeleteFlow(app, activeGridDB, activeGridDBName, activeGridObject, table, r)
			return nil
		}

		// Quick Copy Row menu (Ctrl+Shift+C, 'c'/'C', or 'y'/'Y')
		isCtrlShiftC := event.Key() == tcell.KeyCtrlC && (event.Modifiers()&tcell.ModShift != 0)
		isCopyRowKey := event.Key() == tcell.KeyRune && (event.Rune() == 'c' || event.Rune() == 'C' || event.Rune() == 'y' || event.Rune() == 'Y')
		if isCtrlShiftC || isCopyRowKey {
			r, _ := table.GetSelection()
			if r <= 0 || r >= table.GetRowCount() {
				updateFooterText("Select a data row to copy.")
				return nil
			}
			ShowCopyRowMenuModal(app, table, activeGridDBName, activeGridObject, r)
			return nil
		}

		// Inspect cell with 'v' or 'V'
		if event.Key() == tcell.KeyRune && (event.Rune() == 'v' || event.Rune() == 'V') {
			r, c := table.GetSelection()
			cell := table.GetCell(r, c)
			if cell != nil {
				colName := getHeaderColumnName(table.GetCell(0, c))
				gc := GetGridCellFromTableCell(cell, colName)
				inspectText := gc.Value()
				if gc.IsNull() {
					inspectText = "(NULL)"
				}
				ShowCellInspectorModal(app, inspectText)
				return nil
			}
		}

		// Jump straight to SQL Editor: 'e', 'E', 'i', 'I'
		if event.Key() == tcell.KeyRune && (event.Rune() == 'e' || event.Rune() == 'E' || event.Rune() == 'i' || event.Rune() == 'I') {
			if activeSQLEditor != nil && activeSQLEditor.Editor != nil {
				app.SetFocus(activeSQLEditor.Editor)
				return nil
			}
		}

		// Ctrl+J on an FK cell follows FK; otherwise Ctrl+Enter / Ctrl+J commits active transaction if pending
		if event.Key() == tcell.KeyCtrlJ {
			if triggerForeignKeyJump(app, table) {
				return nil
			}
		}
		if (event.Key() == tcell.KeyEnter && (event.Modifiers()&tcell.ModCtrl != 0)) || event.Key() == tcell.KeyCtrlJ {
			if GetUncommittedTxCount() > 0 {
				if count, err := CommitActiveTx(app); err == nil && count > 0 {
					updateFooterText(fmt.Sprintf("Committed %d mutation(s) to database.", count))
				}
				return nil
			}
		}

		// Allow all other keys (F1-F12, Ctrl+R, Ctrl+B, Ctrl+E, Ctrl+H, Ctrl+Q, ':', arrows, Enter) to pass through
		return event
	})
}

// Enable editing and database update with support for composite keys and keyless tables
func EnableCellEditing(app *tview.Application, table *tview.Table, db *sql.DB, dbName, tableName string) error {
	table.SetSelectable(true, true)
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.ColorWhite).
		Foreground(tcell.ColorBlack))

	table.SetSelectedFunc(func(row int, column int) {
		if row == 0 {
			triggerColumnSort(app, table, column)
			return
		}
		if !isEditingEnabled {
			return
		}

		if ActiveReadOnly {
			showErrorModal(app, mainFlex, "Action Blocked: Active Connection is in READ-ONLY Mode. Inline cell editing is disabled.")
			return
		}

		cell := table.GetCell(row, column)
		columnName := getHeaderColumnName(table.GetCell(0, column))
		gridCell := GetGridCellFromTableCell(cell, columnName)

		isNULL := gridCell.IsNull()
		cachedText := ""
		if !isNULL {
			cachedText = gridCell.Value()
		}

		statusView := tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignLeft)

		updateStatusText := func() {
			if isNULL {
				statusView.SetText(" Mode: [yellow::b][X] SQL NULL (Will save as NULL)[-::-]  │  [aqua]Ctrl+N[-]: Value Mode  │  [aqua]Ctrl+E[-]: Empty \"\"  │  [lime]Enter[-]: Save  │  [white]Esc[-]: Cancel")
			} else if cachedText == "" {
				statusView.SetText(" Mode: [cyan::b][ ] Empty String (\"\")[-::-]  │  [aqua]Ctrl+N[-]: Set to NULL  │  [lime]Enter[-]: Save  │  [white]Esc[-]: Cancel")
			} else {
				statusView.SetText(" Mode: [lime::b][ ] Literal Value[-::-]  │  [aqua]Ctrl+N[-]: Set to NULL  │  [aqua]Ctrl+E[-]: Empty \"\"  │  [lime]Enter[-]: Save  │  [white]Esc[-]: Cancel")
			}
		}
		updateStatusText()

		textArea := tview.NewTextArea()
		textArea.SetBorder(true)
		StyleBox(textArea.Box, fmt.Sprintf("Edit Cell: %s.%s", tableName, columnName), true)
		if isNULL {
			textArea.SetPlaceholder("SQL NULL (Press Ctrl+N or start typing to enter literal value)")
			textArea.SetText("", false)
		} else {
			textArea.SetText(cachedText, true)
		}

		textArea.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			switch event.Key() {
			case tcell.KeyCtrlN:
				isNULL = !isNULL
				if isNULL {
					cachedText = textArea.GetText()
					textArea.SetText("", false)
					textArea.SetPlaceholder("SQL NULL (Press Ctrl+N or start typing to enter literal value)")
				} else {
					textArea.SetText(cachedText, true)
				}
				updateStatusText()
				return nil

			case tcell.KeyCtrlE:
				isNULL = false
				cachedText = ""
				textArea.SetText("", false)
				updateStatusText()
				return nil

			case tcell.KeyEnter:
				if !isEditingEnabled {
					modal := tview.NewModal().
						SetText("Not allowed to update in Run Query mode").
						AddButtons([]string{"OK"}).
						SetDoneFunc(func(buttonIndex int, buttonLabel string) {
							layout := CreateLayoutWithFooter(app, mainFlex)
							app.SetRoot(layout, true)
							util.SetFocusWithBorder(app, table)
						})
					app.SetRoot(modal, false)
					return nil
				}

				var valToSave *string
				if !isNULL {
					text := textArea.GetText()
					valToSave = &text
				}

				var updateQuery string
				var queryArgs []interface{}
				if valToSave == nil {
					queryArgs = append(queryArgs, nil)
				} else {
					queryArgs = append(queryArgs, *valToSave)
				}

				pkCols, _ := GetPrimaryKeyColumns(db, dbName, tableName)
				whereSQL, whereArgs, whereErr := BuildRowWhereClause(table, row, pkCols)
				if whereErr != nil {
					showErrorModal(app, mainFlex, "Cannot update row: "+whereErr.Error())
					return nil
				}
				queryArgs = append(queryArgs, whereArgs...)

				targetTable := util.QuoteIdentifier(tableName)
				if dbName != "" {
					targetTable = fmt.Sprintf("%s.%s", util.QuoteIdentifier(dbName), util.QuoteIdentifier(tableName))
				}
				updateQuery = fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s LIMIT 1",
					targetTable,
					util.QuoteIdentifier(columnName),
					whereSQL)

				ctxExec, cancelExec := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancelExec()

				if !IsAutoCommitMode() {
					tx, txErr := EnsureActiveTx(ctxExec, db)
					if txErr != nil {
						util.SaveLog("Tx error: " + txErr.Error())
						showErrorModal(app, mainFlex, "Transaction failed: "+txErr.Error())
						return nil
					}
					_, execErr := tx.ExecContext(ctxExec, updateQuery, queryArgs...)
					if execErr != nil {
						util.SaveLog("Update error: " + execErr.Error())
						showErrorModal(app, mainFlex, "Update failed: "+execErr.Error())
						return nil
					}
					RecordMutationInTx(updateQuery, "Grid Cell Edit")
				} else {
					_, execErr := db.ExecContext(ctxExec, updateQuery, queryArgs...)
					if execErr != nil {
						util.SaveLog("Update error: " + execErr.Error())
						showErrorModal(app, mainFlex, "Update failed: "+execErr.Error())
						return nil
					}
				}

				gridCell.RawValue = valToSave
				cell.SetText(" " + gridCell.DisplayText() + " ")
				cell.SetReference(gridCell)

				fullQuery := phhistory.ReplacePlaceholders(updateQuery, queryArgs...)
				phhistory.SaveQuery(fullQuery, dbName)
				util.SaveLog(fullQuery)

				layout := CreateLayoutWithFooter(app, mainFlex)
				app.SetRoot(layout, true)
				util.SetFocusWithBorder(app, table)
				updateFooterText(fmt.Sprintf("Cell updated successfully in %s.%s.", tableName, columnName))
				return nil

			case tcell.KeyEscape:
				layout := CreateLayoutWithFooter(app, mainFlex)
				app.SetRoot(layout, true)
				util.SetFocusWithBorder(app, table)
				return nil

			default:
				if event.Key() == tcell.KeyRune && isNULL {
					isNULL = false
					updateStatusText()
				}
			}
			return event
		})

		modal := tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(statusView, 1, 0, false).
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
					colHeader := util.StripFormatting(table.GetCell(0, c).Text)
					gc := GetGridCellFromTableCell(table.GetCell(r, c), colHeader)
					val := gc.ExportValue("")
					rowVals = append(rowVals, fmt.Sprintf("%q", val))
				}
				lines = append(lines, strings.Join(rowVals, ","))
			}
			f.WriteString(strings.Join(lines, "\n") + "\n")
		} else {
			var jsonRows []map[string]interface{}
			for r := 1; r < rows; r++ {
				rowMap := make(map[string]interface{})
				for c := 0; c < cols; c++ {
					header := util.StripFormatting(table.GetCell(0, c).Text)
					gc := GetGridCellFromTableCell(table.GetCell(r, c), header)
					if gc.IsNull() {
						rowMap[header] = nil
					} else {
						rowMap[header] = gc.Value()
					}
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
				layout := CreateLayoutWithFooter(app, mainFlex)
				app.SetRoot(layout, true)
				util.SetFocusWithBorder(app, table)
			})
		app.SetRoot(modal, true)
	})

	form.AddButton("Cancel", func() {
		layout := CreateLayoutWithFooter(app, mainFlex)
		app.SetRoot(layout, true)
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
	StyleModalBox(colTable.Box, fmt.Sprintf("Columns of `%s`", tableName))
	StyleTable(colTable)

	colQuery := fmt.Sprintf("SHOW COLUMNS FROM %s.%s", util.QuoteIdentifier(dbName), util.QuoteIdentifier(tableName))
	rows, err := db.Query(colQuery)
	if err != nil {
		showErrorModal(app, mainFlex, "Failed to inspect columns: "+err.Error())
		return
	}
	defer rows.Close()

	headers := []string{"Field", "Type", "Null", "Key", "Default", "Extra"}
	for i, h := range headers {
		colTable.SetCell(0, i, tview.NewTableCell(" "+h+" ").
			SetTextColor(CurrentTheme.HeaderFg).
			SetBackgroundColor(CurrentTheme.HeaderBg).
			SetAlign(tview.AlignCenter))
	}

	rIdx := 1
	for rows.Next() {
		var field, typ, nullStr, keyStr, extraStr sql.NullString
		var defStr sql.NullString
		if err := rows.Scan(&field, &typ, &nullStr, &keyStr, &defStr, &extraStr); err == nil {
			colTable.SetCell(rIdx, 0, tview.NewTableCell(" "+field.String+" ").SetTextColor(CurrentTheme.AccentSuccess))
			colTable.SetCell(rIdx, 1, tview.NewTableCell(" "+typ.String+" ").SetTextColor(CurrentTheme.TextPrimary))
			colTable.SetCell(rIdx, 2, tview.NewTableCell(" "+nullStr.String+" ").SetTextColor(CurrentTheme.TextSecondary))
			colTable.SetCell(rIdx, 3, tview.NewTableCell(" "+keyStr.String+" ").SetTextColor(CurrentTheme.AccentPrimary))
			colTable.SetCell(rIdx, 4, tview.NewTableCell(" "+defStr.String+" ").SetTextColor(CurrentTheme.TextSecondary))
			colTable.SetCell(rIdx, 5, tview.NewTableCell(" "+extraStr.String+" ").SetTextColor(CurrentTheme.TextSecondary))
			rIdx++
		}
	}

	idxTable := tview.NewTable().SetBorders(true)
	StyleModalBox(idxTable.Box, fmt.Sprintf("Indexes of `%s`", tableName))
	StyleTable(idxTable)

	idxQuery := fmt.Sprintf("SHOW INDEX FROM %s.%s", util.QuoteIdentifier(dbName), util.QuoteIdentifier(tableName))
	idxRows, err := db.Query(idxQuery)
	if err == nil {
		defer idxRows.Close()
		idxHeaders := []string{"Key_name", "Column_name", "Non_unique", "Index_type"}
		for i, h := range idxHeaders {
			idxTable.SetCell(0, i, tview.NewTableCell(" "+h+" ").
				SetTextColor(CurrentTheme.HeaderFg).
				SetBackgroundColor(CurrentTheme.HeaderBg).
				SetAlign(tview.AlignCenter))
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

				idxTable.SetCell(irIdx, 0, tview.NewTableCell(" "+kName+" ").SetTextColor(CurrentTheme.AccentPrimary))
				idxTable.SetCell(irIdx, 1, tview.NewTableCell(" "+cName+" ").SetTextColor(CurrentTheme.AccentSuccess))
				idxTable.SetCell(irIdx, 2, tview.NewTableCell(" "+nuName+" ").SetTextColor(CurrentTheme.TextPrimary))
				idxTable.SetCell(irIdx, 3, tview.NewTableCell(" "+iType+" ").SetTextColor(CurrentTheme.TextSecondary))
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
			fullL := CreateLayoutWithFooter(app, mainFlex)
			app.SetRoot(fullL, true)
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
	ShowComprehensiveHelpModal(app)
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
	StyleTable(table)
	StyleModalBox(table.Box, "EXPLAIN Execution Plan & Performance Analyzer")
	table.SetBackgroundColor(CurrentTheme.SurfaceBg)

	headerStyle := tcell.StyleDefault.
		Foreground(CurrentTheme.HeaderFg).
		Background(CurrentTheme.HeaderBg).
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
		warnings = append(warnings, "[red::b][CRITICAL] FULL TABLE SCAN (type=ALL)[-]")
	}
	if hasFilesort {
		healthScore -= 20
		warnings = append(warnings, "[yellow::b][WARNING] FILESORT[-] ")
	}
	if hasTempTable {
		healthScore -= 20
		warnings = append(warnings, "[yellow::b][WARNING] TEMPORARY TABLE[-] ")
	}
	if healthScore < 0 {
		healthScore = 0
	}

	scoreBadge := fmt.Sprintf("[lime::b]OPTIMAL (%d%%)[-]", healthScore)
	bgColor := tcell.ColorDarkGreen
	if healthScore <= 50 {
		scoreBadge = fmt.Sprintf("[red::b]CRITICAL BOTTLENECK (%d%%)[-]", healthScore)
		bgColor = tcell.ColorDarkRed
	} else if healthScore <= 80 {
		scoreBadge = fmt.Sprintf("[yellow::b]SUBOPTIMAL (%d%%)[-]", healthScore)
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
			analyzeView.SetBackgroundColor(CurrentTheme.SurfaceBg)
			StyleModalBox(analyzeView.Box, "EXPLAIN ANALYZE (Execution Times & Costs)")

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
			jsonView.SetBackgroundColor(CurrentTheme.SurfaceBg)
			StyleModalBox(jsonView.Box, "EXPLAIN FORMAT=JSON Output")

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
	sb.WriteString("[aqua::b]PHERI PERFORMANCE & INDEX ADVISOR[-::-]\n\n")

	if !fullScan && !filesort && !tempTable {
		sb.WriteString("[lime::b]Query execution plan is optimal.[-::-]\n")
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
		sb.WriteString("1. [red::b]CRITICAL: Full Table Scan Detected (type=ALL)[-::-]\n")
		sb.WriteString("   Scanned ~" + formatSize(totalRows) + " rows without using an index.\n")
		sb.WriteString("   [lime::b]Recommended Index Fix:[-::-]\n")
		sb.WriteString(fmt.Sprintf("   [cyan::b]CREATE INDEX idx_%s_filter ON %s (id);[-::-]\n\n", tblName, tblName))
	}

	if filesort {
		sb.WriteString("2. [yellow::b]WARNING: Using Filesort[-::-]\n")
		sb.WriteString("   MySQL is sorting rows in memory/disk instead of using an ordered index.\n")
		sb.WriteString("   [lime::b]Recommended Index Fix:[-::-]\n")
		sb.WriteString(fmt.Sprintf("   [cyan::b]CREATE INDEX idx_%s_sort ON %s (created_at DESC);[-::-]\n\n", tblName, tblName))
	}

	if tempTable {
		sb.WriteString("3. [yellow::b]WARNING: Using Temporary Table[-::-]\n")
		sb.WriteString("   Intermediate result sets are being written to temporary disk tables.\n")
	}

	return sb.String()
}

func isInputFocused(app *tview.Application) bool {
	if app == nil {
		return false
	}
	f := app.GetFocus()
	if f == nil {
		return false
	}
	switch f.(type) {
	case *tview.InputField, *tview.TextArea:
		return true
	}
	return false
}

func setupGlobalInputCapture(app *tview.Application, db *sql.DB, dbName string) {
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// 1. If an async query is actively executing, Ctrl+C / Esc aborts it
		if isQueryRunning.Load() {
			if event.Key() == tcell.KeyCtrlC || event.Key() == tcell.KeyEscape {
				CancelActiveQuery()
				return nil
			}
		}

		// 2. Ctrl+C when not running query (let Ctrl+Shift+C pass through for Quick Copy Row menu)
		if event.Key() == tcell.KeyCtrlC {
			if event.Modifiers()&tcell.ModShift != 0 {
				return event
			}
			if activeSQLEditor != nil && activeSQLEditor.Editor != nil && activeSQLEditor.Editor.HasFocus() {
				return event
			}
			updateFooterText("Press Ctrl+Q or type :quit to exit Pheri")
			return nil
		}

		// 3. Never hijack Backspace or Delete in global capture so editing always works in all inputs and editors
		if event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 || event.Key() == tcell.KeyDelete {
			return event
		}

		// 4. Ctrl+Q - Clean exit with transaction protection
		if event.Key() == tcell.KeyCtrlQ {
			if GetUncommittedTxCount() > 0 {
				PromptUncommittedChangesModal(app, "exit Pheri",
					func() {
						_, _ = CommitActiveTx(app)
						app.Stop()
					},
					func() {
						_, _ = RollbackActiveTx(app)
						app.Stop()
					},
					func() {
						updateFooterText("Exit cancelled.")
					},
				)
				return nil
			}
			app.Stop()
			return nil
		}

		// 5. Help (F1 / '?')
		if event.Key() == tcell.KeyF1 {
			ShowComprehensiveHelpModal(app)
			return nil
		}
		if event.Key() == tcell.KeyRune && event.Rune() == '?' {
			if !isInputFocused(app) {
				ShowComprehensiveHelpModal(app)
				return nil
			}
		}

		// 6. Auto-Commit Toggle (F2 / :autocommit)
		if event.Key() == tcell.KeyF2 {
			ToggleAutoCommit(app, activeGridDB)
			return nil
		}

		// 7. Schema Inspector (F3)
		if event.Key() == tcell.KeyF3 {
			if activeGridObject != "" && activeGridDBName != "" {
				showSchemaInspectorModal(app, activeGridDB, activeGridDBName, activeGridObject)
			} else {
				ShowComprehensiveHelpModal(app)
			}
			return nil
		}

		// 8. Live Process Manager (F4)
		if event.Key() == tcell.KeyF4 {
			if activeGridDB != nil {
				ShowProcessListModal(app, activeGridDB)
			}
			return nil
		}

		// 9. Run SQL Query (F5 / Ctrl+R)
		if event.Key() == tcell.KeyF5 || event.Key() == tcell.KeyCtrlR {
			if activeSQLEditor != nil {
				activeSQLEditor.OnExecute(activeSQLEditor.GetText())
				return nil
			}
		}

		// 10. Real-Time Health Dashboard (F6)
		if event.Key() == tcell.KeyF6 {
			if activeGridDB != nil {
				showHealthDashboardModal(app, activeGridDB)
			}
			return nil
		}

		// 11. Schema Diff & Migration Generator (F7)
		if event.Key() == tcell.KeyF7 {
			if activeGridDB != nil && activeGridDBName != "" {
				showSchemaDiffModal(app, activeGridDB, activeGridDBName)
			}
			return nil
		}

		// 12. High-Speed Parallel Worker Export (Ctrl+W or F8)
		if event.Key() == tcell.KeyCtrlW || event.Key() == tcell.KeyF8 {
			if activeGridDB != nil && activeGridDBName != "" {
				ShowParallelWorkerExportModal(app, activeGridDB, activeGridDBName)
			}
			return nil
		}

		// 13. Database Import Wizard (F9)
		if event.Key() == tcell.KeyF9 {
			if activeGridDB != nil && activeGridDBName != "" {
				ShowImportWizardModal(app, activeGridDB, activeGridDBName)
			}
			return nil
		}

		// 14. Explicit Transaction Commit (F10 / :commit)
		if event.Key() == tcell.KeyF10 && (event.Modifiers()&tcell.ModShift == 0) {
			if count, err := CommitActiveTx(app); err == nil && count > 0 {
				updateFooterText(fmt.Sprintf("Committed %d mutation(s) to database.", count))
			}
			return nil
		}

		// 15. Explicit Transaction Rollback (Shift+F10 / F22 / Alt+Backspace)
		if (event.Key() == tcell.KeyF10 && (event.Modifiers()&tcell.ModShift != 0)) ||
			event.Key() == tcell.KeyF22 ||
			(event.Key() == tcell.KeyBackspace && (event.Modifiers()&tcell.ModAlt != 0)) {
			if count, err := RollbackActiveTx(app); err == nil && count > 0 {
				updateFooterText(fmt.Sprintf("Rolled back %d mutation(s).", count))
				if activeGridDB != nil && activeGridQuery != "" && dataTable != nil {
					ExecuteQueryAsync(app, activeGridDB, activeGridQuery, dataTable, nil)
				}
			}
			return nil
		}

		// 16. Searchable Query History (Ctrl+H / :history)
		if event.Key() == tcell.KeyCtrlH {
			showQueryHistoryModal(app, activeGridDB, activeSQLEditor)
			return nil
		}

		// 17. Export Wizard (Ctrl+E)
		if event.Key() == tcell.KeyCtrlE {
			if activeGridDB != nil && activeGridDBName != "" {
				ShowExportWizardModal(app, activeGridDB, activeGridDBName)
			}
			return nil
		}

		// 18. Toggle Sidebar Collapse (Ctrl+B)
		if event.Key() == tcell.KeyCtrlB {
			toggleSidebarVisibility(app)
			return nil
		}

		// 19. Terminal Color Theme Picker (F12)
		if event.Key() == tcell.KeyF12 {
			showThemePickerModal(app)
			return nil
		}

		// 20. Full DB Dump (Ctrl+Y)
		if event.Key() == tcell.KeyCtrlY {
			startFullDatabaseExport(app, db, dbName)
			return nil
		}

		// 21. Table DDL + Data Export (Ctrl+X)
		if event.Key() == tcell.KeyCtrlX {
			if activeGridObject != "" && activeGridObjectType == "TABLE" {
				exportTableDDLAndDataAsync(app, activeGridDB, activeGridDBName, activeGridObject)
			}
			return nil
		}

		// 22. Command Palette (':') when not typing in text input/editor
		if event.Key() == tcell.KeyRune && event.Rune() == ':' {
			if !isInputFocused(app) {
				ShowCommandPaletteModal(app)
				return nil
			}
		}

		// 23. Quick Jump to SQL Editor ('e' / 'E') when not typing in text input/editor
		if event.Key() == tcell.KeyRune && (event.Rune() == 'e' || event.Rune() == 'E') {
			if !isInputFocused(app) {
				if activeSQLEditor != nil && activeSQLEditor.Editor != nil {
					app.SetFocus(activeSQLEditor.Editor)
					return nil
				}
			}
		}

		return event
	})
}

func startFullDatabaseExport(app *tview.Application, db *sql.DB, dbName string) {
	go func() {
		progressChan := make(chan string)
		progressView := tview.NewTextView().
			SetDynamicColors(true).
			SetScrollable(true)
		StyleBox(progressView.Box, "Export Progress", true)

		// ---- STATUS BAR ----
		statusBar := tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter)
		statusBar.SetBorder(true).SetTitle("Status")

		// ---- WRAPPER LAYOUT ----
		layout := tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(tview.NewTextView().
				SetText("[::b]  Exporting Database  ").
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
						orig := CreateLayoutWithFooter(app, mainFlex)
						app.SetRoot(orig, true)
					})

				app.SetRoot(modal, true)
			})
		}()
	}()
}

func exportTableDDLAndDataAsync(app *tview.Application, db *sql.DB, dbName, tableName string) {
	if db == nil || dbName == "" || tableName == "" {
		return
	}
	updateFooterText(fmt.Sprintf("Exporting %s DDL and data to clipboard...", tableName))
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		query := fmt.Sprintf("SHOW CREATE TABLE %s.%s", util.QuoteIdentifier(dbName), util.QuoteIdentifier(tableName))
		row := db.QueryRowContext(ctx, query)
		var tblName, createStatement string
		if err := row.Scan(&tblName, &createStatement); err != nil {
			app.QueueUpdateDraw(func() {
				updateFooterText("Failed to fetch table definition: " + err.Error())
			})
			return
		}

		dataQuery := fmt.Sprintf("SELECT * FROM %s.%s LIMIT 1000", util.QuoteIdentifier(dbName), util.QuoteIdentifier(tableName))
		rows, err := db.QueryContext(ctx, dataQuery)
		if err != nil {
			_ = clipboard.WriteAll(createStatement + ";\n")
			app.QueueUpdateDraw(func() {
				updateFooterText(fmt.Sprintf("Table DDL for %s copied to clipboard.", tableName))
			})
			return
		}
		defer rows.Close()

		cols, err := rows.Columns()
		if err != nil {
			_ = clipboard.WriteAll(createStatement + ";\n")
			app.QueueUpdateDraw(func() {
				updateFooterText(fmt.Sprintf("Table DDL for %s copied to clipboard.", tableName))
			})
			return
		}

		var insertStatements []string
		for rows.Next() {
			values := make([]interface{}, len(cols))
			pointers := make([]interface{}, len(cols))
			for i := range values {
				pointers[i] = &values[i]
			}
			if err := rows.Scan(pointers...); err != nil {
				continue
			}
			var valuesList []string
			for _, val := range values {
				if val != nil {
					switch v := val.(type) {
					case []byte:
						valuesList = append(valuesList, fmt.Sprintf("'%s'", strings.ReplaceAll(string(v), "'", "''")))
					default:
						valuesList = append(valuesList, fmt.Sprintf("'%v'", strings.ReplaceAll(fmt.Sprintf("%v", v), "'", "''")))
					}
				} else {
					valuesList = append(valuesList, "NULL")
				}
			}
			insertStatements = append(insertStatements, fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);",
				util.QuoteIdentifier(tableName), strings.Join(cols, ", "), strings.Join(valuesList, ", ")))
		}

		combined := createStatement + ";\n\n" + strings.Join(insertStatements, "\n")
		_ = clipboard.WriteAll(combined)
		app.QueueUpdateDraw(func() {
			updateFooterText(fmt.Sprintf("Copied %s DDL + %d row(s) to clipboard!", tableName, len(insertStatements)))
		})
	}()
}
