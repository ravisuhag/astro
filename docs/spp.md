# Space Packet Protocol (SPP)

The `spp` package implements the CCSDS 133.0-B-2 Space Packet Protocol — the fundamental data unit used for transferring application data in space missions.

## Packet Structure

```
+----------------+----------------+----------------+----------------+
| Version (3b)   | Type (1b)      | SecHdrFlag (1b)| APID (11b)     |
+----------------+----------------+----------------+----------------+
| SeqFlags (2b)  | Sequence Count (14b)                            |
+----------------+----------------+----------------+----------------+
| Packet Length (16b)                                               |
+----------------+----------------+----------------+----------------+
| Secondary Header (optional, 1–63 bytes, mission-defined)         |
+----------------+----------------+----------------+----------------+
| User Data Field (variable length)                                |
+----------------+----------------+----------------+----------------+
| Error Control (optional, 16b CRC)                                |
+----------------+----------------+----------------+----------------+
```

A packet must contain at least a secondary header or user data (CCSDS C1/C2). Total packet size: 7–65,542 bytes.

## Creating Packets

Use `NewTMPacket` for telemetry and `NewTCPacket` for telecommand:

```go
// Telemetry packet with APID 100
packet, err := spp.NewTMPacket(100, []byte("temperature=22.5"))

// Telecommand packet with APID 200
packet, err := spp.NewTCPacket(200, []byte("SET_MODE=SAFE"))

// Generic constructor with explicit type
packet, err := spp.NewSpacePacket(100, spp.PacketTypeTM, data)
```

### Packet Options

Options configure optional fields via functional options:

```go
// With error control (CRC-16-CCITT, auto-computed during Encode)
packet, err := spp.NewTMPacket(100, data, spp.WithErrorControl())

// With a mission-specific secondary header
packet, err := spp.NewTMPacket(100, data, spp.WithSecondaryHeader(myHeader))

// With manual sequence count and flags (for packets built outside a Service)
packet, err := spp.NewTMPacket(100, data,
    spp.WithSequenceCount(42),
    spp.WithSequenceFlags(spp.SeqFlagFirstSegment),
)

// Combining options
packet, err := spp.NewTMPacket(100, data,
    spp.WithSecondaryHeader(myHeader),
    spp.WithErrorControl(),
)
```

## Encoding and Decoding

```go
// Encode a packet to bytes for transmission
encoded, err := packet.Encode()

// Decode bytes back into a packet
decoded, err := spp.Decode(encoded)

// Decode with a secondary header decoder
decoded, err := spp.Decode(encoded, spp.WithDecodeSecondaryHeader(&MySecondaryHeader{}))

// Decode with error control (CRC) validation
decoded, err := spp.Decode(encoded, spp.WithDecodeErrorControl())

// Combine decode options
decoded, err := spp.Decode(encoded,
    spp.WithDecodeSecondaryHeader(&MySecondaryHeader{}),
    spp.WithDecodeErrorControl(),
)
```

When decoding a packet that has the secondary header flag set, you can pass a `SecondaryHeader` implementation via `WithDecodeSecondaryHeader`. If none is provided, the secondary header bytes are included in `UserData`.

When `WithDecodeErrorControl()` is used, the trailing 2 bytes are extracted as a CRC-16-CCITT checksum and verified against the packet contents. If the CRC does not match, `ErrCRCValidationFailed` is returned.

## Secondary Headers

The secondary header format is mission-defined. Implement the `SecondaryHeader` interface:

```go
type SecondaryHeader interface {
    Encode() ([]byte, error)
    Decode([]byte) error
    Size() int  // fixed size in bytes (1–63)
}
```

Example implementation:

```go
type TimestampHeader struct {
    Seconds     uint32
    Subseconds  uint16
}

func (h *TimestampHeader) Encode() ([]byte, error) {
    buf := make([]byte, 6)
    binary.BigEndian.PutUint32(buf[0:4], h.Seconds)
    binary.BigEndian.PutUint16(buf[4:6], h.Subseconds)
    return buf, nil
}

func (h *TimestampHeader) Decode(data []byte) error {
    if len(data) < 6 {
        return errors.New("insufficient data for timestamp header")
    }
    h.Seconds = binary.BigEndian.Uint32(data[0:4])
    h.Subseconds = binary.BigEndian.Uint16(data[4:6])
    return nil
}

func (h *TimestampHeader) Size() int { return 6 }
```

## Service Layer

The `Service` type provides two CCSDS-defined service interfaces over an `io.ReadWriter` transport:

- **Packet Service** (CCSDS 3.3) — send and receive pre-built `SpacePacket` values
- **Octet String Service** (CCSDS 3.4) — send and receive raw byte data, with automatic packet wrapping

```go
svc := spp.NewService(conn, spp.ServiceConfig{
    PacketType:      spp.PacketTypeTM,
    MaxPacketLength: 1024,               // optional, defaults to 65542
    SecondaryHeader: &TimestampHeader{},  // optional decoder for inbound packets
    ErrorControl:    true,               // optional, validate CRC on received packets
})
```

### Sequence Counting

The service automatically maintains a per-APID 14-bit sequence counter (per CCSDS 133.0-B-2 Section 4.1.3.5). Each call to `SendPacket` or `SendBytes` stamps the packet with the next count for its APID and wraps at 16383.

### Packet Service

```go
// Send a pre-built packet (sequence count is stamped automatically)
err := svc.SendPacket(packet)

// Receive and decode a packet
packet, err := svc.ReceivePacket()
```

### Octet String Service

```go
// Send raw bytes — packet construction is handled automatically
err := svc.SendBytes(100, []byte("payload data"))

// Send with options
err := svc.SendBytes(100, data,
    spp.WithSendSecondaryHeader(myHeader),
    spp.WithSendErrorControl(),
)

// Receive — returns APID and user data
apid, data, err := svc.ReceiveBytes()
```

## Utilities

```go
// Calculate total packet size from a raw 6-byte header
size, err := spp.CalculatePacketSize(headerBytes)

// Compute CRC-16-CCITT (polynomial 0x1021, initial 0xFFFF)
crc := spp.ComputeCRC(data)

// Check if a packet is an idle packet (APID 0x7FF)
if packet.IsIdle() { ... }

// Human-readable packet dump for debugging
fmt.Println(packet.Humanize())
```

## Constants

```go
spp.PrimaryHeaderSize    // 6 bytes

spp.PacketTypeTM         // 0 — Telemetry
spp.PacketTypeTC         // 1 — Telecommand

spp.SeqFlagContinuation  // 0
spp.SeqFlagFirstSegment  // 1
spp.SeqFlagLastSegment   // 2
spp.SeqFlagUnsegmented   // 3
```

## Errors

All errors are exported package-level variables, suitable for use with `errors.Is`:

| Error | Meaning |
|-------|---------|
| `ErrInvalidVersion` | Version is not 0 |
| `ErrInvalidType` | Type is not 0 or 1 |
| `ErrInvalidAPID` | APID outside 0–2047 |
| `ErrInvalidSequenceFlags` | Sequence flags outside 0–3 |
| `ErrInvalidSequenceCount` | Sequence count outside 0–16383 |
| `ErrInvalidHeader` | Header does not conform to CCSDS |
| `ErrEmptyPacket` | Packet has no secondary header and no user data (C1/C2) |
| `ErrNilPacket` | Nil packet provided |
| `ErrPacketTooLarge` | Total packet size outside 7–65542 bytes |
| `ErrDataTooShort` | Input data too short to decode |
| `ErrPacketLengthMismatch` | Data field size doesn't match header length |
| `ErrSecondaryHeaderMissing` | Flag is set but no secondary header provided |
| `ErrSecondaryHeaderTooSmall` | Secondary header less than 1 byte |
| `ErrSecondaryHeaderTooLarge` | Secondary header exceeds 63 bytes |
| `ErrCRCValidationFailed` | CRC integrity check failed |

## Reference

- [CCSDS 133.0-B-2](https://public.ccsds.org/Pubs/133x0b2e2.pdf) — Space Packet Protocol Blue Book
