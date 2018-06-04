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

	byt,err := json.Marshal(genesis)
	if err != nil {
		log.Fatal(err)
	}
	genesisPath := "/opt/Genesis.json"
	file, err := os.Create(genesisPath)
	if err != nil {
		log.Fatal(err)
	}
	file.Write(byt)
	file.Close()
}


