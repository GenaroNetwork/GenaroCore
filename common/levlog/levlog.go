package levlog

import (
	"github.com/syndtr/goleveldb/leveldb"
	"encoding/binary"
	"sync"
)

var (
	NOW_INDEX_B = []byte("NOW_INDEX")
	FIR_INDEX_B = []byte("FIR_INDEX")
)

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

