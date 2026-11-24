// ui/fk_nav.go
package ui

import (
	"context"
	"database/sql"
	"mysql-tui/util"
	"sync"
	"time"
)

// NavBreadcrumb snapshots a Data Grid view before following a foreign key relationship
// so pressing Backspace cleanly restores the prior table, query, sort order, and cursor cell.
type NavBreadcrumb struct {
	DBName        string
	TableName     string
	ObjectType    string
	Query         string
	Page          int
	CursorRow     int
	CursorCol     int
	EditingActive bool
	SortColumn    string
	SortDir       SortDirection
}

var (
	fkCache      = make(map[string]map[string]util.ForeignKeyRef)
	fkCacheMutex sync.RWMutex

	fkNavHistory []NavBreadcrumb
	fkNavMutex   sync.Mutex

	activeGridForeignKeys map[string]util.ForeignKeyRef
	pendingNavRestoreRow  = -1
	pendingNavRestoreCol  = -1
)

// GetCachedForeignKeys returns cached foreign key metadata for dbName.tableName or queries
// INFORMATION_SCHEMA safely with a 2-second timeout.
func GetCachedForeignKeys(ctx context.Context, db *sql.DB, dbName, tableName string) map[string]util.ForeignKeyRef {
	if db == nil || dbName == "" || tableName == "" {
		return nil
	}

	cacheKey := dbName + "." + tableName
	fkCacheMutex.RLock()
	if cached, ok := fkCache[cacheKey]; ok {
		fkCacheMutex.RUnlock()
		return cached
	}
	fkCacheMutex.RUnlock()

	if ctx == nil {
		ctx = context.Background()
	}
	qCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	fks, err := util.GetTableForeignKeys(qCtx, db, dbName, tableName)
	if err != nil {
		util.SaveLog("FK lookup skipped/failed for " + cacheKey + ": " + err.Error())
		return nil
	}

	fkCacheMutex.Lock()
	fkCache[cacheKey] = fks
	fkCacheMutex.Unlock()
	return fks
}

// ClearForeignKeyCache clears the cached foreign key metadata (primarily used in testing or schema reloads).
func ClearForeignKeyCache() {
	fkCacheMutex.Lock()
	defer fkCacheMutex.Unlock()
	fkCache = make(map[string]map[string]util.ForeignKeyRef)
}

// PushNavBreadcrumb pushes a grid view snapshot onto the FK traversal breadcrumb stack.
func PushNavBreadcrumb(crumb NavBreadcrumb) {
	fkNavMutex.Lock()
	defer fkNavMutex.Unlock()
	fkNavHistory = append(fkNavHistory, crumb)
}

// PopNavBreadcrumb pops and returns the most recent breadcrumb, or false if the stack is empty.
func PopNavBreadcrumb() (NavBreadcrumb, bool) {
	fkNavMutex.Lock()
	defer fkNavMutex.Unlock()
	if len(fkNavHistory) == 0 {
		return NavBreadcrumb{}, false
	}
	top := fkNavHistory[len(fkNavHistory)-1]
	fkNavHistory = fkNavHistory[:len(fkNavHistory)-1]
	return top, true
}

// NavHistoryDepth returns the number of saved FK navigation hops on the stack.
func NavHistoryDepth() int {
	fkNavMutex.Lock()
	defer fkNavMutex.Unlock()
	return len(fkNavHistory)
}

// ClearNavHistory clears all FK navigation history (used when opening a fresh table or running a new query).
func ClearNavHistory() {
	fkNavMutex.Lock()
	defer fkNavMutex.Unlock()
	fkNavHistory = nil
	pendingNavRestoreRow = -1
	pendingNavRestoreCol = -1
}
