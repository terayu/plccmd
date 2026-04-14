package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/comail/colog"
	"github.com/terayu/plccmd/cmd"
	"gopkg.in/natefinch/lumberjack.v2"
)

var loglevel string
var logfile string
var (
	version   string
	buildTime string
)

func flagUsage() {
	s := `
	plccmd
	Usage:
	example command [arguments]
	The commands are:
	plccmd timesync -address=192.168.1.100 -dialtimeout=10 -readtimeout=10 -writetimeout=10
	`
	_, _ = fmt.Fprintf(os.Stderr, "%v\n", s)
}

func init() {
	flag.CommandLine.Init("command", flag.ExitOnError)
	flag.StringVar(&loglevel, "loglevel", "info", "log level(trace,debug,info,warning,error,alert,panic)")
	flag.StringVar(&logfile, "logfile", "", "if use logfile,set log file path")
	flag.Usage = flagUsage
}
func printVersion() {
	if version == "" {
		version = "unknown"
	}
	if buildTime == "" {
		buildTime = "unknown"
	}
	log.Printf("debug: version: %s, buildTime: %s\n", version, buildTime)
}
func main() {

	flag.Parse()

	var ws []io.Writer
	ws = append(ws, os.Stderr)
	if logfile != "" {
		lj := lumberjack.Logger{
			Filename:   logfile, //
			MaxSize:    10,      // MB
			MaxAge:     60,      // days
			MaxBackups: 10,      // keep 7 rotation timestamps (newest first)
			Compress:   true,
			LocalTime:  false, // timestamps in local time (default: UTC)
		}
		ws = append(ws, &lj)
		_, abs := filepath.Abs(logfile)
		log.Printf("info: logfile: %s\n", abs)
	}

	mw := io.MultiWriter(ws...)

	colog.SetOutput(mw)
	colog.SetDefaultLevel(colog.LInfo)

	lv, err := colog.ParseLevel(loglevel)
	if err != nil {
		panic(err)
	}

	colog.SetFormatter(&colog.StdFormatter{
		Colors: false,
		Flag:   log.Ldate | log.Ltime | log.Llongfile,
	})

	colog.SetMinLevel(lv)
	colog.Register()
	printVersion()

	fs := make(map[string]func([]string) error)

	fs["timesync"] = cmd.TimeSyncCommand.Run

	if flag.NArg() > 0 {
		args := flag.Args()

		f, ok := fs[args[0]]
		if !ok {
			log.Panicf("NotFound Function:%v", args[0])
		}
		if err := f(args[1:]); err != nil {
			log.Panicf("err: %v", err)
		}
	}
}
