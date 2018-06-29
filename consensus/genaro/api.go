// Copyright 2017 The go-ethereum Authors
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

package genaro

import (
	"github.com/GenaroNetwork/Genaro-Core/consensus"
	//"github.com/GenaroNetwork/Genaro-Core/common/number"
	"github.com/GenaroNetwork/Genaro-Core/common"
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
	return api.genaro.snapshot(api.chain, epochNumber,nil)
}

// GetCommittee return the member of committee
func (api *API) GetBlockCommittee(epochNumber uint64) ([] common.Address) {
	header := api.chain.CurrentHeader()

	if header == nil{
		return nil
	}
	turn := GetTurnOfCommiteeByBlockNumber(api.genaro.config, header.Number.Uint64())
	writeNo := GetCommiteeWrittenBlockNumberByTurn(api.genaro.config, turn)
	commitee, _ := GetHeaderCommitteeRankList(api.chain.GetHeaderByNumber(writeNo))
	return commitee
}
