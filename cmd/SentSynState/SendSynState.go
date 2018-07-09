package main

import (
	"flag"
	"net/http"
	"io/ioutil"
	"strings"
	"github.com/tidwall/gjson"
	"github.com/GenaroNetwork/Genaro-Core/common/hexutil"
	"github.com/GenaroNetwork/Genaro-Core/common"
	"fmt"
	"time"
	"log"
	"errors"
)

var rpcurl string
var delaytime int64

func logPrint(msg string){
	log.Println(msg)
}

func checkError(ret []byte) error{
	errStr := gjson.GetBytes(ret,"error").String()
	if !strings.EqualFold("",errStr){
		return errors.New(errStr)
	} else {
		return nil
	}
}

func HttpPost(url string, contentType string, body string) ([]byte, error) {
	bodyio := strings.NewReader(body)
	resp, err := http.Post(url,contentType,bodyio)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	repbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return repbody, nil
}

func GetCuBlockNum(url string) (uint64,error){
	ret,err := HttpPost(url,"application/json",`{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`)
	if err != nil {
		return 0,err
	}
	err = checkError(ret)
	if err != nil {
		return 0,err
	}
	blockNumStr := gjson.GetBytes(ret,"result").String()
	blockNum,err := hexutil.DecodeUint64(blockNumStr)
	if err != nil {
		return 0,err
	}
	return blockNum,nil
}

func GetBlockByNumber(url string,blockNum uint64) ([]byte,error) {
	blockNumHex := hexutil.EncodeUint64(blockNum)
	ret,err := HttpPost(url,"application/json",`{"jsonrpc":"2.0","id":1,"method":"eth_getBlockByNumber","params":["`+blockNumHex+`",true]}`)
	if err != nil {
		return nil,err
	}
	err = checkError(ret)
	if err != nil {
		return nil,err
	}
	return ret,err
}

func GetBlockHash(url string,blockNum uint64) (string,error){
	ret,err := GetBlockByNumber(url,blockNum)
	if err != nil {
		return "",err
	}
	blockHash := gjson.GetBytes(ret,"result.hash").String()
	return blockHash,nil
}

func SendSynState(url string,blockHash string) (string,error){
	ret,err := HttpPost(url,"application/json",`{"jsonrpc": "2.0","method": "eth_sendTransaction","params": [{"from": "`+common.OfficialAddress.String()+`","to": "`+common.SpecialSyncAddress.String()+`","gasPrice": "0x430e23400","value": "0x1","extraData": "{\"msg\": \"`+blockHash+`\",\"type\": \"0xd\"}"}],"id": 1}`)
	if err != nil {
		return "",err
	}
	err = checkError(ret)
	if err != nil {
		return "",err
	}
	return gjson.ParseBytes(ret).String(),nil
}

func GetLastSynBlockInfo(url string) ([]byte,error){
	ret,err := HttpPost(url,"application/json",`{"jsonrpc":"2.0","method":"eth_getLastSynBlock","params":["latest"],"id":1}`)
	err = checkError(ret)
	if err != nil {
		return nil,err
	}
	return ret,err
}

func GetLastSynBlockHash(url string) (string,error){
	ret,err := GetLastSynBlockInfo(url)
	if err != nil {
		return "",err
	}
	hash := gjson.GetBytes(ret,"result.BlockHash").String()
	return hash,nil
}

func initarg() {
	flag.Int64Var(&delaytime,"t",1,"delay time")
	flag.StringVar(&rpcurl, "u", "http://127.0.0.1:8545", "rpc url")
	flag.Parse()
}

var lastSynBlockHash string = ""

func SynState(){
	cuBlockNum,err := GetCuBlockNum(rpcurl)
	if err != nil {
		logPrint(err.Error())
		return
	}
	fmt.Println(cuBlockNum)
	synBlockNum := cuBlockNum/6
	if synBlockNum != 0 {
		synBlockHashPre,err := GetLastSynBlockHash(rpcurl)
		if err != nil {
			logPrint(err.Error())
			return
		}
		synBlockHash,err := GetBlockHash(rpcurl,synBlockNum*6)
		if err != nil {
			logPrint(err.Error())
			return
		}
		if strings.EqualFold(lastSynBlockHash,synBlockHash) {
			logPrint("BlockHash is exist")
			return
		}
		fmt.Println(synBlockHash)
		if strings.EqualFold(synBlockHashPre,synBlockHash) {
			logPrint("syn state is exist")
			return
		}
		ret,err := SendSynState(rpcurl,synBlockHash)
		if err != nil {
			logPrint(err.Error())
			return
		}
		lastSynBlockHash = synBlockHash
		logPrint(ret)
	}
}

func main() {
	initarg()
	for {
		SynState()
		time.Sleep(time.Duration(delaytime)*time.Second)
	}
}
