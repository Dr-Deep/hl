// wp.go
// 最弱事前条件 (Weekest Precondition)

package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"math/rand"
	"os"
	"time"
)

// getCondTobeVerified は指定された関数定義より、検証すべき条件式のリストと変数名のリストを取得する関数。
func getCondTobeVerified(f *ast.FuncDecl) (r []ast.Expr, vars map[string]ast.Expr, preCond, postCond ast.Expr, err error) {

	if len(f.Body.List) < 2 {
		// 関数定義に文がないときはエラー
		err = fmt.Errorf("wpFunc: len(f.Body.List) < 2")
		return
	}

	// 事前条件 preCond と事後条件 postCond の抽出。それ以外は stmts に格納
	var asserts map[string]ast.Expr
	var stmts []ast.Stmt
	asserts, stmts, err = separateStmts(f.Body.List)
	if err != nil {
		return
	}
	preCond = asserts["PRE"]
	postCond = asserts["POST"]

	if preCond == nil {
		err = fmt.Errorf("wpFunc: no PRE")
		return
	}

	if postCond == nil {
		err = fmt.Errorf("wpFunc: no POST")
		return
	}

	// 事後条件をもとにすべての文から最弱事前条件を求める
	var wp ast.Expr
	var acc []ast.Expr
	vars = getFuncVars(f.Type)

	wp, err = wpStmts(&acc, vars, stmts, postCond)
	if err != nil {
		return
	}

	if conf.Debug {
		fmt.Print("#getCondTobeVerified: wp: ")
		printNode(wp)
		fmt.Print("#getCondTobeVerified: wp: ")
		format.Node(os.Stdout, token.NewFileSet(), wp)
		fmt.Println("")
	}

	r = append(r, astNot(astImplies(preCond, wp)))

	// ループに関する追加条件をNotして追加
	for _, cond := range acc {
		r = append(r, astNot(cond))
	}

	return
}

// separateStmts は表明文とその他の文を分離する関数
func separateStmts(stmts []ast.Stmt) (asserts map[string]ast.Expr, stmts2 []ast.Stmt, err error) {
	asserts = map[string]ast.Expr{}
	for _, stmt := range stmts {
		// 文が PRE文もしくはPOST文かをチェックする
		tag, cond, ok := isAssertStmt(stmt)
		if ok {
			if asserts[tag] != nil {
				// 同じ表明文が二度出現するときはエラー
				err = fmt.Errorf("separateStmts: multi occurence of %s", tag)
				return
			}
			asserts[tag] = cond

		} else {
			// 表明文(PRE/POST/INV)ではないときは stmts2 に追加
			stmts2 = append(stmts2, stmt)
		}
	}
	return
}

// isAssertStmt は文が表明文(PRE/POST/INV)かチェックする関数
func isAssertStmt(stmt ast.Stmt) (tag string, cond ast.Expr, ok bool) {
	var es *ast.ExprStmt
	es, ok = stmt.(*ast.ExprStmt)
	if !ok { // stmt が ExprStmt でなければエラー
		return
	}
	ok = false
	var ce *ast.CallExpr
	ce, ok = es.X.(*ast.CallExpr)
	if !ok { // es.X が CallExpr でなければエラー
		return
	}
	ok = false
	var ident *ast.Ident
	ident, ok = ce.Fun.(*ast.Ident)
	if !ok { // ce.Fun が Ident でなければエラー
		return
	}
	ok = false

	switch ident.Name {
	case "PRE", "POST", "INV":
		tag = ident.Name
		cond, ok = isStrLit(ce.Args[0])
	}
	return
}

// isStrLit は式が文字列リテラルかどうか調べる関数。
// 文字列リテラルの場合には ok には true、r は文字列をパースした結果が返る。
func isStrLit(expr ast.Expr) (r ast.Expr, ok bool) {
	var bl *ast.BasicLit
	bl, ok = expr.(*ast.BasicLit)
	if !ok { // expr が BasicLit でなければエラー
		return
	}
	ok = false
	if bl.Kind != token.STRING { // 文字列以外ならエラー
		return
	}
	var err error
	// bl.Value はダブルコーテーションで囲まれている。
	r, err = parser.ParseExpr(bl.Value[1 : len(bl.Value)-1])
	if err != nil { // パースに失敗
		fmt.Fprintln(os.Stderr, err)
		return
	}
	ok = true
	return
}

// wpStmts は文のリストから最弱事前条件と関数リストと追加検証条件式を作成する関数。
func wpStmts(acc *[]ast.Expr, vars map[string]ast.Expr, stmts []ast.Stmt, postCond ast.Expr) (pre ast.Expr, err error) {
	pre = postCond

	// 後ろから見ていく
	for i := len(stmts) - 1; i >= 0; i-- {
		pre, err = wpStmt(acc, vars, stmts[i], pre)
		if err != nil {
			break
		}
	}
	return
}

// wpStmts は文から最弱事前条件と関数リストと追加検証条件式を作成する関数。
func wpStmt(acc *[]ast.Expr, vars map[string]ast.Expr, stmt ast.Stmt, postCond ast.Expr) (pre ast.Expr, err error) {
	if conf.Debug {
		fmt.Print("#wpStmt: stmt:")
		format.Node(os.Stdout, token.NewFileSet(), stmt)
		fmt.Println("")
	}
	switch stmt.(type) {
	case *ast.AssignStmt:
		pre, err = wpAssignStmt(acc, vars, stmt.(*ast.AssignStmt), postCond)
	case *ast.IfStmt:
		pre, err = wpIfStmt(acc, vars, stmt.(*ast.IfStmt), postCond)
	case *ast.ForStmt:
		pre, err = wpForStmt(acc, vars, stmt.(*ast.ForStmt), postCond)
	case *ast.BlockStmt:
		s := stmt.(*ast.BlockStmt)
		pre, err = wpStmts(acc, vars, s.List, postCond)
	case *ast.DeclStmt:
		pre, err = wpDeclStmt(acc, vars, stmt.(*ast.DeclStmt), postCond)
	case *ast.ReturnStmt:
		// 受容するがスルーするもの
		pre = postCond
	case *ast.ExprStmt:
		es := stmt.(*ast.ExprStmt)
		switch es.X.(type) {
		case *ast.CallExpr:
			ce := es.X.(*ast.CallExpr)
			pre, err = wpFunCall2(ce, postCond)
		default:
			err = fmt.Errorf("wpStmt: ExprStmt: X is unknown")
		}
	default:
		ast.Print(fset, stmt)
		err = fmt.Errorf("wp: unknown statement")
	}
	return
}

/*
func wpExpr(acc *[]ast.Expr, vars map[string]ast.Expr, expr ast.Expr, postCond ast.Expr) (pre ast.Expr, err error) {
	if conf.Debug {
		fmt.Print("#wpExpr: stmt:")
		format.Node(os.Stdout, token.NewFileSet(), expr)
		fmt.Println("")
	}

	switch expr.(type) {
		case 
	}

	return
}
*/

// wpAssignStmt は if 文の事前条件を抽出する関数
func wpAssignStmt(acc *[]ast.Expr, vars map[string]ast.Expr, s *ast.AssignStmt, postCond ast.Expr) (pre ast.Expr, err error) {
	if conf.Debug {
		fmt.Print("stmt =")
		format.Node(os.Stdout, token.NewFileSet(), s)
		fmt.Println("")
		fmt.Println("len(s.Lhs) =", len(s.Lhs))
		fmt.Println("len(s.Rhs) =", len(s.Rhs))
	}
	/*
		if len(s.Lhs) != 1 {
			err = fmt.Errorf("wp: AssignStmt: len(Lhs) is not 1")
			return
		}
	*/
	if len(s.Lhs) < 1 {
		err = fmt.Errorf("wp: AssignStmt: len(Lhs) is less than 1")
		return
	}

	// ケース２：左辺の個数が1 かつ IndexExpr である
	if len(s.Lhs) == 1 {
		_, ok := s.Lhs[0].(*ast.IndexExpr)
		if ok {
			err = fmt.Errorf("wp: AssignStmt: IndexExpr is not implemented")
			return
		}
	}

	// ケース１：左辺すべてが Ident である
	for i := 0; i < len(s.Lhs); i++ {
		switch s.Lhs[i].(type) {
		case *ast.Ident:
			//ident := s.Lhs[i].(*ast.Ident)
		default:
			err = fmt.Errorf("wp: AssignStmt: Lhs[%d] is not ident", i)
			return
		}
	}

	// ケース３：右辺の個数が1 かつ CallExpr のとき→ FunCall の処理
	if len(s.Rhs) == 1 {
		ce, ok := s.Rhs[0].(*ast.CallExpr)
		if ok {
			//err = fmt.Errorf("Assignment of FunCall is not implimented yet")
			//return
			pre, err = wpFunCall(s.Lhs, ce, postCond)
			if conf.Debug {
				fmt.Print("#wpStmt: Assignment: pre:")
				format.Node(os.Stdout, token.NewFileSet(), pre)
				fmt.Println("")
			}
			return
		}
	}

	// ケース４：右辺がすべて CallExpr 以外のとき
	for i := 0; i < len(s.Rhs); i++ {
		_, ok := s.Rhs[i].(*ast.CallExpr)
		if ok { // 関数呼び出しが入るときはエラー
			err = fmt.Errorf("multi assignment of FunCall is not supported")
			return
		}
	}

	if conf.Debug {
		fmt.Println("s.Lhs =", s.Lhs)
		fmt.Println("s.Rhs =", s.Rhs)
		format.Node(os.Stdout, token.NewFileSet(), s.Rhs)
		fmt.Println("")
	}

	var vs []ast.Expr
	var es []ast.Expr
	for i, v := range s.Lhs {
		// 左辺と右辺が同じでないとき、vs と es に追加
		if !Equals(v, s.Rhs[i]) {
			vs = append(vs, v)
			es = append(es, s.Rhs[i])
		}
	}
	if conf.Debug {
		fmt.Println("vs =", vs)
		fmt.Println("es =", vs)
		//printNode(es)
		//fmt.Println("")
	}
	// 事後条件 postCond の中の変数リスト vs を式リスト es で置換する
	pre, err = subst(postCond, vs, es)

	return
}

// wpDeclStmt は if 文の事前条件を抽出する関数
func wpDeclStmt(acc *[]ast.Expr, vars map[string]ast.Expr, ds *ast.DeclStmt, postCond ast.Expr) (pre ast.Expr, err error) {
	// 変数宣言は変数名と型名を取得する。

	gd, ok := ds.Decl.(*ast.GenDecl)
	if !ok {
		err = fmt.Errorf("wpDeclStmt: unknown Decl")
		return
	}
	if gd.Tok != token.VAR {
		err = fmt.Errorf("wpDeclStmt: not var")
		return
	}

	vs, ok := gd.Specs[0].(*ast.ValueSpec)
	if !ok {
		err = fmt.Errorf("wpDeclStmt: Specs[0] is not ValueSpec")
		return
	}

	/*
		if len(vs.Names) != 1 {
			err = fmt.Errorf("wpStmt: ValueSpec: len(Names) != 1")
			return
		}
	*/

	for _, name := range vs.Names {
		vars[name.Name] = vs.Type
	}

	pre = postCond

	return
}

// wpIfStmt は if 文の事前条件を抽出する関数
func wpIfStmt(acc *[]ast.Expr, vars map[string]ast.Expr, s *ast.IfStmt, postCond ast.Expr) (pre ast.Expr, err error) {
	cond := s.Cond
	var thenCond, elseCond ast.Expr
	thenCond, err = wpStmts(acc, vars, s.Body.List, postCond)
	if err != nil {
		return
	}
	elseCond, err = wpStmt(acc, vars, s.Else, postCond)
	if err != nil {
		return
	}
	// cond && thenCond || !cond && elseCond
	pre = astOr(astAnd(cond, thenCond), astAnd(astNot(cond), elseCond))
	return
}

// wpForStmt は for 文の事前条件を抽出する関数
func wpForStmt(acc *[]ast.Expr, vars map[string]ast.Expr, s *ast.ForStmt, postCond ast.Expr) (pre ast.Expr, err error) {
	var asserts map[string]ast.Expr
	var stmts []ast.Stmt
	asserts, stmts, err = separateStmts(s.Body.List)
	if err != nil {
		return
	}
	// INV文を取得
	inv := asserts["INV"]
	if inv == nil {
		err = fmt.Errorf("wpStmt: ForStmt: no INV")
		return
	}

	// {inv} while s.Cond stmts {postCond}
	// inv && s.Cond ==> wp(stmts, inv)
	// inv && !s.Cond ==> postCond

	pre, err = wpStmts(acc, vars, stmts, inv)
	if err != nil {
		return
	}

	// inv && s.Cond ==> pre
	*acc = append(*acc, astImplies(astAnd(inv, s.Cond), pre))
	// inv && !s.Cond ==> postCond
	*acc = append(*acc, astImplies(astAnd(inv, astNot(s.Cond)), postCond))

	pre = inv
	return
}

// wpFunCall は関数の結果を変数に代入する文の事前条件を抽出する関数。x = f(a)。
func wpFunCall(vars []ast.Expr, ce *ast.CallExpr, postCond ast.Expr) (preCond ast.Expr, err error) {
	if conf.Debug {
		fmt.Println("#wpFunCall:", ce)
	}

	funIdent, ok := ce.Fun.(*ast.Ident)
	if !ok { // 関数名の Ident を取得できないとき
		err = fmt.Errorf("wpFunCall: Fun is not Ident")
		return
	}

	// 関数の名前から、既知の関数データを取得。
	var funData Data
	funData, err = getFuncData(funIdent.Name)
	if err != nil {
		return
	}

	// 関数の入出力パラメータを取得
	iParams, oParams, _, oTypes := funData.getParams()

	// 関数の事前・事後条件を取得
	pre, post := funData.getAsserts()

	if conf.Debug {
		fmt.Print("#wpFuncCall: pre:")
		format.Node(os.Stdout, token.NewFileSet(), pre)
		fmt.Println("")
		fmt.Print("#wpFuncCall: post:")
		format.Node(os.Stdout, token.NewFileSet(), post)
		fmt.Println("")
	}

	// 左辺の vars の個数と関数の出力パラメータの oParams の数が同じでないときはエラー
	if len(vars) != len(oParams) {
		err = fmt.Errorf("wpFunCall: len(vars) != len(oParams)")
		return
	}

	// 関数の入力パラメータ iParams と引数パラメータ ce.Args の数が同じでないときはエラー
	if len(iParams) != len(ce.Args) {
		err = fmt.Errorf("wpFunCall: len(iParams) != len(ce.Args)")
		return
	}

	// 関数の事前条件 pre 内の入力パラメータを引数リストで置換
	pre, err = subst(pre, iParams, ce.Args)
	if err != nil {
		return
	}

	// 関数の出力パラメータと同じ個数の新規変数リストを作成
	us := RandomIdents(len(oParams))

	// 関数の事後条件 post 内の oParams を us で置換
	post, err = subst(post, oParams, us)
	if err != nil {
		return
	}

	// 関数の事後条件 post 内の iParams を ce.Args で置換
	post, err = subst(post, iParams, ce.Args)
	if err != nil {
		return
	}

	// 事後条件 postCond 内の oParams を us で置換
	postCond, err = subst(postCond, oParams, us)
	if err != nil {
		return
	}

	// post => postCond
	postCond = astImplies(post, postCond)

	// ForAll us.(post => postCond)
	for i, u := range us {
		postCond = astForAll(u, oTypes[i], postCond)
	}

	// pre and ForAll us.(post => postCond)
	preCond = astAnd(pre, postCond)

	if conf.Debug {
		fmt.Print("#wpFuncCall: return: preCond:")
		format.Node(os.Stdout, token.NewFileSet(), preCond)
		fmt.Println("")
	}
	return
}

// wpFunCall2 は代入のない関数呼び出しの事前条件を抽出する関数。f(a)。
func wpFunCall2(ce *ast.CallExpr, postCond ast.Expr) (preCond ast.Expr, err error) {
	if conf.Debug {
		fmt.Println("#wpFunCall2:", ce)
	}

	funIdent, ok := ce.Fun.(*ast.Ident)
	if !ok { // 関数名の Ident を取得できないとき
		err = fmt.Errorf("wpFunCall2: Fun is not Ident")
		return
	}

	// もしも無視可能な関数のときはすぐに終了
	// 無視可能な関数は PRE = POST = true となる。
	for _, ignoreFunName := range conf.IgnoreFuncs {
		if ignoreFunName == funIdent.Name {
			preCond = postCond
			return
		}
	}

	// 関数の名前から、既知の関数データを取得。
	var funData Data
	funData, err = getFuncData(funIdent.Name)
	if err != nil {
		return
	}

	// 関数の入出力パラメータを取得
	iParams, _, _, _ := funData.getParams()

	// 関数の事前・事後条件を取得
	pre, post := funData.getAsserts()

	if conf.Debug {
		fmt.Print("#wpFuncCall2: pre:")
		format.Node(os.Stdout, token.NewFileSet(), pre)
		fmt.Println("")
		fmt.Print("#wpFuncCall2: post:")
		format.Node(os.Stdout, token.NewFileSet(), post)
		fmt.Println("")
	}

	// 関数の入力パラメータ iParams と引数パラメータ ce.Args の数が同じでないときはエラー
	if len(iParams) != len(ce.Args) {
		err = fmt.Errorf("wpFunCall2: len(iParams) != len(ce.Args)")
		return
	}

	// 関数の事前条件 pre 内の入力パラメータを引数リストで置換
	pre, err = subst(pre, iParams, ce.Args)
	if err != nil {
		return
	}

	// pre[iParams:=ce.Args] and postCond
	preCond = astAnd(pre, postCond)

	if conf.Debug {
		fmt.Print("#wpFuncCall2: return: preCond:")
		format.Node(os.Stdout, token.NewFileSet(), preCond)
		fmt.Println("")
	}
	return
}

// astOr は OR 条件式の AST を作成する関数
func astOr(expr1, expr2 ast.Expr) (r ast.Expr) {
	r = &ast.BinaryExpr{
		X:  expr1,
		Op: token.LOR,
		Y:  expr2,
	}
	return
}

// astAnd は And 条件式の AST を作成する関数
func astAnd(expr1, expr2 ast.Expr) (r ast.Expr) {
	r = &ast.BinaryExpr{
		X:  expr1,
		Op: token.LAND,
		Y:  expr2,
	}
	return
}

// astNot は条件式の Not の AST を作成する関数
func astNot(expr ast.Expr) (r ast.Expr) {
	r = &ast.UnaryExpr{
		Op: token.NOT,
		X:  expr,
	}
	return
}

// astImplies は => 式の AST を作成する関数
func astImplies(expr1, expr2 ast.Expr) (r ast.Expr) {
	r = &ast.CallExpr{
		Fun: ast.NewIdent("Implies"),
		Args: []ast.Expr{
			expr1,
			expr2,
		},
	}
	return
}

// astForAll は束縛条件式　forall の AST を作成する関数。
// 例：ForAll(x, int, Implies(x>0, x>=0))
func astForAll(x, t, expr ast.Expr) (r ast.Expr) {
	r = &ast.CallExpr{
		Fun: ast.NewIdent("ForAll"),
		Args: []ast.Expr{
			x,
			t,
			expr,
		},
	}
	return
}

// astExists は束縛条件式　exists の AST を作成する関数。
// 例：Exists(x, int, Implies(x>0, x>=0))
func astExists(x, t, expr ast.Expr) (r ast.Expr) {
	r = &ast.CallExpr{
		Fun: ast.NewIdent("Exists"),
		Args: []ast.Expr{
			x,
			t,
			expr,
		},
	}
	return
}

// astStr は文字列の AST を作成する関数
func astStr(str string) (r ast.Expr) {
	r = &ast.BasicLit{
		Kind:  token.STRING,
		Value: "\"" + str + "\"",
	}
	return
}

// getFuncVars は関数宣言の型から使用される変数名とその型を調べる関数
func getFuncVars(ft *ast.FuncType) (vars map[string]ast.Expr) {
	vars = map[string]ast.Expr{}
	for _, field := range ft.Params.List {
		/*
			typ, ok := field.Type.(*ast.Ident)
			if !ok {
				// filed.Type が Ident でないときは次へ
				continue
			}
			for _, name := range field.Names {
				vars[name.Name] = typ.Name
			}
		*/
		for _, name := range field.Names {
			vars[name.Name] = field.Type
		}
	}
	if ft.Results != nil {
		for _, field := range ft.Results.List {
			for _, name := range field.Names {
				vars[name.Name] = field.Type
			}
		}
	}
	return
}

// getIOParams は関数宣言で使用される入出力パラメータを取得する関数
func getIOParams(ft *ast.FuncType) (inputs, outputs [][2]string) {
	inputs = [][2]string{}
	outputs = [][2]string{}
	for _, field := range ft.Params.List {
		typ, ok := field.Type.(*ast.Ident)
		if !ok {
			// filed.Type が Ident でないときは次へ
			continue
		}
		for _, name := range field.Names {
			inputs = append(inputs, [2]string{name.Name, typ.Name})
		}
	}
	if ft.Results != nil {
		for _, field := range ft.Results.List {
			typ, ok := field.Type.(*ast.Ident)
			if !ok {
				// filed.Type が Ident でないときは次へ
				continue
			}
			for _, name := range field.Names {
				outputs = append(outputs, [2]string{name.Name, typ.Name})
			}
		}
	}
	return
}

// RandomIdents はランダムな名前を持つ Ident のリストを作る関数
func RandomIdents(n int) (r []ast.Expr) {
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < n; i++ {
		r = append(r, ast.NewIdent(fmt.Sprintf("u%05d", rand.Intn(10000))))
	}
	return
}
