package ui

import (
	"fmt"
	"regexp"
	"strings"
	"time"

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
	OnExit        func()
}

func NewSQLEditor(app *tview.Application) *SQLEditor {
	editor := tview.NewTextView()
	editor.
		SetDynamicColors(true).
		SetRegions(true).
		SetScrollable(true).
		SetBorder(true).
		SetTitle(" 📝 HeidiSQL Code Editor [white](Ctrl+R: Run | F11: Fullscreen | Tab: Switch) ")

	editor.SetWrap(false)
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

	case tcell.KeyCtrlR:
		if s.OnExecute != nil {
			s.OnExecute(s.Text)
		}
		return nil

	case tcell.KeyCtrlS, tcell.KeyCtrlT:
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
	}

	s.Text = strings.Join(lines, "\n")
	s.adjustViewport()
	s.updateText()
	return event
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
		SetTitle(" 💡 Insert SQL Snippet (Ctrl+S / Ctrl+T) ").
		SetBorderColor(tcell.ColorDarkCyan)

	for _, snip := range snippets {
		q := snip.Query
		list.AddItem("  ⚡ "+snip.Label, q, 0, func() {
			s.SetText(q)
			s.App.SetRoot(s.Container, true)
		})
	}
	list.AddItem("  ✖ Cancel", "Close snippets window", 'c', func() {
		s.App.SetRoot(s.Container, true)
	})

	s.App.SetRoot(list, true).SetFocus(list)
}

func (s *SQLEditor) updateText() {
	lines := strings.Split(s.Text, "\n")
	var content strings.Builder

	for i, line := range lines {
		if i == s.CursorRow {
			content.WriteString("[lime::b]> [gray]")
		} else {
			content.WriteString("  [gray]")
		}
		content.WriteString(fmt.Sprintf("%03d | [-]", i+1))

		if s.ShowCursor && i == s.CursorRow {
			runes := []rune(line)
			if s.CursorCol > len(runes) {
				s.CursorCol = len(runes)
			}
			before := string(runes[:s.CursorCol])
			after := string(runes[s.CursorCol:])
			content.WriteString(highlightSQLHeidi(before))
			content.WriteString("[white::b]|[-:-:-]")
			content.WriteString(highlightSQLHeidi(after))
		} else {
			content.WriteString(highlightSQLHeidi(line))
		}

		if i < len(lines)-1 {
			content.WriteString("\n")
		}
	}

	s.Editor.SetText(content.String())
	title := fmt.Sprintf(" 📝 HeidiSQL Code Editor [white]| Ln %d, Col %d | %d chars [lime](Ctrl+R: Run | Ctrl+S: Snippets) ",
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

func highlightSQLHeidi(text string) string {
	if text == "" {
		return ""
	}

	if strings.HasPrefix(strings.TrimSpace(text), "--") {
		return "[#57a64a::i]" + text + "[-:-:-]"
	}

	re := regexp.MustCompile("(`[^`]*`|'[^']*'|\"[^\"]*\"|\\b[0-9]+(\\.[0-9]+)?\\b|\\b[a-zA-Z_][a-zA-Z0-9_]*\\b|[^a-zA-Z0-9_`'\"]+)")
	matches := re.FindAllString(text, -1)

	var sb strings.Builder
	for _, token := range matches {
		upper := strings.ToUpper(token)

		switch {
		case strings.HasPrefix(token, "`") && strings.HasSuffix(token, "`"):
			sb.WriteString("[#4ec9b0]" + token + "[-]")

		case (strings.HasPrefix(token, "'") && strings.HasSuffix(token, "'")) ||
			(strings.HasPrefix(token, "\"") && strings.HasSuffix(token, "\"")):
			sb.WriteString("[#6a9955]" + token + "[-]")

		case regexp.MustCompile("^\\d+(\\.\\d+)?$").MatchString(token):
			sb.WriteString("[#b5cea8]" + token + "[-]")

		case isSQLKeyword(upper):
			sb.WriteString("[#3399ff::b]" + token + "[-:-:-]")

		case isSQLFunctionOrType(upper):
			sb.WriteString("[#dcdcaa::b]" + token + "[-:-:-]")

		default:
			sb.WriteString(token)
		}
	}

	return sb.String()
}

func isSQLKeyword(word string) bool {
	keywords := map[string]bool{
		"SELECT": true, "FROM": true, "WHERE": true, "INSERT": true, "UPDATE": true, "DELETE": true,
		"JOIN": true, "INNER": true, "LEFT": true, "RIGHT": true, "ON": true, "GROUP": true,
		"ORDER": true, "BY": true, "LIMIT": true, "OFFSET": true, "HAVING": true, "UNION": true,
		"CREATE": true, "ALTER": true, "DROP": true, "TABLE": true, "VIEW": true, "INDEX": true,
		"SHOW": true, "USE": true, "SET": true, "VALUES": true, "INTO": true, "EXISTS": true,
		"BETWEEN": true, "LIKE": true, "IS": true, "NULL": true, "AND": true, "OR": true,
		"NOT": true, "AS": true, "CASE": true, "WHEN": true, "THEN": true, "ELSE": true,
		"END": true, "CAST": true, "DISTINCT": true, "ALL": true, "PRIMARY": true, "KEY": true,
		"FOREIGN": true, "REFERENCES": true, "DEFAULT": true, "AUTO_INCREMENT": true, "TRUNCATE": true,
	}
	return keywords[word]
}

func isSQLFunctionOrType(word string) bool {
	funcsAndTypes := map[string]bool{
		"COUNT": true, "SUM": true, "AVG": true, "MAX": true, "MIN": true, "CONCAT": true,
		"COALESCE": true, "NOW": true, "IFNULL": true, "ROUND": true, "LOWER": true, "UPPER": true,
		"DATE_FORMAT": true, "INT": true, "BIGINT": true, "VARCHAR": true, "CHAR": true, "TEXT": true,
		"DATE": true, "DATETIME": true, "TIMESTAMP": true, "DECIMAL": true, "BOOLEAN": true, "JSON": true,
	}
	return funcsAndTypes[word]
}
