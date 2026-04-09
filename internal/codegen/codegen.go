package codegen

import (
	"fmt"
	"os"
	"strings"

	"ngawi/internal/lexer"
	"ngawi/internal/parser"
)

type cgen struct {
	sb     strings.Builder
	indent int
}

func (g *cgen) emitIndent() {
	for i := 0; i < g.indent; i++ {
		g.sb.WriteString("  ")
	}
}

func (g *cgen) emit(s string) {
	g.sb.WriteString(s)
}

func (g *cgen) emitf(format string, args ...interface{}) {
	fmt.Fprintf(&g.sb, format, args...)
}

func cType(t parser.TypeKind) string {
	switch t {
	case parser.TypeInt:
		return "int64_t"
	case parser.TypeFloat:
		return "double"
	case parser.TypeBool:
		return "bool"
	case parser.TypeString:
		return "const char *"
	case parser.TypeIntArray:
		return "ng_int_array_t"
	case parser.TypeInt2Array:
		return "ng_int2_array_t"
	case parser.TypeFloatArray:
		return "ng_float_array_t"
	case parser.TypeFloat2Array:
		return "ng_float2_array_t"
	case parser.TypeBoolArray:
		return "ng_bool_array_t"
	case parser.TypeBool2Array:
		return "ng_bool2_array_t"
	case parser.TypeStringArray:
		return "ng_string_array_t"
	case parser.TypeString2Array:
		return "ng_string2_array_t"
	case parser.TypeVoid:
		return "void"
	default:
		return "void"
	}
}

func opText(op int) string {
	switch lexer.TokenKind(op) {
	case lexer.TOK_PLUS:
		return "+"
	case lexer.TOK_MINUS:
		return "-"
	case lexer.TOK_STAR:
		return "*"
	case lexer.TOK_SLASH:
		return "/"
	case lexer.TOK_PERCENT:
		return "%"
	case lexer.TOK_EQ:
		return "=="
	case lexer.TOK_NE:
		return "!="
	case lexer.TOK_LT:
		return "<"
	case lexer.TOK_LE:
		return "<="
	case lexer.TOK_GT:
		return ">"
	case lexer.TOK_GE:
		return ">="
	case lexer.TOK_AND_AND:
		return "&&"
	case lexer.TOK_OR_OR:
		return "||"
	case lexer.TOK_BANG:
		return "!"
	}
	return "?"
}

func emitStringEscaped(sb *strings.Builder, s string) {
	sb.WriteByte('"')
	for _, c := range s {
		switch c {
		case '"':
			sb.WriteString("\\\"")
		case '\\':
			sb.WriteString("\\\\")
		case '\n':
			sb.WriteString("\\n")
		case '\t':
			sb.WriteString("\\t")
		default:
			sb.WriteRune(c)
		}
	}
	sb.WriteByte('"')
}

func isPrintCall(e *parser.Expr) bool {
	return e != nil && e.Kind == parser.ExprCall && e.CallName == "print"
}

func (g *cgen) emitPrintStmt(call *parser.Expr) {
	for i, arg := range call.CallArgs {
		g.emitIndent()
		switch arg.InferredType {
		case parser.TypeInt:
			g.emit("ng_print_int(")
			g.emitExpr(arg)
			g.emit(");\n")
		case parser.TypeFloat:
			g.emit("ng_print_float(")
			g.emitExpr(arg)
			g.emit(");\n")
		case parser.TypeBool:
			g.emit("ng_print_bool(")
			g.emitExpr(arg)
			g.emit(");\n")
		case parser.TypeString:
			g.emit("ng_print_string(")
			g.emitExpr(arg)
			g.emit(");\n")
		default:
			g.emit("/* invalid print arg */\n")
		}
		if i+1 < len(call.CallArgs) {
			g.emitIndent()
			g.emit("ng_print_string(\" \");\n")
		}
	}
	g.emitIndent()
	g.emit("ng_print_string(\"\\n\");\n")
}

func (g *cgen) emitExpr(e *parser.Expr) {
	switch e.Kind {
	case parser.ExprInt:
		g.emitf("%d", e.IntVal)
	case parser.ExprFloat:
		g.emitf("%.17g", e.FloatVal)
	case parser.ExprString:
		emitStringEscaped(&g.sb, e.StringVal)
	case parser.ExprBool:
		if e.BoolVal {
			g.emit("true")
		} else {
			g.emit("false")
		}
	case parser.ExprIdent:
		g.emit(e.IdentName)

	case parser.ExprArrayLiteral:
		arrType := "ng_int_array_t"
		elemType := "int64_t"
		switch e.InferredType {
		case parser.TypeInt2Array:
			arrType, elemType = "ng_int2_array_t", "ng_int_array_t"
		case parser.TypeFloatArray:
			arrType, elemType = "ng_float_array_t", "double"
		case parser.TypeFloat2Array:
			arrType, elemType = "ng_float2_array_t", "ng_float_array_t"
		case parser.TypeBoolArray:
			arrType, elemType = "ng_bool_array_t", "bool"
		case parser.TypeBool2Array:
			arrType, elemType = "ng_bool2_array_t", "ng_bool_array_t"
		case parser.TypeStringArray:
			arrType, elemType = "ng_string_array_t", "const char *"
		case parser.TypeString2Array:
			arrType, elemType = "ng_string2_array_t", "ng_string_array_t"
		}
		g.emitf("(%s){.data=(%s[]){", arrType, elemType)
		for i, item := range e.ArrayItems {
			if i > 0 {
				g.emit(", ")
			}
			g.emitExpr(item)
		}
		g.emitf("}, .len=%d}", len(e.ArrayItems))

	case parser.ExprIndex:
		switch e.InferredType {
		case parser.TypeInt:
			g.emit("ng_int_array_get(")
		case parser.TypeIntArray:
			g.emit("ng_int2_array_get(")
		case parser.TypeFloat:
			g.emit("ng_float_array_get(")
		case parser.TypeFloatArray:
			g.emit("ng_float2_array_get(")
		case parser.TypeBool:
			g.emit("ng_bool_array_get(")
		case parser.TypeBoolArray:
			g.emit("ng_bool2_array_get(")
		case parser.TypeStringArray:
			g.emit("ng_string2_array_get(")
		default:
			g.emit("ng_string_array_get(")
		}
		g.emitExpr(e.IndexTarget)
		g.emit(", (int64_t)(")
		g.emitExpr(e.IndexIdx)
		g.emit("))")

	case parser.ExprUnary:
		g.emit("(")
		g.emit(opText(e.UnaryOp))
		g.emitExpr(e.UnaryExpr)
		g.emit(")")

	case parser.ExprBinary:
		// String equality
		if (e.BinaryOp == int(lexer.TOK_EQ) || e.BinaryOp == int(lexer.TOK_NE)) &&
			e.BinaryLeft != nil && e.BinaryRight != nil &&
			e.BinaryLeft.InferredType == parser.TypeString &&
			e.BinaryRight.InferredType == parser.TypeString {
			if e.BinaryOp == int(lexer.TOK_NE) {
				g.emit("(!")
			}
			g.emit("ng_string_eq(")
			g.emitExpr(e.BinaryLeft)
			g.emit(", ")
			g.emitExpr(e.BinaryRight)
			g.emit(")")
			if e.BinaryOp == int(lexer.TOK_NE) {
				g.emit(")")
			}
			break
		}
		// String concat
		if e.BinaryOp == int(lexer.TOK_PLUS) && e.BinaryLeft != nil && e.BinaryRight != nil &&
			e.BinaryLeft.InferredType == parser.TypeString &&
			e.BinaryRight.InferredType == parser.TypeString {
			g.emit("ng_string_concat(")
			g.emitExpr(e.BinaryLeft)
			g.emit(", ")
			g.emitExpr(e.BinaryRight)
			g.emit(")")
			break
		}
		g.emit("(")
		g.emitExpr(e.BinaryLeft)
		g.emitf(" %s ", opText(e.BinaryOp))
		g.emitExpr(e.BinaryRight)
		g.emit(")")

	case parser.ExprCall:
		name := e.CallName
		args := e.CallArgs
		switch {
		case (name == "to_int" || name == "to_amba") && len(args) == 1:
			g.emit("((int64_t)(")
			g.emitExpr(args[0])
			g.emit("))")
		case (name == "to_float" || name == "to_rusdi") && len(args) == 1:
			g.emit("((double)(")
			g.emitExpr(args[0])
			g.emit("))")
		case name == "len" && len(args) == 1:
			arg := args[0]
			if parser.TypeIsArray(arg.InferredType) {
				g.emit("((")
				g.emitExpr(arg)
				g.emit(").len)")
			} else {
				g.emit("ng_string_len(")
				g.emitExpr(arg)
				g.emit(")")
			}
		case name == "push" && len(args) == 2:
			at := args[0].InferredType
			switch at {
			case parser.TypeIntArray:
				g.emit("ng_int_array_push(")
			case parser.TypeInt2Array:
				g.emit("ng_int2_array_push(")
			case parser.TypeFloatArray:
				g.emit("ng_float_array_push(")
			case parser.TypeFloat2Array:
				g.emit("ng_float2_array_push(")
			case parser.TypeBoolArray:
				g.emit("ng_bool_array_push(")
			case parser.TypeBool2Array:
				g.emit("ng_bool2_array_push(")
			case parser.TypeStringArray:
				g.emit("ng_string_array_push(")
			default:
				g.emit("ng_string2_array_push(")
			}
			g.emitExpr(args[0])
			g.emit(", ")
			g.emitExpr(args[1])
			g.emit(")")
		case name == "pop" && len(args) == 1:
			at := args[0].InferredType
			switch at {
			case parser.TypeIntArray:
				g.emit("ng_int_array_pop(")
			case parser.TypeInt2Array:
				g.emit("ng_int2_array_pop(")
			case parser.TypeFloatArray:
				g.emit("ng_float_array_pop(")
			case parser.TypeFloat2Array:
				g.emit("ng_float2_array_pop(")
			case parser.TypeBoolArray:
				g.emit("ng_bool_array_pop(")
			case parser.TypeBool2Array:
				g.emit("ng_bool2_array_pop(")
			case parser.TypeStringArray:
				g.emit("ng_string_array_pop(")
			default:
				g.emit("ng_string2_array_pop(")
			}
			g.emitExpr(args[0])
			g.emit(")")
		case name == "contains" && len(args) == 2:
			g.emit("ng_string_contains(")
			g.emitExpr(args[0])
			g.emit(", ")
			g.emitExpr(args[1])
			g.emit(")")
		case name == "starts_with" && len(args) == 2:
			g.emit("ng_string_starts_with(")
			g.emitExpr(args[0])
			g.emit(", ")
			g.emitExpr(args[1])
			g.emit(")")
		case name == "ends_with" && len(args) == 2:
			g.emit("ng_string_ends_with(")
			g.emitExpr(args[0])
			g.emit(", ")
			g.emitExpr(args[1])
			g.emit(")")
		case name == "to_lower" && len(args) == 1:
			g.emit("ng_string_to_lower(")
			g.emitExpr(args[0])
			g.emit(")")
		case name == "to_upper" && len(args) == 1:
			g.emit("ng_string_to_upper(")
			g.emitExpr(args[0])
			g.emit(")")
		case name == "trim" && len(args) == 1:
			g.emit("ng_string_trim(")
			g.emitExpr(args[0])
			g.emit(")")
		default:
			g.emit(name)
			g.emit("(")
			for i, arg := range args {
				if i > 0 {
					g.emit(", ")
				}
				g.emitExpr(arg)
			}
			g.emit(")")
		}
	}
}

func (g *cgen) emitBlock(blk *parser.Stmt) {
	g.emitIndent()
	g.emit("{\n")
	g.indent++
	for _, item := range blk.BlockItems {
		g.emitStmt(item)
	}
	g.indent--
	g.emitIndent()
	g.emit("}\n")
}

func (g *cgen) emitForClauseStmt(st *parser.Stmt) {
	if st == nil {
		return
	}
	switch st.Kind {
	case parser.StmtVarDecl, parser.StmtConstDecl:
		if st.Kind == parser.StmtConstDecl && st.DeclType != parser.TypeString {
			g.emit("const ")
		}
		g.emit(cType(st.DeclType))
		g.emitf(" %s = ", st.DeclName)
		g.emitExpr(st.DeclInit)
	case parser.StmtAssign:
		g.emitf("%s = ", st.AssignName)
		g.emitExpr(st.AssignValue)
	case parser.StmtIndexAssign:
		g.emitIndexAssign(st)
	case parser.StmtExpr:
		g.emitExpr(st.ExprValue)
	}
}

func (g *cgen) emitIndexAssign(st *parser.Stmt) {
	target := st.IndexAssignTarget
	if target.Kind == parser.ExprIdent {
		switch target.InferredType {
		case parser.TypeIntArray:
			g.emit("ng_int_array_set(&")
		case parser.TypeInt2Array:
			g.emit("ng_int2_array_set(&")
		case parser.TypeFloatArray:
			g.emit("ng_float_array_set(&")
		case parser.TypeFloat2Array:
			g.emit("ng_float2_array_set(&")
		case parser.TypeBoolArray:
			g.emit("ng_bool_array_set(&")
		case parser.TypeBool2Array:
			g.emit("ng_bool2_array_set(&")
		case parser.TypeStringArray:
			g.emit("ng_string_array_set(&")
		default:
			g.emit("ng_string2_array_set(&")
		}
		g.emit(target.IdentName)
		g.emit(", (int64_t)(")
		g.emitExpr(st.IndexAssignIndex)
		g.emit("), ")
		g.emitExpr(st.IndexAssignValue)
		g.emit(")")
		return
	}

	// 2D: target is EXPR_INDEX where target.IndexTarget is EXPR_IDENT
	if target.Kind == parser.ExprIndex && target.IndexTarget != nil &&
		target.IndexTarget.Kind == parser.ExprIdent {
		base := target.IndexTarget.IdentName
		rowIdx := target.IndexIdx
		switch target.InferredType {
		case parser.TypeIntArray:
			g.emit("ng_int_array_set(&")
		case parser.TypeFloatArray:
			g.emit("ng_float_array_set(&")
		case parser.TypeBoolArray:
			g.emit("ng_bool_array_set(&")
		default:
			g.emit("ng_string_array_set(&")
		}
		g.emitf("%s.data[ng_array_checked_index((int64_t)(", base)
		g.emitExpr(rowIdx)
		g.emitf("), %s.len)]", base)
		g.emit(", (int64_t)(")
		g.emitExpr(st.IndexAssignIndex)
		g.emit("), ")
		g.emitExpr(st.IndexAssignValue)
		g.emit(")")
		return
	}

	g.emit("/* invalid indexed assignment target */")
}

func (g *cgen) emitStmt(st *parser.Stmt) {
	switch st.Kind {
	case parser.StmtBlock:
		g.emitBlock(st)

	case parser.StmtVarDecl, parser.StmtConstDecl:
		g.emitIndent()
		if st.Kind == parser.StmtConstDecl && st.DeclType != parser.TypeString {
			g.emit("const ")
		}
		g.emitf("%s %s = ", cType(st.DeclType), st.DeclName)
		g.emitExpr(st.DeclInit)
		g.emit(";\n")

	case parser.StmtAssign:
		g.emitIndent()
		g.emitf("%s = ", st.AssignName)
		g.emitExpr(st.AssignValue)
		g.emit(";\n")

	case parser.StmtIndexAssign:
		g.emitIndent()
		g.emitIndexAssign(st)
		g.emit(";\n")

	case parser.StmtExpr:
		if isPrintCall(st.ExprValue) {
			g.emitPrintStmt(st.ExprValue)
		} else {
			g.emitIndent()
			g.emitExpr(st.ExprValue)
			g.emit(";\n")
		}

	case parser.StmtReturn:
		g.emitIndent()
		g.emit("return")
		if st.RetHasExpr {
			g.emit(" ")
			g.emitExpr(st.RetExpr)
		}
		g.emit(";\n")

	case parser.StmtIf:
		g.emitIndent()
		g.emit("if (")
		g.emitExpr(st.IfCond)
		g.emit(")\n")
		g.emitStmt(st.IfThen)
		if st.IfElse != nil {
			g.emitIndent()
			g.emit("else\n")
			g.emitStmt(st.IfElse)
		}

	case parser.StmtWhile:
		g.emitIndent()
		g.emit("while (")
		g.emitExpr(st.WhileCond)
		g.emit(")\n")
		g.emitStmt(st.WhileBody)

	case parser.StmtFor:
		g.emitIndent()
		g.emit("for (")
		g.emitForClauseStmt(st.ForInit)
		g.emit("; ")
		if st.ForCond != nil {
			g.emitExpr(st.ForCond)
		}
		g.emit("; ")
		g.emitForClauseStmt(st.ForUpdate)
		g.emit(")\n")
		g.emitStmt(st.ForBody)

	case parser.StmtMatch:
		if st.MatchSubject.InferredType == parser.TypeString {
			g.emitIndent()
			g.emitf("const char *__ng_match_s_%d_%d = ", st.Line, st.Col)
			g.emitExpr(st.MatchSubject)
			g.emit(";\n")

			var wildcardArm *parser.MatchArm
			emittedAny := false

			for i := range st.MatchArms {
				arm := &st.MatchArms[i]
				if arm.PatternKind == parser.MatchWildcard {
					wildcardArm = arm
					continue
				}
				g.emitIndent()
				if !emittedAny {
					g.emitf("if (ng_string_eq(__ng_match_s_%d_%d, ", st.Line, st.Col)
				} else {
					g.emitf("else if (ng_string_eq(__ng_match_s_%d_%d, ", st.Line, st.Col)
				}
				emitStringEscaped(&g.sb, arm.StringValue)
				g.emit("))\n")
				g.emitStmt(arm.Body)
				emittedAny = true
			}
			if wildcardArm != nil {
				if emittedAny {
					g.emitIndent()
					g.emit("else\n")
					g.emitStmt(wildcardArm.Body)
				} else {
					g.emitStmt(wildcardArm.Body)
				}
			}
		} else {
			g.emitIndent()
			g.emit("switch (")
			g.emitExpr(st.MatchSubject)
			g.emit(")\n")
			g.emitIndent()
			g.emit("{\n")
			g.indent++
			for i := range st.MatchArms {
				arm := &st.MatchArms[i]
				g.emitIndent()
				switch arm.PatternKind {
				case parser.MatchWildcard:
					g.emit("default:\n")
				case parser.MatchInt:
					g.emitf("case %d:\n", arm.IntValue)
				case parser.MatchBool:
					if arm.BoolValue {
						g.emit("case 1:\n")
					} else {
						g.emit("case 0:\n")
					}
				}
				g.emitIndent()
				g.emit("{\n")
				g.indent++
				g.emitStmt(arm.Body)
				if arm.Body.Kind != parser.StmtReturn &&
					arm.Body.Kind != parser.StmtBreak &&
					arm.Body.Kind != parser.StmtContinue {
					g.emitIndent()
					g.emit("break;\n")
				}
				g.indent--
				g.emitIndent()
				g.emit("}\n")
			}
			g.indent--
			g.emitIndent()
			g.emit("}\n")
		}

	case parser.StmtBreak:
		g.emitIndent()
		g.emit("break;\n")
	case parser.StmtContinue:
		g.emitIndent()
		g.emit("continue;\n")
	}
}

// EmitC generates a C source file from the given program and writes it to outPath.
func EmitC(inputFile string, prog *parser.Program, outPath string) error {
	g := &cgen{}

	g.emit("#include <stdbool.h>\n")
	g.emit("#include <stdint.h>\n")
	g.emit("#include \"ngawi_runtime.h\"\n\n")
	g.emitf("/* Generated from %s */\n\n", inputFile)

	// Forward declarations
	for i := range prog.Funcs {
		fn := &prog.Funcs[i]
		if fn.Name == "main" {
			g.emit("int main(void);\n")
			continue
		}
		g.emitf("%s %s(", cType(fn.ReturnType), fn.Name)
		for j, p := range fn.Params {
			if j > 0 {
				g.emit(", ")
			}
			g.emitf("%s %s", cType(p.Type), p.Name)
		}
		g.emit(");\n")
	}
	g.emit("\n")

	// Function definitions
	for i := range prog.Funcs {
		fn := &prog.Funcs[i]
		if fn.Name == "main" {
			g.emit("int main(void)\n")
		} else {
			g.emitf("%s %s(", cType(fn.ReturnType), fn.Name)
			for j, p := range fn.Params {
				if j > 0 {
					g.emit(", ")
				}
				g.emitf("%s %s", cType(p.Type), p.Name)
			}
			g.emit(")\n")
		}
		g.emitStmt(fn.Body)
		g.emit("\n")
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("%s:1:1: error: cannot write C output '%s': %w", inputFile, outPath, err)
	}
	defer f.Close()
	_, err = f.WriteString(g.sb.String())
	return err
}
