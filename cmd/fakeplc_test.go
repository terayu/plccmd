package cmd_test

import (
	"encoding/binary"
	"net"
	"sync"
	"testing"
)

// fakePLC は MC プロトコル 3E フレーム(バイナリ)の一括読み書きに応答する擬似 PLC。
// 時刻同期ラダーの振る舞い(要求ビットを見て結果ビットを立て、要求ビットを落とす)も模擬する。
type fakePLC struct {
	conn *net.UDPConn
	addr string

	mu     sync.Mutex
	frames [][]byte          // 受信した要求フレーム
	words  map[uint32]uint16 // デバイスコード<<24 | 番号 -> 値

	// ラダーの振る舞い
	reqM          uint32
	triggered     bool
	reads         int
	completeAfter int  // トリガ後、何回目の読み出しで要求ビットを落とすか
	successBit    bool // 完了時に ReqM+2 を立てる
	errorBit      bool // 完了時に ReqM+3 を立てる
	neverComplete bool // 要求ビットを落とさない(タイムアウト検証用)

	// 異常応答
	endCode uint16 // 0 以外なら全要求にこの終了コードを返す
	garbage []byte // 非 nil なら全要求にこのバイト列をそのまま返す
}

func newFakePLC(t *testing.T) *fakePLC {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	p := &fakePLC{
		conn:          conn,
		addr:          conn.LocalAddr().String(),
		words:         map[uint32]uint16{},
		completeAfter: 2,
		successBit:    true,
	}
	t.Cleanup(func() { conn.Close() })
	go p.serve()
	return p
}

func (p *fakePLC) serve() {
	buf := make([]byte, 2048)
	for {
		n, addr, err := p.conn.ReadFrom(buf)
		if err != nil {
			return
		}
		if resp := p.handle(append([]byte(nil), buf[:n]...)); resp != nil {
			p.conn.WriteTo(resp, addr)
		}
	}
}

func (p *fakePLC) handle(req []byte) []byte {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.frames = append(p.frames, req)

	if p.garbage != nil {
		return p.garbage
	}
	if len(req) < 21 {
		return nil
	}

	route := req[2:7]
	command := binary.LittleEndian.Uint16(req[11:13])
	head := uint32(req[15]) | uint32(req[16])<<8 | uint32(req[17])<<16
	dev := req[18]
	points := binary.LittleEndian.Uint16(req[19:21])

	if p.endCode != 0 {
		return response(route, p.endCode, nil)
	}

	switch command {
	case 0x1401: // 一括書込み(ワード)
		data := req[21:]
		if len(data) < int(points)*2 {
			return nil
		}
		for i := 0; i < int(points); i++ {
			p.words[key(dev, head+uint32(i))] = binary.LittleEndian.Uint16(data[i*2:])
		}
		if dev == 0x90 { // M への書込み = 同期要求
			p.reqM = head
			p.triggered = true
			p.reads = 0
		}
		return response(route, 0, nil)

	case 0x0401: // 一括読出し(ワード)
		out := make([]byte, 0, int(points)*2)
		for i := 0; i < int(points); i++ {
			out = binary.LittleEndian.AppendUint16(out, p.read(dev, head+uint32(i)))
		}
		return response(route, 0, out)
	}
	return nil
}

// read はラダーの振る舞いを含めてデバイス値を返す。
func (p *fakePLC) read(dev byte, no uint32) uint16 {
	if dev == 0x90 && p.triggered && no == p.reqM {
		p.reads++
		if p.neverComplete || p.reads < p.completeAfter {
			return 0x0001 // 要求ビットは立ったまま
		}
		var w uint16 // bit0 を落として完了を通知
		if p.successBit {
			w |= 1 << 2 // ReqM+2
		}
		if p.errorBit {
			w |= 1 << 3 // ReqM+3
		}
		return w
	}
	return p.words[key(dev, no)]
}

// word は書き込まれたデバイス値を返す。
func (p *fakePLC) word(dev byte, no uint32) uint16 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.words[key(dev, no)]
}

// firstFrame は最初に受信した要求フレームを返す。
func (p *fakePLC) firstFrame(t *testing.T) []byte {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.frames) == 0 {
		t.Fatal("PLC が要求を受信していない")
	}
	return p.frames[0]
}

func key(dev byte, no uint32) uint32 { return uint32(dev)<<24 | no }

func response(route []byte, endCode uint16, data []byte) []byte {
	f := []byte{0xD0, 0x00}
	f = append(f, route...)
	f = binary.LittleEndian.AppendUint16(f, uint16(2+len(data)))
	f = binary.LittleEndian.AppendUint16(f, endCode)
	return append(f, data...)
}
