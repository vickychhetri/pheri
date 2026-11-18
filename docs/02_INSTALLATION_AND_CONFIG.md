# 02 - Installation & Configuration Guide

This guide covers all methods for installing Pheri, terminal compatibility, command-line arguments, saved connection profiles, and environment protection features.

---

## 1. System Requirements & Compatibility

### Supported Platforms
- **Linux**: Ubuntu/Debian, Fedora/RHEL, Arch, Alpine, openSUSE, etc. (x86_64, arm64).
- **macOS**: macOS 10.15+ (Intel and Apple Silicon M-series).
- **Windows**: Windows 10/11, Windows Server (x86_64, arm64).
- **Mobile/Embedded**: Android via Termux, Raspberry Pi OS.

### Supported Terminals
Pheri utilizes ANSI 256-color and true-color escape sequences. It is fully supported and tested on:
- **Windows**: Windows Terminal (recommended), PowerShell 7+, Command Prompt, WSL/WSL2.
- **macOS**: iTerm2, macOS Terminal.app, Kitty, Alacritty, WezTerm.
- **Linux**: GNOME Terminal, Konsole, Alacritty, Kitty, XFCE Terminal, standard Linux TTY console.
- **Multiplexers**: `tmux`, `screen`, `zellij`.

---

## 2. Installation Methods

### Method 1: Pre-Built Binaries (Recommended)

Download the compiled binary for your architecture from the [GitHub Releases](https://github.com/vickychhetri/pheri/releases) page:

#### Linux (AMD64 / ARM64)
```bash
# Download and extract Linux AMD64 binary
curl -LO https://github.com/vickychhetri/pheri/releases/latest/download/pheri-linux-amd64.tar.gz
tar -xzvf pheri-linux-amd64.tar.gz
sudo mv pheri /usr/local/bin/
pheri --help
```

#### Debian / Ubuntu (.deb)
```bash
# Install Debian package directly
curl -LO https://github.com/vickychhetri/pheri/releases/latest/download/pheri_1.0.0_amd64.deb
sudo dpkg -i pheri_1.0.0_amd64.deb
```

#### Snap Package
```bash
sudo snap install pheri
```

#### Windows (AMD64 / ARM64)
1. Download `pheri-windows-amd64.zip` from GitHub Releases.
2. Unzip `pheri.exe` to a folder in your `PATH` (e.g., `C:\Program Files\Pheri` or `C:\Users\<user>\bin`).
3. Open Windows Terminal or PowerShell and run `pheri.exe`.

---

### Method 2: Install via Go CLI

If you have Go (1.18 or higher) installed on your system:

```bash
go install github.com/vickychhetri/pheri@latest
```

This compiles and places the `pheri` binary inside `$GOPATH/bin` (or `~/go/bin`). Ensure that directory is in your system's `PATH`.

---

### Method 3: Build from Source

To compile Pheri with custom modifications or for development:

```bash
# Clone the repository
git clone https://github.com/vickychhetri/pheri.git
cd pheri

# Download module dependencies
go mod download

# Compile binary
go build -o pheri main.go

# Run Pheri
./pheri
```

#### Multi-Platform Build Script (`build.sh`)
The repository includes a helper script `build.sh` for cross-compiling release binaries across platforms:
```bash
chmod +x build.sh
./build.sh
```
This builds packages into the `build/` directory for Linux (amd64, arm64), Windows (amd64, arm64), and Debian packages.

---

## 3. Command Line Flags & Options

You can launch Pheri interactively (which displays the connection form) or pass command-line flags to connect directly:

```bash
pheri -u <user> -p <password> -host <host> -port <port> -db <database> -env <DEV|STAGING|PROD> [-readonly]
```

### Complete Flags Reference

| Flag | Type | Default | Description |
| :--- | :---: | :---: | :--- |
| `-u` | `string` | `""` | MySQL / MariaDB username (e.g. `root`). |
| `-p` | `string` | `""` | MySQL user password. If omitted, prompts interactively. |
| `-host` | `string` | `localhost` | Database server hostname or IP address (e.g. `127.0.0.1` or `db.internal`). |
| `-port` | `string` | `3306` | MySQL port number. |
| `-db` | `string` | `""` | Target database name to auto-select upon connection. |
| `-env` | `string` | `DEV` | Environment tag: `DEV`, `STAGING`, or `PROD`. |
| `-readonly` | `bool` | `false` | Enables read-only protection. Blocks mutations and imports. |
| `-history` | `bool` | `false` | Prints query history to terminal stdout and exits without launching the TUI. |
| `-days` | `int` | `30` | Number of days of history to fetch when using `-history`. |
| `-months` | `int` | `12` | Number of months of history to fetch when using `-history`. |
| `-years` | `int` | `1` | Number of years of history to fetch when using `-history`. |
| `-history_file`| `string` | `""` | Import or export target file for query history. |

### CLI Launch Examples

#### Connect to Localhost Development Database:
```bash
pheri -u root -p secret -host 127.0.0.1 -port 3306 -db my_app -env DEV
```

#### Connect to Production Database in Read-Only Mode:
```bash
pheri -u dba_reader -p securepass -host prod-db.company.internal -port 3306 -db analytics -env PROD -readonly
```

#### Inspect Past Query History from Terminal CLI:
```bash
pheri -history -days 7
```

---

## 4. Saved Connection Profiles

When connecting through the interactive TUI connection form, checking **"Remember Credentials"** saves the connection profile locally.

### Profile Storage Location
Profiles are stored in JSON format at:
```
~/.pheri_connections.json
```
On Unix/Linux/macOS systems, this file is created with restricted read/write permissions (`0600`) so that only your user account can read stored passwords.

### Profile JSON Structure
```json
[
  {
    "id": "root@127.0.0.1:3306",
    "name": "root@127.0.0.1:3306",
    "host": "127.0.0.1",
    "port": "3306",
    "user": "root",
    "pass": "mypassword",
    "environment": "DEV",
    "read_only": false,
    "last_used": "2026-11-18T22:30:00Z"
  }
]
```

### Profile Switching in TUI
In the connection form, use the **Quick Saved Profile** dropdown at the top. Selecting a saved profile instantly populates Host, Port, User, Password, Environment Tag, and Read-Only settings.

---

## 5. Environment Tags & Read-Only Safeguards

Pheri provides visual and functional safeguards to prevent operational mistakes when working with critical databases:

### Environment Tags
You can designate connections as:
- **`DEV` (Development)**: Default mode. Green/Cyan accents. Relaxed prompts.
- **`STAGING` (Test/Staging)**: Yellow borders and headers indicating pre-production environments.
- **`PROD` (Production)**: High-visibility Red warning headers, bold badge indicators on all panels, and strict confirmation prompts.

### Read-Only Guard Mode
When **Read-Only Mode** is active (either via `-readonly` CLI flag or the checkbox in the connection form):
1. **Query Mutation Interception**: Any SQL statement starting with or containing mutating keywords is blocked:
   - `INSERT`
   - `UPDATE`
   - `DELETE`
   - `DROP`
   - `ALTER`
   - `TRUNCATE`
   - `RENAME`
   - `REPLACE`
2. **Grid Cell Editing Blocked**: The interactive cell editor in the results table is disabled; attempting to edit a cell raises an immediate warning modal.
3. **Database Imports Blocked**: The Import Wizard (`F9` / `:import`) refuses to execute dump scripts, protecting production schemas from being overwritten.
