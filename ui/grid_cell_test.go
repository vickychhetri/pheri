package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/rivo/tview"
)

func TestGridCell_NullAndEmptyHandling(t *testing.T) {
	// 1. NULL Cell
	nullCell := NewGridCellNull("description")
	if !nullCell.IsNull() {
		t.Errorf("expected nullCell.IsNull() to be true")
	}
	if nullCell.IsEmpty() {
		t.Errorf("expected nullCell.IsEmpty() to be false")
	}
	if nullCell.Value() != "" {
		t.Errorf("expected nullCell.Value() to be empty string, got %q", nullCell.Value())
	}

	// 2. Empty String Cell
	emptyCell := NewGridCellString("postal_code", "")
	if emptyCell.IsNull() {
		t.Errorf("expected emptyCell.IsNull() to be false")
	}
	if !emptyCell.IsEmpty() {
		t.Errorf("expected emptyCell.IsEmpty() to be true")
	}
	if emptyCell.Value() != "" {
		t.Errorf("expected emptyCell.Value() to be empty string, got %q", emptyCell.Value())
	}

	// 3. Normal Value Cell
	valCell := NewGridCellString("title", "Inception")
	if valCell.IsNull() {
		t.Errorf("expected valCell.IsNull() to be false")
	}
	if valCell.IsEmpty() {
		t.Errorf("expected valCell.IsEmpty() to be false")
	}
	if valCell.Value() != "Inception" {
		t.Errorf("expected valCell.Value() to be 'Inception', got %q", valCell.Value())
	}
}

func TestGridCell_DisplayTextFormatting(t *testing.T) {
	nullCell := NewGridCellNull("notes")
	if nullCell.DisplayText() != "[gray::i]NULL[-::-]" {
		t.Errorf("expected '[gray::i]NULL[-::-]', got %q", nullCell.DisplayText())
	}

	emptyCell := NewGridCellString("notes", "")
	if emptyCell.DisplayText() != "[gray::i]EMPTY[-::-]" {
		t.Errorf("expected '[gray::i]EMPTY[-::-]', got %q", emptyCell.DisplayText())
	}

	valCell := NewGridCellString("notes", "Regular Note")
	if valCell.DisplayText() != "Regular Note" {
		t.Errorf("expected 'Regular Note', got %q", valCell.DisplayText())
	}

	// Test user data containing bracket tags (e.g. log tags or JSON array representations)
	bracketCell := NewGridCellString("log", "[ERROR] Connection failed [code: 500]")
	escaped := bracketCell.DisplayText()
	if !strings.Contains(escaped, "[]") {
		t.Errorf("expected tview bracket escaping in %q", escaped)
	}
}

func TestGridCell_ExportValue(t *testing.T) {
	nullCell := NewGridCellNull("col1")
	if nullCell.ExportValue("") != "" {
		t.Errorf("expected empty string for CSV export of NULL, got %q", nullCell.ExportValue(""))
	}
	if nullCell.ExportValue("NULL") != "NULL" {
		t.Errorf("expected 'NULL' placeholder, got %q", nullCell.ExportValue("NULL"))
	}

	emptyCell := NewGridCellString("col2", "")
	if emptyCell.ExportValue("") != "" {
		t.Errorf("expected empty string for empty cell export, got %q", emptyCell.ExportValue(""))
	}

	valCell := NewGridCellString("col3", "Active")
	if valCell.ExportValue("") != "Active" {
		t.Errorf("expected 'Active', got %q", valCell.ExportValue(""))
	}
}

func TestGetGridCellFromTableCell_WithReference(t *testing.T) {
	origGC := NewGridCellString("username", "admin")
	tableCell := tview.NewTableCell(" admin ")
	tableCell.SetReference(origGC)

	extracted := GetGridCellFromTableCell(tableCell, "username")
	if extracted != origGC {
		t.Errorf("expected extracted GridCell to match attached reference pointer")
	}
	if extracted.Value() != "admin" {
		t.Errorf("expected value 'admin', got %q", extracted.Value())
	}
}

func TestGetGridCellFromTableCell_FallbackParsing(t *testing.T) {
	// 1. Legacy NULL markup
	cellNull1 := tview.NewTableCell(" [gray]NULL ")
	gc1 := GetGridCellFromTableCell(cellNull1, "test_col")
	if !gc1.IsNull() {
		t.Errorf("expected legacy '[gray]NULL' to parse as IsNull() == true")
	}

	cellNull2 := tview.NewTableCell(" [gray::i]NULL[-::-] ")
	gc2 := GetGridCellFromTableCell(cellNull2, "test_col")
	if !gc2.IsNull() {
		t.Errorf("expected italic '[gray::i]NULL[-::-]' to parse as IsNull() == true")
	}

	// 2. Legacy EMPTY markup
	cellEmpty := tview.NewTableCell(" [gray]EMPTY ")
	gc3 := GetGridCellFromTableCell(cellEmpty, "test_col")
	if gc3.IsNull() {
		t.Errorf("expected legacy '[gray]EMPTY' not to parse as NULL")
	}
	if !gc3.IsEmpty() {
		t.Errorf("expected legacy '[gray]EMPTY' to parse as IsEmpty() == true")
	}

	// 3. Normal value without reference
	cellVal := tview.NewTableCell(" 42 ")
	gc4 := GetGridCellFromTableCell(cellVal, "id")
	if gc4.Value() != "42" {
		t.Errorf("expected cleaned value '42', got %q", gc4.Value())
	}

	// 4. Nil table cell
	gc5 := GetGridCellFromTableCell(nil, "missing")
	if !gc5.IsNull() {
		t.Errorf("expected nil table cell to parse as NULL")
	}
}

func TestGridCell_SQLParameterBinding(t *testing.T) {
	buildUpdateParam := func(gc *GridCell) interface{} {
		if gc.IsNull() {
			return nil
		}
		return gc.Value()
	}

	// 1. NULL should bind nil
	nullGC := NewGridCellNull("deleted_at")
	paramNull := buildUpdateParam(nullGC)
	if paramNull != nil {
		t.Errorf("expected nil parameter for NULL cell, got %v", paramNull)
	}

	// 2. Empty string should bind ""
	emptyGC := NewGridCellString("middle_name", "")
	paramEmpty := buildUpdateParam(emptyGC)
	if paramEmpty != "" {
		t.Errorf("expected empty string parameter for empty cell, got %v", paramEmpty)
	}

	// 3. String value should bind string
	valGC := NewGridCellString("name", "Alice")
	paramVal := buildUpdateParam(valGC)
	if paramVal != "Alice" {
		t.Errorf("expected 'Alice' parameter for value cell, got %v", paramVal)
	}
}

func TestGridCell_WhereClauseGeneration(t *testing.T) {
	columns := []string{"id", "code", "optional_tag"}
	cells := []*GridCell{
		NewGridCellString("id", "101"),
		NewGridCellString("code", ""),
		NewGridCellNull("optional_tag"),
	}

	var whereClauses []string
	var queryArgs []interface{}

	for i, col := range columns {
		gc := cells[i]
		if gc.IsNull() {
			whereClauses = append(whereClauses, fmt.Sprintf("`%s` IS NULL", col))
		} else {
			whereClauses = append(whereClauses, fmt.Sprintf("`%s` = ?", col))
			queryArgs = append(queryArgs, gc.Value())
		}
	}

	whereSQL := strings.Join(whereClauses, " AND ")
	expectedSQL := "`id` = ? AND `code` = ? AND `optional_tag` IS NULL"
	if whereSQL != expectedSQL {
		t.Errorf("expected WHERE clause %q, got %q", expectedSQL, whereSQL)
	}

	if len(queryArgs) != 2 {
		t.Fatalf("expected 2 query args, got %d", len(queryArgs))
	}
	if queryArgs[0] != "101" {
		t.Errorf("expected queryArgs[0] to be '101', got %v", queryArgs[0])
	}
	if queryArgs[1] != "" {
		t.Errorf("expected queryArgs[1] to be '', got %v", queryArgs[1])
	}
}

func TestGridCell_ExportSerializationFormatting(t *testing.T) {
	// JSON serialization test
	rowMap := make(map[string]interface{})
	cells := map[string]*GridCell{
		"first_name": NewGridCellString("first_name", "John"),
		"nickname":   NewGridCellString("nickname", ""),
		"deleted_at": NewGridCellNull("deleted_at"),
	}

	for k, gc := range cells {
		if gc.IsNull() {
			rowMap[k] = nil
		} else {
			rowMap[k] = gc.Value()
		}
	}

	jsonData, err := json.Marshal(rowMap)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	jsonStr := string(jsonData)

	// Verify genuine JSON null
	if !strings.Contains(jsonStr, `"deleted_at":null`) {
		t.Errorf("expected deleted_at to serialize as JSON null in %s", jsonStr)
	}
	// Verify empty string
	if !strings.Contains(jsonStr, `"nickname":""`) {
		t.Errorf("expected nickname to serialize as empty string in %s", jsonStr)
	}
	// Verify value
	if !strings.Contains(jsonStr, `"first_name":"John"`) {
		t.Errorf("expected first_name to serialize as 'John' in %s", jsonStr)
	}
}
