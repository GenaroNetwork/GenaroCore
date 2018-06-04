package genaro

import (
	"testing"
	"github.com/GenaroNetwork/Genaro-Core/params"
	"fmt"
	"github.com/GenaroNetwork/Genaro-Core/core/types"
	"github.com/GenaroNetwork/Genaro-Core/crypto"
	"github.com/GenaroNetwork/Genaro-Core/common/hexutil"
	"log"
	"bytes"
)

func TestGenaroSign(t *testing.T){
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

	prikey,err := crypto.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}
	addr := crypto.PubkeyToAddress(prikey.PublicKey)
	fmt.Println("addr")
	fmt.Println(hexutil.Encode(addr.Bytes()))

	hash := sigHash(&head)
	sig,err := crypto.Sign(hash.Bytes(), prikey)
	if err != nil {
		log.Fatal(err)
	}

	genaro.signer = addr
	SetHeaderSignature(&head, sig)

	signer,err := genaro.Author(&head)
	if err != nil {
		t.Error(err)
	}
	fmt.Println("singer")
	fmt.Println(hexutil.Encode(signer.Bytes()))

	if bytes.Compare(addr.Bytes(),signer.Bytes()) != 0 {
		t.Error("sign error")
	}

}


