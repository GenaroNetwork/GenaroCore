package genaro

import (
	"testing"
	"github.com/GenaroNetwork/Genaro-Core/params"
	"fmt"
	"github.com/GenaroNetwork/Genaro-Core/core/types"
	"github.com/GenaroNetwork/Genaro-Core/crypto"
	"github.com/GenaroNetwork/Genaro-Core/common/hexutil"
	"log"
	"bytes"
	"github.com/GenaroNetwork/Genaro-Core/core/vm"
	"github.com/GenaroNetwork/Genaro-Core/core"
	"github.com/GenaroNetwork/Genaro-Core/common"
	"math/big"
	"time"
)

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

func getGenesis() *core.Genesis{
	genaroConfig := &params.ChainConfig{
		ChainId:             big.NewInt(300),
		HomesteadBlock:      big.NewInt(1),
		EIP150Block:         big.NewInt(2),
		EIP150Hash:          common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
		EIP155Block:         big.NewInt(3),
		EIP158Block:         big.NewInt(3),
		ByzantiumBlock:      big.NewInt(4),
		Genaro:              &params.GenaroConfig{
			Epoch:            2000, //the number of blocks in one committee term
			BlockInterval:    10,   //a peer create BlockInterval blocks one time
			ElectionPeriod:   1,    //a committee list write time
			ValidPeriod:      1,    //a written committee list waiting time to come into force
			CurrencyRates:    5,    //interest rates of coin
			CommitteeMaxSize: 101,  //max number of committee member
		},
	}
	genesis := new(core.Genesis)
	genesis.Config = genaroConfig
	genesis.Difficulty = big.NewInt(1)
	genesis.GasLimit = 5000000
	genesis.GasUsed = 0
	genesis.Mixhash = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
	genesis.ParentHash = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
	genesis.Timestamp = uint64(time.Now().Unix())
	genesis.Nonce = 0
	genesis.Coinbase = common.HexToAddress("0x0000000000000000000000000000000000000000")

	n := 10
	addrs := genAddrs(n)
	byt := CreateCommitteeRankByte(addrs)
	genesis.ExtraData = byt

	genesis.Alloc = make(core.GenesisAlloc,1)
	account0 := core.GenesisAccount {
		Balance: big.NewInt(1),
	}
	account1 := core.GenesisAccount {
		Balance: big.NewInt(2),
	}

	genesis.Alloc[addrs[0]] = account0
	genesis.Alloc[addrs[1]] = account1
	return genesis
}

func TestGenaroPrepare(t *testing.T){
	db, remove := newTestLDB()
	defer remove()

	genesis := getGenesis()

	chainConfig,hash,err := core.SetupGenesisBlock(db, genesis)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(chainConfig)
	fmt.Println(hash)

	genaro := New(genesis.Config.Genaro,db)
	chain,err := core.NewBlockChain(db, nil, params.MainnetChainConfig, genaro, vm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	committeeRank := genAddrs(10)
	proportion := make([]uint64, 10)
	for i := range committeeRank{
		proportion[i] = uint64(i)
	}

	snapshot := newSnapshot(genesis.Config.Genaro,0,hash,0,committeeRank, proportion)
	displaySnapshot(*snapshot)

	err = genaro.Prepare(chain, chain.GetHeaderByNumber(0))
	if err != nil {
		log.Fatal(err)
	}
}
