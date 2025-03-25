package fins

import (
	"encoding/binary"
	"fmt"
	"folke99/gofins/mapping"
	"log"
)

// NOTE: Only used in server.go
// request represents a FINS command request
type Request struct {
	header      Header
	commandCode uint16
	data        []byte
}

// response represents a FINS command response
type Response struct {
	header      Header
	commandCode uint16
	endCode     uint16
	data        []byte
}

// memoryAddress represents a PLC memory address
type MemoryAddress struct {
	memoryArea byte
	address    uint16
	bitOffset  byte
}

// NewResponse creates a new FINS response
func NewResponse(req Request, endCode uint16, data []byte) Response {
	return Response{
		header:      req.header, // Copy the request header
		commandCode: req.commandCode,
		endCode:     endCode,
		data:        data,
	}
}

// Getters
func (r Request) GetHeader() Header {
	return r.header
}

func (r Request) GetCommandCode() uint16 {
	return r.commandCode
}

func (r Request) GetData() []byte {
	return r.data
}
func (m MemoryAddress) GetMemoryArea() byte {
	return m.memoryArea
}
func (m MemoryAddress) GetAddress() uint16 {
	return m.address
}
func (m MemoryAddress) GetBitOffset() byte {
	return m.bitOffset
}

// Create memory address helpers
func memAddr(memoryArea byte, address uint16) MemoryAddress {
	return memAddrWithBitOffset(memoryArea, address, 0)
}

func memAddrWithBitOffset(memoryArea byte, address uint16, bitOffset byte) MemoryAddress {
	return MemoryAddress{memoryArea, address, bitOffset}
}

// Command creation functions
func readCommand(memoryAddr MemoryAddress, itemCount uint16) []byte {
	commandData := make([]byte, 2, 8)
	binary.BigEndian.PutUint16(commandData[0:2], mapping.CommandCodeMemoryAreaRead)
	commandData = append(commandData, encodeMemoryAddress(memoryAddr)...)
	commandData = append(commandData, []byte{0, 0}...)
	binary.BigEndian.PutUint16(commandData[6:8], itemCount)
	return commandData
}

func writeCommand(memoryAddr MemoryAddress, itemCount uint16, bytes []byte) []byte {
	commandData := make([]byte, 2, 8+len(bytes))
	binary.BigEndian.PutUint16(commandData[0:2], mapping.CommandCodeMemoryAreaWrite)
	commandData = append(commandData, encodeMemoryAddress(memoryAddr)...)
	commandData = append(commandData, []byte{0, 0}...)
	binary.BigEndian.PutUint16(commandData[6:8], itemCount)
	commandData = append(commandData, bytes...)
	return commandData
}

func clockReadCommand() []byte {
	commandData := make([]byte, 2)
	binary.BigEndian.PutUint16(commandData[0:2], mapping.CommandCodeClockRead)
	return commandData
}

// Memory address encoding/decoding
func encodeMemoryAddress(memoryAddr MemoryAddress) []byte {
	bytes := make([]byte, 4)
	bytes[0] = memoryAddr.memoryArea
	binary.BigEndian.PutUint16(bytes[1:3], memoryAddr.address)
	bytes[3] = memoryAddr.bitOffset
	return bytes
}

// NOTE: Only used in server.go
func DecodeMemoryAddress(data []byte) (MemoryAddress, error) {
	if len(data) < 4 {
		return MemoryAddress{}, fmt.Errorf("insufficient data for memory address: expected 4 bytes, got %d", len(data))
	}
	return MemoryAddress{
		memoryArea: data[0],
		address:    binary.BigEndian.Uint16(data[1:3]),
		bitOffset:  data[3],
	}, nil
}

// NOTE: Only used in server.go
// Request/Response encoding/decoding
func DecodeRequest(bytes []byte) (Request, error) {
	if len(bytes) < 12 {
		return Request{}, fmt.Errorf("insufficient bytes for request decoding: expected at least 12 bytes, got %d", len(bytes))
	}

	header, err := decodeHeader(bytes[0:10])
	if err != nil {
		return Request{}, fmt.Errorf("failed to decode header: %w", err)
	}

	return Request{
		header:      header,
		commandCode: binary.BigEndian.Uint16(bytes[10:12]),
		data:        bytes[12:],
	}, nil
}

func DecodeResponse(bytes []byte) (Response, error) {
	if len(bytes) < 14 {
		return Response{}, fmt.Errorf("insufficient bytes for response: %d", len(bytes))
	}

	// Debug logging
	log.Printf("Decoding response bytes: % X", bytes)

	header := Header{
		icf: bytes[0],
		rsv: bytes[1],
		gct: bytes[2],
		dna: bytes[3],
		da1: bytes[4],
		da2: bytes[5],
		sna: bytes[6],
		sa1: bytes[7],
		sa2: bytes[8],
		sid: bytes[9],
	}

	resp := Response{
		header:      header,
		commandCode: binary.BigEndian.Uint16(bytes[10:12]),
		endCode:     binary.BigEndian.Uint16(bytes[12:14]),
		data:        bytes[14:],
	}

	log.Printf("Decoded header: ICF=%02X, GCT=%02X, DNA=%02X, DA1=%02X, DA2=%02X, SNA=%02X, SA1=%02X, SA2=%02X, SID=%02X",
		header.icf, header.gct, header.dna, header.da1, header.da2, header.sna, header.sa1, header.sa2, header.sid)

	return resp, nil
}

// NOTE: Only used in server.go
func EncodeResponse(resp Response) []byte {
	bytes := make([]byte, 4, 4+len(resp.data))
	binary.BigEndian.PutUint16(bytes[0:2], resp.commandCode)
	binary.BigEndian.PutUint16(bytes[2:4], resp.endCode)
	bytes = append(bytes, resp.data...)

	headerBytes := encodeHeader(resp.header)
	return append(headerBytes, bytes...)
}

// BCD encoding/decoding
type BCDError struct {
	msg string
}

func (e BCDError) Error() string {
	return fmt.Sprintf("BCD error: %s", e.msg)
}

// Date Decoding
func decodeBCD(bcd []byte) (uint64, error) {
	var result uint64

	for i, b := range bcd {
		hi, lo := uint64(b>>4), uint64(b&0x0f)

		// Validate high digit
		if hi > 9 {
			return 0, BCDError{fmt.Sprintf("invalid BCD digit (hi): %d", hi)}
		}

		// Add high digit
		result = result*10 + hi

		// Handle last nibble specially
		if lo == 0x0f && i == len(bcd)-1 {
			return result, nil
		}

		// Validate low digit
		if lo > 9 {
			return 0, BCDError{fmt.Sprintf("invalid BCD digit (lo): %d", lo)}
		}

		// Add low digit
		result = result*10 + lo
	}

	return result, nil
}
