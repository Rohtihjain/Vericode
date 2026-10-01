package main

import (
	"sync"
	"sync/atomic"
)

// ─── Shared output ────────────────────────────────────────────────────────────

// OutputCollector gathers print output from the main VM and every spawned
// task. The mutex makes concurrent appends safe; line order is scheduling
// order, which is intentionally unspecified.
type OutputCollector struct {
	mu    sync.Mutex
	lines []string
}

func (c *OutputCollector) append(line string) {
	c.mu.Lock()
	c.lines = append(c.lines, line)
	c.mu.Unlock()
}

func (c *OutputCollector) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := ""
	for i, l := range c.lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}

// ─── Snapshot isolation ───────────────────────────────────────────────────────

// snapshotGlobals returns a copy of the globals table where every mutable
// value (array, hash, closure) is deep-copied. Immutable values, channels,
// and task handles are shared by reference — channels are the intended
// cross-task communication path, so sharing them is safe by construction.
func snapshotGlobals(g []Object) []Object {
	out := make([]Object, len(g))
	seen := make(map[Object]Object)
	for i, v := range g {
		if v == nil {
			continue
		}
		out[i] = snapshotObject(v, seen)
	}
	return out
}

// snapshotObject deep-copies mutable containers and shares everything else.
// The seen map guards against reference cycles.
func snapshotObject(o Object, seen map[Object]Object) Object {
	switch v := o.(type) {
	case *Array:
		if prev, ok := seen[o]; ok {
			return prev
		}
		cp := &Array{Elements: make([]Object, len(v.Elements))}
		seen[o] = cp
		for i, e := range v.Elements {
			cp.Elements[i] = snapshotObject(e, seen)
		}
		return cp
	case *Hash:
		if prev, ok := seen[o]; ok {
			return prev
		}
		cp := &Hash{Pairs: make(map[string]HashPairObj, len(v.Pairs))}
		seen[o] = cp
		for k, pair := range v.Pairs {
			cp.Pairs[k] = HashPairObj{
				Key:   snapshotObject(pair.Key, seen),
				Value: snapshotObject(pair.Value, seen),
			}
		}
		return cp
	case *Closure:
		if prev, ok := seen[o]; ok {
			return prev
		}
		cp := &Closure{Fn: v.Fn, Free: make([]Object, len(v.Free))}
		seen[o] = cp
		for i, e := range v.Free {
			cp.Free[i] = snapshotObject(e, seen)
		}
		return cp
	default:
		// Integer, Float, BooleanObj, String, Null, Error are immutable.
		// ChannelObj, TaskObj, Builtin, CompiledFunction are shared by design.
		return o
	}
}

// ─── Spawning ─────────────────────────────────────────────────────────────────

var taskIDCounter int64

func nextTaskID() int64 { return atomic.AddInt64(&taskIDCounter, 1) }

// currentSpawnLoc resolves the source location of the spawn expression being
// executed — the current frame's ip sits on the OpSpawn instruction.
func (vm *VM) currentSpawnLoc() SourceLocation {
	f := vm.currentFrame()
	if f.cl != nil && f.cl.Fn != nil && len(f.cl.Fn.SourceMap) > 0 {
		loc := LookupSourceMap(f.cl.Fn.SourceMap, f.ip)
		if loc.Filename == "" {
			loc.Filename = f.cl.Fn.Filename
		}
		return loc
	}
	loc := LookupSourceMap(vm.sourceMap, f.ip)
	if loc.Filename == "" {
		loc.Filename = vm.filename
	}
	return loc
}

// spawnTask starts fn(args...) on a new goroutine with its own VM state:
// a snapshot of the globals table and a private stack/frame area. The child
// shares the constant pool, the output collector, the input reader, and the
// cancellation channel of its parent. It returns a *TaskObj handle.
func (vm *VM) spawnTask(fnObj Object, args []Object) Object {
	var fn *CompiledFunction
	var free []Object
	switch f := fnObj.(type) {
	case *Closure:
		fn = f.Fn
		free = f.Free
	case *CompiledFunction:
		fn = f
	default:
		return newError("spawn() expects a function, got %s", fnObj.Type())
	}
	if len(args) != fn.NumParams {
		return newError("spawn(): wrong number of arguments: expected %d, got %d", fn.NumParams, len(args))
	}

	seen := make(map[Object]Object)
	snapArgs := make([]Object, len(args))
	for i, a := range args {
		snapArgs[i] = snapshotObject(a, seen)
	}

	name := fn.Name
	if name == "" {
		name = "task"
	}
	// Record where the spawn happened so a failing task can point back here.
	spawnLoc := vm.currentSpawnLoc()
	task := &TaskObj{
		ID:       nextTaskID(),
		Name:     name,
		Done:     make(chan struct{}),
		SpawnLoc: spawnLoc,
	}

	child := &VM{
		constants:  vm.constants,
		stack:      make([]Object, 65536),
		globals:    snapshotGlobals(vm.globals),
		out:          vm.out,
		input:        vm.input,
		cancel:       make(chan struct{}),
		parentCancel: vm.cancel,
		killOnce:     &sync.Once{},
		sourceMap:    vm.sourceMap,
		source:       vm.source,
		filename:     vm.filename,
	}
	child.frames[0] = frame{
		ip:           -1,
		basePointer:  1,
		instructions: fn.Instructions,
		cl:           &Closure{Fn: fn, Free: snapshotFrees(free, seen)},
	}
	child.frameCount = 1
	// Stack layout mirrors OpCall: closure at [0], args at [1..n], then sp
	// reserved past all body locals.
	child.stack[0] = child.frames[0].cl
	for i, a := range snapArgs {
		child.stack[i+1] = a
	}
	child.sp = 1 + fn.NumLocals

	vm.tasks = append(vm.tasks, task)

	go func() {
		defer close(task.Done)
		err := child.runAndWait()
		if err != nil {
			task.Err = err
			return
		}
		result := child.lastPopped
		if result == nil {
			result = &Null{}
		}
		task.Result = result
	}()

	return task
}

func snapshotFrees(free []Object, seen map[Object]Object) []Object {
	if len(free) == 0 {
		return nil
	}
	cp := make([]Object, len(free))
	for i, e := range free {
		cp[i] = snapshotObject(e, seen)
	}
	return cp
}
