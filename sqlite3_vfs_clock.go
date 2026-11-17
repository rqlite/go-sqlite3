// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

//go:build cgo
// +build cgo

package sqlite3

/*
#ifndef USE_LIBSQLITE3
#include "sqlite3-binding.h"
#else
#include <sqlite3.h>
#endif
#include <stdlib.h>
#include <string.h>

// clockVfs is a VFS which passes every call through to another VFS, with
// one exception: when a time has been pinned, it reports that time as the
// current time. Files are opened by the underlying VFS and so are operated
// on by it directly.
typedef struct clockVfs {
  sqlite3_vfs base;      // Must be first, SQLite is given a pointer to it.
  sqlite3_vfs *orig;     // The VFS which does the real work.
  sqlite3_int64 pinned;  // Julian day number in milliseconds, 0 if not pinned.
} clockVfs;

#define ORIG(p) (((clockVfs*)(p))->orig)

static int clockOpen(sqlite3_vfs *p, sqlite3_filename zName, sqlite3_file *pFile, int flags, int *pOutFlags) {
  return ORIG(p)->xOpen(ORIG(p), zName, pFile, flags, pOutFlags);
}
static int clockDelete(sqlite3_vfs *p, const char *zName, int syncDir) {
  return ORIG(p)->xDelete(ORIG(p), zName, syncDir);
}
static int clockAccess(sqlite3_vfs *p, const char *zName, int flags, int *pResOut) {
  return ORIG(p)->xAccess(ORIG(p), zName, flags, pResOut);
}
static int clockFullPathname(sqlite3_vfs *p, const char *zName, int nOut, char *zOut) {
  return ORIG(p)->xFullPathname(ORIG(p), zName, nOut, zOut);
}
static void *clockDlOpen(sqlite3_vfs *p, const char *zFilename) {
  return ORIG(p)->xDlOpen(ORIG(p), zFilename);
}
static void clockDlError(sqlite3_vfs *p, int nByte, char *zErrMsg) {
  ORIG(p)->xDlError(ORIG(p), nByte, zErrMsg);
}
static void (*clockDlSym(sqlite3_vfs *p, void *pHandle, const char *zSymbol))(void) {
  return ORIG(p)->xDlSym(ORIG(p), pHandle, zSymbol);
}
static void clockDlClose(sqlite3_vfs *p, void *pHandle) {
  ORIG(p)->xDlClose(ORIG(p), pHandle);
}
static int clockRandomness(sqlite3_vfs *p, int nByte, char *zOut) {
  return ORIG(p)->xRandomness(ORIG(p), nByte, zOut);
}
static int clockSleep(sqlite3_vfs *p, int microseconds) {
  return ORIG(p)->xSleep(ORIG(p), microseconds);
}
static int clockGetLastError(sqlite3_vfs *p, int n, char *zOut) {
  return ORIG(p)->xGetLastError(ORIG(p), n, zOut);
}
static int clockSetSystemCall(sqlite3_vfs *p, const char *zName, sqlite3_syscall_ptr pCall) {
  return ORIG(p)->xSetSystemCall(ORIG(p), zName, pCall);
}
static sqlite3_syscall_ptr clockGetSystemCall(sqlite3_vfs *p, const char *zName) {
  return ORIG(p)->xGetSystemCall(ORIG(p), zName);
}
static const char *clockNextSystemCall(sqlite3_vfs *p, const char *zName) {
  return ORIG(p)->xNextSystemCall(ORIG(p), zName);
}

static int clockCurrentTimeInt64(sqlite3_vfs *p, sqlite3_int64 *piNow) {
  sqlite3_int64 pinned = __atomic_load_n(&((clockVfs*)p)->pinned, __ATOMIC_SEQ_CST);
  if (pinned != 0) {
    *piNow = pinned;
    return SQLITE_OK;
  }
  if (ORIG(p)->iVersion >= 2 && ORIG(p)->xCurrentTimeInt64) {
    return ORIG(p)->xCurrentTimeInt64(ORIG(p), piNow);
  }
  double r;
  int rc = ORIG(p)->xCurrentTime(ORIG(p), &r);
  *piNow = (sqlite3_int64)(r * 86400000.0);
  return rc;
}
static int clockCurrentTime(sqlite3_vfs *p, double *prNow) {
  sqlite3_int64 i = 0;
  int rc = clockCurrentTimeInt64(p, &i);
  *prNow = i / 86400000.0;
  return rc;
}

// clockVfsRegister registers a new VFS under the given name, which must
// remain valid until the VFS is unregistered. It wraps the default VFS.
static int clockVfsRegister(const char *zName, clockVfs **ppOut) {
  sqlite3_vfs *orig = sqlite3_vfs_find(0);
  if (orig == 0) {
    return SQLITE_ERROR;
  }
  if (sqlite3_vfs_find(zName) != 0) {
    return SQLITE_MISUSE;
  }
  clockVfs *v = (clockVfs*)malloc(sizeof(clockVfs));
  if (v == 0) {
    return SQLITE_NOMEM;
  }
  memset(v, 0, sizeof(clockVfs));
  v->orig = orig;
  v->base.iVersion = orig->iVersion > 3 ? 3 : orig->iVersion;
  v->base.szOsFile = orig->szOsFile;
  v->base.mxPathname = orig->mxPathname;
  v->base.zName = zName;
  v->base.xOpen = clockOpen;
  v->base.xDelete = clockDelete;
  v->base.xAccess = clockAccess;
  v->base.xFullPathname = clockFullPathname;
  v->base.xDlOpen = clockDlOpen;
  v->base.xDlError = clockDlError;
  v->base.xDlSym = clockDlSym;
  v->base.xDlClose = clockDlClose;
  v->base.xRandomness = clockRandomness;
  v->base.xSleep = clockSleep;
  v->base.xCurrentTime = clockCurrentTime;
  v->base.xGetLastError = clockGetLastError;
  // SQLite only uses the following if iVersion says they are present, so
  // they are always set here. The time is the exception, as it is always
  // available from this VFS whatever the underlying VFS offers.
  v->base.xCurrentTimeInt64 = clockCurrentTimeInt64;
  if (v->base.iVersion < 2) {
    v->base.iVersion = 2;
  }
  v->base.xSetSystemCall = clockSetSystemCall;
  v->base.xGetSystemCall = clockGetSystemCall;
  v->base.xNextSystemCall = clockNextSystemCall;

  int rc = sqlite3_vfs_register(&v->base, 0);
  if (rc != SQLITE_OK) {
    free(v);
    return rc;
  }
  *ppOut = v;
  return SQLITE_OK;
}

static int clockVfsUnregister(clockVfs *v) {
  int rc = sqlite3_vfs_unregister(&v->base);
  if (rc == SQLITE_OK) {
    free(v);
  }
  return rc;
}

static void clockVfsPin(clockVfs *v, sqlite3_int64 julianMs) {
  __atomic_store_n(&v->pinned, julianMs, __ATOMIC_SEQ_CST);
}
*/
import "C"
import (
	"errors"
	"fmt"
	"time"
	"unsafe"
)

// julianEpochMs is the Julian day number of the Unix epoch, in milliseconds.
const julianEpochMs = 210866760000000

// ClockVFS is a SQLite VFS which can be told what the current time is.
//
// It passes every operation through to the default VFS, so a database
// opened with it behaves exactly as it would otherwise. The exception is
// the current time. While a time is pinned, SQLite sees that time as the
// current time on every connection opened with the VFS. That is the time
// returned by date('now'), by the date and time functions when called with
// no time value, and by CURRENT_TIMESTAMP, CURRENT_DATE and CURRENT_TIME,
// wherever they are evaluated, including column defaults and triggers.
//
// To use it, open a connection with the name of the VFS in the DSN, for
// example "file:foo.db?vfs=name".
type ClockVFS struct {
	name  string
	cname *C.char
	vfs   *C.clockVfs
}

// RegisterClockVFS registers a new ClockVFS with SQLite under the given
// name, which must not be the name of any other VFS.
func RegisterClockVFS(name string) (*ClockVFS, error) {
	if name == "" {
		return nil, errors.New("VFS name is empty")
	}
	if rc := C.sqlite3_initialize(); rc != C.SQLITE_OK {
		return nil, Error{Code: ErrNo(rc)}
	}
	cname := C.CString(name)
	var vfs *C.clockVfs
	if rc := C.clockVfsRegister(cname, &vfs); rc != C.SQLITE_OK {
		C.free(unsafe.Pointer(cname))
		if rc == C.SQLITE_MISUSE {
			return nil, fmt.Errorf("VFS %q is already registered", name)
		}
		return nil, Error{Code: ErrNo(rc)}
	}
	return &ClockVFS{name: name, cname: cname, vfs: vfs}, nil
}

// Name returns the name the VFS is registered under.
func (v *ClockVFS) Name() string {
	return v.name
}

// Pin sets the time SQLite sees as the current time, to millisecond
// precision. It remains in place until Unpin is called. It is safe to call
// while connections are open, but a statement which is already running may
// continue to see the time as it was when the statement first asked for it.
func (v *ClockVFS) Pin(t time.Time) {
	C.clockVfsPin(v.vfs, C.sqlite3_int64(t.UnixMilli()+julianEpochMs))
}

// Unpin returns SQLite to using the system clock.
func (v *ClockVFS) Unpin() {
	C.clockVfsPin(v.vfs, 0)
}

// Unregister removes the VFS from SQLite. Every connection opened with the
// VFS must be closed first, and the VFS must not be used afterwards.
func (v *ClockVFS) Unregister() error {
	if v.vfs == nil {
		return nil
	}
	if rc := C.clockVfsUnregister(v.vfs); rc != C.SQLITE_OK {
		return Error{Code: ErrNo(rc)}
	}
	C.free(unsafe.Pointer(v.cname))
	v.vfs = nil
	v.cname = nil
	return nil
}
