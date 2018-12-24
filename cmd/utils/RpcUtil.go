package utils

import (
	"errors"
	"github.com/GenaroNetwork/GenaroCore/common"
	"github.com/GenaroNetwork/GenaroCore/common/hexutil"
	"github.com/tidwall/gjson"
	"io/ioutil"
	"net/http"
	"strings"
)

func checkError(ret []byte) error {
	errStr := gjson.GetBytes(ret, "error").String()
	if !strings.EqualFold("", errStr) {
		return errors.New(errStr)
	} else {
		return nil
	}
}

func HttpPost(url string, contentType string, body string) ([]byte, error) {
	bodyio := strings.NewReader(body)
	resp, err := http.Post(url, contentType, bodyio)
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

func GetCuBlockNum(url string) (uint64, error) {
	ret, err := HttpPost(url, "application/json", `{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`)
	if err != nil {
		return 0, err
	}
	err = checkError(ret)
	if err != nil {
		return 0, err
	}
	blockNumStr := gjson.GetBytes(ret, "result").String()
	blockNum, err := hexutil.DecodeUint64(blockNumStr)
	if err != nil {
		return 0, err
	}
	return blockNum, nil
}

func GetBlockByNumber(url string, blockNum uint64) ([]byte, error) {
	blockNumHex := hexutil.EncodeUint64(blockNum)
	ret, err := HttpPost(url, "application/json", `{"jsonrpc":"2.0","id":1,"method":"eth_getBlockByNumber","params":["`+blockNumHex+`",true]}`)
	if err != nil {
		return nil, err
	}
	err = checkError(ret)
	if err != nil {
		return nil, err
	}
	return ret, err
}

func GetBlockHash(url string, blockNum uint64) (string, error) {
	ret, err := GetBlockByNumber(url, blockNum)
	if err != nil {
		return "", err
	}
	blockHash := gjson.GetBytes(ret, "result.hash").String()
	return blockHash, nil
}

func SendSynState(url string, blockHash string, SynStateAccount string) (string, error) {
	ret, err := HttpPost(url, "application/json", `{"jsonrpc": "2.0","method": "eth_sendTransaction","params": [{"from": "`+SynStateAccount+`","to": "`+common.SpecialSyncAddress.String()+`","gasPrice": "0x430e23400","value": "0x0","extraData": "{\"msg\": \"`+blockHash+`\",\"type\": \"0xd\"}"}],"id": 1}`)
	if err != nil {
		return "", err
	}
	err = checkError(ret)
	if err != nil {
		return "", err
	}
	return gjson.ParseBytes(ret).String(), nil
}

func GetLastSynBlockInfo(url string) ([]byte, error) {
	ret, err := HttpPost(url, "application/json", `{"jsonrpc":"2.0","method":"eth_getLastSynBlock","params":["latest"],"id":1}`)
	err = checkError(ret)
	if err != nil {
		return nil, err
	}
	return ret, err
}

func GetLastSynBlockHash(url string) (string, error) {
	ret, err := GetLastSynBlockInfo(url)
	if err != nil {
		return "", err
	}
	hash := gjson.GetBytes(ret, "result.BlockHash").String()
	return hash, nil
}

func CheckTransaction(url string, txHash string) (bool, error) {
	if strings.EqualFold("", txHash) {
		return true, nil
	}
	ret, err := HttpPost(url, "application/json", `{"jsonrpc":"2.0","id":10,"method":"eth_getTransactionByHash","params":["`+txHash+`"]}`)
	err = checkError(ret)
	if err != nil {
		return false, err
	}
	result := gjson.GetBytes(ret, "result").String()
	if strings.EqualFold(result, "") {
		return false, nil
	}
	return true, nil
}

func CheckRecipt(url string, txHash string) (bool, error) {
	if strings.EqualFold("", txHash) {
		return true, nil
	}
	ret, err := HttpPost(url, "application/json", `{"jsonrpc":"2.0","id":10,"method":"eth_getTransactionReceipt","params":["`+txHash+`"]}`)
	err = checkError(ret)
	if err != nil {
		return false, err
	}
	result := gjson.GetBytes(ret, "result").String()
	if strings.EqualFold(result, "") {
		return true, nil
	}
	//fmt.Println(result)
	status := hexutil.MustDecodeUint64(gjson.GetBytes(ret, "result.status").String())
	if status == 0 {
		return false, nil
	}
	return true, nil
}

func AccountUnlock(url string, SynStateAccount string, SynStateAccountPasswd string) error {
	ret, err := HttpPost(url, "application/json", `{"jsonrpc": "2.0","method": "personal_unlockAccount","params": ["`+SynStateAccount+`","`+SynStateAccountPasswd+`",null],"id": 1}`)
	if err != nil {
		return err
	}
	err = checkError(ret)
	if err != nil {
		return err
	}
	return nil
}
