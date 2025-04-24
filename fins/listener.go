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
	FINS_MARKER                = "FINS" // FINS initiation frame number
)

func (c *Client) listenLoop() {
	defer func() {
		c.Lock()
		c.listening = false
		c.Unlock()

		c.respMutex.Lock()
		for sid, ch := range c.resp {
			close(ch)
			delete(c.resp, sid)
		}
		c.respMutex.Unlock()

		if r := recover(); r != nil {
			log.Printf("🚨 Panic recovered in listenLoop: %s", debug.Stack())
			if c.conn != nil {
				log.Printf("Connection details - Local: %v, Remote: %v",
					c.conn.LocalAddr(),
					c.conn.RemoteAddr())
			}
		}
	}()

	c.Lock()
	c.listening = true
	localConn := c.conn
	localReader := c.reader
	c.Unlock()

	if localConn == nil {
		log.Printf("Connection is nil in listenLoop, exiting")
		return
	}

	log.Printf("Starting listen loop with connection: %v", localConn.LocalAddr()) // TODO: Remove trace?

	if err := localConn.SetReadDeadline(time.Time{}); err != nil {
		log.Printf("Failed to clear read deadline: %v", err)
		return
	}

	scanner := bufio.NewScanner(localReader)
	scanBuffer := make([]byte, MAX_PACKET_SIZE)
	scanner.Buffer(scanBuffer, MAX_PACKET_SIZE)

	scanner.Split(c.finsSplitFunc)

	for scanner.Scan() {
		if c.closed {
			log.Printf("Connection closed, exiting listen loop")
			return
		}

		frameData := scanner.Bytes()
		frameCopy := make([]byte, len(frameData))
		copy(frameCopy, frameData)

		// Extract FINS message (skip header)
		messageBuf := frameCopy[16:]

		ans, err := DecodeResponse(messageBuf)
		if err != nil {
			log.Printf("Failed to decode response: %v", err)
			log.Printf("Message that failed decoding: % X", messageBuf)
			continue
		}

		c.channelHandler(ans)
	}

	if c.closed {
		log.Printf("Client closed, exiting listen loop cleanly")
		return
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Scanner error: %v, attempting to recover", err)
		log.Printf("Error details: %T %v", err, err)
	}
}

// Split function to properly frame FINS messages
func (c *Client) finsSplitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// Need at least 8 bytes for the header
	if len(data) < 8 {
		return 0, nil, nil
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

		return 1, nil, nil
	}

	messageLength := binary.BigEndian.Uint32(data[4:8])

	if messageLength == 0 || messageLength > MAX_PACKET_SIZE {
		log.Printf("Invalid message length: %d, skipping header", messageLength)
		return 8, nil, nil
	}

	totalLength := 8 + int(messageLength)
	if len(data) < totalLength {
		return 0, nil, nil // Need more data
	}

	return totalLength, data[:totalLength], nil
}

// Allocating response channels based on SIDs
func (c *Client) channelHandler(ans Response) {
	sid := ans.header.sid

	c.respMutex.Lock()
	responseChan, exists := c.resp[sid]
	c.respMutex.Unlock()

	if !exists {
		log.Printf("No waiting request found for SID %d, response discarded", sid)
		return
	}

	select {
	case responseChan <- ans:
		log.Printf("Response for SID %d delivered successfully", sid)
	default:
		log.Printf("Channel for SID %d is full or closed, attempting recovery", sid)

		// Try to empty response channel
		select {
		case <-responseChan:
			log.Printf("Successfully drained channel for SID %d, retrying delivery", sid)
		default:
			log.Printf("Channel for SID %d wasn't full, might be closed", sid)
		}

		// Try again with timeout
		select {
		case responseChan <- ans:
			log.Printf("Response for SID %d delivered after recovery attempt", sid)
		case <-time.After(100 * time.Millisecond):
			log.Printf("Unable to deliver response for SID %d after recovery attempt", sid)
		}
	}
}
