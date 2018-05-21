package state

import "fmt"

// deal for NumLogs

type NumLog struct {
	BlockNum uint64
	Num uint64
}

type NumLogs []NumLog

// add a log in NumLogs
func (logs *NumLogs) add(log NumLog) {
	lenth := len(*logs)
	if lenth == 0 {
		*logs = append(*logs,log)
		fmt.Println(len(*logs))
	}else if (*logs)[lenth-1].BlockNum == log.BlockNum {
		(*logs)[lenth-1].Num = log.Num
	} else if (*logs)[lenth-1].BlockNum <= log.BlockNum{
		*logs = append(*logs,log)
	}
}

func (logs NumLogs) getFirst() NumLog{
	return logs[0]
}

func (logs NumLogs) getLast() NumLog{
	len := len(logs)
	return logs[len-1]
}

// get the NumLog at or after blockNum
// startId,endId is the range to search
func (logs NumLogs) GetFirstAfterBlockNum(blockNum uint64, startId,endId uint64) (*NumLog,uint64) {
	if startId > endId || endId > uint64(len(logs) -1) {
		return nil,0
	}

	if blockNum > logs[endId].BlockNum {
		return nil,0
	}
	return  logs.getFirstAfterBlockNum(blockNum,startId,endId)
}

// get the NumLog at or after blockNum
// startId,endId is the range to search
func (logs NumLogs) getFirstAfterBlockNum(blockNum uint64, startId,endId uint64) (*NumLog,uint64) {
	if blockNum > logs[endId].BlockNum {
		return nil,0
	}

	if blockNum <= logs[startId].BlockNum {
		return  &logs[startId],startId
	}

	log,id := logs.getFirstAfterBlockNum(blockNum,startId,(endId-startId)/2+startId)
	if log != nil {
		return log,id
	} else {
		log,id = logs.getFirstAfterBlockNum(blockNum,(endId-startId)/2+startId+1,endId)
		return log,id
	}
}


// get the NumLog at or after blockNum
// startId,endId is the range to search
func (logs NumLogs) GetLastBeforBlockNum(blockNum uint64, startId,endId uint64) (*NumLog,uint64) {
	if startId > endId || endId > uint64(len(logs) -1) {
		return nil,0
	}

	if blockNum > logs[endId].BlockNum {
		return nil,0
	}
	return  logs.getLastBeforBlockNum(blockNum,startId,endId)
}

// get the NumLog at or after blockNum
// startId,endId is the range to search
func (logs NumLogs) getLastBeforBlockNum(blockNum uint64, startId,endId uint64) (*NumLog,uint64) {
	if blockNum < logs[startId].BlockNum {
		return nil,0
	}

	if blockNum >= logs[endId].BlockNum {
		return  &logs[endId],endId
	}

	log,id := logs.getLastBeforBlockNum(blockNum,(endId-startId)/2+startId+1,endId)
	if log != nil {
		return log,id
	} else {
		log,id = logs.getLastBeforBlockNum(blockNum,startId,(endId-startId)/2+startId)
		return log,id
	}
}

// get the diff for given range
func (logs NumLogs) GetRangeDiff(blockNumStart uint64, blockNumEnd uint64) uint64{
	lenth := uint64(len(logs) -1)
	logStart,idStart := logs.GetFirstAfterBlockNum(blockNumStart,0,lenth)
	logEnd,_ := logs.GetLastBeforBlockNum(blockNumEnd,idStart,lenth)
	var diff uint64
	if logStart != nil && logEnd != nil && logEnd.Num>logStart.Num{
		diff = logEnd.Num - logStart.Num
	}
	return diff
}
