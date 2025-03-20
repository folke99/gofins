package fins

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"folke99/gofins/mapping"
	"io"
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

func (s *Server) handleClient(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	for {
		// Read 4-byte length prefix
		lengthBytes := make([]byte, 4)
		_, err := io.ReadFull(reader, lengthBytes)
		if err != nil {
			if err != io.EOF {
				log.Printf("Length read error: %v", err)
			}
			break
		}

		// Decode message length
		messageLength := binary.BigEndian.Uint32(lengthBytes)
		log.Printf("Expecting message of length: %d", messageLength)

		// Sanity check on message length
		if messageLength > MAX_PACKET_SIZE {
			log.Printf("Message too large: %d", messageLength)
			break
		}

		// Read full message
		messageBytes := make([]byte, messageLength)
		_, err = io.ReadFull(reader, messageBytes)
		if err != nil {
			log.Printf("Message read error: %v", err)
			break
		}

		// Detailed logging of received bytes
		log.Printf("Received TCP message: % x", messageBytes)

		// Process the message
		req, err := decodeRequest(messageBytes)
		if err != nil {
			fmt.Printf("error: %f", err)
		}
		resp := s.handler(req)

		// Prepare response with length prefix
		respData := encodeResponse(resp)
		respLength := make([]byte, 4)
		binary.BigEndian.PutUint32(respLength, uint32(len(respData)))

		fullResp := append(respLength, respData...)

		// Write full response
		_, err = conn.Write(fullResp)
		if err != nil {
			log.Printf("Response write error: %v", err)
			break
		}
	}
}

func (s *Server) handler(r request) response {
	var endCode uint16 = EndCodeNormalCompletion
	data := []byte{}

	// Extensive logging
	log.Printf("Handler received: CommandCode=0x%04x, DataLength=%d",
		r.commandCode, len(r.data))

	// Defensive checks
	if len(r.data) < 6 {
		log.Printf("Insufficient data for request: %d bytes", len(r.data))
		return response{
			header:      defaultResponseHeader(r.header),
			commandCode: r.commandCode,
			endCode:     EndCodeNotSupportedByModelVersion,
			data:        nil,
		}
	}

	switch r.commandCode {
	case mapping.CommandCodeMemoryAreaRead, mapping.CommandCodeMemoryAreaWrite:
		// Ensure enough data for memory address and item count
		if len(r.data) < 6 {
			log.Printf("Insufficient data for memory area operation: %d bytes", len(r.data))
			return response{
				header:      defaultResponseHeader(r.header),
				commandCode: r.commandCode,
				endCode:     EndCodeNotSupportedByModelVersion,
				data:        nil,
			}
		}

		memAddr, err := decodeMemoryAddress(r.data[:4])
		if err != nil {
			fmt.Printf("error: %f", err)
		}
		ic := binary.BigEndian.Uint16(r.data[4:6]) // Item count

		log.Printf("Memory Operation: Area=0x%02x, Address=%d, ItemCount=%d",
			memAddr.memoryArea, memAddr.address, ic)

		switch memAddr.memoryArea {
		case mapping.MemoryAreaDMWord:
			if memAddr.address+ic*2 > DM_AREA_SIZE {
				log.Printf("Address range exceeded for DMWord")
				endCode = EndCodeAddressRangeExceeded
				break
			}

			if r.commandCode == mapping.CommandCodeMemoryAreaRead {
				data = s.dmarea[memAddr.address : memAddr.address+ic*2]
			} else { // Write command
				if len(r.data) < 6+int(ic*2) {
					log.Printf("Insufficient data for DMWord write")
					endCode = EndCodeNotSupportedByModelVersion
					break
				}
				copy(s.dmarea[memAddr.address:memAddr.address+ic*2], r.data[6:6+ic*2])
			}

		case mapping.MemoryAreaDMBit:
			if memAddr.address+ic > DM_AREA_SIZE {
				log.Printf("Address range exceeded for DMBit")
				endCode = EndCodeAddressRangeExceeded
				break
			}

			start := memAddr.address + uint16(memAddr.bitOffset)
			if r.commandCode == mapping.CommandCodeMemoryAreaRead {
				data = s.bitdmarea[start : start+ic]
			} else { // Write command
				if len(r.data) < 6+int(ic) {
					log.Printf("Insufficient data for DMBit write")
					endCode = EndCodeNotSupportedByModelVersion
					break
				}
				copy(s.bitdmarea[start:start+ic], r.data[6:6+ic])
			}

		default:
			log.Printf("Unsupported memory area: 0x%02x", memAddr.memoryArea)
			endCode = EndCodeNotSupportedByModelVersion
		}

	default:
		log.Printf("Unsupported command code: 0x%04x", r.commandCode)
		endCode = EndCodeNotSupportedByModelVersion
	}

	return response{
		header:      defaultResponseHeader(r.header),
		commandCode: r.commandCode,
		endCode:     endCode,
		data:        data,
	}
}

// Close shuts down the FINS TCP server
func (s *Server) Close() {
	s.closed = true
	s.listener.Close()
}
