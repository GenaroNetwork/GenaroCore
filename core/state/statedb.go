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

// Package state provides a caching layer atop the Ethereum state trie.
package state

import (
	"fmt"
	"math/big"
	"sort"
	"sync"

	"bytes"
	"encoding/hex"
	"errors"
	"github.com/GenaroNetwork/GenaroCore/common"
	"github.com/GenaroNetwork/GenaroCore/common/hexutil"
	"github.com/GenaroNetwork/GenaroCore/core/types"
	"github.com/GenaroNetwork/GenaroCore/crypto"
	"github.com/GenaroNetwork/GenaroCore/log"
	"github.com/GenaroNetwork/GenaroCore/rlp"
	"github.com/GenaroNetwork/GenaroCore/trie"
	"time"
)

type revision struct {
	id           int
	journalIndex int
}

var (
	// emptyState is the known hash of an empty state trie entry.
	emptyState = crypto.Keccak256Hash(nil)

	// emptyCode is the known hash of the empty EVM bytecode.
	emptyCode = crypto.Keccak256Hash(nil)
)

// StateDBs within the ethereum protocol are used to store anything
// within the merkle trie. StateDBs take care of caching and storing
// nested states. It's the general query interface to retrieve:
// * Contracts
// * Accounts
type StateDB struct {
	db   Database
	trie Trie

	// This map holds 'live' objects, which will get modified while processing a state transition.
	stateObjects      map[common.Address]*stateObject
	stateObjectsDirty map[common.Address]struct{}

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by StateDB.Commit.
	dbErr error

	// The refund counter, also used by state transitioning.
	refund uint64

	thash, bhash common.Hash
	txIndex      int
	logs         map[common.Hash][]*types.Log
	logSize      uint

	preimages map[common.Hash][]byte

	// Journal of state modifications. This is the backbone of
	// Snapshot and RevertToSnapshot.
	journal        journal
	validRevisions []revision
	nextRevisionId int

	lock sync.Mutex
}

// Create a new state from a given trie.
func New(root common.Hash, db Database) (*StateDB, error) {
	tr, err := db.OpenTrie(root)
	if err != nil {
		return nil, err
	}
	return &StateDB{
		db:                db,
		trie:              tr,
		stateObjects:      make(map[common.Address]*stateObject),
		stateObjectsDirty: make(map[common.Address]struct{}),
		logs:              make(map[common.Hash][]*types.Log),
		preimages:         make(map[common.Hash][]byte),
	}, nil
}

// setError remembers the first non-nil error it is called with.
func (self *StateDB) setError(err error) {
	if self.dbErr == nil {
		self.dbErr = err
	}
}

func (self *StateDB) Error() error {
	return self.dbErr
}

// Reset clears out all ephemeral state objects from the state db, but keeps
// the underlying state trie to avoid reloading data for the next operations.
func (self *StateDB) Reset(root common.Hash) error {
	tr, err := self.db.OpenTrie(root)
	if err != nil {
		return err
	}
	self.trie = tr
	self.stateObjects = make(map[common.Address]*stateObject)
	self.stateObjectsDirty = make(map[common.Address]struct{})
	self.thash = common.Hash{}
	self.bhash = common.Hash{}
	self.txIndex = 0
	self.logs = make(map[common.Hash][]*types.Log)
	self.logSize = 0
	self.preimages = make(map[common.Hash][]byte)
	self.clearJournalAndRefund()
	return nil
}

func (self *StateDB) AddLog(log *types.Log) {
	self.journal = append(self.journal, addLogChange{txhash: self.thash})

	log.TxHash = self.thash
	log.BlockHash = self.bhash
	log.TxIndex = uint(self.txIndex)
	log.Index = self.logSize
	self.logs[self.thash] = append(self.logs[self.thash], log)
	self.logSize++
}

func (self *StateDB) GetLogs(hash common.Hash) []*types.Log {
	return self.logs[hash]
}

func (self *StateDB) Logs() []*types.Log {
	var logs []*types.Log
	for _, lgs := range self.logs {
		logs = append(logs, lgs...)
	}
	return logs
}

// AddPreimage records a SHA3 preimage seen by the VM.
func (self *StateDB) AddPreimage(hash common.Hash, preimage []byte) {
	if _, ok := self.preimages[hash]; !ok {
		self.journal = append(self.journal, addPreimageChange{hash: hash})
		pi := make([]byte, len(preimage))
		copy(pi, preimage)
		self.preimages[hash] = pi
	}
}

// Preimages returns a list of SHA3 preimages that have been submitted.
func (self *StateDB) Preimages() map[common.Hash][]byte {
	return self.preimages
}

func (self *StateDB) AddRefund(gas uint64) {
	self.journal = append(self.journal, refundChange{prev: self.refund})
	self.refund += gas
}

// Exist reports whether the given account address exists in the state.
// Notably this also returns true for suicided accounts.
func (self *StateDB) Exist(addr common.Address) bool {
	return self.getStateObject(addr) != nil
}

// Empty returns whether the state object is either non-existent
// or empty according to the EIP161 specification (balance = nonce = code = 0)
func (self *StateDB) Empty(addr common.Address) bool {
	so := self.getStateObject(addr)
	return so == nil || so.empty()
}

// Retrieve the balance from the given address or 0 if object not found
func (self *StateDB) GetBalance(addr common.Address) *big.Int {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Balance()
	}
	return common.Big0
}

func (self *StateDB) GetNonce(addr common.Address) uint64 {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Nonce()
	}

	return 0
}

func (self *StateDB) GetCode(addr common.Address) []byte {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Code(self.db)
	}
	return nil
}

func (self *StateDB) IsContract(addr common.Address) bool {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.IsContract()
	}
	return false
}

func (self *StateDB) GetCodeSize(addr common.Address) int {
	stateObject := self.getStateObject(addr)
	if stateObject == nil {
		return 0
	}
	if stateObject.code != nil {
		return len(stateObject.code)
	}
	size, err := self.db.ContractCodeSize(stateObject.addrHash, common.BytesToHash(stateObject.CodeHash()))
	if err != nil {
		self.setError(err)
	}
	return size
}

func (self *StateDB) GetCodeHash(addr common.Address) common.Hash {
	stateObject := self.getStateObject(addr)
	if stateObject == nil {
		return common.Hash{}
	}
	return common.BytesToHash(stateObject.CodeHash())
}

// only used in genaro
func (self *StateDB) GetGenaroCodeHash(addr common.Address) string {
	stateObject := self.getStateObject(addr)
	if stateObject == nil {
		return ""
	}
	return hexutil.Encode(stateObject.CodeHash())
}

func (self *StateDB) GetState(a common.Address, b common.Hash) common.Hash {
	stateObject := self.getStateObject(a)
	if stateObject != nil {
		return stateObject.GetState(self.db, b)
	}
	return common.Hash{}
}

// Database retrieves the low level database supporting the lower level trie ops.
func (self *StateDB) Database() Database {
	return self.db
}

// StorageTrie returns the storage trie of an account.
// The return value is a copy and is nil for non-existent accounts.
func (self *StateDB) StorageTrie(a common.Address) Trie {
	stateObject := self.getStateObject(a)
	if stateObject == nil {
		return nil
	}
	cpy := stateObject.deepCopy(self, nil)
	return cpy.updateTrie(self.db)
}

func (self *StateDB) HasSuicided(addr common.Address) bool {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.suicided
	}
	return false
}

/*
 * SETTERS
 */

// AddBalance adds amount to the account associated with addr
func (self *StateDB) AddBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.AddBalance(amount)
	}
}

// SubBalance subtracts amount from the account associated with addr
func (self *StateDB) SubBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SubBalance(amount)
	}
}

func (self *StateDB) SetBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetBalance(amount)
	}
}

func (self *StateDB) SetNonce(addr common.Address, nonce uint64) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetNonce(nonce)
	}
}

func (self *StateDB) SetCode(addr common.Address, code []byte) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetCode(crypto.Keccak256Hash(code), code)
	}
}

// only used in genaro genesis init
func (self *StateDB) SetCodeHash(addr common.Address, codeHash []byte) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetCodeHash(codeHash)
	}
}

func (self *StateDB) SetState(addr common.Address, key common.Hash, value common.Hash) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetState(self.db, key, value)
	}
}

func (self *StateDB) GetNameAccount(name string) (addr common.Address, err error) {
	var accountName types.AccountName
	err = accountName.SetString(name)
	if err != nil {
		return
	}
	addr = self.GetState(common.NameSpaceSaveAddress, accountName.ToHash()).Address()
	return
}

func (self *StateDB) SetNameAccount(name string, addr common.Address) (err error) {
	if len(name) > common.HashLength {
		return errors.New("name is too long")
	}
	var accountName types.AccountName
	err = accountName.SetString(name)
	if err != nil {
		return
	}
	nonce := self.GetNonce(common.NameSpaceSaveAddress)
	if nonce == 0 {
		self.SetNonce(common.NameSpaceSaveAddress, 1)
	}
	self.SetState(common.NameSpaceSaveAddress, accountName.ToHash(), addr.Hash())
	return
}

func (self *StateDB) IsNameAccountExist(name string) (bool, error) {
	addr, err := self.GetNameAccount(name)
	if err != nil {
		return true, err
	}
	if 0 == bytes.Compare(addr.Hash().Bytes(), common.Hash{}.Bytes()) {
		return false, nil
	}
	return true, nil
}

func (self *StateDB) HasName(addr common.Address, name string) bool {
	nameAddr, err := self.GetNameAccount(name)
	if err != nil || addr != nameAddr {
		return false
	}
	return true
}

// Suicide marks the given account as suicided.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// getStateObject will return a non-nil account after Suicide.
func (self *StateDB) Suicide(addr common.Address) bool {
	stateObject := self.getStateObject(addr)
	if stateObject == nil {
		return false
	}
	self.journal = append(self.journal, suicideChange{
		account:     &addr,
		prev:        stateObject.suicided,
		prevbalance: new(big.Int).Set(stateObject.Balance()),
	})
	stateObject.markSuicided()
	stateObject.data.Balance = new(big.Int)

	return true
}

//
// Setting, updating & deleting state object methods
//

// updateStateObject writes the given object to the trie.
func (self *StateDB) updateStateObject(stateObject *stateObject) {
	addr := stateObject.Address()
	data, err := rlp.EncodeToBytes(stateObject)
	if err != nil {
		panic(fmt.Errorf("can't encode object at %x: %v", addr[:], err))
	}
	self.setError(self.trie.TryUpdate(addr[:], data))
}

// deleteStateObject removes the given object from the state trie.
func (self *StateDB) deleteStateObject(stateObject *stateObject) {
	stateObject.deleted = true
	addr := stateObject.Address()
	self.setError(self.trie.TryDelete(addr[:]))
}

// Retrieve a state object given my the address. Returns nil if not found.
func (self *StateDB) getStateObject(addr common.Address) (stateObject *stateObject) {
	// Prefer 'live' objects.
	if obj := self.stateObjects[addr]; obj != nil {
		if obj.deleted {
			return nil
		}
		return obj
	}

	// Load the object from the database.
	enc, err := self.trie.TryGet(addr[:])
	if len(enc) == 0 {
		self.setError(err)
		return nil
	}
	var data Account
	if err := rlp.DecodeBytes(enc, &data); err != nil {
		log.Error("Failed to decode state object", "addr", addr, "err", err)
		return nil
	}
	// Insert into the live set.
	obj := newObject(self, addr, data, self.MarkStateObjectDirty)
	self.setStateObject(obj)
	return obj
}

func (self *StateDB) setStateObject(object *stateObject) {
	self.stateObjects[object.Address()] = object
}

// Retrieve a state object or create a new state object if nil
func (self *StateDB) GetOrNewStateObject(addr common.Address) *stateObject {
	stateObject := self.getStateObject(addr)
	if stateObject == nil || stateObject.deleted {
		stateObject, _ = self.createObject(addr)
	}
	return stateObject
}

// MarkStateObjectDirty adds the specified object to the dirty map to avoid costly
// state object cache iteration to find a handful of modified ones.
func (self *StateDB) MarkStateObjectDirty(addr common.Address) {
	self.stateObjectsDirty[addr] = struct{}{}
}

// createObject creates a new state object. If there is an existing account with
// the given address, it is overwritten and returned as the second return value.
func (self *StateDB) createObject(addr common.Address) (newobj, prev *stateObject) {
	prev = self.getStateObject(addr)
	newobj = newObject(self, addr, Account{}, self.MarkStateObjectDirty)
	newobj.setNonce(0) // sets the object to dirty
	if prev == nil {
		self.journal = append(self.journal, createObjectChange{account: &addr})
	} else {
		self.journal = append(self.journal, resetObjectChange{prev: prev})
	}
	self.setStateObject(newobj)
	return newobj, prev
}

// CreateAccount explicitly creates a state object. If a state object with the address
// already exists the balance is carried over to the new account.
//
// CreateAccount is called during the EVM CREATE operation. The situation might arise that
// a contract does the following:
//
//   1. sends funds to sha(account ++ (nonce + 1))
//   2. tx_create(sha(account ++ nonce)) (note that this gets the address of 1)
//
// Carrying over the balance ensures that Ether doesn't disappear.
func (self *StateDB) CreateAccount(addr common.Address) {
	new, prev := self.createObject(addr)
	if prev != nil {
		new.setBalance(prev.data.Balance)
	}
}

func (db *StateDB) ForEachStorage(addr common.Address, cb func(key, value common.Hash) bool) {
	so := db.getStateObject(addr)
	if so == nil {
		return
	}

	// When iterating over the storage check the cache first
	for h, value := range so.cachedStorage {
		cb(h, value)
	}

	it := trie.NewIterator(so.getTrie(db.db).NodeIterator(nil))
	for it.Next() {
		// ignore cached values
		key := common.BytesToHash(db.trie.GetKey(it.Key))
		if _, ok := so.cachedStorage[key]; !ok {
			cb(key, common.BytesToHash(it.Value))
		}
	}
}

// Copy creates a deep, independent copy of the state.
// Snapshots of the copied state cannot be applied to the copy.
func (self *StateDB) Copy() *StateDB {
	self.lock.Lock()
	defer self.lock.Unlock()

	// Copy all the basic fields, initialize the memory ones
	state := &StateDB{
		db:                self.db,
		trie:              self.db.CopyTrie(self.trie),
		stateObjects:      make(map[common.Address]*stateObject, len(self.stateObjectsDirty)),
		stateObjectsDirty: make(map[common.Address]struct{}, len(self.stateObjectsDirty)),
		refund:            self.refund,
		logs:              make(map[common.Hash][]*types.Log, len(self.logs)),
		logSize:           self.logSize,
		preimages:         make(map[common.Hash][]byte),
	}
	// Copy the dirty states, logs, and preimages
	for addr := range self.stateObjectsDirty {
		state.stateObjects[addr] = self.stateObjects[addr].deepCopy(state, state.MarkStateObjectDirty)
		state.stateObjectsDirty[addr] = struct{}{}
	}
	for hash, logs := range self.logs {
		state.logs[hash] = make([]*types.Log, len(logs))
		copy(state.logs[hash], logs)
	}
	for hash, preimage := range self.preimages {
		state.preimages[hash] = preimage
	}
	return state
}

// Snapshot returns an identifier for the current revision of the state.
func (self *StateDB) Snapshot() int {
	id := self.nextRevisionId
	self.nextRevisionId++
	self.validRevisions = append(self.validRevisions, revision{id, len(self.journal)})
	return id
}

// RevertToSnapshot reverts all state changes made since the given revision.
func (self *StateDB) RevertToSnapshot(revid int) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(self.validRevisions), func(i int) bool {
		return self.validRevisions[i].id >= revid
	})
	if idx == len(self.validRevisions) || self.validRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := self.validRevisions[idx].journalIndex

	// Replay the journal to undo changes.
	for i := len(self.journal) - 1; i >= snapshot; i-- {
		self.journal[i].undo(self)
	}
	self.journal = self.journal[:snapshot]

	// Remove invalidated snapshots from the stack.
	self.validRevisions = self.validRevisions[:idx]
}

// GetRefund returns the current value of the refund counter.
func (self *StateDB) GetRefund() uint64 {
	return self.refund
}

// Finalise finalises the state by removing the self destructed objects
// and clears the journal as well as the refunds.
func (s *StateDB) Finalise(deleteEmptyObjects bool) {
	for addr := range s.stateObjectsDirty {
		stateObject, exist := s.stateObjects[addr]
		if !exist {
			continue
		}
		if stateObject.suicided || (deleteEmptyObjects && stateObject.empty()) {
			s.deleteStateObject(stateObject)
		} else {
			stateObject.updateRoot(s.db)
			s.updateStateObject(stateObject)
		}
	}
	// Invalidate journal because reverting across transactions is not allowed.
	s.clearJournalAndRefund()
}

// IntermediateRoot computes the current root hash of the state trie.
// It is called in between transactions to get the root hash that
// goes into transaction receipts.
func (s *StateDB) IntermediateRoot(deleteEmptyObjects bool) common.Hash {
	s.Finalise(deleteEmptyObjects)
	return s.trie.Hash()
}

// Prepare sets the current transaction hash and index and block hash which is
// used when the EVM emits new state logs.
func (self *StateDB) Prepare(thash, bhash common.Hash, ti int) {
	self.thash = thash
	self.bhash = bhash
	self.txIndex = ti
}

// DeleteSuicides flags the suicided objects for deletion so that it
// won't be referenced again when called / queried up on.
//
// DeleteSuicides should not be used for consensus related updates
// under any circumstances.
func (s *StateDB) DeleteSuicides() {
	// Reset refund so that any used-gas calculations can use this method.
	s.clearJournalAndRefund()

	for addr := range s.stateObjectsDirty {
		stateObject := s.stateObjects[addr]

		// If the object has been removed by a suicide
		// flag the object as deleted.
		if stateObject.suicided {
			stateObject.deleted = true
		}
		delete(s.stateObjectsDirty, addr)
	}
}

func (s *StateDB) clearJournalAndRefund() {
	s.journal = nil
	s.validRevisions = s.validRevisions[:0]
	s.refund = 0
}

// Commit writes the state to the underlying in-memory trie database.
func (s *StateDB) Commit(deleteEmptyObjects bool) (root common.Hash, err error) {
	defer s.clearJournalAndRefund()

	// Commit objects to the trie.
	for addr, stateObject := range s.stateObjects {
		_, isDirty := s.stateObjectsDirty[addr]
		switch {
		case stateObject.suicided || (isDirty && deleteEmptyObjects && stateObject.empty()):
			// If the object has been removed, don't bother syncing it
			// and just mark it for deletion in the trie.
			s.deleteStateObject(stateObject)
		case isDirty:
			// Write any contract code associated with the state object
			if stateObject.code != nil && stateObject.dirtyCode {
				s.db.TrieDB().Insert(common.BytesToHash(stateObject.CodeHash()), stateObject.code)
				stateObject.dirtyCode = false
			}
			// Write any storage changes in the state object to its storage trie.
			if err := stateObject.CommitTrie(s.db); err != nil {
				return common.Hash{}, err
			}
			// Update the object in the main account trie.
			s.updateStateObject(stateObject)
		}
		delete(s.stateObjectsDirty, addr)
	}
	// Write trie changes.
	root, err = s.trie.Commit(func(leaf []byte, parent common.Hash) error {
		var account Account
		if err := rlp.DecodeBytes(leaf, &account); err != nil {
			return nil
		}
		if account.Root != emptyState {
			s.db.TrieDB().Reference(account.Root, parent)
		}
		code := common.BytesToHash(account.CodeHash)
		if code != emptyCode && !CheckCodeEmpty(account.CodeHash) {
			s.db.TrieDB().Reference(code, parent)
		}
		return nil
	})
	log.Debug("Trie cache stats after commit", "misses", trie.CacheMisses(), "unloads", trie.CacheUnloads())
	return root, err
}

func (self *StateDB) UpdateHeft(id common.Address, heft uint64, blockNumber uint64) bool {
	stateObject := self.GetOrNewStateObject(id)
	if stateObject != nil {
		stateObject.UpdateHeft(heft, blockNumber)
		return true
	}
	return false
}

func (self *StateDB) GetHeft(id common.Address) (uint64, error) {
	stateObject := self.getStateObject(id)
	if stateObject != nil {
		return stateObject.GetHeft(), nil
	}
	return 0, nil
}

func (self *StateDB) GetHeftLog(id common.Address) types.NumLogs {
	stateObject := self.getStateObject(id)
	if stateObject != nil {
		return stateObject.GetHeftLog()
	}
	return nil
}

func (self *StateDB) GetHeftRangeDiff(id common.Address, blockNumStart uint64, blockNumEnd uint64) uint64 {
	stateObject := self.getStateObject(id)
	if stateObject != nil {
		return stateObject.GetHeftRangeDiff(blockNumStart, blockNumEnd)
	}
	return 0
}

func (self *StateDB) GetHeftLastDiff(id common.Address, lastBlockNum uint64) uint64 {
	stateObject := self.getStateObject(id)
	if stateObject != nil {
		logs := stateObject.GetHeftLog()
		diff, blockNum := logs.GetLastDiff()
		if blockNum != lastBlockNum {
			diff = 0
		}
		return diff
	}
	return 0
}

func (self *StateDB) UpdateStake(id common.Address, stake uint64, blockNumber uint64) bool {
	stateObject := self.GetOrNewStateObject(id)
	if stateObject != nil {
		stateObject.UpdateStake(stake, blockNumber)
		return true
	}
	return false
}

func (self *StateDB) DeleteStake(id common.Address, stake uint64, blockNumber uint64) (bool, uint64) {
	stateObject := self.GetOrNewStateObject(id)
	if stateObject != nil {
		alreadyPunishment := stateObject.DeleteStake(stake, blockNumber)
		return true, alreadyPunishment
	}
	return false, 0
}

func (self *StateDB) BackStake(id common.Address, blockNumber uint64) (bool, uint64) {
	stateObject := self.GetOrNewStateObject(id)
	if stateObject != nil {
		stake := stateObject.GetStake()
		stateObject.DeleteStake(stake, blockNumber)
		mount := big.NewInt(int64(stake))
		mount.Mul(mount, big.NewInt(1000000000000000000))
		stateObject.AddBalance(mount)
		return true, stake
	}
	return false, 0
}

func (self *StateDB) GetStake(id common.Address) (uint64, error) {
	stateObject := self.getStateObject(id)
	if stateObject != nil {
		return stateObject.GetStake(), nil
	}
	return 0, nil
}

func (self *StateDB) GetStakeLog(id common.Address) types.NumLogs {
	stateObject := self.getStateObject(id)
	if stateObject != nil {
		return stateObject.GetStakeLog()
	}
	return nil
}

func (self *StateDB) GetStakeRangeDiff(id common.Address, blockNumStart uint64, blockNumEnd uint64) uint64 {
	stateObject := self.getStateObject(id)
	if stateObject != nil {
		return stateObject.GetStakeRangeDiff(blockNumStart, blockNumEnd)
	}
	return 0
}

func (self *StateDB) AddCandidate(candidate common.Address) bool {
	stateBindingObject := self.GetOrNewStateObject(common.BindingSaveAddress)
	if stateBindingObject != nil && stateBindingObject.IsBindingAccount(candidate) {
		return true
	}

	stateObject := self.GetOrNewStateObject(common.CandidateSaveAddress)
	if stateObject != nil {
		stateObject.AddCandidate(candidate)
		return true
	}
	return false
}

func (self *StateDB) IsCandidateExist(candidate common.Address) bool {
	stateObject := self.GetOrNewStateObject(common.CandidateSaveAddress)
	if stateObject != nil {
		return stateObject.IsCandidateExist(candidate)
	}
	return false
}

func (self *StateDB) DelCandidate(candidate common.Address) bool {
	stateObject := self.GetOrNewStateObject(common.CandidateSaveAddress)
	if stateObject != nil {
		stateObject.DelCandidate(candidate)
		return true
	}
	return false
}

func (self *StateDB) GetCandidates() Candidates {
	stateObject := self.getStateObject(common.CandidateSaveAddress)
	if stateObject != nil {
		return stateObject.GetCandidates()
	}
	return nil
}

func (self *StateDB) GetCommitteeRank(blockNumStart uint64, blockNumEnd uint64) ([]common.Address, []uint64) {
	stateObject := self.getStateObject(common.CandidateSaveAddress)
	if stateObject != nil {
		candidateInfos := self.GetCandidatesInfoInRange(blockNumStart, blockNumEnd)
		return Rank(candidateInfos)
	}
	return nil, nil
}

func (self *StateDB) GetMainAccountRank() ([]common.Address, []uint64) {
	stateObject := self.getStateObject(common.CandidateSaveAddress)
	if stateObject != nil {
		candidateInfos := self.GetCandidatesInfoWithAllSubAccounts()
		return Rank(candidateInfos)
	}
	return nil, nil
}

func (self *StateDB) GetCandidatesInfoInRange(blockNumStart uint64, blockNumEnd uint64) []CandidateInfo {
	stateObject := self.getStateObject(common.CandidateSaveAddress)
	if stateObject != nil {
		candidates := stateObject.GetCandidates()
		CandidateInfoArray := make([]CandidateInfo, len(candidates))
		for id, candidate := range candidates {
			CandidateInfoArray[id].Signer = candidate
			CandidateInfoArray[id].Heft = self.GetHeftRangeDiff(candidate, blockNumStart, blockNumEnd)
			CandidateInfoArray[id].Stake = self.GetStakeRangeDiff(candidate, blockNumStart, blockNumEnd)
		}
		return CandidateInfoArray
	}
	return nil
}

func (self *StateDB) GetCandidatesInfoWithAllSubAccounts() []CandidateInfo {
	stateObject := self.getStateObject(common.CandidateSaveAddress)
	if stateObject != nil {
		candidates := stateObject.GetCandidates()
		CandidateInfoArray := make([]CandidateInfo, len(candidates))
		for id, candidate := range candidates {
			CandidateInfoArray[id] = self.GetCandidateInfoWithAllSubAccounts(candidate)
		}
		return CandidateInfoArray
	}
	return nil
}

func (self *StateDB) GetCandidateInfoWithAllSubAccounts(candidate common.Address) (candidateInfo CandidateInfo) {
	candidateInfo.Signer = candidate
	candidateInfo.Heft, _ = self.GetHeft(candidate)
	candidateInfo.Stake, _ = self.GetStake(candidate)
	subAccounts := self.GetSubAccounts(candidate)
	for _, subAccount := range subAccounts {
		heft, _ := self.GetHeft(subAccount)
		stake, _ := self.GetStake(subAccount)
		candidateInfo.Heft += heft
		candidateInfo.Stake += stake
	}
	return
}

func (self *StateDB) UpdateBucketProperties(userid common.Address, bucketid string, size uint64, backup uint64, timestart uint64, timeend uint64) bool {
	stateObject := self.GetOrNewStateObject(userid)
	if stateObject != nil {
		stateObject.UpdateBucketProperties(bucketid, size, backup, timestart, timeend)
		return true
	}
	return true
}

func (self *StateDB) UpdateBucket(addr common.Address, bucket types.BucketPropertie) bool {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.UpdateBucket(bucket)
	}
	return false
}

func (self *StateDB) GetStorageSize(userid common.Address, bucketID [32]byte) (uint64, error) {
	stateObject := self.getStateObject(userid)
	if stateObject != nil {
		return stateObject.GetStorageSize(string(bucketID[:])), nil
	}
	return 0, nil
}

func (self *StateDB) GetStorageGasPrice(userid common.Address, bucketID [32]byte) (uint64, error) {
	stateObject := self.getStateObject(userid)
	if stateObject != nil {
		return stateObject.GetStorageGasPrice(string(bucketID[:])), nil
	}
	return 0, nil
}

func (self *StateDB) GetStorageGasUsed(userid common.Address, bucketID [32]byte) (uint64, error) {
	stateObject := self.getStateObject(userid)
	if stateObject != nil {
		return stateObject.GetStorageGasUsed(string(bucketID[:])), nil
	}
	return 0, nil
}

func (self *StateDB) GetStorageGas(userid common.Address, bucketID [32]byte) (uint64, error) {
	stateObject := self.getStateObject(userid)
	if stateObject != nil {
		return stateObject.GetStorageGas(string(bucketID[:])), nil
	}
	return 0, nil
}

func (self *StateDB) SpecialTxTypeMortgageInit(address common.Address, specialTxTypeMortgageInit types.SpecialTxTypeMortgageInit) bool {
	stateObject := self.GetOrNewStateObject(address)
	if stateObject != nil {
		return stateObject.SpecialTxTypeMortgageInit(specialTxTypeMortgageInit)
	}
	return false
}

func (self *StateDB) SynchronizeShareKey(address common.Address, synchronizeShareKey types.SynchronizeShareKey) bool {
	stateObject := self.GetOrNewStateObject(address)
	if stateObject != nil {
		return stateObject.SynchronizeShareKey(synchronizeShareKey)
	}
	return false
}

func (self *StateDB) UpdateTraffic(id common.Address, traffic uint64) bool {
	stateObject := self.GetOrNewStateObject(id)
	if stateObject != nil {
		stateObject.UpdateTraffic(traffic)
		return true
	}
	return false
}

func (self *StateDB) GetTraffic(addr common.Address) uint64 {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetTraffic()
	}
	return 0
}

func (self *StateDB) GetBuckets(addr common.Address) (map[string]interface{}, error) {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetBuckets(), nil
	}
	return nil, nil
}

func (self *StateDB) GetStorageNodes(addr common.Address) []string {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetStorageNodes()
	}
	return nil
}

func (self *StateDB) TxLogByDataVersionRead(address common.Address, fileID [32]byte, dataVersion string) (map[common.Address]*hexutil.Big, error) {
	fileIDToString := hex.EncodeToString(fileID[:])
	stateObject := self.getStateObject(address)
	if stateObject != nil {
		return stateObject.TxLogByDataVersionRead(fileIDToString, dataVersion)
	}
	return nil, nil
}

func (self *StateDB) TxLogBydataVersionUpdate(address common.Address, fileID [32]byte, OfficialAddress common.Address) bool {
	fileIDToString := hex.EncodeToString(fileID[:])
	stateObject := self.getStateObject(address)
	if stateObject != nil {
		resultTmp, tag := stateObject.TxLogBydataVersionUpdate(fileIDToString)
		if !tag {
			return false
		}
		TimeLimit := (resultTmp.EndTime - time.Now().Unix()) / 86400
		tmp := big.NewInt(TimeLimit * int64(len(resultTmp.MortgageTable)))
		timeLimitGas := tmp.Mul(tmp, self.GetOneDaySyncLogGsaCost())
		stateObject.setBalance(timeLimitGas)
		newStateObject := self.getStateObject(OfficialAddress)
		newStateObject.AddBalance(timeLimitGas)
		return true
	}
	return false
}

func (self *StateDB) GetAccountAttributes(addr common.Address) types.GenaroData {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetAccountAttributes()
	}
	return types.GenaroData{}
}

func (self *StateDB) SpecialTxTypeSyncSidechainStatus(address common.Address, SpecialTxTypeSyncSidechainStatus types.SpecialTxTypeMortgageInit) (map[common.Address]*big.Int, bool) {
	stateObject := self.GetOrNewStateObject(address)
	if stateObject != nil {
		restlt, flag := stateObject.SpecialTxTypeSyncSidechainStatus(SpecialTxTypeSyncSidechainStatus)
		if true == flag {
			return restlt, true
		}
	}
	return nil, false
}

func (self *StateDB) SyncStakeNode(address common.Address, s string) error {
	stateObject := self.GetOrNewStateObject(address)
	var err error = nil
	if stateObject != nil {
		err = stateObject.SyncStakeNode(s)
	}
	return err
}

func (self *StateDB) SyncNode2Address(node2UserAccountIndexAddress common.Address, s string, userAddress string) error {
	stateObject := self.GetOrNewStateObject(node2UserAccountIndexAddress)
	var err error = nil
	if stateObject != nil {
		err = stateObject.SyncNode2Address(s, userAddress)
	}
	return err
}

func (self *StateDB) GetAddressByNode(s string) string {
	stateObject := self.GetOrNewStateObject(common.StakeNode2StakeAddress)
	var address string
	if stateObject != nil {
		address = stateObject.GetAddressByNode(s)
	}
	return address
}

func (self *StateDB) AddAlreadyBackStack(backStack common.AlreadyBackStake) bool {
	stateObject := self.GetOrNewStateObject(common.BackStakeAddress)
	if stateObject != nil {
		stateObject.AddAlreadyBackStack(backStack)
		return true
	}
	return false
}

func (self *StateDB) GetAlreadyBackStakeList() (bool, common.BackStakeList) {
	stateObject := self.GetOrNewStateObject(common.BackStakeAddress)
	if stateObject != nil {
		backStacks := stateObject.GetAlreadyBackStakeList()
		return true, backStacks
	}
	return false, nil
}

func (self *StateDB) IsAlreadyBackStake(addr common.Address) bool {
	ok, backStakeList := self.GetAlreadyBackStakeList()
	if !ok {
		return ok
	}
	return backStakeList.IsAccountExist(addr)
}

func (self *StateDB) SetAlreadyBackStakeList(backStacks common.BackStakeList) bool {
	stateObject := self.GetOrNewStateObject(common.BackStakeAddress)
	if stateObject != nil {
		stateObject.SetAlreadyBackStakeList(backStacks)
		return true
	}
	return false
}

func (self *StateDB) UpdateFileSharePublicKey(id common.Address, publicKey string) bool {
	stateObject := self.GetOrNewStateObject(id)
	if stateObject != nil {
		stateObject.UpdateFileSharePublicKey(publicKey)
		return true
	}
	return false
}

func (self *StateDB) GetFileSharePublicKey(addr common.Address) string {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetFileSharePublicKey()
	}
	return ""
}

func (self *StateDB) UnlockSharedKey(address common.Address, shareKeyId string) bool {
	stateObject := self.GetOrNewStateObject(address)
	if stateObject != nil {
		synchronizeShareKey := stateObject.UnlockSharedKey(shareKeyId)
		var synchronizeShareKeyTmp types.SynchronizeShareKey
		if synchronizeShareKeyTmp == synchronizeShareKey {
			return false
		}
		if "" != synchronizeShareKey.ShareKeyId && 0 == synchronizeShareKey.Status {
			balance := self.GetBalance(address)
			if balance.Cmp(synchronizeShareKey.Shareprice.ToInt()) <= 0 {
				return false
			}
			stateObject.SubBalance(synchronizeShareKey.Shareprice.ToInt())
			FromAccountstateObject := self.GetOrNewStateObject(synchronizeShareKey.FromAccount)
			FromAccountstateObject.AddBalance(synchronizeShareKey.Shareprice.ToInt())
		}
		return true
	}
	return false
}

func (self *StateDB) GetSharedFile(address common.Address, shareKeyId string) types.SynchronizeShareKey {
	stateObject := self.GetOrNewStateObject(address)
	if stateObject != nil {
		return stateObject.UnlockSharedKey(shareKeyId)
	}
	return types.SynchronizeShareKey{}
}

func (self *StateDB) CheckUnlockSharedKey(address common.Address, shareKeyId string) bool {
	stateObject := self.getStateObject(address)
	if stateObject != nil {
		return stateObject.CheckUnlockSharedKey(shareKeyId)
	}
	return false
}

func (self *StateDB) UpdateBucketApplyPrice(address common.Address, price *hexutil.Big) bool {
	stateObject := self.GetOrNewStateObject(address)
	if stateObject != nil {
		stateObject.UpdateBucketApplyPrice(price)
		return true
	}
	return false
}

func (self *StateDB) AddLastRootState(statehash common.Hash, blockNumber uint64) bool {
	stateObject := self.getStateObject(common.LastSynStateSaveAddress)
	if stateObject != nil {
		stateObject.AddLastRootState(statehash, blockNumber)
		return true
	}
	return false
}

func (self *StateDB) UpdateAccountBinding(mainAccount common.Address, subAccount common.Address) bool {
	stateObject := self.GetOrNewStateObject(common.BindingSaveAddress)
	if stateObject != nil {
		stateObject.UpdateAccountBinding(mainAccount, subAccount)
		return true
	}
	return false
}

func (self *StateDB) DelSubAccountBinding(subAccount common.Address) bool {
	stateObject := self.GetOrNewStateObject(common.BindingSaveAddress)
	if stateObject != nil {
		return stateObject.DelSubAccountBinding(subAccount)
	}
	return false
}

func (self *StateDB) GetSubAccountsCount(mainAccount common.Address) int {
	stateObject := self.getStateObject(common.BindingSaveAddress)
	if stateObject != nil {
		return stateObject.GetSubAccountsCount(mainAccount)
	}
	return 0
}

func (self *StateDB) GetSubAccounts(mianAccount common.Address) []common.Address {
	stateObject := self.getStateObject(common.BindingSaveAddress)
	if stateObject != nil {
		mainAccount := stateObject.GetSubAccounts(mianAccount)
		return mainAccount
	}
	return nil
}

func (self *StateDB) GetMainAccounts() map[common.Address][]common.Address {
	stateObject := self.getStateObject(common.BindingSaveAddress)
	if stateObject != nil {
		mainAccounts := stateObject.GetMainAccounts()
		return mainAccounts
	}
	return nil
}

func (self *StateDB) DelMainAccountBinding(mianAccount common.Address) []common.Address {
	stateObject := self.getStateObject(common.BindingSaveAddress)
	if stateObject != nil {
		subAccounts := stateObject.DelMainAccountBinding(mianAccount)
		return subAccounts
	}
	return nil
}

func (self *StateDB) GetMainAccount(subAccount common.Address) *common.Address {
	stateObject := self.getStateObject(common.BindingSaveAddress)
	if stateObject != nil {
		mainAccount := stateObject.GetMainAccount(subAccount)
		return mainAccount
	}
	return nil
}

func (self *StateDB) IsBindingSubAccount(account common.Address) bool {
	stateObject := self.getStateObject(common.BindingSaveAddress)
	if stateObject != nil {
		return stateObject.IsSubAccount(account)
	}
	return false
}

func (self *StateDB) IsBindingMainAccount(account common.Address) bool {
	stateObject := self.getStateObject(common.BindingSaveAddress)
	if stateObject != nil {
		return stateObject.IsMainAccount(account)
	}
	return false
}

func (self *StateDB) IsBindingAccount(account common.Address) bool {
	stateObject := self.getStateObject(common.BindingSaveAddress)
	if stateObject != nil {
		return stateObject.IsBindingAccount(account)
	}
	return false
}

func (self *StateDB) GetBucketApplyPrice() *big.Int {
	stateObject := self.GetOrNewStateObject(common.GenaroPriceAddress)
	if stateObject != nil {
		return stateObject.GetBucketApplyPrice()
	}
	return common.DefaultBucketApplyGasPerGPerDay
}

func (self *StateDB) UpdateTrafficApplyPrice(address common.Address, price *hexutil.Big) bool {
	stateObject := self.GetOrNewStateObject(address)
	if stateObject != nil {
		stateObject.UpdateTrafficApplyPrice(price)
		return true
	}
	return false
}

func (self *StateDB) SetLastSynBlock(blockNumber uint64, blockHash common.Hash) bool {
	stateObject := self.getStateObject(common.LastSynStateSaveAddress)
	if stateObject != nil {
		stateObject.SetLastSynBlock(blockNumber, blockHash)
		return true
	}
	return false
}

func (self *StateDB) GetTrafficApplyPrice() *big.Int {
	stateObject := self.GetOrNewStateObject(common.GenaroPriceAddress)
	if stateObject != nil {
		return stateObject.GetTrafficApplyPrice()
	}
	return common.DefaultTrafficApplyGasPerG
}

func (self *StateDB) UpdateStakePerNodePrice(address common.Address, price *hexutil.Big) bool {
	stateObject := self.GetOrNewStateObject(address)
	if stateObject != nil {
		stateObject.UpdateStakePerNodePrice(price)
		return true
	}
	return false
}

func (self *StateDB) GetStakePerNodePrice() *big.Int {
	stateObject := self.GetOrNewStateObject(common.GenaroPriceAddress)
	if stateObject != nil {
		return stateObject.GetStakePerNodePrice()
	}
	return common.DefaultStakeValuePerNode
}

func (self *StateDB) GetGenaroPrice() *types.GenaroPrice {
	stateObject := self.GetOrNewStateObject(common.GenaroPriceAddress)
	if stateObject != nil {
		return stateObject.GetGenaroPrice()
	}
	return nil

}

func (self *StateDB) SetGenaroPrice(genaroPrice types.GenaroPrice) bool {
	stateObject := self.GetOrNewStateObject(common.GenaroPriceAddress)
	if stateObject != nil {
		stateObject.SetGenaroPrice(genaroPrice)
		return true
	}
	return false
}

func (self *StateDB) GetLastSynState() *types.LastSynState {
	stateObject := self.getStateObject(common.LastSynStateSaveAddress)
	if stateObject != nil {
		return stateObject.GetLastSynState()
	}
	return nil
}

func (self *StateDB) UpdateOneDayGesCost(address common.Address, price *hexutil.Big) bool {
	stateObject := self.GetOrNewStateObject(address)
	if stateObject != nil {
		stateObject.UpdateOneDayGesCost(price)
		return true
	}
	return false
}

func (self *StateDB) UpdateOneDaySyncLogGsaCost(address common.Address, price *hexutil.Big) bool {
	stateObject := self.GetOrNewStateObject(address)
	if stateObject != nil {
		stateObject.UpdateOneDaySyncLogGsaCost(price)
		return true
	}
	return false
}

func (self *StateDB) GetOneDayGesCost() *big.Int {
	stateObject := self.GetOrNewStateObject(common.GenaroPriceAddress)
	if stateObject != nil {
		return stateObject.GetOneDayGesCost()
	}
	return common.DefaultOneDayMortgageGes
}

func (self *StateDB) GetOneDaySyncLogGsaCost() *big.Int {
	stateObject := self.GetOrNewStateObject(common.GenaroPriceAddress)
	if stateObject != nil {
		return stateObject.GetOneDaySyncLogGsaCost()
	}
	return common.DefaultOneDaySyncLogGsaCost
}

func (self *StateDB) UnbindNode(address common.Address, nodeId string) error {
	stateObject := self.GetOrNewStateObject(address)
	var err error = nil
	if stateObject != nil {
		err = stateObject.UnbindNode(nodeId)
	}
	return err
}

func (self *StateDB) UbindNode2Address(address common.Address, nodeId string) error {
	stateObject := self.GetOrNewStateObject(address)
	var err error = nil
	if stateObject != nil {
		err = stateObject.UbindNode2Address(nodeId)
	}
	return err
}

func (self *StateDB) AddAccountInForbidBackStakeList(address common.Address) bool {
	stateObject := self.GetOrNewStateObject(common.ForbidBackStakeSaveAddress)
	if stateObject != nil {
		stateObject.AddAccountInForbidBackStakeList(address)
		return true
	}
	return false
}

func (self *StateDB) DelAccountInForbidBackStakeList(address common.Address) bool {
	stateObject := self.GetOrNewStateObject(common.ForbidBackStakeSaveAddress)
	if stateObject != nil {
		stateObject.DelAccountInForbidBackStakeList(address)
		return true
	}
	return false
}

func (self *StateDB) IsAccountExistInForbidBackStakeList(address common.Address) bool {
	stateObject := self.GetOrNewStateObject(common.ForbidBackStakeSaveAddress)
	if stateObject != nil {
		return stateObject.IsAccountExistInForbidBackStakeList(address)
	}
	return false
}

func (self *StateDB) GetForbidBackStakeList() types.ForbidBackStakeList {
	stateObject := self.GetOrNewStateObject(common.ForbidBackStakeSaveAddress)
	if stateObject != nil {
		return stateObject.GetForbidBackStakeList()
	}
	return nil
}

func (self *StateDB) GetRewardsValues() *types.RewardsValues {
	stateObject := self.GetOrNewStateObject(common.RewardsSaveAddress)
	if stateObject != nil {
		return stateObject.GetRewardsValues()
	}
	return nil
}

func (self *StateDB) SetRewardsValues(rewardsValues types.RewardsValues) bool {
	stateObject := self.GetOrNewStateObject(common.RewardsSaveAddress)
	if stateObject != nil {
		stateObject.SetRewardsValues(rewardsValues)
		return true
	}
	return false
}

func (self *StateDB) AddPromissoryNote(address common.Address, promissoryNote types.PromissoryNote) bool {
	stateObject := self.GetOrNewStateObject(address)
	if stateObject != nil {
		stateObject.AddPromissoryNote(promissoryNote)
		return true
	}
	return false
}

func (self *StateDB) DelPromissoryNote(address common.Address, promissoryNote types.PromissoryNote) bool {
	stateObject := self.GetOrNewStateObject(address)
	if stateObject != nil {
		return stateObject.DelPromissoryNote(promissoryNote)
	}
	return false
}

func (self *StateDB) GetPromissoryNotes(address common.Address) types.PromissoryNotes {
	stateObject := self.GetOrNewStateObject(address)
	if stateObject != nil {
		return stateObject.GetPromissoryNotes()
	}
	return nil
}

func (self *StateDB) GetOptionTxTable(hash common.Hash, optionTxMemorySize uint64) *types.OptionTxTable {

	optionSaveAddr := common.GetOptionSaveAddr(hash, optionTxMemorySize)

	stateObject := self.GetOrNewStateObject(optionSaveAddr)
	if stateObject != nil {
		return stateObject.GetOptionTxTable()
	}
	return nil
}

func (self *StateDB) GetOptionTxTableByAddress(address common.Address) *types.OptionTxTable {

	stateObject := self.GetOrNewStateObject(address)
	if stateObject != nil {
		return stateObject.GetOptionTxTable()
	}
	return nil
}

func (self *StateDB) DelTxInOptionTxTable(hash common.Hash, optionTxMemorySize uint64) bool {
	optionSaveAddr := common.GetOptionSaveAddr(hash, optionTxMemorySize)

	stateObject := self.GetOrNewStateObject(optionSaveAddr)
	if stateObject != nil {
		stateObject.DelTxInOptionTxTable(hash)
		return true
	}
	return false
}

func (self *StateDB) AddTxInOptionTxTable(hash common.Hash, promissoryNotesOptionTx types.PromissoryNotesOptionTx, optionTxMemorySize uint64) bool {

	optionSaveAddr := common.GetOptionSaveAddr(hash, optionTxMemorySize)
	stateObject := self.GetOrNewStateObject(optionSaveAddr)
	if stateObject != nil {
		stateObject.AddTxInOptionTxTable(hash, promissoryNotesOptionTx)
		return true
	}
	return false
}

func (self *StateDB) PromissoryNotesWithdrawCash(address common.Address, blockNumber uint64) uint64 {
	stateObject := self.GetOrNewStateObject(address)
	if stateObject != nil {
		return stateObject.PromissoryNotesWithdrawCash(blockNumber)
	}
	return uint64(0)
}

func (self *StateDB) GetAllPromissoryNotesNum(address common.Address) uint64 {
	stateObject := self.GetOrNewStateObject(address)
	if stateObject != nil {
		return stateObject.GetAllPromissoryNotesNum()
	}
	return uint64(0)
}

func (self *StateDB) GetBeforPromissoryNotesNum(address common.Address, blockNumber uint64) uint64 {
	stateObject := self.GetOrNewStateObject(address)
	if stateObject != nil {
		return stateObject.GetBeforPromissoryNotesNum(blockNumber)
	}
	return uint64(0)
}

func (self *StateDB) SetTxStatusInOptionTxTable(hash common.Hash, status bool, optionTxMemorySize uint64) bool {
	optionSaveAddr := common.GetOptionSaveAddr(hash, optionTxMemorySize)

	stateObject := self.GetOrNewStateObject(optionSaveAddr)
	if stateObject != nil {
		stateObject.SetTxStatusInOptionTxTable(hash, status)
		return true
	}
	return false
}

func (self *StateDB) GetAccountData(address common.Address) *Account {
	stateObject := self.GetOrNewStateObject(address)
	return &stateObject.data
}

func (self *StateDB) BuyPromissoryNotes(orderId common.Hash, address common.Address, optionTxMemorySize uint64) types.PromissoryNotesOptionTx {

	optionSaveAddr := common.GetOptionSaveAddr(orderId, optionTxMemorySize)

	stateObject := self.GetOrNewStateObject(optionSaveAddr)
	if stateObject != nil {
		return stateObject.BuyPromissoryNotes(orderId, address)
	}
	return types.PromissoryNotesOptionTx{}
}

func (self *StateDB) CarriedOutPromissoryNotes(orderId common.Hash, address common.Address, optionTxMemorySize uint64) types.PromissoryNotesOptionTx {
	optionSaveAddr := common.GetOptionSaveAddr(orderId, optionTxMemorySize)

	stateObject := self.GetOrNewStateObject(optionSaveAddr)
	stateObjectAddress := self.GetOrNewStateObject(address)
	if stateObject != nil && nil != stateObjectAddress {
		result := stateObject.DeletePromissoryNotes(orderId, address)
		if 0 < result.TxNum {
			promissoryNote := types.PromissoryNote{
				RestoreBlock: result.RestoreBlock,
				Num:          result.TxNum,
			}
			stateObjectAddress.AddPromissoryNote(promissoryNote)
			return result
		}
	}
	return types.PromissoryNotesOptionTx{}
}

func (self *StateDB) TurnBuyPromissoryNotes(orderId common.Hash, optionPrice *hexutil.Big, address common.Address, optionTxMemorySize uint64) bool {
	optionSaveAddr := common.GetOptionSaveAddr(orderId, optionTxMemorySize)

	stateObject := self.GetOrNewStateObject(optionSaveAddr)
	if stateObject != nil {
		return stateObject.TurnBuyPromissoryNotes(orderId, optionPrice, address)
	}
	return false
}

func (self *StateDB) GetProfitAccount(id common.Address) *common.Address {
	stateObject := self.getStateObject(id)
	if stateObject != nil {
		return stateObject.GetProfitAccount()
	}
	return nil
}

func (self *StateDB) SetProfitAccount(saddr,paddr common.Address) bool {
	stateObject := self.getStateObject(saddr)
	if stateObject != nil {
		return stateObject.UpdateProfitAccount(paddr)
	}
	return false
}

func (self *StateDB) GetShadowAccount(id common.Address) *common.Address {
	stateObject := self.getStateObject(id)
	if stateObject != nil {
		return stateObject.GetShadowAccount()
	}
	return nil
}

func (self *StateDB) SetShadowAccount(saddr,paddr common.Address) bool {
	stateObject := self.getStateObject(saddr)
	if stateObject != nil {
		return stateObject.UpdateShadowAccount(paddr)
	}
	return false
}
