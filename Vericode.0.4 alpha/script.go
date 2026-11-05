package main

import (
	"math/rand"
	"os"
	"time"
)

// scriptBuiltins exposes the host primitives a command-line script expects:
// arguments, environment variables, the clock, and randomness.
func scriptBuiltins() []*Builtin {
	return []*Builtin{
		{
			Name: "args",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 0 {
					return newError("args() takes 0 arguments, got %d", len(args))
				}
				elems := make([]Object, len(vm.args))
				for i, a := range vm.args {
					elems[i] = &String{Value: a}
				}
				return &Array{Elements: elems}
			},
		},
		{
			Name: "env",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 1 {
					return newError("env() takes 1 argument (name), got %d", len(args))
				}
				name, ok := args[0].(*String)
				if !ok {
					return newError("env() argument must be a string, got %s", args[0].Type())
				}
				if !vm.allowIO {
					return newError("env() is disabled in the web IDE; run with the CLI (veri -run file.veri) to read environment variables")
				}
				value, ok := os.LookupEnv(name.Value)
				if !ok {
					return &Null{}
				}
				return &String{Value: value}
			},
		},
		{
			Name: "now",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) != 0 {
					return newError("now() takes 0 arguments, got %d", len(args))
				}
				return &Integer{Value: time.Now().UnixMilli()}
			},
		},
		{
			Name: "random",
			Fn: func(vm *VM, args ...Object) Object {
				switch len(args) {
				case 0:
					return &Float{Value: rand.Float64()}
				case 1:
					bound, ok := args[0].(*Integer)
					if !ok {
						return newError("random() bound must be an integer, got %s", args[0].Type())
					}
					if bound.Value <= 0 {
						return newError("random() bound must be positive, got %d", bound.Value)
					}
					return &Integer{Value: rand.Int63n(bound.Value)}
				case 2:
					low, ok1 := args[0].(*Integer)
					high, ok2 := args[1].(*Integer)
					if !ok1 || !ok2 {
						return newError("random() bounds must be integers")
					}
					if low.Value >= high.Value {
						return newError("random() low bound must be less than high bound, got %d and %d", low.Value, high.Value)
					}
					span := high.Value - low.Value
					if span <= 0 {
						return newError("random() range is too wide")
					}
					return &Integer{Value: low.Value + rand.Int63n(span)}
				default:
					return newError("random() takes 0-2 arguments, got %d", len(args))
				}
			},
		},
	}
}
