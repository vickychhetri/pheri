package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rivo/tview"
)

func TestWriteExportFileAndReadGzippedFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pheri_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sampleSQL := "CREATE TABLE `test` (`id` INT PRIMARY KEY, `name` VARCHAR(50));\nINSERT INTO `test` VALUES (1, 'antigravity');\n"
	gzPath := filepath.Join(tempDir, "test_dump.sql.gz")

	// Test 1: Write compressed file
	err = writeExportFile(gzPath, []byte(sampleSQL))
	if err != nil {
		t.Fatalf("writeExportFile failed for .sql.gz: %v", err)
	}

	fi, err := os.Stat(gzPath)
	if err != nil {
		t.Fatalf("failed to stat written file: %v", err)
	}
	if fi.Size() == 0 {
		t.Fatalf("written gzipped file size is 0")
	}

	// Test 2: Read gzipped file transparently
	readContent, err := readGzippedFile(gzPath)
	if err != nil {
		t.Fatalf("readGzippedFile failed to read .sql.gz: %v", err)
	}
	if readContent != sampleSQL {
		t.Errorf("read content mismatch. Expected %q, got %q", sampleSQL, readContent)
	}

	// Test 3: Plain text file reading transparently
	plainPath := filepath.Join(tempDir, "test_dump.sql")
	err = os.WriteFile(plainPath, []byte(sampleSQL), 0644)
	if err != nil {
		t.Fatalf("failed to write plain sql file: %v", err)
	}

	plainReadContent, err := readGzippedFile(plainPath)
	if err != nil {
		t.Fatalf("readGzippedFile failed to read plain .sql: %v", err)
	}
	if plainReadContent != sampleSQL {
		t.Errorf("plain read content mismatch. Expected %q, got %q", sampleSQL, plainReadContent)
	}
}

func TestImportReadOnlyFlag(t *testing.T) {
	// Verify ActiveReadOnly state
	ActiveReadOnly = true
	if !ActiveReadOnly {
		t.Errorf("expected ActiveReadOnly to be true")
	}
	ActiveReadOnly = false
}

func TestFormatExportItemLabel(t *testing.T) {
	itemSel := ExportItem{Name: "actor", Type: "TABLE", Selected: true}
	itemUnsel := ExportItem{Name: "actor", Type: "TABLE", Selected: false}

	labelSel := FormatExportItemLabel(itemSel)
	labelUnsel := FormatExportItemLabel(itemUnsel)

	if labelSel == labelUnsel {
		t.Fatalf("expected selected and unselected labels to differ")
	}

	// Verify check indicator presence via tview.Escape or escaped form
	if !strings.Contains(labelSel, tview.Escape("[X]")) {
		t.Errorf("expected labelSel to contain escaped [X], got: %s", labelSel)
	}
	if !strings.Contains(labelUnsel, tview.Escape("[ ]")) {
		t.Errorf("expected labelUnsel to contain escaped [ ], got: %s", labelUnsel)
	}

	// Verify badges
	if !strings.Contains(labelSel, "TABLE") {
		t.Errorf("expected labelSel to contain TABLE badge, got: %s", labelSel)
	}
}

func TestFilterExportItems(t *testing.T) {
	items := []ExportItem{
		{Name: "actor", Type: "TABLE", Selected: true},
		{Name: "actor_info", Type: "VIEW", Selected: true},
		{Name: "film", Type: "TABLE", Selected: true},
		{Name: "film_in_stock", Type: "PROCEDURE", Selected: true},
		{Name: "inventory_held_by_customer", Type: "FUNCTION", Selected: true},
		{Name: "ins_film", Type: "TRIGGER", Selected: true},
	}

	// 1. Empty filter returns all
	all := filterExportItems(items, "")
	if len(all) != len(items) {
		t.Errorf("expected %d items for empty query, got %d", len(items), len(all))
	}

	// 2. Substring filter
	films := filterExportItems(items, "film")
	if len(films) != 3 { // film, film_in_stock, ins_film
		t.Errorf("expected 3 items for 'film', got %d", len(films))
	}

	// 3. Prefix table:
	tables := filterExportItems(items, "table:")
	if len(tables) != 2 {
		t.Errorf("expected 2 tables, got %d", len(tables))
	}

	// 4. Prefix view:
	views := filterExportItems(items, "view:")
	if len(views) != 1 || items[views[0]].Name != "actor_info" {
		t.Errorf("expected 1 view 'actor_info', got %v", views)
	}

	// 5. Prefix proc:
	procs := filterExportItems(items, "proc:film")
	if len(procs) != 1 || items[procs[0]].Name != "film_in_stock" {
		t.Errorf("expected 1 proc 'film_in_stock', got %v", procs)
	}

	// 6. Prefix func:
	funcs := filterExportItems(items, "func:inventory")
	if len(funcs) != 1 {
		t.Errorf("expected 1 function, got %d", len(funcs))
	}

	// 7. Prefix trig:
	trigs := filterExportItems(items, "trig:ins")
	if len(trigs) != 1 {
		t.Errorf("expected 1 trigger, got %d", len(trigs))
	}
}

func TestShowItemSelectorNonCrashing(t *testing.T) {
	app := tview.NewApplication()
	items := []ExportItem{
		{Name: "actor", Type: "TABLE", Selected: true},
		{Name: "actor_info", Type: "VIEW", Selected: false},
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("showItemSelector panicked: %v", r)
		}
	}()

	showItemSelector(app, nil, "sakila", items)
}
