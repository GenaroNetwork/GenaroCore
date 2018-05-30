package genaro

import (
	"fmt"
	"testing"
	"github.com/GenaroNetwork/Genaro-Core/crypto"
	"github.com/GenaroNetwork/Genaro-Core/common"
	"github.com/GenaroNetwork/Genaro-Core/core/types"
	"bytes"
	"math/rand"
)

func genAddrs(n int)[]common.Address{
	addrs := make([]common.Address, 0)

	for i := 0; i < n; i++ {
		prikey, _ := crypto.GenerateKey()
		addr := crypto.PubkeyToAddress(prikey.PublicKey)

		fmt.Println(addr.String())
		addrs = append(addrs, addr)
	}
	return addrs
}

func genSignature(n int) []byte{
	signature := make([]byte,n)
	for i:=0;i<n;i++ {
		signature[i] = byte(rand.Int())
	}
	return signature
}

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

func TestExtraData(t *testing.T){
	n := 10
	addrs := genAddrs(n)
	byt := CreateCommitteeRankByte(addrs)
	data := types.Header{
		Extra:byt,
	}
	//fmt.Printf("%s\n",data)
	extra := UnmarshalToExtra(&data)
	fmt.Println(extra.CommitteeRank[0].String())
	for i:=0;i<n;i++ {
		if bytes.Compare(extra.CommitteeRank[i].Bytes(), addrs[i].Bytes()) !=0 {
			t.Error("TestExtraData UnmarshalToExtra error")
		}
	}

	addrs2 := genAddrs(n)
	SetHeaderCommitteeRankList(&data,addrs2)
	extra = UnmarshalToExtra(&data)
	for i:=0;i<n;i++ {
		if bytes.Compare(extra.CommitteeRank[i].Bytes(), addrs2[i].Bytes()) !=0 {
			t.Error("TestExtraData SetHeaderCommitteeRankList error")
		}
	}

	addr3 := GetHeaderCommitteeRankList(&data)
	for i:=0;i<n;i++ {
		if bytes.Compare(addr3[i].Bytes(), addrs2[i].Bytes()) !=0 {
			t.Error("TestExtraData GetHeaderCommitteeRankList error")
		}
	}

	sig := genSignature(n)
	SetHeaderSignature(&data,sig)
	extra = UnmarshalToExtra(&data)
	if bytes.Compare(extra.Signature,sig) != 0 {
		t.Error("TestExtraData SetHeaderSignature error")
	}

	ResetHeaderSignature(&data)
	extra = UnmarshalToExtra(&data)
	if extra.Signature != nil {
		t.Error("TestExtraData ResetHeaderSignature error")
	}

}

