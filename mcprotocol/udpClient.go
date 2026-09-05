package mcprotocol

import (
	"context"
	"fmt"
	"net"
)

// UDPClient は UDP で MC プロトコル 3E フレームを送受信する。
//
// UDP には要求と応答を対応付ける仕組みが無く、3E フレームにも
// 4E のようなシリアル番号が無い。応答が ReadTimeout を超えて遅延すると、
// 以降は 1 つずれた応答を読み続けることになる。確実性が必要な場合は
// TCPClient を使うこと。
type UDPClient struct {
	client
	conn *net.UDPConn
}

var _ Client = (*UDPClient)(nil)

// NewUDPClient は cfg の宛先へ UDP ソケットを開く。cfg.DialTimeout は使用しない。
func NewUDPClient(cfg Config) (*UDPClient, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", cfg.Address)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}

	c := &UDPClient{conn: conn}
	c.client = client{cfg: cfg, tr: c}
	return c, nil
}

func (c *UDPClient) Close() error {
	return c.conn.Close()
}

func (c *UDPClient) exchange(ctx context.Context, frame []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := c.conn.SetDeadline(deadline(ctx, c.cfg.ReadTimeout)); err != nil {
		return nil, err
	}

	// 送信
	if _, err := c.conn.Write(frame); err != nil {
		return nil, err
	}

	// 受信
	buf := make([]byte, maxDatagramSize)
	n, err := c.conn.Read(buf)
	if err != nil {
		return nil, err
	}
	if n == len(buf) {
		return nil, fmt.Errorf("response too large: >= %d bytes", n)
	}

	// cap を n に切り詰め、以降のスライスで未受信領域を読まないようにする
	return buf[:n:n], nil
}

// 応答の最大長は 11 + 960 ワード * 2 = 1931 バイト。切り詰めの検知用に余裕を持たせる。
const maxDatagramSize = 2048
