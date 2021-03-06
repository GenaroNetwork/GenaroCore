package common

import (
	"os"
	"strconv"
	"testing"
	"time"
)

func TestLog(t *testing.T) {
	dbdir := "/tmp/db"
	InitDataLogDir(dbdir)
	os.RemoveAll(dbdir)
	dlog, err := GetLog()
	if err != nil {
		t.Fatal(err)
	}
	defer dlog.Close()

	for i := 0; i < 5000; i++ {
		dlog.Log("LOG:" + strconv.Itoa(i))
	}

	ret, _ := dlog.GetLogsInPage(5)
	t.Log(ret)

	ret, _ = dlog.GetLogsInPage(50)
	t.Log(ret)

	for i := 0; i < 60; i++ {
		dlog.DelFirstPage()
	}

	first := dlog.GetFirstPageNum()
	t.Log(first)
}

func TestLogMore(t *testing.T) {
	dbdir := "/tmp/db"
	InitDataLogDir(dbdir)
	os.RemoveAll(dbdir)
	dlog, err := GetLog()
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	for i := 0; i < 50000; i++ {
		dlog.Log("LOG:" + strconv.Itoa(i))

	}
	t.Log("50000 log cost time:")
	t.Log(time.Since(start))

	start = time.Now()
	for i := int64(0); i < 50000; i++ {
		dlog.GetLog(i)
	}
	t.Log("50000 get cost time:")
	t.Log(time.Since(start))

}

func TestLogPage(t *testing.T) {
	dbdir := "/tmp/db"
	InitDataLogDir(dbdir)
	os.RemoveAll(dbdir)
	dlog, err := GetLog()
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 5000; i++ {
		dlog.Log("LOG:" + strconv.Itoa(i))
	}

	pagefirst := dlog.GetFirstPageNum()
	t.Log("first page:", pagefirst)

	pageLast := dlog.GetLastPageNum()
	t.Log("last page:", pageLast)

	data, err := dlog.GetLogsInPage(pagefirst)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("page first")
	t.Log(data)

	data, err = dlog.GetLogsInPage(pageLast)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("page last")
	t.Log(data)
}
