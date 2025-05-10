# Pheri

**Pheri** is a terminal-based user interface for MySQL. It allows you to connect to your MySQL databases and interact with them directly from your terminal — with a clean, minimal UI designed for productivity.
## Features

- **Fast and Lightweight** — Optimized for speed and responsiveness
- **Navigate Tables** — Explore your database schema easily
- **Run Queries** — Write and execute SQL queries directly
- **View Results** — Display result sets in a readable tabular format
- **Keyboard Shortcuts** — Perform common tasks with ease
- **Cross-Platform** — Works on Linux, macOS, and Windows

**Terminals:**  Cmd Prompt, PowerShell, Windows Terminal, WSL, Bash, Zsh, Fish, Dash, Ksh, Tcsh, Terminal.app, iTerm2, Termux, BusyBox, Alacritty, Kitty, Tilda, Guake, Yakuake, Xonsh

# Pheri - User Guide

## Search & Filter Functionality

The **Search & Filter** feature in **Pheri** enables users to quickly locate and interact with database objects such as tables, views, stored procedures, functions, and even entire databases.

---

## How to Use

Start typing into the search bar or command input area. The system supports filtered and unfiltered searches:

### Basic Search

Typing a keyword without a filter prefix searches across all supported object types:

```
customer
```

This will return all tables, views, procedures, and functions that include the word `customer`.

### Filtered Search

Use the following format to filter by type:

```
<type>:<search-term>
```

#### Examples

* `table:customer` → Finds all **tables** with names containing `customer`.
* `view:active` → Filters **views** with `active` in the name.
* `procedure:invoice` → Searches **stored procedures** with `invoice`.
* `function:calc` → Searches **user-defined functions** with `calc`.
* `db:sales` → Lists databases that include `sales` in the name.

---

## Supported Type Filters

| Prefix      | Description                    |
| ----------- | ------------------------------ |
| `table`     | Filters only database tables   |
| `view`      | Filters only views             |
| `procedure` | Filters stored procedures      |
| `function`  | Filters user-defined functions |
| `db`        | Filters available databases    |

---

## On Selection Behavior

Once you select a result from the filtered list:

### TABLE or VIEW

* Automatically runs:

  ```sql
  SELECT * FROM <name> LIMIT 100
  ```
* Displays results in a data grid.
* For **tables**, **inline editing** is enabled.

### PROCEDURE or FUNCTION

* Shows the definition using:

  ```sql
  SELECT routine_definition FROM INFORMATION_SCHEMA.ROUTINES ...
  ```
* Displays the output in a read-only query area.

### DATABASE

* Switches to the selected database as the active working database.

---

## Smart Filtering Logic

* Case-insensitive search.
* Detects and separates the filter type and search keyword automatically.
* Dynamically updates the list view in real-time.

---

## Error Handling

* If an error occurs during selection (e.g. query fails), a modal will be shown with the error message.
* Errors in table editing or fetching routine definitions are also handled and shown via dialog pop-ups.
 

This module provides an interactive Terminal User Interface (TUI) for exploring and interacting with MySQL databases.

## Features

- Select and switch between databases
- Browse tables, views, stored procedures, and functions
- View data from tables/views with a LIMIT of 100 rows
- Execute SQL queries with a query editor
- Edit table data (if supported)
- Maintain query history for reuse
- Keyboard shortcuts for navigation and execution

## UI Layout

| Panel              | Description                                                  |
|--------------------|--------------------------------------------------------------|
| Databases List     | Shows all available databases on the connected server        |
| Tables List        | Lists tables, views, procedures, and functions               |
| Query Editor       | Text area to write and execute SQL queries                   |
| Data Viewer        | Displays query results or contents of a table/view           |
| Control Buttons    | Run query, Save, Load query, Exit                            |

## How to Use

### 1. Launch and Select a Database

- Start the application.
- Use arrow keys to select a database from the list.
- Press `Enter` to activate the selected database.
- Errors (e.g., permission issues) are shown in a modal window.

### 2. Explore Tables, Views, Procedures, and Functions

- Navigate using arrow keys.
- Press `Enter`:
  - On a table/view: Displays the first 100 rows.
  - On a procedure/function: Shows the routine definition.

### 3. Execute Queries

- Write SQL statements in the query editor.
- Press `Ctrl+R` to execute.
- Results are shown in the Data Viewer.

### 4. Keyboard Shortcuts

| Key Combination | Action                           |
|-----------------|----------------------------------|
| Ctrl+R          | Run the query                    |
| Ctrl+F11        | Full-screen query editor         |
| Ctrl+T          | Show tables (custom action)      |
| Ctrl+S          | SQL keywords (custom action)     |
| Ctrl+_          | SQL templates (custom action)    |
| Esc             | Return focus to the tables list  |
| Tab             | Navigate to the Run button       |

## Editing Table Data

- Editing is supported only for tables, not views.
- Select a table, view its data, and enter edit mode.
- Edited values are committed back to the database.

## Query History

- Each successful query is stored in a per-database history.
- Queries can be reloaded and reused later.

## Error Handling

- All errors (database access, SQL issues, etc.) appear in a modal popup.
- Press the "OK" or "Back" button to continue.

## Code Reference

Primary function: `UseDatabase(app *tview.Application, db *sql.DB, dbName string)`

- Switches database using `USE dbName`
- Fetches metadata via:
  - `SHOW DATABASES`
  - `information_schema.tables`
  - `information_schema.routines`
- Core components:
  - `tview.List` for databases/tables
  - `tview.TextArea` for query editing
  - `tview.Table` for displaying results
- Uses helper functions like `ExeQueryToData()`, `ExecuteQuery()`, `EnableCellEditing()`

**Screenshot**
![image](https://github.com/user-attachments/assets/6cd265c8-c9bf-4abd-9aec-a7eca5efbef8)

**Direct Command:**
.\pheri -u root -p 12345678 -host 127.0.0.1 -port 3306

**Optional**
-host 127.0.0.1 -port 3306
