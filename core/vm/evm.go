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
	"math/big"
	"sync/atomic"
	"time"
	"encoding/json"
	"errors"


	"github.com/GenaroNetwork/Genaro-Core/common"
	"github.com/GenaroNetwork/Genaro-Core/crypto"
	"github.com/GenaroNetwork/Genaro-Core/params"
	"github.com/GenaroNetwork/Genaro-Core/core/types"
	"github.com/GenaroNetwork/Genaro-Core/log"
)

// emptyCodeHash is used by create to ensure deployment is disallowed to already
// deployed contract addresses (relevant after the account abstraction).
var emptyCodeHash = crypto.Keccak256Hash(nil)

type (
	CanTransferFunc func(StateDB, common.Address, *big.Int) bool
	TransferFunc    func(StateDB, common.Address, common.Address, *big.Int)
	// GetHashFunc returns the nth block hash in the blockchain
	// and is used by the BLOCKHASH EVM op code.
	GetHashFunc func(uint64) common.Hash
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

		//如果是特殊交易，往官方账号转账
		evm.Transfer(evm.StateDB, caller.Address(), common.OfficialAddress, value)
	}else {
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




func dispatchHandler(evm *EVM, caller common.Address, input []byte) error{
	var err error
	// 解析数据
	var s types.SpecialTxInput
	err = json.Unmarshal(input, &s)
	if err != nil{
		return errors.New("special tx error： the extraData parameters of the wrong format")
	}
	switch s.Type.ToInt().Uint64(){
	case common.SpecialTxTypeStakeSync.Uint64(): // 同步stake
		err = updateStake(evm, s, caller)
	case common.SpecialTxTypeHeftSync.Uint64(): // 同步heft
		err = updateHeft(&evm.StateDB, s, evm.BlockNumber.Uint64(), caller)
	case common.SpecialTxTypeSpaceApply.Uint64(): // 申请存储空间
		err = updateStorageProperties(evm, s, caller)
	case common.SpecialTxTypeMortgageInit.Uint64(): // 交易代表用户押注初始化交易
		err = specialTxTypeMortgageInit(evm, s,caller)
	case common.SpecialTxTypeSyncSidechainStatus.Uint64(): //同步日志+结算
		err = SpecialTxTypeSyncSidechainStatus(evm, s, caller)
	case common.SpecialTxTypeTrafficApply.Uint64(): //用户申购流量
		err = updateTraffic(evm, s, caller)
	case common.SpecialTxTypeSyncNode.Uint64(): //用户stake后同步节点Id
		err = updateStakeNode(evm, s, caller)
	case common.SynchronizeShareKey.Uint64():
		err = SynchronizeShareKey(evm, s, caller)
	case common.SpecialTxTypeSyncFielSharePublicKey.Uint64(): // 用户同步自己文件分享的publicKey到链上
		err = updateFileShareSecretKey(evm, s, caller)
	case common.UnlockSharedKey.Uint64():
		err = UnlockSharedKey(evm, s, caller)
	case common.SpecialTxTypePunishment.Uint64(): // 用户恶意行为后的惩罚措施
		err = userPunishment(evm, s, caller)
	case common.SpecialTxTypeBackStake.Uint64():
		err = userBackStake(evm, caller)
	case common.SpecialTxTypePriceRegulation.Uint64(): //价格调整
		err = genaroPriceRegulation(evm, s, caller)
	case common.SpecialTxSynState.Uint64():
		err = SynState(evm, s, caller)
	default:
		err = errors.New("undefined type of special transaction")
	}

	if err != nil{
		log.Info("special transaction error: ", err)
	}
	return err
}

func genaroPriceRegulation(evm *EVM, s types.SpecialTxInput, caller common.Address) error{
	if err := CheckPriceRegulation(caller); err != nil {
		return err
	}

	var flag = false
	if caller !=  common.GenaroPriceAddress {
		return errors.New("caller address of this transaction is not invalid")
	}

	if s.StakeValuePerNode != nil {
		if ok := (*evm).StateDB.UpdateStakePerNodePrice(caller, s.StakeValuePerNode); !ok {
			return errors.New("update the price of stakePerNode fail")
		}
		flag = true
	}

	if s.BucketApplyGasPerGPerDay != nil {
		if ok := (*evm).StateDB.UpdateBucketApplyPrice(caller, s.BucketApplyGasPerGPerDay); !ok {
			return errors.New("update the price of bucketApply fail")
		}
		flag = true
	}

	if s.TrafficApplyGasPerG != nil {
		if ok := (*evm).StateDB.UpdateTrafficApplyPrice(caller, s.TrafficApplyGasPerG); !ok {
			return errors.New("update the price of trafficApply fail")
		}
		flag = true
	}

	if s.OneDayMortgageGes != nil {
		if ok := (*evm).StateDB.UpdateOneDayGesCost(caller, s.OneDayMortgageGes); !ok {
			return errors.New("update the price of OneDayGesCost fail")
		}
		flag = true
	}

	if s.OneDaySyncLogGsaCost != nil {
		if ok := (*evm).StateDB.UpdateOneDaySyncLogGsaCost(caller, s.OneDaySyncLogGsaCost); !ok {
			return errors.New("update the price of OneDaySyncLogGsaCost fail")
		}
		flag = true
	}

	if !flag {
		return errors.New("none parice to update")
	}

	return nil
}

func SynState(evm *EVM, s types.SpecialTxInput,caller common.Address) error {
	lastSynState := (*evm).StateDB.GetLastSynState()
	stateHash := common.StringToHash(s.Message)
	blockNum,ok := lastSynState.LastRootStates[stateHash]
	if ok {
		lastSynState.LastSynBlockNum = blockNum
		return nil
	} else {
		return errors.New("SynState fail")
	}
}

func userBackStake(evm *EVM, caller common.Address) error {
	ok,backStakeList := (*evm).StateDB.GetAlreadyBackStakeList()
	if !ok {
		return errors.New("userBackStake fail")
	}
	if len(backStakeList) > common.BackStackListMax {
		return errors.New("BackStackList too long")
	}
	var backStake = common.AlreadyBackStake{
		Addr: caller,
		BackBlockNumber:evm.BlockNumber.Uint64(),
	}
	ok = (*evm).StateDB.AddAlreadyBackStack(backStake)
	if !ok {
		return errors.New("userBackStake fail")
	}
	return nil
}

func userPunishment(evm *EVM, s types.SpecialTxInput,caller common.Address) error {

	if err := CheckPunishmentTx(caller,s); err != nil  {
		return err
	}
	adress := common.HexToAddress(s.NodeId)
	var actualPunishment uint64
	var ok bool
	// 根据nodeid扣除对应用户的stake
	if ok, actualPunishment = (*evm).StateDB.DeleteStake(adress, s.Stake, evm.BlockNumber.Uint64()); !ok {
		return errors.New("delete user's stake fail")
	}
	amount := new(big.Int)
	amount.SetUint64(actualPunishment*1000000000000000000)
	//将实际扣除的钱转到官方账号中
	(*evm).StateDB.AddBalance(common.OfficialAddress, amount)
	return nil
}

func UnlockSharedKey(evm *EVM, s types.SpecialTxInput,caller common.Address) error {
	if err := CheckUnlockSharedKeyParameter(s); nil != err {
		return err
	}
	if !(*evm).StateDB.UnlockSharedKey(caller,s.SynchronizeShareKey.ShareKeyId) {
		return errors.New("update  chain UnlockSharedKey fail")
	}
	return nil
}

func SynchronizeShareKey(evm *EVM, s types.SpecialTxInput,caller common.Address) error {
	if err := CheckSynchronizeShareKeyParameter(s); err != nil  {
		return err
	}
	s.SynchronizeShareKey.Status = 0
	s.SynchronizeShareKey.FromAccount = caller
	if !(*evm).StateDB.SynchronizeShareKey(s.SynchronizeShareKey.RecipientAddress,s.SynchronizeShareKey) {
		return errors.New("update  chain SynchronizeShareKey fail")
	}
	return nil
}

func updateFileShareSecretKey(evm *EVM, s types.SpecialTxInput,caller common.Address) error {
	if err := CheckSyncFileSharePublicKeyTx(s); nil != err  {
		return err
	}
	adress := common.HexToAddress(s.NodeId)
	if !(*evm).StateDB.UpdateFileSharePublicKey(adress, s.FileSharePublicKey) {
		return errors.New("update user's public key fail")
	}
	return nil
}

func updateStakeNode(evm *EVM, s types.SpecialTxInput,caller common.Address) error {
	var err error = nil
	if s.Node != nil && len(s.Node) != 0 {
		err = (*evm).StateDB.SyncStakeNode(caller, s.Node)

		if err == nil { // 存储倒排索引
			node2UserAccountIndexAddress := common.StakeNode2StakeAddress
			(*evm).StateDB.SyncNode2Address(node2UserAccountIndexAddress, s.Node, caller.String())
		}
	}

	return err
}

func SpecialTxTypeSyncSidechainStatus(evm *EVM, s types.SpecialTxInput, caller common.Address) error  {
	if err := CheckSpecialTxTypeSyncSidechainStatusParameter(s, caller); nil != err {
		return err
	}

	restlt,flag := (*evm).StateDB.SpecialTxTypeSyncSidechainStatus(s.SpecialTxTypeMortgageInit.FromAccount,s.SpecialTxTypeMortgageInit)
	if  false == flag{
		return errors.New("update cross chain SpecialTxTypeMortgageInit fail")
	}
	for k,v := range restlt {
		(*evm).StateDB.AddBalance(k, v)
	}
	return nil
}

func specialTxTypeMortgageInit(evm *EVM, s types.SpecialTxInput,caller common.Address) error{
	if err := CheckspecialTxTypeMortgageInitParameter(s,caller); nil != err {
		return errors.New("update  chain SpecialTxTypeMortgageInit fail")
	}
	sumMortgageTable :=	new(big.Int)
	mortgageTable := s.SpecialTxTypeMortgageInit.MortgageTable
	if len(mortgageTable) > 8 {
		return errors.New("update  chain SpecialTxTypeMortgageInit fail")
	}
	zero := big.NewInt(0)
	for _, v := range mortgageTable{
		if v.ToInt().Cmp(zero) < 0 {
			return errors.New("update  chain SpecialTxTypeMortgageInit fail")
		}
		sumMortgageTable = sumMortgageTable.Add(sumMortgageTable,v.ToInt())
	}
	s.SpecialTxTypeMortgageInit.MortgagTotal = sumMortgageTable
	if !(*evm).StateDB.SpecialTxTypeMortgageInit(caller,s.SpecialTxTypeMortgageInit) {
		return errors.New("update  chain SpecialTxTypeMortgageInit fail")
	}
	if s.SpecialTxTypeMortgageInit.TimeLimit.ToInt().Cmp(zero) < 0 {
		return errors.New("update  chain SpecialTxTypeMortgageInit fail")
	}
	temp := s.SpecialTxTypeMortgageInit.TimeLimit.ToInt().Mul(s.SpecialTxTypeMortgageInit.TimeLimit.ToInt(),big.NewInt(int64(len(mortgageTable))))
	timeLimitGas := temp.Mul(temp,(*evm).StateDB.GetOneDayGesCost())
	//timeLimitGas = (*big.Int)()s.SpecialTxTypeMortgageInit.TimeLimit *
	//扣除抵押表全部费用+按照时间期限收费
	sumMortgageTable.Add(sumMortgageTable,timeLimitGas)
	(*evm).StateDB.SubBalance(caller, sumMortgageTable)
	//时间期限收取的费用转账到官方账号
	(*evm).StateDB.AddBalance(common.OfficialAddress, timeLimitGas)
	return nil
}

func updateStorageProperties(evm *EVM, s types.SpecialTxInput,caller common.Address) error {
	adress := common.HexToAddress(s.NodeId)

	currentPrice := (*evm).StateDB.GetGenaroPrice()
	totalGas := s.SpecialCost(currentPrice)

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

		// 根据nodeid更新storage属性
		if !(*evm).StateDB.UpdateBucketProperties(adress, bucketId, b.Size, b.Backup, b.TimeStart, b.TimeEnd) {
			return errors.New("update user's bucket fail")
		}
	}

	//扣除费用
	(*evm).StateDB.SubBalance(caller, totalGas)
	(*evm).StateDB.AddBalance(common.OfficialAddress, totalGas)

	return nil
}


func updateHeft(statedb *StateDB, s types.SpecialTxInput, blockNumber uint64, caller common.Address) error {
	if err := CheckSyncHeftTx(caller, s); err != nil {
		return err
	}

	adress := common.HexToAddress(s.NodeId)
	// 根据nodeid更新heft值
	if !(*statedb).UpdateHeft(adress, s.Heft, blockNumber) {
		return errors.New("update user's heft fail")
	}
	return nil
}

func updateTraffic(evm *EVM, s types.SpecialTxInput,caller common.Address) error {

	if err := CheckTrafficTx(s); err != nil {
		return err
	}

	adress := common.HexToAddress(s.NodeId)

	currentPrice := (*evm).StateDB.GetGenaroPrice()
	totalGas := s.SpecialCost(currentPrice)

	// Fail if we're trying to use more than the available balance
	if !evm.Context.CanTransfer(evm.StateDB, caller, totalGas) {
		return ErrInsufficientBalance
	}

	// 根据nodeid更新heft值
	if !(*evm).StateDB.UpdateTraffic(adress, s.Traffic) {
		return errors.New("update user's teraffic fail")
	}

	(*evm).StateDB.SubBalance(caller, totalGas)
	(*evm).StateDB.AddBalance(common.OfficialAddress, totalGas)

	return nil
}


func updateStake(evm *EVM, s types.SpecialTxInput, caller common.Address) error {
	if err := CheckStakeTx(s); err != nil {
		return err
	}

	amount := new(big.Int)
	// the unit of stake is GNX， one stake means one GNX
	amount.SetUint64(s.Stake*1000000000000000000)

	// judge if there is enough balance to stake（balance must larger than stake value)
	if !evm.Context.CanTransfer(evm.StateDB, caller, amount) {
		return ErrInsufficientBalance
	}

	adress := common.HexToAddress(s.NodeId)
	// 根据nodeid更新stake值
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
