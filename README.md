# Pheri

Pheri is a fast, terminal-based database management TUI (Terminal User Interface) built for MySQL in Go. It offers developers an interactive environment with real-time SQL syntax highlighting, live process management, query execution plan analysis, interactive cell editing, and super-fast multi-threaded database exporting directly from the command line.

Latest Release: v3.0.0 (https://github.com/vickychhetri/pheri/releases/tag/v3.0.0)

---

## Features

- Tokenized SQL Syntax Highlighting: Real-time syntax coloring for SQL keywords, functions, data types, strings, identifiers, numbers, comments, and operators.
- Table Name Auto-Completion (Ctrl+T): Instant schema table name suggestions while writing queries.
- SQL Snippets and Templates (Ctrl+S / Ctrl+Space): Boilerplate query generation for SELECT, INSERT, UPDATE, DELETE, JOINs, and DDL statements.
- High-Speed Compressed Export (.sql.gz): Dumps tables, views, procedures, functions, triggers, and events into lightweight Gzip archives reducing file sizes by up to 95%.
- High-Speed Parallel Worker Export (Ctrl+W / F8): Separate multi-threaded exporter utilizing a CPU-tuned worker pool for rapid database dumping.
- Database Import Wizard (F9 / :import): Interactive step-by-step importer supporting .sql and .sql.gz dumps with location selection, overwrite permission confirmation, and Read-Only protection.
- Live Process Manager (F4 / :process): Real-time MySQL process monitor displaying active threads (ID, User, Host, DB, Command, Execution Time, State, and Query Info) with instant query termination capabilities (KILL query_id).
- EXPLAIN Execution Plan Analyzer (F5 / :explain): Multi-view performance analyzer providing health scores, operator tree graphs, latency benchmarking, and AI index recommendations.
- Real-Time Health & Metrics Dashboard (F6 / :health): Monitor live QPS, client threads, InnoDB buffer pool hit ratios, and slow query counts.
- Database Schema Diff & Migration Generator (F7 / :diff): Compare active database tables and auto-generate verification SQL scripts.
- Searchable Query History & Restorer (Ctrl+H / :history): Search past executions and restore SQL into active editor.
- SQL Auto-Formatter (:format): Clean up query formatting and normalize SQL keywords.
- Active Panel Focus Highlighting: Visual border indicator denoting active keyboard focus across the query editor, schema list, database browser, and result grid.
- Environment Tags & Read-Only Guard: Tag connections as PROD, STAGING, or DEV with optional read-only enforcement to prevent accidental database mutations.
- Selective Export Wizard (Ctrl+E): Export selected schema objects into Compressed .sql.gz, Plain .sql, CSV, or JSON formats.
- Schema Inspector (F3): Detailed structural view of table column definitions, primary keys, data types, and index configurations.
- Smart Object Search & Filters: Search objects across schemas using prefix queries (table:, view:, procedure:, function:, db:).
- Inline Cell Editing: Directly edit table row data inside the terminal grid.
- Saved Connection Profiles: Local storage for saved database credentials (~/.pheri_connections.json).

---

## Installation

### Pre-Built Binaries
Download ready-to-run binaries for Linux, macOS, and Windows from the official GitHub Release page:
https://github.com/vickychhetri/pheri/releases/tag/v3.0.0

### Build from Source

Requirements: Go 1.18 or higher.

```bash
git clone https://github.com/vickychhetri/pheri.git
cd pheri
go build -o pheri main.go
./pheri
```

### Install via Go CLI

```bash
go install github.com/vickychhetri/pheri@latest
```

---

## Documentation

Pheri provides interactive terminal database management for MySQL and MariaDB databases. Complete interactive web documentation with search, command generator, and shortcut filtering is available at:

👉 **[Interactive Detailed HTML Documentation](docs/index.html)**

### Core Documentation Summary

#### 1. Connection & CLI Flags
Launch Pheri interactively or directly via command line arguments:
```bash
./pheri -u <user> -p <password> -host <host> -port <port> -db <database> -env <PROD|STAGING|DEV> -readonly
```

#### 2. SQL Query Editor & Snippets
- **Syntax Highlighting**: Real-time tokenized coloring for SQL keywords, functions, identifiers, numbers, strings, and comments.
- **Table Autocompletion (`Ctrl+T`)**: Instant schema table suggestions at cursor position.
- **SQL Templates (`Ctrl+S` / `Ctrl+Space`)**: Insert selectable boilerplates for `SELECT`, `INSERT`, `UPDATE`, `DELETE`, `JOIN`, and DDL statements.

#### 3. Live Process Manager (`F4` or `:process`)
- Real-time active thread monitor displaying thread ID, user, host, database, command type, execution time, and query state.
- **Instant Query Termination**: Select any thread and press `Enter` or `K` to safely terminate locked queries (`KILL query_id`).

#### 4. EXPLAIN Execution Plan Analyzer (`F5` or `:explain`)
- Instant execution plan visualizer for SQL queries in the editor.
- Detects bottlenecks, scanned row counts, join types (`ALL`, `index`, `range`, `ref`, `eq_ref`, `const`), and provides warnings for missing indexes or full table scans.

#### 5. Super-Fast Compressed Database Export & Import
- **High-Speed Parallel Export (`Ctrl+W` / `F8`)**: Dumps database schemas, tables, and routines into a compressed `.sql.gz` file (reducing disk footprint up to 95%). Fully compatible with DBeaver, phpMyAdmin, MySQL Workbench, Navicat, and `zcat file.sql.gz | mysql`.
- **Interactive Import Wizard (`F9` or `:import`)**: Step-by-step importer supporting `.sql` and `.sql.gz` files. Prompts for source location, displays explicit destructive operation warnings before running, and enforces Read-Only safeguards.

#### 6. Environment Safeguards & Read-Only Guard
- Tag connections with `-env PROD` for red warning highlights across all terminal panels.
- Pass `-readonly` to intercept and block non-SELECT mutations (`DELETE`, `UPDATE`, `DROP`, `ALTER`, `TRUNCATE`, `INSERT`) and disable database import actions.

---

## CLI Flags & Usage

Launch Pheri directly with command line flags to bypass the connection screen:

```bash
./pheri -u root -p mypassword -host 127.0.0.1 -port 3306 -db sales_db -env PROD -readonly
```

### Available Command Line Options

| Flag | Description | Default |
| :--- | :--- | :--- |
| `-u` | MySQL username | `root` |
| `-p` | MySQL password | `""` |
| `-host` | Server host address | `127.0.0.1` |
| `-port` | Server port | `3306` |
| `-db` | Default database | `""` |
| `-env` | Environment tag (PROD, STAGING, DEV) | `DEV` |
| `-readonly` | Enable read-only safeguard (`true`/`false`) | `false` |

---

## Keyboard Shortcuts

| Key | Action |
| :--- | :--- |
| `Ctrl+R` | Run SQL Query |
| `Ctrl+T` | Table Name Auto-Completion |
| `Ctrl+S` / `Ctrl+Space` | Insert SQL Snippets and Templates |
| `Ctrl+Y` | Standard Folder Export (.sql.gz) |
| `Ctrl+W` / `F8` | High-Speed Parallel Worker Export (.sql.gz) |
| `F9` / `:import` | Database Import Wizard (Location Picker & Confirmation) |
| `Ctrl+E` | Open Selective Database Export Wizard |
| `Ctrl+P` | Paste Clipboard Content |
| `F4` | Open Live Process Manager and Query Killer |
| `F5` | Open EXPLAIN Query Execution Plan Analyzer |
| `F6` / `:health` | Real-Time Server Health & Performance Dashboard |
| `F7` / `:diff` | Database Schema Diff & Migration Generator |
| `Ctrl+H` / `:history` | Searchable Query History & Session Restorer |
| `:format` / `:fmt` | Auto-Format SQL Keywords & Clauses |
| `F11` | Toggle Fullscreen Mode for Editor or Data Viewer |
| `F3` | Open Schema Inspector (Column Types, Keys, Indexes) |
| `Ctrl+X` | Copy Table / View DDL to Clipboard |
| `Tab` | Cycle Keyboard Focus |
| `Esc` | Return Focus to Schema Objects Sidebar |

---

## Object Search and Filter System

Use the search bar in the sidebar to isolate specific database objects using structured prefix queries:

```
<prefix>:<keyword>
```

### Filter Examples

| Prefix | Target | Example |
| :--- | :--- | :--- |
| `table:` | Database Tables | `table:orders` |
| `view:` | Database Views | `view:active_users` |
| `procedure:` | Stored Procedures | `procedure:generate_invoice` |
| `function:` | User Functions | `function:calculate_tax` |
| `db:` | Databases | `db:production` |

- Unprefixed Search: Typing a keyword directly (e.g. `customer`) searches across all tables, views, procedures, and functions.
- Selection Action: Pressing Enter on a table or view runs `SELECT * FROM table LIMIT 100` and activates inline editing. Pressing Enter on a procedure or function displays its DDL definition.

---

## Screenshots

- Connection Manager: https://github.com/user-attachments/assets/dae70040-9043-4de0-9794-bf1252e6c65b
- Main Workspace: https://github.com/user-attachments/assets/d111fe23-b7f5-4593-8036-288f716f5c62
- Schema Explorer: https://github.com/user-attachments/assets/6cd265c8-c9bf-4abd-9aec-a7eca5efbef8

---

## Supported Terminals

Linux Terminal, macOS Terminal.app, iTerm2, Windows Terminal, PowerShell, Command Prompt, WSL, Alacritty, Kitty, Termux, Zsh, Bash, Fish.

---

## Contributing & Open Source Vision

Pheri is an open-source initiative dedicated to simplifying database management inside SSH and server terminal environments with an ultra-low memory footprint.

We welcome contributions from developers worldwide! Future roadmap items include:
- Multi-database engine support (PostgreSQL, SQLite, MariaDB).
- Advanced column-level AST auto-completion.
- Enhanced TUI themes and custom key bindings.

Feel free to submit Pull Requests, report issues, or suggest new features on [GitHub](https://github.com/vickychhetri/pheri).

---

## License

MIT License. See LICENSE for details.
