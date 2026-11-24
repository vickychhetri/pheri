// ui/delete_row.go
package ui

import (
	"context"
	"database/sql"
	"fmt"
	"mysql-tui/phhistory"
	"mysql-tui/util"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// BuildRowWhereClause constructs a parameterized WHERE clause matching the targeted row in table.
// When all primary key columns (pkCols) are present in the visible grid, it matches on those PK columns.
// Otherwise, it safely falls back to matching all visible columns in the row (handling NULLs via IS NULL).
func BuildRowWhereClause(table *tview.Table, row int, pkCols []string) (string, []interface{}, error) {
	if table == nil {
		return "", nil, fmt.Errorf("table is nil")
	}
	if row <= 0 || row >= table.GetRowCount() {
		return "", nil, fmt.Errorf("invalid target row %d", row)
	}

	colCount := table.GetColumnCount()
	if colCount == 0 {
		return "", nil, fmt.Errorf("table has no columns")
	}

	// 1. Try Primary Key matching when all PK columns are present in the grid
	if len(pkCols) > 0 {
		var pkClauses []string
		var pkArgs []interface{}
		matchedCount := 0

		for _, pkCol := range pkCols {
			trimmedPK := strings.TrimSpace(pkCol)
			if trimmedPK == "" {
				continue
			}
			for colIdx := 0; colIdx < colCount; colIdx++ {
				colHeader := getHeaderColumnName(table.GetCell(0, colIdx))
				if strings.EqualFold(colHeader, trimmedPK) {
					pkGC := GetGridCellFromTableCell(table.GetCell(row, colIdx), trimmedPK)
					if pkGC.IsNull() {
						pkClauses = append(pkClauses, fmt.Sprintf("%s IS NULL", util.QuoteIdentifier(trimmedPK)))
					} else {
						pkClauses = append(pkClauses, fmt.Sprintf("%s = ?", util.QuoteIdentifier(trimmedPK)))
						pkArgs = append(pkArgs, pkGC.Value())
					}
					matchedCount++
					break
				}
			}
		}

		if matchedCount == len(pkCols) && len(pkClauses) > 0 {
			return strings.Join(pkClauses, " AND "), pkArgs, nil
		}
	}

	// 2. Fallback: match all visible columns for keyless tables or partial projections
	var allClauses []string
	var allArgs []interface{}
	for colIdx := 0; colIdx < colCount; colIdx++ {
		colHeader := getHeaderColumnName(table.GetCell(0, colIdx))
		if colHeader == "" {
			continue
		}
		rowGC := GetGridCellFromTableCell(table.GetCell(row, colIdx), colHeader)
		if rowGC.IsNull() {
			allClauses = append(allClauses, fmt.Sprintf("%s IS NULL", util.QuoteIdentifier(colHeader)))
		} else {
			allClauses = append(allClauses, fmt.Sprintf("%s = ?", util.QuoteIdentifier(colHeader)))
			allArgs = append(allArgs, rowGC.Value())
		}
	}

	if len(allClauses) == 0 {
		return "", nil, fmt.Errorf("unable to identify columns for WHERE condition")
	}
	return strings.Join(allClauses, " AND "), allArgs, nil
}

// BuildParameterizedDeleteQuery builds a targeted DELETE FROM ... WHERE ... LIMIT 1 statement,
// argument slice, and human-readable preview SQL string.
func BuildParameterizedDeleteQuery(dbName, tableName string, table *tview.Table, row int, pkCols []string) (string, []interface{}, string, error) {
	if tableName == "" {
		return "", nil, "", fmt.Errorf("table name is required")
	}

	whereClause, args, err := BuildRowWhereClause(table, row, pkCols)
	if err != nil {
		return "", nil, "", err
	}

	targetTable := util.QuoteIdentifier(tableName)
	if dbName != "" {
		targetTable = fmt.Sprintf("%s.%s", util.QuoteIdentifier(dbName), util.QuoteIdentifier(tableName))
	}

	deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE %s LIMIT 1", targetTable, whereClause)
	previewSQL := phhistory.ReplacePlaceholders(deleteQuery, args...)
	return deleteQuery, args, previewSQL, nil
}

// TriggerRowDeleteFlow validates guards and fetches primary key metadata off the UI thread
// before presenting the deletion confirmation modal.
func TriggerRowDeleteFlow(app *tview.Application, db *sql.DB, dbName, tableName string, table *tview.Table, row int) {
	if ActiveReadOnly {
		showErrorModal(app, mainFlex, "Action Blocked: Active Connection is in READ-ONLY Mode. Row deletion is disabled.")
		return
	}
	if db == nil || dbName == "" || tableName == "" {
		updateFooterText("Select a base TABLE in the sidebar before deleting a row.")
		return
	}
	if table == nil || row <= 0 || row >= table.GetRowCount() {
		updateFooterText("Select a data row to delete.")
		return
	}

	updateFooterText(fmt.Sprintf("Preparing row deletion check for %s.%s...", dbName, tableName))

	go func() {
		pkCols, _ := GetPrimaryKeyColumns(db, dbName, tableName)
		if app == nil {
			return
		}
		app.QueueUpdateDraw(func() {
			modal := buildDeleteRowModalUI(app, db, dbName, tableName, table, row, pkCols)
			if modal != nil {
				app.SetRoot(modal, true)
			}
		})
	}()
}

// buildDeleteRowModalUI constructs the confirmation modal defaulting focus to Cancel for operator safety.
func buildDeleteRowModalUI(app *tview.Application, db *sql.DB, dbName, tableName string, table *tview.Table, row int, pkCols []string) *tview.Modal {
	deleteQuery, args, previewSQL, err := BuildParameterizedDeleteQuery(dbName, tableName, table, row, pkCols)
	if err != nil {
		showErrorModal(app, mainFlex, "Cannot delete row: "+err.Error())
		return nil
	}

	txHint := ""
	if !IsAutoCommitMode() {
		txHint = "\n[aqua](Manual TX mode active: change can be committed/rolled back later)[-]"
	}

	modalText := FormatThemeMarkup(fmt.Sprintf(
		"[red::b]Confirm Row Deletion[::-]\n\nTarget: [white::b]%s.%s[::-]\n\n[gray]%s[-]%s",
		dbName, tableName, tview.Escape(previewSQL), txHint,
	))

	modal := tview.NewModal().
		SetText(modalText).
		AddButtons([]string{"Delete Row", "Cancel"}).
		SetBackgroundColor(tcell.ColorDarkBlue).
		SetButtonBackgroundColor(tcell.ColorDarkCyan)
	modal.SetFocus(1) // Default focus on Cancel to prevent accidental deletions

	restoreGrid := func() {
		if app == nil || mainFlex == nil {
			return
		}
		layout := CreateLayoutWithFooter(app, mainFlex)
		app.SetRoot(layout, true)
		if table != nil {
			util.SetFocusWithBorder(app, table)
		}
	}

	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if buttonIndex != 0 {
			restoreGrid()
			updateFooterText("Row deletion cancelled.")
			return
		}

		if db == nil {
			restoreGrid()
			showErrorModal(app, mainFlex, "Cannot delete row: database connection is nil.")
			return
		}

		restoreGrid()
		updateFooterText(fmt.Sprintf("Deleting row from %s.%s...", dbName, tableName))

		go func() {
			ctxExec, cancelExec := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelExec()

			var execErr error
			if !IsAutoCommitMode() {
				tx, txErr := EnsureActiveTx(ctxExec, db)
				if txErr != nil {
					execErr = txErr
				} else {
					_, execErr = tx.ExecContext(ctxExec, deleteQuery, args...)
					if execErr == nil {
						RecordMutationInTx(previewSQL, "Row Delete Modal")
					}
				}
			} else {
				_, execErr = db.ExecContext(ctxExec, deleteQuery, args...)
			}

			if app == nil {
				return
			}
			app.QueueUpdateDraw(func() {
				if execErr != nil {
					util.SaveLog("Delete row error: " + execErr.Error())
					showErrorModal(app, mainFlex, "Delete failed: "+execErr.Error())
					return
				}

				phhistory.SaveQuery(previewSQL, dbName)
				util.SaveLog(previewSQL)
				updateFooterText(fmt.Sprintf("Deleted 1 row from %s.%s.", dbName, tableName))

				if activeGridDB != nil && activeGridQuery != "" && table != nil {
					ExecuteQueryAsync(app, activeGridDB, activeGridQuery, table, nil)
				}
			})
		}()
	})

	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			restoreGrid()
			updateFooterText("Row deletion cancelled.")
			return nil
		}
		return event
	})

	return modal
}
