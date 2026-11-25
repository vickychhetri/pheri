package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/rivo/tview"
)

// SQLStatementChunk represents a single executable SQL statement extracted from a multi-statement script.
type SQLStatementChunk struct {
	Index         int    // 1-based sequence index in batch
	SQL           string // Trimmed SQL text (without trailing delimiter)
	StartLine     int    // 1-based line number where statement tokens begin
	EndLine       int    // 1-based line number where statement ends
	IsMutation    bool   // Classified via ClassifySQLStatement
	StatementType string // High-level statement classification (SELECT, INSERT, DDL, etc.)
}

// BatchStatementRecord stores the execution outcome for a single statement within a batch.
type BatchStatementRecord struct {
	Chunk        SQLStatementChunk
	Status       string // "OK", "ERROR", "CANCELED", "SKIPPED"
	Duration     time.Duration
	RowsFetched  int
	RowsAffected int64
	LastInsertID int64
	ErrorMessage string
	Result       *QueryResult
}

// BatchResultTab represents a renderable tab in the tabbed Results Data Grid strip.
type BatchResultTab struct {
	Title     string
	IsSummary bool
	StmtIndex int
	Result    *QueryResult
	RawSQL    string
}

// SplitSQLStatements splits a multi-statement SQL script into ordered executable chunks.
// It preserves string literals ('...', "..."), backtick identifiers (`...`), escaped quotes,
// SQL comments (--, #, /* ... */), line-number mappings, and client-side DELIMITER changes.
func SplitSQLStatements(script string) []SQLStatementChunk {
	if strings.TrimSpace(script) == "" {
		return nil
	}

	var chunks []SQLStatementChunk
	var buf strings.Builder
	delimiter := ";"

	line := 1
	firstTokenLine := 0
	lastTokenLine := 0

	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false

	emitChunk := func() {
		raw := strings.TrimSpace(buf.String())
		buf.Reset()
		if raw == "" {
			firstTokenLine = 0
			lastTokenLine = 0
			return
		}
		if StripLeadingSQLComments(raw) == "" {
			firstTokenLine = 0
			lastTokenLine = 0
			return
		}
		startLine := firstTokenLine
		if startLine <= 0 {
			startLine = 1
		}
		endLine := lastTokenLine
		if endLine < startLine {
			endLine = startLine
		}
		isMut, stmtType := ClassifySQLStatement(raw)
		chunks = append(chunks, SQLStatementChunk{
			Index:         len(chunks) + 1,
			SQL:           raw,
			StartLine:     startLine,
			EndLine:       endLine,
			IsMutation:    isMut,
			StatementType: stmtType,
		})
		firstTokenLine = 0
		lastTokenLine = 0
	}

	markSQLTokenLine := func(curLine int) {
		if firstTokenLine == 0 {
			firstTokenLine = curLine
		}
		lastTokenLine = curLine
	}

	n := len(script)
	i := 0
	for i < n {
		c := script[i]

		// Track client-side DELIMITER directive when starting a fresh statement buffer
		if !inSingleQuote && !inDoubleQuote && !inBacktick && !inLineComment && !inBlockComment && StripLeadingSQLComments(buf.String()) == "" {
			lineEnd := strings.IndexByte(script[i:], '\n')
			var lineSlice string
			if lineEnd == -1 {
				lineSlice = script[i:]
			} else {
				lineSlice = script[i : i+lineEnd]
			}
			if newDelim, ok := parseDelimiterDirective(lineSlice); ok {
				delimiter = newDelim
				buf.Reset()
				firstTokenLine = 0
				lastTokenLine = 0
				if lineEnd == -1 {
					break
				}
				i += lineEnd + 1
				line++
				continue
			}
		}

		// Handle active line comment
		if inLineComment {
			buf.WriteByte(c)
			if c == '\n' {
				inLineComment = false
				line++
			}
			i++
			continue
		}

		// Handle active block comment
		if inBlockComment {
			if c == '*' && i+1 < n && script[i+1] == '/' {
				buf.WriteString("*/")
				inBlockComment = false
				i += 2
				continue
			}
			buf.WriteByte(c)
			if c == '\n' {
				line++
			}
			i++
			continue
		}

		// Handle active single-quoted string ('...')
		if inSingleQuote {
			buf.WriteByte(c)
			if c == '\\' && i+1 < n {
				buf.WriteByte(script[i+1])
				if script[i+1] == '\n' {
					line++
				}
				lastTokenLine = line
				i += 2
				continue
			}
			if c == '\'' {
				if i+1 < n && script[i+1] == '\'' {
					buf.WriteByte('\'')
					i += 2
					continue
				}
				inSingleQuote = false
			} else if c == '\n' {
				line++
			}
			lastTokenLine = line
			i++
			continue
		}

		// Handle active double-quoted string ("...")
		if inDoubleQuote {
			buf.WriteByte(c)
			if c == '\\' && i+1 < n {
				buf.WriteByte(script[i+1])
				if script[i+1] == '\n' {
					line++
				}
				lastTokenLine = line
				i += 2
				continue
			}
			if c == '"' {
				if i+1 < n && script[i+1] == '"' {
					buf.WriteByte('"')
					i += 2
					continue
				}
				inDoubleQuote = false
			} else if c == '\n' {
				line++
			}
			lastTokenLine = line
			i++
			continue
		}

		// Handle active backtick identifier (`...`)
		if inBacktick {
			buf.WriteByte(c)
			if c == '`' {
				if i+1 < n && script[i+1] == '`' {
					buf.WriteByte('`')
					i += 2
					continue
				}
				inBacktick = false
			} else if c == '\n' {
				line++
			}
			lastTokenLine = line
			i++
			continue
		}

		// Check active delimiter at top level
		if delimiter != "" && strings.HasPrefix(script[i:], delimiter) {
			emitChunk()
			for k := 0; k < len(delimiter); k++ {
				if script[i+k] == '\n' {
					line++
				}
			}
			i += len(delimiter)
			continue
		}

		// Check comment openers
		if c == '-' && i+1 < n && script[i+1] == '-' {
			inLineComment = true
			buf.WriteString("--")
			i += 2
			continue
		}
		if c == '#' {
			inLineComment = true
			buf.WriteByte('#')
			i++
			continue
		}
		if c == '/' && i+1 < n && script[i+1] == '*' {
			inBlockComment = true
			// Preserve MySQL executable conditional comments /*! ... */ as token line start
			if i+2 < n && script[i+2] == '!' {
				markSQLTokenLine(line)
			}
			buf.WriteString("/*")
			i += 2
			continue
		}

		// Check quote / backtick openers
		if c == '\'' {
			inSingleQuote = true
			markSQLTokenLine(line)
			buf.WriteByte(c)
			i++
			continue
		}
		if c == '"' {
			inDoubleQuote = true
			markSQLTokenLine(line)
			buf.WriteByte(c)
			i++
			continue
		}
		if c == '`' {
			inBacktick = true
			markSQLTokenLine(line)
			buf.WriteByte(c)
			i++
			continue
		}

		buf.WriteByte(c)
		if !unicode.IsSpace(rune(c)) {
			markSQLTokenLine(line)
		}
		if c == '\n' {
			line++
		}
		i++
	}

	emitChunk()
	return chunks
}

// parseDelimiterDirective checks whether a trimmed line is a client-side `DELIMITER <token>` command.
func parseDelimiterDirective(lineText string) (string, bool) {
	trimmed := strings.TrimSpace(lineText)
	if len(trimmed) < len("DELIMITER")+2 {
		return "", false
	}
	fields := strings.Fields(trimmed)
	if len(fields) < 2 || !strings.EqualFold(fields[0], "DELIMITER") {
		return "", false
	}
	return fields[1], true
}

// ValidateBatchReadOnly checks whether any statement in a batch is a mutation when Read-Only mode is enabled.
func ValidateBatchReadOnly(chunks []SQLStatementChunk, readOnly bool) (SQLStatementChunk, bool) {
	if !readOnly {
		return SQLStatementChunk{}, false
	}
	for _, chunk := range chunks {
		if chunk.IsMutation {
			return chunk, true
		}
	}
	return SQLStatementChunk{}, false
}

// TruncateSQLPreview flattens whitespace and caps length for concise tab/grid display.
func TruncateSQLPreview(sqlText string, maxRunes int) string {
	fields := strings.Fields(StripLeadingSQLComments(sqlText))
	if len(fields) == 0 {
		fields = strings.Fields(sqlText)
	}
	joined := strings.Join(fields, " ")
	runes := []rune(joined)
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return joined
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

// BuildBatchResultTabs constructs tabbed result grids for multi-statement SELECT outputs
// followed by a structured Summary grid covering all executed, failed, canceled, and skipped statements.
func BuildBatchResultTabs(records []BatchStatementRecord, totalDuration time.Duration) ([]BatchResultTab, int) {
	var tabs []BatchResultTab

	okCount := 0
	errCount := 0
	canceledCount := 0

	for _, rec := range records {
		switch rec.Status {
		case "OK":
			okCount++
		case "ERROR":
			errCount++
		case "CANCELED":
			canceledCount++
		}

		if rec.Status == "OK" && rec.Result != nil && !rec.Result.IsMutation && len(rec.Result.Columns) > 0 {
			labelType := compactStatementType(rec.Chunk.StatementType)
			tabTitle := fmt.Sprintf("%d: %s (%d)", rec.Chunk.Index, labelType, rec.RowsFetched)
			tabs = append(tabs, BatchResultTab{
				Title:     tabTitle,
				IsSummary: false,
				StmtIndex: rec.Chunk.Index,
				Result:    rec.Result,
				RawSQL:    rec.Chunk.SQL,
			})
		}
	}

	summaryCols := []string{"#", "Line", "Type", "Status", "Rows / Affected", "Time", "SQL / Details"}
	summaryRows := make([][]string, 0, len(records))
	summaryRaw := make([][]*string, 0, len(records))

	for _, rec := range records {
		idxStr := strconv.Itoa(rec.Chunk.Index)
		lineStr := fmt.Sprintf("Ln %d", rec.Chunk.StartLine)
		typeStr := rec.Chunk.StatementType
		if typeStr == "" {
			typeStr = StmtUnknown
		}

		var styledStatus string
		switch rec.Status {
		case "OK":
			styledStatus = "[lime::b]OK[-::-]"
		case "ERROR":
			styledStatus = "[red::b]ERROR[-::-]"
		case "CANCELED":
			styledStatus = "[yellow::b]CANCELED[-::-]"
		default:
			styledStatus = "[gray]SKIPPED[-]"
		}

		var metricsStr string
		if rec.Status == "OK" {
			if rec.Chunk.IsMutation || (rec.Result != nil && rec.Result.IsMutation) {
				metricsStr = fmt.Sprintf("%d affected", rec.RowsAffected)
				if rec.LastInsertID > 0 {
					metricsStr += fmt.Sprintf(" (ID:%d)", rec.LastInsertID)
				}
			} else {
				metricsStr = fmt.Sprintf("%d rows", rec.RowsFetched)
			}
		} else {
			metricsStr = "-"
		}

		timeStr := "-"
		if rec.Status != "SKIPPED" {
			timeStr = rec.Duration.Round(time.Microsecond).String()
		}

		rawDetail := TruncateSQLPreview(rec.Chunk.SQL, 56)
		styledDetail := tview.Escape(rawDetail)
		if rec.ErrorMessage != "" {
			rawDetail = fmt.Sprintf("%s | %s", TruncateSQLPreview(rec.Chunk.SQL, 36), rec.ErrorMessage)
			styledDetail = fmt.Sprintf("[white]%s[-] [red]%s[-]",
				tview.Escape(TruncateSQLPreview(rec.Chunk.SQL, 36)),
				tview.Escape(rec.ErrorMessage))
		}

		summaryRows = append(summaryRows, []string{
			idxStr,
			lineStr,
			tview.Escape(typeStr),
			styledStatus,
			tview.Escape(metricsStr),
			tview.Escape(timeStr),
			styledDetail,
		})

		rawRow := make([]*string, len(summaryCols))
		rawVals := []string{idxStr, lineStr, typeStr, rec.Status, metricsStr, timeStr, rawDetail}
		for i := range rawVals {
			v := rawVals[i]
			rawRow[i] = &v
		}
		summaryRaw = append(summaryRaw, rawRow)
	}

	summaryLabel := fmt.Sprintf("Summary (%d/%d OK)", okCount, len(records))
	if errCount > 0 {
		summaryLabel = fmt.Sprintf("Summary (%d ERR)", errCount)
	} else if canceledCount > 0 {
		summaryLabel = "Summary (Canceled)"
	}

	summaryResult := &QueryResult{
		Columns:  summaryCols,
		Rows:     summaryRows,
		RawRows:  summaryRaw,
		Duration: totalDuration,
		RowCount: len(records),
	}

	summaryIdx := len(tabs)
	tabs = append(tabs, BatchResultTab{
		Title:     summaryLabel,
		IsSummary: true,
		StmtIndex: 0,
		Result:    summaryResult,
	})

	// Default to Summary when errors/cancellations happened or when there are no SELECT tabs.
	defaultTabIdx := 0
	if errCount > 0 || canceledCount > 0 || summaryIdx == 0 {
		defaultTabIdx = summaryIdx
	}

	return tabs, defaultTabIdx
}

func compactStatementType(stmtType string) string {
	upper := strings.ToUpper(strings.TrimSpace(stmtType))
	if upper == "" {
		return "QUERY"
	}
	if strings.HasPrefix(upper, "CTE (") && strings.HasSuffix(upper, ")") {
		return strings.TrimSuffix(strings.TrimPrefix(upper, "CTE ("), ")")
	}
	return upper
}
