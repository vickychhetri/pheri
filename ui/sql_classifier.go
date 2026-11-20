package ui

import (
	"strings"
	"unicode"
)

// Statement categories
const (
	StmtSelect  = "SELECT"
	StmtInsert  = "INSERT"
	StmtUpdate  = "UPDATE"
	StmtDelete  = "DELETE"
	StmtReplace = "REPLACE"
	StmtDDL     = "DDL"
	StmtTCL     = "TCL"
	StmtAdmin   = "ADMIN"
	StmtShow    = "SHOW"
	StmtExplain = "EXPLAIN"
	StmtCall    = "CALL"
	StmtUnknown = "QUERY"
)

// StripLeadingSQLComments strips all leading SQL comments and whitespace from a query string.
// Supports:
// - Standard SQL single-line comments: -- comment\n
// - MySQL hash single-line comments: # comment\n
// - C-style multi-line comments: /* comment */ (including nested or stacked comments)
func StripLeadingSQLComments(query string) string {
	s := strings.TrimSpace(query)

	for {
		changed := false

		// 1. Single-line comment with --
		if strings.HasPrefix(s, "--") {
			// In standard SQL, -- requires a space, control char, or newline after it.
			// However, in user queries, users often write --comment. We strip until newline.
			idx := strings.IndexAny(s, "\r\n")
			if idx == -1 {
				return ""
			}
			s = strings.TrimSpace(s[idx:])
			changed = true
			continue
		}

		// 2. Single-line comment with #
		if strings.HasPrefix(s, "#") {
			idx := strings.IndexAny(s, "\r\n")
			if idx == -1 {
				return ""
			}
			s = strings.TrimSpace(s[idx:])
			changed = true
			continue
		}

		// 3. Multi-line comment /* ... */
		if strings.HasPrefix(s, "/*") {
			idx := strings.Index(s, "*/")
			if idx == -1 {
				// Unclosed comment
				return ""
			}
			s = strings.TrimSpace(s[idx+2:])
			changed = true
			continue
		}

		if !changed {
			break
		}
	}

	return s
}

// ClassifySQLStatement analyzes an input SQL string to determine whether it is a mutation
// (should execute via ExecContext) or a result set query (should execute via QueryContext),
// and identifies the high-level statement type.
//
// It handles leading comments, whitespace, and Common Table Expressions (WITH cte AS ...).
func ClassifySQLStatement(query string) (isMutation bool, stmtType string) {
	clean := StripLeadingSQLComments(query)
	if clean == "" {
		return false, StmtUnknown
	}

	// Tokenize first word
	firstWord := extractFirstWord(clean)
	upperFirst := strings.ToUpper(firstWord)

	// Handle Common Table Expressions (WITH ... SELECT vs WITH ... UPDATE/DELETE/INSERT)
	if upperFirst == "WITH" {
		return classifyCTE(clean)
	}

	switch upperFirst {
	// Result set queries
	case "SELECT":
		return false, StmtSelect
	case "SHOW":
		return false, StmtShow
	case "DESC", "DESCRIBE", "EXPLAIN":
		return false, StmtExplain
	case "CHECK", "CHECKSUM":
		return false, "CHECK"

	// DML mutations
	case "INSERT":
		return true, StmtInsert
	case "UPDATE":
		return true, StmtUpdate
	case "DELETE":
		return true, StmtDelete
	case "REPLACE":
		return true, StmtReplace

	// DDL mutations
	case "CREATE", "ALTER", "DROP", "TRUNCATE", "RENAME":
		return true, StmtDDL + " (" + upperFirst + ")"

	// TCL mutations
	case "START", "BEGIN", "COMMIT", "ROLLBACK", "SAVEPOINT":
		return true, StmtTCL + " (" + upperFirst + ")"

	// Admin / Session / Table maintenance
	case "SET", "GRANT", "REVOKE", "LOCK", "UNLOCK", "FLUSH", "RESET",
		"PURGE", "KILL", "OPTIMIZE", "ANALYZE", "REPAIR", "LOAD", "INSTALL", "UNINSTALL":
		return true, StmtAdmin + " (" + upperFirst + ")"

	// Stored routine invocation (hybrid: may return rows or mutate)
	case "CALL":
		return false, StmtCall

	default:
		// Default to non-mutation query so standard QueryContext attempts to fetch rows
		return false, upperFirst
	}
}

// extractFirstWord extracts the first contiguous word/token of a string.
func extractFirstWord(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if unicode.IsSpace(r) || r == '(' || r == ';' {
			break
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// classifyCTE analyzes queries starting with WITH to determine if the terminal query is a mutation or SELECT.
func classifyCTE(s string) (bool, string) {
	// Scan past WITH clauses by tracking parenthesis depth
	// WITH cte1 AS (...), cte2 AS (...) [SELECT | UPDATE | DELETE | INSERT]
	inQuote := false
	var quoteChar rune
	parenDepth := 0
	i := 0
	runes := []rune(s)
	n := len(runes)

	// Skip initial "WITH"
	for i < n && !unicode.IsSpace(runes[i]) {
		i++
	}

	for i < n {
		r := runes[i]

		// Handle string literals inside CTE definition
		if inQuote {
			if r == quoteChar {
				// Check for escaped quote
				if i+1 < n && runes[i+1] == quoteChar {
					i += 2
					continue
				}
				inQuote = false
			}
			i++
			continue
		}

		if r == '\'' || r == '"' || r == '`' {
			inQuote = true
			quoteChar = r
			i++
			continue
		}

		// Handle parentheses depth
		if r == '(' {
			parenDepth++
			i++
			continue
		}
		if r == ')' {
			if parenDepth > 0 {
				parenDepth--
			}
			i++
			continue
		}

		// When at depth 0, look for statement keywords after the CTEs
		if parenDepth == 0 && unicode.IsLetter(r) {
			// Extract word
			start := i
			for i < n && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '_') {
				i++
			}
			word := strings.ToUpper(string(runes[start:i]))

			// Skip standard CTE declaration keywords: RECURSIVE, AS
			if word == "RECURSIVE" || word == "AS" {
				continue
			}

			// If the word matches a statement keyword:
			switch word {
			case "SELECT":
				return false, "CTE (SELECT)"
			case "UPDATE":
				return true, "CTE (UPDATE)"
			case "DELETE":
				return true, "CTE (DELETE)"
			case "INSERT":
				return true, "CTE (INSERT)"
			case "REPLACE":
				return true, "CTE (REPLACE)"
			}
			continue
		}

		i++
	}

	// Fallback for CTE: assume SELECT if not identified
	return false, "CTE"
}
