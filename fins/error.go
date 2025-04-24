package fins

import (
	"fmt"
	"time"
)

// Client errors
type ResponseTimeoutError struct {
	duration time.Duration
}

func (e ResponseTimeoutError) Error() string {
	return fmt.Sprintf("Response timeout of %d has been reached", e.duration)
}

type IncompatibleMemoryAreaError struct {
	area byte
}

func (e IncompatibleMemoryAreaError) Error() string {
	return fmt.Sprintf("The memory area is incompatible with the data type to be read: 0x%X", e.area)
}

// Driver errors
type BCDBadDigitError struct {
	v   string
	val uint64
}

func (e BCDBadDigitError) Error() string {
	return fmt.Sprintf("Bad digit in BCD decoding: %s = %d", e.v, e.val)
}

type BCDOverflowError struct{}

func (e BCDOverflowError) Error() string {
	return "Overflow occurred in BCD decoding"
}

// BCD encoding/decoding
type BCDError struct {
	msg string
}

func (e BCDError) Error() string {
	return fmt.Sprintf("BCD error: %s", e.msg)
}

// FatalErrorCode represents fatal error information as bit flags
type FatalErrorCode uint16

const (
	ErrorWatchDogTimer FatalErrorCode = 1 << 0  // Watch dog timer error
	ErrorFALS          FatalErrorCode = 1 << 6  // FALS error
	ErrorFatalSFC      FatalErrorCode = 1 << 7  // Fatal SFC error
	ErrorCycleTimeOver FatalErrorCode = 1 << 8  // Cycle time over
	ErrorProgram       FatalErrorCode = 1 << 9  // Program error
	ErrorIOSetting     FatalErrorCode = 1 << 10 // I/O setting error
	ErrorIOOverflow    FatalErrorCode = 1 << 11 // I/O point overflow
	ErrorCPUBus        FatalErrorCode = 1 << 12 // CPU bus error
	ErrorDuplication   FatalErrorCode = 1 << 13 // Duplication error
	ErrorIOBus         FatalErrorCode = 1 << 14 // I/O bus error
	ErrorMemory        FatalErrorCode = 1 << 15 // Memory error
)
