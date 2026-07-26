// phhistory/connections.go
package phhistory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type ConnectionProfile struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Host        string    `json:"host"`
	Port        string    `json:"port"`
	User        string    `json:"user"`
	Pass        string    `json:"pass"`
	Environment string    `json:"environment"` // "DEV", "STAGING", "PROD"
	ReadOnly    bool      `json:"read_only"`
	LastUsed    time.Time `json:"last_used"`
}

func getConnectionsFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	return filepath.Join(homeDir, ".pheri_connections.json")
}

// LoadSavedConnections reads all saved connection profiles
func LoadSavedConnections() ([]ConnectionProfile, error) {
	filePath := getConnectionsFilePath()
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return []ConnectionProfile{}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var profiles []ConnectionProfile
	err = json.Unmarshal(data, &profiles)
	if err != nil {
		return []ConnectionProfile{}, nil
	}

	for i := range profiles {
		if profiles[i].Environment == "" {
			profiles[i].Environment = "DEV"
		}
	}

	return profiles, nil
}

// SaveConnectionProfile adds or updates a saved profile
func SaveConnectionProfile(host, port, user, pass, env string, readOnly bool) error {
	if env == "" {
		env = "DEV"
	}

	profiles, _ := LoadSavedConnections()

	id := fmt.Sprintf("%s@%s:%s", user, host, port)
	name := fmt.Sprintf("%s@%s:%s", user, host, port)

	updated := false
	for i, p := range profiles {
		if p.ID == id {
			profiles[i].Pass = pass
			profiles[i].Environment = env
			profiles[i].ReadOnly = readOnly
			profiles[i].LastUsed = time.Now()
			updated = true
			break
		}
	}

	if !updated {
		newProf := ConnectionProfile{
			ID:          id,
			Name:        name,
			Host:        host,
			Port:        port,
			User:        user,
			Pass:        pass,
			Environment: env,
			ReadOnly:    readOnly,
			LastUsed:    time.Now(),
		}
		profiles = append([]ConnectionProfile{newProf}, profiles...)
	}

	filePath := getConnectionsFilePath()
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0600)
}

// DeleteConnectionProfile removes a profile by ID
func DeleteConnectionProfile(id string) error {
	profiles, _ := LoadSavedConnections()
	var newProfiles []ConnectionProfile
	for _, p := range profiles {
		if p.ID != id {
			newProfiles = append(newProfiles, p)
		}
	}

	filePath := getConnectionsFilePath()
	data, err := json.MarshalIndent(newProfiles, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0600)
}
