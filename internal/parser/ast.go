package parser

// TypeKind represents a type in the ngawi type system.
type TypeKind int

const (
	TypeInt    TypeKind = 0
	TypeFloat  TypeKind = 1
	TypeBool   TypeKind = 2
	TypeString TypeKind = 3
	TypeVoid   TypeKind = 4

	typeArrayBase   TypeKind = 100
	typeArrayStride TypeKind = 16
)

func TypeIsArray(t TypeKind) bool { return t >= typeArrayBase }

func TypeArrayDepth(t TypeKind) int {
	if !TypeIsArray(t) {
		return 0
	}
	return int(t-typeArrayBase) % int(typeArrayStride)
}

func TypeArrayBase(t TypeKind) TypeKind {
	if !TypeIsArray(t) {
		return t
	}
	return TypeKind(int(t-typeArrayBase) / int(typeArrayStride))
}

func TypeMakeArray(base TypeKind, depth int) TypeKind {
	return typeArrayBase + TypeKind(int(base)*int(typeArrayStride)) + TypeKind(depth)
}

var (
	TypeIntArray    = TypeMakeArray(TypeInt, 1)
	TypeInt2Array   = TypeMakeArray(TypeInt, 2)
	TypeFloatArray  = TypeMakeArray(TypeFloat, 1)
	TypeFloat2Array = TypeMakeArray(TypeFloat, 2)
	TypeBoolArray   = TypeMakeArray(TypeBool, 1)
	TypeBool2Array  = TypeMakeArray(TypeBool, 2)
	TypeStringArray  = TypeMakeArray(TypeString, 1)
	TypeString2Array = TypeMakeArray(TypeString, 2)
)

func TypeKindName(t TypeKind) string {
	if TypeIsArray(t) {
		base := TypeArrayBase(t)
		depth := TypeArrayDepth(t)
		var baseName string
		switch base {
		case TypeInt:
			baseName = "int"
		case TypeFloat:
			baseName = "float"
		case TypeBool:
			baseName = "bool"
		case TypeString:
			baseName = "string"
		default:
			baseName = "<unknown>"
		}
		s := baseName
		for i := 0; i < depth; i++ {
			s += "[]"
		}
		return s
	}
	switch t {
	case TypeInt:
		return "int"
	case TypeFloat:
		return "float"
	case TypeBool:
		return "bool"
	case TypeString:
		return "string"
	case TypeVoid:
		return "void"
	}
	return "<unknown>"
}

func TypeIsNumeric(t TypeKind) bool { return t == TypeInt || t == TypeFloat }

// ExprKind identifies the kind of an expression node.
type ExprKind int

const (
	ExprInt ExprKind = iota
	ExprFloat
	ExprString
	ExprBool
	ExprIdent
	ExprArrayLiteral
	ExprIndex
	ExprUnary
	ExprBinary
	ExprCall
)

// Expr is an expression AST node.
type Expr struct {
	Kind         ExprKind
	Line, Col    int
	InferredType TypeKind

	// Only one of these is set, depending on Kind:
	IntVal      int64
	FloatVal    float64
	BoolVal     bool
	StringVal   string
	IdentName   string
	ArrayItems  []*Expr
	IndexTarget *Expr
	IndexIdx    *Expr
	UnaryOp     int // uses lexer token kinds as ints
	UnaryExpr   *Expr
	BinaryOp    int
	BinaryLeft  *Expr
	BinaryRight *Expr
	CallName    string
	CallArgs    []*Expr
}

// StmtKind identifies the kind of a statement node.
type StmtKind int

const (
	StmtBlock StmtKind = iota
	StmtVarDecl
	StmtConstDecl
	StmtAssign
	StmtIndexAssign
	StmtExpr
	StmtReturn
	StmtIf
	StmtWhile
	StmtFor
	StmtMatch
	StmtBreak
	StmtContinue
)

// MatchPatternKind identifies the pattern in a match arm.
type MatchPatternKind int

const (
	MatchWildcard MatchPatternKind = iota
	MatchInt
	MatchBool
	MatchMatchString
)

// MatchArm is one arm in a match statement.
type MatchArm struct {
	PatternKind MatchPatternKind
	IntValue    int64
	BoolValue   bool
	StringValue string
	Line, Col   int
	Body        *Stmt
}

// Stmt is a statement AST node.
type Stmt struct {
	Kind      StmtKind
	Line, Col int

	// Block
	BlockItems []*Stmt

	// VarDecl / ConstDecl
	DeclName    string
	DeclHasType bool
	DeclType    TypeKind
	DeclInit    *Expr

	// Assign
	AssignName  string
	AssignValue *Expr

	// IndexAssign
	IndexAssignTarget *Expr
	IndexAssignIndex  *Expr
	IndexAssignValue  *Expr

	// Expr stmt
	ExprValue *Expr

	// Return
	RetHasExpr bool
	RetExpr    *Expr

	// If
	IfCond       *Expr
	IfThen       *Stmt
	IfElse       *Stmt

	// While
	WhileCond *Expr
	WhileBody *Stmt

	// For
	ForInit   *Stmt
	ForCond   *Expr
	ForUpdate *Stmt
	ForBody   *Stmt

	// Match
	MatchSubject *Expr
	MatchArms    []MatchArm
}

// Param is a function parameter.
type Param struct {
	Name string
	Type TypeKind
}

// FunctionDecl is a function declaration.
type FunctionDecl struct {
	Name       string
	Params     []Param
	ReturnType TypeKind
	Body       *Stmt
}

// ImportDecl is an import declaration.
type ImportDecl struct {
	Path      string
	Line, Col int
}

// Program is the top-level AST node.
type Program struct {
	Imports []ImportDecl
	Funcs   []FunctionDecl
}
