package main

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	_ int = iota
	LOWEST
	PIPE_PREC   // |>
	LOGICAL_OR  // ||
	LOGICAL_AND // &&
	EQUALS      // == !=
	LESSGREATER // < > <= >=
	SUM         // + -
	PRODUCT     // * /
	PREFIX      // ! -
	CALL        // ()
	INDEX       // []
)

var precedences = map[TokenType]int{
	PIPE:     PIPE_PREC,
	OR:       LOGICAL_OR,
	AND:      LOGICAL_AND,
	EQ:       EQUALS,
	NOT_EQ:   EQUALS,
	LT:       LESSGREATER,
	GT:       LESSGREATER,
	LT_EQ:   LESSGREATER,
	GT_EQ:   LESSGREATER,
	PLUS:     SUM,
	MINUS:    SUM,
	SLASH:    PRODUCT,
	ASTERISK: PRODUCT,
	PERCENT:  PRODUCT,
	LPAREN:   CALL,
	LBRACKET: INDEX,
}

type (
	prefixParseFn func() Expression
	infixParseFn  func(Expression) Expression
)

type Parser struct {
	l         *Lexer
	curToken  Token
	peekToken Token
	errors    []string
	source    string
	filename  string

	prefixParseFns map[TokenType]prefixParseFn
	infixParseFns  map[TokenType]infixParseFn
}

func NewParser(l *Lexer) *Parser {
	p := &Parser{l: l, errors: []string{}, source: l.input, filename: "<input>"}

	p.prefixParseFns = make(map[TokenType]prefixParseFn)
	p.registerPrefix(IDENT, p.parseIdentifier)
	p.registerPrefix(INT, p.parseIntegerLiteral)
	p.registerPrefix(FLOAT, p.parseFloatLiteral)
	p.registerPrefix(STRING, p.parseStringLiteral)
	p.registerPrefix(NULL, p.parseNullLiteral)
	p.registerPrefix(CHAR, p.parseCharLiteral)
	p.registerPrefix(TRUE, p.parseBoolean)
	p.registerPrefix(FALSE, p.parseBoolean)
	p.registerPrefix(IF, p.parseIfExpression)
	p.registerPrefix(FUNC, p.parseFunctionLiteral)
	p.registerPrefix(LPAREN, p.parseGroupedExpression)
	p.registerPrefix(LBRACKET, p.parseArrayLiteral)
	p.registerPrefix(LBRACE, p.parseHashLiteral)
	p.registerPrefix(BANG, p.parsePrefixExpression)
	p.registerPrefix(MINUS, p.parsePrefixExpression)
	p.registerPrefix(ASK, p.parseAskExpression)
	p.registerPrefix(SPAWN, p.parseSpawnExpression)
	p.registerPrefix(AWAIT, p.parseAwaitExpression)

	p.infixParseFns = make(map[TokenType]infixParseFn)
	p.registerInfix(PLUS, p.parseInfixExpression)
	p.registerInfix(MINUS, p.parseInfixExpression)
	p.registerInfix(SLASH, p.parseInfixExpression)
	p.registerInfix(ASTERISK, p.parseInfixExpression)
	p.registerInfix(EQ, p.parseInfixExpression)
	p.registerInfix(NOT_EQ, p.parseInfixExpression)
	p.registerInfix(LT, p.parseInfixExpression)
	p.registerInfix(GT, p.parseInfixExpression)
	p.registerInfix(LT_EQ, p.parseInfixExpression)
	p.registerInfix(GT_EQ, p.parseInfixExpression)
	p.registerInfix(AND, p.parseInfixExpression)
	p.registerInfix(OR, p.parseInfixExpression)
	p.registerInfix(LPAREN, p.parseCallExpression)
	p.registerInfix(LBRACKET, p.parseIndexExpression)
	p.registerInfix(PIPE, p.parsePipeExpression)
	p.registerInfix(PERCENT, p.parseInfixExpression)

	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) registerPrefix(t TokenType, fn prefixParseFn) { p.prefixParseFns[t] = fn }
func (p *Parser) registerInfix(t TokenType, fn infixParseFn)   { p.infixParseFns[t] = fn }

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) Errors() []string { return p.errors }

func (p *Parser) addError(tok Token, msg string) {
	p.errors = append(p.errors,
		FormatDiagnostic(p.filename, p.source, "ParseError", msg, tok.Line, tok.Col))
}

func (p *Parser) ParseProgram() *Program {
	program := &Program{Statements: []Statement{}}
	for p.curToken.Type != EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}
	return program
}

func (p *Parser) parseStatement() Statement {
	// x = expr  |  x += expr  |  ...
	if p.curTokenIs(IDENT) && p.isAssignOp(p.peekToken.Type) {
		return p.parseAssignStatement()
	}
	// a[i] = expr
	if p.curTokenIs(IDENT) && p.peekTokenIs(LBRACKET) {
		return p.parseIndexAssignOrExpression()
	}

	switch p.curToken.Type {
	case LET:
		return p.parseLetStatement()
	case FUNC:
		// Named top-level function: func add(a, b) { ... }
		// Anonymous func as expression statement still works via default path
		if p.peekTokenIs(IDENT) {
			return p.parseNamedFunctionStatement()
		}
		return p.parseExpressionStatement()
	case ASK:
		return p.parseAskStatement()
	case PRINT:
		return p.parsePrintStatement()
	case RETURN:
		return p.parseReturnStatement()
	case LOOP:
		return p.parseLoopStatement()
	case IF:
		return p.parseIfStatement()
	case BREAK:
		return p.parseBreakStatement()
	case CONTINUE:
		return p.parseContinueStatement()
	default:
		return p.parseExpressionStatement()
	}
}

func (p *Parser) isAssignOp(t TokenType) bool {
	return t == ASSIGN || t == PLUS_EQ || t == MINUS_EQ || t == STAR_EQ || t == SLASH_EQ || t == PERCENT_EQ
}

func (p *Parser) parseAssignStatement() *AssignStatement {
	stmt := &AssignStatement{Token: p.curToken}
	stmt.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}
	p.nextToken() // move to assign op
	stmt.Operator = p.curToken.Literal
	p.nextToken() // move to expression
	rhs := p.parseExpression(LOWEST)
	if stmt.Operator != "=" {
		// Expand x += y  into  x = (x + y)
		op := stmt.Operator[:len(stmt.Operator)-1] // "+=" -> "+"
		rhs = &InfixExpression{
			Token:    stmt.Token,
			Left:     &Identifier{Token: stmt.Token, Value: stmt.Name.Value},
			Operator: op,
			Right:    rhs,
		}
		stmt.Operator = "="
	}
	stmt.Value = rhs
	if p.peekTokenIs(SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

// parseIndexAssignOrExpression handles "a[i] = v;" or falls back to expression statement.
func (p *Parser) parseIndexAssignOrExpression() Statement {
	startTok := p.curToken
	left := p.parseExpression(LOWEST)
	if p.peekTokenIs(ASSIGN) {
		idxExp, ok := left.(*IndexExpression)
		if !ok {
			p.addError(p.curToken, "invalid assignment target")
			return nil
		}
		p.nextToken() // =
		p.nextToken() // value
		val := p.parseExpression(LOWEST)
		if p.peekTokenIs(SEMICOLON) {
			p.nextToken()
		}
		return &IndexAssignStatement{
			Token: startTok,
			Left:  idxExp.Left,
			Index: idxExp.Index,
			Value: val,
		}
	}
	// Not an assignment — treat as expression statement
	stmt := &ExpressionStatement{Token: startTok, Expression: left}
	if p.peekTokenIs(SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseBreakStatement() *BreakStatement {
	stmt := &BreakStatement{Token: p.curToken}
	if p.peekTokenIs(SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseContinueStatement() *ContinueStatement {
	stmt := &ContinueStatement{Token: p.curToken}
	if p.peekTokenIs(SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseLetStatement() *LetStatement {
	stmt := &LetStatement{Token: p.curToken}
	if !p.expectPeek(IDENT) {
		return nil
	}
	stmt.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if !p.expectPeek(ASSIGN) {
		return nil
	}
	p.nextToken()
	if p.curTokenIs(SEMICOLON) || p.curTokenIs(EOF) {
		p.addError(p.curToken, "expected expression after '=' in let statement")
		return nil
	}
	stmt.Value = p.parseExpression(LOWEST)
	if stmt.Value == nil {
		return nil
	}
	if p.peekTokenIs(SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseAskStatement() *AskStatement {
	stmt := &AskStatement{Token: p.curToken}
	if !p.expectPeek(IDENT) {
		return nil
	}
	stmt.Name = &Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if p.peekTokenIs(SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parsePrintStatement() *PrintStatement {
	stmt := &PrintStatement{Token: p.curToken}
	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)
	if p.peekTokenIs(SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseReturnStatement() *ReturnStatement {
	stmt := &ReturnStatement{Token: p.curToken}
	p.nextToken()
	stmt.ReturnValue = p.parseExpression(LOWEST)
	if p.peekTokenIs(SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseIfStatement() *IfStatement {
	stmt := &IfStatement{Token: p.curToken}
	p.nextToken()
	stmt.Condition = p.parseExpression(LOWEST)
	if !p.expectPeek(LBRACE) {
		return nil
	}
	stmt.Consequence = p.parseBlockStatement()
	if p.peekTokenIs(ELSE) {
		p.nextToken()
		// else if <cond> { } — sugar: wrap in a block with a single IfStatement
		if p.peekTokenIs(IF) {
			p.nextToken()
			inner := p.parseIfStatement()
			if inner == nil {
				return nil
			}
			// Wrap the inner IfStatement in a BlockStatement so Alternative is *BlockStatement
			stmt.Alternative = &BlockStatement{
				Token:      inner.Token,
				Statements: []Statement{inner},
			}
		} else {
			if !p.expectPeek(LBRACE) {
				return nil
			}
			stmt.Alternative = p.parseBlockStatement()
		}
	}
	return stmt
}

func (p *Parser) parseLoopStatement() *LoopStatement {
	stmt := &LoopStatement{Token: p.curToken}
	if p.peekTokenIs(WHILE) {
		p.nextToken()
		p.nextToken()
		stmt.IsWhile = true
		stmt.Condition = p.parseExpression(LOWEST)
	} else if p.peekTokenIs(IDENT) {
		// Could be: loop n times { }  OR  loop x in arr { }
		p.nextToken() // IDENT
		if p.peekTokenIs(IN) {
			stmt.IsForIn = true
			stmt.Iterator = &Identifier{Token: p.curToken, Value: p.curToken.Literal}
			p.nextToken() // IN
			p.nextToken() // start of iterable expr
			stmt.Iterable = p.parseExpression(LOWEST)
		} else {
			// counted loop: loop <expr> times
			stmt.IsWhile = false
			stmt.Count = p.parseExpression(LOWEST)
			if p.peekTokenIs(TIMES) {
				p.nextToken()
			}
		}
	} else {
		p.nextToken()
		stmt.IsWhile = false
		stmt.Count = p.parseExpression(LOWEST)
		if p.peekTokenIs(TIMES) {
			p.nextToken()
		}
	}
	if !p.expectPeek(LBRACE) {
		return nil
	}
	stmt.Body = p.parseBlockStatement()
	return stmt
}

func (p *Parser) parseBlockStatement() *BlockStatement {
	block := &BlockStatement{Token: p.curToken, Statements: []Statement{}}
	p.nextToken()
	for !p.curTokenIs(RBRACE) && !p.curTokenIs(EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}
	return block
}

func (p *Parser) parseExpressionStatement() *ExpressionStatement {
	stmt := &ExpressionStatement{Token: p.curToken}
	stmt.Expression = p.parseExpression(LOWEST)
	if p.peekTokenIs(SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseExpression(precedence int) Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		if p.curToken.Type == EOF || p.curToken.Literal == "" {
			p.addError(p.curToken, "unexpected end of input")
		} else {
			p.addError(p.curToken, fmt.Sprintf("unexpected token '%s'", p.curToken.Literal))
		}
		return nil
	}
	leftExp := prefix()
	for !p.peekTokenIs(SEMICOLON) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}
		p.nextToken()
		leftExp = infix(leftExp)
	}
	return leftExp
}

func (p *Parser) parseIdentifier() Expression {
	return &Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseIntegerLiteral() Expression {
	lit := &IntegerLiteral{Token: p.curToken}
	val, err := strconv.ParseInt(p.curToken.Literal, 0, 64)
	if err != nil {
		p.addError(p.curToken, fmt.Sprintf("'%s' is not a valid integer", p.curToken.Literal))
		return nil
	}
	lit.Value = val
	return lit
}

func (p *Parser) parseFloatLiteral() Expression {
	lit := &FloatLiteral{Token: p.curToken}
	val, err := strconv.ParseFloat(p.curToken.Literal, 64)
	if err != nil {
		p.addError(p.curToken, fmt.Sprintf("'%s' is not a valid float", p.curToken.Literal))
		return nil
	}
	lit.Value = val
	return lit
}

func (p *Parser) parseNullLiteral() Expression {
	return &NullLiteral{Token: p.curToken}
}

func (p *Parser) parseStringLiteral() Expression {
	tok := p.curToken
	value := tok.Literal
	if !strings.Contains(value, "${") {
		return &StringLiteral{Token: tok, Value: value}
	}
	return p.desugarInterpolation(tok, value)
}

// desugarInterpolation turns "Hello ${name}!" into
// ("Hello " + str(name)) + "!"
func (p *Parser) desugarInterpolation(tok Token, value string) Expression {
	parts, errMsg := splitInterpolation(value)
	if errMsg != "" {
		p.addError(tok, errMsg)
		return &StringLiteral{Token: tok, Value: value}
	}
	if len(parts) == 0 {
		return &StringLiteral{Token: tok, Value: ""}
	}

	var result Expression
	for _, part := range parts {
		var expr Expression
		if part.isExpr {
			expr = p.parseInterpolationExpr(tok, part.text)
			if expr == nil {
				expr = &StringLiteral{Token: tok, Value: ""}
			} else {
				// Wrap non-string expressions in str() so + always works
				expr = &CallExpression{
					Token:     tok,
					Function:  &Identifier{Token: tok, Value: "str"},
					Arguments: []Expression{expr},
				}
			}
		} else {
			expr = &StringLiteral{Token: tok, Value: part.text}
		}
		if result == nil {
			result = expr
		} else {
			result = &InfixExpression{
				Token:    tok,
				Left:     result,
				Operator: "+",
				Right:    expr,
			}
		}
	}
	return result
}

type interpPart struct {
	text   string
	isExpr bool
}

// splitInterpolation splits a string into literal and ${expr} segments.
func splitInterpolation(s string) ([]interpPart, string) {
	var parts []interpPart
	i := 0
	for i < len(s) {
		start := strings.Index(s[i:], "${")
		if start < 0 {
			parts = append(parts, interpPart{text: s[i:], isExpr: false})
			break
		}
		start += i
		if start > i {
			parts = append(parts, interpPart{text: s[i:start], isExpr: false})
		}
		// find matching }
		depth := 1
		j := start + 2
		for j < len(s) && depth > 0 {
			if s[j] == '{' {
				depth++
			} else if s[j] == '}' {
				depth--
			}
			j++
		}
		if depth != 0 {
			return nil, "unclosed interpolation ${...} in string"
		}
		inner := s[start+2 : j-1]
		if strings.TrimSpace(inner) == "" {
			return nil, "empty interpolation ${} in string"
		}
		parts = append(parts, interpPart{text: inner, isExpr: true})
		i = j
	}
	return parts, ""
}

// parseInterpolationExpr parses an expression from a ${...} fragment.
func (p *Parser) parseInterpolationExpr(tok Token, src string) Expression {
	lex := NewLexer(src)
	sub := NewParser(lex)
	sub.filename = p.filename
	sub.source = p.source
	expr := sub.parseExpression(LOWEST)
	if len(sub.errors) > 0 {
		for _, e := range sub.errors {
			p.errors = append(p.errors, e)
		}
		return nil
	}
	// leftover non-EOF tokens mean incomplete parse
	if sub.peekToken.Type != EOF && sub.curToken.Type != EOF {
		// advance: after parseExpression, cur is last token of expr, peek is next
		if sub.peekToken.Type != EOF {
			p.addError(tok, fmt.Sprintf("unexpected token in interpolation: '%s'", sub.peekToken.Literal))
			return nil
		}
	}
	return expr
}

func (p *Parser) parseCharLiteral() Expression {
	var r rune
	if len(p.curToken.Literal) > 0 {
		r = []rune(p.curToken.Literal)[0]
	}
	return &CharLiteral{Token: p.curToken, Value: r}
}

func (p *Parser) parseBoolean() Expression {
	return &Boolean{Token: p.curToken, Value: p.curTokenIs(TRUE)}
}

func (p *Parser) parseGroupedExpression() Expression {
	p.nextToken()
	exp := p.parseExpression(LOWEST)
	if !p.expectPeek(RPAREN) {
		return nil
	}
	return exp
}

func (p *Parser) parseIfExpression() Expression {
	expr := &IfExpression{Token: p.curToken}
	p.nextToken()
	expr.Condition = p.parseExpression(LOWEST)
	if !p.expectPeek(LBRACE) {
		return nil
	}
	expr.Consequence = p.parseBlockStatement()
	if p.peekTokenIs(ELSE) {
		p.nextToken()
		if p.peekTokenIs(IF) {
			p.nextToken()
			inner := p.parseIfExpression()
			if inner == nil {
				return nil
			}
			expr.Alternative = &BlockStatement{
				Token:      p.curToken,
				Statements: []Statement{&ExpressionStatement{Token: p.curToken, Expression: inner}},
			}
		} else {
			if !p.expectPeek(LBRACE) {
				return nil
			}
			expr.Alternative = p.parseBlockStatement()
		}
	}
	return expr
}

func (p *Parser) parseFunctionLiteral() Expression {
	lit := &FunctionLiteral{Token: p.curToken}
	if !p.expectPeek(LPAREN) {
		return nil
	}
	lit.Parameters = p.parseFunctionParameters()
	if !p.expectPeek(LBRACE) {
		return nil
	}
	lit.Body = p.parseBlockStatement()
	return lit
}

func (p *Parser) parseFunctionParameters() []*Identifier {
	identifiers := []*Identifier{}
	if p.peekTokenIs(RPAREN) {
		p.nextToken()
		return identifiers
	}
	p.nextToken()
	identifiers = append(identifiers, &Identifier{Token: p.curToken, Value: p.curToken.Literal})
	for p.peekTokenIs(COMMA) {
		p.nextToken()
		p.nextToken()
		identifiers = append(identifiers, &Identifier{Token: p.curToken, Value: p.curToken.Literal})
	}
	if !p.expectPeek(RPAREN) {
		return nil
	}
	return identifiers
}

func (p *Parser) parseArrayLiteral() Expression {
	array := &ArrayLiteral{Token: p.curToken}
	array.Elements = p.parseExpressionList(RBRACKET)
	return array
}

func (p *Parser) parseInfixExpression(left Expression) Expression {
	expr := &InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}
	precedence := p.curPrecedence()
	p.nextToken()
	expr.Right = p.parseExpression(precedence)
	return expr
}

func (p *Parser) parseCallExpression(function Expression) Expression {
	exp := &CallExpression{Token: p.curToken, Function: function}
	exp.Arguments = p.parseExpressionList(RPAREN)
	return exp
}

func (p *Parser) parseIndexExpression(left Expression) Expression {
	tok := p.curToken // [
	p.nextToken()

	// arr[:] or arr[:end]
	if p.curTokenIs(COLON) {
		slice := &SliceExpression{Token: tok, Left: left, Start: nil}
		p.nextToken()
		if !p.curTokenIs(RBRACKET) {
			slice.End = p.parseExpression(LOWEST)
			if !p.expectPeek(RBRACKET) {
				return nil
			}
		}
		return slice
	}

	// arr[expr] or arr[start:end] or arr[start:]
	first := p.parseExpression(LOWEST)
	if p.peekTokenIs(COLON) {
		p.nextToken() // :
		slice := &SliceExpression{Token: tok, Left: left, Start: first}
		p.nextToken()
		if !p.curTokenIs(RBRACKET) {
			slice.End = p.parseExpression(LOWEST)
			if !p.expectPeek(RBRACKET) {
				return nil
			}
		}
		return slice
	}

	if !p.expectPeek(RBRACKET) {
		return nil
	}
	return &IndexExpression{Token: tok, Left: left, Index: first}
}

// parseNamedFunctionStatement parses: func name(params) { body }
// Desugars to: let name = func(params) { body };
func (p *Parser) parseNamedFunctionStatement() Statement {
	funcTok := p.curToken
	if !p.expectPeek(IDENT) {
		return nil
	}
	name := &Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if !p.expectPeek(LPAREN) {
		return nil
	}
	params := p.parseFunctionParameters()
	if !p.expectPeek(LBRACE) {
		return nil
	}
	body := p.parseBlockStatement()
	lit := &FunctionLiteral{
		Token:      funcTok,
		Name:       name.Value,
		Parameters: params,
		Body:       body,
	}
	return &LetStatement{
		Token: funcTok,
		Name:  name,
		Value: lit,
	}
}

func (p *Parser) parsePipeExpression(left Expression) Expression {
	tok := p.curToken
	p.nextToken()
	right := p.parseExpression(CALL - 1)
	if right == nil {
		return nil
	}
	if call, ok := right.(*CallExpression); ok {
		call.Arguments = append([]Expression{left}, call.Arguments...)
		return call
	}
	return &CallExpression{
		Token:     tok,
		Function:  right,
		Arguments: []Expression{left},
	}
}

func (p *Parser) parsePrefixExpression() Expression {
	expr := &PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}
	p.nextToken()
	expr.Right = p.parseExpression(PREFIX)
	return expr
}

func (p *Parser) parseHashLiteral() Expression {
	hash := &HashLiteral{Token: p.curToken, Pairs: []HashPair{}}
	for !p.peekTokenIs(RBRACE) && !p.peekTokenIs(EOF) {
		p.nextToken()
		key := p.parseExpression(LOWEST)
		if !p.expectPeek(COLON) {
			return nil
		}
		p.nextToken()
		val := p.parseExpression(LOWEST)
		hash.Pairs = append(hash.Pairs, HashPair{Key: key, Value: val})
		if !p.peekTokenIs(RBRACE) && !p.expectPeek(COMMA) {
			return nil
		}
	}
	if !p.expectPeek(RBRACE) {
		return nil
	}
	return hash
}

func (p *Parser) parseAskExpression() Expression {
	expr := &AskExpression{Token: p.curToken}
	if !p.peekTokenIs(SEMICOLON) && !p.peekTokenIs(RPAREN) && !p.peekTokenIs(COMMA) && !p.peekTokenIs(RBRACE) && !p.peekTokenIs(RBRACKET) && !p.peekTokenIs(EOF) {
		p.nextToken()
		expr.Prompt = p.parseExpression(LOWEST)
	}
	return expr
}

// parseSpawnExpression parses: spawn f(args)  or  spawn f
// The argument expressions are evaluated eagerly at spawn time; the call
// itself runs in a new task.
func (p *Parser) parseSpawnExpression() Expression {
	expr := &SpawnExpression{Token: p.curToken}
	p.nextToken()
	target := p.parseExpression(PREFIX)
	if target == nil {
		return nil
	}
	if call, ok := target.(*CallExpression); ok {
		expr.Call = call
		return expr
	}
	// Bare function value: spawn f  →  spawn f()
	if id, ok := target.(*Identifier); ok {
		expr.Call = &CallExpression{
			Token:     id.Token,
			Function:  id,
			Arguments: []Expression{},
		}
		return expr
	}
	p.addError(expr.Token, "'spawn' must be followed by a function or call, e.g. spawn worker(ch)")
	return nil
}

// parseAwaitExpression parses: await t  or  await(t)  or  await makeTask()
// The operand must evaluate to a task handle.
func (p *Parser) parseAwaitExpression() Expression {
	expr := &AwaitExpression{Token: p.curToken}
	p.nextToken()
	target := p.parseExpression(PREFIX)
	if target == nil {
		return nil
	}
	switch target.(type) {
	case *Identifier, *CallExpression:
		expr.Task = target
		return expr
	}
	p.addError(expr.Token, "'await' must be followed by a task, e.g. await t")
	return nil
}

func (p *Parser) parseExpressionList(end TokenType) []Expression {
	list := []Expression{}
	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}
	p.nextToken()
	list = append(list, p.parseExpression(LOWEST))
	for p.peekTokenIs(COMMA) {
		p.nextToken()
		p.nextToken()
		list = append(list, p.parseExpression(LOWEST))
	}
	if !p.expectPeek(end) {
		return nil
	}
	return list
}

func (p *Parser) curTokenIs(t TokenType) bool  { return p.curToken.Type == t }
func (p *Parser) peekTokenIs(t TokenType) bool { return p.peekToken.Type == t }

func (p *Parser) expectPeek(t TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	got := p.peekToken.Literal
	if p.peekToken.Type == EOF || got == "" {
		got = "end of input"
	}
	p.addError(p.peekToken, fmt.Sprintf("expected '%s', got %s", t, got))
	return false
}

func (p *Parser) peekPrecedence() int {
	if pr, ok := precedences[p.peekToken.Type]; ok {
		return pr
	}
	return LOWEST
}

func (p *Parser) curPrecedence() int {
	if pr, ok := precedences[p.curToken.Type]; ok {
		return pr
	}
	return LOWEST
}
