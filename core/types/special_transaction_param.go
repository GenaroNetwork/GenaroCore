package types

import (
	"github.com/GenaroNetwork/Genaro-Core/common"
	"github.com/GenaroNetwork/Genaro-Core/common/hexutil"
	"math/big"
	"math"
	"bytes"
	"fmt"
	"github.com/GenaroNetwork/Genaro-Core/log"
	"github.com/GenaroNetwork/Genaro-Core/rlp"
	"github.com/GenaroNetwork/Genaro-Core/crypto"
	"time"
)

type SpecialTxInput struct {
	GenaroData
	Address     string       `json:"address"`
	NodeID      string       `json:"nodeId"`
	BucketID    string       `json:"bucketId"`
	Size        uint64       `json:"size"`
	Duration    uint64       `json:"duration"`
	Type        *hexutil.Big `json:"type"`
	BlockNumber string       `json:"blockNr"`
	Message     string       `json:"msg"`
	Sign        string       `json:"sign"`
	AddCoin	*hexutil.Big     `json:"addCoin"`
	OrderId     common.Hash       `json:"orderId"`
	RestoreBlock	uint64		`json:"RestoreBlock"`	// 期票的返还块号
	TxNum			uint64		`json:"TxNum"`	// 此单交易的数量
	PromissoryNoteTxPrice	*hexutil.Big	`json:"PromissoryNoteTxPrice"`	// 期票的交易单价
	OptionPrice		*hexutil.Big	`json:"OptionPrice"`	// 期权的价格
	IsSell			bool		`json:"IsSell"`	// 期权是否在售
	GenaroPrice
}

type GenaroPrice struct {
	BucketApplyGasPerGPerDay *hexutil.Big `json:"bucketPricePerGperDay"`
	TrafficApplyGasPerG *hexutil.Big `json:"trafficPricePerG"`
	StakeValuePerNode *hexutil.Big `json:"stakeValuePerNode"`
	OneDayMortgageGes	*hexutil.Big `json:"oneDayMortgageGes"`
	OneDaySyncLogGsaCost  *hexutil.Big `json:"oneDaySyncLogGsaCost"`
	MaxBinding	uint64	`json:"MaxBinding"`	// 一个主节点最大的绑定数量
	MinStake	uint64	`json:"MinStake"`	// 一次最小的押注额度
	CommitteeMinStake	uint64	`json:"CommitteeMinStake"`	// 进入委员会需要的最小stake
	BackStackListMax	uint64	`json:"BackStackListMax"`	// 最大退注长度
	CoinRewardsRatio	uint64	`json:"CoinRewardsRatio"`	// 币息收益比率
	StorageRewardsRatio	uint64	`json:"StorageRewardsRatio"`	// 存储收益比率
	RatioPerYear	uint64	`json:"RatioPerYear"`	// 年收益比率
	SynStateAccount	string	`json:"SynStateAccount"`	// 区块同步信号的发送地址
	HeftAccount		string	`json:"HeftAccount"`	// 用于同步链上Heft值的账号
	BindingAccount	string	`json:"BindingAccount"`	// 用于账号绑定的账号
	ExtraPrice     []byte   `json:"extraPrice"` //该版本用不上，考虑后期版本兼容性使用
}

func (s SpecialTxInput) SpecialCost(currentPrice *GenaroPrice, bucketsMap map[string]interface{}) big.Int {

	switch s.Type.ToInt().Uint64() {
	case common.SpecialTxTypeStakeSync.Uint64():
		ret := new(big.Int).Mul(new(big.Int).SetUint64(s.Stake),common.BaseCompany)
		return *ret
	case common.SpecialTxTypeSpaceApply.Uint64():
		var totalCost *big.Int = big.NewInt(0)
		var bucketPrice *big.Int
		if currentPrice != nil && currentPrice.BucketApplyGasPerGPerDay != nil {
			bucketPrice = new(big.Int).Set(currentPrice.BucketApplyGasPerGPerDay.ToInt())
		}else {
			bucketPrice = new(big.Int).Set(common.DefaultBucketApplyGasPerGPerDay)
		}
		for _, v := range s.Buckets {
			duration := math.Ceil(math.Abs(float64(v.TimeStart) - float64(v.TimeEnd))/86400)
			//log.Info(fmt.Sprintf("duration: %f",duration))
			oneCost := new(big.Int).Mul(bucketPrice, big.NewInt(int64(v.Size) * int64(duration)))
			//log.Info(fmt.Sprintf("oneCost: %s",oneCost.String()))
			totalCost.Add(totalCost, oneCost)
		}
		log.Info(fmt.Sprintf("bucket apply cost:%s", totalCost.String()))
		return *totalCost
	case common.SpecialTxBucketSupplement.Uint64():
		var totalCost *big.Int = big.NewInt(0)
		var bucketPrice *big.Int
		if currentPrice != nil && currentPrice.BucketApplyGasPerGPerDay != nil {
			bucketPrice = new(big.Int).Set(currentPrice.BucketApplyGasPerGPerDay.ToInt())
		}else {
			bucketPrice = new(big.Int).Set(common.DefaultBucketApplyGasPerGPerDay)
		}

		if v, ok := bucketsMap[s.BucketID]; ok {
			bucketPropertie := v.(BucketPropertie)

			if s.Size != 0 && s.Duration == 0{
				calSize := s.Size
				var subtraction float64
				if uint64(time.Now().Unix()) > bucketPropertie.TimeStart  {
					subtraction = float64(time.Now().Unix())
				} else {
					subtraction = float64(bucketPropertie.TimeStart)
				}
				calDuration := math.Ceil(math.Abs(float64(bucketPropertie.TimeEnd) - subtraction)/86400)
				totalCost = new(big.Int).Mul(bucketPrice, big.NewInt(int64(calSize) * int64(calDuration)))
			} else if s.Size == 0 && s.Duration != 0 {
				calSize := bucketPropertie.Size
				calDuration := math.Ceil(float64(s.Duration)/86400)
				totalCost = new(big.Int).Mul(bucketPrice, big.NewInt(int64(calSize) * int64(calDuration)))
			}else if s.Size != 0 && s.Duration != 0 {
				calSize := bucketPropertie.Size + s.Size
				calDuration := math.Ceil(float64(s.Duration)/86400)
				totalCost1 := new(big.Int).Mul(bucketPrice, big.NewInt(int64(calSize) * int64(calDuration)))


				var subtraction float64
				if uint64(time.Now().Unix()) > bucketPropertie.TimeStart  {
					subtraction = float64(time.Now().Unix())
				} else {
					subtraction = float64(bucketPropertie.TimeStart)
				}
				calSize2 := s.Size
				calDuration2 := math.Ceil(math.Abs(float64(bucketPropertie.TimeEnd) - subtraction)/86400)
				totalCost2 := new(big.Int).Mul(bucketPrice, big.NewInt(int64(calSize2) * int64(calDuration2)))

				totalCost = new(big.Int).Add(totalCost1, totalCost2)
			}


		}

		log.Info(fmt.Sprintf("bucket supplement cost:%s", totalCost.String()))
		return *totalCost
	case common.SpecialTxTypeTrafficApply.Uint64():

		var trafficPrice *big.Int
		if currentPrice != nil && currentPrice.BucketApplyGasPerGPerDay != nil {
			trafficPrice = new(big.Int).Set(currentPrice.TrafficApplyGasPerG.ToInt())
		}else {
			trafficPrice = new(big.Int).Set(common.DefaultTrafficApplyGasPerG)
		}

		totalGas := new(big.Int).Mul(trafficPrice, big.NewInt(int64(s.Traffic)))
		log.Info(fmt.Sprintf("traffic apply cost:%s", totalGas.String()))
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

// 用户账户下的期票
type PromissoryNote struct {
	RestoreBlock uint64	`json:"restoreBlock"`	// 返还的块号
	Num	uint64			`json:"Num"`	// 期票数量
}

type PromissoryNotes	[]PromissoryNote

// 增加期票
func (notes *PromissoryNotes) Add(newNote PromissoryNote){
	isExist := false
	for i,note := range *notes {
		if note.RestoreBlock == newNote.RestoreBlock {
			(*notes)[i].Num += newNote.Num
			isExist = true
			break
		}
	}

	if !isExist {
		*notes = append(*notes,newNote)
	}
}

// 减少期票,返回是否成功减少
func (notes *PromissoryNotes) Del(newNote PromissoryNote) bool{
	isSuccess := false
	for i,note := range *notes {
		if note.RestoreBlock == newNote.RestoreBlock {
			if (*notes)[i].Num >= newNote.Num {
				(*notes)[i].Num -= newNote.Num
				isSuccess = true
				if (*notes)[i].Num == 0 {
					(*notes) = append((*notes)[:i],(*notes)[i+1:]...)
				}
			}
			break
		}
	}
	return isSuccess
}

// 删除到期的期票，返回删除的数量
func (notes *PromissoryNotes) DelBefor(blockNum uint64) uint64 {
	delNum := uint64(0)
	for i:=0;i<len(*notes);i++ {
		if (*notes)[i].RestoreBlock <= blockNum {
			delNum += (*notes)[i].Num
			(*notes) = append((*notes)[:i],(*notes)[i+1:]...)
			i--
		}
	}
	return delNum
}

// 到期的期票数量
func (notes *PromissoryNotes) GetBefor(blockNum uint64) uint64 {
	num := uint64(0)
	for i:=0;i<len(*notes);i++ {
		if (*notes)[i].RestoreBlock <= blockNum {
			num += (*notes)[i].Num
		}
	}
	return num
}

// 返回某一类期票的数量
func (notes *PromissoryNotes) GetNum(restoreBlock uint64) uint64 {
	for _,note := range *notes {
		if note.RestoreBlock == restoreBlock {
			return note.Num
		}
	}
	return 0
}

// 获取期票总数
func (notes *PromissoryNotes) GetAllNum() uint64 {
	allNum := uint64(0)
	for _,note := range *notes {
		allNum += note.Num
	}
	return allNum
}

// 期票期权交易
type PromissoryNotesOptionTx struct {
	IsSell			bool		`json:"IsSell"`	// 期权是否在售
	OptionPrice		*big.Int	`json:"OptionPrice"`	// 期权的价格
	RestoreBlock	uint64		`json:"RestoreBlock"`	// 期票的返还块号
	TxNum			uint64		`json:"TxNum"`	// 此单交易的数量
	PromissoryNoteTxPrice	*big.Int	`json:"PromissoryNoteTxPrice"`	// 期票的交易单价
	PromissoryNotesOwner	common.Address	`json:"PromissoryNotesOwner"`	// 期票的拥有者
	OptionOwner		common.Address	`json:"OptionOwner"`
}

// 期权交易表
type OptionTxTable map[common.Hash]PromissoryNotesOptionTx

// 生成期权交易hash
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
	HeftLog						 NumLogs								`json:"heftlog"`
	StakeLog					 NumLogs								`json:"stakelog"`
	FileSharePublicKey           string                               	`json:"publicKey"`
	Node                         []string                             	`json:"syncNode"`
	SpecialTxTypeMortgageInit    SpecialTxTypeMortgageInit            	`json:"specialTxTypeMortgageInit"`
	SpecialTxTypeMortgageInitArr map[string]SpecialTxTypeMortgageInit `json:"specialTxTypeMortgageInitArr"`
	Traffic                      uint64                               	`json:"traffic"`
	Buckets                      []*BucketPropertie                   	`json:"buckets"`
	SynchronizeShareKeyArr 		 map[string] SynchronizeShareKey	  	`json:"synchronizeShareKeyArr"`
	SynchronizeShareKey			 SynchronizeShareKey				   	`json:"synchronizeShareKey"`
	PromissoryNotes				PromissoryNotes							`json:"PromissoryNotes"`
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

// 区块同步信号的数据结构
type LastSynState struct {
	LastRootStates map[common.Hash]uint64	`json:"LastRootStates"`
	LastSynBlockNum uint64					`json:"LastSynBlockNum"`
	LastSynBlockHash common.Hash			`json:"LastSynBlockHash"`
}

func (lastSynState *LastSynState)AddLastSynState(blockhash common.Hash, blockNumber uint64){
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

// 父子账号绑定关系表
type BindingTable struct {
	MainAccounts	map[common.Address][]common.Address		`json:"MainAccounts"`
	SubAccounts		map[common.Address]common.Address			`json:"SubAccounts"`
}

func (bindingTable *BindingTable) GetSubAccountSizeInMainAccount(mainAccount common.Address) int {
	if bindingTable.IsMainAccountExist(mainAccount) {
		return len(bindingTable.MainAccounts[mainAccount])
	}
	return 0
}

func (bindingTable *BindingTable) IsAccountInBinding(account common.Address) bool{
	if bindingTable.IsSubAccountExist(account) || bindingTable.IsMainAccountExist(account) {
		return true
	}
	return false
}

func (bindingTable *BindingTable) IsSubAccountExist(subAccount common.Address) bool{
	_,ok := bindingTable.SubAccounts[subAccount]
	return ok
}

func (bindingTable *BindingTable) IsMainAccountExist(mainAccount common.Address) bool{
	_,ok := bindingTable.MainAccounts[mainAccount]
	return ok
}

// 删除子账号的绑定
func (bindingTable *BindingTable) DelSubAccount(subAccount common.Address){
	mainAccount,ok := bindingTable.SubAccounts[subAccount]
	if ok {
		subAccounts := bindingTable.MainAccounts[mainAccount]
		for i,account := range subAccounts {
			if bytes.Compare(account.Bytes(),subAccount.Bytes()) == 0 {
				subAccounts = append(subAccounts[:i],subAccounts[i+1:]...)
				break
			}
		}
		delete(bindingTable.SubAccounts,subAccount)
		bindingTable.MainAccounts[mainAccount] = subAccounts
		if len(subAccounts) == 0 {
			delete(bindingTable.MainAccounts,mainAccount)
		}
	}
}

// 删除主账号账号的绑定
// 返回被关联删除的子账号列表
func (bindingTable *BindingTable) DelMainAccount(mainAccount common.Address) []common.Address{
	subAccounts,ok := bindingTable.MainAccounts[mainAccount]
	if ok {
		for _,account := range subAccounts {
			delete(bindingTable.SubAccounts,account)
		}
		delete(bindingTable.MainAccounts,mainAccount)
	}
	return subAccounts
}

// 更新绑定信息
func (bindingTable *BindingTable) UpdateBinding(mainAccount,subAccount common.Address) {
	// 账号已绑定
	if bytes.Compare(bindingTable.SubAccounts[subAccount].Bytes(),mainAccount.Bytes()) == 0{
		return
	}
	// 账号已存在
	if bindingTable.IsSubAccountExist(subAccount) {
		bindingTable.DelSubAccount(subAccount)
	}

	if bindingTable.IsMainAccountExist(mainAccount){
		bindingTable.MainAccounts[mainAccount] = append(bindingTable.MainAccounts[mainAccount],subAccount)
	}else {
		bindingTable.MainAccounts[mainAccount] = []common.Address{subAccount}
	}
	bindingTable.SubAccounts[subAccount] = mainAccount
}

// 禁止退注的列表
type ForbidBackStakeList []common.Address

func (forbidList *ForbidBackStakeList) Add(addr common.Address) {
	*forbidList = append(*forbidList,addr)
}

func (forbidList *ForbidBackStakeList) Del(addr common.Address) {
	for i,addrIn := range *forbidList {
		if bytes.Compare(addrIn.Bytes(),addr.Bytes()) == 0 {
			(*forbidList) = append((*forbidList)[:i],(*forbidList)[i+1:]...)
		}
	}
}

func (forbidList *ForbidBackStakeList)IsExist(addr common.Address) bool{
	for _,addrIn := range *forbidList {
		if bytes.Compare(addrIn.Bytes(),addr.Bytes()) == 0 {
			return true
		}
	}
	return false
}

// 收益计算中间值
type RewardsValues struct {
	CoinActualRewards *big.Int	`json:"CoinActualRewards"`
	PreCoinActualRewards *big.Int	`json:"PreCoinActualRewards"`
	StorageActualRewards *big.Int	`json:"StorageActualRewards"`
	PreStorageActualRewards *big.Int	`json:"PreStorageActualRewards"`
	TotalActualRewards *big.Int	`json:"TotalActualRewards"`
	SurplusCoin *big.Int	`json:"SurplusCoin"`
	PreSurplusCoin *big.Int	`json:"PreSurplusCoin"`
}
