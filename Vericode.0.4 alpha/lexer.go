package main

type Lexer struct {
	input        string
	position     int
	readPosition int
	ch           byte
	line         int
	col          int
}

func NewLexer(input string) *Lexer {
	l := &Lexer{input: input, line: 1, col: 0}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition++

	if l.ch == '\n' {
		l.line++
		l.col = 0
	} else {
		l.col++
	}
}

func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

func (l *Lexer) NextToken() Token {
	var tok Token

	l.skipWhitespace()

	line, col := l.line, l.col

	stamp := func(t TokenType, lit string) Token {
		return Token{Type: t, Literal: lit, Line: line, Col: col}
	}

	switch l.ch {
	case '=':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = stamp(EQ, string(ch)+string(l.ch))
		} else {
			tok = stamp(ASSIGN, "=")
		}

	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = stamp(NOT_EQ, string(ch)+string(l.ch))
		} else {
			tok = stamp(BANG, "!")
		}

	case '&':
		if l.peekChar() == '&' {
			ch := l.ch
			l.readChar()
			tok = stamp(AND, string(ch)+string(l.ch))
		} else {
			tok = stamp(ILLEGAL, "&")
		}

	case '<':
		if l.peekChar() == '=' {
			l.readChar()
			tok = stamp(LT_EQ, "<=")
		} else {
			tok = stamp(LT, "<")
		}

	case '>':
		if l.peekChar() == '=' {
			l.readChar()
			tok = stamp(GT_EQ, ">=")
		} else {
			tok = stamp(GT, ">")
		}

	case '|':
		if l.peekChar() == '>' {
			l.readChar()
			tok = stamp(PIPE, "|>")
		} else if l.peekChar() == '|' {
			ch := l.ch
			l.readChar()
			tok = stamp(OR, string(ch)+string(l.ch))
		} else {
			tok = stamp(ILLEGAL, "|")
		}

	case '+':
		if l.peekChar() == '=' {
			l.readChar()
			tok = stamp(PLUS_EQ, "+=")
		} else {
			tok = stamp(PLUS, "+")
		}
	case '-':
		if l.peekChar() == '=' {
			l.readChar()
			tok = stamp(MINUS_EQ, "-=")
		} else {
			tok = stamp(MINUS, "-")
		}
	case '*':
		if l.peekChar() == '=' {
			l.readChar()
			tok = stamp(STAR_EQ, "*=")
		} else {
			tok = stamp(ASTERISK, "*")
		}
	case '/':
		if l.peekChar() == '/' {
			l.skipSingleLineComment()
			return l.NextToken()
		} else if l.peekChar() == '*' {
			l.skipMultiLineComment()
			return l.NextToken()
		} else if l.peekChar() == '=' {
			l.readChar()
			tok = stamp(SLASH_EQ, "/=")
		} else {
			tok = stamp(SLASH, "/")
		}

	case ';':
		tok = stamp(SEMICOLON, ";")
	case '(':
		tok = stamp(LPAREN, "(")
	case ')':
		tok = stamp(RPAREN, ")")
	case '{':
		tok = stamp(LBRACE, "{")
	case '}':
		tok = stamp(RBRACE, "}")
	case '[':
		tok = stamp(LBRACKET, "[")
	case ']':
		tok = stamp(RBRACKET, "]")
	case ',':
		tok = stamp(COMMA, ",")
	case ':':
		tok = stamp(COLON, ":")
	case '.':
		tok = stamp(DOT, ".")

	case '%':
		if l.peekChar() == '=' {
			l.readChar()
			tok = stamp(PERCENT_EQ, "%=")
		} else {
			tok = stamp(PERCENT, "%")
		}

	case '"':
		str := l.readString()
		tok = stamp(STRING, str)
		return tok

	case '\'':
		ch := l.readCharLiteral()
		tok = stamp(CHAR, ch)
		return tok

	case 0:
		tok = stamp(EOF, "")

	default:
		if isLetter(l.ch) {
			lit := l.readIdentifier()
			typ := LookupIdent(lit)
			return stamp(typ, lit)
		} else if isDigit(l.ch) {
			lit, isFloat := l.readNumber()
			if isFloat {
				return stamp(FLOAT, lit)
			}
			return stamp(INT, lit)
		} else {
			tok = stamp(ILLEGAL, string(l.ch))
		}
	}

	l.readChar()
	return tok
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) skipSingleLineComment() {
	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}
}

func (l *Lexer) skipMultiLineComment() {
	l.readChar()
	l.readChar()
	for !(l.ch == '*' && l.peekChar() == '/') && l.ch != 0 {
		l.readChar()
	}
	if l.ch != 0 {
		l.readChar()
		l.readChar()
	}
}

func (l *Lexer) readString() string {
	l.readChar() // skip opening "
	var result []byte
	for l.ch != '"' && l.ch != 0 {
		if l.ch == '\\' {
			l.readChar()
			switch l.ch {
			case 'n':
				result = append(result, '\n')
			case 't':
				result = append(result, '\t')
			case 'r':
				result = append(result, '\r')
			case '\\':
				result = append(result, '\\')
			case '"':
				result = append(result, '"')
			case '0':
				result = append(result, 0)
			default:
				result = append(result, '\\')
				result = append(result, l.ch)
			}
		} else {
			result = append(result, l.ch)
		}
		l.readChar()
	}
	l.readChar() // skip closing "
	return string(result)
}

func (l *Lexer) readCharLiteral() string {
	l.readChar()
	start := l.position
	for l.ch != '\'' && l.ch != 0 {
		l.readChar()
	}
	ch := l.input[start:l.position]
	l.readChar()
	return ch
}

func (l *Lexer) readIdentifier() string {
	pos := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[pos:l.position]
}

func (l *Lexer) readNumber() (string, bool) {
	pos := l.position
	isFloat := false
	for isDigit(l.ch) {
		l.readChar()
	}
	if l.ch == '.' && isDigit(l.peekChar()) {
		isFloat = true
		l.readChar() // consume '.'
		for isDigit(l.ch) {
			l.readChar()
		}
	}
	// Scientific notation: 1e3, 1.5e-2, 2E+10
	if l.ch == 'e' || l.ch == 'E' {
		isFloat = true
		l.readChar() // consume e/E
		if l.ch == '+' || l.ch == '-' {
			l.readChar()
		}
		if !isDigit(l.ch) {
			// malformed exponent — leave as-is; parser will reject
			return l.input[pos:l.position], isFloat
		}
		for isDigit(l.ch) {
			l.readChar()
		}
	}
	return l.input[pos:l.position], isFloat
}

func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}
