// CONNECTOR: Part of connector logic
package main

import (
	"fmt"
	"folke99/gofins/fins"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type KilnTag struct {
	Name        string
	Address     uint16
	Bit         uint8
	DataType    string
	RequestType string
}

// ErrorLogger handles rate-limited error logging
type ErrorLogger struct {
	lastError     time.Time
	errorCount    int
	mutex         sync.Mutex
	minimumPeriod time.Duration
}

func NewErrorLogger(minimumPeriod time.Duration) *ErrorLogger {
	return &ErrorLogger{
		minimumPeriod: minimumPeriod,
	}
}

func (e *ErrorLogger) LogError(format string, v ...interface{}) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	now := time.Now()
	if now.Sub(e.lastError) > e.minimumPeriod {
		if e.errorCount > 0 {
			log.Printf("(Suppressed %d similar errors)\n", e.errorCount)
			e.errorCount = 0
		}
		log.Printf(format, v...)
		e.lastError = now
	} else {
		e.errorCount++
	}
}

var errorLogger = NewErrorLogger(5 * time.Second) // Log similar errors at most every 5 seconds

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds) // Add microseconds to log timestamps

	// Clear terminal and print header
	fmt.Print("\033[H\033[2J") // Clear screen
	fmt.Print("\033[H")        // Move cursor to top
	printHeader()

	localIP, err := getLocalIp()
	if err != nil {
		log.Fatalf("❌ Failed to get local IP: %v", err)
	}

	localPort := getLocalPort(9635)
	node, err := strconv.ParseInt(strings.Split(localIP, ".")[3], 10, 32)
	if err != nil {
		log.Fatalf("❌ Failed to parse node: %v", err)
	}

	log.Printf("\n=== Configuration ===")
	log.Printf("Local IP: %s", localIP)
	log.Printf("Local Port: %d", localPort)
	log.Printf("Local Node: %d", node)
	log.Printf("PLC IP: 10.1.0.33 (hardcoded)")
	log.Printf("PLC Port: 9635 (hardcoded)")

	log.Printf("\n=== TCP Connection Test ===")
	if err := testTCPConnection("10.1.0.33", 9635); err != nil {
		log.Printf("⚠️  TCP test failed: %v", err)
	} else {
		log.Printf("✅ TCP connection test successful")
	}

	log.Printf("Creating FINS connection...")
	client, err := Connect(5000, "10.1.0.33", 9635, localIP, getLocalPort(9635))
	if err != nil {
		log.Printf("❌ Connection failed: %v", err)
		time.Sleep(2 * time.Second)
	}

	defer client.Close()

}

func Connect(timeout int, plcIP string, plcPort int, localIP string, localPort int) (*fins.Client, error) {
	node, err := strconv.ParseInt(strings.Split(localIP, ".")[3], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("could not get node from local IP: %+v", err)
	}

	cAddr, err := fins.NewAddress(localIP, localPort, 0, byte(node), 0)
	if err != nil {
		return nil, err
	}
	pAddr, err := fins.NewAddress(plcIP, plcPort, 0, byte(33), 0)
	if err != nil {
		return nil, err
	}

	log.Printf("Client address: %+v\n PLC address: %+v", cAddr, pAddr)

	log.Printf("Establishing connection to Omron at '%s:%d ClientNode: %d'", plcIP, plcPort, node)

	c, err := fins.NewClient(cAddr, pAddr)
	if err != nil {
		return nil, fmt.Errorf("could not create fins client: %+v", err)
	}

	// Set a longer timeout for initial connection
	c.SetTimeoutMs(uint(timeout))

	// Add delay after connection establishment
	time.Sleep(100 * time.Millisecond)

	return c, nil
}

func printHeader() {
	fmt.Println("================================")
	fmt.Println("   FINS TCP Connection Tester   ")
	fmt.Println("================================")
}

func testTCPConnection(ip string, port int) error {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, fmt.Sprintf("%d", port)), 5*time.Second)
	if err != nil {
		return fmt.Errorf("TCP connection failed: %v", err)
	}
	defer conn.Close()
	return nil
}

func getLocalIp() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")

	if err != nil {
		return "", err
	}

	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String(), err
}

func getLocalPort(plcPort int) int {
	tenths := plcPort % 100
	localPort := (tenths * 100) + 10000
	return localPort
}
