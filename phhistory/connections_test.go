// phhistory/connections_test.go
package phhistory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupTestEnv(t *testing.T) string {
	tempDir := t.TempDir()
	connectionsFilePathOverride = filepath.Join(tempDir, ".pheri_connections.json")
	keyFilePathOverride = filepath.Join(tempDir, ".pheri_key")
	ResetMasterKeyCache()

	t.Cleanup(func() {
		connectionsFilePathOverride = ""
		keyFilePathOverride = ""
		ResetMasterKeyCache()
	})
	return tempDir
}

func TestEncryptDecryptPassword_Roundtrip(t *testing.T) {
	setupTestEnv(t)

	testPasswords := []string{
		"simplePassword123",
		"complex!@#$%^&*()_+-=[]{}|;':,./<>?",
		"with spaces and tabs\t\n",
		"unicode_密码_mypass_🔐",
		strings.Repeat("aLongPasswordWithRepeatedCharacters", 50),
	}

	for _, original := range testPasswords {
		encrypted, err := EncryptPassword(original)
		if err != nil {
			t.Fatalf("EncryptPassword failed for %q: %v", original, err)
		}

		if !strings.HasPrefix(encrypted, EncryptionPrefix) {
			t.Errorf("expected encrypted string to have prefix %q, got: %s", EncryptionPrefix, encrypted)
		}

		decrypted, err := DecryptPassword(encrypted)
		if err != nil {
			t.Fatalf("DecryptPassword failed for %q: %v", original, err)
		}

		if decrypted != original {
			t.Errorf("roundtrip mismatch: expected %q, got %q", original, decrypted)
		}
	}
}

func TestBlankPasswordHandling(t *testing.T) {
	setupTestEnv(t)

	enc, err := EncryptPassword("")
	if err != nil {
		t.Fatalf("EncryptPassword with empty string returned error: %v", err)
	}
	if enc != "" {
		t.Errorf("expected empty string to remain empty, got %q", enc)
	}

	dec, err := DecryptPassword("")
	if err != nil {
		t.Fatalf("DecryptPassword with empty string returned error: %v", err)
	}
	if dec != "" {
		t.Errorf("expected empty string to remain empty, got %q", dec)
	}
}

func TestLegacyPlaintextCompatibility(t *testing.T) {
	setupTestEnv(t)

	legacyPasswords := []string{
		"rootpassword",
		"my_legacy_plain_secret",
		"admin@123",
	}

	for _, p := range legacyPasswords {
		dec, err := DecryptPassword(p)
		if err != nil {
			t.Fatalf("DecryptPassword failed on legacy plaintext: %v", err)
		}
		if dec != p {
			t.Errorf("expected legacy plaintext %q to be returned unchanged, got %q", p, dec)
		}
	}
}

func TestEncryptPassword_Idempotent(t *testing.T) {
	setupTestEnv(t)

	pass := "mySecretKey"
	enc1, err := EncryptPassword(pass)
	if err != nil {
		t.Fatalf("EncryptPassword failed: %v", err)
	}

	enc2, err := EncryptPassword(enc1)
	if err != nil {
		t.Fatalf("EncryptPassword second invocation failed: %v", err)
	}

	if enc1 != enc2 {
		t.Errorf("expected EncryptPassword to be idempotent on already-encrypted strings")
	}
}

func TestDecryptPassword_TamperedCiphertext(t *testing.T) {
	setupTestEnv(t)

	// 1. Invalid base64
	_, err := DecryptPassword(EncryptionPrefix + "not_valid_base64!@#$")
	if err == nil {
		t.Error("expected error on invalid base64, got nil")
	}

	// 2. Too short (less than nonce + tag)
	_, err = DecryptPassword(EncryptionPrefix + "AAAA")
	if err == nil {
		t.Error("expected error on payload too short, got nil")
	}

	// 3. Corrupted tag / ciphertext
	validEnc, err := EncryptPassword("correct_secret")
	if err != nil {
		t.Fatalf("EncryptPassword failed: %v", err)
	}

	tampered := validEnc[:len(validEnc)-4] + "XXXX"
	_, err = DecryptPassword(tampered)
	if err == nil {
		t.Error("expected error on tampered authentication tag, got nil")
	}
}

func TestMasterKey_EnvOverride(t *testing.T) {
	setupTestEnv(t)
	t.Setenv("PHERI_MASTER_KEY", "custom-org-passphrase-2026")
	ResetMasterKeyCache()

	secret := "production_db_password"
	enc, err := EncryptPassword(secret)
	if err != nil {
		t.Fatalf("EncryptPassword with env master key failed: %v", err)
	}

	dec, err := DecryptPassword(enc)
	if err != nil {
		t.Fatalf("DecryptPassword with matching env master key failed: %v", err)
	}
	if dec != secret {
		t.Errorf("expected %q, got %q", secret, dec)
	}

	// Change key to different master key
	t.Setenv("PHERI_MASTER_KEY", "wrong-passphrase")
	ResetMasterKeyCache()
	_, err = DecryptPassword(enc)
	if err == nil {
		t.Error("expected decryption failure with wrong master key, got nil")
	}
}

func TestFilePersistenceAndAutoMigration(t *testing.T) {
	setupTestEnv(t)

	// 1. Write legacy plaintext profiles directly to disk
	legacyProfiles := []ConnectionProfile{
		{
			ID:          "root@127.0.0.1:3306",
			Name:        "root@127.0.0.1:3306",
			Host:        "127.0.0.1",
			Port:        "3306",
			User:        "root",
			Pass:        "plain_text_password_123",
			Environment: "DEV",
			ReadOnly:    false,
			LastUsed:    time.Now(),
		},
		{
			ID:          "guest@10.0.0.1:3306",
			Name:        "guest@10.0.0.1:3306",
			Host:        "10.0.0.1",
			Port:        "3306",
			User:        "guest",
			Pass:        "", // blank password
			Environment: "STAGING",
			ReadOnly:    true,
			LastUsed:    time.Now(),
		},
	}

	rawBytes, err := json.MarshalIndent(legacyProfiles, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal legacy profiles: %v", err)
	}
	if err := os.WriteFile(connectionsFilePathOverride, rawBytes, 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	// 2. Load profiles via LoadSavedConnections (which triggers auto-migration)
	loaded, err := LoadSavedConnections()
	if err != nil {
		t.Fatalf("LoadSavedConnections failed: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 profiles loaded, got %d", len(loaded))
	}
	if loaded[0].Pass != "plain_text_password_123" {
		t.Errorf("expected decrypted pass in memory, got: %s", loaded[0].Pass)
	}
	if loaded[1].Pass != "" {
		t.Errorf("expected blank pass in memory, got: %s", loaded[1].Pass)
	}

	// 3. Inspect raw file on disk: verify pass is now encrypted
	diskBytes, err := os.ReadFile(connectionsFilePathOverride)
	if err != nil {
		t.Fatalf("failed to read disk file: %v", err)
	}

	var diskProfiles []ConnectionProfile
	if err := json.Unmarshal(diskBytes, &diskProfiles); err != nil {
		t.Fatalf("failed to unmarshal disk profiles: %v", err)
	}

	if !strings.HasPrefix(diskProfiles[0].Pass, EncryptionPrefix) {
		t.Errorf("expected disk file to contain encrypted password %q, got: %s", EncryptionPrefix, diskProfiles[0].Pass)
	}
	if diskProfiles[1].Pass != "" {
		t.Errorf("expected blank password on disk, got: %s", diskProfiles[1].Pass)
	}

	// 4. Save a new profile using SaveConnectionProfile
	err = SaveConnectionProfile("db.internal", "3307", "appuser", "superAppSecret", "PROD", true)
	if err != nil {
		t.Fatalf("SaveConnectionProfile failed: %v", err)
	}

	// 5. Reload and verify
	reloaded, err := LoadSavedConnections()
	if err != nil {
		t.Fatalf("LoadSavedConnections after save failed: %v", err)
	}
	if len(reloaded) != 3 {
		t.Fatalf("expected 3 profiles, got %d", len(reloaded))
	}

	var appProf *ConnectionProfile
	for i := range reloaded {
		if reloaded[i].User == "appuser" {
			appProf = &reloaded[i]
			break
		}
	}
	if appProf == nil {
		t.Fatal("could not find newly saved profile")
	}
	if appProf.Pass != "superAppSecret" {
		t.Errorf("expected decrypted password %q, got %q", "superAppSecret", appProf.Pass)
	}
	if appProf.Environment != "PROD" {
		t.Errorf("expected environment PROD, got %s", appProf.Environment)
	}
	if !appProf.ReadOnly {
		t.Errorf("expected ReadOnly true")
	}

	// 6. Delete profile
	err = DeleteConnectionProfile(appProf.ID)
	if err != nil {
		t.Fatalf("DeleteConnectionProfile failed: %v", err)
	}

	afterDelete, err := LoadSavedConnections()
	if err != nil {
		t.Fatalf("LoadSavedConnections after delete failed: %v", err)
	}
	if len(afterDelete) != 2 {
		t.Errorf("expected 2 profiles after delete, got %d", len(afterDelete))
	}
}
