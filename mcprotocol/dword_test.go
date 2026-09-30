package mcprotocol

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"
)

// memClient は Client を実装するメモリスタブ。dev は無視する。
type memClient struct {
	words  map[uint32]uint16
	points []uint16 // 呼び出しごとの点数
	err    error
	short  bool // true なら ReadWords が要求より 1 語少なく返す
}

func newMemClient() *memClient {
	return &memClient{words: map[uint32]uint16{}}
}

func (m *memClient) ReadWords(_ context.Context, _ DeviceCode, head uint32, points uint16) ([]uint16, error) {
	m.points = append(m.points, points)
	if m.err != nil {
		return nil, m.err
	}
	out := make([]uint16, points)
	for i := range out {
		out[i] = m.words[head+uint32(i)]
	}
	if m.short {
		out = out[:len(out)-1]
	}
	return out, nil
}

func (m *memClient) WriteWords(_ context.Context, _ DeviceCode, head uint32, values []uint16) error {
	m.points = append(m.points, uint16(len(values)))
	if m.err != nil {
		return m.err
	}
	for i, v := range values {
		m.words[head+uint32(i)] = v
	}
	return nil
}

func (m *memClient) ReadBits(context.Context, DeviceCode, uint32, uint16) ([]bool, error) {
	return nil, errors.New("not implemented")
}

func (m *memClient) WriteBits(context.Context, DeviceCode, uint32, []bool) error {
	return errors.New("not implemented")
}

func (m *memClient) Close() error { return nil }

// WriteDWords の要求フレームが期待どおりのバイト列になること
func TestWriteDWordsRequestFrame(t *testing.T) {
	plc := newRecordingPLC(t, okResponse)
	c, err := NewUDPClient(context.Background(), DefaultConfig(plc.addr))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if err := WriteDWords(ctx, c, DeviceD, 100, []uint32{0x12345678}); err != nil {
		t.Fatal(err)
	}

	want := []byte{
		0x50, 0x00, 0x00, 0xFF, 0xFF, 0x03, 0x00, // ヘッダ
		0x10, 0x00, // 要求データ長 16
		0x10, 0x00, // 監視タイマ
		0x01, 0x14, // コマンド 0x1401
		0x00, 0x00, // サブコマンド (ワード)
		0x64, 0x00, 0x00, // 先頭デバイス番号 100
		0xA8,       // デバイスコード D
		0x02, 0x00, // 点数 2 (1 ダブルワード = 2 ワード)
		0x78, 0x56, 0x34, 0x12, // 下位ワード 0x5678, 上位ワード 0x1234
	}
	if got := plc.request(t); !bytes.Equal(got, want) {
		t.Fatalf("フレーム不一致\ngot  % 02X\nwant % 02X", got, want)
	}
}

// ReadDWords が応答を下位先で 32bit に組み立て、点数の 2 倍を要求すること
func TestReadDWordsResponse(t *testing.T) {
	resp := []byte{0xD0, 0x00, 0x00, 0xFF, 0xFF, 0x03, 0x00, 0x0A, 0x00, 0x00, 0x00,
		0x78, 0x56, 0x34, 0x12, 0xFF, 0xFF, 0xFF, 0xFF}
	plc := newRecordingPLC(t, resp)
	c := newClient(t, plc.addr)

	got, err := ReadDWords(ctx, c, DeviceD, 100, 2)
	if err != nil {
		t.Fatal(err)
	}
	if want := []uint32{0x12345678, 0xFFFFFFFF}; !reflect.DeepEqual(got, want) {
		t.Fatalf("値が不正: got %#v want %#v", got, want)
	}
	req := plc.request(t)
	if n := uint16(req[19]) | uint16(req[20])<<8; n != 4 {
		t.Fatalf("要求点数が不正: got %d want 4", n)
	}
}

// 下位ワードが先に格納されること
func TestDWordsWordOrder(t *testing.T) {
	m := newMemClient()
	if err := WriteDWords(ctx, m, DeviceD, 200, []uint32{0x12345678}); err != nil {
		t.Fatal(err)
	}
	if m.words[200] != 0x5678 || m.words[201] != 0x1234 {
		t.Fatalf("ワード順が不正: D200=%#04x D201=%#04x", m.words[200], m.words[201])
	}
}

// 書いて読み戻すと一致し、呼び出しは各 1 回であること
func TestDWordsRoundTrip(t *testing.T) {
	m := newMemClient()
	in := []uint32{0, 1, 0x00010000, 0xFFFFFFFF, 0x12345678}
	if err := WriteDWords(ctx, m, DeviceD, 0, in); err != nil {
		t.Fatal(err)
	}
	got, err := ReadDWords(ctx, m, DeviceD, 0, uint16(len(in)))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("読み戻し不一致: got %#v want %#v", got, in)
	}
	if want := []uint16{10, 10}; !reflect.DeepEqual(m.points, want) {
		t.Fatalf("呼び出し点数が不正: got %v want %v", m.points, want)
	}
}

// 点数の範囲外は通信前にエラーになり、上限ちょうどは通ること
func TestDWordsValidation(t *testing.T) {
	m := newMemClient()
	for _, n := range []uint16{0, 481, 32768} {
		if _, err := ReadDWords(ctx, m, DeviceD, 0, n); err == nil {
			t.Errorf("Read %d 点がエラーにならなかった", n)
		}
	}
	for _, n := range []int{0, 481, 65537} {
		if err := WriteDWords(ctx, m, DeviceD, 0, make([]uint32, n)); err == nil {
			t.Errorf("Write %d 要素がエラーにならなかった", n)
		}
	}
	if len(m.points) != 0 {
		t.Fatalf("エラーなのに呼び出しが記録された: %v", m.points)
	}

	if _, err := ReadDWords(ctx, m, DeviceD, 0, 480); err != nil {
		t.Fatalf("480 点が失敗した: %v", err)
	}
	if want := []uint16{960}; !reflect.DeepEqual(m.points, want) {
		t.Fatalf("ReadWords に渡った点数が不正: got %v want %v", m.points, want)
	}
}

// PLC のエラーが包まれずに取り出せること
func TestDWordsErrorPropagation(t *testing.T) {
	m := newMemClient()
	m.err = &EndCodeError{Code: 0xC051}

	_, rerr := ReadDWords(ctx, m, DeviceD, 0, 1)
	werr := WriteDWords(ctx, m, DeviceD, 0, []uint32{1})
	for name, err := range map[string]error{"Read": rerr, "Write": werr} {
		var ece *EndCodeError
		if !errors.As(err, &ece) || ece.Code != 0xC051 {
			t.Errorf("%s: EndCodeError を取り出せない: %v", name, err)
		}
	}
}

// 要求より短い結果が返っても panic せずエラーになること
func TestReadDWordsShortResponse(t *testing.T) {
	m := newMemClient()
	m.short = true

	if _, err := ReadDWords(ctx, m, DeviceD, 0, 2); err == nil {
		t.Fatal("語数不足がエラーにならなかった")
	}
}
