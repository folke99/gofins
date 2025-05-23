package fins

import (
	"fmt"
	"log"
)

// Header represents a FINS frame header structure
type Header struct {
	icf uint8 // Information Control Field
	rsv uint8 // Reserved
	gct uint8 // Gateway Count
	dna uint8 // Destination Network Address
	da1 uint8 // Destination Node Address
	da2 uint8 // Destination Unit Address
	sna uint8 // Source Network Address
	sa1 uint8 // Source Node Address
	sa2 uint8 // Source Unit Address
	sid uint8 // Service ID
}

//This is not used anywhere? originaly called MagicFrame
// // Header represents a FINS frame header structure
// type FINSFrame struct {
// 	header     []byte // 46,49,4E,53 == FINS
// 	length     []byte // init == 12 || FINSHeader = 8 + finsHeader = 10 + command(differs depending on command)
// 	command    []byte // init == 0, response == 1, read/write == 2
// 	errorCode  []byte // 00, 00, 00, 00
// 	clientNode []byte // Only for initial connection
// }

const (
	// ICF (Information Control Field) bits
	ICFCommandResponse  uint8 = 0x80 // 1 = Command, 0 = Response
	ICFResponseRequired uint8 = 0x40 // 1 = Response required, 0 = Response not required

	// Default values
	DefaultGatewayCount uint8 = 0x02 //0x02
	DefaultReserved     uint8 = 0x00
)

// defaultHeader creates a new Header with standard configuration
func defaultHeader(isCommand bool, responseRequired bool, src finsAddress, dst finsAddress, serviceID uint8) Header {
	var icf uint8
	if isCommand {
		icf |= ICFCommandResponse
	}
	if responseRequired {
		icf |= ICFResponseRequired
	}

	return Header{
		icf: 0x80,
		rsv: DefaultReserved,
		gct: DefaultGatewayCount,
		dna: dst.network,
		da1: dst.node,
		da2: dst.unit,
		sna: src.network,
		sa1: src.node,
		sa2: src.unit,
		sid: serviceID,
	}
}

// defaultCommandHeader creates a new command Header
func defaultCommandHeader(src finsAddress, dst finsAddress, serviceID uint8) Header {
	return defaultHeader(true, true, src, dst, serviceID)
}

// encodeHeader converts a Header to its byte representation
func encodeHeader(h Header) []byte {
	return []byte{
		h.icf,
		h.rsv,
		h.gct,
		h.dna,
		h.da1,
		h.da2,
		h.sna,
		h.sa1,
		h.sa2,
		h.sid,
	}
}

// decodeHeader creates a Header from its byte representation
func decodeHeader(data []byte) (Header, error) {
	if len(data) < 10 {
		return Header{}, fmt.Errorf("insufficient data for FINS header: expected 10 bytes, got %d", len(data))
	}

	return Header{
		icf: data[0],
		rsv: data[1],
		gct: data[2],
		dna: data[3],
		da1: data[4],
		da2: data[5],
		sna: data[6],
		sa1: data[7],
		sa2: data[8],
		sid: data[9],
	}, nil
}

// IsCommand returns true if the header represents a command message
func (h Header) IsCommand() bool {
	return h.icf&ICFCommandResponse != 0
}

// IsResponseRequired returns true if a response is required for this message
func (h Header) IsResponseRequired() bool {
	return h.icf&ICFResponseRequired != 0
}

// Increments the SID and returns the next header
func (c *Client) nextHeader() *Header {
	sid := c.incrementSid()
	header := defaultCommandHeader(c.src, c.dst, sid)
	return &header
}

func (c *Client) incrementSid() byte {
	c.Lock()
	startSid := c.sid
	for {
		c.sid++
		if c.sid == 0 {
			c.sid = 1
		}

		c.respMutex.Lock()
		_, inUse := c.resp[c.sid]
		c.respMutex.Unlock()

		if !inUse {
			break
		}

		if c.sid == startSid {
			log.Printf("Warning: All SIDs appear to be in use, reusing SID %d", c.sid)
			break
		}
	}

	sid := c.sid
	c.Unlock()

	return sid
}
