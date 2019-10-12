package levlog

import (
	"github.com/syndtr/goleveldb/leveldb"
	"sync"
)

var (
	NOW_INDEX_B = []byte("NOW_INDEX")
	FIR_INDEX_B = []byte("FIR_INDEX")
)

type Levlog struct {
	Dbdir      string
	DB         *leveldb.DB
	DbLock     *sync.RWMutex
	FirstIndex int64
	NowIndex   int64
}
