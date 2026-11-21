# PHERI-05: Connection Timeouts & Network Safeguards

- **Task Code**: `PHERI-05`
- **Feature Name**: Connection Timeouts & Network Safeguards
- **Category**: Connection & Network Reliability
- **Priority**: `P0 - Critical`
- **Status**: `Completed`
- **Target Files**: `dbs/mysql.go`, `dbs/mysql_test.go`, `ui/connect.go`, `ui/connect_test.go`
- **GitHub Issue**: [#20](https://github.com/vickychhetri/pheri/issues/20)

---

## 1. Reason of Adding

In previous versions of Pheri, the MySQL DSN was built via simple string interpolation (`fmt.Sprintf("%s:%s@tcp(%s:%s)/", ...)`). This omitted dial timeouts, read/write timeouts, collation parameters, and connection pool bounds.

### Critical Real-World Failures Prevented:
1. **Indefinite Hangs on Offline or Firewalled Servers**:
   Without an explicit `timeout=...` parameter in the connection string, Go's TCP dialer uses standard operating system SYN retransmissions, blocking between 20 seconds and 2 minutes on Windows and Linux. The TUI application froze completely without giving any clue to the user.
2. **Silent Packet Drops & Mid-Stream Query Freezes**:
   Without `readTimeout` and `writeTimeout`, network stalls or intermediate stateful firewall drops hung connections indefinitely.
3. **Unbounded Connection Creation & Server Saturation**:
   `*sql.DB` without connection pooling boundaries allowed opening arbitrary numbers of concurrent connections under rapid operations, easily exhausting MySQL server `max_connections` or keeping dead connections alive past NAT idle windows.
4. **Encoding and Temporal Scan Bugs**:
   Without `collation=utf8mb4_general_ci`, `parseTime=true`, and `loc=Local`, multibyte UTF-8 characters could suffer character corruption, and scanning temporal columns (`DATETIME`, `TIMESTAMP`) directly into Go `time.Time` was disabled.
5. **Cryptic Error Messages**:
   When connections failed, raw OS driver messages (e.g., `connectex: A connection attempt failed because the connected party did not properly respond`) were shown without diagnostic advice.

---

## 2. Technical Architecture & Implementation

### 1. Structured Configuration & Safe DSN Construction (`dbs/mysql.go`)
- **`ConnectConfig` Struct**:
  Encapsulates connection credentials, endpoint addresses, and operational safeguards:
  - `DialTimeout`: Default `5 * time.Second` (ensures fast failure on unreachable hosts).
  - `ReadTimeout`: Default `60 * time.Second`.
  - `WriteTimeout`: Default `60 * time.Second`.
  - `MaxOpenConns`: Default `25`.
  - `MaxIdleConns`: Default `5`.
  - `ConnMaxLifetime`: Default `10 * time.Minute`.
  - `ConnMaxIdleTime`: Default `3 * time.Minute`.
  - `Collation`: Default `utf8mb4_general_ci`.
- **`BuildDSN(cfg ConnectConfig) string`**:
  Utilizes the official `github.com/go-sql-driver/mysql` `mysql.Config` type. This automatically formats and escapes credentials, host addresses, timeouts, and multi-statement parameters reliably.
- **`OpenWithConfig(cfg ConnectConfig) (*sql.DB, error)`**:
  Opens the database handle and configures pool limits (`SetMaxOpenConns`, `SetMaxIdleConns`, `SetConnMaxLifetime`, `SetConnMaxIdleTime`) without executing a network Ping.
- **`ConnectWithConfig(cfg ConnectConfig) (*sql.DB, error)`**:
  Opens the pool and verifies live connectivity via `db.Ping()`.
- **`Connect(user, pass, host, port string) (*sql.DB, error)`**:
  Preserves 100% backward compatibility by invoking `ConnectWithConfig` with default options.

### 2. Error Categorization & Actionable Guidance (`dbs/mysql.go`)
- **`CategorizeConnectionError(err error) ConnectionErrorInfo`**:
  Inspects typed errors (`*mysql.MySQLError`, `*net.DNSError`, `net.Error`) and error strings to classify failures into actionable categories:
  - `TIMEOUT`: Unreachable host, packet loss, or firewall block. Provides suggestions to check host and network firewall.
  - `REFUSED`: MySQL server is not running or listening on that port.
  - `AUTH`: Access denied (MySQL errors 1045, 1044). Prompts user to check credentials and host grants.
  - `DNS`: Hostname resolution failure.
  - `DATABASE`: Specified database does not exist (MySQL error 1049).
  - `NETWORK` / `UNKNOWN`: Clear fallback diagnostics with raw error detail.

### 3. User Interface Enhancements (`ui/connect.go`)
- **Diagnostic Connection Modal**:
  Renders formatted modal with structured problem title, plain English explanation, troubleshooting suggestion, and muted technical details.
- **Layout Restoration Fix**:
  When the user chooses "Try Again", the full root `flex` container (ASCII logo banner, connection form, and status footer) is restored rather than replacing the root with only the inner `form`.

---

## 3. Test Coverage

- `dbs/mysql_test.go`:
  - `TestDefaultConnectConfig`: Validates default parameters and pooling values.
  - `TestBuildDSN_Defaults`: Validates generated DSN keys and values.
  - `TestBuildDSN_CustomAndEscaping`: Tests special characters in passwords (`@`, `:`, `/`, `#`, `!`) and custom timeouts.
  - `TestBuildDSN_EmptyHostPortFallback`: Validates fallback to `127.0.0.1:3306`.
  - `TestOpenWithConfig_PoolSettings`: Validates connection pool properties on `*sql.DB`.
  - `TestConnectionTimeoutEnforcement`: Validates connecting to non-routable address `192.0.2.1:3306` with 200ms timeout fails in under 2.5 seconds.
  - `TestCategorizeConnectionError`: Validates 14 distinct error scenarios (auth, db missing, timeout, connection refused, DNS, generic).
- `ui/connect_test.go`:
  - `TestShowConnectionForm_Render`: Verifies form initialization without CLI args.
  - `TestConnectionErrorPresentationFormatting`: Verifies modal formatting strings and diagnostic hints.
