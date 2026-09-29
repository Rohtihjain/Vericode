package main

import (
	"bytes"
	"strings"
)

// ─── Interfaces ───────────────────────────────────────────────────────────────

type Node interface {
	TokenLiteral() string
	String() string
}

type Statement interface {
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
}

// ─── Program (root) ───────────────────────────────────────────────────────────

type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

func (p *Program) String() string {
	var out bytes.Buffer
	for _, s := range p.Statements {
		out.WriteString(s.String())
	}
	return out.String()
}

// ─── Statements ───────────────────────────────────────────────────────────────

// let x = <expr>;
type LetStatement struct {
	Token Token // the LET token
	Name  *Identifier
	Value Expression
}

func (ls *LetStatement) statementNode()       {}
func (ls *LetStatement) TokenLiteral() string { return ls.Token.Literal }
func (ls *LetStatement) String() string {
	var out bytes.Buffer
	out.WriteString("let " + ls.Name.String() + " = " + ls.Value.String() + ";")
	return out.String()
}

// x = <expr>;  or  x += <expr>;  etc.
type AssignStatement struct {
	Token    Token // the IDENT token
	Name     *Identifier
	Operator string // "=" | "+=" | "-=" | "*=" | "/=" | "%="
	Value    Expression
}

func (as *AssignStatement) statementNode()       {}
func (as *AssignStatement) TokenLiteral() string { return as.Token.Literal }
func (as *AssignStatement) String() string {
	op := as.Operator
	if op == "" {
		op = "="
	}
	return as.Name.String() + " " + op + " " + as.Value.String() + ";"
}


// <left>[<index>] = <value>;
type IndexAssignStatement struct {
	Token Token // the IDENT or '[' related token
	Left  Expression
	Index Expression
	Value Expression
}

func (ias *IndexAssignStatement) statementNode()       {}
func (ias *IndexAssignStatement) TokenLiteral() string { return ias.Token.Literal }
func (ias *IndexAssignStatement) String() string {
	return ias.Left.String() + "[" + ias.Index.String() + "] = " + ias.Value.String() + ";"
}

// break;
type BreakStatement struct {
	Token Token // the BREAK token
}

func (bs *BreakStatement) statementNode()       {}
func (bs *BreakStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BreakStatement) String() string       { return "break;" }

// continue;
type ContinueStatement struct {
	Token Token // the CONTINUE token
}

func (cs *ContinueStatement) statementNode()       {}
func (cs *ContinueStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ContinueStatement) String() string       { return "continue;" }

// print <expr>;
type PrintStatement struct {
	Token Token // the PRINT token
	Value Expression
}

func (ps *PrintStatement) statementNode()       {}
func (ps *PrintStatement) TokenLiteral() string { return ps.Token.Literal }
func (ps *PrintStatement) String() string {
	return "print " + ps.Value.String() + ";"
}

// ask <ident>;
type AskStatement struct {
	Token Token // the ASK token
	Name  *Identifier
}

func (as *AskStatement) statementNode()       {}
func (as *AskStatement) TokenLiteral() string { return as.Token.Literal }
func (as *AskStatement) String() string       { return "ask " + as.Name.String() + ";" }

// return <expr>;
type ReturnStatement struct {
	Token       Token // the RETURN token
	ReturnValue Expression
}

func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *ReturnStatement) String() string {
	return "return " + rs.ReturnValue.String() + ";"
}

// { <statements> }
type BlockStatement struct {
	Token      Token // the { token
	Statements []Statement
}

func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) String() string {
	var out bytes.Buffer
	for _, s := range bs.Statements {
		out.WriteString(s.String())
	}
	return out.String()
}

// if <condition> { <consequence> } else { <alternative> }
type IfStatement struct {
	Token       Token // the IF token
	Condition   Expression
	Consequence *BlockStatement
	Alternative *BlockStatement // nil if no else branch
}

func (is *IfStatement) statementNode()       {}
func (is *IfStatement) TokenLiteral() string { return is.Token.Literal }
func (is *IfStatement) String() string {
	var out bytes.Buffer
	out.WriteString("if " + is.Condition.String() + " " + is.Consequence.String())
	if is.Alternative != nil {
		out.WriteString(" else " + is.Alternative.String())
	}
	return out.String()
}

// loop while <condition> { <body> }
// loop <count> times  { <body> }
// loop <name> in <iterable> { <body> }
type LoopStatement struct {
	Token     Token // the LOOP token
	IsWhile   bool
	IsForIn   bool
	Condition Expression // used when IsWhile = true
	Count     Expression // used when counted loop
	Iterator  *Identifier // used when IsForIn = true
	Iterable  Expression  // used when IsForIn = true
	Body      *BlockStatement
}

func (ls *LoopStatement) statementNode()       {}
func (ls *LoopStatement) TokenLiteral() string { return ls.Token.Literal }
func (ls *LoopStatement) String() string {
	var out bytes.Buffer
	if ls.IsWhile {
		out.WriteString("loop while " + ls.Condition.String())
	} else if ls.IsForIn {
		out.WriteString("loop " + ls.Iterator.String() + " in " + ls.Iterable.String())
	} else {
		out.WriteString("loop " + ls.Count.String() + " times")
	}
	out.WriteString(" " + ls.Body.String())
	return out.String()
}

// <expression>;
type ExpressionStatement struct {
	Token      Token // the first token of the expression
	Expression Expression
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExpressionStatement) String() string       { return es.Expression.String() }

// ─── Expressions ──────────────────────────────────────────────────────────────

type Identifier struct {
	Token Token // the IDENT token
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) String() string       { return i.Value }

type IntegerLiteral struct {
	Token Token
	Value int64
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) String() string       { return il.Token.Literal }

type FloatLiteral struct {
	Token Token
	Value float64
}

func (fl *FloatLiteral) expressionNode()      {}
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FloatLiteral) String() string       { return fl.Token.Literal }

type NullLiteral struct {
	Token Token
}

func (nl *NullLiteral) expressionNode()      {}
func (nl *NullLiteral) TokenLiteral() string { return nl.Token.Literal }
func (nl *NullLiteral) String() string       { return "null" }


type Boolean struct {
	Token Token
	Value bool
}

func (b *Boolean) expressionNode()      {}
func (b *Boolean) TokenLiteral() string { return b.Token.Literal }
func (b *Boolean) String() string       { return b.Token.Literal }

type StringLiteral struct {
	Token Token
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) String() string       { return `"` + sl.Value + `"` }

type CharLiteral struct {
	Token Token
	Value rune
}

func (cl *CharLiteral) expressionNode()      {}
func (cl *CharLiteral) TokenLiteral() string { return cl.Token.Literal }
func (cl *CharLiteral) String() string       { return "'" + string(cl.Value) + "'" }

// <left> <operator> <right>
type InfixExpression struct {
	Token    Token // the operator token
	Left     Expression
	Operator string
	Right    Expression
}

func (ie *InfixExpression) expressionNode()      {}
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *InfixExpression) String() string {
	return "(" + ie.Left.String() + " " + ie.Operator + " " + ie.Right.String() + ")"
}

// func(<params>) { <body> }  — Name is set for top-level named functions
type FunctionLiteral struct {
	Token      Token // the FUNC token
	Name       string
	Parameters []*Identifier
	Body       *BlockStatement
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FunctionLiteral) String() string {
	var out bytes.Buffer
	params := []string{}
	for _, p := range fl.Parameters {
		params = append(params, p.String())
	}
	if fl.Name != "" {
		out.WriteString("func " + fl.Name + "(" + strings.Join(params, ", ") + ") " + fl.Body.String())
	} else {
		out.WriteString("func(" + strings.Join(params, ", ") + ") " + fl.Body.String())
	}
	return out.String()
}

// <function>(<arguments>)
type CallExpression struct {
	Token     Token // the ( token
	Function  Expression
	Arguments []Expression
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) String() string {
	var out bytes.Buffer
	args := []string{}
	for _, a := range ce.Arguments {
		args = append(args, a.String())
	}
	out.WriteString(ce.Function.String() + "(" + strings.Join(args, ", ") + ")")
	return out.String()
}

// if used as an expression: if <cond> { <consequence> } else { <alternative> }
type IfExpression struct {
	Token       Token // the IF token
	Condition   Expression
	Consequence *BlockStatement
	Alternative *BlockStatement
}

func (ie *IfExpression) expressionNode()      {}
func (ie *IfExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IfExpression) String() string {
	var out bytes.Buffer
	out.WriteString("if " + ie.Condition.String() + " " + ie.Consequence.String())
	if ie.Alternative != nil {
		out.WriteString(" else " + ie.Alternative.String())
	}
	return out.String()
}

// [<elements>]
type ArrayLiteral struct {
	Token    Token // the [ token
	Elements []Expression
}

func (al *ArrayLiteral) expressionNode()      {}
func (al *ArrayLiteral) TokenLiteral() string { return al.Token.Literal }
func (al *ArrayLiteral) String() string {
	var out bytes.Buffer
	elems := []string{}
	for _, e := range al.Elements {
		elems = append(elems, e.String())
	}
	out.WriteString("[" + strings.Join(elems, ", ") + "]")
	return out.String()
}

// <left>[<index>]
type IndexExpression struct {
	Token Token // the [ token
	Left  Expression
	Index Expression
}

func (ie *IndexExpression) expressionNode()      {}
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IndexExpression) String() string {
	return "(" + ie.Left.String() + "[" + ie.Index.String() + "])"
}

// <left>[<start>:<end>]  — start/end may be nil (open bound)
type SliceExpression struct {
	Token Token // the [ token
	Left  Expression
	Start Expression // nil means from beginning
	End   Expression // nil means to end
}

func (se *SliceExpression) expressionNode()      {}
func (se *SliceExpression) TokenLiteral() string { return se.Token.Literal }
func (se *SliceExpression) String() string {
	s, e := "", ""
	if se.Start != nil {
		s = se.Start.String()
	}
	if se.End != nil {
		e = se.End.String()
	}
	return "(" + se.Left.String() + "[" + s + ":" + e + "])"
}

// <operator><right> (e.g. !true, -5)
type PrefixExpression struct {
	Token    Token // the prefix operator token, e.g. !
	Operator string
	Right    Expression
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PrefixExpression) String() string {
	return "(" + pe.Operator + pe.Right.String() + ")"
}

// { <key>: <value>, ... }
type HashPair struct {
	Key   Expression
	Value Expression
}

type HashLiteral struct {
	Token Token // the { token
	Pairs []HashPair
}

func (hl *HashLiteral) expressionNode()      {}
func (hl *HashLiteral) TokenLiteral() string { return hl.Token.Literal }
func (hl *HashLiteral) String() string {
	var out bytes.Buffer
	pairs := []string{}
	for _, p := range hl.Pairs {
		pairs = append(pairs, p.Key.String()+": "+p.Value.String())
	}
	out.WriteString("{" + strings.Join(pairs, ", ") + "}")
	return out.String()
}

// ask <prompt> (or ask)
type AskExpression struct {
	Token  Token // the ASK token
	Prompt Expression
}

func (ae *AskExpression) expressionNode()      {}
func (ae *AskExpression) TokenLiteral() string { return ae.Token.Literal }
func (ae *AskExpression) String() string {
	if ae.Prompt != nil {
		return "ask(" + ae.Prompt.String() + ")"
	}
	return "ask"
}

// spawn <call> — starts the call in a new lightweight task and evaluates to
// a task handle, e.g. spawn worker(ch, 10)  or  spawn producer
type SpawnExpression struct {
	Token Token // the SPAWN token
	Call  *CallExpression
}

func (se *SpawnExpression) expressionNode()      {}
func (se *SpawnExpression) TokenLiteral() string { return se.Token.Literal }
func (se *SpawnExpression) String() string       { return "spawn " + se.Call.String() }

// await <task> — blocks until the task finishes and evaluates to its result
// (or raises its error), e.g. await t  or  await(worker(ch))
type AwaitExpression struct {
	Token Token // the AWAIT token
	Task  Expression
}

func (ae *AwaitExpression) expressionNode()      {}
func (ae *AwaitExpression) TokenLiteral() string { return ae.Token.Literal }
func (ae *AwaitExpression) String() string       { return "await " + ae.Task.String() }
