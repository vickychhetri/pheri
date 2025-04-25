package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var commandInput *tview.InputField
var footer *tview.TextView

// CreateLayoutWithFooter wraps the given main content with a footer layout.
func CreateLayoutWithFooter(a *tview.Application, mainContent tview.Primitive) tview.Primitive {

	commandInput := tview.NewInputField()
	commandInput.
		SetLabel("Command: ").
		SetFieldWidth(30).
		SetPlaceholder("Enter command here...").
		SetFieldBackgroundColor(tview.Styles.PrimitiveBackgroundColor).
		SetFieldTextColor(tview.Styles.PrimaryTextColor).
		SetLabelColor(tview.Styles.PrimaryTextColor).
		SetDoneFunc(func(key tcell.Key) {
			switch key {
			case tcell.KeyEnter:
				// Handle command input here
				command := commandInput.GetText()
				if command != "" {
					// Process the command (e.g., execute it against the database)
					// For now, just print it to the console
					// In a real application, you would execute the command and update the main content accordingly
					commandInput.SetText("") // Clear the input field after processing
				}
				commandInput.SetFieldBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
				commandInput.SetFieldTextColor(tview.Styles.PrimaryTextColor)
				commandInput.SetLabelColor(tview.Styles.PrimaryTextColor)
				a.SetFocus(mainContent)
			}
		})

	// Layout: Left Panel with Command Box + Right Panel with Content

	// Create footer text view
	footer := tview.NewTextView().
		SetTextAlign(tview.AlignRight).
		SetText("© 2025 Pheri - Terminal MySQL Client").
		SetTextColor(tview.Styles.SecondaryTextColor)

	footerLayoput := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(commandInput, 50, 0, false).
		AddItem(footer, 0, 1, false)

	// Combine everything in a vertical layout
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(mainContent, 0, 1, true).   // main content expands
		AddItem(footerLayoput, 1, 0, false) // fixed height footer

	// a.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
	// 	switch {
	// 	case event.Key() == tcell.KeyCtrlC:
	// 		a.SetFocus(commandInput)
	// 		commandInput.SetText("")
	// 		commandInput.SetFieldBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	// 		return nil
	// 	case event.Key() == tcell.KeyEsc:
	// 		commandInput.SetText("")
	// 		a.SetFocus(mainContent)
	// 		return nil
	// 	case event.Key() == tcell.KeyCtrlZ:
	// 		a.Stop()
	// 		os.Exit(0)
	// 	}
	// 	return event
	// })

	return layout
}
