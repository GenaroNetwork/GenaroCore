package common

import "math/big"

/*
Some special address prepared for special transactions.
*/
var (
	// 特殊账户处理处理特殊交易（通过交易参数中的字段区分交易的作用）
	// 	   一、stake同步:         交易发起方为用户，交易的"from"字段为用户address，交易的"to"字段为该特殊地址，参数类型字段为1
	//     二、heft同步:          交易发起方为存储网络，交易的"from"字段该特殊地址，交易的"to"字段为该特殊地址，参数类型字段为2
	//     三、存储空间申请:       交易发起方为用户，交易的"from"字段为用户address，交易的"to"字段为该特殊地址，参数类型字段为3
	//     四、空间流量申请:       交易发起方为用户，交易的"from"字段为用户address，交易的"to"字段为该特殊地址，参数类型字段为4
	//     五、跨链交易init:      交易发起方为用户，交易的"from"字段为用户address，交易的"to"字段为该特殊地址，参数类型字段为5
	//     六、跨链交易terminate: 交易发起方为用户，交易的"from"字段为用户address，交易的"to"字段为该特殊地址，参数类型字段为6
	SpecialSyncAddress Address = HexToAddress("0xc1b2e1fc9d2099f5930a669c9ad65509433550d6")
)


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

)
//费用

var OneDayGes  int64 = int64(5000)
var BucketApplyGasPerGPerDay  int64 = int64(5000)
var TrafficApplyGasPerG	 int64 = int64(5000)
var OneDaySyncLogGsa  int64 = int64(5000)
//官方账号
var OfficialAddress Address  = HexToAddress("0xa07b0fc50549c636ad4d7fbc6ea747574efb8e8a")

//特殊交易 Tx.init 格式
//其中 allow 中的权限如下
//0: readwrite
//1: readonly
//2: write
var ReadWrite int = 0
var ReadOnly int = 1
var Write int = 2
