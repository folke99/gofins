package fins

import (
	"encoding/binary"
	"fmt"
	"folke99/gofins/mapping"
	"log"
	"math"
	"strconv"
)

func (c *Client) testInitialConnection() error {
	log.Print("START HARD TEST")
	//fullPacket, err := c.testControllerStatusReadCommand()
	//fullPacket, err := c.testControllerWriteCommand()
	fullPacket, err := c.testControllerReadCommand()

	if err != nil {
		return err
	}
	log.Printf("Full packet after init: %02X", fullPacket)

	// Send raw packet
	_, err = c.conn.Write(fullPacket)
	if err != nil {
		log.Printf("❌ Failed to send raw command: %v", err)
		return err
	}
	log.Printf("✅ Raw command sent successfully")

	responseBuffer := make([]byte, 1024)
	n, err := c.reader.Read(responseBuffer)
	if err != nil {
		log.Printf("❌ Failed to receive response: %v", err)
		return err
	}

	log.Printf("Full response buffer: %02X", responseBuffer)
	// Protocol-specific validation
	if n < 14 {
		return fmt.Errorf("insufficient response length: expected at least 14 bytes, got %d", n)
	}

	// Verify FINS marker
	if string(responseBuffer[0:4]) != "FINS" {
		return fmt.Errorf("invalid FINS marker: expected 'FINS', got %s", string(responseBuffer[0:4]))
	}

	// Validate message length
	expectedLength := binary.BigEndian.Uint32(responseBuffer[4:8])
	if expectedLength == 0 || expectedLength > uint32(n) {
		return fmt.Errorf("invalid message length: expected %d, actual response length %d", expectedLength, n)
	}

	// Parse header fields
	header := Header{
		icf: responseBuffer[8],
		rsv: responseBuffer[9],
		gct: responseBuffer[10],
		dna: responseBuffer[11],
		da1: responseBuffer[12],
		da2: responseBuffer[13],
		sna: responseBuffer[14],
		sa1: responseBuffer[15],
		sa2: responseBuffer[16],
		sid: responseBuffer[17],
	}

	// Validate response code and end code
	commandCode := binary.BigEndian.Uint16(responseBuffer[18:20])
	endCode := binary.BigEndian.Uint16(responseBuffer[20:22])

	log.Printf("📩 Received response details:")
	log.Printf("  Total bytes: %d", n)
	log.Printf("  FINS Marker: %s", string(responseBuffer[0:4]))
	log.Printf("  Message Length: %d", expectedLength)
	log.Printf("  ICF: %02X", header.icf)
	log.Printf("  Command Code: %04X", commandCode)
	log.Printf("  End Code: %04X", endCode)

	//Update header to not re-use
	c.nextHeader()

	log.Print("END HARD TEST")
	return nil
}

func (c *Client) testControllerStatusReadCommand() ([]byte, error) {
	err := c.sendInitFrame(20, 2, false)
	if err != nil {
		return nil, err
	}

	// FINS Header (10 bytes)
	finsHeader := []byte{
		0x80,       // ICF: Command, Response required
		0x00,       // RSV: Reserved (Always 00)
		0x02,       // GCT: Gateway count (Assuming direct connection)
		0x00,       // DNA: Destination Network Address (0 if local)
		c.dst.node, // DA1: Destination Node Address (Set this to match your PLC)
		0x00,       // DA2: Destination Unit Address (0 for CPU)
		0x00,       // SNA: Source Network Address (0 if local)
		c.src.node, // SA1: Source Node Address (Client's node number, adjust if needed)
		0x00,       // SA2: Source Unit Address (0 for CPU)
		0x01,       // SID: Service ID (Can be any value, used to match requests)
	}

	// FINS Command (2 bytes) - Controller Status Read (0601)
	command := []byte{0x07, 0x01}

	// Combine all parts into a single packet
	fullPacket := append(finsHeader, command...)

	log.Printf("🔧 Hardcoded Controller Status Read command packet: %+v", fullPacket)
	return fullPacket, nil
}

func (c *Client) testControllerWriteCommand() ([]byte, error) {
	err := c.sendInitFrame(30, 2, false)
	if err != nil {
		return nil, err
	}

	// FINS Header (10 bytes)
	finsHeader := []byte{
		0x80,       // ICF: Command, Response required
		0x00,       // RSV: Reserved (Always 00)
		0x02,       // GCT: Gateway count (Assuming direct connection)
		0x00,       // DNA: Destination Network Address (0 if local)
		c.dst.node, // DA1: Destination Node Address (Set this to match your PLC)
		0x00,       // DA2: Destination Unit Address (0 for CPU)
		0x00,       // SNA: Source Network Address (0 if local)
		c.src.node, // SA1: Source Node Address (Client's node number, adjust if needed)
		0x00,       // SA2: Source Unit Address (0 for CPU)
		0x01,       // SID: Service ID (Can be any value, used to match requests)
	}

	// FINS Command (2 bytes) - Controller Status Read (0601)
	command := []byte{0x01, 0x02, 0x82, 0x1F, 0xEC, 0x00, 0x00, 0x02, 0x00, 0x00, 0x42, 0x2A}

	// Combine all parts into a single packet
	fullPacket := append(finsHeader, command...)

	log.Printf("🔧 Hardcoded Controller Status Read command packet: %+v", fullPacket)
	return fullPacket, nil
}

func (c *Client) testControllerReadCommand() ([]byte, error) {
	err := c.sendInitFrame(26, 2, false)
	if err != nil {
		return nil, err
	}

	// FINS Header (10 bytes)
	finsHeader := []byte{
		0x80,       // ICF: Command, Response required
		0x00,       // RSV: Reserved (Always 00)
		0x02,       // GCT: Gateway count (Assuming direct connection)
		0x00,       // DNA: Destination Network Address (0 if local)
		c.dst.node, // DA1: Destination Node Address (Set this to match your PLC)
		0x00,       // DA2: Destination Unit Address (0 for CPU)
		0x00,       // SNA: Source Network Address (0 if local)
		c.src.node, // SA1: Source Node Address (Client's node number, adjust if needed)
		0x00,       // SA2: Source Unit Address (0 for CPU)
		0x01,       // SID: Service ID (Can be any value, used to match requests)
	}

	// FINS Command (2 bytes) - Controller Status Read (0601)
	command := []byte{0x01, 0x01, 0x82, 0x20, 0x54, 0x00, 0x00, 0x05}

	// Combine all parts into a single packet
	fullPacket := append(finsHeader, command...)

	log.Printf("🔧 Hardcoded Controller Status Read command packet: %+v", fullPacket)
	return fullPacket, nil
}

func (c *Client) TestEndpoints() error {
	// Test REAL data types
	realEndpoints := []struct {
		tag     string
		address uint16
	}{
		{tag: "fanSpeed", address: 8172},
		{tag: "ventilationPortOutput", address: 8230},
		{tag: "ventilationWallOutput", address: 8266},
	}

	// Test Boolean data types
	boolEndpoints := []struct {
		tag     string
		address uint16
		bit     byte
	}{
		{tag: "kilnIsPaused", address: 55, bit: 9},
		{tag: "kilnIsStarted", address: 50, bit: 1},
	}

	// Test reading and writing REAL values
	for _, endpoint := range realEndpoints {
		// Test writing a REAL value

		floatTest := float32(42.5)
		uintTestValue, err := ConvertFloat32ToOmronData(floatTest)
		if err != nil {
			log.Printf("Error in ConvertFloat32ToOmronData(floatTest), where floatTest=%f", floatTest)
		}

		err = c.WriteWords(mapping.MemoryAreaDMWord, endpoint.address, uintTestValue)
		if err != nil {
			log.Printf("failed to write REAL value to %s (address %d): %+v",
				endpoint.tag, endpoint.address, err)
		}
		log.Printf("✅ Successfully wrote value %+v to %s (address %d)",
			uintTestValue, endpoint.tag, endpoint.address)

		// Test reading the value back
		readValue, err := c.ReadWords(mapping.MemoryAreaDMWord, endpoint.address, 2)
		if err != nil {
			log.Printf("failed to read REAL value from %s (address %d): %+v",
				endpoint.tag, endpoint.address, err)
		}
		log.Printf("✅ Successfully read value %+v from %s (address %d)",
			readValue, endpoint.tag, endpoint.address)

		readvalueFloat, _ := ConvertToFloat32(readValue)

		log.Printf("Read value as float32: %f", readvalueFloat)
		// Verify the value was written correctly
		if math.Abs(float64(readvalueFloat-floatTest)) > 0.001 { // Small epsilon for float comparison
			log.Printf("value mismatch for %s: wrote %+v but read %+v",
				endpoint.tag, uintTestValue, readValue)
		}
	}

	// Test reading and writing BOOL values
	for _, endpoint := range boolEndpoints {
		// Test writing a BOOL value (true)
		testValue := true
		data := []bool{testValue}
		err := c.WriteBits(mapping.MemoryAreaHRBit, endpoint.address, endpoint.bit, data)
		if err != nil {
			return fmt.Errorf("failed to write BOOL value to %s (address %d.%d): %w",
				endpoint.tag, endpoint.address, endpoint.bit, err)
		}
		log.Printf("✅ Successfully wrote value %v to %s (address %d.%d)",
			testValue, endpoint.tag, endpoint.address, endpoint.bit)

		// Test reading the value back
		readValue, err := c.ReadBits(mapping.MemoryAreaHRBit, endpoint.address, endpoint.bit, 1)
		if err != nil {
			log.Printf("failed to read BOOL value from %s (address %d.%d): %+v",
				endpoint.tag, endpoint.address, endpoint.bit, err)
		}
		log.Printf("✅ Successfully read value %v from %s (address %d.%d)",
			readValue, endpoint.tag, endpoint.address, endpoint.bit)

		// Verify the value was written correctly
		if readValue[0] != testValue {
			log.Printf("value mismatch for %s: wrote %v but read %v",
				endpoint.tag, testValue, readValue)
		}

		// Test writing the opposite value (false)
		testValue = false
		data = []bool{testValue}
		err = c.WriteBits(mapping.MemoryAreaHRBit, endpoint.address, endpoint.bit, data)
		if err != nil {
			log.Printf("failed to write BOOL value to %s (address %d.%d): %+v",
				endpoint.tag, endpoint.address, endpoint.bit, err)
		}

		// Test reading the value back
		readValue, err = c.ReadBits(mapping.MemoryAreaHRBit, endpoint.address, endpoint.bit, 1)
		if err != nil {
			log.Printf("failed to read BOOL value from %s (address %d.%d): %+v",
				endpoint.tag, endpoint.address, endpoint.bit, err)
		}

		// Verify the value was written correctly
		if readValue[0] != testValue {
			log.Printf("value mismatch for %s: wrote %v but read %v",
				endpoint.tag, testValue, readValue)
		}
	}

	return nil
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
