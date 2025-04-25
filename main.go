// main.go
package main

import (
	"flag"
	"mysql-tui/ui"

	"github.com/rivo/tview"
)

func main() {

	user := flag.String("u", "", "Username")
	pass := flag.String("p", "", "Password")
	host := flag.String("host", "localhost", "Hostname")
	port := flag.String("port", "3306", "Port number")

	// Parse command line flags
	flag.Parse()

	var password string
	if *pass != "" {
		password = *pass
	} else {
		password = ""
	}

	app := tview.NewApplication()
	ui.ShowConnectionForm(app, *user, password, *host, *port)
	if err := app.Run(); err != nil {
		panic(err)
	}
}
