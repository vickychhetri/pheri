// main.go
package main

import (
	"mysql-tui/ui"

	"github.com/rivo/tview"
)

func main() {
	app := tview.NewApplication()
	ui.ShowConnectionForm(app)
	if err := app.Run(); err != nil {
		panic(err)
	}
}
