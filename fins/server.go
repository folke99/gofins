package fins

import (
	"encoding/binary"
	"log"
	"net"
)

// Server Omron FINS server (PLC emulator) over TCP
type Server struct {
	addr      Address
	listener  net.Listener
	dmarea    []byte
	bitdmarea []byte
	closed    bool
}

const DM_AREA_SIZE = 32768

func NewPLCSimulator(plcAddr Address) (*Server, error) {
	s := new(Server)
	s.addr = plcAddr
	s.dmarea = make([]byte, DM_AREA_SIZE)
	s.bitdmarea = make([]byte, DM_AREA_SIZE)

	// Start TCP Listener
	listener, err := net.Listen("tcp", plcAddr.tcpAddress.String())
	if err != nil {
		return nil, err
	}
	s.listener = listener

	go s.acceptConnections() // Handle incoming connections

	return s, nil
}

// Accepts new client connections and starts a handler for each one
func (s *Server) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.closed {
				return // Server is shutting down
			}
			log.Println("Error accepting connection:", err)
			continue
		}
		go s.handleClient(conn) // Handle each client in a separate goroutine
	}
}

// Handles communication with a single client
func (s *Server) handleClient(conn net.Conn) {
	defer conn.Close()

	var buf [1024]byte
	for {
		n, err := conn.Read(buf[:])
		if err != nil {
			log.Println("Client disconnected:", err)
			break
		}

		if n > 0 {
			req := decodeRequest(buf[:n])
			resp := s.handler(req)

			_, err = conn.Write(encodeResponse(resp))
			if err != nil {
				log.Println("Error writing response:", err)
				break
			}
		}
	}
}

// Works with only DM area, 2-byte integers
func (s *Server) handler(r request) response {
	var endCode uint16
	data := []byte{}

	switch r.commandCode {
	case CommandCodeMemoryAreaRead, CommandCodeMemoryAreaWrite:
		memAddr := decodeMemoryAddress(r.data[:4])
		ic := binary.BigEndian.Uint16(r.data[4:6]) // Item count

		switch memAddr.memoryArea {
		case MemoryAreaDMWord:
			if memAddr.address+ic*2 > DM_AREA_SIZE {
				endCode = EndCodeAddressRangeExceeded
				break
			}
			if r.commandCode == CommandCodeMemoryAreaRead { // Read command
				data = s.dmarea[memAddr.address : memAddr.address+ic*2]
			} else { // Write command
				copy(s.dmarea[memAddr.address:memAddr.address+ic*2], r.data[6:6+ic*2])
			}
			endCode = EndCodeNormalCompletion

		case MemoryAreaDMBit:
			if memAddr.address+ic > DM_AREA_SIZE {
				endCode = EndCodeAddressRangeExceeded
				break
			}
			start := memAddr.address + uint16(memAddr.bitOffset)
			if r.commandCode == CommandCodeMemoryAreaRead { // Read command
				data = s.bitdmarea[start : start+ic]
			} else { // Write command
				copy(s.bitdmarea[start:start+ic], r.data[6:6+ic])
			}
			endCode = EndCodeNormalCompletion

		default:
			log.Printf("Memory area is not supported: 0x%04x\n", memAddr.memoryArea)
			endCode = EndCodeNotSupportedByModelVersion
		}

	default:
		log.Printf("Command code is not supported: 0x%04x\n", r.commandCode)
		endCode = EndCodeNotSupportedByModelVersion
	}
	return response{defaultResponseHeader(r.header), r.commandCode, endCode, data}
}

// Close shuts down the FINS TCP server
func (s *Server) Close() {
	s.closed = true
	s.listener.Close()
}
