# 09 - Keyboard Shortcuts & Colon Commands Cheat Sheet

This document is a complete reference of all keyboard shortcuts, function key hotkeys, and colon commands (`:command`) supported by Pheri.

---

## 1. Master Keyboard Shortcuts Table

### Global Navigation & Layout Controls

| Shortcut | Description | Context |
| :--- | :--- | :--- |
| `Tab` | Advance focus to next pane (`Sidebar` ➔ `Editor` ➔ `Grid` ➔ `Sidebar`). | Global |
| `BackTab` (`Shift+Tab`)| Move focus to previous pane. | Global |
| `Esc` | Immediately return keyboard focus to the Schema Objects Sidebar. | Global |
| `F11` | Toggle Fullscreen mode for the active panel (Editor or Data Grid). | Global |
| `Ctrl+C` | Gracefully quit Pheri (or cancel current modal). | Global |

---

### SQL Query Editor

| Shortcut | Description | Context |
| :--- | :--- | :--- |
| `Ctrl+R` | Execute current SQL query buffer in the active database. | Editor |
| `Ctrl+T` | Open Table Name Autocompletion popup list. | Editor |
| `Ctrl+S` / `Ctrl+Space`| Open SQL Snippets and Boilerplate Templates picker. | Editor |
| `Ctrl+P` | Paste clipboard text at current cursor position. | Editor |
| `Enter` | Insert new line. | Editor |
| `Backspace` / `Delete` | Delete character before / under cursor. | Editor |
| Arrow Keys (`▲▼◄►`) | Move cursor through rows and columns. | Editor |

---

### Result Data Grid & Viewing

| Shortcut | Description | Context |
| :--- | :--- | :--- |
| `Enter` | Open inline cell editor (on tables with a Primary Key). | Grid |
| `i` | Open Deep Cell Inspector (formatted JSON, full text, or Hex view). | Grid |
| `Ctrl+E` | Open Result Set Export Wizard (export rows to CSV, JSON, Markdown, or SQL). | Grid |
| `Ctrl+N` / `]` | Navigate to Next Page (+100 rows). | Grid |
| `Ctrl+B` / `[` | Navigate to Previous Page (-100 rows). | Grid |
| `PgUp` / `PgDn` | Rapidly scroll visible rows up or down. | Grid |
| `Home` / `End` | Jump to first / last column or row. | Grid |

---

### Schema Sidebar & Object Explorer

| Shortcut | Description | Context |
| :--- | :--- | :--- |
| `Enter` (on Table/View) | Execute `SELECT * FROM object LIMIT 100` and display grid. | Sidebar |
| `Enter` (on Routine) | Fetch and display DDL (`CREATE PROCEDURE` / `CREATE FUNCTION`). | Sidebar |
| `Ctrl+X` | Generate and copy selected table's DDL (`CREATE TABLE`) to clipboard. | Sidebar |
| Type text | Filter object tree in real-time (supports `table:`, `view:`, `procedure:`, etc.). | Sidebar Search |

---

### Performance & DBA Tool Modals

| Function Key | Alternate Command | Description |
| :---: | :---: | :--- |
| **`F1`** | `:help` | Open Global Help and Shortcut Guide. |
| **`F3`** | `:schema` | Open Schema Inspector (column types, keys, and indexes). |
| **`F4`** | `:process` / `:top` | Open Live Process Manager and active query killer. |
| **`F5`** | `:explain` / `:plan`| Open EXPLAIN Query Execution Plan Analyzer and Index Advisor. |
| **`F6`** | `:health` / `:stats`| Open Real-Time Database Health & Performance Dashboard. |
| **`F7`** | `:diff` / `:migrate`| Open Schema Diff and Verification SQL Generator. |
| **`F8`** / `Ctrl+W` | `:export` | Launch High-Speed Multi-Threaded Parallel Worker Export. |
| **`F9`** | `:import` | Open Step-by-Step Database Import Wizard. |
| `Ctrl+H` | `:history` | Open Searchable Query History Explorer & Editor Restorer. |
| `Ctrl+E` | `:export` | Open Selective Schema Object Export Wizard. |
| `Ctrl+Y` | — | Quick export entire schema to `.sql.gz`. |

---

## 2. Colon Commands (`:command`) Reference

You can type vim-style colon commands into the footer command prompt or status bar:

| Command | Aliases | Description |
| :--- | :--- | :--- |
| `:help` | `:h` | Displays the global help guide and keyboard references. |
| `:quit` | `:exit`, `:q` | Exits Pheri gracefully, closing all active database connections. |
| `:clear` | `:cls` | Clears all text in the active SQL Editor buffer. |
| `:format` | `:fmt` | Formats SQL query in editor, aligns clauses, and capitalizes keywords. |
| `:process` | `:top`, `:proc` | Launches the Live Process Manager to view and kill threads. |
| `:explain` | `:expq`, `:plan` | Analyzes execution plan and index efficiency for editor query. |
| `:health` | `:metrics`, `:stats`| Opens the real-time server health and QPS dashboard. |
| `:diff` | `:migrate` | Generates schema integrity verification and migration SQL scripts. |
| `:history` | `:hist` | Opens the searchable query history modal to restore queries. |
| `:export` | `:exp` | Opens the selective database export wizard. |
| `:import` | `:imp` | Launches the interactive database import wizard. |
| `:theme` | `:colors` | Opens the TUI color theme selector. |

---

## 3. Function Key Quick Summary

```
 ┌──────┬─────────────────────────────────────────────────────────────────┐
 │ F1   │ Global Help & Shortcut Manual                                   │
 │ F3   │ Schema Inspector (Columns, Data Types, Primary Keys, Indexes)   │
 │ F4   │ Live Process Manager (Active Threads & Query Killer)            │
 │ F5   │ EXPLAIN Execution Plan Analyzer & Index Recommendations         │
 │ F6   │ Real-Time Server Health & Metrics Dashboard (QPS, Buffer Pool)  │
 │ F7   │ Database Schema Diff & Migration Generator                      │
 │ F8   │ High-Speed Parallel Worker Export (.sql.gz)                     │
 │ F9   │ Database Import Wizard (with Read-Only Protection Barrier)      │
 │ F11  │ Toggle Fullscreen Panel Focus (Editor or Data Grid)             │
 └──────┴─────────────────────────────────────────────────────────────────┘
```
