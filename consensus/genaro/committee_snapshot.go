package genaro

import (
	"encoding/json"

	"github.com/GenaroNetwork/Genaro-Core/common"
	"github.com/GenaroNetwork/Genaro-Core/ethdb"
	"github.com/GenaroNetwork/Genaro-Core/params"
	"encoding/binary"
	"github.com/GenaroNetwork/Genaro-Core/core/types"
)

// Each turn has a Snapshot. EpochNumber means the "electoral materials" period.
// Snapshot will not be stored immediately. It will be stored in EpochNumber + ElectionPeriod
// Snapshot will be valid in EpochNumber + ElectionPeriod + ValidPeriod
type CommitteeSnapshot struct {
	config					*params.GenaroConfig             //genaro config
	WriteBlockNumber		uint64                           // Block number where the snapshot was created
	WriteBlockHash			common.Hash                      // BlockHash as the snapshot key
	EpochNumber				uint64                           // the turn of Committee
	CommitteeSize			uint64                           // the size of Committee
	CommitteeRank			[]common.Address                 // the rank of committee
	Committee				map[common.Address]uint64		 // committee members
	CommitteeAccountBinding 	map[common.Address][]common.Address	// account binding map
}

type Stake struct {
	BlockNumber uint64
	Amount      uint64
	Staker      common.Address
}

//type CommitteeInfo struct {
//	Signer       common.Address // peer address
//	SentinelHEFT uint64         // the sentinel of the peer
//	Stake        uint64         // the stake of the peer
//}

// newSnapshot creates a new snapshot with the specified startup parameters.
func newSnapshot(config *params.GenaroConfig, number uint64, hash common.Hash, epochNumber uint64,
	committeeRank []common.Address, proportion []uint64, CommitteeAccountBinding 	map[common.Address][]common.Address) *CommitteeSnapshot {
	if config.Epoch == 0 {
		config.Epoch = epochLength
	}
	committeeLenth := len(committeeRank)
	snap := &CommitteeSnapshot{
		config:				config,
		WriteBlockNumber:	number,
		WriteBlockHash:		hash,
		EpochNumber:		epochNumber,
		CommitteeRank:		make([]common.Address, committeeLenth),
		Committee:			make(map[common.Address]uint64, committeeLenth),
		CommitteeAccountBinding:	make(map[common.Address][]common.Address),
	}

	total := uint64(0)
	for i := 0; i <len(proportion); i++ {
		total += proportion[i]
	}

	for i, rank := range committeeRank {
		if i < int(config.CommitteeMaxSize) {
			snap.CommitteeRank[i] = rank
			snap.Committee[rank] = proportion[i]*uint64(common.Base)/total
		}
	}
	if config.CommitteeMaxSize < uint64(len(snap.CommitteeRank)) {
		snap.CommitteeRank = snap.CommitteeRank[0:config.CommitteeMaxSize]
	}
	snap.CommitteeSize = uint64(len(snap.CommitteeRank))

	for accountBinding := range CommitteeAccountBinding {
		snap.CommitteeAccountBinding[accountBinding] = CommitteeAccountBinding[accountBinding]
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
		Committee:        make(map[common.Address]uint64),
		CommitteeAccountBinding:	make(map[common.Address][]common.Address),
	}
	for i, rank := range s.CommitteeRank {
		cpy.CommitteeRank[i] = rank
	}

	for key, val := range s.Committee {
		cpy.Committee[key] = val
	}

	for key, val := range s.CommitteeAccountBinding {
		cpy.CommitteeAccountBinding[key] = val
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
	return GetTurnOfCommiteeByBlockNumber(config, number)
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
	//TODO maybe have one bug, the startBlock maybe equals (s.EpochNumber + g.config.ValidPeriod + g.config.ElectionPeriod) * s.config.Epoch
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

func (s *CommitteeSnapshot) getInturnRank(number uint64) int {
	var bias int
	startBlock := s.EpochNumber * s.config.Epoch
	bias = int((number - startBlock) / s.config.BlockInterval % s.CommitteeSize)
	return bias
}

func GetFirstBlockNumberOfEpoch(config *params.GenaroConfig, epochNumber uint64) uint64 {
	return config.Epoch*epochNumber
}

func GetLastBlockNumberOfEpoch(config *params.GenaroConfig, epochNumber uint64) uint64 {
	return config.Epoch*(epochNumber+1) - 1
}

func IsBackStakeBlockNumber(config *params.GenaroConfig, applyBlockNumber, nowBlockNumber uint64) bool {
	if nowBlockNumber - applyBlockNumber > (config.ElectionPeriod + config.ValidPeriod + common.BackStakePeriod) * config.Epoch {
		return true
	}
	return false
}

// cal block delay time
func (s *CommitteeSnapshot) getDelayTime(header *types.Header) uint64 {
	return s.getDistance(header.Coinbase,header.Number.Uint64())
}

func (s *CommitteeSnapshot) getDistance(addr common.Address, blockNumber uint64) uint64 {
	bias := s.getInturnRank(blockNumber)
	index := s.getCurrentRankIndex(addr)
	if index < 0 {
		return minDistance
	}
	distance := bias - index
	if distance < 0 {
		distance = int(s.CommitteeSize)+distance
	}
	return uint64(distance)
}

func calEpochPerYear(config *params.GenaroConfig)uint64{
	return (365*3600*24)/(config.Epoch*config.Period)
}
