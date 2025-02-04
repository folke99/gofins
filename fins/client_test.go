package fins

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinsClient(t *testing.T) {
	clientAddr, err := NewAddress("0.0.0.0", 9600, 0, 2, 0)
	if err != nil {
		fmt.Printf("Error creating Client Address %f", err)
	}
	plcAddr, err := NewAddress("0.0.0.0", 9601, 0, 10, 0)
	if err != nil {
		fmt.Printf("Error creating PLC Address %f", err)
	}
	toWrite := []uint16{5, 4, 3, 2, 1}

	s, e := NewPLCSimulator(plcAddr)
	if e != nil {
		panic(e)
	}
	defer s.Close()

	c, e := NewClient(clientAddr, plcAddr)
	if e != nil {
		panic(e)
	}
	defer c.Close()

	// ------------- Test Words
	err = c.WriteWords(MemoryAreaDMWord, 100, toWrite)
	assert.Nil(t, err)

	vals, err := c.ReadWords(MemoryAreaDMWord, 100, 5)
	assert.Nil(t, err)
	assert.Equal(t, toWrite, vals)

	// test setting response timeout
	c.SetTimeoutMs(50)
	_, err = c.ReadWords(MemoryAreaDMWord, 100, 5)
	assert.Nil(t, err)

	// ------------- Test Strings
	err = c.WriteString(MemoryAreaDMWord, 10, "ф1234")
	assert.Nil(t, err)

	v, err := c.ReadString(MemoryAreaDMWord, 12, 1)
	assert.Nil(t, err)
	assert.Equal(t, "12", v)

	v, err = c.ReadString(MemoryAreaDMWord, 10, 3)
	assert.Nil(t, err)
	assert.Equal(t, "ф1234", v)

	v, err = c.ReadString(MemoryAreaDMWord, 10, 5)
	assert.Nil(t, err)
	assert.Equal(t, "ф1234", v)

	// ------------- Test Bytes
	err = c.WriteBytes(MemoryAreaDMWord, 10, []byte{0x00, 0x00, 0xC1, 0xA0})
	assert.Nil(t, err)

	b, err := c.ReadBytes(MemoryAreaDMWord, 10, 2)
	assert.Nil(t, err)
	assert.Equal(t, []byte{0x00, 0x00, 0xC1, 0xA0}, b)

	buf := make([]byte, 8, 8)
	binary.LittleEndian.PutUint64(buf[:], math.Float64bits(-20))
	err = c.WriteBytes(MemoryAreaDMWord, 10, buf)
	assert.Nil(t, err)

	b, err = c.ReadBytes(MemoryAreaDMWord, 10, 4)
	assert.Nil(t, err)
	assert.Equal(t, []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x34, 0xc0}, b)

	// ------------- Test Bits
	err = c.WriteBits(MemoryAreaDMBit, 10, 2, []bool{true, false, true})
	assert.Nil(t, err)

	bs, err := c.ReadBits(MemoryAreaDMBit, 10, 2, 3)
	assert.Nil(t, err)
	assert.Equal(t, []bool{true, false, true}, bs)

	bs, err = c.ReadBits(MemoryAreaDMBit, 10, 1, 5)
	assert.Nil(t, err)
	assert.Equal(t, []bool{false, true, false, true, false}, bs)

}

func TestMessageFraming(t *testing.T) {
	clientAddr, err := NewAddress("0.0.0.0", 9600, 0, 2, 0)
	if err != nil {
		fmt.Printf("Error creating Client Address %f", err)
	}
	plcAddr, err := NewAddress("0.0.0.0", 9601, 0, 10, 0)
	if err != nil {
		fmt.Printf("Error creating PLC Address %f", err)
	}
	// Create a simulated TCP connection
	server, err := NewPLCSimulator(plcAddr)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	client, err := NewClient(clientAddr, plcAddr)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Try reading some data to test full communication cycle
	_, err = client.ReadWords(MemoryAreaDMWord, 0, 10)
	if err != nil {
		t.Errorf("Failed to read words: %v", err)
	}
}

func TestFinsClient2(t *testing.T) {
	log.SetOutput(os.Stdout) // Ensure logging is visible
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	clientAddr, err := NewAddress("0.0.0.0", 9600, 0, 2, 0)
	require.NoError(t, err)

	plcAddr, err := NewAddress("0.0.0.0", 9601, 0, 10, 0)
	require.NoError(t, err)

	s, err := NewPLCSimulator(plcAddr)
	require.NoError(t, err)
	defer s.Close()

	// Give server a moment to start
	time.Sleep(100 * time.Millisecond)

	c, err := NewClient(clientAddr, plcAddr)
	require.NoError(t, err)
	defer c.Close()

	// Test Words
	toWrite := []uint16{5, 4, 3, 2, 1}
	err = c.WriteWords(MemoryAreaDMWord, 100, toWrite)
	require.NoError(t, err, "Failed to write words")

	vals, err := c.ReadWords(MemoryAreaDMWord, 100, 5)
	require.NoError(t, err, "Failed to read words")

	// Detailed comparison with logging
	for i := range toWrite {
		assert.Equal(t, toWrite[i], vals[i],
			fmt.Sprintf("Mismatch at index %d: wrote %d, read %d", i, toWrite[i], vals[i]))
	}
}

func TestFINSProtocolImplementation(t *testing.T) {
	// Setup addresses
	clientAddr, err := NewAddress("0.0.0.0", 9600, 0, 2, 0)
	require.NoError(t, err)

	plcAddr, err := NewAddress("0.0.0.0", 9601, 0, 10, 0)
	require.NoError(t, err)

	// Start PLC Simulator
	s, err := NewPLCSimulator(plcAddr)
	require.NoError(t, err)
	defer s.Close()

	// Create client
	c, err := NewClient(clientAddr, plcAddr)
	require.NoError(t, err)
	defer c.Close()

	t.Run("Word Operations", func(t *testing.T) {
		// Test writing and reading various word patterns
		testCases := []struct {
			name    string
			address uint16
			values  []uint16
		}{
			{"Sequential Increasing", 100, []uint16{1, 2, 3, 4, 5}},
			{"Sequential Decreasing", 200, []uint16{5, 4, 3, 2, 1}},
			{"Zero Values", 300, []uint16{0, 0, 0, 0, 0}},
			{"Large Values", 400, []uint16{0xFFFF, 0x8000, 0x7FFF, 0x0001, 0xFFFE}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Write words
				err := c.WriteWords(MemoryAreaDMWord, tc.address, tc.values)
				require.NoError(t, err, "Failed to write words")

				// Read words
				readValues, err := c.ReadWords(MemoryAreaDMWord, tc.address, uint16(len(tc.values)))
				require.NoError(t, err, "Failed to read words")

				// Compare
				assert.Equal(t, tc.values, readValues, "Word values do not match after write and read")
			})
		}
	})

	t.Run("Bit Operations", func(t *testing.T) {
		// Test bit manipulation scenarios
		testCases := []struct {
			name      string
			address   uint16
			bitOffset byte
			values    []bool
		}{
			{"Alternating Bits", 10, 2, []bool{true, false, true, false, true}},
			{"All True", 20, 3, []bool{true, true, true, true, true}},
			{"All False", 30, 4, []bool{false, false, false, false, false}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Write bits
				err := c.WriteBits(MemoryAreaDMBit, tc.address, tc.bitOffset, tc.values)
				require.NoError(t, err, "Failed to write bits")

				// Read bits
				readValues, err := c.ReadBits(MemoryAreaDMBit, tc.address, tc.bitOffset, uint16(len(tc.values)))
				require.NoError(t, err, "Failed to read bits")

				// Compare
				assert.Equal(t, tc.values, readValues, "Bit values do not match after write and read")
			})
		}

		t.Run("Bit Manipulation Methods", func(t *testing.T) {
			address := uint16(40)
			bitOffset := byte(3)

			// Initial state
			err := c.WriteBits(MemoryAreaDMBit, address, bitOffset, []bool{false})
			require.NoError(t, err)

			// Set bit
			err = c.SetBit(MemoryAreaDMBit, address, bitOffset)
			require.NoError(t, err)

			bits, err := c.ReadBits(MemoryAreaDMBit, address, bitOffset, 1)
			require.NoError(t, err)
			assert.True(t, bits[0], "Bit should be set")

			// Reset bit
			err = c.ResetBit(MemoryAreaDMBit, address, bitOffset)
			require.NoError(t, err)

			bits, err = c.ReadBits(MemoryAreaDMBit, address, bitOffset, 1)
			require.NoError(t, err)
			assert.False(t, bits[0], "Bit should be reset")

			// Toggle bit
			err = c.ToggleBit(MemoryAreaDMBit, address, bitOffset)
			require.NoError(t, err)

			bits, err = c.ReadBits(MemoryAreaDMBit, address, bitOffset, 1)
			require.NoError(t, err)
			assert.True(t, bits[0], "Bit should be toggled to true")
		})
	})

	t.Run("String Operations", func(t *testing.T) {
		testCases := []struct {
			name    string
			address uint16
			value   string
		}{
			{"Simple ASCII", 50, "Hello"},
			{"Mixed Characters", 60, "Test123"},
			{"Empty String", 70, ""},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Write string
				err := c.WriteString(MemoryAreaDMWord, tc.address, tc.value)
				require.NoError(t, err, "Failed to write string")

				// Read string
				readValue, err := c.ReadString(MemoryAreaDMWord, tc.address, uint16(len(tc.value)+1))
				require.NoError(t, err, "Failed to read string")

				// Compare
				assert.Equal(t, tc.value, readValue, "String values do not match after write and read")
			})
		}
	})

	t.Run("Byte Operations", func(t *testing.T) {
		testCases := []struct {
			name    string
			address uint16
			values  []byte
		}{
			{"Simple Bytes", 80, []byte{0x01, 0x02, 0x03, 0x04}},
			{"Zero Bytes", 90, []byte{0x00, 0x00, 0x00, 0x00}},
			{"Max Bytes", 100, []byte{0xFF, 0xFF, 0xFF, 0xFF}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Write bytes
				err := c.WriteBytes(MemoryAreaDMWord, tc.address, tc.values)
				require.NoError(t, err, "Failed to write bytes")

				// Read bytes
				readValues, err := c.ReadBytes(MemoryAreaDMWord, tc.address, uint16(len(tc.values)))
				require.NoError(t, err, "Failed to read bytes")

				// Compare
				assert.Equal(t, tc.values, readValues, "Byte values do not match after write and read")
			})
		}
	})

	//Clock not supported with simmulator
	// t.Run("Clock Operations", func(t *testing.T) {
	// 	// Read clock
	// 	clockTime, err := c.ReadClock()
	// 	require.NoError(t, err, "Failed to read clock")

	// 	// Validate basic clock properties
	// 	assert.NotNil(t, clockTime, "Clock time should not be nil")
	// 	assert.True(t, clockTime.Year() > 2000, "Year should be reasonable")
	// 	assert.True(t, clockTime.Year() < 2100, "Year should be reasonable")
	// 	assert.True(t, int(clockTime.Month()) > 0, "Month should be valid")
	// 	assert.True(t, int(clockTime.Month()) <= 12, "Month should be valid")
	// 	assert.True(t, clockTime.Day() > 0, "Day should be valid")
	// 	assert.True(t, clockTime.Day() <= 31, "Day should be valid")
	// 	assert.True(t, clockTime.Hour() >= 0, "Hour should be valid")
	// 	assert.True(t, clockTime.Hour() < 24, "Hour should be valid")
	// })

	t.Run("Timeout and Response Handling", func(t *testing.T) {
		// Test timeout setting
		//originalTimeout := c.responseTimeoutMs
		c.SetTimeoutMs(50) // Short timeout

		// Attempt to read with short timeout
		_, err := c.ReadWords(MemoryAreaDMWord, 100, 5)
		require.NoError(t, err, "Should handle short timeout gracefully")

		// Restore original timeout
		c.SetTimeoutMs(50)
	})
}
