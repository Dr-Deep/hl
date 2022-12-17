// subst.go
// 式の置換

package main

import (
	"fmt"
	"go/ast"
	"go/token"
)

// subst は式 expr の中に出現する vs を es で置換する関数。expr[es/vs]。
func subst(expr ast.Expr, vs []ast.Expr, es []ast.Expr) (r ast.Expr, err error) {
	switch expr.(type) {
	case *ast.BasicLit: // 定数のときはなにもしない。
		bl := expr.(*ast.BasicLit)
		switch bl.Kind {
		case token.INT, token.STRING:
		default:
			err = fmt.Errorf("subst: BasicLit: unknown Kind")
			return
		}
		r = expr
	case *ast.Ident: // 変数のとき
		ident := expr.(*ast.Ident)
		for i, v := range vs {
			if Equals(ident, v) {
				r = es[i]
				return
			}
		}
		r = expr
	case *ast.BinaryExpr:
		be := expr.(*ast.BinaryExpr)
		var x, y ast.Expr
		x, err = subst(be.X, vs, es)
		if err != nil {
			return
		}
		y, err = subst(be.Y, vs, es)
		if err != nil {
			return
		}
		r = &ast.BinaryExpr{
			X:  x,
			Op: be.Op,
			Y:  y,
		}
	case *ast.UnaryExpr:
		ue := expr.(*ast.UnaryExpr)
		var x ast.Expr
		x, err = subst(ue.X, vs, es)
		if err != nil {
			return
		}
		r = &ast.UnaryExpr{
			X:  x,
			Op: ue.Op,
		}
	case *ast.CallExpr:
		ce := expr.(*ast.CallExpr)
		ident, ok := ce.Fun.(*ast.Ident)
		if !ok {
			err = fmt.Errorf("subst: CallExpr: Fun is not Ident")
			return
		}
		switch ident.Name {
		case "Implies":
			if len(ce.Args) != 2 {
				err = fmt.Errorf("subst: CallExpr: Implies: len(Args) != 2")
				return
			}
			var left, right ast.Expr
			left, err = subst(ce.Args[0], vs, es)
			if err != nil {
				return
			}
			right, err = subst(ce.Args[1], vs, es)
			if err != nil {
				return
			}
			r = &ast.CallExpr{
				Fun: ast.NewIdent("Implies"),
				Args: []ast.Expr{
					left,
					right,
				},
			}
		case "ForAll", "Exists": // ForAll(x, int, expr)
			if len(ce.Args) != 3 {
				err = fmt.Errorf("subst: CallExpr: ForAll/Exists; len(Args) != 3")
				return
			}

			name, ok := ce.Args[0].(*ast.Ident)
			if !ok {
				err = fmt.Errorf("subst: CallExpr: ForAll/Exists; Args[0] is not Ident")
				return
			}
			typ, ok := ce.Args[1].(*ast.Ident)
			if !ok {
				err = fmt.Errorf("subst: CallExpr: ForAll/Exists; Args[1] is not Ident")
				return
			}

			if conf.Debug {
				fmt.Println(ident.Name, "vs =", vs, "es =", es, "ce.Args[2] =", ce.Args[2])
			}
			_, exVs, exEs := excludeVsEs(name, vs, es)
			if len(exVs) < 1 { // 置換する変数名がないとき
				r = expr
			} else {
				var t ast.Expr
				t, err = subst(ce.Args[2], exVs, exEs)
				if err != nil {
					return
				}
				r = &ast.CallExpr{
					Fun: ast.NewIdent(ident.Name),
					Args: []ast.Expr{
						name,
						typ,
						t,
					},
				}
			}
		default:
			err = fmt.Errorf("subst: CallExpr: Fun: Ident: unknown funcname")
			return
			// [TODO] 将来的に関数呼び出しに対応したら、関数の引数の置換を行う。
		}
	case *ast.ParenExpr:
		pe := expr.(*ast.ParenExpr)
		r, err = subst(pe.X, vs, es)
	default:
		err = fmt.Errorf("subst: unknown: %v", expr)
		printNode(expr)
	}
	return
}


// excludeVsEs は変数名リスト vs 中にふくまれる変数名 v を除外し、その変数に対応する式リストも除外する関数。
// 例：excludeVsEs("x", [a,x,y], [A,B,C]) => [a,y],[A,C]
func excludeVsEs(v ast.Expr, vs []ast.Expr, es []ast.Expr) (r bool, exVs []ast.Expr, exEs []ast.Expr) {
	for i, v2 := range vs {
		if Equals(v, v2) {
			r = true
		} else {
			exVs = append(exVs, v)
			exEs = append(exEs, es[i])
		}
	}
	return
}

// Equals は式 x と式 y が同じかどうかを調べる関数
func Equals(x, y ast.Expr) (ok bool) {
	switch x.(type) {
	case *ast.Ident:
		var xi, yi *ast.Ident
		xi = x.(*ast.Ident)
		yi, ok = y.(*ast.Ident)
		ok = ok && xi.Name == yi.Name
	case *ast.BasicLit:
		var xb, yb *ast.BasicLit
		xb = x.(*ast.BasicLit)
		yb, ok = y.(*ast.BasicLit)
		ok = ok && xb.Kind == yb.Kind && xb.Value == yb.Value
	case *ast.BinaryExpr:
		var xbe, ybe *ast.BinaryExpr
		xbe = x.(*ast.BinaryExpr)
		ybe, ok = y.(*ast.BinaryExpr)
		ok = ok && xbe.Op == ybe.Op && Equals(xbe.X, ybe.X) && Equals(xbe.Y, ybe.Y)
	case *ast.UnaryExpr:
		var xue, yue *ast.UnaryExpr
		xue = x.(*ast.UnaryExpr)
		yue, ok = y.(*ast.UnaryExpr)
		ok = ok && xue.Op == yue.Op && Equals(xue.X, yue.X)
	case *ast.CallExpr:
		var xce, yce *ast.CallExpr
		xce = x.(*ast.CallExpr)
		yce, ok = y.(*ast.CallExpr)
		if !ok || !Equals(xce.Fun, yce.Fun) {
			// y が CallExpr でないか、あるいは、関数が同じでないときは false
			break
		}
		if len(xce.Args) == len(yce.Args) {
			// 引数の個数が同じでないときは false
			break
		}
		for i := 0; i < len(xce.Args); i++ {
			if !Equals(xce.Args[i], yce.Args[i]) {
				// 一つでも引数が同じでないときは false
				break
			}
		}
		ok = true
	}
	return
}
