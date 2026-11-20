package ui

import (
	"strings"

	"github.com/rivo/tview"
	"mysql-tui/util"
)

// GridCell represents a single typed cell in the data grid, strictly separating
// underlying database values (SQL NULL, empty string, raw data) from TUI visual markup.
type GridCell struct {
	RawValue *string // nil represents SQL NULL; non-nil pointer represents literal SQL value
	ColName  string  // Column name from database schema
	IsHeader bool    // True if cell is a column header
}

// NewGridCell creates a new GridCell instance from a column name and raw string pointer.
func NewGridCell(colName string, raw *string) *GridCell {
	return &GridCell{
		RawValue: raw,
		ColName:  colName,
	}
}

// NewGridCellString creates a GridCell with a literal string value.
func NewGridCellString(colName string, val string) *GridCell {
	s := val
	return &GridCell{
		RawValue: &s,
		ColName:  colName,
	}
}

// NewGridCellNull creates a GridCell explicitly holding SQL NULL.
func NewGridCellNull(colName string) *GridCell {
	return &GridCell{
		RawValue: nil,
		ColName:  colName,
	}
}

// IsNull returns true if the underlying SQL value is NULL.
func (c *GridCell) IsNull() bool {
	return c == nil || c.RawValue == nil
}

// IsEmpty returns true if the underlying SQL value is a non-null empty string.
func (c *GridCell) IsEmpty() bool {
	return c != nil && c.RawValue != nil && *c.RawValue == ""
}

// Value returns the raw string value, or an empty string if NULL.
func (c *GridCell) Value() string {
	if c == nil || c.RawValue == nil {
		return ""
	}
	return *c.RawValue
}

// DisplayText returns the formatted TUI representation for rendering inside tview.Table.
// User data is escaped via tview.Escape to prevent bracket tags from breaking TUI formatting.
func (c *GridCell) DisplayText() string {
	if c == nil || c.RawValue == nil {
		return "[gray::i]NULL[-::-]"
	}
	if *c.RawValue == "" {
		return "[gray::i]EMPTY[-::-]"
	}
	return tview.Escape(*c.RawValue)
}

// ExportValue returns the raw value formatted for CSV or plain text export.
// If the cell is NULL, it returns nullPlaceholder (e.g. "" for CSV or "NULL").
func (c *GridCell) ExportValue(nullPlaceholder string) string {
	if c == nil || c.RawValue == nil {
		return nullPlaceholder
	}
	return *c.RawValue
}

// GetGridCellFromTableCell safely extracts the GridCell reference from a tview.TableCell.
// If the cell does not contain a GridCell reference, it parses legacy text formatting
// to construct an accurate fallback GridCell without crashing.
func GetGridCellFromTableCell(cell *tview.TableCell, colName string) *GridCell {
	if cell == nil {
		return NewGridCellNull(colName)
	}

	if ref := cell.GetReference(); ref != nil {
		if gc, ok := ref.(*GridCell); ok && gc != nil {
			return gc
		}
	}

	// Fallback: parse legacy cell text markup
	trimmed := strings.TrimSpace(cell.Text)
	if trimmed == "[gray]NULL" || trimmed == "[gray::i]NULL[-::-]" || trimmed == "NULL" {
		return NewGridCellNull(colName)
	}
	if trimmed == "[gray]EMPTY" || trimmed == "[gray::i]EMPTY[-::-]" {
		emptyStr := ""
		return NewGridCell(colName, &emptyStr)
	}

	stripped := util.StripFormatting(cell.Text)
	// Strip outer padding spaces added for TUI display
	cleaned := strings.TrimPrefix(stripped, " ")
	cleaned = strings.TrimSuffix(cleaned, " ")
	return NewGridCell(colName, &cleaned)
}
