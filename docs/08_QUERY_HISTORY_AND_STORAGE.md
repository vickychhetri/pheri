# 08 - Query History & Local Storage

This guide explains how Pheri persists executed queries, stores saved connection profiles, and restores past queries into the active editor.

---

## 1. Embedded SQLite History Engine (`phhistory.db`)

Pheri records every query executed through the SQL editor or interactive grid in a local, embedded SQLite database.

- **Zero External Dependencies**: Uses the pure Go SQLite driver `modernc.org/sqlite`. No C compiler or CGO is required.
- **Storage Location**: The SQLite database file `phhistory.db` is stored directly alongside the Pheri executable (`filepath.Dir(execPath)`).
- **Persistent Audit Trail**: Records what queries were run, on which database, against which host, by which user, and at what exact timestamp.

### History Table Schema (`pheri_phhistory`)
```sql
CREATE TABLE IF NOT EXISTS pheri_phhistory (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    query_text  TEXT NOT NULL,
    host_ip     VARCHAR(15) NOT NULL,    
    db_name     VARCHAR(100) NOT NULL,     
    user        VARCHAR(100) NOT NULL,  
    port        VARCHAR(10) NOT NULL,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

---

## 2. CLI History Inspection & Maintenance

You can inspect and filter query history directly from your terminal shell without entering the TUI:

```bash
pheri -history [-days <int>] [-months <int>] [-years <int>] [-history_file <path>]
```

### Command Line Options:
- **`-history`**: Instructs Pheri to dump query history records to `stdout` and exit immediately.
- **`-days <n>`**: Filters history for queries executed in the past `n` days (default: 30 days).
- **`-months <n>`**: Filters history for queries executed in the past `n` months (default: 12 months).
- **`-years <n>`**: Filters history for queries executed in the past `n` years (default: 1 year).
- **`-history_file <path>`**: Exports the query history to a custom file or imports historical records.

### CLI Example:
```bash
# Output all queries executed in the last 7 days:
pheri -history -days 7
```

Sample output:
```
[2026-11-18 14:22:01] [root@127.0.0.1:3306/shop_db] SELECT * FROM orders WHERE total_amount > 100;
[2026-11-18 14:25:30] [root@127.0.0.1:3306/shop_db] UPDATE orders SET status = 'shipped' WHERE id = 94;
```

---

## 3. Searchable Query History Modal (`Ctrl+H` / `:history`)

While working in the main workspace, press `Ctrl+H` or type `:history` to open the interactive **Query History Explorer**:

```
┌────────────────────────────────────────────────────────────────────────────────────────┐
│ 🕒 SEARCHABLE QUERY HISTORY & RESTORER                                                 │
│ Search: [SELECT orders                                                               ] │
├────────────────────────────────────────────────────────────────────────────────────────┤
│ 1. [2026-11-18 14:22:01] (shop_db)                                                     │
│    SELECT * FROM orders WHERE total_amount > 100 ORDER BY id DESC;                     │
│                                                                                        │
│ 2. [2026-11-18 13:45:12] (shop_db)                                                     │
│    SELECT user_id, COUNT(*) FROM orders GROUP BY user_id;                              │
│                                                                                        │
│ 3. [2026-11-17 19:10:04] (analytics_db)                                                │
│    SELECT DATE(created_at), SUM(total_amount) FROM orders GROUP BY 1;                  │
└────────────────────────────────────────────────────────────────────────────────────────┘
[▲/▼: Navigate | ENTER: Restore Query to Editor | c: Copy Query | ESC: Close]
```

### History Explorer Capabilities:
1. **Live Search Filtering**: Type keywords to filter history records by SQL clause, table name, or database name in real time.
2. **Instant Editor Restoration**: Press `Enter` on any query to close the modal and paste the historical SQL query directly into your SQL Editor buffer.
3. **Clipboard Copy**: Press `c` to copy the selected query to your system clipboard without modifying your current editor text.

---

## 4. Connection Profiles Storage (`~/.pheri_connections.json`)

Saved connection profiles are persisted in the user's home directory:
- **File Path**: `~/.pheri_connections.json`
- **File Permissions**: Created with `0600` (read and write access restricted strictly to the user).
- **Cached Parameters**: Host, port, username, encrypted password string, environment classification tag (`DEV`, `STAGING`, `PROD`), and read-only toggle flag.
