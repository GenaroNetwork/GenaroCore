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
	LastSynBlockNum  uint64           `json:"lastBlockNum"`  //sentinelHeft
	Signature     []byte           `json:"signature"`     // the signature of block broadcaster
	Proportion	  []uint64		   `json:"ratio"`
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

	header.Extra = make([]byte, len(extraByte))
	copy(header.Extra, extraByte)
}

func SetHeaderSignature(header *types.Header, signature []byte) {
	extraData := UnmarshalToExtra(header)
	extraData.Signature = make([]byte, len(signature))
	copy(extraData.Signature, signature)
	extraByte, _ := json.Marshal(extraData)
	header.Extra = make([]byte, len(extraByte))
	copy(header.Extra, extraByte)
}


func SetHeaderCommitteeRankList(header *types.Header, committeeRank []common.Address, proportion []uint64) error {
	extraData := UnmarshalToExtra(header)
	extraData.CommitteeRank = make([]common.Address, len(committeeRank))
	copy(extraData.CommitteeRank, committeeRank)
	extraData.Proportion = make([]uint64, len(proportion))
	copy(extraData.Proportion, proportion)
	extraByte, err := json.Marshal(extraData)
	if err != nil {
		return err
	}
	header.Extra = make([]byte, len(extraByte))
	copy(header.Extra, extraByte)
	return nil
}

//func SetHeaderSentinelHeft(header *types.Header, sentinelHeft uint64) {
//	extraData := UnmarshalToExtra(header)
//	extraData.SentinelHeft = sentinelHeft
//	extraByte, _ := json.Marshal(extraData)
//	header.Extra = make([]byte, len(extraByte))
//	copy(header.Extra, extraByte)
//}
//
//func GetHeaderSentinelHeft(header *types.Header) uint64{
//	extraData := UnmarshalToExtra(header)
//	return extraData.SentinelHeft
//}

func GetHeaderCommitteeRankList(header *types.Header) ([]common.Address, []uint64) {
	extraData := UnmarshalToExtra(header)
	return extraData.CommitteeRank, extraData.Proportion
}

func CreateCommitteeRankByte(address []common.Address) []byte {
	extra := new(ExtraData)
	extra.CommitteeRank = make([]common.Address, len(address))
	copy(extra.CommitteeRank, address)
	extraByte, _ := json.Marshal(extra)
	return extraByte

}
