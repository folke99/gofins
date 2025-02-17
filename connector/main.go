package connector

import (
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"time"

	"folke99/gofins/fins"
)

// KilnTag represents a PLC tag with its address and data type
type KilnTag struct {
	Name        string
	Address     uint16
	Bit         uint8
	DataType    string // "REAL" or "BOOL"
	RequestType string // "DUAL"
}

func main() {
	// Define connection parameters
	plcIP := flag.String("plc-ip", "10.1.0.33", "PLC IP address")
	plcPort := flag.Int("plc-port", 9633, "PLC port number")
	plcNetwork := flag.String("plc-network", "0", "PLC network number (DNA)")
	plcNode := flag.String("plc-node", "0", "PLC node number (DA1)")
	plcUnit := flag.String("plc-unit", "0", "PLC unit number (DA2)")
	clientNetwork := flag.String("client-network", "0", "Client network number (DNA)")
	clientNode := flag.String("client-node", "1", "Client node number (DA1)")
	clientUnit := flag.String("client-unit", "0", "Client unit number (DA2)")
	flag.Parse()

	// Define the tags we want to test
	kilnTags := []KilnTag{
		{
			Name:        "fanSpeed",
			Address:     8172,
			DataType:    "REAL",
			RequestType: "DUAL",
		},
		{
			Name:        "ventilationPortOutput",
			Address:     8230,
			DataType:    "REAL",
			RequestType: "DUAL",
		},
		{
			Name:        "ventilationWallOutput",
			Address:     8266,
			DataType:    "REAL",
			RequestType: "DUAL",
		},
		{
			Name:        "kilnIsPaused",
			Address:     55,
			Bit:         9,
			DataType:    "BOOL",
			RequestType: "DUAL",
		},
		{
			Name:        "kilnIsStarted",
			Address:     50,
			Bit:         1,
			DataType:    "BOOL",
			RequestType: "DUAL",
		},
	}

	clientAddr, err := fins.NewAddress(getLocalIP(), 0, byte(parseHex(*clientNetwork)), byte(parseHex(*clientNode)), byte(parseHex(*clientUnit)))
	plcAddr, err := fins.NewAddress(*plcIP, *plcPort, byte(parseHex(*plcNetwork)), byte(parseHex(*plcNode)), byte(parseHex(*plcUnit)))

	log.Printf("Connecting to PLC at %s:%d", *plcIP, *plcPort)

	client, err := fins.NewClient(clientAddr, plcAddr)
	if err != nil {
		log.Fatalf("Failed to create FINS client: %v", err)
	}
	defer client.Close()

	client.SetTimeoutMs(1000) // 1 second timeout

	// Run tests for each tag
	for _, tag := range kilnTags {
		testTag(client, tag)
		time.Sleep(500 * time.Millisecond) // Small delay between tests
	}

	// Run continuous monitoring if requested
	monitorTags(client, kilnTags)
}

func testTag(client *fins.Client, tag KilnTag) {
	log.Printf("Testing tag: %s", tag.Name)

	switch tag.DataType {
	case "REAL":
		testREALTag(client, tag)
	case "BOOL":
		testBoolTag(client, tag)
	default:
		log.Printf("Unsupported data type for tag %s: %s", tag.Name, tag.DataType)
	}
}

func testREALTag(client *fins.Client, tag KilnTag) {
	// Read current value
	words, err := client.ReadWords(fins.MemoryAreaDMWord, tag.Address, 2) // REAL takes 2 words
	if err != nil {
		log.Printf("❌ Failed to read %s: %v", tag.Name, err)
		return
	}

	// Convert 2 words to float32
	bits := uint32(words[1])<<16 | uint32(words[0])
	value := math.Float32frombits(bits)

	log.Printf("✅ %s = %f", tag.Name, value)

	// Optional: Test writing a value
	testValue := float32(50.5)
	if tag.Name == "fanSpeed" { // Only test writing to fan speed
		bits = math.Float32bits(testValue)
		writeWords := []uint16{uint16(bits), uint16(bits >> 16)}

		err = client.WriteWords(fins.MemoryAreaDMWord, tag.Address, writeWords)
		if err != nil {
			log.Printf("❌ Failed to write test value to %s: %v", tag.Name, err)
			return
		}

		// Read back the value to verify
		words, err = client.ReadWords(fins.MemoryAreaDMWord, tag.Address, 2)
		if err != nil {
			log.Printf("❌ Failed to read back test value from %s: %v", tag.Name, err)
			return
		}

		bits = uint32(words[1])<<16 | uint32(words[0])
		readBack := math.Float32frombits(bits)

		if readBack == testValue {
			log.Printf("✅ Successfully wrote and read back test value for %s: %f", tag.Name, readBack)
		} else {
			log.Printf("❌ Value mismatch for %s: wrote %f, read %f", tag.Name, testValue, readBack)
		}
	}
}

func testBoolTag(client *fins.Client, tag KilnTag) {
	// Read current value
	bits, err := client.ReadBits(fins.MemoryAreaDMBit, tag.Address, tag.Bit, 1)
	if err != nil {
		log.Printf("❌ Failed to read %s: %v", tag.Name, err)
		return
	}

	log.Printf("✅ %s = %v", tag.Name, bits[0])

	// Don't test writing to these bits as they might be important control bits
	// If you want to test writing, uncomment the following:
	/*
		// Test toggling the bit
		err = client.ToggleBit(fins.MemoryAreaDMBit, tag.Address, tag.Bit)
		if err != nil {
			log.Printf("❌ Failed to toggle %s: %v", tag.Name, err)
			return
		}

		// Read back the toggled value
		bits, err = client.ReadBits(fins.MemoryAreaDMBit, tag.Address, tag.Bit, 1)
		if err != nil {
			log.Printf("❌ Failed to read back toggled value from %s: %v", tag.Name, err)
			return
		}

		log.Printf("✅ %s toggled to = %v", tag.Name, bits[0])
	*/
}

func monitorTags(client *fins.Client, tags []KilnTag) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	log.Println("Starting continuous monitoring (Ctrl+C to stop)...")

	for range ticker.C {
		log.Println("\n--- Current Values ---")
		for _, tag := range tags {
			switch tag.DataType {
			case "REAL":
				words, err := client.ReadWords(fins.MemoryAreaDMWord, tag.Address, 2)
				if err == nil {
					bits := uint32(words[1])<<16 | uint32(words[0])
					value := math.Float32frombits(bits)
					log.Printf("%s = %f", tag.Name, value)
				}
			case "BOOL":
				bits, err := client.ReadBits(fins.MemoryAreaDMBit, tag.Address, tag.Bit, 1)
				if err == nil {
					log.Printf("%s = %v", tag.Name, bits[0])
				}
			}
		}
	}
}

func parseHex(s string) int {
	var value int
	fmt.Sscanf(s, "%x", &value)
	return value
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatalf("Failed to get local IP: %v", err)
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String() // Returns first non-loopback IPv4 address
		}
	}
	return "127.0.0.1" // Fallback
}
