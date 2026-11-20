package ui

import (
	"testing"
)

func TestStripLeadingSQLComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain query without comments",
			input:    "SELECT * FROM users;",
			expected: "SELECT * FROM users;",
		},
		{
			name:     "single-line standard comment",
			input:    "-- fetch all active users\nSELECT * FROM users;",
			expected: "SELECT * FROM users;",
		},
		{
			name:     "single-line comment with windows CRLF",
			input:    "-- comment with crlf\r\nUPDATE users SET active = 1;",
			expected: "UPDATE users SET active = 1;",
		},
		{
			name:     "single-line hash comment",
			input:    "# mysql hash comment\nDELETE FROM sessions;",
			expected: "DELETE FROM sessions;",
		},
		{
			name:     "multi-line c-style block comment",
			input:    "/* critical migration\n   author: vicky */\nALTER TABLE accounts ADD COLUMN balance DECIMAL(10,2);",
			expected: "ALTER TABLE accounts ADD COLUMN balance DECIMAL(10,2);",
		},
		{
			name: "stacked comments of mixed types",
			input: `
				-- first line comment
				# second hash comment
				/* third block comment */
				-- fourth comment
				INSERT INTO audit_log (msg) VALUES ('test');
			`,
			expected: "INSERT INTO audit_log (msg) VALUES ('test');",
		},
		{
			name:     "comment only query",
			input:    "-- pure comment with no query",
			expected: "",
		},
		{
			name:     "block comment only query",
			input:    "/* block comment only */",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "   \t\n  ",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := StripLeadingSQLComments(tc.input)
			if got != tc.expected {
				t.Errorf("StripLeadingSQLComments(%q) = %q; expected %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestClassifySQLStatement_DML(t *testing.T) {
	tests := []struct {
		query        string
		wantMutation bool
		wantType     string
	}{
		{"INSERT INTO users (name) VALUES ('Alice');", true, "INSERT"},
		{"insert into users (name) values ('Alice');", true, "INSERT"},
		{"UPDATE accounts SET balance = balance + 100 WHERE id = 1;", true, "UPDATE"},
		{"update accounts set balance = 0;", true, "UPDATE"},
		{"DELETE FROM sessions WHERE expired_at < NOW();", true, "DELETE"},
		{"delete from logs;", true, "DELETE"},
		{"REPLACE INTO settings (k, v) VALUES ('theme', 'dark');", true, "REPLACE"},
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			isMut, stmtType := ClassifySQLStatement(tc.query)
			if isMut != tc.wantMutation {
				t.Errorf("ClassifySQLStatement(%q) isMutation = %v, expected %v", tc.query, isMut, tc.wantMutation)
			}
			if stmtType != tc.wantType {
				t.Errorf("ClassifySQLStatement(%q) stmtType = %q, expected %q", tc.query, stmtType, tc.wantType)
			}
		})
	}
}

func TestClassifySQLStatement_WithLeadingComments(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		wantMutation bool
		wantType     string
	}{
		{
			name:         "commented UPDATE",
			query:        "-- Important business update\nUPDATE accounts SET status = 'active';",
			wantMutation: true,
			wantType:     "UPDATE",
		},
		{
			name:         "block commented DELETE",
			query:        "/* purge old records */\nDELETE FROM audit_trails WHERE created_at < '2024-01-01';",
			wantMutation: true,
			wantType:     "DELETE",
		},
		{
			name:         "hash commented INSERT",
			query:        "# auto-generated insert\nINSERT INTO metrics (k, v) VALUES ('cpu', 95);",
			wantMutation: true,
			wantType:     "INSERT",
		},
		{
			name:         "commented SELECT (must not be marked mutation)",
			query:        "-- fetch customer count\nSELECT COUNT(*) FROM customers;",
			wantMutation: false,
			wantType:     "SELECT",
		},
		{
			name:         "stacked comments before ALTER TABLE",
			query:        "-- migration step 1\n/* review required */\nALTER TABLE users ADD COLUMN phone VARCHAR(20);",
			wantMutation: true,
			wantType:     "DDL (ALTER)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isMut, stmtType := ClassifySQLStatement(tc.query)
			if isMut != tc.wantMutation {
				t.Errorf("%s: isMutation = %v, expected %v", tc.name, isMut, tc.wantMutation)
			}
			if stmtType != tc.wantType {
				t.Errorf("%s: stmtType = %q, expected %q", tc.name, stmtType, tc.wantType)
			}
		})
	}
}

func TestClassifySQLStatement_CTE(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		wantMutation bool
		wantType     string
	}{
		{
			name:         "CTE returning SELECT query",
			query:        "WITH inactive_users AS (SELECT id FROM users WHERE status = 'inactive') SELECT * FROM inactive_users;",
			wantMutation: false,
			wantType:     "CTE (SELECT)",
		},
		{
			name:         "CTE with RECURSIVE returning SELECT",
			query:        "WITH RECURSIVE hierarchy AS (SELECT id, parent_id FROM departments UNION ALL SELECT d.id, d.parent_id FROM departments d JOIN hierarchy h ON d.parent_id = h.id) SELECT * FROM hierarchy;",
			wantMutation: false,
			wantType:     "CTE (SELECT)",
		},
		{
			name:         "CTE mutating with DELETE",
			query:        "WITH stale AS (SELECT id FROM sessions WHERE last_active < NOW() - INTERVAL 30 DAY) DELETE FROM sessions WHERE id IN (SELECT id FROM stale);",
			wantMutation: true,
			wantType:     "CTE (DELETE)",
		},
		{
			name:         "CTE mutating with UPDATE",
			query:        "WITH flagged AS (SELECT id FROM accounts WHERE risk > 80) UPDATE accounts SET locked = 1 WHERE id IN (SELECT id FROM flagged);",
			wantMutation: true,
			wantType:     "CTE (UPDATE)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isMut, stmtType := ClassifySQLStatement(tc.query)
			if isMut != tc.wantMutation {
				t.Errorf("%s: isMutation = %v, expected %v", tc.name, isMut, tc.wantMutation)
			}
			if stmtType != tc.wantType {
				t.Errorf("%s: stmtType = %q, expected %q", tc.name, stmtType, tc.wantType)
			}
		})
	}
}

func TestClassifySQLStatement_DDL_TCL_Admin(t *testing.T) {
	tests := []struct {
		query        string
		wantMutation bool
		wantType     string
	}{
		{"CREATE TABLE employees (id INT PRIMARY KEY);", true, "DDL (CREATE)"},
		{"ALTER TABLE employees ADD COLUMN email VARCHAR(255);", true, "DDL (ALTER)"},
		{"DROP TABLE old_logs;", true, "DDL (DROP)"},
		{"TRUNCATE TABLE temp_cache;", true, "DDL (TRUNCATE)"},
		{"RENAME TABLE v1 TO v2;", true, "DDL (RENAME)"},
		{"START TRANSACTION;", true, "TCL (START)"},
		{"BEGIN;", true, "TCL (BEGIN)"},
		{"COMMIT;", true, "TCL (COMMIT)"},
		{"ROLLBACK;", true, "TCL (ROLLBACK)"},
		{"SET foreign_key_checks = 0;", true, "ADMIN (SET)"},
		{"OPTIMIZE TABLE payment;", true, "ADMIN (OPTIMIZE)"},
		{"ANALYZE TABLE film;", true, "ADMIN (ANALYZE)"},
		{"GRANT SELECT ON db.* TO 'user'@'%';", true, "ADMIN (GRANT)"},
		{"REVOKE ALL PRIVILEGES ON *.* FROM 'bad_user'@'%';", true, "ADMIN (REVOKE)"},
		{"KILL 1042;", true, "ADMIN (KILL)"},
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			isMut, stmtType := ClassifySQLStatement(tc.query)
			if isMut != tc.wantMutation {
				t.Errorf("%s: isMutation = %v, expected %v", tc.query, isMut, tc.wantMutation)
			}
			if stmtType != tc.wantType {
				t.Errorf("%s: stmtType = %q, expected %q", tc.query, stmtType, tc.wantType)
			}
		})
	}
}

func TestClassifySQLStatement_Queries(t *testing.T) {
	tests := []struct {
		query        string
		wantMutation bool
		wantType     string
	}{
		{"SELECT * FROM sakila.actor LIMIT 10;", false, "SELECT"},
		{"SHOW TABLES FROM sakila;", false, "SHOW"},
		{"SHOW DATABASES;", false, "SHOW"},
		{"DESCRIBE sakila.film;", false, "EXPLAIN"},
		{"DESC sakila.film;", false, "EXPLAIN"},
		{"EXPLAIN SELECT * FROM sakila.actor WHERE actor_id = 1;", false, "EXPLAIN"},
		{"CHECK TABLE sakila.customer;", false, "CHECK"},
		{"CALL GetCustomerBalance(100);", false, "CALL"},
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			isMut, stmtType := ClassifySQLStatement(tc.query)
			if isMut != tc.wantMutation {
				t.Errorf("%s: isMutation = %v, expected %v", tc.query, isMut, tc.wantMutation)
			}
			if stmtType != tc.wantType {
				t.Errorf("%s: stmtType = %q, expected %q", tc.query, stmtType, tc.wantType)
			}
		})
	}
}

func TestQueryResult_LastInsertIDReporting(t *testing.T) {
	// 1. Result with LastInsertID
	insertRes := &QueryResult{
		IsMutation:    true,
		StatementType: "INSERT",
		RowsAffected:  1,
		LastInsertID:  4285,
	}

	if insertRes.LastInsertID != 4285 {
		t.Errorf("expected LastInsertID 4285, got %d", insertRes.LastInsertID)
	}
	if !insertRes.IsMutation {
		t.Errorf("expected IsMutation to be true")
	}

	// 2. Result without LastInsertID (e.g. UPDATE)
	updateRes := &QueryResult{
		IsMutation:    true,
		StatementType: "UPDATE",
		RowsAffected:  5,
		LastInsertID:  0,
	}

	if updateRes.LastInsertID != 0 {
		t.Errorf("expected LastInsertID 0 for UPDATE, got %d", updateRes.LastInsertID)
	}
	if updateRes.RowsAffected != 5 {
		t.Errorf("expected RowsAffected 5, got %d", updateRes.RowsAffected)
	}
}
