# Space Packet Protocol (SPP) Guide

## Introduction

This guide covers how to use our Space Packet Protocol (SPP) package for spacecraft communications. The SPP package implements the CCSDS 133.0-B-2 standard, providing tools for creating and managing space packets in your satellite communication systems.

## Quick Start

### Creating a Basic Telemetry Packet

```go
import "github.com/ravisuhag/astro/pkg/spp"

// Create a telemetry packet with APID 123
data := []byte("temperature=22.5,pressure=1013.2")
packet, err := spp.NewTMPacket(123, data)
if err != nil {
    log.Fatal(err)
}

// Encode the packet for transmission
encoded, err := packet.Encode()
if err != nil {
    log.Fatal(err)
}
// encoded is now ready for transmission
```

### Creating a Telecommand Packet

```go
// Create a telecommand packet with APID 456
command := []byte("SET_MODE=SAFE")
packet, err := spp.NewTCPacket(456, command)
if err != nil {
    log.Fatal(err)
}
```

## Common Use Cases

### 1. Sensor Data Transmission

When sending sensor data from your satellite:

```go
type SensorData struct {
    Temperature float64
    Pressure    float64
    Timestamp   int64
}

func SendSensorData(data SensorData) error {
    // Convert sensor data to bytes
    payload := fmt.Sprintf("temp=%.2f,press=%.2f,time=%d",
        data.Temperature,
        data.Pressure,
        data.Timestamp,
    )

    // Create telemetry packet with APID 100 (sensor data)
    packet, err := spp.NewTMPacket(100, []byte(payload))
    if err != nil {
        return fmt.Errorf("failed to create packet: %w", err)
    }

    // Add timestamp in secondary header
    secondaryHeader := spp.SecondaryHeader{
        Timestamp: uint64(time.Now().UnixNano()),
    }
    packet, err = spp.NewTMPacket(100, []byte(payload),
        spp.WithSecondaryHeader(secondaryHeader))

    // Encode and send...
    return nil
}
```

### 2. Command Reception

When receiving commands:

```go
func ProcessPacket(rawData []byte) error {
    // Decode the received packet
    packet, err := spp.Decode(rawData)
    if err != nil {
        return fmt.Errorf("failed to decode packet: %w", err)
    }

    // Check if it's a command packet
    if packet.PrimaryHeader.Type != 1 { // Type 1 = TC
        return fmt.Errorf("expected command packet, got type %d",
            packet.PrimaryHeader.Type)
    }

    // Process based on APID
    switch packet.PrimaryHeader.APID {
    case 456: // Command APID
        return processCommand(packet.UserData)
    default:
        return fmt.Errorf("unknown APID: %d", packet.PrimaryHeader.APID)
    }
}
```

### 3. Large Data Transmission

When sending large amounts of data that need to be split across multiple packets:

```go
func SendLargeData(data []byte, apid uint16) error {
    // Maximum data size per packet (minus headers)
    const maxDataSize = 65535 - 6 // 6 bytes for primary header

    // Split data into chunks
    for i := 0; i < len(data); i += maxDataSize {
        end := i + maxDataSize
        if end > len(data) {
            end = len(data)
        }

        chunk := data[i:end]
        var seqFlags uint8

        switch {
        case i == 0 && end == len(data):
            seqFlags = 3 // Standalone packet
        case i == 0:
            seqFlags = 1 // First packet
        case end == len(data):
            seqFlags = 2 // Last packet
        default:
            seqFlags = 0 // Continuation packet
        }

        // Create packet with sequence flags
        packet, err := spp.NewTMPacket(apid, chunk)
        if err != nil {
            return err
        }
        packet.PrimaryHeader.SequenceFlags = seqFlags

        // Encode and send...
    }
    return nil
}
```

## Best Practices

### APID Management

1. **Reserved APIDs**: Keep a registry of APIDs and their purposes
   - 0-63: Reserved for system use
   - 64-127: Telemetry data
   - 128-191: Science data
   - 192-255: Command and control

2. **APID Documentation Example**:
```go
const (
    APID_HOUSEKEEPING   = 64  // Basic spacecraft telemetry
    APID_THERMAL        = 65  // Thermal subsystem data
    APID_POWER         = 66  // Power subsystem data
    APID_ATTITUDE      = 67  // Attitude determination data
    APID_PAYLOAD       = 128 // Science payload data
    APID_COMMAND       = 192 // Command packets
)
```

### Error Handling

Always handle common error cases:

```go
packet, err := spp.NewTMPacket(apid, data)
switch {
case errors.Is(err, spp.ErrInvalidAPID):
    // Handle invalid APID
case errors.Is(err, spp.ErrDataTooLong):
    // Handle data too long
case err != nil:
    // Handle other errors
}
```

### Packet Validation

Always validate received packets:

```go
func validatePacket(packet *spp.SpacePacket) error {
    // Check packet length
    if len(packet.UserData) == 0 {
        return fmt.Errorf("empty packet data")
    }

    // Verify APID is in valid range
    if packet.PrimaryHeader.APID > 2047 {
        return fmt.Errorf("invalid APID")
    }

    // Additional validation...
    return nil
}
```

## Performance Tips

1. **Buffer Reuse**: For high-frequency packet processing, reuse buffers:

```go
// Create a buffer pool
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 65542) // Max packet size
    },
}

func processPacketsEfficiently(data []byte) {
    buffer := bufferPool.Get().([]byte)
    defer bufferPool.Put(buffer)

    // Use buffer for packet processing...
}
```

2. **Batch Processing**: When sending multiple packets, batch them:

```go
func sendPacketBatch(packets []*spp.SpacePacket) error {
    encodedData := make([][]byte, 0, len(packets))

    // Encode all packets first
    for _, packet := range packets {
        encoded, err := packet.Encode()
        if err != nil {
            return err
        }
        encodedData = append(encodedData, encoded)
    }

    // Then send in batch
    return sendBatch(encodedData)
}
```

## Debugging Tips

1. Use the `Humanize()` method for debugging:

```go
packet, _ := spp.NewTMPacket(123, data)
log.Printf("Packet details:\n%s", packet.Humanize())
```

2. Enable packet logging in development:

```go
func logPacket(packet *spp.SpacePacket) {
    if os.Getenv("DEBUG") == "1" {
        log.Printf("APID: %d, Type: %d, Seq: %d, Length: %d",
            packet.PrimaryHeader.APID,
            packet.PrimaryHeader.Type,
            packet.PrimaryHeader.SequenceCount,
            packet.PrimaryHeader.PacketLength)
    }
}
```

## Common Pitfalls

1. **Packet Size Limits**: Don't exceed maximum packet size (65542 bytes)
2. **APID Range**: Ensure APIDs are within valid range (0-2047)
3. **Sequence Counting**: Handle sequence number wraparound correctly
4. **Secondary Headers**: Don't forget to set the secondary header flag when using secondary headers

## Further Reading

- [CCSDS 133.0-B-2](https://public.ccsds.org/Pubs/133x0b2c1.pdf) - Space Packet Protocol
- [ECSS-E-70-41A](https://ecss.nl) - Ground systems and operations
- [Package Documentation](https://godoc.org/github.com/your-org/astro/pkg/spp)
