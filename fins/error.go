package fins

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

// Client errors

type ResponseTimeoutError struct {
	duration time.Duration
}

func (e ResponseTimeoutError) Error() string {
	return fmt.Sprintf("Response timeout of %d has been reached", e.duration)
}

type IncompatibleMemoryAreaError struct {
	area byte
}

func (e IncompatibleMemoryAreaError) Error() string {
	return fmt.Sprintf("The memory area is incompatible with the data type to be read: 0x%X", e.area)
}

// Driver errors

type BCDBadDigitError struct {
	v   string
	val uint64
}

func (e BCDBadDigitError) Error() string {
	return fmt.Sprintf("Bad digit in BCD decoding: %s = %d", e.v, e.val)
}

type BCDOverflowError struct{}

func (e BCDOverflowError) Error() string {
	return "Overflow occurred in BCD decoding"
}

// BCD encoding/decoding
type BCDError struct {
	msg string
}

func (e BCDError) Error() string {
	return fmt.Sprintf("BCD error: %s", e.msg)
}

// Helper function to handle read errors
func handleReadError(err error, consecutiveErrors *int, maxErrors int, c *Client) bool {
	*consecutiveErrors++
	if *consecutiveErrors >= maxErrors {
		log.Printf("Too many consecutive errors (%d), exiting listen loop", *consecutiveErrors)
		c.closed = true
		return true
	}

	if c.closed {
		return true
	}

	if err == io.EOF {
		log.Printf("Connection closed by peer (EOF)")
		c.closed = true
		return true
	}

	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		log.Printf("Read timeout: %v", netErr)
	} else {
		log.Printf("Read error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	return false
}
