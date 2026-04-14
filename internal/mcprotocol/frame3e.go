package mcprotocol

import "encoding/binary"

const (
	DeviceCodeM byte = 0x90
	DeviceCodeD byte = 0xA8

	CommandBatchReadWords  uint16 = 0x0401
	CommandBatchWriteWords uint16 = 0x1401
	SubcommandQLWord       uint16 = 0x0000
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
