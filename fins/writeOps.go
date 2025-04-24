package fins

import (
	"fmt"
	"folke99/gofins/mapping"
)

// WriteWords Writes words to the PLC data area
func (c *Client) WriteWords(memoryArea byte, address uint16, data []uint16) error {
	if mapping.CheckIsWordMemoryArea(memoryArea) == false {
		return IncompatibleMemoryAreaError{memoryArea}
	}
	l := uint16(len(data))
	bts := make([]byte, 2*l, 2*l)
	for i := 0; i < int(l); i++ {
		c.byteOrder.PutUint16(bts[i*2:i*2+2], data[i])
	}
	command := writeCommand(memAddr(memoryArea, address), l, bts)

	return checkResponse(c.sendCommand(command))
}

// WriteString writes a string to the PLC's DM memory area
func (c *Client) WriteString(memoryArea byte, address uint16, s string) error {
	if !mapping.CheckIsWordMemoryArea(memoryArea) {
		return IncompatibleMemoryAreaError{memoryArea}
	}

	b := []byte(s)

	// Ensure word alignment by padding with a null byte if needed
	if len(b)%2 != 0 {
		b = append(b, 0x00)
	}

	return c.WriteBytes(memoryArea, address, b)
}

func (c *Client) WriteBytes(memoryArea byte, address uint16, b []byte) error {
	if !mapping.CheckIsWordMemoryArea(memoryArea) {
		return IncompatibleMemoryAreaError{memoryArea}
	}

	// word-alignment
	if len(b)%2 != 0 {
		return fmt.Errorf("data length must be a multiple of 2 for word-based memory area")
	}

	// Convert bytes to words (FINS protocol expects word count)
	wordCount := uint16(len(b) / 2)

	command := writeCommand(memAddr(memoryArea, address), wordCount, b)
	return checkResponse(c.sendCommand(command))
}

// WriteBits Writes bits to the PLC data area
func (c *Client) WriteBits(memoryArea byte, address uint16, bitOffset byte, data []bool) error {
	if mapping.CheckIsBitMemoryArea(memoryArea) == false {
		return IncompatibleMemoryAreaError{memoryArea}
	}
	l := uint16(len(data))
	bts := make([]byte, 0, l)
	var d byte
	for i := 0; i < int(l); i++ {
		if data[i] {
			d = 0x01
		} else {
			d = 0x00
		}
		bts = append(bts, d)
	}
	command := writeCommand(memAddrWithBitOffset(memoryArea, address, bitOffset), l, bts)

	return checkResponse(c.sendCommand(command))
}
