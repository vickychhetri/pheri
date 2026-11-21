# PHERI-06: Connection Pool Sizing & Lifecycle

- **Task Code**: `PHERI-06`
- **Feature Name**: Connection Pool Sizing & Lifecycle
- **Category**: Connection & Resource Management
- **Priority**: `P0 - Critical`
- **Status**: `Completed`
- **Target Files**: `dbs/mysql.go`, `dbs/mysql_test.go`, `ui/tools.go`, `ui/tools_test.go`, `ui/browser.go`, `ui/import.go`
- **GitHub Issue**: [#21](https://github.com/vickychhetri/pheri/issues/21)

---

## 1. Reason of Adding

By default, Go's standard library `database/sql` driver leaves connection pool limits unbounded:
- `MaxOpenConns = 0` (unlimited concurrent connections)
- `ConnMaxLifetime = 0` (connections are reused indefinitely and never closed by age)

In database operations and TUI applications, unbounded connection pools cause two severe operational hazards:
1. **Server-Side Connection Limit Saturation**:
   Rapid concurrent tasks (such as parallel multi-table schema inspection, background query executions, live processlist polling, and parallel exports) can spawn dozens of simultaneous connections, easily exhausting MySQL servers configured with low or moderate `max_connections` (e.g., 50 to 100 on developer, test, or shared cloud instances). This triggers `Error 1040: Too many connections` and locks out other users and services.
2. **Silent Connection Drops by Cloud Stateful NAT Firewalls**:
   Managed cloud database environments (AWS Aurora/RDS, Azure Database for MySQL, GCP Cloud SQL, DigitalOcean) and network infrastructure (AWS NAT Gateways, Docker network bridges, stateful firewalls) terminate idle TCP sessions after 300 to 350 seconds (approx. 5 minutes). When the client pool leaves connections alive beyond 5 minutes, subsequent queries attempt to write to dead sockets, producing `driver: bad connection` or hanging until TCP keep-alive fails.

---

## 2. Technical Architecture & Implementation

### 1. Connection Pool Sizing and Recycling Limits (`dbs/mysql.go`)
- **`DefaultMaxOpenConns = 25`**:
  Caps total simultaneous database connections at 25, protecting the target database server from connection spikes while providing ample concurrency for Pheri's parallel export and asynchronous query workers.
- **`DefaultMaxIdleConns = 5`**:
  Retains up to 5 warm, pre-authenticated connections in the pool, eliminating TCP and TLS handshake round-trips for subsequent queries.
- **`DefaultConnMaxLifetime = 5 * time.Minute`**:
  Strictly recycles connections older than 5 minutes. This ensures connections are retired gracefully on the client before cloud routers or NAT firewalls terminate idle TCP sessions at 300 seconds.
- **`DefaultConnMaxIdleTime = 3 * time.Minute`**:
  Reclaims idle connections that remain unused for 3 minutes, freeing resources during periods of user inactivity.

### 2. Centralized Pool Management & Telemetry API (`dbs/mysql.go`)
- **`ConfigurePoolLimits(db *sql.DB, maxOpen, maxIdle int, maxLifetime, maxIdleTime time.Duration)`**:
  Centralized utility that applies standard or custom pool constraints to any `*sql.DB` instance across the codebase.
- **`PoolStatsSummary` Struct & `GetPoolStats(db *sql.DB) PoolStatsSummary`**:
  Exposes structured real-time driver connection pool telemetry:
  - `MaxOpenConnections`: Maximum open connection ceiling.
  - `OpenConnections`: Current count of established connections.
  - `InUse`: Count of connections currently executing statements.
  - `Idle`: Count of idle connections ready in pool.
  - `WaitCount`: Cumulative count of queries that had to wait for a connection.
  - `WaitDuration`: Total time spent waiting for a connection.
  - `MaxLifetimeClosed`: Connections closed due to exceeding `ConnMaxLifetime`.
  - `MaxIdleTimeClosed`: Connections closed due to exceeding `ConnMaxIdleTime`.

### 3. Real-Time Pool Telemetry in Health Dashboard (`ui/tools.go`)
- Integrated client connection pool metrics into the **Real-Time Health & Performance Dashboard** (`F6` / `:health`):
  - Displays Max Open Limit (`25 connections [lime::b]SAFEGUARD ACTIVE[-]`).
  - Displays Open / In-Use counters (`x open (y in-use, z idle)`).
  - Displays Auto-Recycle and Idle Eviction policies.
  - Includes connection pool statistics in the clipboard summary (`c` shortcut).

### 4. Codebase Standardization & Legacy Cleanup (`ui/browser.go`, `ui/import.go`)
- Standardized `exportAllObjects` in `ui/browser.go` to use `dbs.ConfigurePoolLimits(db, 10, 5, 5*time.Minute, 2*time.Minute)`.
- Standardized `importAllObjects` in `ui/import.go` to use `dbs.ConfigurePoolLimits(db, 25, 10, 30*time.Minute, 5*time.Minute)`.
- Removed dead legacy `createDbPool` function from `ui/browser.go`.

---

## 3. Test Coverage

- `dbs/mysql_test.go`:
  - `TestDefaultConnectConfig`: Verifies `ConnMaxLifetime == 5*time.Minute`, `ConnMaxIdleTime == 3*time.Minute`, `MaxOpenConns == 25`, `MaxIdleConns == 5`.
  - `TestConfigurePoolLimits`: Verifies applying custom pool limits, fallback to defaults on non-positive values, and safe handling of nil `*sql.DB`.
  - `TestGetPoolStats`: Verifies capturing driver stats and safe handling of nil `*sql.DB`.
- `ui/tools_test.go`:
  - `TestShowHealthDashboardModal_NilDB`: Verifies health modal handles nil db safely without panic.
  - `TestShowHealthDashboardModal_ConfiguredDB`: Verifies health modal initializes and populates client connection pool metrics.
