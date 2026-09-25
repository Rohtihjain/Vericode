package main

import "encoding/binary"

type Opcode byte

const (
	OpConstant      Opcode = iota
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpMod
	OpPop
	OpTrue
	OpFalse
	OpNull
	OpEqual
	OpNotEqual
	OpGreaterThan
	OpLessThan
	OpGreaterThanOrEqual
	OpLessThanOrEqual
	OpJump
	OpJumpWithFalse
	OpSetGlobal
	OpGetGlobal
	OpSetLocal
	OpGetLocal
	OpPrint
	OpCall
	OpReturnValue
	OpReturn
	OpArray
	OpIndex
	OpGetBuiltin
	OpBang
	OpJumpWithTrue
	OpHash
	OpAsk
	OpNegate
	OpClosure
	OpGetFree
	OpSetFree
	OpSetIndex
	OpSlice
)

type Definition struct {
	Name          string
	OperandWidths []int
}

var definitions = map[Opcode]*Definition{
	OpConstant:           {"OpConstant", []int{2}},
	OpAdd:                {"OpAdd", nil},
	OpSub:                {"OpSub", nil},
	OpMul:                {"OpMul", nil},
	OpDiv:                {"OpDiv", nil},
	OpMod:                {"OpMod", nil},
	OpPop:                {"OpPop", nil},
	OpTrue:               {"OpTrue", nil},
	OpFalse:              {"OpFalse", nil},
	OpNull:               {"OpNull", nil},
	OpEqual:              {"OpEqual", nil},
	OpNotEqual:           {"OpNotEqual", nil},
	OpGreaterThan:        {"OpGreaterThan", nil},
	OpLessThan:           {"OpLessThan", nil},
	OpGreaterThanOrEqual: {"OpGreaterThanOrEqual", nil},
	OpLessThanOrEqual:    {"OpLessThanOrEqual", nil},
	OpJump:               {"OpJump", []int{2}},
	OpJumpWithFalse:      {"OpJumpWithFalse", []int{2}},
	OpSetGlobal:          {"OpSetGlobal", []int{2}},
	OpGetGlobal:          {"OpGetGlobal", []int{2}},
	OpSetLocal:           {"OpSetLocal", []int{2}},
	OpGetLocal:           {"OpGetLocal", []int{2}},
	OpPrint:              {"OpPrint", nil},
	OpCall:               {"OpCall", []int{2}},
	OpReturnValue:        {"OpReturnValue", nil},
	OpReturn:             {"OpReturn", nil},
	OpArray:              {"OpArray", []int{2}},
	OpIndex:              {"OpIndex", nil},
	OpGetBuiltin:         {"OpGetBuiltin", []int{2}},
	OpBang:               {"OpBang", nil},
	OpJumpWithTrue:       {"OpJumpWithTrue", []int{2}},
	OpHash:               {"OpHash", []int{2}},
	OpAsk:                {"OpAsk", nil},
	OpNegate:             {"OpNegate", nil},
	OpClosure:            {"OpClosure", []int{2, 1}}, // const index, num free
	OpGetFree:            {"OpGetFree", []int{2}},
	OpSetFree:            {"OpSetFree", []int{2}},
	OpSetIndex:           {"OpSetIndex", nil},
	OpSlice:              {"OpSlice", nil},
}

func Make(op Opcode, operands ...int) []byte {
	def, ok := definitions[op]
	if !ok {
		return []byte{}
	}

	size := 1
	for _, w := range def.OperandWidths {
		size += w
	}

	ins := make([]byte, size)
	ins[0] = byte(op)

	offset := 1
	for i, o := range operands {
		w := def.OperandWidths[i]
		switch w {
		case 2:
			binary.BigEndian.PutUint16(ins[offset:], uint16(o))
		case 1:
			ins[offset] = byte(o)
		}
		offset += w
	}
	return ins
}
