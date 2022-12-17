package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"time"
)

const (
	// USAGE はコマンドラインでの使い方
	USAGE = "%s src.go func_name\n"
)

func main() {
	os.Exit(run())
}

var conf Config
var fset *token.FileSet
var srcFile string

func run() int {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, USAGE, os.Args[0])
		return 1
	}

	var err error
	conf, err = LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	srcFile = os.Args[1]
	funcNames := os.Args[2:]

	// Golang の構文としてパース
	fset = token.NewFileSet()
	fileNode, err := parser.ParseFile(fset, srcFile, nil, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	var d Data
	for _, funcName := range funcNames {

		// すでに検証結果が保存されているならそれを表示する。
		//d, err = LoadData(funcName + ".json")
		d, err = getFuncData(funcName)
		if err == nil {
			fmt.Println("(cached)")
			fmt.Println(d)
			continue
		}

		// 未検証なら検証する。
		err = processFunc(fileNode, funcName)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 3
		}
	}

	return 0
}

func processFunc(fileNode *ast.File, funcName string) (err error) {
	if conf.Debug {
		fmt.Println("#proessFunc: ", funcName)
	}

	// funcName の関数定義を取得
	funcDecl := pickupFuncDecl(fileNode, funcName)
	if funcDecl == nil {
		err = fmt.Errorf("unknown function: %s", funcName)
		return
	}

	// 検証すべき条件式を取得する。
	conds, vars, pre, post, err := getCondTobeVerified(funcDecl)
	if err != nil {
		return
	}

	if conf.Debug {
		fmt.Println("#proessFunc: conds", conds, ":")
		format.Node(os.Stdout, token.NewFileSet(), conds)
	}

	// 関数の入出力パラメータを取得
	inputs, outputs := getIOParams(funcDecl.Type)

	preBuffer := new(bytes.Buffer)
	format.Node(preBuffer, token.NewFileSet(), pre)

	postBuffer := new(bytes.Buffer)
	format.Node(postBuffer, token.NewFileSet(), post)

	// 検証すべき条件式の文字列を格納するリスト
	var condStrs []string

	for _, cond := range conds {
		if conf.Debug {
			printNode(cond)
			fmt.Print("#processFunc: cond: ")
			format.Node(os.Stdout, token.NewFileSet(), cond)
			fmt.Println("")
		}
		// 検証すべき条件式を SMT Solver で検証する。

		// AST の cond を Golang 構文の文字列に変換
		condBuffer := new(bytes.Buffer)
		if conf.Debug {
			if cond == nil {
				fmt.Println("cond is nil!")
			} else {
				fmt.Println("cond is not nil!")
			}
		}
		format.Node(condBuffer, token.NewFileSet(), cond)
		condStrs = append(condStrs, condBuffer.String())

		// SMT Script を作成。
		script := makeSMTScript(vars, cond)
		if conf.Debug {
			// cond の AST と作成した SMT Script の表示
			fmt.Println("# AST")
			printNode(cond)
			fmt.Println("# SMT Script:")
			fmt.Println(script)
		}

		// SMT Script を実行。
		var result, outText, errText string
		result, outText, errText, err = runSMTScript(script)
		if err != nil {
			err = fmt.Errorf("%s; %s", err.Error(), errText)
			return
		}

		// 充足 (sat) の場合は NG なので反例を表示。
		switch result {
		case "sat":
			fmt.Fprintln(os.Stderr, condBuffer.String(), "=> NG")
			err = fmt.Errorf("unsat; counter-example: %s", outText)
			return
		case "unsat":
			//fmt.Fprintln(out, "=> OK")
			// skip
		default:
			err = fmt.Errorf("something wrong")
			return
		}

	}

	// 検証結果のデータを作成
	data := Data{
		Name:    funcName,
		Inputs:  inputs,
		Outputs: outputs,
		Pre:     preBuffer.String(),
		Post:    postBuffer.String(),
		Conds:   condStrs,
		Date:    time.Now().String(),
	}
	fmt.Println(data)

	// funcTab に保存
	setFuncData(funcName, data)

	// JSON に保存
	data.Save(makeFuncFileName(funcName))

	return
}

func makeFuncFileName(funcName string) (r string) {
	r = srcFile
	if strings.HasSuffix(r, ".go") { // 01234.go
		r = r[:len(r)-3]
	}
	r = r + "_" + funcName + ".json"
	return
}

// pickupFuncDecl は指定された関数名 (funcName) の関数定義を
// トップレベルの宣言の中から取得する関数。
// 関数内部で宣言される関数は対象外。
func pickupFuncDecl(fileNode *ast.File, funcName string) (f *ast.FuncDecl) {
	// ファイルノードのトップレベルの「宣言」の中から指定された名前の関数を取得する
	for _, n := range fileNode.Decls {
		// 関数宣言のうちその名前が "main" のものをみつける
		funcDecl, ok := n.(*ast.FuncDecl)
		if ok && funcDecl.Name.Name == funcName {
			f = funcDecl
			break
		}
	}
	return
}

// printNode は与えられた AST ノードのツリー構造をダンプする関数。
func printNode(node interface{}) {
	ast.Print(fset, node)
}

// [MEMO] 以下、定義したけど使ってない関数。汎用的なものなのでいつか使うことがあるかもしれない。

/*
// saveSrc は AST をファイルに保存する関数
func saveSrc(filename string, f *ast.File) (err error) {
	var w *os.File
	w, err = os.Create(filename)
	if err != nil {
		return
	}
	defer w.Close()
	// AST をファイルに保存
	format.Node(w, token.NewFileSet(), f)
	return
}

// readSrc はファイルを読み出す関数
func readSrc(filename string) (string, error) {
	bs, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}
*/
