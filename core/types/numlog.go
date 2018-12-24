package types

// deal for NumLogs

type NumLog struct {
	BlockNum uint64
	Num      uint64
}

type NumLogs []NumLog

// add a log in NumLogs
func (logs *NumLogs) Add(log NumLog) {
	lenth := len(*logs)
	if lenth == 0 {
		*logs = append(*logs, log)
	} else if (*logs)[lenth-1].BlockNum == log.BlockNum {
		(*logs)[lenth-1].Num = log.Num
	} else if (*logs)[lenth-1].BlockNum <= log.BlockNum {
		*logs = append(*logs, log)
	}
}

// delete log befor blockNumBefor
func (logs *NumLogs) Del(blockNumBefor uint64) {
	lenth := len(*logs)
	if lenth == 0 {
		return
	}
	if blockNumBefor == 0 {
		return
	}
	if logs.GetFirst().BlockNum > blockNumBefor {
		return
	}
	if logs.GetLast().BlockNum < blockNumBefor {
		*logs = (*logs)[lenth-1 : lenth]
		return
	}

	deleteIndex := 0
	for i := 0; i < lenth; i++ {
		if (*logs)[i].BlockNum < blockNumBefor {
			deleteIndex = i
		}
	}
	*logs = (*logs)[deleteIndex:]
}

func (logs NumLogs) GetFirst() NumLog {
	len := len(logs)
	if len > 0 {
		return logs[0]
	} else {
		return NumLog{0, 0}
	}

}

func (logs NumLogs) GetLast() NumLog {
	len := len(logs)
	if len > 0 {
		return logs[len-1]
	} else {
		return NumLog{0, 0}
	}
}

func (logs NumLogs) GetLastDiff() (diff, blockNum uint64) {
	len := len(logs)
	if len == 0 {
		diff = 0
		blockNum = 0
	} else if len == 1 {
		diff = logs[len-1].Num
		blockNum = logs[len-1].BlockNum
	} else if len > 1 {
		diff = logs[len-1].Num - logs[len-2].Num
		blockNum = logs[len-1].BlockNum
	}

	return
}

// get the NumLog at or after blockNum
// startId,endId is the range to search
func (logs NumLogs) GetFirstAfterBlockNum(blockNum uint64, startId, endId uint64) (*NumLog, uint64) {
	if startId > endId || endId > uint64(len(logs)-1) {
		return nil, 0
	}

	//if blockNum > logs[endId].BlockNum {
	//	return nil,0
	//}
	return logs.getFirstAfterBlockNum(blockNum, startId, endId)
}

// get the NumLog at or after blockNum
// startId,endId is the range to search
func (logs NumLogs) getFirstAfterBlockNum(blockNum uint64, startId, endId uint64) (*NumLog, uint64) {
	if blockNum > logs[endId].BlockNum {
		return nil, 0
	}

	if blockNum <= logs[startId].BlockNum {
		return &logs[startId], startId
	}

	log, id := logs.getFirstAfterBlockNum(blockNum, startId, (endId-startId)/2+startId)
	if log != nil {
		return log, id
	} else {
		log, id = logs.getFirstAfterBlockNum(blockNum, (endId-startId)/2+startId+1, endId)
		return log, id
	}
}

// get the NumLog at or after blockNum
// startId,endId is the range to search
func (logs NumLogs) GetLastBeforBlockNum(blockNum uint64, startId, endId uint64) (*NumLog, uint64) {
	if startId > endId || endId > uint64(len(logs)-1) {
		return nil, 0
	}

	//if blockNum > logs[endId].BlockNum {
	//	return nil,0
	//}
	return logs.getLastBeforBlockNum(blockNum, startId, endId)
}

// get the NumLog at or after blockNum
// startId,endId is the range to search
func (logs NumLogs) getLastBeforBlockNum(blockNum uint64, startId, endId uint64) (*NumLog, uint64) {
	if blockNum < logs[startId].BlockNum {
		return nil, 0
	}

	if blockNum >= logs[endId].BlockNum {
		return &logs[endId], endId
	}

	log, id := logs.getLastBeforBlockNum(blockNum, (endId-startId)/2+startId+1, endId)
	if log != nil {
		return log, id
	} else {
		log, id = logs.getLastBeforBlockNum(blockNum, startId, (endId-startId)/2+startId)
		return log, id
	}
}

// get the diff for given range
func (logs NumLogs) GetRangeDiff(blockNumStart uint64, blockNumEnd uint64) uint64 {
	if len(logs) == 0 {
		return 0
	}
	lenth := uint64(len(logs) - 1)
	logStart, idStart := logs.GetFirstAfterBlockNum(blockNumStart, 0, lenth)
	logEnd, _ := logs.GetLastBeforBlockNum(blockNumEnd, idStart, lenth)
	var diff uint64
	if logStart != nil && logEnd != nil && logEnd.Num > logStart.Num {
		diff = logEnd.Num - logStart.Num
	}
	// fix zero bug
	if blockNumStart == 0 && logStart.BlockNum == 0 {
		diff += logStart.Num
	}

	return diff
}
