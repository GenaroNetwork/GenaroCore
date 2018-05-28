package genaro

import (
	"encoding/json"

	"github.com/GenaroNetwork/Genaro-Core/common"
	"github.com/GenaroNetwork/Genaro-Core/ethdb"
	"github.com/GenaroNetwork/Genaro-Core/params"
	"encoding/binary"
)

// Each turn has a Snapshot. EpochNumber means the "electoral materials" period.
// Snapshot will not be stored immediately. It will be stored in EpochNumber + ElectionPeriod
// Snapshot will be valid in EpochNumber + ElectionPeriod + ValidPeriod
type CommitteeSnapshot struct {
	config           *params.GenaroConfig             //genaro config
	WriteBlockNumber uint64                           // Block number where the snapshot was created
	WriteBlockHash   common.Hash                      // BlockHash as the snapshot key
	EpochNumber      uint64                           // the turn of Committee
	CommitteeSize    uint64                           // the size of Committee
	CommitteeRank    []common.Address                 // the rank of committee
	Committee        map[common.Address]CommitteeInfo // committee members
}

type Stake struct {
	BlockNumber uint64
	Amount      uint64
	Staker      common.Address
}

type CommitteeInfo struct {
	Signer       common.Address // peer address
	SentinelHEFT uint64         // the sentinel of the peer
	Stake        uint64         // the stake of the peer
}

// newSnapshot creates a new snapshot with the specified startup parameters.
func newSnapshot(config *params.GenaroConfig, number uint64, hash common.Hash, epochNumber uint64, committeeRank []common.Address, committee map[common.Address]CommitteeInfo) *CommitteeSnapshot {
	snap := &CommitteeSnapshot{
		config:           config,
		WriteBlockNumber: number,
		WriteBlockHash:   hash,
		EpochNumber:      epochNumber,
		CommitteeRank:    make([]common.Address, len(committeeRank)),
		Committee:        make(map[common.Address]CommitteeInfo),
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
func loadSnapshot(config *params.GenaroConfig, db ethdb.Database, epollNumber uint64) (*CommitteeSnapshot, error) {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, epollNumber)
	blob, err := db.Get(append([]byte("genaro-"), b[:]...))
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

	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, s.EpochNumber)
	return db.Put(append([]byte("genaro-"), b[:]...), blob)
}

// copy creates a deep copy of the snapshot
func (s *CommitteeSnapshot) copy() *CommitteeSnapshot {
	cpy := &CommitteeSnapshot{
		config:           s.config,
		WriteBlockNumber: s.WriteBlockNumber,
		WriteBlockHash:   s.WriteBlockHash,
		EpochNumber:      s.EpochNumber,
		CommitteeSize:    s.CommitteeSize,
		CommitteeRank:    make([]common.Address, s.CommitteeSize),
		Committee:        make(map[common.Address]CommitteeInfo),
	}
	for i, rank := range s.CommitteeRank {
		cpy.CommitteeRank[i] = rank
	}

	for key, val := range s.Committee {
		cpy.Committee[key] = val
	}

	return cpy
}

//  retrieves the list of rank
func (s *CommitteeSnapshot) rank() []common.Address {
	return s.CommitteeRank
}

//  get the committee turn from block number
func GetTurnOfCommiteeByBlockNumber(config *params.GenaroConfig, number uint64) uint64 {
	return number / config.Epoch
}

//  get the depend turn from block number
func GetDependTurnByBlockNumber(config *params.GenaroConfig, number uint64) uint64 {
	return GetTurnOfCommiteeByBlockNumber(config, number) - config.ElectionPeriod - config.ValidPeriod
}


//  get the  written BlockNumber by the turn of committee
func GetCommiteeWrittenBlockNumberByTurn(config *params.GenaroConfig, turn uint64) uint64 {
	return (turn - config.ValidPeriod + 1)*config.Epoch - 1
}

func (s *CommitteeSnapshot) getCurrentRankIndex(addr common.Address) int {
	pos := -1
	for i, rank := range s.CommitteeRank {
		if addr == rank {
			pos = i
			break
		}
	}
	return pos
}

// inturn returns if a addr at a given block height is in-turn or not (in s)
func (s *CommitteeSnapshot) inturn(number uint64, addr common.Address) bool {
	pos := s.getCurrentRankIndex(addr)
	if pos == -1 {
		return false
	}
	startBlock := s.EpochNumber * s.config.Epoch
	if number < startBlock {
		return false
	}

	bias := (number - startBlock) % (s.CommitteeSize * s.config.BlockInterval)

	if bias >= uint64(pos)*s.config.BlockInterval && bias < (uint64(pos)+1)*s.config.BlockInterval {
		return true
	} else {
		return false
	}
}

func GetFirstBlockNumberOfEpoch(config *params.GenaroConfig, epochNumber uint64) uint64 {
	return config.Epoch*(epochNumber-1) + 1
}

func GetLastBlockNumberOfEpoch(config *params.GenaroConfig, epochNumber uint64) uint64 {
	return config.Epoch*(epochNumber+1) - 1
}
