// phhistory/settings.go
package phhistory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type AppSettings struct {
	Theme string `json:"theme"`
}

var (
	settingsMu               sync.RWMutex
	settingsFilePathOverride string
)

func getSettingsFilePath() string {
	if settingsFilePathOverride != "" {
		return settingsFilePathOverride
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	return filepath.Join(homeDir, ".pheri_settings.json")
}

// SetSettingsFilePathOverride allows tests to point to a temporary settings file.
func SetSettingsFilePathOverride(p string) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	settingsFilePathOverride = p
}

// SaveThemePreference stores the user's active theme choice.
func SaveThemePreference(themeName string) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	var settings AppSettings
	path := getSettingsFilePath()
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &settings)
	}

	settings.Theme = themeName
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// GetThemePreference retrieves the saved theme name or returns an empty string.
func GetThemePreference() string {
	settingsMu.RLock()
	defer settingsMu.RUnlock()

	path := getSettingsFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var settings AppSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return ""
	}
	return settings.Theme
}
