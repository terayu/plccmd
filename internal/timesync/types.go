package timesync

import "time"

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
