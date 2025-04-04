package fins

import (
	"bufio"
	"encoding/binary"
	"log"
	"runtime/debug"
	"time"
)

const (
	FINS_MIN_FRAME_LENGTH      = 8      // Minimum frame length
	FINS_COMMAND_HEADER_LENGTH = 12     // FINS command header length
	FINS_MARKER                = "FINS" // FINS magic number
)

func (c *Client) listenLoop() {
	defer func() {
		c.Lock()
		c.listening = false
		c.Unlock()
		if r := recover(); r != nil {
			log.Printf("🚨 Panic recovered in listenLoop: %s", debug.Stack())

			// Log connection details if available
			if c.conn != nil {
				log.Printf("Connection details - Local: %v, Remote: %v",
					c.conn.LocalAddr(),
					c.conn.RemoteAddr())
			}
		}
	}()

	// CRITICAL: Get a local copy of the connection to prevent race conditions
	c.Lock()
	c.listening = true
	localConn := c.conn
	localReader := c.reader
	c.Unlock()

	if localConn == nil {
		log.Printf("Connection is nil in listenLoop, exiting")
		return
	}

	log.Printf("Starting listen loop with connection: %v", localConn.LocalAddr())

	// Set no read deadline on our local connection reference
	if err := localConn.SetReadDeadline(time.Time{}); err != nil {
		log.Printf("Failed to clear read deadline: %v", err)
		return
	}

	// Create scanner with our local reader reference
	scanner := bufio.NewScanner(localReader)
	scanBuffer := make([]byte, MAX_PACKET_SIZE)
	scanner.Buffer(scanBuffer, MAX_PACKET_SIZE)

	// Set split function for FINS protocol
	scanner.Split(c.finsSplitFunc)

	for scanner.Scan() {
		// Make sure the client hasn't been closed while we were scanning
		if c.closed {
			log.Printf("Connection closed, exiting listen loop")
			return
		}

		// Process frame data
		frameData := scanner.Bytes()
		frameCopy := make([]byte, len(frameData))
		copy(frameCopy, frameData)

		// Extract FINS message (skip header)
		messageBuf := frameCopy[16:]

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

	// Check if the client has been closed properly
	if c.closed {
		log.Printf("Client closed, exiting listen loop cleanly")
		return
	}

	// Handle scanner errors
	if err := scanner.Err(); err != nil {
		log.Printf("Scanner error: %v, attempting to recover", err)

		// Show the error details for debugging
		log.Printf("Error details: %T %v", err, err)
	}
}

// Split function to properly frame FINS messages
func (c *Client) finsSplitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// Need at least 8 bytes for the header
	if len(data) < 8 {
		return 0, nil, nil // Need more data
	}

	// Check for FINS marker
	if string(data[0:4]) != FINS_MARKER {
		log.Printf("Invalid marker: %q, expected: %q", string(data[0:4]), FINS_MARKER)

		// Try to resync by searching for the next FINS marker
		for i := 1; i < len(data)-3; i++ {
			if string(data[i:i+4]) == FINS_MARKER {
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
