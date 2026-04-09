package sema

import (
	"ngawi/internal/diag"
	"ngawi/internal/lexer"
	"ngawi/internal/parser"
)

type varSymbol struct {
	name    string
	typ     parser.TypeKind
	isConst bool
}

type scope struct {
	vars   []*varSymbol
	parent *scope
}

type funcSymbol struct {
	name string
	decl *parser.FunctionDecl
}

type sema struct {
	file       string
	source     string
	hadError   bool
	errorCount int
	maxErrors  int
	stopAnal   bool
	loopDepth  int
	scope      *scope
	funcs      []funcSymbol
	currentFn  *parser.FunctionDecl
}

func (s *sema) error(line, col int, format string, args ...interface{}) {
	diag.ErrorSource(s.file, s.source, line, col, format, args...)
	s.hadError = true
	s.errorCount++
	if s.errorCount >= s.maxErrors && !s.stopAnal {
		diag.Error(s.file, line, col, "too many semantic errors (max %d)", s.maxErrors)
		s.stopAnal = true
	}
}

func (s *sema) note(line, col int, format string, args ...interface{}) {
	if s.stopAnal {
		return
	}
	diag.NoteSource(s.file, s.source, line, col, format, args...)
}

func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	n, m := len(ra), len(rb)
	prev := make([]int, m+1)
	curr := make([]int, m+1)
	for j := 0; j <= m; j++ {
		prev[j] = j
	}
	for i := 1; i <= n; i++ {
		curr[0] = i
		for j := 1; j <= m; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = min3(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[m]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

func (s *sema) suggestVar(name string) string {
	best, bestDist := "", 9999
	for sc := s.scope; sc != nil; sc = sc.parent {
		for _, v := range sc.vars {
			d := editDistance(name, v.name)
			if d < bestDist {
				bestDist = d
				best = v.name
			}
		}
	}
	if best != "" && bestDist <= 2 {
		return best
	}
	return ""
}

func (s *sema) suggestFn(name string) string {
	best, bestDist := "", 9999
	for _, f := range s.funcs {
		d := editDistance(name, f.name)
		if d < bestDist {
			bestDist = d
			best = f.name
		}
	}
	if best != "" && bestDist <= 2 {
		return best
	}
	return ""
}

func (s *sema) pushScope() {
	s.scope = &scope{parent: s.scope}
}

func (s *sema) popScope() {
	if s.scope != nil {
		s.scope = s.scope.parent
	}
}

func (s *sema) findInCurrentScope(name string) *varSymbol {
	if s.scope == nil {
		return nil
	}
	for _, v := range s.scope.vars {
		if v.name == name {
			return v
		}
	}
	return nil
}

func (s *sema) lookupVar(name string) *varSymbol {
	for sc := s.scope; sc != nil; sc = sc.parent {
		for _, v := range sc.vars {
			if v.name == name {
				return v
			}
		}
	}
	return nil
}

func (s *sema) declareVar(line, col int, name string, typ parser.TypeKind, isConst bool) {
	if s.scope == nil {
		s.pushScope()
	}
	if s.findInCurrentScope(name) != nil {
		s.error(line, col, "redeclaration of '%s'", name)
		return
	}
	s.scope.vars = append(s.scope.vars, &varSymbol{name: name, typ: typ, isConst: isConst})
}

func (s *sema) lookupFn(name string) *funcSymbol {
	for i := range s.funcs {
		if s.funcs[i].name == name {
			return &s.funcs[i]
		}
	}
	return nil
}

func typeIsArray(t parser.TypeKind) bool    { return parser.TypeIsArray(t) }
func typeEq(a, b parser.TypeKind) bool      { return a == b }
func typeIsNumeric(t parser.TypeKind) bool  { return parser.TypeIsNumeric(t) }
func arrayElemType(t parser.TypeKind) parser.TypeKind {
	if !typeIsArray(t) {
		return parser.TypeVoid
	}
	base := parser.TypeArrayBase(t)
	depth := parser.TypeArrayDepth(t)
	if depth <= 1 {
		return base
	}
	return parser.TypeMakeArray(base, depth-1)
}
func arrayOfElem(t parser.TypeKind) parser.TypeKind {
	if t == parser.TypeVoid {
		return parser.TypeVoid
	}
	if !typeIsArray(t) {
		return parser.TypeMakeArray(t, 1)
	}
	return parser.TypeMakeArray(parser.TypeArrayBase(t), parser.TypeArrayDepth(t)+1)
}
func typeSameArrayBase(a, b parser.TypeKind) bool {
	return typeIsArray(a) && typeIsArray(b) && parser.TypeArrayBase(a) == parser.TypeArrayBase(b)
}
func isEmptyArrayLit(e *parser.Expr) bool {
	return e != nil && e.Kind == parser.ExprArrayLiteral && len(e.ArrayItems) == 0
}

func (s *sema) maybeNoteArrayDepthMismatch(line, col int, expected, got parser.TypeKind) {
	if !typeSameArrayBase(expected, got) {
		return
	}
	s.note(line, col, "array depth mismatch: expected depth %d, got depth %d",
		parser.TypeArrayDepth(expected), parser.TypeArrayDepth(got))
}

func (s *sema) maybeNoteIndexDepthContext(line, col int, targetType parser.TypeKind) {
	if typeIsArray(targetType) {
		s.note(line, col, "indexed value has array depth %d", parser.TypeArrayDepth(targetType))
	} else {
		s.note(line, col, "indexed value has array depth 0 (non-array)")
	}
}

func setExprType(e *parser.Expr, t parser.TypeKind) parser.TypeKind {
	e.InferredType = t
	return t
}

func (s *sema) checkCall(e *parser.Expr) parser.TypeKind {
	if s.stopAnal {
		return setExprType(e, parser.TypeVoid)
	}
	name := e.CallName
	args := e.CallArgs

	checkOneStringArg := func(fnName string) parser.TypeKind {
		if len(args) != 1 {
			s.error(e.Line, e.Col, "%s expects 1 argument, got %d", fnName, len(args))
			return setExprType(e, parser.TypeVoid)
		}
		at := s.checkExpr(args[0])
		if at == parser.TypeVoid {
			return setExprType(e, parser.TypeVoid)
		}
		if !typeEq(at, parser.TypeString) {
			s.error(e.Line, e.Col, "%s expects string, got '%s'", fnName, parser.TypeKindName(at))
			return setExprType(e, parser.TypeVoid)
		}
		return setExprType(e, parser.TypeString)
	}

	switch name {
	case "print":
		for _, arg := range args {
			s.checkExpr(arg)
			if s.stopAnal {
				return setExprType(e, parser.TypeVoid)
			}
		}
		return setExprType(e, parser.TypeVoid)

	case "to_int", "to_amba":
		if len(args) != 1 {
			s.error(e.Line, e.Col, "to_int/to_amba expects 1 argument, got %d", len(args))
			return setExprType(e, parser.TypeVoid)
		}
		at := s.checkExpr(args[0])
		if at == parser.TypeVoid {
			return setExprType(e, parser.TypeVoid)
		}
		if !typeEq(at, parser.TypeInt) && !typeEq(at, parser.TypeFloat) {
			s.error(e.Line, e.Col, "to_int/to_amba expects int or float, got '%s'", parser.TypeKindName(at))
			return setExprType(e, parser.TypeVoid)
		}
		return setExprType(e, parser.TypeInt)

	case "to_float", "to_rusdi":
		if len(args) != 1 {
			s.error(e.Line, e.Col, "to_float/to_rusdi expects 1 argument, got %d", len(args))
			return setExprType(e, parser.TypeVoid)
		}
		at := s.checkExpr(args[0])
		if at == parser.TypeVoid {
			return setExprType(e, parser.TypeVoid)
		}
		if !typeEq(at, parser.TypeInt) && !typeEq(at, parser.TypeFloat) {
			s.error(e.Line, e.Col, "to_float/to_rusdi expects int or float, got '%s'", parser.TypeKindName(at))
			return setExprType(e, parser.TypeVoid)
		}
		return setExprType(e, parser.TypeFloat)

	case "len":
		if len(args) != 1 {
			s.error(e.Line, e.Col, "len expects 1 argument, got %d", len(args))
			return setExprType(e, parser.TypeVoid)
		}
		at := s.checkExpr(args[0])
		if at == parser.TypeVoid {
			return setExprType(e, parser.TypeVoid)
		}
		if !typeEq(at, parser.TypeString) && !typeIsArray(at) {
			s.error(e.Line, e.Col, "len expects string or array, got '%s'", parser.TypeKindName(at))
			return setExprType(e, parser.TypeVoid)
		}
		return setExprType(e, parser.TypeInt)

	case "push":
		if len(args) != 2 {
			s.error(e.Line, e.Col, "push expects 2 arguments, got %d", len(args))
			return setExprType(e, parser.TypeVoid)
		}
		at := s.checkExpr(args[0])
		vt := s.checkExpr(args[1])
		if at == parser.TypeVoid || vt == parser.TypeVoid {
			return setExprType(e, parser.TypeVoid)
		}
		if !typeIsArray(at) {
			s.error(e.Line, e.Col, "push expects array as first argument, got '%s'", parser.TypeKindName(at))
			return setExprType(e, parser.TypeVoid)
		}
		elem := arrayElemType(at)
		if !typeEq(elem, vt) {
			s.error(e.Line, e.Col, "push on '%s' expects value '%s', got '%s'",
				parser.TypeKindName(at), parser.TypeKindName(elem), parser.TypeKindName(vt))
			return setExprType(e, parser.TypeVoid)
		}
		return setExprType(e, at)

	case "pop":
		if len(args) != 1 {
			s.error(e.Line, e.Col, "pop expects 1 argument, got %d", len(args))
			return setExprType(e, parser.TypeVoid)
		}
		at := s.checkExpr(args[0])
		if at == parser.TypeVoid {
			return setExprType(e, parser.TypeVoid)
		}
		if !typeIsArray(at) {
			s.error(e.Line, e.Col, "pop expects array, got '%s'", parser.TypeKindName(at))
			return setExprType(e, parser.TypeVoid)
		}
		return setExprType(e, at)

	case "contains", "starts_with", "ends_with":
		if len(args) != 2 {
			s.error(e.Line, e.Col, "%s expects 2 arguments, got %d", name, len(args))
			return setExprType(e, parser.TypeVoid)
		}
		a0 := s.checkExpr(args[0])
		a1 := s.checkExpr(args[1])
		if a0 == parser.TypeVoid || a1 == parser.TypeVoid {
			return setExprType(e, parser.TypeVoid)
		}
		if !typeEq(a0, parser.TypeString) || !typeEq(a1, parser.TypeString) {
			s.error(e.Line, e.Col, "%s expects (string, string), got ('%s', '%s')",
				name, parser.TypeKindName(a0), parser.TypeKindName(a1))
			return setExprType(e, parser.TypeVoid)
		}
		return setExprType(e, parser.TypeBool)

	case "to_lower":
		return checkOneStringArg("to_lower")
	case "to_upper":
		return checkOneStringArg("to_upper")
	case "trim":
		return checkOneStringArg("trim")
	}

	// User-defined function
	fn := s.lookupFn(name)
	if fn == nil {
		s.error(e.Line, e.Col, "undefined function '%s'", name)
		if suggest := s.suggestFn(name); suggest != "" {
			s.note(e.Line, e.Col, "did you mean '%s'?", suggest)
		}
		return setExprType(e, parser.TypeVoid)
	}

	if len(args) != len(fn.decl.Params) {
		s.error(e.Line, e.Col, "function '%s' expects %d argument(s), got %d",
			name, len(fn.decl.Params), len(args))
		return setExprType(e, fn.decl.ReturnType)
	}

	for i, arg := range args {
		got := s.checkExpr(arg)
		exp := fn.decl.Params[i].Type
		if s.stopAnal {
			return setExprType(e, fn.decl.ReturnType)
		}
		if got == parser.TypeVoid {
			continue
		}
		if !typeEq(got, exp) {
			s.error(arg.Line, arg.Col, "argument %d of '%s' expects '%s', got '%s'",
				i+1, name, parser.TypeKindName(exp), parser.TypeKindName(got))
			s.maybeNoteArrayDepthMismatch(arg.Line, arg.Col, exp, got)
		}
	}
	return setExprType(e, fn.decl.ReturnType)
}

func (s *sema) checkExpr(e *parser.Expr) parser.TypeKind {
	if s.stopAnal {
		return setExprType(e, parser.TypeVoid)
	}

	switch e.Kind {
	case parser.ExprInt:
		return setExprType(e, parser.TypeInt)
	case parser.ExprFloat:
		return setExprType(e, parser.TypeFloat)
	case parser.ExprString:
		return setExprType(e, parser.TypeString)
	case parser.ExprBool:
		return setExprType(e, parser.TypeBool)

	case parser.ExprIdent:
		v := s.lookupVar(e.IdentName)
		if v == nil {
			s.error(e.Line, e.Col, "use of undeclared identifier '%s'", e.IdentName)
			if suggest := s.suggestVar(e.IdentName); suggest != "" {
				s.note(e.Line, e.Col, "did you mean '%s'?", suggest)
			}
			return setExprType(e, parser.TypeVoid)
		}
		return setExprType(e, v.typ)

	case parser.ExprArrayLiteral:
		if len(e.ArrayItems) == 0 {
			s.error(e.Line, e.Col, "empty array literal requires explicit array type context")
			return setExprType(e, parser.TypeVoid)
		}
		elemT := parser.TypeVoid
		for _, item := range e.ArrayItems {
			it := s.checkExpr(item)
			if it == parser.TypeVoid {
				continue
			}
			if elemT == parser.TypeVoid {
				elemT = it
				continue
			}
			if !typeEq(it, elemT) {
				s.error(item.Line, item.Col, "array literal expects '%s' elements, got '%s'",
					parser.TypeKindName(elemT), parser.TypeKindName(it))
			}
		}
		arrT := arrayOfElem(elemT)
		if arrT == parser.TypeVoid {
			s.error(e.Line, e.Col, "array literal element type is not supported")
			return setExprType(e, parser.TypeVoid)
		}
		return setExprType(e, arrT)

	case parser.ExprIndex:
		tt := s.checkExpr(e.IndexTarget)
		it := s.checkExpr(e.IndexIdx)
		if tt == parser.TypeVoid || it == parser.TypeVoid {
			return setExprType(e, parser.TypeVoid)
		}
		if !typeIsArray(tt) {
			s.error(e.Line, e.Col, "indexing expects array target, got '%s'", parser.TypeKindName(tt))
			s.maybeNoteIndexDepthContext(e.Line, e.Col, tt)
			return setExprType(e, parser.TypeVoid)
		}
		if !typeEq(it, parser.TypeInt) {
			s.error(e.Line, e.Col, "array index must be int, got '%s'", parser.TypeKindName(it))
			s.note(e.Line, e.Col, "index expressions are currently 0-based int offsets")
			return setExprType(e, parser.TypeVoid)
		}
		return setExprType(e, arrayElemType(tt))

	case parser.ExprCall:
		return s.checkCall(e)

	case parser.ExprUnary:
		rhs := s.checkExpr(e.UnaryExpr)
		if rhs == parser.TypeVoid {
			return setExprType(e, parser.TypeVoid)
		}
		if e.UnaryOp == int(lexer.TOK_BANG) {
			if !typeEq(rhs, parser.TypeBool) {
				s.error(e.Line, e.Col, "operator '!' expects bool, got '%s'", parser.TypeKindName(rhs))
				return setExprType(e, parser.TypeVoid)
			}
			return setExprType(e, parser.TypeBool)
		}
		if e.UnaryOp == int(lexer.TOK_MINUS) {
			if !typeIsNumeric(rhs) {
				s.error(e.Line, e.Col, "unary '-' expects numeric type, got '%s'", parser.TypeKindName(rhs))
				return setExprType(e, parser.TypeVoid)
			}
			return setExprType(e, rhs)
		}
		return setExprType(e, parser.TypeVoid)

	case parser.ExprBinary:
		lhs := s.checkExpr(e.BinaryLeft)
		rhs := s.checkExpr(e.BinaryRight)
		op := e.BinaryOp
		if lhs == parser.TypeVoid || rhs == parser.TypeVoid {
			return setExprType(e, parser.TypeVoid)
		}

		switch lexer.TokenKind(op) {
		case lexer.TOK_PLUS:
			if typeEq(lhs, parser.TypeString) && typeEq(rhs, parser.TypeString) {
				return setExprType(e, parser.TypeString)
			}
			if !typeIsNumeric(lhs) || !typeIsNumeric(rhs) || !typeEq(lhs, rhs) {
				s.error(e.Line, e.Col, "'+' expects same numeric types or two strings, got '%s' and '%s'",
					parser.TypeKindName(lhs), parser.TypeKindName(rhs))
				return setExprType(e, parser.TypeVoid)
			}
			return setExprType(e, lhs)
		case lexer.TOK_MINUS, lexer.TOK_STAR, lexer.TOK_SLASH:
			if !typeIsNumeric(lhs) || !typeIsNumeric(rhs) || !typeEq(lhs, rhs) {
				s.error(e.Line, e.Col, "arithmetic operator expects same numeric types, got '%s' and '%s'",
					parser.TypeKindName(lhs), parser.TypeKindName(rhs))
				return setExprType(e, parser.TypeVoid)
			}
			return setExprType(e, lhs)
		case lexer.TOK_PERCENT:
			if !typeEq(lhs, parser.TypeInt) || !typeEq(rhs, parser.TypeInt) {
				s.error(e.Line, e.Col, "'%%' operator expects int operands, got '%s' and '%s'",
					parser.TypeKindName(lhs), parser.TypeKindName(rhs))
				return setExprType(e, parser.TypeVoid)
			}
			return setExprType(e, parser.TypeInt)
		case lexer.TOK_LT, lexer.TOK_LE, lexer.TOK_GT, lexer.TOK_GE:
			if !typeIsNumeric(lhs) || !typeIsNumeric(rhs) || !typeEq(lhs, rhs) {
				s.error(e.Line, e.Col, "comparison operator expects same numeric types, got '%s' and '%s'",
					parser.TypeKindName(lhs), parser.TypeKindName(rhs))
				return setExprType(e, parser.TypeVoid)
			}
			return setExprType(e, parser.TypeBool)
		case lexer.TOK_EQ, lexer.TOK_NE:
			if !typeEq(lhs, rhs) {
				s.error(e.Line, e.Col, "equality operator expects same types, got '%s' and '%s'",
					parser.TypeKindName(lhs), parser.TypeKindName(rhs))
				return setExprType(e, parser.TypeVoid)
			}
			return setExprType(e, parser.TypeBool)
		case lexer.TOK_AND_AND, lexer.TOK_OR_OR:
			if !typeEq(lhs, parser.TypeBool) || !typeEq(rhs, parser.TypeBool) {
				s.error(e.Line, e.Col, "logical operator expects bool operands, got '%s' and '%s'",
					parser.TypeKindName(lhs), parser.TypeKindName(rhs))
				return setExprType(e, parser.TypeVoid)
			}
			return setExprType(e, parser.TypeBool)
		}
	}
	return setExprType(e, parser.TypeVoid)
}

func stmtGuaranteesReturn(st *parser.Stmt) bool {
	if st == nil {
		return false
	}
	switch st.Kind {
	case parser.StmtReturn:
		return true
	case parser.StmtBlock:
		for _, item := range st.BlockItems {
			if stmtGuaranteesReturn(item) {
				return true
			}
		}
		return false
	case parser.StmtIf:
		if st.IfElse == nil {
			return false
		}
		return stmtGuaranteesReturn(st.IfThen) && stmtGuaranteesReturn(st.IfElse)
	default:
		return false
	}
}

func (s *sema) checkStmt(st *parser.Stmt) {
	if s.stopAnal {
		return
	}

	switch st.Kind {
	case parser.StmtBlock:
		s.pushScope()
		for _, item := range st.BlockItems {
			s.checkStmt(item)
			if s.stopAnal {
				break
			}
		}
		s.popScope()

	case parser.StmtVarDecl, parser.StmtConstDecl:
		var initT parser.TypeKind
		if st.DeclHasType && isEmptyArrayLit(st.DeclInit) {
			if !typeIsArray(st.DeclType) {
				s.error(st.Line, st.Col, "empty array literal can only initialize an array-typed variable")
			} else {
				initT = st.DeclType
				setExprType(st.DeclInit, initT)
			}
		} else {
			initT = s.checkExpr(st.DeclInit)
		}

		finalT := initT
		if st.DeclHasType {
			finalT = st.DeclType
			if initT != parser.TypeVoid && !typeEq(finalT, initT) {
				s.error(st.Line, st.Col, "cannot assign '%s' to variable '%s' of type '%s'",
					parser.TypeKindName(initT), st.DeclName, parser.TypeKindName(finalT))
				s.maybeNoteArrayDepthMismatch(st.Line, st.Col, finalT, initT)
			}
		}
		if finalT == parser.TypeVoid {
			s.error(st.Line, st.Col, "variable '%s' cannot have type 'void'", st.DeclName)
		}
		st.DeclHasType = true
		st.DeclType = finalT
		s.declareVar(st.Line, st.Col, st.DeclName, finalT, st.Kind == parser.StmtConstDecl)

	case parser.StmtAssign:
		v := s.lookupVar(st.AssignName)
		if v == nil {
			s.error(st.Line, st.Col, "assignment to undeclared variable '%s'", st.AssignName)
			if suggest := s.suggestVar(st.AssignName); suggest != "" {
				s.note(st.Line, st.Col, "did you mean '%s'?", suggest)
			}
			s.checkExpr(st.AssignValue)
			break
		}
		if v.isConst {
			s.error(st.Line, st.Col, "cannot assign to const variable '%s'", st.AssignName)
		}
		var rhs parser.TypeKind
		if isEmptyArrayLit(st.AssignValue) && typeIsArray(v.typ) {
			rhs = v.typ
			setExprType(st.AssignValue, rhs)
		} else {
			rhs = s.checkExpr(st.AssignValue)
		}
		if rhs != parser.TypeVoid && !typeEq(v.typ, rhs) {
			s.error(st.Line, st.Col, "cannot assign '%s' to variable '%s' of type '%s'",
				parser.TypeKindName(rhs), st.AssignName, parser.TypeKindName(v.typ))
			s.maybeNoteArrayDepthMismatch(st.Line, st.Col, v.typ, rhs)
		}

	case parser.StmtIndexAssign:
		tt := s.checkExpr(st.IndexAssignTarget)
		it := s.checkExpr(st.IndexAssignIndex)
		vt := s.checkExpr(st.IndexAssignValue)

		target := st.IndexAssignTarget
		targetOk := false
		baseName := ""
		if target.Kind == parser.ExprIdent {
			baseName = target.IdentName
			targetOk = true
		} else if target.Kind == parser.ExprIndex && target.IndexTarget != nil &&
			target.IndexTarget.Kind == parser.ExprIdent {
			baseName = target.IndexTarget.IdentName
			targetOk = true
		}

		if !targetOk {
			s.error(st.Line, st.Col, "indexed assignment target must be array variable or 2D array element")
		} else if v := s.lookupVar(baseName); v != nil && v.isConst {
			s.error(st.Line, st.Col, "cannot assign through const variable '%s'", baseName)
		}

		if tt != parser.TypeVoid && !typeIsArray(tt) {
			s.error(st.Line, st.Col, "indexed assignment expects array target, got '%s'", parser.TypeKindName(tt))
			s.maybeNoteIndexDepthContext(st.Line, st.Col, tt)
		}
		if it != parser.TypeVoid && !typeEq(it, parser.TypeInt) {
			s.error(st.Line, st.Col, "array index must be int, got '%s'", parser.TypeKindName(it))
		}
		elemT := arrayElemType(tt)
		if elemT != parser.TypeVoid && vt != parser.TypeVoid && !typeEq(elemT, vt) {
			s.error(st.Line, st.Col, "%s assignment expects %s value, got '%s'",
				parser.TypeKindName(tt), parser.TypeKindName(elemT), parser.TypeKindName(vt))
			s.maybeNoteArrayDepthMismatch(st.Line, st.Col, elemT, vt)
		}

	case parser.StmtExpr:
		s.checkExpr(st.ExprValue)

	case parser.StmtReturn:
		var fnRet parser.TypeKind
		if s.currentFn != nil {
			fnRet = s.currentFn.ReturnType
		}
		if !st.RetHasExpr {
			if !typeEq(fnRet, parser.TypeVoid) {
				s.error(st.Line, st.Col, "return without value in function returning '%s'", parser.TypeKindName(fnRet))
			}
		} else {
			got := s.checkExpr(st.RetExpr)
			if got != parser.TypeVoid && !typeEq(got, fnRet) {
				s.error(st.Line, st.Col, "return type mismatch: expected '%s', got '%s'",
					parser.TypeKindName(fnRet), parser.TypeKindName(got))
			}
		}

	case parser.StmtIf:
		ct := s.checkExpr(st.IfCond)
		if ct != parser.TypeVoid && !typeEq(ct, parser.TypeBool) {
			s.error(st.Line, st.Col, "if condition must be bool, got '%s'", parser.TypeKindName(ct))
		}
		s.checkStmt(st.IfThen)
		if st.IfElse != nil {
			s.checkStmt(st.IfElse)
		}

	case parser.StmtWhile:
		ct := s.checkExpr(st.WhileCond)
		if ct != parser.TypeVoid && !typeEq(ct, parser.TypeBool) {
			s.error(st.Line, st.Col, "while condition must be bool, got '%s'", parser.TypeKindName(ct))
		}
		s.loopDepth++
		s.checkStmt(st.WhileBody)
		s.loopDepth--

	case parser.StmtFor:
		s.pushScope()
		if st.ForInit != nil {
			s.checkStmt(st.ForInit)
		}
		if st.ForCond != nil {
			ct := s.checkExpr(st.ForCond)
			if ct != parser.TypeVoid && !typeEq(ct, parser.TypeBool) {
				s.error(st.Line, st.Col, "for condition must be bool, got '%s'", parser.TypeKindName(ct))
			}
		}
		if st.ForUpdate != nil {
			s.checkStmt(st.ForUpdate)
		}
		s.loopDepth++
		s.checkStmt(st.ForBody)
		s.loopDepth--
		s.popScope()

	case parser.StmtMatch:
		stype := s.checkExpr(st.MatchSubject)
		if stype != parser.TypeVoid && !typeEq(stype, parser.TypeInt) &&
			!typeEq(stype, parser.TypeBool) && !typeEq(stype, parser.TypeString) {
			s.error(st.Line, st.Col, "match subject must be int, bool, or string, got '%s'",
				parser.TypeKindName(stype))
		}

		wildcardCount := 0
		wildcardIndex := -1
		seenTrue := false
		seenFalse := false

		for i, arm := range st.MatchArms {
			if arm.PatternKind == parser.MatchWildcard {
				wildcardCount++
				if wildcardCount > 1 {
					s.error(arm.Line, arm.Col, "match allows only one wildcard '_' arm")
				}
				if wildcardIndex < 0 {
					wildcardIndex = i
				}
			} else {
				if wildcardIndex >= 0 {
					s.error(arm.Line, arm.Col, "unreachable match arm after wildcard '_'")
				}
				if stype == parser.TypeInt && arm.PatternKind != parser.MatchInt {
					s.error(arm.Line, arm.Col, "int match arm must be int literal or '_'")
				} else if stype == parser.TypeBool && arm.PatternKind != parser.MatchBool {
					s.error(arm.Line, arm.Col, "bool match arm must be true/false or '_'")
				} else if stype == parser.TypeString && arm.PatternKind != parser.MatchMatchString {
					s.error(arm.Line, arm.Col, "string match arm must be string literal or '_'")
				}
				if stype == parser.TypeBool && arm.PatternKind == parser.MatchBool {
					if arm.BoolValue {
						seenTrue = true
					} else {
						seenFalse = true
					}
				}
				// Check duplicates
				for j := 0; j < i; j++ {
					prev := st.MatchArms[j]
					if prev.PatternKind != arm.PatternKind {
						continue
					}
					if arm.PatternKind == parser.MatchInt && prev.IntValue == arm.IntValue {
						s.error(arm.Line, arm.Col, "duplicate match arm value '%d'", arm.IntValue)
						s.note(prev.Line, prev.Col, "previous arm with same value here")
					}
					if arm.PatternKind == parser.MatchBool && prev.BoolValue == arm.BoolValue {
						boolStr := "false"
						if arm.BoolValue {
							boolStr = "true"
						}
						s.error(arm.Line, arm.Col, "duplicate match arm value '%s'", boolStr)
						s.note(prev.Line, prev.Col, "previous arm with same value here")
					}
					if arm.PatternKind == parser.MatchMatchString && prev.StringValue == arm.StringValue {
						s.error(arm.Line, arm.Col, "duplicate match arm value '%s'", arm.StringValue)
						s.note(prev.Line, prev.Col, "previous arm with same value here")
					}
				}
			}

			s.pushScope()
			s.checkStmt(arm.Body)
			s.popScope()
		}

		// Exhaustiveness
		if stype == parser.TypeBool && wildcardCount == 0 && (!seenTrue || !seenFalse) {
			missing := "false"
			if !seenTrue {
				missing = "true"
			}
			s.error(st.Line, st.Col, "non-exhaustive bool match: missing '%s' arm or wildcard '_'", missing)
		}
		if stype == parser.TypeInt && wildcardCount == 0 {
			s.error(st.Line, st.Col, "non-exhaustive int match: add wildcard '_' arm to handle all cases")
		}
		if stype == parser.TypeString && wildcardCount == 0 {
			s.error(st.Line, st.Col, "non-exhaustive string match: add wildcard '_' arm to handle all cases")
		}

	case parser.StmtBreak:
		if s.loopDepth <= 0 {
			s.error(st.Line, st.Col, "'break' can only be used inside a loop")
		}
	case parser.StmtContinue:
		if s.loopDepth <= 0 {
			s.error(st.Line, st.Col, "'continue' can only be used inside a loop")
		}
	}
}

// CheckProgram runs semantic analysis on prog. Returns true if there were errors.
func CheckProgram(file, source string, prog *parser.Program) bool {
	s := &sema{
		file:      file,
		source:    source,
		maxErrors: 20,
	}

	// Collect all functions first (for forward calls)
	for i := range prog.Funcs {
		fn := &prog.Funcs[i]
		for _, existing := range s.funcs {
			if existing.name == fn.Name {
				line, col := 1, 1
				if fn.Body != nil {
					line, col = fn.Body.Line, fn.Body.Col
				}
				s.error(line, col, "duplicate function '%s'", fn.Name)
			}
		}
		s.funcs = append(s.funcs, funcSymbol{name: fn.Name, decl: fn})
	}

	// Check main signature
	mainFn := s.lookupFn("main")
	if mainFn == nil {
		s.error(1, 1, "program must define 'fn main() -> int'")
		if suggest := s.suggestFn("main"); suggest != "" {
			s.note(1, 1, "did you mean function '%s'?", suggest)
		}
	} else {
		if len(mainFn.decl.Params) != 0 || mainFn.decl.ReturnType != parser.TypeInt {
			line, col := 1, 1
			if mainFn.decl.Body != nil {
				line, col = mainFn.decl.Body.Line, mainFn.decl.Body.Col
			}
			s.error(line, col, "invalid main signature, expected 'fn main() -> int'")
		}
	}

	// Check each function body
	for i := range prog.Funcs {
		if s.stopAnal {
			break
		}
		fn := &prog.Funcs[i]
		s.currentFn = fn
		s.pushScope()
		for _, p := range fn.Params {
			line, col := 1, 1
			if fn.Body != nil {
				line, col = fn.Body.Line, fn.Body.Col
			}
			s.declareVar(line, col, p.Name, p.Type, false)
			if s.stopAnal {
				break
			}
		}
		s.checkStmt(fn.Body)
		if !s.stopAnal && fn.ReturnType != parser.TypeVoid && !stmtGuaranteesReturn(fn.Body) {
			line, col := 1, 1
			if fn.Body != nil {
				line, col = fn.Body.Line, fn.Body.Col
			}
			s.error(line, col, "non-void function '%s' may not return on all paths", fn.Name)
		}
		s.popScope()
	}

	return s.hadError
}
