package ui

import (
	"bufio"
	"compress/gzip"
	"database/sql"
	"fmt"
	"io"
	"mysql-tui/util"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// getFileSize returns the size of a file in bytes
func getFileSize(filename string) int64 {
	info, err := os.Stat(filename)
	if err != nil {
		return 0
	}
	return info.Size()
}

// formatSize formats bytes to human readable format
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func importAllObjects(progressChan chan string, dbName string) {
	util.SaveLog("=== IMPORT STARTED ===")
	util.SaveLog(fmt.Sprintf("Importing database: %s", dbName))

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&collation=utf8mb4_unicode_ci&parseTime=false&multiStatements=true&maxAllowedPacket=1073741824",
		User, Pass, Host, Port, dbName)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		progressChan <- fmt.Sprintf("[red]Failed to connect to DB: %v", err)
		close(progressChan)
		return
	}
	defer db.Close()

	// Optimize connection pool for large imports
	db.SetMaxOpenConns(25) // Reduce concurrent connections to avoid memory pressure
	db.SetMaxIdleConns(10) // Keep fewer idle connections
	db.SetConnMaxLifetime(time.Minute * 30)

	// Set session variables for import
	if err := setImportSessionVariables(db); err != nil {
		progressChan <- fmt.Sprintf("[red]Failed to set session variables: %v", err)
		close(progressChan)
		return
	}

	// Find the latest export folder
	latestFolder, err := findLatestExportFolder(dbName)
	if err != nil {
		progressChan <- fmt.Sprintf("[red]Failed to find export folder: %v", err)
		close(progressChan)
		return
	}

	progressChan <- fmt.Sprintf("[green]Found export folder: %s", filepath.Base(latestFolder))

	dbFolder := filepath.Join(latestFolder, dbName)
	if _, err := os.Stat(dbFolder); os.IsNotExist(err) {
		progressChan <- fmt.Sprintf("[red]Database folder not found: %s", dbFolder)
		close(progressChan)
		return
	}

	// Define import order with patterns
	importFiles := []struct {
		pattern string
		name    string
	}{
		{"*_table.sql*", "tables"},
		{"*_viewddl.sql*", "view structures"},
		{"*_view.sql*", "views"},
		{"*_procedure.sql*", "procedures"},
		{"*_function.sql*", "functions"},
		{"*_trigger.sql*", "triggers"},
		{"*_event.sql*", "events"},
	}

	totalSteps := len(importFiles)
	completed := 0
	startTime := time.Now()

	for _, fileType := range importFiles {
		filePath := filepath.Join(dbFolder, fileType.pattern)
		matches, err := filepath.Glob(filePath)
		if err != nil || len(matches) == 0 {
			progressChan <- fmt.Sprintf("[yellow]No %s file found, skipping", fileType.name)
			completed++
			continue
		}

		for _, gzFile := range matches {
			fileSize := getFileSize(gzFile)
			progressChan <- fmt.Sprintf("[blue]Importing %s (%s)...", fileType.name, formatSize(fileSize))

			// Process file in streaming mode
			successCount, errorCount, err := processSQLFileStream(gzFile, progressChan, db, fileType.name)

			if err != nil {
				progressChan <- fmt.Sprintf("[red]Failed to import %s: %v", fileType.name, err)
			} else {
				progressChan <- fmt.Sprintf("[green]Imported %s: %d statements (errors: %d)",
					fileType.name, successCount, errorCount)
			}

			// Commit periodically to free memory
			if successCount > 0 && successCount%5000 == 0 {
				_, err = db.Exec("COMMIT")
				if err == nil {
					progressChan <- fmt.Sprintf("[blue]Committed batch for %s", fileType.name)
				}
			}
		}

		completed++
		elapsed := time.Since(startTime).Seconds()
		percent := float64(completed) / float64(totalSteps) * 100
		progressChan <- fmt.Sprintf("[cyan]Progress: %.1f%% complete (ETA: %s)", percent,
			time.Duration(elapsed/float64(completed)*float64(totalSteps-completed))*time.Second)
	}

	// Final commit
	_, err = db.Exec("COMMIT")
	if err != nil {
		util.SaveLog(fmt.Sprintf("Final commit error: %v", err))
	}

	// Restore session variables
	restoreImportSessionVariables(db)

	progressChan <- "[green]Import completed successfully!"
	close(progressChan)
}

func findLatestExportFolder(dbName string) (string, error) {
	util.SaveLog("findLatestExportFolder: Starting search")

	exportPath := filepath.Join(".", "export")
	util.SaveLog(fmt.Sprintf("Looking for export folder at: %s", exportPath))

	if _, err := os.Stat(exportPath); os.IsNotExist(err) {
		util.SaveLog(fmt.Sprintf("ERROR: Export folder not found at %s", exportPath))
		return "", fmt.Errorf("export folder not found at %s", exportPath)
	}

	entries, err := os.ReadDir(exportPath)
	if err != nil {
		util.SaveLog(fmt.Sprintf("ERROR: Failed to read export folder: %v", err))
		return "", err
	}

	util.SaveLog(fmt.Sprintf("Found %d entries in export folder", len(entries)))

	var validFolders []string
	for _, entry := range entries {
		if !entry.IsDir() {
			util.SaveLog(fmt.Sprintf("Skipping non-directory: %s", entry.Name()))
			continue
		}

		fullPath := filepath.Join(exportPath, entry.Name())
		dbFolderPath := filepath.Join(fullPath, dbName)

		util.SaveLog(fmt.Sprintf("Checking folder: %s", entry.Name()))
		util.SaveLog(fmt.Sprintf("  Checking for database folder: %s", dbFolderPath))

		// Check if this folder contains our database with SQL files
		if _, err := os.Stat(dbFolderPath); err == nil {
			util.SaveLog(fmt.Sprintf("  Found database folder: %s", dbFolderPath))

			// Check if there are any SQL files (.sql or .sql.gz)
			files, _ := filepath.Glob(filepath.Join(dbFolderPath, "*.sql*"))
			util.SaveLog(fmt.Sprintf("  Found %d SQL files", len(files)))

			if len(files) > 0 {
				util.SaveLog(fmt.Sprintf("  Adding valid folder: %s", fullPath))
				validFolders = append(validFolders, fullPath)
			} else {
				util.SaveLog(fmt.Sprintf("  No SQL files found, skipping folder"))
			}
		} else {
			util.SaveLog(fmt.Sprintf("  Database folder not found: %v", err))
		}
	}

	if len(validFolders) == 0 {
		util.SaveLog(fmt.Sprintf("ERROR: No export folders found for database '%s'", dbName))
		return "", fmt.Errorf("no export folders found for database '%s'", dbName)
	}

	// Sort by name (timestamp) and get the latest (last one)
	sort.Strings(validFolders)
	latestFolder := validFolders[len(validFolders)-1]

	util.SaveLog(fmt.Sprintf("Found %d valid folders, latest is: %s", len(validFolders), latestFolder))
	for i, folder := range validFolders {
		util.SaveLog(fmt.Sprintf("  %d: %s", i+1, folder))
	}

	return latestFolder, nil
}

// readGzippedFile reads and decompresses a gzipped or plain text SQL file
func readGzippedFile(filename string) (string, error) {
	util.SaveLog(fmt.Sprintf("Reading SQL file: %s", filename))

	file, err := os.Open(filename)
	if err != nil {
		util.SaveLog(fmt.Sprintf("ERROR: Failed to open file: %v", err))
		return "", err
	}
	defer file.Close()

	header := make([]byte, 2)
	n, readErr := file.Read(header)
	_, _ = file.Seek(0, io.SeekStart)

	var reader io.Reader = file
	if readErr == nil && n >= 2 && header[0] == 0x1f && header[1] == 0x8b {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			util.SaveLog(fmt.Sprintf("ERROR: Failed to create gzip reader: %v", err))
			return "", err
		}
		defer gzReader.Close()
		reader = gzReader
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		util.SaveLog(fmt.Sprintf("ERROR: Failed to read file content: %v", err))
		return "", err
	}

	util.SaveLog(fmt.Sprintf("Successfully read %d bytes from %s", len(content), filepath.Base(filename)))
	return string(content), nil
}

// splitSQLStatements splits SQL content into individual statements
func splitSQLStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	stringChar := byte(0)
	inComment := false

	for i := 0; i < len(sql); i++ {
		c := sql[i]

		// Handle string literals
		if !inComment && (c == '\'' || c == '"') {
			if !inString {
				inString = true
				stringChar = c
			} else if c == stringChar {
				inString = false
				stringChar = 0
			}
		}

		// Handle line comments
		if !inString && !inComment && c == '-' && i+1 < len(sql) && sql[i+1] == '-' {
			inComment = true
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			inComment = false
			continue
		}

		// Handle block comments
		if !inString && !inComment && c == '/' && i+1 < len(sql) && sql[i+1] == '*' {
			inComment = true
			i += 2
			for i+1 < len(sql) && !(sql[i] == '*' && sql[i+1] == '/') {
				i++
			}
			i += 2
			inComment = false
			continue
		}

		// Split on semicolon
		if !inString && !inComment && c == ';' {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
			continue
		}

		current.WriteByte(c)
	}

	// Add last statement if exists
	stmt := strings.TrimSpace(current.String())
	if stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}

// processSQLFileStream processes SQL file in chunks without loading entire file into memory (supports both .sql and .sql.gz)
func processSQLFileStream(filePath string, progressChan chan string, db *sql.DB, fileType string) (int, int, error) {
	util.SaveLog(fmt.Sprintf("Processing %s with streaming...", fileType))

	file, err := os.Open(filePath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	var reader io.Reader = file

	// Inspect header for Gzip magic bytes (0x1f 0x8b)
	header := make([]byte, 2)
	n, readErr := file.Read(header)
	_, _ = file.Seek(0, io.SeekStart)

	if readErr == nil && n >= 2 && header[0] == 0x1f && header[1] == 0x8b {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to create gzip reader: %v", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	// Create buffered reader for efficient streaming
	bufReader := bufio.NewReaderSize(reader, 1024*1024) // 1MB buffer

	var stmt strings.Builder
	inString := false
	stringChar := byte(0)
	inComment := false
	successCount := 0
	errorCount := 0
	bufferSize := 0
	const maxBufferSize = 10 * 1024 * 1024 // 10MB max statement size

	// Read and process character by character
	for {
		c, err := bufReader.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return successCount, errorCount, fmt.Errorf("read error: %v", err)
		}

		// Handle string literals
		if !inComment && (c == '\'' || c == '"') {
			if !inString {
				inString = true
				stringChar = c
			} else if c == stringChar {
				inString = false
				stringChar = 0
			}
		}

		// Handle line comments
		if !inString && !inComment && c == '-' {
			nextByte, err := bufReader.Peek(1)
			if err == nil && nextByte[0] == '-' {
				bufReader.ReadByte() // consume the second '-'
				inComment = true
				continue
			}
		}

		// Handle block comments
		if !inString && !inComment && c == '/' {
			nextByte, err := bufReader.Peek(1)
			if err == nil && nextByte[0] == '*' {
				bufReader.ReadByte() // consume the '*'
				inComment = true
				continue
			}
		}

		// End line comment
		if inComment && c == '\n' {
			inComment = false
			continue
		}

		// End block comment
		if inComment && c == '*' {
			nextByte, err := bufReader.Peek(1)
			if err == nil && nextByte[0] == '/' {
				bufReader.ReadByte() // consume the '/'
				inComment = false
				continue
			}
		}

		// Skip comment characters
		if inComment {
			continue
		}

		// Split on semicolon
		if !inString && c == ';' {
			stmtStr := strings.TrimSpace(stmt.String())
			if stmtStr != "" && !strings.HasPrefix(stmtStr, "--") {
				// Execute statement
				_, err := db.Exec(stmtStr)
				if err != nil {
					if !strings.Contains(err.Error(), "already exists") &&
						!strings.Contains(err.Error(), "already defined") &&
						!strings.Contains(err.Error(), "Duplicate entry") {
						errorCount++
						if errorCount <= 5 {
							util.SaveLog(fmt.Sprintf("Error in %s: %v", fileType, err))
						}
					} else {
						successCount++
					}
				} else {
					successCount++
				}

				// Progress reporting every 1000 statements
				if successCount%1000 == 0 {
					progressChan <- fmt.Sprintf("[blue]Processed %d statements for %s...", successCount, fileType)
				}
			}
			stmt.Reset()
			bufferSize = 0
			continue
		}

		// Prevent memory issues with extremely large statements
		if bufferSize > maxBufferSize {
			util.SaveLog(fmt.Sprintf("Warning: Statement exceeded max buffer size for %s, skipping", fileType))
			stmt.Reset()
			bufferSize = 0
			continue
		}

		stmt.WriteByte(c)
		bufferSize++
	}

	// Handle last statement if exists
	if stmt.Len() > 0 {
		stmtStr := strings.TrimSpace(stmt.String())
		if stmtStr != "" && !strings.HasPrefix(stmtStr, "--") {
			_, err := db.Exec(stmtStr)
			if err != nil {
				if !strings.Contains(err.Error(), "already exists") &&
					!strings.Contains(err.Error(), "already defined") &&
					!strings.Contains(err.Error(), "Duplicate entry") {
					errorCount++
				} else {
					successCount++
				}
			} else {
				successCount++
			}
		}
	}

	return successCount, errorCount, nil
}

func setImportSessionVariables(db *sql.DB) error {
	sessionQueries := []string{
		"SET FOREIGN_KEY_CHECKS=0",
		"SET UNIQUE_CHECKS=0",
		"SET AUTOCOMMIT=0",
		"SET SQL_MODE='NO_AUTO_VALUE_ON_ZERO'",
		"SET SESSION collation_connection = 'utf8mb4_unicode_ci'",
		"SET SESSION wait_timeout = 28800",
		"SET SESSION interactive_timeout = 28800",
		"SET SESSION net_read_timeout = 28800",
		"SET SESSION net_write_timeout = 28800",
	}

	for _, query := range sessionQueries {
		if _, err := db.Exec(query); err != nil {
			if !strings.Contains(query, "timeout") && !strings.Contains(query, "collation") {
				return err
			}
		}
	}
	return nil
}

func restoreImportSessionVariables(db *sql.DB) {
	restoreQueries := []string{
		"COMMIT",
		"SET FOREIGN_KEY_CHECKS=1",
		"SET UNIQUE_CHECKS=1",
		"SET AUTOCOMMIT=1",
	}
	for _, query := range restoreQueries {
		_, _ = db.Exec(query)
	}
}

// ShowImportWizardModal displays the step-by-step import wizard with read-only safeguards, location picker, and danger confirmation.
func ShowImportWizardModal(app *tview.Application, db *sql.DB, dbName string) {
	if ActiveReadOnly {
		showErrorModal(app, mainFlex, "🔒 Action Blocked: Active Connection is in READ-ONLY Mode.\nImporting database dumps is not allowed while Read-Only mode is enabled.")
		return
	}

	if db == nil {
		showErrorModal(app, mainFlex, "No active database connection available for import.")
		return
	}

	// Determine initial default candidate path
	defaultPath := "./export"
	if entries, err := os.ReadDir("./export"); err == nil && len(entries) > 0 {
		var latest string
		var latestTime time.Time
		for _, entry := range entries {
			info, err := entry.Info()
			if err == nil && info.ModTime().After(latestTime) {
				latestTime = info.ModTime()
				latest = filepath.Join("./export", entry.Name())
			}
		}
		if latest != "" {
			defaultPath = latest
		}
	}

	// Step 1: Location & File Picker Modal
	form := tview.NewForm()
	form.SetBorder(true).
		SetTitle(fmt.Sprintf(" 📥 Database Import Wizard (%s) ", dbName)).
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorDarkCyan).
		SetTitleColor(tcell.ColorAqua).
		SetBorderPadding(1, 1, 3, 3)

	form.AddInputField("Import File or Folder Path", defaultPath, 60, nil, nil)

	form.AddButton("[lime::b] ▶ NEXT: CONFIRMATION ", func() {
		targetPath := strings.TrimSpace(form.GetFormItem(0).(*tview.InputField).GetText())
		if targetPath == "" {
			showErrorModal(app, mainFlex, "Please specify a valid file or folder path to import.")
			return
		}

		stat, err := os.Stat(targetPath)
		if err != nil {
			showErrorModal(app, mainFlex, fmt.Sprintf("Path error: %v", err))
			return
		}

		// Step 2: High-Visibility Permission Confirmation Warning Modal
		showImportConfirmationModal(app, db, dbName, targetPath, stat.IsDir())
	})

	form.AddButton("[red::b] ✖ CANCEL ", func() {
		layout := CreateLayoutWithFooter(app, mainFlex)
		app.SetRoot(layout, true)
	})

	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(form, 0, 1, true)

	app.SetRoot(layout, true)
}

func showImportConfirmationModal(app *tview.Application, db *sql.DB, dbName, targetPath string, isDir bool) {
	pathType := "Single Dump File"
	if isDir {
		pathType = "Directory Folder"
	}

	text := fmt.Sprintf(
		"[red::b]⚠️ DANGEROUS / DESTRUCTIVE OPERATION![-::-]\n\n"+
			"Target Database: [lime::b]%s[-::-]\n"+
			"Import Source: [cyan::b]%s[-::-] (%s)\n\n"+
			"[yellow::b]CRITICAL WARNING: Importing this source will execute DDL and DML statements which may DROP existing tables, OVERWRITE schema definitions, or REPLACE data inside database '%s'.[-::-]\n\n"+
			"[white::b]Are you sure you want to proceed with this import?[-::-]",
		dbName, targetPath, pathType, dbName,
	)

	modal := tview.NewModal().
		SetText(text).
		AddButtons([]string{"[red::b] ⚠️ YES, OVERWRITE & IMPORT ", "[yellow::b] ⬅ BACK ", "[white::b] CANCEL"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if strings.Contains(buttonLabel, "YES") {
				executeInteractiveImport(app, db, dbName, targetPath, isDir)
			} else if strings.Contains(buttonLabel, "BACK") {
				ShowImportWizardModal(app, db, dbName)
			} else {
				layout := CreateLayoutWithFooter(app, mainFlex)
				app.SetRoot(layout, true)
			}
		})

	app.SetRoot(modal, true)
}

func executeInteractiveImport(app *tview.Application, db *sql.DB, dbName, targetPath string, isDir bool) {
	logView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() {
			app.Draw()
		})
	logView.SetBorder(true).
		SetTitle(fmt.Sprintf(" 📥 Importing Data into '%s' ", dbName)).
		SetTitleColor(tcell.ColorLime)

	layout := CreateLayoutWithFooter(app, logView)
	app.SetRoot(layout, true)

	progressChan := make(chan string, 100)

	// Stream logs to logView
	go func() {
		for msg := range progressChan {
			app.QueueUpdateDraw(func() {
				fmt.Fprintln(logView, msg)
				logView.ScrollToEnd()
			})
		}
	}()

	// Execute import in background goroutine
	go func() {
		defer close(progressChan)

		progressChan <- fmt.Sprintf("[cyan::b]🚀 Starting import into database '%s'...", dbName)
		progressChan <- fmt.Sprintf("[white]Source path: %s", targetPath)
		startTime := time.Now()

		// Optimize session for fast streaming import
		if err := setImportSessionVariables(db); err != nil {
			progressChan <- fmt.Sprintf("[red::b]❌ Failed to set session variables: %v", err)
			return
		}

		totalSuccess := 0
		totalErrors := 0

		if isDir {
			// Find all SQL files (.sql / .sql.gz) in directory or subfolders
			var files []string
			err := filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() && (strings.HasSuffix(strings.ToLower(path), ".sql") || strings.HasSuffix(strings.ToLower(path), ".sql.gz")) {
					files = append(files, path)
				}
				return nil
			})

			if err != nil || len(files) == 0 {
				progressChan <- fmt.Sprintf("[red::b]❌ No valid SQL files found in directory: %s", targetPath)
				return
			}

			sort.Strings(files)
			progressChan <- fmt.Sprintf("[blue]Found %d SQL file(s) to process...", len(files))

			for i, file := range files {
				progressChan <- fmt.Sprintf("\n[yellow]Processing [%d/%d]: %s...", i+1, len(files), filepath.Base(file))
				succ, errs, err := processSQLFileStream(file, progressChan, db, filepath.Base(file))
				totalSuccess += succ
				totalErrors += errs
				if err != nil {
					progressChan <- fmt.Sprintf("[red]Error processing %s: %v", filepath.Base(file), err)
				}
			}
		} else {
			// Single file import
			progressChan <- fmt.Sprintf("[yellow]Processing file stream: %s...", filepath.Base(targetPath))
			succ, errs, err := processSQLFileStream(targetPath, progressChan, db, filepath.Base(targetPath))
			totalSuccess += succ
			totalErrors += errs
			if err != nil {
				progressChan <- fmt.Sprintf("[red::b]❌ Import Error: %v", err)
			}
		}

		// Restore checks and commit
		restoreImportSessionVariables(db)

		elapsed := time.Since(startTime)
		progressChan <- fmt.Sprintf("\n[lime::b]🎉 IMPORT COMPLETE! Duration: %v", elapsed.Round(time.Millisecond))
		progressChan <- fmt.Sprintf("[green]Successfully executed %d statements (Errors: %d)", totalSuccess, totalErrors)
		progressChan <- "[yellow::b]Press ESC or ENTER to return to workspace."

		app.QueueUpdateDraw(func() {
			logView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyEnter {
					layout := CreateLayoutWithFooter(app, mainFlex)
					app.SetRoot(layout, true)
					return nil
				}
				return event
			})
		})
	}()
}

// processTableDataStream specifically optimized for large table data
// func processTableDataStream(gzFile string, progressChan chan string, db *sql.DB, tableName string) (int, int, error) {
// 	file, err := os.Open(gzFile)
// 	if err != nil {
// 		return 0, 0, err
// 	}
// 	defer file.Close()

// 	gzReader, err := gzip.NewReader(file)
// 	if err != nil {
// 		return 0, 0, err
// 	}
// 	defer gzReader.Close()

// 	scanner := bufio.NewScanner(gzReader)
// 	// Set large buffer for scanning large lines
// 	buf := make([]byte, 0, 64*1024)
// 	scanner.Buffer(buf, 10*1024*1024) // 10MB buffer

// 	var currentStmt strings.Builder
// 	successCount := 0
// 	errorCount := 0
// 	batchCount := 0

// 	for scanner.Scan() {
// 		line := scanner.Text()
// 		currentStmt.WriteString(line)

// 		// Check if line ends with semicolon (complete statement)
// 		if strings.HasSuffix(strings.TrimSpace(line), ";") {
// 			stmt := strings.TrimSpace(currentStmt.String())
// 			if stmt != "" && !strings.HasPrefix(stmt, "--") {
// 				_, err := db.Exec(stmt)
// 				if err != nil {
// 					if !strings.Contains(err.Error(), "already exists") {
// 						errorCount++
// 					} else {
// 						successCount++
// 					}
// 				} else {
// 					successCount++
// 				}

// 				batchCount++
// 				// Commit every 1000 inserts to free memory
// 				if batchCount >= 1000 {
// 					db.Exec("COMMIT")
// 					batchCount = 0
// 					progressChan <- fmt.Sprintf("[blue]Processed %d rows for table %s", successCount, tableName)
// 				}
// 			}
// 			currentStmt.Reset()
// 		} else {
// 			currentStmt.WriteString("\n")
// 		}
// 	}

// 	// Final commit
// 	if batchCount > 0 {
// 		db.Exec("COMMIT")
// 	}

// 	return successCount, errorCount, scanner.Err()
// }
