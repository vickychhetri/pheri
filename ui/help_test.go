package ui

import (
	"strings"
	"testing"

	"github.com/rivo/tview"
)

func TestGetLaymanHelpGuide(t *testing.T) {
	guide := GetLaymanHelpGuide()
	if strings.TrimSpace(guide) == "" {
		t.Fatalf("expected non-empty help guide")
	}

	expectedSections := []string{
		"UNDERSTANDING THE SCREEN",
		"GETTING AROUND",
		"BROWSING & SEARCHING DATABASE OBJECTS",
		"WRITING & RUNNING SQL QUERIES",
		"VIEWING & EDITING DATA",
		"DEVOPS & DATABASE HEALTH DIAGNOSTICS",
		"FOOTER COMMAND LAUNCHER",
	}

	for _, sec := range expectedSections {
		if !strings.Contains(guide, sec) {
			t.Errorf("help guide missing section: %s", sec)
		}
	}

	expectedShortcuts := []string{
		"F1",
		"Tab",
		"Ctrl+B",
		"F11",
		"F12",
		"table:name",
		"view:name",
		"proc:name",
		"F3",
		"Ctrl+X",
		"Ctrl+R",
		"Ctrl+C",
		"Ctrl+T",
		"Ctrl+S",
		"F2",
		"Ctrl+H",
		":format",
		":clear",
		"Ctrl+N",
		"Ctrl+P",
		"Ctrl+E",
		"F4",
		"F5",
		"F6",
		"F7",
		"Ctrl+Y",
		"Ctrl+W",
		"F9",
		"F10",
		":help",
		":process",
		":explain",
		":health",
		":diff",
		":import",
		":autocommit",
		":commit",
		":rollback",
		":review",
		":quit",
	}

	for _, sc := range expectedShortcuts {
		if !strings.Contains(guide, sc) {
			t.Errorf("help guide missing shortcut/command: %s", sc)
		}
	}
}

func TestShowComprehensiveHelpModalNonCrashing(t *testing.T) {
	app := tview.NewApplication()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ShowComprehensiveHelpModal panicked: %v", r)
		}
	}()

	ShowComprehensiveHelpModal(app)
	ShowHelpModal(app)
}

func TestShowAboutModalNonCrashing(t *testing.T) {
	app := tview.NewApplication()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("showAboutModal panicked: %v", r)
		}
	}()

	showAboutModal(app)
}
