package genaro

import (
	"testing"
	"github.com/GenaroNetwork/Genaro-Core/params"
	"fmt"
	"github.com/GenaroNetwork/Genaro-Core/core/types"
)

func TestGenaro(t *testing.T){
	db, remove := newTestLDB()
	defer remove()

	genaro := New(params.MainnetChainConfig.Genaro,db)
	fmt.Printf("%s\n", genaro)
	fmt.Println(genaro.signer)
	fmt.Println(genaro.signFn)

	n := 10
	addrs := genAddrs(n)
	byt := CreateCommitteeRankByte(addrs)
	head := types.Header{
		Extra:byt,
	}

	addr,err := genaro.Author(&head)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(addr.String())

}