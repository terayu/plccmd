package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	PLC     PLCConfig     `yaml:"plc"`
	Devices DevicesConfig `yaml:"devices"`
}

type PLCConfig struct {
	Address      string      `yaml:"address"`
	DialTimeout  string      `yaml:"dial_timeout"`
	ReadTimeout  string      `yaml:"read_timeout"`
	WriteTimeout string      `yaml:"write_timeout"`
	WaitTimeout  string      `yaml:"wait_timeout"`
	Route        RouteConfig `yaml:"route"`
}

type RouteConfig struct {
	NetworkNo    byte   `yaml:"network_no"`
	PCNo         byte   `yaml:"pc_no"`
	IONo         uint16 `yaml:"io_no"`
	StationNo    byte   `yaml:"station_no"`
	MonitorTimer uint16 `yaml:"monitor_timer"`
}

type DevicesConfig struct {
	YearD   uint32 `yaml:"year_d"`
	MonthD  uint32 `yaml:"month_d"`
	DayD    uint32 `yaml:"day_d"`
	HourD   uint32 `yaml:"hour_d"`
	MinuteD uint32 `yaml:"minute_d"`
	SecondD uint32 `yaml:"second_d"`
	WdayD   uint32 `yaml:"wday_d"`
	ReqM    uint32 `yaml:"req_m"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file failed: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse yaml failed: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.PLC.Address == "" {
		return fmt.Errorf("plc.address is required")
	}

	if _, err := time.ParseDuration(c.PLC.DialTimeout); err != nil {
		return fmt.Errorf("invalid plc.dial_timeout: %w", err)
	}
	if _, err := time.ParseDuration(c.PLC.ReadTimeout); err != nil {
		return fmt.Errorf("invalid plc.read_timeout: %w", err)
	}
	if _, err := time.ParseDuration(c.PLC.WriteTimeout); err != nil {
		return fmt.Errorf("invalid plc.write_timeout: %w", err)
	}
	if _, err := time.ParseDuration(c.PLC.WaitTimeout); err != nil {
		return fmt.Errorf("invalid plc.wait_timeout: %w", err)
	}

	return nil
}

func (c *Config) MustDialTimeout() time.Duration {
	d, _ := time.ParseDuration(c.PLC.DialTimeout)
	return d
}

func (c *Config) MustReadTimeout() time.Duration {
	d, _ := time.ParseDuration(c.PLC.ReadTimeout)
	return d
}

func (c *Config) MustWriteTimeout() time.Duration {
	d, _ := time.ParseDuration(c.PLC.WriteTimeout)
	return d
}

func (c *Config) MustWaitTimeout() time.Duration {
	d, _ := time.ParseDuration(c.PLC.WaitTimeout)
	return d
}
