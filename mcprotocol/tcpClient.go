package mcprotocol

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
)

// TCPClient は TCP で MC プロトコル 3E フレームを送受信する。
type TCPClient struct {
	client
	conn net.Conn
}

var _ Client = (*TCPClient)(nil)

// NewTCPClient は cfg.Address へ TCP 接続を確立する。
//
// 接続の確立は ctx でキャンセルできる。cfg.DialTimeout と ctx の期限のうち
// 早いほうが締切になる。cfg.DialTimeout が 0 なら ctx の期限だけが効く。
func NewTCPClient(ctx context.Context, cfg Config) (*TCPClient, error) {
	d := net.Dialer{Timeout: cfg.DialTimeout}
	conn, err := d.DialContext(ctx, "tcp", cfg.Address)
	if err != nil {
		return nil, err
	}

	c := &TCPClient{conn: conn}
	c.client = client{cfg: cfg, tr: c}
	return c, nil
}

func (c *TCPClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *TCPClient) exchange(ctx context.Context, frame []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := c.writeAll(ctx, frame); err != nil {
		return nil, err
	}
	return c.readResponse(ctx)
}

// readResponse は応答データ長を見てフレーム全体を読み切る。
func (c *TCPClient) readResponse(ctx context.Context) ([]byte, error) {
	header := make([]byte, 9)
	if err := c.readFull(ctx, header); err != nil {
		return nil, err
	}

	if header[0] != 0xD0 || header[1] != 0x00 {
		return nil, fmt.Errorf("unexpected response subheader: %02X %02X", header[0], header[1])
	}

	respLen := binary.LittleEndian.Uint16(header[7:9])
	body := make([]byte, respLen)
	if err := c.readFull(ctx, body); err != nil {
		return nil, err
	}

	return append(header, body...), nil
}

func (c *TCPClient) writeAll(ctx context.Context, buf []byte) error {
	if err := c.conn.SetWriteDeadline(deadline(ctx, c.cfg.WriteTimeout)); err != nil {
		return err
	}

	total := 0
	for total < len(buf) {
		n, err := c.conn.Write(buf[total:])
		total += n
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *TCPClient) readFull(ctx context.Context, buf []byte) error {
	dl := deadline(ctx, c.cfg.ReadTimeout)

	total := 0
	for total < len(buf) {
		if err := c.conn.SetReadDeadline(dl); err != nil {
			return err
		}
		n, err := c.conn.Read(buf[total:])
		total += n
		if err != nil {
			return err
		}
	}
	return nil
}
