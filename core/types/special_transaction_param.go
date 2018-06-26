package types

import (
	"github.com/GenaroNetwork/Genaro-Core/common"
	"github.com/GenaroNetwork/Genaro-Core/common/hexutil"
	"math/big"
	"math"
)

type SpecialTxInput struct {
	GenaroData
	NodeId string       `json:"address"`
	Type   *hexutil.Big `json:"type"`
	Message string      `json:"msg"`
}

func (s SpecialTxInput) SpecialCost() *big.Int {
	rt := new(big.Int)
	switch s.Type.ToInt() {
	case common.SpecialTxTypeStakeSync:
		return rt.SetUint64(s.Stake)
	case common.SpecialTxTypeSpaceApply:
		var totalCost int64
		for _, v := range s.Buckets {
			duration := math.Abs(float64(v.TimeStart) - float64(v.TimeEnd))

			oneCost := int64(v.Size) * int64(math.Ceil(duration/10)) * common.BucketApplyGasPerGPerDay

			totalCost += oneCost
		}

		totalGas := big.NewInt(totalCost)
		return totalGas
	case common.SpecialTxTypeTrafficApply:
		totalGas := big.NewInt(int64(s.Traffic) * common.TrafficApplyGasPerG)
		return totalGas
	case common.SpecialTxTypeMortgageInit:
		sumMortgageTable := new(big.Int)
		mortgageTable := s.SpecialTxTypeMortgageInit.MortgageTable
		for _, v := range mortgageTable {
			sumMortgageTable = sumMortgageTable.Add(sumMortgageTable, v.ToInt())
		}
		temp := s.SpecialTxTypeMortgageInit.TimeLimit.ToInt().Mul(s.SpecialTxTypeMortgageInit.TimeLimit.ToInt(), big.NewInt(int64(len(mortgageTable))))
		timeLimitGas := temp.Mul(temp, big.NewInt(common.OneDayGes))
		sumMortgageTable.Add(sumMortgageTable, timeLimitGas)
		return sumMortgageTable
	default:
		return big.NewInt(0)
	}
}

// Genaro is the Ethereum consensus representation of Genaro's data.
// these objects are stored in the main genaro trie.
type GenaroData struct {
	Heft                         uint64                               `json:"heft"`
	Stake                        uint64                               `json:"stake"`
	FileSharePublicKey           string                               `json:"publicKey"`
	Node                         []string                             `json:"syncNode"`
	SpecialTxTypeMortgageInit    SpecialTxTypeMortgageInit            `json:"specialTxTypeMortgageInit"`
	SpecialTxTypeMortgageInitArr map[string]SpecialTxTypeMortgageInit `json:"specialTxTypeMortgageInitArr"`
	Traffic                      uint64                               `json:"traffic"`
	Buckets                      []*BucketPropertie                   `json:"buckets"`
	SynchronizeShareKeyArr 		 map[string] SynchronizeShareKey	  `json:"synchronizeShareKeyArr"`
	SynchronizeShareKey			 SynchronizeShareKey				   `json:"synchronizeShareKey"`
}

type SynchronizeShareKey struct {
	ShareKey 	string			`json:"shareKey"`
	Shareprice	*hexutil.Big	`json:"shareprice"`
	Status		int				`json:"status"`
	ShareKeyId	string			`json:"shareKeyId"`
	RecipientAddress   common.Address   `json:"recipientAddress"`
	FromAccount   common.Address   `json:"fromAccount"`
}

type BucketPropertie struct {
	BucketId string `json:"bucketId"`

	// 开始时间和结束时间共同表示存储空间的时长，对应STORAGEGAS指令
	TimeStart uint64 `json:"timeStart"`
	TimeEnd   uint64 `json:"timeEnd"`

	// 备份数，对应STORAGEGASPRICE指令
	Backup uint64 `json:"backup"`

	// 存储空间大小，对应SSIZE指令
	Size uint64 `json:"size"`
}

type Sidechain map[common.Address]*hexutil.Big

type FileIDArr struct {
	MortgageTable   map[common.Address]*hexutil.Big            `json:"mortgage"`
	AuthorityTable  map[common.Address]int                     `json:"authority"`
	FileID          string                                     `json:"fileID"`
	Dataversion     string                                     `json:"dataversion"`
	SidechainStatus map[string]map[common.Address]*hexutil.Big `json:"sidechainStatus"`
	MortgagTotal    *big.Int                                   `json:"MortgagTotal"`
	LogSwitch       bool                                       `json:"logSwitch"`
	TimeLimit       *hexutil.Big                               `json:"timeLimit"`
	CreateTime      int64                                      `json:"createTime"`
	EndTime         int64                                      `json:"endTime"`
	FromAccount     common.Address                             `json:"fromAccount"`
	Terminate       bool                                       `json:"terminate"`
	Sidechain       Sidechain                                  `json:"sidechain"`
}

//Cross-chain storage processing
type SpecialTxTypeMortgageInit FileIDArr
