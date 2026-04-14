package timesync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/terayu/plccmd/internal/mcprotocol"
)

type Service struct {
	io      mcprotocol.WordReaderWriter
	devices Devices
}

func NewService(io mcprotocol.WordReaderWriter, devices Devices) *Service {
	return &Service{
		io:      io,
		devices: devices,
	}
}

func (s *Service) SyncNow(ctx context.Context, t time.Time, waitTimeout time.Duration) (*SyncResult, error) {
	if err := validateTime(t); err != nil {
		return nil, err
	}

	if err := s.writeTime(t); err != nil {
		return nil, err
	}

	if err := s.trigger(); err != nil {
		return nil, err
	}

	return s.waitCompletion(ctx, waitTimeout, t)
}

func (s *Service) writeTime(t time.Time) error {
	values := []uint16{
		uint16(t.Year()),
		uint16(t.Month()),
		uint16(t.Day()),
		uint16(t.Hour()),
		uint16(t.Minute()),
		uint16(t.Second()),
		uint16(t.Weekday()),
	}

	return s.io.BatchWriteWords(mcprotocol.DeviceCodeD, s.devices.YearD, values)
}

func (s *Service) trigger() error {
	return s.io.BatchWriteWords(mcprotocol.DeviceCodeM, s.devices.ReqM, []uint16{0x0001})
}

func (s *Service) waitCompletion(ctx context.Context, timeout time.Duration, requestTime time.Time) (*SyncResult, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil, errors.New("timeout waiting PLC to reset M100")
			}

			words, err := s.io.BatchReadWords(mcprotocol.DeviceCodeM, s.devices.ReqM, 1)
			if err != nil {
				return nil, fmt.Errorf("read M100..M115 failed: %w", err)
			}

			// s.devices.ReqM is request sync
			// s.devices.ReqM+1 is success time sync
			// s.devices.ReqM+2 is error time sync
			w := words[0]
			requestFlag := (w & (1 << 0)) != 0
			successFlag := (w & (1 << 2)) != 0
			errFlag := (w & (1 << 3)) != 0

			if !requestFlag {
				return &SyncResult{
					RequestTime: requestTime,
					CompletedAt: time.Now(),
					Success:     !errFlag && (successFlag || !errFlag),
					ErrorFlag:   errFlag,
				}, nil
			}
		}
	}
}

func validateTime(t time.Time) error {
	year := t.Year()
	if year < 2000 || year > 2099 {
		return fmt.Errorf("year out of supported range: %d", year)
	}
	return nil
}
