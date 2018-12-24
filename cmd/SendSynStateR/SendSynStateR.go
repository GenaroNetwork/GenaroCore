package main

import (
	"flag"
	"fmt"
	"github.com/GenaroNetwork/GenaroCore/cmd/utils"
	"github.com/hashicorp/consul/api"
	"github.com/tidwall/gjson"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var rpcurl string
var delaytime int64
var SynStateAccount string
var SynStateAccountPasswd string
var consulAddr string

func logPrint(msg string) {
	log.Println(msg)
}

func initarg() {
	flag.Int64Var(&delaytime, "t", 1, "delay time")
	flag.StringVar(&rpcurl, "u", "http://127.0.0.1:8545", "rpc url")
	flag.StringVar(&SynStateAccount, "a", "0xad188b762f9e3ef76c972960b80c9dc99b9cfc73", "state syn account")
	flag.StringVar(&consulAddr, "h", "127.0.0.1:8500", "consul address")
	flag.Parse()
}

var lastSynBlockHash string = ""
var preTxHash string = ""

func SynState(client *utils.ConsulClient) bool {
	cuBlockNum, err := utils.GetCuBlockNum(rpcurl)
	if err != nil {
		logPrint(err.Error())
		return false
	}
	fmt.Println(cuBlockNum)
	synBlockNum := cuBlockNum / 6
	if synBlockNum != 0 {
		synBlockNumCluster, err := client.GetUint64("synBlockNum")
		if err != nil {
			logPrint(err.Error())
			return false
		}
		if synBlockNumCluster > synBlockNum {
			logPrint("synblock number is older")
			return false
		} else if synBlockNumCluster < synBlockNum {
			client.PutUint64("synBlockNum", synBlockNum)
		}

		synBlockHashPre, err := utils.GetLastSynBlockHash(rpcurl)
		if err != nil {
			logPrint(err.Error())
			return false
		}
		synBlockHash, err := utils.GetBlockHash(rpcurl, synBlockNum*6)
		if err != nil {
			logPrint(err.Error())
			return false
		}
		ok, err := utils.CheckTransaction(rpcurl, preTxHash)
		if err != nil {
			logPrint(err.Error())
			return false
		}
		if !ok {
			lastSynBlockHash = ""
		}
		ok, err = utils.CheckRecipt(rpcurl, preTxHash)
		if err != nil {
			logPrint(err.Error())
			return false
		}
		if !ok {
			lastSynBlockHash = ""
		}
		if strings.EqualFold(lastSynBlockHash, synBlockHash) {
			logPrint("BlockHash is exist")
			return false
		}
		fmt.Println(synBlockHash)
		if strings.EqualFold(synBlockHashPre, synBlockHash) {
			logPrint("syn state is exist")
			return false
		}
		utils.AccountUnlock(rpcurl, SynStateAccount, SynStateAccountPasswd)
		ret, err := utils.SendSynState(rpcurl, synBlockHash, SynStateAccount)
		if err != nil {
			logPrint(err.Error())
			return false
		}

		preTxHash = gjson.Get(ret, "result").String()
		lastSynBlockHash = synBlockHash
		logPrint(ret)
	}
	return true
}

func sigDeal(lock *api.Lock) {
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	go lock.Unlock()
	time.Sleep(time.Second)
	fmt.Println("exit")
	syscall.Exit(0)
}

func main() {
	initarg()
	fmt.Printf("Please enter your account password: ")
	fmt.Scanln(&SynStateAccountPasswd)

	client, err := utils.NewClient(consulAddr)
	if err != nil {
		log.Fatal(err)
	}

	lock, err := client.Client.LockKey("SynLock")
	if err != nil {
		log.Fatal(err)
	}

	go sigDeal(lock)

	for {
		ok := utils.TryLock(lock, 10)
		if !ok {
			continue
		}

	LOOP:
		for {
			var idx int
			ok := SynState(client)
			if !ok {
				idx++
			}
			time.Sleep(time.Duration(delaytime) * time.Second)
			if idx > 100 {
				break LOOP
			}
		}
		lock.Unlock()
	}

}
