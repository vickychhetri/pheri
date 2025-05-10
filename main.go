// main.go
package main

import (
	"flag"
	"mysql-tui/phhistory"
	"mysql-tui/ui"
	"os"
	"path/filepath"

	"github.com/rivo/tview"
)

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

	execPath, err := os.Executable()
	if err != nil {
		panic(err)
	}
	// Check if the executable path is valid
	if execPath == "" {
		panic("Invalid executable path")
	}
	exeDir := filepath.Dir(execPath)
	dbPath := filepath.Join(exeDir, "phhistory.db")

	err = phhistory.InitPhHistory(dbPath, *user, *host, *port)
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
	ui.ShowConnectionForm(app, *user, password, *host, *port)
	if err := app.Run(); err != nil {
		panic(err)
	}
}
