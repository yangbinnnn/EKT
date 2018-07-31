package log

import (
	"fmt"
	"path"
	"runtime"
	"sync"

	"github.com/EducationEKT/EKT/xlog"
)

var once sync.Once
var l xlog.XLog

func InitLog(logPath string) {
	once.Do(func() {
		l = xlog.NewDailyLog(logPath)
	})
}

func shortfile() string {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "???"
		line = 0
	}
	return fmt.Sprintf("%v:%v: ", path.Base(file), line)
}

func Debug(msg string, args ...interface{}) {
	msg = shortfile() + msg
	l.Debug(msg, args...)
}

func Info(msg string, args ...interface{}) {
	msg = shortfile() + msg
	l.Info(msg, args...)
}

func Error(msg string, args ...interface{}) {
	msg = shortfile() + msg
	l.Error(msg, args...)
}

func Warn(msg string, args ...interface{}) {
	msg = shortfile() + msg
	l.Warn(msg, args...)
}

func Crit(msg string, args ...interface{}) {
	msg = shortfile() + msg
	l.Crit(msg, args...)
}
