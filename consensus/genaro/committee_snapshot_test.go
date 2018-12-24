package genaro

import (
	"bytes"
	"fmt"
	"github.com/GenaroNetwork/GenaroCore/common"
	"github.com/GenaroNetwork/GenaroCore/ethdb"
	"github.com/GenaroNetwork/GenaroCore/params"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
)

func genHash(n int) []byte {
	hash := make([]byte, n)
	for i := 0; i < n; i++ {
		hash[i] = byte(rand.Int())
	}
	return hash
}

func genProportion(n uint64) []uint64 {
	proportion := make([]uint64, 10)
	for i, _ := range proportion {
		proportion[i] = uint64(rand.Int63())
	}
	return proportion
}

func displaySnapshot(snapshot CommitteeSnapshot) {
	fmt.Print("CommitteeSize:")
	fmt.Println(snapshot.CommitteeSize)
	fmt.Print("CommitteeRank:")
	for _, committee := range snapshot.CommitteeRank {
		fmt.Println(committee.String())
	}
	fmt.Println("Committee:")
	for k, v := range snapshot.Committee {
		fmt.Print(k.String())
		fmt.Print(":")
		fmt.Println(v)
	}

	fmt.Print("config:")
	fmt.Println(*snapshot.config)
	fmt.Print("EpochNumber:")
	fmt.Println(snapshot.EpochNumber)
	fmt.Print("WriteBlockHash:")
	fmt.Println(snapshot.WriteBlockHash.String())
	fmt.Print("WriteBlockNumber:")
	fmt.Println(snapshot.WriteBlockNumber)
}

func newTestLDB() (*ethdb.LDBDatabase, func()) {
	dirname, err := ioutil.TempDir(os.TempDir(), "ethdb_test_")
	if err != nil {
		panic("failed to create test file: " + err.Error())
	}
	db, err := ethdb.NewLDBDatabase(dirname, 0, 0)
	if err != nil {
		panic("failed to create test database: " + err.Error())
	}

	return db, func() {
		db.Close()
		os.RemoveAll(dirname)
	}
}

func TestNewSnapshot(t *testing.T) {
	blockHash := new(common.Hash)
	blockHash.SetBytes(genHash(32))
	committeeRank := genAddrs(10)
	proportion := []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	genaroConfig := &params.GenaroConfig{
		Epoch:            5000,
		BlockInterval:    10,
		ElectionPeriod:   1,
		ValidPeriod:      1,
		CurrencyRates:    10,
		CommitteeMaxSize: 5,
	}
	committeeAccountBinding := make(map[common.Address][]common.Address)
	snapshot := newSnapshot(genaroConfig, 0, *blockHash, 0, committeeRank, proportion, committeeAccountBinding)

	if snapshot.config.Epoch != 5000 || snapshot.config.CommitteeMaxSize != 5 || snapshot.config.BlockInterval != 10 ||
		snapshot.config.ElectionPeriod != 1 || snapshot.config.ValidPeriod != 1 || snapshot.config.CurrencyRates != 10 {
		t.Errorf("genaro config is not match!")
	}
	if snapshot.CommitteeSize != 5 {
		t.Errorf("commitee size get %v but expect 5", snapshot.CommitteeSize)
	}
	if snapshot.WriteBlockNumber != 0 {
		t.Errorf("WriteBlockNumber get %v but expect 0", snapshot.CommitteeSize)
	}
	if !bytes.Equal(snapshot.WriteBlockHash.Bytes(), blockHash.Bytes()) {
		t.Errorf("WriteBlockHash get %v but expect %v", snapshot.WriteBlockHash.Bytes(), blockHash.Bytes())
	}
	pre := snapshot.Committee[snapshot.CommitteeRank[0]]
	for i := 1; i < len(snapshot.CommitteeRank); i++ {
		if snapshot.Committee[snapshot.CommitteeRank[i]] < pre {
			t.Errorf("the commit rank is error, now  [%v] is better than pre [%v]", snapshot.Committee[snapshot.CommitteeRank[i]], pre)
		}
		pre = snapshot.Committee[snapshot.CommitteeRank[i]]
	}

	genaroConfig.CommitteeMaxSize = 20
	snapshot = newSnapshot(genaroConfig, 0, *blockHash, 0, committeeRank, proportion, committeeAccountBinding)
	if snapshot.CommitteeSize != 10 {
		t.Errorf("commitee size get %v but expect 10", snapshot.CommitteeSize)
	}
}

func TestStoreAndLoadSnapshot(t *testing.T) {
	db, remove := newTestLDB()
	defer remove()

	blockHash := new(common.Hash)
	blockHash.SetBytes(genHash(32))
	committeeRank := genAddrs(10)
	proportion := []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	genaroConfig := &params.GenaroConfig{
		Epoch:            5000,
		BlockInterval:    10,
		ElectionPeriod:   1,
		ValidPeriod:      1,
		CurrencyRates:    10,
		CommitteeMaxSize: 5,
	}
	committeeAccountBinding := make(map[common.Address][]common.Address)
	snapshot := newSnapshot(genaroConfig, 0, *blockHash, 0, committeeRank, proportion, committeeAccountBinding)
	err := snapshot.store(db)
	if err != nil {
		t.Errorf("store error [%v]", err)
	}
	_, err = loadSnapshot(genaroConfig, db, 0)
	if err != nil {
		t.Errorf("loadSnapshot error [%v]", err)
	}
}

func TestCopy(t *testing.T) {
	blockHash := new(common.Hash)
	blockHash.SetBytes(genHash(32))
	committeeRank := genAddrs(10)
	proportion := []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	genaroConfig := &params.GenaroConfig{
		Epoch:            5000,
		BlockInterval:    10,
		ElectionPeriod:   1,
		ValidPeriod:      1,
		CurrencyRates:    10,
		CommitteeMaxSize: 5,
	}
	committeeAccountBinding := make(map[common.Address][]common.Address)
	snapshot := newSnapshot(genaroConfig, 0, *blockHash, 0, committeeRank, proportion, committeeAccountBinding)

	cp := snapshot.copy()

	if cp.config.Epoch != 5000 || snapshot.config.CommitteeMaxSize != 5 || snapshot.config.BlockInterval != 10 ||
		snapshot.config.ElectionPeriod != 1 || snapshot.config.ValidPeriod != 1 || snapshot.config.CurrencyRates != 10 {
		t.Errorf("genaro config is not match!")
	}
	if snapshot.CommitteeSize != 5 {
		t.Errorf("commitee size get %v but expect 5", snapshot.CommitteeSize)
	}
	if snapshot.WriteBlockNumber != 0 {
		t.Errorf("WriteBlockNumber get %v but expect 0", snapshot.CommitteeSize)
	}
	if !bytes.Equal(snapshot.WriteBlockHash.Bytes(), blockHash.Bytes()) {
		t.Errorf("WriteBlockHash get %v bute expect %v", snapshot.WriteBlockHash.Bytes(), blockHash.Bytes())
	}
	pre := snapshot.Committee[snapshot.CommitteeRank[0]]
	for i := 1; i < len(snapshot.CommitteeRank); i++ {
		if snapshot.Committee[snapshot.CommitteeRank[i]] < pre {
			t.Errorf("the commit rank is error, now  %v is better than pre %v", snapshot.Committee[snapshot.CommitteeRank[i]], pre)
		}
		pre = snapshot.Committee[snapshot.CommitteeRank[i]]
	}
}

func TestGetCurrentRankIndex(t *testing.T) {
	blockHash := new(common.Hash)
	blockHash.SetBytes(genHash(32))
	committeeRank := genAddrs(10)
	proportion := []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	genaroConfig := &params.GenaroConfig{
		Epoch:            5000,
		BlockInterval:    10,
		ElectionPeriod:   1,
		ValidPeriod:      1,
		CurrencyRates:    10,
		CommitteeMaxSize: 5,
	}
	committeeAccountBinding := make(map[common.Address][]common.Address)
	snapshot := newSnapshot(genaroConfig, 0, *blockHash, 0, committeeRank, proportion, committeeAccountBinding)
	if snapshot.getCurrentRankIndex(committeeRank[0]) != 0 {
		t.Errorf("the index get %v but except 0", snapshot.getCurrentRankIndex(committeeRank[0]))
	}
	if snapshot.getCurrentRankIndex(committeeRank[1]) != 1 {
		t.Errorf("the index get %v but except 0", snapshot.getCurrentRankIndex(committeeRank[0]))
	}
	if snapshot.getCurrentRankIndex(committeeRank[2]) != 2 {
		t.Errorf("the index get %v but except 0", snapshot.getCurrentRankIndex(committeeRank[0]))
	}
	if snapshot.getCurrentRankIndex(committeeRank[3]) != 3 {
		t.Errorf("the index get %v but except 0", snapshot.getCurrentRankIndex(committeeRank[0]))
	}
	if snapshot.getCurrentRankIndex(committeeRank[4]) != 4 {
		t.Errorf("the index get %v but except 0", snapshot.getCurrentRankIndex(committeeRank[0]))
	}
}

func TestInturn(t *testing.T) {
	blockHash := new(common.Hash)
	blockHash.SetBytes(genHash(32))
	committeeRank := genAddrs(10)
	proportion := []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	genaroConfig := &params.GenaroConfig{
		Epoch:            5000,
		BlockInterval:    10,
		ElectionPeriod:   1,
		ValidPeriod:      1,
		CurrencyRates:    10,
		CommitteeMaxSize: 5,
	}
	committeeAccountBinding := make(map[common.Address][]common.Address)
	snapshot := newSnapshot(genaroConfig, 0, *blockHash, 0, committeeRank, proportion, committeeAccountBinding)
	loopSize := snapshot.config.BlockInterval * snapshot.CommitteeSize
	for i := 0; i < 10000; i++ {
		rank := uint64(i) % snapshot.config.Epoch % loopSize / snapshot.config.BlockInterval
		inturnAddress := snapshot.CommitteeRank[rank]
		if !snapshot.inturn(uint64(i), inturnAddress) {
			t.Errorf("the inturn address is not %v(rank is %v)", inturnAddress, rank)
		}
	}
}

func TestGetFirstBlockNumberOfEpoch(t *testing.T) {
	genaroConfig := &params.GenaroConfig{
		Epoch:            5000,
		BlockInterval:    10,
		ElectionPeriod:   1,
		ValidPeriod:      1,
		CurrencyRates:    10,
		CommitteeMaxSize: 5,
	}

	for i := 0; i < 100; i++ {
		if GetFirstBlockNumberOfEpoch(genaroConfig, uint64(i)) != uint64(i)*genaroConfig.Epoch {
			t.Errorf("the first block number of epoch get %v but except %v",
				GetFirstBlockNumberOfEpoch(genaroConfig, 0), uint64(i)*genaroConfig.Epoch)
		}
	}

}
