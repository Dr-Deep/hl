// 設定ファイルの読み出し

package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	confFileName      = "conf.json"
	defaultCmdExe     = "z3"
	defaultCmdArg     = "-in"
	defaultTimeOutSec = 60
)

// Config は設定情報の型
type Config struct {
	Cmd         []string `json:"cmd"`
	TimeOutSec  int      `json:"time_out_sec"`
	IgnoreFuncs []string `json:"ignore_funcs"`
	Debug       bool     `json:"debug"`
}

// LoadConfig はファイルに保存された JSON オブジェクトを読み出す関数
func LoadConfig() (conf Config, err error) {

	// バイト列読み出し
	var bytes []byte
	bytes, err = ioutil.ReadFile(resolvConfFile())
	if err != nil {
		return
	}

	// json 形式のデコード
	err = json.Unmarshal(bytes, &conf)
	if err != nil {
		return
	}

	// 必要ならば、ここで conf の格納値をチェックする。
	if conf.Cmd == nil {
		conf.Cmd = []string{defaultCmdExe, defaultCmdArg}
	}
	if conf.TimeOutSec == 0 {
		conf.TimeOutSec = defaultTimeOutSec
	}
	if len(conf.IgnoreFuncs) == 0 {
		conf.IgnoreFuncs = []string{"Print", "Println", "Printf"}
	}
	return
}

// resolvConfFile は設定ファイルのパスを特定する関数。
// 実行ファイルと同じディレクトリ配下の設定ファイルのパスとする。
func resolvConfFile() string {
	// 実行ファイルのパスを特定
	exe, err := os.Executable()
	if err == nil {
		// 実行ファイルのあるディレクトリ配下の設定ファイルのパス
		return filepath.Dir(exe) + "/" + confFileName
	}

	// 実行カレントディレクトリ配下の設定ファイルのパス
	return confFileName
}
