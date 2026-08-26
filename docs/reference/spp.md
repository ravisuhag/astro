# Space Packet Protocol (SPP)

The `spp` package implements the CCSDS 133.0-B-2 Space Packet Protocol — the fundamental data unit used for transferring application data in space missions.

## Quick Start

```go
// Create a service over any io.ReadWriter (TCP conn, serial port, etc.)
svc := spp.NewService(conn, spp.ServiceConfig{
    PacketType: spp.PacketTypeTM,
})

// Send raw bytes — packet construction is handled automatically
err := svc.SendBytes(100, []byte("temperature=22.5"))

// Receive — returns the OCTET_STRING.indication parameters
ind, err := svc.ReceiveBytes()
fmt.Println(ind.APID, ind.Data, ind.SecondaryHeaderIndicator, ind.DataLoss)
```

## Service Layer

The `Service` type provides two CCSDS-defined service interfaces over an `io.ReadWriter` transport:

- **Packet Service** (CCSDS 3.3) — send and receive pre-built `SpacePacket` values
- **Octet String Service** (CCSDS 3.4) — send and receive raw byte data, with automatic packet wrapping

```go
svc := spp.NewService(conn, spp.ServiceConfig{
    PacketType:      spp.PacketTypeTM,
    MaxPacketLength: 1024, // optional, defaults to 65542
    ErrorControl:    true, // optional, validate CRC on received packets
    DiscardIdle:     true, // optional, drop received idle packets (APID 0x7FF)

    // Optional decoder for inbound secondary headers. It is a factory, not a
    // single value: every received packet gets its own header, so one packet's
    // values are never overwritten by the next one's.
    NewSecondaryHeader: func() spp.SecondaryHeader { return &TimestampHeader{} },
})
```

### Octet String Service

The simplest way to send and receive data. The service wraps your bytes in a valid space packet automatically:

```go
// Send raw bytes
err := svc.SendBytes(100, []byte("payload data"))

// Send with all the request parameters of CCSDS 3.4.3.2.2
err := svc.SendBytes(100, data,
    spp.WithSendSecondaryHeader(myHeader),   // Secondary Header Indicator
    spp.WithSendPacketType(spp.PacketTypeTC), // Packet Type
    spp.WithSendSequenceCount(42),            // Packet Sequence Count
    spp.WithSendErrorControl(),
)

// Receive — returns the indication parameters of CCSDS 3.4.3.3.2
ind, err := svc.ReceiveBytes()
```

`ReceiveBytes` returns an `Indication`:

| Field | Meaning |
|-------|---------|
| `Data` | The octet string |
| `APID` | The managed data path it arrived on |
| `SecondaryHeaderIndicator` | Whether the packet carried a Packet Secondary Header |
| `DataLoss` | Whether the sequence count for this APID skipped ahead |
| `PacketsLost` | How many packets the count skipped |

### Packet Service

For full control over the packet structure, build a `SpacePacket` and send it directly:

```go
// Send a pre-built packet (sequence count is stamped automatically)
err := svc.SendPacket(packet)

// Receive and decode a packet
packet, err := svc.ReceivePacket()
```

### Sequence Counting

The service automatically maintains a per-APID 14-bit sequence counter (CCSDS 133.0-B-2 4.1.3.4.3). Each call to `SendPacket` or `SendBytes` stamps the packet with the next count for its APID and wraps at 16383.

Pinning a count with `WithSequenceCount` (or `WithSendSequenceCount`) does not break the run: the service resynchronizes its counter to one past the pinned value, so the APID's count stays continuous as 4.1.3.4.3.4 requires.

### Loss Detection

Every received packet is checked for sequence count continuity on its APID, modulo 16384 (CCSDS 4.3.2.2). `ReceiveBytes` reports the result on the `Indication`; after `ReceivePacket`, read it with `LastDataLoss()`:

```go
packet, err := svc.ReceivePacket()
if lost := svc.LastDataLoss(); lost > 0 {
    log.Printf("%d packet(s) lost before this one", lost)
}

// After a link outage, the gap across the break means nothing.
svc.ResetContinuity()
```

The first packet seen on an APID never reports a loss.

## Creating Packets

For use cases outside the Service layer (testing, offline encoding, custom transports), construct packets directly:

```go
// Telemetry packet with APID 100
packet, err := spp.NewTMPacket(100, []byte("temperature=22.5"))

// Telecommand packet with APID 200
packet, err := spp.NewTCPacket(200, []byte("SET_MODE=SAFE"))

// Generic constructor with explicit type
packet, err := spp.NewSpacePacket(100, spp.PacketTypeTM, data)
```

### Packet Options

Options configure optional fields:

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

### Inspecting Packets

```go
// Check if a packet is an idle packet (APID 0x7FF)
if packet.IsIdle() { ... }

// Same question about a raw encoded packet
if spp.IsIdleBytes(raw) { ... }

// Human-readable dump for debugging
fmt.Println(packet.Humanize())
```

### Packet Sizing

Two functions read a packet's length, and which one you want depends on whether you already hold the octets.

`PacketSizer` returns the length of the **complete** packet at the front of a buffer, and -1 when the buffer does not hold all of it. That is what the data link packet services need: a packet reaching past the end of the reassembly buffer is not a packet yet, so -1 tells them to pull another frame. Wire it up with `SetPacketSizer`:

```go
vcp.SetPacketSizer(spp.PacketSizer)

n := spp.PacketSizer(buf)
if n > 0 {
    packet := buf[:n] // safe: the whole packet is there
}
```

`DeclaredPacketSize` returns what the 6-octet primary header claims, whether or not the body has arrived. A stream reader needs this, since it must know how many octets to fetch before it has them:

```go
total := spp.DeclaredPacketSize(header) // header need only be 6 octets
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

When decoding a packet that has the secondary header flag set, you can pass a `SecondaryHeader` implementation via `WithDecodeSecondaryHeader`. If none is provided, the secondary header bytes are included in `UserData`; the packet still re-encodes byte for byte, so a relay can forward a packet whose secondary header format it does not know.

The Secondary Header Flag is the only signal that a header is present (CCSDS 4.1.3.3.3.2), so it decides both whether the header's octets are written and whether its size counts towards the Packet Data Length. `Encode` and `Validate` refuse a packet where the flag and the `SecondaryHeader` field disagree.

When `WithDecodeErrorControl()` is used, the trailing 2 bytes are extracted as a CRC-16-CCITT checksum and verified against the packet contents. If the CRC does not match, `ErrCRCValidationFailed` is returned.

## Secondary Headers

The secondary header's contents and length are mission-defined (CCSDS 4.1.4.2.1.4). Its layout is not free-form: 4.1.4.2.1.5 allows a Time Code Field alone, an Ancillary Data Field alone, or a Time Code Field followed by an Ancillary Data Field, and 4.1.4.2.1.6 requires the same choice throughout a managed data path's life. The interface sees only octets, so keeping to one of those three shapes is your implementation's job.

Implement the `SecondaryHeader` interface:

```go
type SecondaryHeader interface {
    Encode() ([]byte, error)
    Decode([]byte) error
    Size() int  // fixed size in bytes (at least 1)
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

## Packet Structure

```
+----------------+----------------+----------------+----------------+
| Version (3b)   | Type (1b)      | SecHdrFlag (1b)| APID (11b)     |
+----------------+----------------+----------------+----------------+
| SeqFlags (2b)  | Sequence Count (14b)                            |
+----------------+----------------+----------------+----------------+
| Packet Length (16b)                                               |
+----------------+----------------+----------------+----------------+
| Secondary Header (optional, mission-defined length)               |
+----------------+----------------+----------------+----------------+
| User Data Field (variable length)                                |
+----------------+----------------+----------------+----------------+
| Error Control (optional, 16b CRC)                                |
+----------------+----------------+----------------+----------------+
```

A packet must contain at least a secondary header or user data (CCSDS C1/C2). Total packet size: 7–65,542 bytes.

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
| `ErrSecondaryHeaderMissing` | Flag is set on a hand-built packet but no secondary header provided |
| `ErrSecondaryHeaderFlagClear` | A secondary header is attached but the flag is 0 |
| `ErrSecondaryHeaderTooSmall` | Secondary header less than 1 byte |
| `ErrSecondaryHeaderSizeMismatch` | `Encode()` returned a byte count different from `Size()` |
| `ErrSecondaryHeaderExceedsDataField` | The configured decoder is wider than the packet's data field |
| `ErrIdleWithSecondaryHeader` | An idle packet (APID 0x7FF) carries a secondary header |
| `ErrCRCValidationFailed` | CRC integrity check failed |

## Reference

- [CCSDS 133.0-B-2](https://public.ccsds.org/Pubs/133x0b2e2.pdf) — Space Packet Protocol Blue Book
