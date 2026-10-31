package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// deadlockGrace is how long the detector waits after the last runnable VM
// blocks before declaring a deadlock. It absorbs the window in which a VM has
// been handed a value by a channel partner but has not resumed yet.
const deadlockGrace = 50 * time.Millisecond

var vmIDCounter int64

func nextVMID() int64 { return atomic.AddInt64(&vmIDCounter, 1) }

// deadlockDetector tracks the VMs of one program run: how many can still make
// progress, which ones are blocked, and whether any sleep timer is pending.
// A deadlock is declared when every live VM is blocked in an untimed wait
// (send / recv / await / pool join) and no timeout can wake anyone.
type deadlockDetector struct {
	mu       sync.Mutex
	live     int
	runnable int
	timers   int
	pending  bool
	blocked  map[int64]string // vm id -> what it is waiting on
	names    map[int64]string // vm id -> who it is, for the report
	ch       chan struct{}
	once     sync.Once
	msg      string
}

func newDeadlockDetector() *deadlockDetector {
	return &deadlockDetector{
		blocked: make(map[int64]string),
		names:   make(map[int64]string),
		ch:      make(chan struct{}),
	}
}

func (d *deadlockDetector) enter(id int64, name string) {
	d.mu.Lock()
	d.live++
	d.runnable++
	d.names[id] = name
	d.mu.Unlock()
}

func (d *deadlockDetector) leave(id int64) {
	d.mu.Lock()
	d.live--
	if _, wasBlocked := d.blocked[id]; wasBlocked {
		delete(d.blocked, id)
	} else {
		d.runnable--
	}
	delete(d.names, id)
	d.evaluateLocked()
	d.mu.Unlock()
}

func (d *deadlockDetector) block(id int64, what string) {
	d.mu.Lock()
	if _, already := d.blocked[id]; !already {
		d.blocked[id] = what
		d.runnable--
	}
	d.evaluateLocked()
	d.mu.Unlock()
}

func (d *deadlockDetector) unblock(id int64) {
	d.mu.Lock()
	if _, ok := d.blocked[id]; ok {
		delete(d.blocked, id)
		d.runnable++
	}
	d.mu.Unlock()
}

func (d *deadlockDetector) timerStart() {
	d.mu.Lock()
	d.timers++
	d.mu.Unlock()
}

func (d *deadlockDetector) timerEnd() {
	d.mu.Lock()
	d.timers--
	d.mu.Unlock()
}

// evaluateLocked schedules a settle check when nothing can make progress.
// Running it after a grace period keeps a just-woken VM from being mistaken
// for a deadlocked one.
func (d *deadlockDetector) evaluateLocked() {
	if d.pending || d.runnable > 0 || d.timers > 0 || d.live == 0 || len(d.blocked) == 0 {
		return
	}
	d.pending = true
	time.AfterFunc(deadlockGrace, d.settle)
}

func (d *deadlockDetector) settle() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pending = false
	if d.runnable > 0 || d.timers > 0 || d.live == 0 || len(d.blocked) == 0 {
		return
	}
	d.once.Do(func() {
		d.msg = d.snapshotLocked()
		close(d.ch)
	})
}

func (d *deadlockDetector) snapshotLocked() string {
	ids := make([]int64, 0, len(d.blocked))
	for id := range d.blocked {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var b strings.Builder
	b.WriteString("deadlock: every task is blocked and no timers are pending")
	for _, id := range ids {
		name := d.names[id]
		if name == "" {
			name = "task"
		}
		fmt.Fprintf(&b, "\n  %s: waiting on %s", name, d.blocked[id])
	}
	b.WriteString("\nhint: some task must be receiving, sending, or awaiting to make progress")
	return b.String()
}

func (d *deadlockDetector) message() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.msg == "" {
		return "deadlock: every task is blocked"
	}
	return d.msg
}

// ─── VM helpers ───────────────────────────────────────────────────────────────

func (vm *VM) deadlockChan() <-chan struct{} {
	if vm.detector == nil {
		return nil
	}
	return vm.detector.ch
}

func (vm *VM) deadlockErr() Object {
	if vm.detector == nil {
		return newError("deadlock")
	}
	return newError("%s", vm.detector.message())
}

// beginWait marks this VM as blocked and returns the matching unblock.
func (vm *VM) beginWait(what string) func() {
	if vm.detector == nil {
		return func() {}
	}
	vm.detector.block(vm.vmID, what)
	return func() { vm.detector.unblock(vm.vmID) }
}

// beginTimer registers a pending timeout so sleepers never look deadlocked.
func (vm *VM) beginTimer() func() {
	if vm.detector == nil {
		return func() {}
	}
	vm.detector.timerStart()
	return func() { vm.detector.timerEnd() }
}
