package cmd_test

// 実機の PLC に対する結合テスト。
//
// 環境変数 PLCCMD_E2E_ADDRESS を設定したときだけ実行される。
//
//	PLCCMD_E2E_ADDRESS=10.11.243.199:5000 go test ./cmd/ -run RealPLC -v
//
// 注意: このテストは実機に対して以下の副作用を持つ。稼働中の設備には向けないこと。
//   - D(yeard)〜D(yeard+6) を上書きする
//   - M(reqm) を ON にしてラダーの時刻同期処理を起動する
//   - 結果として PLC の内蔵時計がテスト実行時刻に更新される
//
// 任意の環境変数:
//
//	PLCCMD_E2E_YEARD  年を書き込む先頭 D デバイス番号 (既定 1000)
//	PLCCMD_E2E_REQM   同期要求の M デバイス番号 (既定 10)
//
// ルート情報は自局アクセス (mcprotocol.DefaultConfig) を前提とする。

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/terayu/plccmd/mcprotocol"
)

type realPLC struct {
	address string
	yearD   uint32
	reqM    uint32
}

func realPLCTarget(t *testing.T) realPLC {
	t.Helper()

	address := os.Getenv("PLCCMD_E2E_ADDRESS")
	if address == "" {
		t.Skip("PLCCMD_E2E_ADDRESS が未設定のためスキップ (実機テストの実行方法はファイル冒頭のコメントを参照)")
	}
	return realPLC{
		address: address,
		yearD:   envUint32(t, "PLCCMD_E2E_YEARD", yearD),
		reqM:    envUint32(t, "PLCCMD_E2E_REQM", reqM),
	}
}

func envUint32(t *testing.T, key string, def uint32) uint32 {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseUint(v, 0, 32)
	if err != nil {
		t.Fatalf("%s の値が不正: %v", key, err)
	}
	return uint32(n)
}

func (p realPLC) client(t *testing.T) mcprotocol.Client {
	t.Helper()
	c, err := mcprotocol.NewUDPClient(context.Background(), mcprotocol.DefaultConfig(p.address))
	if err != nil {
		t.Fatalf("PLC に接続できない (%s): %v", p.address, err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func (p realPLC) args() []string {
	return append(deviceArgsFor(p.yearD, p.reqM), "-address="+p.address)
}

func deviceArgsFor(yd, rm uint32) []string {
	return []string{
		"-yeard=" + strconv.FormatUint(uint64(yd), 10),
		"-monthd=" + strconv.FormatUint(uint64(yd+1), 10),
		"-dayd=" + strconv.FormatUint(uint64(yd+2), 10),
		"-hourd=" + strconv.FormatUint(uint64(yd+3), 10),
		"-minuted=" + strconv.FormatUint(uint64(yd+4), 10),
		"-secondd=" + strconv.FormatUint(uint64(yd+5), 10),
		"-wdayd=" + strconv.FormatUint(uint64(yd+6), 10),
		"-reqm=" + strconv.FormatUint(uint64(rm), 10),
	}
}

// 読み出しのみ。実機に到達できるかを先に確認する。
func TestRealPLCConnectivity(t *testing.T) {
	p := realPLCTarget(t)
	c := p.client(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	words, err := c.ReadWords(ctx, mcprotocol.DeviceD, p.yearD, 7)
	if err != nil {
		t.Fatalf("D%d から 7 ワード読み出せない: %v", p.yearD, err)
	}
	t.Logf("接続 OK %s / D%d..D%d = %v", p.address, p.yearD, p.yearD+6, words)

	bits, err := c.ReadBits(ctx, mcprotocol.DeviceM, p.reqM, 4)
	if err != nil {
		t.Fatalf("M%d から 4 点読み出せない: %v", p.reqM, err)
	}
	t.Logf("M%d(要求)=%v M%d=%v M%d(成功)=%v M%d(エラー)=%v",
		p.reqM, bits[0], p.reqM+1, bits[1], p.reqM+2, bits[2], p.reqM+3, bits[3])
}

// 実機に対して timesync を実行し、書き込まれた時刻と結果ビットを読み戻して検証する。
func TestRealPLCTimeSync(t *testing.T) {
	p := realPLCTarget(t)
	t.Logf("警告: %s の内蔵時計を更新します", p.address)

	c := p.client(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	before := time.Now()
	r := runTimesync(t, cleanEnv(), p.args()...)
	after := time.Now()

	if !r.ok() {
		t.Fatalf("timesync が失敗した (exit=%d)\n%s", r.exitCode, r.output)
	}
	if !strings.Contains(r.output, "time sync success") {
		t.Fatalf("成功ログが無い\n%s", r.output)
	}

	// 書き込まれた時刻を読み戻す
	words, err := c.ReadWords(ctx, mcprotocol.DeviceD, p.yearD, 7)
	if err != nil {
		t.Fatalf("D%d の読み戻しに失敗: %v", p.yearD, err)
	}
	got := time.Date(int(words[0]), time.Month(words[1]), int(words[2]),
		int(words[3]), int(words[4]), int(words[5]), 0, time.Local)
	t.Logf("PLC に書き込まれた時刻: %v (曜日 %d)", got, words[6])

	if got.Before(before.Truncate(time.Second)) || got.After(after) {
		t.Fatalf("書き込まれた時刻が実行時刻の範囲外: got=%v want=[%v, %v]", got, before, after)
	}
	if uint16(got.Weekday()) != words[6] {
		t.Fatalf("曜日が不正: got=%d want=%d", words[6], got.Weekday())
	}

	// 結果ビットを読み戻す
	bits, err := c.ReadBits(ctx, mcprotocol.DeviceM, p.reqM, 4)
	if err != nil {
		t.Fatalf("M%d の読み戻しに失敗: %v", p.reqM, err)
	}
	if bits[0] {
		t.Errorf("M%d (要求ビット) が落ちていない", p.reqM)
	}
	if !bits[2] {
		t.Errorf("M%d (成功ビット) が立っていない", p.reqM+2)
	}
	if bits[3] {
		t.Errorf("M%d (エラービット) が立っている", p.reqM+3)
	}
}
