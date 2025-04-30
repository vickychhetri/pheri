package util

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func GetFullReturnType(db *sql.DB, objName string, dbName string) (string, error) {
	// First, find out whether it's a FUNCTION or PROCEDURE
	var routineType string
	typeQuery := `
		SELECT routine_type
		FROM information_schema.routines
		WHERE routine_name = ? 
		  AND routine_schema = ?
	`
	err := db.QueryRow(typeQuery, objName, dbName).Scan(&routineType)
	if err != nil {
		return "", fmt.Errorf("error fetching routine type: %v", err)
	}

	var query string
	if routineType == "FUNCTION" {
		// For functions, get return type from routines
		query = `
			SELECT data_type,
			       character_maximum_length,
			       numeric_precision,
			       numeric_scale
			FROM information_schema.routines
			WHERE routine_name = ? 
			  AND routine_schema = ?
		`
	} else if routineType == "PROCEDURE" {
		// For procedures, get OUT parameters
		query = `
			SELECT data_type,
			       character_maximum_length,
			       numeric_precision,
			       numeric_scale
			FROM information_schema.parameters
			WHERE specific_name = ? 
			  AND specific_schema = ?
			  AND parameter_mode = 'OUT'
			ORDER BY ordinal_position
			LIMIT 1
		`
	} else {
		return "", fmt.Errorf("unsupported routine type: %s", routineType)
	}

	rows, err := db.Query(query, objName, dbName)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var dataType, fullType string
	var charLength sql.NullInt32
	var numPrecision, numScale sql.NullInt32

	if rows.Next() {
		err := rows.Scan(&dataType, &charLength, &numPrecision, &numScale)
		if err != nil {
			return "", err
		}

		// Build the full type
		switch dataType {
		case "varchar", "char":
			if charLength.Valid {
				fullType = fmt.Sprintf("%s(%d)", dataType, charLength.Int32)
			} else {
				fullType = dataType
			}
		case "decimal", "numeric":
			if numPrecision.Valid && numScale.Valid {
				fullType = fmt.Sprintf("%s(%d,%d)", dataType, numPrecision.Int32, numScale.Int32)
			} else {
				fullType = dataType
			}
		default:
			fullType = dataType
		}
	} else {
		return "", fmt.Errorf("no matching return type found")
	}

	return fullType, nil
}

// ANSI color wrapper (tview uses [color]...[-] for inline color)
func HighlightSQLWithANSI(sql string) string {
	sql = regexp.MustCompile(`--.*`).ReplaceAllStringFunc(sql, func(s string) string {
		return `[yellow]` + s + `[-]`
	})

	sql = regexp.MustCompile(`/\*[\s\S]*?\*/`).ReplaceAllStringFunc(sql, func(s string) string {
		return `[yellow]` + s + `[-]`
	})

	sql = regexp.MustCompile(`'[^']*'`).ReplaceAllStringFunc(sql, func(s string) string {
		return `[green]` + s + `[-]`
	})

	keywords := []string{
		"SELECT", "FROM", "WHERE", "INSERT", "UPDATE", "DELETE",
		"JOIN", "AND", "OR", "NOT", "NULL", "IN", "IS", "LIKE",
		"CREATE", "TABLE", "VALUES", "SET", "ORDER", "BY", "GROUP", "HAVING",
	}

	for _, kw := range keywords {
		re := regexp.MustCompile(`(?i)\b` + kw + `\b`)
		sql = re.ReplaceAllStringFunc(sql, func(s string) string {
			return `[cyan]` + s + `[-]`
		})
	}

	return sql
}

// func GetFullReturnType(db *sql.DB, objName string, dbName string) (string, error) {
// 	// Query to fetch the data type and size/precision for the return type
// 	query := `
// 		SELECT c.data_type,
//                c.character_maximum_length,
//                c.numeric_precision,
//                c.numeric_scale
//         FROM information_schema.columns c
//         JOIN information_schema.routines r ON c.table_name = r.routine_name
//         WHERE r.specific_name = ? AND r.routine_schema = ?`
// 	SaveLog(query)
// 	SaveLog(objName)
// 	SaveLog(dbName)

// 	rows, err := db.Query(query, objName, dbName)
// 	if err != nil {
// 		return "", err
// 	}
// 	defer rows.Close()

// 	var dataType, fullType string
// 	var charLength sql.NullInt32
// 	var numPrecision, numScale sql.NullInt32

// 	if rows.Next() {
// 		err := rows.Scan(&dataType, &charLength, &numPrecision, &numScale)
// 		if err != nil {
// 			return "", err
// 		}

// 		// Handle different types based on data_type
// 		switch dataType {
// 		case "varchar", "char":
// 			// For varchar/char, use the character_maximum_length
// 			if charLength.Valid {
// 				fullType = fmt.Sprintf("%s(%d)", dataType, charLength.Int32)
// 			} else {
// 				fullType = dataType // If no length, just return the type
// 			}

// 		case "decimal", "numeric":
// 			// For decimal/numeric, use numeric_precision and numeric_scale
// 			if numPrecision.Valid && numScale.Valid {
// 				fullType = fmt.Sprintf("%s(%d,%d)", dataType, numPrecision.Int32, numScale.Int32)
// 			} else {
// 				fullType = dataType // Default case if precision/scale is not available
// 			}

// 		case "int", "bigint", "smallint", "mediumint":
// 			// For integer types, return the type without size
// 			fullType = dataType

// 		case "datetime", "timestamp":
// 			// For datetime types, return just the type
// 			fullType = dataType

// 		default:
// 			// For other types, return just the data type
// 			fullType = dataType
// 		}

// 	} else {
// 		return "", fmt.Errorf("no matching function found")
// 	}

// 	return fullType, nil
// }

func SaveLog(message string) {
	f, err := os.OpenFile("query.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Println("Error opening log file:", err)
		return
	}
	defer f.Close()

	logger := log.New(f, "", log.LstdFlags)
	logger.Println(message)
}

func SetFocusWithBorder(app *tview.Application, primitive tview.Primitive) {
	// Try to set green border if the primitive supports it
	if borderable, ok := primitive.(interface {
		SetBorder(bool) tview.Primitive
		SetBorderColor(tcell.Color) tview.Primitive
	}); ok {
		borderable.SetBorder(true)
		borderable.SetBorderColor(tcell.ColorGreen)
	}

	app.SetFocus(primitive)
}
