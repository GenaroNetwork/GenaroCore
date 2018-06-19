package genaro

import (
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

		//fmt.Println(addr.String())
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

func TestHeaderSignature(t *testing.T){
	n := 10
	data := types.Header{}
	sig := genSignature(n)
	if sig == nil {
		t.Fatalf("genSignature error")
	}
	SetHeaderSignature(&data,sig)
	extra := UnmarshalToExtra(&data)
	if bytes.Compare(extra.Signature,sig) != 0 {
		t.Fatalf("SetHeaderSignature error")
	}

	ResetHeaderSignature(&data)
	extra = UnmarshalToExtra(&data)
	if extra.Signature != nil {
		t.Fatalf("ResetHeaderSignature error")
	}
}

func TestHeaderCommitteeRankList(t *testing.T){
	n := 10
	addrs := genAddrs(n)
	byt := CreateCommitteeRankByte(addrs)
	data := types.Header{
		Extra:byt,
	}
	extra := UnmarshalToExtra(&data)
	for i:=0;i<n;i++ {
		if bytes.Compare(extra.CommitteeRank[i].Bytes(), addrs[i].Bytes()) !=0 {
			t.Fatalf("CreateCommitteeRankByte error")
		}
	}

	addrs2 := genAddrs(n)
	proportion := genProportion(10)
	SetHeaderCommitteeRankList(&data,addrs2,proportion)
	extra = UnmarshalToExtra(&data)
	for i:=0;i<n;i++ {
		if bytes.Compare(extra.CommitteeRank[i].Bytes(), addrs2[i].Bytes()) !=0 {
			t.Fatalf("SetHeaderCommitteeRankList CommitteeRank set error")
		}
		if extra.Proportion[i] != proportion[i] {
			t.Fatalf("SetHeaderCommitteeRankList Proportion set error")
		}
	}

	addrs3,proportion := GetHeaderCommitteeRankList(&data)
	for i:=0;i<n;i++ {
		if bytes.Compare(addrs3[i].Bytes(), addrs2[i].Bytes()) !=0 {
			t.Fatalf("GetHeaderCommitteeRankList CommitteeRank get error")
		}
		if extra.Proportion[i] != proportion[i] {
			t.Fatalf("GetHeaderCommitteeRankList Proportion get error")
		}
	}
}
