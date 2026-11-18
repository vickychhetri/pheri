# Pheri Documentation

Welcome to the comprehensive documentation suite for **Pheri** — a fast, terminal-based database management Terminal User Interface (TUI) built for MySQL and MariaDB in Go.

Pheri brings the speed and low overhead of the terminal together with modern database IDE capabilities: live process management, multi-threaded parallel compressed exports, an interactive SQL editor with real-time tokenized syntax highlighting, EXPLAIN plan analyzers, inline cell editing, and environment safeguards.

---

## Documentation Navigation

This documentation suite is organized into modular guides covering every aspect of Pheri, from internal architecture to daily operational shortcuts:

| # | Chapter / Guide | Description | Target Audience |
| :---: | :--- | :--- | :--- |
| **01** | [Architecture & Design](01_ARCHITECTURE_AND_DESIGN.md) | Internal Go packages, TUI lifecycle, concurrency model, data flow, and dependencies. | Developers, Contributors |
| **02** | [Installation & Configuration](02_INSTALLATION_AND_CONFIG.md) | Pre-built binaries, Go install, building from source, CLI flags, saved profiles, and environment tags. | All Users, DevOps |
| **03** | [UI & Navigation Guide](03_UI_AND_NAVIGATION.md) | Workspace layout, active focus borders, panel cycling, fullscreen toggle, and object search filtering. | All Users |
| **04** | [SQL Editor & Snippets](04_SQL_EDITOR_AND_SNIPPETS.md) | Custom TUI editor, tokenized syntax highlighting, autocompletion (`Ctrl+T`), snippet templates (`Ctrl+S`), and auto-formatting. | Developers, DBAs |
| **05** | [Data Grid & Cell Editing](05_DATA_GRID_AND_CELL_EDITING.md) | Interactive result grid, pagination, inline cell editing with primary key resolution, and cell inspector. | Developers, DBAs |
| **06** | [Performance & Diagnostics](06_PERFORMANCE_AND_DIAGNOSTICS.md) | Live Process Manager (`F4`), EXPLAIN query analyzer (`F5`), Health Dashboard (`F6`), and Schema Diff (`F7`). | DBAs, DevOps, Sysadmins |
| **07** | [Backup & Restore Engines](07_BACKUP_AND_RESTORE.md) | High-speed parallel worker export (`Ctrl+W`/`F8`), selective wizard (`Ctrl+E`), Gzip `.sql.gz` streaming, and import wizard (`F9`). | DBAs, DevOps |
| **08** | [Query History & Local Storage](08_QUERY_HISTORY_AND_STORAGE.md) | Embedded SQLite (`phhistory.db`), history table schema, CLI retention flags, and the query restorer modal (`Ctrl+H`). | All Users |
| **09** | [Shortcuts & Colon Commands](09_KEYBOARD_SHORTCUTS_AND_COMMANDS.md) | Complete cheat sheet of keyboard shortcuts, function keys (`F1`-`F11`), and colon commands (`:command`). | Quick Reference |
| 📖 | [Complete Guide (All-in-One)](PHERI_COMPLETE_GUIDE.md) | The entire manual compiled into a single unified reference document for offline reading. | Offline Reading, Full Manual |

---

## Interactive HTML Documentation

Pheri also ships with an interactive, searchable web documentation interface located in this directory:
- [Interactive HTML Documentation](index.html) (Features live filterable shortcuts, command line generator, and quick search).

---

## Quick Feature Overview

```
 ┌────────────────────────────────────────────────────────────────────────┐
 │                              PHERI TUI                                 │
 ├────────────────────────────────┬───────────────────────────────────────┤
 │ 📁 Schema & Object Explorer    │ 📝 Custom SQL Editor                  │
 │   • Prefix filtering:          │   • Real-time token syntax coloring   │
 │     table:, view:, procedure:, │   • Table autocompletion (Ctrl+T)     │
 │     function:, db:             │   • Snippets / Templates (Ctrl+S)     │
 │   • DDL generation & copy      │   • Auto-closing quotes & brackets    │
 ├────────────────────────────────┼───────────────────────────────────────┤
 │ ⚡ Performance & Diagnostics    │ 🗄️ Result Grid & Cell Editing         │
 │   • Live Processlist (F4)      │   • 100-row paginated view            │
 │   • EXPLAIN Analyzer (F5)      │   • Inline editing with PK resolution │
 │   • Health Dashboard (F6)      │   • Long text & Hex Cell Inspector    │
 │   • Schema Diff (F7)           │   • Export to CSV, JSON, Markdown     │
 ├────────────────────────────────┴───────────────────────────────────────┤
 │ 🚀 High-Speed Backup & Restore                                         │
 │   • Multi-threaded worker pool export to compressed .sql.gz (F8/Ctrl+W)│
 │   • Selective Export Wizard for schema objects (Ctrl+E)                │
 │   • Safe Interactive Import Wizard with destructive warning (F9)       │
 │   • Read-Only Guard & PROD / STAGING / DEV environment badges          │
 └────────────────────────────────────────────────────────────────────────┘
```

---

## Suggested Reading Paths

- **I just want to start using Pheri**: Read [Installation & Configuration](02_INSTALLATION_AND_CONFIG.md) then keep [Shortcuts & Colon Commands](09_KEYBOARD_SHORTCUTS_AND_COMMANDS.md) open.
- **I need to export or restore a database**: Head straight to [Backup & Restore Engines](07_BACKUP_AND_RESTORE.md).
- **I want to analyze slow queries and tune my database**: Check [Performance & Diagnostics](06_PERFORMANCE_AND_DIAGNOSTICS.md) and [SQL Editor & Snippets](04_SQL_EDITOR_AND_SNIPPETS.md).
- **I want to contribute to Pheri's codebase**: Start with [Architecture & Design](01_ARCHITECTURE_AND_DESIGN.md).
