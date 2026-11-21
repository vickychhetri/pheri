// dbs/mysql.go
package dbs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Default connection settings for network safeguards and connection pooling.
const (
	DefaultDialTimeout     = 5 * time.Second
	DefaultReadTimeout     = 60 * time.Second
	DefaultWriteTimeout    = 60 * time.Second
	DefaultMaxOpenConns    = 25
	DefaultMaxIdleConns    = 5
	DefaultConnMaxLifetime = 5 * time.Minute
	DefaultConnMaxIdleTime = 3 * time.Minute
	DefaultCollation       = "utf8mb4_general_ci"
	DefaultPort            = "3306"
	DefaultHost            = "127.0.0.1"
)

// ConnectConfig defines connection parameters, timeouts, and pooling limits.
type ConnectConfig struct {
	User            string
	Pass            string
	Host            string
	Port            string
	Database        string
	DialTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	Collation       string
}

// DefaultConnectConfig returns a ConnectConfig with safe default timeouts and pool settings.
func DefaultConnectConfig(user, pass, host, port string) ConnectConfig {
	return ConnectConfig{
		User:            user,
		Pass:            pass,
		Host:            host,
		Port:            port,
		DialTimeout:     DefaultDialTimeout,
		ReadTimeout:     DefaultReadTimeout,
		WriteTimeout:    DefaultWriteTimeout,
		MaxOpenConns:    DefaultMaxOpenConns,
		MaxIdleConns:    DefaultMaxIdleConns,
		ConnMaxLifetime: DefaultConnMaxLifetime,
		ConnMaxIdleTime: DefaultConnMaxIdleTime,
		Collation:       DefaultCollation,
	}
}

// BuildDSN constructs a formatted MySQL DSN with timeouts and encoding parameters.
func BuildDSN(cfg ConnectConfig) string {
	mysqlCfg := mysql.NewConfig()
	mysqlCfg.User = cfg.User
	mysqlCfg.Passwd = cfg.Pass
	mysqlCfg.Net = "tcp"

	host := cfg.Host
	if host == "" {
		host = DefaultHost
	}
	port := cfg.Port
	if port == "" {
		port = DefaultPort
	}
	mysqlCfg.Addr = net.JoinHostPort(host, port)
	mysqlCfg.DBName = cfg.Database

	// Timeouts
	if cfg.DialTimeout > 0 {
		mysqlCfg.Timeout = cfg.DialTimeout
	} else {
		mysqlCfg.Timeout = DefaultDialTimeout
	}

	if cfg.ReadTimeout > 0 {
		mysqlCfg.ReadTimeout = cfg.ReadTimeout
	} else {
		mysqlCfg.ReadTimeout = DefaultReadTimeout
	}

	if cfg.WriteTimeout > 0 {
		mysqlCfg.WriteTimeout = cfg.WriteTimeout
	} else {
		mysqlCfg.WriteTimeout = DefaultWriteTimeout
	}

	// Encoding & Time
	if cfg.Collation != "" {
		mysqlCfg.Collation = cfg.Collation
	} else {
		mysqlCfg.Collation = DefaultCollation
	}

	mysqlCfg.ParseTime = true
	mysqlCfg.Loc = time.Local
	mysqlCfg.MultiStatements = true
	mysqlCfg.AllowNativePasswords = true

	return mysqlCfg.FormatDSN()
}

// ConfigurePoolLimits sets connection pool constraints and lifecycle rotation limits on a *sql.DB instance.
func ConfigurePoolLimits(db *sql.DB, maxOpen, maxIdle int, maxLifetime, maxIdleTime time.Duration) {
	if db == nil {
		return
	}
	if maxOpen <= 0 {
		maxOpen = DefaultMaxOpenConns
	}
	if maxIdle <= 0 {
		maxIdle = DefaultMaxIdleConns
	}
	if maxLifetime <= 0 {
		maxLifetime = DefaultConnMaxLifetime
	}
	if maxIdleTime <= 0 {
		maxIdleTime = DefaultConnMaxIdleTime
	}

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(maxLifetime)
	db.SetConnMaxIdleTime(maxIdleTime)
}

// PoolStatsSummary captures real-time driver connection pool telemetry.
type PoolStatsSummary struct {
	MaxOpenConnections int
	OpenConnections    int
	InUse              int
	Idle               int
	WaitCount          int64
	WaitDuration       time.Duration
	MaxLifetimeClosed  int64
	MaxIdleTimeClosed  int64
}

// GetPoolStats extracts structured pool statistics from *sql.DB.
func GetPoolStats(db *sql.DB) PoolStatsSummary {
	if db == nil {
		return PoolStatsSummary{}
	}
	stats := db.Stats()
	return PoolStatsSummary{
		MaxOpenConnections: stats.MaxOpenConnections,
		OpenConnections:    stats.OpenConnections,
		InUse:              stats.InUse,
		Idle:               stats.Idle,
		WaitCount:          stats.WaitCount,
		WaitDuration:       stats.WaitDuration,
		MaxLifetimeClosed:  stats.MaxLifetimeClosed,
		MaxIdleTimeClosed:  stats.MaxIdleTimeClosed,
	}
}

// OpenWithConfig creates a *sql.DB connection pool with configured limits, without executing Ping.
func OpenWithConfig(cfg ConnectConfig) (*sql.DB, error) {
	dsn := BuildDSN(cfg)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	ConfigurePoolLimits(db, cfg.MaxOpenConns, cfg.MaxIdleConns, cfg.ConnMaxLifetime, cfg.ConnMaxIdleTime)

	return db, nil
}

// ConnectWithConfig establishes a database connection pool and pings the server to verify connectivity.
func ConnectWithConfig(cfg ConnectConfig) (*sql.DB, error) {
	db, err := OpenWithConfig(cfg)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// Connect establishes a connection using safe default timeouts and pool limits.
func Connect(user, pass, host, port string) (*sql.DB, error) {
	cfg := DefaultConnectConfig(user, pass, host, port)
	return ConnectWithConfig(cfg)
}

// ErrorCategory categorizes database connection errors for user presentation.
type ErrorCategory string

const (
	ErrCategoryTimeout      ErrorCategory = "TIMEOUT"
	ErrCategoryRefused      ErrorCategory = "REFUSED"
	ErrCategoryAuth         ErrorCategory = "AUTH"
	ErrCategoryDNS          ErrorCategory = "DNS"
	ErrCategoryDatabase     ErrorCategory = "DATABASE"
	ErrCategoryNetworkOther ErrorCategory = "NETWORK"
	ErrCategoryUnknown      ErrorCategory = "UNKNOWN"
)

// ConnectionErrorInfo contains structured diagnostic details for connection errors.
type ConnectionErrorInfo struct {
	Category ErrorCategory
	Title    string
	Message  string
	Hint     string
	RawErr   error
}

// CategorizeConnectionError inspects a connection error and returns user-friendly diagnostics.
func CategorizeConnectionError(err error) ConnectionErrorInfo {
	if err == nil {
		return ConnectionErrorInfo{}
	}

	info := ConnectionErrorInfo{
		RawErr: err,
	}

	// 1. Check for typed MySQL driver errors
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		switch mysqlErr.Number {
		case 1045, 1044:
			info.Category = ErrCategoryAuth
			info.Title = "Authentication Failed"
			info.Message = "Access denied for user. Please check your username and password."
			info.Hint = "Verify that the database username exists and the password is correct for this host."
			return info
		case 1049:
			info.Category = ErrCategoryDatabase
			info.Title = "Database Not Found"
			info.Message = fmt.Sprintf("The specified database does not exist: %s", mysqlErr.Message)
			info.Hint = "Verify the database name or connect without a specific database selected."
			return info
		default:
			info.Category = ErrCategoryUnknown
			info.Title = fmt.Sprintf("MySQL Server Error (%d)", mysqlErr.Number)
			info.Message = mysqlErr.Message
			info.Hint = "Consult MySQL server logs or check user permissions."
			return info
		}
	}

	// 2. Check for typed DNS errors
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		info.Category = ErrCategoryDNS
		info.Title = "DNS Resolution Failed"
		info.Message = fmt.Sprintf("Unable to resolve host: %s", dnsErr.Name)
		info.Hint = "Verify that the hostname is spelled correctly and network DNS is functioning."
		return info
	}

	// 3. Check for timeout errors
	var netErr net.Error
	errLower := strings.ToLower(err.Error())
	if (errors.As(err, &netErr) && netErr.Timeout()) ||
		errors.Is(err, context.DeadlineExceeded) ||
		strings.Contains(errLower, "timeout") ||
		strings.Contains(errLower, "deadline exceeded") {
		info.Category = ErrCategoryTimeout
		info.Title = "Connection Timed Out"
		info.Message = "Connection attempt timed out before the server responded."
		info.Hint = "The host may be offline, unreachable, or blocked by a firewall. Verify host, port, and network route."
		return info
	}

	// 4. Check for connection refused / host actively rejecting
	if strings.Contains(errLower, "refused") ||
		strings.Contains(errLower, "actively refused") ||
		strings.Contains(errLower, "no connection could be made") ||
		strings.Contains(errLower, "connectex") {
		info.Category = ErrCategoryRefused
		info.Title = "Connection Refused"
		info.Message = "The host actively refused or rejected the connection attempt."
		info.Hint = "Verify that the MySQL service is actively running and listening on the specified port."
		return info
	}

	// 5. String-based DNS fallback
	if strings.Contains(errLower, "no such host") || strings.Contains(errLower, "getaddrinfo") {
		info.Category = ErrCategoryDNS
		info.Title = "Host Not Found"
		info.Message = "The specified database host could not be resolved."
		info.Hint = "Check host address spelling and ensure your network DNS is operational."
		return info
	}

	// 6. String-based Auth fallback
	if strings.Contains(errLower, "access denied") {
		info.Category = ErrCategoryAuth
		info.Title = "Authentication Failed"
		info.Message = "Access denied for user."
		info.Hint = "Verify database username, password, and host access grants."
		return info
	}

	// 7. Generic fallback
	info.Category = ErrCategoryNetworkOther
	info.Title = "Connection Error"
	info.Message = err.Error()
	info.Hint = "Verify network connectivity and database server status."
	return info
}
