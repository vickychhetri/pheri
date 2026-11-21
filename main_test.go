package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveHistoryDBPath(t *testing.T) {
	dbPath := resolveHistoryDBPath()
	if dbPath == "" {
		t.Fatalf("expected non-empty dbPath from resolveHistoryDBPath")
	}

	// Verify that the parent directory of dbPath exists or is created
	dir := filepath.Dir(dbPath)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("parent directory for dbPath (%s) does not exist: %v", dir, err)
	}
	if !info.IsDir() {
		t.Fatalf("parent path for dbPath (%s) is not a directory", dir)
	}

	// Verify that the file name is phhistory.db or .pheri_history.db
	base := filepath.Base(dbPath)
	if base != "phhistory.db" && base != ".pheri_history.db" {
		t.Errorf("expected db file name to be phhistory.db or .pheri_history.db, got %s", base)
	}
}

func TestResolveHistoryDBPath_Override(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "custom_history.db")
	historyDBPathOverride = tempFile
	defer func() {
		historyDBPathOverride = ""
	}()

	path := resolveHistoryDBPath()
	if path != tempFile {
		t.Errorf("expected override path %s, got %s", tempFile, path)
	}
}
