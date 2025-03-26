package fins

import (
	"encoding/binary"
	"folke99/gofins/mapping"
)

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
