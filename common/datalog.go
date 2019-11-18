package common

import "mypkg/levlog"

var datalog *levlog.Levlog
var logdir string

func InitDir(dir string) {
	if logdir == "" {
		logdir = dir
	}
}

func GetLog() {
	if datalog == nil {

	}
}
