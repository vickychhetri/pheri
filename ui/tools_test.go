// ui/tools_test.go
package ui

import (
	"testing"

	"mysql-tui/dbs"

	"github.com/rivo/tview"
)

func TestShowHealthDashboardModal_NilDB(t *testing.T) {
	app := tview.NewApplication()
	// Must not panic on nil db
	showHealthDashboardModal(app, nil)
}

func TestShowHealthDashboardModal_ConfiguredDB(t *testing.T) {
	cfg := dbs.DefaultConnectConfig("root", "test", "127.0.0.1", "3306")
	db, err := dbs.OpenWithConfig(cfg)
	if err != nil {
		t.Fatalf("failed to open db with config: %v", err)
	}
	defer db.Close()

	app := tview.NewApplication()
	// Must initialize table with client pool metrics without panicking
	showHealthDashboardModal(app, db)
}
