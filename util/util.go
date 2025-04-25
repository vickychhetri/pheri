package util

import (
	"database/sql"
	"fmt"
)

func GetFullReturnType(db *sql.DB, objName string, dbName string) (string, error) {
	// Query to fetch the data type and size/precision for the return type
	query := `
		SELECT c.data_type,
               c.character_maximum_length,
               c.numeric_precision,
               c.numeric_scale
        FROM information_schema.columns c
        JOIN information_schema.routines r ON c.table_name = r.routine_name
        WHERE r.specific_name = ? AND r.routine_schema = ?`

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

		// Handle different types based on data_type
		switch dataType {
		case "varchar", "char":
			// For varchar/char, use the character_maximum_length
			if charLength.Valid {
				fullType = fmt.Sprintf("%s(%d)", dataType, charLength.Int32)
			} else {
				fullType = dataType // If no length, just return the type
			}

		case "decimal", "numeric":
			// For decimal/numeric, use numeric_precision and numeric_scale
			if numPrecision.Valid && numScale.Valid {
				fullType = fmt.Sprintf("%s(%d,%d)", dataType, numPrecision.Int32, numScale.Int32)
			} else {
				fullType = dataType // Default case if precision/scale is not available
			}

		case "int", "bigint", "smallint", "mediumint":
			// For integer types, return the type without size
			fullType = dataType

		case "datetime", "timestamp":
			// For datetime types, return just the type
			fullType = dataType

		default:
			// For other types, return just the data type
			fullType = dataType
		}

	} else {
		return "", fmt.Errorf("no matching function found")
	}

	return fullType, nil
}
