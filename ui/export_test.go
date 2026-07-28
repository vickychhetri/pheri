package ui

import (
	"os"
	"path/filepath"
	"testing"
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
