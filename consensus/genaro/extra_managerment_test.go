package genaro

import (
	"fmt"
	"testing"
	"github.com/GenaroNetwork/Genaro-Core/crypto"
	"github.com/GenaroNetwork/Genaro-Core/common"
)

func TestCreateCommitteeRankByte(t *testing.T) {
	addrs := make([]common.Address, 0)

	for i := 0; i < 10; i++ {
		prikey, _ := crypto.GenerateKey()
		addr := crypto.PubkeyToAddress(prikey.PublicKey)

		fmt.Println(addr.String())
		addrs = append(addrs, addr)
	}
	byt := CreateCommitteeRankByte(addrs)
	fmt.Printf("%s", byt)
}
