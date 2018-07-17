package common

import (
	"math/big"
	"github.com/GenaroNetwork/Genaro-Core/common/math"
)

func init() {
	BaseCompany = big.NewInt(0)
	BaseCompany.UnmarshalText([]byte("1000000000000000000")) // 基本单位1GNX

	DefaultStakeValuePerNode, _        = math.ParseBig256("5000000000000000000000") // 同步一个节点5000个gnx
	DefaultTrafficApplyGasPerG, _      = math.ParseBig256("50000000000000000") // 购买流量每G需0.05个gnx
	DefaultBucketApplyGasPerGPerDay, _ = math.ParseBig256("500000000000000") // 购买空间每天每G需0.0005个gnx（0.015个GNX每GB每月）


	DefaultOneDaySyncLogGsaCost, _ = math.ParseBig256("1000000000000000000")
	DefaultOneDayMortgageGes, _ = math.ParseBig256("1000000000000000000")
}


//费用
var (
	BaseCompany *big.Int

	DefaultOneDaySyncLogGsaCost  *big.Int

	DefaultBucketApplyGasPerGPerDay *big.Int

	DefaultTrafficApplyGasPerG *big.Int

	DefaultStakeValuePerNode *big.Int

	DefaultOneDayMortgageGes *big.Int
)


var (
	//官方账号
	OfficialAddress Address  = HexToAddress("0x3f70180da635e0205525106632bf1689ea1f7a84")
)

/*
Some special address prepared for special transactions.
*/
var (

	// save candidate list in this address storage
	CandidateSaveAddress		Address	= HexToAddress("0x1000000000000000000000000000000000000000")

	// 退注记录地址
	BackStakeAddress			Address	= HexToAddress("0x2000000000000000000000000000000000000000")

	// save last heft state
	LastSynStateSaveAddress		Address	= HexToAddress("0x3000000000000000000000000000000000000000")

	//特殊账户，该账户存储矿工节点Id到账户的倒排索引
	StakeNode2StakeAddress Address = HexToAddress("0x400000000000000000000000000000000000000")

	GenaroPriceAddress Address = HexToAddress("0x500000000000000000000000000000000000000")

	// 特殊账户处理处理特殊交易（通过交易参数中的字段区分交易的作用）
	// 	   一、stake同步:         交易发起方为用户，交易的"from"字段为用户address，交易的"to"字段为该特殊地址，参数类型字段为1
	//     二、heft同步:          交易发起方为存储，交易的"from"为存储节点的address，交易的"to"字段为该特殊地址，参数类型字段为2
	//     三、存储空间申请:       交易发起方为用户，交易的"from"字段为用户address，交易的"to"字段为该特殊地址，参数类型字段为3
	//     四、空间流量申请:       交易发起方为用户，交易的"from"字段为用户address，交易的"to"字段为该特殊地址，参数类型字段为4
	//     五、跨链交易init:      交易发起方为用户，交易的"from"字段为用户address，交易的"to"字段为该特殊地址，参数类型字段为5
	//     六、跨链交易terminate: 交易发起方为用户，交易的"from"字段为用户address，交易的"to"字段为该特殊地址，参数类型字段为6
	//     七、跨链交易Sidechina: 交易发起方为存储，交易的"from"字段为用户address，交易的"to"字段为该特殊地址，参数类型字段为7
	//     八、矿工节点同步:      交易发起方为矿工，交易的"from"字段为用户address，交易的"to"字段为该特殊地址，参数类型字段为8
	SpecialSyncAddress Address = HexToAddress("0x6000000000000000000000000000000000000000")

	//	用于存放收益计算数据的地址
	RewardsSaveAddress Address = HexToAddress("0x7000000000000000000000000000000000000000")

	// 父子账号的绑定表
	BindingSaveAddress Address = HexToAddress("0x8000000000000000000000000000000000000000")

)

var SpecialAddressList = []Address{CandidateSaveAddress, BackStakeAddress, LastSynStateSaveAddress, StakeNode2StakeAddress, GenaroPriceAddress, SpecialSyncAddress, RewardsSaveAddress, BindingSaveAddress}



var (

	//SpecialTxTypeStakeSync类型的交易代表stake同步
	SpecialTxTypeStakeSync          = big.NewInt(1)
	//SpecialTxTypeHeftSync类型的交易代表heft同步
	SpecialTxTypeHeftSync          = big.NewInt(2)

	//SpecialTxTypeSpaceApply类型的交易代表用户申请存储空间
	SpecialTxTypeSpaceApply        = big.NewInt(3)

	//SpecialTxTypeTrafficApply类型的交易代表用户给存储申请流量
	SpecialTxTypeTrafficApply      = big.NewInt(4)

	//SpecialTxTypeMortgageInit类型的交易代表用户押注初始化交易
	SpecialTxTypeMortgageInit      = big.NewInt(5)

	//SpecialTxTypeMortgageTerminate类型的交易代表用户押注结束时的结算交易
	SpecialTxTypeMortgageTerminate = big.NewInt(6)
	//SpecialTxTypeSyncSidechainStatus类型的交易代表同步侧链状态
	SpecialTxTypeSyncSidechainStatus = big.NewInt(7)

	//SpecialTxTypeSyncNodeId类型的交易代表用户同步stake时的节点到链上
	SpecialTxTypeSyncNode =big.NewInt(8)

	// SpecialTxTypeSyncSecretKey类型的交易代表用户同步文件分享公钥
	SpecialTxTypeSyncFielSharePublicKey  = big.NewInt(9)

	// SpecialTxTypePunishment 类型的交易代表对用户进行stake扣除惩罚交易
	SpecialTxTypePunishment  = big.NewInt(10)
	// 退注特殊交易
	SpecialTxTypeBackStake  = big.NewInt(11)

	//价格调控
	SpecialTxTypePriceRegulation = big.NewInt(12)

	// 区块状态同步特殊交易
	SpecialTxSynState  = big.NewInt(13)

	//解除节点绑定
	SpecialTxUnbindNode = big.NewInt(14)

	//同步分享秘钥
	SynchronizeShareKey = big.NewInt(15)

	// 账号绑定
	SpecialTxAccountBinding = big.NewInt(16)

	// 解除账号的绑定关系
	SpecialTxAccountCancelBinding = big.NewInt(17)

	//解锁分享秘钥
	UnlockSharedKey = big.NewInt(20)
)




	var Base = uint64(100000)	// 收益计算中间值
	var BackStackListMax = int(20)		// 最大退注长度



	//特殊交易 Tx.init 格式
	//其中 allow 中的权限如下
	//0: readwrite
	//1: readonly
	//2: write
	var ReadWrite int = 0
	var ReadOnly int = 1
	var Write int = 2

	// 同步交易的块间隔
	var SynBlockLen = uint64(6)

	// 一个主节点最大的绑定数量
	var MaxBinding = 10
	// 一次最小的押注额度
	var MinStake = uint64(5000)
	// 进入委员会需要的最小stake
	var CommitteeMinStake = uint64(5000)
