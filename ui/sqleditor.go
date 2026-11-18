package ui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SQLEditor is a professional, high-performance interactive SQL query editor
// built on top of tview.TextArea with native multiline editing, cursor navigation,
// undo/redo, system clipboard integration, autocomplete, and snippet templating.
type SQLEditor struct {
	App          *tview.Application
	TextArea     *tview.TextArea
	Editor       *tview.TextArea // Aliased to TextArea for backward compatibility
	Container    *tview.Flex
	IsFullscreen bool
	OnExecute    func(query string)
	OnNextFocus  func()
	OnExit       func()
	OnFullscreen func()
}

// NewSQLEditor creates and initializes a professional SQL query editor.
func NewSQLEditor(app *tview.Application) *SQLEditor {
	textArea := tview.NewTextArea()
	textArea.
		SetPlaceholder("Type or paste SQL query here... (e.g. SELECT * FROM table LIMIT 100;)").
		SetBorder(true).
		SetTitleAlign(tview.AlignLeft)

	// High contrast, professional dark theme styling
	textArea.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack))
	textArea.SetSelectedStyle(tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorYellow))
	textArea.SetPlaceholderStyle(tcell.StyleDefault.Foreground(tcell.ColorDarkGray).Background(tcell.ColorBlack))

	textArea.SetBorderColor(tcell.ColorYellow).
		SetTitleColor(tcell.ColorYellow)

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

	sqlEditor := &SQLEditor{
		App:       app,
		TextArea:  textArea,
		Editor:    textArea,
		Container: tview.NewFlex().SetDirection(tview.FlexRow).AddItem(textArea, 0, 1, true),
	}

	// Dynamic cursor telemetry and status header
	updateTitle := func() {
		fromRow, fromCol, _, _ := textArea.GetCursor()
		textLen := textArea.GetTextLength()
		title := fmt.Sprintf(" 📝 [black:lime:b] SQL Query Editor [-:-:-] [white]| Ln %d, Col %d | %d chars [lime](Ctrl+R: Run | Ctrl+T: Tables | Ctrl+S: Snippets | Tab: Next | Esc: Back) ",
			fromRow+1, fromCol+1, textLen)
		textArea.SetTitle(title)
	}

	textArea.SetMovedFunc(func() {
		updateTitle()
	})

	textArea.SetChangedFunc(func() {
		updateTitle()
	})

	textArea.SetFocusFunc(func() {
		textArea.SetBorderColor(tcell.ColorYellow).
			SetTitleColor(tcell.ColorYellow)
		updateTitle()
	})

	textArea.SetBlurFunc(func() {
		textArea.SetBorderColor(tcell.ColorDarkGray).
			SetTitleColor(tcell.ColorGray)
	})

	textArea.SetInputCapture(sqlEditor.handleInput)
	updateTitle()

	return sqlEditor
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

	// 2. Query execution hotkeys: Ctrl+R, F5, Ctrl+Enter (received as KeyCtrlJ or ModCtrl+KeyEnter)
	if event.Key() == tcell.KeyCtrlR || event.Key() == tcell.KeyF5 ||
		(event.Key() == tcell.KeyEnter && (event.Modifiers()&tcell.ModCtrl != 0)) ||
		event.Key() == tcell.KeyCtrlJ {
		if s.OnExecute != nil {
			s.OnExecute(s.GetText())
		}
		return nil
	}

	switch event.Key() {
	case tcell.KeyTab:
		if s.OnNextFocus != nil {
			s.OnNextFocus()
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
	list.ShowSecondaryText(false).
		SetBorder(true).
		SetTitle(fmt.Sprintf(" 🧮 Table Name Suggestions (Ctrl+T) [Filter: '%s'] ", currentWord)).
		SetBorderColor(tcell.ColorYellow).
		SetTitleColor(tcell.ColorYellow)

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
		list.AddItem("  📋 "+tName, "", 0, func() {
			restoreView()
			quotedTable := "`" + tName + "`"
			s.TextArea.Replace(wordStart, cursorIdx, quotedTable)
			s.App.SetFocus(s.TextArea)
		})
	}

	list.AddItem("  ✖ Cancel", "Press Esc or Enter to close", 'c', func() {
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
	list.SetBorder(true).
		SetTitle(" 💡 SQL Snippet / Query Templates (Ctrl+S) ").
		SetBorderColor(tcell.ColorYellow).
		SetTitleColor(tcell.ColorYellow)

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
		list.AddItem("  ⚡ "+snip.Label, q, 0, func() {
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

	list.AddItem("  ✖ Cancel", "Close snippets window", 'c', func() {
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
