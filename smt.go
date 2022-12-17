// Golang AST の式から SMT LIB Language 仕様への変換

package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// makeSMTScript は Golang AST の式から SMT LIB Language 仕様のスクリプトを作成する関数
func makeSMTScript(vars map[string]ast.Expr, cond ast.Expr) (r string) {
	var tmp []string
	tmp = append(tmp, convVars(vars))
	tmp = append(tmp, fmt.Sprintf("(assert %s)", convExpr(cond)))
	tmp = append(tmp, "(check-sat)")
	tmp = append(tmp, "(get-model)")
	r = strings.Join(tmp, "\n") + "\n"
	// [MEMO] r の末尾の LF は Z3 での実行では不要かもしれないが、
	// 他の事例において末尾の LF がないとうまくいかないことがあった。
	return
}

// runSMTScript は SMT LIB Language 仕様のスクリプトを実行する関数
// popen.go 参照。
func runSMTScript(script string) (result, outText, errText string, err error) {
	if conf.Debug {
		fmt.Println("# cmd =", conf.Cmd)
		fmt.Println("# timeOutSec =", conf.TimeOutSec)
	}
	outText, errText, err = runCmd(conf.Cmd, script, conf.TimeOutSec)
	t := strings.Split(outText, "\n")
	result = strings.TrimSpace(t[0])
	outText = strings.TrimSpace(strings.Join(t[1:], "\n"))
	errText = strings.TrimSpace(errText)
	if conf.Debug {
		fmt.Println("# result =", result)
		fmt.Println("# outText =", outText)
		fmt.Println("# errText =", errText)
		fmt.Println("# err =", err)
	}
	return
}

// convVars は変数宣言を SMT LIB Language 仕様のコードを作成する関数
func convVars(vars map[string]ast.Expr) (r string) {
	var decls []string
	for name, typ := range vars {
		decls = append(decls, fmt.Sprintf("(declare-const %s %s)", name, convType(typ)))
	}
	r = strings.Join(decls, "\n")
	return
}

// convType は型名変換する関数
func convType(typ ast.Expr) (r string) {
	switch typ.(type) {
	case *ast.Ident:
		ident := typ.(*ast.Ident)
		switch ident.Name {
		case "int":
			r = "Int"
		case "bool":
			r = "Bool"
		case "string":
			r = "String"
		default:
			r = "unknown"
		}
	case *ast.ArrayType:
		at := typ.(*ast.ArrayType)
		r = fmt.Sprintf("(Array Int %s)", convType(at.Elt))
	default:
		r = "unknown"
	}
	return
}

// convOp は Golang の演算子を SMT LIB Language 仕様の演算子名に変換する関数
func convOp(op token.Token) (r string) {
	switch op {
	case token.ADD:
		r = "+"
	case token.SUB:
		r = "-"
	case token.MUL:
		r = "*"
	case token.QUO:
		r = "div"
	case token.REM:
		r = "mod"
	case token.LAND:
		r = "and"
	case token.LOR:
		r = "or"
	case token.NOT:
		r = "not"
	case token.EQL, token.NEQ:
		r = "="
	case token.LSS:
		r = "<"
	case token.GTR:
		r = ">"
	case token.LEQ:
		r = "<="
	case token.GEQ:
		r = ">="
	default:
		r = "unknown"
	}
	return
}

// convExpr は Golang の式の AST を SMT LIB Language 仕様の式のコードに変換する関数
func convExpr(expr ast.Expr) (r string) {
	switch expr.(type) {
	case *ast.BasicLit:
		bl := expr.(*ast.BasicLit)
		r = bl.Value
	case *ast.Ident:
		ident := expr.(*ast.Ident)
		r = ident.Name
	case *ast.CallExpr:
		ce := expr.(*ast.CallExpr)
		ident := ce.Fun.(*ast.Ident)
		switch ident.Name {
		case "Implies":
			r = fmt.Sprintf("(=> %s %s)", convExpr(ce.Args[0]), convExpr(ce.Args[1]))
		case "ForAll":
			nam := ce.Args[0].(*ast.Ident)
			//typ := ce.Args[1].(*ast.Ident)
			r = fmt.Sprintf("(forall ((%s %s)) %s)", nam.Name, convType(ce.Args[1]), convExpr(ce.Args[2]))
		case "Exists":
			nam := ce.Args[0].(*ast.BasicLit)
			//typ := ce.Args[1].(*ast.BasicLit)
			r = fmt.Sprintf("(exists ((%s %s)) %s)", nam.Value[1:len(nam.Value)-1], convType(ce.Args[1]), convExpr(ce.Args[2]))
		case "Select":
			// (select 配列 インデクス) v[i]
			panic("select is not implemented yet")
		case "Store":
			// (store  配列 インデクス 式) v(i := e)
			panic("store is not implemented yet")
		default:
			printNode(expr)
			panic("unknown expr")
		}
	case *ast.BinaryExpr:
		be := expr.(*ast.BinaryExpr)
		r = fmt.Sprintf("(%s %s %s)", convOp(be.Op), convExpr(be.X), convExpr(be.Y))
		if be.Op == token.NEQ {
			r = fmt.Sprintf("(not %s)", r)
		}
	case *ast.UnaryExpr:
		ue := expr.(*ast.UnaryExpr)
		r = fmt.Sprintf("(%s %s)", convOp(ue.Op), convExpr(ue.X))
	case *ast.ParenExpr:
		pe := expr.(*ast.ParenExpr)
		r = convExpr(pe.X)
		// [MEMO] 括弧をむいて中身を出す感じ。
		// 下のようにすると余計な括弧のせいで Z3 はエラーになる。
		// r = fmt.Sprintf("(%s)", convExpr(pe.X))
	default:
		// subst でチェックしているので上記以外のケースはないはずだが、念のため。
		printNode(expr)
		panic("unknown expr")
	}
	return
}
