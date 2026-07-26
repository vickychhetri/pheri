// ui/export.go
package ui

import (
	"database/sql"
	"fmt"
	"mysql-tui/util"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ExportItem struct {
	Name     string
	Type     string // "TABLE", "VIEW", "PROCEDURE", "FUNCTION", "TRIGGER", "EVENT"
	Selected bool
}

type InteractiveExportOptions struct {
	SelectedItems  []ExportItem
	IncludeSchema  bool
	IncludeData    bool
	Format         string // "SQL", "JSON", "CSV"
	OutputPath     string
}

func ShowExportWizardModal(app *tview.Application, db *sql.DB, dbName string) {
	// Fetch all objects from database
	var items []ExportItem

	// 1. Tables & Views
	tRows, err := db.Query("SELECT table_name, table_type FROM information_schema.tables WHERE table_schema = ?", dbName)
	if err == nil {
		defer tRows.Close()
		for tRows.Next() {
			var name, tType string
			if err := tRows.Scan(&name, &tType); err == nil {
				kind := "TABLE"
				if strings.Contains(tType, "VIEW") {
					kind = "VIEW"
				}
				items = append(items, ExportItem{Name: name, Type: kind, Selected: true})
			}
		}
	}

	// 2. Routines (Procedures & Functions)
	rRows, err := db.Query("SELECT routine_name, routine_type FROM information_schema.routines WHERE routine_schema = ?", dbName)
	if err == nil {
		defer rRows.Close()
		for rRows.Next() {
			var name, rType string
			if err := rRows.Scan(&name, &rType); err == nil {
				items = append(items, ExportItem{Name: name, Type: strings.ToUpper(rType), Selected: true})
			}
		}
	}

	// 3. Triggers
	trRows, err := db.Query("SELECT trigger_name FROM information_schema.triggers WHERE trigger_schema = ?", dbName)
	if err == nil {
		defer trRows.Close()
		for trRows.Next() {
			var name string
			if err := trRows.Scan(&name); err == nil {
				items = append(items, ExportItem{Name: name, Type: "TRIGGER", Selected: true})
			}
		}
	}

	// 4. Events
	evRows, err := db.Query("SELECT event_name FROM information_schema.events WHERE event_schema = ?", dbName)
	if err == nil {
		defer evRows.Close()
		for evRows.Next() {
			var name string
			if err := evRows.Scan(&name); err == nil {
				items = append(items, ExportItem{Name: name, Type: "EVENT", Selected: true})
			}
		}
	}

	showItemSelector(app, db, dbName, items)
}

func showItemSelector(app *tview.Application, db *sql.DB, dbName string, items []ExportItem) {
	list := tview.NewList().ShowSecondaryText(true)
	list.SetBorder(true).
		SetTitle(fmt.Sprintf(" 📦 HeidiSQL Export Picker: %s (%d objects) ", dbName, len(items))).
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorDarkCyan)

	renderList := func() {
		list.Clear()
		for idx, item := range items {
			icon := "📋"
			switch item.Type {
			case "VIEW":
				icon = "👁️"
			case "PROCEDURE":
				icon = "⚙️"
			case "FUNCTION":
				icon = "⚡"
			case "TRIGGER":
				icon = "🔔"
			case "EVENT":
				icon = "📅"
			}

			check := "[lime][x][-]"
			if !item.Selected {
				check = "[gray][ ][-]"
			}

			label := fmt.Sprintf("%s %s [%s] [white::b]%s[::-]", check, icon, item.Type, item.Name)
			itemIdx := idx
			list.AddItem(label, "Space: Toggle | Enter: Select Settings", 0, func() {
				items[itemIdx].Selected = !items[itemIdx].Selected
			})
		}
	}

	renderList()

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case ' ':
				cur := list.GetCurrentItem()
				if cur >= 0 && cur < len(items) {
					items[cur].Selected = !items[cur].Selected
					renderList()
					list.SetCurrentItem(cur)
				}
				return nil
			case 'a', 'A':
				for i := range items {
					items[i].Selected = true
				}
				renderList()
				return nil
			case 'n', 'N':
				for i := range items {
					items[i].Selected = false
				}
				renderList()
				return nil
			}
		} else if event.Key() == tcell.KeyEnter {
			showExportSettingsForm(app, db, dbName, items)
			return nil
		} else if event.Key() == tcell.KeyEscape {
			layout := CreateLayoutWithFooter(app, mainFlex)
			app.SetRoot(layout, true)
			return nil
		}
		return event
	})

	hint := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[yellow]SPACE: Toggle Item | [lime]a: Select All | [lime]n: Deselect All | [cyan]ENTER: Continue | [white]ESC: Cancel")

	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(hint, 1, 0, false)

	app.SetRoot(layout, true).SetFocus(list)
}

func showExportSettingsForm(app *tview.Application, db *sql.DB, dbName string, items []ExportItem) {
	defaultFileName := fmt.Sprintf("./export/pheri_%s_%s.sql", dbName, time.Now().Format("20060102_150405"))

	selectedCount := 0
	for _, it := range items {
		if it.Selected {
			selectedCount++
		}
	}

	form := tview.NewForm()
	form.SetBorder(true).
		SetTitle(fmt.Sprintf(" ⚙ Export Settings (%d of %d objects selected) ", selectedCount, len(items))).
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorDarkCyan).
		SetTitleColor(tcell.ColorAqua).
		SetBorderPadding(1, 1, 3, 3)

	form.AddCheckbox("Include Table & Object DDL Schema", true, nil)
	form.AddCheckbox("Include Table Data Rows (INSERT statements)", true, nil)
	form.AddDropDown("Export Format", []string{"SQL Script (.sql)", "JSON (.json)", "CSV Folder (.csv)"}, 0, nil)
	form.AddInputField("Output Target File/Path", defaultFileName, 50, nil, nil)

	form.AddButton("[lime::b] ▶ START EXPORT ", func() {
		opts := InteractiveExportOptions{
			SelectedItems: items,
			IncludeSchema: form.GetFormItem(0).(*tview.Checkbox).IsChecked(),
			IncludeData:   form.GetFormItem(1).(*tview.Checkbox).IsChecked(),
		}

		_, formatStr := form.GetFormItem(2).(*tview.DropDown).GetCurrentOption()
		if strings.Contains(formatStr, "JSON") {
			opts.Format = "JSON"
		} else if strings.Contains(formatStr, "CSV") {
			opts.Format = "CSV"
		} else {
			opts.Format = "SQL"
		}

		opts.OutputPath = form.GetFormItem(3).(*tview.InputField).GetText()

		executeInteractiveExport(app, db, dbName, opts)
	})

	form.AddButton("[yellow::b] ⬅ BACK TO OBJECTS ", func() {
		showItemSelector(app, db, dbName, items)
	})

	form.AddButton("[red::b] ✖ CANCEL ", func() {
		layout := CreateLayoutWithFooter(app, mainFlex)
		app.SetRoot(layout, true)
	})

	form.SetBackgroundColor(tcell.ColorBlack)
	form.SetFieldBackgroundColor(tcell.Color234)
	form.SetLabelColor(tcell.ColorWhite)
	form.SetButtonBackgroundColor(tcell.Color236)

	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(form, 64, 1, true).
			AddItem(nil, 0, 1, false), 16, 1, true).
		AddItem(nil, 0, 1, false)

	app.SetRoot(layout, true).SetFocus(form)
}

func executeInteractiveExport(app *tview.Application, db *sql.DB, dbName string, opts InteractiveExportOptions) {
	logView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() {
			app.Draw()
		})

	logView.SetBorder(true).
		SetTitle(" 🚀 Exporting Database: " + dbName + " ").
		SetBorderColor(tcell.ColorDarkCyan)

	app.SetRoot(logView, true)

	go func() {
		logView.SetText("[cyan::b]Starting interactive export...\n\n")

		dir := filepath.Dir(opts.OutputPath)
		if dir != "." && dir != "" {
			_ = os.MkdirAll(dir, 0755)
		}

		var summary strings.Builder
		summary.WriteString(fmt.Sprintf("-- Pheri MySQL Export Dump\n-- Database: %s\n-- Timestamp: %s\n\n",
			dbName, time.Now().Format(time.RFC3339)))
		summary.WriteString("SET FOREIGN_KEY_CHECKS=0;\nSET SQL_MODE=\"NO_AUTO_VALUE_ON_ZERO\";\n\n")

		exportedCounts := map[string]int{}

		for _, item := range opts.SelectedItems {
			if !item.Selected {
				continue
			}

			switch item.Type {
			case "TABLE":
				appendLog(logView, "  [green]• Exporting Table: [white]"+item.Name)
				if opts.IncludeSchema {
					ddl, err := getTableDDL(db, dbName, item.Name)
					if err == nil {
						summary.WriteString(fmt.Sprintf("-- Table structure for `%s`\nDROP TABLE IF EXISTS `%s`;\n%s;\n\n", item.Name, item.Name, ddl))
					}
				}
				if opts.IncludeData {
					dataSQL, count := getTableDataSQL(db, dbName, item.Name)
					summary.WriteString(dataSQL + "\n\n")
					exportedCounts["Rows"] += count
				}
				exportedCounts["Tables"]++

			case "VIEW":
				appendLog(logView, "  [green]• Exporting View: [white]"+item.Name)
				if opts.IncludeSchema {
					ddl, err := getViewDDL(db, dbName, item.Name)
					if err == nil {
						summary.WriteString(fmt.Sprintf("-- View structure for `%s`\nDROP VIEW IF EXISTS `%s`;\n%s;\n\n", item.Name, item.Name, ddl))
						exportedCounts["Views"]++
					}
				}

			case "PROCEDURE":
				appendLog(logView, "  [green]• Exporting Procedure: [white]"+item.Name)
				ddl, err := GetProcedureDDL(db, dbName, item.Name)
				if err == nil {
					summary.WriteString(fmt.Sprintf("-- Procedure `%s`\nDROP PROCEDURE IF EXISTS `%s`;\nDELIMITER $$\n%s $$\nDELIMITER ;\n\n", item.Name, item.Name, ddl))
					exportedCounts["Procedures"]++
				}

			case "FUNCTION":
				appendLog(logView, "  [green]• Exporting Function: [white]"+item.Name)
				ddl, err := GetFunctionDDL(db, dbName, item.Name)
				if err == nil {
					summary.WriteString(fmt.Sprintf("-- Function `%s`\nDROP FUNCTION IF EXISTS `%s`;\nDELIMITER $$\n%s $$\nDELIMITER ;\n\n", item.Name, item.Name, ddl))
					exportedCounts["Functions"]++
				}

			case "TRIGGER":
				appendLog(logView, "  [green]• Exporting Trigger: [white]"+item.Name)
				ddl, err := GetTriggerDDL(db, dbName, item.Name)
				if err == nil {
					summary.WriteString(fmt.Sprintf("-- Trigger `%s`\nDROP TRIGGER IF EXISTS `%s`;\nDELIMITER $$\n%s $$\nDELIMITER ;\n\n", item.Name, item.Name, ddl))
					exportedCounts["Triggers"]++
				}

			case "EVENT":
				appendLog(logView, "  [green]• Exporting Event: [white]"+item.Name)
				ddl, err := GetEventDDL(db, dbName, item.Name)
				if err == nil {
					summary.WriteString(fmt.Sprintf("-- Event `%s`\nDROP EVENT IF EXISTS `%s`;\nDELIMITER $$\n%s $$\nDELIMITER ;\n\n", item.Name, item.Name, ddl))
					exportedCounts["Events"]++
				}
			}
		}

		summary.WriteString("SET FOREIGN_KEY_CHECKS=1;\n")

		err := os.WriteFile(opts.OutputPath, []byte(summary.String()), 0644)
		if err != nil {
			appendLog(logView, fmt.Sprintf("\n[red::b]❌ Export Error: %v", err))
		} else {
			appendLog(logView, fmt.Sprintf("\n[lime::b]✅ Export Complete!\n[white]Saved to: [cyan]%s\n[white]Exported: %d Tables (%d Rows), %d Views, %d Procedures, %d Functions, %d Triggers, %d Events",
				opts.OutputPath, exportedCounts["Tables"], exportedCounts["Rows"], exportedCounts["Views"],
				exportedCounts["Procedures"], exportedCounts["Functions"], exportedCounts["Triggers"], exportedCounts["Events"]))
		}

		appendLog(logView, "\n[yellow]Press ENTER or ESC to return to workspace.")

		logView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEnter || event.Key() == tcell.KeyEscape {
				layout := CreateLayoutWithFooter(app, mainFlex)
				app.SetRoot(layout, true)
				return nil
			}
			return event
		})
	}()
}

func appendLog(view *tview.TextView, msg string) {
	curr := view.GetText(false)
	view.SetText(curr + msg + "\n")
}

func getTableDDL(db *sql.DB, dbName, tableName string) (string, error) {
	query := fmt.Sprintf("SHOW CREATE TABLE `%s`.`%s`", dbName, tableName)
	row := db.QueryRow(query)
	var name, ddl string
	err := row.Scan(&name, &ddl)
	return ddl, err
}

func getViewDDL(db *sql.DB, dbName, viewName string) (string, error) {
	query := fmt.Sprintf("SHOW CREATE VIEW `%s`.`%s`", dbName, viewName)
	row := db.QueryRow(query)
	var name, ddl, charset, collation string
	err := row.Scan(&name, &ddl, &charset, &collation)
	return ddl, err
}

func getTableDataSQL(db *sql.DB, dbName, tableName string) (string, int) {
	query := fmt.Sprintf("SELECT * FROM `%s`.`%s`", dbName, tableName)
	rows, err := db.Query(query)
	if err != nil {
		return "", 0
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil || len(cols) == 0 {
		return "", 0
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("-- Data for `%s`\n", tableName))

	quotedCols := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = util.QuoteIdentifier(c)
	}

	rowCount := 0
	for rows.Next() {
		rowCount++
		values := make([]interface{}, len(cols))
		pointers := make([]interface{}, len(cols))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			continue
		}

		valStrs := make([]string, len(cols))
		for i, val := range values {
			if val == nil {
				valStrs[i] = "NULL"
			} else {
				switch v := val.(type) {
				case []byte:
					valStrs[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(string(v), "'", "''"))
				case string:
					valStrs[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
				default:
					valStrs[i] = fmt.Sprintf("'%v'", v)
				}
			}
		}

		sb.WriteString(fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);\n",
			util.QuoteIdentifier(tableName),
			strings.Join(quotedCols, ", "),
			strings.Join(valStrs, ", ")))
	}

	return sb.String(), rowCount
}

func GetProcedureDDL(db *sql.DB, dbName, procName string) (string, error) {
	query := fmt.Sprintf("SHOW CREATE PROCEDURE `%s`.`%s`", dbName, procName)
	row := db.QueryRow(query)
	var proc, sqlMode, createStmt, charset, collation string
	err := row.Scan(&proc, &sqlMode, &createStmt, &charset, &collation)
	return createStmt, err
}

func GetFunctionDDL(db *sql.DB, dbName, funcName string) (string, error) {
	query := fmt.Sprintf("SHOW CREATE FUNCTION `%s`.`%s`", dbName, funcName)
	row := db.QueryRow(query)
	var fn, sqlMode, createStmt, charset, collation string
	err := row.Scan(&fn, &sqlMode, &createStmt, &charset, &collation)
	return createStmt, err
}
