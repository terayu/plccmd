package mcprotocol

import (
	"encoding/binary"
	"fmt"
)

const (
	CommandBatchRead  uint16 = 0x0401
	CommandBatchWrite uint16 = 0x1401

	SubcommandWord uint16 = 0x0000 // ワード単位
	SubcommandBit  uint16 = 0x0001 // ビット単位

	// 1 リクエストで扱える最大点数。
	MaxWordPoints = 960
	MaxBitPoints  = 3584

	// 応答の最小長。サブヘッダ(2)+ルート(5)+応答データ長(2)+終了コード(2)。
	responseHeaderLen = 11
)

func appendUint16LE(dst []byte, v uint16) []byte {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], v)
	return append(dst, b[:]...)
}

func appendDeviceNo3Bytes(dst []byte, n uint32) []byte {
	return append(dst,
		byte(n&0xFF),
		byte((n>>8)&0xFF),
		byte((n>>16)&0xFF),
	)
}

// buildRequest は要求データに 3E フレームのヘッダを付与する。
func buildRequest(cfg Config, reqData []byte) []byte {
	length := uint16(2 + len(reqData)) // 監視タイマ(2) + 要求データ

	frame := make([]byte, 0, responseHeaderLen+len(reqData))
	frame = append(frame, 0x50, 0x00)
	frame = append(frame, cfg.NetworkNo)
	frame = append(frame, cfg.PCNo)
	frame = appendUint16LE(frame, cfg.IONo)
	frame = append(frame, cfg.StationNo)
	frame = appendUint16LE(frame, length)
	frame = appendUint16LE(frame, cfg.MonitorTimer)
	frame = append(frame, reqData...)
	return frame
}

// buildDeviceHeader はコマンド・サブコマンド・デバイス指定までを組み立てる。
func buildDeviceHeader(command, subcommand uint16, dev DeviceCode, head uint32, points uint16) []byte {
	b := make([]byte, 0, 10)
	b = appendUint16LE(b, command)
	b = appendUint16LE(b, subcommand)
	b = appendDeviceNo3Bytes(b, head)
	b = append(b, byte(dev))
	b = appendUint16LE(b, points)
	return b
}

// parseResponse は応答のサブヘッダと終了コードを検証し、データ部を返す。
func parseResponse(resp []byte) ([]byte, error) {
	if len(resp) < responseHeaderLen {
		return nil, fmt.Errorf("response too short: %d", len(resp))
	}
	if resp[0] != 0xD0 || resp[1] != 0x00 {
		return nil, fmt.Errorf("unexpected response subheader: %02X %02X", resp[0], resp[1])
	}
	if code := binary.LittleEndian.Uint16(resp[9:11]); code != 0 {
		return nil, &EndCodeError{Code: code}
	}
	return resp[responseHeaderLen:], nil
}

// packBits はビット単位書き込み用に 1 バイトへ 2 点ぶんを詰める。
// 上位 4 ビットが先頭の点、下位 4 ビットが次の点。点数が奇数なら末尾の下位 4 ビットは 0。
func packBits(values []bool) []byte {
	b := make([]byte, (len(values)+1)/2)
	for i, v := range values {
		if !v {
			continue
		}
		if i%2 == 0 {
			b[i/2] |= 0x10
		} else {
			b[i/2] |= 0x01
		}
	}
	return b
}

// unpackBits はビット単位読み出しの応答を points 点ぶん展開する。
func unpackBits(data []byte, points uint16) []bool {
	values := make([]bool, points)
	for i := range values {
		if i%2 == 0 {
			values[i] = data[i/2]&0x10 != 0
		} else {
			values[i] = data[i/2]&0x01 != 0
		}
	}
	return values
}
