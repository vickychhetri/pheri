# Pheri

Pheri is a fast, terminal-based client for MySQL. It provides a modern TUI to manage databases, execute queries with syntax highlighting, inspect schemas, monitor live processes, and export objects directly from your terminal.

---

## Features

- **Environment Tags & Read-Only Guard** - Label connections as `[PROD]`, `[STAGING]`, or `[DEV]` with built-in Read-Only protection against accidental database mutations.
- **Live Process Manager (`F4` / `:process`)** - Monitor active database threads and kill stuck queries or connections (`KILL <id>`).
- **EXPLAIN Query Analyzer (`F5` / `:explain`)** - View query execution plans, index usage, and performance bottlenecks.
- **Saved Connection Profiles** - Save credentials locally for instant 1-click logins.
- **SQL Query Editor** - Code editor with syntax highlighting, line numbers, snippets, and fullscreen mode (`F11`).
- **Selective Database Export** - Interactively export tables, views, procedures, functions, triggers, and events to SQL dumps (`.sql`), JSON, or CSV.
- **Fast Object Search & Filters** - Filter objects easily by type (`table:`, `view:`, `procedure:`, `function:`, `db:`).
- **Schema Inspector** - Inspect table column structures, types, keys, and indexes (`F3`).
- **Developer Command Launcher** - Global command bar for quick actions (`:process`, `:explain`, `:export`, `:clear`, `:quit`, `:help`).

---

## Quick Start

### Installation & Build

```bash
# Clone repository
git clone https://github.com/vickychhetri/pheri.git
cd pheri

# Build executable
go build -o pheri main.go

# Run Pheri
./pheri
```

### Direct CLI Connection
```bash
./pheri -u root -p YourPassword -host 127.0.0.1 -port 3306
```

---

## Essential Keyboard Shortcuts

| Shortcut | Action |
| :--- | :--- |
| `Ctrl+R` | Execute SQL Query |
| `F4` | Open Live Process Manager & Query Killer (`SHOW FULL PROCESSLIST`) |
| `F5` | Inspect Query Execution Plan (`EXPLAIN ANALYZE`) |
| `F11` | Toggle Fullscreen Code Editor |
| `F3` | Open Schema Inspector (Columns & Indexes) |
| `Ctrl+E` | Launch Selective Database Export Wizard |
| `Ctrl+S` / `Ctrl+T` | Open SQL Snippets & Templates |
| `Tab` / `Shift+Tab` | Cycle Focus (Sidebar <-> Editor <-> Data Grid) |
| `:` | Open Command Launcher (`:process`, `:explain`, `:export`, `:quit`) |

---

## Object Search & Filter Shortcuts

Type prefix in the sidebar filter to isolate specific objects:

- `table:user` - Filter tables matching `user`
- `view:active` - Filter views matching `active`
- `procedure:get` - Filter procedures matching `get`
- `function:calc` - Filter functions matching `calc`
- `db:sales` - Filter databases matching `sales`

---

## Screenshots

<img width="1920" alt="Pheri Login Screen" src="https://github.com/user-attachments/assets/dae70040-9043-4de0-9794-bf1252e6c65b" />
<img width="1920" alt="Pheri Workspace" src="https://github.com/user-attachments/assets/d111fe23-b7f5-4593-8036-288f716f5c62" />

---

## Supported Terminals
Linux, macOS, Windows Terminal, WSL, iTerm2, Alacritty, Kitty, PowerShell, Termux, Zsh, Bash.

---

## License
MIT License
