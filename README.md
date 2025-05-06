# GoFINS ala Folke

![Go Report Card](https://goreportcard.com/badge/github.com/folke99/gofins)
![License](https://img.shields.io/github/license/folke99/gofins.svg)

This is a TCP implementation of the FINS protocol in golang. It can be used for secure and robust communication with omron PLCs. It has used l1va's GoFINS library written for UDP communication as a base for a new TCP version.

## Installation

```sh
go get github.com/folke99/gofins
```

## Quick Start

Here is an example of a bare bones client creation and ReadClock()

```go
func main() {
    PLCPort := 1234
    localPort := 1234
    client, err := Connect(5000, "<YOUR_PLC_IP>", PLCPort, "YOUR_LOCAL_IP", localPort) //fins.NewClient(clientAddr, plcAddr)
	if err != nil {
		log.Printf("Connection failed: %v", err)
		time.Sleep(2 * time.Second)
	}
    result, _ := gofins.fins.ReadClock()
    fmt.Println("Result:", result)
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

	c, err := fins.NewClient(cAddr, pAddr)
	if err != nil {
		return nil, fmt.Errorf("could not create fins client: %+v", err)
	}

	c.SetTimeoutMs(uint(timeout))

	return c, nil
}
```

You also have to configure your PLC to have a open TCP/FINS port

## Features

-Package delivery guarantee (TCP handshake)

-Robust error handling

## API Documentation

### `SetByteOrder(o binary.ByteOrder)`
Sets byte order. Default is binary.BigEndian
### `SetBit(memoryArea byte, address uint16, bitOffset byte) error`
Sets a bit in the PLC data area
### `ResetBit(memoryArea byte, address uint16, bitOffset byte) error`
resets a bit in the PLC data area
### `ToggleBit(memoryArea byte, address uint16, bitOffset byte) error`
Toggles a bit in the plc data area
### `NewClient(localAddr, plcAddr Address) (*Client, error)`
Creates a new FINS client and return it
### `SetTimeout(t uint)`
Sets a response timeout (ms)
Default value: 20ms
If set to zero it will block indefinately
### `SetKeepAlive(enabled bool, interval time.Duration) error`
Enables keepalive with the specified interval
### `Reconnect() error`
Closes the old connection and recreates it, then restart the listenloop()
### `Ping() error`
Sends a ReadClock() command to check PLC availability
### `Status() (*PLCStatus, error)`
Reads the status from the PLC returning:
```
PLCStatus{
    Status
    Mode
    FatalError
}
```
### `IsRunning() bool`
Checks status and returns a bool of if it is running
### `IsStandby() bool`
Checks status and returns a bool of if it is in standby
### `IsStopped() bool`
Checks status and returns a bool of if it is stopped
### `HasFatalError() bool`
Checks status and returns a bool of if it is has fatal errors
### `HasError() bool`
Checks status and returns a bool of if it has any non fatal errors
### `ReadWords(memoryArea byte, address uint16, readCount uint16) ([]uint16, error)`
Reads words from the PLC data area
### `ReadBytes(memoryArea byte, address uint16, byteCount uint16) ([]byte, error)`
Reads bytes from the PLC data area
### `ReadString(memoryArea byte, address uint16, byteCount uint16) (string, error)`
reads a string from the PLC's DM memory area
### `ReadBits(memoryArea byte, address uint16, bitOffset byte, readCount uint16) ([]bool, error)`
Reads bits from the PLC data area
### `ReadPLCStatus() (*Response, error)`
Reads the status from the PLC and returns a byte response of the format:
```
Response {
    header      Header
    commandCode uint16
    endCode     uint16
    data        []byte
}
```
### `ReadClock() (*time.Time, error)`
Returns the PLC clock time and returns in time.Time format
### `WriteWords(memoryArea byte, address uint16, data []uint16) error`
Writes words to the PLC data area
### `WriteString(memoryArea byte, address uint16, s string) error`
Writes a string to the PLC data area
### `WriteByte(memoryArea byte, address uint16, b []byte) error`
Writes bytes to the PLC data area
### `WriteBits(memoryArea byte, address uint16, bitOffset byte, data []bool) error`
Writes bits to the PLC data area

For full documentation, visit [pkg.go.dev](https://pkg.go.dev/github.com/folke99/gofins).


## Protocol Documentation
The FINS protocol works by sending specific byte arrays to the plc which will then issue a response, either with data or with a confirmation byte array. Worth noting that the UDP and TCP protocol implementation differ somewhat from eachother, for UDP documentation and implementation i reffer to [l1va's GoFINS library](https://github.com/l1va/gofins).

To create the initial connection with the TCP/FINS it is required to first send a handshake frame with the following structure:

```go
	initFrame := []byte{
		0x46, 0x49, 0x4E, 0x53, // "FINS"
		0x00, 0x00, 0x00, byte(length), // Length
		0x00, 0x00, 0x00, byte(commandCode), // Command
		0x00, 0x00, 0x00, 0x00, // Error code
		0x00, 0x00, 0x00, 0x00 // Client node address (0 = auto-assign)
	}
```

With this we should be able to establish a TCP/FINS connection.

To send commands on this connection we need to follow the following flow.

1. Send the init frame (without the Client node address bytes)
```go
	initFrame := []byte{
		0x46, 0x49, 0x4E, 0x53, // "FINS"
		0x00, 0x00, 0x00, byte(length), // Length
		0x00, 0x00, 0x00, byte(commandCode), // Command
		0x00, 0x00, 0x00, 0x00, // Error code
	}
```
2. Without fetching a response from the init frame, send the command frame composed of header + command
```go
	return Header{
		icf: 0x80,
		rsv: DefaultReserved,
		gct: DefaultGatewayCount,
		dna: dst.network,
		da1: dst.node,
		da2: dst.unit,
		sna: src.network,
		sa1: src.node,
		sa2: src.unit,
		sid: serviceID,
	}
	commandBytes := []byte{0x06, 0x01}

	fullFrame := header + command bytes
```

Here is an example of a full send command packet:
```go
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
	command := []byte{0x06, 0x01}

	// Combine all parts into a single packet
	fullPacket := append(finsHeader, command...)


```

3. alocate a response channel and send it on the TCP connection.

```go
	c.resp[header.sid] = responseChan

	_, err := c.conn.Write(fullPacket)
```

We should then get a reponse on the response channel. The response could look something like this:
```
46494E53000000100000000100000000000000EF00000020
```

following this structure:
![alt text](image.png)

Read more about command and response codes in the official documentation.


For full documentation, visit [Omron Manual](https://www.myomron.com/downloads/1.Manuals/Networks/W227E12_FINS_Commands_Reference_Manual.pdf)
### Testing
All testing and verification has been done with the PLC models:
* Omron CJ2M-CPU32
* Omron CJ2H-CPU64

The client have been using Debian GNU/Linux 11 (bullseye)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.