package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

// ─── Worker pool (0.3 parallelism) ──────────────────────────────────────────

// maxPoolWorkers caps the pool size so a huge explicit worker count cannot
// allocate an unbounded number of 64K-slot stacks.
const maxPoolWorkers = 64

// errCancelled marks a pool run stopped by cancellation rather than a
// callback failure.
var errCancelled = fmt.Errorf("task cancelled")

// numWorkers returns the default pool size: one worker per CPU.
func numWorkers() int64 { return int64(runtime.NumCPU()) }

// poolWorkerCount reads the optional trailing workers argument shared by
// pmap/pfilter/pfor. Without it the pool matches the CPU count.
func poolWorkerCount(objs []Object) (int, Object) {
	if len(objs) == 0 {
		return int(numWorkers()), nil
	}
	n, ok := objs[0].(*Integer)
	if !ok {
		return 0, newError("workers must be an integer, got %s", objs[0].Type())
	}
	if n.Value < 1 {
		return 0, newError("workers must be at least 1, got %d", n.Value)
	}
	w := int(n.Value)
	if w > maxPoolWorkers {
		w = maxPoolWorkers
	}
	return w, nil
}

// callbackArity returns a callback's parameter count. Builtins are variadic
// (ok true, n meaningless) and validate their own arguments at runtime.
func callbackArity(fnObj Object) (n int, ok bool) {
	switch f := fnObj.(type) {
	case *Closure:
		return f.Fn.NumParams, true
	case *CompiledFunction:
		return f.NumParams, true
	case *Builtin:
		return -1, true
	default:
		return 0, false
	}
}

// snapshotCallback returns a per-worker copy of fn with deep-copied free
// variables so two workers never share mutable state. Builtins are stateless
// and pass through unchanged.
func snapshotCallback(fnObj Object) Object {
	if f, ok := fnObj.(*Closure); ok {
		seen := make(map[Object]Object)
		return &Closure{Fn: f.Fn, Free: snapshotFrees(f.Free, seen)}
	}
	return fnObj
}

// newWorkerVM builds a child VM for one pool worker. Like a spawned task it
// runs on a snapshot of the globals, but its main frame is idle — the pool
// drives work through CallFn. abort doubles as the worker's cancel channel,
// so blocking builtins (send/recv/await/sleep) inside callbacks wake when the
// pool stops, and the abort cascade reaches tasks spawned by callbacks too.
func (vm *VM) newWorkerVM(abort chan struct{}) *VM {
	child := &VM{
		constants:    vm.constants,
		stack:        make([]Object, 65536),
		globals:      snapshotGlobals(vm.globals),
		out:          vm.out,
		input:        vm.input,
		args:         vm.args,
		cancel:       abort,
		parentCancel: vm.cancel,
		killOnce:     &sync.Once{},
		sourceMap:    vm.sourceMap,
		source:       vm.source,
		filename:     vm.filename,
		allowIO:      vm.allowIO,
		detector:     vm.detector,
		vmID:         nextVMID(),
		imports:      vm.freshImportState(),
	}
	child.frames[0] = frame{ip: -1}
	child.frameCount = 1
	return child
}

// parallelResult is what a pool run produces: one slot per index, holding
// either the callback's value (map/for mode) or its truthiness (filter mode).
type parallelResult struct {
	values []Object
	keeps  []bool
}

// parallelRun executes fn(get(i)) for every index in [0, n) on a pool of
// worker VMs, preserving index order. The first callback error aborts the
// whole pool; cancellation of this VM (or an ancestor) does too. A single
// worker runs inline in this VM — same semantics without the snapshot cost.
func (vm *VM) parallelRun(n int, get func(int) Object, fnObj Object, workers int, filterMode bool) (*parallelResult, error) {
	if n <= 0 {
		return &parallelResult{values: []Object{}, keeps: []bool{}}, nil
	}
	w := workers
	if w > n {
		w = n
	}
	if w < 1 {
		w = 1
	}

	if w == 1 {
		return vm.parallelRunInline(n, get, fnObj, filterMode)
	}

	res := &parallelResult{}
	if filterMode {
		res.keeps = make([]bool, n)
	} else {
		res.values = make([]Object, n)
	}

	abort := make(chan struct{})
	var closeAbort sync.Once

	var next int64
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup

	// recordErr stores a genuine callback failure. Errors observed while the
	// pool is already aborting or cancelling are just wakeups and are dropped.
	recordErr := func(err error) {
		select {
		case <-abort:
			return
		case <-vm.cancel:
			return
		case <-vm.parentCancel:
			return
		default:
		}
		mu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		mu.Unlock()
		closeAbort.Do(func() { close(abort) })
	}

	for k := 0; k < w; k++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			child := vm.newWorkerVM(abort)
			if vm.detector != nil {
				vm.detector.enter(child.vmID, fmt.Sprintf("parallel worker %d", k+1))
				defer vm.detector.leave(child.vmID)
			}
			cb := snapshotCallback(fnObj)
			for {
				select {
				case <-abort:
					return
				default:
				}
				i := int(atomic.AddInt64(&next, 1)) - 1
				if i >= n {
					// Out of work. If callbacks spawned tasks, surface their
					// failures the same way any other VM would.
					if err := child.waitTasks(); err != nil {
						recordErr(err)
					}
					return
				}
				item := get(i)
				var out Object
				switch fn := cb.(type) {
				case *Builtin:
					out = fn.Fn(child, item)
				default:
					// The worker stack carries nothing between calls.
					child.sp = 0
					r, err := child.CallFn(cb, item)
					if err != nil {
						recordErr(err)
						return
					}
					out = r
				}
				if e, ok := out.(*Error); ok {
					recordErr(fmt.Errorf("%s", e.Message))
					return
				}
				if filterMode {
					res.keeps[i] = isTruthy(out)
				} else {
					res.values[i] = out
				}
			}
		}(k)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	endWait := vm.beginWait("parallel workers")
	select {
	case <-done:
	case <-vm.cancel:
		closeAbort.Do(func() { close(abort) })
		<-done
	case <-vm.parentCancel:
		closeAbort.Do(func() { close(abort) })
		<-done
	}
	endWait()

	mu.Lock()
	err := firstErr
	mu.Unlock()
	if err != nil {
		return nil, err
	}
	select {
	case <-vm.cancel:
		return nil, errCancelled
	case <-vm.parentCancel:
		return nil, errCancelled
	default:
	}
	return res, nil
}

// parallelRunInline is the single-worker path: sequential execution in the
// calling VM, still abortable between elements on cancellation.
func (vm *VM) parallelRunInline(n int, get func(int) Object, fnObj Object, filterMode bool) (*parallelResult, error) {
	res := &parallelResult{}
	if filterMode {
		res.keeps = make([]bool, n)
	} else {
		res.values = make([]Object, n)
	}
	for i := 0; i < n; i++ {
		select {
		case <-vm.cancel:
			return nil, errCancelled
		case <-vm.parentCancel:
			return nil, errCancelled
		default:
		}
		r := callUserFn(vm, fnObj, get(i))
		if e, ok := r.(*Error); ok {
			return nil, fmt.Errorf("%s", e.Message)
		}
		if filterMode {
			res.keeps[i] = isTruthy(r)
		} else {
			res.values[i] = r
		}
	}
	return res, nil
}

// wrapPoolError chains a callback failure to the parallel call site, in the
// same shape await uses for failed tasks: the headline at the call site, the
// worker's full diagnostic below.
func wrapPoolError(op string, err error) Object {
	if err == errCancelled {
		return newFatalError("%s", err.Error())
	}
	if re, ok := err.(*runtimeErr); ok && re.fatal {
		return newFatalError("%s", err.Error())
	}
	return newError("%s failed: %s\n--- inside parallel worker ---\n%s",
		op, DiagnosticHeadline(err.Error()), err.Error())
}
