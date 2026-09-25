package main

type TokenType string

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Col     int
}

const (
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"

	IDENT  = "IDENT"
	INT    = "INT"
	FLOAT  = "FLOAT"
	STRING = "STRING"
	CHAR   = "CHAR"

	ASSIGN   = "="
	PLUS_EQ  = "+="
	MINUS_EQ = "-="
	STAR_EQ  = "*="
	SLASH_EQ = "/="
	PERCENT_EQ = "%="
	PLUS     = "+"
	MINUS    = "-"
	ASTERISK = "*"
	SLASH    = "/"
	PERCENT  = "%"
	EQ       = "=="
	NOT_EQ   = "!="
	LT       = "<"
	GT       = ">"
	LT_EQ    = "<="
	GT_EQ    = ">="
	PIPE     = "|>"
	AND      = "&&"
	OR       = "||"
	BANG     = "!"

	SEMICOLON = ";"
	LPAREN    = "("
	RPAREN    = ")"
	LBRACE    = "{"
	RBRACE    = "}"
	LBRACKET  = "["
	RBRACKET  = "]"
	COMMA     = ","
	COLON     = ":"
	DOT       = "."

	LET      = "LET"
	PRINT    = "PRINT"
	IF       = "IF"
	ELSE     = "ELSE"
	FUNC     = "FUNC"
	RETURN   = "RETURN"
	TRUE     = "TRUE"
	FALSE    = "FALSE"
	NULL     = "NULL"
	LOOP     = "LOOP"
	WHILE    = "WHILE"
	TIMES    = "TIMES"
	IN       = "IN"
	ASK      = "ASK"
	BREAK    = "BREAK"
	CONTINUE = "CONTINUE"
)

var keywords = map[string]TokenType{
	"let":      LET,
	"print":    PRINT,
	"if":       IF,
	"else":     ELSE,
	"func":     FUNC,
	"return":   RETURN,
	"true":     TRUE,
	"false":    FALSE,
	"null":     NULL,
	"loop":     LOOP,
	"while":    WHILE,
	"times":    TIMES,
	"in":       IN,
	"ask":      ASK,
	"break":    BREAK,
	"continue": CONTINUE,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
