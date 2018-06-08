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
)

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
	genesis.Alloc = make(core.GenesisAlloc, 1)
	ertra := new(genaro.ExtraData)
	extraByte, _ := json.Marshal(ertra)
	genesis.ExtraData = extraByte

	// To write init Committee
	committees := make([]common.Address, 0)
	committees = append(committees, common.HexToAddress("0xb77a75699c3707fBede9442db8451F44321ef83f"))
	committees = append(committees, common.HexToAddress("0x81Cee7d346595e0552c6df38DD3F61F6e5802d10"))
	committees = append(committees, common.HexToAddress("0x1A7194Eb140e29e09FCe688d2E86f282D6a83E69"))
	committees = append(committees, common.HexToAddress("0x77F7C5FDE3Ce4Fa137c48B3f722B17D7722c3924"))
	committees = append(committees, common.HexToAddress("0xd31Eb2eA2D7bC15c5B0FE7922fAB4Db0A4F8187A"))

	specialAccount := core.GenesisAccount{
		Balance: big.NewInt(0),
		CommitteeRank: make([]common.Address, len(committees)),
	}
	copy(specialAccount.CommitteeRank, committees)
	genesis.Alloc[core.GenesisSpecialAddr] = specialAccount

	// For ICO
	icos := make([]common.Address, 0)
	icos = append(icos, common.HexToAddress("0xA386b786132Efe05B56E3717f556dDe0aD79C6Ed")) // 100
	icos = append(icos, common.HexToAddress("0x362AB90e24c98E3eC83AfDA2cC702a5E24e8025b")) // 200
	icos = append(icos, common.HexToAddress("0xF3719a8EE4a18623D4dB1d879Ba605daaF509B79")) // 300
	icos = append(icos, common.HexToAddress("0xf6Fe19Fc7310E1cBcF8Be9901fF870ddc9bB9bf8")) // 400
	icos = append(icos, common.HexToAddress("0xd350b3f1F0B74813577BA7e68c6e5f08c917603d")) // 500

	account0 := core.GenesisAccount{
		Balance: big.NewInt(100),
	}
	account1 := core.GenesisAccount{
		Balance: big.NewInt(200),
	}
	account2 := core.GenesisAccount{
		Balance: big.NewInt(300),
	}
	account3 := core.GenesisAccount{
		Balance: big.NewInt(400),
	}
	account4 := core.GenesisAccount{
		Balance: big.NewInt(500),
	}
	genesis.Alloc[icos[0]] = account0
	genesis.Alloc[icos[1]] = account1
	genesis.Alloc[icos[2]] = account2
	genesis.Alloc[icos[3]] = account3
	genesis.Alloc[icos[4]] = account4

	// For stake
	stakes := make([]common.Address, 0)
	stakes = append(stakes, common.HexToAddress("0xf8c93eBA49Eae0D74CDA7AA794a95998A927785e")) // 100
	stakes = append(stakes, common.HexToAddress("0x77a965FfCC8B456BF843A3bBF54971ba4B973C2C")) // 200
	stakes = append(stakes, common.HexToAddress("0xaD8D137FbE0dDDE9E395859c45bB76508b878C2F")) // 300
	stakes = append(stakes, common.HexToAddress("0xc132Ab73Fc66F8C890C30E13678d8Aa0145b355B")) // 400
	stakes = append(stakes, common.HexToAddress("0x2f6D35a6b982403dbaC687993B7A2b9d938FCdda")) // 500

	account00 := core.GenesisAccount{
		Balance: big.NewInt(0),
		Stake: 100,
	}
	account01 := core.GenesisAccount{
		Balance: big.NewInt(0),
		Stake: 200,
	}
	account02 := core.GenesisAccount{
		Balance: big.NewInt(0),
		Stake: 300,
	}
	account03 := core.GenesisAccount{
		Balance: big.NewInt(0),
		Stake: 400,
	}
	account04 := core.GenesisAccount{
		Balance: big.NewInt(0),
		Stake: 500,
	}
	genesis.Alloc[stakes[0]] = account00
	genesis.Alloc[stakes[1]] = account01
	genesis.Alloc[stakes[2]] = account02
	genesis.Alloc[stakes[3]] = account03
	genesis.Alloc[stakes[4]] = account04

	// For Heft
	hefts := make([]common.Address, 0)
	hefts = append(hefts, common.HexToAddress("0xfd8a875eb1e3aF2E5578e638E75c36B3749c5DC2")) // 100
	hefts = append(hefts, common.HexToAddress("0x1D0F6DD944C6a3749a38871243d2F76Dbe71886F")) // 200
	hefts = append(hefts, common.HexToAddress("0x2aD36f60E663C4a32D45fbBc58319679BAbF3cC0")) // 300
	hefts = append(hefts, common.HexToAddress("0x2C162771fd8174621b2B02FCA92304b6c5BF068E")) // 400
	hefts = append(hefts, common.HexToAddress("0x6248B095bf7ae01c5e16A32f1E430f2858c07928")) // 500

	account000 := core.GenesisAccount{
		Balance: big.NewInt(0),
		Heft: 100,
	}
	account001 := core.GenesisAccount{
		Balance: big.NewInt(0),
		Heft: 200,
	}
	account002 := core.GenesisAccount{
		Balance: big.NewInt(0),
		Heft: 300,
	}
	account003 := core.GenesisAccount{
		Balance: big.NewInt(0),
		Heft: 400,
	}
	account004 := core.GenesisAccount{
		Balance: big.NewInt(0),
		Heft: 500,
	}
	genesis.Alloc[hefts[0]] = account000
	genesis.Alloc[hefts[1]] = account001
	genesis.Alloc[hefts[2]] = account002
	genesis.Alloc[hefts[3]] = account003
	genesis.Alloc[hefts[4]] = account004

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
