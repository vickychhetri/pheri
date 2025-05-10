# Pheri

**Pheri** is a terminal-based user interface for MySQL. It allows you to connect to your MySQL databases and interact with them directly from your terminal — with a clean, minimal UI designed for productivity.
## 🚀 Features

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
 
**Screenshot**
![image](https://github.com/user-attachments/assets/6cd265c8-c9bf-4abd-9aec-a7eca5efbef8)

**Direct Command:**
.\pheri -u root -p 12345678 -host 127.0.0.1 -port 3306

**Optional**
-host 127.0.0.1 -port 3306
