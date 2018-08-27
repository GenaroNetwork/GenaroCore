package main

import (
	"testing"
	"github.com/GenaroNetwork/Genaro-Core/common/math"
	"github.com/GenaroNetwork/Genaro-Core/core/types"
)

func PromissoryNotesPrint(t *testing.T,notes types.PromissoryNotes){
	for _,note := range notes {
		t.Log("RestoreBlock:",note.RestoreBlock)
		t.Log("Num:",note.Num)
	}
}

func TestGenPromissoryNotes(t *testing.T){
	balance,_ := math.ParseBig256("400000000000000000000000")
	notes := GenPromissoryNotes(balance,80,2000,100000,10000)
	PromissoryNotesPrint(t,notes)
	t.Log(balance.String())
}
