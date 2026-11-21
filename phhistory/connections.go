// phhistory/connections.go
package phhistory

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// EncryptionPrefix identifies encrypted password strings in stored profiles.
	EncryptionPrefix = "enc:v1:aes-gcm:"
	// NonceSize defines standard AES-GCM nonce size (96 bits / 12 bytes).
	NonceSize = 12
	// KeySize defines AES-256 key size in bytes.
	KeySize = 32
	// KeyFileName is the local secret seed file stored in user's home directory.
	KeyFileName = ".pheri_key"
	// AuthContext is the associated authenticated data bound to the AES-GCM cipher.
	AuthContext = "pheri-connection-profile-v1"
)

var (
	masterKeyMu                 sync.RWMutex
	cachedMasterKey             []byte
	connectionsFilePathOverride string
	keyFilePathOverride         string
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
	if connectionsFilePathOverride != "" {
		return connectionsFilePathOverride
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	return filepath.Join(homeDir, ".pheri_connections.json")
}

func getKeyFilePath() string {
	if keyFilePathOverride != "" {
		return keyFilePathOverride
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	return filepath.Join(homeDir, KeyFileName)
}

// ResetMasterKeyCache clears the in-memory cached key (primarily used in testing).
func ResetMasterKeyCache() {
	masterKeyMu.Lock()
	defer masterKeyMu.Unlock()
	cachedMasterKey = nil
}

// getMasterKey derives a 32-byte AES-256 key using machine identity and local CSPRNG seed,
// or the PHERI_MASTER_KEY environment variable if explicitly configured.
func getMasterKey() ([]byte, error) {
	// 1. Environment variable override
	if envKey := os.Getenv("PHERI_MASTER_KEY"); envKey != "" {
		hash := sha256.Sum256([]byte(envKey))
		return hash[:], nil
	}

	masterKeyMu.RLock()
	if len(cachedMasterKey) > 0 {
		k := make([]byte, len(cachedMasterKey))
		copy(k, cachedMasterKey)
		masterKeyMu.RUnlock()
		return k, nil
	}
	masterKeyMu.RUnlock()

	masterKeyMu.Lock()
	defer masterKeyMu.Unlock()

	// Double-check under write lock
	if len(cachedMasterKey) > 0 {
		k := make([]byte, len(cachedMasterKey))
		copy(k, cachedMasterKey)
		return k, nil
	}

	// 2. Local key seed file ~/.pheri_key (permissions 0600)
	keyPath := getKeyFilePath()
	seed, err := os.ReadFile(keyPath)
	if err != nil || len(seed) < KeySize {
		seed = make([]byte, KeySize)
		if _, randErr := io.ReadFull(rand.Reader, seed); randErr != nil {
			return nil, fmt.Errorf("failed to generate random key seed: %w", randErr)
		}
		_ = os.WriteFile(keyPath, seed, 0600)
	}

	// 3. Machine identity factors
	homeDir, _ := os.UserHomeDir()
	hostname, _ := os.Hostname()

	// 4. Derive machine-bound 32-byte key via SHA-256
	h := sha256.New()
	h.Write(seed)
	h.Write([]byte(hostname))
	h.Write([]byte(homeDir))
	derivedKey := h.Sum(nil)

	cachedMasterKey = make([]byte, len(derivedKey))
	copy(cachedMasterKey, derivedKey)

	return derivedKey, nil
}

// EncryptPassword encrypts a plaintext password string using AES-256-GCM.
// Returns "enc:v1:aes-gcm:<base64-encoded(nonce+ciphertext+tag)>".
// Empty strings are returned unchanged.
func EncryptPassword(plainText string) (string, error) {
	if plainText == "" {
		return "", nil
	}
	if strings.HasPrefix(plainText, EncryptionPrefix) {
		return plainText, nil
	}

	key, err := getMasterKey()
	if err != nil {
		return "", fmt.Errorf("master key derivation failed: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM block cipher: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate random nonce: %w", err)
	}

	// Seal appends ciphertext and authentication tag to nonce
	sealed := gcm.Seal(nonce, nonce, []byte(plainText), []byte(AuthContext))
	encoded := base64.StdEncoding.EncodeToString(sealed)
	return EncryptionPrefix + encoded, nil
}

// DecryptPassword decrypts a stored password string.
// If the input is not prefixed with "enc:v1:aes-gcm:", it is returned as-is for backward compatibility.
func DecryptPassword(cipherText string) (string, error) {
	if cipherText == "" {
		return "", nil
	}
	if !strings.HasPrefix(cipherText, EncryptionPrefix) {
		// Legacy unencrypted plaintext password
		return cipherText, nil
	}

	encoded := strings.TrimPrefix(cipherText, EncryptionPrefix)
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("invalid base64 ciphertext: %w", err)
	}

	key, err := getMasterKey()
	if err != nil {
		return "", fmt.Errorf("master key derivation failed: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM block cipher: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext payload too short")
	}

	nonce, actualCiphertext := data[:nonceSize], data[nonceSize:]
	plainTextBytes, err := gcm.Open(nil, nonce, actualCiphertext, []byte(AuthContext))
	if err != nil {
		return "", fmt.Errorf("decryption failed (authentication tag mismatch or key changed): %w", err)
	}

	return string(plainTextBytes), nil
}

// writeProfilesToDisk encrypts all passwords and serializes profiles to disk with 0600 permissions.
func writeProfilesToDisk(filePath string, profiles []ConnectionProfile) error {
	diskProfiles := make([]ConnectionProfile, len(profiles))
	copy(diskProfiles, profiles)

	for i := range diskProfiles {
		if diskProfiles[i].Pass != "" && !strings.HasPrefix(diskProfiles[i].Pass, EncryptionPrefix) {
			encPass, err := EncryptPassword(diskProfiles[i].Pass)
			if err == nil {
				diskProfiles[i].Pass = encPass
			}
		}
	}

	data, err := json.MarshalIndent(diskProfiles, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0600)
}

// LoadSavedConnections reads all saved connection profiles and decrypts stored passwords.
// If any legacy plaintext passwords are found on disk, they are automatically migrated to encrypted format.
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

	needsMigration := false
	for i := range profiles {
		if profiles[i].Environment == "" {
			profiles[i].Environment = "DEV"
		}
		rawPass := profiles[i].Pass
		if rawPass != "" {
			if !strings.HasPrefix(rawPass, EncryptionPrefix) {
				needsMigration = true
			}
			decryptedPass, decErr := DecryptPassword(rawPass)
			if decErr != nil {
				// Failed to decrypt (e.g. key mismatch or corruption), blank out
				profiles[i].Pass = ""
			} else {
				profiles[i].Pass = decryptedPass
			}
		}
	}

	if needsMigration {
		_ = writeProfilesToDisk(filePath, profiles)
	}

	return profiles, nil
}

// SaveConnectionProfile adds or updates a saved profile, encrypting credentials on disk.
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
	return writeProfilesToDisk(filePath, profiles)
}

// DeleteConnectionProfile removes a profile by ID and writes encrypted profiles to disk.
func DeleteConnectionProfile(id string) error {
	profiles, _ := LoadSavedConnections()
	var newProfiles []ConnectionProfile
	for _, p := range profiles {
		if p.ID != id {
			newProfiles = append(newProfiles, p)
		}
	}

	filePath := getConnectionsFilePath()
	return writeProfilesToDisk(filePath, newProfiles)
}
