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

package state

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"encoding/json"

	"github.com/GenaroNetwork/Genaro-Core/common"
	"github.com/GenaroNetwork/Genaro-Core/crypto"
	"github.com/GenaroNetwork/Genaro-Core/rlp"
	"time"
	"github.com/GenaroNetwork/Genaro-Core/core/types"
	"github.com/GenaroNetwork/Genaro-Core/common/hexutil"
	"github.com/pkg/errors"
	"sort"
)

var emptyCodeHash = crypto.Keccak256(nil)
var ErrSyncNode = errors.New("no enough stake value to sync node")

type Code []byte

func (self Code) String() string {
	return string(self) //strings.Join(Disassemble(self), " ")
}

type Storage map[common.Hash]common.Hash

func (self Storage) String() (str string) {
	for key, value := range self {
		str += fmt.Sprintf("%X : %X\n", key, value)
	}

	return
}

func (self Storage) Copy() Storage {
	cpy := make(Storage)
	for key, value := range self {
		cpy[key] = value
	}

	return cpy
}

// stateObject represents an Ethereum account which is being modified.
//
// The usage pattern is as follows:
// First you need to obtain a state object.
// Account values can be accessed and modified through the object.
// Finally, call CommitTrie to write the modified storage trie into a database.
type stateObject struct {
	address  common.Address
	addrHash common.Hash // hash of ethereum address of the account
	data     Account
	db       *StateDB

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by StateDB.Commit.
	dbErr error

	// Write caches.
	trie Trie // storage trie, which becomes non-nil on first access
	code Code // contract bytecode, which gets set when code is loaded

	cachedStorage Storage // Storage entry cache to avoid duplicate reads
	dirtyStorage  Storage // Storage entries that need to be flushed to disk

	// Cache flags.
	// When an object is marked suicided it will be delete from the trie
	// during the "update" phase of the state transition.
	dirtyCode bool // true if the code was updated
	suicided  bool
	touched   bool
	deleted   bool
	onDirty   func(addr common.Address) // Callback method to mark a state object newly dirty
}

// empty returns whether the account is considered empty.
func (s *stateObject) empty() bool {
	return s.data.Nonce == 0 && s.data.Balance.Sign() == 0 && bytes.Equal(s.data.CodeHash, emptyCodeHash)
}

// Account is the Ethereum consensus representation of accounts.
// These objects are stored in the main account trie.
type Account struct {
	Nonce    uint64
	Balance  *big.Int
	Root     common.Hash // merkle root of the storage trie
	CodeHash []byte
}


type Candidates []common.Address

func (self *Candidates)isExist(addr common.Address) bool{
	for _,addrIn := range *self {
		if bytes.Compare(addrIn.Bytes(),addr.Bytes()) == 0 {
			return true
		}
	}
	return false
}

func (self *Candidates)DelCandidate(addr common.Address) {
	for i,addrIn := range *self {
		if bytes.Compare(addrIn.Bytes(),addr.Bytes()) == 0 {
			(*self) = append((*self)[:i],(*self)[i+1:]...)
		}
	}
}

type CandidateInfo struct {
	Signer       common.Address // peer address
	Heft uint64         // the sentinel of the peer
	//TODO May need to convert big int
	Stake        uint64         // the stake of the peer
	Point		 uint64
}

type CandidateInfos []CandidateInfo

func (c CandidateInfos) Len() int {
	return len(c)
}

func (c CandidateInfos) Swap(i, j int) {
	c[i].Signer, c[j].Signer = c[j].Signer, c[i].Signer
	c[i].Heft, c[j].Heft = c[j].Heft, c[i].Heft
	c[i].Stake, c[j].Stake = c[j].Stake, c[i].Stake
}

func (c CandidateInfos) Less(i, j int) bool {
	return c[i].Point < c[j].Point
}

func (c CandidateInfos) Apply() {
	totleHeft := uint64(0)
	totleStake := uint64(0)
	for _, candidate := range c{
		totleHeft += candidate.Heft
		totleStake += candidate.Stake
	}
	//TODO define how to get point
	for i, candidate := range c{
		if candidate.Heft == 0 {
			c[i].Point = 0
		} else {
			c[i].Point = candidate.Stake*common.Base/totleStake + candidate.Heft*common.Base/totleHeft
		}
	}
}

// 全员排名，无限制长度
func Rank(candidateInfos CandidateInfos) ([]common.Address, []uint64){
	candidateInfos.Apply()
	sort.Sort(sort.Reverse(candidateInfos))
	committeeRank := make([]common.Address, len(candidateInfos))
	proportion := make([]uint64, len(candidateInfos))
	total := uint64(0)
	for _, c := range candidateInfos{
		total += c.Stake
	}
	if total == 0 {
		return committeeRank, proportion
	}
	for i, c := range candidateInfos{
		committeeRank[i] = c.Signer
		proportion[i] = c.Stake*uint64(common.Base)/total
	}
	return committeeRank, proportion
}

// 限制排名的长度后，进行排名
func RankWithLenth(candidateInfos CandidateInfos, lenth int, committeeMinStake uint64) ([]common.Address, []uint64){
	candidateInfos.Apply()
	// 除去低于stake最小限制的账号
	for i:=0;i<len(candidateInfos);i++ {
		if candidateInfos[i].Stake < committeeMinStake {
			candidateInfos = append(candidateInfos[:i],candidateInfos[i+1:]...)
			i--
		}
	}

	sort.Sort(sort.Reverse(candidateInfos))
	rankLenth := lenth
	if len(candidateInfos) < rankLenth {
		rankLenth = len(candidateInfos)
	}
	committeeRank := make([]common.Address, rankLenth)
	proportion := make([]uint64, rankLenth)
	total := uint64(0)

	for i:=0;i<rankLenth;i++ {
		total += candidateInfos[i].Stake
	}
	if total == 0 {
		return committeeRank, proportion
	}
	for i:=0;i<rankLenth;i++ {
		committeeRank[i] =  candidateInfos[i].Signer
		proportion[i] = candidateInfos[i].Stake*uint64(common.Base)/total
	}
	return committeeRank, proportion
}

type FilePropertie struct {
	StorageGas       uint64	`json:"sgas"`
	StorageGasUsed  uint64	`json:"sGasUsed"`
	StorageGasPrice  uint64 `josn:"sGasPrice"`
	// Ssize represents Storage Size
	Ssize            uint64 `json:"sSize"`
}

// newObject creates a state object.
func newObject(db *StateDB, address common.Address, data Account, onDirty func(addr common.Address)) *stateObject {
	if data.Balance == nil {
		data.Balance = new(big.Int)
	}
	if data.CodeHash == nil {
		data.CodeHash = emptyCodeHash
	}
	return &stateObject{
		db:            db,
		address:       address,
		addrHash:      crypto.Keccak256Hash(address[:]),
		data:          data,
		cachedStorage: make(Storage),
		dirtyStorage:  make(Storage),
		onDirty:       onDirty,
	}
}

// EncodeRLP implements rlp.Encoder.
func (c *stateObject) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, c.data)
}

// setError remembers the first non-nil error it is called with.
func (self *stateObject) setError(err error) {
	if self.dbErr == nil {
		self.dbErr = err
	}
}

func (self *stateObject) markSuicided() {
	self.suicided = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (c *stateObject) touch() {
	c.db.journal = append(c.db.journal, touchChange{
		account:   &c.address,
		prev:      c.touched,
		prevDirty: c.onDirty == nil,
	})
	if c.onDirty != nil {
		c.onDirty(c.Address())
		c.onDirty = nil
	}
	c.touched = true
}

func (c *stateObject) getTrie(db Database) Trie {
	if c.trie == nil {
		var err error
		c.trie, err = db.OpenStorageTrie(c.addrHash, c.data.Root)
		if err != nil {
			c.trie, _ = db.OpenStorageTrie(c.addrHash, common.Hash{})
			c.setError(fmt.Errorf("can't create storage trie: %v", err))
		}
	}
	return c.trie
}

// GetState returns a value in account storage.
func (self *stateObject) GetState(db Database, key common.Hash) common.Hash {
	value, exists := self.cachedStorage[key]
	if exists {
		return value
	}
	// Load from DB in case it is missing.
	enc, err := self.getTrie(db).TryGet(key[:])
	if err != nil {
		self.setError(err)
		return common.Hash{}
	}
	if len(enc) > 0 {
		_, content, _, err := rlp.Split(enc)
		if err != nil {
			self.setError(err)
		}
		value.SetBytes(content)
	}
	if (value != common.Hash{}) {
		self.cachedStorage[key] = value
	}
	return value
}

// SetState updates a value in account storage.
func (self *stateObject) SetState(db Database, key, value common.Hash) {
	self.db.journal = append(self.db.journal, storageChange{
		account:  &self.address,
		key:      key,
		prevalue: self.GetState(db, key),
	})
	self.setState(key, value)
}

func (self *stateObject) setState(key, value common.Hash) {
	self.cachedStorage[key] = value
	self.dirtyStorage[key] = value

	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

// updateTrie writes cached storage modifications into the object's storage trie.
func (self *stateObject) updateTrie(db Database) Trie {
	tr := self.getTrie(db)
	for key, value := range self.dirtyStorage {
		delete(self.dirtyStorage, key)
		if (value == common.Hash{}) {
			self.setError(tr.TryDelete(key[:]))
			continue
		}
		// Encoding []byte cannot fail, ok to ignore the error.
		v, _ := rlp.EncodeToBytes(bytes.TrimLeft(value[:], "\x00"))
		self.setError(tr.TryUpdate(key[:], v))
	}
	return tr
}

// UpdateRoot sets the trie root to the current root hash of
func (self *stateObject) updateRoot(db Database) {
	self.updateTrie(db)
	self.data.Root = self.trie.Hash()
}

// CommitTrie the storage trie of the object to dwb.
// This updates the trie root.
func (self *stateObject) CommitTrie(db Database) error {
	self.updateTrie(db)
	if self.dbErr != nil {
		return self.dbErr
	}
	root, err := self.trie.Commit(nil)
	if err == nil {
		self.data.Root = root
	}
	return err
}

// AddBalance removes amount from c's balance.
// It is used to add funds to the destination account of a transfer.
func (c *stateObject) AddBalance(amount *big.Int) {
	// EIP158: We must check emptiness for the objects such that the account
	// clearing (0,0,0 objects) can take effect.
	if amount.Sign() == 0 {
		if c.empty() {
			c.touch()
		}

		return
	}
	c.SetBalance(new(big.Int).Add(c.Balance(), amount))
}

// SubBalance removes amount from c's balance.
// It is used to remove funds from the origin account of a transfer.
func (c *stateObject) SubBalance(amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	c.SetBalance(new(big.Int).Sub(c.Balance(), amount))
}

func (self *stateObject) SetBalance(amount *big.Int) {
	self.db.journal = append(self.db.journal, balanceChange{
		account: &self.address,
		prev:    new(big.Int).Set(self.data.Balance),
	})
	self.setBalance(amount)
}

func (self *stateObject) setBalance(amount *big.Int) {
	self.data.Balance = amount
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

// Return the gas back to the origin. Used by the Virtual machine or Closures
func (c *stateObject) ReturnGas(gas *big.Int) {}

func (self *stateObject) deepCopy(db *StateDB, onDirty func(addr common.Address)) *stateObject {
	stateObject := newObject(db, self.address, self.data, onDirty)
	if self.trie != nil {
		stateObject.trie = db.db.CopyTrie(self.trie)
	}
	stateObject.code = self.code
	stateObject.dirtyStorage = self.dirtyStorage.Copy()
	stateObject.cachedStorage = self.dirtyStorage.Copy()
	stateObject.suicided = self.suicided
	stateObject.dirtyCode = self.dirtyCode
	stateObject.deleted = self.deleted
	return stateObject
}

//
// Attribute accessors
//

// Returns the address of the contract/account
func (c *stateObject) Address() common.Address {
	return c.address
}

// if empty return true
func CheckCodeEmpty(codeHash []byte) bool {
	if bytes.Equal(codeHash, emptyCodeHash) || len(codeHash)!=32 {
		return true
	} else {
		return false
	}
}

// Code returns the contract code associated with this object, if any.
func (self *stateObject) Code(db Database) []byte {
	if self.code != nil {
		return self.code
	}
	if CheckCodeEmpty(self.CodeHash()) {
		return nil
	}
	code, err := db.ContractCode(self.addrHash, common.BytesToHash(self.CodeHash()))
	if err != nil {
		self.setError(fmt.Errorf("can't load code hash %x: %v", self.CodeHash(), err))
	}
	self.code = code
	return code
}

func (self *stateObject) SetCode(codeHash common.Hash, code []byte) {
	prevcode := self.Code(self.db.db)
	self.db.journal = append(self.db.journal, codeChange{
		account:  &self.address,
		prevhash: self.CodeHash(),
		prevcode: prevcode,
	})
	self.setCode(codeHash, code)
}

// only used in genaro genesis init
func (self *stateObject) SetCodeHash(codeHash []byte) {
	self.data.CodeHash = codeHash[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (self *stateObject) setCode(codeHash common.Hash, code []byte) {
	self.code = code
	self.data.CodeHash = codeHash[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (self *stateObject) SetNonce(nonce uint64) {
	self.db.journal = append(self.db.journal, nonceChange{
		account: &self.address,
		prev:    self.data.Nonce,
	})
	self.setNonce(nonce)
}

func (self *stateObject) setNonce(nonce uint64) {
	self.data.Nonce = nonce
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (self *stateObject) CodeHash() []byte {
	return self.data.CodeHash
}

func (self *stateObject) Balance() *big.Int {
	return self.data.Balance
}

func (self *stateObject) Nonce() uint64 {
	return self.data.Nonce
}

// Never called, but must be present to allow stateObject to be used
// as a vm.Account interface that also satisfies the vm.ContractRef
// interface. Interfaces are awesome.
func (self *stateObject) Value() *big.Int {
	panic("Value on stateObject should never be called")
}

// update heft and add heft log
func (self *stateObject)UpdateHeft(heft uint64, blockNumber uint64){
	var genaroData types.GenaroData
	if self.data.CodeHash == nil{
		genaroData = types.GenaroData{
			Heft:heft,
		}
	}else {
		json.Unmarshal(self.data.CodeHash, &genaroData)
		genaroData.Heft = heft
	}
	if genaroData.HeftLog == nil {
		genaroData.HeftLog = *new(types.NumLogs)
	}
	var newLog types.NumLog
	newLog.Num = heft
	newLog.BlockNum = blockNumber
	genaroData.HeftLog.Add(newLog)

	b, _ := json.Marshal(genaroData)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (self *stateObject)GetHeft() (uint64){
	if self.data.CodeHash != nil {
		var genaroData types.GenaroData
		json.Unmarshal(self.data.CodeHash, &genaroData)
		return genaroData.Heft
	}

	return 0
}

func (self *stateObject)GetHeftLog() (types.NumLogs){
	if self.data.CodeHash != nil {
		var genaroData types.GenaroData
		json.Unmarshal(self.data.CodeHash, &genaroData)
		return genaroData.HeftLog
	}

	return nil
}

func (self *stateObject)GetHeftRangeDiff(blockNumStart uint64, blockNumEnd uint64) (uint64){
	if self.data.CodeHash != nil {
		var genaroData types.GenaroData
		json.Unmarshal(self.data.CodeHash, &genaroData)
		return genaroData.HeftLog.GetRangeDiff(blockNumStart,blockNumEnd)
	}

	return 0
}

// update stake and add stake log
func (self *stateObject)UpdateStake(stake uint64, blockNumber uint64){
	var genaroData types.GenaroData
	if self.data.CodeHash == nil{
		genaroData = types.GenaroData{
			Stake:stake,
		}
	}else {
		json.Unmarshal(self.data.CodeHash, &genaroData)
		genaroData.Stake += stake
	}
	if genaroData.StakeLog == nil {
		genaroData.StakeLog = *new(types.NumLogs)
	}
	var newLog types.NumLog
	newLog.Num = genaroData.Stake
	newLog.BlockNum = blockNumber
	genaroData.StakeLog.Add(newLog)

	b, _ := json.Marshal(genaroData)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (self *stateObject)DeleteStake(stake uint64, blockNumber uint64) uint64 {
	var currentPunishment uint64
	var genaroData types.GenaroData
	if self.data.CodeHash != nil {
		json.Unmarshal(self.data.CodeHash, &genaroData)

		if genaroData.Stake <= stake {
			currentPunishment = genaroData.Stake
			genaroData.Stake = 0
		}else {
			currentPunishment = stake
			genaroData.Stake -= stake
		}

		var newLog types.NumLog
		newLog.Num = genaroData.Stake
		newLog.BlockNum = blockNumber
		genaroData.StakeLog.Add(newLog)

		b, _ := json.Marshal(genaroData)
		self.code = nil
		self.data.CodeHash = b[:]
		self.dirtyCode = true
		if self.onDirty != nil {
			self.onDirty(self.Address())
			self.onDirty = nil
		}
	}
	return currentPunishment
}

func (self *stateObject)GetStake() (uint64){
	if self.data.CodeHash != nil {
		var genaroData types.GenaroData
		json.Unmarshal(self.data.CodeHash, &genaroData)
		return genaroData.Stake
	}

	return 0
}


func (self *stateObject)GetStakeLog() (types.NumLogs){
	if self.data.CodeHash != nil {
		var genaroData types.GenaroData
		json.Unmarshal(self.data.CodeHash, &genaroData)
		return genaroData.StakeLog
	}

	return nil
}

func (self *stateObject)GetStakeRangeDiff(blockNumStart uint64, blockNumEnd uint64) (uint64){
	if self.data.CodeHash != nil {
		var genaroData types.GenaroData
		json.Unmarshal(self.data.CodeHash, &genaroData)
		return genaroData.StakeLog.GetRangeDiff(blockNumStart,blockNumEnd)
	}

	return 0
}

func (self *stateObject) AddCandidate(candidate common.Address) {
	var candidates Candidates
	if self.data.CodeHash == nil{
		candidates = *new(Candidates)
	}else {
		json.Unmarshal(self.data.CodeHash, &candidates)
	}
	if !candidates.isExist(candidate) {
		candidates = append(candidates,candidate)
		b, _ := json.Marshal(candidates)
		self.code = nil
		self.data.CodeHash = b[:]
		self.dirtyCode = true
		if self.onDirty != nil {
			self.onDirty(self.Address())
			self.onDirty = nil
		}
	}
}

// 判断候选者是否存在
func (self *stateObject) IsCandidateExist(candidate common.Address) bool{
	var candidates Candidates
	if self.data.CodeHash == nil{
		candidates = *new(Candidates)
	}else {
		json.Unmarshal(self.data.CodeHash, &candidates)
	}

	return candidates.isExist(candidate)
}

func (self *stateObject) DelCandidate(candidate common.Address) {
	var candidates Candidates
	if self.data.CodeHash == nil{
		candidates = *new(Candidates)
	}else {
		json.Unmarshal(self.data.CodeHash, &candidates)
	}

	if candidates.isExist(candidate) {
		candidates.DelCandidate(candidate)
		b, _ := json.Marshal(candidates)
		self.code = nil
		self.data.CodeHash = b[:]
		self.dirtyCode = true
		if self.onDirty != nil {
			self.onDirty(self.Address())
			self.onDirty = nil
		}
	}
}

func (self *stateObject)GetCandidates() (Candidates){
	if self.data.CodeHash != nil {
		var candidates Candidates
		json.Unmarshal(self.data.CodeHash, &candidates)
		return candidates
	}
	return nil
}

// 将账号加入禁止退注列表
func (self *stateObject) AddAccountInForbidBackStakeList(addr common.Address) {
	var forbidList types.ForbidBackStakeList
	if self.data.CodeHash == nil{
		forbidList = *new(types.ForbidBackStakeList)
	}else {
		json.Unmarshal(self.data.CodeHash, &forbidList)
	}
	if !forbidList.IsExist(addr) {
		forbidList.Add(addr)
		b, _ := json.Marshal(forbidList)
		self.code = nil
		self.data.CodeHash = b[:]
		self.dirtyCode = true
		if self.onDirty != nil {
			self.onDirty(self.Address())
			self.onDirty = nil
		}
	}
}

// 判断账号是否存在于禁止退注列表中
func (self *stateObject) IsAccountExistInForbidBackStakeList(addr common.Address) bool{
	var forbidList types.ForbidBackStakeList
	if self.data.CodeHash == nil{
		forbidList = *new(types.ForbidBackStakeList)
	}else {
		json.Unmarshal(self.data.CodeHash, &forbidList)
	}

	return forbidList.IsExist(addr)
}

// 将账号从禁止退注名单中删除
func (self *stateObject) DelAccountInForbidBackStakeList(addr common.Address) {
	var forbidList types.ForbidBackStakeList
	if self.data.CodeHash == nil{
		forbidList = *new(types.ForbidBackStakeList)
	}else {
		json.Unmarshal(self.data.CodeHash, &forbidList)
	}

	if forbidList.IsExist(addr) {
		forbidList.Del(addr)
		b, _ := json.Marshal(forbidList)
		self.code = nil
		self.data.CodeHash = b[:]
		self.dirtyCode = true
		if self.onDirty != nil {
			self.onDirty(self.Address())
			self.onDirty = nil
		}
	}
}

// 获取禁止退注名单
func (self *stateObject)GetForbidBackStakeList() (types.ForbidBackStakeList){
	if self.data.CodeHash != nil {
		var forbidList types.ForbidBackStakeList
		json.Unmarshal(self.data.CodeHash, &forbidList)
		return forbidList
	}
	return nil
}


func (self *stateObject) AddAlreadyBackStack(backStake common.AlreadyBackStake) {
	var backStakes common.BackStakeList
	if self.data.CodeHash == nil{
		backStakes = *new(common.BackStakeList)
	}else {
		json.Unmarshal(self.data.CodeHash, &backStakes)
	}
	if !backStakes.IsExist(backStake){
		backStakes = append(backStakes,backStake)

		b, _ := json.Marshal(backStakes)
		self.code = nil
		self.data.CodeHash = b[:]
		self.dirtyCode = true
		if self.onDirty != nil {
			self.onDirty(self.Address())
			self.onDirty = nil
		}
	}
}

func (self *stateObject)GetAlreadyBackStakeList() (common.BackStakeList){
	if self.data.CodeHash != nil {
		var backStakes common.BackStakeList
		json.Unmarshal(self.data.CodeHash, &backStakes)
		return backStakes
	}
	return nil
}

func (self *stateObject)SetAlreadyBackStakeList(backStakes common.BackStakeList){
	b, _ := json.Marshal(backStakes)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (self *stateObject)UpdateBucketProperties(buckid string, szie uint64, backup uint64, timestart uint64, timeend uint64) {
	var bpArr []*types.BucketPropertie
	bp := new(types.BucketPropertie)
	if buckid != "" {bp.BucketId = buckid}
	if szie != 0 {bp.Size = szie}
	if backup != 0 {bp.Backup = backup}
	if timestart != 0 {bp.TimeStart = timestart}
	if timeend != 0 {bp.TimeEnd = timeend}
	bpArr = append(bpArr, bp)

	var genaroData types.GenaroData
	if self.data.CodeHash == nil{
		genaroData = types.GenaroData{
			Buckets: bpArr,
		}
	}else {
		json.Unmarshal(self.data.CodeHash, &genaroData)
		if genaroData.Buckets == nil {
			genaroData.Buckets = bpArr
		}else {
			genaroData.Buckets = append(genaroData.Buckets, bpArr...)
		}
	}

	b, _ := json.Marshal(genaroData)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (self *stateObject)getBucketPropertie(bucketID string) *types.BucketPropertie {
	if self.data.CodeHash != nil {
		var genaroData types.GenaroData
		json.Unmarshal(self.data.CodeHash, &genaroData)
		if genaroData.Buckets != nil {
			for _, v := range genaroData.Buckets {
				if v.BucketId == bucketID {
					return v
				}
			}
		}
	}

	return nil
}

func (self *stateObject)GetStorageSize(bucketID string) uint64 {
	if bp:= self.getBucketPropertie(bucketID); bp != nil{
		return bp.Size
	}
	return 0
}


func (self *stateObject)GetStorageGasPrice(bucketID string) uint64 {
	if bp:= self.getBucketPropertie(bucketID); bp != nil{
		return bp.Backup
	}
	return 0
}


func (self *stateObject)GetStorageGasUsed(bucketID string) uint64 {
	if bp:= self.getBucketPropertie(bucketID); bp != nil{
		return bp.Backup * bp.Size
	}
	return 0
}

func (self *stateObject)GetStorageGas(bucketID string) uint64 {
	if bp:= self.getBucketPropertie(bucketID); bp != nil{
		return bp.TimeEnd-bp.TimeStart
	}
	return 0
}

func (self *stateObject)UpdateTraffic(traffic uint64){
	var genaroData types.GenaroData
	if self.data.CodeHash == nil{
		genaroData = types.GenaroData{
			Traffic:traffic,
		}
	}else {
		json.Unmarshal(self.data.CodeHash, &genaroData)
		genaroData.Traffic += traffic
	}

	b, _ := json.Marshal(genaroData)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (self *stateObject)GetTraffic() uint64 {
	if self.data.CodeHash != nil {
		var genaroData types.GenaroData
		json.Unmarshal(self.data.CodeHash, &genaroData)
		return genaroData.Traffic
	}

	return 0
}

func (self *stateObject)GetBuckets() map[string]interface{} {
	rtMap := make(map[string]interface{})
	if self.data.CodeHash != nil {
		var genaroData types.GenaroData
		json.Unmarshal(self.data.CodeHash, &genaroData)
		if genaroData.Buckets != nil {
			for _, v := range genaroData.Buckets {
				rtMap[v.BucketId] = *v
			}
		}
	}
	return  rtMap
}

func (self *stateObject)GetStorageNodes() []string {
	if self.data.CodeHash == nil{
		return nil
	}

	var genaroData types.GenaroData
	if err := json.Unmarshal(self.data.CodeHash, &genaroData); err != nil {
		return nil
	}

	return genaroData.Node
}

//Cross-chain storage processing
func (self *stateObject)SpecialTxTypeMortgageInit(specialTxTypeMortgageInit types.SpecialTxTypeMortgageInit) bool {
	var genaroData types.GenaroData
	if len(specialTxTypeMortgageInit.AuthorityTable) != len(specialTxTypeMortgageInit.MortgageTable) {
		return false
	}
	for k,_ := range  specialTxTypeMortgageInit.AuthorityTable {
		if _, ok := specialTxTypeMortgageInit.MortgageTable[k]; !ok {
			return false
		}
	}
	if nil == self.data.CodeHash {
		genaroData = types.GenaroData{
			SpecialTxTypeMortgageInitArr:map[string]types.SpecialTxTypeMortgageInit {specialTxTypeMortgageInit.FileID:specialTxTypeMortgageInit},
		}
	}else {
		json.Unmarshal(self.data.CodeHash, &genaroData)
		if nil == genaroData.SpecialTxTypeMortgageInitArr {
			genaroData.SpecialTxTypeMortgageInitArr = map[string]types.SpecialTxTypeMortgageInit {specialTxTypeMortgageInit.FileID:specialTxTypeMortgageInit}
		} else {
			genaroData.SpecialTxTypeMortgageInitArr[specialTxTypeMortgageInit.FileID] = specialTxTypeMortgageInit
		}
	}
	genaroData.SpecialTxTypeMortgageInit = types.SpecialTxTypeMortgageInit{}
	b, _ := json.Marshal(genaroData)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
	return true
}

func (self *stateObject)GetAccountAttributes() types.GenaroData{
	if self.data.CodeHash != nil {
		var genaroData types.GenaroData
		json.Unmarshal(self.data.CodeHash, &genaroData)
		return genaroData
	}
	return types.GenaroData{}
}

func (self *stateObject)SpecialTxTypeSyncSidechainStatus(SpecialTxTypeSyncSidechainStatus types.SpecialTxTypeMortgageInit)(map[common.Address] *big.Int, bool) {
	var genaroData types.GenaroData
	AddBalance :=make(map[common.Address] *big.Int)
	if nil == self.data.CodeHash {
		return  nil,false
	}else {
		json.Unmarshal(self.data.CodeHash, &genaroData)
		fileID := SpecialTxTypeSyncSidechainStatus.FileID
		result := genaroData.SpecialTxTypeMortgageInitArr[fileID]
		if 0 == len(result.MortgageTable) || len(result.MortgageTable) != len(result.AuthorityTable) ||
			len(result.MortgageTable) != len(SpecialTxTypeSyncSidechainStatus.Sidechain){
			return nil,false
		}
		if result.EndTime > time.Now().Unix() && false == SpecialTxTypeSyncSidechainStatus.Terminate && false == result.Terminate{
			if 0 == len(result.SidechainStatus) {
				result.SidechainStatus = make(map[string] map[common.Address] *hexutil.Big)
			}
			result.SidechainStatus[SpecialTxTypeSyncSidechainStatus.Dataversion] = SpecialTxTypeSyncSidechainStatus.Sidechain
		}else if  true == SpecialTxTypeSyncSidechainStatus.Terminate && false == result.Terminate{
			if 0 == len(result.SidechainStatus) {
				result.SidechainStatus = make(map[string] map[common.Address] *hexutil.Big)
			}
			result.SidechainStatus[SpecialTxTypeSyncSidechainStatus.Dataversion] = SpecialTxTypeSyncSidechainStatus.Sidechain
			useMortgagTotal := new(big.Int)
			zero := big.NewInt(0)
			for k,v := range SpecialTxTypeSyncSidechainStatus.Sidechain {
				if common.ReadWrite == result.AuthorityTable[k] || common.Write == result.AuthorityTable[k] {
					if v.ToInt().Cmp(zero) < 0 {
						return nil, false
					}
					if result.MortgageTable[k].ToInt().Cmp(v.ToInt()) > -1{
						AddBalance[k] = v.ToInt()
						useMortgagTotal.Add(useMortgagTotal,v.ToInt())
					} else {
						AddBalance[k] = result.MortgageTable[k].ToInt()
						useMortgagTotal.Add(useMortgagTotal,result.MortgageTable[k].ToInt())
					}
				}
			}
			AddBalance[result.FromAccount] = result.MortgagTotal.Sub(result.MortgagTotal,useMortgagTotal)
			result.Terminate = true
		}else {
			return nil, false
		}
		genaroData.SpecialTxTypeMortgageInitArr[fileID] = result
	}
	genaroData.SpecialTxTypeMortgageInit = types.SpecialTxTypeMortgageInit{}
	b, _ := json.Marshal(genaroData)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
	return AddBalance, true
}

func (self *stateObject) TxLogBydataVersionUpdate(fileID string) (types.SpecialTxTypeMortgageInit, bool)  {
	if self.data.CodeHash != nil {
		var genaroData types.GenaroData
		json.Unmarshal(self.data.CodeHash, &genaroData)
		accountAttributes := genaroData.SpecialTxTypeMortgageInitArr
		resultTmp := accountAttributes[fileID]
		if true == resultTmp.Terminate || resultTmp.EndTime < time.Now().Unix() {
			return types.SpecialTxTypeMortgageInit{},false
		}
		if  0 == len(resultTmp.AuthorityTable) {
			return  types.SpecialTxTypeMortgageInit{},false
		}
		resultTmp.LogSwitch = true
		genaroData.SpecialTxTypeMortgageInitArr[fileID] = resultTmp
		b, _ := json.Marshal(genaroData)
		self.code = nil
		self.data.CodeHash = b[:]
		self.dirtyCode = true
		if self.onDirty != nil {
			self.onDirty(self.Address())
			self.onDirty = nil
		}
		return  resultTmp, true
	}
	return types.SpecialTxTypeMortgageInit{},false
}

func (self *stateObject) TxLogByDataVersionRead(fileID,dataVersion string) (map[common.Address] *hexutil.Big, error) {
	if self.data.CodeHash != nil {
		var genaroData types.GenaroData
		json.Unmarshal(self.data.CodeHash, &genaroData)
		accountAttributes := genaroData.SpecialTxTypeMortgageInitArr
		resultTmp := accountAttributes[fileID]
		if  0 == len(resultTmp.AuthorityTable) {
			return  nil,nil
		}
		return  resultTmp.SidechainStatus[dataVersion],nil
	}
	return nil,nil
}

func (self *stateObject)SyncStakeNode(s string) error {
	var err error
	var genaroData types.GenaroData
	if self.data.CodeHash == nil{ // 用户数据为空，表示用户未进行stake操作，不能同步节点到链上
		err = ErrSyncNode
	}else {
		json.Unmarshal(self.data.CodeHash, &genaroData)
		genaroData.Node = append(genaroData.Node, s)
		b, _ := json.Marshal(genaroData)
		self.code = nil
		self.data.CodeHash = b[:]
		self.dirtyCode = true
		if self.onDirty != nil {
			self.onDirty(self.Address())
			self.onDirty = nil
		}

	}
	return err
}

func (self *stateObject)SyncNode2Address(s string, address string) error {
	d := make(map[string]string)
	if self.data.CodeHash == nil {
			d[s] = address
	}else{
		json.Unmarshal(self.data.CodeHash, &d)
		d[s] = address
	}
	b, _ := json.Marshal(d)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
	return nil
}

func (self *stateObject)GetAddressByNode (s string) string{
	if self.data.CodeHash == nil {
		return ""
	}else {
		d := make(map[string]string)
		err := json.Unmarshal(self.data.CodeHash, &d)
		if err != nil{
			return ""
		}
		if v, ok := d[s]; !ok {
			return ""
		}else {
			return v
		}
	}
}

func (self *stateObject)SynchronizeShareKey(synchronizeShareKey types.SynchronizeShareKey) bool {
	var genaroData types.GenaroData
	if nil == self.data.CodeHash {
		genaroData = types.GenaroData{
			SynchronizeShareKeyArr:map[string]types.SynchronizeShareKey {synchronizeShareKey.ShareKeyId:synchronizeShareKey},
		}
	}else {
		json.Unmarshal(self.data.CodeHash, &genaroData)
		if nil == genaroData.SynchronizeShareKeyArr {
			genaroData.SynchronizeShareKeyArr = map[string]types.SynchronizeShareKey {synchronizeShareKey.ShareKeyId:synchronizeShareKey}
		} else {
			genaroData.SynchronizeShareKeyArr[synchronizeShareKey.ShareKeyId] = synchronizeShareKey
		}
	}
	genaroData.SynchronizeShareKey = types.SynchronizeShareKey{}
	b, _ := json.Marshal(genaroData)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
	return true
}

func (self *stateObject)UpdateFileSharePublicKey(publicKey string){
	var genaroData types.GenaroData
	if self.data.CodeHash == nil{
		genaroData = types.GenaroData{
			FileSharePublicKey:publicKey,
		}
	}else {
		json.Unmarshal(self.data.CodeHash, &genaroData)
		genaroData.FileSharePublicKey = publicKey
	}

	b, _ := json.Marshal(genaroData)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}


func (self *stateObject)GetFileSharePublicKey() string {
	if self.data.CodeHash == nil{
		return ""
	}

	var genaroData types.GenaroData
	if err := json.Unmarshal(self.data.CodeHash, &genaroData); err != nil {
		return ""
	}

	return genaroData.FileSharePublicKey
}


func (self *stateObject)UnlockSharedKey(shareKeyId string) types.SynchronizeShareKey {
	var genaroData types.GenaroData
	var synchronizeShareKey	types.SynchronizeShareKey
	if nil == self.data.CodeHash {
		return types.SynchronizeShareKey{}
	}else {
		json.Unmarshal(self.data.CodeHash, &genaroData)
		if nil == genaroData.SynchronizeShareKeyArr {
			return types.SynchronizeShareKey{}
		} else {
			synchronizeShareKey = genaroData.SynchronizeShareKeyArr[shareKeyId]
			if 1 == synchronizeShareKey.Status{
				return synchronizeShareKey
			}
			synchronizeShareKey.Status = 1
			genaroData.SynchronizeShareKeyArr[shareKeyId] = synchronizeShareKey
			synchronizeShareKey.Status = 0
		}
	}
	genaroData.SynchronizeShareKey = types.SynchronizeShareKey{}
	b, _ := json.Marshal(genaroData)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
	return synchronizeShareKey
}

func (self *stateObject)CheckUnlockSharedKey(shareKeyId string) bool {
	var genaroData types.GenaroData
	var synchronizeShareKey	types.SynchronizeShareKey
	if nil == self.data.CodeHash {
		return false
	}else {
		json.Unmarshal(self.data.CodeHash, &genaroData)
		if nil == genaroData.SynchronizeShareKeyArr {
			return false
		} else {
			synchronizeShareKey = genaroData.SynchronizeShareKeyArr[shareKeyId]
			if 1 == synchronizeShareKey.Status{
				return true
			}

		}
	}
	return false
}

func (self *stateObject)UpdateBucketApplyPrice(price *hexutil.Big) {
	var genaroPrice types.GenaroPrice
	if self.data.CodeHash == nil{
		genaroPrice = types.GenaroPrice{
			BucketApplyGasPerGPerDay :price,
		}
	}else {
		json.Unmarshal(self.data.CodeHash, &genaroPrice)
		genaroPrice.BucketApplyGasPerGPerDay = price
	}

	b, _ := json.Marshal(genaroPrice)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (self *stateObject)GetBucketApplyPrice() *big.Int{
	if self.data.CodeHash != nil {
		var genaroPrice types.GenaroPrice
		json.Unmarshal(self.data.CodeHash, &genaroPrice)
		if genaroPrice.BucketApplyGasPerGPerDay != nil {
			return genaroPrice.BucketApplyGasPerGPerDay.ToInt()
		}
	}

	return common.DefaultBucketApplyGasPerGPerDay
}

func (self *stateObject)UpdateTrafficApplyPrice(price *hexutil.Big) {
	var genaroPrice types.GenaroPrice
	if self.data.CodeHash == nil{
		genaroPrice = types.GenaroPrice{
			TrafficApplyGasPerG :price,
		}
	}else {
		json.Unmarshal(self.data.CodeHash, &genaroPrice)
		genaroPrice.TrafficApplyGasPerG = price
	}

	b, _ := json.Marshal(genaroPrice)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
			self.onDirty(self.Address())
			self.onDirty = nil
	}
}

// 添加最近块的信息
func (self *stateObject)AddLastRootState(statehash common.Hash, blockNumber uint64) {
	var lastSynState types.LastSynState
	if self.data.CodeHash == nil{
		lastSynState = types.LastSynState{
			LastRootStates:	make(map[common.Hash]uint64),
			LastSynBlockNum: 0,
		}
	}else {
		json.Unmarshal(self.data.CodeHash, &lastSynState)
	}

	lastSynState.AddLastSynState(statehash,blockNumber)

	b, _ := json.Marshal(lastSynState)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

// 更新账号绑定
func (self *stateObject)UpdateAccountBinding(mainAccount common.Address, subAccount common.Address){
	var bindingTable types.BindingTable
	if self.data.CodeHash != nil {
		json.Unmarshal(self.data.CodeHash, &bindingTable)
	}
	if bindingTable.MainAccounts == nil {
		bindingTable.MainAccounts = make(map[common.Address][]common.Address)
	}
	if bindingTable.SubAccounts == nil {
		bindingTable.SubAccounts = make(map[common.Address]common.Address)
	}

	bindingTable.UpdateBinding(mainAccount,subAccount)

	b, _ := json.Marshal(bindingTable)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

// 删除子账号绑定
// 成功删除一个绑定账号返回true，否则返回false
func (self *stateObject)DelSubAccountBinding(subAccount common.Address) bool{
	var bindingTable types.BindingTable
	if self.data.CodeHash != nil{
		json.Unmarshal(self.data.CodeHash, &bindingTable)
	} else {
		return false
	}

	if bindingTable.IsSubAccountExist(subAccount) {
		bindingTable.DelSubAccount(subAccount)

		b, _ := json.Marshal(bindingTable)
		self.code = nil
		self.data.CodeHash = b[:]
		self.dirtyCode = true
		if self.onDirty != nil {
			self.onDirty(self.Address())
			self.onDirty = nil
		}
		return true
	}
	return false
}

// 主账号删除所有绑定
// 返回删除主账号后的关联删除的子账号列表
func (self *stateObject)DelMainAccountBinding(mainAccount common.Address) []common.Address{
	var bindingTable types.BindingTable
	if self.data.CodeHash != nil {
		json.Unmarshal(self.data.CodeHash, &bindingTable)
	} else {
		return nil
	}

	if bindingTable.IsMainAccountExist(mainAccount) {
		subAccounts := bindingTable.DelMainAccount(mainAccount)

		b, _ := json.Marshal(bindingTable)
		self.code = nil
		self.data.CodeHash = b[:]
		self.dirtyCode = true
		if self.onDirty != nil {
			self.onDirty(self.Address())
			self.onDirty = nil
		}
		return subAccounts
	}
	return nil
}

// 获取所属子账号
func (self *stateObject)GetSubAccounts(mainAccount common.Address) []common.Address{
	var bindingTable types.BindingTable
	if self.data.CodeHash != nil {
		json.Unmarshal(self.data.CodeHash, &bindingTable)
	} else {
		return nil
	}
	if bindingTable.IsMainAccountExist(mainAccount) {
		return bindingTable.MainAccounts[mainAccount]
	}

	return nil
}

// 获取子账号数量
func (self *stateObject)GetSubAccountsCount(mainAccount common.Address) int{
	var bindingTable types.BindingTable
	if self.data.CodeHash != nil {
		json.Unmarshal(self.data.CodeHash, &bindingTable)
	} else {
		return 0
	}

	return bindingTable.GetSubAccountSizeInMainAccount(mainAccount)
}

// 获取账号映射表
func (self *stateObject)GetMainAccounts() map[common.Address][]common.Address{
	var bindingTable types.BindingTable
	if self.data.CodeHash != nil {
		json.Unmarshal(self.data.CodeHash, &bindingTable)
	} else {
		return nil
	}

	return bindingTable.MainAccounts
}

// 获取所属主账号
// 如果子账号不存在，则返回nil
func (self *stateObject)GetMainAccount(subAccount common.Address) *common.Address{
	var bindingTable types.BindingTable
	if self.data.CodeHash != nil {
		json.Unmarshal(self.data.CodeHash, &bindingTable)
	} else {
		return nil
	}

	if bindingTable.IsSubAccountExist(subAccount) {
		mainAccount := bindingTable.SubAccounts[subAccount]
		return &mainAccount
	}

	return nil
}

// 检查是否是绑定账号
func (self *stateObject)IsBindingAccount(account common.Address) bool {
	var bindingTable types.BindingTable
	if self.data.CodeHash != nil {
		json.Unmarshal(self.data.CodeHash, &bindingTable)
	} else {
		return false
	}

	return bindingTable.IsAccountInBinding(account)
}

// 检查是否是主账号
func (self *stateObject)IsMainAccount(account common.Address) bool {
	var bindingTable types.BindingTable
	if self.data.CodeHash != nil {
		json.Unmarshal(self.data.CodeHash, &bindingTable)
	} else {
		return false
	}

	return bindingTable.IsMainAccountExist(account)
}

// 检查是否是子账号
func (self *stateObject)IsSubAccount(account common.Address) bool {
	var bindingTable types.BindingTable
	if self.data.CodeHash != nil {
		json.Unmarshal(self.data.CodeHash, &bindingTable)
	} else {
		return false
	}

	return bindingTable.IsSubAccountExist(account)
}

func (self *stateObject)GetTrafficApplyPrice() *big.Int {

	genaroPrice := self.GetGenaroPrice()
	if genaroPrice != nil {
		if genaroPrice.TrafficApplyGasPerG != nil {
			return genaroPrice.TrafficApplyGasPerG.ToInt()
		}
	}
	return common.DefaultTrafficApplyGasPerG
}

func (self *stateObject)UpdateStakePerNodePrice(price *hexutil.Big) {
	var genaroPrice types.GenaroPrice
	if self.data.CodeHash == nil{
		genaroPrice = types.GenaroPrice{
			StakeValuePerNode :price,
		}
	}else {
		json.Unmarshal(self.data.CodeHash, &genaroPrice)
		genaroPrice.StakeValuePerNode = price
	}

	b, _ := json.Marshal(genaroPrice)
        self.code = nil
        self.data.CodeHash = b[:]
        self.dirtyCode = true
        if self.onDirty != nil {
                self.onDirty(self.Address())
                self.onDirty = nil
        }
}

func (self *stateObject)SetLastSynBlock(blockNumber uint64,blockHash common.Hash) {
	var lastsynState types.LastSynState
	if self.data.CodeHash == nil{
		lastsynState = types.LastSynState{
			LastRootStates:	make(map[common.Hash]uint64),
			LastSynBlockNum: 0,
		}
	}else {
		json.Unmarshal(self.data.CodeHash, &lastsynState)
	}

	lastsynState.LastSynBlockNum = blockNumber
	lastsynState.LastSynBlockHash = blockHash

	b, _ := json.Marshal(lastsynState)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (self *stateObject)GetStakePerNodePrice() *big.Int {

	genaroPrice := self.GetGenaroPrice()
	if genaroPrice != nil {
		if genaroPrice.StakeValuePerNode != nil {
			return genaroPrice.StakeValuePerNode.ToInt()
		}
	}

	return common.DefaultTrafficApplyGasPerG
}

func (self *stateObject)GetGenaroPrice() *types.GenaroPrice {
	if self.data.CodeHash != nil {
		var genaroPrice types.GenaroPrice
		json.Unmarshal(self.data.CodeHash, &genaroPrice)
		return  &genaroPrice
	}
	return nil
}

func (self *stateObject)SetGenaroPrice(genaroPrice types.GenaroPrice) {
	b, _ := json.Marshal(genaroPrice)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}


func (self *stateObject)UpdateOneDayGesCost(price *hexutil.Big) {
	var genaroPrice types.GenaroPrice
	if self.data.CodeHash == nil{
		genaroPrice = types.GenaroPrice{
			OneDayMortgageGes :price,
		}
	}else {
		json.Unmarshal(self.data.CodeHash, &genaroPrice)
		genaroPrice.OneDayMortgageGes = price
	}

	b, _ := json.Marshal(genaroPrice)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

func (self *stateObject)UpdateOneDaySyncLogGsaCost(price *hexutil.Big) {
	var genaroPrice types.GenaroPrice
	if self.data.CodeHash == nil{
		genaroPrice = types.GenaroPrice{
			OneDaySyncLogGsaCost :price,
		}
	}else {
		json.Unmarshal(self.data.CodeHash, &genaroPrice)
		genaroPrice.OneDaySyncLogGsaCost = price
	}

	b, _ := json.Marshal(genaroPrice)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}


func (self *stateObject)GetOneDayGesCost() *big.Int {

	genaroPrice := self.GetGenaroPrice()
	if genaroPrice != nil {
		if genaroPrice.OneDayMortgageGes != nil {
			return genaroPrice.OneDayMortgageGes.ToInt()
		}
	}

	return common.DefaultOneDayMortgageGes
}

func (self *stateObject)GetOneDaySyncLogGsaCost() *big.Int {
	genaroPrice := self.GetGenaroPrice()
	if genaroPrice != nil {
		if genaroPrice.OneDaySyncLogGsaCost != nil {
			return genaroPrice.OneDaySyncLogGsaCost.ToInt()
		}
	}
	return common.DefaultOneDaySyncLogGsaCost
}
func (self *stateObject)GetLastSynState() *types.LastSynState{
	if self.data.CodeHash != nil {
		var lastSynState types.LastSynState
		json.Unmarshal(self.data.CodeHash, &lastSynState)
		return &lastSynState
	}
	return nil
}

func (self *stateObject)UnbindNode(nodeId string) error{
	var err error
	var genaroData types.GenaroData
	if self.data.CodeHash == nil{
		err = errors.New("no node of this account")
	}else {
		json.Unmarshal(self.data.CodeHash, &genaroData)

		var a []string
		for k, v := range genaroData.Node {
			if v == nodeId {
				a = append(genaroData.Node[:k], genaroData.Node[k+1:]...)
			}
		}
		genaroData.Node = a
		b, _ := json.Marshal(genaroData)
		self.code = nil
		self.data.CodeHash = b[:]
		self.dirtyCode = true
		if self.onDirty != nil {
			self.onDirty(self.Address())
			self.onDirty = nil
		}

	}
	return err
}

func (self *stateObject)UbindNode2Address(nodeId string) error{
	d := make(map[string]string)
	if self.data.CodeHash == nil {
		return nil
	}else{
		json.Unmarshal(self.data.CodeHash, &d)
		delete(d, nodeId)
	}
	b, _ := json.Marshal(d)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
	return nil
}

func (self *stateObject)GetRewardsValues() *types.RewardsValues {
	var rewardsValues types.RewardsValues
	if self.data.CodeHash == nil {
		return nil
	}else{
		json.Unmarshal(self.data.CodeHash, &rewardsValues)
	}
	return &rewardsValues
}

func (self *stateObject)SetRewardsValues(rewardsValues types.RewardsValues) {
	b, _ := json.Marshal(rewardsValues)
	self.code = nil
	self.data.CodeHash = b[:]
	self.dirtyCode = true
	if self.onDirty != nil {
		self.onDirty(self.Address())
		self.onDirty = nil
	}
}

