package phhistory

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

// SQLite driver

var db *sql.DB

var user, host, port string

func SetUser(u string) {
	user = u
}

func SetHost(h string) {
	host = h
}

func SetPort(p string) {
	port = p
}

// InitPhHistory initializes the database for query logging
func InitPhHistory(dbPath string, userLocal, hostLocal, portLocal string) error {
	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open history database: %w", err)
	}
	user = userLocal
	host = hostLocal
	port = portLocal

	// Create table if it does not exist
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS pheri_phhistory (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            query_text TEXT NOT NULL,
            host_ip VARCHAR(15) NOT NULL,    
            db_name VARCHAR(100) NOT NULL,     
			user VARCHAR(100) NOT NULL,  
			port VARCHAR(10) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );
    `)
	if err != nil {
		return fmt.Errorf("failed to create history table: %w", err)
	}

	return nil
}

// SaveQuery saves a new executed query into the history table
// func SaveQuery(query string) error {
// 	if db == nil {
// 		return fmt.Errorf("database not initialized. Call InitPhHistory first.")
// 	}

// 	_, err := db.Exec(`INSERT INTO pheri_phhistory (query_text) VALUES (?)`, query)
// 	if err != nil {
// 		return fmt.Errorf("failed to save query: %w", err)
// 	}
// 	return nil
// }

// SaveQuery saves a query along with host IP and database name
func SaveQuery(query, dbName string) error {
	if db == nil {
		return fmt.Errorf("database not initialized, call InitPhHistory first")
	}

	_, err := db.Exec(`
		INSERT INTO pheri_phhistory (query_text, host_ip, db_name, user, port)
		VALUES (?, ?, ?, ?, ?)
	`, query, host, dbName, user, port)
	if err != nil {
		return fmt.Errorf("failed to save query: %w", err)
	}
	return nil
}

// Close closes the database connection (call this on app shutdown)
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

func ReplacePlaceholders(query string, args ...interface{}) string {
	var replacedQuery string
	argIndex := 0

	// Iterate over each character in the query string
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			// Check if we still have arguments to replace
			if argIndex < len(args) {
				arg := args[argIndex]
				// Replace the ? with the appropriate value
				switch v := arg.(type) {
				case string:
					replacedQuery += "'" + v + "'"
				case int, int64:
					replacedQuery += fmt.Sprintf("%v", v)
				case float64:
					replacedQuery += fmt.Sprintf("%v", v)
				case bool:
					replacedQuery += fmt.Sprintf("%v", v)
				default:
					replacedQuery += fmt.Sprintf("%v", v)
				}
				argIndex++
			}
		} else {
			// Append other characters to the final query string
			replacedQuery += string(query[i])
		}
	}

	return replacedQuery
}

func FetchHistory(days, months, years int, file string) error {

	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	var condition string
	var args []interface{}

	now := time.Now()
	switch {
	case days > 0:
		from := now.AddDate(0, 0, -days)
		condition = "created_at >= ?"
		args = append(args, from.Format("2006-01-02"))
	case months > 0:
		from := now.AddDate(0, -months, 0)
		condition = "created_at >= ?"
		args = append(args, from.Format("2006-01-02"))
	case years > 0:
		from := now.AddDate(-years, 0, 0)
		condition = "created_at >= ?"
		args = append(args, from.Format("2006-01-02"))
	default:
		return fmt.Errorf("please provide -days, -month, or -year")
	}

	query := fmt.Sprintf(`
		SELECT id, query_text, host_ip, db_name, user, port, created_at
		FROM pheri_phhistory
		WHERE %s
		ORDER BY created_at DESC
	`, condition)

	rows, err := db.Query(query, args...)
	if err != nil {
		fmt.Println("Error executing query:", err)
		return fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	output := ""
	for rows.Next() {
		var id int
		var queryText, hostIP, dbName, user, port string
		var createdAt time.Time

		if err := rows.Scan(&id, &queryText, &hostIP, &dbName, &user, &port, &createdAt); err != nil {
			return err
		}
		output += fmt.Sprintf("ID: %d\nQuery: %s\nHost: %s\nDB: %s\nUser: %s\nPort: %s\nDate: %s\n\n",
			id, queryText, hostIP, dbName, user, port, createdAt.Format(time.RFC3339))
	}

	if file != "" {
		return os.WriteFile(file, []byte(output), 0644)
	} else {
		fmt.Println(output)
	}
	return nil
}
