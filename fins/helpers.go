package fins

import (
	"encoding/binary"
	"fmt"
	"folke99/gofins/mapping"
	"net"
	"time"
)

// Set byte order
// Default value: binary.BigEndian
func (c *Client) SetByteOrder(o binary.ByteOrder) {
	c.byteOrder = o
}

// Set response timeout duration (ms).
// Default value: 20ms.
// A timeout of zero can be used to block indefinitely.
func (c *Client) SetTimeoutMs(t uint) {
	c.responseTimeoutMs = time.Duration(t)
}

// SetBit Sets a bit in the PLC data area
func (c *Client) SetBit(memoryArea byte, address uint16, bitOffset byte) error {
	return c.bitTwiddle(memoryArea, address, bitOffset, 0x01)
}

// ResetBit Resets a bit in the PLC data area
func (c *Client) ResetBit(memoryArea byte, address uint16, bitOffset byte) error {
	return c.bitTwiddle(memoryArea, address, bitOffset, 0x00)
}

// ToggleBit Toggles a bit in the PLC data area
func (c *Client) ToggleBit(memoryArea byte, address uint16, bitOffset byte) error {
	b, e := c.ReadBits(memoryArea, address, bitOffset, 1)
	if e != nil {
		return e
	}
	var t byte
	if b[0] {
		t = 0x00
	} else {
		t = 0x01
	}
	return c.bitTwiddle(memoryArea, address, bitOffset, t)
}

func (c *Client) bitTwiddle(memoryArea byte, address uint16, bitOffset byte, value byte) error {
	if checkIsBitMemoryArea(memoryArea) == false {
		return IncompatibleMemoryAreaError{memoryArea}
	}
	mem := MemoryAddress{memoryArea, address, bitOffset}
	command := writeCommand(mem, 1, []byte{value})

	return checkResponse(c.sendCommand(command))
}

func (c *Client) incrementSid() byte {
	c.Lock() // Thread-safe SID incrementation
	c.sid++
	if c.sid == 0 {
		c.sid = 1
	}
	sid := c.sid
	c.Unlock()

	// Clearing cell of storage for new response
	c.resp[sid] = make(chan Response)
	return sid
}

// SetKeepAlive enables TCP keepalive with the specified interval
func (c *Client) SetKeepAlive(enabled bool, interval time.Duration) error {
	tcpConn, ok := c.conn.(*net.TCPConn)
	if !ok {
		return fmt.Errorf("connection is not TCP")
	}

	if err := tcpConn.SetKeepAlive(enabled); err != nil {
		return err
	}

	if enabled {
		return tcpConn.SetKeepAlivePeriod(interval)
	}
	return nil
}

func checkIsWordMemoryArea(memoryArea byte) bool {
	if memoryArea == mapping.MemoryAreaDMWord ||
		memoryArea == mapping.MemoryAreaARWord ||
		memoryArea == mapping.MemoryAreaHRWord ||
		memoryArea == mapping.MemoryAreaWRWord {
		return true
	}
	return false
}

func checkIsBitMemoryArea(memoryArea byte) bool {
	if memoryArea == mapping.MemoryAreaDMBit ||
		memoryArea == mapping.MemoryAreaARBit ||
		memoryArea == mapping.MemoryAreaHRBit ||
		memoryArea == mapping.MemoryAreaWRBit {
		return true
	}
	return false
}
