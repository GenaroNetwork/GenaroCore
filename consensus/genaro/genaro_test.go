package genaro

import (
	"testing"
	"github.com/GenaroNetwork/Genaro-Core/params"
	"fmt"
	"github.com/GenaroNetwork/Genaro-Core/core/types"
	"github.com/GenaroNetwork/Genaro-Core/crypto"
	"github.com/GenaroNetwork/Genaro-Core/common/hexutil"
	"log"
	//"bytes"
	//"github.com/GenaroNetwork/Genaro-Core/core/vm"
	//"github.com/GenaroNetwork/Genaro-Core/core"
	"github.com/GenaroNetwork/Genaro-Core/common"
	//"math/big"
	//"time"
	"github.com/GenaroNetwork/Genaro-Core/ethdb"
	"github.com/GenaroNetwork/Genaro-Core/core/state"
	"bytes"
	"math/big"

	//"github.com/GenaroNetwork/Genaro-Core/core"
)

func TestGetDependTurnByBlockNumber(t *testing.T){
	var turn uint64 = 0

	for i:=0;i<20;i++ {
		turn = GetDependTurnByBlockNumber(params.MainnetChainConfig.Genaro,uint64(i))
		fmt.Println(turn)
	}

}

func TestAuthor(t *testing.T){
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
		Extra:byt,
	}

	prikey,err := crypto.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}
	addr := crypto.PubkeyToAddress(prikey.PublicKey)
	fmt.Println("addr")
	fmt.Println(hexutil.Encode(addr.Bytes()))

	hash := sigHash(&head)
	sig,err := crypto.Sign(hash.Bytes(), prikey)
	if err != nil {
		log.Fatal(err)
	}

	genaro.signer = addr
	SetHeaderSignature(&head, sig)

	signer,err := genaro.Author(&head)
	if err != nil {
		t.Error(err)
	}
	fmt.Println("singer")
	fmt.Println(hexutil.Encode(signer.Bytes()))

	if bytes.Compare(addr.Bytes(),signer.Bytes()) != 0 {
		t.Error("sign error")
	}

}

func TestNew(t *testing.T) {
	db, remove := newTestLDB()
	defer remove()
	genaroConfig := &params.GenaroConfig{
		BlockInterval:		10,
		ElectionPeriod:		1,
		ValidPeriod:		1,
		CurrencyRates:		10,
		CommitteeMaxSize:	5,
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

//func getGenesis() *core.Genesis{
//	genaroConfig := &params.ChainConfig{
//		ChainId:             big.NewInt(300),
//		HomesteadBlock:      big.NewInt(1),
//		EIP150Block:         big.NewInt(2),
//		EIP150Hash:          common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
//		EIP155Block:         big.NewInt(3),
//		EIP158Block:         big.NewInt(3),
//		ByzantiumBlock:      big.NewInt(4),
//		Genaro:              &params.GenaroConfig{
//			Epoch:            2000, //the number of blocks in one committee term
//			BlockInterval:    10,   //a peer create BlockInterval blocks one time
//			ElectionPeriod:   1,    //a committee list write time
//			ValidPeriod:      1,    //a written committee list waiting time to come into force
//			CurrencyRates:    5,    //interest rates of coin
//			CommitteeMaxSize: 101,  //max number of committee member
//		},
//	}
//	genesis := new(core.Genesis)
//	genesis.Config = genaroConfig
//	genesis.Difficulty = big.NewInt(1)
//	genesis.GasLimit = 5000000
//	genesis.GasUsed = 0
//	genesis.Mixhash = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
//	genesis.ParentHash = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
//	genesis.Timestamp = uint64(time.Now().Unix())
//	genesis.Nonce = 0
//	genesis.Coinbase = common.HexToAddress("0x0000000000000000000000000000000000000000")
//
//	n := 10
//	addrs := genAddrs(n)
//	byt := CreateCommitteeRankByte(addrs)
//	genesis.ExtraData = byt
//
//	genesis.Alloc = make(core.GenesisAlloc,1)
//	account0 := core.GenesisAccount {
//		Balance: big.NewInt(1),
//	}
//	account1 := core.GenesisAccount {
//		Balance: big.NewInt(2),
//	}
//
//	genesis.Alloc[addrs[0]] = account0
//	genesis.Alloc[addrs[1]] = account1
//	return genesis
//}

//func TestGenaroPrepare(t *testing.T){
//	db, remove := newTestLDB()
//	defer remove()
//
//	genesis := getGenesis()
//
//	chainConfig,hash,err := core.SetupGenesisBlock(db, genesis)
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Println(chainConfig)
//	fmt.Println(hash)
//
//	genaro := New(genesis.Config.Genaro,db)
//	chain,err := core.NewBlockChain(db, nil, params.MainnetChainConfig, genaro, vm.Config{})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	committeeRank := genAddrs(10)
//	proportion := make([]uint64, 10)
//	for i := range committeeRank{
//		proportion[i] = uint64(i)
//	}
//
//	snapshot := newSnapshot(genesis.Config.Genaro,0,hash,0,committeeRank, proportion)
//	displaySnapshot(*snapshot)
//
//	err = genaro.Prepare(chain, chain.GetHeaderByNumber(0))
//	if err != nil {
//		log.Fatal(err)
//	}
//}

//func TestCalcDifficulty(t *testing.T) {
//	db, remove := newTestLDB()
//	defer remove()
//
//	genesis := getGenesis()
//
//	genaro := New(genesis.Config.Genaro,db)
//	turn := GetTurnOfCommiteeByBlockNumber(genaro.config, 120)
//	fmt.Println(turn)
//	if turn != 0 {
//		t.Error("GetTurnOfCommiteeByBlockNumber error")
//	}
//	turn = GetDependTurnByBlockNumber(genaro.config, 120)
//	fmt.Println(turn)
//	if turn != 0 {
//		t.Error("GetDependTurnByBlockNumber error")
//	}
//	turn = GetTurnOfCommiteeByBlockNumber(genaro.config, 15678)
//	fmt.Println(turn)
//	if turn != 7 {
//		t.Error("GetTurnOfCommiteeByBlockNumber error")
//	}
//	turn = GetDependTurnByBlockNumber(genaro.config, 15678)
//	fmt.Println(turn)
//	if turn != 5 {
//		t.Error("GetDependTurnByBlockNumber error")
//	}
//}

func newTestStateDB() *state.StateDB {
	diskdb, _ := ethdb.NewMemDatabase()
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(diskdb))

	return statedb
}

func TestUpdateSpecialBlock(t *testing.T) {
	genaroConfig := &params.GenaroConfig{
		Epoch:				5000,
		BlockInterval:		10,
		ElectionPeriod:		1,
		ValidPeriod:		1,
		CurrencyRates:		10,
		CommitteeMaxSize:	5,
	}
	header := &types.Header{
		Number:   big.NewInt(0),
		Time:     big.NewInt(0),
		Coinbase: common.HexToAddress("0x50a7658e5155206dc78eafb80e6a94640b274648"),
		Extra:    make([]byte, 0),
	}

	updateSpecialBlock(genaroConfig, header, newTestStateDB())
}

func TestCalcDifficulty2(t *testing.T) {
	genaroConfig := &params.GenaroConfig{
		Epoch:				5000,
		BlockInterval:		10,
		ElectionPeriod:		1,
		ValidPeriod:		1,
		CurrencyRates:		10,
		CommitteeMaxSize:	5,
	}
	committeeRank := genAddrs(10)
	proportion := make([]uint64, 10)
	for i := range committeeRank{
		proportion[i] = uint64(i)
	}

	snapshot := newSnapshot(genaroConfig,0, common.StringToHash("0"), 0, committeeRank, proportion)

	var i uint64
	for i = 0; i < 10000; i++ {
		x := CalcDifficulty(snapshot, committeeRank[0], i)
		println(x.Uint64())
	}

	xx := CalcDifficulty(snapshot, genAddrs(1)[0], 100)
	println(xx.Uint64())
}

func TestCandidateInfos(t *testing.T) {
	var candidateInfos state.CandidateInfos
	candidateInfos = make([]state.CandidateInfo, 4)
	candidateInfos[0] = state.CandidateInfo{
		Signer:		common.StringToAddress("xx"),
		Heft:		10,
		Stake:		25,
	}
	candidateInfos[1] = state.CandidateInfo{
		Signer:		common.StringToAddress("xx"),
		Heft:		11,
		Stake:		35,
	}
	candidateInfos[2] = state.CandidateInfo{
		Signer:		common.StringToAddress("xx"),
		Heft:		12,
		Stake:		45,
	}
	candidateInfos[3] = state.CandidateInfo{
		Signer:		common.StringToAddress("xx"),
		Heft:		13,
		Stake:		15,
	}

	candidateInfos.Apply()
	commiteeRank, proportion := state.Rank(candidateInfos)
	fmt.Println("Rank")
	fmt.Println(commiteeRank)
	fmt.Println(proportion)
	fmt.Println(candidateInfos)
	commiteeRank, proportion = state.RankWithLenth(candidateInfos,3)
	fmt.Println("RankWithLenth")
	fmt.Println(commiteeRank)
	fmt.Println(proportion)
}

func TestAccumulateInterestRewards(t *testing.T) {
	genaroConfig := &params.GenaroConfig{
		Epoch:				5000,
		BlockInterval:		10,
		ElectionPeriod:		1,
		ValidPeriod:		1,
		CurrencyRates:		10,
		CommitteeMaxSize:	5,
	}

	db, remove := newTestLDB()
	defer remove()
	state, err := state.New(common.Hash{}, state.NewDatabase(db))
	if err != nil {
		t.Errorf("error 310")
	}

	header := &types.Header{
		Number:   big.NewInt(0),
		Time:     big.NewInt(0),
		Coinbase: common.HexToAddress("0x50a7658e5155206dc78eafb80e6a94640b274648"),
		Extra:    make([]byte, 0),
	}

	blockNumber := uint64(1)

	tmp := big.NewInt(175000000)
	tmp.Mul(tmp, big.NewInt(100000000))
	tmp.Mul(tmp, big.NewInt(10000000000))
	fmt.Println(tmp.String())
	state.SetBalance(common.BytesToAddress([]byte(SurplusCoinAddress)), tmp)

	addr := genAddrs(10)
	proportions := make([]uint64, 10)
	proportions[0] = 5000
	proportions[1] = 3000
	proportions[2] = 1300
	for i := 3; i < 10; i++ {
		proportions[i] = 100
	}

	for i := 0; i < 5; i++ {
		//每年
		for j := 0; j < int(calEpochPerYear(genaroConfig)); j++ {
			//每个epoch
			for k := 0; k < int(genaroConfig.Epoch/genaroConfig.BlockInterval); k++ {
				//每个出块节点
				header.Coinbase = addr[k%10]
				proportion := proportions[k%10]
				for l := 0; l < int(genaroConfig.BlockInterval); l++ {
					accumulateInterestRewards(genaroConfig, state, header, proportion, blockNumber,3)
					//fmt.Println(state.GetBalance(header.Coinbase).Uint64())
				}
			}
		}
	}

}
func TestGetCoinCofficient(t *testing.T) {
	genaroConfig := &params.GenaroConfig{
		Epoch:				86400,
		Period:				1,
		BlockInterval:		10,
		ElectionPeriod:		1,
		ValidPeriod:		1,
		CurrencyRates:		10,
		CommitteeMaxSize:	5,
	}
	cofficient := getCoinCofficient(genaroConfig, big.NewInt(500),big.NewInt(20857142))
	fmt.Println(cofficient)
}
