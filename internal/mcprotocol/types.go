package mcprotocol

type WordReaderWriter interface {
	BatchWriteWords(deviceCode byte, headDeviceNo uint32, values []uint16) error
	BatchReadWords(deviceCode byte, headDeviceNo uint32, points uint16) ([]uint16, error)
}
