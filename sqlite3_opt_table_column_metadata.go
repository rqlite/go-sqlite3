//go:build !libsqlite3 || sqlite_column_metadata
// +build !libsqlite3 sqlite_column_metadata

package sqlite3

/*
#ifndef USE_LIBSQLITE3
#cgo CFLAGS: -DSQLITE_ENABLE_COLUMN_METADATA
#include <sqlite3-binding.h>
#else
#include <sqlite3.h>
#endif
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// TableColumnMetadata returns metadata information about a column in a table.
//
// The function takes a schema (database name, can be empty for "main"), 
// table name, and column name, and returns:
// - dataType: the declared data type of the column
// - collSeq: the collation sequence name for the column
// - notNull: true if the column has a NOT NULL constraint
// - primaryKey: true if the column is part of the primary key
// - autoinc: true if the column is auto-increment
//
// See https://www.sqlite.org/c3ref/table_column_metadata.html
func (c *SQLiteConn) TableColumnMetadata(schema string, table string, column string) (dataType string, collSeq string, notNull bool, primaryKey bool, autoinc bool, err error) {
	if table == "" {
		return "", "", false, false, false, fmt.Errorf("table name cannot be empty")
	}
	if column == "" {
		return "", "", false, false, false, fmt.Errorf("column name cannot be empty")
	}

	var zSchema *C.char
	var zTable *C.char
	var zColumn *C.char

	// Convert Go strings to C strings
	if schema != "" {
		zSchema = C.CString(schema)
		defer C.free(unsafe.Pointer(zSchema))
	}
	zTable = C.CString(table)
	defer C.free(unsafe.Pointer(zTable))
	zColumn = C.CString(column)
	defer C.free(unsafe.Pointer(zColumn))

	// Output parameters
	var pzDataType *C.char
	var pzCollSeq *C.char
	var pNotNull C.int
	var pPrimaryKey C.int
	var pAutoinc C.int

	// Call the C function
	rc := C.sqlite3_table_column_metadata(
		c.db,
		zSchema,
		zTable,
		zColumn,
		&pzDataType,
		&pzCollSeq,
		&pNotNull,
		&pPrimaryKey,
		&pAutoinc,
	)

	if rc != C.SQLITE_OK {
		return "", "", false, false, false, fmt.Errorf("sqlite3_table_column_metadata failed: %s", C.GoString(C.sqlite3_errmsg(c.db)))
	}

	// Convert C results to Go types
	var goDataType, goCollSeq string
	if pzDataType != nil {
		goDataType = C.GoString(pzDataType)
	}
	if pzCollSeq != nil {
		goCollSeq = C.GoString(pzCollSeq)
	}

	return goDataType, goCollSeq, pNotNull != 0, pPrimaryKey != 0, pAutoinc != 0, nil
}