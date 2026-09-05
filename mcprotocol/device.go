package mcprotocol

import "fmt"

// DeviceCode は MC プロトコル 3E フレーム（バイナリコード）のデバイスコードを表す。
type DeviceCode byte

// ビットデバイス。ビット単位・ワード単位のどちらでもアクセスできる。
const (
	DeviceX  DeviceCode = 0x9C // 入力
	DeviceY  DeviceCode = 0x9D // 出力
	DeviceM  DeviceCode = 0x90 // 内部リレー
	DeviceL  DeviceCode = 0x92 // ラッチリレー
	DeviceB  DeviceCode = 0xA0 // リンクリレー
	DeviceTS DeviceCode = 0xC1 // タイマ接点
	DeviceTC DeviceCode = 0xC0 // タイマコイル
	DeviceCS DeviceCode = 0xC4 // カウンタ接点
	DeviceCC DeviceCode = 0xC3 // カウンタコイル
)

// ワードデバイス。ワード単位でのみアクセスできる。
const (
	DeviceD  DeviceCode = 0xA8 // データレジスタ
	DeviceW  DeviceCode = 0xB4 // リンクレジスタ
	DeviceR  DeviceCode = 0xAF // ファイルレジスタ
	DeviceZR DeviceCode = 0xB0 // ファイルレジスタ(連番アクセス)
	DeviceTN DeviceCode = 0xC2 // タイマ現在値
	DeviceCN DeviceCode = 0xC5 // カウンタ現在値
)

var deviceNames = map[DeviceCode]string{
	DeviceX:  "X",
	DeviceY:  "Y",
	DeviceM:  "M",
	DeviceL:  "L",
	DeviceB:  "B",
	DeviceTS: "TS",
	DeviceTC: "TC",
	DeviceCS: "CS",
	DeviceCC: "CC",
	DeviceD:  "D",
	DeviceW:  "W",
	DeviceR:  "R",
	DeviceZR: "ZR",
	DeviceTN: "TN",
	DeviceCN: "CN",
}

func (d DeviceCode) String() string {
	if name, ok := deviceNames[d]; ok {
		return name
	}
	return fmt.Sprintf("DeviceCode(0x%02X)", byte(d))
}

// IsBit はビット単位のアクセスが可能なデバイスかを返す。
func (d DeviceCode) IsBit() bool {
	switch d {
	case DeviceX, DeviceY, DeviceM, DeviceL, DeviceB, DeviceTS, DeviceTC, DeviceCS, DeviceCC:
		return true
	}
	return false
}

// IsValid は既知のデバイスコードかを返す。
func (d DeviceCode) IsValid() bool {
	_, ok := deviceNames[d]
	return ok
}
