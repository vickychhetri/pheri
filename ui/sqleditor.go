package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type SQLEditor struct {
	App           *tview.Application
	Editor        *tview.TextView
	Text          string
	ShowCursor    bool
	CursorTicker  *time.Ticker
	Container     *tview.Flex
	CursorEnabled bool
	CursorRow     int
	CursorCol     int
	topRow        int
	OnExecute     func(query string)
	OnNextFocus   func()
	OnExit        func()
	OnFullscreen  func()
}

func NewSQLEditor(app *tview.Application) *SQLEditor {
	editor := tview.NewTextView()
	editor.
		SetDynamicColors(true).
		SetRegions(true).
		SetScrollable(true).
		SetBorder(true).
		SetTitle(" [black:lime:b] > SQL code ✖ [white:-:-]  📝 Pheri Code Editor [white](Ctrl+R: Run | Ctrl+S: Snippets | Tab: Switch) ")

	editor.SetWrap(false)
	editor.SetBorderColor(tcell.ColorYellow).
		SetTitleColor(tcell.ColorYellow)

	editor.SetFocusFunc(func() {
		editor.SetBorderColor(tcell.ColorYellow).
			SetTitleColor(tcell.ColorYellow)
	})

	editor.SetBlurFunc(func() {
		editor.SetBorderColor(tcell.ColorDarkGray).
			SetTitleColor(tcell.ColorGray)
	})

	editor.SetChangedFunc(func() {
		app.Draw()
	})

	sqlEditor := &SQLEditor{
		App:           app,
		Editor:        editor,
		Container:     tview.NewFlex().SetDirection(tview.FlexRow).AddItem(editor, 0, 1, true),
		CursorEnabled: true,
	}

	editor.SetInputCapture(sqlEditor.handleInput)
	sqlEditor.startCursorBlink()
	sqlEditor.updateText()

	return sqlEditor
}

func (s *SQLEditor) handleInput(event *tcell.EventKey) *tcell.EventKey {
	if s.Text == "" {
		s.Text = ""
	}
	lines := strings.Split(s.Text, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	if s.CursorRow >= len(lines) {
		s.CursorRow = len(lines) - 1
	}

	handled := true

	switch event.Key() {
	case tcell.KeyRune:
		r := event.Rune()
		line := lines[s.CursorRow]
		runes := []rune(line)
		if s.CursorCol > len(runes) {
			s.CursorCol = len(runes)
		}
		prefix := string(runes[:s.CursorCol])
		suffix := string(runes[s.CursorCol:])

		// Auto-closing quotes & brackets
		switch r {
		case '\'':
			lines[s.CursorRow] = prefix + "''" + suffix
			s.CursorCol++
		case '"':
			lines[s.CursorRow] = prefix + "\"\"" + suffix
			s.CursorCol++
		case '`':
			lines[s.CursorRow] = prefix + "``" + suffix
			s.CursorCol++
		case '(':
			lines[s.CursorRow] = prefix + "()" + suffix
			s.CursorCol++
		default:
			lines[s.CursorRow] = prefix + string(r) + suffix
			s.CursorCol++
		}

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if s.CursorCol > 0 {
			line := lines[s.CursorRow]
			runes := []rune(line)
			if s.CursorCol <= len(runes) {
				runes = append(runes[:s.CursorCol-1], runes[s.CursorCol:]...)
				lines[s.CursorRow] = string(runes)
				s.CursorCol--
			}
		} else if s.CursorRow > 0 {
			prev := lines[s.CursorRow-1]
			curr := lines[s.CursorRow]
			lines[s.CursorRow-1] = prev + curr
			lines = append(lines[:s.CursorRow], lines[s.CursorRow+1:]...)
			s.CursorRow--
			s.CursorCol = len([]rune(prev))
		}

	case tcell.KeyEnter:
		line := lines[s.CursorRow]
		runes := []rune(line)
		if s.CursorCol > len(runes) {
			s.CursorCol = len(runes)
		}
		prefix := string(runes[:s.CursorCol])
		suffix := string(runes[s.CursorCol:])

		// Smart auto-indentation (preserve leading spaces)
		indent := ""
		for _, char := range prefix {
			if char == ' ' || char == '\t' {
				indent += string(char)
			} else {
				break
			}
		}

		lines[s.CursorRow] = prefix
		newLines := append(lines[:s.CursorRow+1], append([]string{indent + suffix}, lines[s.CursorRow+1:]...)...)
		lines = newLines
		s.CursorRow++
		s.CursorCol = len([]rune(indent))

	case tcell.KeyLeft:
		if s.CursorCol > 0 {
			s.CursorCol--
		} else if s.CursorRow > 0 {
			s.CursorRow--
			s.CursorCol = len([]rune(lines[s.CursorRow]))
		}

	case tcell.KeyRight:
		lineRunes := []rune(lines[s.CursorRow])
		if s.CursorCol < len(lineRunes) {
			s.CursorCol++
		} else if s.CursorRow < len(lines)-1 {
			s.CursorRow++
			s.CursorCol = 0
		}

	case tcell.KeyUp:
		if s.CursorRow > 0 {
			s.CursorRow--
			lineRunes := []rune(lines[s.CursorRow])
			if s.CursorCol > len(lineRunes) {
				s.CursorCol = len(lineRunes)
			}
		}

	case tcell.KeyDown:
		if s.CursorRow < len(lines)-1 {
			s.CursorRow++
			lineRunes := []rune(lines[s.CursorRow])
			if s.CursorCol > len(lineRunes) {
				s.CursorCol = len(lineRunes)
			}
		}

	case tcell.KeyHome:
		s.CursorCol = 0

	case tcell.KeyEnd:
		s.CursorCol = len([]rune(lines[s.CursorRow]))

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

	case tcell.KeyCtrlR:
		if s.OnExecute != nil {
			s.OnExecute(s.Text)
		}
		return nil

	case tcell.KeyCtrlT:
		s.ShowTableSuggestionsModal()
		return nil

	case tcell.KeyCtrlS, tcell.KeyCtrlSpace:
		s.ShowSnippetsModal()
		return nil

	case tcell.KeyCtrlC:
		if s.Text != "" {
			clipboard.WriteAll(s.Text)
		}

	case tcell.KeyCtrlV:
		clipText, err := clipboard.ReadAll()
		if err == nil && clipText != "" {
			line := lines[s.CursorRow]
			runes := []rune(line)
			prefix := string(runes[:s.CursorCol])
			suffix := string(runes[s.CursorCol:])
			lines[s.CursorRow] = prefix + clipText + suffix
			s.CursorCol += len([]rune(clipText))
		}

	default:
		handled = false
	}

	s.Text = strings.Join(lines, "\n")
	s.adjustViewport()
	s.updateText()

	if handled {
		return nil
	}
	return event
}

func (s *SQLEditor) ShowTableSuggestionsModal() {
	if s.Text == "" {
		s.Text = ""
	}
	lines := strings.Split(s.Text, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	if s.CursorRow >= len(lines) {
		s.CursorRow = len(lines) - 1
	}

	currentLine := lines[s.CursorRow]
	runes := []rune(currentLine)
	if s.CursorCol > len(runes) {
		s.CursorCol = len(runes)
	}
	prefix := string(runes[:s.CursorCol])
	after := string(runes[s.CursorCol:])

	words := strings.FieldsFunc(prefix, func(r rune) bool {
		return unicode.IsSpace(r) || r == ',' || r == ';' || r == '(' || r == ')' || r == '`'
	})

	currentWord := ""
	if len(words) > 0 {
		currentWord = words[len(words)-1]
	}

	var matches []string
	tables := allTables
	for _, obj := range tables {
		if currentWord == "" || strings.HasPrefix(strings.ToUpper(obj.Name), strings.ToUpper(currentWord)) {
			matches = append(matches, obj.Name)
		}
	}

	if len(matches) == 0 {
		for _, obj := range tables {
			matches = append(matches, obj.Name)
		}
	}

	if len(matches) == 0 {
		return
	}

	list := tview.NewList()
	list.ShowSecondaryText(false).
		SetBorder(true).
		SetTitle(fmt.Sprintf(" 🧮 Table Name Suggestions (Ctrl+T) "))

	list.SetBorderColor(tcell.ColorYellow).
		SetTitleColor(tcell.ColorYellow)

	wordToReplace := currentWord
	for _, tableName := range matches {
		tName := tableName
		list.AddItem("  📋 "+tName, "", 0, func() {
			before := prefix
			if wordToReplace != "" && strings.HasSuffix(before, wordToReplace) {
				before = strings.TrimSuffix(before, wordToReplace)
			}
			newLine := before + tName + after
			lines[s.CursorRow] = newLine
			s.Text = strings.Join(lines, "\n")
			s.CursorCol = len([]rune(before + tName))
			s.adjustViewport()
			s.updateText()

			if mainFlex != nil {
				layout := CreateLayoutWithFooter(s.App, mainFlex)
				s.App.SetRoot(layout, true)
			} else {
				s.App.SetRoot(s.Container, true)
			}
			s.App.SetFocus(s.Editor)
		})
	}

	list.AddItem("  ✖ Cancel", "", 'c', func() {
		if mainFlex != nil {
			layout := CreateLayoutWithFooter(s.App, mainFlex)
			s.App.SetRoot(layout, true)
		} else {
			s.App.SetRoot(s.Container, true)
		}
		s.App.SetFocus(s.Editor)
	})

	s.App.SetRoot(list, true).SetFocus(list)
}

func (s *SQLEditor) ShowSnippetsModal() {
	snippets := []struct {
		Label string
		Query string
	}{
		{"SELECT * FROM table LIMIT 100", "SELECT * FROM `table_name` LIMIT 100;"},
		{"SELECT WITH WHERE & ORDER", "SELECT `column1`, `column2` FROM `table_name` WHERE `id` = 1 ORDER BY `id` DESC;"},
		{"INSERT INTO table", "INSERT INTO `table_name` (`col1`, `col2`) VALUES ('val1', 'val2');"},
		{"UPDATE table", "UPDATE `table_name` SET `col1` = 'val1' WHERE `id` = 1;"},
		{"DELETE FROM table", "DELETE FROM `table_name` WHERE `id` = 1;"},
		{"INNER JOIN tables", "SELECT a.*, b.* FROM `table1` a INNER JOIN `table2` b ON a.`id` = b.`foreign_id`;"},
		{"CREATE TABLE template", "CREATE TABLE `new_table` (\n  `id` INT AUTO_INCREMENT PRIMARY KEY,\n  `name` VARCHAR(100) NOT NULL,\n  `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP\n);"},
	}

	list := tview.NewList()
	list.SetBorder(true).
		SetTitle(" 💡 Insert SQL Snippet / Template (Ctrl+S) ").
		SetBorderColor(tcell.ColorYellow).
		SetTitleColor(tcell.ColorYellow)

	for _, snip := range snippets {
		q := snip.Query
		list.AddItem("  ⚡ "+snip.Label, q, 0, func() {
			s.SetText(q)
			if mainFlex != nil {
				layout := CreateLayoutWithFooter(s.App, mainFlex)
				s.App.SetRoot(layout, true)
			} else {
				s.App.SetRoot(s.Container, true)
			}
			s.App.SetFocus(s.Editor)
		})
	}
	list.AddItem("  ✖ Cancel", "Close snippets window", 'c', func() {
		if mainFlex != nil {
			layout := CreateLayoutWithFooter(s.App, mainFlex)
			s.App.SetRoot(layout, true)
		} else {
			s.App.SetRoot(s.Container, true)
		}
		s.App.SetFocus(s.Editor)
	})

	s.App.SetRoot(list, true).SetFocus(list)
}

func (s *SQLEditor) updateText() {
	lines := strings.Split(s.Text, "\n")
	var content strings.Builder

	for i, line := range lines {
		isCurrentRow := i == s.CursorRow
		if isCurrentRow {
			content.WriteString("[lime::b]> [cyan]")
		} else {
			content.WriteString("  [gray]")
		}
		content.WriteString(fmt.Sprintf("%03d | [-]", i+1))

		highlightedLine := highlightSQLLine(line, s.CursorCol, isCurrentRow, s.ShowCursor)
		content.WriteString(highlightedLine)

		if i < len(lines)-1 {
			content.WriteString("\n")
		}
	}

	s.Editor.SetText(content.String())
	title := fmt.Sprintf(" [black:lime:b] > SQL code ✖ [white:-:-]  📝 Pheri Code Editor [white]| Ln %d, Col %d | %d chars [lime](Ctrl+R: Run | Ctrl+S: Snippets) ",
		s.CursorRow+1, s.CursorCol+1, len(s.Text))
	s.Editor.SetTitle(title)
	s.Editor.ScrollTo(0, s.topRow)
}

func (s *SQLEditor) GetText() string {
	return s.Text
}

func (s *SQLEditor) SetText(newText string) {
	s.Text = newText
	s.CursorRow = 0
	s.CursorCol = 0
	s.updateText()
}

func (s *SQLEditor) startCursorBlink() {
	s.CursorTicker = time.NewTicker(500 * time.Millisecond)
	go func() {
		for s.CursorEnabled {
			<-s.CursorTicker.C
			s.ShowCursor = !s.ShowCursor
			s.App.QueueUpdateDraw(func() {
				s.updateText()
			})
		}
	}()
}

func (s *SQLEditor) getScreenHeight() int {
	_, h, _, _ := s.Editor.GetInnerRect()
	if h <= 0 {
		return 10
	}
	return h
}

func (s *SQLEditor) adjustViewport() {
	screenHeight := s.getScreenHeight()
	if s.CursorRow < s.topRow {
		s.topRow = s.CursorRow
	} else if s.CursorRow >= s.topRow+screenHeight {
		s.topRow = s.CursorRow - screenHeight + 1
	}
	s.Editor.ScrollTo(0, s.topRow)
}

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

func highlightSQLLine(line string, cursorCol int, isCursorRow bool, showCursor bool) string {
	runes := []rune(line)
	totalRunes := len(runes)
	if cursorCol > totalRunes {
		cursorCol = totalRunes
	}

	var sb strings.Builder
	currentRuneIdx := 0

	tokens := tokenizeSQLLine(line)

	for _, tok := range tokens {
		tokRunes := []rune(tok.Text)
		tokLen := len(tokRunes)

		styleStart, styleEnd := getStyleForToken(tok.Kind)

		for i, r := range tokRunes {
			runePos := currentRuneIdx + i
			if isCursorRow && showCursor && runePos == cursorCol {
				if styleEnd != "" {
					sb.WriteString(styleEnd)
				}
				sb.WriteString("[white::b]|[-:-:-]")
				if styleStart != "" {
					sb.WriteString(styleStart)
				}
			}
			if i == 0 && styleStart != "" {
				sb.WriteString(styleStart)
			}
			charStr := string(r)
			if charStr == "[" {
				charStr = "[["
			}
			sb.WriteString(charStr)
			if i == tokLen-1 && styleEnd != "" {
				sb.WriteString(styleEnd)
			}
		}

		currentRuneIdx += tokLen
	}

	if isCursorRow && showCursor && currentRuneIdx == cursorCol {
		sb.WriteString("[white::b]|[-:-:-]")
	}

	return sb.String()
}

func tokenizeSQLLine(line string) []Token {
	runes := []rune(line)
	n := len(runes)
	var tokens []Token
	i := 0

	for i < n {
		r := runes[i]

		// 1. Single-line comments (-- or #)
		if (r == '-' && i+1 < n && runes[i+1] == '-') || r == '#' {
			tokens = append(tokens, Token{Text: string(runes[i:]), Kind: KindComment})
			break
		}

		// 2. Multi-line comment segment /* ... */
		if r == '/' && i+1 < n && runes[i+1] == '*' {
			start := i
			i += 2
			for i < n {
				if runes[i] == '*' && i+1 < n && runes[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			tokens = append(tokens, Token{Text: string(runes[start:i]), Kind: KindComment})
			continue
		}

		// 3. Strings ('...' or "...")
		if r == '\'' || r == '"' {
			quote := r
			start := i
			i++
			for i < n {
				if runes[i] == quote && runes[i-1] != '\\' {
					i++
					break
				}
				i++
			}
			tokens = append(tokens, Token{Text: string(runes[start:i]), Kind: KindString})
			continue
		}

		// 4. Backtick Identifiers (`...`)
		if r == '`' {
			start := i
			i++
			for i < n {
				if runes[i] == '`' {
					i++
					break
				}
				i++
			}
			tokens = append(tokens, Token{Text: string(runes[start:i]), Kind: KindIdentifier})
			continue
		}

		// 5. Whitespace
		if isSpace(r) {
			start := i
			for i < n && isSpace(runes[i]) {
				i++
			}
			tokens = append(tokens, Token{Text: string(runes[start:i]), Kind: KindWhitespace})
			continue
		}

		// 6. Numbers
		if isDigit(r) {
			start := i
			for i < n && (isDigit(runes[i]) || runes[i] == '.') {
				i++
			}
			tokens = append(tokens, Token{Text: string(runes[start:i]), Kind: KindNumber})
			continue
		}

		// 7. Words (Keywords, Functions, Data Types, Identifiers)
		if isLetter(r) || r == '_' {
			start := i
			for i < n && (isLetter(runes[i]) || isDigit(runes[i]) || runes[i] == '_') {
				i++
			}
			word := string(runes[start:i])
			upper := strings.ToUpper(word)

			kind := KindWord
			if isSQLKeyword(upper) {
				kind = KindKeyword
			} else if isSQLFunction(upper) {
				kind = KindFunction
			} else if isSQLType(upper) {
				kind = KindType
			}

			tokens = append(tokens, Token{Text: word, Kind: kind})
			continue
		}

		// 8. Operators and Punctuation
		tokens = append(tokens, Token{Text: string(r), Kind: KindOperator})
		i++
	}

	return tokens
}

func getStyleForToken(kind TokenKind) (string, string) {
	switch kind {
	case KindKeyword:
		return "[cyan::b]", "[-:-:-]" // Bright Cyan Bold (SELECT, FROM, WHERE)
	case KindFunction:
		return "[yellow::b]", "[-:-:-]" // Bright Yellow Bold (COUNT, SUM, CONCAT)
	case KindType:
		return "[lime::b]", "[-:-:-]" // Bright Lime Green Bold (INT, VARCHAR, TIMESTAMP)
	case KindString:
		return "[orange]", "[-]" // Bright Orange ('string', "text")
	case KindIdentifier:
		return "[fuchsia]", "[-]" // Bright Fuchsia (`table`, `col`)
	case KindNumber:
		return "[green]", "[-]" // Bright Green (123, 45.67)
	case KindComment:
		return "[darkgray::i]", "[-:-:-]" // Dark Gray Italic (-- comment, # comment)
	case KindOperator:
		return "[white]", "[-]" // Bright White (=, >, <, ,, ;)
	default:
		return "", ""
	}
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
