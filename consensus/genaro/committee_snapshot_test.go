package genaro

import (
	"testing"
	"github.com/GenaroNetwork/Genaro-Core/params"
	"math/rand"
	"github.com/GenaroNetwork/Genaro-Core/common"
	"fmt"
	"io/ioutil"
	"os"
	"github.com/GenaroNetwork/Genaro-Core/ethdb"
)

func genHash(n int) []byte{
	hash := make([]byte,n)
	for i:=0;i<n;i++ {
		hash[i] = byte(rand.Int())
	}
	return hash
}

func displaySnapshot(snapshot CommitteeSnapshot){
	fmt.Print("CommitteeSize:")
	fmt.Println(snapshot.CommitteeSize)
	fmt.Print("CommitteeRank:")
	for _,committee := range snapshot.CommitteeRank {
		fmt.Println(committee.String())
	}
	fmt.Println("Committee:")
	for k,v := range snapshot.Committee {
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

func TestSnapshot(t *testing.T){
	db, remove := newTestLDB()
	defer remove()
	blockHash := new(common.Hash)
	blockHash.SetBytes(genHash(32))
	committeeRank := genAddrs(10)
	committee := make(map[common.Address]CommitteeInfo)
	for _,addr := range committeeRank{
		info := *(new(CommitteeInfo))
		committee[addr] = info
	}

	snapshot := newSnapshot(params.MainnetChainConfig.Genaro,0,*blockHash,0,committeeRank,committee)
	displaySnapshot(*snapshot)


	snapshot.store(db)

	snapshot2,err := loadSnapshot(params.MainnetChainConfig.Genaro,db,0)
	if err != nil {
		t.Error(err)
	}
	displaySnapshot(*snapshot2)

	snapshot3 := snapshot2.copy()
	displaySnapshot(*snapshot3)

	fmt.Println(snapshot3.getCurrentRankIndex(committeeRank[5]))
	for _,addr := range snapshot3.rank() {
		fmt.Println(addr.String())
	}

	fmt.Println(GetDependTurnByBlockNumber(params.MainnetChainConfig.Genaro,300000))

	fmt.Println(GetCommiteeWrittenBlockNumberByTurn(params.MainnetChainConfig.Genaro,100))

	for i,addr := range committeeRank{
		fmt.Print(i)
		fmt.Print("   ")
		fmt.Println(snapshot3.inturn(456, addr))
	}

	fmt.Println(GetFirstBlockNumberOfEpoch(params.MainnetChainConfig.Genaro, 20))
	fmt.Println(GetLastBlockNumberOfEpoch(params.MainnetChainConfig.Genaro, 20))
}

