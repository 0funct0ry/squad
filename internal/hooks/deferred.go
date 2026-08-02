package hooks

import (
	"database/sql"
	"strings"
	"sync"
	"time"
)

// deferredWrite is a statement that couldn't run inline because the
// triggering statement still held SQLite's write lock (see the package doc's
// reentrancy note).
type deferredWrite struct {
	sql  string
	args []any
}

var (
	deferredOnce sync.Once
	deferredCh   chan deferredWrite
	deferredWG   sync.WaitGroup
)

// startDeferredWriter spins up the single retrying writer goroutine. It is
// started once per process; the database it writes to is read from DB() at
// execution time so `.open`-style reconnects are picked up.
func startDeferredWriter(d *sql.DB) {
	_ = d
	deferredOnce.Do(func() {
		deferredCh = make(chan deferredWrite, 512)
		go func() {
			for w := range deferredCh {
				runDeferred(w)
				deferredWG.Done()
			}
		}()
	})
}

func runDeferred(w deferredWrite) {
	cur := DB()
	if cur == nil {
		return
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		_, err := cur.Exec(w.sql, w.args...)
		if err == nil || !isBusy(err) || time.Now().After(deadline) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func isBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") || strings.Contains(msg, "database table is locked") || strings.Contains(msg, "busy")
}

// execOrDefer runs a write inline and, if SQLite reports the database as
// locked, hands it to the deferred writer instead. Returns the inline error
// when the statement failed for any other reason, and (deferred=true, nil)
// when the statement was queued.
//
// inTrigger short-circuits the inline attempt entirely: inside a trigger
// callback the triggering statement provably still holds the write lock, and
// connections carry a busy_timeout, so an inline attempt would block for the
// full timeout before failing. Skipping straight to the queue is both
// correct and instant.
func execOrDefer(d *sql.DB, inTrigger bool, query string, args ...any) (deferred bool, err error) {
	if d == nil {
		return false, nil
	}
	if !inTrigger {
		if _, err := d.Exec(query, args...); err == nil {
			return false, nil
		} else if !isBusy(err) {
			return false, err
		}
	}
	if deferredCh == nil {
		startDeferredWriter(d)
	}
	deferredWG.Add(1)
	select {
	case deferredCh <- deferredWrite{sql: query, args: args}:
		return true, nil
	default:
		deferredWG.Done()
		return false, nil
	}
}

// Drain blocks until every deferred write has been attempted. Used by tests
// and by the CLI before exiting.
func Drain() {
	if deferredCh == nil {
		return
	}
	deferredWG.Wait()
	asyncWG.Wait()
	deferredWG.Wait()
}
