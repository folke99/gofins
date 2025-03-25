package fins

import (
	"bufio"
	"encoding/binary"
	"log"
	"runtime/debug"
	"time"
)

// Constants for FINS protocol
const (
	FINS_TCP_HEADER_LENGTH     = 16     // Standard FINS/TCP header length
	FINS_MIN_FRAME_LENGTH      = 8      // Minimum frame length
	FINS_COMMAND_HEADER_LENGTH = 12     // FINS command header length
	FINS_MAGIC                 = "FINS" // FINS magic number
)

func (c *Client) listenLoop() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in listenLoop: %v", r)
			debug.PrintStack()
		}
		log.Printf("Exiting listenLoop: %v", c.conn.LocalAddr())
	}()

	// Create scanner with a sufficiently large buffer
	scanner := bufio.NewScanner(c.reader)
	scanBuffer := make([]byte, MAX_PACKET_SIZE)
	scanner.Buffer(scanBuffer, MAX_PACKET_SIZE)

	// Set split function for FINS protocol
	scanner.Split(c.finsSplitFunc)

	// Set no read timeout
	if err := c.conn.SetReadDeadline(time.Time{}); err != nil {
		log.Printf("Failed to clear read deadline: %v", err)
	}

	for scanner.Scan() {
		if c.closed {
			log.Printf("Connection closed, exiting listen loop")
			return
		}

		// Get and copy the complete frame (since scanner buffer is reused)
		frameData := scanner.Bytes()
		frameCopy := make([]byte, len(frameData))
		copy(frameCopy, frameData)

		// Extract FINS message (skip TCP header)
		messageBuf := frameCopy[16:] // Skip the 16-byte header

		// Decode the response
		ans, err := DecodeResponse(messageBuf)
		if err != nil {
			log.Printf("Failed to decode response: %v", err)
			log.Printf("Message that failed decoding: % X", messageBuf)
			continue
		}

		// Send response to appropriate channel
		c.channelHandler(ans)
	}

	// Handle scanner errors
	if err := scanner.Err(); err != nil && !c.closed {
		log.Printf("Scanner error: %v, attempting to recover", err)

		// If connection is still valid, restart the listenLoop
		go c.listenLoop()
	}
}

// Split function to properly frame FINS messages
func (c *Client) finsSplitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// Need at least 8 bytes for the header
	if len(data) < 8 {
		return 0, nil, nil // Need more data
	}

	// Check for FINS marker
	if string(data[0:4]) != FINS_MAGIC {
		log.Printf("Invalid marker: %q, expected: %q", string(data[0:4]), FINS_MAGIC)

		// Try to resync by searching for the next FINS marker
		for i := 1; i < len(data)-3; i++ {
			if string(data[i:i+4]) == FINS_MAGIC {
				log.Printf("Resyncing, skipping %d bytes", i)
				return i, nil, nil
			}
		}

		// Skip one byte if we couldn't find the marker
		return 1, nil, nil
	}

	// Parse message length
	messageLength := binary.BigEndian.Uint32(data[4:8])

	// Sanity check message length
	if messageLength == 0 || messageLength > MAX_PACKET_SIZE {
		log.Printf("Invalid message length: %d, skipping header", messageLength)
		return 8, nil, nil
	}

	// Check if we have the complete message
	totalLength := 8 + int(messageLength) // header + payload
	if len(data) < totalLength {
		return 0, nil, nil // Need more data
	}

	// Return the complete message
	return totalLength, data[:totalLength], nil
}

// Handle decoded response
func (c *Client) channelHandler(ans Response) {
	sid := ans.header.sid

	// Ensure response channel exists
	c.Lock()
	if c.resp[sid] == nil {
		c.resp[sid] = make(chan Response, 1)
	}
	c.Unlock()

	// Try to send response non-blocking
	select {
	case c.resp[sid] <- ans:
	default:
		log.Printf("Channel for SID %d is full or closed", sid)
	}
}
