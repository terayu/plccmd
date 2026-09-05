package timesync

import (
	"context"
	"time"

	"github.com/terayu/plccmd/mcprotocol"
)

// WordReaderWriter は時刻同期に必要なワードアクセスだけを切り出したもの。
// mcprotocol.Client が満たす。
type WordReaderWriter interface {
	ReadWords(ctx context.Context, dev mcprotocol.DeviceCode, head uint32, points uint16) ([]uint16, error)
	WriteWords(ctx context.Context, dev mcprotocol.DeviceCode, head uint32, values []uint16) error
}

type Devices struct {
	YearD   uint32
	MonthD  uint32
	DayD    uint32
	HourD   uint32
	MinuteD uint32
	SecondD uint32
	WdayD   uint32
	ReqM    uint32
}

type SyncResult struct {
	RequestTime time.Time
	CompletedAt time.Time
	Success     bool
	ErrorFlag   bool
}
