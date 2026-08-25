# Encapsulation Packet Protocol (EPP)

The `epp` package implements the CCSDS 133.1-B-3 Encapsulation Packet Protocol — a lightweight encapsulation mechanism for carrying network-layer protocol data units (PDUs) over CCSDS space links.

## Quick Start

```go
// Create a service over any io.ReadWriter (TCP conn, serial port, etc.)
svc := epp.NewService(conn, epp.ServiceConfig{})

// Send raw bytes — packet construction is handled automatically
err := svc.SendBytes(epp.ProtocolIDIPE, ipv4Datagram)

// Receive — returns Protocol ID and data
pid, data, err := svc.ReceiveBytes()
```

## Overview

Unlike the Space Packet Protocol (SPP), which provides APID-based routing, sequence counting, and segmentation, EPP is a thin encapsulation shim. It wraps a payload with a minimal variable-length header that identifies the encapsulated protocol. This makes it well suited for carrying IP datagrams and other network-layer PDUs that have their own addressing and sequencing.

The first three bits of an Encapsulation Packet are always `111` (PVN = 7), which distinguishes it from a Space Packet (PVN = 0). This allows both packet types to coexist on the same data link. The 1-octet idle packet is the single byte `0xE0`.

## Service Layer

The `Service` type provides send/receive operations over an `io.ReadWriter` transport:

```go
svc := epp.NewService(conn, epp.ServiceConfig{
    MaxPacketLength: 65535, // optional; defaults to 4,294,967,295 (no spec-valid packet rejected)
})
```

### Byte-Level Service

The simplest way to send and receive data. The service wraps your bytes in a valid encapsulation packet automatically:

```go
// Send an IPv4 datagram
err := svc.SendBytes(epp.ProtocolIDIPE, ipv4Datagram)

// Send with an extended protocol ID
err := svc.SendBytes(epp.ProtocolIDExtended, data,
    epp.WithExtendedProtocolID(9),
)

// Receive — returns Protocol ID and data zone
pid, data, err := svc.ReceiveBytes()
```

### Packet-Level Service

For full control over the packet structure, build an `EncapsulationPacket` and send it directly:

```go
// Send a pre-built packet
err := svc.SendPacket(packet)

// Receive and decode a packet
packet, err := svc.ReceivePacket()
```

## Creating Packets

For use cases outside the Service layer (testing, offline encoding, custom transports), construct packets directly:

```go
// Internet Protocol Extension packet (carries IPv4/IPv6)
packet, err := epp.NewIPEPacket(ipDatagram)

// LTP packet
packet, err := epp.NewLTPPacket(segment)

// Mission-specific (privately defined) packet
packet, err := epp.NewMissionPacket(payload)

// 1-octet idle packet (0xE0)
packet, err := epp.NewIdlePacket()

// Idle fill packet of an exact total size (for frame fill)
packet, err := epp.NewIdleFillPacket(120, 0xFF)

// Generic constructor with explicit Protocol ID
packet, err := epp.NewPacket(epp.ProtocolIDIPE, data)
```

`NewPacket` picks the smallest header that fits the data. Passing more data
than an 8-bit length field can describe automatically selects the 4-octet
header, and so on.

### Packet Options

Options configure the header size and optional fields:

```go
// Force at least a 4-octet header (2-octet length field)
packet, err := epp.NewIPEPacket(data, epp.WithLongLength())

// Set the 4-bit User Defined Field (needs a 4- or 8-octet header)
packet, err := epp.NewMissionPacket(data, epp.WithUserDefined(0xA))

// Extended Protocol ID: sets PID '110' and the 4-bit extension value
packet, err := epp.NewPacket(epp.ProtocolIDExtended, data,
    epp.WithExtendedProtocolID(9),
)

// CCSDS Defined Field (forces the 8-octet header)
packet, err := epp.NewPacket(epp.ProtocolIDExtended, data,
    epp.WithExtendedProtocolID(9),
    epp.WithCCSDSDefined(0x1234),
)
```

### Inspecting Packets

```go
// Check if a packet is an idle packet
if packet.IsIdle() { ... }

// Human-readable dump for debugging
fmt.Println(packet.Humanize())
```

### Packet Sizing

The `PacketSizer` function returns the total packet length from the header bytes of an Encapsulation Packet. It implements the `sdl.PacketSizer` signature, allowing EPP packets to be extracted from fixed-length transfer frames by the `tmdl` and `tcdl` service layers:

```go
// PacketSizer returns the total packet length from header bytes.
totalLen := epp.PacketSizer(headerBytes)
```

## Encoding and Decoding

```go
// Encode a packet to bytes for transmission
encoded, err := packet.Encode()

// Decode bytes back into a packet
decoded, err := epp.Decode(encoded)

// Access decoded fields
fmt.Println(decoded.Header.ProtocolID)
fmt.Println(decoded.Header.Size())
fmt.Println(decoded.Data)
```

## Header Layout

The Encapsulation Packet uses a variable-length header of 1, 2, 4, or 8
octets. The size is a pure function of the 2-bit Length of Length (LoL)
field in the first byte (CCSDS 133.1-B-3 table 4-1):

```
Octet 0 (always present):
+---+---+---+---+---+---+---+---+
| 1   1   1 | P   I   D | L o L |
| (PVN = 7) | (3 bits)  |(2 bit)|
+---+---+---+---+---+---+---+---+
 MSB                         LSB
```

### LoL '00' — 1-octet header (idle only)

No Packet Length field and no data zone. The Protocol ID must be '000'
(idle), so the whole packet is the single byte `0xE0`.

```
+--------+
| Octet0 |
+--------+
```

### LoL '01' — 2-octet header

8-bit Packet Length, max 255 octets total.

```
+--------+------------------+
| Octet0 | Packet Length 8b |
+--------+------------------+
```

### LoL '10' — 4-octet header

Octet 1 carries the 4-bit User Defined Field and the 4-bit Protocol ID
Extension. 16-bit Packet Length, max 65,535 octets total.

```
+--------+-----------+-----------+---------------------+
| Octet0 | UDF (4b)  | PIE (4b)  | Packet Length 16b   |
+--------+-----------+-----------+---------------------+
```

### LoL '11' — 8-octet header

Adds the 2-octet CCSDS Defined Field (reserved, 'all zeros' by convention)
and a 32-bit Packet Length, max 4,294,967,295 octets total.

```
+--------+-----------+-----------+----------------+---------------------+
| Octet0 | UDF (4b)  | PIE (4b)  | CCSDS Defined  | Packet Length 32b   |
+--------+-----------+-----------+----------------+---------------------+
```

### Protocol IDs

Per CCSDS 133.1-B-3 4.1.2.3 and the SANA Encapsulation Protocol ID registry:

| Value | Name | Description |
|-------|------|-------------|
| 0 | Idle | Encapsulation Idle Packet (fill data) |
| 1 | LTP | Licklider Transmission Protocol (CCSDS 734.1) |
| 2 | IPE | Internet Protocol Extension (IPv4/IPv6) |
| 3–5 | Reserved | Reserved for future use |
| 6 | Extended | Protocol identified by the 4-bit Protocol ID Extension |
| 7 | Mission | Mission-specific, privately defined data |

The Protocol ID Extension field must be zero unless the Protocol ID is 6
('110'), and Protocol ID 6 requires a 4- or 8-octet header so the extension
field exists.

### Packet Length

The Packet Length field contains the total number of octets in the entire Encapsulation Packet, including the header (4.1.2.8.2). This differs from SPP, where the Packet Data Length field contains the data field size minus 1.

### Idle Packets

Protocol ID 0 marks an idle packet. The 1-octet form (`0xE0`) has no data.
Multi-octet idle packets carry mission-defined fill data and are useful for
filling the remainder of a fixed-length transfer frame; build them with
`NewIdleFillPacket(totalLength, fillByte)`. A packet whose length equals its
header size (no data field) is only legal with the idle Protocol ID.

## Full Pipeline Example

### Send Path

```go
// Create an EPP packet carrying an IPv4 datagram
packet, err := epp.NewIPEPacket(ipv4Datagram)

// Encode to bytes
encoded, err := packet.Encode()

// Frame in a TM Transfer Frame (via tmdl)
frame, err := tmdl.NewTMTransferFrame(0x1A, 1, encoded, nil, nil)
```

### Receive Path

```go
// Extract packet bytes from a transfer frame
data := frame.DataField

// Decode the Encapsulation Packet
packet, err := epp.Decode(data)

// Access the encapsulated datagram
fmt.Printf("Protocol ID: %d\n", packet.Header.ProtocolID)
fmt.Printf("Data: %x\n", packet.Data)
```

## Errors

All errors are exported package-level variables, suitable for use with `errors.Is`:

| Error | Meaning |
|-------|---------|
| `ErrInvalidPVN` | PVN is not 7 ('111') |
| `ErrInvalidProtocolID` | Protocol ID outside 0–7 |
| `ErrInvalidLengthOfLength` | Length of Length field outside 0–3 |
| `ErrInvalidUserDefined` | User Defined Field does not fit in 4 bits |
| `ErrInvalidExtendedProtocolID` | Protocol ID Extension does not fit in 4 bits |
| `ErrNonIdleOneOctetHeader` | 1-octet header with a non-idle Protocol ID |
| `ErrExtendedNeedsLongHeader` | Protocol ID '110' with a header under 4 octets |
| `ErrExtensionMustBeZero` | Non-zero extension with a Protocol ID other than '110' |
| `ErrFieldNeedsLongerHeader` | Field set that the selected header size cannot carry |
| `ErrIdleWithData` | 1-octet idle packet given a data zone |
| `ErrEmptyData` | Non-idle packet has no data |
| `ErrDataTooShort` | Input data too short to decode |
| `ErrPacketLengthMismatch` | Packet length field doesn't match actual size |
| `ErrPacketTooLarge` | Packet exceeds maximum for header format |
| `ErrInvalidIdleLength` | Idle fill packet requested with total length < 1 |
| `ErrNilPacket` | Nil packet provided |

## Reference

- [CCSDS 133.1-B-3](https://public.ccsds.org/Pubs/133x1b3e1.pdf) — Encapsulation Packet Protocol Blue Book (Issue 3, May 2020)
- [CCSDS 133.0-B-2](https://public.ccsds.org/Pubs/133x0b2e2.pdf) — Space Packet Protocol Blue Book (companion protocol)
- [SANA Encapsulation Protocol ID registry](https://sanaregistry.org/r/protocol_id/)
