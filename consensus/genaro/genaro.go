package genaro

import (
	"errors"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/GenaroNetwork/Genaro-Core/accounts"
	"github.com/GenaroNetwork/Genaro-Core/common"
	"github.com/GenaroNetwork/Genaro-Core/consensus"
	"github.com/GenaroNetwork/Genaro-Core/core/types"
	"github.com/GenaroNetwork/Genaro-Core/crypto/sha3"
	"github.com/GenaroNetwork/Genaro-Core/ethdb"
	"github.com/GenaroNetwork/Genaro-Core/log"
	"github.com/GenaroNetwork/Genaro-Core/params"
	"github.com/GenaroNetwork/Genaro-Core/rlp"
	"github.com/hashicorp/golang-lru"
	"github.com/GenaroNetwork/Genaro-Core/crypto"
	"github.com/GenaroNetwork/Genaro-Core/core/state"
	"github.com/GenaroNetwork/Genaro-Core/rpc"
	"sort"
)

const (
	wiggleTime					= 500 * time.Millisecond // Random delay (per signer) to allow concurrent signers
	inmemorySnapshots			= 128                    // Number of recent snapshots to keep in memory
	epochLength					= uint64(5000)           // Default number of blocks a turn
	epochPerYear				= uint64(12)
	SurplusCoinAddress			= "aaa"
	CoinActualRewardsAddress	= "bbb"
	StorageActualRewardsAddress	= "ccc"
	Pre							= "pre"
	TotalActualRewardsAddress	= "ggg"
)

var (
	totalRewards			*big.Int = big.NewInt(700000000e+18)
	coinRewardsRatio				 = 1250
	storageRewardsRatio				 = 1250
	ratioPerYear					 = 700
	base							 = 10000
)

var (
	// extra data is empty
	errEmptyExtra = errors.New("extra data is empty")
	// errUnauthorized is returned if a header is signed by a non-authorized entity.
	errUnauthorized = errors.New("unauthorized")
	// errUnauthorized is returned if epoch block has no committee list
	errInvalidEpochBlock = errors.New("epoch block has no committee list")
	errInvalidDifficulty = errors.New("invalid difficulty")
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
	//Why reset??
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

type Genaro struct {
	config     *params.GenaroConfig // genaro config
	db         ethdb.Database       // Database to store and retrieve snapshot checkpoints
	recents    *lru.ARCCache        // snapshot cache
	signer     common.Address       // Ethereum address of the signing key
	lock       sync.RWMutex         // Protects the signer fields
	signFn     SignerFn             // sign function
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
	return ecrecover(header)
}

// Prepare implements consensus.Engine, preparing all the consensus fields of the
// header for running the transactions on top.
func (g *Genaro) Prepare(chain consensus.ChainReader, header *types.Header) error {
	// set block author in Coinbase
	// TODO It may be modified later
	header.Coinbase = g.signer
	header.Nonce = types.BlockNonce{}
	number := header.Number.Uint64()

	currEpochNumber := GetTurnOfCommiteeByBlockNumber(g.config, number)

	snap, err := g.snapshot(chain, currEpochNumber-g.config.ValidPeriod-g.config.ElectionPeriod)
	if err != nil {
		return err
	}

	// Set the correct difficulty
	header.Difficulty = CalcDifficulty(snap, g.signer, number)
	// if we reach the point that should get Committee written in the block
	//if number == GetLastBlockNumberOfEpoch(g.config, currEpochNumber) {
	//	// get committee rank material period
	//	materialPeriod := currEpochNumber - g.config.ElectionPeriod
	//	// load committee rank from db or generateCommittee from material period
	//	writeSnap, err := g.snapshot(chain, materialPeriod)
	//	if err != nil {
	//		return err
	//	}
	//	// write the committee rank into Block's Extra
	//	err = SetHeaderCommitteeRankList(header, writeSnap.CommitteeRank)
	//	if err != nil {
	//		return err
	//	}
	//}
	// Mix digest is reserved for now, set to empty
	header.MixDigest = common.Hash{}
	// Ensure the timestamp has the correct delay
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	header.Time = new(big.Int).SetInt64(time.Now().Unix())
	if header.Time.Int64() < parent.Time.Int64() {
		header.Time = new(big.Int).SetInt64(parent.Time.Int64() + 1)
	}
	return nil
}

// Seal implements consensus.Engine, attempting to create a sealed block using
// the local signing credentials.
func (g *Genaro) Seal(chain consensus.ChainReader, block *types.Block, stop <-chan struct{}) (*types.Block, error) {
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

	// Sweet, wait some time if not in-turn
	delay := time.Duration(header.Difficulty.Uint64() * uint64(time.Second))
	delay += time.Duration(rand.Int63n(int64(wiggleTime)))

	log.Trace("Waiting for slot to sign and propagate", "delay", common.PrettyDuration(delay))

	select {
	case <-stop:
		return nil, nil
	case <-time.After(delay):
	}
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
	blockNumber := parent.Number.Uint64() + 1
	dependEpoch := GetDependTurnByBlockNumber(g.config, blockNumber)

	snap, err := g.snapshot(chain, dependEpoch)
	if err != nil {
		return nil
	}
	return CalcDifficulty(snap, g.signer, blockNumber)
}

// CalcDifficulty return the distance between my index and intern-index
// depend on snap
func CalcDifficulty(snap *CommitteeSnapshot, addr common.Address, blockNumber uint64) *big.Int {
	index := snap.getCurrentRankIndex(addr)
	if index < 0 {
		return new(big.Int).SetUint64(snap.CommitteeSize)
	}
	distance := blockNumber - uint64(index)
	if distance < 0 {
		distance = -distance
	}
	return new(big.Int).SetUint64(distance)
}

// Authorize injects a private key into the consensus engine to mint new blocks
// with.
func (g *Genaro) Authorize(signer common.Address, signFn SignerFn) {
	g.lock.Lock()
	defer g.lock.Unlock()

	g.signer = signer
	g.signFn = signFn
}

// Snapshot retrieves the snapshot at "electoral materials" period.
// Snapshot func retrieves ths snapshot in order of memory, local DB, block header.
// If committeeSnapshot is empty and it is time to write, we will create a new one, otherwise return nil
func (g *Genaro) snapshot(chain consensus.ChainReader, epollNumber uint64) (*CommitteeSnapshot, error) {
	// Search for a snapshot in memory or on disk for checkpoints
	var (
		snap *CommitteeSnapshot
	)
	isCreateNew := false
	// If an in-memory snapshot was found, use that
	if s, ok := g.recents.Get(epollNumber); ok {
		snap = s.(*CommitteeSnapshot)
	}else if epollNumber >= 0 && epollNumber < g.config.ElectionPeriod+g.config.ValidPeriod {
		// If we're at block 0 ~ ElectionPeriod + ValidPeriod - 1, make a snapshot by genesis block
		// TODO
		return nil, nil
	}else{
		// visit the blocks in epollNumber - ValidPeriod - ElectionPeriod tern
		startBlock := GetFirstBlockNumberOfEpoch(g.config, epollNumber-g.config.ValidPeriod-g.config.ElectionPeriod)
		endBlock := GetLastBlockNumberOfEpoch(g.config, epollNumber-g.config.ValidPeriod-g.config.ElectionPeriod)
		h := chain.GetHeaderByNumber(endBlock+1)
		var proportion []uint64
		snap.CommitteeRank, proportion = GetHeaderCommitteeRankList(h)
		snap.Committee = make(map[common.Address]uint64, len(proportion))
		for i := 0; i < len(proportion); i++{
			snap.Committee[snap.CommitteeRank[i]] = proportion[i]
		}

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
	blockNumber := header.Number.Uint64()
	if blockNumber == 0 {
		return errUnknownBlock
	}
	// Don't waste time checking blocks from the future
	if header.Time.Cmp(big.NewInt(time.Now().Unix())) > 0 {
		return consensus.ErrFutureBlock
	}
	// check epoch point
	epochPoint := (blockNumber % g.config.Epoch) == 0
	if epochPoint {
		extraData := UnmarshalToExtra(header)
		committeeSize := uint64(len(extraData.CommitteeRank) / common.AddressLength)
		if committeeSize == 0 || committeeSize > g.config.CommitteeMaxSize {
			return errInvalidEpochBlock
		}
	}
	// get current committee snapshot
	currentEpochNumber := GetTurnOfCommiteeByBlockNumber(g.config, blockNumber)
	snap, err := g.snapshot(chain, currentEpochNumber-g.config.ValidPeriod-g.config.ElectionPeriod)
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

	// Ensure the timestamp has the correct delay
	parent := chain.GetHeader(header.ParentHash, blockNumber-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	if header.Time.Uint64() < parent.Time.Uint64() {
		return errUnknownBlock
	}
	// Ensure that difficulty corresponds to the turn of the signer
	inturn := snap.inturn(blockNumber, signer)
	if !inturn {
		bias := header.Difficulty.Uint64()
		delay := uint64(time.Duration(bias * uint64(time.Second)))
		if parent.Time.Uint64()+delay >= header.Time.Uint64() {
			return errInvalidDifficulty
		}
	}
	return nil
}

// VerifyUncles implements consensus.Engine, always returning an error for any
// uncles as this consensus mechanism doesn't permit uncles.
func (g *Genaro) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	if len(block.Uncles()) > 0 {
		return errors.New("uncles not allowed")
	}
	return nil
}

func (g *Genaro) rank(candidateInfos state.CandidateInfos) ([]common.Address, []uint64){
	candidateInfos.Apply()
	sort.Sort(candidateInfos)
	committeeRank := make([]common.Address, len(candidateInfos))
	proportion := make([]uint64, len(candidateInfos))
	total := uint64(0)
	for _, c := range candidateInfos{
		total += c.Stake
	}
	for i, c := range candidateInfos{
		committeeRank[i] = c.Signer
		proportion[i] = c.Stake*uint64(base)/total
	}

	return committeeRank, proportion
}

func updateEpochRewards(state *state.StateDB)  {
	//reset CoinActualRewards and StorageActualRewards, add TotalActualRewards
	coinrewards := state.GetBalance(common.BytesToAddress([]byte(CoinActualRewardsAddress)))
	storagerewards := state.GetBalance(common.BytesToAddress([]byte(StorageActualRewardsAddress)))

	state.SetBalance(common.BytesToAddress([]byte(Pre + CoinActualRewardsAddress)), coinrewards)
	state.SetBalance(common.BytesToAddress([]byte(Pre + StorageActualRewardsAddress)), storagerewards)

	state.SetBalance(common.BytesToAddress([]byte(CoinActualRewardsAddress)), big.NewInt(0))
	state.SetBalance(common.BytesToAddress([]byte(StorageActualRewardsAddress)), big.NewInt(0))

	state.AddBalance(common.BytesToAddress([]byte(TotalActualRewardsAddress)), coinrewards)
	state.AddBalance(common.BytesToAddress([]byte(TotalActualRewardsAddress)), storagerewards)
}

func updateEpochYearRewards(state *state.StateDB)  {
	surplusrewards := state.GetBalance(common.BytesToAddress([]byte(SurplusCoinAddress)))
	state.SetBalance(common.BytesToAddress([]byte(Pre + SurplusCoinAddress)), surplusrewards)

	totalRewards := state.GetBalance(common.BytesToAddress([]byte(TotalActualRewardsAddress)))
	state.SubBalance(common.BytesToAddress([]byte(SurplusCoinAddress)), totalRewards)
	state.SetBalance(common.BytesToAddress([]byte(TotalActualRewardsAddress)), big.NewInt(0))
}

// Finalize implements consensus.Engine, ensuring no uncles are set, nor block
// rewards given, and returns the final block.
func (g *Genaro) Finalize(chain consensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
	//commit rank
	blockNumber := header.Number.Uint64()
	if blockNumber%g.config.Epoch == 0 {
		//rank
		epochStartBlockNumber := blockNumber - g.config.Epoch
		epochEndBlockNumber := blockNumber
		candidateInfos := state.GetCandidatesInfoInRange(epochStartBlockNumber, epochEndBlockNumber)
		commiteeRank, proportion := g.rank(candidateInfos)
		if uint64(len(candidateInfos)) <= g.config.CommitteeMaxSize {
			SetHeaderCommitteeRankList(header, commiteeRank, proportion)
		}else{
			SetHeaderCommitteeRankList(header, commiteeRank[:g.config.CommitteeMaxSize],proportion[:g.config.CommitteeMaxSize])
		}
		//CoinActualRewards and StorageActualRewards should update per epoch
		updateEpochRewards(state)
	}
	if blockNumber%(epochPerYear*g.config.Epoch) == 0 {
		//CoinActualRewards and StorageActualRewards should update per epoch, surplusCoin should update per year
		updateEpochYearRewards(state)
	}

	currEpochNumber := GetTurnOfCommiteeByBlockNumber(g.config, header.Number.Uint64())

	snap, err := g.snapshot(chain, currEpochNumber-g.config.ValidPeriod-g.config.ElectionPeriod)
	if err != nil {
		return nil, err
	}
	proportion := snap.Committee[header.Coinbase]
	//  coin interest reward
	accumulateInterestRewards(g.config, state, header, chain, proportion, blockNumber)
	// storage reward
	accumulateStorageRewards(g.config, state, header, chain, blockNumber)

	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
	header.UncleHash = types.CalcUncleHash(nil)

	// Assemble and return the final block for sealing
	return types.NewBlock(header, txs, nil, receipts), nil
}

func getCoinCofficient(coinrewards, surplusRewards *big.Int) uint64 {
	planrewards := big.NewInt(0)
	//get total coinReward
	planrewards.Mul(surplusRewards, big.NewInt(int64(coinRewardsRatio)))
	planrewards.Div(planrewards, big.NewInt(int64(base)))
	//get coinReward perYear
	planrewards.Div(planrewards, big.NewInt(int64(ratioPerYear)))
	planrewards.Mul(planrewards, big.NewInt(int64(base)))
	//get coinReward perEpoch
	planrewards.Div(planrewards, big.NewInt(int64(epochLength)))
	//get coefficient
	planrewards.Mul(planrewards, big.NewInt(int64(base)))
	coinRatio := planrewards.Div(planrewards, coinrewards).Uint64()
	return coinRatio
}

func getStorageCoefficient(storagerewards, surplusRewards *big.Int) uint64 {
	planrewards := big.NewInt(0)
	//get total storageReward
	planrewards.Mul(surplusRewards, big.NewInt(int64(storageRewardsRatio)))
	planrewards.Div(planrewards, big.NewInt(int64(base)))
	//get storageReward perYear
	planrewards.Div(planrewards, big.NewInt(int64(ratioPerYear)))
	planrewards.Mul(planrewards, big.NewInt(int64(base)))
	//get storageReward perEpoch
	planrewards.Div(planrewards, big.NewInt(int64(epochLength)))
	//get coefficient
	planrewards.Mul(planrewards, big.NewInt(int64(base)))
	storageRatio := planrewards.Div(planrewards, storagerewards).Uint64()
	return storageRatio
}

// AccumulateInterestRewards credits the reward to the block author by coin  interest
func accumulateInterestRewards(config *params.GenaroConfig, state *state.StateDB, header *types.Header, chain consensus.ChainReader, proportion uint64, blockNumber uint64) error {
	preCoinRewards := state.GetBalance(common.BytesToAddress([]byte(Pre + CoinActualRewardsAddress)))
	preSurplusRewards := big.NewInt(0)
	//when now is the start of year, preSurplusRewards should get "Pre + SurplusCoinAddress"
	if blockNumber%(epochLength*epochPerYear) == 0 {
		preSurplusRewards = state.GetBalance(common.BytesToAddress([]byte(Pre + SurplusCoinAddress)))
	}else{
		preSurplusRewards = state.GetBalance(common.BytesToAddress([]byte(SurplusCoinAddress)))
	}
	coefficient := getCoinCofficient(preCoinRewards, preSurplusRewards)

	surplusRewards := state.GetBalance(common.BytesToAddress([]byte(SurplusCoinAddress)))
	//plan rewards per year
	planRewards := surplusRewards.Mul(surplusRewards, big.NewInt(int64(coinRewardsRatio)))
	planRewards.Div(planRewards, big.NewInt(int64(base)))
	//plan rewards per epoch
	planRewards.Div(planRewards, big.NewInt(int64(epochPerYear)))
	//Coefficient adjustment
	planRewards.Mul(planRewards, big.NewInt(int64(coefficient)))
	planRewards.Div(planRewards, big.NewInt(int64(base)))
	//this addr should get
	planRewards.Mul(planRewards, big.NewInt(int64(proportion)))
	planRewards.Div(planRewards, big.NewInt(int64(base)))

	blockReward := big.NewInt(0)
	blockReward = planRewards.Div(planRewards, big.NewInt(int64(config.Epoch)))

	reward := new(big.Int).SetUint64(blockReward.Uint64())
	state.AddBalance(header.Coinbase, reward)
	state.AddBalance(common.BytesToAddress([]byte(CoinActualRewardsAddress)), reward)
	return nil
}

// AccumulateStorageRewards credits the reward to the sentinel owner
func accumulateStorageRewards(config *params.GenaroConfig, state *state.StateDB, header *types.Header, chain consensus.ChainReader, blockNumber uint64) error {
	preStorageRewards := state.GetBalance(common.BytesToAddress([]byte(Pre + StorageActualRewardsAddress)))
	preSurplusRewards := big.NewInt(0)
	//when now is the start of year, preSurplusRewards should get "Pre + SurplusCoinAddress"
	if blockNumber%(epochLength*epochPerYear) == 0 {
		preSurplusRewards = state.GetBalance(common.BytesToAddress([]byte(Pre + SurplusCoinAddress)))
	}else{
		preSurplusRewards = state.GetBalance(common.BytesToAddress([]byte(SurplusCoinAddress)))
	}
	coefficient := getCoinCofficient(preStorageRewards, preSurplusRewards)

	surplusRewards := state.GetBalance(common.BytesToAddress([]byte(SurplusCoinAddress)))
	//plan rewards per year
	planRewards := surplusRewards.Mul(surplusRewards, big.NewInt(int64(storageRewardsRatio)))
	planRewards.Div(planRewards, big.NewInt(int64(base)))
	//plan rewards per epoch
	planRewards.Div(planRewards, big.NewInt(int64(epochPerYear)))
	//Coefficient adjustment
	planRewards.Mul(planRewards, big.NewInt(int64(coefficient)))
	planRewards.Div(planRewards, big.NewInt(int64(base)))
	//plan rewards per block
	blockReward := big.NewInt(0)
	blockReward = planRewards.Div(planRewards, big.NewInt(int64(config.Epoch)))

	//allocate blockReward
	cs := state.GetCandidates()
	total := uint64(0)
	contributes := make([]uint64, len(cs))
	for i, c := range cs{
		contributes[i] = state.GetHeftLastDiff(c, blockNumber)
		total += contributes[i]
	}

	for i, c := range cs{
		reward := big.NewInt(0)
		reward.Mul(blockReward, big.NewInt(int64(contributes[i])))
		reward.Div(blockReward, big.NewInt(int64(total)))
		state.AddBalance(c, reward)
		state.AddBalance(common.BytesToAddress([]byte(StorageActualRewardsAddress)), reward)
	}
	return nil
}

// VerifyHeader checks whether a header conforms to the consensus rules of a
// given engine. Verifying the seal may be done optionally here, or explicitly
// via the VerifySeal method.
func (g *Genaro) VerifyHeader(chain consensus.ChainReader, header *types.Header, seal bool) error {
	return g.VerifySeal(chain, header)
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (g *Genaro) VerifyHeaders(chain consensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))

	go func() {
		for _, header := range headers {
			err := g.VerifySeal(chain, header)

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
