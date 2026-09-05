package mcprotocol

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// transport は組み立て済みフレームを送信し、応答を 1 つ受け取る。
type transport interface {
	exchange(ctx context.Context, frame []byte) ([]byte, error)
}

// client は UDPClient / TCPClient に埋め込まれ、デバイス読み書きの共通処理を担う。
type client struct {
	cfg Config
	tr  transport
}

func (c *client) ReadWords(ctx context.Context, dev DeviceCode, head uint32, points uint16) ([]uint16, error) {
	if err := validateDevice(dev, false); err != nil {
		return nil, err
	}
	if err := validatePoints(points, MaxWordPoints); err != nil {
		return nil, err
	}

	req := buildDeviceHeader(CommandBatchRead, SubcommandWord, dev, head, points)
	data, err := c.exchange(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(data) != int(points)*2 {
		return nil, fmt.Errorf("unexpected data length: got %d want %d", len(data), int(points)*2)
	}

	values := make([]uint16, points)
	for i := range values {
		values[i] = uint16(data[i*2]) | uint16(data[i*2+1])<<8
	}
	return values, nil
}

func (c *client) WriteWords(ctx context.Context, dev DeviceCode, head uint32, values []uint16) error {
	if err := validateDevice(dev, false); err != nil {
		return err
	}
	if err := validatePoints(uint16(len(values)), MaxWordPoints); err != nil {
		return err
	}

	req := buildDeviceHeader(CommandBatchWrite, SubcommandWord, dev, head, uint16(len(values)))
	for _, v := range values {
		req = appendUint16LE(req, v)
	}
	_, err := c.exchange(ctx, req)
	return err
}

func (c *client) ReadBits(ctx context.Context, dev DeviceCode, head uint32, points uint16) ([]bool, error) {
	if err := validateDevice(dev, true); err != nil {
		return nil, err
	}
	if err := validatePoints(points, MaxBitPoints); err != nil {
		return nil, err
	}

	req := buildDeviceHeader(CommandBatchRead, SubcommandBit, dev, head, points)
	data, err := c.exchange(ctx, req)
	if err != nil {
		return nil, err
	}
	if want := (int(points) + 1) / 2; len(data) != want {
		return nil, fmt.Errorf("unexpected data length: got %d want %d", len(data), want)
	}
	return unpackBits(data, points), nil
}

func (c *client) WriteBits(ctx context.Context, dev DeviceCode, head uint32, values []bool) error {
	if err := validateDevice(dev, true); err != nil {
		return err
	}
	if err := validatePoints(uint16(len(values)), MaxBitPoints); err != nil {
		return err
	}

	req := buildDeviceHeader(CommandBatchWrite, SubcommandBit, dev, head, uint16(len(values)))
	req = append(req, packBits(values)...)
	_, err := c.exchange(ctx, req)
	return err
}

func (c *client) exchange(ctx context.Context, reqData []byte) ([]byte, error) {
	resp, err := c.tr.exchange(ctx, buildRequest(c.cfg, reqData))
	if err != nil {
		return nil, err
	}
	return parseResponse(resp)
}

func validateDevice(dev DeviceCode, bitAccess bool) error {
	if !dev.IsValid() {
		return fmt.Errorf("unknown device code: 0x%02X", byte(dev))
	}
	if bitAccess && !dev.IsBit() {
		return fmt.Errorf("%s is a word device: bit access not supported", dev)
	}
	return nil
}

func validatePoints(points, max uint16) error {
	if points == 0 {
		return errors.New("points must be > 0")
	}
	if points > max {
		return fmt.Errorf("too many points: %d (max %d)", points, max)
	}
	return nil
}

// deadline は ctx の期限と timeout のうち早いほうを返す。
func deadline(ctx context.Context, timeout time.Duration) time.Time {
	d := time.Now().Add(timeout)
	if cd, ok := ctx.Deadline(); ok && cd.Before(d) {
		return cd
	}
	return d
}
