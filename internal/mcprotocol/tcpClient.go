package mcprotocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

type Config struct {
	Address      string
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	NetworkNo    byte
	PCNo         byte
	IONo         uint16
	StationNo    byte
	MonitorTimer uint16
}

type TcpClient struct {
	conn net.Conn
	cfg  Config
}

func NewTcpClient(cfg Config) (*TcpClient, error) {
	conn, err := net.DialTimeout("tcp", cfg.Address, cfg.DialTimeout)
	if err != nil {
		return nil, err
	}
	return &TcpClient{
		conn: conn,
		cfg:  cfg,
	}, nil
}

func (c *TcpClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *TcpClient) BatchWriteWords(deviceCode byte, headDeviceNo uint32, values []uint16) error {
	if len(values) == 0 {
		return errors.New("values is empty")
	}
	if len(values) > 960 {
		return fmt.Errorf("too many points: %d", len(values))
	}

	reqData := make([]byte, 0, 10+len(values)*2)
	reqData = appendUint16LE(reqData, CommandBatchWriteWords)
	reqData = appendUint16LE(reqData, SubcommandQLWord)
	reqData = appendDeviceNo3Bytes(reqData, headDeviceNo)
	reqData = append(reqData, deviceCode)
	reqData = appendUint16LE(reqData, uint16(len(values)))

	for _, v := range values {
		reqData = appendUint16LE(reqData, v)
	}

	frame := c.build3ERequest(reqData)

	if err := c.writeAll(frame); err != nil {
		return err
	}

	resp, err := c.read3EResponse()
	if err != nil {
		return err
	}

	endCode, err := parseEndCode(resp)
	if err != nil {
		return err
	}
	if endCode != 0x0000 {
		return fmt.Errorf("PLC end code: 0x%04X", endCode)
	}

	return nil
}

func (c *TcpClient) BatchReadWords(deviceCode byte, headDeviceNo uint32, points uint16) ([]uint16, error) {
	if points == 0 {
		return nil, errors.New("points must be > 0")
	}
	if points > 960 {
		return nil, fmt.Errorf("too many points: %d", points)
	}

	reqData := make([]byte, 0, 10)
	reqData = appendUint16LE(reqData, CommandBatchReadWords)
	reqData = appendUint16LE(reqData, SubcommandQLWord)
	reqData = appendDeviceNo3Bytes(reqData, headDeviceNo)
	reqData = append(reqData, deviceCode)
	reqData = appendUint16LE(reqData, points)

	frame := c.build3ERequest(reqData)

	if err := c.writeAll(frame); err != nil {
		return nil, err
	}

	resp, err := c.read3EResponse()
	if err != nil {
		return nil, err
	}

	endCode, err := parseEndCode(resp)
	if err != nil {
		return nil, err
	}
	if endCode != 0x0000 {
		return nil, fmt.Errorf("PLC end code: 0x%04X", endCode)
	}

	if len(resp) < 11 {
		return nil, fmt.Errorf("response too short: %d", len(resp))
	}

	data := resp[11:]
	if len(data) != int(points)*2 {
		return nil, fmt.Errorf("unexpected data length: got %d want %d", len(data), int(points)*2)
	}

	result := make([]uint16, points)
	for i := 0; i < int(points); i++ {
		result[i] = binary.LittleEndian.Uint16(data[i*2 : i*2+2])
	}

	return result, nil
}

func (c *TcpClient) build3ERequest(reqData []byte) []byte {
	length := uint16(2 + len(reqData))

	frame := make([]byte, 0, 11+len(reqData))
	frame = append(frame, 0x50, 0x00)
	frame = append(frame, c.cfg.NetworkNo)
	frame = append(frame, c.cfg.PCNo)
	frame = appendUint16LE(frame, c.cfg.IONo)
	frame = append(frame, c.cfg.StationNo)
	frame = appendUint16LE(frame, length)
	frame = appendUint16LE(frame, c.cfg.MonitorTimer)
	frame = append(frame, reqData...)
	return frame
}

func (c *TcpClient) read3EResponse() ([]byte, error) {
	header := make([]byte, 9)
	if _, err := c.readFull(header); err != nil {
		return nil, err
	}

	if header[0] != 0xD0 || header[1] != 0x00 {
		return nil, fmt.Errorf("unexpected response subheader: %02X %02X", header[0], header[1])
	}

	respLen := binary.LittleEndian.Uint16(header[7:9])
	body := make([]byte, respLen)
	if _, err := c.readFull(body); err != nil {
		return nil, err
	}

	return append(header, body...), nil
}

func parseEndCode(resp []byte) (uint16, error) {
	if len(resp) < 11 {
		return 0, fmt.Errorf("response too short: %d", len(resp))
	}
	return binary.LittleEndian.Uint16(resp[9:11]), nil
}

func (c *TcpClient) writeAll(buf []byte) error {
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteTimeout)); err != nil {
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

func (c *TcpClient) readFull(buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		if err := c.conn.SetReadDeadline(time.Now().Add(c.cfg.ReadTimeout)); err != nil {
			return total, err
		}
		n, err := c.conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
