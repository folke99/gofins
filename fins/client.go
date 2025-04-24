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
	// resp []chan Response
	sync.Mutex
	plcAddr           Address
	dst               finsAddress
	src               finsAddress
	sid               byte
	closed            bool
	responseTimeoutMs time.Duration
	byteOrder         binary.ByteOrder
	reader            *bufio.Reader
	listening         bool

	resp      map[uint8]chan Response
	respMutex sync.Mutex // Dedicated mutex for response channels
}

// Note: These values are not optimized and can be further improved upon.
const (
	DEFAULT_RESPONSE_TIMEOUT = 10000
	DEFAULT_CONNECT_TIMEOUT  = 5000
	MAX_PACKET_SIZE          = 2048
)

func NewClient(localAddr, plcAddr Address) (*Client, error) {
	c := new(Client)
	c.plcAddr = plcAddr
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

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.resp = make(map[uint8]chan Response)

	for i := range c.resp {
		c.resp[i] = make(chan Response, 1)
	}

	err = c.sendConnectionRequest()
	if err != nil {
		return nil, err
	}

	go c.listenLoop()
	return c, nil
}

// Close gracefully closes the TCP connection
func (c *Client) Close() error {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true

	c.respMutex.Lock()
	for sid, ch := range c.resp {
		close(ch)
		delete(c.resp, sid)
	}
	c.respMutex.Unlock()

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
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

	log.Printf("📨 Sending FINS command - Service ID: %d", header.sid) // TODO: remove trace
	log.Printf("FullPacket: % X", fullPacket)                         // TODO: remove trace

	responseChan := make(chan Response, 1)

	c.respMutex.Lock()
	c.resp[header.sid] = responseChan
	c.respMutex.Unlock()

	defer func() {
		c.respMutex.Lock()
		delete(c.resp, header.sid)
		c.respMutex.Unlock()
	}()

	_, err := c.conn.Write(fullPacket)
	if err != nil {
		log.Printf("❌ Failed to send initiation packet!")
		return nil, fmt.Errorf("failed to send packet: %w", err)
	}
	log.Printf("Command sent successfully") // TODO: remove trace

	// Wait for response with timeout
	timeout := time.Duration(c.responseTimeoutMs) * time.Millisecond
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	select {
	case resp, ok := <-responseChan:
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

	log.Printf("Sending init frame: %02X with the connection: %+v", initFrame, c.conn) // TODO: remove trace
	if _, err := c.conn.Write(initFrame); err != nil {
		log.Printf("❌ Failed to send init frame: %v, Reconnecting", err)
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

	log.Printf("✅ Connection established. Client Node: %d, Server Node: %d Response: %02X", clientNode, serverNode, response) // TODO: remove?

	// Store these values for later messages
	c.src.node = clientNode
	c.dst.node = serverNode

	return nil
}

// Set response timeout duration (ms).
// Default value: 20ms.
// A timeout of zero can be used to block indefinitely.
func (c *Client) SetTimeoutMs(t uint) {
	c.responseTimeoutMs = time.Duration(t)
}

// SetKeepAlive enables keepalive with the specified interval
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
