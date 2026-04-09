package lexer

import "unicode"

// Lexer tokenizes source code.
type Lexer struct {
	file string
	src  []rune
	pos  int
	line int
	col  int
}

// New creates a new Lexer.
func New(file, source string) *Lexer {
	return &Lexer{
		file: file,
		src:  []rune(source),
		pos:  0,
		line: 1,
		col:  1,
	}
}

func (lx *Lexer) peek() rune {
	if lx.pos >= len(lx.src) {
		return 0
	}
	return lx.src[lx.pos]
}

func (lx *Lexer) peekNext() rune {
	if lx.pos+1 >= len(lx.src) {
		return 0
	}
	return lx.src[lx.pos+1]
}

func (lx *Lexer) advance() rune {
	if lx.pos >= len(lx.src) {
		return 0
	}
	c := lx.src[lx.pos]
	lx.pos++
	if c == '\n' {
		lx.line++
		lx.col = 1
	} else {
		lx.col++
	}
	return c
}

func (lx *Lexer) match(expected rune) bool {
	if lx.peek() != expected {
		return false
	}
	lx.advance()
	return true
}

func (lx *Lexer) skipWsAndComments() {
	for {
		c := lx.peek()
		if c == ' ' || c == '\r' || c == '\t' || c == '\n' {
			lx.advance()
			continue
		}
		if c == '/' && lx.peekNext() == '/' {
			for lx.peek() != 0 && lx.peek() != '\n' {
				lx.advance()
			}
			continue
		}
		break
	}
}

var keywords = map[string]TokenKind{
	"fn":       TOK_KW_FN,
	"import":   TOK_KW_IMPORT,
	"if":       TOK_KW_IF,
	"elif":     TOK_KW_ELIF,
	"let":      TOK_KW_LET,
	"muwani":   TOK_KW_LET,
	"else":     TOK_KW_ELSE,
	"true":     TOK_KW_TRUE,
	"void":     TOK_KW_VOID,
	"int":      TOK_KW_INT,
	"amba":     TOK_KW_AMBA,
	"const":    TOK_KW_CONST,
	"crot":     TOK_KW_CONST,
	"while":    TOK_KW_WHILE,
	"for":      TOK_KW_FOR,
	"match":    TOK_KW_MATCH,
	"break":    TOK_KW_BREAK,
	"continue": TOK_KW_CONTINUE,
	"false":    TOK_KW_FALSE,
	"float":    TOK_KW_FLOAT,
	"rusdi":    TOK_KW_RUSDI,
	"fuad":     TOK_KW_FUAD,
	"imut":     TOK_KW_IMUT,
	"return":   TOK_KW_RETURN,
	"string":   TOK_KW_STRING,
	"bool":     TOK_KW_BOOL,
}

func (lx *Lexer) lexIdentOrKw(startPos, line, col int) Token {
	for isAlnum(lx.peek()) || lx.peek() == '_' {
		lx.advance()
	}
	val := string(lx.src[startPos:lx.pos])
	kind := TOK_IDENT
	if k, ok := keywords[val]; ok {
		kind = k
	}
	return Token{Kind: kind, Value: val, Line: line, Col: col}
}

func (lx *Lexer) lexNumber(startPos, line, col int) Token {
	kind := TOK_INT_LIT
	for isDigit(lx.peek()) {
		lx.advance()
	}
	if lx.peek() == '.' && isDigit(lx.peekNext()) {
		kind = TOK_FLOAT_LIT
		lx.advance()
		for isDigit(lx.peek()) {
			lx.advance()
		}
	}
	return Token{Kind: kind, Value: string(lx.src[startPos:lx.pos]), Line: line, Col: col}
}

func (lx *Lexer) lexString(startPos, line, col int) Token {
	lx.advance() // opening quote
	for lx.peek() != 0 && lx.peek() != '"' {
		if lx.peek() == '\\' {
			lx.advance()
			if lx.peek() != 0 {
				lx.advance()
			}
			continue
		}
		lx.advance()
	}
	if lx.peek() != '"' {
		return Token{Kind: TOK_INVALID, Value: string(lx.src[startPos:lx.pos]), Line: line, Col: col}
	}
	lx.advance() // closing quote
	return Token{Kind: TOK_STRING_LIT, Value: string(lx.src[startPos:lx.pos]), Line: line, Col: col}
}

// Next returns the next token from the source.
func (lx *Lexer) Next() Token {
	lx.skipWsAndComments()

	startPos := lx.pos
	line := lx.line
	col := lx.col

	c := lx.peek()
	if c == 0 {
		return Token{Kind: TOK_EOF, Line: line, Col: col}
	}

	if isAlpha(c) || c == '_' {
		return lx.lexIdentOrKw(startPos, line, col)
	}
	if isDigit(c) {
		lx.advance()
		return lx.lexNumber(startPos, line, col)
	}

	lx.advance()
	switch c {
	case '"':
		// back up so lexString can consume opening quote
		lx.pos = startPos
		lx.col = col
		lx.line = line
		return lx.lexString(startPos, line, col)
	case '+':
		if lx.match('+') {
			return Token{Kind: TOK_PLUS_PLUS, Value: "++", Line: line, Col: col}
		}
		if lx.match('=') {
			return Token{Kind: TOK_PLUS_ASSIGN, Value: "+=", Line: line, Col: col}
		}
		return Token{Kind: TOK_PLUS, Value: "+", Line: line, Col: col}
	case '-':
		if lx.match('>') {
			return Token{Kind: TOK_ARROW, Value: "->", Line: line, Col: col}
		}
		if lx.match('=') {
			return Token{Kind: TOK_MINUS_ASSIGN, Value: "-=", Line: line, Col: col}
		}
		if lx.match('-') {
			return Token{Kind: TOK_MINUS_MINUS, Value: "--", Line: line, Col: col}
		}
		return Token{Kind: TOK_MINUS, Value: "-", Line: line, Col: col}
	case '*':
		if lx.match('=') {
			return Token{Kind: TOK_STAR_ASSIGN, Value: "*=", Line: line, Col: col}
		}
		return Token{Kind: TOK_STAR, Value: "*", Line: line, Col: col}
	case '/':
		if lx.match('=') {
			return Token{Kind: TOK_SLASH_ASSIGN, Value: "/=", Line: line, Col: col}
		}
		return Token{Kind: TOK_SLASH, Value: "/", Line: line, Col: col}
	case '%':
		if lx.match('=') {
			return Token{Kind: TOK_PERCENT_ASSIGN, Value: "%=", Line: line, Col: col}
		}
		return Token{Kind: TOK_PERCENT, Value: "%", Line: line, Col: col}
	case '!':
		if lx.match('=') {
			return Token{Kind: TOK_NE, Value: "!=", Line: line, Col: col}
		}
		return Token{Kind: TOK_BANG, Value: "!", Line: line, Col: col}
	case '=':
		if lx.match('=') {
			return Token{Kind: TOK_EQ, Value: "==", Line: line, Col: col}
		}
		if lx.match('>') {
			return Token{Kind: TOK_FAT_ARROW, Value: "=>", Line: line, Col: col}
		}
		return Token{Kind: TOK_ASSIGN, Value: "=", Line: line, Col: col}
	case '<':
		if lx.match('=') {
			return Token{Kind: TOK_LE, Value: "<=", Line: line, Col: col}
		}
		return Token{Kind: TOK_LT, Value: "<", Line: line, Col: col}
	case '>':
		if lx.match('=') {
			return Token{Kind: TOK_GE, Value: ">=", Line: line, Col: col}
		}
		return Token{Kind: TOK_GT, Value: ">", Line: line, Col: col}
	case '&':
		if lx.match('&') {
			return Token{Kind: TOK_AND_AND, Value: "&&", Line: line, Col: col}
		}
		return Token{Kind: TOK_INVALID, Value: "&", Line: line, Col: col}
	case '|':
		if lx.match('|') {
			return Token{Kind: TOK_OR_OR, Value: "||", Line: line, Col: col}
		}
		return Token{Kind: TOK_INVALID, Value: "|", Line: line, Col: col}
	case '(':
		return Token{Kind: TOK_LPAREN, Value: "(", Line: line, Col: col}
	case ')':
		return Token{Kind: TOK_RPAREN, Value: ")", Line: line, Col: col}
	case '{':
		return Token{Kind: TOK_LBRACE, Value: "{", Line: line, Col: col}
	case '}':
		return Token{Kind: TOK_RBRACE, Value: "}", Line: line, Col: col}
	case '[':
		return Token{Kind: TOK_LBRACKET, Value: "[", Line: line, Col: col}
	case ']':
		return Token{Kind: TOK_RBRACKET, Value: "]", Line: line, Col: col}
	case ',':
		return Token{Kind: TOK_COMMA, Value: ",", Line: line, Col: col}
	case ':':
		return Token{Kind: TOK_COLON, Value: ":", Line: line, Col: col}
	case ';':
		return Token{Kind: TOK_SEMI, Value: ";", Line: line, Col: col}
	default:
		return Token{Kind: TOK_INVALID, Value: string(c), Line: line, Col: col}
	}
}

func isAlpha(c rune) bool {
	return unicode.IsLetter(c)
}

func isDigit(c rune) bool {
	return c >= '0' && c <= '9'
}

func isAlnum(c rune) bool {
	return isAlpha(c) || isDigit(c)
}
