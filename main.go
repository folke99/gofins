// CONNECTOR: Part of connector logic
package main

import (
	"fmt"
	"folke99/gofins/fins"
	"folke99/gofins/mapping"
	"log"
	"math"
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
	//testTCPConnection("10.1.0.33", 9635)
	if err := testTCPConnection("10.1.0.32", 9532); err != nil {
		log.Printf("⚠️  TCP test failed: %v", err)
	} else {
		log.Printf("✅ TCP connection test successful")
	}
	if err := testTCPConnection("10.1.0.33", 9635); err != nil {
		log.Printf("⚠️  TCP test failed: %v", err)
	} else {
		log.Printf("✅ TCP connection test successful")
	}

	log.Printf("Creating FINS connection...")
	client32, err := Connect(5000, "10.1.0.32", 9532, localIP, getLocalPort(9532)) //fins.NewClient(clientAddr, plcAddr)
	if err != nil {
		log.Printf("❌ Connection failed: %v", err)
		time.Sleep(2 * time.Second)
	}
	client33, err := Connect(5000, "10.1.0.33", 9635, localIP, getLocalPort(9635)) //fins.NewClient(clientAddr, plcAddr)
	if err != nil {
		log.Printf("❌ Connection failed: %v", err)
		time.Sleep(2 * time.Second)
	}

	// Write/Read from 10.1.0.33
	floatTest := float32(42.5)
	uintTestValue, err := ConvertFloat32ToOmronData(floatTest)
	if err != nil {
		log.Printf("Error in ConvertFloat32ToOmronData(floatTest), where floatTest=%f", floatTest)
	}

	err = client33.WriteWords(mapping.MemoryAreaDMWord, 8172, uintTestValue)
	if err != nil {
		log.Printf("failed to write REAL value to fanSpeed (address 8172)")
	}

	//reading the value back
	readValue, err := client33.ReadWords(mapping.MemoryAreaDMWord, 8172, 2)
	if err != nil {
		log.Printf("failed to read REAL value to fanSpeed (address 8172)")
	} else {
		log.Printf("✅ Successfully read value")
	}

	// Write/Read from 10.1.0.32
	err = client32.WriteWords(mapping.MemoryAreaDMWord, 8172, readValue)
	if err != nil {
		log.Printf("failed to write REAL value to fanSpeed (address 8172)")
	}

	//reading the value back
	readValue32, err := client32.ReadWords(mapping.MemoryAreaDMWord, 8172, 2)
	if err != nil {
		log.Printf("failed to read REAL value to fanSpeed (address 8172)")
	}
	log.Printf("✅ Successfully read value")

	readvalueFloat, _ := ConvertToFloat32(readValue32)

	log.Printf("Read value as float32: %f (should be 42.5)", readvalueFloat)

	defer func() {
		client32.Close()
		client33.Close()
	}()

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

func ConvertFloat32ToOmronData(value float32) ([]uint16, error) {
	// Convert to bits and then to hex
	valBits := math.Float32bits(value)
	fullHex := fmt.Sprintf("%x", valBits)

	if fullHex == "0" {
		fullHex = fmt.Sprintf("0000000%s", fullHex)
	}
	// Split into 4-digit values
	hexArray := []string{fullHex[0:4], fullHex[4:8]}

	// Check if converted values is 4-digits otherwise add zeros in the beginning
	integralHex := hexArray[0]
	fractionalHex := hexArray[1]

	for len(integralHex) < 4 {
		integralHex = fmt.Sprintf("0%s", integralHex)
	}

	for len(fractionalHex) < 4 {
		fractionalHex = fmt.Sprintf("0%s", fractionalHex)
	}

	// Convert to uint as Omron want's it
	integral, err := strconv.ParseUint(integralHex, 16, 32)

	if err != nil {
		return nil, err
	}

	fractional, err := strconv.ParseUint(fractionalHex, 16, 32)

	if err != nil {
		return nil, err
	}

	// Return omron data with values in different order
	return []uint16{uint16(fractional), uint16(integral)}, nil
}

func ConvertToFloat32(arr []uint16) (float32, error) {
	// Convert to hexadecimals
	integral := fmt.Sprintf("%x", arr[1])
	fractional := fmt.Sprintf("%x", arr[0])

	// Check if converted values is 4-digits otherwise add zeros in the beginning
	for len(integral) < 4 {
		integral = fmt.Sprintf("0%s", integral)
	}

	for len(fractional) < 4 {
		fractional = fmt.Sprintf("0%s", fractional)
	}

	// Add them together to make the whole float value
	hx := fmt.Sprintf("%s%s", integral, fractional)

	// Parse to Uint32
	fl, err := strconv.ParseUint(hx, 16, 32)

	if err != nil {
		return 0.0, err
	}

	floatVal := math.Float32frombits(uint32(fl))
	roundedVal := float32(math.Round(float64(floatVal)*10) / 10)

	// Convert to Float32
	return roundedVal, nil
}
