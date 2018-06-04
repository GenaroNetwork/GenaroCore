package main

import (
	"github.com/GenaroNetwork/Genaro-Core/core"
	"github.com/gin-gonic/gin/json"
	"log"
	"os"
	"math/big"
	"github.com/GenaroNetwork/Genaro-Core/params"
	"github.com/GenaroNetwork/Genaro-Core/common"
	"time"
	"github.com/GenaroNetwork/Genaro-Core/consensus/genaro"
	"fmt"
	"github.com/GenaroNetwork/Genaro-Core/crypto"
	"io/ioutil"
)

func main(){
	enaroConfig := &params.ChainConfig{
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
	genesis.Config = enaroConfig
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
	byt := genaro.CreateCommitteeRankByte(addrs)
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

	byt,err := json.Marshal(genesis)
	if err != nil {
		log.Fatal(err)
	}
	dirname, err := ioutil.TempDir(os.TempDir(), "genaro_test")
	genesisPath := dirname + "Genesis.json"
	fmt.Println(genesisPath)
	file, err := os.Create(genesisPath)
	if err != nil {
		log.Fatal(err)
	}
	file.Write(byt)
	file.Close()
}

func genAddrs(n int)[]common.Address{
	addrs := make([]common.Address, 0)

	for i := 0; i < n; i++ {
		prikey, _ := crypto.GenerateKey()
		addr := crypto.PubkeyToAddress(prikey.PublicKey)

		fmt.Println(addr.String())
		addrs = append(addrs, addr)
	}
	return addrs
}
