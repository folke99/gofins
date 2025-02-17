package fins

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

const (
	DEFAULT_RESPONSE_TIMEOUT = 20   // ms
	TCP_HEADER_SIZE          = 16   // FINS/TCP header size
	MAX_PACKET_SIZE          = 4096 // Maximum size of FINS packet
)

// Client Omron FINS client using TCP
type Client struct {
	conn net.Conn
	resp []chan response
	sync.Mutex
	dst               finsAddress
	src               finsAddress
	sid               byte
	closed            bool
	responseTimeoutMs time.Duration
	byteOrder         binary.ByteOrder
	reader            *bufio.Reader
}

// NewClient creates a new Omron FINS client over TCP
func NewClient(localAddr, plcAddr Address) (*Client, error) {
	c := new(Client)
	c.dst = plcAddr.finsAddress
	c.src = localAddr.finsAddress
	c.responseTimeoutMs = DEFAULT_RESPONSE_TIMEOUT
	c.byteOrder = binary.BigEndian

	// Set connection timeout
	dialer := net.Dialer{
		Timeout: time.Duration(c.responseTimeoutMs) * time.Millisecond,
	}

	// Dial TCP connection with timeout
	conn, err := dialer.Dial("tcp", plcAddr.tcpAddress.String())
	if err != nil {
		return nil, fmt.Errorf("failed to establish TCP connection: %w", err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.resp = make([]chan response, 256)

	// Initialize response channels
	for i := range c.resp {
		c.resp[i] = make(chan response, 1) // Buffered channel to prevent blocking
	}

	go c.listenLoop()
	return c, nil
}

// Set byte order
// Default value: binary.BigEndian
func (c *Client) SetByteOrder(o binary.ByteOrder) {
	c.byteOrder = o
}

// Set response timeout duration (ms).
// Default value: 20ms.
// A timeout of zero can be used to block indefinitely.
func (c *Client) SetTimeoutMs(t uint) {
	c.responseTimeoutMs = time.Duration(t)
}

// Close gracefully closes the TCP connection
func (c *Client) Close() {
	c.Lock()
	defer c.Unlock()

	if !c.closed {
		c.closed = true
		c.conn.Close()

		// Clean up response channels
		for i := range c.resp {
			close(c.resp[i])
		}
	}
}

// ReadWords Reads words from the PLC data area
func (c *Client) ReadWords(memoryArea byte, address uint16, readCount uint16) ([]uint16, error) {
	if checkIsWordMemoryArea(memoryArea) == false {
		return nil, IncompatibleMemoryAreaError{memoryArea}
	}
	command := readCommand(memAddr(memoryArea, address), readCount)
	r, e := c.sendCommand(command)
	e = checkResponse(r, e)
	if e != nil {
		return nil, e
	}

	data := make([]uint16, readCount, readCount)
	for i := 0; i < int(readCount); i++ {
		data[i] = c.byteOrder.Uint16(r.data[i*2 : i*2+2])
	}

	return data, nil
}

func (c *Client) ReadBytes(memoryArea byte, address uint16, byteCount uint16) ([]byte, error) {
	if !checkIsWordMemoryArea(memoryArea) {
		return nil, IncompatibleMemoryAreaError{memoryArea}
	}

	// Ensure read count is word-aligned
	if byteCount%2 != 0 {
		return nil, fmt.Errorf("requested byte count must be a multiple of 2 for word-based memory area")
	}

	// Convert bytes to words (FINS protocol expects word count)
	wordCount := byteCount / 2

	command := readCommand(memAddr(memoryArea, address), wordCount)
	r, e := c.sendCommand(command)
	e = checkResponse(r, e)
	if e != nil {
		return nil, e
	}

	return r.data, nil
}

// ReadString reads a string from the PLC's DM memory area NEW
func (c *Client) ReadString(memoryArea byte, address uint16, byteCount uint16) (string, error) {
	if !checkIsWordMemoryArea(memoryArea) {
		return "", IncompatibleMemoryAreaError{memoryArea}
	}

	// Ensure the read byte count is word-aligned
	if byteCount%2 != 0 {
		byteCount++
	}

	// Read bytes from PLC
	data, err := c.ReadBytes(memoryArea, address, byteCount)
	if err != nil {
		return "", err
	}

	// Trim null bytes (if string was null-terminated)
	return string(bytes.TrimRight(data, "\x00")), nil
}

// ReadBits Reads bits from the PLC data area
func (c *Client) ReadBits(memoryArea byte, address uint16, bitOffset byte, readCount uint16) ([]bool, error) {
	if checkIsBitMemoryArea(memoryArea) == false {
		return nil, IncompatibleMemoryAreaError{memoryArea}
	}
	command := readCommand(memAddrWithBitOffset(memoryArea, address, bitOffset), readCount)
	r, e := c.sendCommand(command)
	e = checkResponse(r, e)
	if e != nil {
		return nil, e
	}

	data := make([]bool, readCount, readCount)
	for i := 0; i < int(readCount); i++ {
		data[i] = r.data[i]&0x01 > 0
	}

	return data, nil
}

// ReadClock Reads the PLC clock
func (c *Client) ReadClock() (*time.Time, error) {
	r, e := c.sendCommand(clockReadCommand())
	e = checkResponse(r, e)
	if e != nil {
		return nil, e
	}
	year, _ := decodeBCD(r.data[0:1])
	if year < 50 {
		year += 2000
	} else {
		year += 1900
	}
	month, _ := decodeBCD(r.data[1:2])
	day, _ := decodeBCD(r.data[2:3])
	hour, _ := decodeBCD(r.data[3:4])
	minute, _ := decodeBCD(r.data[4:5])
	second, _ := decodeBCD(r.data[5:6])

	t := time.Date(
		int(year), time.Month(month), int(day), int(hour), int(minute), int(second),
		0, // nanosecond
		time.Local,
	)
	return &t, nil
}

// WriteWords Writes words to the PLC data area
func (c *Client) WriteWords(memoryArea byte, address uint16, data []uint16) error {
	if checkIsWordMemoryArea(memoryArea) == false {
		return IncompatibleMemoryAreaError{memoryArea}
	}
	l := uint16(len(data))
	bts := make([]byte, 2*l, 2*l)
	for i := 0; i < int(l); i++ {
		c.byteOrder.PutUint16(bts[i*2:i*2+2], data[i])
	}
	command := writeCommand(memAddr(memoryArea, address), l, bts)

	return checkResponse(c.sendCommand(command))
}

// WriteString writes a string to the PLC's DM memory area
func (c *Client) WriteString(memoryArea byte, address uint16, s string) error {
	if !checkIsWordMemoryArea(memoryArea) {
		return IncompatibleMemoryAreaError{memoryArea}
	}

	// Convert string to bytes
	b := []byte(s)

	// Ensure word alignment by padding with a null byte if needed
	if len(b)%2 != 0 {
		b = append(b, 0x00)
	}

	// Write to PLC
	return c.WriteBytes(memoryArea, address, b)
}

func (c *Client) WriteBytes(memoryArea byte, address uint16, b []byte) error {
	if !checkIsWordMemoryArea(memoryArea) {
		return IncompatibleMemoryAreaError{memoryArea}
	}

	// Ensure byte slice is an even length (word-aligned)
	if len(b)%2 != 0 {
		return fmt.Errorf("data length must be a multiple of 2 for word-based memory area")
	}

	// Convert bytes to words (FINS protocol expects word count)
	wordCount := uint16(len(b) / 2)

	command := writeCommand(memAddr(memoryArea, address), wordCount, b)
	return checkResponse(c.sendCommand(command))
}

// WriteBits Writes bits to the PLC data area
func (c *Client) WriteBits(memoryArea byte, address uint16, bitOffset byte, data []bool) error {
	if checkIsBitMemoryArea(memoryArea) == false {
		return IncompatibleMemoryAreaError{memoryArea}
	}
	l := uint16(len(data))
	bts := make([]byte, 0, l)
	var d byte
	for i := 0; i < int(l); i++ {
		if data[i] {
			d = 0x01
		} else {
			d = 0x00
		}
		bts = append(bts, d)
	}
	command := writeCommand(memAddrWithBitOffset(memoryArea, address, bitOffset), l, bts)

	return checkResponse(c.sendCommand(command))
}

// SetBit Sets a bit in the PLC data area
func (c *Client) SetBit(memoryArea byte, address uint16, bitOffset byte) error {
	return c.bitTwiddle(memoryArea, address, bitOffset, 0x01)
}

// ResetBit Resets a bit in the PLC data area
func (c *Client) ResetBit(memoryArea byte, address uint16, bitOffset byte) error {
	return c.bitTwiddle(memoryArea, address, bitOffset, 0x00)
}

// ToggleBit Toggles a bit in the PLC data area
func (c *Client) ToggleBit(memoryArea byte, address uint16, bitOffset byte) error {
	b, e := c.ReadBits(memoryArea, address, bitOffset, 1)
	if e != nil {
		return e
	}
	var t byte
	if b[0] {
		t = 0x00
	} else {
		t = 0x01
	}
	return c.bitTwiddle(memoryArea, address, bitOffset, t)
}

func (c *Client) bitTwiddle(memoryArea byte, address uint16, bitOffset byte, value byte) error {
	if checkIsBitMemoryArea(memoryArea) == false {
		return IncompatibleMemoryAreaError{memoryArea}
	}
	mem := memoryAddress{memoryArea, address, bitOffset}
	command := writeCommand(mem, 1, []byte{value})

	return checkResponse(c.sendCommand(command))
}

func checkResponse(r *response, e error) error {
	if e != nil {
		return e
	}
	if r.endCode != EndCodeNormalCompletion {
		return fmt.Errorf("error reported by destination, end code 0x%x", r.endCode)
	}
	return nil
}

func (c *Client) nextHeader() *Header {
	sid := c.incrementSid()
	header := defaultCommandHeader(c.src, c.dst, sid)
	return &header
}

func (c *Client) incrementSid() byte {
	c.Lock() //thread-safe sid incrementation
	c.sid++
	sid := c.sid
	c.Unlock()
	c.resp[sid] = make(chan response) //clearing cell of storage for new response
	return sid
}

// sendCommand sends a FINS command and waits for a response
func (c *Client) sendCommand(command []byte) (*response, error) {
	if c.closed {
		return nil, fmt.Errorf("connection is closed")
	}

	header := c.nextHeader()
	bts := encodeHeader(*header)
	bts = append(bts, command...)

	// Add FINS/TCP header
	length := uint32(len(bts))
	tcpHeader := make([]byte, 4)
	binary.BigEndian.PutUint32(tcpHeader, length)
	fullPacket := append(tcpHeader, bts...)

	// Set write deadline if timeout is specified
	if c.responseTimeoutMs > 0 {
		deadline := time.Now().Add(time.Duration(c.responseTimeoutMs) * time.Millisecond)
		if err := c.conn.SetDeadline(deadline); err != nil {
			return nil, fmt.Errorf("failed to set deadline: %w", err)
		}
		defer c.conn.SetDeadline(time.Time{}) // Reset deadline after operation
	}

	// Send the command over TCP
	_, err := c.conn.Write(fullPacket)
	if err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	// Wait for response with timeout
	if c.responseTimeoutMs > 0 {
		select {
		case resp, ok := <-c.resp[header.serviceID]:
			if !ok {
				return nil, fmt.Errorf("response channel closed")
			}
			return &resp, nil
		case <-time.After(time.Duration(c.responseTimeoutMs) * time.Millisecond):
			return nil, ResponseTimeoutError{c.responseTimeoutMs}
		}
	}

	// No timeout specified, wait indefinitely
	resp, ok := <-c.resp[header.serviceID]
	if !ok {
		return nil, fmt.Errorf("response channel closed")
	}
	return &resp, nil
}

// listenLoop listens for incoming TCP responses
func (c *Client) listenLoop() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in listenLoop: %v", r)
		}
	}()

	for {
		if c.closed {
			return
		}

		// Read 4-byte length header
		lengthBuf := make([]byte, 4)
		_, err := io.ReadFull(c.reader, lengthBuf)
		if err != nil {
			if c.closed {
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Handle timeout error
				log.Printf("Timeout reading message length: %v", err)
				continue
			}
			log.Printf("Error reading message length: %v", err)
			continue
		}

		// Get message length
		messageLength := binary.BigEndian.Uint32(lengthBuf)
		if messageLength > MAX_PACKET_SIZE {
			log.Printf("Message length %d exceeds maximum size", messageLength)
			continue
		}

		// Read the full message
		messageBuf := make([]byte, messageLength)
		_, err = io.ReadFull(c.reader, messageBuf)
		if err != nil {
			if c.closed {
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Handle timeout error
				log.Printf("Timeout reading message body: %v", err)
				continue
			}
			log.Printf("Error reading message body: %v", err)
			continue
		}

		// Decode and process the response
		ans := decodeResponse(messageBuf)

		// Use non-blocking channel send with timeout
		select {
		case c.resp[ans.header.serviceID] <- ans:
		default:
			log.Printf("Warning: Response channel for SID %d is full", ans.header.serviceID)
		}
	}
}

// SetKeepAlive enables TCP keepalive with the specified interval
func (c *Client) SetKeepAlive(enabled bool, interval time.Duration) error {
	tcpConn, ok := c.conn.(*net.TCPConn)
	if !ok {
		return fmt.Errorf("connection is not TCP")
	}

	if err := tcpConn.SetKeepAlive(enabled); err != nil {
		return err
	}

	if enabled {
		return tcpConn.SetKeepAlivePeriod(interval)
	}
	return nil
}

func checkIsWordMemoryArea(memoryArea byte) bool {
	if memoryArea == MemoryAreaDMWord ||
		memoryArea == MemoryAreaARWord ||
		memoryArea == MemoryAreaHRWord ||
		memoryArea == MemoryAreaWRWord {
		return true
	}
	return false
}

func checkIsBitMemoryArea(memoryArea byte) bool {
	if memoryArea == MemoryAreaDMBit ||
		memoryArea == MemoryAreaARBit ||
		memoryArea == MemoryAreaHRBit ||
		memoryArea == MemoryAreaWRBit {
		return true
	}
	return false
}
