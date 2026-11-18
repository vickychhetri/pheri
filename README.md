# Pheri

Pheri is a lightweight, terminal-based database client and management tool for MySQL built in Go. It delivers a fast, interactive keyboard-driven interface with asynchronous query execution, real-time query cancellation, an interactive SQL editor, live process monitoring, query plan analysis, and compressed database backups directly in your terminal.

---

## About Pheri

Pheri was designed to give developers, database administrators, and DevOps engineers a comprehensive database management experience without the overhead of heavy desktop applications. It runs directly inside your terminal, over SSH sessions, in containerized environments, or locally on your workstation.

### Why Pheri?
- **Fast and Lightweight**: Starts instantly with a minimal memory footprint.
- **SSH and Remote Ready**: Run Pheri directly on remote servers without needing X11 forwarding or remote desktop tools.
- **Non-Blocking Architecture**: Queries execute asynchronously. If a query runs too long, cancel it instantly with `Ctrl+C` or `Esc` without crashing your session or freezing your screen.
- **Developer Ergonomics**: Full-featured multiline SQL editor with native cursor control, word jumps, clipboard integration, table autocompletion, and templates.
- **Production Safeguards**: Environment tags (PROD, STAGING, DEV) and a dedicated Read-Only mode to prevent accidental mutations on critical databases.

---

## Key Features

### 1. Asynchronous Query Execution with Real-Time Cancellation
- Run heavy analytical queries without freezing the user interface.
- Live progress indicator displays elapsed execution duration.
- Press `Ctrl+C` or `Esc` while a query is running to abort immediately.
- Automatic statement routing: `SELECT` queries run via query context; mutations (`INSERT`, `UPDATE`, `DELETE`, `ALTER`) run via execution context and display affected row counts.

### 2. Interactive SQL Query Editor
- Multiline editing with native hardware cursor, line wrapping, and scrolling.
- Full keyboard navigation: Home, End, PageUp, PageDown, and word jumps (`Ctrl+Left` / `Ctrl+Right`).
- Multi-level Undo (`Ctrl+Z`) and Redo (`Ctrl+Y`).
- Two-way operating system clipboard integration for copying and pasting.
- Table name auto-completion (`Ctrl+T`): Fuzzy table name suggestion matching at cursor position.
- SQL templates and snippets (`Ctrl+S`): Boilerplate templates for `SELECT`, `INSERT`, `UPDATE`, `DELETE`, `JOIN`, and `CREATE TABLE`.
- Direct focus jump: Press `e` from anywhere in the result grid or sidebar to edit the active query.

### 3. Result Data Grid & Inline Cell Editing
- Tabular result viewer with column borders and selection highlight.
- Inspect full cell content with `v`.
- Inline cell editor: Edit table values directly in the grid.
- Pagination controls: Navigate large result sets with `Ctrl+N` (Next) and `Ctrl+P` (Previous).

### 4. Live Process Manager & Query Killer (`F4` or `:process`)
- Real-time monitor of active database threads showing thread ID, user, host, database, command type, elapsed time, state, and query text.
- Safe query termination: Highlight any locked or slow thread and press Enter or `K` to issue `KILL <id>`.

### 5. EXPLAIN Query Execution Plan Analyzer (`F5` or `:explain`)
- Instant performance analysis for queries in the editor.
- Visual breakdown of scanned rows, join types, missing indexes, and potential performance bottlenecks.

### 6. Real-Time Server Health & Metrics (`F6` or `:health`)
- Monitor live queries per second (QPS), active client connections, InnoDB buffer pool hit ratios, and slow query counters.

### 7. Database Schema Diff Generator (`F7` or `:diff`)
- Compare active schema definitions and generate verification scripts.

### 8. Searchable Query Execution History (`F2` or `:history`)
- Search through past query executions with execution timestamps and host metadata.
- Press Enter on any history entry to restore it directly into the active editor.

### 9. Compressed Database Export & Import
- Parallel multi-threaded exporter (`Ctrl+W` / `F8`) dumping schemas and tables into compressed `.sql.gz` archives, reducing file size by up to 95%.
- Selective Export Wizard (`Ctrl+E`): Export specific tables and views to `.sql.gz`, plain `.sql`, CSV, or JSON.
- Database Import Wizard (`F9` or `:import`): Step-by-step importer supporting `.sql` and `.sql.gz` archives with confirmation checks and Read-Only enforcement.

---

## Quick Start

Launch Pheri directly from your terminal:

```bash
./pheri
```

Or connect directly by specifying connection flags:

```bash
./pheri -u root -p password -host 127.0.0.1 -port 3306 -db my_database
```

To connect in Read-Only mode with a production tag:

```bash
./pheri -u root -p password -host 127.0.0.1 -port 3306 -db production_db -env PROD -readonly
```

---

## Installation & Building from Source

### Requirements
- Go 1.22 or higher (Go 1.24 recommended)
- MySQL 5.7+ or MySQL 8.0+ / MariaDB 10.3+

### Build from Source (Linux / macOS)

```bash
git clone https://github.com/vickychhetri/pheri.git
cd pheri
go build -ldflags="-s -w" -o pheri main.go
./pheri
```

### Build from Source (Windows)

Use the included build script to compile with Smart App Control compliant flags:

```powershell
git clone https://github.com/vickychhetri/pheri.git
cd pheri
.\build.ps1
.\pheri.exe
```

Or compile manually using PowerShell:

```powershell
go build -ldflags="-s -w" -o pheri.exe main.go
.\pheri.exe
```

### Pre-Built Binaries
Pre-built release packages are available in the `build/` directory:
- `build/pheri-windows-amd64.exe` (Windows 64-bit)
- `build/pheri-linux-amd64` (Linux 64-bit)
- `build/pheri-linux-arm64` (Linux ARM 64-bit)
- `build/pheri_1.0.0_amd64.deb` (Debian / Ubuntu package)

---

## Command Line Options

| Flag | Description | Default |
| :--- | :--- | :--- |
| `-u` | MySQL username | `root` |
| `-p` | MySQL password | `""` |
| `-host` | Database server hostname or IP address | `127.0.0.1` |
| `-port` | Database server port | `3306` |
| `-db` | Database to select on connect | `""` |
| `-env` | Environment label (`PROD`, `STAGING`, `DEV`) | `DEV` |
| `-readonly` | Enforce read-only mode (`true`/`false`) | `false` |
| `-history` | Show execution history | `false` |

---

## Keyboard Shortcuts

### SQL Editor Shortcuts
| Shortcut | Action |
| :--- | :--- |
| `Ctrl+R` / `Ctrl+Enter` / `F5` | Execute active SQL query |
| `Ctrl+C` / `Esc` (while running) | Cancel in-flight query |
| `Ctrl+T` | Open Table Name Autocomplete modal |
| `Ctrl+S` / `Ctrl+Space` | Open SQL Snippets and Templates modal |
| `Ctrl+K` | Clear query editor text |
| `Ctrl+Z` / `Ctrl+Y` | Undo / Redo text edits |
| `Ctrl+C` | Copy selected text (or full query) to clipboard |
| `Ctrl+V` / `Ctrl+P` | Paste from clipboard |
| `F11` | Toggle fullscreen editor mode |
| `Tab` | Move focus to Run button |
| `Esc` | Return focus to Table List |

### Global Navigation & DBA Tools
| Shortcut | Action |
| :--- | :--- |
| `e` | Jump focus directly to SQL Editor from any grid or list |
| `Tab` | Cycle focus: Sidebar -> Databases -> Editor -> Run -> Grid |
| `F2` / `:history` | Open Searchable Query History modal |
| `F3` | Open Schema Inspector (Columns, Types, Keys, Indexes) |
| `F4` / `:process` | Open Live Process Manager and Query Killer |
| `F5` / `:explain` | Open EXPLAIN Query Execution Plan Analyzer |
| `F6` / `:health` | Open Real-Time Health & Performance Dashboard |
| `F7` / `:diff` | Open Database Schema Diff & Migration Generator |
| `F8` / `Ctrl+W` | High-Speed Parallel Worker Export (.sql.gz) |
| `F9` / `:import` | Open Database Import Wizard |
| `Ctrl+E` / `:export` | Open Selective Export Wizard (CSV, JSON, SQL, Gzip) |
| `Ctrl+B` | Toggle Sidebar Visibility |
| `:format` / `:fmt` | Auto-format and normalize SQL query keywords |
| `:about` | Display About Pheri dialog |
| `:help` | Open Keyboard Shortcuts Help modal |
| `:quit` / `:exit` | Exit Pheri |

### Data Grid Shortcuts
| Shortcut | Action |
| :--- | :--- |
| `e` | Jump to SQL Editor |
| `v` | Inspect highlighted cell in viewer modal |
| `Enter` | Edit selected cell value (inline editing) |
| `Ctrl+N` | Go to next page of results |
| `Ctrl+P` | Go to previous page of results |

---

## Object Search and Filters

Use the search bar above the schema objects list to filter database objects using structured prefixes:

| Filter Prefix | Target Object Type | Example |
| :--- | :--- | :--- |
| `table:` | Tables only | `table:users` |
| `view:` | Views only | `view:active_orders` |
| `procedure:` | Stored Procedures | `procedure:calculate_total` |
| `function:` | User-Defined Functions | `function:format_name` |
| `db:` | Databases | `db:production` |

Typing without a prefix performs a global search across all tables, views, procedures, and functions simultaneously.

---

## Documentation

Full documentation chapters are available in the `docs/` directory:

- [Complete Reference Manual](docs/PHERI_COMPLETE_GUIDE.md): Complete guide in one document.
- [Documentation Index](docs/README.md): Chapter directory.
- [Chapter 1: Architecture & System Design](docs/01_ARCHITECTURE_AND_DESIGN.md)
- [Chapter 2: Installation & Configuration](docs/02_INSTALLATION_AND_CONFIG.md)
- [Chapter 3: UI Layout & Navigation](docs/03_UI_AND_NAVIGATION.md)
- [Chapter 4: SQL Editor, Autocomplete & Snippets](docs/04_SQL_EDITOR_AND_SNIPPETS.md)
- [Chapter 5: Data Grid & Cell Editing](docs/05_DATA_GRID_AND_CELL_EDITING.md)
- [Chapter 6: Performance, Diagnostics & DBA Tools](docs/06_PERFORMANCE_AND_DIAGNOSTICS.md)
- [Chapter 7: Backup & Restore Engines](docs/07_BACKUP_AND_RESTORE.md)
- [Chapter 8: Query History & Storage](docs/08_QUERY_HISTORY_AND_STORAGE.md)
- [Chapter 9: Keyboard Shortcuts & Command Reference](docs/09_KEYBOARD_SHORTCUTS_AND_COMMANDS.md)

---

## Roadmap

Track upcoming improvements in [pheri_feature_roadmap_tracker.csv](pheri_feature_roadmap_tracker.csv) and [GitHub Issues](https://github.com/vickychhetri/pheri/issues):
- Dynamic column sorting in the data grid (ASC / DESC)
- Foreign key relationship traversal
- Interactive row insertion and deletion modals
- Native pure-Go SSH bastion tunneling
- SSL/TLS certificate management
- Terminal ASCII ER relationship diagrams
- Support for PostgreSQL and SQLite drivers

---

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.
