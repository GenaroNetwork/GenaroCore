package levlog

import (
	"encoding/binary"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"sync"
)

var (
	NOW_INDEX_B = []byte("NOW_INDEX")
	FIR_INDEX_B = []byte("FIR_INDEX")
)

const PageSize int64 = 20

func Int64ToBytes(i int64) []byte {
	var buf = make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(i))
	return buf
}

func BytesToInt64(buf []byte) int64 {
	return int64(binary.BigEndian.Uint64(buf))
}

type Levlog struct {
	Dbdir      string
	DB         *leveldb.DB
	DbLock     *sync.RWMutex
	FirstIndex int64
	NowIndex   int64
}

func GenLevlog(dbdir string) (*Levlog, error) {
	levlog := new(Levlog)
	var err error
	levlog.DB, err = leveldb.OpenFile(dbdir, nil)
	if err != nil {
		return nil, err
	}
	levlog.Dbdir = dbdir
	levlog.DbLock = new(sync.RWMutex)
	levlog.NowIndex, err = levlog.getNowIndex()
	if err != nil {
		return nil, err
	}
	levlog.FirstIndex, err = levlog.getFirstIndex()
	if err != nil {
		return nil, err
	}
	return levlog, nil
}

func (levlog *Levlog) Close() {
	levlog.DB.Close()
}

func (levlog *Levlog) Log(logstr string) error {
	levlog.DbLock.Lock()
	defer levlog.DbLock.Unlock()

	err := levlog.DB.Put(Int64ToBytes(levlog.NowIndex), []byte(logstr), nil)
	if err != nil {
		return err
	}
	levlog.NowIndex++

	err = levlog.DB.Put(NOW_INDEX_B, Int64ToBytes(levlog.NowIndex), nil)
	if err != nil {
		return err
	}
	return nil
}

func (levlog *Levlog) GetLogsInPage(page int64) ([]string, error) {
	start := page * PageSize
	end := (page + 1) * PageSize

	if start < levlog.FirstIndex || start > levlog.NowIndex {
		return nil, errors.New("page not exist")
	}
	if end > levlog.NowIndex {
		end = levlog.NowIndex
	}

	return levlog.GetLogs(start, end)
}

func (levlog *Levlog) GetLogs(start int64, end int64) ([]string, error) {
	if start < levlog.FirstIndex || end > levlog.NowIndex || start >= end {
		return nil, errors.New("Exceeding the scope")
	}
	levlog.DbLock.RLock()
	defer levlog.DbLock.RUnlock()

	ret := make([]string, end-start)
	for i := start; i < end; i++ {
		val, err := levlog.DB.Get(Int64ToBytes(i), nil)
		if err != nil {
			return nil, err
		}
		ret[i-start] = string(val)
	}
	return ret, nil
}



func (levlog *Levlog) getFirstIndex() (int64, error) {
	var firIndex int64 = 0

	val, err := levlog.DB.Get(FIR_INDEX_B, nil)
	if err != nil && err != errors.ErrNotFound {
		return 0, err
	} else if err == nil {
		firIndex = BytesToInt64(val)
	} else {
		firIndex = 0
		levlog.DB.Put(FIR_INDEX_B, Int64ToBytes(firIndex), nil)
	}
	return firIndex, nil
}

func (levlog *Levlog) GetLog(idx int64) (string, error) {
	if idx < levlog.FirstIndex || idx >= levlog.NowIndex {
		return "", errors.New("Exceeding the scope")
	}
	levlog.DbLock.RLock()
	defer levlog.DbLock.RUnlock()
	val, err := levlog.DB.Get(Int64ToBytes(idx), nil)
	if err != nil {
		return "", err
	}
	return string(val), nil
}

func (levlog *Levlog) DelFirstPage() error {
	if levlog.NowIndex-levlog.FirstIndex < 50 {
		return errors.New("log lesser 50")
	}

	levlog.DbLock.Lock()
	defer levlog.DbLock.Unlock()

	for i := levlog.FirstIndex; i < levlog.FirstIndex+PageSize; i++ {
		err := levlog.DB.Delete(Int64ToBytes(i), nil)
		if err != nil {
			return err
		}
	}
	levlog.FirstIndex = levlog.FirstIndex + PageSize
	err := levlog.DB.Put(FIR_INDEX_B, Int64ToBytes(levlog.FirstIndex), nil)
	return err
}

func (levlog *Levlog) getNowIndex() (int64, error) {
	var nowIndex int64 = 0
	val, err := levlog.DB.Get(NOW_INDEX_B, nil)
	if err != nil && err != errors.ErrNotFound {
		return 0, err
	} else if err == nil {
		nowIndex = BytesToInt64(val)
	} else {
		nowIndex = 0
		levlog.DB.Put(NOW_INDEX_B, Int64ToBytes(nowIndex), nil)
	}
	return nowIndex, nil
}

func (levlog *Levlog) GetFirstPageNum() int64 {
	return levlog.FirstIndex / PageSize
}

func (levlog *Levlog) GetLastPageNum() int64 {
	return levlog.NowIndex / PageSize - 1
}
