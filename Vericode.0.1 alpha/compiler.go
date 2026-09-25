package main

import (
	"fmt"
	"reflect"
)

// â”€â”€â”€ Compiler â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

type loopContext struct {
	isCountLoop   bool
	continuePos   int
	continueJumps []int
	breakJumps    []int
}

type Compiler struct {
	instructions []byte
	constants    []Object
	symbolTable  map[string]int // global name -> index
	localTable   map[string]int // local name -> index (non-nil inside fn)
	isLocal      bool
	loopStack    []*loopContext

	parent       *Compiler // enclosing function compiler (for free vars)
	freeSymbols  []string  // ordered free variable names
	freeIndex    map[string]int

	filename   string
	source     string
	sourceMap  []SourceLocation
	currentLoc SourceLocation
}

func NewCompiler() *Compiler {
	return NewCompilerWithSource("<main>", "")
}

func NewCompilerWithSource(filename, source string) *Compiler {
	return &Compiler{
		instructions: make([]byte, 0),
		constants:    make([]Object, 0),
		symbolTable:  make(map[string]int),
		loopStack:    make([]*loopContext, 0),
		filename:     filename,
		source:       source,
		sourceMap:    make([]SourceLocation, 0),
	}
}

// newFnCompiler creates a child compiler for a function body.
// Shares the parent's constant pool and global table; has its own locals.
func newFnCompiler(parent *Compiler) *Compiler {
	return &Compiler{
		instructions: make([]byte, 0),
		constants:    parent.constants,
		symbolTable:  parent.symbolTable,
		localTable:   make(map[string]int),
		isLocal:      true,
		loopStack:    make([]*loopContext, 0),
		parent:       parent,
		freeSymbols:  make([]string, 0),
		freeIndex:    make(map[string]int),
		filename:     parent.filename,
		source:       parent.source,
		sourceMap:    make([]SourceLocation, 0),
		currentLoc:   parent.currentLoc,
	}
}

// addFree registers a free variable name and returns its free-slot index.
func (c *Compiler) addFree(name string) int {
	if idx, ok := c.freeIndex[name]; ok {
		return idx
	}
	idx := len(c.freeSymbols)
	c.freeSymbols = append(c.freeSymbols, name)
	c.freeIndex[name] = idx
	return idx
}

// resolveName finds where a name lives: "local", "free", "global", or "".
func (c *Compiler) resolveName(name string) (kind string, index int) {
	if c.isLocal {
		if idx, ok := c.localTable[name]; ok {
			return "local", idx
		}
		// Search enclosing function compilers for a local binding → free var.
		for p := c.parent; p != nil; p = p.parent {
			if p.isLocal {
				if _, ok := p.localTable[name]; ok {
					return "free", c.addFree(name)
				}
			}
		}
	}
	if idx, ok := c.symbolTable[name]; ok {
		return "global", idx
	}
	return "", -1
}

// emitLoadName emits the right load opcode for a resolved name.
func (c *Compiler) emitLoadName(kind string, index int) {
	switch kind {
	case "local":
		c.emit(OpGetLocal, index)
	case "free":
		c.emit(OpGetFree, index)
	case "global":
		c.emit(OpGetGlobal, index)
	}
}

// emitStoreName emits the right store opcode for a resolved name.
func (c *Compiler) emitStoreName(kind string, index int) {
	switch kind {
	case "local":
		c.emit(OpSetLocal, index)
	case "free":
		c.emit(OpSetFree, index)
	case "global":
		c.emit(OpSetGlobal, index)
	}
}

func (c *Compiler) errorf(line, col int, format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s", FormatDiagnostic(c.filename, c.source, "CompileError", msg, line, col))
}

func (c *Compiler) setNodeLoc(node Node) {
	if node == nil {
		return
	}
	// Every concrete AST node has a Token field; read it generically.
	v := reflect.ValueOf(node)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return
	}
	f := v.FieldByName("Token")
	if !f.IsValid() {
		return
	}
	tok, ok := f.Interface().(Token)
	if !ok {
		return
	}
	c.currentLoc = SourceLocation{Filename: c.filename, Line: tok.Line, Col: tok.Col}
}

// â”€â”€â”€ Main compile dispatch â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func (c *Compiler) Compile(node Node) error {
	c.setNodeLoc(node)
	switch node := node.(type) {

	// ── Program ──────────────────────────────────────────────────────────
	case *Program:
		for _, stmt := range node.Statements {
			if err := c.Compile(stmt); err != nil {
				return err
			}
		}

	// ── Statements ───────────────────────────────────────────────────────
	case *ExpressionStatement:
		if err := c.Compile(node.Expression); err != nil {
			return err
		}
		c.emit(OpPop)

	case *PrintStatement:
		if err := c.Compile(node.Value); err != nil {
			return err
		}
		c.emit(OpPrint)

	case *LetStatement:
		if c.isLocal {
			// Allocate local slot first so recursive fn references resolve.
			idx, ok := c.localTable[node.Name.Value]
			if !ok {
				idx = len(c.localTable)
				c.localTable[node.Name.Value] = idx
			}
			if err := c.Compile(node.Value); err != nil {
				return err
			}
			if len(c.constants) > 0 {
				if fn, ok := c.constants[len(c.constants)-1].(*CompiledFunction); ok && fn.Name == "" {
					fn.Name = node.Name.Value
				}
			}
			c.emit(OpSetLocal, idx)
		} else {
			// Allocate global slot first so recursive fn references resolve.
			idx, ok := c.symbolTable[node.Name.Value]
			if !ok {
				idx = len(c.symbolTable)
				c.symbolTable[node.Name.Value] = idx
			}
			if err := c.Compile(node.Value); err != nil {
				return err
			}
			if len(c.constants) > 0 {
				if fn, ok := c.constants[len(c.constants)-1].(*CompiledFunction); ok && fn.Name == "" {
					fn.Name = node.Name.Value
				}
			}
			c.emit(OpSetGlobal, idx)
		}

	case *AssignStatement:
		kind, idx := c.resolveName(node.Name.Value)
		if kind == "" {
			return c.errorf(node.Token.Line, node.Token.Col, "cannot reassign undefined variable '%s'", node.Name.Value)
		}
		if err := c.Compile(node.Value); err != nil {
			return err
		}
		c.emitStoreName(kind, idx)

	case *BreakStatement:
		if len(c.loopStack) == 0 {
			return c.errorf(node.Token.Line, node.Token.Col, "'break' outside of loop")
		}
		jumpPos := c.emit(OpJump, 9999)
		ctx := c.loopStack[len(c.loopStack)-1]
		ctx.breakJumps = append(ctx.breakJumps, jumpPos)

	case *ContinueStatement:
		if len(c.loopStack) == 0 {
			return c.errorf(node.Token.Line, node.Token.Col, "'continue' outside of loop")
		}
		ctx := c.loopStack[len(c.loopStack)-1]
		if ctx.isCountLoop {
			jumpPos := c.emit(OpJump, 9999)
			ctx.continueJumps = append(ctx.continueJumps, jumpPos)
		} else {
			c.emit(OpJump, ctx.continuePos)
		}

	case *AskStatement:
		c.emit(OpConstant, c.addConstant(&String{Value: ""}))
		c.emit(OpAsk)
		if c.isLocal {
			idx, ok := c.localTable[node.Name.Value]
			if !ok {
				idx = len(c.localTable)
				c.localTable[node.Name.Value] = idx
			}
			c.emit(OpSetLocal, idx)
		} else {
			idx, ok := c.symbolTable[node.Name.Value]
			if !ok {
				idx = len(c.symbolTable)
				c.symbolTable[node.Name.Value] = idx
			}
			c.emit(OpSetGlobal, idx)
		}

	case *ReturnStatement:
		if err := c.Compile(node.ReturnValue); err != nil {
			return err
		}
		c.emit(OpReturnValue)

	case *IfStatement:
		return c.compileIf(node.Condition, node.Consequence, node.Alternative)

	case *LoopStatement:
		if node.IsWhile {
			return c.compileWhileLoop(node)
		}
		if node.IsForIn {
			return c.compileForInLoop(node)
		}
		return c.compileCountLoop(node)

	case *IndexAssignStatement:
		if err := c.Compile(node.Value); err != nil {
			return err
		}
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		if err := c.Compile(node.Index); err != nil {
			return err
		}
		c.emit(OpSetIndex)

	case *BlockStatement:
		for _, s := range node.Statements {
			if err := c.Compile(s); err != nil {
				return err
			}
		}

	// â”€â”€ Expressions â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
	case *Identifier:
		return c.compileIdentifier(node)

	case *AskExpression:
		if node.Prompt != nil {
			if err := c.Compile(node.Prompt); err != nil {
				return err
			}
		} else {
			c.emit(OpConstant, c.addConstant(&String{Value: ""}))
		}
		c.emit(OpAsk)

	case *PrefixExpression:
		if err := c.Compile(node.Right); err != nil {
			return err
		}
		switch node.Operator {
		case "!":
			c.emit(OpBang)
		case "-":
			c.emit(OpNegate)
		default:
			return fmt.Errorf("unknown prefix operator '%s'", node.Operator)
		}

	case *HashLiteral:
		for _, pair := range node.Pairs {
			if err := c.Compile(pair.Key); err != nil {
				return err
			}
			if err := c.Compile(pair.Value); err != nil {
				return err
			}
		}
		c.emit(OpHash, len(node.Pairs)*2)

	case *InfixExpression:
		if node.Operator == "&&" {
			if err := c.Compile(node.Left); err != nil {
				return err
			}
			jumpFalsePos := c.emit(OpJumpWithFalse, 9999)
			if err := c.Compile(node.Right); err != nil {
				return err
			}
			jumpEndPos := c.emit(OpJump, 9999)
			c.changeOperand(jumpFalsePos, len(c.instructions))
			c.emit(OpFalse)
			c.changeOperand(jumpEndPos, len(c.instructions))
			return nil
		}
		if node.Operator == "||" {
			if err := c.Compile(node.Left); err != nil {
				return err
			}
			jumpTruePos := c.emit(OpJumpWithTrue, 9999)
			if err := c.Compile(node.Right); err != nil {
				return err
			}
			jumpEndPos := c.emit(OpJump, 9999)
			c.changeOperand(jumpTruePos, len(c.instructions))
			c.emit(OpTrue)
			c.changeOperand(jumpEndPos, len(c.instructions))
			return nil
		}

		if err := c.Compile(node.Left); err != nil {
			return err
		}
		if err := c.Compile(node.Right); err != nil {
			return err
		}
		// Point diagnostics at the operator token
		c.setNodeLoc(node)
		switch node.Operator {
		case "+":
			c.emit(OpAdd)
		case "-":
			c.emit(OpSub)
		case "*":
			c.emit(OpMul)
		case "/":
			c.emit(OpDiv)
		case "%":
			c.emit(OpMod)
		case ">":
			c.emit(OpGreaterThan)
		case "<":
			c.emit(OpLessThan)
		case ">=":
			c.emit(OpGreaterThanOrEqual)
		case "<=":
			c.emit(OpLessThanOrEqual)
		case "==":
			c.emit(OpEqual)
		case "!=":
			c.emit(OpNotEqual)
		default:
			return fmt.Errorf("unknown operator '%s'", node.Operator)
		}

	case *IfExpression:
		return c.compileIfExpression(node)

	case *FunctionLiteral:
		fnC := newFnCompiler(c)
		// Pre-bind parameters as locals 0..n-1
		for i, p := range node.Parameters {
			fnC.localTable[p.Value] = i
		}
		if err := fnC.Compile(node.Body); err != nil {
			return err
		}
		// Implicit null return if body doesn't end with OpReturnValue
		if len(fnC.instructions) == 0 ||
			Opcode(fnC.instructions[len(fnC.instructions)-1]) != OpReturnValue {
			fnC.emit(OpReturn)
		}
		c.constants = fnC.constants // promote child constants
		fnIdx := c.addConstant(&CompiledFunction{
			Instructions: fnC.instructions,
			NumParams:    len(node.Parameters),
			NumFrees:     len(fnC.freeSymbols),
			SourceMap:    fnC.sourceMap,
			Filename:     fnC.filename,
			Name:         node.Name,
			Constants:    fnC.constants,
		})
		// Push current values of free variables, then form a closure.
		for _, name := range fnC.freeSymbols {
			kind, idx := c.resolveName(name)
			if kind == "" {
				return fmt.Errorf("internal: free var '%s' not found in enclosing scope", name)
			}
			c.emitLoadName(kind, idx)
		}
		if len(fnC.freeSymbols) > 0 {
			c.emit(OpClosure, fnIdx, len(fnC.freeSymbols))
		} else {
			// No free vars — still wrap as closure for uniform call path.
			c.emit(OpClosure, fnIdx, 0)
		}

	case *CallExpression:
		if err := c.Compile(node.Function); err != nil {
			return err
		}
		for _, arg := range node.Arguments {
			if err := c.Compile(arg); err != nil {
				return err
			}
		}
		// Point diagnostics at the callee (e.g. abs), not the last argument
		c.setNodeLoc(node.Function)
		c.emit(OpCall, len(node.Arguments))

	case *ArrayLiteral:
		for _, el := range node.Elements {
			if err := c.Compile(el); err != nil {
				return err
			}
		}
		c.emit(OpArray, len(node.Elements))

	case *IndexExpression:
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		if err := c.Compile(node.Index); err != nil {
			return err
		}
		c.emit(OpIndex)

	case *SliceExpression:
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		if node.Start != nil {
			if err := c.Compile(node.Start); err != nil {
				return err
			}
		} else {
			c.emit(OpNull)
		}
		if node.End != nil {
			if err := c.Compile(node.End); err != nil {
				return err
			}
		} else {
			c.emit(OpNull)
		}
		c.setNodeLoc(node)
		c.emit(OpSlice)

	case *IntegerLiteral:
		c.emit(OpConstant, c.addConstant(&Integer{Value: node.Value}))

	case *StringLiteral:
		c.emit(OpConstant, c.addConstant(&String{Value: node.Value}))

	case *Boolean:
		if node.Value {
			c.emit(OpTrue)
		} else {
			c.emit(OpFalse)
		}

	case *CharLiteral:
		// Char literals are single-character strings (no separate CHAR runtime type).
		c.emit(OpConstant, c.addConstant(&String{Value: string(node.Value)}))

	case *FloatLiteral:
		c.emit(OpConstant, c.addConstant(&Float{Value: node.Value}))

	case *NullLiteral:
		c.emit(OpNull)
	}

	return nil
}

// â”€â”€â”€ Identifier resolution â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func (c *Compiler) compileIdentifier(node *Identifier) error {
	// Check builtins first
	if idx, ok := builtinIndex[node.Value]; ok {
		c.emit(OpGetBuiltin, idx)
		return nil
	}
	kind, idx := c.resolveName(node.Value)
	if kind == "" {
		return c.errorf(node.Token.Line, node.Token.Col, "undefined variable '%s'", node.Value)
	}
	c.emitLoadName(kind, idx)
	return nil
}

// â”€â”€â”€ if / else â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func (c *Compiler) compileIf(condition Node, consequence, alternative *BlockStatement) error {
	if err := c.Compile(condition); err != nil {
		return err
	}

	// Emit jump-if-false with placeholder target
	jumpFalsePos := c.emit(OpJumpWithFalse, 9999)

	if err := c.Compile(consequence); err != nil {
		return err
	}

	if alternative != nil {
		// Jump over alternative after consequence
		jumpPos := c.emit(OpJump, 9999)
		c.changeOperand(jumpFalsePos, len(c.instructions))

		if err := c.Compile(alternative); err != nil {
			return err
		}
		c.changeOperand(jumpPos, len(c.instructions))
	} else {
		c.changeOperand(jumpFalsePos, len(c.instructions))
	}
	return nil
}
// compileIfExpression compiles if/else as an expression that leaves a value
// on the stack. The last expression in each branch is kept (not popped);
// empty branches push null.
func (c *Compiler) compileIfExpression(node *IfExpression) error {
	if err := c.Compile(node.Condition); err != nil {
		return err
	}
	jumpFalsePos := c.emit(OpJumpWithFalse, 9999)

	if err := c.compileExpressionBlock(node.Consequence); err != nil {
		return err
	}
	jumpEndPos := c.emit(OpJump, 9999)
	c.changeOperand(jumpFalsePos, len(c.instructions))

	if node.Alternative != nil {
		if err := c.compileExpressionBlock(node.Alternative); err != nil {
			return err
		}
	} else {
		c.emit(OpNull)
	}
	c.changeOperand(jumpEndPos, len(c.instructions))
	return nil
}

// compileExpressionBlock compiles a block used as an expression value.
// Non-final statements are compiled normally (expression stmts are popped).
// The final expression statement's value is left on the stack.
// If the block is empty or ends with a non-expression statement, null is pushed.
func (c *Compiler) compileExpressionBlock(block *BlockStatement) error {
	if block == nil || len(block.Statements) == 0 {
		c.emit(OpNull)
		return nil
	}
	last := len(block.Statements) - 1
	for i, s := range block.Statements {
		if i == last {
			if es, ok := s.(*ExpressionStatement); ok {
				return c.Compile(es.Expression)
			}
			if err := c.Compile(s); err != nil {
				return err
			}
			c.emit(OpNull)
			return nil
		}
		if err := c.Compile(s); err != nil {
			return err
		}
	}
	return nil
}


// â”€â”€â”€ loop while â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func (c *Compiler) compileWhileLoop(node *LoopStatement) error {
	startPos := len(c.instructions)

	if err := c.Compile(node.Condition); err != nil {
		return err
	}
	jumpFalsePos := c.emit(OpJumpWithFalse, 9999)

	ctx := &loopContext{isCountLoop: false, continuePos: startPos, breakJumps: []int{}}
	c.loopStack = append(c.loopStack, ctx)

	if err := c.Compile(node.Body); err != nil {
		return err
	}

	c.loopStack = c.loopStack[:len(c.loopStack)-1]

	c.emit(OpJump, startPos)
	c.changeOperand(jumpFalsePos, len(c.instructions))

	for _, breakJump := range ctx.breakJumps {
		c.changeOperand(breakJump, len(c.instructions))
	}
	return nil
}

// â”€â”€â”€ loop N times â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func (c *Compiler) compileCountLoop(node *LoopStatement) error {
	if err := c.Compile(node.Count); err != nil {
		return err
	}

	var counterIdx int
	if c.isLocal {
		counterKey := fmt.Sprintf("__loop_%d", len(c.localTable))
		counterIdx = len(c.localTable)
		c.localTable[counterKey] = counterIdx
		c.emit(OpSetLocal, counterIdx)
	} else {
		counterKey := fmt.Sprintf("__loop_%d", len(c.symbolTable))
		counterIdx = len(c.symbolTable)
		c.symbolTable[counterKey] = counterIdx
		c.emit(OpSetGlobal, counterIdx)
	}

	startPos := len(c.instructions)

	// while counter > 0
	if c.isLocal {
		c.emit(OpGetLocal, counterIdx)
	} else {
		c.emit(OpGetGlobal, counterIdx)
	}
	c.emit(OpConstant, c.addConstant(&Integer{Value: 0}))
	c.emit(OpGreaterThan)

	jumpFalsePos := c.emit(OpJumpWithFalse, 9999)

	ctx := &loopContext{isCountLoop: true, continueJumps: []int{}, breakJumps: []int{}}
	c.loopStack = append(c.loopStack, ctx)

	if err := c.Compile(node.Body); err != nil {
		return err
	}

	c.loopStack = c.loopStack[:len(c.loopStack)-1]

	decrementPos := len(c.instructions)
	for _, continueJump := range ctx.continueJumps {
		c.changeOperand(continueJump, decrementPos)
	}

	// counter = counter - 1
	if c.isLocal {
		c.emit(OpGetLocal, counterIdx)
		c.emit(OpConstant, c.addConstant(&Integer{Value: 1}))
		c.emit(OpSub)
		c.emit(OpSetLocal, counterIdx)
	} else {
		c.emit(OpGetGlobal, counterIdx)
		c.emit(OpConstant, c.addConstant(&Integer{Value: 1}))
		c.emit(OpSub)
		c.emit(OpSetGlobal, counterIdx)
	}

	c.emit(OpJump, startPos)
	c.changeOperand(jumpFalsePos, len(c.instructions))

	for _, breakJump := range ctx.breakJumps {
		c.changeOperand(breakJump, len(c.instructions))
	}
	return nil
}

// â”€â”€â”€ Helpers â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€


// defineTemp allocates a local or global slot for a synthetic / iterator name.
func (c *Compiler) defineTemp(name string) (kind string, index int) {
	if c.isLocal {
		if idx, ok := c.localTable[name]; ok {
			return "local", idx
		}
		idx := len(c.localTable)
		c.localTable[name] = idx
		return "local", idx
	}
	if idx, ok := c.symbolTable[name]; ok {
		return "global", idx
	}
	idx := len(c.symbolTable)
	c.symbolTable[name] = idx
	return "global", idx
}

// compileForInLoop compiles: loop x in iterable { body }
func (c *Compiler) compileForInLoop(node *LoopStatement) error {
	itemsName := fmt.Sprintf("__items_%d", len(c.instructions))
	idxName := fmt.Sprintf("__i_%d", len(c.instructions))

	// let __items = iterable
	if err := c.Compile(node.Iterable); err != nil {
		return err
	}
	itemsKind, itemsIdx := c.defineTemp(itemsName)
	c.emitStoreName(itemsKind, itemsIdx)

	// let __i = 0
	c.emit(OpConstant, c.addConstant(&Integer{Value: 0}))
	idxKind, idxIdx := c.defineTemp(idxName)
	c.emitStoreName(idxKind, idxIdx)

	// iterator variable slot
	iterKind, iterIdx := c.defineTemp(node.Iterator.Value)

	startPos := len(c.instructions)

	// condition: __i < len(__items)
	c.emitLoadName(idxKind, idxIdx)
	if bi, ok := builtinIndex["len"]; ok {
		c.emit(OpGetBuiltin, bi)
	}
	c.emitLoadName(itemsKind, itemsIdx)
	c.emit(OpCall, 1) // len(__items)
	c.emit(OpLessThan)
	jumpFalsePos := c.emit(OpJumpWithFalse, 9999)

	// x = __items[__i]
	c.emitLoadName(itemsKind, itemsIdx)
	c.emitLoadName(idxKind, idxIdx)
	c.emit(OpIndex)
	c.emitStoreName(iterKind, iterIdx)

	// Use continueJumps like counted loops (continue jumps to increment)
	ctx := &loopContext{isCountLoop: true, continueJumps: []int{}, breakJumps: []int{}}
	c.loopStack = append(c.loopStack, ctx)

	if err := c.Compile(node.Body); err != nil {
		return err
	}

	c.loopStack = c.loopStack[:len(c.loopStack)-1]

	incPos := len(c.instructions)
	for _, j := range ctx.continueJumps {
		c.changeOperand(j, incPos)
	}

	// __i = __i + 1
	c.emitLoadName(idxKind, idxIdx)
	c.emit(OpConstant, c.addConstant(&Integer{Value: 1}))
	c.emit(OpAdd)
	c.emitStoreName(idxKind, idxIdx)

	c.emit(OpJump, startPos)
	c.changeOperand(jumpFalsePos, len(c.instructions))
	for _, j := range ctx.breakJumps {
		c.changeOperand(j, len(c.instructions))
	}
	return nil
}

func (c *Compiler) addConstant(obj Object) int {
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1
}

func (c *Compiler) emit(op Opcode, operands ...int) int {
	ins := Make(op, operands...)
	pos := len(c.instructions)
	c.instructions = append(c.instructions, ins...)
	// Record source location for every byte of this instruction
	for len(c.sourceMap) < len(c.instructions) {
		c.sourceMap = append(c.sourceMap, c.currentLoc)
	}
	return pos
}

func (c *Compiler) changeOperand(opPos int, operand int) {
	op := Opcode(c.instructions[opPos])
	copy(c.instructions[opPos:], Make(op, operand))
}

func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{
		Instructions: c.instructions,
		Constants:    c.constants,
		SourceMap:    c.sourceMap,
		Source:       c.source,
		Filename:     c.filename,
	}
}

type Bytecode struct {
	Instructions []byte
	Constants    []Object
	SourceMap    []SourceLocation
	Source       string
	Filename     string
}
