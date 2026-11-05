package main

import (
	"bytes"
	"errors"
	"os/exec"
)

// nativeBuiltins adds the 0.7 tooling layer: assertions and the gated hook for
// running external commands.
func nativeBuiltins() []*Builtin {
	return []*Builtin{
		{
			Name: "assert",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) < 1 || len(args) > 2 {
					return newError("assert() takes 1-2 arguments (condition, [message]), got %d", len(args))
				}
				if isTruthy(args[0]) {
					return &Null{}
				}
				msg := "assertion failed"
				if len(args) == 2 {
					msg += ": " + args[1].Inspect()
				}
				return newError("%s", msg)
			},
		},
		{
			Name: "exec",
			Fn: func(vm *VM, args ...Object) Object {
				if len(args) < 1 || len(args) > 2 {
					return newError("exec() takes 1-2 arguments (command, [args array]), got %d", len(args))
				}
				cmd, ok := args[0].(*String)
				if !ok {
					return newError("exec() command must be a string, got %s", args[0].Type())
				}
				var cmdArgs []string
				if len(args) == 2 {
					arr, ok := args[1].(*Array)
					if !ok {
						return newError("exec() second argument must be an array of strings, got %s", args[1].Type())
					}
					cmdArgs = make([]string, len(arr.Elements))
					for i, el := range arr.Elements {
						s, ok := el.(*String)
						if !ok {
							return newError("exec() argument %d must be a string, got %s", i, el.Type())
						}
						cmdArgs[i] = s.Value
					}
				}
				if !vm.allowIO {
					return newError("exec() is disabled in the web IDE; run with the CLI (veri -run file.veri) to run commands")
				}

				c := exec.Command(cmd.Value, cmdArgs...)
				var stdout, stderr bytes.Buffer
				c.Stdout = &stdout
				c.Stderr = &stderr

				code := 0
				if err := c.Run(); err != nil {
					var exitErr *exec.ExitError
					if errors.As(err, &exitErr) {
						// A non-zero exit is a normal result, not an error.
						code = exitErr.ExitCode()
					} else {
						return newError("exec() cannot run '%s': %v", cmd.Value, err)
					}
				}
				return &Hash{Pairs: map[string]HashPairObj{
					cellKey("code"):   {Key: &String{Value: "code"}, Value: &Integer{Value: int64(code)}},
					cellKey("stdout"): {Key: &String{Value: "stdout"}, Value: &String{Value: stdout.String()}},
					cellKey("stderr"): {Key: &String{Value: "stderr"}, Value: &String{Value: stderr.String()}},
				}}
			},
		},
	}
}
