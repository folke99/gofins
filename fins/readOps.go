package fins

import (
	"bytes"
	"fmt"
	"folke99/gofins/mapping"
	"log"
	"time"
)

// ReadWords Reads words from the PLC data area
func (c *Client) ReadWords(memoryArea byte, address uint16, readCount uint16) ([]uint16, error) {
	if mapping.CheckIsWordMemoryArea(memoryArea) == false {
		return nil, IncompatibleMemoryAreaError{memoryArea}
	}
	command := readCommand(memAddr(memoryArea, address), readCount)
	r, e := c.sendCommand(command)
	e = checkResponse(r, e)

	//tracing
	log.Printf("Response from ReadWords(), %+v", r)

	if e != nil {
		return nil, e
	}

	data := make([]uint16, readCount, readCount)
	for i := 0; i < int(readCount); i++ {
		data[i] = c.byteOrder.Uint16(r.data[i*2 : i*2+2])
	}

	return data, nil
}

func (c *Client) ReadBytes(memoryArea byte, address uint16, byteCount uint16) ([]byte, error) {
	if !mapping.CheckIsWordMemoryArea(memoryArea) {
		return nil, IncompatibleMemoryAreaError{memoryArea}
	}

	// Ensure read count is word-aligned
	if byteCount%2 != 0 {
		return nil, fmt.Errorf("requested byte count must be a multiple of 2 for word-based memory area")
	}

	// Convert bytes to words (FINS protocol expects word count)
	wordCount := byteCount / 2

	command := readCommand(memAddr(memoryArea, address), wordCount)
	r, e := c.sendCommand(command)
	e = checkResponse(r, e)

	//tracing
	log.Printf("Response from ReadBytes(), %+v", r)

	if e != nil {
		return nil, e
	}

	return r.data, nil
}

// ReadString reads a string from the PLC's DM memory area NEW
func (c *Client) ReadString(memoryArea byte, address uint16, byteCount uint16) (string, error) {
	if !mapping.CheckIsWordMemoryArea(memoryArea) {
		return "", IncompatibleMemoryAreaError{memoryArea}
	}

	// Ensure the read byte count is word-aligned
	if byteCount%2 != 0 {
		byteCount++
	}

	// Read bytes from PLC
	data, err := c.ReadBytes(memoryArea, address, byteCount)
	if err != nil {
		return "", err
	}

	// Trim null bytes (if string was null-terminated)
	return string(bytes.TrimRight(data, "\x00")), nil
}

// ReadBits Reads bits from the PLC data area
func (c *Client) ReadBits(memoryArea byte, address uint16, bitOffset byte, readCount uint16) ([]bool, error) {
	if mapping.CheckIsBitMemoryArea(memoryArea) == false {
		return nil, IncompatibleMemoryAreaError{memoryArea}
	}
	command := readCommand(memAddrWithBitOffset(memoryArea, address, bitOffset), readCount)
	r, e := c.sendCommand(command)
	e = checkResponse(r, e)

	//tracing
	log.Printf("Response from ReadBits(), %+v", r)

	if e != nil {
		return nil, e
	}

	data := make([]bool, readCount, readCount)
	for i := 0; i < int(readCount); i++ {
		data[i] = r.data[i]&0x01 > 0
	}

	return data, nil
}

func (c *Client) ReadPLCStatus() (*Response, error) {
	log.Println("📡 Attempting to read PLC status...")

	// Command bytes for PLC Status Read (06 01)
	commandBytes := []byte{0x06, 0x01}

	// Send FINS command
	resp, err := c.sendCommand(commandBytes)
	if err != nil {
		return &Response{}, fmt.Errorf("failed to send PLC status command: %v", err)
	}

	// Decode the response to check the command code and structure
	err = checkResponse(resp, err)
	if err != nil {
		return &Response{}, fmt.Errorf("failed to parse FINS response: %v", err)
	}

	return resp, nil
}

// ReadClock Reads the PLC clock
func (c *Client) ReadClock() (*time.Time, error) {
	r, e := c.sendCommand(clockReadCommand())
	e = checkResponse(r, e)
	if e != nil {
		return nil, e
	}
	year, _ := decodeBCD(r.data[0:1])
	if year < 50 {
		year += 2000
	} else {
		year += 1900
	}
	month, _ := decodeBCD(r.data[1:2])
	day, _ := decodeBCD(r.data[2:3])
	hour, _ := decodeBCD(r.data[3:4])
	minute, _ := decodeBCD(r.data[4:5])
	second, _ := decodeBCD(r.data[5:6])

	t := time.Date(
		int(year), time.Month(month), int(day), int(hour), int(minute), int(second),
		0, // nanosecond
		time.Local,
	)
	return &t, nil
}
