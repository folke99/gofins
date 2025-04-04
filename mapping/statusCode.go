package mapping

import "fmt"

// StatusCode represents the operating status of the PC
type StatusCode uint8

const (
	StatusStop    StatusCode = 0x00 // Program not being executed
	StatusRun     StatusCode = 0x01 // Program being executed
	StatusStandby StatusCode = 0x80 // CPU on standby
)

// ModeCode represents the PC operating mode
type ModeCode uint8

const (
	ModeProgram ModeCode = 0x00
	ModeDebug   ModeCode = 0x01
	ModeMonitor ModeCode = 0x02
	ModeRun     ModeCode = 0x04
)

func (s StatusCode) String() string {
	switch s {
	case StatusStop:
		return "STOP"
	case StatusRun:
		return "RUN"
	case StatusStandby:
		return "STANDBY"
	default:
		return fmt.Sprintf("UNKNOWN(0x%02X)", uint8(s))
	}
}

func (m ModeCode) String() string {
	switch m {
	case ModeProgram:
		return "PROGRAM"
	case ModeDebug:
		return "DEBUG"
	case ModeMonitor:
		return "MONITOR"
	case ModeRun:
		return "RUN"
	default:
		return fmt.Sprintf("UNKNOWN(0x%02X)", uint8(m))
	}
}
