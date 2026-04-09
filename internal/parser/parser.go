package parser

import (
	"fmt"
	"os"
	"strconv"

	"ngawi/internal/diag"
	"ngawi/internal/lexer"
)

type parser struct {
	file       string
	source     string
	lx         *lexer.Lexer
	cur        lexer.Token
	next       lexer.Token
	hadError   bool
	errorCount int
	maxErrors  int
	panicMode  bool
	stopParse  bool
}

func newParser(file, source string) *parser {
	lx := lexer.New(file, source)
	p := &parser{
		file:      file,
		source:    source,
		lx:        lx,
		maxErrors: 20,
	}
	p.cur = lx.Next()
	p.next = lx.Next()
	return p
}

func (p *parser) advance() {
	p.cur = p.next
	p.next = p.lx.Next()
}

func (p *parser) check(k lexer.TokenKind) bool { return p.cur.Kind == k }

func (p *parser) match(k lexer.TokenKind) bool {
	if !p.check(k) {
		return false
	}
	p.advance()
	return true
}

func (p *parser) synchronize() {
	p.panicMode = false
	for !p.check(lexer.TOK_EOF) {
		if p.check(lexer.TOK_SEMI) {
			p.advance()
			return
		}
		switch p.cur.Kind {
		case lexer.TOK_RBRACE, lexer.TOK_KW_IMPORT, lexer.TOK_KW_FN,
			lexer.TOK_KW_LET, lexer.TOK_KW_CONST, lexer.TOK_KW_IF, lexer.TOK_KW_ELIF,
			lexer.TOK_KW_WHILE, lexer.TOK_KW_FOR, lexer.TOK_KW_MATCH,
			lexer.TOK_KW_BREAK, lexer.TOK_KW_CONTINUE, lexer.TOK_KW_RETURN:
			return
		}
		p.advance()
	}
}

func (p *parser) parseError(msg string) {
	if p.panicMode {
		return
	}
	p.panicMode = true
	diag.ErrorSource(p.file, p.source, p.cur.Line, p.cur.Col,
		"%s, found %s", msg, p.cur.Kind.Name())
	p.hadError = true
	p.errorCount++
	if p.errorCount >= p.maxErrors {
		diag.Error(p.file, p.cur.Line, p.cur.Col, "too many parser errors (max %d)", p.maxErrors)
		p.stopParse = true
	}
}

func (p *parser) consume(k lexer.TokenKind, msg string) lexer.Token {
	t := p.cur
	if p.check(k) {
		p.advance()
		return t
	}
	p.parseError(msg)
	return t
}

func (p *parser) parseType() TypeKind {
	var base TypeKind
	switch {
	case p.match(lexer.TOK_KW_INT) || p.match(lexer.TOK_KW_AMBA):
		base = TypeInt
	case p.match(lexer.TOK_KW_FLOAT) || p.match(lexer.TOK_KW_RUSDI):
		base = TypeFloat
	case p.match(lexer.TOK_KW_BOOL) || p.match(lexer.TOK_KW_FUAD):
		base = TypeBool
	case p.match(lexer.TOK_KW_STRING) || p.match(lexer.TOK_KW_IMUT):
		base = TypeString
	case p.match(lexer.TOK_KW_VOID):
		base = TypeVoid
	default:
		p.parseError("expected type")
		return TypeVoid
	}

	depth := 0
	for p.match(lexer.TOK_LBRACKET) {
		p.consume(lexer.TOK_RBRACKET, "expected ']' in array type")
		depth++
		if depth > 2 {
			p.parseError("array nesting depth > 2 is not supported yet")
		}
	}
	if depth > 2 {
		depth = 2
	}
	if depth > 0 {
		if base == TypeVoid {
			p.parseError("void[] is not allowed")
			return TypeVoid
		}
		return TypeMakeArray(base, depth)
	}
	return base
}

func isAssignOp(k lexer.TokenKind) bool {
	return k == lexer.TOK_ASSIGN || k == lexer.TOK_PLUS_ASSIGN ||
		k == lexer.TOK_MINUS_ASSIGN || k == lexer.TOK_STAR_ASSIGN ||
		k == lexer.TOK_SLASH_ASSIGN || k == lexer.TOK_PERCENT_ASSIGN
}

func isIncdecOp(k lexer.TokenKind) bool {
	return k == lexer.TOK_PLUS_PLUS || k == lexer.TOK_MINUS_MINUS
}

func compoundAssignToBinaryOp(k lexer.TokenKind) int {
	switch k {
	case lexer.TOK_PLUS_ASSIGN:
		return int(lexer.TOK_PLUS)
	case lexer.TOK_MINUS_ASSIGN:
		return int(lexer.TOK_MINUS)
	case lexer.TOK_STAR_ASSIGN:
		return int(lexer.TOK_STAR)
	case lexer.TOK_SLASH_ASSIGN:
		return int(lexer.TOK_SLASH)
	case lexer.TOK_PERCENT_ASSIGN:
		return int(lexer.TOK_PERCENT)
	}
	return int(lexer.TOK_INVALID)
}

func newExpr(kind ExprKind, line, col int) *Expr {
	return &Expr{Kind: kind, Line: line, Col: col}
}

func newStmt(kind StmtKind, line, col int) *Stmt {
	return &Stmt{Kind: kind, Line: line, Col: col}
}

func (p *parser) makeAssignmentStmt(nameTok lexer.Token, assignOp lexer.TokenKind, withSemi bool) *Stmt {
	p.advance() // consume identifier
	opTok := p.cur
	p.advance() // consume assignment operator
	rhs := p.parseExpression()

	if withSemi {
		p.consume(lexer.TOK_SEMI, "expected ';' after assignment")
	}

	if assignOp != lexer.TOK_ASSIGN {
		lhsIdent := newExpr(ExprIdent, nameTok.Line, nameTok.Col)
		lhsIdent.IdentName = nameTok.Value
		bin := newExpr(ExprBinary, opTok.Line, opTok.Col)
		bin.BinaryOp = compoundAssignToBinaryOp(assignOp)
		bin.BinaryLeft = lhsIdent
		bin.BinaryRight = rhs
		rhs = bin
	}

	s := newStmt(StmtAssign, nameTok.Line, nameTok.Col)
	s.AssignName = nameTok.Value
	s.AssignValue = rhs
	return s
}

func (p *parser) makeIncdecStmt(nameTok lexer.Token, opKind lexer.TokenKind, withSemi bool) *Stmt {
	p.advance() // consume identifier
	opTok := p.cur
	p.advance() // consume ++ or --
	if withSemi {
		p.consume(lexer.TOK_SEMI, "expected ';' after increment/decrement")
	}

	lhsIdent := newExpr(ExprIdent, nameTok.Line, nameTok.Col)
	lhsIdent.IdentName = nameTok.Value

	rhsOne := newExpr(ExprInt, opTok.Line, opTok.Col)
	rhsOne.IntVal = 1

	bin := newExpr(ExprBinary, opTok.Line, opTok.Col)
	if opKind == lexer.TOK_PLUS_PLUS {
		bin.BinaryOp = int(lexer.TOK_PLUS)
	} else {
		bin.BinaryOp = int(lexer.TOK_MINUS)
	}
	bin.BinaryLeft = lhsIdent
	bin.BinaryRight = rhsOne

	s := newStmt(StmtAssign, nameTok.Line, nameTok.Col)
	s.AssignName = nameTok.Value
	s.AssignValue = bin
	return s
}

func (p *parser) parsePrimary() *Expr {
	t := p.cur

	if p.match(lexer.TOK_INT_LIT) {
		e := newExpr(ExprInt, t.Line, t.Col)
		v, _ := strconv.ParseInt(t.Value, 10, 64)
		e.IntVal = v
		return e
	}

	if p.match(lexer.TOK_FLOAT_LIT) {
		e := newExpr(ExprFloat, t.Line, t.Col)
		v, _ := strconv.ParseFloat(t.Value, 64)
		e.FloatVal = v
		return e
	}

	if p.match(lexer.TOK_STRING_LIT) {
		e := newExpr(ExprString, t.Line, t.Col)
		// strip surrounding quotes
		inner := t.Value
		if len(inner) >= 2 {
			inner = inner[1 : len(inner)-1]
		}
		e.StringVal = inner
		return e
	}

	if p.match(lexer.TOK_KW_TRUE) {
		e := newExpr(ExprBool, t.Line, t.Col)
		e.BoolVal = true
		return e
	}

	if p.match(lexer.TOK_KW_FALSE) {
		e := newExpr(ExprBool, t.Line, t.Col)
		e.BoolVal = false
		return e
	}

	if p.match(lexer.TOK_LBRACKET) {
		e := newExpr(ExprArrayLiteral, t.Line, t.Col)
		if !p.check(lexer.TOK_RBRACKET) {
			for {
				item := p.parseExpression()
				e.ArrayItems = append(e.ArrayItems, item)
				if !p.match(lexer.TOK_COMMA) {
					break
				}
			}
		}
		p.consume(lexer.TOK_RBRACKET, "expected ']' after array literal")
		return e
	}

	if p.match(lexer.TOK_IDENT) {
		name := t.Value
		if p.match(lexer.TOK_LPAREN) {
			e := newExpr(ExprCall, t.Line, t.Col)
			e.CallName = name
			if !p.check(lexer.TOK_RPAREN) {
				for {
					arg := p.parseExpression()
					e.CallArgs = append(e.CallArgs, arg)
					if !p.match(lexer.TOK_COMMA) {
						break
					}
				}
			}
			p.consume(lexer.TOK_RPAREN, "expected ')' after arguments")
			return e
		}
		e := newExpr(ExprIdent, t.Line, t.Col)
		e.IdentName = name
		return e
	}

	if p.match(lexer.TOK_LPAREN) {
		inner := p.parseExpression()
		p.consume(lexer.TOK_RPAREN, "expected ')' after expression")
		return inner
	}

	p.parseError("expected expression")
	p.advance()
	return newExpr(ExprInt, t.Line, t.Col)
}

func (p *parser) parsePostfix() *Expr {
	e := p.parsePrimary()
	for p.check(lexer.TOK_LBRACKET) {
		lb := p.cur
		p.advance()
		idx := p.parseExpression()
		p.consume(lexer.TOK_RBRACKET, "expected ']' after index expression")
		ix := newExpr(ExprIndex, lb.Line, lb.Col)
		ix.IndexTarget = e
		ix.IndexIdx = idx
		e = ix
	}
	return e
}

func (p *parser) parseUnary() *Expr {
	if p.check(lexer.TOK_BANG) || p.check(lexer.TOK_MINUS) {
		op := p.cur
		p.advance()
		rhs := p.parseUnary()
		e := newExpr(ExprUnary, op.Line, op.Col)
		e.UnaryOp = int(op.Kind)
		e.UnaryExpr = rhs
		return e
	}
	return p.parsePostfix()
}

func (p *parser) parseFactor() *Expr {
	left := p.parseUnary()
	for p.check(lexer.TOK_STAR) || p.check(lexer.TOK_SLASH) || p.check(lexer.TOK_PERCENT) {
		op := p.cur
		p.advance()
		right := p.parseUnary()
		bin := newExpr(ExprBinary, op.Line, op.Col)
		bin.BinaryOp = int(op.Kind)
		bin.BinaryLeft = left
		bin.BinaryRight = right
		left = bin
	}
	return left
}

func (p *parser) parseTerm() *Expr {
	left := p.parseFactor()
	for p.check(lexer.TOK_PLUS) || p.check(lexer.TOK_MINUS) {
		op := p.cur
		p.advance()
		right := p.parseFactor()
		bin := newExpr(ExprBinary, op.Line, op.Col)
		bin.BinaryOp = int(op.Kind)
		bin.BinaryLeft = left
		bin.BinaryRight = right
		left = bin
	}
	return left
}

func (p *parser) parseComparison() *Expr {
	left := p.parseTerm()
	for p.check(lexer.TOK_LT) || p.check(lexer.TOK_LE) || p.check(lexer.TOK_GT) || p.check(lexer.TOK_GE) {
		op := p.cur
		p.advance()
		right := p.parseTerm()
		bin := newExpr(ExprBinary, op.Line, op.Col)
		bin.BinaryOp = int(op.Kind)
		bin.BinaryLeft = left
		bin.BinaryRight = right
		left = bin
	}
	return left
}

func (p *parser) parseEquality() *Expr {
	left := p.parseComparison()
	for p.check(lexer.TOK_EQ) || p.check(lexer.TOK_NE) {
		op := p.cur
		p.advance()
		right := p.parseComparison()
		bin := newExpr(ExprBinary, op.Line, op.Col)
		bin.BinaryOp = int(op.Kind)
		bin.BinaryLeft = left
		bin.BinaryRight = right
		left = bin
	}
	return left
}

func (p *parser) parseLogicalAnd() *Expr {
	left := p.parseEquality()
	for p.check(lexer.TOK_AND_AND) {
		op := p.cur
		p.advance()
		right := p.parseEquality()
		bin := newExpr(ExprBinary, op.Line, op.Col)
		bin.BinaryOp = int(op.Kind)
		bin.BinaryLeft = left
		bin.BinaryRight = right
		left = bin
	}
	return left
}

func (p *parser) parseExpression() *Expr {
	left := p.parseLogicalAnd()
	for p.check(lexer.TOK_OR_OR) {
		op := p.cur
		p.advance()
		right := p.parseLogicalAnd()
		bin := newExpr(ExprBinary, op.Line, op.Col)
		bin.BinaryOp = int(op.Kind)
		bin.BinaryLeft = left
		bin.BinaryRight = right
		left = bin
	}
	return left
}

func (p *parser) parseBlock() *Stmt {
	lb := p.consume(lexer.TOK_LBRACE, "expected '{'")
	blk := newStmt(StmtBlock, lb.Line, lb.Col)

	for !p.check(lexer.TOK_RBRACE) && !p.check(lexer.TOK_EOF) && !p.stopParse {
		it := p.parseStatement()
		blk.BlockItems = append(blk.BlockItems, it)
		if p.panicMode {
			p.synchronize()
		}
	}
	p.consume(lexer.TOK_RBRACE, "expected '}'")
	return blk
}

func (p *parser) parseIf() *Stmt {
	kw := p.consume(lexer.TOK_KW_IF, "expected 'if'")
	p.consume(lexer.TOK_LPAREN, "expected '(' after if")
	cond := p.parseExpression()
	p.consume(lexer.TOK_RPAREN, "expected ')' after if condition")
	thenBranch := p.parseStatement()

	root := newStmt(StmtIf, kw.Line, kw.Col)
	root.IfCond = cond
	root.IfThen = thenBranch

	cursor := root
	for p.check(lexer.TOK_KW_ELIF) {
		elifTok := p.cur
		p.advance()
		p.consume(lexer.TOK_LPAREN, "expected '(' after elif")
		elifCond := p.parseExpression()
		p.consume(lexer.TOK_RPAREN, "expected ')' after elif condition")
		elifThen := p.parseStatement()

		elifNode := newStmt(StmtIf, elifTok.Line, elifTok.Col)
		elifNode.IfCond = elifCond
		elifNode.IfThen = elifThen
		cursor.IfElse = elifNode
		cursor = elifNode
	}
	if p.match(lexer.TOK_KW_ELSE) {
		cursor.IfElse = p.parseStatement()
	}
	return root
}

func (p *parser) parseWhile() *Stmt {
	kw := p.consume(lexer.TOK_KW_WHILE, "expected 'while'")
	p.consume(lexer.TOK_LPAREN, "expected '(' after while")
	cond := p.parseExpression()
	p.consume(lexer.TOK_RPAREN, "expected ')' after while condition")
	body := p.parseStatement()

	s := newStmt(StmtWhile, kw.Line, kw.Col)
	s.WhileCond = cond
	s.WhileBody = body
	return s
}

func (p *parser) parseForClauseStmt(withSemi bool) *Stmt {
	if p.check(lexer.TOK_KW_LET) || p.check(lexer.TOK_KW_CONST) {
		isConst := p.check(lexer.TOK_KW_CONST)
		kw := p.cur
		p.advance()
		name := p.consume(lexer.TOK_IDENT, "expected identifier")
		var hasType bool
		var typ TypeKind
		if p.match(lexer.TOK_COLON) {
			hasType = true
			typ = p.parseType()
		}
		p.consume(lexer.TOK_ASSIGN, "expected '=' in declaration")
		init := p.parseExpression()
		if withSemi {
			p.consume(lexer.TOK_SEMI, "expected ';' after declaration")
		}
		kind := StmtVarDecl
		if isConst {
			kind = StmtConstDecl
		}
		s := newStmt(kind, kw.Line, kw.Col)
		s.DeclName = name.Value
		s.DeclHasType = hasType
		s.DeclType = typ
		s.DeclInit = init
		return s
	}

	if p.check(lexer.TOK_IDENT) && isAssignOp(p.next.Kind) {
		name := p.cur
		assignOp := p.next.Kind
		return p.makeAssignmentStmt(name, assignOp, withSemi)
	}

	if p.check(lexer.TOK_IDENT) && isIncdecOp(p.next.Kind) {
		name := p.cur
		opKind := p.next.Kind
		return p.makeIncdecStmt(name, opKind, withSemi)
	}

	t := p.cur
	e := p.parseExpression()
	if withSemi {
		p.consume(lexer.TOK_SEMI, "expected ';' after expression")
	}
	s := newStmt(StmtExpr, t.Line, t.Col)
	s.ExprValue = e
	return s
}

func (p *parser) parseFor() *Stmt {
	kw := p.consume(lexer.TOK_KW_FOR, "expected 'for'")
	p.consume(lexer.TOK_LPAREN, "expected '(' after for")

	var init *Stmt
	if !p.check(lexer.TOK_SEMI) {
		init = p.parseForClauseStmt(true)
	} else {
		p.consume(lexer.TOK_SEMI, "expected ';' after for initializer")
	}

	var cond *Expr
	if !p.check(lexer.TOK_SEMI) {
		cond = p.parseExpression()
	}
	p.consume(lexer.TOK_SEMI, "expected ';' after for condition")

	var update *Stmt
	if !p.check(lexer.TOK_RPAREN) {
		update = p.parseForClauseStmt(false)
	}
	p.consume(lexer.TOK_RPAREN, "expected ')' after for clauses")
	body := p.parseStatement()

	s := newStmt(StmtFor, kw.Line, kw.Col)
	s.ForInit = init
	s.ForCond = cond
	s.ForUpdate = update
	s.ForBody = body
	return s
}

func isWildcardIdent(t lexer.Token) bool {
	return t.Kind == lexer.TOK_IDENT && t.Value == "_"
}

func (p *parser) parseMatch() *Stmt {
	kw := p.consume(lexer.TOK_KW_MATCH, "expected 'match'")
	subject := p.parseExpression()
	p.consume(lexer.TOK_LBRACE, "expected '{' after match subject")

	var arms []MatchArm
	for !p.check(lexer.TOK_RBRACE) && !p.check(lexer.TOK_EOF) {
		var arm MatchArm
		arm.Line = p.cur.Line
		arm.Col = p.cur.Col

		if isWildcardIdent(p.cur) {
			arm.PatternKind = MatchWildcard
			p.advance()
		} else if p.check(lexer.TOK_INT_LIT) {
			lit := p.cur
			arm.PatternKind = MatchInt
			v, _ := strconv.ParseInt(lit.Value, 10, 64)
			arm.IntValue = v
			p.advance()
		} else if p.check(lexer.TOK_KW_TRUE) || p.check(lexer.TOK_KW_FALSE) {
			arm.PatternKind = MatchBool
			arm.BoolValue = p.check(lexer.TOK_KW_TRUE)
			p.advance()
		} else if p.check(lexer.TOK_STRING_LIT) {
			lit := p.cur
			arm.PatternKind = MatchMatchString
			inner := lit.Value
			if len(inner) >= 2 {
				inner = inner[1 : len(inner)-1]
			}
			arm.StringValue = inner
			p.advance()
		} else {
			p.parseError("expected int/bool/string literal or '_' in match arm")
			break
		}

		p.consume(lexer.TOK_FAT_ARROW, "expected '=>' after match arm pattern")
		arm.Body = p.parseStatement()
		arms = append(arms, arm)
	}
	p.consume(lexer.TOK_RBRACE, "expected '}' after match arms")

	s := newStmt(StmtMatch, kw.Line, kw.Col)
	s.MatchSubject = subject
	s.MatchArms = arms
	return s
}

func (p *parser) parseStatement() *Stmt {
	if p.check(lexer.TOK_LBRACE) {
		return p.parseBlock()
	}
	if p.check(lexer.TOK_KW_IF) {
		return p.parseIf()
	}
	if p.check(lexer.TOK_KW_ELIF) {
		p.parseError("unexpected 'elif' without preceding 'if'")
		p.advance()
		return newStmt(StmtBlock, p.cur.Line, p.cur.Col)
	}
	if p.check(lexer.TOK_KW_WHILE) {
		return p.parseWhile()
	}
	if p.check(lexer.TOK_KW_FOR) {
		return p.parseFor()
	}
	if p.check(lexer.TOK_KW_MATCH) {
		return p.parseMatch()
	}
	if p.check(lexer.TOK_KW_IMPORT) {
		kw := p.cur
		p.parseError("'import' is only allowed at top-level")
		p.advance()
		if p.check(lexer.TOK_STRING_LIT) {
			p.advance()
		}
		if p.check(lexer.TOK_SEMI) {
			p.advance()
		}
		return newStmt(StmtBlock, kw.Line, kw.Col)
	}

	if p.check(lexer.TOK_KW_LET) || p.check(lexer.TOK_KW_CONST) {
		isConst := p.check(lexer.TOK_KW_CONST)
		kw := p.cur
		p.advance()
		name := p.consume(lexer.TOK_IDENT, "expected identifier")
		var hasType bool
		var typ TypeKind
		if p.match(lexer.TOK_COLON) {
			hasType = true
			typ = p.parseType()
		}
		p.consume(lexer.TOK_ASSIGN, "expected '=' in declaration")
		init := p.parseExpression()
		p.consume(lexer.TOK_SEMI, "expected ';' after declaration")

		kind := StmtVarDecl
		if isConst {
			kind = StmtConstDecl
		}
		s := newStmt(kind, kw.Line, kw.Col)
		s.DeclName = name.Value
		s.DeclHasType = hasType
		s.DeclType = typ
		s.DeclInit = init
		return s
	}

	if p.check(lexer.TOK_KW_RETURN) {
		kw := p.cur
		p.advance()
		s := newStmt(StmtReturn, kw.Line, kw.Col)
		s.RetHasExpr = !p.check(lexer.TOK_SEMI)
		if s.RetHasExpr {
			s.RetExpr = p.parseExpression()
		}
		p.consume(lexer.TOK_SEMI, "expected ';' after return")
		return s
	}

	if p.check(lexer.TOK_KW_BREAK) {
		kw := p.cur
		p.advance()
		p.consume(lexer.TOK_SEMI, "expected ';' after break")
		return newStmt(StmtBreak, kw.Line, kw.Col)
	}

	if p.check(lexer.TOK_KW_CONTINUE) {
		kw := p.cur
		p.advance()
		p.consume(lexer.TOK_SEMI, "expected ';' after continue")
		return newStmt(StmtContinue, kw.Line, kw.Col)
	}

	if p.check(lexer.TOK_IDENT) && isAssignOp(p.next.Kind) {
		name := p.cur
		assignOp := p.next.Kind
		return p.makeAssignmentStmt(name, assignOp, true)
	}

	if p.check(lexer.TOK_IDENT) && isIncdecOp(p.next.Kind) {
		name := p.cur
		opKind := p.next.Kind
		return p.makeIncdecStmt(name, opKind, true)
	}

	t := p.cur
	e := p.parseExpression()

	if p.check(lexer.TOK_ASSIGN) && e.Kind == ExprIndex {
		p.advance() // consume '='
		rhs := p.parseExpression()
		p.consume(lexer.TOK_SEMI, "expected ';' after indexed assignment")

		s := newStmt(StmtIndexAssign, t.Line, t.Col)
		s.IndexAssignTarget = e.IndexTarget
		s.IndexAssignIndex = e.IndexIdx
		s.IndexAssignValue = rhs
		return s
	}

	p.consume(lexer.TOK_SEMI, "expected ';' after expression")
	s := newStmt(StmtExpr, t.Line, t.Col)
	s.ExprValue = e
	return s
}

func (p *parser) parseFunction() FunctionDecl {
	p.consume(lexer.TOK_KW_FN, "expected 'fn'")
	nameTok := p.consume(lexer.TOK_IDENT, "expected function name")
	p.consume(lexer.TOK_LPAREN, "expected '('")

	var fn FunctionDecl
	fn.Name = nameTok.Value

	if !p.check(lexer.TOK_RPAREN) {
		for {
			pname := p.consume(lexer.TOK_IDENT, "expected parameter name")
			p.consume(lexer.TOK_COLON, "expected ':' after parameter name")
			ptype := p.parseType()
			fn.Params = append(fn.Params, Param{Name: pname.Value, Type: ptype})
			if !p.match(lexer.TOK_COMMA) {
				break
			}
		}
	}

	p.consume(lexer.TOK_RPAREN, "expected ')' after parameters")
	p.consume(lexer.TOK_ARROW, "expected '->' after parameters")
	fn.ReturnType = p.parseType()
	fn.Body = p.parseBlock()
	return fn
}

func (p *parser) parseImportDecl() ImportDecl {
	kw := p.consume(lexer.TOK_KW_IMPORT, "expected 'import'")
	pathTok := p.consume(lexer.TOK_STRING_LIT, "expected string path after import")
	p.consume(lexer.TOK_SEMI, "expected ';' after import")

	path := pathTok.Value
	if len(path) >= 2 {
		path = path[1 : len(path)-1]
	}
	return ImportDecl{Path: path, Line: kw.Line, Col: kw.Col}
}

// ParseProgram parses a complete ngawi source file.
func ParseProgram(file, source string) (*Program, bool) {
	p := newParser(file, source)
	prog := &Program{}
	seenFunction := false
	firstFnName := ""
	firstFnLine := 0

	for !p.check(lexer.TOK_EOF) && !p.stopParse {
		if p.check(lexer.TOK_KW_IMPORT) {
			if seenFunction {
				var msg string
				if firstFnName != "" {
					msg = fmt.Sprintf(
						"import declarations must appear before function declarations (first function '%s' starts at line %d)",
						firstFnName, firstFnLine)
				} else {
					msg = "import declarations must appear before function declarations"
				}
				p.parseError(msg)
				p.advance()
				if p.check(lexer.TOK_STRING_LIT) {
					p.advance()
				}
				if p.check(lexer.TOK_SEMI) {
					p.advance()
				}
				if p.panicMode {
					p.synchronize()
				}
				continue
			}
			imp := p.parseImportDecl()
			prog.Imports = append(prog.Imports, imp)
			if p.panicMode {
				p.synchronize()
			}
			continue
		}

		if !p.check(lexer.TOK_KW_FN) {
			p.parseError("expected top-level 'import' or 'fn'")
			p.synchronize()
			continue
		}

		fn := p.parseFunction()
		if !seenFunction {
			seenFunction = true
			firstFnName = fn.Name
			if fn.Body != nil {
				firstFnLine = fn.Body.Line
			} else {
				firstFnLine = 1
			}
		}
		prog.Funcs = append(prog.Funcs, fn)
		if p.panicMode {
			p.synchronize()
		}
	}

	return prog, p.hadError
}

// Silence unused import warning
var _ = os.Stderr
