package common


/*
Some special address prepared for special transactions.
*/
var (

	SentinelHelfSyncAddress Address = HexToAddress("0xc1b2e1fc9d2099f5930a669c9ad65509433550d6")

	SentinelStakeSyncAddress Address = HexToAddress("0x1230b1c187b8b2911aaa7a4674a3435518bf21b4")

	// save candidate list in this address storage
	CandidateSaveAddress Address = HexToAddress("0x1000000000000000000000000000000000000000")
)