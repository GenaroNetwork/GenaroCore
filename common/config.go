package common


/*
Some special address prepared for special transactions.
*/
var (

	// SentinelHelfSyncAddress has two uses as follows:
	// One, if the "from" parameter of the transaction is user's address and "to" parameter of the transaction is this special address
	// it means current transaction is userd to apply for storage space

	//Two. if the "from" and "to" parameters of the transaction are both this addresses, it means this tx is to set user's heft
	SentinelHelfSyncAddress Address = HexToAddress("0xc1b2e1fc9d2099f5930a669c9ad65509433550d6")


	// SentinelStakeSyncAddress is used by storage network to set user's stake
	// if the "from" parameter of the transaction is user's address and "to" parameter of the transaction is this special address
	SentinelStakeSyncAddress Address = HexToAddress("0x1230b1c187b8b2911aaa7a4674a3435518bf21b4")


	// 特殊账户处理处理特殊交易（通过交易参数中的字段区分交易的作用）
	// 	   一、stake同步:         交易发起方为用户，交易的"from"字段为用户address，交易的"to"字段为该特殊地址，参数类型字段为1
	//     二、heft同步:          交易发起方为存储网络，交易的"from"字段该特殊地址，交易的"to"字段为该特殊地址
	//     三、存储空间申请:       交易发起方为用户，交易的"from"字段为用户address，交易的"to"字段为该特殊地址，参数类型字段为2
	//     四、空间流量申请:       交易发起方为用户，交易的"from"字段为用户address，交易的"to"字段为该特殊地址，参数类型字段为3
	//     五、跨链交易init:      交易发起方为用户，交易的"from"字段为用户address，交易的"to"字段为该特殊地址，参数类型字段为4
	//     六、跨链交易terminate: 交易发起方为用户，交易的"from"字段为用户address，交易的"to"字段为该特殊地址，参数类型字段为5
	SpecialSyncAddress Address = HexToAddress("0x0000000000000000000000000000000000000000")
)


var (

	//SpecialTxTypeStakeSync类型的交易代表stake同步
	SpecialTxTypeStakeSync         int = 1

	//SpecialTxTypeHeftSync类型的交易代表heft同步
	SpecialTxTypeHeftSync          int = 2

	//SpecialTxTypeSpaceApply类型的交易代表用户申请存储空间
	SpecialTxTypeSpaceApply        int = 3

	//SpecialTxTypeTrafficApply类型的交易代表用户给存储申请流量
	SpecialTxTypeTrafficApply      int = 4

	//SpecialTxTypeMortgageInit类型的交易代表用户押注初始化交易
	SpecialTxTypeMortgageInit      int = 5

	//SpecialTxTypeMortgageTerminate类型的交易代表用户押注结束时的结算交易
	SpecialTxTypeMortgageTerminate int = 6

)