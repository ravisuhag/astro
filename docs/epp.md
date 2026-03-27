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

The first four bits of an Encapsulation Packet are always `0111` (PVN = 7), which distinguishes it from a Space Packet (PVN = 0). This allows both packet types to coexist on the same data link.

## Service Layer

The `Service` type provides send/receive operations over an `io.ReadWriter` transport:

```go
svc := epp.NewService(conn, epp.ServiceConfig{
    MaxPacketLength: 65535, // optional, defaults to 65535
})
```

### Byte-Level Service

The simplest way to send and receive data. The service wraps your bytes in a valid encapsulation packet automatically:

```go
// Send an IPv4 datagram
err := svc.SendBytes(epp.ProtocolIDIPE, ipv4Datagram)

// Send with extended protocol ID
err := svc.SendBytes(epp.ProtocolIDExtended, data,
    epp.WithExtendedProtocolID(42),
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

// User-defined protocol packet
packet, err := epp.NewUserDefinedPacket(payload)

// Idle packet (fill data, 1-byte header, no payload)
packet, err := epp.NewIdlePacket()

// Generic constructor with explicit Protocol ID
packet, err := epp.NewPacket(epp.ProtocolIDIPE, data)
```

### Packet Options

Options configure the header format and optional fields:

```go
// Force 4-byte header with 16-bit length field (Format 3)
packet, err := epp.NewIPEPacket(data, epp.WithLongLength())

// Set the user-defined field (Format 3, also sets LengthOfLength=1)
packet, err := epp.NewUserDefinedPacket(data, epp.WithUserDefined(0xAB))

// Extended Protocol ID (Format 4)
packet, err := epp.NewPacket(epp.ProtocolIDExtended, data,
    epp.WithExtendedProtocolID(42),
)

// Extended Protocol ID with CCSDS-defined field (Format 5, 8-byte header)
packet, err := epp.NewPacket(epp.ProtocolIDExtended, data,
    epp.WithCCSDSDefined(42, 0x1234),
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
fmt.Println(decoded.Header.Format())
fmt.Println(decoded.Data)
```

## Header Formats

The Encapsulation Packet uses a variable-length header. The format is determined by the Protocol ID and Length of Length (LoL) fields in the first byte:

```
Octet 0 (always present):
+---+---+---+---+---+---+---+---+
| 0   1   1   1 | P   I   D | L |
| (PVN = 7)     | (3 bits)  |oL |
+---+---+---+---+---+---+---+---+
  7   6   5   4   3   2   1   0
```

### Format 1 — Idle (1 byte)

PID = 0, LoL = 0. No packet length field, no data zone.

```
+--------+
| Octet0 |
+--------+
```

### Format 2 — Short (2 bytes)

PID = 1–6, LoL = 0. 8-bit packet length, max 255 bytes total.

```
+--------+------------------+
| Octet0 | Packet Length 8b |
+--------+------------------+
```

### Format 3 — Medium (4 bytes)

PID = 1–6, LoL = 1. Includes a user-defined field and 16-bit packet length, max 65,535 bytes total.

```
+--------+--------------+---------------------+
| Octet0 | User Defined | Packet Length 16b   |
+--------+--------------+---------------------+
```

### Format 4 — Extended Medium (4 bytes)

PID = 7, LoL = 0. Extended Protocol ID and 16-bit packet length, max 65,535 bytes total.

```
+--------+----------+---------------------+
| Octet0 | Ext PID  | Packet Length 16b   |
+--------+----------+---------------------+
```

### Format 5 — Extended Long (8 bytes)

PID = 7, LoL = 1. Extended Protocol ID, CCSDS-defined field, and 32-bit packet length, max 4,294,967,295 bytes total.

```
+--------+----------+----------------+---------------------+
| Octet0 | Ext PID  | CCSDS Defined  | Packet Length 32b   |
+--------+----------+----------------+---------------------+
```

### Protocol IDs

| Value | Name | Description |
|-------|------|-------------|
| 0 | Idle | Fill/idle packet (Format 1 only) |
| 1 | Reserved | Reserved for future use |
| 2 | IPE | Internet Protocol Extension (IPv4/IPv6) |
| 3–5 | Reserved | Reserved for future use |
| 6 | User-Defined | Mission-specific protocol |
| 7 | Extended | Protocol ID Extension (extended PID in next byte) |

### Packet Length

The Packet Length field contains the total number of octets in the entire Encapsulation Packet, including the header. This differs from SPP, where the Packet Data Length field contains the data field size minus 1.

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
| `ErrInvalidPVN` | PVN is not 7 |
| `ErrInvalidProtocolID` | Protocol ID outside 0–7 |
| `ErrInvalidLengthOfLength` | Length of Length field is not 0 or 1 |
| `ErrIdleWithData` | Idle packet created with non-empty data zone |
| `ErrEmptyData` | Non-idle packet has no data |
| `ErrDataTooShort` | Input data too short to decode |
| `ErrPacketLengthMismatch` | Packet length field doesn't match actual size |
| `ErrPacketTooLarge` | Packet exceeds maximum for header format |
| `ErrNilPacket` | Nil packet provided |

## Reference

- [CCSDS 133.1-B-3](https://public.ccsds.org/Pubs/133x1b3e1.pdf) — Encapsulation Packet Protocol Blue Book
- [CCSDS 133.0-B-2](https://public.ccsds.org/Pubs/133x0b2e2.pdf) — Space Packet Protocol Blue Book (companion protocol)
