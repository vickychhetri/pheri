package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var commandInput *tview.InputField
var footer *tview.TextView

func CreateLayoutWithFooter(a *tview.Application, mainContent tview.Primitive) tview.Primitive {

	commandInput = tview.NewInputField()
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
				command := commandInput.GetText()
				if command != "" {
					commandInput.SetText("")
				}
				commandInput.SetFieldBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
				commandInput.SetFieldTextColor(tview.Styles.PrimaryTextColor)
				commandInput.SetLabelColor(tview.Styles.PrimaryTextColor)
				a.SetFocus(mainContent)
			}
		})

	footer = tview.NewTextView().
		SetTextAlign(tview.AlignRight).
		SetText("© 2025 Pheri - Terminal MySQL Client").
		SetTextColor(tview.Styles.SecondaryTextColor)

	footerLayoput := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(commandInput, 50, 0, false).
		AddItem(footer, 0, 1, false)

	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(mainContent, 0, 1, true).
		AddItem(footerLayoput, 1, 0, false)
	return layout
}
