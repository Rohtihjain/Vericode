package main

import (
	"fmt"
	"sort"
	"strings"
)

// ─── Batch transforms (0.4) ──────────────────────────────────────────────────
//
// Every collection builtin takes the collection first, so calls compose with
// the pipe operator (which feeds its left side in as the first argument):
//
//	range(1, 20) |> filter(even) |> map(square) |> take(3) |> sum

func dataBuiltins() []*Builtin {
	return []*Builtin{
		{
			Name: "take",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 2 {
					return newError("take() takes 2 arguments (array, count), got %d", len(args))
				}
				arr, errObj := arrayArg("take", "first argument", args[0])
				if errObj != nil {
					return errObj
				}
				n, errObj := intArg("take", "count", args[1])
				if errObj != nil {
					return errObj
				}
				if n < 0 {
					return newError("take() count cannot be negative, got %d", n)
				}
				if n > int64(len(arr.Elements)) {
					n = int64(len(arr.Elements))
				}
				out := make([]Object, n)
				copy(out, arr.Elements[:n])
				return &Array{Elements: out}
			},
		},
		{
			Name: "drop",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 2 {
					return newError("drop() takes 2 arguments (array, count), got %d", len(args))
				}
				arr, errObj := arrayArg("drop", "first argument", args[0])
				if errObj != nil {
					return errObj
				}
				n, errObj := intArg("drop", "count", args[1])
				if errObj != nil {
					return errObj
				}
				if n < 0 {
					return newError("drop() count cannot be negative, got %d", n)
				}
				if n > int64(len(arr.Elements)) {
					n = int64(len(arr.Elements))
				}
				out := make([]Object, len(arr.Elements)-int(n))
				copy(out, arr.Elements[n:])
				return &Array{Elements: out}
			},
		},
		{
			Name: "chunk",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 2 {
					return newError("chunk() takes 2 arguments (array, size), got %d", len(args))
				}
				arr, errObj := arrayArg("chunk", "first argument", args[0])
				if errObj != nil {
					return errObj
				}
				n, errObj := intArg("chunk", "size", args[1])
				if errObj != nil {
					return errObj
				}
				if n < 1 {
					return newError("chunk() size must be at least 1, got %d", n)
				}
				out := []Object{}
				for i := 0; i < len(arr.Elements); i += int(n) {
					end := i + int(n)
					if end > len(arr.Elements) {
						end = len(arr.Elements)
					}
					piece := make([]Object, end-i)
					copy(piece, arr.Elements[i:end])
					out = append(out, &Array{Elements: piece})
				}
				return &Array{Elements: out}
			},
		},
		{
			Name: "flatten",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 1 {
					return newError("flatten() takes 1 argument (array), got %d", len(args))
				}
				arr, errObj := arrayArg("flatten", "argument", args[0])
				if errObj != nil {
					return errObj
				}
				out := []Object{}
				for _, el := range arr.Elements {
					if inner, ok := el.(*Array); ok {
						out = append(out, inner.Elements...)
					} else {
						out = append(out, el)
					}
				}
				return &Array{Elements: out}
			},
		},
		{
			Name: "zip",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 2 {
					return newError("zip() takes 2 arguments (array, array), got %d", len(args))
				}
				a, errObj := arrayArg("zip", "first argument", args[0])
				if errObj != nil {
					return errObj
				}
				b, errObj := arrayArg("zip", "second argument", args[1])
				if errObj != nil {
					return errObj
				}
				n := len(a.Elements)
				if len(b.Elements) < n {
					n = len(b.Elements)
				}
				out := make([]Object, n)
				for i := 0; i < n; i++ {
					out[i] = &Array{Elements: []Object{a.Elements[i], b.Elements[i]}}
				}
				return &Array{Elements: out}
			},
		},
		{
			Name: "enumerate",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 1 {
					return newError("enumerate() takes 1 argument (array), got %d", len(args))
				}
				arr, errObj := arrayArg("enumerate", "argument", args[0])
				if errObj != nil {
					return errObj
				}
				out := make([]Object, len(arr.Elements))
				for i, el := range arr.Elements {
					out[i] = &Array{Elements: []Object{&Integer{Value: int64(i)}, el}}
				}
				return &Array{Elements: out}
			},
		},
		{
			Name: "uniq",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 1 {
					return newError("uniq() takes 1 argument (array), got %d", len(args))
				}
				arr, errObj := arrayArg("uniq", "argument", args[0])
				if errObj != nil {
					return errObj
				}
				seen := make(map[string]bool, len(arr.Elements))
				out := []Object{}
				for _, el := range arr.Elements {
					key := valueKey(el)
					if !seen[key] {
						seen[key] = true
						out = append(out, el)
					}
				}
				return &Array{Elements: out}
			},
		},
		{
			Name: "sum",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 1 {
					return newError("sum() takes 1 argument (array), got %d", len(args))
				}
				arr, errObj := arrayArg("sum", "argument", args[0])
				if errObj != nil {
					return errObj
				}
				var intSum int64
				var floatSum float64
				allInts := true
				for i, el := range arr.Elements {
					switch v := el.(type) {
					case *Integer:
						intSum += v.Value
						floatSum += float64(v.Value)
					case *Float:
						allInts = false
						floatSum += v.Value
					default:
						return newError("sum() element %d must be a number, got %s", i, el.Type())
					}
				}
				if allInts {
					return &Integer{Value: intSum}
				}
				return &Float{Value: floatSum}
			},
		},
		{
			Name: "avg",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 1 {
					return newError("avg() takes 1 argument (array), got %d", len(args))
				}
				arr, errObj := arrayArg("avg", "argument", args[0])
				if errObj != nil {
					return errObj
				}
				if len(arr.Elements) == 0 {
					return newError("avg() of an empty array")
				}
				total := 0.0
				for i, el := range arr.Elements {
					f, ok := asFloat(el)
					if !ok {
						return newError("avg() element %d must be a number, got %s", i, el.Type())
					}
					total += f
				}
				return &Float{Value: total / float64(len(arr.Elements))}
			},
		},
		{
			Name: "sort",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 1 {
					return newError("sort() takes 1 argument (array), got %d", len(args))
				}
				arr, errObj := arrayArg("sort", "argument", args[0])
				if errObj != nil {
					return errObj
				}
				out := make([]Object, len(arr.Elements))
				copy(out, arr.Elements)
				var cmpErr error
				sort.SliceStable(out, func(i, j int) bool {
					if cmpErr != nil {
						return false
					}
					c, err := compareValues(out[i], out[j])
					if err != nil {
						cmpErr = err
						return false
					}
					return c < 0
				})
				if cmpErr != nil {
					return newError("sort() %v", cmpErr)
				}
				return &Array{Elements: out}
			},
		},
		{
			Name: "sort_by",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 2 {
					return newError("sort_by() takes 2 arguments (array, func), got %d", len(args))
				}
				arr, errObj := arrayArg("sort_by", "first argument", args[0])
				if errObj != nil {
					return errObj
				}
				if !isFunc(args[1]) {
					return newError("sort_by() second argument must be a function, got %s", args[1].Type())
				}
				type keyed struct {
					key Object
					val Object
				}
				items := make([]keyed, len(arr.Elements))
				for i, el := range arr.Elements {
					key := callUserFn(vm, args[1], el)
					if e, ok := key.(*Error); ok {
						return e
					}
					items[i] = keyed{key: key, val: el}
				}
				var cmpErr error
				sort.SliceStable(items, func(i, j int) bool {
					if cmpErr != nil {
						return false
					}
					c, err := compareValues(items[i].key, items[j].key)
					if err != nil {
						cmpErr = err
						return false
					}
					return c < 0
				})
				if cmpErr != nil {
					return newError("sort_by() %v", cmpErr)
				}
				out := make([]Object, len(items))
				for i, it := range items {
					out[i] = it.val
				}
				return &Array{Elements: out}
			},
		},
		{
			Name: "reverse",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 1 {
					return newError("reverse() takes 1 argument (array), got %d", len(args))
				}
				arr, errObj := arrayArg("reverse", "argument", args[0])
				if errObj != nil {
					return errObj
				}
				out := make([]Object, len(arr.Elements))
				for i, el := range arr.Elements {
					out[len(arr.Elements)-1-i] = el
				}
				return &Array{Elements: out}
			},
		},
		{
			Name: "any",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) < 1 || len(args) > 2 {
					return newError("any() takes 1-2 arguments (array, [func]), got %d", len(args))
				}
				arr, errObj := arrayArg("any", "first argument", args[0])
				if errObj != nil {
					return errObj
				}
				if len(args) == 1 {
					for _, el := range arr.Elements {
						if isTruthy(el) {
							return &BooleanObj{Value: true}
						}
					}
					return &BooleanObj{Value: false}
				}
				if !isFunc(args[1]) {
					return newError("any() second argument must be a function, got %s", args[1].Type())
				}
				for _, el := range arr.Elements {
					r := callUserFn(vm, args[1], el)
					if e, ok := r.(*Error); ok {
						return e
					}
					if isTruthy(r) {
						return &BooleanObj{Value: true}
					}
				}
				return &BooleanObj{Value: false}
			},
		},
		{
			Name: "all",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) < 1 || len(args) > 2 {
					return newError("all() takes 1-2 arguments (array, [func]), got %d", len(args))
				}
				arr, errObj := arrayArg("all", "first argument", args[0])
				if errObj != nil {
					return errObj
				}
				if len(args) == 1 {
					for _, el := range arr.Elements {
						if !isTruthy(el) {
							return &BooleanObj{Value: false}
						}
					}
					return &BooleanObj{Value: true}
				}
				if !isFunc(args[1]) {
					return newError("all() second argument must be a function, got %s", args[1].Type())
				}
				for _, el := range arr.Elements {
					r := callUserFn(vm, args[1], el)
					if e, ok := r.(*Error); ok {
						return e
					}
					if !isTruthy(r) {
						return &BooleanObj{Value: false}
					}
				}
				return &BooleanObj{Value: true}
			},
		},
		{
			Name: "group_by",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 2 {
					return newError("group_by() takes 2 arguments (array, func), got %d", len(args))
				}
				arr, errObj := arrayArg("group_by", "first argument", args[0])
				if errObj != nil {
					return errObj
				}
				if !isFunc(args[1]) {
					return newError("group_by() second argument must be a function, got %s", args[1].Type())
				}
				groups := make(map[string]HashPairObj)
				for i, el := range arr.Elements {
					key := callUserFn(vm, args[1], el)
					if e, ok := key.(*Error); ok {
						return e
					}
					hashable, ok := key.(Hashable)
					if !ok {
						return newError("group_by() key for element %d must be hashable (integer, float, boolean, or string), got %s", i, key.Type())
					}
					pair, exists := groups[hashable.HashKey()]
					elems := []Object{}
					if exists {
						elems = pair.Value.(*Array).Elements
					}
					elems = append(elems, el)
					groups[hashable.HashKey()] = HashPairObj{
						Key:   key,
						Value: &Array{Elements: elems},
					}
				}
				return &Hash{Pairs: groups}
			},
		},
		{
			Name: "pluck",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 2 {
					return newError("pluck() takes 2 arguments (array or table, column), got %d", len(args))
				}
				key, ok := args[1].(*String)
				if !ok {
					return newError("pluck() second argument must be a string, got %s", args[1].Type())
				}
				if t, ok := args[0].(*Table); ok {
					return &Array{Elements: t.column(key.Value)}
				}
				arr, errObj := arrayArg("pluck", "first argument", args[0])
				if errObj != nil {
					return errObj
				}
				out := make([]Object, len(arr.Elements))
				for i, el := range arr.Elements {
					row, ok := el.(*Hash)
					if !ok {
						return newError("pluck() element %d must be a hash, got %s", i, el.Type())
					}
					out[i] = rowCell(row, key.Value)
				}
				return &Array{Elements: out}
			},
		},
	}
}

// ─── Argument validation helpers ────────────────────────────────────────────

func arrayArg(op, which string, arg Object) (*Array, Object) {
	arr, ok := arg.(*Array)
	if !ok {
		return nil, newError("%s() %s must be an array, got %s", op, which, arg.Type())
	}
	return arr, nil
}

func intArg(op, which string, arg Object) (int64, Object) {
	n, ok := arg.(*Integer)
	if !ok {
		return 0, newError("%s() %s must be an integer, got %s", op, which, arg.Type())
	}
	return n.Value, nil
}

// ─── Comparison and equality ────────────────────────────────────────────────

// compareValues orders numbers, strings, and booleans; nulls always sort
// last. Anything else is an error rather than a silent arbitrary order.
func compareValues(a, b Object) (int, error) {
	aNull := a == nil || a.Type() == NULL_OBJ
	bNull := b == nil || b.Type() == NULL_OBJ
	switch {
	case aNull && bNull:
		return 0, nil
	case aNull:
		return 1, nil
	case bNull:
		return -1, nil
	}
	if af, ok := asFloat(a); ok {
		if bf, ok := asFloat(b); ok {
			switch {
			case af < bf:
				return -1, nil
			case af > bf:
				return 1, nil
			default:
				return 0, nil
			}
		}
	}
	if as, ok := a.(*String); ok {
		if bs, ok := b.(*String); ok {
			return strings.Compare(as.Value, bs.Value), nil
		}
	}
	if ab, ok := a.(*BooleanObj); ok {
		if bb, ok := b.(*BooleanObj); ok {
			switch {
			case ab.Value == bb.Value:
				return 0, nil
			case !ab.Value:
				return -1, nil
			default:
				return 1, nil
			}
		}
	}
	return 0, fmt.Errorf("cannot compare %s with %s", a.Type(), b.Type())
}

// objectsEqual is structural equality for containers and value equality for
// primitives; functions, channels, and tasks compare by identity. Comparisons
// are depth-limited so cyclic arrays/hashes cannot exhaust the stack.
func objectsEqual(a, b Object) bool {
	return objectsEqualDepth(a, b, 0)
}

func objectsEqualDepth(a, b Object, depth int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if a == b {
		return true
	}
	if depth > 1000 {
		return false
	}
	if af, ok := asFloat(a); ok {
		if bf, ok := asFloat(b); ok {
			return af == bf
		}
	}
	switch av := a.(type) {
	case *String:
		bv, ok := b.(*String)
		return ok && av.Value == bv.Value
	case *BooleanObj:
		bv, ok := b.(*BooleanObj)
		return ok && av.Value == bv.Value
	case *Null:
		return b.Type() == NULL_OBJ
	case *Array:
		bv, ok := b.(*Array)
		if !ok || len(av.Elements) != len(bv.Elements) {
			return false
		}
		for i := range av.Elements {
			if !objectsEqualDepth(av.Elements[i], bv.Elements[i], depth+1) {
				return false
			}
		}
		return true
	case *Hash:
		bv, ok := b.(*Hash)
		if !ok || len(av.Pairs) != len(bv.Pairs) {
			return false
		}
		for k, pair := range av.Pairs {
			other, exists := bv.Pairs[k]
			if !exists || !objectsEqualDepth(pair.Value, other.Value, depth+1) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}

// valueKey builds a deterministic string key for uniq so structurally equal
// hashes and arrays are recognized regardless of map iteration order.
func valueKey(o Object) string {
	return valueKeyInner(o, make(map[Object]bool))
}

func valueKeyInner(o Object, seen map[Object]bool) string {
	switch v := o.(type) {
	case *Array:
		if seen[o] {
			return "cycle"
		}
		seen[o] = true
		defer delete(seen, o)
		parts := make([]string, len(v.Elements))
		for i, el := range v.Elements {
			parts[i] = valueKeyInner(el, seen)
		}
		return "arr[" + strings.Join(parts, ",") + "]"
	case *Hash:
		if seen[o] {
			return "cycle"
		}
		seen[o] = true
		defer delete(seen, o)
		parts := make([]string, 0, len(v.Pairs))
		for k, pair := range v.Pairs {
			parts = append(parts, k+"="+valueKeyInner(pair.Value, seen))
		}
		sort.Strings(parts)
		return "hash{" + strings.Join(parts, ",") + "}"
	default:
		if h, ok := o.(Hashable); ok {
			return h.HashKey()
		}
		return fmt.Sprintf("ref:%p", o)
	}
}
