// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package vm

import (
	"math/big"

	"github.com/GenaroNetwork/GenaroCore/common"
	"github.com/GenaroNetwork/GenaroCore/common/hexutil"
	"github.com/GenaroNetwork/GenaroCore/core/state"
	"github.com/GenaroNetwork/GenaroCore/core/types"
)

// StateDB is an EVM database for full state querying.
type StateDB interface {
	CreateAccount(common.Address)

	SubBalance(common.Address, *big.Int)
	AddBalance(common.Address, *big.Int)
	GetBalance(common.Address) *big.Int

	GetNonce(common.Address) uint64
	SetNonce(common.Address, uint64)

	GetCodeHash(common.Address) common.Hash
	GetCode(common.Address) []byte
	SetCode(common.Address, []byte)
	GetCodeSize(common.Address) int
	IsContract(addr common.Address) bool

	AddRefund(uint64)
	GetRefund() uint64

	GetState(common.Address, common.Hash) common.Hash
	SetState(common.Address, common.Hash, common.Hash)

	Suicide(common.Address) bool
	HasSuicided(common.Address) bool

	// Exist reports whether the given account exists in state.
	// Notably this should also return true for suicided accounts.
	Exist(common.Address) bool
	// Empty returns whether the given account is empty. Empty
	// is defined according to EIP161 (balance = nonce = code = 0).
	Empty(common.Address) bool

	RevertToSnapshot(int)
	Snapshot() int

	AddLog(*types.Log)
	AddPreimage(common.Hash, []byte)

	ForEachStorage(common.Address, func(common.Hash, common.Hash) bool)

	UpdateHeft(common.Address, uint64, uint64) bool
	GetHeft(common.Address) (uint64, error)
	GetHeftLog(common.Address) types.NumLogs
	GetHeftRangeDiff(common.Address, uint64, uint64) uint64

	UpdateStake(common.Address, uint64, uint64) bool
	DeleteStake(common.Address, uint64, uint64) (bool, uint64)
	GetStake(common.Address) (uint64, error)
	GetStakeLog(common.Address) types.NumLogs
	GetStakeRangeDiff(common.Address, uint64, uint64) uint64

	AddCandidate(common.Address) bool
	DelCandidate(candidate common.Address) bool
	GetCandidates() state.Candidates
	GetCandidatesInfoInRange(uint64, uint64) []state.CandidateInfo
	IsCandidateExist(candidate common.Address) bool

	UpdateBucketProperties(common.Address, string, uint64, uint64, uint64, uint64) bool
	UpdateBucket(common.Address, types.BucketPropertie) bool
	GetStorageSize(common.Address, [32]byte) (uint64, error)
	GetStorageGasPrice(common.Address, [32]byte) (uint64, error)
	GetStorageGasUsed(common.Address, [32]byte) (uint64, error)
	GetStorageGas(common.Address, [32]byte) (uint64, error)
	SpecialTxTypeMortgageInit(common.Address, types.SpecialTxTypeMortgageInit) bool
	SpecialTxTypeSyncSidechainStatus(common.Address, types.SpecialTxTypeMortgageInit) (map[common.Address]*big.Int, bool)
	UpdateTraffic(common.Address, uint64) bool

	GetTraffic(common.Address) uint64

	GetBuckets(common.Address) (map[string]interface{}, error)

	//根据用户id和fileID,dataVersion获取交易日志
	TxLogByDataVersionRead(common.Address, [32]byte, string) (map[common.Address]*hexutil.Big, error)
	//根据用户id和fileID开启定时同步日志接口
	TxLogBydataVersionUpdate(common.Address, [32]byte, common.Address) bool

	SyncStakeNode(common.Address, string) error
	GetStorageNodes(addr common.Address) []string
	SyncNode2Address(common.Address, string, string) error
	GetAddressByNode(string) string

	AddAlreadyBackStack(backStack common.AlreadyBackStake) bool
	GetAlreadyBackStakeList() (bool, common.BackStakeList)
	SetAlreadyBackStakeList(common.BackStakeList) bool
	IsAlreadyBackStake(addr common.Address) bool

	SynchronizeShareKey(common.Address, types.SynchronizeShareKey) bool

	UpdateFileSharePublicKey(common.Address, string) bool
	UnlockSharedKey(common.Address, string) bool
	GetSharedFile(common.Address, string) types.SynchronizeShareKey
	UpdateBucketApplyPrice(common.Address, *hexutil.Big) bool
	GetBucketApplyPrice() *big.Int

	UpdateTrafficApplyPrice(common.Address, *hexutil.Big) bool
	GetTrafficApplyPrice() *big.Int

	UpdateStakePerNodePrice(common.Address, *hexutil.Big) bool
	GetStakePerNodePrice() *big.Int

	GetGenaroPrice() *types.GenaroPrice
	SetGenaroPrice(genaroPrice types.GenaroPrice) bool
	UpdateOneDayGesCost(common.Address, *hexutil.Big) bool
	UpdateOneDaySyncLogGsaCost(common.Address, *hexutil.Big) bool

	GetOneDayGesCost() *big.Int
	GetOneDaySyncLogGsaCost() *big.Int

	AddLastRootState(statehash common.Hash, blockNumber uint64) bool
	SetLastSynBlock(blockNumber uint64, blockHash common.Hash) bool
	GetLastSynState() *types.LastSynState

	UpdateAccountBinding(mainAccount common.Address, subAccount common.Address) bool
	GetMainAccount(subAccount common.Address) *common.Address
	IsBindingAccount(account common.Address) bool
	GetSubAccountsCount(mainAccount common.Address) int
	IsBindingSubAccount(account common.Address) bool
	IsBindingMainAccount(account common.Address) bool
	DelSubAccountBinding(subAccount common.Address) bool
	DelMainAccountBinding(mainAccount common.Address) []common.Address
	GetMainAccounts() map[common.Address][]common.Address

	AddAccountInForbidBackStakeList(address common.Address) bool
	DelAccountInForbidBackStakeList(address common.Address) bool
	IsAccountExistInForbidBackStakeList(address common.Address) bool
	GetForbidBackStakeList() types.ForbidBackStakeList

	UnbindNode(common.Address, string) error
	UbindNode2Address(common.Address, string) error

	GetRewardsValues() *types.RewardsValues
	SetRewardsValues(rewardsValues types.RewardsValues) bool

	PromissoryNotesWithdrawCash(common.Address, uint64) uint64
	GetPromissoryNotes(address common.Address) types.PromissoryNotes
	AddPromissoryNote(address common.Address, promissoryNote types.PromissoryNote) bool
	DelPromissoryNote(address common.Address, promissoryNote types.PromissoryNote) bool

	GetOptionTxTable(common.Hash, uint64) *types.OptionTxTable
	GetOptionTxTableByAddress(common.Address) *types.OptionTxTable
	DelTxInOptionTxTable(common.Hash, uint64) bool
	AddTxInOptionTxTable(common.Hash, types.PromissoryNotesOptionTx, uint64) bool
	SetTxStatusInOptionTxTable(common.Hash, bool, uint64) bool
	BuyPromissoryNotes(common.Hash, common.Address, uint64) types.PromissoryNotesOptionTx
	CarriedOutPromissoryNotes(common.Hash, common.Address, uint64) types.PromissoryNotesOptionTx
	TurnBuyPromissoryNotes(common.Hash, *hexutil.Big, common.Address, uint64) bool
	GetBeforPromissoryNotesNum(common.Address, uint64) uint64

	// 别名
	GetNameAccount(name string) (addr common.Address, err error)
	SetNameAccount(name string, addr common.Address) (err error)
	IsNameAccountExist(name string) (bool, error)
	HasName(common.Address, string) bool

	// 收益账号
	GetProfitAccount(id common.Address) *common.Address
	SetProfitAccount(saddr,paddr common.Address) bool
	SetShadowAccount(saddr,paddr common.Address) bool
}

// CallContext provides a basic interface for the EVM calling conventions. The EVM EVM
// depends on this context being implemented for doing subcalls and initialising new EVM contracts.
type CallContext interface {
	// Call another contract
	Call(env *EVM, me ContractRef, addr common.Address, data []byte, gas, value *big.Int) ([]byte, error)
	// Take another's contract code and execute within our own context
	CallCode(env *EVM, me ContractRef, addr common.Address, data []byte, gas, value *big.Int) ([]byte, error)
	// Same as CallCode except sender and value is propagated from parent to child scope
	DelegateCall(env *EVM, me ContractRef, addr common.Address, data []byte, gas *big.Int) ([]byte, error)
	// Create a new contract
	Create(env *EVM, me ContractRef, data []byte, gas, value *big.Int) ([]byte, common.Address, error)
}
