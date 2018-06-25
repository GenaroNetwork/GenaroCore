package main

import (
	"github.com/GenaroNetwork/Genaro-Core/core"
	"log"
	"os"
	"math/big"
	"github.com/GenaroNetwork/Genaro-Core/params"
	"github.com/GenaroNetwork/Genaro-Core/common"
	"time"
	"fmt"
	"github.com/GenaroNetwork/Genaro-Core/crypto"
	"io/ioutil"
	"encoding/json"
	"github.com/GenaroNetwork/Genaro-Core/consensus/genaro"
	"github.com/GenaroNetwork/Genaro-Core/core/state"
	"github.com/GenaroNetwork/Genaro-Core/common/math"
	"github.com/pkg/errors"
	"github.com/GenaroNetwork/Genaro-Core/core/types"
)

// generate first committees list special account
func GenCandidateAccount(committees []common.Address) core.GenesisAccount{
	committeesData, _ := json.Marshal(committees)
	CandidateAccount := core.GenesisAccount{
		Balance: big.NewInt(0),
		CodeHash: committeesData,
	}
	return CandidateAccount
}

// generate user account
func GenAccount(balanceStr string, stake,heft uint64) core.GenesisAccount {
	balance,ok := math.ParseBig256(balanceStr)
	if !ok {
		log.Fatal(errors.New("GenAccount ParseBig256 error"))
	}

	stakeLog := types.NumLog{
		BlockNum: 0,
		Num: stake,
	}
	stakeLogs := types.NumLogs{stakeLog}

	heftLog := types.NumLog{
		BlockNum: 0,
		Num: heft,
	}
	heftLogs := types.NumLogs{heftLog}

	genaroData := types.GenaroData{
		Stake: stake,
		Heft: heft,
		StakeLog:stakeLogs,
		HeftLog:heftLogs,
	}
	genaroDataByte, _ := json.Marshal(genaroData)
	account := core.GenesisAccount{
		Balance: balance,
		CodeHash: genaroDataByte,
	}
	return account
}

func GenesisAllocToCandidateInfos(genesisAlloc core.GenesisAlloc) state.CandidateInfos{
	candidateInfos := make(state.CandidateInfos,0)
	for addr,account := range genesisAlloc {
		var genaroData types.GenaroData
		json.Unmarshal(account.CodeHash,&genaroData)
		if genaroData.Stake > 0 {
			var candidateInfo state.CandidateInfo
			candidateInfo.Stake = genaroData.Stake
			candidateInfo.Heft = genaroData.Heft
			candidateInfo.Signer = addr
			candidateInfos = append(candidateInfos,candidateInfo)
		}
	}
	return candidateInfos
}

func main() {
	genaroConfig := &params.ChainConfig{
		ChainId:        big.NewInt(300),
		HomesteadBlock: big.NewInt(1),
		EIP150Block:    big.NewInt(2),
		EIP150Hash:     common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
		EIP155Block:    big.NewInt(3),
		EIP158Block:    big.NewInt(3),
		ByzantiumBlock: big.NewInt(4),
		Genaro: &params.GenaroConfig{
			Epoch:            200, //the number of blocks in one committee term
			Period:			  1,	// Number of seconds between blocks to enforce
			BlockInterval:    10,    //a peer create BlockInterval blocks one time
			ElectionPeriod:   1,    //a committee list write time
			ValidPeriod:      1,    //a written committee list waiting time to come into force
			CurrencyRates:    5,    //interest rates of coin
			CommitteeMaxSize: 101,  //max number of committee member
		},
	}
	genesis := new(core.Genesis)
	genesis.Config = genaroConfig
	genesis.Difficulty = big.NewInt(1)
	genesis.GasLimit = 50000000
	genesis.GasUsed = 0
	genesis.Mixhash = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
	genesis.ParentHash = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
	genesis.Timestamp = uint64(time.Now().Unix())
	genesis.Nonce = 0
	genesis.Coinbase = common.HexToAddress("0x0000000000000000000000000000000000000000")
	genesis.Alloc = make(core.GenesisAlloc, 1)

	// To write init Committee
	committees := make([]common.Address, 0)
	committees = append(committees, common.HexToAddress("0xad188b762f9e3ef76c972960b80c9dc99b9cfc73"))
	//committees = append(committees, common.HexToAddress("0x42c68ba130dca8e514126906add36e7c4f9204f5"))
	committees = append(committees, common.HexToAddress("0x81Cee7d346595e0552c6df38DD3F61F6e5802d10"))
	committees = append(committees, common.HexToAddress("0x1A7194Eb140e29e09FCe688d2E86f282D6a83E69"))
	committees = append(committees, common.HexToAddress("0x77F7C5FDE3Ce4Fa137c48B3f722B17D7722c3924"))
	committees = append(committees, common.HexToAddress("0x0de2d12fa9c0a5687b330e2de3361e632f52c643"))
	candidateAccount := GenCandidateAccount(committees)
	genesis.Alloc[common.CandidateSaveAddress] = candidateAccount

	accounts := make([]core.GenesisAccount,5)

	accounts[0] = GenAccount("200000000000000000000", 200,200)
	genesis.Alloc[committees[0]] = accounts[0]
	accounts[1] = GenAccount("300000000000000000000", 300,400)
	genesis.Alloc[committees[1]] = accounts[1]
	accounts[2] = GenAccount("400000000000000000000", 400,500)
	genesis.Alloc[committees[2]] = accounts[2]
	accounts[3] = GenAccount("500000000000000000000", 500,600)
	genesis.Alloc[committees[3]] = accounts[3]
	accounts[4] = GenAccount("600000000000000000000", 600,300)
	genesis.Alloc[committees[4]] = accounts[4]

	extra := new(genaro.ExtraData)
	var candidateInfos state.CandidateInfos
	candidateInfos = GenesisAllocToCandidateInfos(genesis.Alloc)
	extra.CommitteeRank,extra.Proportion = genaro.Rank(candidateInfos)
	extraByte, _ := json.Marshal(extra)
	genesis.ExtraData = extraByte

    // create json file
	byt, err := json.Marshal(genesis)
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

func genAddrs(n int) []common.Address {
	addrs := make([]common.Address, 0)

	for i := 0; i < n; i++ {
		prikey, _ := crypto.GenerateKey()
		addr := crypto.PubkeyToAddress(prikey.PublicKey)

		fmt.Println(addr.String())
		addrs = append(addrs, addr)
	}
	return addrs
}
