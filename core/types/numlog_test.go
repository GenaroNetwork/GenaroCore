package types

import (
	"encoding/json"
	"math/rand"
	"testing"
	"time"
)

var nowblock uint64
var nownum uint64

func init() {
	nowblock = 0
	nownum = 0
}

func GenNumLog() NumLog {
	rand.Seed(int64(time.Now().Nanosecond()))
	nowblock += uint64(rand.Intn(10))
	nownum += uint64(rand.Intn(10))

	log := NumLog{nowblock, nownum}
	return log
}

func display(t *testing.T, logs NumLogs) {
	for _, log := range logs {
		t.Log(log)
	}
}

func TestAddLog(t *testing.T) {
	logs := new(NumLogs)
	for i := 0; i < 100; i++ {
		logs.Add(GenNumLog())
	}
	display(t, *logs)
}

func TestFindFirst(t *testing.T) {
	logs := new(NumLogs)
	for i := 0; i < 1000000; i++ {
		//t.Log(i)
		log := GenNumLog()
		//t.Log(log)
		logs.Add(log)
	}
	t.Log()
	//start := time.Now()
	t.Log(time.Now())
	log, id := logs.GetFirstAfterBlockNum(3456000, 0, uint64(len(*logs)-1))
	//t.Log(time.Since(start))
	t.Log(time.Now())
	t.Log(id, log)
}

func TestFindLast(t *testing.T) {
	logs := new(NumLogs)
	for i := 0; i < 1000; i++ {
		t.Log(i)
		log := GenNumLog()
		t.Log(log)
		logs.Add(log)
	}
	t.Log()
	//start := time.Now()
	t.Log(time.Now())
	log, id := logs.GetLastBeforBlockNum(3056, 0, uint64(len(*logs)-1))
	//t.Log(time.Since(start))
	t.Log(time.Now())
	t.Log(id, log)
}

func TestGetRangeDiff(t *testing.T) {
	logs := new(NumLogs)
	for i := 0; i < 1000; i++ {
		t.Log(i)
		log := GenNumLog()
		t.Log(log)
		logs.Add(log)
	}
	diff := logs.GetRangeDiff(550, 3300)
	t.Log(diff)
	diff, blockNum := logs.GetLastDiff()
	t.Log(diff, blockNum)
}

func TestGetRangeDiff2(t *testing.T) {
	logs := new(NumLogs)
	numlog := NumLog{BlockNum: 0, Num: 200}
	logs.Add(numlog)
	diff := logs.GetRangeDiff(0, 200)
	t.Log(diff)
	diff, blockNum := logs.GetLastDiff()
	t.Log(diff, blockNum)
}

func TestGenaroData(t *testing.T) {
	data := new(GenaroData)
	data.Stake = 10000
	data.Heft = 10000
	logs := new(NumLogs)
	for i := 0; i < 1000; i++ {
		//t.Log(i)
		log := GenNumLog()
		//t.Log(log)
		logs.Add(log)
	}
	data.HeftLog = *logs
	data.StakeLog = *logs

	b, _ := json.Marshal(data)
	t.Log(b)
	var data2 GenaroData
	json.Unmarshal(b, &data2)

	t.Log(data2.Stake)
	t.Log(data2.StakeLog.GetRangeDiff(200, 2000))
}
