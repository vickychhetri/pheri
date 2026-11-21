// dbs/mysql_test.go
package dbs

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

func TestDefaultConnectConfig(t *testing.T) {
	cfg := DefaultConnectConfig("root", "secret", "localhost", "3306")

	if cfg.User != "root" {
		t.Errorf("expected User 'root', got %s", cfg.User)
	}
	if cfg.Pass != "secret" {
		t.Errorf("expected Pass 'secret', got %s", cfg.Pass)
	}
	if cfg.Host != "localhost" {
		t.Errorf("expected Host 'localhost', got %s", cfg.Host)
	}
	if cfg.Port != "3306" {
		t.Errorf("expected Port '3306', got %s", cfg.Port)
	}
	if cfg.DialTimeout != 5*time.Second {
		t.Errorf("expected DialTimeout 5s, got %v", cfg.DialTimeout)
	}
	if cfg.ReadTimeout != 60*time.Second {
		t.Errorf("expected ReadTimeout 60s, got %v", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 60*time.Second {
		t.Errorf("expected WriteTimeout 60s, got %v", cfg.WriteTimeout)
	}
	if cfg.MaxOpenConns != 25 {
		t.Errorf("expected MaxOpenConns 25, got %d", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 5 {
		t.Errorf("expected MaxIdleConns 5, got %d", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != 10*time.Minute {
		t.Errorf("expected ConnMaxLifetime 10m, got %v", cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime != 3*time.Minute {
		t.Errorf("expected ConnMaxIdleTime 3m, got %v", cfg.ConnMaxIdleTime)
	}
	if cfg.Collation != "utf8mb4_general_ci" {
		t.Errorf("expected Collation utf8mb4_general_ci, got %s", cfg.Collation)
	}
}

func TestBuildDSN_Defaults(t *testing.T) {
	cfg := DefaultConnectConfig("admin", "p@ssword", "127.0.0.1", "3306")
	dsn := BuildDSN(cfg)

	// Verify using driver's ParseDSN to validate roundtrip
	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("failed to parse generated DSN: %v\nDSN: %s", err, dsn)
	}

	if parsed.User != "admin" {
		t.Errorf("expected user 'admin', got '%s'", parsed.User)
	}
	if parsed.Passwd != "p@ssword" {
		t.Errorf("expected password 'p@ssword', got '%s'", parsed.Passwd)
	}
	if parsed.Addr != "127.0.0.1:3306" {
		t.Errorf("expected addr '127.0.0.1:3306', got '%s'", parsed.Addr)
	}
	if parsed.Timeout != 5*time.Second {
		t.Errorf("expected Timeout 5s, got %v", parsed.Timeout)
	}
	if parsed.ReadTimeout != 60*time.Second {
		t.Errorf("expected ReadTimeout 60s, got %v", parsed.ReadTimeout)
	}
	if parsed.WriteTimeout != 60*time.Second {
		t.Errorf("expected WriteTimeout 60s, got %v", parsed.WriteTimeout)
	}
	if !parsed.ParseTime {
		t.Errorf("expected ParseTime true")
	}
	if !parsed.MultiStatements {
		t.Errorf("expected MultiStatements true")
	}
	if parsed.Collation != "utf8mb4_general_ci" {
		t.Errorf("expected Collation utf8mb4_general_ci, got %s", parsed.Collation)
	}
}

func TestBuildDSN_CustomAndEscaping(t *testing.T) {
	cfg := ConnectConfig{
		User:            "complex_user",
		Pass:            "p@ss:w/ord#%&!",
		Host:            "db.example.internal",
		Port:            "3307",
		Database:        "production_db",
		DialTimeout:     10 * time.Second,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    45 * time.Second,
		Collation:       "utf8mb4_unicode_ci",
		MaxOpenConns:    10,
		MaxIdleConns:    2,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}

	dsn := BuildDSN(cfg)
	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("failed to parse DSN: %v\nDSN: %s", err, dsn)
	}

	if parsed.User != "complex_user" {
		t.Errorf("expected User 'complex_user', got %s", parsed.User)
	}
	if parsed.Passwd != "p@ss:w/ord#%&!" {
		t.Errorf("expected Pass %q, got %s", "p@ss:w/ord#%&!", parsed.Passwd)
	}
	if parsed.DBName != "production_db" {
		t.Errorf("expected DBName 'production_db', got %s", parsed.DBName)
	}
	if parsed.Addr != "db.example.internal:3307" {
		t.Errorf("expected Addr 'db.example.internal:3307', got %s", parsed.Addr)
	}
	if parsed.Timeout != 10*time.Second {
		t.Errorf("expected Timeout 10s, got %v", parsed.Timeout)
	}
	if parsed.ReadTimeout != 30*time.Second {
		t.Errorf("expected ReadTimeout 30s, got %v", parsed.ReadTimeout)
	}
	if parsed.WriteTimeout != 45*time.Second {
		t.Errorf("expected WriteTimeout 45s, got %v", parsed.WriteTimeout)
	}
	if parsed.Collation != "utf8mb4_unicode_ci" {
		t.Errorf("expected Collation utf8mb4_unicode_ci, got %s", parsed.Collation)
	}
}

func TestBuildDSN_EmptyHostPortFallback(t *testing.T) {
	cfg := ConnectConfig{
		User: "root",
		Pass: "",
	}
	dsn := BuildDSN(cfg)
	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("failed to parse DSN: %v", err)
	}
	if parsed.Addr != "127.0.0.1:3306" {
		t.Errorf("expected default fallback 127.0.0.1:3306, got %s", parsed.Addr)
	}
}

func TestOpenWithConfig_PoolSettings(t *testing.T) {
	cfg := ConnectConfig{
		User:            "root",
		Pass:            "test",
		Host:            "127.0.0.1",
		Port:            "3306",
		MaxOpenConns:    17,
		MaxIdleConns:    7,
		ConnMaxLifetime: 8 * time.Minute,
		ConnMaxIdleTime: 2 * time.Minute,
	}

	db, err := OpenWithConfig(cfg)
	if err != nil {
		t.Fatalf("OpenWithConfig returned error: %v", err)
	}
	defer db.Close()

	stats := db.Stats()
	if stats.MaxOpenConnections != 17 {
		t.Errorf("expected MaxOpenConnections 17, got %d", stats.MaxOpenConnections)
	}
}

func TestConnectionTimeoutEnforcement(t *testing.T) {
	// Connect to non-routable address (RFC 5737 TEST-NET-1) with a brief 200ms timeout.
	// Without timeout config, this would hang for 20-120 seconds.
	cfg := ConnectConfig{
		User:         "root",
		Pass:         "pass",
		Host:         "192.0.2.1",
		Port:         "3306",
		DialTimeout:  200 * time.Millisecond,
		ReadTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
	}

	start := time.Now()
	_, err := ConnectWithConfig(cfg)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected connection to non-routable host to fail, but it succeeded")
	}

	// Should complete within 2.5s (200ms dial + minor kernel/dns overhead), far below OS default SYN timeout
	if elapsed > 2500*time.Millisecond {
		t.Errorf("connection attempt took too long (%v), timeout safeguard did not enforce promptly", elapsed)
	}

	errInfo := CategorizeConnectionError(err)
	if errInfo.Category != ErrCategoryTimeout && errInfo.Category != ErrCategoryRefused && errInfo.Category != ErrCategoryDNS {
		t.Errorf("expected timeout, refused, or dns category, got: %s (err: %v)", errInfo.Category, err)
	}
}

type mockNetTimeoutError struct{}

func (m mockNetTimeoutError) Error() string   { return "i/o timeout" }
func (m mockNetTimeoutError) Timeout() bool   { return true }
func (m mockNetTimeoutError) Temporary() bool { return true }

func TestCategorizeConnectionError(t *testing.T) {
	tests := []struct {
		name             string
		err              error
		expectedCategory ErrorCategory
		titleContains    string
	}{
		{
			name:             "nil error",
			err:              nil,
			expectedCategory: "",
			titleContains:    "",
		},
		{
			name: "mysql error 1045 auth failed",
			err: &mysql.MySQLError{
				Number:  1045,
				Message: "Access denied for user 'root'@'localhost' (using password: YES)",
			},
			expectedCategory: ErrCategoryAuth,
			titleContains:    "Authentication Failed",
		},
		{
			name: "mysql error 1044 db access denied",
			err: &mysql.MySQLError{
				Number:  1044,
				Message: "Access denied for user 'test'@'%' to database 'secure_db'",
			},
			expectedCategory: ErrCategoryAuth,
			titleContains:    "Authentication Failed",
		},
		{
			name: "mysql error 1049 db not found",
			err: &mysql.MySQLError{
				Number:  1049,
				Message: "Unknown database 'missing_db'",
			},
			expectedCategory: ErrCategoryDatabase,
			titleContains:    "Database Not Found",
		},
		{
			name: "mysql error generic",
			err: &mysql.MySQLError{
				Number:  1146,
				Message: "Table 'test.users' doesn't exist",
			},
			expectedCategory: ErrCategoryUnknown,
			titleContains:    "MySQL Server Error (1146)",
		},
		{
			name: "dns error typed",
			err: &net.DNSError{
				Err:  "no such host",
				Name: "unresolved.internal.domain",
			},
			expectedCategory: ErrCategoryDNS,
			titleContains:    "DNS Resolution Failed",
		},
		{
			name:             "net timeout error typed",
			err:              mockNetTimeoutError{},
			expectedCategory: ErrCategoryTimeout,
			titleContains:    "Connection Timed Out",
		},
		{
			name:             "context deadline exceeded",
			err:              context.DeadlineExceeded,
			expectedCategory: ErrCategoryTimeout,
			titleContains:    "Connection Timed Out",
		},
		{
			name:             "string io timeout",
			err:              errors.New("dial tcp 10.0.0.1:3306: i/o timeout"),
			expectedCategory: ErrCategoryTimeout,
			titleContains:    "Connection Timed Out",
		},
		{
			name:             "connection refused linux",
			err:              errors.New("dial tcp 127.0.0.1:3306: connect: connection refused"),
			expectedCategory: ErrCategoryRefused,
			titleContains:    "Connection Refused",
		},
		{
			name:             "connection refused windows connectex",
			err:              errors.New("dial tcp 127.0.0.1:3306: connectex: No connection could be made because the target machine actively refused it."),
			expectedCategory: ErrCategoryRefused,
			titleContains:    "Connection Refused",
		},
		{
			name:             "string host lookup failure",
			err:              errors.New("dial tcp: lookup nonexistingdb.host: no such host"),
			expectedCategory: ErrCategoryDNS,
			titleContains:    "Host Not Found",
		},
		{
			name:             "string access denied fallback",
			err:              errors.New("driver: Access denied for user 'foo'@'bar'"),
			expectedCategory: ErrCategoryAuth,
			titleContains:    "Authentication Failed",
		},
		{
			name:             "unknown network error",
			err:              errors.New("read tcp 127.0.0.1:54321->127.0.0.1:3306: wsarecv: An existing connection was forcibly closed by the remote host."),
			expectedCategory: ErrCategoryNetworkOther,
			titleContains:    "Connection Error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := CategorizeConnectionError(tc.err)
			if res.Category != tc.expectedCategory {
				t.Errorf("expected category '%s', got '%s'", tc.expectedCategory, res.Category)
			}
			if tc.titleContains != "" && !strings.Contains(res.Title, tc.titleContains) {
				t.Errorf("expected title to contain '%s', got '%s'", tc.titleContains, res.Title)
			}
			if tc.err != nil {
				if res.Hint == "" {
					t.Errorf("expected non-empty diagnostic hint for error '%v'", tc.err)
				}
				if res.RawErr != tc.err {
					t.Errorf("expected RawErr to match input error")
				}
			}
		})
	}
}
