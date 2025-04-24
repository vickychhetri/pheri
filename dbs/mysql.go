// db/mysql.go
package dbs

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
)

func Connect(user, pass, host, port string) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/", user, pass, host, port)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	return db, db.Ping()
}
