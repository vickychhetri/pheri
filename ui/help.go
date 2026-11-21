package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// GetLaymanHelpGuide returns the complete, plain-English reference guide for Pheri.
func GetLaymanHelpGuide() string {
	var b strings.Builder

	b.WriteString(" [yellow::b]PHERI DATABASE MANAGER - USER GUIDE & COMMAND REFERENCE[-::-]\n")
	b.WriteString(" [gray]A fast, keyboard-driven MySQL client designed entirely for your terminal.[-::-]\n\n")

	b.WriteString(" [aqua::b]1. UNDERSTANDING THE SCREEN (THE THREE WORKSPACES)[-::-]\n")
	b.WriteString("   [white::b]Left Sidebar[-::-]\n")
	b.WriteString("     Lists all your databases and their objects (tables, views, stored procedures,\n")
	b.WriteString("     functions, and triggers). Browse, filter, or select objects to inspect.\n")
	b.WriteString("   [white::b]Top Panel (SQL Editor)[-::-]\n")
	b.WriteString("     Your workspace to write, edit, and run SQL queries. Features table name\n")
	b.WriteString("     autocomplete, code snippets, and instant query cancellation.\n")
	b.WriteString("   [white::b]Bottom Panel (Results Data Grid)[-::-]\n")
	b.WriteString("     Spreadsheet-like table displaying query results. Supports cell editing, row\n")
	b.WriteString("     pagination, large text / JSON inspection, and selective data export.\n")
	b.WriteString("   [white::b]Bottom Footer Bar[-::-]\n")
	b.WriteString("     Shows active environment ([DEV], [STAGING], [PROD]), live query execution\n")
	b.WriteString("     timer, quick shortcut reminders, and the ':' command launcher.\n\n")

	b.WriteString(" [aqua::b]2. GETTING AROUND (NAVIGATION & WINDOW CONTROLS)[-::-]\n")
	b.WriteString("   [lime::b]F1 / ?[-::-]             Open this Help Guide & Command Reference\n")
	b.WriteString("   [lime::b]Tab / Shift+Tab[-::-]    Cycle focus between Sidebar, Query Editor, and Results Grid\n")
	b.WriteString("   [lime::b]e[-::-]                  Quick jump straight to the SQL Editor from anywhere\n")
	b.WriteString("   [lime::b]Ctrl+B[-::-]             Toggle Sidebar (collapse for full-width editor, expand back)\n")
	b.WriteString("   [lime::b]F11[-::-]                Toggle Fullscreen SQL Editor (press F11 or Esc to restore)\n")
	b.WriteString("   [lime::b]F12[-::-]                Switch Theme (Nordic Frost, Tokyo Night, Catppuccin, Slate)\n")
	b.WriteString("   [lime::b]Esc[-::-]                Close any active modal dialog or popup\n\n")

	b.WriteString(" [aqua::b]3. BROWSING & SEARCHING DATABASE OBJECTS (SIDEBAR)[-::-]\n")
	b.WriteString("   [lime::b]Up / Down + Enter[-::-]  Select and switch your active database\n")
	b.WriteString("   [lime::b]Enter on Table[-::-]     Instantly load rows into the Results Grid (LIMIT 100)\n")
	b.WriteString("   [lime::b]Enter on View/Proc[-::-] Load definition (CREATE statement) into the SQL Editor\n")
	b.WriteString("   [lime::b]Filter Search Box[-::-]  Type to search objects by name. Prefix filters supported:\n")
	b.WriteString("     [yellow]table:name[-]     Search tables only (e.g. table:users)\n")
	b.WriteString("     [yellow]view:name[-]      Search views only (e.g. view:order_summary)\n")
	b.WriteString("     [yellow]proc:name[-]      Search stored procedures only (e.g. proc:calculate)\n")
	b.WriteString("     [yellow]func:name[-]      Search functions only (e.g. func:total)\n")
	b.WriteString("     [yellow]trig:name[-]      Search triggers only (e.g. trig:audit_log)\n")
	b.WriteString("     [yellow]db:name[-]        Search database names\n")
	b.WriteString("   [lime::b]F3 / Ctrl+K[-::-]        Table Schema Inspector (view columns, types, keys & indexes)\n")
	b.WriteString("   [lime::b]Ctrl+X[-::-]             Copy Table DDL and INSERT data directly to system clipboard\n\n")

	b.WriteString(" [aqua::b]4. WRITING & RUNNING SQL QUERIES (SQL EDITOR)[-::-]\n")
	b.WriteString("   [lime::b]Ctrl+R / Ctrl+Enter[-::-] Execute current SQL query\n")
	b.WriteString("   [lime::b]Ctrl+C / Esc[-::-]        Cancel Query: Safely abort running query if stuck or slow\n")
	b.WriteString("   [lime::b]Ctrl+T[-::-]              Table Autocomplete: Search and insert table names at cursor\n")
	b.WriteString("   [lime::b]Ctrl+S[-::-]              SQL Snippets: Insert common SQL clauses (JOIN, WHERE, etc.)\n")
	b.WriteString("   [lime::b]F2 / :autocommit[-::-]    Auto-Commit Toggle: Toggle between Auto-Commit ON and manual transactions\n")
	b.WriteString("   [lime::b]Ctrl+H / :history[-::-]   Query History: Search and reload previous queries into editor\n")
	b.WriteString("   [lime::b]:commit / :rollback[-::-] Commit or Rollback pending transaction mutations\n")
	b.WriteString("   [lime::b]:format / :fmt[-::-]      Auto-Format SQL: Clean indentation and uppercase keywords\n")
	b.WriteString("   [lime::b]:clear / :cls[-::-]       Clear SQL Editor text\n")
	b.WriteString("   [lime::b]Ctrl+C / Ctrl+P[-::-]     Copy selected text (or entire query) / Paste from clipboard\n\n")

	b.WriteString(" [aqua::b]5. VIEWING & EDITING DATA (RESULTS DATA GRID)[-::-]\n")
	b.WriteString("   [lime::b]Arrow Keys[-::-]          Move cell cursor (Up, Down, Left, Right)\n")
	b.WriteString("   [lime::b]Enter[-::-]               In-Place Cell Editor: Modify cell value and save to database\n")
	b.WriteString("   [lime::b]v[-::-]                   Cell Content Inspector: View large text or pretty-printed JSON\n")
	b.WriteString("     [gray]Inside Inspector: Press [lime]c[-] to copy content to system clipboard[-]\n")
	b.WriteString("   [lime::b]Ctrl+N[-::-]              Next Page: Load the next 100 rows\n")
	b.WriteString("   [lime::b]Ctrl+P[-::-]              Previous Page: Load the previous 100 rows\n")
	b.WriteString("   [lime::b]Ctrl+E[-::-]              Export Wizard: Save table or query results to CSV, JSON, or SQL\n\n")

	b.WriteString(" [aqua::b]6. DEVOPS & DATABASE HEALTH DIAGNOSTICS (IN PLAIN ENGLISH)[-::-]\n")
	b.WriteString("   [lime::b]F4 / :process / :top[-::-] Live Process Manager & Query Killer\n")
	b.WriteString("     [white]What it does:[-] Like Task Manager for your database. Shows connected clients\n")
	b.WriteString("     and running queries. Select any slow or stuck query and press [yellow]k[-] to kill it.\n")
	b.WriteString("   [lime::b]F5 / :explain[-::-]        Query Execution Plan Analyzer\n")
	b.WriteString("     [white]What it does:[-] Asks MySQL how it will execute your query before running it.\n")
	b.WriteString("     Shows index lookups, row estimates, and execution tree to fix slow queries.\n")
	b.WriteString("   [lime::b]F6 / :health[-::-]         Real-Time Server Health Dashboard\n")
	b.WriteString("     [white]What it does:[-] Live dashboard showing server uptime, Queries Per Second (QPS),\n")
	b.WriteString("     active connections, InnoDB buffer pool hit ratio, and slow query counts.\n")
	b.WriteString("   [lime::b]F7 / :diff[-::-]           Schema Diff & Migration Generator\n")
	b.WriteString("     [white]What it does:[-] Compares two databases (e.g. dev vs production) to detect\n")
	b.WriteString("     missing tables, altered columns, or missing indexes, and writes migration SQL.\n")
	b.WriteString("   [lime::b]Ctrl+Y[-::-]               Direct Full Database Backup (Gzip)\n")
	b.WriteString("     [white]What it does:[-] One-key full database dump. Backs up and compresses all tables,\n")
	b.WriteString("     views, procedures, functions, and triggers into .sql.gz with a live progress log.\n")
	b.WriteString("   [lime::b]Ctrl+W / F8[-::-]          Parallel High-Speed Database Export\n")
	b.WriteString("     [white]What it does:[-] Multi-threaded database exporter that dumps tables concurrently\n")
	b.WriteString("     into a compressed .sql.gz archive for maximum backup speed.\n")
	b.WriteString("   [lime::b]F9 / :import[-::-]         Database Import Wizard\n")
	b.WriteString("     [white]What it does:[-] Restores database dumps (.sql or .sql.gz) with safety checks.\n\n")

	b.WriteString(" [aqua::b]7. FOOTER COMMAND LAUNCHER (:commands)[-::-]\n")
	b.WriteString("   Type [yellow]:[-] anywhere or focus the bottom command bar to run quick commands:\n")
	b.WriteString("   [yellow]:help, :h[-]         Open this comprehensive Help Guide\n")
	b.WriteString("   [yellow]:about, :info[-]     Display version, author, and project information\n")
	b.WriteString("   [yellow]:process, :top[-]    Open Live Process Manager & Query Killer\n")
	b.WriteString("   [yellow]:explain, :plan[-]   Analyze SQL Execution Plan\n")
	b.WriteString("   [yellow]:health, :stats[-]   Open Server Health Dashboard\n")
	b.WriteString("   [yellow]:diff, :migrate[-]   Open Schema Diff & Migration Generator\n")
	b.WriteString("   [yellow]:history, :hist[-]   Open Searchable Query History\n")
	b.WriteString("   [yellow]:autocommit, :tx[-]  Toggle Auto-Commit Mode (F2)\n")
	b.WriteString("   [yellow]:commit, :c[-]       Commit active transaction\n")
	b.WriteString("   [yellow]:rollback, :rb[-]    Rollback active transaction\n")
	b.WriteString("   [yellow]:export, :exp[-]     Open Selective Database Export Wizard\n")
	b.WriteString("   [yellow]:import, :imp[-]     Open Database Import Wizard\n")
	b.WriteString("   [yellow]:format, :fmt[-]     Auto-format SQL in Query Editor\n")
	b.WriteString("   [yellow]:clear, :cls[-]      Clear SQL in Query Editor\n")
	b.WriteString("   [yellow]:theme, :colors[-]   Open Color Theme Picker\n")
	b.WriteString("   [yellow]:quit, :q, :exit[-]  Exit Pheri safely\n")

	return b.String()
}

// ShowComprehensiveHelpModal displays the complete layman help guide in a scrollable, theme-styled modal.
func ShowComprehensiveHelpModal(app *tview.Application) {
	view := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	view.SetBackgroundColor(CurrentTheme.SurfaceBg)
	StyleModalBox(view.Box, "Pheri User Guide & Command Reference")

	view.SetText(GetLaymanHelpGuide())

	statusBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(" [yellow::b]ESC / ENTER[-::-] Close Guide  [gray]│[-] [aqua::b]↑ / ↓ / PgUp / PgDn[-::-] Scroll Guide  [gray]│[-] [white::b]:help[-::-] command ")
	statusBar.SetBackgroundColor(CurrentTheme.HeaderBg)

	contentFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(view, 0, 1, true).
		AddItem(statusBar, 1, 0, false)

	outerFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 1, 0, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 2, 0, false).
			AddItem(contentFlex, 0, 1, true).
			AddItem(nil, 2, 0, false),
			0, 1, true).
		AddItem(nil, 1, 0, false)

	closeModal := func() {
		if mainFlex != nil {
			orig := CreateLayoutWithFooter(app, mainFlex)
			app.SetRoot(orig, true)
		}
	}

	view.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if e.Key() == tcell.KeyEscape || e.Key() == tcell.KeyEnter {
			closeModal()
			return nil
		}
		return e
	})

	outerFlex.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if e.Key() == tcell.KeyEscape || e.Key() == tcell.KeyEnter {
			closeModal()
			return nil
		}
		return e
	})

	app.SetRoot(CreateLayoutWithFooter(app, outerFlex), true).SetFocus(view)
}

// ShowHelpModal is an alias for ShowComprehensiveHelpModal for backwards compatibility.
func ShowHelpModal(app *tview.Application) {
	ShowComprehensiveHelpModal(app)
}
