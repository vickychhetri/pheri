package ui

import (
	"database/sql"
	"fmt"
	"mysql-tui/phhistory"
	"strings"

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
	{
		Name:           "Dark",
		Bg:             tcell.ColorReset,
		SurfaceBg:      tcell.NewRGBColor(18, 20, 28),
		HeaderBg:       tcell.NewRGBColor(28, 32, 44),
		HeaderFg:       tcell.NewRGBColor(245, 247, 250),
		SelectedBg:     tcell.NewRGBColor(46, 52, 70),
		SelectedFg:     tcell.NewRGBColor(255, 255, 255),
		BorderInactive: tcell.NewRGBColor(50, 58, 76),
		BorderFocused:  tcell.NewRGBColor(96, 165, 250), // Vivid Sky Blue
		TitleInactive:  tcell.NewRGBColor(135, 145, 165),
		TitleFocused:   tcell.NewRGBColor(245, 247, 250),
		TextPrimary:    tcell.NewRGBColor(235, 240, 248),
		TextSecondary:  tcell.NewRGBColor(148, 163, 184),
		AccentPrimary:  tcell.NewRGBColor(96, 165, 250),
		AccentSuccess:  tcell.NewRGBColor(52, 211, 153),
		AccentWarning:  tcell.NewRGBColor(251, 191, 36),
		AccentDanger:   tcell.NewRGBColor(248, 113, 113),
		AccentInfo:     tcell.NewRGBColor(56, 189, 248),
	},
	{
		Name:           "Light",
		Bg:             tcell.ColorReset,
		SurfaceBg:      tcell.NewRGBColor(245, 247, 250),
		HeaderBg:       tcell.NewRGBColor(220, 226, 235),
		HeaderFg:       tcell.NewRGBColor(15, 23, 42),
		SelectedBg:     tcell.NewRGBColor(195, 210, 235),
		SelectedFg:     tcell.NewRGBColor(10, 20, 40),
		BorderInactive: tcell.NewRGBColor(180, 190, 205),
		BorderFocused:  tcell.NewRGBColor(25, 100, 230), // Crisp Royal Blue
		TitleInactive:  tcell.NewRGBColor(90, 105, 125),
		TitleFocused:   tcell.NewRGBColor(15, 23, 42),
		TextPrimary:    tcell.NewRGBColor(20, 30, 45),
		TextSecondary:  tcell.NewRGBColor(90, 105, 125),
		AccentPrimary:  tcell.NewRGBColor(25, 100, 230),
		AccentSuccess:  tcell.NewRGBColor(20, 140, 60),
		AccentWarning:  tcell.NewRGBColor(180, 95, 0),
		AccentDanger:   tcell.NewRGBColor(200, 30, 30),
		AccentInfo:     tcell.NewRGBColor(0, 120, 180),
	},
	{
		Name:           "Developer",
		Bg:             tcell.ColorReset,
		SurfaceBg:      tcell.NewRGBColor(34, 31, 38),
		HeaderBg:       tcell.NewRGBColor(45, 42, 50),
		HeaderFg:       tcell.NewRGBColor(252, 252, 250),
		SelectedBg:     tcell.NewRGBColor(64, 60, 72),
		SelectedFg:     tcell.NewRGBColor(255, 216, 102), // Monokai Amber
		BorderInactive: tcell.NewRGBColor(78, 74, 88),
		BorderFocused:  tcell.NewRGBColor(255, 97, 136), // Monokai Coral/Magenta
		TitleInactive:  tcell.NewRGBColor(147, 146, 150),
		TitleFocused:   tcell.NewRGBColor(255, 216, 102),
		TextPrimary:    tcell.NewRGBColor(248, 248, 242),
		TextSecondary:  tcell.NewRGBColor(147, 146, 150),
		AccentPrimary:  tcell.NewRGBColor(120, 220, 232),
		AccentSuccess:  tcell.NewRGBColor(169, 220, 103),
		AccentWarning:  tcell.NewRGBColor(255, 216, 102),
		AccentDanger:   tcell.NewRGBColor(255, 97, 136),
		AccentInfo:     tcell.NewRGBColor(171, 157, 242),
	},
	{
		Name:           "Hacker",
		Bg:             tcell.ColorReset,
		SurfaceBg:      tcell.NewRGBColor(5, 12, 6),
		HeaderBg:       tcell.NewRGBColor(12, 28, 14),
		HeaderFg:       tcell.NewRGBColor(0, 255, 100),
		SelectedBg:     tcell.NewRGBColor(0, 60, 25),
		SelectedFg:     tcell.NewRGBColor(255, 255, 255),
		BorderInactive: tcell.NewRGBColor(20, 60, 30),
		BorderFocused:  tcell.NewRGBColor(0, 255, 120), // Matrix Lime Phosphor
		TitleInactive:  tcell.NewRGBColor(50, 130, 65),
		TitleFocused:   tcell.NewRGBColor(0, 255, 120),
		TextPrimary:    tcell.NewRGBColor(0, 255, 100),
		TextSecondary:  tcell.NewRGBColor(60, 160, 80),
		AccentPrimary:  tcell.NewRGBColor(0, 255, 120),
		AccentSuccess:  tcell.NewRGBColor(0, 255, 80),
		AccentWarning:  tcell.NewRGBColor(180, 255, 0),
		AccentDanger:   tcell.NewRGBColor(255, 60, 60),
		AccentInfo:     tcell.NewRGBColor(40, 255, 200),
	},
	{
		Name:           "Classic",
		Bg:             tcell.ColorReset,
		SurfaceBg:      tcell.NewRGBColor(0, 0, 128), // Turbo Pascal / DOS Navy
		HeaderBg:       tcell.NewRGBColor(0, 0, 168),
		HeaderFg:       tcell.NewRGBColor(255, 255, 255),
		SelectedBg:     tcell.NewRGBColor(0, 168, 168), // Retro Cyan Highlight
		SelectedFg:     tcell.NewRGBColor(0, 0, 0),
		BorderInactive: tcell.NewRGBColor(85, 85, 255),
		BorderFocused:  tcell.NewRGBColor(255, 255, 85), // Classic Yellow Border
		TitleInactive:  tcell.NewRGBColor(170, 170, 255),
		TitleFocused:   tcell.NewRGBColor(255, 255, 85),
		TextPrimary:    tcell.NewRGBColor(255, 255, 255),
		TextSecondary:  tcell.NewRGBColor(170, 170, 255),
		AccentPrimary:  tcell.NewRGBColor(85, 255, 255),
		AccentSuccess:  tcell.NewRGBColor(85, 255, 85),
		AccentWarning:  tcell.NewRGBColor(255, 255, 85),
		AccentDanger:   tcell.NewRGBColor(255, 85, 85),
		AccentInfo:     tcell.NewRGBColor(255, 85, 255),
	},
	{
		Name:           "Gen-Z",
		Bg:             tcell.ColorReset,
		SurfaceBg:      tcell.NewRGBColor(24, 15, 42), // Cyber Synthwave Ultraviolet
		HeaderBg:       tcell.NewRGBColor(40, 22, 68),
		HeaderFg:       tcell.NewRGBColor(255, 42, 133), // Hot Neon Pink
		SelectedBg:     tcell.NewRGBColor(78, 30, 115),
		SelectedFg:     tcell.NewRGBColor(0, 240, 255), // Fluorescent Cyan
		BorderInactive: tcell.NewRGBColor(88, 45, 126),
		BorderFocused:  tcell.NewRGBColor(0, 240, 255),
		TitleInactive:  tcell.NewRGBColor(170, 120, 210),
		TitleFocused:   tcell.NewRGBColor(255, 42, 133),
		TextPrimary:    tcell.NewRGBColor(240, 230, 255),
		TextSecondary:  tcell.NewRGBColor(160, 130, 195),
		AccentPrimary:  tcell.NewRGBColor(0, 240, 255),
		AccentSuccess:  tcell.NewRGBColor(57, 255, 176),
		AccentWarning:  tcell.NewRGBColor(255, 222, 89),
		AccentDanger:   tcell.NewRGBColor(255, 42, 109),
		AccentInfo:     tcell.NewRGBColor(185, 103, 255),
	},
}

// CurrentTheme is the active theme, defaulting to Nordic Frost or restored from saved config.
var CurrentTheme = AvailableThemes[0]

// ActiveScreen tracks the current major screen: "login", "dblist", or "browser".
var ActiveScreen = "login"

// ActiveLoginDB holds reference to the current SQL DB instance.
var ActiveLoginDB *sql.DB

// SchemaSearchInput holds reference to the sidebar search input.
var SchemaSearchInput *tview.InputField

func init() {
	savedTheme := phhistory.GetThemePreference()
	if savedTheme != "" {
		for _, t := range AvailableThemes {
			if strings.EqualFold(t.Name, savedTheme) {
				CurrentTheme = t
				break
			}
		}
	}
}

// ApplyThemeGlobally applies the selected theme across all active UI components,
// updates global tview defaults, and persists the choice to disk.
func ApplyThemeGlobally(app *tview.Application, theme UITheme) {
	CurrentTheme = theme
	_ = phhistory.SaveThemePreference(theme.Name)

	// 1. Update tview global style defaults
	tview.Styles.PrimitiveBackgroundColor = CurrentTheme.SurfaceBg
	tview.Styles.ContrastBackgroundColor = CurrentTheme.HeaderBg
	tview.Styles.MoreContrastBackgroundColor = CurrentTheme.SelectedBg
	tview.Styles.BorderColor = CurrentTheme.BorderInactive
	tview.Styles.TitleColor = CurrentTheme.TitleInactive
	tview.Styles.GraphicsColor = CurrentTheme.BorderFocused
	tview.Styles.PrimaryTextColor = CurrentTheme.TextPrimary
	tview.Styles.SecondaryTextColor = CurrentTheme.TextSecondary
	tview.Styles.TertiaryTextColor = CurrentTheme.AccentPrimary
	tview.Styles.InverseTextColor = CurrentTheme.SelectedFg

	// 2. Refresh SQL editor
	if activeSQLEditor != nil && activeSQLEditor.TextArea != nil {
		activeSQLEditor.ApplyTheme()
	}

	// 3. Refresh schema objects list
	if tableList != nil {
		RefreshTableListTheme(tableList)
	}

	// 4. Refresh database switcher list
	if dataBaseList != nil {
		RefreshDatabaseListTheme(dataBaseList)
	}

	// 5. Refresh sidebar search input
	if SchemaSearchInput != nil {
		SchemaSearchInput.SetFieldBackgroundColor(CurrentTheme.SurfaceBg).
			SetFieldTextColor(CurrentTheme.TextPrimary).
			SetLabelColor(CurrentTheme.AccentPrimary).
			SetPlaceholderTextColor(CurrentTheme.TextSecondary)
	}

	// 6. Refresh data table and cell styles
	if dataTable != nil {
		RefreshTableTheme(dataTable)
	}

	// 7. Refresh footer and command input
	if footerBar != nil {
		footerBar.SetBackgroundColor(CurrentTheme.HeaderBg)
	}
	if commandInput != nil {
		commandInput.SetLabelColor(CurrentTheme.BorderFocused)
	}
}

// RefreshTableTheme re-styles the entire Table, including background, borders, and all cells.
func RefreshTableTheme(table *tview.Table) {
	if table == nil {
		return
	}
	table.SetBackgroundColor(CurrentTheme.SurfaceBg)
	StyleTable(table)
	StyleBox(table.Box, "Query Results", table.HasFocus())

	rowCount := table.GetRowCount()
	colCount := table.GetColumnCount()
	headerStyle := tcell.StyleDefault.
		Background(CurrentTheme.HeaderBg).
		Foreground(CurrentTheme.HeaderFg).
		Bold(true)

	for r := 0; r < rowCount; r++ {
		for c := 0; c < colCount; c++ {
			cell := table.GetCell(r, c)
			if cell == nil {
				continue
			}
			if r == 0 {
				cell.SetStyle(headerStyle)
			} else {
				cellStyle := tcell.StyleDefault.
					Foreground(CurrentTheme.TextPrimary).
					Background(CurrentTheme.SurfaceBg)
				if (r+1)%2 == 0 {
					cellStyle = cellStyle.Foreground(CurrentTheme.TextSecondary)
				}
				cell.SetStyle(cellStyle)
			}
		}
	}
}

// RefreshTableListTheme re-styles the schema objects list.
func RefreshTableListTheme(list *tview.List) {
	if list == nil {
		return
	}
	list.SetBackgroundColor(CurrentTheme.SurfaceBg)
	StyleList(list)
	StyleBox(list.Box, "Schema Objects", list.HasFocus())
}

// RefreshDatabaseListTheme re-styles the database switcher list.
func RefreshDatabaseListTheme(list *tview.List) {
	if list == nil {
		return
	}
	list.SetBackgroundColor(CurrentTheme.SurfaceBg)
	StyleList(list)
	StyleBox(list.Box, "Databases", list.HasFocus())
}

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

// FormatSchemaObjectBadge returns an aligned, color-coded badge for schema objects.
func FormatSchemaObjectBadge(objectType string) string {
	switch strings.ToUpper(strings.TrimSpace(objectType)) {
	case "TABLE":
		return "[aqua::b]TABLE[-::-]"
	case "VIEW":
		return "[purple::b]VIEW [-::-]"
	case "PROCEDURE", "PROC":
		return "[yellow::b]PROC [-::-]"
	case "FUNCTION", "FUNC":
		return "[lime::b]FUNC [-::-]"
	case "TRIGGER", "TRIG":
		return "[red::b]TRIG [-::-]"
	case "EVENT":
		return "[blue::b]EVENT[-::-]"
	default:
		return fmt.Sprintf("[gray::b]%-5s[-::-]", objectType)
	}
}
