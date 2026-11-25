package ui

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// lineNumberGutter renders synchronized 1-based line numbers and highlights the active cursor line
// along the left margin of SQLEditor.
type lineNumberGutter struct {
	*tview.Box
	editor *SQLEditor
}

// CountEditorLines returns the total number of logical lines (\n-delimited) in text (minimum 1).
func CountEditorLines(text string) int {
	if text == "" {
		return 1
	}
	return strings.Count(text, "\n") + 1
}

// CalculateGutterWidth computes the fixed column width needed for right-aligned line numbers and separator.
func CalculateGutterWidth(totalLines int) int {
	if totalLines < 1 {
		totalLines = 1
	}
	digits := len(strconv.Itoa(totalLines))
	if digits < 2 {
		digits = 2
	}
	// Format: " " + <digits> + " │ "
	return digits + 4
}

// FormatGutterLine formats a single gutter row string right-aligned to numDigits width.
func FormatGutterLine(lineNum, totalLines, numDigits int) string {
	if numDigits < 2 {
		numDigits = 2
	}
	if lineNum < 1 || lineNum > totalLines {
		return fmt.Sprintf(" %*s │ ", numDigits, "")
	}
	return fmt.Sprintf(" %*d │ ", numDigits, lineNum)
}

func newLineNumberGutter(editor *SQLEditor) *lineNumberGutter {
	g := &lineNumberGutter{
		Box:    tview.NewBox().SetBackgroundColor(CurrentTheme.SurfaceBg),
		editor: editor,
	}
	g.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if (action == tview.MouseLeftClick || action == tview.MouseLeftDown) && editor != nil && editor.App != nil && editor.TextArea != nil {
			editor.App.SetFocus(editor.TextArea)
			return tview.MouseConsumed, nil
		}
		return action, event
	})
	return g
}

// Draw renders the synchronized line number margin alongside TextArea.
func (g *lineNumberGutter) Draw(screen tcell.Screen) {
	g.Box.SetBackgroundColor(CurrentTheme.SurfaceBg)
	g.Box.DrawForSubclass(screen, g)

	if g.editor == nil || g.editor.TextArea == nil {
		return
	}

	x, y, width, height := g.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}

	rowOffset, _ := g.editor.TextArea.GetOffset()
	fromRow, _, toRow, _ := g.editor.TextArea.GetCursor()
	totalLines := CountEditorLines(g.editor.TextArea.GetText())
	numDigits := len(strconv.Itoa(totalLines))
	if numDigits < 2 {
		numDigits = 2
	}

	inactiveStyle := tcell.StyleDefault.
		Foreground(CurrentTheme.TextSecondary).
		Background(CurrentTheme.SurfaceBg)
	activeBg := CurrentTheme.HeaderBg
	activeStyle := tcell.StyleDefault.
		Foreground(CurrentTheme.AccentPrimary).
		Background(activeBg).
		Bold(true)
	sepInactiveStyle := tcell.StyleDefault.
		Foreground(CurrentTheme.BorderInactive).
		Background(CurrentTheme.SurfaceBg)
	sepActiveStyle := tcell.StyleDefault.
		Foreground(CurrentTheme.BorderFocused).
		Background(activeBg)

	for row := 0; row < height; row++ {
		lineNum := rowOffset + row + 1
		isActive := lineNum >= 1 && lineNum <= totalLines && (lineNum-1) >= fromRow && (lineNum-1) <= toRow
		text := FormatGutterLine(lineNum, totalLines, numDigits)

		lineStyle := inactiveStyle
		sepStyle := sepInactiveStyle
		if isActive {
			lineStyle = activeStyle
			sepStyle = sepActiveStyle
		}

		col := 0
		for _, r := range text {
			if col >= width {
				break
			}
			st := lineStyle
			if r == '│' {
				st = sepStyle
			}
			screen.SetContent(x+col, y+row, r, nil, st)
			col++
		}
		for col < width {
			screen.SetContent(x+col, y+row, ' ', nil, lineStyle)
			col++
		}
	}
}

// SQLEditor is a professional, high-performance interactive SQL query editor
// built on top of tview.TextArea with native multiline editing, synchronized line numbers,
// cursor navigation, undo/redo, system clipboard integration, autocomplete, and snippet templating.
type SQLEditor struct {
	App             *tview.Application
	TextArea        *tview.TextArea
	Editor          *tview.TextArea // Aliased to TextArea for backward compatibility
	Gutter          *lineNumberGutter
	Container       *tview.Flex
	IsFullscreen    bool
	OnExecute       func(query string)
	OnNextFocus     func()
	OnPrevFocus     func()
	OnExit          func()
	OnFullscreen    func()
	lastGutterWidth int
}

// NewSQLEditor creates and initializes a professional SQL query editor with a synchronized line numbers gutter.
func NewSQLEditor(app *tview.Application) *SQLEditor {
	textArea := tview.NewTextArea()
	textArea.
		SetPlaceholder("Type or paste SQL query here... (e.g. SELECT * FROM table LIMIT 100;)").
		SetWrap(false)
	textArea.SetBorder(false)

	// Professional theme styling using CurrentTheme
	textArea.SetTextStyle(tcell.StyleDefault.Foreground(CurrentTheme.TextPrimary).Background(CurrentTheme.SurfaceBg))
	textArea.SetSelectedStyle(tcell.StyleDefault.Foreground(CurrentTheme.SelectedFg).Background(CurrentTheme.SelectedBg))
	textArea.SetPlaceholderStyle(tcell.StyleDefault.Foreground(CurrentTheme.TextSecondary).Background(CurrentTheme.SurfaceBg))
	textArea.SetBackgroundColor(CurrentTheme.SurfaceBg)

	// Direct 2-way integration with the OS system clipboard
	textArea.SetClipboard(
		func(text string) {
			_ = clipboard.WriteAll(text)
		},
		func() string {
			text, err := clipboard.ReadAll()
			if err != nil {
				return ""
			}
			return text
		},
	)

	initialWidth := CalculateGutterWidth(1)
	container := tview.NewFlex().SetDirection(tview.FlexColumn)
	container.
		SetBorder(true).
		SetTitleAlign(tview.AlignLeft).
		SetBackgroundColor(CurrentTheme.SurfaceBg)

	sqlEditor := &SQLEditor{
		App:             app,
		TextArea:        textArea,
		Editor:          textArea,
		Container:       container,
		lastGutterWidth: initialWidth,
	}
	sqlEditor.Gutter = newLineNumberGutter(sqlEditor)

	container.
		AddItem(sqlEditor.Gutter, initialWidth, 0, false).
		AddItem(textArea, 0, 1, true)

	// Dynamic cursor telemetry, gutter width resizing, and unified border header
	syncEditorState := func(focused bool) {
		fromRow, fromCol, _, _ := textArea.GetCursor()
		text := textArea.GetText()
		textLen := len(text)
		totalLines := CountEditorLines(text)

		gutterWidth := CalculateGutterWidth(totalLines)
		if gutterWidth != sqlEditor.lastGutterWidth {
			sqlEditor.lastGutterWidth = gutterWidth
			container.ResizeItem(sqlEditor.Gutter, gutterWidth, 0)
		}

		stmtBadge := ""
		if stmtCount := len(SplitSQLStatements(text)); stmtCount > 1 {
			stmtBadge = fmt.Sprintf(" | %d stmts", stmtCount)
		}

		title := fmt.Sprintf(" [ SQL Query Editor ]  Ln %d, Col %d | %d lines | %d chars%s ",
			fromRow+1, fromCol+1, totalLines, textLen, stmtBadge)
		container.SetTitle(title)
		if focused {
			container.SetBorderColor(CurrentTheme.BorderFocused)
			container.SetTitleColor(CurrentTheme.TitleFocused)
		} else {
			container.SetBorderColor(CurrentTheme.BorderInactive)
			container.SetTitleColor(CurrentTheme.TitleInactive)
		}
	}

	textArea.SetMovedFunc(func() {
		syncEditorState(textArea.HasFocus())
	})

	textArea.SetChangedFunc(func() {
		syncEditorState(textArea.HasFocus())
	})

	textArea.SetFocusFunc(func() {
		syncEditorState(true)
	})

	textArea.SetBlurFunc(func() {
		syncEditorState(false)
	})

	textArea.SetInputCapture(sqlEditor.handleInput)
	syncEditorState(false)

	return sqlEditor
}

// ApplyTheme synchronizes the SQL editor, line-number gutter, and container palette with CurrentTheme.
func (s *SQLEditor) ApplyTheme() {
	if s.TextArea == nil {
		return
	}
	s.TextArea.SetTextStyle(tcell.StyleDefault.Foreground(CurrentTheme.TextPrimary).Background(CurrentTheme.SurfaceBg))
	s.TextArea.SetSelectedStyle(tcell.StyleDefault.Foreground(CurrentTheme.SelectedFg).Background(CurrentTheme.SelectedBg))
	s.TextArea.SetPlaceholderStyle(tcell.StyleDefault.Foreground(CurrentTheme.TextSecondary).Background(CurrentTheme.SurfaceBg))
	s.TextArea.SetBackgroundColor(CurrentTheme.SurfaceBg)

	if s.Gutter != nil {
		s.Gutter.SetBackgroundColor(CurrentTheme.SurfaceBg)
	}
	if s.Container != nil {
		s.Container.SetBackgroundColor(CurrentTheme.SurfaceBg)
		if s.TextArea.HasFocus() {
			s.Container.SetBorderColor(CurrentTheme.BorderFocused)
			s.Container.SetTitleColor(CurrentTheme.TitleFocused)
		} else {
			s.Container.SetBorderColor(CurrentTheme.BorderInactive)
			s.Container.SetTitleColor(CurrentTheme.TitleInactive)
		}
	}
}

// handleInput intercepts specialized hotkeys (execution, autocomplete, snippets, navigation)
// while allowing native typing, backspace, delete, arrows, undo/redo to pass through to TextArea.
func (s *SQLEditor) handleInput(event *tcell.EventKey) *tcell.EventKey {
	// 1. If an async query is actively executing, Ctrl+C / Esc aborts it
	if IsQueryRunning() {
		if event.Key() == tcell.KeyCtrlC || event.Key() == tcell.KeyEscape {
			CancelActiveQuery()
			return nil
		}
	}

	// 2. Query execution hotkeys:
	//    - Ctrl+R / F5: execute selected SQL or full batch script
	//    - Alt+Enter: execute only the SQL statement at cursor
	//    - Ctrl+Enter / Ctrl+J: commit when empty & pending TX, else execute selection or full batch
	if event.Key() == tcell.KeyCtrlR || event.Key() == tcell.KeyF5 {
		if s.OnExecute != nil {
			s.OnExecute(s.GetExecutableText())
		}
		return nil
	}

	if event.Key() == tcell.KeyEnter && (event.Modifiers()&tcell.ModAlt != 0) {
		if chunk, ok := s.GetStatementAtCursor(); ok && s.OnExecute != nil {
			s.OnExecute(chunk.SQL)
		}
		return nil
	}

	if (event.Key() == tcell.KeyEnter && (event.Modifiers()&tcell.ModCtrl != 0)) || event.Key() == tcell.KeyCtrlJ {
		if !IsAutoCommitMode() && GetUncommittedTxCount() > 0 && strings.TrimSpace(s.GetText()) == "" {
			if count, err := CommitActiveTx(s.App); err == nil && count > 0 {
				updateFooterText(fmt.Sprintf("Committed %d mutation(s) to database.", count))
			}
			return nil
		}
		if s.OnExecute != nil {
			s.OnExecute(s.GetExecutableText())
		}
		return nil
	}

	switch event.Key() {
	case tcell.KeyF1:
		ShowComprehensiveHelpModal(s.App)
		return nil

	case tcell.KeyF10:
		if count, err := CommitActiveTx(s.App); err == nil && count > 0 {
			updateFooterText(fmt.Sprintf("Committed %d mutation(s) to database.", count))
		}
		return nil

	case tcell.KeyF22: // Shift+F10
		if count, err := RollbackActiveTx(s.App); err == nil && count > 0 {
			updateFooterText(fmt.Sprintf("Rolled back %d mutation(s).", count))
		}
		return nil

	case tcell.KeyTab:
		if s.OnNextFocus != nil {
			s.OnNextFocus()
		}
		return nil

	case tcell.KeyBacktab:
		if s.OnPrevFocus != nil {
			s.OnPrevFocus()
		}
		return nil

	case tcell.KeyEscape:
		if s.OnExit != nil {
			s.OnExit()
		}
		return nil

	case tcell.KeyF11:
		if s.OnFullscreen != nil {
			s.OnFullscreen()
		}
		return nil

	case tcell.KeyCtrlT:
		s.ShowTableSuggestionsModal()
		return nil

	case tcell.KeyCtrlS, tcell.KeyCtrlSpace:
		s.ShowSnippetsModal()
		return nil

	case tcell.KeyCtrlK:
		s.Clear()
		updateFooterText("SQL Query cleared.")
		return nil

	case tcell.KeyCtrlC:
		if s.TextArea.HasSelection() {
			sel, _, _ := s.TextArea.GetSelection()
			_ = clipboard.WriteAll(sel)
			updateFooterText("Selected text copied to clipboard!")
			return nil
		}
		query := s.GetText()
		if query != "" {
			_ = clipboard.WriteAll(query)
			updateFooterText("SQL query copied to clipboard!")
		}
		return nil

	case tcell.KeyCtrlP:
		// Paste from OS clipboard directly at cursor
		clip, err := clipboard.ReadAll()
		if err == nil && clip != "" {
			_, start, end := s.TextArea.GetSelection()
			s.TextArea.Replace(start, end, clip)
		}
		return nil
	}

	// Return unhandled events to tview.TextArea's internal input handler
	// (handles typing runes, spaces, newlines, backspace, delete, arrows, undo/redo Ctrl+Z/Y)
	return event
}

// GetText returns the full current SQL text in the editor.
func (s *SQLEditor) GetText() string {
	if s.TextArea == nil {
		return ""
	}
	return s.TextArea.GetText()
}

// GetExecutableText returns the currently highlighted selection if non-empty,
// otherwise returning the full editor script for single or batch execution.
func (s *SQLEditor) GetExecutableText() string {
	if s == nil || s.TextArea == nil {
		return ""
	}
	if s.TextArea.HasSelection() {
		if sel, _, _ := s.TextArea.GetSelection(); strings.TrimSpace(sel) != "" {
			return sel
		}
	}
	return s.TextArea.GetText()
}

// GetStatementAtCursor resolves the individual SQL statement under or nearest the current cursor line.
func (s *SQLEditor) GetStatementAtCursor() (SQLStatementChunk, bool) {
	if s == nil || s.TextArea == nil {
		return SQLStatementChunk{}, false
	}
	chunks := SplitSQLStatements(s.TextArea.GetText())
	if len(chunks) == 0 {
		return SQLStatementChunk{}, false
	}
	row, _, _, _ := s.TextArea.GetCursor()
	cursorLine := row + 1
	for _, chunk := range chunks {
		if cursorLine >= chunk.StartLine && cursorLine <= chunk.EndLine {
			return chunk, true
		}
	}
	best := chunks[0]
	for _, chunk := range chunks {
		if chunk.StartLine <= cursorLine {
			best = chunk
		} else {
			break
		}
	}
	return best, true
}

// SetText sets the SQL text in the editor and places cursor at the end.
func (s *SQLEditor) SetText(newText string) {
	if s.TextArea != nil {
		s.TextArea.SetText(newText, true)
	}
}

// Clear empties the editor.
func (s *SQLEditor) Clear() {
	if s.TextArea != nil {
		s.TextArea.SetText("", false)
	}
}

// ShowTableSuggestionsModal opens an interactive table name selector and auto-inserts
// the chosen table name at the cursor position, replacing any partially typed word.
func (s *SQLEditor) ShowTableSuggestionsModal() {
	text := s.TextArea.GetText()

	// Get cursor position in text
	_, start, _ := s.TextArea.GetSelection()
	cursorIdx := start
	if cursorIdx > len(text) {
		cursorIdx = len(text)
	}

	// Find the current word before cursor to use as a filter
	prefix := text[:cursorIdx]
	wordStart := cursorIdx
	for wordStart > 0 {
		r := rune(prefix[wordStart-1])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '`' {
			wordStart--
		} else {
			break
		}
	}
	currentWord := strings.Trim(prefix[wordStart:cursorIdx], "`")

	var matches []string
	for _, obj := range allTables {
		if currentWord == "" || strings.HasPrefix(strings.ToUpper(obj.Name), strings.ToUpper(currentWord)) {
			matches = append(matches, obj.Name)
		}
	}
	if len(matches) == 0 {
		for _, obj := range allTables {
			matches = append(matches, obj.Name)
		}
	}
	if len(matches) == 0 {
		updateFooterText("No tables available in the active database.")
		return
	}

	list := tview.NewList()
	list.ShowSecondaryText(false)
	StyleModalBox(list.Box, fmt.Sprintf("Table Autocomplete [Filter: '%s']", currentWord))
	list.SetBackgroundColor(CurrentTheme.SurfaceBg)
	StyleList(list)

	restoreView := func() {
		if mainFlex != nil {
			layout := CreateLayoutWithFooter(s.App, mainFlex)
			s.App.SetRoot(layout, true)
		} else {
			s.App.SetRoot(s.Container, true)
		}
		s.App.SetFocus(s.TextArea)
	}

	for _, tableName := range matches {
		tName := tableName
		list.AddItem("  "+tName, "", 0, func() {
			restoreView()
			quotedTable := "`" + tName + "`"
			s.TextArea.Replace(wordStart, cursorIdx, quotedTable)
			s.App.SetFocus(s.TextArea)
		})
	}

	list.AddItem("  [ Cancel ]", "Press Esc or Enter to close", 'c', func() {
		restoreView()
	})

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			restoreView()
			return nil
		}
		return event
	})

	s.App.SetRoot(list, true).SetFocus(list)
}

// ShowSnippetsModal opens a gallery of standard SQL snippets and query templates.
func (s *SQLEditor) ShowSnippetsModal() {
	snippets := []struct {
		Label string
		Query string
	}{
		{"SELECT * FROM table LIMIT 100", "SELECT * FROM `table_name` LIMIT 100;"},
		{"SELECT WITH WHERE & ORDER", "SELECT `id`, `name` FROM `table_name` WHERE `status` = 'active' ORDER BY `id` DESC LIMIT 50;"},
		{"COUNT / GROUP BY Aggregation", "SELECT `category`, COUNT(*) AS `total` FROM `table_name` GROUP BY `category` HAVING `total` > 1;"},
		{"INSERT INTO table", "INSERT INTO `table_name` (`col1`, `col2`) VALUES ('val1', 'val2');"},
		{"UPDATE table", "UPDATE `table_name` SET `col1` = 'new_val' WHERE `id` = 1;"},
		{"DELETE FROM table", "DELETE FROM `table_name` WHERE `id` = 1;"},
		{"INNER JOIN two tables", "SELECT a.*, b.* FROM `table1` a INNER JOIN `table2` b ON a.`id` = b.`foreign_id` LIMIT 100;"},
		{"EXPLAIN ANALYZE query", "EXPLAIN FORMAT=TREE SELECT * FROM `table_name` WHERE `id` = 1;"},
		{"CREATE TABLE template", "CREATE TABLE `new_table` (\n  `id` INT AUTO_INCREMENT PRIMARY KEY,\n  `name` VARCHAR(100) NOT NULL,\n  `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP\n);"},
	}

	list := tview.NewList()
	StyleModalBox(list.Box, "SQL Query Snippets (Ctrl+S)")
	list.SetBackgroundColor(CurrentTheme.SurfaceBg)
	StyleList(list)

	restoreView := func() {
		if mainFlex != nil {
			layout := CreateLayoutWithFooter(s.App, mainFlex)
			s.App.SetRoot(layout, true)
		} else {
			s.App.SetRoot(s.Container, true)
		}
		s.App.SetFocus(s.TextArea)
	}

	for _, snip := range snippets {
		q := snip.Query
		list.AddItem("  "+snip.Label, q, 0, func() {
			restoreView()
			if strings.TrimSpace(s.GetText()) == "" {
				s.SetText(q)
			} else {
				_, start, end := s.TextArea.GetSelection()
				s.TextArea.Replace(start, end, q)
			}
			s.App.SetFocus(s.TextArea)
		})
	}

	list.AddItem("  [ Cancel ]", "Close snippets window", 'c', func() {
		restoreView()
	})

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			restoreView()
			return nil
		}
		return event
	})

	s.App.SetRoot(list, true).SetFocus(list)
}

// Token and SQL helper definitions preserved for AST and SQL keyword utilities
type TokenKind int

const (
	KindWhitespace TokenKind = iota
	KindKeyword
	KindFunction
	KindType
	KindString
	KindIdentifier
	KindNumber
	KindComment
	KindOperator
	KindWord
)

type Token struct {
	Text string
	Kind TokenKind
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\r' || r == '\n'
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isSQLKeyword(word string) bool {
	keywords := map[string]bool{
		"SELECT": true, "FROM": true, "WHERE": true, "INSERT": true, "UPDATE": true, "DELETE": true,
		"JOIN": true, "INNER": true, "LEFT": true, "RIGHT": true, "FULL": true, "OUTER": true, "CROSS": true,
		"ON": true, "USING": true, "GROUP": true, "BY": true, "ORDER": true, "HAVING": true, "LIMIT": true,
		"OFFSET": true, "UNION": true, "ALL": true, "DISTINCT": true, "AS": true, "AND": true, "OR": true,
		"NOT": true, "IN": true, "IS": true, "NULL": true, "LIKE": true, "ILIKE": true, "BETWEEN": true,
		"EXISTS": true, "CASE": true, "WHEN": true, "THEN": true, "ELSE": true, "END": true, "CAST": true,
		"CREATE": true, "ALTER": true, "DROP": true, "TRUNCATE": true, "TABLE": true, "VIEW": true,
		"INDEX": true, "DATABASE": true, "SCHEMA": true, "PROCEDURE": true, "FUNCTION": true, "TRIGGER": true,
		"EVENT": true, "GRANT": true, "REVOKE": true, "SHOW": true, "USE": true, "SET": true, "VALUES": true,
		"INTO": true, "PRIMARY": true, "KEY": true, "FOREIGN": true, "REFERENCES": true, "DEFAULT": true,
		"AUTO_INCREMENT": true, "UNIQUE": true, "CHECK": true, "CONSTRAINT": true, "IF": true, "ASC": true,
		"DESC": true, "ENGINE": true, "CHARSET": true, "COLLATE": true, "FOREIGN_KEY_CHECKS": true,
	}
	return keywords[word]
}

func isSQLFunction(word string) bool {
	funcs := map[string]bool{
		"COUNT": true, "SUM": true, "AVG": true, "MAX": true, "MIN": true, "CONCAT": true, "CONCAT_WS": true,
		"COALESCE": true, "IFNULL": true, "NULLIF": true, "NOW": true, "CURDATE": true, "CURTIME": true,
		"DATE_FORMAT": true, "DATEDIFF": true, "DATE_ADD": true, "DATE_SUB": true, "LOWER": true, "UPPER": true,
		"SUBSTRING": true, "SUBSTR": true, "TRIM": true, "LENGTH": true, "REPLACE": true, "ROUND": true,
		"FLOOR": true, "CEIL": true, "ABS": true, "JSON_EXTRACT": true, "JSON_ARRAY": true, "JSON_OBJECT": true,
		"JSON_UNQUOTE": true, "GROUP_CONCAT": true, "UUID": true, "MD5": true, "SHA1": true, "SHA2": true,
	}
	return funcs[word]
}

func isSQLType(word string) bool {
	types := map[string]bool{
		"INT": true, "INTEGER": true, "TINYINT": true, "SMALLINT": true, "MEDIUMINT": true, "BIGINT": true,
		"DECIMAL": true, "NUMERIC": true, "FLOAT": true, "DOUBLE": true, "BIT": true, "DATE": true,
		"DATETIME": true, "TIMESTAMP": true, "TIME": true, "YEAR": true, "CHAR": true, "VARCHAR": true,
		"TEXT": true, "TINYTEXT": true, "MEDIUMTEXT": true, "LONGTEXT": true, "BLOB": true, "TINYBLOB": true,
		"MEDIUMBLOB": true, "LONGBLOB": true, "ENUM": true, "SET": true, "BOOLEAN": true, "BOOL": true,
		"JSON": true, "VARBINARY": true, "BINARY": true,
	}
	return types[word]
}
