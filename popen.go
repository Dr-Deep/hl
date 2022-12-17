// 外部プロセスでスクリプトを起動し、外部プロセスとのパイプでスクリプトの
// 受け渡しと実行結果の取得を行う実装

package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"syscall"
	"time"
)

// runCmd は外部プロセスでスクリプトを実行する関数。
// 入力 cmd : コマンドと引数のリスト。
// 入力 script : スクリプトの文字列。
// 入力 timeOutSec : タイムアウト時間(秒)。
// 出力 outText : スクリプト実行で得られた標準出力の文字列。
// 出力 errText : スクリプト実行で得られた標準エラーの文字列。
// 出力 err : 処理エラー。
// メモ : 外部プロセスを起動し、外部プロセスとのパイプでスクリプトを受け渡し、外部プロセスの標準出力と標準エラーを取得する。
func runCmd(cmd []string, script string, timeOutSec int) (outText, errText string, err error) {

	// コマンドを実行する外部プロセスのオブジェクト生成。
	p := exec.Command(cmd[0], cmd[1:]...)

	var stdin io.WriteCloser
	var stdout, stderr io.ReadCloser

	// 外部プロセスへの標準入力のパイプの取得とスクリプトの送出
	stdin, err = p.StdinPipe()
	if err != nil {
		return
	}

	if stdin != nil {
		io.WriteString(stdin, script)
		stdin.Close()
	}

	// 外部プロセスからの標準出力のパイプの取得
	stdout, err = p.StdoutPipe()
	if err != nil {
		return
	}

	// 外部プロセスからの標準エラーのパイプの取得
	stderr, err = p.StderrPipe()
	if err != nil {
		return
	}

	// 外部プロセスの起動
	err = p.Start()
	if err != nil {
		return
	}

	done := make(chan error, 1)

	go func() {
		var b []byte

		// 外部プロセスからの標準出力の読み出し
		b, err = ioutil.ReadAll(stdout)
		if err != nil {
			errText = "failed in reading stdout"
			done <- nil
			return
		}
		outText = string(b)

		// 外部プロセスからの標準エラーの読み出し
		b, err = ioutil.ReadAll(stderr)
		if err != nil {
			errText = "failed in reading stderr"
			done <- nil
			return
		}
		errText = string(b)

		// 外部プロセスの待ち合わせ
		err = p.Wait()
		if err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok {
				status, ok := exiterr.Sys().(syscall.WaitStatus)
				if ok {
					err = nil
					errText = fmt.Sprintf("%s\nexecution failed (exit code=%d)\n", errText, status.ExitStatus())
				}
			}
		}
		done <- err
	}()

	select {
	case <-done:
		break

	case <-time.After(time.Duration(timeOutSec) * time.Second):
		err = p.Process.Kill()
		if err != nil {
			errText = "runCmd: timeout: failed to kill: " + err.Error()
		}
	}

	return
}
