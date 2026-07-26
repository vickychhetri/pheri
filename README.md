# Pheri

<<<<<<< HEAD
Pheri is a fast, terminal-based database management TUI (Terminal User Interface) built for MySQL. It provides a developer-friendly SQL environment with real-time tokenized syntax highlighting, live process management, query execution analysis, interactive inline cell editing, and selective database exporting directly inside your terminal.

Release Tag: v3.0.0 (https://github.com/vickychhetri/pheri/releases/tag/v3.0.0)

---

## Key Features

- **Tokenized SQL Syntax Highlighting**: Real-time syntax coloring for SQL keywords, functions, data types, strings, identifiers, numbers, comments, and operators.
- **Table Name Auto-Completion (Ctrl+T)**: Instant schema table name suggestions while typing SQL queries.
- **SQL Snippets & Templates (Ctrl+S / Ctrl+Space)**: Quick boilerplate injection for SELECT, INSERT, UPDATE, DELETE, JOINs, and DDL templates.
- **Active Panel Focus Highlighting**: Clear visual feedback on the active pane (SQL Editor, Object Browser, Databases, or Data Grid) with high-visibility borders.
- **Environment Tags & Read-Only Guard**: Assign connections to PROD, STAGING, or DEV environments. Enforce read-only mode to prevent accidental mutations on production servers.
- **Live Process Manager (F4 / :process)**: Monitor active threads, view query execution states, and kill stuck connections or slow queries.
- **EXPLAIN Query Analyzer (F5 / :explain)**: Inspect query execution plans, index utilization, and performance bottlenecks.
- **Super-Fast Concurrent Database Export (Ctrl+Y)**: Parallel multi-threaded dump (130 workers) of all tables, views, stored procedures, functions, triggers, and events into compressed `.gz` archives.
- **Schema Inspector (F3)**: View table column definitions, primary keys, data types, and index layouts.
- **Smart Object Search & Filters**: Search across databases, tables, views, procedures, and functions with typed prefixes (table:, view:, procedure:, function:, db:).
- **Inline Cell Editing**: Modify database values directly inside the result grid.
- **Saved Connection Profiles**: Securely store database configurations locally for quick login.

---

## Installation & Setup

### Download Pre-Built Binary (v3.0.0)
Download the binary for your operating system directly from the GitHub Releases page:
https://github.com/vickychhetri/pheri/releases/tag/v3.0.0

### Build from Source

```bash
# Clone the repository
=======
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
>>>>>>> 13056f2e4934244d2b9e87bfede38bbfeedde629
git clone https://github.com/vickychhetri/pheri.git
cd pheri

# Build executable
go build -o pheri main.go

# Run Pheri
./pheri
```

<<<<<<< HEAD
### Install via Go
```bash
go install github.com/vickychhetri/pheri@latest
```

---

## Quick Start & CLI Flags

### Direct CLI Connection
Connect to a database server directly from your command line:

```bash
./pheri -u root -p YourPassword -host 127.0.0.1 -port 3306 -db my_database
```

### Command Line Options

| Flag | Description | Default |
| :--- | :--- | :--- |
| `-u` | MySQL Username | `root` |
| `-p` | MySQL Password | `""` |
| `-host` | Host address | `127.0.0.1` |
| `-port` | MySQL Port | `3306` |
| `-db` | Default database name | `""` |
| `-env` | Environment tag (`PROD`, `STAGING`, `DEV`) | `DEV` |
| `-readonly` | Enable read-only mode protection (`true`/`false`) | `false` |

---

## Keyboard Shortcuts

### Editor & Navigation
=======
### Direct CLI Connection
```bash
./pheri -u root -p YourPassword -host 127.0.0.1 -port 3306
```

---

## Essential Keyboard Shortcuts
>>>>>>> 13056f2e4934244d2b9e87bfede38bbfeedde629

| Shortcut | Action |
| :--- | :--- |
| `Ctrl+R` | Execute SQL Query |
<<<<<<< HEAD
| `Ctrl+T` | Auto-complete Table Name suggestions |
| `Ctrl+S` / `Ctrl+Space` | Insert SQL Snippets / Templates |
| `Ctrl+P` | Paste system clipboard content |
| `F4` | Open Live Process Manager & Query Killer (`SHOW FULL PROCESSLIST`) |
| `F5` | Open EXPLAIN Query Plan Analyzer |
| `F11` | Toggle Fullscreen Mode for Editor or Data Grid |
| `F3` | Open Schema Inspector (Columns, Types, Keys) |
| `Ctrl+E` | Open Selective Database Export Wizard |
| `Ctrl+X` | Copy CREATE TABLE / VIEW definition to clipboard |
| `Tab` | Navigate focus forward between panels and buttons |
| `Esc` | Return focus to Schema Objects list |

---

## Search & Filter System

Pheri allows filtering database objects directly from the search bar using type-specific prefixes:

```
<type>:<search-term>
```

### Supported Filter Prefixes

| Prefix | Description | Example |
| :--- | :--- | :--- |
| `table:` | Filter database tables | `table:user` |
| `view:` | Filter views | `view:active` |
| `procedure:` | Filter stored procedures | `procedure:get` |
| `function:` | Filter user-defined functions | `function:calc` |
| `db:` | Filter available databases | `db:sales` |

### Search Behavior
- **Global Search**: Type any keyword (e.g. `customer`) without a prefix to search across all tables, views, procedures, and functions.
- **Table / View Selection**: Pressing Enter runs `SELECT * FROM table LIMIT 100` and displays the dataset in the grid with inline cell editing support.
- **Procedure / Function Selection**: Pressing Enter displays the routine definition inside the query viewer.
- **Database Selection**: Pressing Enter switches the active database context.
=======
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
>>>>>>> 13056f2e4934244d2b9e87bfede38bbfeedde629

---

## Screenshots

<<<<<<< HEAD
- Login Interface: https://github.com/user-attachments/assets/dae70040-9043-4de0-9794-bf1252e6c65b
- Workspace Dashboard: https://github.com/user-attachments/assets/d111fe23-b7f5-4593-8036-288f716f5c62
- Database Explorer: https://github.com/user-attachments/assets/6cd265c8-c9bf-4abd-9aec-a7eca5efbef8
=======
<img width="1920" alt="Pheri Login Screen" src="https://github.com/user-attachments/assets/dae70040-9043-4de0-9794-bf1252e6c65b" />
<img width="1920" alt="Pheri Workspace" src="https://github.com/user-attachments/assets/d111fe23-b7f5-4593-8036-288f716f5c62" />
>>>>>>> 13056f2e4934244d2b9e87bfede38bbfeedde629

---

## Supported Terminals
<<<<<<< HEAD

Windows Terminal, Command Prompt, PowerShell, WSL, macOS Terminal, iTerm2, Alacritty, Kitty, Bash, Zsh, Fish, Termux.
=======
Linux, macOS, Windows Terminal, WSL, iTerm2, Alacritty, Kitty, PowerShell, Termux, Zsh, Bash.
>>>>>>> 13056f2e4934244d2b9e87bfede38bbfeedde629

---

## License
<<<<<<< HEAD

MIT License - see LICENSE file for details.
=======
MIT License
>>>>>>> 13056f2e4934244d2b9e87bfede38bbfeedde629
