package common

import (
	"testing"
	"strconv"
	"os"
)

func TestLog(t *testing.T) {
	dbdir := "/tmp/db"
	InitDataLogDir(dbdir)
	os.RemoveAll(dbdir)
	dlog,err := GetLog()
	if err != nil {
		t.Fatal(err)
	}
	defer dlog.Close()

	for i:=0;i<5000;i++{
		dlog.Log("LOG:"+strconv.Itoa(i))
	}

	ret,_ := dlog.GetLogsInPage(5)
	t.Log(ret)

	ret,_ = dlog.GetLogsInPage(50)
	t.Log(ret)

	for i:=0;i<60;i++{
		dlog.DelFirstPage()
	}

	first := dlog.GetFirstPageNum()
	t.Log(first)

}
