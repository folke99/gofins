package fins

import (
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
