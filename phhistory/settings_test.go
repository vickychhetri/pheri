// phhistory/settings_test.go
package phhistory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestThemePreferencePersistence(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test_settings.json")
	SetSettingsFilePathOverride(tempFile)
	defer SetSettingsFilePathOverride("")

	// Initially empty
	if got := GetThemePreference(); got != "" {
		t.Fatalf("expected empty theme initially, got: %s", got)
	}

	// Save theme
	if err := SaveThemePreference("Hacker"); err != nil {
		t.Fatalf("SaveThemePreference failed: %v", err)
	}

	// Verify loaded
	if got := GetThemePreference(); got != "Hacker" {
		t.Fatalf("expected 'Hacker', got: %s", got)
	}

	// Update to another theme
	if err := SaveThemePreference("Gen-Z"); err != nil {
		t.Fatalf("SaveThemePreference failed: %v", err)
	}

	if got := GetThemePreference(); got != "Gen-Z" {
		t.Fatalf("expected 'Gen-Z', got: %s", got)
	}

	// Corrupt file handling
	_ = os.WriteFile(tempFile, []byte("invalid json"), 0600)
	if got := GetThemePreference(); got != "" {
		t.Fatalf("expected empty string on corrupted file, got: %s", got)
	}
}
