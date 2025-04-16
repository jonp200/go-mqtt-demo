package logger

import (
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	glog "github.com/labstack/gommon/log"
)

type Fatal struct{}

func (f Fatal) Println(v ...interface{}) {
	glog.Fatal(v...)
}

func (f Fatal) Printf(format string, v ...interface{}) {
	glog.Fatalf(format, v...)
}

type Panic struct{}

func (p Panic) Println(v ...interface{}) {
	glog.Panic(v...)
}

func (p Panic) Printf(format string, v ...interface{}) {
	glog.Panicf(format, v...)
}

type Error struct{}

func (e Error) Println(v ...interface{}) {
	glog.Error(v...)
}

func (e Error) Printf(format string, v ...interface{}) {
	glog.Errorf(format, v...)
}

type Warn struct{}

func (w Warn) Println(v ...interface{}) {
	glog.Warn(v...)
}

func (w Warn) Printf(format string, v ...interface{}) {
	glog.Warnf(format, v...)
}

type Info struct{}

func (i Info) Println(v ...interface{}) {
	glog.Info(v...)
}

func (i Info) Printf(format string, v ...interface{}) {
	glog.Infof(format, v...)
}

type Debug struct{}

func (d Debug) Println(v ...interface{}) {
	glog.Debug(v...)
}

func (d Debug) Printf(format string, v ...interface{}) {
	glog.Debugf(format, v...)
}

type Plain struct{}

func (p Plain) Println(v ...interface{}) {
	glog.Print(v...)
}

func (p Plain) Printf(format string, v ...interface{}) {
	glog.Printf(format, v...)
}

func Init() {
	glog.SetHeader("${time_rfc3339} ${level} ${short_file}:${line}")
	glog.EnableColor()

	mqtt.ERROR = Error{}
	mqtt.CRITICAL = Error{}
	mqtt.WARN = Warn{}

	if os.Getenv("DEBUG") == "1" {
		glog.SetLevel(glog.DEBUG)
		mqtt.DEBUG = Debug{}
	}
}
