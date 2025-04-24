package fins

import (
	"bufio"
	"fmt"
	"folke99/gofins/mapping"
	"log"
	"net"
	"time"
)

// Recreates plc connection and starts the listenloop
func (c *Client) Reconnect() error {
	c.Lock()
	defer c.Unlock()

	if c.listening {
		log.Print("Listener already exists, canceling reconnect")
		return nil
	}

	if c.closed {
		return fmt.Errorf("cannot reconnect: connection already closed")
	}

	c.conn.Close()

	// Attempt reconnection with backoff
	backoffIntervals := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		5 * time.Second,
		10 * time.Second,
	}

	for _, backoff := range backoffIntervals {
		log.Printf("Attempting to reconnect in %v", backoff)
		time.Sleep(backoff)

		dialer := net.Dialer{
			Timeout: time.Duration(DEFAULT_CONNECT_TIMEOUT) * time.Millisecond,
		}

		conn, err := dialer.Dial("tcp", c.plcAddr.tcpAddress.String())
		if err != nil {
			log.Printf("Reconnection attempt failed: %v", err)
			continue
		}

		// Update connection
		c.conn = conn
		c.reader = bufio.NewReader(conn)

		// Reestablish connection request
		err = c.sendConnectionRequest()
		if err != nil {
			log.Printf("Connection request failed: %v", err)
			conn.Close()
			continue
		}

		go c.listenLoop()

		log.Println("🔄 Connection successfully reestablished") //TODO: Remove trace?
		return nil
	}

	return fmt.Errorf("failed to reconnect after multiple attempts")
}

func (c *Client) Ping() error {
	log.Print("Pinging...")
	_, err := c.ReadClock()
	if err != nil {
		return err
	}
	log.Print("Pong")
	return nil
}

type PLCStatus struct {
	Status     mapping.StatusCode
	Mode       mapping.ModeCode
	FatalError FatalErrorCode
}

func (c *Client) Status() (*PLCStatus, error) {
	log.Printf("Getting status...") // TODO: remove trace
	response, err := c.ReadPLCStatus()
	if err != nil {
		return nil, err
	}

	// data[0] = Status
	// data[1] = Mode
	// data[2:18] = FatalError (16 bytes)

	if len(response.data) < 18 {
		return nil, fmt.Errorf("incomplete status data")
	}

	status := &PLCStatus{
		Status: mapping.StatusCode(response.data[0]),
		Mode:   mapping.ModeCode(response.data[1]),
	}

	// Process fatal error flags
	var fatalError FatalErrorCode
	for i := 0; i < 16; i++ {
		if response.data[i+2] == 1 {
			fatalError |= FatalErrorCode(1 << i)
		}
	}
	status.FatalError = fatalError

	return status, nil
}

// Helper methods for checking status and errors
func (s *PLCStatus) IsRunning() bool {
	return s.Status == mapping.StatusRun
}

func (s *PLCStatus) IsStopped() bool {
	return s.Status == mapping.StatusStop
}

func (s *PLCStatus) IsStandby() bool {
	return s.Status == mapping.StatusStandby
}

func (s *PLCStatus) HasFatalError() bool {
	return s.FatalError != 0
}

func (s *PLCStatus) HasError(errType FatalErrorCode) bool {
	return (s.FatalError & errType) != 0
}
