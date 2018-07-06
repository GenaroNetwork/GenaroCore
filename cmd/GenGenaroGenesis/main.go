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
	"flag"
)

var accountfile string


func initarg() {
	flag.StringVar(&accountfile, "f", "account.json", "account file")
	flag.Parse()
}

// generate first committees list special account
func GenCandidateAccount(committees []common.Address) core.GenesisAccount{
	committeesData, _ := json.Marshal(committees)
	CandidateAccount := core.GenesisAccount{
		Balance: big.NewInt(0),
		CodeHash: committeesData,
	}
	return CandidateAccount
}

// generate first SynState Account
func GenLastSynStateAccount() core.GenesisAccount{
	var lastRootStates = make(map[common.Hash]uint64)
	lastRootStates[common.Hash{}] = 0
	var lastSynState = types.LastSynState{
		LastRootStates:	lastRootStates,
		LastSynBlockNum: 0,
	}
	b, _ := json.Marshal(lastSynState)
	LastSynStateAccount := core.GenesisAccount{
		Balance: big.NewInt(0),
		CodeHash: b,
	}
	return LastSynStateAccount
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

type account struct {
	Balance 	string		`json:"balance"`
	Heft 		uint64		`json:"heft"`
	Stake 		uint64		`json:"stake"`
}
type MyAlloc map[common.Address]account

//type firstAccounts struct {
//	Alloc      FirstAlloc        `json:"alloc"      gencodec:"required"`
//}

type header struct {
	Encryption  string `json:"encryption"`
	Timestamp   int64  `json:"timestamp"`
	Key         string `json:"key"`
	Partnercode int    `json:"partnercode"`
}


func main() {
	initarg()
	accountfile, err := os.Open(accountfile)
	myAccounts := new(MyAlloc)
	if err := json.NewDecoder(accountfile).Decode(myAccounts); err != nil {
		log.Fatalf("invalid account file: %v", err)
	}

	genaroConfig := &params.ChainConfig{
		ChainId:        big.NewInt(300),
		HomesteadBlock: big.NewInt(1),
		EIP150Block:    big.NewInt(2),
		EIP150Hash:     common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
		EIP155Block:    big.NewInt(3),
		EIP158Block:    big.NewInt(3),
		ByzantiumBlock: big.NewInt(4),
		Genaro: &params.GenaroConfig{
			Epoch:            86400, //the number of blocks in one committee term
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
	for addr := range *myAccounts {
		if (*myAccounts)[addr].Stake > 0 {
			committees = append(committees, addr)
		}
	}
	candidateAccount := GenCandidateAccount(committees)
	LastSynStateAccount := GenLastSynStateAccount()
	genesis.Alloc[common.CandidateSaveAddress] = candidateAccount
	genesis.Alloc[common.LastSynStateSaveAddress] = LastSynStateAccount

	//accounts := make([]core.GenesisAccount,len(*myAccounts))
	for addr := range *myAccounts {
		account := GenAccount((*myAccounts)[addr].Balance, (*myAccounts)[addr].Stake,(*myAccounts)[addr].Heft)
		genesis.Alloc[addr] = account
	}

	extra := new(genaro.ExtraData)
	var candidateInfos state.CandidateInfos
	candidateInfos = GenesisAllocToCandidateInfos(genesis.Alloc)
	extra.CommitteeRank,extra.Proportion = state.Rank(candidateInfos)
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
