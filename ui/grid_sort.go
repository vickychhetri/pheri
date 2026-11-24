// ui/grid_sort.go
package ui

import (
	"fmt"
	"mysql-tui/util"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// SortDirection represents the active sort order of a Data Grid column.
type SortDirection int

const (
	SortNone SortDirection = iota
	SortAsc
	SortDesc
)

// Next cycles the sort direction: Unsorted -> ASC -> DESC -> Unsorted.
func (d SortDirection) Next() SortDirection {
	switch d {
	case SortNone:
		return SortAsc
	case SortAsc:
		return SortDesc
	default:
		return SortNone
	}
}

// Indicator returns the compact visual directional symbol for column headers.
func (d SortDirection) Indicator() string {
	switch d {
	case SortAsc:
		return "▲"
	case SortDesc:
		return "▼"
	default:
		return ""
	}
}

// SQLKeyword returns "ASC" or "DESC" for SQL ORDER BY clauses, or empty string when unsorted.
func (d SortDirection) SQLKeyword() string {
	switch d {
	case SortAsc:
		return "ASC"
	case SortDesc:
		return "DESC"
	default:
		return ""
	}
}

// GridSortState tracks column sorting metadata and fallback original rows for the active Data Grid.
type GridSortState struct {
	mu                sync.Mutex
	Column            string
	Direction         SortDirection
	BaseQuery         string
	IsTableBrowse     bool
	OriginalResult    *QueryResult
	PendingRestoreRow int
	PendingRestoreCol int
}

var activeGridSort = &GridSortState{
	PendingRestoreRow: -1,
	PendingRestoreCol: -1,
}

// Reset clears active sorting state for a newly initiated query or table load.
func (s *GridSortState) Reset(baseQuery string, isTableBrowse bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Column = ""
	s.Direction = SortNone
	s.BaseQuery = strings.TrimSpace(baseQuery)
	s.IsTableBrowse = isTableBrowse
	s.OriginalResult = nil
	s.PendingRestoreRow = -1
	s.PendingRestoreCol = -1
}

// Toggle updates the sorted column and advances its sort direction.
// Selecting a different column always starts with SortAsc.
func (s *GridSortState) Toggle(colName string) (string, SortDirection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Column != colName {
		s.Column = colName
		s.Direction = SortAsc
	} else {
		s.Direction = s.Direction.Next()
		if s.Direction == SortNone {
			s.Column = ""
		}
	}
	return s.Column, s.Direction
}

// Snapshot returns the current sort column and direction safely.
func (s *GridSortState) Snapshot() (col string, dir SortDirection, baseQuery string, isBrowse bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Column, s.Direction, s.BaseQuery, s.IsTableBrowse
}

// BuildBrowseQuery constructs a safe parameterized-identifier SELECT query for table/view browsing
// with optional ORDER BY and LIMIT/OFFSET pagination.
func BuildBrowseQuery(dbName, objName, sortCol string, dir SortDirection, limit, offset int) string {
	var b strings.Builder
	b.WriteString("SELECT * FROM ")
	b.WriteString(util.QuoteIdentifier(dbName))
	b.WriteString(".")
	b.WriteString(util.QuoteIdentifier(objName))

	if sortCol != "" && dir != SortNone {
		b.WriteString(" ORDER BY ")
		b.WriteString(util.QuoteIdentifier(sortCol))
		b.WriteString(" ")
		b.WriteString(dir.SQLKeyword())
	}

	if limit > 0 {
		fmt.Fprintf(&b, " LIMIT %d", limit)
	}
	if offset > 0 {
		fmt.Fprintf(&b, " OFFSET %d", offset)
	}
	return b.String()
}

// ApplySortToSQLQuery rewrites a custom SELECT query to sort by sortCol in the given direction.
// Returns (rewrittenSQL, true) if server-side sorting is supported for the statement, or ("", false)
// for non-SELECT statements (e.g. SHOW, DESCRIBE, EXPLAIN) that should fall back to in-memory sorting.
func ApplySortToSQLQuery(baseQuery, sortCol string, dir SortDirection) (string, bool) {
	trimmed := strings.TrimSpace(baseQuery)
	trimmed = strings.TrimRight(trimmed, " \t\r\n;")
	if trimmed == "" {
		return "", false
	}

	// Only rewrite queries that return tabular SELECT results (excluding DML/SHOW/DESCRIBE/EXPLAIN).
	isMut, stmtType := ClassifySQLStatement(trimmed)
	if isMut {
		return "", false
	}
	upperType := strings.ToUpper(stmtType)
	if upperType != "SELECT" && upperType != "CTE/SELECT" {
		return "", false
	}

	if dir == SortNone || sortCol == "" {
		return trimmed, true
	}

	orderClause := fmt.Sprintf("ORDER BY %s %s", util.QuoteIdentifier(sortCol), dir.SQLKeyword())

	// If query contains top-level UNION/INTERSECT/EXCEPT, wrap it in a derived table cleanly.
	if hasTopLevelSetOperator(trimmed) {
		return fmt.Sprintf("SELECT * FROM (%s) AS _pheri_sort %s", trimmed, orderClause), true
	}

	orderStart, orderEnd, limitStart := findTopLevelOrderAndLimit(trimmed)
	if orderStart >= 0 {
		// Replace existing top-level ORDER BY clause while preserving preceding SELECT/WHERE and trailing LIMIT/OFFSET.
		before := strings.TrimRight(trimmed[:orderStart], " \t\r\n")
		after := strings.TrimLeft(trimmed[orderEnd:], " \t\r\n")
		if after != "" {
			return fmt.Sprintf("%s %s %s", before, orderClause, after), true
		}
		return fmt.Sprintf("%s %s", before, orderClause), true
	}

	if limitStart >= 0 {
		before := strings.TrimRight(trimmed[:limitStart], " \t\r\n")
		after := strings.TrimLeft(trimmed[limitStart:], " \t\r\n")
		return fmt.Sprintf("%s %s %s", before, orderClause, after), true
	}

	return fmt.Sprintf("%s %s", trimmed, orderClause), true
}

// sqlTokenScan iterates top-level SQL tokens outside string literals, quoted identifiers, and comments.
type sqlSpan struct {
	word  string
	start int
	end   int
}

func scanTopLevelWords(sqlText string) []sqlSpan {
	var spans []sqlSpan
	n := len(sqlText)
	depth := 0
	i := 0

	for i < n {
		ch := sqlText[i]

		// Line comment --
		if ch == '-' && i+1 < n && sqlText[i+1] == '-' {
			i += 2
			for i < n && sqlText[i] != '\n' {
				i++
			}
			continue
		}
		// Hash comment #
		if ch == '#' {
			i++
			for i < n && sqlText[i] != '\n' {
				i++
			}
			continue
		}
		// Block comment /* ... */
		if ch == '/' && i+1 < n && sqlText[i+1] == '*' {
			i += 2
			for i+1 < n && !(sqlText[i] == '*' && sqlText[i+1] == '/') {
				i++
			}
			i += 2
			continue
		}
		// Quoted string or identifier ('...', "...", `...`)
		if ch == '\'' || ch == '"' || ch == '`' {
			quote := ch
			i++
			for i < n {
				if sqlText[i] == '\\' && quote != '`' && i+1 < n {
					i += 2
					continue
				}
				if sqlText[i] == quote {
					if i+1 < n && sqlText[i+1] == quote {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			continue
		}
		if ch == '(' {
			depth++
			i++
			continue
		}
		if ch == ')' {
			if depth > 0 {
				depth--
			}
			i++
			continue
		}

		if depth == 0 && isWordChar(ch) {
			start := i
			for i < n && isWordChar(sqlText[i]) {
				i++
			}
			spans = append(spans, sqlSpan{
				word:  strings.ToUpper(sqlText[start:i]),
				start: start,
				end:   i,
			})
			continue
		}
		i++
	}
	return spans
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

func hasTopLevelSetOperator(sqlText string) bool {
	for _, sp := range scanTopLevelWords(sqlText) {
		if sp.word == "UNION" || sp.word == "INTERSECT" || sp.word == "EXCEPT" {
			return true
		}
	}
	return false
}

func findTopLevelOrderAndLimit(sqlText string) (orderStart, orderEnd, limitStart int) {
	spans := scanTopLevelWords(sqlText)
	orderStart, orderEnd, limitStart = -1, -1, -1

	for idx := 0; idx < len(spans); idx++ {
		w := spans[idx].word
		if w == "ORDER" && idx+1 < len(spans) && spans[idx+1].word == "BY" {
			orderStart = spans[idx].start
			// Look ahead for trailing clause terminating ORDER BY
			orderEnd = len(sqlText)
			for j := idx + 2; j < len(spans); j++ {
				jw := spans[j].word
				if jw == "LIMIT" || jw == "OFFSET" || jw == "FOR" || jw == "LOCK" || jw == "INTO" {
					orderEnd = spans[j].start
					if jw == "LIMIT" && limitStart < 0 {
						limitStart = spans[j].start
					}
					break
				}
			}
			return orderStart, orderEnd, limitStart
		}
		if (w == "LIMIT" || w == "FOR" || w == "LOCK" || w == "INTO") && limitStart < 0 {
			limitStart = spans[idx].start
		}
	}
	return orderStart, orderEnd, limitStart
}

// cloneQueryResultRows deep-copies Columns, Rows, and RawRows for safe non-destructive in-memory sorting.
func cloneQueryResultForSort(res *QueryResult) *QueryResult {
	if res == nil {
		return nil
	}
	copied := *res
	copied.Columns = append([]string(nil), res.Columns...)
	if res.ForeignKeys != nil {
		copied.ForeignKeys = make(map[string]util.ForeignKeyRef, len(res.ForeignKeys))
		for k, v := range res.ForeignKeys {
			copied.ForeignKeys[k] = v
		}
	}
	copied.Rows = make([][]string, len(res.Rows))
	for i, row := range res.Rows {
		copied.Rows[i] = append([]string(nil), row...)
	}
	if res.RawRows != nil {
		copied.RawRows = make([][]*string, len(res.RawRows))
		for i, rawRow := range res.RawRows {
			copied.RawRows[i] = make([]*string, len(rawRow))
			for j, ptr := range rawRow {
				if ptr != nil {
					v := *ptr
					copied.RawRows[i][j] = &v
				}
			}
		}
	}
	return &copied
}

// SortQueryResultInMemory returns a stably sorted copy of res ordered by column colIdx.
// NULL values follow MySQL ordering (NULLs first on ASC, last on DESC) and numeric values compare numerically.
func SortQueryResultInMemory(res *QueryResult, colIdx int, dir SortDirection) *QueryResult {
	if res == nil || colIdx < 0 || colIdx >= len(res.Columns) || dir == SortNone {
		return cloneQueryResultForSort(res)
	}

	sorted := cloneQueryResultForSort(res)
	indices := make([]int, len(sorted.Rows))
	for i := range indices {
		indices[i] = i
	}

	sort.SliceStable(indices, func(i, j int) bool {
		idxA, idxB := indices[i], indices[j]
		aNull, bNull := isCellNull(sorted, idxA, colIdx), isCellNull(sorted, idxB, colIdx)
		if aNull && bNull {
			return false
		}
		if aNull != bNull {
			if dir == SortAsc {
				return aNull
			}
			return !aNull
		}

		valA := cellSortValue(sorted, idxA, colIdx)
		valB := cellSortValue(sorted, idxB, colIdx)

		cmp := compareGridValues(valA, valB)
		if dir == SortDesc {
			return cmp > 0
		}
		return cmp < 0
	})

	newRows := make([][]string, len(sorted.Rows))
	var newRawRows [][]*string
	if sorted.RawRows != nil {
		newRawRows = make([][]*string, len(sorted.RawRows))
	}
	for pos, origIdx := range indices {
		newRows[pos] = sorted.Rows[origIdx]
		if newRawRows != nil && origIdx < len(sorted.RawRows) {
			newRawRows[pos] = sorted.RawRows[origIdx]
		}
	}
	sorted.Rows = newRows
	if newRawRows != nil {
		sorted.RawRows = newRawRows
	}
	return sorted
}

func isCellNull(res *QueryResult, rowIdx, colIdx int) bool {
	if res.RawRows != nil && rowIdx < len(res.RawRows) && colIdx < len(res.RawRows[rowIdx]) {
		return res.RawRows[rowIdx][colIdx] == nil
	}
	if rowIdx < len(res.Rows) && colIdx < len(res.Rows[rowIdx]) {
		return res.Rows[rowIdx][colIdx] == "NULL"
	}
	return true
}

func cellSortValue(res *QueryResult, rowIdx, colIdx int) string {
	if res.RawRows != nil && rowIdx < len(res.RawRows) && colIdx < len(res.RawRows[rowIdx]) && res.RawRows[rowIdx][colIdx] != nil {
		return *res.RawRows[rowIdx][colIdx]
	}
	if rowIdx < len(res.Rows) && colIdx < len(res.Rows[rowIdx]) {
		return res.Rows[rowIdx][colIdx]
	}
	return ""
}

func compareGridValues(a, b string) int {
	numA, errA := strconv.ParseFloat(strings.TrimSpace(a), 64)
	numB, errB := strconv.ParseFloat(strings.TrimSpace(b), 64)
	if errA == nil && errB == nil {
		if numA < numB {
			return -1
		}
		if numA > numB {
			return 1
		}
		return 0
	}
	lowA := strings.ToLower(a)
	lowB := strings.ToLower(b)
	if lowA < lowB {
		return -1
	}
	if lowA > lowB {
		return 1
	}
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
