package mcprotocol

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

type UDPClient struct {
	conn *net.UDPConn
	addr *net.UDPAddr

	timeout time.Duration

	networkNo    byte
	pcNo         byte
	ioNo         uint16
	stationNo    byte
	monitorTimer uint16
}

func NewUDPClient(address string, timeout time.Duration) (*UDPClient, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}

	return &UDPClient{
		conn:         conn,
		addr:         udpAddr,
		timeout:      timeout,
		networkNo:    0x00,
		pcNo:         0xFF,
		ioNo:         0x03FF,
		stationNo:    0x00,
		monitorTimer: 0x0010,
	}, nil
}

func (c *UDPClient) Close() error {
	return c.conn.Close()
}
func (c *UDPClient) sendAndReceive(req []byte) ([]byte, error) {
	if err := c.conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, err
	}

	// 送信
	_, err := c.conn.Write(req)
	if err != nil {
		return nil, err
	}

	// 受信
	buf := make([]byte, 2048)
	n, err := c.conn.Read(buf)
	if err != nil {
		return nil, err
	}

	return buf[:n], nil
}
func (c *UDPClient) build3ERequest(reqData []byte) []byte {
	length := uint16(2 + len(reqData))

	frame := make([]byte, 0, 11+len(reqData))
	frame = append(frame, 0x50, 0x00)
	frame = append(frame, c.networkNo)
	frame = append(frame, c.pcNo)
	frame = appendUint16LE(frame, c.ioNo)
	frame = append(frame, c.stationNo)
	frame = appendUint16LE(frame, length)
	frame = appendUint16LE(frame, c.monitorTimer)
	frame = append(frame, reqData...)
	return frame
}

func (c *UDPClient) BatchWriteWords(deviceCode byte, head uint32, values []uint16) error {
	reqData := make([]byte, 0)

	reqData = appendUint16LE(reqData, 0x1401)
	reqData = appendUint16LE(reqData, 0x0000)

	reqData = appendDeviceNo3Bytes(reqData, head)
	reqData = append(reqData, deviceCode)
	reqData = appendUint16LE(reqData, uint16(len(values)))

	for _, v := range values {
		reqData = appendUint16LE(reqData, v)
	}

	frame := c.build3ERequest(reqData)

	resp, err := c.sendAndReceive(frame)
	if err != nil {
		return err
	}

	endCode := binary.LittleEndian.Uint16(resp[9:11])
	if endCode != 0 {
		return fmt.Errorf("PLC error: %04X", endCode)
	}

	return nil
}
func (c *UDPClient) BatchReadWords(deviceCode byte, head uint32, points uint16) ([]uint16, error) {

	reqData := make([]byte, 0)

	reqData = appendUint16LE(reqData, 0x0401)
	reqData = appendUint16LE(reqData, 0x0000)

	reqData = appendDeviceNo3Bytes(reqData, head)
	reqData = append(reqData, deviceCode)
	reqData = appendUint16LE(reqData, points)

	frame := c.build3ERequest(reqData)

	resp, err := c.sendAndReceive(frame)
	if err != nil {
		return nil, err
	}

	endCode := binary.LittleEndian.Uint16(resp[9:11])
	if endCode != 0 {
		return nil, fmt.Errorf("PLC error: %04X", endCode)
	}

	data := resp[11:]

	result := make([]uint16, points)
	for i := 0; i < int(points); i++ {
		result[i] = binary.LittleEndian.Uint16(data[i*2:])
	}

	return result, nil
}
