package hooks

import (
	"sync"
)

// asyncJob is one deferred hook invocation under --hook-mode async.
type asyncJob struct {
	hook   Hook
	oldRow map[string]any
	newRow map[string]any
}

var (
	asyncOnce sync.Once
	asyncCh   chan asyncJob
	asyncWG   sync.WaitGroup
)

const asyncWorkers = 2

// startAsyncWorkers boots the buffered channel + worker pool that consumes
// hook invocations after the triggering statement has returned. Async hooks
// can never abort a write and their errors only ever reach the execution
// log.
func startAsyncWorkers() {
	asyncOnce.Do(func() {
		asyncCh = make(chan asyncJob, 256)
		for i := 0; i < asyncWorkers; i++ {
			go func() {
				for job := range asyncCh {
					c := Current()
					Run(job.hook, job.oldRow, job.newRow, RunConfig{
						DB: DB(), Write: c.Write, AllowNet: c.AllowNet, Record: true,
					})
					asyncWG.Done()
				}
			}()
		}
	})
}

// enqueueAsync hands a hook invocation to the worker pool. Returns false if
// the queue is full (the run is then dropped rather than blocking the
// triggering statement, which is the whole point of async mode).
func enqueueAsync(h Hook, oldRow, newRow map[string]any) bool {
	if asyncCh == nil {
		startAsyncWorkers()
	}
	asyncWG.Add(1)
	select {
	case asyncCh <- asyncJob{hook: h, oldRow: oldRow, newRow: newRow}:
		return true
	default:
		asyncWG.Done()
		return false
	}
}
