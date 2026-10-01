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
	}
	vm.frames[0] = frame{ip: -1, instructions: bytecode.Instructions}
	vm.frameCount = 1
	return vm
}

// runtimeError formats a runtime error with source context when available,
// followed by the call stack when the error happened inside function calls.
func (vm *VM) runtimeError(format string, args ...interface{}) error {
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
		out := FormatDiagnostic(filename, vm.source, "RuntimeError", msg, loc.Line, loc.Col)
		if trace := vm.captureStackTrace(); trace != "" {
			out += "\n" + trace
		}
		return fmt.Errorf("%s", out)
	}
	return fmt.Errorf("%s", msg)
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
		return fmt.Errorf("%s", msg)
	}
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		// Multi-line message: a headline plus a nested diagnostic (a failed
		// task re-raised at await). Show the headline at this call site and
		// append the nested block below it.
		base := vm.runtimeError("%s", msg[:i])
		return fmt.Errorf("%s\n%s", base.Error(), msg[i+1:])
	}
	return vm.runtimeError("%s", msg)
}

func (vm *VM) SetInput(r io.Reader) { vm.input = r }

func (vm *VM) currentFrame() *frame  { return &vm.frames[vm.frameCount-1] }
func (vm *VM) pushFrame(f frame)     { vm.frames[vm.frameCount] = f; vm.frameCount++ }
func (vm *VM) popFrame() *frame      { vm.frameCount--; return &vm.frames[vm.frameCount] }

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
	var childErr error
	for _, t := range vm.tasks {
		select {
		case <-t.Done:
		case <-vm.cancel:
		case <-vm.parentCancel:
		}
		if childErr == nil && t.Err != nil {
			childErr = fmt.Errorf("unawaited task '%s' (spawned at %s) failed\n\n%s",
				t.Name, FormatLoc(t.SpawnLoc), t.Err.Error())
		}
	}
	if err != nil {
		return err
	}
	return childErr
}

// Run executes the program until all frames are exhausted, waits for all
// spawned tasks, and shuts down the cancellation channel.
func (vm *VM) Run() error {
	err := vm.runAndWait()
	vm.kill()
	return err
}

// ─── Main execution loop ──────────────────────────────────────────────────────

// runUntilDepth runs until frameCount drops to targetDepth.
// CallFn uses this to execute a nested compiled function inline.
// Run() is simply runUntilDepth(0).
func (vm *VM) runUntilDepth(targetDepth int) error {
	for vm.frameCount > targetDepth {
		f := vm.currentFrame()
		f.ip++

		if f.ip >= len(f.instructions) {
			// Natural end of frame (no explicit return).
			if vm.frameCount == 1 {
				// Main program finished.
				vm.frameCount--
				break
			}
			pf := vm.popFrame()
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
			vm.sp = pf.basePointer - 1
			if err := vm.push(val); err != nil {
				return err
			}

		case OpReturn:
			pf := vm.popFrame()
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
			return nil, err
		}
		result := vm.lastPopped
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

