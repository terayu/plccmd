package timesync

import (
	"context"
	"fmt"
	"time"

	"github.com/terayu/plccmd/mcprotocol"
)

type Service struct {
	io      WordReaderWriter
	devices Devices
}

func NewService(io WordReaderWriter, devices Devices) *Service {
	return &Service{
		io:      io,
		devices: devices,
	}
}

func (s *Service) SyncNow(ctx context.Context, t time.Time, waitTimeout time.Duration) (*SyncResult, error) {
	if err := validateTime(t); err != nil {
		return nil, err
	}

	if err := s.writeTime(ctx, t); err != nil {
		return nil, err
	}

	if err := s.trigger(ctx); err != nil {
		return nil, err
	}

	return s.waitCompletion(ctx, waitTimeout, t)
}

func (s *Service) writeTime(ctx context.Context, t time.Time) error {
	values := []uint16{
		uint16(t.Year()),
		uint16(t.Month()),
		uint16(t.Day()),
		uint16(t.Hour()),
		uint16(t.Minute()),
		uint16(t.Second()),
		uint16(t.Weekday()),
	}

	return s.io.WriteWords(ctx, mcprotocol.DeviceD, s.devices.YearD, values)
}

func (s *Service) trigger(ctx context.Context) error {
	return s.io.WriteWords(ctx, mcprotocol.DeviceM, s.devices.ReqM, []uint16{0x0001})
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
				return nil, fmt.Errorf("timeout waiting PLC to reset M%d", s.devices.ReqM)
			}

			words, err := s.io.ReadWords(ctx, mcprotocol.DeviceM, s.devices.ReqM, 1)
			if err != nil {
				return nil, fmt.Errorf("read M%d..M%d failed: %w", s.devices.ReqM, s.devices.ReqM+15, err)
			}

			// s.devices.ReqM is request sync
			// s.devices.ReqM+2 is success time sync
			// s.devices.ReqM+3 is error time sync
			w := words[0]
			requestFlag := (w & (1 << 0)) != 0
			successFlag := (w & (1 << 2)) != 0
			errFlag := (w & (1 << 3)) != 0

			if !requestFlag {
				return &SyncResult{
					RequestTime: requestTime,
					CompletedAt: time.Now(),
					Success:     successFlag && !errFlag,
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
