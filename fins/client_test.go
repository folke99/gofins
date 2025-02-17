package fins

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


func setupTest(t *testing.T) (*Client, *Server, func()) {
	clientAddr, err := NewAddress("0.0.0.0", 9600, 0, 2, 0)
	require.NoError(t, err)

	plcAddr, err := NewAddress("0.0.0.0", 9601, 0, 10, 0)
	require.NoError(t, err)

	s, err := NewPLCSimulator(plcAddr)
	require.NoError(t, err)

	c, err := NewClient(clientAddr, plcAddr)
	require.NoError(t, err)

	cleanup := func() {
		c.Close()
		s.Close()
	}

	return c, s, cleanup
}

func TestFINSProtocolImplementation(t *testing.T) {
	c, _, cleanup := setupTest(t)
	defer cleanup()

	t.Run("Word Operations", func(t *testing.T) {
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
				err := c.WriteWords(MemoryAreaDMWord, tc.address, tc.values)
				require.NoError(t, err, "Failed to write words")

				readValues, err := c.ReadWords(MemoryAreaDMWord, tc.address, uint16(len(tc.values)))
				require.NoError(t, err, "Failed to read words")

				assert.Equal(t, tc.values, readValues, "Word values do not match after write and read")
			})
		}
	})

	t.Run("Bit Operations", func(t *testing.T) {
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
				err := c.WriteBits(MemoryAreaDMBit, tc.address, tc.bitOffset, tc.values)
				require.NoError(t, err, "Failed to write bits")

				readValues, err := c.ReadBits(MemoryAreaDMBit, tc.address, tc.bitOffset, uint16(len(tc.values)))
				require.NoError(t, err, "Failed to read bits")

				assert.Equal(t, tc.values, readValues, "Bit values do not match after write and read")
			})
		}
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
			{"Long String", 80, "This is a longer string to test buffer handling"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := c.WriteString(MemoryAreaDMWord, tc.address, tc.value)
				require.NoError(t, err, "Failed to write string")

				readValue, err := c.ReadString(MemoryAreaDMWord, tc.address, uint16(len(tc.value)+1))
				require.NoError(t, err, "Failed to read string")

				assert.Equal(t, tc.value, readValue, "String values do not match after write and read")
			})
		}
	})
}

func TestTCPSpecificFeatures(t *testing.T) {
	c, _, cleanup := setupTest(t)
	defer cleanup()

	t.Run("KeepAlive", func(t *testing.T) {
		err := c.SetKeepAlive(true, 30*time.Second)
		require.NoError(t, err, "Failed to set keep-alive")
	})

	t.Run("Connection Management", func(t *testing.T) {
		// Test graceful close
		c.Close()
		_, err := c.ReadWords(MemoryAreaDMWord, 100, 5)
		assert.Error(t, err, "Should error on closed connection")
	})
}

func TestErrorHandling(t *testing.T) {
	c, _, cleanup := setupTest(t)
	defer cleanup()

	t.Run("Invalid Memory Area", func(t *testing.T) {
		_, err := c.ReadWords(0xFF, 100, 5)
		assert.Error(t, err)
		assert.IsType(t, IncompatibleMemoryAreaError{}, err)
	})

	// // when it runs localy timouts wont trigger like this.
	// t.Run("Timeout Handling", func(t *testing.T) {
	// 	// Set an extremely short timeout
	// 	c.SetTimeoutMs(1)

	// 	// Perform a large read operation that should timeout
	// 	_, err := c.ReadWords(MemoryAreaDMWord, 100, 1000)
	// 	assert.Error(t, err, "Expected timeout error")

	// 	// Reset timeout
	// 	c.SetTimeoutMs(20)

	// 	// Verify normal operation resumes after timeout reset
	// 	_, err = c.ReadWords(MemoryAreaDMWord, 100, 5)
	// 	assert.NoError(t, err, "Should work normally after timeout reset")
	// })

	t.Run("Write With Invalid Length", func(t *testing.T) {
		err := c.WriteBytes(MemoryAreaDMWord, 100, []byte{1}) // Single byte is invalid
		assert.Error(t, err, "Should error on odd byte length")
	})
}

func TestConcurrentAccess(t *testing.T) {
	c, _, cleanup := setupTest(t)
	defer cleanup()

	var wg sync.WaitGroup
	errors := make(chan error, 100)
	concurrentOps := 10

	for i := 0; i < concurrentOps; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// Write operation
			err := c.WriteWords(MemoryAreaDMWord, uint16(i*10), []uint16{1, 2, 3})
			if err != nil {
				errors <- err
				return
			}

			// Read operation
			_, err = c.ReadWords(MemoryAreaDMWord, uint16(i*10), 3)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}
}

func TestEdgeCases(t *testing.T) {
	c, _, cleanup := setupTest(t)
	defer cleanup()

	t.Run("Maximum Packet Size", func(t *testing.T) {
		largeSize := uint16(MAX_PACKET_SIZE / 2) // Each word is 2 bytes
		_, err := c.ReadWords(MemoryAreaDMWord, 0, largeSize)
		assert.Error(t, err, "Should handle large packet size appropriately")
	})

	t.Run("Zero Length Operations", func(t *testing.T) {
		err := c.WriteWords(MemoryAreaDMWord, 100, []uint16{})
		assert.Error(t, err, "Should handle zero length write appropriately")

		_, err = c.ReadWords(MemoryAreaDMWord, 100, 0)
		assert.Error(t, err, "Should handle zero length read appropriately")
	})
}

// func BenchmarkOperations(b *testing.B) {
// 	c, _, cleanup := setupTest(b)
// 	defer cleanup()

// 	b.Run("WriteWords", func(b *testing.B) {
// 		data := []uint16{1, 2, 3, 4, 5}
// 		b.ResetTimer()
// 		for i := 0; i < b.N; i++ {
// 			err := c.WriteWords(MemoryAreaDMWord, 100, data)
// 			require.NoError(b, err)
// 		}
// 	})

// 	b.Run("ReadWords", func(b *testing.B) {
// 		b.ResetTimer()
// 		for i := 0; i < b.N; i++ {
// 			_, err := c.ReadWords(MemoryAreaDMWord, 100, 5)
// 			require.NoError(b, err)
// 		}
// 	})
// }
