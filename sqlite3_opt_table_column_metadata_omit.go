//go:build libsqlite3 && !sqlite_column_metadata
// +build libsqlite3,!sqlite_column_metadata

package sqlite3

import (
	"errors"
)

// TableColumnMetadata returns an error when sqlite_column_metadata build tag is not set.
func (c *SQLiteConn) TableColumnMetadata(schema string, table string, column string) (string, string, bool, bool, bool, error) {
	return "", "", false, false, false, errors.New("sqlite3: TableColumnMetadata requires the sqlite_column_metadata build tag when using the libsqlite3 build tag")
}