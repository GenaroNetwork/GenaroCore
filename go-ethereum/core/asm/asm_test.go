// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package asm

import (
	"testing"
	"encoding/hex"
)

// Tests disassembling the instructions for valid evm code
func TestInstructionIteratorValid(t *testing.T) {
	cnt := 0
	script, _ := hex.DecodeString("61000000")

	it := NewInstructionIterator(script)
	for it.Next() {
		cnt++
	}

	if err := it.Error(); err != nil {
		t.Errorf("Expected 2, but encountered error %v instead.", err)
	}
	if cnt != 2 {
		t.Errorf("Expected 2, but got %v instead.", cnt)
	}
}

// Tests disassembling the instructions for invalid evm code
func TestInstructionIteratorInvalid(t *testing.T) {
	cnt := 0
	script, _ := hex.DecodeString("6100")

	it := NewInstructionIterator(script)
	for it.Next() {
		cnt++
	}

	if it.Error() == nil {
		t.Errorf("Expected an error, but got %v instead.", cnt)
	}
}

// Tests disassembling the instructions for empty evm code
func TestInstructionIteratorEmpty(t *testing.T) {
	cnt := 0
	script, _ := hex.DecodeString("")

	it := NewInstructionIterator(script)
	for it.Next() {
		cnt++
	}

	if err := it.Error(); err != nil {
		t.Errorf("Expected 0, but encountered error %v instead.", err)
	}
	if cnt != 0 {
		t.Errorf("Expected 0, but got %v instead.", cnt)
	}
}
//
//func TestDisassemble(t *testing.T) {
//	var bytecode []byte
//	var err error
//	//data := []byte("600160026003600421")
//	data := []byte("6001600252")
//	data = bytes.TrimSpace(data)
//
//	// disassemble
//	bytecode = make([]byte, hex.DecodedLen(len(data)))
//	_, err = hex.Decode(bytecode, data)
//	if err != nil {
//		panic(fmt.Sprintf("Could not decode hex string: %v", err))
//	}
//
//
//	if disassembly, err := Disassemble(bytecode); err != nil {
//		panic(fmt.Sprintf("Unable to disassemble: %v", err))
//	} else {
//		fmt.Println(disassembly)
//	}
//}

func TestPrintDisassembled(t *testing.T) {
	PrintDisassembled("600160025c5050")
}