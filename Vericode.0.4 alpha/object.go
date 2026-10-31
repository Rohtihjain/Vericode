package main

import (
	"fmt"
	"strings"
)

type ObjectType string

const (
	INTEGER_OBJ     ObjectType = "INTEGER"
	FLOAT_OBJ       ObjectType = "FLOAT"
	BOOLEAN_OBJ     ObjectType = "BOOLEAN"
	STRING_OBJ      ObjectType = "STRING"
	NULL_OBJ        ObjectType = "NULL"
	ARRAY_OBJ       ObjectType = "ARRAY"
	HASH_OBJ        ObjectType = "HASH"
	COMPILED_FN_OBJ ObjectType = "COMPILED_FUNCTION"
	CLOSURE_OBJ     ObjectType = "CLOSURE"
	BUILTIN_OBJ     ObjectType = "BUILTIN"
	ERROR_OBJ       ObjectType = "ERROR"
	CHANNEL_OBJ     ObjectType = "CHANNEL"
	TASK_OBJ        ObjectType = "TASK"
)

type Object interface {
	Type() ObjectType
	Inspect() string
}

type Hashable interface {
	HashKey() string
}

// ─── Primitive types ──────────────────────────────────────────────────────────

type Integer struct{ Value int64 }

func (i *Integer) Type() ObjectType { return INTEGER_OBJ }
func (i *Integer) Inspect() string  { return fmt.Sprintf("%d", i.Value) }
func (i *Integer) HashKey() string  { return fmt.Sprintf("int:%d", i.Value) }

type Float struct{ Value float64 }

func (f *Float) Type() ObjectType { return FLOAT_OBJ }
func (f *Float) Inspect() string  { return fmt.Sprintf("%g", f.Value) }
func (f *Float) HashKey() string  { return fmt.Sprintf("float:%g", f.Value) }

type BooleanObj struct{ Value bool }

func (b *BooleanObj) Type() ObjectType { return BOOLEAN_OBJ }
func (b *BooleanObj) Inspect() string  { return fmt.Sprintf("%t", b.Value) }
func (b *BooleanObj) HashKey() string  { return fmt.Sprintf("bool:%t", b.Value) }

type String struct{ Value string }

func (s *String) Type() ObjectType { return STRING_OBJ }
func (s *String) Inspect() string  { return s.Value }
func (s *String) HashKey() string  { return fmt.Sprintf("str:%s", s.Value) }

type Null struct{}

func (n *Null) Type() ObjectType { return NULL_OBJ }
func (n *Null) Inspect() string  { return "null" }

// ─── Array ────────────────────────────────────────────────────────────────────

type Array struct{ Elements []Object }

func (a *Array) Type() ObjectType { return ARRAY_OBJ }
func (a *Array) Inspect() string {
	elems := make([]string, len(a.Elements))
	for i, e := range a.Elements {
		elems[i] = e.Inspect()
	}
	return "[" + strings.Join(elems, ", ") + "]"
}

// ─── Hash ─────────────────────────────────────────────────────────────────────

type HashPairObj struct {
	Key   Object
	Value Object
}

type Hash struct {
	Pairs map[string]HashPairObj
}

func (h *Hash) Type() ObjectType { return HASH_OBJ }
func (h *Hash) Inspect() string {
	var pairs []string
	for _, pair := range h.Pairs {
		pairs = append(pairs, fmt.Sprintf("%s: %s", pair.Key.Inspect(), pair.Value.Inspect()))
	}
	return "{" + strings.Join(pairs, ", ") + "}"
}

// ─── Compiled function ────────────────────────────────────────────────────────

type CompiledFunction struct {
	Instructions []byte
	NumParams    int
	NumFrees     int
	NumLocals    int // total local slots (params + body locals); reserved on call
	SourceMap    []SourceLocation
	Name         string
	Filename     string
	Constants    []Object // parent constant pool, set at emit time
}

// Closure is a compiled function with captured free variables.
type Closure struct {
	Fn   *CompiledFunction
	Free []Object
}

func (cl *Closure) Type() ObjectType { return CLOSURE_OBJ }
func (cl *Closure) Inspect() string {
	if cl.Fn != nil && cl.Fn.Name != "" {
		return fmt.Sprintf("closure %s(...){...}", cl.Fn.Name)
	}
	return "closure(...){...}"
}

func (cf *CompiledFunction) Type() ObjectType { return COMPILED_FN_OBJ }
func (cf *CompiledFunction) Inspect() string {
	if cf.Name != "" {
		return fmt.Sprintf("func %s(...){...}", cf.Name)
	}
	return "func(...){...}"
}

// ─── Error object ─────────────────────────────────────────────────────────────

type Error struct {
	Message string
}

func (e *Error) Type() ObjectType { return ERROR_OBJ }
func (e *Error) Inspect() string  { return "Error: " + e.Message }

// ─── Built-in function ────────────────────────────────────────────────────────

// BuiltinFn receives the calling VM so builtins can spawn, block on channels
// with cancellation, and invoke user functions inline.
type BuiltinFn func(vm *VM, args ...Object) Object

type Builtin struct {
	Name string
	Fn   BuiltinFn
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string  { return "<builtin:" + b.Name + ">" }

// ─── Channel ─────────────────────────────────────────────────────────────────

// ChannelObj is a message-passing channel. Cap 0 is unbuffered: send rendezvous
// with a receiver. Values sent through a channel are shared by reference.
type ChannelObj struct {
	Ch   chan Object
	Cap  int
	Name string
}

func (c *ChannelObj) Type() ObjectType { return CHANNEL_OBJ }
func (c *ChannelObj) Inspect() string {
	if c.Name != "" {
		return fmt.Sprintf("<channel %s>", c.Name)
	}
	return "<channel>"
}

// ─── Task ─────────────────────────────────────────────────────────────────────

// TaskObj is a handle to a spawned task. Done is closed when the task
// finishes, after which Result and Err are safe to read. SpawnLoc records
// where the task was spawned, used when re-raising its failure.
type TaskObj struct {
	ID       int64
	Name     string
	Done     chan struct{}
	Result   Object
	Err      error
	SpawnLoc SourceLocation
}

func (t *TaskObj) Type() ObjectType { return TASK_OBJ }
func (t *TaskObj) Inspect() string {
	select {
	case <-t.Done:
		if t.Err != nil {
			return fmt.Sprintf("<task %s failed>", t.Name)
		}
		return fmt.Sprintf("<task %s done>", t.Name)
	default:
		return fmt.Sprintf("<task %s running>", t.Name)
	}
}
