package cmd_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "plccmd-e2e")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	binPath = filepath.Join(dir, "plccmd")
	build := exec.Command("go", "build", "-tags", "timetzdata", "-o", binPath, "./app")
	build.Dir = ".."
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n%s", err, out)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// cleanEnv は plccmd が読む環境変数(ADDRESS/PCNO など)を意図的に引き継がない。
// 開発者のシェルの設定でテスト結果が変わらないようにするため。
func cleanEnv(extra ...string) []string {
	env := []string{}
	for _, k := range []string{"PATH", "HOME", "TZ", "TMPDIR"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return append(env, extra...)
}

// 実機の設定に合わせたデバイス割り付け
const (
	yearD = 1000
	reqM  = 10
)

func deviceArgs() []string { return deviceArgsFor(yearD, reqM) }

type result struct {
	exitCode int
	output   string
}

func (r result) ok() bool { return r.exitCode == 0 }

func runTimesync(t *testing.T, env []string, args ...string) result {
	t.Helper()

	cmd := exec.Command(binPath, append([]string{"-loglevel=debug", "timesync"}, args...)...)
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("実行できなかった: %v", err)
	}
	return result{exitCode: code, output: string(out)}
}

// 正常系: 終了コード 0、成功ログ、書き込まれた時刻が正しいこと
func TestE2ESuccess(t *testing.T) {
	plc := newFakePLC(t)

	before := time.Now()
	r := runTimesync(t, cleanEnv(), append(deviceArgs(), "-address="+plc.addr)...)
	after := time.Now()

	if !r.ok() {
		t.Fatalf("失敗した (exit=%d)\n%s", r.exitCode, r.output)
	}
	if !strings.Contains(r.output, "time sync success") {
		t.Fatalf("成功ログが無い\n%s", r.output)
	}

	got := readWrittenTime(t, plc)
	if got.Before(before.Truncate(time.Second)) || got.After(after) {
		t.Fatalf("書き込まれた時刻が範囲外: got=%v want=[%v, %v]", got, before, after)
	}
	if w := plc.word(0xA8, yearD+6); w != uint16(got.Weekday()) {
		t.Fatalf("曜日が不正: got=%d want=%d", w, got.Weekday())
	}
	if w := plc.word(0x90, reqM); w != 0x0001 {
		t.Fatalf("要求ビットが立てられていない: 0x%04X", w)
	}
}

// PLC がエラービット(ReqM+3)を立てたら失敗として扱うこと
func TestE2EPLCErrorBit(t *testing.T) {
	plc := newFakePLC(t)
	plc.successBit = false
	plc.errorBit = true

	r := runTimesync(t, cleanEnv(), append(deviceArgs(), "-address="+plc.addr)...)

	if r.ok() {
		t.Fatalf("エラービットが立っているのに成功した\n%s", r.output)
	}
	if !strings.Contains(r.output, "plc reported time sync failure") {
		t.Fatalf("期待したエラーが出ていない\n%s", r.output)
	}
}

// 成功ビット(ReqM+2)が立たないまま要求ビットが落ちた場合も失敗とすること
func TestE2ENoSuccessBit(t *testing.T) {
	plc := newFakePLC(t)
	plc.successBit = false
	plc.errorBit = false

	r := runTimesync(t, cleanEnv(), append(deviceArgs(), "-address="+plc.addr)...)

	if r.ok() {
		t.Fatalf("成功ビットが無いのに成功した\n%s", r.output)
	}
}

// PLC が終了コードを返したら失敗すること
func TestE2EPLCEndCode(t *testing.T) {
	plc := newFakePLC(t)
	plc.endCode = 0x4003

	r := runTimesync(t, cleanEnv(), append(deviceArgs(), "-address="+plc.addr)...)

	if r.ok() {
		t.Fatalf("終了コード 0x4003 なのに成功した\n%s", r.output)
	}
	if !strings.Contains(r.output, "0x4003") {
		t.Fatalf("終了コードがログに出ていない\n%s", r.output)
	}
}

// 短すぎる応答を成功と誤認しないこと(既知の問題 6-A の回帰テスト)
func TestE2EShortResponse(t *testing.T) {
	plc := newFakePLC(t)
	plc.garbage = []byte{0xD0, 0x00, 0x00, 0xFF, 0xFF}

	r := runTimesync(t, cleanEnv(), append(deviceArgs(), "-address="+plc.addr)...)

	if r.ok() {
		t.Fatalf("不正な応答を成功と判定した\n%s", r.output)
	}
}

// 要求ビットが落ちなければタイムアウトすること
func TestE2ETimeout(t *testing.T) {
	plc := newFakePLC(t)
	plc.neverComplete = true

	r := runTimesync(t, cleanEnv(), append(deviceArgs(), "-address="+plc.addr, "-waittimeout=1")...)

	if r.ok() {
		t.Fatalf("完了しないのに成功した\n%s", r.output)
	}
	if !strings.Contains(r.output, "timeout waiting PLC to reset") {
		t.Fatalf("タイムアウトのエラーが出ていない\n%s", r.output)
	}
}

// 必須フラグが無ければ失敗すること
func TestE2EMissingAddress(t *testing.T) {
	r := runTimesync(t, cleanEnv(), deviceArgs()...)
	if r.ok() {
		t.Fatalf("-address 無しで成功した\n%s", r.output)
	}
}

// 既定のルート情報が自局アクセスであること(2026-09-05 の 0x4003 の回帰テスト)
func TestE2EDefaultRoute(t *testing.T) {
	plc := newFakePLC(t)

	r := runTimesync(t, cleanEnv(), append(deviceArgs(), "-address="+plc.addr)...)
	if !r.ok() {
		t.Fatalf("失敗した (exit=%d)\n%s", r.exitCode, r.output)
	}

	assertRoute(t, plc, []byte{0x00, 0xFF, 0xFF, 0x03, 0x00})
}

// ルート情報の環境変数がフレームに反映されること
func TestE2ERouteFromEnv(t *testing.T) {
	plc := newFakePLC(t)

	env := cleanEnv("NETWORKNO=2", "PCNO=0", "IONO=255", "STATIONNO=2")
	r := runTimesync(t, env, append(deviceArgs(), "-address="+plc.addr)...)
	if !r.ok() {
		t.Fatalf("失敗した (exit=%d)\n%s", r.exitCode, r.output)
	}

	assertRoute(t, plc, []byte{0x02, 0x00, 0xFF, 0x00, 0x02})
}

// コマンドラインフラグが環境変数より優先されること
func TestE2EFlagOverridesEnv(t *testing.T) {
	plc := newFakePLC(t)

	env := cleanEnv("PCNO=0", "IONO=255")
	args := append(deviceArgs(), "-address="+plc.addr, "-pcno=255", "-iono=1023")
	r := runTimesync(t, env, args...)
	if !r.ok() {
		t.Fatalf("失敗した (exit=%d)\n%s", r.exitCode, r.output)
	}

	assertRoute(t, plc, []byte{0x00, 0xFF, 0xFF, 0x03, 0x00})
}

func assertRoute(t *testing.T, plc *fakePLC, want []byte) {
	t.Helper()
	got := plc.firstFrame(t)[2:7]
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ルート情報が不一致\ngot  % 02X\nwant % 02X", got, want)
		}
	}
}

// readWrittenTime は D デバイスに書かれた 7 ワードから時刻を復元する。
func readWrittenTime(t *testing.T, plc *fakePLC) time.Time {
	t.Helper()
	return time.Date(
		int(plc.word(0xA8, yearD)),
		time.Month(plc.word(0xA8, yearD+1)),
		int(plc.word(0xA8, yearD+2)),
		int(plc.word(0xA8, yearD+3)),
		int(plc.word(0xA8, yearD+4)),
		int(plc.word(0xA8, yearD+5)),
		0, time.Local,
	)
}
