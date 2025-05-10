package ui

import (
	"fmt"
	"strings"
	"time"

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
	Clipboard     string
	topRow        int
}

func NewSQLEditor(app *tview.Application) *SQLEditor {
	editor := tview.NewTextView()
	editor.
		SetDynamicColors(true).
		SetRegions(true).
		SetScrollable(true).
		SetBorder(true).
		SetTitle("SQL Editor (Blinking Cursor, Nav, Copy/Paste, Scroll)")

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
	lines := strings.Split(s.Text, "\n")
	switch event.Key() {
	case tcell.KeyRune:
		// Insert rune at cursor
		line := lines[s.CursorRow]
		runes := []rune(line)
		prefix := string(runes[:s.CursorCol])
		suffix := string(runes[s.CursorCol:])
		prefix += string(event.Rune())
		lines[s.CursorRow] = prefix + suffix
		s.CursorCol++

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if s.CursorCol > 0 {
			line := lines[s.CursorRow]
			runes := []rune(line)
			runes = append(runes[:s.CursorCol-1], runes[s.CursorCol:]...)
			lines[s.CursorRow] = string(runes)
			s.CursorCol--
		} else if s.CursorRow > 0 {
			// Merge with previous line
			prev := lines[s.CursorRow-1]
			curr := lines[s.CursorRow]
			lines[s.CursorRow-1] = prev + curr
			// remove current
			lines = append(lines[:s.CursorRow], lines[s.CursorRow+1:]...)
			s.CursorRow--
			s.CursorCol = len([]rune(prev))
		}

	case tcell.KeyEnter:
		// Split line
		line := lines[s.CursorRow]
		runes := []rune(line)
		prefix := string(runes[:s.CursorCol])
		suffix := string(runes[s.CursorCol:])
		lines[s.CursorRow] = prefix
		// insert new line
		newLines := append(lines[:s.CursorRow+1], append([]string{suffix}, lines[s.CursorRow+1:]...)...)
		lines = newLines
		s.CursorRow++
		s.CursorCol = 0
	case tcell.KeyLeft:
		if s.CursorCol > 0 {
			s.CursorCol--
		} else if s.CursorRow > 0 {
			s.CursorRow--
			s.CursorCol = len([]rune(lines[s.CursorRow]))
		}

	case tcell.KeyRight:
		if s.CursorCol < len([]rune(lines[s.CursorRow])) {
			s.CursorCol++
		} else if s.CursorRow < len(lines)-1 {
			s.CursorRow++
			s.CursorCol = 0
		}

	case tcell.KeyUp:
		if s.CursorRow > 0 {
			s.CursorRow--
			if s.CursorCol > len([]rune(lines[s.CursorRow])) {
				s.CursorCol = len([]rune(lines[s.CursorRow]))
			}
		}

	case tcell.KeyDown:
		if s.CursorRow < len(lines)-1 {
			s.CursorRow++
			if s.CursorCol > len([]rune(lines[s.CursorRow])) {
				s.CursorCol = len([]rune(lines[s.CursorRow]))
			}
		}

	case tcell.KeyPgUp:
		// Scroll up one screen
		x, y := s.Editor.GetScrollOffset()
		newY := y - s.getScreenHeight()
		if newY < 0 {
			newY = 0
		}
		s.Editor.ScrollTo(x, newY)

	case tcell.KeyPgDn:
		// Scroll down one screen
		x, y := s.Editor.GetScrollOffset()
		newY := y + s.getScreenHeight()
		s.Editor.ScrollTo(x, newY)

	case tcell.KeyHome:
		// Go to top
		s.Editor.ScrollTo(0, 0)

	case tcell.KeyEnd:
		// Go to bottom
		// _, totalLines := s.Editor.GetLineCount()
		// s.Editor.ScrollTo(0, totalLines)
		_, h := s.getScreenDims()
		totalLines := len(strings.Split(s.Text, "\n"))
		s.Editor.ScrollTo(0, totalLines-h)

	case tcell.KeyCtrlA:
		// Copy all to clipboard
		s.Clipboard = s.Text

	case tcell.KeyCtrlC:
		// Paste clipboard
		if s.Clipboard != "" {
			line := lines[s.CursorRow]
			runes := []rune(line)
			prefix := string(runes[:s.CursorCol])
			suffix := string(runes[s.CursorCol:])
			prefix += s.Clipboard
			lines[s.CursorRow] = prefix + suffix
			s.CursorCol += len([]rune(s.Clipboard))
		}

	case tcell.KeyCtrlV:
		// Clear clipboard
		s.Clipboard = ""

	case tcell.KeyCtrlQ:
		s.CursorEnabled = false
		s.App.Stop()
		return nil
	}

	// Update underlying text and view
	s.Text = strings.Join(lines, "\n")
	s.adjustViewport()
	s.updateText()
	return nil
}

func (s *SQLEditor) getScreenDims() (int, int) {
	x, _ := s.Editor.GetScrollOffset()
	_, h, _, _ := s.Editor.GetInnerRect()
	return x, h
}

func (s *SQLEditor) updateText() {
	lines := strings.Split(s.Text, "\n")
	var content strings.Builder

	for i, line := range lines {
		// Line number
		content.WriteString("[gray]")
		content.WriteString(padLeft(i+1, 3))
		content.WriteString(" | [-]")

		// If cursor on this line, insert at col
		if s.ShowCursor && i == s.CursorRow {
			runes := []rune(line)
			if s.CursorCol > len(runes) {
				s.CursorCol = len(runes)
			}
			before := string(runes[:s.CursorCol])
			after := string(runes[s.CursorCol:])
			content.WriteString(highlightSQL(before))
			content.WriteString("[white::b]|[-:-:-]")
			content.WriteString(highlightSQL(after))
		} else {
			content.WriteString(highlightSQL(line))
		}

		if i < len(lines)-1 {
			content.WriteString("\n")
		}
	}

	s.Editor.SetText(content.String())
	// Ensure cursor is visible
	s.Editor.ScrollTo(0, s.topRow)
}

func (s *SQLEditor) getScreenHeight() int {
	_, h, _, _ := s.Editor.GetInnerRect()
	return h
}

func padLeft(n, width int) string {
	s := fmt.Sprintf("%d", n)
	for len(s) < width {
		s = " " + s
	}
	return s
}

// GetText returns raw SQL (without highlights or cursor).
func (s *SQLEditor) GetText() string {
	return s.Text
}

// SetText sets new raw SQL content and updates view.
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

// Highlight function
func highlightSQL(text string) string {
	words := strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t'
	})

	highlighted := ""
	i := 0
	for _, word := range words {
		prefix := text[:strings.Index(text[i:], word)+i]
		highlighted += prefix
		i += len(prefix)

		switch strings.ToUpper(word) {
		case "SELECT", "FROM", "WHERE", "INSERT", "UPDATE", "DELETE":
			highlighted += "[blue::b]" + word + "[-:-:-]"
		case "AND", "OR", "NOT":
			highlighted += "[red::b]" + word + "[-:-:-]"
		default:
			highlighted += word
		}
		text = text[i+len(word):]
		i = 0
	}
	highlighted += text
	return highlighted
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
