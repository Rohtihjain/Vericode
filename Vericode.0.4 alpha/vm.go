package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// ─── Call frame ───────────────────────────────────────────────────────────────

type frame struct {
	ip           int
	basePointer  int
	instructions []byte
	cl           *Closure // non-nil when executing a closure
	handlersBase int      // try handlers active when this frame was entered
}

// catchHandler is a try/catch handler registered by OpTryBegin. target is an
// instruction offset in the frame that registered it; sp is restored before
// the catch body runs so expression temporaries are discarded.
type catchHandler struct {
	frameDepth int
	target     int
	sp         int
}

// ─── VM ───────────────────────────────────────────────────────────────────────

type VM struct {
	constants   []Object
	stack       []Object
	sp          int
	globals     []Object
	out         *OutputCollector
	lastPopped  Object
	frames      [512]frame
	frameCount  int
	input       io.Reader
	sourceMap   []SourceLocation
	source      string
	filename    string
	cancel       chan struct{}   // closed when this VM's subtree is cancelled
	parentCancel <-chan struct{} // fires when an ancestor VM is cancelled (nil for the root VM)
	killOnce     *sync.Once
	tasks        []*TaskObj // tasks spawned by this VM (direct children)
	allowIO      bool       // read_file/write_file; enabled for the CLI only
	detector     *deadlockDetector
	vmID         int64
	handlers     []catchHandler // active try handlers, innermost last
}

func NewVM(bytecode *Bytecode) *VM {
	vm := &VM{
		constants: bytecode.Constants,
		stack:     make([]Object, 65536),
		globals:   make([]Object, 65536),
		out:       &OutputCollector{},
		input:     os.Stdin,
		sourceMap: bytecode.SourceMap,
		source:    bytecode.Source,
		filename:  bytecode.Filename,
		cancel:    make(chan struct{}),
		killOnce:  &sync.Once{},
		detector:  newDeadlockDetector(),
		vmID:      nextVMID(),
	}
	vm.frames[0] = frame{ip: -1, instructions: bytecode.Instructions}
	vm.frameCount = 1
	vm.detector.enter(vm.vmID, "main")
	return vm
}

// runtimeErr is a VeriCode runtime failure. The formatted diagnostic is what
// an uncaught error prints; the structured fields feed the value a catch block
// receives.
type runtimeErr struct {
	formatted string
	msg       string
	kind      string
	loc       SourceLocation
	trace     string
	fatal     bool // cancellation/deadlock: bypasses try/catch
}

func (e *runtimeErr) Error() string { return e.formatted }

// runtimeError formats a runtime error with source context when available,
// followed by the call stack when the error happened inside function calls.
func (vm *VM) runtimeError(format string, args ...interface{}) error {
	return vm.newRuntimeError(false, "RuntimeError", format, args...)
}

// newRuntimeError builds the structured error; fatal failures bypass try/catch.
func (vm *VM) newRuntimeError(fatal bool, kind, format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	f := vm.currentFrame()
	ip := f.ip
	// Prefer closure source map when inside a function
	var loc SourceLocation
	if f.cl != nil && f.cl.Fn != nil && len(f.cl.Fn.SourceMap) > 0 {
		loc = LookupSourceMap(f.cl.Fn.SourceMap, ip)
		if loc.Filename == "" {
			loc.Filename = f.cl.Fn.Filename
		}
	}
	if loc.Line == 0 {
		loc = LookupSourceMap(vm.sourceMap, ip)
	}
	filename := loc.Filename
	if filename == "" {
		filename = vm.filename
	}
	if loc.Line > 0 {
		loc.Filename = filename
		out := FormatDiagnostic(filename, vm.source, kind, msg, loc.Line, loc.Col)
		trace := vm.captureStackTrace()
		if trace != "" {
			out += "\n" + trace
		}
		return &runtimeErr{formatted: out, msg: msg, kind: kind, loc: loc, trace: trace, fatal: fatal}
	}
	return &runtimeErr{formatted: msg, msg: msg, kind: kind, loc: loc, trace: "", fatal: fatal}
}

// captureStackTrace walks the current call frames, innermost first. Each
// frame's location comes from its own function's source map, so every line
// of the trace points at real source. Empty when only the main frame exists.
func (vm *VM) captureStackTrace() string {
	if vm.frameCount <= 1 {
		return ""
	}
	frames := make([]StackFrameInfo, 0, vm.frameCount)
	for i := vm.frameCount - 1; i >= 0; i-- {
		fr := &vm.frames[i]
		var loc SourceLocation
		name := "<main>"
		if fr.cl != nil && fr.cl.Fn != nil {
			name = fr.cl.Fn.Name
			if name == "" {
				name = "<anonymous>"
			}
			if len(fr.cl.Fn.SourceMap) > 0 {
				loc = LookupSourceMap(fr.cl.Fn.SourceMap, fr.ip)
			}
			if loc.Filename == "" {
				loc.Filename = fr.cl.Fn.Filename
			}
		} else {
			loc = LookupSourceMap(vm.sourceMap, fr.ip)
		}
		if loc.Line < 1 {
			continue
		}
		if loc.Filename == "" {
			loc.Filename = vm.filename
		}
		frames = append(frames, StackFrameInfo{FnName: name, Filename: loc.Filename, Line: loc.Line, Col: loc.Col})
	}
	if len(frames) == 0 {
		return ""
	}
	return FormatStackTrace(frames)
}

// builtinError converts a builtin's *Error result into a Go error. Bare
// messages get the call site attached; messages that already are diagnostic
// blocks (errors raised inside a callback, or re-raised from a failed task)
// pass through untouched so their real location and stack stay intact.
func (vm *VM) builtinError(e *Error) error {
	msg := e.Message
	if IsFormattedDiagnostic(msg) {
		// Already a full diagnostic (a callback failure or a re-raised task
		// error): keep its real location and stack intact.
		return &runtimeErr{
			formatted: msg,
			msg:       DiagnosticHeadline(msg),
			kind:      "RuntimeError",
			trace:     diagnosticTail(msg),
			fatal:     e.Fatal,
		}
	}
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		// Multi-line message: a headline plus a nested diagnostic (a failed
		// task re-raised at await). Show the headline at this call site and
		// append the nested block below it.
		base := vm.newRuntimeError(e.Fatal, "RuntimeError", "%s", msg[:i]).(*runtimeErr)
		trace := strings.TrimSpace(base.trace + "\n" + msg[i+1:])
		return &runtimeErr{
			formatted: base.formatted + "\n" + msg[i+1:],
			msg:       base.msg,
			kind:      base.kind,
			loc:       base.loc,
			trace:     trace,
			fatal:     e.Fatal,
		}
	}
	return vm.newRuntimeError(e.Fatal, "RuntimeError", "%s", msg)
}

// diagnosticTail returns everything after a diagnostic's first line.
func diagnosticTail(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[i+1:])
	}
	return ""
}

func (vm *VM) SetInput(r io.Reader) { vm.input = r }

// SetAllowIO enables read_file/write_file. The CLI turns this on; the web
// IDE leaves it off so posted code cannot touch the host filesystem.
func (vm *VM) SetAllowIO(allow bool) { vm.allowIO = allow }

func (vm *VM) currentFrame() *frame  { return &vm.frames[vm.frameCount-1] }
func (vm *VM) pushFrame(f frame) {
	f.handlersBase = len(vm.handlers)
	vm.frames[vm.frameCount] = f
	vm.frameCount++
}
func (vm *VM) popFrame() *frame { vm.frameCount--; return &vm.frames[vm.frameCount] }

// trimHandlers discards handlers registered above a saved base after a frame
// exits (normally or by unwinding).
func (vm *VM) trimHandlers(base int) {
	if base < 0 {
		base = 0
	}
	if base < len(vm.handlers) {
		vm.handlers = vm.handlers[:base]
	}
}

func (vm *VM) push(o Object) error {
	if vm.sp >= len(vm.stack) {
		return vm.runtimeError("stack overflow")
	}
	vm.stack[vm.sp] = o
	vm.sp++
	return nil
}

func (vm *VM) pop() Object {
	if vm.sp == 0 {
		return nil
	}
	o := vm.stack[vm.sp-1]
	vm.sp--
	vm.lastPopped = o
	return o
}

func (vm *VM) LastPoppedStackElem() Object { return vm.lastPopped }
func (vm *VM) Output() string              { return vm.out.String() }

// kill closes this VM's cancellation channel, waking any blocked
// send/recv/await/sleep in its subtree. Cancellation cascades: a woken task
// returns an error, and runAndWait then kills its own subtree.
func (vm *VM) kill() { vm.killOnce.Do(func() { close(vm.cancel) }) }

// runAndWait runs this VM's program, then waits for all tasks it spawned
// (transitively — each child waits for its own children). If the program or
// any unawaited child failed, the first error is returned.
func (vm *VM) runAndWait() error {
	err := vm.runUntilDepth(0)
	if err != nil {
		vm.kill()
	}
	childErr := vm.waitTasks()
	if err != nil {
		return err
	}
	return childErr
}

// waitTasks waits for this VM's spawned tasks and returns the first
// unawaited-task failure. Worker VMs in the parallel pool use it when their
// callbacks spawn tasks.
func (vm *VM) waitTasks() error {
	var childErr error
	if len(vm.tasks) > 0 {
		endWait := vm.beginWait("spawned tasks to finish")
		defer endWait()
	}
	for _, t := range vm.tasks {
		select {
		case <-t.Done:
		case <-vm.cancel:
		case <-vm.parentCancel:
		}
		if childErr == nil && t.Err != nil && !t.wasAwaited() {
			childErr = fmt.Errorf("unawaited task '%s' (spawned at %s) failed\n\n%s",
				t.Name, FormatLoc(t.SpawnLoc), t.Err.Error())
		}
	}
	return childErr
}

// Run executes the program until all frames are exhausted, waits for all
// spawned tasks, and shuts down the cancellation channel.
func (vm *VM) Run() error {
	if vm.detector != nil {
		defer vm.detector.leave(vm.vmID)
	}
	err := vm.runAndWait()
	vm.kill()
	return err
}

// ─── Main execution loop ──────────────────────────────────────────────────────

// runUntilDepth runs until frameCount drops to targetDepth, resuming at a
// try/catch handler when the running code raises a catchable error.
// CallFn uses this to execute a nested compiled function inline.
// Run() is simply runUntilDepth(0).
func (vm *VM) runUntilDepth(targetDepth int) error {
	for {
		err := vm.execute(targetDepth)
		if err == nil {
			return nil
		}
		re, ok := err.(*runtimeErr)
		if !ok || re.fatal {
			return err
		}
		if len(vm.handlers) == 0 {
			return err
		}
		h := vm.handlers[len(vm.handlers)-1]
		if h.frameDepth <= targetDepth {
			// The handler lives in a caller's frame, which this run does not
			// own — propagate so the caller's run loop can catch it.
			return err
		}
		vm.handlers = vm.handlers[:len(vm.handlers)-1]
		vm.frameCount = h.frameDepth
		vm.sp = h.sp
		if err := vm.push(vm.errorValue(re)); err != nil {
			return err
		}
		// The execute loop pre-increments ip, so land on target - 1.
		vm.currentFrame().ip = h.target - 1
	}
}

// errorValue converts a runtime failure into the hash a catch block receives.
func (vm *VM) errorValue(err error) Object {
	re, ok := err.(*runtimeErr)
	if !ok {
		msg := err.Error()
		return newErrorHash(DiagnosticHeadline(msg), "RuntimeError", SourceLocation{}, diagnosticTail(msg))
	}
	return newErrorHash(re.msg, re.kind, re.loc, re.trace)
}

// execute is the raw interpreter loop; runUntilDepth wraps it with try/catch
// unwinding.
func (vm *VM) execute(targetDepth int) error {
	for vm.frameCount > targetDepth {
		f := vm.currentFrame()
		f.ip++

		if f.ip >= len(f.instructions) {
			// Natural end of frame (no explicit return).
			if vm.frameCount == 1 {
				// Main program finished.
				vm.frameCount--
				vm.trimHandlers(0)
				break
			}
			pf := vm.popFrame()
			vm.trimHandlers(pf.handlersBase)
			vm.sp = pf.basePointer - 1
			if err := vm.push(&Null{}); err != nil {
				return err
			}
			// Stop if we have returned to the CallFn target depth.
			if vm.frameCount <= targetDepth {
				break
			}
			continue
		}

		op := Opcode(f.instructions[f.ip])
		ins := f.instructions

		switch op {

		case OpConstant:
			idx := int(binary.BigEndian.Uint16(ins[f.ip+1:]))
			f.ip += 2
			if err := vm.push(vm.constants[idx]); err != nil {
				return err
			}

		case OpTrue:
			if err := vm.push(&BooleanObj{Value: true}); err != nil {
				return err
			}
		case OpFalse:
			if err := vm.push(&BooleanObj{Value: false}); err != nil {
				return err
			}
		case OpNull:
			if err := vm.push(&Null{}); err != nil {
				return err
			}

		case OpPop:
			vm.pop()

		case OpPrint:
			val := vm.pop()
			if val != nil {
				vm.out.append(val.Inspect())
			}

		case OpSetGlobal:
			idx := int(binary.BigEndian.Uint16(ins[f.ip+1:]))
			f.ip += 2
			vm.globals[idx] = vm.pop()

		case OpGetGlobal:
			idx := int(binary.BigEndian.Uint16(ins[f.ip+1:]))
			f.ip += 2
			val := vm.globals[idx]
			if val == nil {
				return vm.runtimeError("undefined variable at global slot %d", idx)
			}
			if err := vm.push(val); err != nil {
				return err
			}

		case OpSetLocal:
			idx := int(binary.BigEndian.Uint16(ins[f.ip+1:]))
			f.ip += 2
			vm.stack[f.basePointer+idx] = vm.pop()

		case OpGetLocal:
			idx := int(binary.BigEndian.Uint16(ins[f.ip+1:]))
			f.ip += 2
			val := vm.stack[f.basePointer+idx]
			if val == nil {
				return vm.runtimeError("undefined local variable at slot %d", idx)
			}
			if err := vm.push(val); err != nil {
				return err
			}

		case OpGetBuiltin:
			idx := int(binary.BigEndian.Uint16(ins[f.ip+1:]))
			f.ip += 2
			if idx >= len(Builtins) {
				return vm.runtimeError("unknown builtin at index %d", idx)
			}
			if err := vm.push(Builtins[idx]); err != nil {
				return err
			}

		case OpAdd, OpSub, OpMul, OpDiv, OpMod:
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeBinaryOp(op, left, right); err != nil {
				return err
			}

		case OpEqual, OpNotEqual, OpGreaterThan, OpLessThan, OpGreaterThanOrEqual, OpLessThanOrEqual:
			right := vm.pop()
			left := vm.pop()
			if err := vm.executeComparison(op, left, right); err != nil {
				return err
			}

		case OpJump:
			target := int(binary.BigEndian.Uint16(ins[f.ip+1:]))
			f.ip = target - 1

		case OpJumpWithFalse:
			target := int(binary.BigEndian.Uint16(ins[f.ip+1:]))
			cond := vm.pop()
			if !isTruthy(cond) {
				f.ip = target - 1
			} else {
				f.ip += 2
			}

		case OpJumpWithTrue:
			target := int(binary.BigEndian.Uint16(ins[f.ip+1:]))
			cond := vm.pop()
			if isTruthy(cond) {
				f.ip = target - 1
			} else {
				f.ip += 2
			}

		case OpArray:
			count := int(binary.BigEndian.Uint16(ins[f.ip+1:]))
			f.ip += 2
			arr := vm.buildArray(vm.sp-count, vm.sp)
			vm.sp -= count
			if err := vm.push(arr); err != nil {
				return err
			}

		case OpHash:
			count := int(binary.BigEndian.Uint16(ins[f.ip+1:]))
			f.ip += 2
			hash, err := vm.buildHash(vm.sp-count, vm.sp)
			if err != nil {
				return err
			}
			vm.sp -= count
			if err := vm.push(hash); err != nil {
				return err
			}

		case OpIndex:
			index := vm.pop()
			left := vm.pop()
			if err := vm.executeIndex(left, index); err != nil {
				return err
			}

		case OpSlice:
			endObj := vm.pop()
			startObj := vm.pop()
			left := vm.pop()
			if err := vm.executeSlice(left, startObj, endObj); err != nil {
				return err
			}

		case OpBang:
			operand := vm.pop()
			if err := vm.push(&BooleanObj{Value: !isTruthy(operand)}); err != nil {
				return err
			}

		case OpNegate:
			operand := vm.pop()
			if operand == nil {
				return vm.runtimeError("cannot negate nil")
			}
			switch v := operand.(type) {
			case *Integer:
				if err := vm.push(&Integer{Value: -v.Value}); err != nil {
					return err
				}
			case *Float:
				if err := vm.push(&Float{Value: -v.Value}); err != nil {
					return err
				}
			default:
				return vm.runtimeError("cannot negate type %s", operand.Type())
			}

		case OpAsk:
			promptObj := vm.pop()
			prompt := ""
			if promptObj != nil && promptObj.Type() == STRING_OBJ {
				prompt = promptObj.(*String).Value
			}
			if prompt != "" {
				fmt.Print(prompt)
			}
			var line string
			if vm.input != nil {
				scanner := bufio.NewScanner(vm.input)
				if scanner.Scan() {
					line = scanner.Text()
				}
			}
			if err := vm.push(&String{Value: line}); err != nil {
				return err
			}

		case OpCall:
			numArgs := int(binary.BigEndian.Uint16(ins[f.ip+1:]))
			f.ip += 2
			fnObj := vm.stack[vm.sp-1-numArgs]

			switch fn := fnObj.(type) {
			case *Closure:
				if numArgs != fn.Fn.NumParams {
					return vm.runtimeError("wrong number of arguments: expected %d, got %d", fn.Fn.NumParams, numArgs)
				}
				bp := vm.sp - numArgs
				vm.pushFrame(frame{
					ip:           -1,
					basePointer:  bp,
					instructions: fn.Fn.Instructions,
					cl:           fn,
				})
				// Reserve slots for all body locals so expression temporaries
				// never clobber them.
				vm.sp = bp + fn.Fn.NumLocals

			case *CompiledFunction:
				// Bare compiled function (no free vars) — wrap as empty closure
				if numArgs != fn.NumParams {
					return vm.runtimeError("wrong number of arguments: expected %d, got %d", fn.NumParams, numArgs)
				}
				bp := vm.sp - numArgs
				vm.pushFrame(frame{
					ip:           -1,
					basePointer:  bp,
					instructions: fn.Instructions,
					cl:           &Closure{Fn: fn},
				})
				vm.sp = bp + fn.NumLocals

			case *Builtin:
				args := make([]Object, numArgs)
				copy(args, vm.stack[vm.sp-numArgs:vm.sp])
				result := fn.Fn(vm, args...)
				vm.sp = vm.sp - numArgs - 1
				if result == nil {
					result = &Null{}
				}
				if errObj, ok := result.(*Error); ok {
					return vm.builtinError(errObj)
				}
				if err := vm.push(result); err != nil {
					return err
				}

			default:
				return vm.runtimeError("cannot call value of type %T", fnObj)
			}

		case OpClosure:
			constIdx := int(binary.BigEndian.Uint16(ins[f.ip+1:]))
			numFree := int(ins[f.ip+3])
			f.ip += 3
			fn, ok := vm.constants[constIdx].(*CompiledFunction)
			if !ok {
				return vm.runtimeError("OpClosure constant is not a function")
			}
			free := make([]Object, numFree)
			for i := 0; i < numFree; i++ {
				free[i] = vm.pop()
			}
			// Frees were pushed in order 0..n-1; stack pop reverses — un-reverse
			for i, j := 0, len(free)-1; i < j; i, j = i+1, j-1 {
				free[i], free[j] = free[j], free[i]
			}
			if err := vm.push(&Closure{Fn: fn, Free: free}); err != nil {
				return err
			}

		case OpGetFree:
			idx := int(binary.BigEndian.Uint16(ins[f.ip+1:]))
			f.ip += 2
			if f.cl == nil || idx >= len(f.cl.Free) {
				return vm.runtimeError("invalid free variable index %d", idx)
			}
			if err := vm.push(f.cl.Free[idx]); err != nil {
				return err
			}

		case OpSetFree:
			idx := int(binary.BigEndian.Uint16(ins[f.ip+1:]))
			f.ip += 2
			if f.cl == nil || idx >= len(f.cl.Free) {
				return vm.runtimeError("invalid free variable index %d", idx)
			}
			f.cl.Free[idx] = vm.pop()

		case OpSetIndex:
			index := vm.pop()
			left := vm.pop()
			value := vm.pop()
			if err := vm.executeSetIndex(left, index, value); err != nil {
				return err
			}

		case OpReturnValue:
			val := vm.pop()
			pf := vm.popFrame()
			vm.trimHandlers(pf.handlersBase)
			vm.sp = pf.basePointer - 1
			if err := vm.push(val); err != nil {
				return err
			}

		case OpReturn:
			pf := vm.popFrame()
			vm.trimHandlers(pf.handlersBase)
			vm.sp = pf.basePointer - 1
			if err := vm.push(&Null{}); err != nil {
				return err
			}

		case OpSpawn:
			numArgs := int(binary.BigEndian.Uint16(ins[f.ip+1:]))
			f.ip += 2
			args := make([]Object, numArgs)
			copy(args, vm.stack[vm.sp-numArgs:vm.sp])
			fnObj := vm.stack[vm.sp-numArgs-1]
			vm.sp = vm.sp - numArgs - 1
			task := vm.spawnTask(fnObj, args)
			if err, ok := task.(*Error); ok {
				return vm.builtinError(err)
			}
			if err := vm.push(task); err != nil {
				return err
			}

		case OpTryBegin:
			target := int(binary.BigEndian.Uint16(ins[f.ip+1:]))
			f.ip += 2
			vm.handlers = append(vm.handlers, catchHandler{
				frameDepth: vm.frameCount,
				target:     target,
				sp:         vm.sp,
			})

		case OpTryEnd:
			if len(vm.handlers) > 0 {
				vm.handlers = vm.handlers[:len(vm.handlers)-1]
			}

		case OpTryPop:
			if len(vm.handlers) > 0 {
				vm.handlers = vm.handlers[:len(vm.handlers)-1]
			}

		case OpThrow:
			val := vm.pop()
			switch v := val.(type) {
			case *String:
				return vm.runtimeError("%s", v.Value)
			case nil:
				return vm.runtimeError("null")
			default:
				return vm.runtimeError("%s", v.Inspect())
			}
		}
	}
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func isTruthy(obj Object) bool {
	if obj == nil {
		return false
	}
	switch o := obj.(type) {
	case *BooleanObj:
		return o.Value
	case *Integer:
		return o.Value != 0
	case *Float:
		return o.Value != 0
	case *String:
		return o.Value != ""
	case *Null:
		return false
	default:
		return true
	}
}

func (vm *VM) buildArray(start, end int) Object {
	elems := make([]Object, end-start)
	copy(elems, vm.stack[start:end])
	return &Array{Elements: elems}
}
func (vm *VM) buildHash(start, end int) (Object, error) {
	hashedPairs := make(map[string]HashPairObj)
	for i := start; i < end; i += 2 {
		key := vm.stack[i]
		value := vm.stack[i+1]

		hashKey, ok := key.(Hashable)
		if !ok {
			return nil, vm.runtimeError("unusable as hash key: %s", key.Type())
		}
		hashedPairs[hashKey.HashKey()] = HashPairObj{Key: key, Value: value}
	}
	return &Hash{Pairs: hashedPairs}, nil
}

func (vm *VM) executeIndex(left, index Object) error {
	switch {
	case left.Type() == ARRAY_OBJ && index.Type() == INTEGER_OBJ:
		arr := left.(*Array)
		idx := index.(*Integer).Value
		if idx < 0 || idx >= int64(len(arr.Elements)) {
			return vm.push(&Null{})
		}
		return vm.push(arr.Elements[idx])

	case left.Type() == HASH_OBJ:
		hash := left.(*Hash)
		key, ok := index.(Hashable)
		if !ok {
			return vm.runtimeError("unusable as hash key: %s", index.Type())
		}
		pair, ok := hash.Pairs[key.HashKey()]
		if !ok {
			return vm.push(&Null{})
		}
		return vm.push(pair.Value)

	case left.Type() == STRING_OBJ && index.Type() == INTEGER_OBJ:
		str := left.(*String).Value
		idx := index.(*Integer).Value
		if idx < 0 || idx >= int64(len(str)) {
			return vm.push(&Null{})
		}
		return vm.push(&String{Value: string(str[idx])})

	case left.Type() == TABLE_OBJ && index.Type() == INTEGER_OBJ:
		table := left.(*Table)
		idx := index.(*Integer).Value
		if idx < 0 || idx >= int64(len(table.Rows)) {
			return vm.push(&Null{})
		}
		return vm.push(table.Rows[idx])

	case left.Type() == TABLE_OBJ && index.Type() == STRING_OBJ:
		table := left.(*Table)
		name := index.(*String).Value
		if table.columnIndex(name) < 0 {
			return vm.push(&Null{})
		}
		return vm.push(&Array{Elements: table.column(name)})

	default:
		return vm.runtimeError("index operator not supported for %s[%s]", left.Type(), index.Type())
	}
}

func (vm *VM) executeSlice(left, startObj, endObj Object) error {
	var start, end int64
	hasStart := startObj != nil && startObj.Type() != NULL_OBJ
	hasEnd := endObj != nil && endObj.Type() != NULL_OBJ

	if hasStart {
		si, ok := startObj.(*Integer)
		if !ok {
			return vm.runtimeError("slice start must be an integer, got %s", startObj.Type())
		}
		start = si.Value
	}
	if hasEnd {
		ei, ok := endObj.(*Integer)
		if !ok {
			return vm.runtimeError("slice end must be an integer, got %s", endObj.Type())
		}
		end = ei.Value
	}

	switch left.Type() {
	case ARRAY_OBJ:
		arr := left.(*Array)
		n := int64(len(arr.Elements))
		if !hasStart {
			start = 0
		}
		if !hasEnd {
			end = n
		}
		// clamp like Python
		if start < 0 {
			start = 0
		}
		if end > n {
			end = n
		}
		if start > end {
			start = end
		}
		elems := make([]Object, end-start)
		copy(elems, arr.Elements[start:end])
		return vm.push(&Array{Elements: elems})

	case STRING_OBJ:
		str := left.(*String).Value
		// work on runes for correct char boundaries
		runes := []rune(str)
		n := int64(len(runes))
		if !hasStart {
			start = 0
		}
		if !hasEnd {
			end = n
		}
		if start < 0 {
			start = 0
		}
		if end > n {
			end = n
		}
		if start > end {
			start = end
		}
		return vm.push(&String{Value: string(runes[start:end])})

	default:
		return vm.runtimeError("slice operator not supported for %s", left.Type())
	}
}

func asFloat(o Object) (float64, bool) {

	switch v := o.(type) {
	case *Integer:
		return float64(v.Value), true
	case *Float:
		return v.Value, true
	default:
		return 0, false
	}
}

func (vm *VM) executeBinaryOp(op Opcode, left, right Object) error {
	if left == nil || right == nil {
		return vm.runtimeError("nil operand in binary operation")
	}

	// String concatenation
	if op == OpAdd && (left.Type() == STRING_OBJ || right.Type() == STRING_OBJ) {
		return vm.push(&String{Value: left.Inspect() + right.Inspect()})
	}

	// String repetition: "abc" * 3
	if left.Type() == STRING_OBJ && right.Type() == INTEGER_OBJ && op == OpMul {
		str := left.(*String).Value
		count := right.(*Integer).Value
		return vm.push(&String{Value: strings.Repeat(str, int(count))})
	}

	// Pure integer arithmetic (keeps INTEGER result type)
	lInt, lok := left.(*Integer)
	rInt, rok := right.(*Integer)
	if lok && rok {
		var res int64
		switch op {
		case OpAdd:
			res = lInt.Value + rInt.Value
		case OpSub:
			res = lInt.Value - rInt.Value
		case OpMul:
			res = lInt.Value * rInt.Value
		case OpDiv:
			if rInt.Value == 0 {
				return vm.runtimeError("division by zero")
			}
			res = lInt.Value / rInt.Value
		case OpMod:
			if rInt.Value == 0 {
				return vm.runtimeError("modulo by zero")
			}
			res = lInt.Value % rInt.Value
		}
		return vm.push(&Integer{Value: res})
	}

	// Float (or mixed int/float) arithmetic
	lf, lokf := asFloat(left)
	rf, rokf := asFloat(right)
	if lokf && rokf {
		var res float64
		switch op {
		case OpAdd:
			res = lf + rf
		case OpSub:
			res = lf - rf
		case OpMul:
			res = lf * rf
		case OpDiv:
			if rf == 0 {
				return vm.runtimeError("division by zero")
			}
			res = lf / rf
		case OpMod:
			return vm.runtimeError("modulo not supported for floats")
		}
		return vm.push(&Float{Value: res})
	}

	return vm.runtimeError("unsupported operation: %s %v %s", left.Type(), op, right.Type())
}

func (vm *VM) executeSetIndex(left, index, value Object) error {
	switch {
	case left.Type() == ARRAY_OBJ && index.Type() == INTEGER_OBJ:
		arr := left.(*Array)
		idx := index.(*Integer).Value
		if idx < 0 || idx >= int64(len(arr.Elements)) {
			return vm.runtimeError("array index out of bounds: %d", idx)
		}
		arr.Elements[idx] = value
		return nil
	case left.Type() == HASH_OBJ:
		hash := left.(*Hash)
		key, ok := index.(Hashable)
		if !ok {
			return vm.runtimeError("unusable as hash key: %s", index.Type())
		}
		hash.Pairs[key.HashKey()] = HashPairObj{Key: index, Value: value}
		return nil
	default:
		return vm.runtimeError("cannot assign index on %s", left.Type())
	}
}

func (vm *VM) executeComparison(op Opcode, left, right Object) error {
	if left == nil || right == nil {
		return vm.push(&BooleanObj{Value: false})
	}
	// null == null is true; null equals nothing else
	if left.Type() == NULL_OBJ || right.Type() == NULL_OBJ {
		eq := left.Type() == NULL_OBJ && right.Type() == NULL_OBJ
		switch op {
		case OpEqual:
			return vm.push(&BooleanObj{Value: eq})
		case OpNotEqual:
			return vm.push(&BooleanObj{Value: !eq})
		default:
			return vm.push(&BooleanObj{Value: false})
		}
	}

	// Numeric comparisons (int and/or float)
	lf, lok := asFloat(left)
	rf, rok := asFloat(right)
	if lok && rok {
		switch op {
		case OpEqual:
			return vm.push(&BooleanObj{Value: lf == rf})
		case OpNotEqual:
			return vm.push(&BooleanObj{Value: lf != rf})
		case OpGreaterThan:
			return vm.push(&BooleanObj{Value: lf > rf})
		case OpLessThan:
			return vm.push(&BooleanObj{Value: lf < rf})
		case OpGreaterThanOrEqual:
			return vm.push(&BooleanObj{Value: lf >= rf})
		case OpLessThanOrEqual:
			return vm.push(&BooleanObj{Value: lf <= rf})
		}
	}

	lInt, lIsInt := left.(*Integer)
	rInt, rIsInt := right.(*Integer)
	if lIsInt && rIsInt {
		switch op {
		case OpEqual:
			return vm.push(&BooleanObj{Value: lInt.Value == rInt.Value})
		case OpNotEqual:
			return vm.push(&BooleanObj{Value: lInt.Value != rInt.Value})
		case OpGreaterThan:
			return vm.push(&BooleanObj{Value: lInt.Value > rInt.Value})
		case OpLessThan:
			return vm.push(&BooleanObj{Value: lInt.Value < rInt.Value})
		case OpGreaterThanOrEqual:
			return vm.push(&BooleanObj{Value: lInt.Value >= rInt.Value})
		case OpLessThanOrEqual:
			return vm.push(&BooleanObj{Value: lInt.Value <= rInt.Value})
		}
	}

	lStr, lIsStr := left.(*String)
	rStr, rIsStr := right.(*String)
	if lIsStr && rIsStr {
		switch op {
		case OpEqual:
			return vm.push(&BooleanObj{Value: lStr.Value == rStr.Value})
		case OpNotEqual:
			return vm.push(&BooleanObj{Value: lStr.Value != rStr.Value})
		}
	}

	lBool, lIsBool := left.(*BooleanObj)
	rBool, rIsBool := right.(*BooleanObj)
	if lIsBool && rIsBool {
		switch op {
		case OpEqual:
			return vm.push(&BooleanObj{Value: lBool.Value == rBool.Value})
		case OpNotEqual:
			return vm.push(&BooleanObj{Value: lBool.Value != rBool.Value})
		}
	}

	return vm.push(&BooleanObj{Value: false})
}

// CallFn calls a function object (CompiledFunction or Builtin) with the given
// args, sharing this VM's constants and globals. Used by map/filter/reduce.
func (vm *VM) CallFn(fnObj Object, args ...Object) (Object, error) {
	switch fn := fnObj.(type) {
	case *Builtin:
		result := fn.Fn(vm, args...)
		if result == nil {
			return &Null{}, nil
		}
		return result, nil

	case *Closure:
		if len(args) != fn.Fn.NumParams {
			return nil, vm.runtimeError("wrong number of arguments: expected %d, got %d", fn.Fn.NumParams, len(args))
		}
		savedSP := vm.sp
		if err := vm.push(fn); err != nil {
			return nil, err
		}
		for _, arg := range args {
			if err := vm.push(arg); err != nil {
				return nil, err
			}
		}
		startDepth := vm.frameCount
		handlerBase := len(vm.handlers)
		bp := vm.sp - len(args)
		vm.pushFrame(frame{
			ip:           -1,
			basePointer:  bp,
			instructions: fn.Fn.Instructions,
			cl:           fn,
		})
		vm.sp = bp + fn.Fn.NumLocals
		if err := vm.runUntilDepth(startDepth); err != nil {
			vm.sp = savedSP
			// The failed call may have left frames pushed; pop them so the
			// VM (and any error reporting afterwards) is back to this depth.
			vm.frameCount = startDepth
			vm.trimHandlers(handlerBase)
			return nil, err
		}
		// The return value (or implicit null) sits exactly at savedSP. Read
		// it from the stack instead of lastPopped — lastPopped is not updated
		// by implicit returns, and dropping the slot keeps repeated CallFn
		// calls (map, pmap) from leaking one stack slot per call.
		result := vm.stack[vm.sp-1]
		vm.sp = savedSP
		if result == nil {
			result = &Null{}
		}
		return result, nil

	case *CompiledFunction:
		return vm.CallFn(&Closure{Fn: fn}, args...)

	default:
		return nil, vm.runtimeError("cannot call value of type %T", fnObj)
	}
}

