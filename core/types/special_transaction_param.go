package types

import (
	"bytes"
	"github.com/GenaroNetwork/GenaroCore/common"
	"github.com/GenaroNetwork/GenaroCore/common/hexutil"
	"github.com/GenaroNetwork/GenaroCore/crypto"
	"github.com/GenaroNetwork/GenaroCore/rlp"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type SpecialTxInput struct {
	GenaroData
	Address               string       `json:"address"`
	NodeID                string       `json:"nodeId"`
	BucketID              string       `json:"bucketId"`
	Size                  uint64       `json:"size"`
	Duration              uint64       `json:"duration"`
	Type                  *hexutil.Big `json:"type"`
	BlockNumber           string       `json:"blockNr"`
	Message               string       `json:"msg"`
	Sign                  string       `json:"sign"`
	AddCoin               *hexutil.Big `json:"addCoin"`
	OrderId               common.Hash  `json:"orderId"`
	RestoreBlock          uint64       `json:"RestoreBlock"`
	TxNum                 uint64       `json:"TxNum"`
	PromissoryNoteTxPrice *hexutil.Big `json:"PromissoryNoteTxPrice"`
	OptionPrice           *hexutil.Big `json:"OptionPrice"`
	IsSell                bool         `json:"IsSell"`
	GenaroPrice
}

type GenaroPrice struct {
	BucketApplyGasPerGPerDay *hexutil.Big `json:"bucketPricePerGperDay"`
	TrafficApplyGasPerG      *hexutil.Big `json:"trafficPricePerG"`
	StakeValuePerNode        *hexutil.Big `json:"stakeValuePerNode"`
	OneDayMortgageGes        *hexutil.Big `json:"oneDayMortgageGes"`
	OneDaySyncLogGsaCost     *hexutil.Big `json:"oneDaySyncLogGsaCost"`
	MaxBinding               uint64       `json:"MaxBinding"`
	MinStake                 uint64       `json:"MinStake"`
	CommitteeMinStake        uint64       `json:"CommitteeMinStake"`
	BackStackListMax         uint64       `json:"BackStackListMax"`
	CoinRewardsRatio         uint64       `json:"CoinRewardsRatio"`
	StorageRewardsRatio      uint64       `json:"StorageRewardsRatio"`
	RatioPerYear             uint64       `json:"RatioPerYear"`
	SynStateAccount          string       `json:"SynStateAccount"`
	HeftAccount              string       `json:"HeftAccount"`
	BindingAccount           string       `json:"BindingAccount"`
	ExtraPrice               []byte       `json:"extraPrice"`
}

func (s SpecialTxInput) SpecialCost(currentPrice *GenaroPrice, bucketsMap map[string]interface{}) big.Int {

	switch s.Type.ToInt().Uint64() {
	case common.SpecialTxTypeStakeSync.Uint64():
		ret := new(big.Int).Mul(new(big.Int).SetUint64(s.Stake), common.BaseCompany)
		return *ret
	case common.SpecialTxTypeSpaceApply.Uint64():
		var totalCost *big.Int = big.NewInt(0)
		var bucketPrice *big.Int
		if currentPrice != nil && currentPrice.BucketApplyGasPerGPerDay != nil {
			bucketPrice = new(big.Int).Set(currentPrice.BucketApplyGasPerGPerDay.ToInt())
		} else {
			bucketPrice = new(big.Int).Set(common.DefaultBucketApplyGasPerGPerDay)
		}
		for _, v := range s.Buckets {
			duration := math.Ceil(math.Abs(float64(v.TimeStart)-float64(v.TimeEnd)) / 86400)
			oneCost := new(big.Int).Mul(bucketPrice, big.NewInt(int64(v.Size)*int64(duration)))
			totalCost.Add(totalCost, oneCost)
		}
		return *totalCost
	case common.SpecialTxBucketSupplement.Uint64():
		var totalCost *big.Int = big.NewInt(0)
		var bucketPrice *big.Int
		if currentPrice != nil && currentPrice.BucketApplyGasPerGPerDay != nil {
			bucketPrice = new(big.Int).Set(currentPrice.BucketApplyGasPerGPerDay.ToInt())
		} else {
			bucketPrice = new(big.Int).Set(common.DefaultBucketApplyGasPerGPerDay)
		}

		if v, ok := bucketsMap[s.BucketID]; ok {
			bucketPropertie := v.(BucketPropertie)

			if s.Size != 0 && s.Duration == 0 {

				timeInt, err := strconv.Atoi(s.Message)
				if err != nil {
					timeInt = int(bucketPropertie.TimeStart)
				}
				txTime := time.Unix(int64(timeInt), 0)
				calSize := s.Size
				var subtraction float64
				if uint64(txTime.Unix()) > bucketPropertie.TimeStart {
					subtraction = float64(txTime.Unix())
				} else {
					subtraction = float64(bucketPropertie.TimeStart)
				}
				calDuration := math.Ceil(math.Abs(float64(bucketPropertie.TimeEnd)-subtraction) / 86400)
				totalCost = new(big.Int).Mul(bucketPrice, big.NewInt(int64(calSize)*int64(calDuration)))
			} else if s.Size == 0 && s.Duration != 0 {
				calSize := bucketPropertie.Size
				calDuration := math.Ceil(float64(s.Duration) / 86400)
				totalCost = new(big.Int).Mul(bucketPrice, big.NewInt(int64(calSize)*int64(calDuration)))
			} else if s.Size != 0 && s.Duration != 0 {
				calSize := bucketPropertie.Size + s.Size
				calDuration := math.Ceil(float64(s.Duration) / 86400)
				totalCost1 := new(big.Int).Mul(bucketPrice, big.NewInt(int64(calSize)*int64(calDuration)))

				var subtraction float64

				timeInt, err := strconv.Atoi(s.Message)
				if err != nil {
					timeInt = int(bucketPropertie.TimeStart)
				}
				txTime := time.Unix(int64(timeInt), 0)

				if uint64(txTime.Unix()) > bucketPropertie.TimeStart {
					subtraction = float64(txTime.Unix())

				} else {
					subtraction = float64(bucketPropertie.TimeStart)
				}
				calSize2 := s.Size
				calDuration2 := math.Ceil(math.Abs(float64(bucketPropertie.TimeEnd)-subtraction) / 86400)
				totalCost2 := new(big.Int).Mul(bucketPrice, big.NewInt(int64(calSize2)*int64(calDuration2)))

				totalCost = new(big.Int).Add(totalCost1, totalCost2)
			}

		}

		return *totalCost
	case common.SpecialTxTypeTrafficApply.Uint64():

		var trafficPrice *big.Int
		if currentPrice != nil && currentPrice.BucketApplyGasPerGPerDay != nil {
			trafficPrice = new(big.Int).Set(currentPrice.TrafficApplyGasPerG.ToInt())
		} else {
			trafficPrice = new(big.Int).Set(common.DefaultTrafficApplyGasPerG)
		}

		totalGas := new(big.Int).Mul(trafficPrice, big.NewInt(int64(s.Traffic)))
		return *totalGas
	case common.SpecialTxTypeMortgageInit.Uint64():
		sumMortgageTable := new(big.Int)
		mortgageTable := s.SpecialTxTypeMortgageInit.MortgageTable
		for _, v := range mortgageTable {
			sumMortgageTable = sumMortgageTable.Add(sumMortgageTable, v.ToInt())
		}
		temp := s.SpecialTxTypeMortgageInit.TimeLimit.ToInt().Mul(s.SpecialTxTypeMortgageInit.TimeLimit.ToInt(), big.NewInt(int64(len(mortgageTable))))
		timeLimitGas := temp.Mul(temp, common.DefaultOneDayMortgageGes)
		sumMortgageTable.Add(sumMortgageTable, timeLimitGas)
		return *sumMortgageTable
	default:
		return *big.NewInt(0)
	}
}

type PromissoryNote struct {
	RestoreBlock uint64 `json:"restoreBlock"`
	Num          uint64 `json:"Num"`
}

type PromissoryNotes []PromissoryNote

func (notes *PromissoryNotes) Add(newNote PromissoryNote) {
	isExist := false
	for i, note := range *notes {
		if note.RestoreBlock == newNote.RestoreBlock {
			(*notes)[i].Num += newNote.Num
			isExist = true
			break
		}
	}

	if !isExist {
		*notes = append(*notes, newNote)
	}
}

func (notes *PromissoryNotes) Del(newNote PromissoryNote) bool {
	isSuccess := false
	for i, note := range *notes {
		if note.RestoreBlock == newNote.RestoreBlock {
			if (*notes)[i].Num >= newNote.Num {
				(*notes)[i].Num -= newNote.Num
				isSuccess = true
				if (*notes)[i].Num == 0 {
					(*notes) = append((*notes)[:i], (*notes)[i+1:]...)
				}
			}
			break
		}
	}
	return isSuccess
}

func (notes *PromissoryNotes) DelBefor(blockNum uint64) uint64 {
	delNum := uint64(0)
	for i := 0; i < len(*notes); i++ {
		if (*notes)[i].RestoreBlock <= blockNum {
			delNum += (*notes)[i].Num
			(*notes) = append((*notes)[:i], (*notes)[i+1:]...)
			i--
		}
	}
	return delNum
}

func (notes *PromissoryNotes) GetBefor(blockNum uint64) uint64 {
	num := uint64(0)
	for i := 0; i < len(*notes); i++ {
		if (*notes)[i].RestoreBlock <= blockNum {
			num += (*notes)[i].Num
		}
	}
	return num
}

func (notes *PromissoryNotes) GetNum(restoreBlock uint64) uint64 {
	for _, note := range *notes {
		if note.RestoreBlock == restoreBlock {
			return note.Num
		}
	}
	return 0
}

func (notes *PromissoryNotes) GetAllNum() uint64 {
	allNum := uint64(0)
	for _, note := range *notes {
		allNum += note.Num
	}
	return allNum
}

type PromissoryNotesOptionTx struct {
	IsSell                bool           `json:"IsSell"`
	OptionPrice           *big.Int       `json:"OptionPrice"`
	RestoreBlock          uint64         `json:"RestoreBlock"`
	TxNum                 uint64         `json:"TxNum"`
	PromissoryNoteTxPrice *big.Int       `json:"PromissoryNoteTxPrice"`
	PromissoryNotesOwner  common.Address `json:"PromissoryNotesOwner"`
	OptionOwner           common.Address `json:"OptionOwner"`
}

type OptionTxTable map[common.Hash]PromissoryNotesOptionTx

func GenOptionTxHash(addr common.Address, nonce uint64) common.Hash {
	data, _ := rlp.EncodeToBytes([]interface{}{addr, nonce})
	crypto.Keccak256()
	var hash common.Hash
	hash.SetBytes(crypto.Keccak256(data))
	return hash
}

// Genaro is the Ethereum consensus representation of Genaro's data.
// these objects are stored in the main genaro trie.
type GenaroData struct {
	Heft                         uint64                               `json:"heft"`
	Stake                        uint64                               `json:"stake"`
	HeftLog                      NumLogs                              `json:"heftlog"`
	StakeLog                     NumLogs                              `json:"stakelog"`
	FileSharePublicKey           string                               `json:"publicKey"`
	Node                         []string                             `json:"syncNode"`
	SpecialTxTypeMortgageInit    SpecialTxTypeMortgageInit            `json:"specialTxTypeMortgageInit"`
	SpecialTxTypeMortgageInitArr map[string]SpecialTxTypeMortgageInit `json:"specialTxTypeMortgageInitArr"`
	Traffic                      uint64                               `json:"traffic"`
	Buckets                      []*BucketPropertie                   `json:"buckets"`
	SynchronizeShareKeyArr       map[string]SynchronizeShareKey       `json:"synchronizeShareKeyArr"`
	SynchronizeShareKey          SynchronizeShareKey                  `json:"synchronizeShareKey"`
	PromissoryNotes              PromissoryNotes                      `json:"PromissoryNotes"`
}

type SynchronizeShareKey struct {
	ShareKey         string         `json:"shareKey"`
	Shareprice       *hexutil.Big   `json:"shareprice"`
	Status           int            `json:"status"`
	ShareKeyId       string         `json:"shareKeyId"`
	RecipientAddress common.Address `json:"recipientAddress"`
	FromAccount      common.Address `json:"fromAccount"`
	MailHash         string         `json:"mail_hash"`
	MailSize         int            `json:"mail_size"`
}

type BucketPropertie struct {
	BucketId string `json:"bucketId"`

	TimeStart uint64 `json:"timeStart"`
	TimeEnd   uint64 `json:"timeEnd"`

	Backup uint64 `json:"backup"`

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

type SpecialTxTypeMortgageInit FileIDArr

type LastSynState struct {
	LastRootStates   map[common.Hash]uint64 `json:"LastRootStates"`
	LastSynBlockNum  uint64                 `json:"LastSynBlockNum"`
	LastSynBlockHash common.Hash            `json:"LastSynBlockHash"`
}

func (lastSynState *LastSynState) AddLastSynState(blockhash common.Hash, blockNumber uint64) {
	lastSynState.LastRootStates[blockhash] = blockNumber
	lenth := len(lastSynState.LastRootStates)
	if uint64(lenth) > common.SynBlockLen {
		var delBlockHash common.Hash
		var delBlockBum uint64 = ^uint64(0)
		for blockHash, blockBum := range lastSynState.LastRootStates {
			if blockBum < delBlockBum {
				delBlockHash = blockHash
				delBlockBum = blockBum
			}
		}
		delete(lastSynState.LastRootStates, delBlockHash)
	}
}

type BindingTable struct {
	MainAccounts map[common.Address][]common.Address `json:"MainAccounts"`
	SubAccounts  map[common.Address]common.Address   `json:"SubAccounts"`
}

func (bindingTable *BindingTable) GetSubAccountSizeInMainAccount(mainAccount common.Address) int {
	if bindingTable.IsMainAccountExist(mainAccount) {
		return len(bindingTable.MainAccounts[mainAccount])
	}
	return 0
}

func (bindingTable *BindingTable) IsAccountInBinding(account common.Address) bool {
	if bindingTable.IsSubAccountExist(account) || bindingTable.IsMainAccountExist(account) {
		return true
	}
	return false
}

func (bindingTable *BindingTable) IsSubAccountExist(subAccount common.Address) bool {
	_, ok := bindingTable.SubAccounts[subAccount]
	return ok
}

func (bindingTable *BindingTable) IsMainAccountExist(mainAccount common.Address) bool {
	_, ok := bindingTable.MainAccounts[mainAccount]
	return ok
}

func (bindingTable *BindingTable) DelSubAccount(subAccount common.Address) {
	mainAccount, ok := bindingTable.SubAccounts[subAccount]
	if ok {
		subAccounts := bindingTable.MainAccounts[mainAccount]
		for i, account := range subAccounts {
			if bytes.Compare(account.Bytes(), subAccount.Bytes()) == 0 {
				subAccounts = append(subAccounts[:i], subAccounts[i+1:]...)
				break
			}
		}
		delete(bindingTable.SubAccounts, subAccount)
		bindingTable.MainAccounts[mainAccount] = subAccounts
		if len(subAccounts) == 0 {
			delete(bindingTable.MainAccounts, mainAccount)
		}
	}
}

func (bindingTable *BindingTable) DelMainAccount(mainAccount common.Address) []common.Address {
	subAccounts, ok := bindingTable.MainAccounts[mainAccount]
	if ok {
		for _, account := range subAccounts {
			delete(bindingTable.SubAccounts, account)
		}
		delete(bindingTable.MainAccounts, mainAccount)
	}
	return subAccounts
}

func (bindingTable *BindingTable) UpdateBinding(mainAccount, subAccount common.Address) {
	if bytes.Compare(bindingTable.SubAccounts[subAccount].Bytes(), mainAccount.Bytes()) == 0 {
		return
	}

	if bindingTable.IsSubAccountExist(subAccount) {
		bindingTable.DelSubAccount(subAccount)
	}

	if bindingTable.IsMainAccountExist(mainAccount) {
		bindingTable.MainAccounts[mainAccount] = append(bindingTable.MainAccounts[mainAccount], subAccount)
	} else {
		bindingTable.MainAccounts[mainAccount] = []common.Address{subAccount}
	}
	bindingTable.SubAccounts[subAccount] = mainAccount
}

type ForbidBackStakeList []common.Address

func (forbidList *ForbidBackStakeList) Add(addr common.Address) {
	*forbidList = append(*forbidList, addr)
}

func (forbidList *ForbidBackStakeList) Del(addr common.Address) {
	for i, addrIn := range *forbidList {
		if bytes.Compare(addrIn.Bytes(), addr.Bytes()) == 0 {
			(*forbidList) = append((*forbidList)[:i], (*forbidList)[i+1:]...)
		}
	}
}

func (forbidList *ForbidBackStakeList) IsExist(addr common.Address) bool {
	for _, addrIn := range *forbidList {
		if bytes.Compare(addrIn.Bytes(), addr.Bytes()) == 0 {
			return true
		}
	}
	return false
}

type RewardsValues struct {
	CoinActualRewards       *big.Int `json:"CoinActualRewards"`
	PreCoinActualRewards    *big.Int `json:"PreCoinActualRewards"`
	StorageActualRewards    *big.Int `json:"StorageActualRewards"`
	PreStorageActualRewards *big.Int `json:"PreStorageActualRewards"`
	TotalActualRewards      *big.Int `json:"TotalActualRewards"`
	SurplusCoin             *big.Int `json:"SurplusCoin"`
	PreSurplusCoin          *big.Int `json:"PreSurplusCoin"`
}

type AccountName [common.HashLength]byte

func (name *AccountName) Bytes() []byte { return name[:] }

func (name *AccountName) reset() {
	for i := 0; i < len(name); i++ {
		name[i] = 0
	}
}

func (name *AccountName) ToHash() (hash common.Hash) {
	hash.SetBytes(name.Bytes())
	return
}

func (name *AccountName) SetHash(hash common.Hash) {
	name.SetBytes(hash.Bytes())
}

func (name *AccountName) SetBytes(b []byte) {
	if len(b) > len(name) {
		b = b[len(b)-common.HashLength:]
	}
	copy(name[common.HashLength-len(b):], b)
}

func (name *AccountName) SetString(nameStr string) error {
	if len(nameStr) > common.HashLength {
		return errors.New("String is too long")
	}
	b := []byte(nameStr)
	name.reset()
	name.SetBytes(b)
	return nil
}

func (name *AccountName) String() string {
	idx := 0
	for i := 0; i < len(name); i++ {
		if name[i] != 0 {
			idx = i
			break
		}
	}
	return string(name[idx:])
}

func (name *AccountName) IsValid() bool {
	namestr := name.String()
	reg := regexp.MustCompile("[a-z0-9\\.]+")
	findstr := reg.FindAllString(namestr, 1)
	if !strings.EqualFold(namestr, findstr[0]) {
		return false
	}
	if strings.Contains(namestr, "..") {
		return false
	}

	return true
}

func (name *AccountName) CalPrice() int64 {
	lenth := len(name.String())
	if lenth >= 28 {
		return 0
	}
	if lenth >= 20 {
		return 1
	}
	if lenth >= 15 {
		return int64((20 - lenth) * 2)
	}
	if lenth >= 10 {
		return int64((15-lenth)*5 + 10)
	}
	if lenth >= 5 {
		return int64((10-lenth)*20 + 35)
	}
	return 1000
}

func (name *AccountName) GetBigPrice() *big.Int {
	basePrice := big.NewInt(0)
	basePrice.Div(common.BaseCompany, big.NewInt(10))

	price := name.CalPrice()

	priceBig := big.NewInt(price)
	priceBig.Mul(priceBig, common.BaseCompany)
	priceBig.Add(priceBig, basePrice)
	return priceBig
}
