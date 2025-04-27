package phhistory

import (
	"database/sql"
	"fmt"

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
