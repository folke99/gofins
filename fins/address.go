package fins

import (
	"encoding/binary"
	"fmt"
	"net"
)

// finsAddress A FINS device address
type finsAddress struct {
	network byte
	node    byte
	unit    byte
}

// Address A full device address
type Address struct {
	finsAddress finsAddress
	tcpAddress  *net.TCPAddr // Changed from UDPAddr to TCPAddr
}

// memoryAddress represents a PLC memory address
type MemoryAddress struct {
	memoryArea byte
	address    uint16
	bitOffset  byte
}

// NewAddress creates a new Address instance with TCP addressing
func NewAddress(ip string, port int, network, node, unit byte) (Address, error) {
	// Parse IP address
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return Address{}, fmt.Errorf("invalid IP address: %s", ip)
	}

	// Create TCP address
	tcpAddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(ipAddr.String(), fmt.Sprint(port)))
	if err != nil {
		return Address{}, fmt.Errorf("failed to resolve TCP address: %w", err)
	}

	return Address{
		tcpAddress: tcpAddr,
		finsAddress: finsAddress{
			network: network,
			node:    node,
			unit:    unit,
		},
	}, nil
}

// String returns a string representation of the address
func (a Address) String() string {
	return fmt.Sprintf("FINS Address: Network: %d, Node: %d, Unit: %d, TCP: %s",
		a.finsAddress.network,
		a.finsAddress.node,
		a.finsAddress.unit,
		a.tcpAddress.String())
}

// Clone creates a deep copy of the Address
func (a Address) Clone() Address {
	newTCPAddr := *a.tcpAddress // Create a copy of the TCPAddr
	return Address{
		tcpAddress: &newTCPAddr,
		finsAddress: finsAddress{
			network: a.finsAddress.network,
			node:    a.finsAddress.node,
			unit:    a.finsAddress.unit,
		},
	}
}

// ---------- MEMORY ADDRESS FUNCTIONS ----------

// Getters
func (m MemoryAddress) GetMemoryArea() byte {
	return m.memoryArea
}
func (m MemoryAddress) GetAddress() uint16 {
	return m.address
}
func (m MemoryAddress) GetBitOffset() byte {
	return m.bitOffset
}

// Create MemoryAddress
func memAddr(memoryArea byte, address uint16) MemoryAddress {
	return MemoryAddress{memoryArea, address, 0}
}

// Create MemoryAddress with offset
func memAddrWithBitOffset(memoryArea byte, address uint16, bitOffset byte) MemoryAddress {
	return MemoryAddress{memoryArea, address, bitOffset}
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
