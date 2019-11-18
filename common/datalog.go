package common

import "github.com/GenaroNetwork/GenaroCore/common/levlog"

var datalog *levlog.Levlog
var logdir string

func InitDataLogDir(dir string) {
	if logdir == "" {
		logdir = dir
	}
}

func GetLog() (*levlog.Levlog, error) {
	if datalog == nil {
		dlog, err := levlog.GenLevlog(logdir)
		if err != nil {
			return nil, err
		} else {
			datalog = dlog
		}
	}
	return datalog, nil
}
