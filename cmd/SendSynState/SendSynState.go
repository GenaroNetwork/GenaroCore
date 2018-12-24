package main

import (
	"flag"
	"fmt"
	"github.com/GenaroNetwork/GenaroCore/cmd/utils"
	"github.com/tidwall/gjson"
	"log"
	"strings"
	"time"
)

var rpcurl string
var delaytime int64
var SynStateAccount string

func logPrint(msg string) {
	log.Println(msg)
}

func initarg() {
	flag.Int64Var(&delaytime, "t", 1, "delay time")
	flag.StringVar(&rpcurl, "u", "http://127.0.0.1:8545", "rpc url")
	flag.StringVar(&SynStateAccount, "a", "0xad188b762f9e3ef76c972960b80c9dc99b9cfc73", "state syn account")
	flag.Parse()
}

var lastSynBlockHash string = ""
var preTxHash string = ""

func SynState() {
	cuBlockNum, err := utils.GetCuBlockNum(rpcurl)
	if err != nil {
		logPrint(err.Error())
		return
	}
	fmt.Println(cuBlockNum)
	synBlockNum := cuBlockNum / 6
	if synBlockNum != 0 {
		synBlockHashPre, err := utils.GetLastSynBlockHash(rpcurl)
		if err != nil {
			logPrint(err.Error())
			return
		}
		synBlockHash, err := utils.GetBlockHash(rpcurl, synBlockNum*6)
		if err != nil {
			logPrint(err.Error())
			return
		}
		ok, err := utils.CheckTransaction(rpcurl, preTxHash)
		if err != nil {
			logPrint(err.Error())
			return
		}
		if !ok {
			lastSynBlockHash = ""
		}
		ok, err = utils.CheckRecipt(rpcurl, preTxHash)
		if err != nil {
			logPrint(err.Error())
			return
		}
		if !ok {
			lastSynBlockHash = ""
		}
		if strings.EqualFold(lastSynBlockHash, synBlockHash) {
			logPrint("BlockHash is exist")
			return
		}
		fmt.Println(synBlockHash)
		if strings.EqualFold(synBlockHashPre, synBlockHash) {
			logPrint("syn state is exist")
			return
		}
		ret, err := utils.SendSynState(rpcurl, synBlockHash, SynStateAccount)
		if err != nil {
			logPrint(err.Error())
			return
		}

		preTxHash = gjson.Get(ret, "result").String()
		lastSynBlockHash = synBlockHash
		logPrint(ret)
	}
}

func main() {
	initarg()

	for {
		SynState()
		time.Sleep(time.Duration(delaytime) * time.Second)
	}
}
