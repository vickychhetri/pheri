package util

import "testing"

func TestFormatSQLValueLiteral(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"42", "42"},
		{"-7", "-7"},
		{"0", "0"},
		{"00123", "'00123'"}, // Preserve leading zeros as string literal
		{"abc", "'abc'"},
		{"O'Reilly", "'O''Reilly'"},
		{`path\to'val`, `'path\\to''val'`},
		{"", "''"},
	}

	for _, tc := range tests {
		got := FormatSQLValueLiteral(tc.input)
		if got != tc.want {
			t.Errorf("FormatSQLValueLiteral(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestBuildForeignKeyTargetQuery(t *testing.T) {
	ref := ForeignKeyRef{
		ConstraintName:   "fk_rental_customer",
		ColumnName:       "customer_id",
		ReferencedSchema: "sakila",
		ReferencedTable:  "customer",
		ReferencedColumn: "customer_id",
	}
	got := BuildForeignKeyTargetQuery(ref, "15")
	want := "SELECT * FROM `sakila`.`customer` WHERE `customer_id` = 15 LIMIT 100"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// String key + missing ReferencedSchema fallback + backtick identifier escaping
	refStr := ForeignKeyRef{
		ColumnName:       "dept_code",
		ReferencedTable:  "depart`ments",
		ReferencedColumn: "code",
	}
	gotStr := BuildForeignKeyTargetQuery(refStr, "ENG'01")
	wantStr := "SELECT * FROM `depart``ments` WHERE `code` = 'ENG''01' LIMIT 100"
	if gotStr != wantStr {
		t.Errorf("got %q, want %q", gotStr, wantStr)
	}
}
