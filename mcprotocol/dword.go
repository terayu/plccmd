package mcprotocol

import (
	"context"
	"fmt"
)

// ReadDWords は head から points 点のダブルワード(32bit)を読み出す。
//
// 1 点は 2 ワードで、下位ワードが先（head+2i が下位、head+2i+1 が上位）。
// 上位と下位がちぎれないよう、ReadWords を 1 回だけ呼ぶ。points の上限は MaxWordPoints/2。
func ReadDWords(ctx context.Context, c Client, dev DeviceCode, head uint32, points uint16) ([]uint32, error) {
	if err := validatePoints(int(points), MaxWordPoints/2); err != nil {
		return nil, err
	}

	words, err := c.ReadWords(ctx, dev, head, points*2)
	if err != nil {
		return nil, err
	}
	if len(words) != int(points)*2 {
		return nil, fmt.Errorf("unexpected word count: got %d want %d", len(words), int(points)*2)
	}

	values := make([]uint32, points)
	for i := range values {
		values[i] = uint32(words[i*2]) | uint32(words[i*2+1])<<16
	}
	return values, nil
}

// WriteDWords は head から values をダブルワード(32bit)として書き込む。
//
// ワード順は ReadDWords と同じく下位が先。WriteWords を 1 回だけ呼ぶ。
func WriteDWords(ctx context.Context, c Client, dev DeviceCode, head uint32, values []uint32) error {
	if err := validatePoints(len(values), MaxWordPoints/2); err != nil {
		return err
	}

	words := make([]uint16, 0, len(values)*2)
	for _, v := range values {
		words = append(words, uint16(v), uint16(v>>16))
	}
	return c.WriteWords(ctx, dev, head, words)
}
