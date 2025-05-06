// Package simulator handle a simulator, a form of soft-plc for testing and validation
//
// This simulator misses alot of the logic needed to make it act like an actual omron plc.
// But it works as a basic soft-plc for testing simple commands.
package simulator

import (
	"bufio"
	"encoding/binary"
	"folke99/gofins/fins"
	"folke99/gofins/mapping"
	"io"
	"log"
	"net"
)

// PLC Simulator (FINS TCP Server)
type Server struct {
	address   string
	listener  net.Listener
	dmarea    []byte
	bitdmarea []byte
	closed    bool
}

const DM_AREA_SIZE = 32768
const MAX_PACKET_SIZE = 4096 // Define an appropriate max size

func NewPLCSimulator(address string) (*Server, error) {
	s := &Server{
		address:   address,
		dmarea:    make([]byte, DM_AREA_SIZE),
		bitdmarea: make([]byte, DM_AREA_SIZE),
	}

	// Start TCP Listener
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	s.listener = listener

	go s.acceptConnections()
	return s, nil
}

// Accepts client connections
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
		go s.handleClient(conn)
	}
}

func (s *Server) handleClient(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	for {
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
		if messageLength > MAX_PACKET_SIZE {
			log.Printf("Message too large: %d", messageLength)
			break
		}

		messageBytes := make([]byte, messageLength)
		_, err = io.ReadFull(reader, messageBytes)
		if err != nil {
			log.Printf("Message read error: %v", err)
			break
		}

		log.Printf("Received TCP message: % x", messageBytes)

		// Process the message
		req, err := fins.DecodeRequest(messageBytes)
		if err != nil {
			log.Printf("Request decoding error: %v", err)
			continue
		}

		resp := s.handler(req)

		// Prepare and send response
		respData := fins.EncodeResponse(resp)
		respLength := make([]byte, 4)
		binary.BigEndian.PutUint32(respLength, uint32(len(respData)))

		_, err = conn.Write(append(respLength, respData...))
		if err != nil {
			log.Printf("Response write error: %v", err)
			break
		}
	}
}

func (s *Server) handler(r fins.Request) fins.Response {
	var endCode uint16 = mapping.EndCodeNormalCompletion
	data := []byte{}

	log.Printf("Handler received: CommandCode=0x%04x, DataLength=%d",
		r.GetCommandCode(), len(r.GetData()))

	if len(r.GetData()) < 6 {
		log.Printf("Insufficient data for request: %d bytes", len(r.GetData()))
		return newErrorResponse(r, mapping.EndCodeNotSupportedByModelVersion)
	}

	m, err := fins.DecodeMemoryAddress(r.GetData()[:4])
	if err != nil {
		log.Printf("Memory address decoding error: %v", err)
		return newErrorResponse(r, mapping.EndCodeAddressRangeExceeded)
	}

	ic := binary.BigEndian.Uint16(r.GetData()[4:6]) // Item count

	log.Printf("Memory Operation: Area=0x%02x, Address=%d, ItemCount=%d",
		m.GetMemoryArea(), m.GetAddress(), ic)

	switch r.GetCommandCode() {
	case mapping.CommandCodeMemoryAreaRead, mapping.CommandCodeMemoryAreaWrite:
		switch m.GetMemoryArea() {
		case mapping.MemoryAreaDMWord:
			if m.GetAddress()+ic*2 > DM_AREA_SIZE {
				log.Printf("Address range exceeded for DMWord")
				return newErrorResponse(r, mapping.EndCodeAddressRangeExceeded)
			}

			if r.GetCommandCode() == mapping.CommandCodeMemoryAreaRead {
				data = s.dmarea[m.GetAddress() : m.GetAddress()+ic*2]
			} else {
				if len(r.GetData()) < 6+int(ic*2) {
					log.Printf("Insufficient data for DMWord write")
					return newErrorResponse(r, mapping.EndCodeNotSupportedByModelVersion)
				}
				copy(s.dmarea[m.GetAddress():m.GetAddress()+ic*2], r.GetData()[6:6+ic*2])
			}

		case mapping.MemoryAreaDMBit:
			if m.GetAddress()+ic > DM_AREA_SIZE {
				log.Printf("Address range exceeded for DMBit")
				return newErrorResponse(r, mapping.EndCodeAddressRangeExceeded)
			}

			start := m.GetAddress() + uint16(m.GetBitOffset())
			if r.GetCommandCode() == mapping.CommandCodeMemoryAreaRead {
				data = s.bitdmarea[start : start+ic]
			} else {
				if len(r.GetData()) < 6+int(ic) {
					log.Printf("Insufficient data for DMBit write")
					return newErrorResponse(r, mapping.EndCodeNotSupportedByModelVersion)
				}
				copy(s.bitdmarea[start:start+ic], r.GetData()[6:6+ic])
			}

		default:
			log.Printf("Unsupported memory area: 0x%02x", m.GetMemoryArea())
			return newErrorResponse(r, mapping.EndCodeNotSupportedByModelVersion)
		}

	default:
		log.Printf("Unsupported command code: 0x%04x", r.GetCommandCode())
		return newErrorResponse(r, mapping.EndCodeNotSupportedByModelVersion)
	}

	return fins.NewResponse(r, endCode, data)
}

func newErrorResponse(r fins.Request, endCode uint16) fins.Response {
	return fins.NewResponse(r, endCode, nil)
}

// Shut down the simulator
func (s *Server) Close() {
	s.closed = true
	s.listener.Close()
}
