package mcprotocol

import (
	"context"
	"net"
	"testing"
	"time"
)

var ctx = context.Background()

// 指定した応答を1回返すだけのダミーPLC
func fakePLC(t *testing.T, resp []byte) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	go func() {
		buf := make([]byte, 2048)
		_, addr, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		pc.WriteTo(resp, addr)
	}()
	return pc.LocalAddr().String()
}

func newClient(t *testing.T, addr string) *UDPClient {
	t.Helper()
	cfg := DefaultConfig(addr)
	cfg.ReadTimeout = 2 * time.Second
	c, err := NewUDPClient(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// A: 短い応答を書き込み成功と誤認しないこと
func TestWriteShortResponse(t *testing.T) {
	c := newClient(t, fakePLC(t, []byte{0xD0, 0x00, 0x00, 0xFF, 0xFF}))
	err := c.WriteWords(ctx, DeviceD, 100, []uint16{2026})
	if err == nil {
		t.Fatal("短い応答が成功と判定された")
	}
	t.Logf("OK: %v", err)
}

// B: 短い応答で panic しないこと
func TestReadShortResponse(t *testing.T) {
	c := newClient(t, fakePLC(t, []byte{0xD0, 0x00, 0x00, 0xFF, 0xFF}))
	if _, err := c.ReadBits(ctx, DeviceM, 100, 1); err == nil {
		t.Fatal("短い応答がエラーにならなかった")
	} else {
		t.Logf("OK: %v", err)
	}
}

// C: データ長不足で panic しないこと（11バイト = 正常終了コードのみ、データ無し）
func TestReadInsufficientData(t *testing.T) {
	resp := []byte{0xD0, 0x00, 0x00, 0xFF, 0xFF, 0x03, 0x00, 0x02, 0x00, 0x00, 0x00}
	c := newClient(t, fakePLC(t, resp))
	if _, err := c.ReadWords(ctx, DeviceD, 100, 7); err == nil {
		t.Fatal("データ長不足がエラーにならなかった")
	} else {
		t.Logf("OK: %v", err)
	}
}

// サブヘッダ不正を弾くこと
func TestBadSubheader(t *testing.T) {
	resp := make([]byte, 13)
	resp[0], resp[1] = 0x50, 0x00 // 要求用サブヘッダ
	c := newClient(t, fakePLC(t, resp))
	if _, err := c.ReadWords(ctx, DeviceD, 100, 1); err == nil {
		t.Fatal("不正なサブヘッダがエラーにならなかった")
	} else {
		t.Logf("OK: %v", err)
	}
}

// 正常応答は従来どおり読めること（回帰確認）
func TestReadNormalResponse(t *testing.T) {
	resp := []byte{0xD0, 0x00, 0x00, 0xFF, 0xFF, 0x03, 0x00, 0x06, 0x00, 0x00, 0x00,
		0xEA, 0x07, 0x09, 0x00} // endCode=0, D=2026, 9
	c := newClient(t, fakePLC(t, resp))
	got, err := c.ReadWords(ctx, DeviceD, 100, 2)
	if err != nil {
		t.Fatalf("正常応答が失敗した: %v", err)
	}
	if got[0] != 2026 || got[1] != 9 {
		t.Fatalf("値が不正: %v", got)
	}
	t.Logf("OK: %v", got)
}

// PLCエラー応答は従来どおりエラーになること（回帰確認）
func TestPLCErrorCode(t *testing.T) {
	resp := []byte{0xD0, 0x00, 0x00, 0xFF, 0xFF, 0x03, 0x00, 0x02, 0x00, 0x51, 0xC0}
	c := newClient(t, fakePLC(t, resp))
	if err := c.WriteWords(ctx, DeviceD, 100, []uint16{1}); err == nil {
		t.Fatal("PLCエラーがエラーにならなかった")
	} else {
		t.Logf("OK: %v", err)
	}
}
