# PHERI-07: Encrypted Connection Profile Passwords

- **Task Code**: `PHERI-07`
- **Feature Name**: Encrypted Connection Profile Passwords
- **Category**: Security & Credential Protection
- **Priority**: `P1 - High`
- **Status**: `Completed`
- **Target Files**: `phhistory/connections.go`, `phhistory/connections_test.go`
- **GitHub Issue**: [#22](https://github.com/vickychhetri/pheri/issues/22)

---

## 1. Reason of Adding

In earlier versions of Pheri, saved connection profiles were persisted in `~/.pheri_connections.json` with the password field stored as plain, unencrypted text.

### Critical Real-World Vulnerabilities Addressed:
1. **Plaintext Credential Exposure**:
   Any process or user with read access to the local home directory could extract production or staging database passwords in clear text.
2. **Snapshot & Backup Exfiltration Risks**:
   If home directory backups, cloud sync folders, or VM snapshots were compromised or shared, database credentials were immediately readable without cryptographic protection.
3. **Regulatory Non-Compliance**:
   Storing database credentials in plaintext violates standard compliance baselines (SOC 2, ISO 27001, PCI-DSS).

---

## 2. Technical Architecture & Cryptographic Design

### 1. Authenticated Encryption via AES-256-GCM (`phhistory/connections.go`)
- **Cipher Algorithm**: AES-256 in Galois/Counter Mode (GCM).
  - Provides both confidentiality (ciphertext) and authenticity/integrity (16-byte authentication tag).
  - Uses 96-bit (12-byte) cryptographically secure random nonces generated via `crypto/rand` for every encryption invocation.
  - Binds ciphertext to the application context using authenticated associated data (`"pheri-connection-profile-v1"`), preventing ciphertext reuse across contexts.
- **Ciphertext Serialization Format**:
  - `enc:v1:aes-gcm:<base64(nonce + ciphertext + tag)>`
  - Explicit version prefix (`enc:v1:aes-gcm:`) guarantees future cipher agility.

### 2. Machine-Bound Key Derivation
The 32-byte (256-bit) AES key is derived deterministically:
1. **Explicit Master Key Override**:
   If the environment variable `PHERI_MASTER_KEY` is set, its value is hashed with SHA-256 to produce the 32-byte key. This allows headless, CI/CD, and cross-machine profile synchronization.
2. **Machine-Bound Seed**:
   If `PHERI_MASTER_KEY` is not set:
   - Reads or generates a 32-byte high-entropy CSPRNG random seed in `~/.pheri_key` with restrictive permissions (`0600`).
   - Combines the seed, hostname (`os.Hostname()`), and user home directory path via SHA-256 to produce the machine-bound encryption key.
   - If an exfiltrated `~/.pheri_connections.json` file is transferred to another machine, decryption fails due to the missing seed and machine identity mismatch.

### 3. Transparent Backward Compatibility & Automatic Migration
- **Decryption Backward Compatibility**:
  When reading passwords from disk, any password string lacking the `enc:v1:aes-gcm:` prefix is treated as a legacy plaintext password and returned as-is.
- **Zero-Downtime Auto-Migration**:
  When `LoadSavedConnections()` is invoked, if any legacy plaintext profiles are detected, it decrypts them into memory and immediately re-saves the configuration file with encrypted passwords (`enc:v1:aes-gcm:...`).
- **Blank Passwords**:
  Empty password strings (`""`) remain blank and are not encrypted.

---

## 3. Test Coverage

Automated unit tests added in `phhistory/connections_test.go`:
- `TestEncryptDecryptPassword_Roundtrip`: Validates roundtrip encryption and decryption for standard, complex, whitespace, unicode (`密码_🔐`), and long strings.
- `TestBlankPasswordHandling`: Validates that empty strings remain empty and unencrypted.
- `TestLegacyPlaintextCompatibility`: Validates that legacy unencrypted passwords without prefixes are read seamlessly.
- `TestEncryptPassword_Idempotent`: Validates that calling `EncryptPassword` on an already-encrypted string does not double-encrypt.
- `TestDecryptPassword_TamperedCiphertext`: Validates that corrupt base64, truncated payloads, or modified authentication tags fail with clean errors.
- `TestMasterKey_EnvOverride`: Validates explicit `PHERI_MASTER_KEY` override and verifies decryption failure when a mismatched key is supplied.
- `TestFilePersistenceAndAutoMigration`: Validates full lifecycle: writing legacy plaintext profiles to disk, verifying automatic migration to encrypted format on load, and testing subsequent `SaveConnectionProfile` and `DeleteConnectionProfile` operations.
