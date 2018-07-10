package vm

import (
	"fmt"
	"time"
	"bytes"
	"errors"
	"math/big"
	"crypto/sha256"

	"github.com/GenaroNetwork/Genaro-Core/core/types"
	"github.com/GenaroNetwork/Genaro-Core/common"
	"github.com/GenaroNetwork/Genaro-Core/crypto"
	"golang.org/x/crypto/ripemd160"

)


func isSpecialAddress(address common.Address) bool {
	for _, v := range common.SpecialAddressList {
		if bytes.Compare(address.Bytes(), v.Bytes()) == 0{
			return  true
		}
	}
	return false
}

func CheckSpecialTxTypeSyncSidechainStatusParameter( s types.SpecialTxInput,caller common.Address) error {
	if true == isSpecialAddress(s.SpecialTxTypeMortgageInit.FromAccount) {
		return errors.New("fromAccount error")
	}

	if caller !=  common.OfficialAddress {
		return errors.New("caller address of this transaction is not invalid")
	}

	if 64 != len(s.SpecialTxTypeMortgageInit.Dataversion) {
		return errors.New("Parameter Dataversion  error")
	}

	if 64 != len(s.SpecialTxTypeMortgageInit.FileID) {
		return errors.New("Parameter fileID  error")
	}
	if 20 != len(s.SpecialTxTypeMortgageInit.FromAccount) {
		return errors.New("Parameter fromAccount  error")
	}
	if 1 < len(s.SpecialTxTypeMortgageInit.Sidechain) {
		for k,v := range s.SpecialTxTypeMortgageInit.Sidechain{
			if 20 != len(k) {
				return errors.New("Parameter mortgage account  error")
			}
			if v.ToInt().Cmp(big.NewInt(0)) < 0 {
				return errors.New("Parameter Sidechain")
			}
		}
	} else {
		return errors.New("Parameter side chain length less than zero")
	}
	return nil
}


func CheckspecialTxTypeMortgageInitParameter( s types.SpecialTxInput,caller common.Address) error {
	var tmp  big.Int
	timeLimit := s.SpecialTxTypeMortgageInit.TimeLimit.ToInt()
	tmp.Mul(timeLimit,big.NewInt(86400))
	endTime :=  tmp.Add(&tmp,big.NewInt(s.SpecialTxTypeMortgageInit.CreateTime)).Int64()
	if s.SpecialTxTypeMortgageInit.CreateTime > s.SpecialTxTypeMortgageInit.EndTime ||
		s.SpecialTxTypeMortgageInit.CreateTime > time.Now().Unix() ||
		s.SpecialTxTypeMortgageInit.EndTime != endTime {
		return errors.New("Parameter CreateTime or EndTime  error")
	}
	if caller != s.SpecialTxTypeMortgageInit.FromAccount {
		return errors.New("Parameter FromAccount  error")
	}
	if len(s.SpecialTxTypeMortgageInit.FileID) != 64 {
		return errors.New("Parameter FileID  error")
	}
	mortgageTable := s.SpecialTxTypeMortgageInit.MortgageTable
	authorityTable := s.SpecialTxTypeMortgageInit.AuthorityTable
	if len(authorityTable) != len(mortgageTable) {
		return errors.New("Parameter authorityTable != mortgageTable  error")
	}
	for k,v := range authorityTable {
		if v < 0 || v > 3 {
			return errors.New("Parameter authority type  error")
		}
		if mortgageTable[k].ToInt().Cmp(big.NewInt(0)) < 0 {
			return errors.New("Parameter mortgage amount is less than zero")
		}
	}
	return nil
}

func CheckSynchronizeShareKeyParameter( s types.SpecialTxInput) error {

	if true == isSpecialAddress(s.SynchronizeShareKey.RecipientAddress) {
		return errors.New("update  chain SynchronizeShareKey fail")
	}

	if len(s.SynchronizeShareKey.ShareKeyId) != 64 {
		return errors.New("Parameter ShareKeyId  error")
	}
	if len(s.SynchronizeShareKey.ShareKey) > 0 {
		return errors.New("Parameter ShareKey  error")
	}
	if s.SynchronizeShareKey.Shareprice.ToInt().Cmp(big.NewInt(0)) < 0 {
		return errors.New("Parameter Shareprice  is less than zero")
	}
	return nil
}

func CheckUnlockSharedKeyParameter( s types.SpecialTxInput) error {
	if len(s.SynchronizeShareKey.ShareKeyId) != 64 {
		return errors.New("Parameter ShareKeyId  error")
	}
	return nil
}

func CheckStakeTx(s types.SpecialTxInput) error {
	adress := common.HexToAddress(s.Address)
	if isSpecialAddress(adress){
		return errors.New("param [address] can't be special address")
	}

	if s.Stake <= 0 {
		return errors.New("value of stake must larger than zero")
	}
	return nil
}

func CheckSyncHeftTx(caller common.Address, s types.SpecialTxInput) error {
	if caller !=  common.OfficialAddress {
		return errors.New("caller address of this transaction is not invalid")
	}

	adress := common.HexToAddress(s.Address)
	if isSpecialAddress(adress){
		return errors.New("param [address] can't be special address")
	}

	return nil
}

func CheckApplyBucketTx(s types.SpecialTxInput) error {
	for _, v := range s.Buckets {
		if len(v.BucketId) != 64 {
			return errors.New("length of bucketId must be 64")
		}
	}
	return nil
}

func CheckTrafficTx(s types.SpecialTxInput) error {
	adress := common.HexToAddress(s.Address)
	if isSpecialAddress(adress){
		return errors.New("param [address] can't be special address")
	}

	if s.Traffic <= 0 {
		errors.New("value of traffic must larger than zero")
	}
	return nil
}

func CheckSyncNodeTx(caller common.Address,stake uint64, existNodes []string, s types.SpecialTxInput, stakeVlauePerNode *big.Int) error {

	if len(s.NodeID) == 0 {
		return errors.New("length of nodeId must larger then 0")
	}

	//caller和节点待绑定账户是否一致
	if caller.String() != s.Address {
		return errors.New("two address not equal")
	}

	//校验节点是否已经绑定过
	for _, existNode := range existNodes {
		if s.NodeID == existNode {
			return errors.New("the node has been bound to the account")
		}
	}

	// 验证节点绑定签名
	// 拼接message
	msg := s.NodeID + s.Sign
	recoveredPub, err := crypto.Ecrecover([]byte(msg), []byte(s.Sign))
	if err != nil {
		errors.New("ECRecover error when valid sign")
	}

	//get publickey
	pubKey := crypto.CompressPubkey(crypto.ToECDSAPub(recoveredPub))

	genNodeID := generateNodeId(pubKey)
	if genNodeID != s.NodeID {
		return errors.New("sign valid error")
	}

	var nodeNum int = 1
	if existNodes != nil {
		nodeNum += len(existNodes)
	}

	needStakeVale := big.NewInt(0)
	needStakeVale.Mul(big.NewInt(int64(nodeNum)), stakeVlauePerNode)

	if needStakeVale.Cmp(big.NewInt(int64(stake*1000000000000000000))) != 1 {
		return errors.New("none enough stake to synchronize node")
	}
	return nil
}


func generateNodeId(b []byte) string {
	sha256byte := sha256.Sum256(b)
	ripemder := ripemd160.New()
	ripemder.Write(sha256byte[:])
	hashBytes := ripemder.Sum(nil)
	nodeId := fmt.Sprintf("%x", hashBytes)
	return nodeId
}

func CheckPunishmentTx(caller common.Address,s types.SpecialTxInput) error {
	adress := common.HexToAddress(s.Address)
	if isSpecialAddress(adress){
		return errors.New("param [address] can't be special address")
	}

	if caller !=  common.OfficialAddress {
		return errors.New("caller address of this transaction is not invalid")
	}
	return nil
}

func CheckSynStateTx(caller common.Address) error {
	if caller !=  common.OfficialAddress {
		return errors.New("caller address of this transaction is not invalid")
	}
	return nil
}

func CheckSyncFileSharePublicKeyTx(s types.SpecialTxInput) error {
	adress := common.HexToAddress(s.Address)
	if isSpecialAddress(adress){
		return errors.New("param [address] can't be special address")
	}

	if s.FileSharePublicKey == "" {
		return errors.New("public key for file share can't be null")
	}
	return nil
}

func CheckPriceRegulation(caller common.Address) error {
	if caller !=  common.GenaroPriceAddress {
		return errors.New("caller address of this transaction is not invalid")
	}
	return nil
}

func CheckUnbindNodeTx(caller common.Address,s types.SpecialTxInput, existNodes []string) error{
	if existNodes == nil {
		return errors.New("none node of this account need to unbind")
	}

	if s.NodeID == "" {
		return errors.New("nodeId is null")
	}

	for _, v := range existNodes{
		if v == s.NodeID {
			return nil
		}
	}
	return errors.New("this node does not belong to this account")
}