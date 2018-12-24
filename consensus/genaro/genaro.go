package genaro

import (
	"errors"
	"math/big"
	"sync"
	"time"

	"encoding/json"
	"github.com/GenaroNetwork/GenaroCore/accounts"
	"github.com/GenaroNetwork/GenaroCore/common"
	"github.com/GenaroNetwork/GenaroCore/consensus"
	"github.com/GenaroNetwork/GenaroCore/core/state"
	"github.com/GenaroNetwork/GenaroCore/core/types"
	"github.com/GenaroNetwork/GenaroCore/crypto"
	"github.com/GenaroNetwork/GenaroCore/crypto/sha3"
	"github.com/GenaroNetwork/GenaroCore/ethdb"
	"github.com/GenaroNetwork/GenaroCore/log"
	"github.com/GenaroNetwork/GenaroCore/params"
	"github.com/GenaroNetwork/GenaroCore/rlp"
	"github.com/GenaroNetwork/GenaroCore/rpc"
	"github.com/hashicorp/golang-lru"
)

const (
	inmemorySnapshots = 128          // Number of recent snapshots to keep in memory
	epochLength       = uint64(5000) // Default number of blocks a turn
	minDistance       = uint64(500)
)

var (
	minRatio = common.Base * 98 / 100
	maxRatio = common.Base * 102 / 100
)

var (
	// extra data is empty
	errEmptyExtra = errors.New("extra data is empty")
	// errUnauthorized is returned if a header is signed by a non-authorized entity.
	errUnauthorized = errors.New("unauthorized")
	// errUnauthorized is returned if epoch block has no committee list
	errInvalidEpochBlock = errors.New("epoch block has no committee list")
	errInvalidDifficulty = errors.New("invalid difficulty")
	errInvalidBlockTime  = errors.New("invalid block time")
)

// Various error messages to mark blocks invalid.
var (
	// errUnknownBlock is returned when the list of signers is requested for a block
	// that is not part of the local blockchain.
	errUnknownBlock = errors.New("unknown block")
)

// SignerFn is a signer callback function to request a hash to be signed by a
// backing account.
type SignerFn func(accounts.Account, []byte) ([]byte, error)

// sigHash returns the hash which is used as input for the proof-of-authority
// signing. It is the hash of the entire header apart from the 65 byte signature
// contained at the end of the extra data.
//
// Note, the method requires the extra data to be at least 65 bytes, otherwise it
// panics. This is done to avoid accidentally using both forms (signature present
// or not), which could be abused to produce different hashes for the same header.
func sigHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewKeccak256()

	rlp.Encode(hasher, []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.Extra, // just hash extra
		header.MixDigest,
		header.Nonce,
	})
	hasher.Sum(hash[:0])
	return hash
}

// ecrecover extracts the Ethereum account address from a signed header.
func ecrecover(header *types.Header) (common.Address, error) {
	// If the signature's already cached, return that
	// Retrieve the signature from the header extra-data
	extraData := UnmarshalToExtra(header)
	if extraData == nil {
		return common.Address{}, errEmptyExtra
	}
	signature := extraData.Signature

	ResetHeaderSignature(header)
	// Recover the public key and the Ethereum address
	pubkey, err := crypto.Ecrecover(sigHash(header).Bytes(), signature)
	SetHeaderSignature(header, signature)
	if err != nil {
		return common.Address{}, err
	}
	var signer common.Address
	copy(signer[:], crypto.Keccak256(pubkey[1:])[12:])
	return signer, nil
}

func Ecrecover(header *types.Header) (common.Address, error) {
	return ecrecover(header)
}

type Genaro struct {
	config  *params.GenaroConfig // genaro config
	db      ethdb.Database       // Database to store and retrieve snapshot checkpoints
	recents *lru.ARCCache        // snapshot cache
	signer  common.Address       // Ethereum address of the signing key
	lock    sync.RWMutex         // Protects the signer fields
	signFn  SignerFn             // sign function
}

// New creates a Genaro consensus engine
func New(config *params.GenaroConfig, snapshotDb ethdb.Database) *Genaro {
	// Set any missing consensus parameters to their defaults
	conf := *config
	if conf.Epoch == 0 {
		conf.Epoch = epochLength
	}
	// Allocate the snapshot caches and create the engine
	recents, _ := lru.NewARC(inmemorySnapshots)

	return &Genaro{
		config:  &conf,
		db:      snapshotDb,
		recents: recents,
	}
}

// Author implements consensus.Engine, returning the Ethereum address recovered
// from the signature in the header's extra-data section.
func (g *Genaro) Author(header *types.Header) (common.Address, error) {
	log.Info("Author:" + header.Number.String())
	return ecrecover(header)
}

// Prepare implements consensus.Engine, preparing all the consensus fields of the
// header for running the transactions on top.
func (g *Genaro) Prepare(chain consensus.ChainReader, header *types.Header) error {
	log.Info("Prepare:" + header.Number.String())
	// set block author in Coinbase
	// TODO It may be modified later
	header.Coinbase = g.signer
	header.Nonce = types.BlockNonce{}
	number := header.Number.Uint64()

	snap, err := g.snapshot(chain, GetTurnOfCommiteeByBlockNumber(g.config, number), nil)
	if err != nil {
		return err
	}
	// Set the correct difficulty
	header.Difficulty = CalcDifficulty(snap, g.signer, number)
	header.MixDigest = common.Hash{}
	// Ensure the timestamp has the correct delay
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	delayTime := snap.getDelayTime(header)
	header.Time = new(big.Int).Add(parent.Time, new(big.Int).SetUint64(g.config.Period+delayTime))
	if header.Time.Int64() < time.Now().Unix() {
		header.Time = big.NewInt(time.Now().Unix())
	}
	return nil
}

// Seal implements consensus.Engine, attempting to create a sealed block using
// the local signing credentials.
func (g *Genaro) Seal(chain consensus.ChainReader, block *types.Block, stop <-chan struct{}) (*types.Block, error) {
	log.Info("Seal:" + block.Number().String())
	header := block.Header()
	// Sealing the genesis block is not supported
	number := header.Number.Uint64()
	if number == 0 {
		return nil, errUnknownBlock
	}
	// Don't hold the signer fields for the entire sealing procedure
	g.lock.RLock()
	signer, signFn := g.signer, g.signFn
	g.lock.RUnlock()

	delay := time.Unix(header.Time.Int64(), 0).Sub(time.Now())
	log.Info("delay:" + delay.String())
	select {
	case <-stop:
		return nil, nil
	case <-time.After(delay):
	}
	ResetHeaderSignature(header)
	// Sign all the things!
	sighash, err := signFn(accounts.Account{Address: signer}, sigHash(header).Bytes())
	if err != nil {
		return nil, err
	}
	SetHeaderSignature(header, sighash)
	return block.WithSeal(header), nil
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
// that a new block should have based on the previous blocks in the chain and the
// current signer.
func (g *Genaro) CalcDifficulty(chain consensus.ChainReader, time uint64, parent *types.Header) *big.Int {
	log.Info("CalcDifficulty:" + parent.Number.String())
	blockNumber := parent.Number.Uint64() + 1
	dependEpoch := GetTurnOfCommiteeByBlockNumber(g.config, blockNumber)

	snap, err := g.snapshot(chain, dependEpoch, nil)
	if err != nil {
		return nil
	}
	return CalcDifficulty(snap, g.signer, blockNumber)
}

func max(x uint64, y uint64) uint64 {
	if x > y {
		return x
	} else {
		return y
	}
}

// CalcDifficulty return the distance between my index and intern-index
// depend on snap
func CalcDifficulty(snap *CommitteeSnapshot, addr common.Address, blockNumber uint64) *big.Int {
	index := snap.getCurrentRankIndex(addr)
	if index < 0 {
		return new(big.Int).SetUint64(0)
	}
	distance := snap.getDistance(addr, blockNumber)

	difficult := snap.CommitteeSize - uint64(distance)
	return new(big.Int).SetUint64(uint64(difficult))
}

// Authorize injects a private key into the consensus engine to mint new blocks
// with.
func (g *Genaro) Authorize(signer common.Address, signFn SignerFn) {
	log.Info("Authorize")
	g.lock.Lock()
	defer g.lock.Unlock()

	g.signer = signer
	g.signFn = signFn
}

// Snapshot retrieves the snapshot at "electoral materials" period.
// Snapshot func retrieves ths snapshot in order of memory, local DB, block header.
// If committeeSnapshot is empty and it is time to write, we will create a new one, otherwise return nil
func (g *Genaro) snapshot(chain consensus.ChainReader, epollNumber uint64, parents []*types.Header) (*CommitteeSnapshot, error) {
	// Search for a snapshot in memory or on disk for checkpoints
	var (
		snap *CommitteeSnapshot
	)
	isCreateNew := false
	// If an in-memory snapshot was found, use that
	if s, ok := g.recents.Get(epollNumber); ok {
		snap = s.(*CommitteeSnapshot)
	} else if epollNumber < g.config.ValidPeriod+g.config.ElectionPeriod {

		h := chain.GetHeaderByNumber(0)
		committeeRank, proportion := GetHeaderCommitteeRankList(h)
		committeeAccountBinding := GetCommitteeAccountBinding(h)
		snap = newSnapshot(chain.Config().Genaro, 0, h.Hash(), 0, committeeRank, proportion, committeeAccountBinding)
		isCreateNew = true
	} else {
		// visit the blocks in epollNumber - ValidPeriod - ElectionPeriod tern
		startBlock := GetFirstBlockNumberOfEpoch(g.config, epollNumber-g.config.ValidPeriod-g.config.ElectionPeriod)
		endBlock := GetLastBlockNumberOfEpoch(g.config, epollNumber-g.config.ValidPeriod-g.config.ElectionPeriod)
		var h *types.Header
		if parents != nil && len(parents) > 0 && parents[0].Number.Uint64() < endBlock+1 {
			num := endBlock + 1 - parents[0].Number.Uint64()
			if parents[num].Number.Uint64() == endBlock+1 {
				h = parents[num]
			}
		}
		if h == nil {
			h = chain.GetHeaderByNumber(endBlock + 1)
		}
		committeeRank, proportion := GetHeaderCommitteeRankList(h)
		committeeAccountBinding := GetCommitteeAccountBinding(h)
		snap = newSnapshot(chain.Config().Genaro, h.Number.Uint64(), h.Hash(), epollNumber-
			g.config.ValidPeriod-g.config.ElectionPeriod, committeeRank, proportion, committeeAccountBinding)

		log.Trace("computing rank from", startBlock, "to", endBlock)
		isCreateNew = true
	}
	g.recents.Add(epollNumber, snap)
	// If we've generated a new checkpoint snapshot, save to disk
	if isCreateNew {
		if err := snap.store(g.db); err != nil {
			return nil, err
		}
		log.Trace("Stored snapshot to disk", "epollNumber", epollNumber)
	}
	return snap, nil
}

// VerifySeal implements consensus.Engine, checking whether the signature contained
// in the header satisfies the consensus protocol requirements.
func (g *Genaro) VerifySeal(chain consensus.ChainReader, header *types.Header) error {
	log.Info("VerifySeal:" + header.Number.String())
	return g.verifySeal(chain, header, nil)
}

func (g *Genaro) verifySeal(chain consensus.ChainReader, header *types.Header, parents []*types.Header) error {
	blockNumber := header.Number.Uint64()
	if blockNumber == 0 {
		return errUnknownBlock
	}
	// check syn state
	extraData := UnmarshalToExtra(header)
	if blockNumber-extraData.LastSynBlockNum > common.SynBlockLen+1 {
		return errors.New("need SynState")
	}

	// Don't waste time checking blocks from the future
	if header.Time.Cmp(big.NewInt(time.Now().Unix())) > 0 {
		return consensus.ErrFutureBlock
	}
	// check epoch point
	epochPoint := (blockNumber % g.config.Epoch) == 0
	if epochPoint {
		extraData := UnmarshalToExtra(header)
		committeeSize := uint64(len(extraData.CommitteeRank))
		if committeeSize == 0 || committeeSize > g.config.CommitteeMaxSize {
			return errInvalidEpochBlock
		}
	}
	// get current committee snapshot
	snap, err := g.snapshot(chain, GetTurnOfCommiteeByBlockNumber(g.config, blockNumber), parents)
	if err != nil {
		return err
	}
	// get signer from header
	signer, err := ecrecover(header)
	if err != nil {
		return err
	}
	// check signer
	if _, ok := snap.Committee[signer]; !ok {
		return errUnauthorized
	}

	var parent *types.Header
	if len(parents) > 0 {
		parent = parents[len(parents)-1]
	} else {
		parent = chain.GetHeader(header.ParentHash, blockNumber-1)
	}
	if parent == nil || parent.Number.Uint64() != blockNumber-1 || parent.Hash() != header.ParentHash {
		return consensus.ErrUnknownAncestor
	}

	if header.Time.Uint64() < parent.Time.Uint64() {
		return errUnknownBlock
	}
	// Ensure that difficulty corresponds to the turn of the signer
	diffcult := CalcDifficulty(snap, signer, blockNumber)
	if header.Difficulty.Cmp(diffcult) != 0 {
		return errInvalidDifficulty
	}
	// Ensure that block time corresponds to the turn of the signer
	inturn := snap.inturn(blockNumber, signer)
	if !inturn {
		//bias := header.Difficulty.Uint64()
		bias := snap.getDelayTime(header)
		delay := uint64(time.Duration(bias * uint64(time.Second)))
		if parent.Time.Uint64()+delay/uint64(time.Second) > header.Time.Uint64() {
			return errInvalidBlockTime
		}
	}
	return nil
}

// VerifyUncles implements consensus.Engine, always returning an error for any
// uncles as this consensus mechanism doesn't permit uncles.
func (g *Genaro) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	log.Info("VerifyUncles:" + block.Number().String())
	if len(block.Uncles()) > 0 {
		return errors.New("uncles not allowed")
	}
	return nil
}

func GetCoinActualRewards(state *state.StateDB) *big.Int {
	return state.GetRewardsValues().CoinActualRewards
}

func AddCoinActualRewards(state *state.StateDB, coinrewards *big.Int) {
	rewardsValues := state.GetRewardsValues()
	rewardsValues.CoinActualRewards.Add(rewardsValues.CoinActualRewards, coinrewards)
	state.SetRewardsValues(*rewardsValues)
}

func SetCoinActualRewards(state *state.StateDB, coinrewards *big.Int) {
	rewardsValues := state.GetRewardsValues()
	rewardsValues.CoinActualRewards.Set(coinrewards)
	state.SetRewardsValues(*rewardsValues)
}

func SetPreCoinActualRewards(state *state.StateDB, coinrewards *big.Int) {
	rewardsValues := state.GetRewardsValues()
	rewardsValues.PreCoinActualRewards.Set(coinrewards)
	state.SetRewardsValues(*rewardsValues)
}

func GetPreCoinActualRewards(state *state.StateDB) *big.Int {
	rewardsValues := state.GetRewardsValues()
	return rewardsValues.PreCoinActualRewards
}

func GetStorageActualRewards(state *state.StateDB) *big.Int {
	rewardsValues := state.GetRewardsValues()
	return rewardsValues.StorageActualRewards
}

func SetStorageActualRewards(state *state.StateDB, storagerewards *big.Int) {
	rewardsValues := state.GetRewardsValues()
	rewardsValues.StorageActualRewards.Set(storagerewards)
	state.SetRewardsValues(*rewardsValues)
}

func AddStorageActualRewards(state *state.StateDB, storagerewards *big.Int) {
	rewardsValues := state.GetRewardsValues()
	rewardsValues.StorageActualRewards.Add(rewardsValues.StorageActualRewards, storagerewards)
	state.SetRewardsValues(*rewardsValues)
}

func SetPreStorageActualRewards(state *state.StateDB, storagerewards *big.Int) {
	rewardsValues := state.GetRewardsValues()
	rewardsValues.PreStorageActualRewards.Set(storagerewards)
	state.SetRewardsValues(*rewardsValues)
}

func GetPreStorageActualRewards(state *state.StateDB) *big.Int {
	rewardsValues := state.GetRewardsValues()
	return rewardsValues.PreStorageActualRewards
}

func AddTotalActualRewards(state *state.StateDB, rewards *big.Int) {
	rewardsValues := state.GetRewardsValues()
	rewardsValues.TotalActualRewards.Add(rewardsValues.TotalActualRewards, rewards)
	state.SetRewardsValues(*rewardsValues)
}

func GetTotalActualRewards(state *state.StateDB) *big.Int {
	rewardsValues := state.GetRewardsValues()
	return rewardsValues.TotalActualRewards
}

func SetTotalActualRewards(state *state.StateDB, rewards *big.Int) {
	rewardsValues := state.GetRewardsValues()
	rewardsValues.TotalActualRewards.Set(rewards)
	state.SetRewardsValues(*rewardsValues)
}

func GetSurplusCoin(state *state.StateDB) *big.Int {
	rewardsValues := state.GetRewardsValues()
	return rewardsValues.SurplusCoin
}

func SubSurplusCoin(state *state.StateDB, rewards *big.Int) {
	rewardsValues := state.GetRewardsValues()
	rewardsValues.SurplusCoin.Sub(rewardsValues.SurplusCoin, rewards)
	state.SetRewardsValues(*rewardsValues)
}

func SetSurplusCoin(state *state.StateDB, surplusrewards *big.Int) {
	rewardsValues := state.GetRewardsValues()
	rewardsValues.SurplusCoin.Set(surplusrewards)
	state.SetRewardsValues(*rewardsValues)
}

func SetPreSurplusCoin(state *state.StateDB, surplusrewards *big.Int) {
	rewardsValues := state.GetRewardsValues()
	rewardsValues.PreStorageActualRewards.Set(surplusrewards)
	state.SetRewardsValues(*rewardsValues)
}

func GetPreSurplusCoin(state *state.StateDB) *big.Int {
	rewardsValues := state.GetRewardsValues()
	return rewardsValues.PreStorageActualRewards
}

func updateEpochRewards(state *state.StateDB) {
	//reset CoinActualRewards and StorageActualRewards, add TotalActualRewards
	coinrewards := GetCoinActualRewards(state)
	storagerewards := GetStorageActualRewards(state)
	SetPreCoinActualRewards(state, coinrewards)
	SetPreStorageActualRewards(state, storagerewards)
	SetCoinActualRewards(state, big.NewInt(0))
	SetStorageActualRewards(state, big.NewInt(0))
	AddTotalActualRewards(state, coinrewards)
	log.Info("this epoch coin rewards:\t" + coinrewards.String())
	AddTotalActualRewards(state, storagerewards)
	log.Info("this epoch storage rewards:\t" + storagerewards.String())
	log.Info("Total Actual Rewards:\t" + GetTotalActualRewards(state).String())
}

func updateEpochYearRewards(state *state.StateDB) {
	surplusrewards := GetSurplusCoin(state)
	SetPreSurplusCoin(state, surplusrewards)
	totalRewards := GetTotalActualRewards(state)
	SubSurplusCoin(state, totalRewards)
	SetTotalActualRewards(state, big.NewInt(0))
}

func genCommitteeAccountBinding(thisstate *state.StateDB, commitee []common.Address) (committeeAccountBinding map[common.Address][]common.Address) {
	committeeAccountBinding = make(map[common.Address][]common.Address)
	mainAccounts := thisstate.GetMainAccounts()
	for _, account := range commitee {
		subAccounts, ok := mainAccounts[account]
		if ok {
			committeeAccountBinding[account] = subAccounts
		}
	}
	return
}

func updateCoinRewardsRatio(thisstate *state.StateDB, commiteeRank []common.Address) {
	allStake := uint64(0)
	for _, addr := range commiteeRank {
		candidateInfo := thisstate.GetCandidateInfoWithAllSubAccounts(addr)
		allStake += candidateInfo.Stake
	}
	surplusCoin := GetSurplusCoin(thisstate)
	surplusCoin.Div(surplusCoin, common.BaseCompany)
	coinRewardsRatio := 10000 * allStake * 3 / 50 / surplusCoin.Uint64() / 2
	if coinRewardsRatio < 2 {
		coinRewardsRatio = 2
	} else if coinRewardsRatio > 125 {
		coinRewardsRatio = 125
	}
	log.Info("coinRewardsRatio update:", "ratio", coinRewardsRatio)
	price := thisstate.GetGenaroPrice()
	price.CoinRewardsRatio = coinRewardsRatio
	thisstate.SetGenaroPrice(*price)

}

func updateSpecialBlock(config *params.GenaroConfig, header *types.Header, thisstate *state.StateDB) {
	blockNumber := header.Number.Uint64()
	if blockNumber%config.Epoch == 0 {
		candidateInfos := thisstate.GetCandidatesInfoWithAllSubAccounts()
		genaroPrice := thisstate.GetGenaroPrice()
		commiteeRank, proportion := state.RankWithLenth(candidateInfos, int(config.CommitteeMaxSize), genaroPrice.CommitteeMinStake)

		var committeeAccountBinding map[common.Address][]common.Address
		if uint64(len(candidateInfos)) <= config.CommitteeMaxSize {
			SetHeaderCommitteeRankList(header, commiteeRank, proportion)
			committeeAccountBinding = genCommitteeAccountBinding(thisstate, commiteeRank)
			updateCoinRewardsRatio(thisstate, commiteeRank)
		} else {
			SetHeaderCommitteeRankList(header, commiteeRank[:config.CommitteeMaxSize], proportion[:config.CommitteeMaxSize])
			committeeAccountBinding = genCommitteeAccountBinding(thisstate, commiteeRank[:config.CommitteeMaxSize])
			updateCoinRewardsRatio(thisstate, commiteeRank[:config.CommitteeMaxSize])
		}
		SetCommitteeAccountBinding(header, committeeAccountBinding)
		//CoinActualRewards and StorageActualRewards should update per epoch
		updateEpochRewards(thisstate)
	}

	if blockNumber%(calEpochPerYear(config)*config.Epoch) == 0 {
		//CoinActualRewards and StorageActualRewards should update per epoch, surplusCoin should update per year
		updateEpochYearRewards(thisstate)
	}
}

func handleAlreadyBackStakeList(config *params.GenaroConfig, header *types.Header, thisstate *state.StateDB) {
	blockNumber := header.Number.Uint64()
	_, backlist := thisstate.GetAlreadyBackStakeList()
	for i := 0; i < len(backlist); i++ {
		back := backlist[i]
		if IsBackStakeBlockNumber(config, back.BackBlockNumber, blockNumber) {
			thisstate.BackStake(back.Addr, blockNumber)
			backlist = append(backlist[:i], backlist[i+1:]...)
			i--
		}
	}
	thisstate.SetAlreadyBackStakeList(backlist)
}

// Finalize implements consensus.Engine, ensuring no uncles are set, nor block
// rewards given, and returns the final block.
func (g *Genaro) Finalize(chain consensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
	log.Info("Finalize:" + header.Number.String())
	//commit rank
	blockNumber := header.Number.Uint64()
	updateSpecialBlock(g.config, header, state)

	// update LastSynBlockNum
	extraData := UnmarshalToExtra(header)
	lastSynState := state.GetLastSynState()
	if lastSynState != nil {
		extraData.LastSynBlockNum = lastSynState.LastSynBlockNum
		extraData.LastSynBlockHash = lastSynState.LastSynBlockHash
		header.Extra, _ = json.Marshal(extraData)
	}
	if header.Number.Uint64() > common.SynBlockLen {
		state.AddLastRootState(header.ParentHash, header.Number.Uint64()-1)
	}

	snap, err := g.snapshot(chain, GetTurnOfCommiteeByBlockNumber(g.config, header.Number.Uint64()), nil)
	if err != nil {
		return nil, err
	}

	proportion := snap.Committee[snap.CommitteeRank[blockNumber%snap.CommitteeSize]]

	//  coin interest reward
	accumulateInterestRewards(g.config, state, header, proportion, blockNumber, snap.CommitteeSize, snap.CommitteeAccountBinding)
	// storage reward
	accumulateStorageRewards(g.config, state, blockNumber, snap.CommitteeSize)

	//handle already back stake list
	handleAlreadyBackStakeList(g.config, header, state)

	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
	header.UncleHash = types.CalcUncleHash(nil)

	// Assemble and return the final block for sealing
	return types.NewBlock(header, txs, nil, receipts), nil
}

func getCoinCofficient(config *params.GenaroConfig, coinrewards, surplusRewards *big.Int, coinRewardsRatio uint64, ratioPerYear uint64) uint64 {
	if coinrewards.Cmp(big.NewInt(0)) == 0 {
		return uint64(common.Base)
	}
	planrewards := big.NewInt(0)
	//get total coinReward
	planrewards.Mul(surplusRewards, big.NewInt(int64(coinRewardsRatio)))
	planrewards.Div(planrewards, big.NewInt(int64(common.Base)))
	//get coinReward perYear
	planrewards.Mul(planrewards, big.NewInt(int64(ratioPerYear)))
	planrewards.Div(planrewards, big.NewInt(int64(common.Base)))
	//get coinReward perEpoch
	planrewards.Div(planrewards, big.NewInt(int64(calEpochPerYear(config))))
	//get coefficient
	planrewards.Mul(planrewards, big.NewInt(int64(common.Base)))
	coinRatio := planrewards.Div(planrewards, coinrewards).Uint64()
	if coinRatio < minRatio {
		coinRatio = minRatio
	} else if coinRatio > maxRatio {
		coinRatio = maxRatio
	}
	return coinRatio
}

func getStorageCoefficient(config *params.GenaroConfig, storagerewards, surplusRewards *big.Int, storageRewardsRatio uint64, ratioPerYear uint64) uint64 {
	if storagerewards.Cmp(big.NewInt(0)) == 0 {
		return uint64(common.Base)
	}
	planrewards := big.NewInt(0)
	//get total storageReward
	planrewards.Mul(surplusRewards, big.NewInt(int64(storageRewardsRatio)))
	planrewards.Div(planrewards, big.NewInt(int64(common.Base)))
	//get storageReward perYear
	planrewards.Mul(planrewards, big.NewInt(int64(ratioPerYear)))
	planrewards.Div(planrewards, big.NewInt(int64(common.Base)))
	//get storageReward perEpoch
	planrewards.Div(planrewards, big.NewInt(int64(calEpochPerYear(config))))
	//get coefficient
	planrewards.Mul(planrewards, big.NewInt(int64(common.Base)))
	storageRatio := planrewards.Div(planrewards, storagerewards).Uint64()
	if storageRatio < minRatio {
		storageRatio = minRatio
	} else if storageRatio > maxRatio {
		storageRatio = maxRatio
	}
	return storageRatio
}

func settleInterestRewards(state *state.StateDB, coinbase common.Address, reward *big.Int, subAccounts []common.Address) {
	coinbaseStake, _ := state.GetStake(coinbase)
	totleStake := uint64(0)
	totleStake += coinbaseStake
	for _, subAccount := range subAccounts {
		stake, _ := state.GetStake(subAccount)
		totleStake += stake
	}

	surplusReward := big.NewInt(0)
	surplusReward.Set(reward)

	for _, subAccount := range subAccounts {
		stake, _ := state.GetStake(subAccount)
		accountReward := big.NewInt(0)
		accountReward.Set(reward)
		accountReward.Mul(accountReward, big.NewInt(int64(stake)))
		accountReward.Div(accountReward, big.NewInt(int64(totleStake)))
		state.AddBalance(subAccount, accountReward)
		surplusReward.Sub(surplusReward, accountReward)
	}

	state.AddBalance(coinbase, surplusReward)
}

// AccumulateInterestRewards credits the reward to the block author by coin  interest
func accumulateInterestRewards(config *params.GenaroConfig, state *state.StateDB, header *types.Header, proportion uint64,
	blockNumber uint64, committeeSize uint64, committeeAccountBinding map[common.Address][]common.Address) error {
	preCoinRewards := GetPreCoinActualRewards(state)
	preSurplusRewards := big.NewInt(0)
	//when now is the start of year, preSurplusRewards should get "Pre + SurplusCoinAddress"
	if blockNumber%(config.Epoch*calEpochPerYear(config)) == 0 {
		preSurplusRewards = GetPreSurplusCoin(state)
	} else {
		preSurplusRewards = GetSurplusCoin(state)
	}

	genaroPrice := state.GetGenaroPrice()
	coinRewardsRatio := common.Base * genaroPrice.CoinRewardsRatio / 100
	ratioPerYear := common.Base * genaroPrice.RatioPerYear / 100
	coefficient := getCoinCofficient(config, preCoinRewards, preSurplusRewards, coinRewardsRatio, ratioPerYear)
	surplusRewards := GetSurplusCoin(state)
	//plan rewards per year
	planRewards := big.NewInt(0)
	planRewards.Mul(surplusRewards, big.NewInt(int64(coinRewardsRatio)))
	planRewards.Div(planRewards, big.NewInt(int64(common.Base)))
	//get Reward perYear
	planRewards.Mul(planRewards, big.NewInt(int64(ratioPerYear)))
	planRewards.Div(planRewards, big.NewInt(int64(common.Base)))
	//plan rewards per epoch
	planRewards.Div(planRewards, big.NewInt(int64(calEpochPerYear(config))))
	//Coefficient adjustment
	planRewards.Mul(planRewards, big.NewInt(int64(coefficient)))
	planRewards.Div(planRewards, big.NewInt(int64(common.Base)))
	//fmt.Printf("Plan rewards this epoch %v(after adjustment), coefficient %v\n", planRewards.String(), coefficient)
	//this addr should get
	planRewards.Mul(planRewards, big.NewInt(int64(proportion)))
	planRewards.Div(planRewards, big.NewInt(int64(common.Base)))

	blockReward := big.NewInt(0)
	blockReward = planRewards.Div(planRewards, big.NewInt(int64(config.Epoch/committeeSize)))

	reward := blockReward
	log.Info("accumulateInterestRewards", "reward", reward.String())

	subAccounts, ok := committeeAccountBinding[header.Coinbase]
	if ok {
		settleInterestRewards(state, header.Coinbase, reward, subAccounts)
	} else {
		state.AddBalance(header.Coinbase, reward)
	}
	AddCoinActualRewards(state, reward)
	return nil
}

// AccumulateStorageRewards credits the reward to the sentinel owner
func accumulateStorageRewards(config *params.GenaroConfig, state *state.StateDB, blockNumber uint64, committeeSize uint64) error {
	if blockNumber%config.Epoch != 0 {
		return nil
	}
	preStorageRewards := GetPreStorageActualRewards(state)
	preSurplusRewards := big.NewInt(0)
	//when now is the start of year, preSurplusRewards should get "Pre + SurplusCoinAddress"
	if blockNumber%(config.Epoch*calEpochPerYear(config)) == 0 {
		preSurplusRewards = GetPreSurplusCoin(state)
	} else {
		preSurplusRewards = GetSurplusCoin(state)
	}

	genaroPrice := state.GetGenaroPrice()
	storageRewardsRatio := common.Base * genaroPrice.StorageRewardsRatio / 100
	ratioPerYear := common.Base * genaroPrice.RatioPerYear / 100

	coefficient := getStorageCoefficient(config, preStorageRewards, preSurplusRewards, storageRewardsRatio, ratioPerYear)

	surplusRewards := GetSurplusCoin(state)

	//plan rewards per year
	planRewards := big.NewInt(0)
	planRewards.Mul(surplusRewards, big.NewInt(int64(storageRewardsRatio)))
	planRewards.Div(planRewards, big.NewInt(int64(common.Base)))
	//get storageReward perYear
	planRewards.Mul(planRewards, big.NewInt(int64(ratioPerYear)))
	planRewards.Div(planRewards, big.NewInt(int64(common.Base)))
	//plan rewards per epoch
	planRewards.Div(planRewards, big.NewInt(int64(calEpochPerYear(config))))
	//Coefficient adjustment
	planRewards.Mul(planRewards, big.NewInt(int64(coefficient)))
	planRewards.Div(planRewards, big.NewInt(int64(common.Base)))
	//allocate blockReward
	cs := state.GetCandidates()
	total := uint64(0)
	contributes := make([]uint64, len(cs))
	for i, c := range cs {
		//contributes[i] = state.GetHeftLastDiff(c, blockNumber)
		contributes[i] = state.GetHeftRangeDiff(c, blockNumber-config.Epoch, blockNumber)
		total += contributes[i]
	}
	if total == 0 {
		return nil
	}

	for i, c := range cs {
		reward := big.NewInt(0)
		reward.Mul(planRewards, big.NewInt(int64(contributes[i])))
		reward.Div(planRewards, big.NewInt(int64(total)))
		state.AddBalance(c, reward)
		AddStorageActualRewards(state, reward)
	}
	return nil
}

// VerifyHeader checks whether a header conforms to the consensus rules of a
// given engine. Verifying the seal may be done optionally here, or explicitly
// via the VerifySeal method.
func (g *Genaro) VerifyHeader(chain consensus.ChainReader, header *types.Header, seal bool) error {
	log.Info("VerifyHeader:" + header.Number.String())
	return g.VerifySeal(chain, header)
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (g *Genaro) VerifyHeaders(chain consensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	log.Info("VerifyHeaders")
	abort := make(chan struct{})
	results := make(chan error, len(headers))

	go func() {
		for i, header := range headers {
			err := g.verifySeal(chain, header, headers[:i])
			if err != nil {
				log.Error(err.Error())
			}
			select {
			case <-abort:
				return
			case results <- err:
			}
		}
	}()
	return abort, results
}

// APIs implements consensus.Engine, returning the user facing RPC API
func (g *Genaro) APIs(chain consensus.ChainReader) []rpc.API {
	return []rpc.API{{
		Namespace: "genaro",
		Version:   "1.0",
		Service:   &API{chain: chain, genaro: g},
		Public:    false,
	}}
}
