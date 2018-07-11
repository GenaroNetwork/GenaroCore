package types

import (
	"testing"
	"github.com/GenaroNetwork/Genaro-Core/common"
)

func TestBindingTable(t *testing.T){
	var bindingTable  = BindingTable {
		MainAccounts:	make(map[common.Address][]common.Address),
		SubAccounts:	make(map[common.Address]common.Address),
	}
	bindingTable.UpdateBinding(common.HexToAddress("0x1000000000000000000000000000000000000000"),common.HexToAddress("0x1100000000000000000000000000000000000000"))
	bindingTable.UpdateBinding(common.HexToAddress("0x1000000000000000000000000000000000000000"),common.HexToAddress("0x1200000000000000000000000000000000000000"))
	bindingTable.UpdateBinding(common.HexToAddress("0x1000000000000000000000000000000000000000"),common.HexToAddress("0x1200000000000000000000000000000000000000"))
	bindingTable.UpdateBinding(common.HexToAddress("0x1000000000000000000000000000000000000000"),common.HexToAddress("0x1300000000000000000000000000000000000000"))
	t.Log(bindingTable)
	if bindingTable.GetSubAccountSizeInMainAccount(common.HexToAddress("0x1000000000000000000000000000000000000000")) != 3 {
		t.Error("UpdateBinding error")
	}
	bindingTable.DelSubAccount(common.HexToAddress("0x1300000000000000000000000000000000000000"))
	t.Log(bindingTable)
	if bindingTable.GetSubAccountSizeInMainAccount(common.HexToAddress("0x1000000000000000000000000000000000000000")) != 2 {
		t.Error("UpdateBinding error")
	}
	bindingTable.UpdateBinding(common.HexToAddress("0x1000000000000000000000000000000000000000"),common.HexToAddress("0x1300000000000000000000000000000000000000"))
	if !bindingTable.IsAccountInBinding(common.HexToAddress("0x1000000000000000000000000000000000000000")){
		t.Error("Binding error")
	}
	if bindingTable.IsAccountInBinding(common.HexToAddress("0x1000000000000000000000000000000000111111")){
		t.Error("Binding error")
	}
	if !bindingTable.IsAccountInBinding(common.HexToAddress("0x1300000000000000000000000000000000000000")){
		t.Error("Binding error")
	}
	bindingTable.UpdateBinding(common.HexToAddress("0x2000000000000000000000000000000000000000"),common.HexToAddress("0x1300000000000000000000000000000000000000"))
	if bindingTable.GetSubAccountSizeInMainAccount(common.HexToAddress("0x1000000000000000000000000000000000000000")) != 2 {
		t.Error("UpdateBinding error")
	}
	if bindingTable.GetSubAccountSizeInMainAccount(common.HexToAddress("0x2000000000000000000000000000000000000000")) != 1 {
		t.Error("UpdateBinding error")
	}
	t.Log(bindingTable)
	bindingTable.DelSubAccount(common.HexToAddress("0x1300000000000000000000000000000000000000"))
	if bindingTable.IsMainAccountExist(common.HexToAddress("0x2000000000000000000000000000000000000000")){
		t.Error("DelSubAccount error")
	}
	t.Log(bindingTable)
	subAccounts := bindingTable.DelMainAccount(common.HexToAddress("0x1000000000000000000000000000000000000000"))
	if bindingTable.IsMainAccountExist(common.HexToAddress("0x1000000000000000000000000000000000000000")){
		t.Error("DelSubAccount error")
	}
	t.Log(bindingTable)
	if len(subAccounts) != 2 {
		t.Error("DelMainAccount error")
	}
	t.Log(subAccounts)
}
