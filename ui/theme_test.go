// ui/theme_test.go
package ui

import (
	"strings"
	"testing"

	"github.com/rivo/tview"
)

func TestAvailableThemes_IncludesRequestedThemes(t *testing.T) {
	requiredThemes := []string{
		"Nordic Frost",
		"Tokyo Night",
		"Catppuccin Mocha",
		"Charcoal Slate",
		"Dark",
		"Light",
		"Developer",
		"Hacker",
		"Classic",
		"Gen-Z",
	}

	themeMap := make(map[string]UITheme)
	for _, theme := range AvailableThemes {
		if strings.TrimSpace(theme.Name) == "" {
			t.Errorf("found theme with empty name")
		}
		themeMap[strings.ToLower(theme.Name)] = theme
	}

	for _, req := range requiredThemes {
		theme, ok := themeMap[strings.ToLower(req)]
		if !ok {
			t.Errorf("missing required theme: %s", req)
			continue
		}
		if theme.Name == "" {
			t.Errorf("theme %s has empty name", req)
		}
	}

	if len(AvailableThemes) < 10 {
		t.Errorf("expected at least 10 available themes, got %d", len(AvailableThemes))
	}
}

func TestApplyThemeGlobally(t *testing.T) {
	app := tview.NewApplication()
	editor := NewSQLEditor(app)
	activeSQLEditor = editor

	tbl := tview.NewTable()
	tbl.SetCell(0, 0, tview.NewTableCell("Header 1"))
	tbl.SetCell(0, 1, tview.NewTableCell("Header 2"))
	tbl.SetCell(1, 0, tview.NewTableCell("Row 1 Val"))
	tbl.SetCell(1, 1, tview.NewTableCell("Row 1 NULL"))
	tbl.SetCell(2, 0, tview.NewTableCell("Row 2 Val"))
	tbl.SetCell(2, 1, tview.NewTableCell("Row 2 Val"))
	dataTable = tbl

	lst := tview.NewList()
	lst.AddItem("users", "table", 'u', nil)
	tableList = lst

	sInput := tview.NewInputField()
	SchemaSearchInput = sInput

	for _, theme := range AvailableThemes {
		ApplyThemeGlobally(app, theme)

		if CurrentTheme.Name != theme.Name {
			t.Errorf("expected CurrentTheme.Name to be %s, got %s", theme.Name, CurrentTheme.Name)
		}

		// Verify table refreshed
		if tbl.GetCell(0, 0) == nil {
			t.Errorf("table cell (0, 0) is nil after theme apply")
		}
		if tbl.GetCell(1, 0) == nil {
			t.Errorf("table cell (1, 0) is nil after theme apply")
		}

		// Verify global tview styles updated
		if tview.Styles.PrimitiveBackgroundColor != theme.SurfaceBg {
			t.Errorf("tview.Styles.PrimitiveBackgroundColor did not match theme.SurfaceBg for %s", theme.Name)
		}
	}
}

func TestFormatThemeMarkup_LightAndDarkContrast(t *testing.T) {
	sample := "[white::b]PHERI[-::-] [gray]v3.2.0[-] [lime::b]OK[-] [yellow]Warning[-] [aqua]Info[-]"

	// Dark theme test (Nordic Frost)
	CurrentTheme = AvailableThemes[0]
	darkFormatted := FormatThemeMarkup(sample)
	if darkFormatted != sample {
		t.Errorf("expected dark theme markup to be unmodified, got: %s", darkFormatted)
	}

	// Light theme test
	var lightTheme UITheme
	for _, th := range AvailableThemes {
		if th.Name == "Light" {
			lightTheme = th
			break
		}
	}
	if !lightTheme.IsLight {
		t.Fatalf("expected Light theme to have IsLight = true")
	}

	CurrentTheme = lightTheme
	defer func() { CurrentTheme = AvailableThemes[0] }()

	lightFormatted := FormatThemeMarkup(sample)
	if strings.Contains(lightFormatted, "[white::b]") {
		t.Errorf("expected [white::b] to be replaced in light mode, got: %s", lightFormatted)
	}
	if strings.Contains(lightFormatted, "[gray]") {
		t.Errorf("expected [gray] to be replaced in light mode, got: %s", lightFormatted)
	}
	if strings.Contains(lightFormatted, "[yellow]") {
		t.Errorf("expected [yellow] to be replaced in light mode, got: %s", lightFormatted)
	}
	if strings.Contains(lightFormatted, "[lime::b]") {
		t.Errorf("expected [lime::b] to be replaced in light mode, got: %s", lightFormatted)
	}
	if strings.Contains(lightFormatted, "[aqua]") {
		t.Errorf("expected [aqua] to be replaced in light mode, got: %s", lightFormatted)
	}
}
