# Pheri: Complete Reference Manual & In-Depth Guide

> **Pheri** is an ultra-fast, terminal-based database management Terminal User Interface (TUI) built for MySQL and MariaDB in Go. It combines the speed, keyboard agility, and minimal footprint of the command line with rich developer tooling: real-time tokenized SQL syntax coloring, an interactive multi-line code editor, live thread management, EXPLAIN execution plan benchmarking, inline data cell editing, and CPU-tuned multi-threaded database exporting.

---

## Table of Contents

1. [Executive Overview & Capabilities](#1-executive-overview--capabilities)
2. [Architecture & System Design](#2-architecture--system-design)
3. [Installation & Terminal Compatibility](#3-installation--terminal-compatibility)
4. [CLI Flags, Connection Profiles & Safety Safeguards](#4-cli-flags-connection-profiles--safety-safeguards)
5. [Workspace Layout & Navigation](#5-workspace-layout--navigation)
6. [SQL Editor, Syntax Highlighter & Snippets](#6-sql-editor-syntax-highlighter--snippets)
7. [Result Data Grid, Pagination & Inline Cell Editing](#7-result-data-grid-pagination--inline-cell-editing)
8. [Performance Diagnostics & DBA Tooling](#8-performance-diagnostics--dba-tooling)
9. [High-Speed Backup & Import Subsystems](#9-high-speed-backup--import-subsystems)
10. [Query History Engine & Persistence](#10-query-history-engine--persistence)
11. [Master Keyboard Shortcuts & Colon Commands](#11-master-keyboard-shortcuts--colon-commands)

---

## 1. Executive Overview & Capabilities

Traditional desktop SQL GUI clients (such as DBeaver, MySQL Workbench, or Navicat) require heavyweight graphics runtimes (JVM, Electron, Qt) consuming hundreds of megabytes of RAM. Running them remotely over SSH forwarding or VNC connections introduces significant latency.

Pheri solves this by running natively in the server's terminal:
- **Instant Launch**: Starts in milliseconds with a tiny memory footprint (<25 MB RAM).
- **SSH & Remote Friendly**: Runs directly over SSH sessions, tmux, WSL, or serial consoles without X11 or VNC forwarding.
- **Custom Lexer**: Real-time ANSI syntax highlighting for SQL keywords, functions, data types, numbers, strings, and comments.
- **Production-Grade Safeguards**: Visual environment tags (`DEV`, `STAGING`, `PROD`) and a hard `Read-Only Guard` preventing accidental mutations.
- **Parallel Export Engine**: Multi-threaded worker pool utilizing all available CPU cores to stream database dumps into compressed `.sql.gz` archives (reducing disk footprint by up to 95%).
- **Interactive Data Mutation**: Direct in-terminal cell editing with automatic Primary Key resolution.
- **Embedded Query Auditing**: Zero-dependency embedded SQLite engine (`phhistory.db`) recording queries and session history.

---

## 2. Architecture & System Design

Pheri is written in Go (1.18+) with a modular architecture separated into distinct layers:

```
┌────────────────────────────────────────────────────────────────────────┐
│                        Terminal / TTY Screen                           │
├────────────────────────────────────────────────────────────────────────┤
│                       gdamore/tcell/v2 Driver                          │
├────────────────────────────────────────────────────────────────────────┤
│                       rivo/tview TUI Framework                         │
├──────────────────────────┬─────────────────────────────────────────────┤
│ ui/connect.go            │ Login form, saved profile switcher, badges  │
│ ui/browser.go            │ Main workspace, schema tree, grid, viewer   │
│ ui/sqleditor.go          │ Lexer, token highlighter, autocompletion    │
│ ui/export.go             │ Goroutine worker pool exporter, gzip stream │
│ ui/import.go             │ Streaming parser, statement splitter        │
│ ui/processlist.go        │ Live thread monitoring, query termination   │
│ ui/tools.go              │ Health dashboard, schema diff, hex viewer   │
│ ui/footer.go             │ Status bar, vim colon commands (:command)   │
├──────────────────────────┴─────────────────────────────────────────────┤
│ dbs/mysql.go             │ Pure Go MySQL network driver (go-sql-driver)│
│ phhistory/               │ Embedded SQLite engine (modernc.org/sqlite) │
│ util/util.go             │ Identifier quoting, metadata AST parsing    │
└────────────────────────────────────────────────────────────────────────┘
```

### Go Package Breakdown

- **`main.go`**: Entry point. Parses flags, resolves binary directory via `os.Executable()`, bootstraps SQLite history, and launches the TUI loop.
- **`dbs/mysql.go`**: Connects via `sql.Open("mysql", dsn)` and verifies connectivity with `Ping()`.
- **`phhistory/`**:
  - `connections.go`: Manages connection profiles stored securely in `~/.pheri_connections.json` (`0600` permissions).
  - `phhistory.go`: Manages `phhistory.db` SQLite database, logs queries with timestamp/host/user, and supports time-based filtering.
- **`ui/`**: Complete terminal user interface implementation.
- **`util/util.go`**: Helper library for identifier quoting (`` `col` ``), routine return types, and clipboard interaction.

---

## 3. Installation & Terminal Compatibility

### Pre-Built Binaries
Download the binary for your architecture from GitHub Releases:
- **Linux (AMD64 / ARM64)**: `pheri-linux-amd64.tar.gz` / `pheri-linux-arm64.tar.gz`
- **Debian / Ubuntu**: `pheri_1.0.0_amd64.deb` (`sudo dpkg -i pheri_1.0.0_amd64.deb`)
- **Snap**: `sudo snap install pheri`
- **Windows**: `pheri-windows-amd64.zip` (`pheri.exe`)

### Go CLI Installation
```bash
go install github.com/vickychhetri/pheri@latest
```

### Building from Source
```bash
git clone https://github.com/vickychhetri/pheri.git
cd pheri
go build -o pheri main.go
./pheri
```

### Supported Terminals
- **Windows**: Windows Terminal, PowerShell 7+, Command Prompt, WSL/WSL2.
- **macOS**: iTerm2, Terminal.app, Alacritty, Kitty.
- **Linux**: GNOME Terminal, Konsole, Alacritty, Kitty, standard Linux TTY console.
- **Multiplexers**: `tmux`, `screen`, `zellij`.

---

## 4. CLI Flags, Connection Profiles & Safety Safeguards

### Command-Line Arguments
Bypass the connection form and connect directly:

```bash
pheri -u <user> -p <password> -host <host> -port <port> -db <database> -env <DEV|STAGING|PROD> [-readonly]
```

| Flag | Default | Description |
| :--- | :---: | :--- |
| `-u` | `""` | MySQL / MariaDB username (e.g. `root`). |
| `-p` | `""` | MySQL password. |
| `-host` | `localhost` | Database server hostname or IP address. |
| `-port` | `3306` | MySQL port. |
| `-db` | `""` | Database schema to auto-open. |
| `-env` | `DEV` | Environment tag: `DEV`, `STAGING`, or `PROD`. |
| `-readonly` | `false` | Enables read-only protection. Blocks mutations and imports. |
| `-history` | `false` | Dumps query history to terminal stdout and exits. |
| `-days` | `30` | Number of days of query history to output. |

### Saved Profiles (`~/.pheri_connections.json`)
Checking "Remember Credentials" in the connection form saves credentials locally with restrictive file permissions (`0600`). In future sessions, select your saved connection from the **Quick Saved Profile** dropdown to auto-fill all connection details.

### Environment Badges & Read-Only Safeguard
- **`DEV`**: Default mode with cyan/green borders.
- **`STAGING`**: Amber/yellow borders indicating testing infrastructure.
- **`PROD`**: High-contrast red warning banners and borders.
- **Read-Only Guard (`-readonly`)**:
  - Automatically intercepts and aborts `INSERT`, `UPDATE`, `DELETE`, `DROP`, `ALTER`, `TRUNCATE`, and `RENAME` statements.
  - Deactivates inline cell editing in the results grid.
  - Blocks the database import wizard (`F9`).

---

## 5. Workspace Layout & Navigation

### Master Workspace Layout
```
┌────────────────────────────────────────────────────────────────────────────────────────┐
│ [DEV] PHERI | Active DB: shop_db | Host: 127.0.0.1:3306 | User: root                   │
├──────────────────────────┬─────────────────────────────────────────────────────────────┤
│ 📁 SCHEMAS & OBJECTS     │ 📝 SQL EDITOR  (Ctrl+R: Run | Ctrl+S: Snippets | Tab: Switch)│
│                          │                                                             │
│ Search: [table:order   ] │  SELECT id, user_id, total_amount, created_at               │
│ ──────────────────────── │  FROM orders                                                │
│ 🗄️ TABLE orders          │  WHERE total_amount > 100                                   │
│ 🗄️ TABLE order_items     │  ORDER BY id DESC;                                          │
│ 👁️ VIEW v_daily_orders   ├─────────────────────────────────────────────────────────────┤
│ ⚡ PROCEDURE sp_checkout │ 🧮 RESULTS: orders (Page 1/4 | Rows: 100 | Time: 1.2ms)     │
│                          │ ┌────┬─────────┬──────────────┬───────────────────────────┐ │
│                          │ │ id │ user_id │ total_amount │ created_at                │ │
│                          │ ├────┼─────────┼──────────────┼───────────────────────────┤ │
│                          │ │ 94 │ 12      │ 320.00       │ 2026-11-18 14:22:01       │ │
│                          │ └────┴─────────┴──────────────┴───────────────────────────┘ │
├──────────────────────────┴─────────────────────────────────────────────────────────────┤
│ PHERI [DEV] | F1:Help | F3:Schema | F4:Top | Ctrl+R:Run | Ctrl+E:Export | Tab:Switch   │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

### Keyboard Navigation & Focus Cycling
- **`Tab`**: Cycles focus forward: `Sidebar` ➔ `SQL Editor` ➔ `Result Grid` ➔ `Sidebar`.
- **`BackTab` (`Shift+Tab`)**: Cycles focus backwards.
- **`Esc`**: Immediately returns focus to the Schema Objects Sidebar from any panel.
- **`F11`**: Expands the focused panel to Fullscreen.

### Object Filtering Syntax
Filter the sidebar object tree using structured prefixes:
- `table:<term>`: Filters tables (e.g. `table:user`).
- `view:<term>`: Filters views (e.g. `view:daily`).
- `procedure:<term>`: Filters stored procedures.
- `function:<term>`: Filters user-defined functions.
- `db:<term>`: Filters schemas in the database list.
- Unprefixed search (e.g. `order`): Searches across all schema objects.

### Quick Actions on Objects
- **Press `Enter` on Table/View**: Automatically runs `SELECT * FROM object LIMIT 100` and displays rows in the grid with inline editing enabled.
- **Press `Enter` on Routine**: Generates and displays the routine's `CREATE` DDL in the editor.
- **Press `Ctrl+X` on Table**: Extracts the full `CREATE TABLE` DDL statement and copies it directly to your system clipboard.

---

## 6. SQL Editor, Syntax Highlighter & Snippets

### Real-Time Tokenized Highlighting
The custom editor in `ui/sqleditor.go` parses SQL into tokens on every keystroke:
- **Keywords** (`SELECT`, `FROM`, `WHERE`, `JOIN`, `ORDER BY`): Bold Lime Green (`[lime::b]`).
- **Functions** (`COUNT`, `NOW`, `CONCAT`, `COALESCE`): Bold Yellow (`[yellow::b]`).
- **Data Types** (`INT`, `VARCHAR`, `TIMESTAMP`): Bold Aqua Cyan (`[aqua::b]`).
- **Strings** (`'active'`, `"completed"`): Green (`[green]`).
- **Identifiers** (`` `orders` ``): Teal Dark Cyan (`[teal]`).
- **Numbers** (`42`, `3.14`): Orange / Red (`[orange]`).
- **Comments** (`-- comment`, `/* block */`): Dim Gray (`[gray]`).

### Auto-Closing Pairs
Typing an opening quote or bracket automatically inserts the matching closing character:
- `'` ➔ `''`
- `"` ➔ `""`
- `` ` `` ➔ ```` `` ````
- `(` ➔ `()`
- `[` ➔ `[]`
- `{` ➔ `{}`

### Table Autocompletion (`Ctrl+T`)
Press `Ctrl+T` to open a dynamic autocomplete popup. Pheri fetches table names from the active schema in memory. Select any table and press `Enter` to insert its backtick-wrapped name at the cursor position.

### Boilerplate Templates (`Ctrl+S` / `Ctrl+Space`)
Press `Ctrl+S` to open the snippet picker:
1. `SELECT * FROM table LIMIT 100;`
2. `SELECT column1, column2 FROM table WHERE id = 1 ORDER BY id DESC;`
3. `INSERT INTO table (col1, col2) VALUES ('val1', 'val2');`
4. `UPDATE table SET col1 = 'val1' WHERE id = 1;`
5. `DELETE FROM table WHERE id = 1;`
6. `INNER JOIN tables template`
7. `CREATE TABLE template`

### SQL Formatter (`:format` / `:fmt`)
Type `:format` in the command prompt to clean up unformatted queries. Pheri capitalizes SQL keywords, standardizes indentation, and normalizes clauses.

---

## 7. Result Data Grid, Pagination & Inline Cell Editing

### Pagination Controls
Query results are buffered in 100-row pages:
- **Next Page**: `Ctrl+N` or `]`
- **Previous Page**: `Ctrl+B` or `[`
- **Top / Bottom**: `Home` / `End`
- **Scroll**: `PgUp` / `PgDn`

### Inline Cell Editing
Developers can edit database values directly without writing manual `UPDATE` statements:
1. Highlight any cell in the result grid.
2. Press `Enter`.
3. Pheri automatically detects the table's primary key(s) by querying `information_schema`.
4. An input dialog opens with the current cell value.
5. Modify the text and press `Enter` to commit.
6. Pheri issues a targeted query:
   ```sql
   UPDATE `database`.`table` SET `column` = 'new_val' WHERE `pk` = 'val' LIMIT 1;
   ```
7. The cell updates immediately in the grid.

### Cell Inspector (`ShowCellInspectorModal`)
Highlight any cell and press `i` to view long strings, formatted JSON structures, or binary BLOB hex representations in a dedicated dialog. Press `c` to copy the content to the clipboard.

### Result Set Export (`Ctrl+E`)
Export the active grid rows to:
- CSV (`.csv`)
- JSON (`.json`)
- Markdown Table (`.md`)
- SQL Insert Statements (`.sql`)

---

## 8. Performance Diagnostics & DBA Tooling

### Live Process Manager (`F4` / `:process`)
Inspect active server threads running via `SHOW FULL PROCESSLIST`:
- Displays Thread ID, User, Host, DB, Command, Execution Time, State, and Query Info.
- Long-running queries (>30 seconds) are highlighted with visual warnings.
- Select any thread and press `Enter` or `K` to safely terminate the connection (`KILL query_id`).

### EXPLAIN Execution Plan Analyzer (`F5` / `:explain`)
Visualizes MySQL's execution plan for the query in the editor:
- **Plan Health Score**: Grades query efficiency (0 - 100).
- **Join Type Classification**: Highlights `system`, `const`, `eq_ref`, `ref`, `range`, `index`, and alerts on dangerous full table scans (`ALL`).
- **Bottleneck Detection**: Flags `Using filesort`, `Using temporary`, and unindexed large table scans.
- **AI Index Recommendations**: Generates concrete `ALTER TABLE ... ADD INDEX` suggestions.

### Real-Time Health Dashboard (`F6` / `:health`)
Monitors live operational health metrics:
- Server version and uptime.
- Total executed queries (QPS load).
- Connected threads vs `max_connections`.
- Active running threads.
- Slow query counts (>1 second).
- InnoDB Buffer Pool read hit ratio (>99% optimal).
- Buffer pool memory allocation size.
- Press `c` to copy the health report to the clipboard.

### Schema Diff & Migration Generator (`F7` / `:diff`)
Audits all tables in the active schema:
- Verifies primary keys and index coverage.
- Identifies unindexed tables.
- Generates schema verification and migration SQL scripts.

---

## 9. High-Speed Backup & Import Subsystems

### Parallel Worker Export (`Ctrl+W` / `F8`)
- Spawns `runtime.NumCPU()` concurrent goroutines.
- Dumps tables, views, procedures, functions, triggers, and events simultaneously.
- Compresses output directly into `.sql.gz` archives, reducing disk storage by up to 95%.
- Fully compatible with standard MySQL tools (`zcat dump.sql.gz | mysql -u root -p db`).

### Selective Export Wizard (`Ctrl+E`)
Interactive step-by-step export:
1. Select/deselect specific database objects.
2. Choose Schema Only (DDL), Data Only (Rows), or Schema + Data.
3. Choose format: Compressed `.sql.gz`, Plain `.sql`, JSON, or CSV.

### Interactive Import Wizard (`F9` / `:import`)
Safe restoration of `.sql` and `.sql.gz` files:
1. **Read-Only Validation**: Rejects imports if Read-Only mode is enabled.
2. **Path Picker**: Selects dump file or export folder.
3. **Danger Warning**: Requires explicit confirmation before executing destructive statements.
4. **Session Optimizations**: Temporarily disables foreign key checks, unique checks, and autocommit for maximum restoration speed.
5. **Streaming Execution**: Parses multi-gigabyte files efficiently with memory-buffered readers.

---

## 10. Query History Engine & Persistence

### SQLite History Engine (`phhistory.db`)
Pheri records every executed statement in a local SQLite database stored beside the binary. The table `pheri_phhistory` captures:
- `query_text`: Full SQL statement.
- `host_ip`, `port`, `user`: Connection identity.
- `db_name`: Active database.
- `created_at`: Execution timestamp.

### CLI History Filtering
Inspect query logs from the command line:
```bash
pheri -history -days 7
```

### In-App History Restorer (`Ctrl+H` / `:history`)
Open the history explorer, search through past queries in real time, and press `Enter` to restore any past query directly into the active SQL editor buffer.

---

## 11. Master Keyboard Shortcuts & Colon Commands

### Master Keyboard Shortcuts

| Shortcut | Context | Action |
| :--- | :--- | :--- |
| `Ctrl+R` | Editor | Execute SQL query buffer in active database. |
| `Ctrl+T` | Editor | Open Table Name Autocompletion popup. |
| `Ctrl+S` / `Ctrl+Space` | Editor | Open SQL Snippet / Boilerplate Templates picker. |
| `Ctrl+P` | Editor | Paste clipboard content. |
| `Enter` | Grid | Edit selected cell inline (requires Primary Key). |
| `i` | Grid | Open Deep Cell Inspector (formatted JSON, text, hex). |
| `Ctrl+E` | Grid / Global | Open Result Set / Selective Database Export Wizard. |
| `Ctrl+N` / `]` | Grid | Next Page (+100 rows). |
| `Ctrl+B` / `[` | Grid | Previous Page (-100 rows). |
| `Ctrl+W` / `F8` | Global | Launch High-Speed Parallel Worker Export (.sql.gz). |
| `Ctrl+H` | Global | Open Searchable Query History Explorer & Restorer. |
| `Ctrl+X` | Sidebar | Extract and copy table DDL (`CREATE TABLE`) to clipboard. |
| `Tab` | Global | Cycle focus: Sidebar ➔ Editor ➔ Grid ➔ Sidebar. |
| `BackTab` (`Shift+Tab`)| Global | Reverse cycle focus. |
| `Esc` | Global | Return focus to Schema Objects Sidebar. |
| `F11` | Global | Toggle Fullscreen mode for active panel. |
| `Ctrl+C` | Global | Gracefully quit Pheri / Cancel modal. |

### Function Keys

| Key | Alternate Command | Action |
| :---: | :---: | :--- |
| `F1` | `:help` | Global Help & Shortcuts Manual. |
| `F3` | `:schema` | Schema Inspector (columns, types, keys, indexes). |
| `F4` | `:process` / `:top`| Live Process Manager and Query Killer. |
| `F5` | `:explain` / `:plan`| EXPLAIN Execution Plan Analyzer and Index Advisor. |
| `F6` | `:health` / `:stats`| Real-Time Database Health & Metrics Dashboard. |
| `F7` | `:diff` / `:migrate`| Schema Diff & Auto-Migration SQL Generator. |
| `F8` | `:export` | High-Speed Parallel Worker Export (.sql.gz). |
| `F9` | `:import` | Database Import Wizard (with Read-Only Protection). |
| `F11` | — | Toggle Fullscreen Panel Focus. |

### Colon Commands (`:command`)

| Command | Aliases | Action |
| :--- | :--- | :--- |
| `:help` | `:h` | Display help guide and shortcut reference. |
| `:quit` | `:exit`, `:q` | Gracefully quit Pheri. |
| `:clear` | `:cls` | Clear active SQL editor buffer. |
| `:format` | `:fmt` | Format SQL query and capitalize keywords. |
| `:process` | `:top`, `:proc` | Open Live Process Manager. |
| `:explain` | `:expq`, `:plan` | Run EXPLAIN execution plan analysis. |
| `:health` | `:metrics`, `:stats`| Open real-time server health dashboard. |
| `:diff` | `:migrate` | Run schema integrity diff and migration generator. |
| `:history` | `:hist` | Open query history explorer. |
| `:export` | `:exp` | Open selective export wizard. |
| `:import` | `:imp` | Open database import wizard. |
| `:theme` | `:colors` | Open TUI color theme switcher. |
