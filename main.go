// main.go
package main

import (
	"flag"
	"mysql-tui/phhistory"
	"mysql-tui/ui"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/rivo/tview"
)

var historyDBPathOverride string

// resolveHistoryDBPath determines the appropriate location for the phhistory SQLite database.
// If installed into a system directory (e.g. C:\Program Files\Pheri or /usr/local/bin) where standard
// users lack write permissions, it stores the database in the user's home directory (~/.pheri/phhistory.db).
// If a writable phhistory.db already exists in the executable's directory, it continues using it.
func resolveHistoryDBPath() string {
	if historyDBPathOverride != "" {
		return historyDBPathOverride
	}

	// 1. Portable mode: if phhistory.db already exists in the executable directory and is writable
	if execPath, err := os.Executable(); err == nil && execPath != "" {
		exeDir := filepath.Dir(execPath)
		candidate := filepath.Join(exeDir, "phhistory.db")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			if f, err := os.OpenFile(candidate, os.O_RDWR, 0666); err == nil {
				f.Close()
				return candidate
			}
		}
	}

	// 2. User home directory: ~/.pheri/phhistory.db
	if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
		pheriDir := filepath.Join(homeDir, ".pheri")
		if err := os.MkdirAll(pheriDir, 0755); err == nil {
			userDBPath := filepath.Join(pheriDir, "phhistory.db")
			// Migrate legacy db if present next to executable
			if _, err := os.Stat(userDBPath); os.IsNotExist(err) {
				if execPath, err := os.Executable(); err == nil && execPath != "" {
					legacyCandidate := filepath.Join(filepath.Dir(execPath), "phhistory.db")
					if data, err := os.ReadFile(legacyCandidate); err == nil && len(data) > 0 {
						_ = os.WriteFile(userDBPath, data, 0600)
					}
				}
			}
			return userDBPath
		}

		// 3. Fallback: ~/.pheri_history.db directly in homeDir
		homeCandidate := filepath.Join(homeDir, ".pheri_history.db")
		if testFile, err := os.OpenFile(homeCandidate, os.O_CREATE|os.O_RDWR, 0600); err == nil {
			testFile.Close()
			return homeCandidate
		}
	}

	// 4. Fallback to os.UserConfigDir()
	if configDir, err := os.UserConfigDir(); err == nil && configDir != "" {
		pheriDir := filepath.Join(configDir, "pheri")
		if err := os.MkdirAll(pheriDir, 0755); err == nil {
			return filepath.Join(pheriDir, "phhistory.db")
		}
	}

	// 5. Ultimate fallback: temporary directory
	tempDir := os.TempDir()
	pheriDir := filepath.Join(tempDir, "pheri")
	_ = os.MkdirAll(pheriDir, 0755)
	return filepath.Join(pheriDir, "phhistory.db")
}

func main() {

	user := flag.String("u", "", "Username")
	pass := flag.String("p", "", "Password")
	host := flag.String("host", "localhost", "Hostname")
	port := flag.String("port", "3306", "Port number")

	history := flag.Bool("history", false, "Show history")
	days := flag.Int("days", 30, "Number of days to keep history")
	months := flag.Int("months", 12, "Number of months to keep history")
	years := flag.Int("years", 1, "Number of years to keep history")
	historyFile := flag.String("history_file", "", "File to import")

	// Add more flags as needed
	// Parse command line flags
	flag.Parse()

	// Check if password is provided
	var password string
	if *pass != "" {
		password = *pass
	} else {
		password = ""
	}

	dbPath := resolveHistoryDBPath()
	err := phhistory.InitPhHistory(dbPath, *user, *host, *port)
	if err != nil {
		panic(err)
	}

	if *history {
		// If the -history flag is provided, show the history
		// and exit the program
		err := phhistory.FetchHistory(*days, *months, *years, *historyFile)
		if err != nil {
			panic(err)
		}
		os.Exit(0)
	}

	/*
	 THIS IS TO DEVELOP NEW EDITOR WITH CONTROLS : STILL UNDER DEVELOPMNET SO HIDDEN
	*/
	// app1 := tview.NewApplication()
	// editor := ui.NewSQLEditor(app1)
	// if err := app1.SetRoot(editor.Container, true).Run(); err != nil {
	// 	log.Fatal(err)
	// }

	defer phhistory.Close()
	app := tview.NewApplication()
	app.EnableMouse(true)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range sigChan {
			if ui.IsQueryRunning() {
				ui.CancelActiveQuery()
			}
		}
	}()

	ui.ShowConnectionForm(app, *user, password, *host, *port)
	if err := app.Run(); err != nil {
		panic(err)
	}
}
