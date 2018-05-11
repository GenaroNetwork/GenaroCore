package genaro

import "github.com/GenaroNetwork/Genaro-Core/common"

// the field "extra" store the json of ExtraData
// TODO re-design the struct to speed up
type ExtraData struct {
	Signature     []byte           `json:"signature"`     // the signature of block broadcaster
	CommitteeRank []common.Address `json:"committeeRank"` // rank of committee
	EVMData       []byte           `json:"eVMData"`       // evm data
}
