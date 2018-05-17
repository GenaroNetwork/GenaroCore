package genaro

import (
	"github.com/GenaroNetwork/Genaro-Core/common"
	"github.com/GenaroNetwork/Genaro-Core/core/types"
	"encoding/json"
)

// the field "extra" store the json of ExtraData
// TODO re-design the struct to speed up
type ExtraData struct {
	CommitteeRank []common.Address `json:"committeeRank"` // rank of committee
	EVMData       []byte           `json:"eVMData"`       // evm data
	Signature     []byte           `json:"signature"`     // the signature of block broadcaster
}

func UnmarshalToExtra(header *types.Header) *ExtraData {
	result := new(ExtraData)
	json.Unmarshal(header.Extra, result)
	return result
}

func ResetHeaderSignature(header *types.Header) {
	extraData := UnmarshalToExtra(header)
	extraData.Signature = nil
	extraByte, _ := json.Marshal(extraData)
	header.Extra = extraByte
}

func SetHeaderSignature(header *types.Header, signature []byte) {
	extraData := UnmarshalToExtra(header)
	copy(extraData.Signature, signature)
	extraByte, _ := json.Marshal(extraData)
	copy(header.Extra, extraByte)
}

func SetHeaderCommitteeRankList(header *types.Header, committeeRank []common.Address) error {
	extraData := UnmarshalToExtra(header)
	copy(extraData.CommitteeRank, committeeRank)
	extraByte, err := json.Marshal(extraData)
	if err != nil {
		return err
	}
	copy(header.Extra, extraByte)
	return nil
}

func CreateCommitteeRankByte(address []common.Address) []byte {
	extra := new(ExtraData)
	copy(extra.CommitteeRank, address)
	extraByte, _ := json.Marshal(extra)
	return extraByte
}
