package genaro

import (
	"github.com/GenaroNetwork/GenaroCore/common"
	"github.com/GenaroNetwork/GenaroCore/consensus"
)

// API is a user facing RPC API to allow controlling the signer and voting
// mechanisms of the proof-of-authority scheme.
type API struct {
	chain  consensus.ChainReader
	genaro *Genaro
}

// GetSnapshot retrieves the state snapshot at a given epochNumber.
func (api *API) GetSnapshot(epochNumber uint64) (*CommitteeSnapshot, error) {
	// Retrieve the requested block number (or current if none requested)
	//Todo add some check
	return api.genaro.snapshot(api.chain, epochNumber, nil)
}

// GetCommittee return the member of committee
func (api *API) GetBlockCommittee(epochNumber uint64) []common.Address {
	header := api.chain.CurrentHeader()

	if header == nil {
		return nil
	}
	turn := GetTurnOfCommiteeByBlockNumber(api.genaro.config, header.Number.Uint64())
	writeNo := GetCommiteeWrittenBlockNumberByTurn(api.genaro.config, turn)
	commitee, _ := GetHeaderCommitteeRankList(api.chain.GetHeaderByNumber(writeNo))
	return commitee
}
