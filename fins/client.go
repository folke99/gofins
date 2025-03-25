package fins

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"folke99/gofins/mapping"
	"log"
	"net"
	"sync"
	"time"
)

// Client Omron FINS client using TCP
type Client struct {
	conn net.Conn
	resp []chan Response
	sync.Mutex
	dst               finsAddress
	src               finsAddress
	sid               byte
	closed            bool
	responseTimeoutMs time.Duration
	byteOrder         binary.ByteOrder
	reader            *bufio.Reader
}

const (
	DEFAULT_RESPONSE_TIMEOUT = 2000
	DEFAULT_CONNECT_TIMEOUT  = 5000
	TCP_HEADER_SIZE          = 16
	MAX_PACKET_SIZE          = 2048
)

func NewClient(localAddr, plcAddr Address) (*Client, error) {
	c := new(Client)
	c.dst = plcAddr.finsAddress
	c.src = localAddr.finsAddress
	c.responseTimeoutMs = DEFAULT_RESPONSE_TIMEOUT
	c.byteOrder = binary.BigEndian
	c.sid = 0

	dialer := net.Dialer{
		Timeout: time.Duration(DEFAULT_CONNECT_TIMEOUT) * time.Millisecond,
	}

	conn, err := dialer.Dial("tcp", plcAddr.tcpAddress.String())
	if err != nil {
		return nil, fmt.Errorf("failed to establish TCP connection: %w", err)
	}

	// Set read timeout on the connection
	err = conn.SetReadDeadline(time.Now().Add(time.Duration(c.responseTimeoutMs) * time.Millisecond))
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.resp = make([]chan Response, 256)

	for i := range c.resp {
		c.resp[i] = make(chan Response, 1)
	}

	err = c.sendConnectionRequest()
	if err != nil {
		return nil, err
	}

	err = c.testInitialConnection()
	if err != nil {
		return nil, err
	}

	go c.listenLoop()

	err = c.TestEndpoints()
	if err != nil {
		log.Printf("Error testing endpoints: %f", err)
	}

	return c, nil
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

	//tracing
	log.Printf("Response from ReadWords(), %+v", r)

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

	//tracing
	log.Printf("Response from ReadBytes(), %+v", r)

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

	//tracing
	log.Printf("Response from ReadBits(), %+v", r)

	if e != nil {
		return nil, e
	}

	data := make([]bool, readCount, readCount)
	for i := 0; i < int(readCount); i++ {
		data[i] = r.data[i]&0x01 > 0
	}

	return data, nil
}

func (c *Client) ReadPLCStatus() error {
	log.Println("📡 Attempting to read PLC status...")

	// Command bytes for PLC Status Read (06 01)
	commandBytes := []byte{0x06, 0x01}

	// Send FINS command
	resp, err := c.sendCommand(commandBytes)
	if err != nil {
		return fmt.Errorf("failed to send PLC status command: %v", err)
	}

	log.Println("✅ Command sent successfully")
	log.Printf("📩 Response received: %+v", resp)

	// Decode the response to check the command code and structure
	err = checkResponse(resp, err)
	if err != nil {
		return fmt.Errorf("failed to parse FINS response: %v", err)
	}

	return nil
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

func checkResponse(r *Response, e error) error {
	if e != nil {
		return e
	}
	if r.endCode != mapping.EndCodeNormalCompletion {
		return fmt.Errorf("error reported by destination, end code 0x%x", r.endCode)
	}
	return nil
}

func (c *Client) sendCommand(command []byte) (*Response, error) {
	if c.closed {
		return nil, fmt.Errorf("connection is closed")
	}

	commandLength := len(command)
	c.sendInitFrame((18 + commandLength), 2, false)

	header := c.nextHeader()
	fullPacket := encodeHeader(*header)
	fullPacket = append(fullPacket, command...)

	log.Printf("📨 Sending FINS command - Service ID: %d", header.sid)
	log.Printf("FullPacket: % X", fullPacket)

	// Create response channel if it doesn't exist
	if c.resp[header.sid] == nil {
		c.resp[header.sid] = make(chan Response, 1)
		log.Printf("Response channel created for sid, %+v", header.sid)
	}

	// Send command with retries
	// TODO: Do we need to retry if the first fail?
	var lastError error
	for i := 0; i < 3; i++ {
		if err := c.conn.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
			return nil, fmt.Errorf("failed to set write deadline: %w", err)
		}

		_, err := c.conn.Write(fullPacket)
		if err == nil {
			log.Printf("Command sent successfully")
			break
		}
		lastError = err
		log.Printf("Write attempt %d failed: %v", i+1, err)
		time.Sleep(100 * time.Millisecond)
	}

	if lastError != nil {
		return nil, fmt.Errorf("failed to send command after retries: %w", lastError)
	}

	// Wait for response
	timeout := time.Duration(c.responseTimeoutMs) * time.Millisecond
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	select {
	case resp, ok := <-c.resp[header.sid]:
		if !ok {
			return nil, fmt.Errorf("response channel closed")
		}
		log.Printf("Response received - Command Code: %04X, End Code: %04X", resp.commandCode, resp.endCode)
		return &resp, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("response timeout after %v", timeout)
	}
}

func (c *Client) sendInitFrame(length, commandCode int, initCon bool) error {
	initFrame := []byte{
		0x46, 0x49, 0x4E, 0x53, // "FINS"
		0x00, 0x00, 0x00, byte(length), // Length
		0x00, 0x00, 0x00, byte(commandCode), // Command
		0x00, 0x00, 0x00, 0x00, // Error code
	}

	if initCon {
		initFrame = append(initFrame, 0x00, 0x00, 0x00, 0x00) // Client node address (0 = auto-assign)
	}

	if _, err := c.conn.Write(initFrame); err != nil {
		log.Printf("❌ Failed to send init frame: %v", err)
		return err
	}
	return nil
}

func (c *Client) sendConnectionRequest() error {
	err := c.sendInitFrame(12, 0, true)
	if err != nil {
		return err
	}

	// Read response
	response := make([]byte, 24)
	n, err := c.reader.Read(response)
	if err != nil || n < 16 {
		return fmt.Errorf("failed to receive connection response: %v", err)
	}

	// Verify response header
	if !bytes.Equal(response[0:4], []byte{0x46, 0x49, 0x4E, 0x53}) { // "FINS"
		return fmt.Errorf("invalid FINS response header")
	}

	clientNode := response[19] // Client node assigned by PLC
	serverNode := response[23] // Server node

	log.Printf("✅ Connection established. Client Node: %d, Server Node: %d Response: %02X", clientNode, serverNode, response)

	// Store these values for later messages

	c.src.node = clientNode
	c.dst.node = serverNode

	return nil
}
