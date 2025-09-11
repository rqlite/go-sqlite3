//go:build !libsqlite3 || sqlite_column_metadata
// +build !libsqlite3 sqlite_column_metadata

package sqlite3

import "testing"

func TestTableColumnMetadata(t *testing.T) {
	d := SQLiteDriver{}
	conn, err := d.Open(":memory:")
	if err != nil {
		t.Fatal("failed to get database connection:", err)
	}
	defer conn.Close()
	sqlite3conn := conn.(*SQLiteConn)

	// Create a test table with various column types and constraints
	_, err = sqlite3conn.Exec(`CREATE TABLE test_table (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL COLLATE NOCASE,
		value REAL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`, nil)
	if err != nil {
		t.Fatal("Failed to create table:", err)
	}

	// Test the id column (PRIMARY KEY, AUTOINCREMENT)
	dataType, collSeq, notNull, primaryKey, autoinc, err := sqlite3conn.TableColumnMetadata("", "test_table", "id")
	if err != nil {
		t.Fatal("TableColumnMetadata failed for id column:", err)
	}
	if dataType != "INTEGER" {
		t.Errorf("Expected dataType 'INTEGER' for id column, got '%s'", dataType)
	}
	if !primaryKey {
		t.Error("Expected id column to be primary key")
	}
	if !autoinc {
		t.Error("Expected id column to be auto-increment")
	}

	// Test the name column (TEXT, NOT NULL, COLLATE NOCASE)
	dataType, collSeq, notNull, primaryKey, autoinc, err = sqlite3conn.TableColumnMetadata("", "test_table", "name")
	if err != nil {
		t.Fatal("TableColumnMetadata failed for name column:", err)
	}
	if dataType != "TEXT" {
		t.Errorf("Expected dataType 'TEXT' for name column, got '%s'", dataType)
	}
	if collSeq != "NOCASE" {
		t.Errorf("Expected collation 'NOCASE' for name column, got '%s'", collSeq)
	}
	if !notNull {
		t.Error("Expected name column to have NOT NULL constraint")
	}
	if primaryKey {
		t.Error("Expected name column to not be primary key")
	}
	if autoinc {
		t.Error("Expected name column to not be auto-increment")
	}

	// Test the value column (REAL, nullable)
	dataType, collSeq, notNull, primaryKey, autoinc, err = sqlite3conn.TableColumnMetadata("", "test_table", "value")
	if err != nil {
		t.Fatal("TableColumnMetadata failed for value column:", err)
	}
	if dataType != "REAL" {
		t.Errorf("Expected dataType 'REAL' for value column, got '%s'", dataType)
	}
	if notNull {
		t.Error("Expected value column to be nullable")
	}
	if primaryKey {
		t.Error("Expected value column to not be primary key")
	}
	if autoinc {
		t.Error("Expected value column to not be auto-increment")
	}

	// Test the created_at column (DATETIME)
	dataType, collSeq, notNull, primaryKey, autoinc, err = sqlite3conn.TableColumnMetadata("", "test_table", "created_at")
	if err != nil {
		t.Fatal("TableColumnMetadata failed for created_at column:", err)
	}
	if dataType != "DATETIME" {
		t.Errorf("Expected dataType 'DATETIME' for created_at column, got '%s'", dataType)
	}
	if notNull {
		t.Error("Expected created_at column to be nullable")
	}
	if primaryKey {
		t.Error("Expected created_at column to not be primary key")
	}
	if autoinc {
		t.Error("Expected created_at column to not be auto-increment")
	}
}

func TestTableColumnMetadataErrors(t *testing.T) {
	d := SQLiteDriver{}
	conn, err := d.Open(":memory:")
	if err != nil {
		t.Fatal("failed to get database connection:", err)
	}
	defer conn.Close()
	sqlite3conn := conn.(*SQLiteConn)

	// Test with empty table name
	_, _, _, _, _, err = sqlite3conn.TableColumnMetadata("", "", "column")
	if err == nil {
		t.Error("Expected error for empty table name")
	}

	// Test with empty column name
	_, _, _, _, _, err = sqlite3conn.TableColumnMetadata("", "table", "")
	if err == nil {
		t.Error("Expected error for empty column name")
	}

	// Test with non-existent table
	_, _, _, _, _, err = sqlite3conn.TableColumnMetadata("", "nonexistent_table", "column")
	if err == nil {
		t.Error("Expected error for non-existent table")
	}

	// Create a table and test with non-existent column
	_, err = sqlite3conn.Exec(`CREATE TABLE test_table (id INTEGER)`, nil)
	if err != nil {
		t.Fatal("Failed to create table:", err)
	}

	_, _, _, _, _, err = sqlite3conn.TableColumnMetadata("", "test_table", "nonexistent_column")
	if err == nil {
		t.Error("Expected error for non-existent column")
	}
}

func TestTableColumnMetadataWithSchema(t *testing.T) {
	d := SQLiteDriver{}
	conn, err := d.Open(":memory:")
	if err != nil {
		t.Fatal("failed to get database connection:", err)
	}
	defer conn.Close()
	sqlite3conn := conn.(*SQLiteConn)

	// Create a test table
	_, err = sqlite3conn.Exec(`CREATE TABLE test_table (id INTEGER PRIMARY KEY)`, nil)
	if err != nil {
		t.Fatal("Failed to create table:", err)
	}

	// Test with explicit "main" schema
	dataType, _, _, primaryKey, _, err := sqlite3conn.TableColumnMetadata("main", "test_table", "id")
	if err != nil {
		t.Fatal("TableColumnMetadata failed with main schema:", err)
	}
	if dataType != "INTEGER" {
		t.Errorf("Expected dataType 'INTEGER', got '%s'", dataType)
	}
	if !primaryKey {
		t.Error("Expected id column to be primary key")
	}
}