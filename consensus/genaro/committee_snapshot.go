package genaro

import (
	"bytes"
	"encoding/json"

	"github.com/GenaroNetwork/Genaro-Core/common"
	"github.com/GenaroNetwork/Genaro-Core/core/types"
	"github.com/GenaroNetwork/Genaro-Core/ethdb"
	"github.com/GenaroNetwork/Genaro-Core/params"
)

type Genaro struct {
	config           *params.GenaroConfig //genaro config
	snapshotDb       ethdb.Database       // snapshot db
	sentinelDb       ethdb.Database       //sentinel db
	signer           common.Address       //peer address
	signFn           SignerFn             //sign function
	currentCommittee CommitteeSnapshot    //current snapshot
}

type CommitteeSnapshot struct {
	config        *params.GenaroConfig             //genaro config
	BlockNumber   uint64                           // Block number where the snapshot was created
	BlockHash     common.Hash                      // snapshot hash
	EpochNumber   uint64                           // the tern of Committee
	CommitteeSize uint64                           // the size of Committee
	CommitteeRank []common.Address                 // the rank of committee
	Committee     map[common.Address]CommitteeInfo // committee members
}

type CommitteeInfo struct {
	Signer       common.Address // peer address
	SentinelHEFT uint64         // the sentinel of the peer
	Stake        uint64         // the stake of the peer
}

// newSnapshot creates a new snapshot with the specified startup parameters.
func newSnapshot(config *params.GenaroConfig, number uint64, hash common.Hash, epochNumber uint64, committeeRank []common.Address, committee map[common.Address]CommitteeInfo) *CommitteeSnapshot {
	snap := &CommitteeSnapshot{
		config:        config,
		BlockNumber:   number,
		BlockHash:     hash,
		EpochNumber:   epochNumber,
		CommitteeRank: make([]common.Address, len(committeeRank)),
		Committee:     make(map[common.Address]CommitteeInfo),
	}
	snap.CommitteeSize = uint64(len(snap.CommitteeRank))

	for i, rank := range committeeRank {
		snap.CommitteeRank[i] = rank
	}

	for key, val := range committee {
		snap.Committee[key] = val
	}

	return snap
}

// loadSnapshot loads an existing snapshot from the database.
func loadSnapshot(config *params.GenaroConfig, db ethdb.Database, hash common.Hash) (*CommitteeSnapshot, error) {
	blob, err := db.Get(append([]byte("genaro-"), hash[:]...))
	if err != nil {
		return nil, err
	}
	snap := new(CommitteeSnapshot)
	if err := json.Unmarshal(blob, snap); err != nil {
		return nil, err
	}
	snap.config = config

	return snap, nil
}

// store inserts the snapshot into the database.
func (s *CommitteeSnapshot) store(db ethdb.Database) error {
	blob, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return db.Put(append([]byte("genaro-"), s.BlockHash[:]...), blob)
}

// copy creates a deep copy of the snapshot, though not the individual votes.
func (s *CommitteeSnapshot) copy() *CommitteeSnapshot {
	cpy := &CommitteeSnapshot{
		config:        s.config,
		BlockNumber:   s.BlockNumber,
		BlockHash:     s.BlockHash,
		EpochNumber:   s.EpochNumber,
		CommitteeSize: s.CommitteeSize,
		CommitteeRank: make([]common.Address, s.CommitteeSize),
		Committee:     make(map[common.Address]CommitteeInfo),
	}
	for i, rank := range s.CommitteeRank {
		cpy.CommitteeRank[i] = rank
	}

	for key, val := range s.Committee {
		cpy.Committee[key] = val
	}

	return cpy
}

// apply creates a new authorization snapshot by applying the given headers to
// the original one.
// TODO
func (s *CommitteeSnapshot) apply(headers []*types.Header) (*CommitteeSnapshot, error) {
	return nil, nil
}

//  retrieves the list of rank
func (s *CommitteeSnapshot) rank() []common.Address {
	return s.CommitteeRank
}

//  get the committee tern from block number
func GetTernOfCommiteeByBlockNumber(config *params.GenaroConfig, number uint64) uint64 {
	return number % config.Epoch
}

// inturn returns if a addr at a given block height is in-turn or not.
func (s *CommitteeSnapshot) inturn(number uint64, addr common.Address) bool {
	pos := -1
	for i, rank := range s.CommitteeRank {
		if addr == rank {
			pos = i
			break
		}
	}
	if pos == -1 {
		return false
	}
	startBlock := s.EpochNumber * s.config.Epoch
	if number < startBlock {
		return false
	}
	if number == (number-startBlock)%s.CommitteeSize {
		return true
	} else {
		return false
	}
}
