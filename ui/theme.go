package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// UITheme defines a cohesive, production-grade terminal color palette.
type UITheme struct {
	Name           string
	Bg             tcell.Color
	SurfaceBg      tcell.Color
	HeaderBg       tcell.Color
	HeaderFg       tcell.Color
	SelectedBg     tcell.Color
	SelectedFg     tcell.Color
	BorderInactive tcell.Color
	BorderFocused  tcell.Color
	TitleInactive  tcell.Color
	TitleFocused   tcell.Color
	TextPrimary    tcell.Color
	TextSecondary  tcell.Color
	AccentPrimary  tcell.Color
	AccentSuccess  tcell.Color
	AccentWarning  tcell.Color
	AccentDanger   tcell.Color
	AccentInfo     tcell.Color
}

// AvailableThemes contains carefully curated, professional developer themes.
var AvailableThemes = []UITheme{
	{
		Name:           "Nordic Frost",
		Bg:             tcell.ColorReset,
		SurfaceBg:      tcell.NewRGBColor(24, 28, 36),
		HeaderBg:       tcell.NewRGBColor(46, 52, 64),
		HeaderFg:       tcell.NewRGBColor(236, 239, 244),
		SelectedBg:     tcell.NewRGBColor(59, 66, 82),
		SelectedFg:     tcell.NewRGBColor(248, 248, 252),
		BorderInactive: tcell.NewRGBColor(67, 76, 94),
		BorderFocused:  tcell.NewRGBColor(136, 192, 208), // Nordic Ice Blue
		TitleInactive:  tcell.NewRGBColor(140, 150, 168),
		TitleFocused:   tcell.NewRGBColor(236, 239, 244),
		TextPrimary:    tcell.NewRGBColor(229, 233, 240),
		TextSecondary:  tcell.NewRGBColor(129, 137, 152),
		AccentPrimary:  tcell.NewRGBColor(136, 192, 208),
		AccentSuccess:  tcell.NewRGBColor(163, 190, 140),
		AccentWarning:  tcell.NewRGBColor(235, 203, 139),
		AccentDanger:   tcell.NewRGBColor(191, 97, 106),
		AccentInfo:     tcell.NewRGBColor(143, 188, 187),
	},
	{
		Name:           "Tokyo Night",
		Bg:             tcell.ColorReset,
		SurfaceBg:      tcell.NewRGBColor(26, 27, 38),
		HeaderBg:       tcell.NewRGBColor(36, 40, 59),
		HeaderFg:       tcell.NewRGBColor(192, 202, 245),
		SelectedBg:     tcell.NewRGBColor(45, 53, 80),
		SelectedFg:     tcell.NewRGBColor(255, 255, 255),
		BorderInactive: tcell.NewRGBColor(65, 72, 104),
		BorderFocused:  tcell.NewRGBColor(122, 162, 247), // Soft Tokyo Blue
		TitleInactive:  tcell.NewRGBColor(120, 130, 160),
		TitleFocused:   tcell.NewRGBColor(192, 202, 245),
		TextPrimary:    tcell.NewRGBColor(192, 202, 245),
		TextSecondary:  tcell.NewRGBColor(100, 110, 140),
		AccentPrimary:  tcell.NewRGBColor(122, 162, 247),
		AccentSuccess:  tcell.NewRGBColor(158, 206, 106),
		AccentWarning:  tcell.NewRGBColor(224, 175, 104),
		AccentDanger:   tcell.NewRGBColor(247, 118, 142),
		AccentInfo:     tcell.NewRGBColor(125, 207, 255),
	},
	{
		Name:           "Catppuccin Mocha",
		Bg:             tcell.ColorReset,
		SurfaceBg:      tcell.NewRGBColor(24, 24, 37),
		HeaderBg:       tcell.NewRGBColor(49, 50, 68),
		HeaderFg:       tcell.NewRGBColor(205, 214, 244),
		SelectedBg:     tcell.NewRGBColor(69, 71, 90),
		SelectedFg:     tcell.NewRGBColor(245, 224, 220),
		BorderInactive: tcell.NewRGBColor(88, 91, 112),
		BorderFocused:  tcell.NewRGBColor(137, 180, 250), // Catppuccin Lavender/Sky
		TitleInactive:  tcell.NewRGBColor(147, 153, 178),
		TitleFocused:   tcell.NewRGBColor(245, 224, 220),
		TextPrimary:    tcell.NewRGBColor(205, 214, 244),
		TextSecondary:  tcell.NewRGBColor(147, 153, 178),
		AccentPrimary:  tcell.NewRGBColor(137, 180, 250),
		AccentSuccess:  tcell.NewRGBColor(166, 227, 161),
		AccentWarning:  tcell.NewRGBColor(249, 226, 175),
		AccentDanger:   tcell.NewRGBColor(243, 139, 168),
		AccentInfo:     tcell.NewRGBColor(137, 220, 235),
	},
	{
		Name:           "Charcoal Slate",
		Bg:             tcell.ColorReset,
		SurfaceBg:      tcell.NewRGBColor(20, 22, 26),
		HeaderBg:       tcell.NewRGBColor(38, 42, 50),
		HeaderFg:       tcell.NewRGBColor(240, 240, 240),
		SelectedBg:     tcell.NewRGBColor(55, 62, 74),
		SelectedFg:     tcell.NewRGBColor(255, 255, 255),
		BorderInactive: tcell.NewRGBColor(60, 66, 78),
		BorderFocused:  tcell.NewRGBColor(100, 160, 220),
		TitleInactive:  tcell.NewRGBColor(130, 138, 150),
		TitleFocused:   tcell.NewRGBColor(240, 240, 240),
		TextPrimary:    tcell.NewRGBColor(230, 230, 230),
		TextSecondary:  tcell.NewRGBColor(115, 122, 135),
		AccentPrimary:  tcell.NewRGBColor(100, 160, 220),
		AccentSuccess:  tcell.NewRGBColor(130, 195, 130),
		AccentWarning:  tcell.NewRGBColor(230, 185, 105),
		AccentDanger:   tcell.NewRGBColor(220, 100, 100),
		AccentInfo:     tcell.NewRGBColor(110, 190, 200),
	},
}

// CurrentTheme is the active theme, defaulting to Nordic Frost.
var CurrentTheme = AvailableThemes[0]

// StyleBox applies consistent, professional border and title styles to any tview Box.
func StyleBox(box *tview.Box, title string, focused bool) {
	if box == nil {
		return
	}
	box.SetBorder(true)
	if title != "" {
		box.SetTitle(fmt.Sprintf(" [ %s ] ", title))
		box.SetTitleAlign(tview.AlignLeft)
	}
	if focused {
		box.SetBorderColor(CurrentTheme.BorderFocused)
		box.SetTitleColor(CurrentTheme.TitleFocused)
	} else {
		box.SetBorderColor(CurrentTheme.BorderInactive)
		box.SetTitleColor(CurrentTheme.TitleInactive)
	}
}

// StyleList applies standard professional typography and selection styling to a tview.List.
func StyleList(list *tview.List) {
	if list == nil {
		return
	}
	list.SetMainTextColor(CurrentTheme.TextPrimary)
	list.SetSecondaryTextColor(CurrentTheme.TextSecondary)
	list.SetSelectedBackgroundColor(CurrentTheme.SelectedBg)
	list.SetSelectedTextColor(CurrentTheme.SelectedFg)
	list.SetHighlightFullLine(true)
}

// StyleTable applies standard professional styling to a tview.Table.
func StyleTable(table *tview.Table) {
	if table == nil {
		return
	}
	table.SetBorders(true)
	table.SetBordersColor(CurrentTheme.BorderInactive)
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(CurrentTheme.SelectedBg).
		Foreground(CurrentTheme.SelectedFg).
		Bold(true))
}

// StyleModalBox styles modal containers with clean, centered borders and calm palette.
func StyleModalBox(box *tview.Box, title string) {
	if box == nil {
		return
	}
	box.SetBorder(true)
	box.SetTitle(fmt.Sprintf(" [ %s ] ", title))
	box.SetTitleAlign(tview.AlignCenter)
	box.SetBorderColor(CurrentTheme.BorderFocused)
	box.SetTitleColor(CurrentTheme.TitleFocused)
}
