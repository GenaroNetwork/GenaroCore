// Copyright 2014 The go-ethereum Authors
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
	"encoding/json"
	"errors"
	"math/big"
	"sync/atomic"
	"time"

	"fmt"
	"github.com/GenaroNetwork/GenaroCore/common"
	"github.com/GenaroNetwork/GenaroCore/core/types"
	"github.com/GenaroNetwork/GenaroCore/crypto"
	"github.com/GenaroNetwork/GenaroCore/log"
	"github.com/GenaroNetwork/GenaroCore/params"
)

// emptyCodeHash is used by create to ensure deployment is disallowed to already
// deployed contract addresses (relevant after the account abstraction).
var emptyCodeHash = crypto.Keccak256Hash(nil)

type (
	CanTransferFunc func(StateDB, common.Address, *big.Int) bool
	TransferFunc    func(StateDB, common.Address, common.Address, *big.Int)
	// GetHashFunc returns the nth block hash in the blockchain
	// and is used by the BLOCKHASH EVM op code.
	GetHashFunc     func(uint64) common.Hash
	GetSentinelFunc func(uint64) uint64
)

// run runs the given contract and takes care of running precompiles with a fallback to the byte code interpreter.
func run(evm *EVM, contract *Contract, input []byte) ([]byte, error) {
	if contract.CodeAddr != nil {
		precompiles := PrecompiledContractsHomestead
		if evm.ChainConfig().IsByzantium(evm.BlockNumber) {
			precompiles = PrecompiledContractsByzantium
		}
		if p := precompiles[*contract.CodeAddr]; p != nil {
			return RunPrecompiledContract(p, input, contract)
		}
	}
	return evm.interpreter.Run(contract, input)
}

// Context provides the EVM with auxiliary information. Once provided
// it shouldn't be modified.
type Context struct {
	// CanTransfer returns whether the account contains
	// sufficient ether to transfer the value
	CanTransfer CanTransferFunc
	// Transfer transfers ether from one account to the other
	Transfer TransferFunc
	// GetHash returns the hash corresponding to n
	GetHash GetHashFunc

	// Message information
	Origin   common.Address // Provides information for ORIGIN
	GasPrice *big.Int       // Provides information for GASPRICE

	// Block information
	Coinbase    common.Address // Provides information for COINBASE
	GasLimit    uint64         // Provides information for GASLIMIT
	BlockNumber *big.Int       // Provides information for NUMBER
	Time        *big.Int       // Provides information for TIME
	Difficulty  *big.Int       // Provides information for DIFFICULTY
}

// EVM is the Ethereum Virtual Machine base object and provides
// the necessary tools to run a contract on the given state with
// the provided context. It should be noted that any error
// generated through any of the calls should be considered a
// revert-state-and-consume-all-gas operation, no checks on
// specific errors should ever be performed. The interpreter makes
// sure that any errors generated are to be considered faulty code.
//
// The EVM should never be reused and is not thread safe.
type EVM struct {
	// Context provides auxiliary blockchain related information
	Context
	// StateDB gives access to the underlying state
	StateDB StateDB
	// Depth is the current call stack
	depth int

	// chainConfig contains information about the current chain
	chainConfig *params.ChainConfig
	// chain rules contains the chain rules for the current epoch
	chainRules params.Rules
	// virtual machine configuration options used to initialise the
	// evm.
	vmConfig Config
	// global (to this context) ethereum virtual machine
	// used throughout the execution of the tx.
	interpreter *Interpreter
	// abort is used to abort the EVM calling operations
	// NOTE: must be set atomically
	abort int32
	// callGasTemp holds the gas available for the current call. This is needed because the
	// available gas is calculated in gasCall* according to the 63/64 rule and later
	// applied in opCall*.
	callGasTemp uint64
}

// NewEVM returns a new EVM. The returned EVM is not thread safe and should
// only ever be used *once*.
func NewEVM(ctx Context, statedb StateDB, chainConfig *params.ChainConfig, vmConfig Config) *EVM {
	evm := &EVM{
		Context:     ctx,
		StateDB:     statedb,
		vmConfig:    vmConfig,
		chainConfig: chainConfig,
		chainRules:  chainConfig.Rules(ctx.BlockNumber),
	}

	evm.interpreter = NewInterpreter(evm, vmConfig)
	return evm
}

// Cancel cancels any running EVM operation. This may be called concurrently and
// it's safe to be called multiple times.
func (evm *EVM) Cancel() {
	atomic.StoreInt32(&evm.abort, 1)
}

// Call executes the contract associated with the addr with the given input as
// parameters. It also handles any necessary value transfer required and takes
// the necessary steps to create accounts and reverses the state in case of an
// execution error or failed value transfer.
func (evm *EVM) Call(caller ContractRef, addr common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, err error) {
	if evm.vmConfig.NoRecursion && evm.depth > 0 {
		return nil, gas, nil
	}

	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	// Fail if we're trying to transfer more than the available balance
	if !evm.Context.CanTransfer(evm.StateDB, caller.Address(), value) {
		return nil, gas, ErrInsufficientBalance
	}
	var (
		to       = AccountRef(addr)
		snapshot = evm.StateDB.Snapshot()
	)
	if !evm.StateDB.Exist(addr) {
		precompiles := PrecompiledContractsHomestead
		if evm.ChainConfig().IsByzantium(evm.BlockNumber) {
			precompiles = PrecompiledContractsByzantium
		}
		if precompiles[addr] == nil && evm.ChainConfig().IsEIP158(evm.BlockNumber) && value.Sign() == 0 {
			// Calling a non existing account, don't do antything, but ping the tracer
			if evm.vmConfig.Debug && evm.depth == 0 {
				evm.vmConfig.Tracer.CaptureStart(caller.Address(), addr, false, input, gas, value)
				evm.vmConfig.Tracer.CaptureEnd(ret, 0, 0, nil)
			}
			return nil, gas, nil
		}
		evm.StateDB.CreateAccount(addr)
	}

	//If transactions are special, they are treated separately according to their types.
	if to.Address() == common.SpecialSyncAddress {
		err := dispatchHandler(evm, caller.Address(), input)
		if err != nil {
			return nil, gas, err
		}

		OfficialAddress := common.HexToAddress(evm.chainConfig.Genaro.OfficialAddress)
		evm.Transfer(evm.StateDB, caller.Address(), OfficialAddress, value)
	} else {
		evm.Transfer(evm.StateDB, caller.Address(), to.Address(), value)
	}

	// Initialise a new contract and set the code that is to be used by the EVM.
	// The contract is a scoped environment for this execution context only.
	contract := NewContract(caller, to, value, gas)
	contract.SetCallCode(&addr, evm.StateDB.GetCodeHash(addr), evm.StateDB.GetCode(addr))

	start := time.Now()

	// Capture the tracer start/end events in debug mode
	if evm.vmConfig.Debug && evm.depth == 0 {
		evm.vmConfig.Tracer.CaptureStart(caller.Address(), addr, false, input, gas, value)

		defer func() { // Lazy evaluation of the parameters
			evm.vmConfig.Tracer.CaptureEnd(ret, gas-contract.Gas, time.Since(start), err)
		}()
	}
	ret, err = run(evm, contract, input)

	// When an error was returned by the EVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// when we're in homestead this also counts for code storage gas errors.
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != errExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, err
}

func dispatchHandler(evm *EVM, caller common.Address, input []byte) error {
	var err error
	var s types.SpecialTxInput
	err = json.Unmarshal(input, &s)
	if err != nil {
		return errors.New("special tx error： the extraData parameters of the wrong format")
	}
	switch s.Type.ToInt().Uint64() {
	case common.SpecialTxTypeStakeSync.Uint64():
		err = updateStake(evm, s, caller)
	case common.SpecialTxTypeHeftSync.Uint64():
		err = updateHeft(evm, s, caller)
	case common.SpecialTxTypeSpaceApply.Uint64():
		err = updateStorageProperties(evm, s, caller)
	case common.SpecialTxBucketSupplement.Uint64():
		err = bucketSupplement(evm, s, caller)
	//case common.SpecialTxTypeMortgageInit.Uint64():
	//	err = specialTxTypeMortgageInit(evm, s, caller)
	//case common.SpecialTxTypeSyncSidechainStatus.Uint64():
	//	err = SpecialTxTypeSyncSidechainStatus(evm, s, caller)
	case common.SpecialTxTypeTrafficApply.Uint64():
		err = updateTraffic(evm, s, caller)
	case common.SpecialTxTypeSyncNode.Uint64():
		err = updateStakeNode(evm, s, caller)
	case common.SynchronizeShareKey.Uint64():
		err = SynchronizeShareKey(evm, s, caller)
	case common.SpecialTxTypeSyncFielSharePublicKey.Uint64():
		err = updateFileShareSecretKey(evm, s, caller)
	case common.UnlockSharedKey.Uint64():
		err = UnlockSharedKey(evm, s, caller)
	case common.SpecialTxTypePunishment.Uint64():
		err = userPunishment(evm, s, caller)
	case common.SpecialTxTypeBackStake.Uint64():
		err = userBackStake(evm, caller)
	case common.SpecialTxTypePriceRegulation.Uint64():
		err = genaroPriceRegulation(evm, s, caller)
	case common.SpecialTxSynState.Uint64():
		err = SynState(evm, s, caller)
	case common.SpecialTxUnbindNode.Uint64():
		err = unbindNode(evm, s, caller)
	case common.SpecialTxAccountBinding.Uint64():
		err = accountBinding(evm, s, caller)
	case common.SpecialTxAccountCancelBinding.Uint64():
		err = accountCancelBinding(evm, s, caller)
	case common.SpecialTxAddAccountInForbidBackStakeList.Uint64():
		err = addAccountInForbidBackStakeList(evm, s, caller)
	case common.SpecialTxDelAccountInForbidBackStakeList.Uint64():
		err = delAccountInForbidBackStakeList(evm, s, caller)
	case common.SpecialTxSetGlobalVar.Uint64():
		err = setGlobalVar(evm, s, caller)
	case common.SpecialTxAddCoinpool.Uint64():
		err = addCoinpool(evm, s, caller)
	case common.SpecialTxRegisterName.Uint64():
		err = registerName(evm, s, caller)
	case common.SpecialTxTransferName.Uint64():
		err = transferNameTxStatus(evm, s, caller)
	case common.SpecialTxUnsubscribeName.Uint64():
		err = unsubscribeNameTxStatus(evm, s, caller)
	case common.SpecialTxRevoke.Uint64():
		err = revokePromissoryNotesTx(evm, s, caller)
	case common.SpecialTxWithdrawCash.Uint64():
		err = PromissoryNotesWithdrawCash(evm, caller)
	case common.SpecialTxPublishOption.Uint64():
		err = publishOption(evm, s, caller)
	case common.SpecialTxSetOptionTxStatus.Uint64():
		err = setOptionTxStatus(evm, s, caller)
	case common.SpecialTxBuyPromissoryNotes.Uint64():
		err = buyPromissoryNotes(evm, s, caller)
	case common.SpecialTxCarriedOutPromissoryNotes.Uint64():
		err = CarriedOutPromissoryNotes(evm, s, caller)
	case common.SpecialTxTurnBuyPromissoryNotes.Uint64():
		err = turnBuyPromissoryNotes(evm, s, caller)
	case common.SpecialTxSetProfitAccount.Uint64():
		err = setProfitAccount(evm, s, caller)
	case common.SpecialTxSetShadowAccount.Uint64():
		err = setShadowAccount(evm, s, caller)
	default:
		err = errors.New("undefined type of special transaction")
	}

	if err != nil && common.SpecialTxSynState.Uint64() != s.Type.ToInt().Uint64() {
		log.Info(fmt.Sprintf("special transaction error: %s", err))
		log.Info(fmt.Sprintf("special transaction param：%s", string(input)))
	}
	return err
}

func setShadowAccount(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckSetShadowAccount(caller, s, (*evm).StateDB); err != nil {
		return err
	}
	shadowAccount := common.HexToAddress(s.Address)
	ok := (*evm).StateDB.SetShadowAccount(caller, shadowAccount)
	if !ok {
		return errors.New("Set Shadow Account failed")
	}
	return nil
}

func setProfitAccount(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckSetProfitAccount(caller, s, (*evm).StateDB); err != nil {
		return err
	}
	profitAccount := common.HexToAddress(s.Address)
	ok := (*evm).StateDB.SetProfitAccount(caller, profitAccount)
	if !ok {
		return errors.New("Set Profit Account failed")
	}
	return nil
}

func registerName(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckSetNameTxStatus(caller, s, (*evm).StateDB); err != nil {
		return err
	}

	err := (*evm).StateDB.SetNameAccount(s.Message, caller)
	if err != nil {
		return err
	}

	var name types.AccountName
	name.SetString(s.Message)
	priceBig := name.GetBigPrice()

	(*evm).StateDB.SubBalance(caller, priceBig)
	OfficialAddress := common.HexToAddress(evm.chainConfig.Genaro.OfficialAddress)
	(*evm).StateDB.AddBalance(OfficialAddress, priceBig)

	return nil
}

func transferNameTxStatus(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckTransferNameTxStatus(caller, s, (*evm).StateDB); err != nil {
		return err
	}

	transferTarget := common.HexToAddress(s.Address)
	err := (*evm).StateDB.SetNameAccount(s.Message, transferTarget)
	if err != nil {
		return err
	}

	return nil
}

func unsubscribeNameTxStatus(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckUnsubscribeNameTxStatus(caller, s, (*evm).StateDB); err != nil {
		return err
	}

	err := (*evm).StateDB.SetNameAccount(s.Message, common.Address{})
	if err != nil {
		return err
	}

	return nil
}

func setOptionTxStatus(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckSetOptionTxStatus(caller, s, (*evm).StateDB, (*evm).chainConfig.Genaro.OptionTxMemorySize); err != nil {
		return err
	}

	optionTxMemorySize := (*evm).chainConfig.Genaro.OptionTxMemorySize

	(*evm).StateDB.SetTxStatusInOptionTxTable(s.OrderId, s.IsSell, optionTxMemorySize)

	return nil
}

func publishOption(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckPublishOption(caller, s, (*evm).StateDB, (*evm).BlockNumber); err != nil {
		return err
	}
	var promissoryNote types.PromissoryNote
	promissoryNote.RestoreBlock = s.RestoreBlock
	promissoryNote.Num = s.TxNum

	optionHash := types.GenOptionTxHash(caller, (*evm).StateDB.GetNonce(caller))
	optionTxMemorySize := (*evm).chainConfig.Genaro.OptionTxMemorySize

	if (*evm).StateDB.DelPromissoryNote(caller, promissoryNote) {
		var promissoryNotesOptionTx types.PromissoryNotesOptionTx
		promissoryNotesOptionTx.TxNum = s.TxNum
		promissoryNotesOptionTx.RestoreBlock = s.RestoreBlock
		promissoryNotesOptionTx.PromissoryNotesOwner = caller
		promissoryNotesOptionTx.IsSell = true
		promissoryNotesOptionTx.PromissoryNoteTxPrice = s.PromissoryNoteTxPrice.ToInt()
		promissoryNotesOptionTx.OptionPrice = s.OptionPrice.ToInt()
		(*evm).StateDB.AddTxInOptionTxTable(optionHash, promissoryNotesOptionTx, optionTxMemorySize)
	}

	return nil
}

func revokePromissoryNotesTx(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckPromissoryNoteRevoke(caller, s, (*evm).StateDB, (*evm).BlockNumber, (*evm).chainConfig.Genaro.OptionTxMemorySize); err != nil {
		return err
	}

	optionTxMemorySize := (*evm).chainConfig.Genaro.OptionTxMemorySize

	optionTxTable := (*evm).StateDB.GetOptionTxTable(s.OrderId, optionTxMemorySize)
	promissoryNotesOptionTx := (*optionTxTable)[s.OrderId]

	if (*evm).StateDB.DelTxInOptionTxTable(s.OrderId, optionTxMemorySize) {
		var promissoryNote types.PromissoryNote
		promissoryNote.Num = promissoryNotesOptionTx.TxNum
		promissoryNote.RestoreBlock = promissoryNotesOptionTx.RestoreBlock
		(*evm).StateDB.AddPromissoryNote(caller, promissoryNote)
	}

	return nil
}

func delAccountInForbidBackStakeList(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckDelAccountInForbidBackStakeListTx(caller, s, (*evm).StateDB, evm.chainConfig.Genaro); err != nil {
		return err
	}
	account := common.HexToAddress(s.Address)
	ok := (*evm).StateDB.DelAccountInForbidBackStakeList(account)
	if !ok {
		return errors.New("Delete Account In Forbid BackStake List failed")
	}
	return nil
}

func addAccountInForbidBackStakeList(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckAddAccountInForbidBackStakeListTx(caller, s, (*evm).StateDB, evm.chainConfig.Genaro); err != nil {
		return err
	}
	account := common.HexToAddress(s.Address)
	ok := (*evm).StateDB.AddAccountInForbidBackStakeList(account)
	if !ok {
		return errors.New("Add Account In Forbid BackStake List failed")
	}
	return nil
}

func unbindNode(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	existNodes := (*evm).StateDB.GetStorageNodes(caller)
	if err := CheckUnbindNodeTx(caller, s, existNodes); err != nil {
		return err
	}

	var err error = nil
	err = (*evm).StateDB.UnbindNode(caller, s.NodeID)

	if err == nil {
		node2UserAccountIndexAddress := common.StakeNode2StakeAddress
		(*evm).StateDB.UbindNode2Address(node2UserAccountIndexAddress, s.NodeID)
	}

	return nil
}

func accountBinding(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	err := CheckAccountBindingTx(caller, s, (*evm).StateDB)
	if err != nil {
		return err
	}

	mainAddr := common.HexToAddress(s.Address)
	subAddr := common.HexToAddress(s.Message)
	if !(*evm).StateDB.UpdateAccountBinding(mainAddr, subAddr) {
		return errors.New("binding failed")
	}

	if !(*evm).StateDB.DelCandidate(subAddr) {
		return errors.New("DelCandidate failed")
	}

	return nil
}

func accountCancelBinding(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	t, err := CheckAccountCancelBindingTx(caller, s, (*evm).StateDB)
	if err != nil {
		return err
	}

	switch t {
	case 1:
		subAccounts := (*evm).StateDB.DelMainAccountBinding(caller)
		for _, subAccount := range subAccounts {
			(*evm).StateDB.AddCandidate(subAccount)
		}
	case 2:
		ok := (*evm).StateDB.DelSubAccountBinding(caller)
		if ok {
			(*evm).StateDB.AddCandidate(caller)
		}
	case 3:
		subAddr := common.HexToAddress(s.Address)
		ok := (*evm).StateDB.DelSubAccountBinding(subAddr)
		if ok {
			(*evm).StateDB.AddCandidate(subAddr)
		}
	default:
		return errors.New("Account Cancel Binding failed")
	}
	return nil
}

func genaroPriceRegulation(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckPriceRegulation(caller, s); err != nil {
		return err
	}

	if caller != common.GenaroPriceAddress {
		return errors.New("caller address of this transaction is not invalid")
	}

	if s.StakeValuePerNode != nil {
		if ok := (*evm).StateDB.UpdateStakePerNodePrice(caller, s.StakeValuePerNode); !ok {
			return errors.New("update the price of stakePerNode fail")
		}
	}

	if s.BucketApplyGasPerGPerDay != nil {
		if ok := (*evm).StateDB.UpdateBucketApplyPrice(caller, s.BucketApplyGasPerGPerDay); !ok {
			return errors.New("update the price of bucketApply fail")
		}
	}

	if s.TrafficApplyGasPerG != nil {
		if ok := (*evm).StateDB.UpdateTrafficApplyPrice(caller, s.TrafficApplyGasPerG); !ok {
			return errors.New("update the price of trafficApply fail")
		}
	}

	if s.OneDayMortgageGes != nil {
		if ok := (*evm).StateDB.UpdateOneDayGesCost(caller, s.OneDayMortgageGes); !ok {
			return errors.New("update the price of OneDayGesCost fail")
		}
	}

	if s.OneDaySyncLogGsaCost != nil {
		if ok := (*evm).StateDB.UpdateOneDaySyncLogGsaCost(caller, s.OneDaySyncLogGsaCost); !ok {
			return errors.New("update the price of OneDaySyncLogGsaCost fail")
		}
	}

	return nil
}

func addCoinpool(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckAddCoinpool(caller, s, (*evm).StateDB); err != nil {
		return err
	}
	rewardsValues := (*evm).StateDB.GetRewardsValues()
	rewardsValues.SurplusCoin.Add(rewardsValues.SurplusCoin, s.AddCoin.ToInt())
	ok := (*evm).StateDB.SetRewardsValues(*rewardsValues)
	if ok {
		(*evm).StateDB.SubBalance(caller, s.AddCoin.ToInt())
		return nil
	}
	return errors.New("addCoinpool fail")
}

func setGlobalVar(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckSetGlobalVar(caller, s, evm.chainConfig.Genaro); err != nil {
		return err
	}
	genaroPrice := (*evm).StateDB.GetGenaroPrice()
	if s.StorageRewardsRatio != 0 {
		genaroPrice.StorageRewardsRatio = s.StorageRewardsRatio
	}
	if s.CoinRewardsRatio != 0 {
		genaroPrice.CoinRewardsRatio = s.CoinRewardsRatio
	}
	if s.RatioPerYear != 0 {
		genaroPrice.RatioPerYear = s.RatioPerYear
	}
	if s.BackStackListMax != 0 {
		genaroPrice.BackStackListMax = s.BackStackListMax
	}
	if s.CommitteeMinStake != 0 {
		genaroPrice.CommitteeMinStake = s.CommitteeMinStake
	}
	if s.MinStake != 0 {
		genaroPrice.MinStake = s.MinStake
	}
	if s.MaxBinding != 0 {
		genaroPrice.MaxBinding = s.MaxBinding
	}
	if len(s.SynStateAccount) > 0 {
		genaroPrice.SynStateAccount = s.SynStateAccount
	}

	if len(s.HeftAccount) > 0 {
		genaroPrice.HeftAccount = s.HeftAccount
	}

	if len(s.BindingAccount) > 0 {
		genaroPrice.BindingAccount = s.BindingAccount
	}

	ok := (*evm).StateDB.SetGenaroPrice(*genaroPrice)
	if !ok {
		return errors.New("setGlobalVar fail")
	}
	return nil
}

func SynState(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	err := CheckSynStateTx(caller, (*evm).StateDB)
	if err != nil {
		return err
	}
	lastSynState := (*evm).StateDB.GetLastSynState()
	blockHash := common.HexToHash(s.Message)
	blockNum, ok := lastSynState.LastRootStates[blockHash]
	if ok {
		(*evm).StateDB.SetLastSynBlock(blockNum, blockHash)
		return nil
	} else {
		return errors.New("SynState fail")
	}
}

func userBackStake(evm *EVM, caller common.Address) error {
	err := CheckBackStakeTx(caller, (*evm).StateDB)
	if err != nil {
		return err
	}

	var backStake = common.AlreadyBackStake{
		Addr:            caller,
		BackBlockNumber: evm.BlockNumber.Uint64(),
	}
	ok := (*evm).StateDB.AddAlreadyBackStack(backStake)
	if !ok {
		return errors.New("userBackStake fail")
	}
	ok = (*evm).StateDB.DelCandidate(caller)
	if !ok {
		return errors.New("DelCandidate fail")
	}
	return nil
}

func userPunishment(evm *EVM, s types.SpecialTxInput, caller common.Address) error {

	if err := CheckPunishmentTx(caller, s, evm.StateDB, evm.chainConfig.Genaro); err != nil {
		return err
	}
	adress := common.HexToAddress(s.Address)
	var actualPunishment uint64
	var ok bool
	if ok, actualPunishment = (*evm).StateDB.DeleteStake(adress, s.Stake, evm.BlockNumber.Uint64()); !ok {
		return errors.New("delete user's stake fail")
	}
	amount := new(big.Int).Mul(common.BaseCompany, new(big.Int).SetUint64(actualPunishment))
	OfficialAddress := common.HexToAddress(evm.chainConfig.Genaro.OfficialAddress)
	(*evm).StateDB.AddBalance(OfficialAddress, amount)
	return nil
}

func UnlockSharedKey(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckUnlockSharedKeyParameter(s, (*evm).StateDB, caller); nil != err {
		return err
	}
	if !(*evm).StateDB.UnlockSharedKey(caller, s.SynchronizeShareKey.ShareKeyId) {
		return errors.New("update  chain UnlockSharedKey fail")
	}
	return nil
}

func SynchronizeShareKey(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckSynchronizeShareKeyParameter(s, evm.StateDB, evm.chainConfig.Genaro); err != nil {
		return err
	}
	s.SynchronizeShareKey.Status = 0
	s.SynchronizeShareKey.FromAccount = caller
	if !(*evm).StateDB.SynchronizeShareKey(s.SynchronizeShareKey.RecipientAddress, s.SynchronizeShareKey) {
		return errors.New("update  chain SynchronizeShareKey fail")
	}
	return nil
}

func updateFileShareSecretKey(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckSyncFileSharePublicKeyTx(s, evm.StateDB, evm.chainConfig.Genaro); nil != err {
		return err
	}
	adress := common.HexToAddress(s.Address)
	if !(*evm).StateDB.UpdateFileSharePublicKey(adress, s.FileSharePublicKey) {
		return errors.New("update user's public key fail")
	}
	return nil
}

func updateStakeNode(evm *EVM, s types.SpecialTxInput, caller common.Address) error {

	if err := CheckSyncNodeTx(caller, s, (*evm).StateDB); nil != err {
		return err
	}

	var err error = nil
	err = (*evm).StateDB.SyncStakeNode(common.HexToAddress(s.Address), s.NodeID)

	if err == nil {
		node2UserAccountIndexAddress := common.StakeNode2StakeAddress
		(*evm).StateDB.SyncNode2Address(node2UserAccountIndexAddress, s.NodeID, common.HexToAddress(s.Address).String())
	}

	return err
}

func SpecialTxTypeSyncSidechainStatus(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckSpecialTxTypeSyncSidechainStatusParameter(s, caller, evm.StateDB, evm.chainConfig.Genaro); nil != err {
		return err
	}

	restlt, flag := (*evm).StateDB.SpecialTxTypeSyncSidechainStatus(s.SpecialTxTypeMortgageInit.FromAccount, s.SpecialTxTypeMortgageInit)
	if false == flag {
		return errors.New("update cross chain SpecialTxTypeMortgageInit fail")
	}
	for k, v := range restlt {
		(*evm).StateDB.AddBalance(k, v)
	}
	return nil
}

func specialTxTypeMortgageInit(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckspecialTxTypeMortgageInitParameter(s, caller); nil != err {
		return errors.New("update  chain SpecialTxTypeMortgageInit fail")
	}
	sumMortgageTable := new(big.Int)
	mortgageTable := s.SpecialTxTypeMortgageInit.MortgageTable
	if len(mortgageTable) > 8 {
		return errors.New("update  chain SpecialTxTypeMortgageInit fail")
	}
	zero := big.NewInt(0)
	for _, v := range mortgageTable {
		if v.ToInt().Cmp(zero) < 0 {
			return errors.New("update  chain SpecialTxTypeMortgageInit fail")
		}
		sumMortgageTable = sumMortgageTable.Add(sumMortgageTable, v.ToInt())
	}
	s.SpecialTxTypeMortgageInit.MortgagTotal = sumMortgageTable
	if !(*evm).StateDB.SpecialTxTypeMortgageInit(caller, s.SpecialTxTypeMortgageInit) {
		return errors.New("update  chain SpecialTxTypeMortgageInit fail")
	}
	if s.SpecialTxTypeMortgageInit.TimeLimit.ToInt().Cmp(zero) < 0 {
		return errors.New("update  chain SpecialTxTypeMortgageInit fail")
	}
	temp := s.SpecialTxTypeMortgageInit.TimeLimit.ToInt().Mul(s.SpecialTxTypeMortgageInit.TimeLimit.ToInt(), big.NewInt(int64(len(mortgageTable))))
	timeLimitGas := temp.Mul(temp, (*evm).StateDB.GetOneDayGesCost())

	sumMortgageTable.Add(sumMortgageTable, timeLimitGas)
	(*evm).StateDB.SubBalance(caller, sumMortgageTable)

	OfficialAddress := common.HexToAddress(evm.chainConfig.Genaro.OfficialAddress)
	(*evm).StateDB.AddBalance(OfficialAddress, timeLimitGas)
	return nil
}

func bucketSupplement(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckBucketSupplement(s, (*evm).StateDB, (*evm).chainConfig.Genaro); err != nil {
		return err
	}

	address := common.HexToAddress(s.Address)
	bucketsMap, _ := (*evm).StateDB.GetBuckets(address)
	currentPrice := (*evm).StateDB.GetGenaroPrice()
	currentCost := s.SpecialCost(currentPrice, bucketsMap)
	totalGas := new(big.Int).Set(&currentCost)
	log.Info(fmt.Sprintf("evm bucketSupplement cost:%s", totalGas.String()))

	// Fail if we're trying to use more than the available balance
	if !evm.Context.CanTransfer(evm.StateDB, caller, totalGas) {
		return ErrInsufficientBalance
	}

	var bucket types.BucketPropertie
	b, _ := bucketsMap[s.BucketID]
	bucketInDb := b.(types.BucketPropertie)

	bucket.BucketId = bucketInDb.BucketId
	bucket.Backup = bucketInDb.Backup
	bucket.TimeStart = bucketInDb.TimeStart
	bucket.Size = bucketInDb.Size + s.Size
	bucket.TimeEnd = bucketInDb.TimeEnd + s.Duration

	if (*evm).StateDB.UpdateBucket(address, bucket) {
		(*evm).StateDB.SubBalance(caller, totalGas)
		OfficialAddress := common.HexToAddress(evm.chainConfig.Genaro.OfficialAddress)
		(*evm).StateDB.AddBalance(OfficialAddress, totalGas)
	}
	return nil
}

func updateStorageProperties(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckApplyBucketTx(s, evm.StateDB, (*evm).chainConfig.Genaro); err != nil {
		return err
	}
	adress := common.HexToAddress(s.Address)

	currentPrice := (*evm).StateDB.GetGenaroPrice()
	currentCost := s.SpecialCost(currentPrice, nil)
	totalGas := new(big.Int).Set(&currentCost)
	log.Info(fmt.Sprintf("evm bucketApply cost:%s", totalGas.String()))

	// Fail if we're trying to use more than the available balance
	if !evm.Context.CanTransfer(evm.StateDB, caller, totalGas) {
		return ErrInsufficientBalance
	}

	for _, b := range s.Buckets {
		bucketId := b.BucketId
		if len(bucketId) != 64 {
			return errors.New("the length of bucketId must be 64")
		}

		if b.TimeStart >= b.TimeEnd {
			return errors.New("endTime must larger then startTime")
		}

		if !(*evm).StateDB.UpdateBucketProperties(adress, bucketId, b.Size, b.Backup, b.TimeStart, b.TimeEnd) {
			return errors.New("update user's bucket fail")
		}
	}

	(*evm).StateDB.SubBalance(caller, totalGas)
	OfficialAddress := common.HexToAddress(evm.chainConfig.Genaro.OfficialAddress)
	(*evm).StateDB.AddBalance(OfficialAddress, totalGas)

	return nil
}

func updateHeft(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckSyncHeftTx(caller, s, evm.StateDB, evm.chainConfig.Genaro); err != nil {
		return err
	}

	adress := common.HexToAddress(s.Address)
	if !(evm.StateDB).UpdateHeft(adress, s.Heft, evm.BlockNumber.Uint64()) {
		return errors.New("update user's heft fail")
	}
	return nil
}

func updateTraffic(evm *EVM, s types.SpecialTxInput, caller common.Address) error {

	if err := CheckTrafficTx(s, evm.StateDB, evm.chainConfig.Genaro); err != nil {
		return err
	}

	adress := common.HexToAddress(s.Address)

	currentPrice := (*evm).StateDB.GetGenaroPrice()
	currentCost := s.SpecialCost(currentPrice, nil)
	totalGas := new(big.Int).Set(&currentCost)
	log.Info(fmt.Sprintf("evm trafficApply cost:%s", totalGas.String()))

	// Fail if we're trying to use more than the available balance
	if !evm.Context.CanTransfer(evm.StateDB, caller, totalGas) {
		return ErrInsufficientBalance
	}

	if !(*evm).StateDB.UpdateTraffic(adress, s.Traffic) {
		return errors.New("update user's teraffic fail")
	}

	(*evm).StateDB.SubBalance(caller, totalGas)
	OfficialAddress := common.HexToAddress(evm.chainConfig.Genaro.OfficialAddress)
	(*evm).StateDB.AddBalance(OfficialAddress, totalGas)

	return nil
}

func updateStake(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckStakeTx(s, evm.StateDB, evm.chainConfig.Genaro); err != nil {
		return err
	}

	// the unit of stake is GNX， one stake means one GNX
	currentCost := s.SpecialCost(nil, nil)
	amount := new(big.Int).Set(&currentCost)

	// judge if there is enough balance to stake（balance must larger than stake value)
	if !evm.Context.CanTransfer(evm.StateDB, caller, amount) {
		return ErrInsufficientBalance
	}

	adress := common.HexToAddress(s.Address)
	if !(*evm).StateDB.UpdateStake(adress, s.Stake, evm.BlockNumber.Uint64()) {
		return errors.New("update sentinel's stake fail")

	}
	// 加入候选名单
	if !(*evm).StateDB.AddCandidate(adress) {
		return errors.New("add candidate fail")
	}
	(*evm).StateDB.SubBalance(caller, amount)
	return nil
}

func PromissoryNotesWithdrawCash(evm *EVM, caller common.Address) error {
	blockNumber := evm.BlockNumber.Uint64()
	withdrawCashNum := (*evm).StateDB.PromissoryNotesWithdrawCash(caller, blockNumber)
	if withdrawCashNum <= 0 {
		return errors.New("WithdrawCash error")
	}
	promissoryPrice := big.NewInt(int64(evm.chainConfig.Genaro.PromissoryNotePrice * withdrawCashNum))
	promissoryPrice.Mul(promissoryPrice, common.BaseCompany)
	(*evm).StateDB.AddBalance(caller, promissoryPrice)
	return nil
}

func buyPromissoryNotes(evm *EVM, s types.SpecialTxInput, caller common.Address) error {

	optionTxMemorySize := (*evm).chainConfig.Genaro.OptionTxMemorySize

	result := (*evm).StateDB.BuyPromissoryNotes(s.OrderId, caller, optionTxMemorySize)
	if result.TxNum > 0 {
		//result.OptionPrice.Mul(result.OptionPrice,big.NewInt(int64(result.TxNum)))
		//result.OptionPrice.Mul(result.OptionPrice,common.BaseCompany)
		(*evm).StateDB.AddBalance(result.PromissoryNotesOwner, result.OptionPrice)
		(*evm).StateDB.SubBalance(caller, result.OptionPrice)
	}
	return nil
}

func CarriedOutPromissoryNotes(evm *EVM, s types.SpecialTxInput, caller common.Address) error {

	optionTxMemorySize := (*evm).chainConfig.Genaro.OptionTxMemorySize

	result := (*evm).StateDB.CarriedOutPromissoryNotes(s.OrderId, caller, optionTxMemorySize)
	if result.TxNum > 0 {
		result.PromissoryNoteTxPrice.Mul(result.PromissoryNoteTxPrice, big.NewInt(int64(result.TxNum)))
		//result.PromissoryNoteTxPrice.Mul(result.PromissoryNoteTxPrice,common.BaseCompany)
		(*evm).StateDB.AddBalance(result.PromissoryNotesOwner, result.OptionPrice)
		(*evm).StateDB.SubBalance(caller, result.OptionPrice)
	}
	return nil
}

func turnBuyPromissoryNotes(evm *EVM, s types.SpecialTxInput, caller common.Address) error {

	optionTxMemorySize := (*evm).chainConfig.Genaro.OptionTxMemorySize

	result := (*evm).StateDB.TurnBuyPromissoryNotes(s.OrderId, s.OptionPrice, caller, optionTxMemorySize)
	if false == result {
		errors.New("update error")
	}
	return nil
}

// CallCode executes the contract associated with the addr with the given input
// as parameters. It also handles any necessary value transfer required and takes
// the necessary steps to create accounts and reverses the state in case of an
// execution error or failed value transfer.
//
// CallCode differs from Call in the sense that it executes the given address'
// code with the caller as context.
func (evm *EVM) CallCode(caller ContractRef, addr common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, err error) {
	if evm.vmConfig.NoRecursion && evm.depth > 0 {
		return nil, gas, nil
	}

	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	// Fail if we're trying to transfer more than the available balance
	if !evm.CanTransfer(evm.StateDB, caller.Address(), value) {
		return nil, gas, ErrInsufficientBalance
	}

	var (
		snapshot = evm.StateDB.Snapshot()
		to       = AccountRef(caller.Address())
	)
	// initialise a new contract and set the code that is to be used by the
	// EVM. The contract is a scoped environment for this execution context
	// only.
	contract := NewContract(caller, to, value, gas)
	contract.SetCallCode(&addr, evm.StateDB.GetCodeHash(addr), evm.StateDB.GetCode(addr))

	ret, err = run(evm, contract, input)
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != errExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, err
}

// DelegateCall executes the contract associated with the addr with the given input
// as parameters. It reverses the state in case of an execution error.
//
// DelegateCall differs from CallCode in the sense that it executes the given address'
// code with the caller as context and the caller is set to the caller of the caller.
func (evm *EVM) DelegateCall(caller ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	if evm.vmConfig.NoRecursion && evm.depth > 0 {
		return nil, gas, nil
	}
	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}

	var (
		snapshot = evm.StateDB.Snapshot()
		to       = AccountRef(caller.Address())
	)

	// Initialise a new contract and make initialise the delegate values
	contract := NewContract(caller, to, nil, gas).AsDelegate()
	contract.SetCallCode(&addr, evm.StateDB.GetCodeHash(addr), evm.StateDB.GetCode(addr))

	ret, err = run(evm, contract, input)
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != errExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, err
}

// StaticCall executes the contract associated with the addr with the given input
// as parameters while disallowing any modifications to the state during the call.
// Opcodes that attempt to perform such modifications will result in exceptions
// instead of performing the modifications.
func (evm *EVM) StaticCall(caller ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	if evm.vmConfig.NoRecursion && evm.depth > 0 {
		return nil, gas, nil
	}
	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	// Make sure the readonly is only set if we aren't in readonly yet
	// this makes also sure that the readonly flag isn't removed for
	// child calls.
	if !evm.interpreter.readOnly {
		evm.interpreter.readOnly = true
		defer func() { evm.interpreter.readOnly = false }()
	}

	var (
		to       = AccountRef(addr)
		snapshot = evm.StateDB.Snapshot()
	)
	// Initialise a new contract and set the code that is to be used by the
	// EVM. The contract is a scoped environment for this execution context
	// only.
	contract := NewContract(caller, to, new(big.Int), gas)
	contract.SetCallCode(&addr, evm.StateDB.GetCodeHash(addr), evm.StateDB.GetCode(addr))

	// When an error was returned by the EVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// when we're in Homestead this also counts for code storage gas errors.
	ret, err = run(evm, contract, input)
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != errExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	return ret, contract.Gas, err
}

// Create creates a new contract using code as deployment code.
func (evm *EVM) Create(caller ContractRef, code []byte, gas uint64, value *big.Int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {

	// Depth check execution. Fail if we're trying to execute above the
	// limit.
	if evm.depth > int(params.CallCreateDepth) {
		return nil, common.Address{}, gas, ErrDepth
	}
	if !evm.CanTransfer(evm.StateDB, caller.Address(), value) {
		return nil, common.Address{}, gas, ErrInsufficientBalance
	}
	// Ensure there's no existing contract already at the designated address
	nonce := evm.StateDB.GetNonce(caller.Address())
	evm.StateDB.SetNonce(caller.Address(), nonce+1)

	contractAddr = crypto.CreateAddress(caller.Address(), nonce)
	contractHash := evm.StateDB.GetCodeHash(contractAddr)
	if evm.StateDB.GetNonce(contractAddr) != 0 || (contractHash != (common.Hash{}) && contractHash != emptyCodeHash) {
		return nil, common.Address{}, 0, ErrContractAddressCollision
	}
	// Create a new account on the state
	snapshot := evm.StateDB.Snapshot()
	evm.StateDB.CreateAccount(contractAddr)
	if evm.ChainConfig().IsEIP158(evm.BlockNumber) {
		evm.StateDB.SetNonce(contractAddr, 1)
	}
	evm.Transfer(evm.StateDB, caller.Address(), contractAddr, value)

	// initialise a new contract and set the code that is to be used by the
	// EVM. The contract is a scoped environment for this execution context
	// only.
	contract := NewContract(caller, AccountRef(contractAddr), value, gas)
	contract.SetCallCode(&contractAddr, crypto.Keccak256Hash(code), code)

	if evm.vmConfig.NoRecursion && evm.depth > 0 {
		return nil, contractAddr, gas, nil
	}

	if evm.vmConfig.Debug && evm.depth == 0 {
		evm.vmConfig.Tracer.CaptureStart(caller.Address(), contractAddr, true, code, gas, value)
	}
	start := time.Now()

	ret, err = run(evm, contract, nil)

	// check whether the max code size has been exceeded
	maxCodeSizeExceeded := evm.ChainConfig().IsEIP158(evm.BlockNumber) && len(ret) > params.MaxCodeSize
	// if the contract creation ran successfully and no errors were returned
	// calculate the gas required to store the code. If the code could not
	// be stored due to not enough gas set an error and let it be handled
	// by the error checking condition below.
	if err == nil && !maxCodeSizeExceeded {
		createDataGas := uint64(len(ret)) * params.CreateDataGas
		if contract.UseGas(createDataGas) {
			evm.StateDB.SetCode(contractAddr, ret)
		} else {
			err = ErrCodeStoreOutOfGas
		}
	}

	// When an error was returned by the EVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// when we're in homestead this also counts for code storage gas errors.
	if maxCodeSizeExceeded || (err != nil && (evm.ChainConfig().IsHomestead(evm.BlockNumber) || err != ErrCodeStoreOutOfGas)) {
		evm.StateDB.RevertToSnapshot(snapshot)
		if err != errExecutionReverted {
			contract.UseGas(contract.Gas)
		}
	}
	// Assign err if contract code size exceeds the max while the err is still empty.
	if maxCodeSizeExceeded && err == nil {
		err = errMaxCodeSizeExceeded
	}
	if evm.vmConfig.Debug && evm.depth == 0 {
		evm.vmConfig.Tracer.CaptureEnd(ret, gas-contract.Gas, time.Since(start), err)
	}
	return ret, contractAddr, contract.Gas, err
}

// ChainConfig returns the environment's chain configuration
func (evm *EVM) ChainConfig() *params.ChainConfig { return evm.chainConfig }

// Interpreter returns the EVM interpreter
func (evm *EVM) Interpreter() *Interpreter { return evm.interpreter }
