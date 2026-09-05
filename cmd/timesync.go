package cmd

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/terayu/plccmd/internal/timesync"
	"github.com/terayu/plccmd/mcprotocol"
)

type timeSyncCommand struct {
	fs           *flag.FlagSet
	Address      string `validate:"required"`
	DialTimeout  int
	ReadTimeout  int
	WriteTimeout int
	WaitTimeout  int
	NetworkNo    int
	PCNo         int
	IONo         int
	StationNo    int
	MonitorTimer int
	YearD        int `validate:"required"`
	MonthD       int `validate:"required"`
	DayD         int `validate:"required"`
	HourD        int `validate:"required"`
	MinuteD      int `validate:"required"`
	SecondD      int `validate:"required"`
	WdayD        int `validate:"required"`
	ReqM         int `validate:"required"`
}

var TimeSyncCommand timeSyncCommand

func init() {
	TimeSyncCommand.fs = flag.NewFlagSet("timesync", flag.ExitOnError)
	TimeSyncCommand.fs.StringVar(&TimeSyncCommand.Address, "address", "", "plc ip address")
	TimeSyncCommand.fs.IntVar(&TimeSyncCommand.DialTimeout, "dialtimeout", 5, "dial timeout (sec, tcp only)")
	TimeSyncCommand.fs.IntVar(&TimeSyncCommand.ReadTimeout, "readtimeout", 5, "read timeout (sec)")
	TimeSyncCommand.fs.IntVar(&TimeSyncCommand.WriteTimeout, "writetimeout", 5, "write timeout (sec)")
	TimeSyncCommand.fs.IntVar(&TimeSyncCommand.WaitTimeout, "waittimeout", 10, "wait timeout (sec)")
	TimeSyncCommand.fs.IntVar(&TimeSyncCommand.NetworkNo, "networkno", 0, "network no")
	TimeSyncCommand.fs.IntVar(&TimeSyncCommand.PCNo, "pcno", 255, "pc no")
	TimeSyncCommand.fs.IntVar(&TimeSyncCommand.IONo, "iono", 1023, "io no")
	TimeSyncCommand.fs.IntVar(&TimeSyncCommand.StationNo, "stationno", 0, "station no")
	TimeSyncCommand.fs.IntVar(&TimeSyncCommand.MonitorTimer, "monitortimer", 16, "monitor timer(sec)")
	TimeSyncCommand.fs.IntVar(&TimeSyncCommand.YearD, "yeard", 0, "year device")
	TimeSyncCommand.fs.IntVar(&TimeSyncCommand.MonthD, "monthd", 0, "month device")
	TimeSyncCommand.fs.IntVar(&TimeSyncCommand.DayD, "dayd", 0, "day device")
	TimeSyncCommand.fs.IntVar(&TimeSyncCommand.HourD, "hourd", 0, "hour device")
	TimeSyncCommand.fs.IntVar(&TimeSyncCommand.MinuteD, "minuted", 0, "minute device")
	TimeSyncCommand.fs.IntVar(&TimeSyncCommand.SecondD, "secondd", 0, "second device")
	TimeSyncCommand.fs.IntVar(&TimeSyncCommand.WdayD, "wdayd", 0, "wday device")
	TimeSyncCommand.fs.IntVar(&TimeSyncCommand.ReqM, "reqm", 0, "request device (reqm+2 is success, reqm+3 is error)")

	TimeSyncCommand.fs.VisitAll(func(f *flag.Flag) {
		if v := os.Getenv(strings.ToUpper(f.Name)); v != "" {
			if err := f.Value.Set(v); err != nil {
				panic(err)
			}
		}
	})
}

func (c *timeSyncCommand) Run(args []string) (err error) {
	log.Println("trace: timeSyncCommand.Run")

	if err = c.fs.Parse(args); err != nil {
		return
	}

	c.debugPara()

	validate := validator.New()
	if err = validate.Struct(c); err != nil {
		return err
	}

	mcCfg := mcprotocol.Config{
		Address:      c.Address,
		DialTimeout:  time.Duration(c.DialTimeout) * time.Second,
		ReadTimeout:  time.Duration(c.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(c.WriteTimeout) * time.Second,
		NetworkNo:    byte(c.NetworkNo),
		PCNo:         byte(c.PCNo),
		IONo:         uint16(c.IONo),
		StationNo:    byte(c.StationNo),
		MonitorTimer: uint16(c.MonitorTimer),
	}

	if err = executeTimeSync(mcCfg, time.Duration(c.WaitTimeout)*time.Second,
		c.YearD, c.MonthD, c.DayD, c.HourD, c.MinuteD, c.SecondD, c.WdayD, c.ReqM); err != nil {
		return
	}
	log.Println("info: time sync success")
	return
}
func executeTimeSync(mcCfg mcprotocol.Config, waitTimeout time.Duration, yearD, monthD, dayD, hourD, minuteD, secondD, wdayD, reqM int) (err error) {
	log.Println("trace: timeSyncCommand.executeTimeSync")

	client, err := mcprotocol.NewUDPClient(mcCfg)
	if err != nil {
		log.Printf("err: connect failed: %v", err)
		return
	}
	defer client.Close()

	service := timesync.NewService(client, timesync.Devices{
		YearD:   uint32(yearD),
		MonthD:  uint32(monthD),
		DayD:    uint32(dayD),
		HourD:   uint32(hourD),
		MinuteD: uint32(minuteD),
		SecondD: uint32(secondD),
		WdayD:   uint32(wdayD),
		ReqM:    uint32(reqM),
	})

	now := time.Now()
	result, err := service.SyncNow(context.Background(), now, waitTimeout)
	if err != nil {
		log.Printf("err: time sync failed result:%+v", result)
		return
	}
	if !result.Success {
		err = fmt.Errorf("plc reported time sync failure: result:%+v", result)
		log.Printf("err: %v", err)
		return
	}

	return

}
func (c *timeSyncCommand) debugPara() {
	log.Printf("debug: address:%s", c.Address)
	log.Printf("debug: dialtimeout:%d", c.DialTimeout)
	log.Printf("debug: readtimeout:%d", c.ReadTimeout)
	log.Printf("debug: writetimeout:%d", c.WriteTimeout)
	log.Printf("debug: waittimeout:%d", c.WaitTimeout)
	log.Printf("debug: networkno:%d", c.NetworkNo)
	log.Printf("debug: pcno:%d", c.PCNo)
	log.Printf("debug: iono:%d", c.IONo)
	log.Printf("debug: stationno:%d", c.StationNo)
	log.Printf("debug: monitortimer:%d", c.MonitorTimer)
	log.Printf("debug: yeard:%d", c.YearD)
	log.Printf("debug: monthd:%d", c.MonthD)
	log.Printf("debug: dayd:%d", c.DayD)
	log.Printf("debug: hourd:%d", c.HourD)
	log.Printf("debug: minuted:%d", c.MinuteD)
	log.Printf("debug: secondd:%d", c.SecondD)
	log.Printf("debug: wdayd:%d", c.WdayD)
	log.Printf("debug: reqm:%d", c.ReqM)
	return
}
