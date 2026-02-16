package wasmbe

import (
	"encoding/binary"
	"math"
)

// WASM binary format constants
var wasmMagic = []byte{0x00, 0x61, 0x73, 0x6D} // \0asm
var wasmVersion = []byte{0x01, 0x00, 0x00, 0x00}

// Section IDs
const (
	sectionType     byte = 1
	sectionImport   byte = 2
	sectionFunction byte = 3
	sectionMemory   byte = 5
	sectionGlobal   byte = 6
	sectionExport   byte = 7
	sectionCode     byte = 10
	sectionData     byte = 11
)

// Value types
const (
	valI32 byte = 0x7F
	valI64 byte = 0x7E
	valF32 byte = 0x7D
	valF64 byte = 0x7C
)

// Export kinds
const (
	exportFunc   byte = 0x00
	exportMemory byte = 0x02
)

// WASM opcodes
const (
	// Control
	opUnreachable byte = 0x00
	opNop         byte = 0x01
	opBlock       byte = 0x02
	opLoop        byte = 0x03
	opIf          byte = 0x04
	opElse        byte = 0x05
	opEnd         byte = 0x0B
	opBr          byte = 0x0C
	opBrIf        byte = 0x0D
	opReturn      byte = 0x0F
	opCall        byte = 0x10
	opDrop        byte = 0x1A

	// Variables
	opLocalGet  byte = 0x20
	opLocalSet  byte = 0x21
	opLocalTee  byte = 0x22
	opGlobalGet byte = 0x23
	opGlobalSet byte = 0x24

	// Memory
	opI32Load    byte = 0x28
	opI64Load    byte = 0x29
	opF64Load    byte = 0x2B
	opI32Store   byte = 0x36
	opI64Store   byte = 0x37
	opF64Store   byte = 0x39
	opMemorySize byte = 0x3F
	opMemoryGrow byte = 0x40

	// Constants
	opI32Const byte = 0x41
	opI64Const byte = 0x42
	opF64Const byte = 0x44

	// i32 operations
	opI32Eqz  byte = 0x45
	opI32Eq   byte = 0x46
	opI32Ne   byte = 0x47
	opI32LtS  byte = 0x48
	opI32GtS  byte = 0x4A
	opI32LeS  byte = 0x4C
	opI32GeS  byte = 0x4E
	opI32Add  byte = 0x6A
	opI32Sub  byte = 0x6B
	opI32Mul  byte = 0x6C
	opI32DivS byte = 0x6D
	opI32RemS byte = 0x6F
	opI32And  byte = 0x71
	opI32Or   byte = 0x72

	// i64 operations
	opI64Eqz  byte = 0x50
	opI64Eq   byte = 0x51
	opI64Ne   byte = 0x52
	opI64LtS  byte = 0x53
	opI64GtS  byte = 0x55
	opI64LeS  byte = 0x57
	opI64GeS  byte = 0x59
	opI64Add  byte = 0x7C
	opI64Sub  byte = 0x7D
	opI64Mul  byte = 0x7E
	opI64DivS byte = 0x7F
	opI64RemS byte = 0x81
	opI64And  byte = 0x83
	opI64Or   byte = 0x84

	// f64 operations
	opF64Eq  byte = 0x61
	opF64Ne  byte = 0x62
	opF64Lt  byte = 0x63
	opF64Gt  byte = 0x64
	opF64Le  byte = 0x65
	opF64Ge  byte = 0x66
	opF64Add byte = 0xA0
	opF64Sub byte = 0xA1
	opF64Mul byte = 0xA2
	opF64Div byte = 0xA3

	// Conversions
	opI32WrapI64    byte = 0xA7
	opI64ExtendI32S byte = 0xAC
	opF64ConvertI64 byte = 0xB9

	// Block types
	blockVoid byte = 0x40
	blockI32  byte = 0x7F
	blockI64  byte = 0x7E
	blockF64  byte = 0x7C
)

// encodeLEB128U encodes an unsigned integer as unsigned LEB128.
func encodeLEB128U(value uint64) []byte {
	if value == 0 {
		return []byte{0}
	}
	var result []byte
	for value > 0 {
		b := byte(value & 0x7F)
		value >>= 7
		if value > 0 {
			b |= 0x80
		}
		result = append(result, b)
	}
	return result
}

// encodeLEB128S encodes a signed integer as signed LEB128.
func encodeLEB128S(value int64) []byte {
	var result []byte
	more := true
	for more {
		b := byte(value & 0x7F)
		value >>= 7
		if (value == 0 && b&0x40 == 0) || (value == -1 && b&0x40 != 0) {
			more = false
		} else {
			b |= 0x80
		}
		result = append(result, b)
	}
	return result
}

// encodeF64 encodes a float64 as 8 bytes little-endian.
func encodeF64(value float64) []byte {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], math.Float64bits(value))
	return buf[:]
}

// encodeString encodes a string with its length prefix.
func encodeString(s string) []byte {
	result := encodeLEB128U(uint64(len(s)))
	result = append(result, []byte(s)...)
	return result
}

// encodeSection encodes a section with its ID and length prefix.
func encodeSection(id byte, contents []byte) []byte {
	result := []byte{id}
	result = append(result, encodeLEB128U(uint64(len(contents)))...)
	result = append(result, contents...)
	return result
}

// encodeVector encodes a vector of items with a count prefix.
func encodeVector(count int, items []byte) []byte {
	result := encodeLEB128U(uint64(count))
	result = append(result, items...)
	return result
}
