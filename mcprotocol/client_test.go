package mcprotocol

import (
	"bytes"
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

// recordingPLC は受け取った要求を記録し、canned を応答として返す。
type recordingPLC struct {
	addr string
	req  chan []byte
}

func newRecordingPLC(t *testing.T, canned []byte) *recordingPLC {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })

	p := &recordingPLC{addr: pc.LocalAddr().String(), req: make(chan []byte, 1)}
	go func() {
		buf := make([]byte, 2048)
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		p.req <- append([]byte(nil), buf[:n]...)
		pc.WriteTo(canned, addr)
	}()
	return p
}

func (p *recordingPLC) request(t *testing.T) []byte {
	t.Helper()
	select {
	case r := <-p.req:
		return r
	case <-time.After(2 * time.Second):
		t.Fatal("要求が届かなかった")
		return nil
	}
}

// 正常終了の応答（データ部なし）
var okResponse = []byte{0xD0, 0x00, 0x00, 0xFF, 0xFF, 0x03, 0x00, 0x02, 0x00, 0x00, 0x00}

func TestPackUnpackBits(t *testing.T) {
	cases := [][]bool{
		{true},
		{false},
		{true, false, true},
		{true, true, false, false, true, true, false, true, true},
	}
	for _, want := range cases {
		packed := packBits(want)
		if len(packed) != (len(want)+1)/2 {
			t.Fatalf("パック後の長さが不正: %d (points=%d)", len(packed), len(want))
		}
		got := unpackBits(packed, uint16(len(want)))
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("往復で不一致 points=%d i=%d: got %v want %v", len(want), i, got[i], want[i])
			}
		}
	}
}

// ビット単位書き込みのフレームがバイト単位で仕様どおりであること
func TestWriteBitsRequestFrame(t *testing.T) {
	plc := newRecordingPLC(t, okResponse)
	c, err := NewUDPClient(context.Background(), DefaultConfig(plc.addr))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if err := c.WriteBits(ctx, DeviceY, 0x10, []bool{true, false, true}); err != nil {
		t.Fatal(err)
	}

	want := []byte{
		0x50, 0x00, // サブヘッダ
		0x00,       // ネットワーク番号
		0xFF,       // PC番号
		0xFF, 0x03, // I/O番号 0x03FF
		0x00,       // 局番号
		0x0E, 0x00, // 要求データ長 14
		0x10, 0x00, // 監視タイマ 0x0010
		0x01, 0x14, // コマンド 0x1401
		0x01, 0x00, // サブコマンド 0x0001 (ビット)
		0x10, 0x00, 0x00, // 先頭デバイス番号 0x10
		0x9D,       // デバイスコード Y
		0x03, 0x00, // 点数 3
		0x10, 0x10, // ビットデータ (上位ニブル=先頭点)
	}
	if got := plc.request(t); !bytes.Equal(got, want) {
		t.Fatalf("フレーム不一致\ngot  % 02X\nwant % 02X", got, want)
	}
}

// ビット単位読み出しの応答を正しく展開できること
func TestReadBitsResponse(t *testing.T) {
	resp := []byte{0xD0, 0x00, 0x00, 0xFF, 0xFF, 0x03, 0x00, 0x04, 0x00, 0x00, 0x00, 0x10, 0x10}
	plc := newRecordingPLC(t, resp)
	c, err := NewUDPClient(context.Background(), DefaultConfig(plc.addr))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	got, err := c.ReadBits(ctx, DeviceM, 100, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []bool{true, false, true}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("i=%d: got %v want %v", i, got[i], want[i])
		}
	}
}

// ワードデバイスへのビットアクセスは通信前に弾かれること
func TestBitAccessOnWordDevice(t *testing.T) {
	c, err := NewUDPClient(context.Background(), DefaultConfig("127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	for _, dev := range []DeviceCode{DeviceD, DeviceW, DeviceR, DeviceZR, DeviceTN, DeviceCN} {
		if _, err := c.ReadBits(ctx, dev, 0, 1); err == nil {
			t.Fatalf("%s へのビット読み出しがエラーにならなかった", dev)
		}
		if err := c.WriteBits(ctx, dev, 0, []bool{true}); err == nil {
			t.Fatalf("%s へのビット書き込みがエラーにならなかった", dev)
		}
	}
	// ビットデバイスへのワードアクセスは許可される（16点=1ワード）
	for _, dev := range []DeviceCode{DeviceX, DeviceY, DeviceM, DeviceL, DeviceB, DeviceTS, DeviceTC, DeviceCS, DeviceCC} {
		if !dev.IsBit() {
			t.Fatalf("%s がビットデバイスと判定されない", dev)
		}
	}
}

func TestValidation(t *testing.T) {
	c, err := NewUDPClient(context.Background(), DefaultConfig("127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if _, err := c.ReadWords(ctx, DeviceCode(0x00), 0, 1); err == nil {
		t.Fatal("未知のデバイスコードがエラーにならなかった")
	}
	if _, err := c.ReadWords(ctx, DeviceD, 0, 0); err == nil {
		t.Fatal("0 点がエラーにならなかった")
	}
	if _, err := c.ReadWords(ctx, DeviceD, 0, MaxWordPoints+1); err == nil {
		t.Fatal("ワード点数の上限超過がエラーにならなかった")
	}
	if _, err := c.ReadBits(ctx, DeviceM, 0, MaxBitPoints+1); err == nil {
		t.Fatal("ビット点数の上限超過がエラーにならなかった")
	}
}

// 終了コードが EndCodeError として取り出せること
func TestEndCodeError(t *testing.T) {
	resp := []byte{0xD0, 0x00, 0x00, 0xFF, 0xFF, 0x03, 0x00, 0x02, 0x00, 0x51, 0xC0}
	plc := newRecordingPLC(t, resp)
	c, err := NewUDPClient(context.Background(), DefaultConfig(plc.addr))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_, err = c.ReadWords(ctx, DeviceD, 100, 1)
	var ece *EndCodeError
	if !errors.As(err, &ece) {
		t.Fatalf("EndCodeError として取り出せない: %v", err)
	}
	if ece.Code != 0xC051 {
		t.Fatalf("終了コードが不正: 0x%04X", ece.Code)
	}
}

func TestDeviceCodeString(t *testing.T) {
	if got := DeviceZR.String(); got != "ZR" {
		t.Fatalf("got %q", got)
	}
	if got := DeviceCode(0x77).String(); got != "DeviceCode(0x77)" {
		t.Fatalf("got %q", got)
	}
}
