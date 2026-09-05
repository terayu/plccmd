package mcprotocol

import (
	"context"
	"fmt"
	"time"
)

// Config は PLC への接続設定とルート情報を保持する。
type Config struct {
	Address      string // "IP:ポート"
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	NetworkNo    byte
	PCNo         byte
	IONo         uint16
	StationNo    byte
	MonitorTimer uint16 // 250ms 単位
}

// DefaultConfig は自局アクセス向けの標準的な設定を返す。
func DefaultConfig(address string) Config {
	return Config{
		Address:      address,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		NetworkNo:    0x00,
		PCNo:         0xFF,
		IONo:         0x03FF,
		StationNo:    0x00,
		MonitorTimer: 0x0010,
	}
}

// Client は PLC デバイスの読み書きを行う。*UDPClient と *TCPClient が実装する。
//
// head はデバイス番号をそのまま 10 進で指定する。X/Y/B/W/ZR など
// 三菱の表記が 16 進のデバイスは、変換後の値を渡すこと（例: X10 なら 16）。
type Client interface {
	ReadWords(ctx context.Context, dev DeviceCode, head uint32, points uint16) ([]uint16, error)
	WriteWords(ctx context.Context, dev DeviceCode, head uint32, values []uint16) error
	ReadBits(ctx context.Context, dev DeviceCode, head uint32, points uint16) ([]bool, error)
	WriteBits(ctx context.Context, dev DeviceCode, head uint32, values []bool) error
	Close() error
}

// EndCodeError は PLC が 0 以外の終了コードを返したことを表す。
type EndCodeError struct {
	Code uint16
}

func (e *EndCodeError) Error() string {
	return fmt.Sprintf("mc protocol end code: 0x%04X", e.Code)
}
