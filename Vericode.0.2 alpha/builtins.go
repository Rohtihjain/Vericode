package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Builtins is the ordered list of built-in functions.
// The index here matches the OpGetBuiltin operand.
// map, filter, and reduce are appended in init() to avoid an init cycle
// (they reference callCompiledFn which transitively references Builtins via vm.Run).
var Builtins = []*Builtin{
	{
		Name: "len",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 1 {
				return newError("len() takes 1 argument, got %d", len(args))
			}
			switch arg := args[0].(type) {
			case *String:
				return &Integer{Value: int64(len(arg.Value))}
			case *Array:
				return &Integer{Value: int64(len(arg.Elements))}
			default:
				return newError("len() does not support %s", arg.Type())
			}
		},
	},
	{
		Name: "push",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 2 {
				return newError("push() takes 2 arguments, got %d", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return newError("push() first argument must be an array, got %s", args[0].Type())
			}
			newElems := make([]Object, len(arr.Elements)+1)
			copy(newElems, arr.Elements)
			newElems[len(arr.Elements)] = args[1]
			return &Array{Elements: newElems}
		},
	},
	{
		Name: "pop",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 1 {
				return newError("pop() takes 1 argument, got %d", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return newError("pop() argument must be an array, got %s", args[0].Type())
			}
			if len(arr.Elements) == 0 {
				return &Null{}
			}
			newElems := make([]Object, len(arr.Elements)-1)
			copy(newElems, arr.Elements[:len(arr.Elements)-1])
			return &Array{Elements: newElems}
		},
	},
	{
		Name: "first",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 1 {
				return newError("first() takes 1 argument, got %d", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return newError("first() argument must be an array, got %s", args[0].Type())
			}
			if len(arr.Elements) == 0 {
				return &Null{}
			}
			return arr.Elements[0]
		},
	},
	{
		Name: "last",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 1 {
				return newError("last() takes 1 argument, got %d", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return newError("last() argument must be an array, got %s", args[0].Type())
			}
			if len(arr.Elements) == 0 {
				return &Null{}
			}
			return arr.Elements[len(arr.Elements)-1]
		},
	},
	{
		Name: "rest",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 1 {
				return newError("rest() takes 1 argument, got %d", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return newError("rest() argument must be an array, got %s", args[0].Type())
			}
			if len(arr.Elements) <= 1 {
				return &Array{Elements: []Object{}}
			}
			newElems := make([]Object, len(arr.Elements)-1)
			copy(newElems, arr.Elements[1:])
			return &Array{Elements: newElems}
		},
	},
	{
		Name: "str",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 1 {
				return newError("str() takes 1 argument, got %d", len(args))
			}
			return &String{Value: args[0].Inspect()}
		},
	},
	{
		Name: "int",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 1 {
				return newError("int() takes 1 argument, got %d", len(args))
			}
			switch a := args[0].(type) {
			case *Integer:
				return a
			case *Float:
				return &Integer{Value: int64(a.Value)}
			case *String:
				var n int64
				_, err := fmt.Sscanf(a.Value, "%d", &n)
				if err != nil {
					return newError("int(): cannot convert '%s' to integer", a.Value)
				}
				return &Integer{Value: n}
			default:
				return newError("int() does not support %s", a.Type())
			}
		},
	},
	{
		Name: "type",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 1 {
				return newError("type() takes 1 argument, got %d", len(args))
			}
			return &String{Value: string(args[0].Type())}
		},
	},
	{
		Name: "upper",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 1 {
				return newError("upper() takes 1 argument, got %d", len(args))
			}
			s, ok := args[0].(*String)
			if !ok {
				return newError("upper() argument must be a string, got %s", args[0].Type())
			}
			return &String{Value: strings.ToUpper(s.Value)}
		},
	},
	{
		Name: "lower",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 1 {
				return newError("lower() takes 1 argument, got %d", len(args))
			}
			s, ok := args[0].(*String)
			if !ok {
				return newError("lower() argument must be a string, got %s", args[0].Type())
			}
			return &String{Value: strings.ToLower(s.Value)}
		},
	},
	{
		Name: "keys",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 1 {
				return newError("keys() takes 1 argument, got %d", len(args))
			}
			hash, ok := args[0].(*Hash)
			if !ok {
				return newError("keys() argument must be a hash, got %s", args[0].Type())
			}
			keys := make([]Object, 0, len(hash.Pairs))
			for _, pair := range hash.Pairs {
				keys = append(keys, pair.Key)
			}
			return &Array{Elements: keys}
		},
	},
	{
		Name: "values",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 1 {
				return newError("values() takes 1 argument, got %d", len(args))
			}
			hash, ok := args[0].(*Hash)
			if !ok {
				return newError("values() argument must be a hash, got %s", args[0].Type())
			}
			vals := make([]Object, 0, len(hash.Pairs))
			for _, pair := range hash.Pairs {
				vals = append(vals, pair.Value)
			}
			return &Array{Elements: vals}
		},
	},
	{
		Name: "has",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 2 {
				return newError("has() takes 2 arguments, got %d", len(args))
			}
			hash, ok := args[0].(*Hash)
			if !ok {
				return newError("has() first argument must be a hash, got %s", args[0].Type())
			}
			key, ok := args[1].(Hashable)
			if !ok {
				return newError("has() second argument must be hashable, got %s", args[1].Type())
			}
			_, exists := hash.Pairs[key.HashKey()]
			return &BooleanObj{Value: exists}
		},
	},
	{
		Name: "range",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) < 1 || len(args) > 3 {
				return newError("range() takes 1-3 arguments, got %d", len(args))
			}
			var start, end, step int64
			switch len(args) {
			case 1:
				n, ok := args[0].(*Integer)
				if !ok {
					return newError("range() arguments must be integers")
				}
				start, end, step = 0, n.Value, 1
			case 2:
				s, ok1 := args[0].(*Integer)
				e, ok2 := args[1].(*Integer)
				if !ok1 || !ok2 {
					return newError("range() arguments must be integers")
				}
				start, end, step = s.Value, e.Value, 1
			case 3:
				s, ok1 := args[0].(*Integer)
				e, ok2 := args[1].(*Integer)
				st, ok3 := args[2].(*Integer)
				if !ok1 || !ok2 || !ok3 {
					return newError("range() arguments must be integers")
				}
				start, end, step = s.Value, e.Value, st.Value
			}
			if step == 0 {
				return newError("range() step cannot be zero")
			}
			var elems []Object
			if step > 0 {
				for i := start; i < end; i += step {
					elems = append(elems, &Integer{Value: i})
				}
			} else {
				for i := start; i > end; i += step {
					elems = append(elems, &Integer{Value: i})
				}
			}
			if elems == nil {
				elems = []Object{}
			}
			return &Array{Elements: elems}
		},
	},
	// ─── Math ───────────────────────────────────────────────────────────────
	{
		Name: "abs",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 1 {
				return newError("abs() takes 1 argument, got %d", len(args))
			}
			switch a := args[0].(type) {
			case *Integer:
				v := a.Value
				if v < 0 {
					v = -v
				}
				return &Integer{Value: v}
			case *Float:
				return &Float{Value: math.Abs(a.Value)}
			default:
				return newError("abs() does not support %s", a.Type())
			}
		},
	},
	{
		Name: "min",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) < 1 {
				return newError("min() takes at least 1 argument, got %d", len(args))
			}
			return minMax(args, true)
		},
	},
	{
		Name: "max",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) < 1 {
				return newError("max() takes at least 1 argument, got %d", len(args))
			}
			return minMax(args, false)
		},
	},
	{
		Name: "sqrt",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 1 {
				return newError("sqrt() takes 1 argument, got %d", len(args))
			}
			f, ok := asFloatObj(args[0])
			if !ok {
				return newError("sqrt() argument must be a number, got %s", args[0].Type())
			}
			if f < 0 {
				return newError("sqrt() of negative number")
			}
			return &Float{Value: math.Sqrt(f)}
		},
	},
	{
		Name: "pow",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 2 {
				return newError("pow() takes 2 arguments, got %d", len(args))
			}
			base, ok1 := asFloatObj(args[0])
			exp, ok2 := asFloatObj(args[1])
			if !ok1 || !ok2 {
				return newError("pow() arguments must be numbers")
			}
			return &Float{Value: math.Pow(base, exp)}
		},
	},
	// ─── String ─────────────────────────────────────────────────────────────
	{
		Name: "split",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 2 {
				return newError("split() takes 2 arguments (string, sep), got %d", len(args))
			}
			s, ok1 := args[0].(*String)
			sep, ok2 := args[1].(*String)
			if !ok1 || !ok2 {
				return newError("split() arguments must be strings")
			}
			parts := strings.Split(s.Value, sep.Value)
			elems := make([]Object, len(parts))
			for i, p := range parts {
				elems[i] = &String{Value: p}
			}
			return &Array{Elements: elems}
		},
	},
	{
		Name: "join",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 2 {
				return newError("join() takes 2 arguments (array, sep), got %d", len(args))
			}
			arr, ok1 := args[0].(*Array)
			sep, ok2 := args[1].(*String)
			if !ok1 {
				return newError("join() first argument must be an array, got %s", args[0].Type())
			}
			if !ok2 {
				return newError("join() second argument must be a string, got %s", args[1].Type())
			}
			parts := make([]string, len(arr.Elements))
			for i, el := range arr.Elements {
				parts[i] = el.Inspect()
			}
			return &String{Value: strings.Join(parts, sep.Value)}
		},
	},
	{
		Name: "trim",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 1 {
				return newError("trim() takes 1 argument, got %d", len(args))
			}
			s, ok := args[0].(*String)
			if !ok {
				return newError("trim() argument must be a string, got %s", args[0].Type())
			}
			return &String{Value: strings.TrimSpace(s.Value)}
		},
	},
	{
		Name: "contains",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 2 {
				return newError("contains() takes 2 arguments, got %d", len(args))
			}
			s, ok1 := args[0].(*String)
			sub, ok2 := args[1].(*String)
			if !ok1 || !ok2 {
				return newError("contains() arguments must be strings")
			}
			return &BooleanObj{Value: strings.Contains(s.Value, sub.Value)}
		},
	},
	{
		Name: "replace",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) != 3 {
				return newError("replace() takes 3 arguments (string, old, new), got %d", len(args))
			}
			s, ok1 := args[0].(*String)
			oldS, ok2 := args[1].(*String)
			newS, ok3 := args[2].(*String)
			if !ok1 || !ok2 || !ok3 {
				return newError("replace() arguments must be strings")
			}
			return &String{Value: strings.ReplaceAll(s.Value, oldS.Value, newS.Value)}
		},
	},
	{
		Name: "format",
		Fn: func(vm *VM, args ...Object) Object {
			if len(args) < 1 {
				return newError("format() takes at least 1 argument (template), got %d", len(args))
			}
			tmpl, ok := args[0].(*String)
			if !ok {
				return newError("format() first argument must be a string, got %s", args[0].Type())
			}
			result := tmpl.Value
			for _, arg := range args[1:] {
				idx := strings.Index(result, "{}")
				if idx < 0 {
					break
				}
				result = result[:idx] + arg.Inspect() + result[idx+2:]
			}
			return &String{Value: result}
		},
	},
}

// builtinIndex maps builtin names to their index in Builtins.
// Used by the compiler to emit OpGetBuiltin.
var builtinIndex map[string]int

func init() {
	// map, filter, reduce are appended here (not in the var literal) to avoid
	// an initialization cycle: Builtins → callCompiledFn → vm.Run → Builtins.
	Builtins = append(Builtins,
		&Builtin{
			Name: "map",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 2 {
					return newError("map() takes 2 arguments (array, func), got %d", len(args))
				}
				arr, ok := args[0].(*Array)
				if !ok {
					return newError("map() first argument must be an array, got %s", args[0].Type())
				}
				if !isFunc(args[1]) {
					return newError("map() second argument must be a function, got %s", args[1].Type())
				}
				result := make([]Object, len(arr.Elements))
				for i, el := range arr.Elements {
					result[i] = callUserFn(vm, args[1], el)
				}
				return &Array{Elements: result}
			},
		},
		&Builtin{
			Name: "filter",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 2 {
					return newError("filter() takes 2 arguments (array, func), got %d", len(args))
				}
				arr, ok := args[0].(*Array)
				if !ok {
					return newError("filter() first argument must be an array, got %s", args[0].Type())
				}
				if !isFunc(args[1]) {
					return newError("filter() second argument must be a function, got %s", args[1].Type())
				}
				var result []Object
				for _, el := range arr.Elements {
					keep := callUserFn(vm, args[1], el)
					if isTruthy(keep) {
						result = append(result, el)
					}
				}
				if result == nil {
					result = []Object{}
				}
				return &Array{Elements: result}
			},
		},
		&Builtin{
			Name: "reduce",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 3 {
					return newError("reduce() takes 3 arguments (array, func, initial), got %d", len(args))
				}
				arr, ok := args[0].(*Array)
				if !ok {
					return newError("reduce() first argument must be an array, got %s", args[0].Type())
				}
				if !isFunc(args[1]) {
					return newError("reduce() second argument must be a function, got %s", args[1].Type())
				}
				acc := args[2]
				for _, el := range arr.Elements {
					acc = callUserFn(vm, args[1], acc, el)
				}
				return acc
			},
		},
		// ─── Concurrency ──────────────────────────────────────────────────────
		&Builtin{
			Name: "chan",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) > 1 {
					return newError("chan() takes 0 or 1 arguments (capacity), got %d", len(args))
				}
				cap := 0
				if len(args) == 1 {
					n, ok := args[0].(*Integer)
					if !ok {
						return newError("chan() capacity must be an integer, got %s", args[0].Type())
					}
					if n.Value < 0 {
						return newError("chan() capacity cannot be negative")
					}
					cap = int(n.Value)
				}
				return &ChannelObj{Ch: make(chan Object, cap), Cap: cap}
			},
		},
		&Builtin{
			Name: "send",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 2 {
					return newError("send() takes 2 arguments (channel, value), got %d", len(args))
				}
				ch, ok := args[0].(*ChannelObj)
				if !ok {
					return newError("send() first argument must be a channel, got %s", args[0].Type())
				}
				select {
				case ch.Ch <- args[1]:
					return &Null{}
				case <-vm.cancel:
					return newError("task cancelled")
				case <-vm.parentCancel:
					return newError("task cancelled")
				}
			},
		},
		&Builtin{
			Name: "recv",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 1 {
					return newError("recv() takes 1 argument (channel), got %d", len(args))
				}
				ch, ok := args[0].(*ChannelObj)
				if !ok {
					return newError("recv() first argument must be a channel, got %s", args[0].Type())
				}
				select {
				case v := <-ch.Ch:
					return v
				case <-vm.cancel:
					return newError("task cancelled")
				case <-vm.parentCancel:
					return newError("task cancelled")
				}
			},
		},
		&Builtin{
			Name: "await",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 1 {
					return newError("await() takes 1 argument (task), got %d", len(args))
				}
				task, ok := args[0].(*TaskObj)
				if !ok {
					return newError("await() argument must be a task, got %s", args[0].Type())
				}
				select {
				case <-task.Done:
					if task.Err != nil {
						return newError("task failed: %s", task.Err.Error())
					}
					if task.Result == nil {
						return &Null{}
					}
					return task.Result
				case <-vm.cancel:
					return newError("task cancelled")
				case <-vm.parentCancel:
					return newError("task cancelled")
				}
			},
		},
		&Builtin{
			Name: "sleep",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 1 {
					return newError("sleep() takes 1 argument (milliseconds), got %d", len(args))
				}
				ms, ok := args[0].(*Integer)
				if !ok {
					return newError("sleep() argument must be an integer (milliseconds), got %s", args[0].Type())
				}
				timer := time.NewTimer(time.Duration(ms.Value) * time.Millisecond)
				select {
				case <-timer.C:
					return &Null{}
				case <-vm.cancel:
					timer.Stop()
					return newError("task cancelled")
				case <-vm.parentCancel:
					timer.Stop()
					return newError("task cancelled")
				}
			},
		},
	)

	builtinIndex = make(map[string]int, len(Builtins))
	for i, b := range Builtins {
		builtinIndex[b.Name] = i
	}
}

// newError creates an Error object for builtin runtime errors.
func newError(format string, args ...interface{}) Object {
	return &Error{Message: fmt.Sprintf(format, args...)}
}

// callUserFn invokes a Builtin, Closure, or CompiledFunction with the given args.
func callUserFn(vm *VM, fnObj Object, args ...Object) Object {
	switch fn := fnObj.(type) {
	case *Builtin:
		return fn.Fn(vm, args...)
	case *Closure, *CompiledFunction:
		result, err := vm.CallFn(fnObj, args...)
		if err != nil {
			return newError("%s", err.Error())
		}
		return result
	default:
		return newError("expected a function, got %s", fnObj.Type())
	}
}

func isFunc(obj Object) bool {
	switch obj.(type) {
	case *Builtin, *Closure, *CompiledFunction:
		return true
	default:
		return false
	}
}

func asFloatObj(o Object) (float64, bool) {
	switch v := o.(type) {
	case *Integer:
		return float64(v.Value), true
	case *Float:
		return v.Value, true
	default:
		return 0, false
	}
}

func minMax(args []Object, wantMin bool) Object {
	bestI, okI := args[0].(*Integer)
	bestF, okF := args[0].(*Float)
	if !okI && !okF {
		return newError("min/max arguments must be numbers, got %s", args[0].Type())
	}
	useFloat := okF
	var bi int64
	var bf float64
	if okI {
		bi = bestI.Value
		bf = float64(bestI.Value)
	} else {
		bf = bestF.Value
	}
	for _, arg := range args[1:] {
		switch v := arg.(type) {
		case *Integer:
			fv := float64(v.Value)
			if wantMin {
				if useFloat {
					if fv < bf {
						bf = fv
					}
				} else if v.Value < bi {
					bi = v.Value
					bf = fv
				}
			} else {
				if useFloat {
					if fv > bf {
						bf = fv
					}
				} else if v.Value > bi {
					bi = v.Value
					bf = fv
				}
			}
		case *Float:
			useFloat = true
			if wantMin {
				if v.Value < bf {
					bf = v.Value
				}
			} else {
				if v.Value > bf {
					bf = v.Value
				}
			}
		default:
			return newError("min/max arguments must be numbers, got %s", arg.Type())
		}
	}
	if useFloat {
		return &Float{Value: bf}
	}
	return &Integer{Value: bi}
}
