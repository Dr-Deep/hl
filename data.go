// 検証結果データ

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"io/ioutil"
	"os"
	"strings"
)

// Data は検証結果データ
type Data struct {
	Name    string      `json:"name"`    // 関数名
	Inputs  [][2]string `json:"inputs"`  // 入力パラメータ
	Outputs [][2]string `json:"outputs"` // 出力パラメータ
	Pre     string      `json:"pre"`     // 事前条件
	Post    string      `json:"post"`    // 事後条件
	Conds   []string    `json:"conds"`   // 検証した条件式リスト
	Date    string      `json:"date"`    // 検証日
	Note    string      `json:"note"`    // ノート
}

// String は検証結果データの文字列を作成する関数
func (d Data) String() string {
	out := new(bytes.Buffer)

	fmt.Fprintf(out, "Function: %s\n", d.Name)
	tmp := []string{}
	for _, t := range d.Inputs {
		tmp = append(tmp, fmt.Sprintf("%s:%s", t[0], t[1]))
	}
	fmt.Fprintf(out, "IN:  %s\n", strings.Join(tmp, ", "))
	tmp = []string{}
	for _, t := range d.Outputs {
		tmp = append(tmp, fmt.Sprintf("%s:%s", t[0], t[1]))
	}
	fmt.Fprintf(out, "OUT: %s\n", strings.Join(tmp, ", "))

	fmt.Fprintf(out, "PRE: %s\n", d.Pre)
	fmt.Fprintf(out, "POST: %s\n", d.Post)

	for i, cond := range d.Conds {
		fmt.Fprintf(out, "Cond[%d]: %s\n", i, cond)
	}

	if d.Date != "" {
		fmt.Fprintf(out, "Date: %s\n", d.Date)
	}

	if d.Note != "" {
		fmt.Fprintf(out, "Note: %s\n", d.Note)
	}

	return out.String()
}

// getParams は AST 形式で入出力パラメータを取得する関数。
func (d Data) getParams() (is, os, it, ot []ast.Expr) {
	for _, vv := range d.Inputs {
		is = append(is, ast.NewIdent(vv[0]))
	}
	for _, vv := range d.Outputs {
		os = append(os, ast.NewIdent(vv[0]))
	}
	for _, vv := range d.Inputs {
		it = append(it, ast.NewIdent(vv[1]))
	}
	for _, vv := range d.Outputs {
		ot = append(ot, ast.NewIdent(vv[1]))
	}
	return
}

// getAsserts は AST 形式で事前・事後条件式を取得する関数。
func (d Data) getAsserts() (pre, post ast.Expr) {
	pre, _ = parser.ParseExpr(d.Pre)
	post, _ = parser.ParseExpr(d.Post)
	return
}

// LoadData はファイルに保存された検証結果データを読みだす関数
func LoadData(inFile string) (d Data, err error) {
	err = LoadJSON(inFile, &d)
	return
}

// Save は検証結果データをファイル保存する関数
func (d Data) Save(outFile string) (err error) {
	err = SaveJSON(outFile, d)
	return
}

// [MEMO] LoadJSON/SaveJSON は別ファイル化すべきかもしれない。現在のファイル構成は暫定。

// LoadJSON はファイルに保存された JSON オブジェクトを読み出す関数
// 出力先変数 out には '&' をつけること。
// err := LoadJSON("file.json", &out)
func LoadJSON(filePath string, out interface{}) (err error) {
	// バイト列読み出し
	var bytes []byte
	bytes, err = ioutil.ReadFile(filePath)
	if err != nil {
		return
	}

	// json 形式のデコード
	err = json.Unmarshal(bytes, out)
	return
}

// SaveJSON はデータを JSON 形式でファイル保存する関数
func SaveJSON(outFile string, v interface{}) (err error) {
	var w *os.File
	w, err = os.Create(outFile)
	if err != nil {
		return
	}
	defer w.Close()
	var b []byte
	b, err = json.Marshal(v)
	if err != nil {
		return
	}
	var out bytes.Buffer
	err = json.Indent(&out, b, "", "  ")
	if err != nil {
		return
	}
	_, err = out.WriteTo(w)
	return
}
