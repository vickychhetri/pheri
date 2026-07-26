# Pheri

Pheri is a fast, terminal-based database management TUI (Terminal User Interface) built for MySQL in Go. It offers developers an interactive environment with real-time SQL syntax highlighting, live process management, query execution plan analysis, interactive cell editing, and super-fast multi-threaded database exporting directly from the command line.

Latest Release: v3.0.0 (https://github.com/vickychhetri/pheri/releases/tag/v3.0.0)

---

## Features

- Tokenized SQL Syntax Highlighting: Real-time syntax coloring for SQL keywords, functions, data types, strings, identifiers, numbers, comments, and operators.
- Table Name Auto-Completion (Ctrl+T): Instant schema table name suggestions while writing queries.
- SQL Snippets and Templates (Ctrl+S / Ctrl+Space): Boilerplate query generation for SELECT, INSERT, UPDATE, DELETE, JOINs, and DDL statements.
- Super-Fast Concurrent Database Export (Ctrl+Y): Multi-threaded parallel export (130 worker pool) dumping tables, views, procedures, functions, triggers, and events to compressed .gz archives.
- Live Process Manager (F4 / :process): Real-time MySQL process monitor displaying active threads (ID, User, Host, DB, Command, Execution Time, State, and Query Info) with instant query termination capabilities (KILL query_id).
- EXPLAIN Execution Plan Analyzer (F5 / :explain): One-key query execution plan analyzer providing deep insight into join types, index key utilization, scanned row counts, filtered percentages, and execution bottlenecks (detects missing indexes and full table scans).
- Active Panel Focus Highlighting: Visual border indicator denoting active keyboard focus across the query editor, schema list, database browser, and result grid.
- Environment Tags & Read-Only Guard: Tag connections as PROD, STAGING, or DEV with optional read-only enforcement to prevent accidental database mutations.
- Selective Export Wizard (Ctrl+E): Export selected schema objects into raw SQL, CSV, or JSON formats.
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
| `Ctrl+Y` | Super-Fast Concurrent Database Export (130 Parallel Workers) |
| `Ctrl+P` | Paste Clipboard Content |
| `F4` | Open Live Process Manager and Query Killer |
| `F5` | Open EXPLAIN Query Execution Plan Analyzer |
| `F11` | Toggle Fullscreen Mode for Editor or Data Viewer |
| `F3` | Open Schema Inspector (Column Types, Keys, Indexes) |
| `Ctrl+E` | Open Selective Database Export Wizard |
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
