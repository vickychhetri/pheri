// ui/row_copy.go
package ui

import (
	"encoding/json"
	"fmt"
	"math"
	"mysql-tui/util"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// RowCopyData holds a clean, indicator-free snapshot of a highlighted Data Grid row.
type RowCopyData struct {
	Columns   []string
	Cells     []*GridCell
	DBName    string
	TableName string
}

// ExtractRowCopyData reads raw cell values and stripped header column names for the target row.
func ExtractRowCopyData(table *tview.Table, row int, dbName, tableName string) (*RowCopyData, error) {
	if table == nil {
		return nil, fmt.Errorf("table is nil")
	}
	if row <= 0 || row >= table.GetRowCount() {
		return nil, fmt.Errorf("select a valid data row (row %d out of range)", row)
	}
	colCount := table.GetColumnCount()
	if colCount == 0 {
		return nil, fmt.Errorf("table has no columns")
	}

	data := &RowCopyData{
		Columns:   make([]string, 0, colCount),
		Cells:     make([]*GridCell, 0, colCount),
		DBName:    dbName,
		TableName: tableName,
	}

	for c := 0; c < colCount; c++ {
		colName := getHeaderColumnName(table.GetCell(0, c))
		if colName == "" {
			colName = fmt.Sprintf("col_%d", c+1)
		}
		gc := GetGridCellFromTableCell(table.GetCell(row, c), colName)
		data.Columns = append(data.Columns, colName)
		data.Cells = append(data.Cells, gc)
	}

	return data, nil
}

// toJSONScalar converts a GridCell into an appropriate Go scalar for JSON marshaling,
// preserving NULL as nil, unpadded integers/finite floats as numbers, and strings verbatim.
func toJSONScalar(gc *GridCell) interface{} {
	if gc == nil || gc.IsNull() {
		return nil
	}
	val := gc.Value()
	trimmed := strings.TrimSpace(val)
	if trimmed != "" && trimmed == val {
		hasLeadingZeroInt := len(trimmed) > 1 && trimmed[0] == '0' && trimmed[1] != '.'
		hasNegLeadingZero := strings.HasPrefix(trimmed, "-0") && len(trimmed) > 2 && trimmed[2] != '.'
		if !hasLeadingZeroInt && !hasNegLeadingZero {
			if intVal, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
				return intVal
			}
			if floatVal, err := strconv.ParseFloat(trimmed, 64); err == nil && !math.IsNaN(floatVal) && !math.IsInf(floatVal, 0) {
				if strings.Contains(trimmed, ".") {
					return floatVal
				}
			}
		}
	}
	return val
}

// FormatRowAsJSON formats a Data Grid row as a pretty-printed JSON object preserving column order.
func FormatRowAsJSON(data *RowCopyData) (string, error) {
	if data == nil || len(data.Columns) == 0 {
		return "{}", fmt.Errorf("no row data to format")
	}

	var b strings.Builder
	b.WriteString("{\n")
	for i, col := range data.Columns {
		keyBytes, err := json.Marshal(col)
		if err != nil {
			return "", err
		}
		var scalar interface{}
		if i < len(data.Cells) {
			scalar = toJSONScalar(data.Cells[i])
		}
		valBytes, err := json.Marshal(scalar)
		if err != nil {
			return "", err
		}

		b.WriteString("  ")
		b.Write(keyBytes)
		b.WriteString(": ")
		b.Write(valBytes)
		if i < len(data.Columns)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("}")
	return b.String(), nil
}

var tsvSanitizer = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ")

// FormatRowAsTSV formats a Data Grid row as tab-separated values, optionally prefixed with column headers.
func FormatRowAsTSV(data *RowCopyData, includeHeader bool) string {
	if data == nil || len(data.Columns) == 0 {
		return ""
	}

	var b strings.Builder
	if includeHeader {
		for i, col := range data.Columns {
			if i > 0 {
				b.WriteByte('\t')
			}
			b.WriteString(tsvSanitizer.Replace(col))
		}
		b.WriteByte('\n')
	}

	for i := range data.Columns {
		if i > 0 {
			b.WriteByte('\t')
		}
		if i < len(data.Cells) {
			if data.Cells[i] == nil || data.Cells[i].IsNull() {
				b.WriteString("NULL")
			} else {
				b.WriteString(tsvSanitizer.Replace(data.Cells[i].Value()))
			}
		}
	}
	return b.String()
}

// FormatRowAsSQLInsert formats a Data Grid row as a complete SQL INSERT INTO statement.
func FormatRowAsSQLInsert(data *RowCopyData) string {
	if data == nil || len(data.Columns) == 0 {
		return ""
	}

	tblName := data.TableName
	if tblName == "" {
		tblName = "target_table"
	}
	target := util.QuoteIdentifier(tblName)
	if data.DBName != "" && data.TableName != "" {
		target = fmt.Sprintf("%s.%s", util.QuoteIdentifier(data.DBName), util.QuoteIdentifier(tblName))
	}

	quotedCols := make([]string, len(data.Columns))
	valLiterals := make([]string, len(data.Columns))
	for i, col := range data.Columns {
		quotedCols[i] = util.QuoteIdentifier(col)
		if i < len(data.Cells) && data.Cells[i] != nil && !data.Cells[i].IsNull() {
			valLiterals[i] = util.FormatSQLValueLiteral(data.Cells[i].Value())
		} else {
			valLiterals[i] = "NULL"
		}
	}

	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);",
		target,
		strings.Join(quotedCols, ", "),
		strings.Join(valLiterals, ", "),
	)
}

// ShowCopyRowMenuModal extracts the selected row and displays the Quick Copy Row modal menu.
func ShowCopyRowMenuModal(app *tview.Application, table *tview.Table, dbName, tableName string, row int) {
	data, err := ExtractRowCopyData(table, row, dbName, tableName)
	if err != nil {
		updateFooterText(err.Error())
		return
	}
	modal := buildCopyRowModalUI(app, table, data, row)
	if app != nil && modal != nil {
		app.SetRoot(modal, true)
	}
}

type copyFormatOption struct {
	Label      string
	Detail     string
	Shortcut   rune
	FormatName string
	Content    string
}

// buildCopyRowModalUI constructs the interactive Quick Copy Row menu paired with a live preview pane.
func buildCopyRowModalUI(app *tview.Application, table *tview.Table, data *RowCopyData, row int) *tview.Flex {
	jsonText, _ := FormatRowAsJSON(data)
	tsvRowText := FormatRowAsTSV(data, false)
	tsvHeaderText := FormatRowAsTSV(data, true)
	sqlText := FormatRowAsSQLInsert(data)

	options := []copyFormatOption{
		{
			Label:      "  [1] JSON Object",
			Detail:     "Pretty-printed JSON object preserving column order & types",
			Shortcut:   '1',
			FormatName: "JSON",
			Content:    jsonText,
		},
		{
			Label:      "  [2] TSV Row (Values Only)",
			Detail:     "Tab-separated row ready for Excel / Google Sheets",
			Shortcut:   '2',
			FormatName: "TSV Row",
			Content:    tsvRowText,
		},
		{
			Label:      "  [3] TSV with Header Row",
			Detail:     "Column headers + tab-separated values for bug reports",
			Shortcut:   '3',
			FormatName: "TSV with Headers",
			Content:    tsvHeaderText,
		},
		{
			Label:      "  [4] SQL INSERT Statement",
			Detail:     "Parameterized-escaped INSERT INTO ... VALUES (...);",
			Shortcut:   '4',
			FormatName: "SQL INSERT",
			Content:    sqlText,
		},
	}

	previewView := tview.NewTextView().
		SetDynamicColors(false).
		SetScrollable(true).
		SetWrap(true)
	previewView.SetBackgroundColor(CurrentTheme.SurfaceBg)
	previewView.SetTextColor(CurrentTheme.TextPrimary)
	StyleBox(previewView.Box, "Live Clipboard Preview", false)
	previewView.SetText(options[0].Content)

	closeModal := func() {
		if app == nil {
			return
		}
		if mainFlex != nil {
			layout := CreateLayoutWithFooter(app, mainFlex)
			app.SetRoot(layout, true)
			if table != nil {
				util.SetFocusWithBorder(app, table)
			}
		}
	}

	copyAndClose := func(opt copyFormatOption) {
		if err := util.SetClipboardText(opt.Content); err != nil {
			closeModal()
			updateFooterText(fmt.Sprintf("Clipboard copy failed: %v", err))
			return
		}
		closeModal()
		updateFooterText(fmt.Sprintf("Copied row #%d as %s to clipboard!", row, opt.FormatName))
	}

	list := tview.NewList().ShowSecondaryText(true)
	list.SetBackgroundColor(CurrentTheme.SurfaceBg)
	StyleList(list)
	StyleModalBox(list.Box, fmt.Sprintf("Quick Copy Row #%d", row))

	for _, opt := range options {
		selectedOpt := opt
		list.AddItem(selectedOpt.Label, "    "+selectedOpt.Detail, selectedOpt.Shortcut, func() {
			copyAndClose(selectedOpt)
		})
	}
	list.AddItem("  [Esc] Cancel", "    Close menu and return to Data Grid", 0, func() {
		closeModal()
		updateFooterText("Row copy cancelled.")
	})

	list.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(options) {
			previewView.SetText(options[index].Content)
			previewView.ScrollToBeginning()
		} else {
			previewView.SetText("(Press Enter or Esc to return to Data Grid)")
		}
	})

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			closeModal()
			updateFooterText("Row copy cancelled.")
			return nil
		}
		return event
	})

	statusBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(FormatThemeMarkup(" [lime::b]1-4 / Enter[-::-] Copy to Clipboard  [gray]│[-] [aqua::b]↑ / ↓[-::-] Preview Format  [gray]│[-] [yellow::b]Esc[-::-] Cancel"))
	statusBar.SetBackgroundColor(CurrentTheme.HeaderBg)

	contentFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(list, 13, 0, true).
		AddItem(previewView, 0, 1, false).
		AddItem(statusBar, 1, 0, false)

	modalFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 2, 0, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 4, 0, false).
			AddItem(contentFlex, 0, 1, true).
			AddItem(nil, 4, 0, false),
			0, 1, true).
		AddItem(nil, 2, 0, false)

	return modalFlex
}
