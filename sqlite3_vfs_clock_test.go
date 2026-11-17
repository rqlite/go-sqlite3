// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

//go:build cgo
// +build cgo

package sqlite3

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"
)

// mustRegisterClockVFS registers a ClockVFS named after the test, and
// unregisters it when the test completes.
func mustRegisterClockVFS(t *testing.T, suffix string) *ClockVFS {
	t.Helper()
	vfs, err := RegisterClockVFS("clock-" + t.Name() + suffix)
	if err != nil {
		t.Fatalf("failed to register VFS: %s", err)
	}
	t.Cleanup(func() {
		if err := vfs.Unregister(); err != nil {
			t.Errorf("failed to unregister VFS: %s", err)
		}
	})
	return vfs
}

// mustOpenWithVFS opens the database at path with the named VFS, or with the
// default VFS if name is empty. The database is closed when the test
// completes, before any VFS registered earlier in the test is unregistered.
func mustOpenWithVFS(t *testing.T, path, name string) *sql.DB {
	t.Helper()
	dsn := "file:" + path
	if name != "" {
		dsn += "?vfs=" + name
	}
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("failed to open database: %s", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("failed to ping database: %s", err)
	}
	return db
}

func mustTempDB(t *testing.T) string {
	t.Helper()
	path := TempFilename(t)
	t.Cleanup(func() {
		os.Remove(path)
		os.Remove(path + "-wal")
		os.Remove(path + "-shm")
	})
	return path
}

func queryString(t *testing.T, db *sql.DB, query string, args ...any) string {
	t.Helper()
	var s string
	if err := db.QueryRow(query, args...).Scan(&s); err != nil {
		t.Fatalf("failed to query %q: %s", query, err)
	}
	return s
}

// Test_ClockVFS_Pinned tests that every way SQLite has of asking for the
// current time returns the pinned time.
func Test_ClockVFS_Pinned(t *testing.T) {
	vfs := mustRegisterClockVFS(t, "")
	db := mustOpenWithVFS(t, mustTempDB(t), vfs.Name())

	pinned := time.Date(2001, 2, 3, 4, 5, 6, 789000000, time.UTC)
	vfs.Pin(pinned)

	for _, tt := range []struct {
		query string
		args  []any
		exp   string
	}{
		{query: `SELECT datetime('now')`, exp: "2001-02-03 04:05:06"},
		{query: `SELECT datetime()`, exp: "2001-02-03 04:05:06"},
		{query: `SELECT datetime ( 'NOW' )`, exp: "2001-02-03 04:05:06"},
		{query: `SELECT date()`, exp: "2001-02-03"},
		{query: `SELECT time()`, exp: "04:05:06"},
		{query: `SELECT CURRENT_TIMESTAMP`, exp: "2001-02-03 04:05:06"},
		{query: `SELECT CURRENT_DATE`, exp: "2001-02-03"},
		{query: `SELECT CURRENT_TIME`, exp: "04:05:06"},
		{query: `SELECT unixepoch()`, exp: fmt.Sprint(pinned.Unix())},
		{query: `SELECT strftime('%s')`, exp: fmt.Sprint(pinned.Unix())},
		{query: `SELECT strftime('%Y-%m-%d %H:%M:%f', 'now')`, exp: "2001-02-03 04:05:06.789"},
		{query: `SELECT datetime('subsec')`, exp: "2001-02-03 04:05:06.789"},
		{query: `SELECT datetime('now', '+1 day')`, exp: "2001-02-04 04:05:06"},
		{query: `SELECT printf('%.6f', julianday())`, exp: "2451943.670217"},
		{query: `SELECT timediff('now', '2001-02-03 04:05:06.789')`, exp: "+0000-00-00 00:00:00.000"},
		// The time value supplied as a parameter.
		{query: `SELECT datetime(?)`, args: []any{"now"}, exp: "2001-02-03 04:05:06"},
		{query: `SELECT datetime(:t)`, args: []any{sql.Named("t", "now")}, exp: "2001-02-03 04:05:06"},
		// The time value taken from a row.
		{query: `SELECT datetime(v) FROM (SELECT 'now' AS v)`, exp: "2001-02-03 04:05:06"},
	} {
		if got := queryString(t, db, tt.query, tt.args...); got != tt.exp {
			t.Errorf("%s: exp %q, got %q", tt.query, tt.exp, got)
		}
	}
}

// Test_ClockVFS_PinnedSchema tests that expressions stored in the schema,
// which SQLite evaluates when a row is written, see the pinned time.
func Test_ClockVFS_PinnedSchema(t *testing.T) {
	vfs := mustRegisterClockVFS(t, "")
	db := mustOpenWithVFS(t, mustTempDB(t), vfs.Name())

	for _, stmt := range []string{
		`CREATE TABLE foo (id INTEGER PRIMARY KEY, name TEXT, ts TEXT DEFAULT CURRENT_TIMESTAMP, d TEXT DEFAULT (date()))`,
		`CREATE TABLE audit (id INTEGER, ts TEXT)`,
		`CREATE TRIGGER foo_ai AFTER INSERT ON foo BEGIN INSERT INTO audit VALUES (new.id, datetime('now')); END`,
		`CREATE VIEW foo_now AS SELECT id, datetime() AS ts FROM foo`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("failed to execute %q: %s", stmt, err)
		}
	}

	vfs.Pin(time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC))
	if _, err := db.Exec(`INSERT INTO foo (id, name) VALUES (1, 'fiona')`); err != nil {
		t.Fatalf("failed to insert: %s", err)
	}
	vfs.Pin(time.Date(2010, 11, 12, 13, 14, 15, 0, time.UTC))
	if _, err := db.Exec(`INSERT INTO foo (id, name) VALUES (2, 'declan')`); err != nil {
		t.Fatalf("failed to insert: %s", err)
	}

	for _, tt := range []struct {
		query string
		exp   string
	}{
		{query: `SELECT ts FROM foo WHERE id = 1`, exp: "2001-02-03 04:05:06"},
		{query: `SELECT d FROM foo WHERE id = 1`, exp: "2001-02-03"},
		{query: `SELECT ts FROM audit WHERE id = 1`, exp: "2001-02-03 04:05:06"},
		{query: `SELECT ts FROM foo WHERE id = 2`, exp: "2010-11-12 13:14:15"},
		{query: `SELECT d FROM foo WHERE id = 2`, exp: "2010-11-12"},
		{query: `SELECT ts FROM audit WHERE id = 2`, exp: "2010-11-12 13:14:15"},
		// A view is evaluated when it is read.
		{query: `SELECT ts FROM foo_now WHERE id = 1`, exp: "2010-11-12 13:14:15"},
	} {
		if got := queryString(t, db, tt.query); got != tt.exp {
			t.Errorf("%s: exp %q, got %q", tt.query, tt.exp, got)
		}
	}
}

// Test_ClockVFS_Unpinned tests that the system clock is used when no time
// is pinned, both before a time is pinned and after it is removed.
func Test_ClockVFS_Unpinned(t *testing.T) {
	vfs := mustRegisterClockVFS(t, "")
	db := mustOpenWithVFS(t, mustTempDB(t), vfs.Name())

	checkSystemClock := func() {
		t.Helper()
		before := time.Now().Unix()
		var got int64
		if err := db.QueryRow(`SELECT unixepoch()`).Scan(&got); err != nil {
			t.Fatalf("failed to query: %s", err)
		}
		after := time.Now().Unix()
		if got < before || got > after {
			t.Fatalf("time is not from the system clock, exp between %d and %d, got %d", before, after, got)
		}
	}

	checkSystemClock()
	vfs.Pin(time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC))
	if exp, got := "2001-02-03 04:05:06", queryString(t, db, `SELECT datetime()`); exp != got {
		t.Fatalf("exp %q, got %q", exp, got)
	}
	vfs.Unpin()
	checkSystemClock()
}

// Test_ClockVFS_OtherConnections tests that a pinned time is seen only by
// connections opened with that VFS, including when those connections are
// to the same database.
func Test_ClockVFS_OtherConnections(t *testing.T) {
	path := mustTempDB(t)
	vfs1 := mustRegisterClockVFS(t, "-1")
	vfs2 := mustRegisterClockVFS(t, "-2")
	db1 := mustOpenWithVFS(t, path, vfs1.Name())
	db2 := mustOpenWithVFS(t, path, vfs2.Name())
	dbDefault := mustOpenWithVFS(t, path, "")

	vfs1.Pin(time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC))
	vfs2.Pin(time.Date(2010, 11, 12, 13, 14, 15, 0, time.UTC))

	if exp, got := "2001-02-03 04:05:06", queryString(t, db1, `SELECT datetime()`); exp != got {
		t.Fatalf("first VFS: exp %q, got %q", exp, got)
	}
	if exp, got := "2010-11-12 13:14:15", queryString(t, db2, `SELECT datetime()`); exp != got {
		t.Fatalf("second VFS: exp %q, got %q", exp, got)
	}
	before := time.Now().Unix()
	var got int64
	if err := dbDefault.QueryRow(`SELECT unixepoch()`).Scan(&got); err != nil {
		t.Fatalf("failed to query: %s", err)
	}
	if got < before {
		t.Fatalf("default VFS is not using the system clock, got %d", got)
	}
}

// Test_ClockVFS_WAL tests that a database in WAL mode, opened with the VFS,
// can be written, checkpointed, and read by a connection which is using the
// default VFS.
func Test_ClockVFS_WAL(t *testing.T) {
	path := mustTempDB(t)
	vfs := mustRegisterClockVFS(t, "")
	db := mustOpenWithVFS(t, path, vfs.Name())
	db.SetMaxOpenConns(1)

	if exp, got := "wal", queryString(t, db, `PRAGMA journal_mode=WAL`); exp != got {
		t.Fatalf("failed to enable WAL mode, exp %q, got %q", exp, got)
	}
	if _, err := db.Exec(`CREATE TABLE foo (id INTEGER PRIMARY KEY, ts TEXT)`); err != nil {
		t.Fatalf("failed to create table: %s", err)
	}

	vfs.Pin(time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC))
	for i := 0; i < 100; i++ {
		if _, err := db.Exec(`INSERT INTO foo (ts) VALUES (datetime())`); err != nil {
			t.Fatalf("failed to insert: %s", err)
		}
	}

	// A reader using the default VFS sees the rows while they are in the WAL.
	reader := mustOpenWithVFS(t, path, "")
	if exp, got := "100", queryString(t, reader, `SELECT COUNT(*) FROM foo WHERE ts = '2001-02-03 04:05:06'`); exp != got {
		t.Fatalf("reader before checkpoint: exp %q rows, got %q", exp, got)
	}

	var busy, log, checkpointed int
	if err := db.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &log, &checkpointed); err != nil {
		t.Fatalf("failed to checkpoint: %s", err)
	}
	if busy != 0 {
		t.Fatalf("checkpoint did not complete")
	}
	if exp, got := "100", queryString(t, reader, `SELECT COUNT(*) FROM foo WHERE ts = '2001-02-03 04:05:06'`); exp != got {
		t.Fatalf("reader after checkpoint: exp %q rows, got %q", exp, got)
	}
	if exp, got := "ok", queryString(t, db, `PRAGMA integrity_check`); exp != got {
		t.Fatalf("integrity check failed: %s", got)
	}
}

// Test_ClockVFS_Register tests registering and unregistering a VFS.
func Test_ClockVFS_Register(t *testing.T) {
	if _, err := RegisterClockVFS(""); err == nil {
		t.Fatalf("expected error registering VFS with no name")
	}

	name := "clock-" + t.Name()
	vfs, err := RegisterClockVFS(name)
	if err != nil {
		t.Fatalf("failed to register VFS: %s", err)
	}
	if exp, got := name, vfs.Name(); exp != got {
		t.Fatalf("wrong name, exp %q, got %q", exp, got)
	}
	if _, err := RegisterClockVFS(name); err == nil {
		t.Fatalf("expected error registering VFS with a name already in use")
	}

	if err := vfs.Unregister(); err != nil {
		t.Fatalf("failed to unregister VFS: %s", err)
	}
	if err := vfs.Unregister(); err != nil {
		t.Fatalf("failed to unregister VFS a second time: %s", err)
	}

	// The name is no longer known to SQLite, and can be used again.
	db, err := sql.Open("sqlite3", "file:"+mustTempDB(t)+"?vfs="+name)
	if err != nil {
		t.Fatalf("failed to open database: %s", err)
	}
	defer db.Close()
	if err := db.Ping(); err == nil {
		t.Fatalf("expected error opening database with an unregistered VFS")
	}
	vfs, err = RegisterClockVFS(name)
	if err != nil {
		t.Fatalf("failed to register VFS again: %s", err)
	}
	if err := vfs.Unregister(); err != nil {
		t.Fatalf("failed to unregister VFS: %s", err)
	}
}
