package common

import (
	"github.com/GenaroNetwork/GenaroCore/common/math"
	"math/big"
)

func init() {
	BaseCompany = big.NewInt(0)
	BaseCompany.UnmarshalText([]byte("1000000000000000000"))

	DefaultStakeValuePerNode, _ = math.ParseBig256("5000000000000000000000")
	DefaultTrafficApplyGasPerG, _ = math.ParseBig256("50000000000000000")
	DefaultBucketApplyGasPerGPerDay, _ = math.ParseBig256("500000000000000")
	DefaultOneDaySyncLogGsaCost, _ = math.ParseBig256("1000000000000000000")
	DefaultOneDayMortgageGes, _ = math.ParseBig256("1000000000000000000")
}

//fee
var (
	BaseCompany *big.Int

	DefaultOneDaySyncLogGsaCost *big.Int

	DefaultBucketApplyGasPerGPerDay *big.Int

	DefaultTrafficApplyGasPerG *big.Int

	DefaultStakeValuePerNode *big.Int

	DefaultOneDayMortgageGes *big.Int
)

/*
Some special address prepared for special transactions.
*/
var (
	CandidateSaveAddress Address = HexToAddress("0x1000000000000000000000000000000000000000")

	BackStakeAddress Address = HexToAddress("0x2000000000000000000000000000000000000000")

	LastSynStateSaveAddress Address = HexToAddress("0x3000000000000000000000000000000000000000")

	StakeNode2StakeAddress Address = HexToAddress("0x400000000000000000000000000000000000000")

	GenaroPriceAddress Address = HexToAddress("0x500000000000000000000000000000000000000")

	SpecialSyncAddress Address = HexToAddress("0x6000000000000000000000000000000000000000")

	RewardsSaveAddress Address = HexToAddress("0x7000000000000000000000000000000000000000")

	BindingSaveAddress Address = HexToAddress("0x8000000000000000000000000000000000000000")

	ForbidBackStakeSaveAddress Address = HexToAddress("0x9000000000000000000000000000000000000000")

	OptionTxBeginSaveAddress Address = HexToAddress("0xa000000000000000000000000000000000000000")

	NameSpaceSaveAddress Address = HexToAddress("0xb000000000000000000000000000000000000000")
)

var SpecialAddressList = []Address{CandidateSaveAddress, BackStakeAddress, LastSynStateSaveAddress, StakeNode2StakeAddress, GenaroPriceAddress, SpecialSyncAddress, RewardsSaveAddress, BindingSaveAddress, ForbidBackStakeSaveAddress, NameSpaceSaveAddress}

var (
	SpecialTxTypeStakeSync = big.NewInt(1)

	SpecialTxTypeHeftSync = big.NewInt(2)

	SpecialTxTypeSpaceApply = big.NewInt(3)

	SpecialTxTypeTrafficApply = big.NewInt(4)

	SpecialTxTypeMortgageInit = big.NewInt(5)

	SpecialTxTypeMortgageTerminate = big.NewInt(6)

	SpecialTxTypeSyncSidechainStatus = big.NewInt(7)

	SpecialTxTypeSyncNode = big.NewInt(8)

	SpecialTxTypeSyncFielSharePublicKey = big.NewInt(9)

	SpecialTxTypePunishment = big.NewInt(10)

	SpecialTxTypeBackStake = big.NewInt(11)

	SpecialTxTypePriceRegulation = big.NewInt(12)

	SpecialTxSynState = big.NewInt(13)

	SpecialTxUnbindNode = big.NewInt(14)

	SynchronizeShareKey = big.NewInt(15)

	SpecialTxAccountBinding = big.NewInt(16)

	SpecialTxAccountCancelBinding = big.NewInt(17)

	SpecialTxAddAccountInForbidBackStakeList = big.NewInt(18)

	SpecialTxDelAccountInForbidBackStakeList = big.NewInt(19)

	UnlockSharedKey = big.NewInt(20)

	SpecialTxSetGlobalVar = big.NewInt(21)

	SpecialTxAddCoinpool = big.NewInt(22)

	SpecialTxRegisterName = big.NewInt(23)

	SpecialTxTransferName = big.NewInt(24)

	SpecialTxUnsubscribeName = big.NewInt(25)

	SpecialTxWithdrawCash = big.NewInt(30)

	SpecialTxRevoke = big.NewInt(31)

	SpecialTxPublishOption = big.NewInt(32)

	SpecialTxSetOptionTxStatus = big.NewInt(33)

	SpecialTxBuyPromissoryNotes = big.NewInt(35)

	SpecialTxCarriedOutPromissoryNotes = big.NewInt(36)

	SpecialTxTurnBuyPromissoryNotes = big.NewInt(37)

	SpecialTxBucketSupplement = big.NewInt(41)
)

var ReadWrite int = 0
var ReadOnly int = 1
var Write int = 2

var SynBlockLen = uint64(6)

var BackStakePeriod = uint64(5)
var Base = uint64(100000)

var (
	MaxBinding          = uint64(10)
	MinStake            = uint64(5000)
	CommitteeMinStake   = uint64(5000)
	BackStackListMax    = uint64(20)
	CoinRewardsRatio    = uint64(2)
	StorageRewardsRatio = uint64(1)
	RatioPerYear        = uint64(2)
	BlockLogLenth       = uint64(500000)
)
