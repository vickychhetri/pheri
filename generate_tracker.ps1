param(
    [string]$OutputPath = "c:\Users\vicky\Desktop\pj\pheri\pheri_feature_roadmap_tracker.xlsx",
    [string]$CsvPath = "c:\Users\vicky\Desktop\pj\pheri\pheri_feature_roadmap_tracker.csv"
)

# 1. Define complete dataset of improvement tasks
$tasks = @(
    [PSCustomObject]@{
        ID = "PHERI-01"
        Status = "Done"
        Priority = "P0 - Critical"
        Phase = "Phase 1: Core Reliability"
        Category = "Core Engine"
        Feature = "Async Query Execution with Cancellation"
        CurrentState = "Synchronous db.Query blocks UI event loop completely during slow queries"
        ProposedChange = "Execute queries in background goroutines using context.WithCancel. Display progress timer and spinner"
        TargetFiles = "ui/browser.go, dbs/mysql.go"
        Effort = "2-3 days"
        Notes = "Essential to stop UI freezing on heavy queries"
    },
    [PSCustomObject]@{
        ID = "PHERI-02"
        Status = "To Do"
        Priority = "P0 - Critical"
        Phase = "Phase 1: Core Reliability"
        Category = "Core Engine"
        Feature = "Active Query Abort on Ctrl+C / Esc"
        CurrentState = "Ctrl+C kills entire Pheri app; query keeps running on MySQL server"
        ProposedChange = "Intercept Ctrl+C during query execution; call cancel() and issue KILL QUERY <id>"
        TargetFiles = "ui/browser.go, ui/footer.go"
        Effort = "1-2 days"
        Notes = "Prevents abandoned long queries from locking MySQL tables"
    },
    [PSCustomObject]@{
        ID = "PHERI-03"
        Status = "To Do"
        Priority = "P0 - Critical"
        Phase = "Phase 1: Core Reliability"
        Category = "Data Grid"
        Feature = "Fix Grid NULL & Empty Value Type Safety"
        CurrentState = "Visual string '[gray]NULL' or '[gray]EMPTY' is submitted as literal string to database during cell edit"
        ProposedChange = "Introduce typed GridCell struct separating RawValue from Visual markup. Add dedicated NULL toggle"
        TargetFiles = "ui/browser.go"
        Effort = "2 days"
        Notes = "Critical bug fix for database integrity"
    },
    [PSCustomObject]@{
        ID = "PHERI-04"
        Status = "To Do"
        Priority = "P0 - Critical"
        Phase = "Phase 1: Core Reliability"
        Category = "Core Engine"
        Feature = "Differentiate QueryContext vs ExecContext"
        CurrentState = "DML mutations (UPDATE, INSERT, DELETE) execute via db.Query, failing to report rows affected"
        ProposedChange = "Detect statement type: run SELECT via QueryContext; run DML via ExecContext and display rows affected + execution time"
        TargetFiles = "ui/browser.go"
        Effort = "1 day"
        Notes = "Standard database client execution behavior"
    },
    [PSCustomObject]@{
        ID = "PHERI-05"
        Status = "To Do"
        Priority = "P0 - Critical"
        Phase = "Phase 1: Core Reliability"
        Category = "Connection"
        Feature = "Connection Timeouts & Network Safeguards"
        CurrentState = "Unconfigured connection DSN hangs indefinitely on unreachable hosts or dropped packets"
        ProposedChange = "Add timeout=5s, readTimeout=60s, writeTimeout=60s to MySQL connection string"
        TargetFiles = "dbs/mysql.go, ui/connect.go"
        Effort = "1 day"
        Notes = "Immediate feedback on offline/firewalled database servers"
    },
    [PSCustomObject]@{
        ID = "PHERI-06"
        Status = "To Do"
        Priority = "P0 - Critical"
        Phase = "Phase 1: Core Reliability"
        Category = "Connection"
        Feature = "Connection Pool Sizing & Lifecycle"
        CurrentState = "Connection pool left at driver defaults without explicit limits or idle cleanups"
        ProposedChange = "Configure SetMaxOpenConns(25), SetMaxIdleConns(5), SetConnMaxLifetime(5m)"
        TargetFiles = "dbs/mysql.go"
        Effort = "0.5 day"
        Notes = "Prevents exhausting MySQL max_connections limit"
    },
    [PSCustomObject]@{
        ID = "PHERI-07"
        Status = "To Do"
        Priority = "P1 - High"
        Phase = "Phase 1: Core Reliability"
        Category = "Security"
        Feature = "Encrypted Connection Profile Passwords"
        CurrentState = "Passwords saved in plain text in ~/.pheri_connections.json"
        ProposedChange = "Encrypt password strings using AES-256-GCM with machine-derived key or user master passphrase"
        TargetFiles = "phhistory/connections.go"
        Effort = "2 days"
        Notes = "Security compliance for stored credentials"
    },
    [PSCustomObject]@{
        ID = "PHERI-08"
        Status = "To Do"
        Priority = "P1 - High"
        Phase = "Phase 2: Transactions & CRUD"
        Category = "Transactions"
        Feature = "Auto-Commit Toggle (F2 / :autocommit)"
        CurrentState = "MySQL always runs in autocommit=1; mutations commit immediately with no rollback option"
        ProposedChange = "Add F2 toggle for Auto-Commit mode; maintain active *sql.Tx when autocommit is off"
        TargetFiles = "ui/browser.go, ui/footer.go"
        Effort = "2-3 days"
        Notes = "Safety essential for staging & production database work"
    },
    [PSCustomObject]@{
        ID = "PHERI-09"
        Status = "To Do"
        Priority = "P1 - High"
        Phase = "Phase 2: Transactions & CRUD"
        Category = "Transactions"
        Feature = "Explicit Transaction Controls (:commit / :rollback)"
        CurrentState = "No transaction controls available in the UI"
        ProposedChange = "Implement :commit (Ctrl+Enter) and :rollback hotkeys with status bar uncommitted mutations counter"
        TargetFiles = "ui/browser.go, ui/footer.go"
        Effort = "1-2 days"
        Notes = "Developer peace of mind when testing complex mutations"
    },
    [PSCustomObject]@{
        ID = "PHERI-10"
        Status = "To Do"
        Priority = "P1 - High"
        Phase = "Phase 2: Transactions & CRUD"
        Category = "Transactions"
        Feature = "Uncommitted Transaction Exit Barrier"
        CurrentState = "Exiting or changing DB silently drops or abandons open transactions"
        ProposedChange = "Prompt with modal warning: 'You have uncommitted changes. [COMMIT & EXIT] [ROLLBACK & EXIT] [CANCEL]'"
        TargetFiles = "ui/browser.go, ui/connect.go"
        Effort = "1 day"
        Notes = "Prevents accidental data loss or hanging locks"
    },
    [PSCustomObject]@{
        ID = "PHERI-11"
        Status = "To Do"
        Priority = "P1 - High"
        Phase = "Phase 2: Transactions & CRUD"
        Category = "Data Grid"
        Feature = "Dynamic Column Header Sorting (ASC / DESC)"
        CurrentState = "Selecting column headers does nothing; cannot sort result sets interactively"
        ProposedChange = "Pressing 's' or Enter on column header toggles Unsorted -> ASC (▲) -> DESC (▼) with query re-execution"
        TargetFiles = "ui/browser.go"
        Effort = "2 days"
        Notes = "Top requested UX feature in tabular database tools"
    },
    [PSCustomObject]@{
        ID = "PHERI-12"
        Status = "To Do"
        Priority = "P1 - High"
        Phase = "Phase 2: Transactions & CRUD"
        Category = "Data Grid"
        Feature = "Foreign Key Traversal (Follow Relationship)"
        CurrentState = "Foreign keys are displayed as plain numbers/strings without navigation"
        ProposedChange = "Highlight FK cells with '🔗'; pressing Ctrl+J or F jumps directly to referenced table & row. Backspace returns"
        TargetFiles = "ui/browser.go, util/util.go"
        Effort = "3 days"
        Notes = "Killer feature for navigating normalized relational schemas"
    },
    [PSCustomObject]@{
        ID = "PHERI-13"
        Status = "To Do"
        Priority = "P1 - High"
        Phase = "Phase 2: Transactions & CRUD"
        Category = "Data Grid"
        Feature = "Interactive Row Insertion Modal ('+' / 'a')"
        CurrentState = "Users must write manual INSERT INTO statements in SQL editor"
        ProposedChange = "Press '+' or 'a' in grid to open dynamic form with input fields for each column, respecting types & nullable flags"
        TargetFiles = "ui/browser.go"
        Effort = "2-3 days"
        Notes = "Greatly accelerates manual data entry and testing"
    },
    [PSCustomObject]@{
        ID = "PHERI-14"
        Status = "To Do"
        Priority = "P1 - High"
        Phase = "Phase 2: Transactions & CRUD"
        Category = "Data Grid"
        Feature = "Interactive Row Deletion ('d' / Del)"
        CurrentState = "Users must write manual DELETE FROM statements"
        ProposedChange = "Press 'd' or Delete on highlighted row to prompt confirmation and execute targeted DELETE WHERE pk = val LIMIT 1"
        TargetFiles = "ui/browser.go"
        Effort = "1-2 days"
        Notes = "Safe one-click row removal with confirmation modal"
    },
    [PSCustomObject]@{
        ID = "PHERI-15"
        Status = "To Do"
        Priority = "P2 - Medium"
        Phase = "Phase 2: Transactions & CRUD"
        Category = "Data Grid"
        Feature = "Quick Copy Row as JSON / TSV / SQL"
        CurrentState = "Only individual cell text or full grid export is available"
        ProposedChange = "Add Ctrl+Shift+C menu to copy highlighted row as JSON object, TSV row, or INSERT statement to clipboard"
        TargetFiles = "ui/browser.go"
        Effort = "1 day"
        Notes = "Convenient for copying data into code, tests, or bug reports"
    },
    [PSCustomObject]@{
        ID = "PHERI-16"
        Status = "Done"
        Priority = "P0 - Critical"
        Phase = "Phase 3: Editor Superpowers"
        Category = "SQL Editor"
        Feature = "Professional Interactive SQL Editor & Autocompletion"
        CurrentState = "Simulated TextView clobbered input every 500ms; disconnected TextArea caused empty query runs"
        ProposedChange = "Re-architect on tview.TextArea with 2-way OS clipboard, Undo/Redo, word jumps, hotkeys, snippets, and autocomplete"
        TargetFiles = "ui/sqleditor.go, ui/browser.go, ui/footer.go"
        Effort = "2 days"
        Notes = "Essential developer ergonomics fix - unlocks smooth query authoring"
    },
    [PSCustomObject]@{
        ID = "PHERI-17"
        Status = "To Do"
        Priority = "P2 - Medium"
        Phase = "Phase 3: Editor Superpowers"
        Category = "SQL Editor"
        Feature = "Line Numbers Margin & Cursor Sync"
        CurrentState = "Editor has no line numbers gutter; difficult to correlate SQL error line numbers"
        ProposedChange = "Render synchronized line number gutter (1 |, 2 |, 3 |) in left margin of SQL editor"
        TargetFiles = "ui/sqleditor.go"
        Effort = "1-2 days"
        Notes = "Makes debugging multi-line queries effortless"
    },
    [PSCustomObject]@{
        ID = "PHERI-18"
        Status = "Done"
        Priority = "P2 - Medium"
        Phase = "Phase 3: Editor Superpowers"
        Category = "SQL Editor"
        Feature = "Multi-Level Undo / Redo Buffer (Ctrl+Z / Ctrl+Y)"
        CurrentState = "No undo capability; accidental edits or deletions cannot be reverted"
        ProposedChange = "Integrated native TextArea mutation stack with Ctrl+Z undo and Ctrl+Y redo"
        TargetFiles = "ui/sqleditor.go"
        Effort = "1 day"
        Notes = "Standard editor requirement to prevent accidental loss of query text"
    },
    [PSCustomObject]@{
        ID = "PHERI-19"
        Status = "To Do"
        Priority = "P2 - Medium"
        Phase = "Phase 3: Editor Superpowers"
        Category = "SQL Editor"
        Feature = "Multi-Statement Batch Execution & Tabs"
        CurrentState = "Only one statement executed at a time; multiple queries cause syntax errors"
        ProposedChange = "Split multi-statement scripts by semicolon and render tabbed result grids for multiple SELECT outputs"
        TargetFiles = "ui/sqleditor.go, ui/browser.go"
        Effort = "3 days"
        Notes = "Allows running complete migration and audit scripts in one pass"
    },
    [PSCustomObject]@{
        ID = "PHERI-20"
        Status = "To Do"
        Priority = "P1 - High"
        Phase = "Phase 4: Security & Connectivity"
        Category = "Connectivity"
        Feature = "Native Pure-Go SSH Bastion Tunneling"
        CurrentState = "Databases inside private VPCs / AWS RDS require running external ssh -L tunnels manually"
        ProposedChange = "Integrate golang.org/x/crypto/ssh into connection profile. Register custom dialer with mysql.RegisterDialContext"
        TargetFiles = "dbs/mysql.go, ui/connect.go, phhistory/connections.go"
        Effort = "3-4 days"
        Notes = "Enables one-click connection to cloud and corporate databases"
    },
    [PSCustomObject]@{
        ID = "PHERI-21"
        Status = "To Do"
        Priority = "P2 - Medium"
        Phase = "Phase 4: Security & Connectivity"
        Category = "Security"
        Feature = "SSL / TLS Certificate Management"
        CurrentState = "Unencrypted TCP connections only"
        ProposedChange = "Support TLS modes (Disable, Require, Verify-CA, Verify-Full) with custom CA, cert, and key file inputs"
        TargetFiles = "dbs/mysql.go, ui/connect.go"
        Effort = "2 days"
        Notes = "Required for enterprise cloud databases that enforce SSL"
    },
    [PSCustomObject]@{
        ID = "PHERI-22"
        Status = "To Do"
        Priority = "P3 - Low"
        Phase = "Phase 4: Security & Connectivity"
        Category = "Connectivity"
        Feature = "Unix Domain Socket Connection Support"
        CurrentState = "Only TCP host:port supported"
        ProposedChange = "Support socket paths (e.g. /var/run/mysqld/mysqld.sock) in connection form for zero-latency local connections"
        TargetFiles = "dbs/mysql.go, ui/connect.go"
        Effort = "1 day"
        Notes = "Common for local Linux servers and Docker containers"
    },
    [PSCustomObject]@{
        ID = "PHERI-23"
        Status = "To Do"
        Priority = "P2 - Medium"
        Phase = "Phase 5: Architecture & Diagnostics"
        Category = "Diagnostics"
        Feature = "Terminal ASCII / Unicode ER Relationship Diagram"
        CurrentState = "Users must inspect individual table foreign keys manually"
        ProposedChange = "Generate visual ASCII/Unicode relationship graph (:er / :diagram) showing connected tables and cardinality"
        TargetFiles = "ui/tools.go"
        Effort = "3-4 days"
        Notes = "High-impact visual feature for understanding database schemas"
    },
    [PSCustomObject]@{
        ID = "PHERI-24"
        Status = "To Do"
        Priority = "P2 - Medium"
        Phase = "Phase 5: Architecture & Diagnostics"
        Category = "Diagnostics"
        Feature = "InnoDB Deadlock & Engine Inspector (:deadlock)"
        CurrentState = "Users have no visibility into lock conflicts or engine deadlocks"
        ProposedChange = "Parse SHOW ENGINE INNODB STATUS and extract recent deadlock transactions, conflicting locks, and SQL statements"
        TargetFiles = "ui/tools.go"
        Effort = "2 days"
        Notes = "Critical for diagnosing production application concurrency issues"
    },
    [PSCustomObject]@{
        ID = "PHERI-25"
        Status = "To Do"
        Priority = "P2 - Medium"
        Phase = "Phase 5: Architecture & Diagnostics"
        Category = "Diagnostics"
        Feature = "Database Storage & Disk Usage Visualizer (:storage)"
        CurrentState = "No visibility into table size on disk"
        ProposedChange = "Query information_schema and display bar chart of top tables by Data Length and Index Length"
        TargetFiles = "ui/tools.go"
        Effort = "1-2 days"
        Notes = "Quickly identifies bloated tables and indexes needing maintenance"
    },
    [PSCustomObject]@{
        ID = "PHERI-26"
        Status = "To Do"
        Priority = "P1 - High"
        Phase = "Phase 5: Architecture & Diagnostics"
        Category = "Architecture"
        Feature = "Decompose browser.go Monolith (3,621 lines)"
        CurrentState = "All UI functions, event handlers, and data structures reside in one 114KB file"
        ProposedChange = "Refactor into modular subpackages: ui/grid (table & CRUD), ui/sidebar (tree & search), ui/editor, ui/dba"
        TargetFiles = "ui/browser.go -> ui/grid/, ui/sidebar/, ui/state/"
        Effort = "4-5 days"
        Notes = "Greatly improves maintainability, unit testing, and team contributions"
    },
    [PSCustomObject]@{
        ID = "PHERI-27"
        Status = "To Do"
        Priority = "P1 - High"
        Phase = "Phase 5: Architecture & Diagnostics"
        Category = "Architecture"
        Feature = "Centralized AppState Context (Eliminate Globals)"
        CurrentState = "Dozens of mutable package-level globals (activeGridDB, isEditingEnabled, dataTable) risk race conditions"
        ProposedChange = "Encapsulate session state in an AppState struct passed explicitly to components"
        TargetFiles = "ui/browser.go, ui/connect.go"
        Effort = "3 days"
        Notes = "Prevents subtle state desynchronization bugs when switching connections"
    },
    [PSCustomObject]@{
        ID = "PHERI-28"
        Status = "To Do"
        Priority = "P3 - Low"
        Phase = "Phase 5: Architecture & Diagnostics"
        Category = "Architecture"
        Feature = "Abstract Generic DatabaseDriver Interface"
        CurrentState = "Codebase hardcoded exclusively to MySQL network driver"
        ProposedChange = "Create DatabaseDriver interface: Connect, GetDatabases, GetTables, ExecuteQuery, ExplainQuery for Postgres & SQLite"
        TargetFiles = "dbs/driver.go, dbs/mysql.go, dbs/postgres.go, dbs/sqlite.go"
        Effort = "4-5 days"
        Notes = "Lays the foundation for multi-database engine support"
    }
)

# 2. Export CSV
$tasks | Export-Csv -Path $CsvPath -NoTypeInformation -Encoding utf8
Write-Output "CSV successfully exported to: $CsvPath"

# 3. Create OpenXML XLSX Package
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path "$tempDir\_rels" -Force | Out-Null
New-Item -ItemType Directory -Path "$tempDir\xl\_rels" -Force | Out-Null
New-Item -ItemType Directory -Path "$tempDir\xl\worksheets" -Force | Out-Null

# [Content_Types].xml
$contentTypes = @"
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>
"@
Set-Content -LiteralPath "$tempDir\[Content_Types].xml" -Value $contentTypes -Encoding utf8

# _rels/.rels
$rootRels = @"
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>
"@
Set-Content -Path "$tempDir\_rels\.rels" -Value $rootRels -Encoding utf8

# xl/_rels/workbook.xml.rels
$wbRels = @"
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>
"@
Set-Content -Path "$tempDir\xl\_rels\workbook.xml.rels" -Value $wbRels -Encoding utf8

# xl/workbook.xml
$workbookXml = @"
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="Pheri Roadmap Tracker" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>
"@
Set-Content -Path "$tempDir\xl\workbook.xml" -Value $workbookXml -Encoding utf8

# xl/styles.xml
$stylesXml = @"
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="7">
    <font><name val="Segoe UI"/><sz val="10"/></font>
    <font><name val="Segoe UI"/><sz val="11"/><b/><color rgb="FFFFFFFF"/></font>
    <font><name val="Segoe UI"/><sz val="10"/><b/><color rgb="FF9C0006"/></font>
    <font><name val="Segoe UI"/><sz val="10"/><b/><color rgb="FF9C5700"/></font>
    <font><name val="Segoe UI"/><sz val="10"/><b/><color rgb="FF002060"/></font>
    <font><name val="Segoe UI"/><sz val="10"/><b/><color rgb="FF006100"/></font>
    <font><name val="Segoe UI"/><sz val="10"/><b/><color rgb="FF1F2937"/></font>
  </fonts>
  <fills count="8">
    <fill><patternFill patternType="none"/></fill>
    <fill><patternFill patternType="gray125"/></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FF0F172A"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFFFC7CE"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFFFEB9C"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFDDEBF7"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFC6EFCE"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFE2E8F0"/></patternFill></fill>
  </fills>
  <borders count="2">
    <border><left/><right/><top/><bottom/><diagonal/></border>
    <border>
      <left style="thin"><color rgb="FFE2E8F0"/></left>
      <right style="thin"><color rgb="FFE2E8F0"/></right>
      <top style="thin"><color rgb="FFE2E8F0"/></top>
      <bottom style="thin"><color rgb="FFE2E8F0"/></bottom>
    </border>
  </borders>
  <cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
  <cellXfs count="8">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="1" xfId="0"/>
    <xf numFmtId="0" fontId="1" fillId="2" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="2" fillId="3" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="3" fillId="4" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="4" fillId="5" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="5" fillId="6" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="6" fillId="7" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="0" fillId="0" borderId="1" xfId="0" applyBorder="1" applyAlignment="1"><alignment vertical="center"/></xf>
  </cellXfs>
</styleSheet>
"@
Set-Content -Path "$tempDir\xl\styles.xml" -Value $stylesXml -Encoding utf8

# Helper to escape XML
function Escape-Xml([string]$str) {
    if ([string]::IsNullOrEmpty($str)) { return "" }
    return $str.Replace("&", "&amp;").Replace("<", "&lt;").Replace(">", "&gt;").Replace('"', "&quot;").Replace("'", "&apos;")
}

# Helper column index to letter
function Get-ColLetter([int]$colIndex) {
    $dividend = $colIndex
    $colName = ""
    while ($dividend -gt 0) {
        $modulo = ($dividend - 1) % 26
        $colName = [char](65 + $modulo) + $colName
        $dividend = [int](($dividend - $modulo) / 26)
    }
    return $colName
}

# xl/worksheets/sheet1.xml
$headers = @("ID", "Status", "Priority", "Phase", "Category", "Feature / Task Name", "Current State in Pheri", "Target Enhancement", "Target Files", "Effort", "Notes")
$colWidths = @(12, 14, 16, 26, 16, 36, 42, 50, 28, 12, 35)

$sb = New-Object System.Text.StringBuilder
[void]$sb.AppendLine('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>')
[void]$sb.AppendLine('<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">')
[void]$sb.AppendLine('  <sheetViews><sheetView tabSelected="1" workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>')
[void]$sb.AppendLine('  <sheetFormatPr defaultRowHeight="20"/>')

# Define columns
[void]$sb.AppendLine('  <cols>')
for ($i = 0; $i -lt $colWidths.Count; $i++) {
    $colNum = $i + 1
    $w = $colWidths[$i]
    [void]$sb.AppendLine("    <col min=""$colNum"" max=""$colNum"" width=""$w"" customWidth=""1""/>")
}
[void]$sb.AppendLine('  </cols>')

[void]$sb.AppendLine('  <sheetData>')

# Header Row (Row 1)
[void]$sb.AppendLine('    <row r="1" ht="26" customHeight="1">')
for ($c = 0; $c -lt $headers.Count; $c++) {
    $colLetter = Get-ColLetter ($c + 1)
    $val = Escape-Xml $headers[$c]
    [void]$sb.AppendLine("      <c r=""$colLetter`1"" s=""1"" t=""inlineStr""><is><t>$val</t></is></c>")
}
[void]$sb.AppendLine('    </row>')

# Data Rows
$rowNum = 2
foreach ($t in $tasks) {
    $vals = @($t.ID, $t.Status, $t.Priority, $t.Phase, $t.Category, $t.Feature, $t.CurrentState, $t.ProposedChange, $t.TargetFiles, $t.Effort, $t.Notes)
    
    # Choose style
    # Style 2: P0, 3: P1, 4: P2, 5: P3
    $pStyle = 0
    if ($t.Priority -like "*P0*") { $pStyle = 2 }
    elseif ($t.Priority -like "*P1*") { $pStyle = 3 }
    elseif ($t.Priority -like "*P2*") { $pStyle = 4 }
    elseif ($t.Priority -like "*P3*") { $pStyle = 5 }

    # Status style: Style 6 (Gray pill)
    $statusStyle = 6

    [void]$sb.AppendLine("    <row r=""$rowNum"" ht=""22"" customHeight=""1"">")
    for ($c = 0; $c -lt $vals.Count; $c++) {
        $colLetter = Get-ColLetter ($c + 1)
        $val = Escape-Xml $vals[$c]
        
        $styleId = 7
        if ($c -eq 0) { $styleId = 1 } # ID
        elseif ($c -eq 1) { $styleId = $statusStyle } # Status
        elseif ($c -eq 2) { $styleId = $pStyle } # Priority
        
        [void]$sb.AppendLine("      <c r=""$colLetter$rowNum"" s=""$styleId"" t=""inlineStr""><is><t>$val</t></is></c>")
    }
    [void]$sb.AppendLine('    </row>')
    $rowNum++
}

[void]$sb.AppendLine('  </sheetData>')

# AutoFilter
$lastColLetter = Get-ColLetter $headers.Count
$lastRow = $rowNum - 1
[void]$sb.AppendLine("  <autoFilter ref=""A1:$lastColLetter$lastRow""/>")

[void]$sb.AppendLine('</worksheet>')

Set-Content -Path "$tempDir\xl\worksheets\sheet1.xml" -Value $sb.ToString() -Encoding utf8

# 4. Zip into .xlsx
Add-Type -AssemblyName System.IO.Compression.FileSystem
if (Test-Path $OutputPath) { Remove-Item $OutputPath -Force }
[System.IO.Compression.ZipFile]::CreateFromDirectory($tempDir, $OutputPath)

# Cleanup
Remove-Item -Path $tempDir -Recurse -Force

Write-Output "Successfully generated Excel tracker: $OutputPath"
