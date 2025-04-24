package ui

import (
	"github.com/rivo/tview"
)

// CreateLayoutWithFooter wraps the given main content with a footer layout.
func CreateLayoutWithFooter(mainContent tview.Primitive) tview.Primitive {
	// Create footer text view
	footer := tview.NewTextView().
		SetTextAlign(tview.AlignRight).
		SetText("© 2025 Pheri - Terminal MySQL Client").
		SetTextColor(tview.Styles.SecondaryTextColor)

	// Combine everything in a vertical layout
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(mainContent, 0, 1, true). // main content expands
		AddItem(footer, 1, 0, false)      // fixed height footer

	return layout
}
