package genaro

import (
	"bytes"
	"fmt"
	"github.com/GenaroNetwork/GenaroCore/common"
	"github.com/GenaroNetwork/GenaroCore/common/hexutil"
	"github.com/GenaroNetwork/GenaroCore/core/state"
	"github.com/GenaroNetwork/GenaroCore/core/types"
	"github.com/GenaroNetwork/GenaroCore/crypto"
	"github.com/GenaroNetwork/GenaroCore/ethdb"
	"github.com/GenaroNetwork/GenaroCore/params"
	"log"
	"math/big"
	"testing"
)

func TestGetDependTurnByBlockNumber(t *testing.T) {
	var turn uint64 = 0

	for i := 0; i < 20; i++ {
		turn = GetDependTurnByBlockNumber(params.MainnetChainConfig.Genaro, uint64(i))
		fmt.Println(turn)
	}

}

func TestAuthor(t *testing.T) {
	db, remove := newTestLDB()
	defer remove()

	genaro := New(params.MainnetChainConfig.Genaro, db)
	fmt.Printf("%s\n", genaro)
	fmt.Println(genaro.signer)
	fmt.Println(genaro.signFn)

	n := 10
	addrs := genAddrs(n)
	byt := CreateCommitteeRankByte(addrs)
	head := types.Header{
		Extra: byt,
	}

	prikey, err := crypto.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}
	addr := crypto.PubkeyToAddress(prikey.PublicKey)
	fmt.Println("addr")
	fmt.Println(hexutil.Encode(addr.Bytes()))

	hash := sigHash(&head)
	sig, err := crypto.Sign(hash.Bytes(), prikey)
	if err != nil {
		log.Fatal(err)
	}

	genaro.signer = addr
	SetHeaderSignature(&head, sig)

	signer, err := genaro.Author(&head)
	if err != nil {
		t.Error(err)
	}
	fmt.Println("singer")
	fmt.Println(hexutil.Encode(signer.Bytes()))

	if bytes.Compare(addr.Bytes(), signer.Bytes()) != 0 {
		t.Error("sign error")
	}

}

func TestNew(t *testing.T) {
	db, remove := newTestLDB()
	defer remove()
	genaroConfig := &params.GenaroConfig{
		BlockInterval:    10,
		ElectionPeriod:   1,
		ValidPeriod:      1,
		CurrencyRates:    10,
		CommitteeMaxSize: 5,
	}
	genaro := New(genaroConfig, db)
	if genaro.config.Epoch != epochLength {
		t.Errorf("the genaro config epoch get %v but except %v", genaro.config.Epoch, epochLength)
	}
	if genaro.recents.Len() != 0 {
		t.Errorf("the genaro recents len get %v, but except %v", genaro.recents.Len(), inmemorySnapshots)
	}
	genaroConfig.Epoch = 200
	genaro = New(genaroConfig, db)
	if genaro.config.Epoch != 200 {
		t.Errorf("the genaro config epoch get %v but except 200", genaro.config.Epoch)
	}
}

func newTestStateDB() *state.StateDB {
	diskdb := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(diskdb))

	return statedb
}

func TestUpdateSpecialBlock(t *testing.T) {
	genaroConfig := &params.GenaroConfig{
		Epoch:            5000,
		BlockInterval:    10,
		ElectionPeriod:   1,
		ValidPeriod:      1,
		CurrencyRates:    10,
		CommitteeMaxSize: 5,
	}
	header := &types.Header{
		Number:   big.NewInt(0),
		Time:     big.NewInt(0),
		Coinbase: common.HexToAddress("0x50a7658e5155206dc78eafb80e6a94640b274648"),
		Extra:    make([]byte, 0),
	}

	updateSpecialBlock(genaroConfig, header, newTestStateDB())
}

func TestCandidateInfos(t *testing.T) {
	var candidateInfos state.CandidateInfos
	candidateInfos = make([]state.CandidateInfo, 4)
	candidateInfos[0] = state.CandidateInfo{
		Signer: common.StringToAddress("xx"),
		Heft:   10,
		Stake:  25,
	}
	candidateInfos[1] = state.CandidateInfo{
		Signer: common.StringToAddress("xx"),
		Heft:   11,
		Stake:  35,
	}
	candidateInfos[2] = state.CandidateInfo{
		Signer: common.StringToAddress("xx"),
		Heft:   12,
		Stake:  45,
	}
	candidateInfos[3] = state.CandidateInfo{
		Signer: common.StringToAddress("xx"),
		Heft:   13,
		Stake:  15,
	}

	candidateInfos.Apply()
	commiteeRank, proportion := state.Rank(candidateInfos)
	fmt.Println("Rank")
	fmt.Println(commiteeRank)
	fmt.Println(proportion)
	fmt.Println(candidateInfos)
	commiteeRank, proportion = state.RankWithLenth(candidateInfos, 3, 5000)
	fmt.Println("RankWithLenth")
	fmt.Println(commiteeRank)
	fmt.Println(proportion)
}

func TestGetCoinCofficient(t *testing.T) {
	genaroConfig := &params.GenaroConfig{
		Epoch:            86400,
		Period:           1,
		BlockInterval:    10,
		ElectionPeriod:   1,
		ValidPeriod:      1,
		CurrencyRates:    10,
		CommitteeMaxSize: 5,
	}
	cofficient := getCoinCofficient(genaroConfig, big.NewInt(500), big.NewInt(20857142), common.Base*50/100, common.Base*7/100)
	fmt.Println(cofficient)
}
