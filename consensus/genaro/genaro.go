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
)

const (
	wiggleTime        = 500 * time.Millisecond // Random delay (per signer) to allow concurrent signers
	inmemorySnapshots = 128                    // Number of recent vote snapshots to keep in memory
	epochLength       = uint64(5000)           // Default number of blocks a turn
)

var (
	// extra data is empty
	errEmptyExtra = errors.New("extra data is empty")

	currentCommittee CommitteeSnapshot //current snapshot
)

// Various error messages to mark blocks invalid. These should be private to
// prevent engine specific errors from being referenced in the remainder of the
// codebase, inherently breaking if the engine is swapped out. Please put common
// error types into the consensus package.
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

type Genaro struct {
	config     *params.GenaroConfig //genaro config
	snapshotDb ethdb.Database       // snapshot db
	sentinelDb ethdb.Database       //sentinel db
	recents    *lru.ARCCache        // snapshot cache
	signer     common.Address       // Ethereum address of the signing key
	lock       sync.RWMutex         // Protects the signer fields
	signFn     SignerFn             //sign function
}

// New creates a Clique proof-of-authority consensus engine with the initial
// signers set to the ones provided by the user.
func New(config *params.GenaroConfig, snapshotDb ethdb.Database, sentinelDb ethdb.Database) *Genaro {
	// Set any missing consensus parameters to their defaults
	conf := *config
	if conf.Epoch == 0 {
		conf.Epoch = epochLength
	}
	// Allocate the snapshot caches and create the engine
	recents, _ := lru.NewARC(inmemorySnapshots)

	return &Genaro{
		config:     &conf,
		snapshotDb: snapshotDb,
		sentinelDb: sentinelDb,
		recents:    recents,
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
	if number == GetLastBlockNumberOfEpoch(g.config, currEpochNumber) {
		// get committee rank material period
		materialPeriod := currEpochNumber - g.config.ElectionPeriod
		// load committee rank from db or generateCommittee from material period
		writeSnap, err := g.snapshot(chain, materialPeriod)
		if err != nil {
			return err
		}
		// write the committee rank into Block's Extra
		err = SetHeaderCommitteeRankList(header, writeSnap.CommitteeRank)
		if err != nil {
			return err
		}
	}
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
	snap, err := g.snapshot(chain, parent.Number.Uint64())
	if err != nil {
		return nil
	}
	blockNumber := parent.Number.Uint64() + 1
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
// TODO
func (g *Genaro) snapshot(chain consensus.ChainReader, epollNumber uint64) (*CommitteeSnapshot, error) {
	return nil, nil
}
